package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bookworm/api/internal/auth"
	"github.com/bookworm/api/internal/chunk"
	"github.com/bookworm/api/internal/config"
	"github.com/bookworm/api/internal/db"
	"github.com/bookworm/api/internal/ingest"
	"github.com/bookworm/api/internal/store"
)

// Run against a disposable PostgreSQL database, never production:
// TEST_DATABASE_URL=postgres://... go test ./internal/api -run TestRatingsAndGenres
func TestRatingsAndGenres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	router := New(&config.Config{}, st).Routes()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	tokens := []string{}
	for i := 0; i < 2; i++ {
		handle := fmt.Sprintf("r%d%s", i, suffix)
		u, err := st.CreateUser(ctx, handle+"@example.com", handle, "Test", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
		token, hash, err := auth.NewSessionToken()
		if err != nil {
			t.Fatal(err)
		}
		if err := st.CreateSession(ctx, u.ID, hash, false, "test", "", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
	}
	book := ingest.Book{Slug: "rating-test-" + suffix, Title: "Test Book", Author: "Test Author", Language: "en", License: "public_domain", Genres: []string{"fantasy", "adventure"}, Pages: chunk.Split("A short test book with immutable page anchors.")}
	ing := ingest.New(pool)
	if _, err := ing.Publish(ctx, book); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM books WHERE slug=$1`, book.Slug)
	stored, err := st.BookBySlug(ctx, book.Slug)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := st.PageWindow(ctx, stored.EditionID, "", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Genres) != 2 {
		t.Fatalf("missing genres: %v", stored.Genres)
	}
	// Metadata refresh must not reset genres or immutable pages.
	book.Genres = nil
	result, err := ing.Publish(ctx, book)
	if err != nil || result.Published {
		t.Fatalf("metadata refresh replaced edition: %v", err)
	}
	again, err := st.BookBySlug(ctx, book.Slug)
	if err != nil || len(again.Genres) != 2 {
		t.Fatalf("genres not preserved: %v", err)
	}
	after, err := st.PageWindow(ctx, again.EditionID, "", 0, 10)
	if err != nil || after[0].ID != pages[0].ID {
		t.Fatal("page anchors changed")
	}
	if _, err := pool.Exec(ctx, `UPDATE books SET genres=$2 WHERE id=$1`, stored.ID, []string{"a", "b", "c", "d", "e", "f"}); err == nil {
		t.Fatal("database accepted six genres")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO book_ratings (user_id,book_id,stars) SELECT id,$1,6 FROM users WHERE handle LIKE $2 LIMIT 1`, stored.ID, "r0"+suffix); err == nil {
		t.Fatal("database accepted six stars")
	}
	call := func(method, path, body, token string, status int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s: %d instead of %d: %s", method, path, rec.Code, status, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	endpoint := "/v1/books/" + book.Slug + "/rating"
	call("PUT", endpoint, `{"stars":3}`, "", 401)
	for _, body := range []string{`{"stars":0}`, `{"stars":6}`, `{"stars":1.5}`, `{"stars":"5"}`, `{}`} {
		call("PUT", endpoint, body, tokens[0], 400)
	}
	call("PUT", "/v1/books/nonexistent/rating", `{"stars":3}`, tokens[0], 404)
	for stars := 1; stars <= 5; stars++ {
		call("PUT", endpoint, fmt.Sprintf(`{"stars":%d}`, stars), tokens[0], 200)
		var response struct {
			Ratings map[string]int `json:"ratings"`
		}
		if err := json.Unmarshal(call("GET", "/v1/me/ratings", "", tokens[0], 200), &response); err != nil {
			t.Fatal(err)
		}
		if response.Ratings[book.Slug] != stars {
			t.Fatal("rating was not saved or updated")
		}
	}
	var other struct {
		Ratings map[string]int `json:"ratings"`
	}
	json.Unmarshal(call("GET", "/v1/me/ratings", "", tokens[1], 200), &other)
	if len(other.Ratings) != 0 {
		t.Fatal("another user's ratings leaked")
	}
	call("GET", "/v1/me/ratings", "", "", 401)
}
