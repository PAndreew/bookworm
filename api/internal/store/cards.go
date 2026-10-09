package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReaderCard struct {
	Page
	CardID       string     `json:"card_id"`
	Kind         string     `json:"kind"`
	SourceIdx    int        `json:"source_idx"`
	StartOff     int        `json:"start_off"`
	Parts        []CardPart `json:"parts"`
	SentenceEnds []int      `json:"sentence_ends"`
}

// EnsureReaderCards builds once per edition, without modifying any source page.
// Advisory locking makes replacement atomic with read-history/bookmark migration.
// Version-5 layouts prefer whole sentences across chapters; original pages remain immutable.
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
	if count > 0 && version >= 5 {
		return tx.Commit(ctx)
	}
	if err = replaceReaderCards(ctx, tx, editionID, title, author); err != nil {
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
		if err := s.pool.QueryRow(ctx, `SELECT COALESCE(min(c.idx),$3) FROM reader_cards c LEFT JOIN reader_card_parts part ON part.card_id=c.id JOIN pages p ON p.id=COALESCE(part.page_id,c.page_id) WHERE c.edition_id=$1 AND NOT c.archived AND p.idx >= $2`, editionID, sourceFrom, count).Scan(&from); err != nil {
			return nil, 0, err
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id,COALESCE(p.id::text,''),c.idx,c.kind,COALESCE(p.idx,0),c.start_off,
 COALESCE(c.content,CASE WHEN c.kind='text' THEN substring(p.content FROM c.start_off+1 FOR c.end_off-c.start_off) ELSE c.title END),
 (SELECT count(*) FROM page_likes l WHERE l.page_id=p.id),
 EXISTS(SELECT 1 FROM page_likes l WHERE l.page_id=p.id AND l.user_id=NULLIF($2,'')::uuid), c.sentence_ends
 FROM reader_cards c LEFT JOIN pages p ON p.id=c.page_id WHERE c.edition_id=$1 AND NOT c.archived AND c.idx >= $3 ORDER BY c.idx LIMIT $4`, editionID, viewerID, from, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	cards := []ReaderCard{}
	for rows.Next() {
		var c ReaderCard
		if err := rows.Scan(&c.CardID, &c.ID, &c.Idx, &c.Kind, &c.SourceIdx, &c.StartOff, &c.Content, &c.LikeCount, &c.LikedByMe, &c.SentenceEnds); err != nil {
			return nil, 0, err
		}
		c.CharCount = len([]rune(c.Content))
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()
	ids := make([]string, 0, len(cards))
	byID := map[string]*ReaderCard{}
	for i := range cards {
		ids = append(ids, cards[i].CardID)
		byID[cards[i].CardID] = &cards[i]
	}
	parts, err := s.pool.Query(ctx, `SELECT card_id,page_id,start_off,end_off,card_start FROM reader_card_parts WHERE card_id=ANY($1::uuid[]) ORDER BY card_id,ordinal`, ids)
	if err != nil {
		return nil, 0, err
	}
	defer parts.Close()
	for parts.Next() {
		var id string
		var p CardPart
		if err = parts.Scan(&id, &p.PageID, &p.StartOff, &p.EndOff, &p.CardStart); err != nil {
			return nil, 0, err
		}
		byID[id].Parts = append(byID[id].Parts, p)
	}
	return cards, count, parts.Err()
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
	err = tx.QueryRow(ctx, `SELECT c.kind,c.edition_id,c.page_id,c.idx,COALESCE(p.idx,0),b.license FROM reader_cards c LEFT JOIN pages p ON p.id=c.page_id JOIN book_editions e ON e.id=c.edition_id AND e.state='published' JOIN books b ON b.id=e.book_id WHERE c.id=$1 AND NOT c.archived`, cardID).Scan(&kind, &edition, &pageID, &idx, &sourceIdx, &license)
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
