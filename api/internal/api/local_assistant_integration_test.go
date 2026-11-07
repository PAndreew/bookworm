package api

import (
	"context"
	"encoding/json"
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
	"github.com/bookworm/api/internal/chunk"
	"github.com/bookworm/api/internal/config"
	"github.com/bookworm/api/internal/db"
	"github.com/bookworm/api/internal/ingest"
	"github.com/bookworm/api/internal/store"
)

func TestLocalAssistantAllowance(t *testing.T) {
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
	suffix := fmt.Sprint(time.Now().UnixNano())
	user, err := st.CreateUser(ctx, "local"+suffix+"@example.invalid", "l"+suffix, "Local test", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
	token, hash, _ := auth.NewSessionToken()
	if err = st.CreateSession(ctx, user.ID, hash, false, "test", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	fake := &fakeAssistant{}
	server := New(&config.Config{}, st)
	server.assistant = fake
	router := server.Routes()
	call := func(method, body, session string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/v1/assistant/local", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if session != "" {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session})
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if got := call("GET", "", "").Code; got != 401 {
		t.Fatalf("unauthenticated status %d", got)
	}
	usage := func() store.LocalAssistantUsage {
		t.Helper()
		rec := call("GET", "", token)
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body.String())
		}
		var u store.LocalAssistantUsage
		if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable allowance")
		}
		return u
	}
	if u := usage(); u.Limit != 10 || u.Used != 0 || u.ResetsAt.Day() != 1 {
		t.Fatalf("initial usage %+v", u)
	}
	if got := call("POST", `{"card_id":"not-a-uuid"}`, token).Code; got != 400 {
		t.Fatal(got)
	}
	if usage().Used != 0 {
		t.Fatal("invalid source charged")
	}
	book := ingest.Book{Slug: "local-" + suffix, Title: "Local fixture", Author: "Tester", License: "public_domain", Pages: chunk.Split(strings.Repeat("A short literary reading passage. ", 30))}
	if _, err = ingest.New(pool).Publish(ctx, book); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM books WHERE slug=$1`, book.Slug)
	stored, err := st.BookBySlug(ctx, book.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureReaderCards(ctx, stored.EditionID); err != nil {
		t.Fatal(err)
	}
	var cardID string
	if err = pool.QueryRow(ctx, `SELECT c.id FROM reader_cards c JOIN book_editions e ON e.id=c.edition_id JOIN books b ON b.id=e.book_id WHERE b.slug=$1 AND c.kind='text' LIMIT 1`, book.Slug).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"card_id":%q}`, cardID)
	var success atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code := call("POST", body, token).Code
			if code == 200 {
				success.Add(1)
			} else if code != 429 {
				t.Errorf("concurrent status %d", code)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 || usage().Used != 1 {
		t.Fatal("cooldown not atomic", success.Load())
	}
	if fake.calls.Load() != 0 {
		t.Fatal("local allowance contacted provider")
	}
	var cloudCount int
	if err = pool.QueryRow(ctx, `SELECT COALESCE((SELECT requests FROM assistant_monthly_usage WHERE user_id=$1),0)`, user.ID).Scan(&cloudCount); err != nil || cloudCount != 0 {
		t.Fatal("local consumed cloud quota", err)
	}
	_, err = pool.Exec(ctx, `UPDATE assistant_local_usage SET requests=10,last_request_at=now()-interval '10 seconds' WHERE user_id=$1`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if code := call("POST", body, token).Code; code != 429 {
		t.Fatal("free exhaustion", code)
	}
	_, err = pool.Exec(ctx, `UPDATE users SET tier='plus' WHERE id=$1`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if usage().Limit != 100 {
		t.Fatal("upgrade did not increase allowance")
	}
	if code := call("POST", body, token).Code; code != 200 {
		t.Fatal("plus reservation", code)
	}
	_, err = pool.Exec(ctx, `UPDATE users SET tier='pro' WHERE id=$1`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if usage().Limit != 1000 {
		t.Fatal("pro limit")
	}
	_, err = pool.Exec(ctx, `UPDATE assistant_local_usage SET month=month-interval '1 month' WHERE user_id=$1`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if usage().Used != 0 {
		t.Fatal("monthly reset")
	}
	// Hosted AI remains paid-only; no free cloud fallback was introduced.
	_, err = pool.Exec(ctx, `UPDATE users SET tier='free' WHERE id=$1`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/v1/assistant", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 402 || fake.calls.Load() != 0 {
		t.Fatal("free cloud boundary changed")
	}
}
