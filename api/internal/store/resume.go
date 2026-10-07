package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

// LastRead returns the authenticated reader's latest saved position, including
// private books. Never use this query for another user's public profile.
type ReadingPosition struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	PageIdx int    `json:"page_idx"`
}

func (s *Store) LastRead(ctx context.Context, userID string) (*ReadingPosition, error) {
	var p ReadingPosition
	err := s.pool.QueryRow(ctx, `SELECT b.slug, b.title, rp.last_page_idx
 FROM reading_progress rp
 JOIN book_editions e ON e.id = rp.edition_id
 JOIN books b ON b.id = e.book_id
 WHERE rp.user_id = $1 AND e.state = 'published'
 ORDER BY rp.last_read_at DESC, rp.edition_id LIMIT 1`, userID).Scan(&p.Slug, &p.Title, &p.PageIdx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
