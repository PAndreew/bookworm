package store

import (
	"context"
	"github.com/bookworm/api/internal/auth"
	"github.com/bookworm/api/internal/db"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Use a disposable database: BW_EMAIL_TEST_DATABASE_URL=... go test ./internal/store -run TestEmail
func TestEmailFlows(t *testing.T) {
	url := os.Getenv("BW_EMAIL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable BW_EMAIL_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := New(pool)
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	suffix := strings.ToLower(token[:10])
	u, err := st.CreateUser(ctx, suffix+"@example.com", suffix, suffix, "old", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM email_outbox WHERE recipient=$1`, u.Email)
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
	queue := func(p string, h []byte, ttl time.Duration) {
		t.Helper()
		if err := st.QueueAuthEmail(ctx, u.ID, p, h, ttl, u.Email, "Test", "body"); err != nil {
			t.Fatal(err)
		}
	}
	queue("verify", hash, 24*time.Hour)
	_, secondHash, _ := auth.NewSessionToken()
	queue("verify", secondHash, 24*time.Hour)
	if err = st.ConsumeEmailToken(ctx, secondHash, "verify", ""); err != ErrNotFound {
		t.Fatalf("cooldown token accepted: %v", err)
	}
	if err = st.ConsumeEmailToken(ctx, hash, "reset", "new"); err != ErrNotFound {
		t.Fatalf("wrong purpose accepted: %v", err)
	}
	if err = st.ConsumeEmailToken(ctx, hash, "verify", ""); err != nil {
		t.Fatal(err)
	}
	if err = st.ConsumeEmailToken(ctx, hash, "verify", ""); err != ErrNotFound {
		t.Fatalf("replay accepted: %v", err)
	}
	u, err = st.UserByID(ctx, u.ID)
	if err != nil || !u.EmailVerified {
		t.Fatal("email not verified", err)
	}
	_, resetHash, _ := auth.NewSessionToken()
	queue("reset", resetHash, 30*time.Minute)
	_, sessionHash, _ := auth.NewSessionToken()
	if err = st.CreateSession(ctx, u.ID, sessionHash, false, "test", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = st.ConsumeEmailToken(ctx, resetHash, "reset", "newhash"); err != nil {
		t.Fatal(err)
	}
	u, err = st.UserByID(ctx, u.ID)
	if err != nil || u.PasswordHash() != "newhash" {
		t.Fatal("password not updated", err)
	}
	var active int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id=$1 AND revoked_at IS NULL`, u.ID).Scan(&active); err != nil || active != 0 {
		t.Fatal("sessions not revoked", err)
	}
	_, expiredHash, _ := auth.NewSessionToken()
	queue("reset", expiredHash, -time.Minute)
	if err = st.ConsumeEmailToken(ctx, expiredHash, "reset", "bad"); err != ErrNotFound {
		t.Fatalf("expired token accepted: %v", err)
	}
	// Exercise durable retries against a fake Resend transport; never send actual mail.
	var failDelivery atomic.Bool
	failDelivery.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" || r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing provider headers")
		}
		if failDelivery.Load() {
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	client := server.Client()
	client.Transport = redirectTransport{base: client.Transport, url: server.URL}
	if err = st.sendNextEmail(ctx, client, "test", "test@example.com"); err == nil {
		t.Fatal("provider failure not reported")
	}
	var retried int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE recipient=$1 AND attempts=1 AND next_attempt>now()`, u.Email).Scan(&retried); err != nil || retried != 1 {
		t.Fatal("failed email not retained for retry", err)
	}
	failDelivery.Store(false)
	if _, err = pool.Exec(ctx, `UPDATE email_outbox SET next_attempt=now() WHERE recipient=$1`, u.Email); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = st.sendNextEmail(ctx, client, "test", "test@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	var remaining int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE recipient=$1`, u.Email).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatal("delivered mail retained", err)
	}
}

type redirectTransport struct {
	base http.RoundTripper
	url  string
}

func (rt redirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, rt.url, r.Body)
	if err != nil {
		return nil, err
	}
	req.Header = r.Header
	return rt.base.RoundTrip(req)
}
