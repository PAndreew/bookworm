package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bookworm/api/internal/chunk"
	"github.com/bookworm/api/internal/db"
	"github.com/bookworm/api/internal/ingest"
	"github.com/bookworm/api/internal/store"
)

func TestCardsAndMonthlyQuota(t *testing.T) {
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
	user, err := st.CreateUser(ctx, "cards"+suffix+"@example.com", "c"+suffix, "Cards", "test-hash", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "Europe/Bucharest")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
	// Existing case-insensitive uniqueness must reject variants too.
	if _, err := st.CreateUser(ctx, "duplicate"+suffix+"@example.com", strings.ToUpper(user.Handle), "Duplicate", "hash", user.Birthdate, "UTC"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("case-insensitive handle uniqueness: %v", err)
	}
	books := []*store.Book{}
	for _, license := range []string{"public_domain", "licensed"} {
		slug := "cards-" + license + "-" + suffix
		_, err := ingest.New(pool).Publish(ctx, ingest.Book{Slug: slug, Title: "Card Fixture", Author: "Tester", License: license, Pages: chunk.Split("CHAPTER 1.\n\n" + strings.Repeat("This is a sentence with some words. ", 60))})
		if err != nil {
			t.Fatal(err)
		}
		b, err := st.BookBySlug(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		books = append(books, b)
		defer pool.Exec(ctx, `DELETE FROM books WHERE id=$1`, b.ID)
	}
	now := time.Now()
	public := books[0]
	licensed := books[1]
	source, err := st.PageWindow(ctx, public.EditionID, "", 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	cards, count, err := st.CardWindow(ctx, public.EditionID, user.ID, 0, 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	if cards[0].Kind != "title" || cards[1].Kind != "chapter" || count <= public.PageCount {
		t.Fatal("missing title/chapter or balanced text cards")
	}
	for _, c := range cards {
		if c.Kind == "text" && (c.CharCount > 400 || c.CharCount < 1) {
			t.Fatal("invalid text size")
		}
	}
	for _, c := range cards[:2] {
		d, _, err := st.RecordCard(ctx, user.ID, c.CardID, now)
		if err != nil || !d.Allowed || d.Metered {
			t.Fatal("header was metered", err)
		}
	}
	total, err := st.PagesReadToday(ctx, user.ID, store.LocalDay(user.Timezone, now))
	if err != nil || total != 0 {
		t.Fatal("headers counted toward streak", err)
	}
	d, _, err := st.RecordCard(ctx, user.ID, cards[2].CardID, now)
	if err != nil || !d.Allowed || d.Metered {
		t.Fatal("free public reading failed", err)
	}
	position, err := st.LastRead(ctx, user.ID)
	if err != nil || position.CardIdx == nil || *position.CardIdx != cards[2].Idx || position.PageIdx != cards[2].SourceIdx {
		t.Fatal("exact card resume failed", err)
	}
	mapped, _, err := st.CardWindow(ctx, public.EditionID, "", 0, source[len(source)-1].Idx, 12)
	if err != nil || len(mapped) == 0 || mapped[0].SourceIdx < source[len(source)-1].Idx {
		t.Fatal("legacy resume mapping failed", err)
	}
	again, err := st.PageWindow(ctx, public.EditionID, "", 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	for i := range source {
		if source[i].ID != again[i].ID || source[i].Content != again[i].Content {
			t.Fatal("immutable page changed")
		}
	}
	paid, _, err := st.CardWindow(ctx, licensed.EditionID, user.ID, 0, 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	if d, _, err = st.RecordCard(ctx, user.ID, paid[2].CardID, now); err != nil || d.Allowed {
		t.Fatal("free licensed access allowed", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE users SET tier='pro' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	d, _, err = st.RecordCard(ctx, user.ID, paid[2].CardID, now)
	if err != nil || !d.Metered || d.Remaining != 999 {
		t.Fatal("Pro quota not 1000", d, err)
	}
	first, reset := store.TierPro.QuotaPeriod(user.Timezone, now)
	if first.Day() != 1 || reset.Day() != 1 || !reset.After(now) {
		t.Fatal("monthly reset wrong")
	}
	// Fill the remaining 999 slots without slow individual HTTP requests.
	if _, err = pool.Exec(ctx, `WITH extra AS (INSERT INTO reader_cards(edition_id,idx,page_id,kind,start_off,end_off) SELECT $1,10000+n,$2,'text',0,1 FROM generate_series(1,999) n RETURNING id) INSERT INTO card_reads(user_id,card_id,local_day,metered) SELECT $3,id,$4,true FROM extra`, licensed.EditionID, paid[2].ID, user.ID, store.LocalDay(user.Timezone, now).Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	used, err := st.LicensedUsage(ctx, user.ID, store.TierPro, user.Timezone, now)
	if err != nil || used != 1000 {
		t.Fatal("monthly usage wrong", used, err)
	}
	d, _, err = st.RecordCard(ctx, user.ID, paid[3].CardID, now)
	if err != nil || d.Allowed || d.Reason != "monthly_limit_reached" {
		t.Fatal("monthly cap not enforced", d, err)
	}
	d, _, err = st.RecordCard(ctx, user.ID, paid[2].CardID, now)
	if err != nil || !d.Allowed || d.Metered {
		t.Fatal("reread consumed allowance", err)
	}
	d, _, err = st.RecordCard(ctx, user.ID, paid[0].CardID, now)
	if err != nil || !d.Allowed || d.Metered {
		t.Fatal("title blocked at quota", err)
	}
	d, _, err = st.RecordCard(ctx, user.ID, cards[3].CardID, now)
	if err != nil || !d.Allowed || d.Metered {
		t.Fatal("public content blocked at quota", err)
	}
	// A new month frees the allowance without deleting any historical reading.
	d, _, err = st.RecordCard(ctx, user.ID, paid[3].CardID, reset.Add(time.Hour))
	if err != nil || !d.Allowed || !d.Metered || d.Remaining != 999 {
		t.Fatal("monthly rollover failed", d, err)
	}

	// Regression: character-only v1 cards overflow with many short paragraphs.
	// Refinement retains IDs/annotations, remaps later positions once, and keeps
	// historical streak/quota counts unchanged while granting inherited coverage.
	slug := "line-budget-" + suffix
	_, err = ingest.New(pool).Publish(ctx, ingest.Book{Slug: slug, Title: "Line Fixture", Author: "Tester", Language: "en", License: "licensed", Pages: chunk.Split("CHAPTER 1.\n\n" + strings.Repeat("Short heading.\n\n", 12) + "CHAPTER 2.\n\nContinuation: this ordinary readable text remains on one card.")})
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.BookBySlug(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM books WHERE id=$1`, b.ID)
	src, err := st.PageWindow(ctx, b.EditionID, "", 0, 10)
	if err != nil || len(src) != 2 {
		t.Fatal("line fixture pages", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO reader_cards(edition_id,idx,page_id,kind,title,start_off,end_off) VALUES
	($1,0,NULL,'title','Fixture',0,0),($1,1,$2,'chapter','CHAPTER 1',0,0),($1,2,$2,'text',NULL,0,$4),
	($1,3,$3,'chapter','CHAPTER 2',0,0),($1,4,$3,'text',NULL,0,$5)`, b.EditionID, src[0].ID, src[1].ID, len([]rune(src[0].Content)), len([]rune(src[1].Content)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE book_editions SET reader_card_count=5 WHERE id=$1`, b.EditionID); err != nil {
		t.Fatal(err)
	}
	var firstID, laterID string
	pool.QueryRow(ctx, `SELECT id FROM reader_cards WHERE edition_id=$1 AND idx=2`, b.EditionID).Scan(&firstID)
	pool.QueryRow(ctx, `SELECT id FROM reader_cards WHERE edition_id=$1 AND idx=4`, b.EditionID).Scan(&laterID)
	future := reset.Add(2 * time.Hour)
	for _, id := range []string{firstID, laterID} {
		if _, _, err = st.RecordCard(ctx, user.ID, id, future); err != nil {
			t.Fatal(err)
		}
	}
	day := store.LocalDay(user.Timezone, future)
	beforePages, err := st.PagesReadToday(ctx, user.ID, day)
	if err != nil {
		t.Fatal(err)
	}
	beforeUsed, err := st.LicensedUsage(ctx, user.ID, store.TierPro, user.Timezone, future)
	if err != nil {
		t.Fatal(err)
	}
	refined, newCount, err := st.CardWindow(ctx, b.EditionID, user.ID, 0, 0, 12)
	if err != nil || newCount != 6 {
		t.Fatal("line refinement", newCount, err)
	}
	if refined[2].CardID != firstID || refined[5].CardID != laterID {
		t.Fatal("existing card IDs lost")
	}
	pos, err := st.LastRead(ctx, user.ID)
	if err != nil || pos.CardIdx == nil || *pos.CardIdx != 5 || pos.PageIdx != 1 {
		t.Fatal("refined resume mapping", pos, err)
	}
	afterPages, _ := st.PagesReadToday(ctx, user.ID, day)
	afterUsed, _ := st.LicensedUsage(ctx, user.ID, store.TierPro, user.Timezone, future)
	if beforePages != afterPages || beforeUsed != afterUsed {
		t.Fatal("refinement changed historical counts")
	}
	d, _, err = st.RecordCard(ctx, user.ID, refined[3].CardID, future)
	if err != nil || !d.Allowed || d.Metered {
		t.Fatal("already-read coverage charged again", d, err)
	}
	afterUsed, _ = st.LicensedUsage(ctx, user.ID, store.TierPro, user.Timezone, future)
	if afterUsed != beforeUsed {
		t.Fatal("inherited coverage consumed quota")
	}
}
