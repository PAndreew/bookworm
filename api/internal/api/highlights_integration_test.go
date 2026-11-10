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

func TestReaderHighlightsBookScopeAndCanonicalQuote(t *testing.T) {
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
	router := New(&config.Config{}, st).Routes()
	suffix := fmt.Sprint(time.Now().UnixNano())
	tokens := []string{}
	for i := 0; i < 2; i++ {
		handle := fmt.Sprintf("hl%d%s", i, suffix)
		u, err := st.CreateUser(ctx, handle+"@example.invalid", handle, "Highlight QA", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
		token, hash, _ := auth.NewSessionToken()
		if err = st.CreateSession(ctx, u.ID, hash, false, "test", "", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
	}
	slugs, pages, quotes := []string{}, []string{}, []string{}
	for i := 0; i < 2; i++ {
		slug := fmt.Sprintf("highlights-%s-%d", suffix, i)
		if _, err = ingest.New(pool).Publish(ctx, ingest.Book{Slug: slug, Title: "Highlight fixture", Author: suffix, License: "public_domain", Pages: chunk.Split(strings.Repeat("A 😀 Unicode sentence for testing saved highlights. ", 12))}); err != nil {
			t.Fatal(err)
		}
		b, err := st.BookBySlug(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Exec(ctx, `DELETE FROM books WHERE id=$1`, b.ID)
		var id, text string
		if err = pool.QueryRow(ctx, `SELECT id,content FROM pages WHERE edition_id=$1 ORDER BY idx LIMIT 1`, b.EditionID).Scan(&id, &text); err != nil {
			t.Fatal(err)
		}
		slugs = append(slugs, slug)
		pages = append(pages, id)
		quotes = append(quotes, string([]rune(text)[2:20]))
	}
	call := func(method, path, body, token string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec
	}
	call("GET", "/v1/highlights?book="+slugs[0], "", "", 401)
	for i, page := range pages {
		var result struct {
			Highlight store.Highlight `json:"highlight"`
		}
		rec := call("POST", "/v1/highlights", fmt.Sprintf(`{"page_id":%q,"start_off":2,"end_off":20,"quote":"fabricated client quote","share":false}`, page), tokens[0], 201)
		if err = json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Highlight.Quote != quotes[i] {
			t.Fatalf("canonical quote mismatch: %+v %v", result, err)
		}
	}
	for i, slug := range slugs {
		var result struct {
			Highlights []store.Highlight `json:"highlights"`
		}
		rec := call("GET", "/v1/highlights?book="+slug, "", tokens[0], 200)
		if rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("private marks are cacheable")
		}
		if err = json.Unmarshal(rec.Body.Bytes(), &result); err != nil || len(result.Highlights) != 1 || result.Highlights[0].PageID != pages[i] {
			t.Fatalf("book scope: %+v %v", result, err)
		}
		rec = call("GET", "/v1/highlights?book="+slug, "", tokens[1], 200)
		json.Unmarshal(rec.Body.Bytes(), &result)
		if len(result.Highlights) != 0 {
			t.Fatal("other account's marks leaked")
		}
	}
	var all struct {
		Highlights []store.Highlight `json:"highlights"`
	}
	json.Unmarshal(call("GET", "/v1/highlights", "", tokens[0], 200).Body.Bytes(), &all)
	if len(all.Highlights) != 2 {
		t.Fatal("unfiltered quote library regressed")
	}
}
