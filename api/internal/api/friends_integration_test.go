package api

import (
	"context"
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

func TestFriendReadingRanksRespectCreditsDatesAndPrivacy(t *testing.T) {
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
	users := []*store.User{}
	for i := 0; i < 4; i++ {
		h := fmt.Sprintf("fr%d%s", i, suffix)
		u, e := st.CreateUser(ctx, h+"@example.com", h, "Friend", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "Pacific/Kiritimati")
		if e != nil {
			t.Fatal(e)
		}
		users = append(users, u)
		defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
		if i > 0 {
			if e = st.RequestFriend(ctx, users[0].ID, u.ID); e != nil {
				t.Fatal(e)
			}
			if e = st.AcceptFriend(ctx, u.ID, users[0].ID); e != nil {
				t.Fatal(e)
			}
		}
	}
	editions := []string{}
	pages := []string{}
	cards := []string{}
	for i := 0; i < 2; i++ {
		slug := fmt.Sprintf("friend-%s-%d", suffix, i)
		_, e := ingest.New(pool).Publish(ctx, ingest.Book{Slug: slug, Title: slug, Author: "Test", License: "public_domain", Genres: []string{"fiction"}, Pages: chunk.Split(strings.Repeat("A real sentence for the friends test. ", 40))})
		if e != nil {
			t.Fatal(e)
		}
		b, e := st.BookBySlug(ctx, slug)
		if e != nil {
			t.Fatal(e)
		}
		defer pool.Exec(ctx, `DELETE FROM books WHERE id=$1`, b.ID)
		if e = st.EnsureReaderCards(ctx, b.EditionID); e != nil {
			t.Fatal(e)
		}
		var p, c string
		if e = pool.QueryRow(ctx, `SELECT id FROM pages WHERE edition_id=$1 ORDER BY idx LIMIT 1`, b.EditionID).Scan(&p); e != nil {
			t.Fatal(e)
		}
		if e = pool.QueryRow(ctx, `SELECT id FROM reader_cards WHERE edition_id=$1 AND kind='text' ORDER BY idx LIMIT 1`, b.EditionID).Scan(&c); e != nil {
			t.Fatal(e)
		}
		editions = append(editions, b.EditionID)
		pages = append(pages, p)
		cards = append(cards, c)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	for _, p := range pages {
		exec(`INSERT INTO page_reads(user_id,page_id,local_day,metered) VALUES($1,$2,(now() AT TIME ZONE 'Pacific/Kiritimati')::date,false)`, users[0].ID, p)
	}
	for _, offset := range []int{0, 29, 30} {
		exec(`INSERT INTO card_reads(user_id,card_id,local_day,metered) VALUES($1,$2,(now() AT TIME ZONE 'Pacific/Kiritimati')::date-$3::int,false)`, users[1].ID, cards[0], offset)
	}
	exec(`INSERT INTO card_reads(user_id,card_id,local_day,metered,inherited) VALUES($1,$2,(now() AT TIME ZONE 'Pacific/Kiritimati')::date,false,true)`, users[1].ID, cards[1])
	// Navigation is not reading, and viewer-private books can still be compared.
	exec(`INSERT INTO reading_progress(user_id,edition_id,is_private) VALUES($1,$2,false),($3,$2,true)`, users[1].ID, editions[1], users[0].ID)
	for _, p := range pages {
		exec(`INSERT INTO page_reads(user_id,page_id,local_day,metered) VALUES($1,$2,(now() AT TIME ZONE 'Pacific/Kiritimati')::date,false)`, users[2].ID, p)
	}
	exec(`INSERT INTO page_reads(user_id,page_id,local_day,metered) VALUES($1,$2,(now() AT TIME ZONE 'Pacific/Kiritimati')::date,false)`, users[3].ID, pages[1])
	exec(`INSERT INTO reading_progress(user_id,edition_id,is_private) VALUES($1,$2,true)`, users[3].ID, editions[1])
	exec(`UPDATE streaks SET current_days=7,longest_days=7,last_day=(now() AT TIME ZONE 'Pacific/Kiritimati')::date-3 WHERE user_id=$1`, users[1].ID)
	exec(`UPDATE streaks SET current_days=2,longest_days=2,last_day=(now() AT TIME ZONE 'Pacific/Kiritimati')::date WHERE user_id=$1`, users[2].ID)
	check := func() {
		t.Helper()
		f, e := st.ListFriends(ctx, users[0].ID)
		if e != nil {
			t.Fatal(e)
		}
		if len(f) != 3 || f[0].ID != users[2].ID || f[0].CommonBooks != 2 || f[0].Pages30Days != 2 || f[0].StreakDays != 2 || f[1].CommonBooks != 1 || f[1].Pages30Days != 2 || f[1].StreakDays != 0 || f[2].CommonBooks != 0 || f[2].Pages30Days != 0 {
			t.Fatalf("incorrect ranks: %+v", f)
		}
	}
	check()
	exec(`UPDATE reading_progress SET is_private=false WHERE user_id=$1`, users[3].ID)
	exec(`UPDATE users SET visibility='private' WHERE id=$1`, users[3].ID)
	check()
	profile, e := st.ProfileFor(ctx, users[0].ID, users[2].Handle)
	if e != nil || profile.ID != users[2].ID {
		t.Fatalf("profile identity: %+v %v", profile, e)
	}
}
