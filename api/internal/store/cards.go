package store

import (
	"context"
	"errors"
	"time"

	"github.com/bookworm/api/internal/chunk"
	"github.com/jackc/pgx/v5"
)

type ReaderCard struct {
	Page
	CardID    string `json:"card_id"`
	Kind      string `json:"kind"`
	SourceIdx int    `json:"source_idx"`
	StartOff  int    `json:"start_off"`
}

// EnsureReaderCards builds once per edition, without modifying any source page.
// Advisory locking makes simultaneous requests/refinements safe. Version-2 layouts
// stay stable; older layouts refine with IDs, historical coverage and bookmarks preserved.
func (s *Store) EnsureReaderCards(ctx context.Context, editionID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, editionID); err != nil {
		return err
	}
	var count, version int
	var title, author string
	if err = tx.QueryRow(ctx, `SELECT e.reader_card_count,b.title,b.author,e.reader_layout_version FROM book_editions e JOIN books b ON b.id=e.book_id WHERE e.id=$1`, editionID).Scan(&count, &title, &author, &version); err != nil {
		return err
	}
	if count > 0 {
		if version < 2 {
			if err := refineReaderCards(ctx, tx, editionID); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}
	rows, err := tx.Query(ctx, `SELECT id,content,COALESCE(chapter_title,'') FROM pages WHERE edition_id=$1 ORDER BY idx`, editionID)
	if err != nil {
		return err
	}
	type source struct{ id, content, chapter string }
	sources := []source{}
	for rows.Next() {
		var p source
		if err = rows.Scan(&p.id, &p.content, &p.chapter); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	batch := &pgx.Batch{}
	add := func(pageID *string, kind, heading string, start, end int) {
		batch.Queue(`INSERT INTO reader_cards(edition_id,idx,page_id,kind,title,start_off,end_off) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7)`, editionID, count, pageID, kind, heading, start, end)
		count++
	}
	add(nil, "title", title+"\n"+author, 0, 0)
	chapter := ""
	for _, p := range sources {
		if p.chapter != "" && p.chapter != chapter {
			add(&p.id, "chapter", p.chapter, 0, 0)
			chapter = p.chapter
		}
		for _, span := range chunk.Cards(p.content) {
			add(&p.id, "text", "", span.Start, span.End)
		}
	}
	results := tx.SendBatch(ctx, batch)
	if err = results.Close(); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE book_editions SET reader_card_count=$2,reader_layout_version=2 WHERE id=$1`, editionID, count); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CardWindow(ctx context.Context, editionID, viewerID string, from, sourceFrom, limit int) ([]ReaderCard, int, error) {
	if err := s.EnsureReaderCards(ctx, editionID); err != nil {
		return nil, 0, err
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT reader_card_count FROM book_editions WHERE id=$1`, editionID).Scan(&count); err != nil {
		return nil, 0, err
	}
	if sourceFrom > 0 {
		if err := s.pool.QueryRow(ctx, `SELECT COALESCE(min(c.idx),$3) FROM reader_cards c JOIN pages p ON p.id=c.page_id WHERE c.edition_id=$1 AND p.idx >= $2`, editionID, sourceFrom, count).Scan(&from); err != nil {
			return nil, 0, err
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id,COALESCE(p.id::text,''),c.idx,c.kind,COALESCE(p.idx,0),c.start_off,
 CASE WHEN c.kind='text' THEN substring(p.content FROM c.start_off+1 FOR c.end_off-c.start_off) ELSE c.title END,
 (SELECT count(*) FROM page_likes l WHERE l.page_id=p.id),
 EXISTS(SELECT 1 FROM page_likes l WHERE l.page_id=p.id AND l.user_id=NULLIF($2,'')::uuid)
 FROM reader_cards c LEFT JOIN pages p ON p.id=c.page_id WHERE c.edition_id=$1 AND c.idx >= $3 ORDER BY c.idx LIMIT $4`, editionID, viewerID, from, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	cards := []ReaderCard{}
	for rows.Next() {
		var c ReaderCard
		if err := rows.Scan(&c.CardID, &c.ID, &c.Idx, &c.Kind, &c.SourceIdx, &c.StartOff, &c.Content, &c.LikeCount, &c.LikedByMe); err != nil {
			return nil, 0, err
		}
		c.CharCount = len([]rune(c.Content))
		cards = append(cards, c)
	}
	return cards, count, rows.Err()
}

func (t Tier) LicensedCardLimit() int {
	if t == TierPro {
		return 1000
	}
	return t.DailyLicensedPageLimit()
}
func (t Tier) QuotaPeriod(tz string, now time.Time) (time.Time, time.Time) {
	day := LocalDay(tz, now)
	if t == TierPro {
		first := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
		return first, first.AddDate(0, 1, 0)
	}
	return day, day.AddDate(0, 0, 1)
}
func (s *Store) LicensedUsage(ctx context.Context, userID string, tier Tier, tz string, now time.Time) (int, error) {
	first, next := tier.QuotaPeriod(tz, now)
	var used int
	err := s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM card_reads WHERE user_id=$1 AND metered AND local_day >= $2 AND local_day < $3) + (SELECT count(*) FROM page_reads WHERE user_id=$1 AND metered AND local_day >= $2 AND local_day < $3)`, userID, first.Format("2006-01-02"), next.Format("2006-01-02")).Scan(&used)
	return used, err
}

// RecordCard atomically checks quota, records only text cards, and advances both
// original-page and exact-card bookmarks. Lock the user to serialize competing reads.
func (s *Store) RecordCard(ctx context.Context, userID, cardID string, now time.Time) (MeterDecision, time.Time, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MeterDecision{}, time.Time{}, err
	}
	defer tx.Rollback(ctx)
	// Lock the edition before the user: refinements can safely copy historical
	// coverage without deadlocking on a reader waiting for the layout lock.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended(c.edition_id::text,0)) FROM reader_cards c WHERE c.id=$1`, cardID); err != nil {
		return MeterDecision{}, time.Time{}, err
	}
	var tier Tier
	var tz string
	if err = tx.QueryRow(ctx, `SELECT tier,timezone FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&tier, &tz); err != nil {
		return MeterDecision{}, time.Time{}, err
	}
	day := LocalDay(tz, now)
	var kind, edition, license string
	var pageID *string
	var idx, sourceIdx int
	err = tx.QueryRow(ctx, `SELECT c.kind,c.edition_id,c.page_id,c.idx,COALESCE(p.idx,0),b.license FROM reader_cards c LEFT JOIN pages p ON p.id=c.page_id JOIN book_editions e ON e.id=c.edition_id AND e.state='published' JOIN books b ON b.id=e.book_id WHERE c.id=$1`, cardID).Scan(&kind, &edition, &pageID, &idx, &sourceIdx, &license)
	if errors.Is(err, pgx.ErrNoRows) {
		return MeterDecision{}, day, ErrNotFound
	}
	if err != nil {
		return MeterDecision{}, day, err
	}
	if license != "public_domain" && tier == TierFree {
		return MeterDecision{Reason: "upgrade_required"}, day, nil
	}
	if kind != "text" {
		return MeterDecision{Allowed: true, Remaining: -1}, day, nil
	}
	first, next := tier.QuotaPeriod(tz, now)
	var already bool
	var used int
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM card_reads WHERE user_id=$1 AND card_id=$2 AND local_day >= $3 AND local_day < $4), (SELECT count(*) FROM card_reads WHERE user_id=$1 AND metered AND local_day >= $3 AND local_day < $4)+(SELECT count(*) FROM page_reads WHERE user_id=$1 AND metered AND local_day >= $3 AND local_day < $4)`, userID, cardID, first.Format("2006-01-02"), next.Format("2006-01-02")).Scan(&already, &used); err != nil {
		return MeterDecision{}, day, err
	}
	d := MeterDecision{Allowed: true, Remaining: -1}
	if license != "public_domain" {
		limit := tier.LicensedCardLimit()
		d.Remaining = max(limit-used, 0)
		if !already && used >= limit {
			reason := "daily_limit_reached"
			if tier == TierPro {
				reason = "monthly_limit_reached"
			}
			return MeterDecision{Reason: reason}, day, nil
		}
		d.Metered = !already
		if d.Metered {
			d.Remaining--
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO card_reads(user_id,card_id,local_day,metered) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,card_id,local_day) DO UPDATE SET inherited=false`, userID, cardID, day.Format("2006-01-02"), d.Metered); err != nil {
		return MeterDecision{}, day, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO reading_progress(user_id,edition_id,last_page_idx,furthest_idx,last_read_at,last_card_idx) VALUES($1,$2,$3,$3,now(),$4) ON CONFLICT(user_id,edition_id) DO UPDATE SET last_page_idx=$3,furthest_idx=GREATEST(reading_progress.furthest_idx,$3),last_read_at=now(),last_card_idx=$4`, userID, edition, sourceIdx, idx); err != nil {
		return MeterDecision{}, day, err
	}
	return d, day, tx.Commit(ctx)
}
