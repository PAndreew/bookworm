package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bookworm/api/internal/db"
)

func TestReadingActivityFirstReadsOnly(t *testing.T) {
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
	st := New(pool)
	suffix := fmt.Sprint(time.Now().UnixNano())
	u, err := st.CreateUser(ctx, "activity"+suffix+"@example.com", "act"+suffix, "Activity", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "Pacific/Kiritimati")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
	var book, edition, page string
	if err = pool.QueryRow(ctx, `INSERT INTO books(slug,title,author,license) VALUES($1,'Activity','Test','public_domain') RETURNING id`, "activity-"+suffix).Scan(&book); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM books WHERE id=$1`, book)
	if err = pool.QueryRow(ctx, `INSERT INTO book_editions(book_id,state,chunker_version,page_count,word_count) VALUES($1,'published',1,1,1) RETURNING id`, book).Scan(&edition); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO pages(edition_id,idx,content,char_count,content_sha256) VALUES($1,0,'Test',4,'\\x00') RETURNING id`, edition).Scan(&page); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC) // July 2 in the reader's timezone.
	today := LocalDay(u.Timezone, now)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	add := func(idx int, offsets []int, inherited bool, kind string) {
		t.Helper()
		var card string
		if err := pool.QueryRow(ctx, `INSERT INTO reader_cards(edition_id,idx,page_id,kind,start_off,end_off) VALUES($1,$2,$3,$4,0,1) RETURNING id`, edition, idx, page, kind).Scan(&card); err != nil {
			t.Fatal(err)
		}
		for _, offset := range offsets {
			exec(`INSERT INTO card_reads(user_id,card_id,local_day,metered,inherited) VALUES($1,$2,$3,false,$4)`, u.ID, card, today.AddDate(0, 0, -offset).Format("2006-01-02"), inherited)
		}
	}
	add(0, []int{30, 0}, false, "text") // old card reread today does not count.
	add(1, []int{29, 0}, false, "text") // boundary day counts once.
	add(2, []int{0}, false, "text")
	add(3, []int{0}, true, "text")   // inherited coverage does not count.
	add(4, []int{0}, false, "title") // non-text cards do not count.
	days, err := st.ReadingActivity(ctx, u.ID, u.Timezone, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 30 || days[0].Cards != 1 || days[29].Cards != 1 || days[29].Date != "2026-07-02" {
		t.Fatalf("unexpected activity: %+v", days)
	}
	total := 0
	for _, day := range days {
		total += day.Cards
	}
	if total != 2 {
		t.Fatalf("total=%d, want 2", total)
	}
}
