package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) BookPosition(ctx context.Context, userID, editionID string) (*ReadingPosition, error) {
	var p ReadingPosition
	err := s.pool.QueryRow(ctx, `SELECT b.slug,b.title,rp.last_page_idx,rp.last_card_idx FROM reading_progress rp JOIN book_editions e ON e.id=rp.edition_id JOIN books b ON b.id=e.book_id WHERE rp.user_id=$1 AND rp.edition_id=$2 AND e.state='published'`, userID, editionID).Scan(&p.Slug, &p.Title, &p.PageIdx, &p.CardIdx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

// SaveBookPosition records navigation, not reading credit. Opening a heading or
// moving quickly never consumes allowance or earns streak credit.
func (s *Store) SaveBookPosition(ctx context.Context, userID, editionID, cardID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended($1,0))`, editionID); err != nil {
		return err
	}
	var idx, sourceIdx int
	err = tx.QueryRow(ctx, `SELECT c.idx,COALESCE(p.idx,0) FROM reader_cards c LEFT JOIN pages p ON p.id=c.page_id WHERE c.id=$1 AND c.edition_id=$2 AND NOT c.archived`, cardID, editionID).Scan(&idx, &sourceIdx)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO reading_progress(user_id,edition_id,last_page_idx,furthest_idx,last_read_at,last_card_idx) VALUES($1,$2,$3,$3,now(),$4) ON CONFLICT(user_id,edition_id) DO UPDATE SET last_page_idx=$3,furthest_idx=GREATEST(reading_progress.furthest_idx,$3),last_read_at=now(),last_card_idx=$4`, userID, editionID, sourceIdx, idx)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
