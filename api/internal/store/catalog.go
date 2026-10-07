package store

import (
	"context"
	"errors"
	"time"

	"strings"

	"github.com/jackc/pgx/v5"
)

// ListBooks returns published books, optionally filtered by licence.
func (s *Store) ListBooks(ctx context.Context, license string, limit, offset int) ([]Book, error) {
	const q = `
		SELECT b.id, b.slug, b.title, b.author, b.description, b.cover_url,
		       b.language, b.license, e.id, e.page_count, e.word_count, b.genres
		FROM books b
		JOIN book_editions e ON e.book_id = b.id AND e.state = 'published'
		WHERE ($1 = '' OR b.license = $1::license_kind)
		ORDER BY b.title, b.id
		LIMIT $2 OFFSET $3`

	rows, err := s.pool.Query(ctx, q, license, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	books := []Book{}
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.Slug, &b.Title, &b.Author, &b.Description,
			&b.CoverURL, &b.Language, &b.License, &b.EditionID, &b.PageCount,
			&b.WordCount, &b.Genres); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

func (s *Store) BookBySlug(ctx context.Context, slug string) (*Book, error) {
	const q = `
		SELECT b.id, b.slug, b.title, b.author, b.description, b.cover_url,
		       b.language, b.license, e.id, e.page_count, e.word_count, b.genres
		FROM books b
		JOIN book_editions e ON e.book_id = b.id AND e.state = 'published'
		WHERE b.slug = $1`

	var b Book
	err := s.pool.QueryRow(ctx, q, slug).Scan(&b.ID, &b.Slug, &b.Title, &b.Author,
		&b.Description, &b.CoverURL, &b.Language, &b.License, &b.EditionID,
		&b.PageCount, &b.WordCount, &b.Genres)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// PageWindow fetches a slice of pages with like counts, which is what the feed
// scrolls through. The window keeps the payload small on mobile data.
func (s *Store) PageWindow(ctx context.Context, editionID, viewerID string, fromIdx, limit int) ([]Page, error) {
	const q = `
		SELECT p.id, p.idx, p.chapter_title, p.content, p.char_count,
		       (SELECT count(*) FROM page_likes l WHERE l.page_id = p.id),
		       EXISTS (SELECT 1 FROM page_likes l
		               WHERE l.page_id = p.id AND l.user_id = NULLIF($2,'')::uuid)
		FROM pages p
		WHERE p.edition_id = $1 AND p.idx >= $3
		ORDER BY p.idx
		LIMIT $4`

	rows, err := s.pool.Query(ctx, q, editionID, viewerID, fromIdx, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pages := []Page{}
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.ID, &p.Idx, &p.ChapterTitle, &p.Content,
			&p.CharCount, &p.LikeCount, &p.LikedByMe); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// PageMeta is what the meter needs to know about a page before serving it.
type PageMeta struct {
	PageID    string
	EditionID string
	Idx       int
	License   string
}

func (s *Store) PageMeta(ctx context.Context, pageID string) (*PageMeta, error) {
	const q = `
		SELECT p.id, p.edition_id, p.idx, b.license
		FROM pages p
		JOIN book_editions e ON e.id = p.edition_id
		JOIN books b ON b.id = e.book_id
		WHERE p.id = $1 AND e.state = 'published'`

	var m PageMeta
	err := s.pool.QueryRow(ctx, q, pageID).Scan(&m.PageID, &m.EditionID, &m.Idx, &m.License)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ---------------------------------------------------------- reading & metering

// MeteredPagesToday counts licensed pages the user has consumed on a local day.
func (s *Store) MeteredPagesToday(ctx context.Context, userID string, localDay time.Time) (int, error) {
	const q = `SELECT count(*) FROM page_reads
	           WHERE user_id = $1 AND local_day = $2 AND metered`
	var n int
	err := s.pool.QueryRow(ctx, q, userID, localDay).Scan(&n)
	return n, err
}

func (s *Store) PagesReadToday(ctx context.Context, userID string, localDay time.Time) (int, error) {
	const q = `SELECT count(*) FROM page_reads WHERE user_id = $1 AND local_day = $2`
	var n int
	err := s.pool.QueryRow(ctx, q, userID, localDay).Scan(&n)
	return n, err
}

func (s *Store) HasReadToday(ctx context.Context, userID, pageID string, localDay time.Time) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM page_reads
	           WHERE user_id = $1 AND page_id = $2 AND local_day = $3)`
	var ok bool
	err := s.pool.QueryRow(ctx, q, userID, pageID, localDay).Scan(&ok)
	return ok, err
}

// RecordRead writes the read and advances progress in one transaction, so the meter
// and the bookmark can never disagree.
func (s *Store) RecordRead(ctx context.Context, userID, pageID, editionID string, idx int, localDay time.Time, metered bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// ON CONFLICT DO NOTHING: re-reading the same page on the same day is free and
	// must not double-count against the quota.
	if _, err := tx.Exec(ctx, `
		INSERT INTO page_reads (user_id, page_id, local_day, metered)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, page_id, local_day) DO NOTHING`,
		userID, pageID, localDay, metered); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO reading_progress (user_id, edition_id, last_page_idx, furthest_idx, last_read_at)
		VALUES ($1, $2, $3, $3, now())
		ON CONFLICT (user_id, edition_id) DO UPDATE
		SET last_page_idx = EXCLUDED.last_page_idx,
		    furthest_idx  = GREATEST(reading_progress.furthest_idx, EXCLUDED.furthest_idx),
		    last_read_at  = now()`,
		userID, editionID, idx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// --------------------------------------------------------------------- streaks

func (s *Store) GetStreak(ctx context.Context, userID string) (Streak, error) {
	const q = `SELECT current_days, longest_days, last_day, freezes_left
	           FROM streaks WHERE user_id = $1`

	var st Streak
	var lastDay *time.Time
	err := s.pool.QueryRow(ctx, q, userID).
		Scan(&st.CurrentDays, &st.LongestDays, &lastDay, &st.FreezesLeft)
	if errors.Is(err, pgx.ErrNoRows) {
		return Streak{}, nil // a user with no row simply has no streak yet
	}
	if err != nil {
		return Streak{}, err
	}
	if lastDay != nil {
		st.LastDay = *lastDay
	}
	return st, nil
}

func (s *Store) SaveStreak(ctx context.Context, userID string, st Streak) error {
	const q = `
		INSERT INTO streaks (user_id, current_days, longest_days, last_day, freezes_left)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET current_days = EXCLUDED.current_days,
		    longest_days = EXCLUDED.longest_days,
		    last_day     = EXCLUDED.last_day,
		    freezes_left = EXCLUDED.freezes_left`

	var lastDay *time.Time
	if !st.LastDay.IsZero() {
		lastDay = &st.LastDay
	}
	_, err := s.pool.Exec(ctx, q, userID, st.CurrentDays, st.LongestDays, lastDay, st.FreezesLeft)
	return err
}

// ---------------------------------------------------------- likes & highlights

func (s *Store) LikePage(ctx context.Context, userID, pageID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO page_likes (user_id, page_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, userID, pageID)
	return err
}

func (s *Store) UnlikePage(ctx context.Context, userID, pageID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM page_likes WHERE user_id = $1 AND page_id = $2`, userID, pageID)
	return err
}

type Highlight struct {
	ID        string    `json:"id"`
	PageID    string    `json:"page_id"`
	StartOff  int       `json:"start_off"`
	EndOff    int       `json:"end_off"`
	Quote     string    `json:"quote"`
	ShareSlug *string   `json:"share_slug"`
	CreatedAt time.Time `json:"created_at"`
	BookTitle string    `json:"book_title,omitempty"`
	Author    string    `json:"author,omitempty"`
}

// CreateHighlight stores a highlight. An empty shareSlug is written as NULL, not
// as the empty string: share_slug carries a UNIQUE index, and two unshared
// highlights would otherwise collide on ”. In Postgres, NULLs do not conflict.
func (s *Store) CreateHighlight(ctx context.Context, userID, pageID string, start, end int, quote, shareSlug string) (*Highlight, error) {
	var slug *string
	if shareSlug != "" {
		slug = &shareSlug
	}

	const q = `
		INSERT INTO highlights (user_id, page_id, start_off, end_off, quote, share_slug)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, page_id, start_off, end_off, quote, share_slug, created_at`

	var h Highlight
	err := s.pool.QueryRow(ctx, q, userID, pageID, start, end, quote, slug).
		Scan(&h.ID, &h.PageID, &h.StartOff, &h.EndOff, &h.Quote, &h.ShareSlug, &h.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *Store) DeleteHighlight(ctx context.Context, userID, id string) error {
	// Scoped by user_id: you cannot delete someone else's highlight by guessing an id.
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM highlights WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListHighlights(ctx context.Context, userID string, limit, offset int) ([]Highlight, error) {
	const q = `
		SELECT h.id, h.page_id, h.start_off, h.end_off, h.quote, h.share_slug,
		       h.created_at, b.title, b.author
		FROM highlights h
		JOIN pages p ON p.id = h.page_id
		JOIN book_editions e ON e.id = p.edition_id
		JOIN books b ON b.id = e.book_id
		WHERE h.user_id = $1
		ORDER BY h.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Highlight{}
	for rows.Next() {
		var h Highlight
		if err := rows.Scan(&h.ID, &h.PageID, &h.StartOff, &h.EndOff, &h.Quote,
			&h.ShareSlug, &h.CreatedAt, &h.BookTitle, &h.Author); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// HighlightBySlug powers the public quote page that shared links open.
func (s *Store) HighlightBySlug(ctx context.Context, slug string) (*Highlight, error) {
	const q = `
		SELECT h.id, h.page_id, h.start_off, h.end_off, h.quote, h.share_slug,
		       h.created_at, b.title, b.author
		FROM highlights h
		JOIN pages p ON p.id = h.page_id
		JOIN book_editions e ON e.id = p.edition_id AND e.state = 'published'
		JOIN books b ON b.id = e.book_id
		WHERE h.share_slug = $1`

	var h Highlight
	err := s.pool.QueryRow(ctx, q, slug).Scan(&h.ID, &h.PageID, &h.StartOff, &h.EndOff,
		&h.Quote, &h.ShareSlug, &h.CreatedAt, &h.BookTitle, &h.Author)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// QuoteFromPage re-slices a quote from the stored page text using rune offsets.
//
// The client sends offsets, never the quote itself. If we stored client-supplied
// text, anyone could POST a highlight attributing invented words to a real book
// and then share that as a quote card.
func (s *Store) QuoteFromPage(ctx context.Context, pageID string, start, end int) (string, error) {
	var content string
	err := s.pool.QueryRow(ctx,
		`SELECT p.content FROM pages p
		 JOIN book_editions e ON e.id = p.edition_id AND e.state = 'published'
		 WHERE p.id = $1`, pageID).Scan(&content)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	runes := []rune(content)
	if start < 0 || end > len(runes) || start >= end {
		return "", ErrForbidden
	}
	return strings.TrimSpace(string(runes[start:end])), nil
}

// FileTakedown records an infringement notice. The book slug is optional: a notice
// that names no book is still worth keeping rather than rejecting.
func (s *Store) FileTakedown(ctx context.Context, email, bookSlug, claim string) error {
	var editionID *string
	if bookSlug != "" {
		var id string
		err := s.pool.QueryRow(ctx, `
			SELECT e.id FROM book_editions e
			JOIN books b ON b.id = e.book_id
			WHERE b.slug = $1 AND e.state = 'published'`, bookSlug).Scan(&id)
		if err == nil {
			editionID = &id
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO takedown_notices (edition_id, reporter_email, claim)
		VALUES ($1, $2, $3)`, editionID, email, claim)
	return err
}
