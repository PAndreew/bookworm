package api

import (
	"context"
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

func TestCardReportValidation(t *testing.T) {
	id := "12345678-1234-1234-1234-123456789abc"
	for _, category := range []string{"layout", "text", "break"} {
		if !validCardReport(id, cardReportRequest{category, strings.Repeat("😀", 1000)}) {
			t.Fatal("valid report rejected")
		}
	}
	for _, req := range []cardReportRequest{{"unknown", ""}, {"layout", strings.Repeat("x", 1001)}} {
		if validCardReport(id, req) {
			t.Fatal("invalid report accepted")
		}
	}
	if validCardReport("not-a-uuid", cardReportRequest{"layout", ""}) {
		t.Fatal("invalid UUID accepted")
	}
}
func TestCardReportsAndCrossSourceLayout(t *testing.T) {
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
	user, err := st.CreateUser(ctx, "reports"+suffix+"@example.com", "p"+suffix, "Reporter", "hash", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
	token, hash, _ := auth.NewSessionToken()
	if err = st.CreateSession(ctx, user.ID, hash, false, "test", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	pending, pendingHash, _ := auth.NewSessionToken()
	st.CreateSession(ctx, user.ID, pendingHash, true, "test", "", time.Now().Add(time.Hour))
	var public, licensed *store.Book
	for _, license := range []string{"public_domain", "licensed"} {
		slug := "report-" + license + "-" + suffix
		_, err = ingest.New(pool).Publish(ctx, ingest.Book{Slug: slug, Title: "Report Fixture", Author: "Tester", License: license, Pages: []chunk.Page{{Idx: 0, Content: strings.Repeat("word ", 60) + "Mr.", ChapterTitle: "One", SHA256: []byte("a")}, {Idx: 1, Content: "Bingley " + strings.Repeat("said some familiar words. ", 400), ChapterTitle: "One", SHA256: []byte("b")}}})
		if err != nil {
			t.Fatal(err)
		}
		b, e := st.BookBySlug(ctx, slug)
		if e != nil {
			t.Fatal(e)
		}
		defer pool.Exec(ctx, `DELETE FROM books WHERE id=$1`, b.ID)
		if license == "public_domain" {
			public = b
		} else {
			licensed = b
		}
	}
	cards, _, err := st.CardWindow(ctx, public.EditionID, user.ID, 0, 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cards {
		if strings.Contains(c.Content, "Mr. Bingley") {
			found = true
			if len(c.Parts) != 2 {
				t.Fatal("missing cross-source anchors")
			}
		}
	}
	if !found {
		t.Fatal("Mr. split or fake paragraph retained")
	}
	original, err := st.PageWindow(ctx, public.EditionID, "", 0, 30)
	if err != nil || !strings.HasSuffix(original[0].Content, "Mr.") {
		t.Fatal("source text changed", err)
	}
	paid, _, err := st.CardWindow(ctx, licensed.EditionID, user.ID, 0, 0, 12)
	if err != nil {
		t.Fatal(err)
	}
	router := New(&config.Config{}, st).Routes()
	call := func(id, body, cookie string, status int) {
		t.Helper()
		req := httptest.NewRequest("POST", "/v1/cards/"+id+"/reports", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: cookie})
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("report %s: got %d want %d: %s", id, rec.Code, status, rec.Body.String())
		}
	}
	payload := `{"category":"break","note":"Mr. has strange spacing."}`
	call(cards[2].CardID, payload, "", 401)
	call(cards[2].CardID, payload, pending, 401)
	call("bad", payload, token, 400)
	call(cards[2].CardID, `{"category":"invalid"}`, token, 400)
	call(cards[2].CardID, `{"category":"text","note":"`+strings.Repeat("x", 1001)+`"}`, token, 400)
	call("00000000-0000-0000-0000-000000000000", payload, token, 404)
	call(paid[2].CardID, payload, token, 403)
	call(cards[2].CardID, payload, token, 201)
	call(cards[2].CardID, payload, token, 201)
	var reports int
	pool.QueryRow(ctx, `SELECT count(*) FROM card_reports WHERE user_id=$1`, user.ID).Scan(&reports)
	if reports != 1 {
		t.Fatal("duplicate report stored", reports)
	}
	for _, c := range cards[3:22] {
		call(c.CardID, payload, token, 201)
	}
	call(cards[22].CardID, payload, token, 429)
	call(cards[2].CardID, payload, token, 201) // retry remains idempotent at cap
}
