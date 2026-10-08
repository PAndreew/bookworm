package store

import (
	"context"
	"strings"
	"time"
)

// LibraryBook adds only the requesting reader's bookmarks to public metadata.
type LibraryBook struct {
	Book
	LastReadAt    *time.Time `json:"last_read_at"`
	LastPageIdx   *int       `json:"last_page_idx"`
	LastCardIdx   *int       `json:"last_card_idx"`
	Latest        bool       `json:"is_latest"`
	RatingAverage float64    `json:"rating_average"`
	RatingCount   int        `json:"rating_count"`
}

func (s *Store) ListLibraryBooks(ctx context.Context, viewer, license, search, genre, sort string, mine bool, limit, offset int) ([]LibraryBook, error) {
	const q = `
 SELECT b.id,b.slug,b.title,b.author,b.description,b.cover_url,b.language,b.license,
 e.id,e.page_count,e.word_count,b.genres,e.reader_card_count,
 rp.last_read_at,rp.last_page_idx,rp.last_card_idx,
 COALESCE(rp.last_read_at >= now()-interval '1 month' AND rp.edition_id=(
 SELECT p.edition_id FROM reading_progress p JOIN book_editions pe ON pe.id=p.edition_id AND pe.state='published'
 WHERE p.user_id=NULLIF($6,'')::uuid ORDER BY p.last_read_at DESC,p.edition_id LIMIT 1),false),
 COALESCE(reviews.average,0),COALESCE(reviews.total,0)
 FROM books b JOIN book_editions e ON e.book_id=b.id AND e.state='published'
 LEFT JOIN reading_progress rp ON rp.edition_id=e.id AND rp.user_id=NULLIF($6,'')::uuid
 LEFT JOIN LATERAL (SELECT avg(stars)::float8 AS average,count(*)::integer AS total FROM book_ratings WHERE book_id=b.id) reviews ON true
 WHERE ($1='' OR b.license=$1::license_kind)
 AND ($2='' OR lower(b.title || ' ' || b.author) LIKE $2 ESCAPE '\')
 AND ($3='' OR b.genres @> ARRAY[$3]::text[])
 AND (NOT $8 OR rp.user_id IS NOT NULL)
 ORDER BY
 CASE WHEN $7='recent' AND ($8 OR rp.last_read_at >= now()-interval '1 month') THEN rp.last_read_at END DESC NULLS LAST,
 CASE WHEN $7='rating' THEN reviews.average END DESC NULLS LAST,
 CASE WHEN $7 IN ('rating','reviews') THEN reviews.total END DESC NULLS LAST,
 CASE WHEN $7='shortest' THEN e.page_count END ASC,
 CASE WHEN $7='longest' THEN e.page_count END DESC,
 b.title,b.id
 LIMIT $4 OFFSET $5`
	pattern := ""
	if search != "" {
		pattern = "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(strings.ToLower(search)) + "%"
	}
	rows, err := s.pool.Query(ctx, q, license, pattern, genre, limit, offset, viewer, sort, mine)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	books := []LibraryBook{}
	for rows.Next() {
		var b LibraryBook
		if err := rows.Scan(&b.ID, &b.Slug, &b.Title, &b.Author, &b.Description, &b.CoverURL, &b.Language, &b.License, &b.EditionID, &b.PageCount, &b.WordCount, &b.Genres, &b.CardCount, &b.LastReadAt, &b.LastPageIdx, &b.LastCardIdx, &b.Latest, &b.RatingAverage, &b.RatingCount); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}
