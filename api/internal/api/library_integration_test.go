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

func TestPersonalLibraryAndPositions(t *testing.T) {
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
	users := []*store.User{}
	tokens := []string{}
	for i := 0; i < 2; i++ {
		handle := fmt.Sprintf("lib%d%s", i, suffix)
		u, err := st.CreateUser(ctx, handle+"@example.com", handle, "Library", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
		token, hash, err := auth.NewSessionToken()
		if err != nil {
			t.Fatal(err)
		}
		if err = st.CreateSession(ctx, u.ID, hash, false, "test", "", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		users = append(users, u)
		tokens = append(tokens, token)
	}
	books := []*store.Book{}
	for i, title := range []string{"A old", "B new", "Z latest"} {
		slug := fmt.Sprintf("lib-%s-%d", suffix, i)
		_, err := ingest.New(pool).Publish(ctx, ingest.Book{Slug: slug, Title: title, Author: suffix, License: "public_domain", Genres: []string{"fiction"}, Pages: chunk.Split(strings.Repeat("A sentence for the library fixture. ", 80*(i+1)))})
		if err != nil {
			t.Fatal(err)
		}
		b, err := st.BookBySlug(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		books = append(books, b)
		defer pool.Exec(ctx, `DELETE FROM books WHERE id=$1`, b.ID)
		if err = st.EnsureReaderCards(ctx, b.EditionID); err != nil {
			t.Fatal(err)
		}
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
			t.Fatalf("%s %s: %d want %d: %s", method, path, rec.Code, status, rec.Body.String())
		}
		if status == 200 && strings.Contains(path, "/books?") && rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("personal catalogue is cacheable")
		}
		return rec.Body.Bytes()
	}
	list := func(query, token string) []store.LibraryBook {
		t.Helper()
		var response struct {
			Books []store.LibraryBook `json:"books"`
		}
		if err := json.Unmarshal(call("GET", "/v1/books?q="+suffix+query, "", token, 200), &response); err != nil {
			t.Fatal(err)
		}
		return response.Books
	}
	cards, _, err := st.CardWindow(ctx, books[2].EditionID, users[0].ID, 0, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "/v1/books/" + books[2].Slug + "/position"
	call("POST", endpoint, fmt.Sprintf(`{"card_id":%q}`, cards[0].CardID), tokens[0], 200)
	call("POST", endpoint, fmt.Sprintf(`{"card_id":%q}`, cards[2].CardID), tokens[0], 200)
	var position struct {
		Position *store.ReadingPosition `json:"position"`
	}
	json.Unmarshal(call("GET", endpoint, "", tokens[0], 200), &position)
	if position.Position == nil || position.Position.CardIdx == nil || *position.Position.CardIdx != cards[2].Idx {
		t.Fatal("per-book position not saved")
	}
	var other struct {
		Position *store.ReadingPosition `json:"position"`
	}
	json.Unmarshal(call("GET", endpoint, "", tokens[1], 200), &other)
	if other.Position != nil {
		t.Fatal("position leaked to another reader")
	}
	call("GET", endpoint, "", "", 401)
	call("POST", endpoint, `{"card_id":"not-a-uuid"}`, tokens[0], 400)
	call("POST", "/v1/books/"+books[0].Slug+"/position", fmt.Sprintf(`{"card_id":%q}`, cards[2].CardID), tokens[0], 404)
	if _, err = pool.Exec(ctx, `INSERT INTO reading_progress(user_id,edition_id,last_page_idx,furthest_idx,last_read_at) VALUES($1,$2,0,0,now()-interval '2 months')`, users[0].ID, books[0].EditionID); err != nil {
		t.Fatal(err)
	}
	got := list("", tokens[0])
	if len(got) != 3 || got[0].Slug != books[2].Slug || !got[0].Latest || got[1].Slug != books[0].Slug {
		t.Fatalf("recent ordering: %+v", got)
	}
	got = list("&limit=1", tokens[0])
	if len(got) != 1 || got[0].Slug != books[2].Slug {
		t.Fatal("recency applied after pagination")
	}
	got = list("&mine=true", tokens[0])
	if len(got) != 2 {
		t.Fatal("My books must include old opened books, excluding unopened books")
	}
	if len(list("&mine=true", tokens[1])) != 0 {
		t.Fatal("other reader's books leaked")
	}
	call("GET", "/v1/books?mine=true", "", "", 401)
	got = list("", "")
	for _, b := range got {
		if b.LastReadAt != nil || b.LastCardIdx != nil || b.Latest {
			t.Fatal("anonymous library leaked reading history")
		}
	}
	if err = st.RateBook(ctx, users[0].ID, books[0].ID, 5); err != nil {
		t.Fatal(err)
	}
	if err = st.RateBook(ctx, users[0].ID, books[1].ID, 4); err != nil {
		t.Fatal(err)
	}
	if err = st.RateBook(ctx, users[1].ID, books[1].ID, 4); err != nil {
		t.Fatal(err)
	}
	if got = list("&sort=rating", tokens[0]); got[0].Slug != books[0].Slug || got[0].RatingAverage != 5 {
		t.Fatal("average-rating sort failed")
	}
	if got = list("&sort=reviews", tokens[0]); got[0].Slug != books[1].Slug || got[0].RatingCount != 2 {
		t.Fatal("review-count sort failed")
	}
	for _, sort := range []string{"shortest", "longest"} {
		got = list("&sort="+sort, tokens[0])
		for i := 1; i < len(got); i++ {
			if (sort == "shortest" && got[i-1].PageCount > got[i].PageCount) || (sort == "longest" && got[i-1].PageCount < got[i].PageCount) {
				t.Fatal("length sort failed")
			}
		}
	}
	call("GET", "/v1/books?sort=invalid", "", "", 400)
	pages, err := st.PagesReadToday(ctx, users[0].ID, store.LocalDay("UTC", time.Now()))
	if err != nil || pages != 0 {
		t.Fatal("navigation earned streak credit", pages, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE books SET license='licensed' WHERE id=$1`, books[2].ID); err != nil {
		t.Fatal(err)
	}
	call("POST", endpoint, fmt.Sprintf(`{"card_id":%q}`, cards[0].CardID), tokens[0], 402)
	if _, err = pool.Exec(ctx, `UPDATE users SET tier='pro' WHERE id=$1`, users[0].ID); err != nil {
		t.Fatal(err)
	}
	call("POST", endpoint, fmt.Sprintf(`{"card_id":%q}`, cards[0].CardID), tokens[0], 200)
	used, err := st.LicensedUsage(ctx, users[0].ID, store.TierPro, "UTC", time.Now())
	if err != nil || used != 0 {
		t.Fatal("navigation consumed quota", used, err)
	}
}
