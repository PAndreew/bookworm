package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bookworm/api/internal/auth"
	"github.com/bookworm/api/internal/config"
	"github.com/bookworm/api/internal/db"
	"github.com/bookworm/api/internal/store"
)

func TestBYOKMetadataAndAtomicQuota(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable TEST_DATABASE_URL")
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
	st := store.New(pool)
	handle := fmt.Sprintf("byok%d", time.Now().UnixNano())
	user, err := st.CreateUser(ctx, handle+"@example.invalid", handle, "BYOK Test", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
	token, hash, _ := auth.NewSessionToken()
	if err = st.CreateSession(ctx, user.ID, hash, false, "test", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	router := New(&config.Config{}, st).Routes()
	call := func(method, path, body string, signed bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if signed {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if r := call("POST", "/v1/byok/reservations", "{}", false); r.Code != 401 {
		t.Fatalf("anonymous: %d", r.Code)
	}
	for _, body := range []string{`{"key":"do-not-store"}`, `{"phrase":"private"}`, `{"provider":"openai"}`, `null`, `{} {}`} {
		r := call("POST", "/v1/byok/reservations", body, true)
		if r.Code != 400 {
			t.Fatalf("unexpected body accepted %q: %d", body, r.Code)
		}
	}
	if r := call("GET", "/v1/byok/quota", "", true); r.Code != 200 || !strings.Contains(r.Body.String(), `"used":0`) || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("initial quota: %s", r.Body.String())
	}
	var success atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := st.ReserveBYOKAttempt(ctx, user.ID)
			if err == nil {
				success.Add(1)
			} else if err != store.ErrBYOKLimit {
				t.Errorf("reserve: %v", err)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 50 {
		t.Fatalf("got %d reservations", success.Load())
	}
	if r := call("POST", "/v1/byok/reservations", "{}", true); r.Code != 429 {
		t.Fatalf("exhausted: %d %s", r.Code, r.Body.String())
	}
	q, err := st.BYOKQuota(ctx, user.ID)
	if err != nil || q.Used != 50 || q.Limit != 50 || q.ResetsAt.Day() != 1 || !q.ResetsAt.After(time.Now()) {
		t.Fatalf("quota: %+v %v", q, err)
	}
	var paid int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM assistant_monthly_usage WHERE user_id=$1`, user.ID).Scan(&paid); err != nil || paid != 0 {
		t.Fatal("BYOK changed paid allowance", err)
	}
	// Previous-month usage must not reduce the current allowance.
	_, err = pool.Exec(ctx, `UPDATE byok_monthly_usage SET month=(date_trunc('month',now() AT TIME ZONE 'UTC')-interval '1 month')::date WHERE user_id=$1`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r := call("POST", "/v1/byok/reservations", "{}", true); r.Code != 200 || !strings.Contains(r.Body.String(), `"used":1`) {
		t.Fatalf("new month: %d %s", r.Code, r.Body.String())
	}
}
