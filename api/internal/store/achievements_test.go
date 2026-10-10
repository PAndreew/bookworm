package store

import (
	"context"
	"fmt"
	"github.com/bookworm/api/internal/db"
	"os"
	"testing"
	"time"
)

func TestAchievementsBackfillAndShare(t *testing.T) {
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
	user, err := st.CreateUser(ctx, "awards"+suffix+"@example.com", "aw"+suffix[len(suffix)-12:], "Award reader", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
	var book, edition, page string
	if err = pool.QueryRow(ctx, `INSERT INTO books(slug,title,author,license) VALUES($1,'Achievement test','Test','public_domain') RETURNING id`, "awards-"+suffix).Scan(&book); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM books WHERE id=$1`, book)
	if err = pool.QueryRow(ctx, `INSERT INTO book_editions(book_id,state,chunker_version,page_count,word_count,reader_card_count,reader_layout_version) VALUES($1,'published',1,1,1,102,4) RETURNING id`, book).Scan(&edition); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO pages(edition_id,idx,content,char_count,content_sha256) VALUES($1,0,'Test',4,'\x00') RETURNING id`, edition).Scan(&page); err != nil {
		t.Fatal(err)
	}
	get := func() map[string]bool {
		t.Helper()
		awards, e := st.Achievements(ctx, user.ID)
		if e != nil {
			t.Fatal(e)
		}
		result := map[string]bool{}
		for _, a := range awards {
			result[a.ID] = a.UnlockedAt != nil
		}
		return result
	}
	if get()["first-card"] {
		t.Fatal("new reader already earned achievement")
	}
	// A title and an inherited read do not award a milestone.
	_, err = pool.Exec(ctx, `WITH cards AS (INSERT INTO reader_cards(edition_id,idx,page_id,kind,start_off,end_off) VALUES($1,0,$2,'title',0,1),($1,1,$2,'text',0,1) RETURNING id,kind) INSERT INTO card_reads(user_id,card_id,local_day,metered,inherited) SELECT $3,id,current_date,false,kind='text' FROM cards`, edition, page, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a := get(); a["first-card"] || !a["full-book"] {
		t.Fatal("inherited coverage should complete book but not count as personal read", a)
	}
	_, err = pool.Exec(ctx, `WITH c AS (INSERT INTO reader_cards(edition_id,idx,page_id,kind,start_off,end_off) SELECT $1,n,$2,'text',0,1 FROM generate_series(2,100) n RETURNING id) INSERT INTO card_reads(user_id,card_id,local_day,metered) SELECT $3,id,current_date,false FROM c`, edition, page, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a := get(); !a["first-card"] || a["hundred-cards"] || !a["full-book"] {
		t.Fatal("99 reads", a)
	}
	_, err = pool.Exec(ctx, `UPDATE card_reads SET inherited=false WHERE user_id=$1 AND inherited`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a := get(); !a["hundred-cards"] || !a["full-book"] {
		t.Fatal("complete book", a)
	}
	if _, _, err = st.SharedAchievement(ctx, user.Handle, "hundred-cards"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = st.SharedAchievement(ctx, user.Handle, "full-book"); err != nil {
		t.Fatal("private-book milestone should be shareable without disclosing book", err)
	}
	if _, _, err = st.SharedAchievement(ctx, user.Handle, "not-real"); err != ErrNotFound {
		t.Fatal("invalid public award", err)
	}
}
