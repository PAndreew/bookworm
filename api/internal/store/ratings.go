package store

import "context"

// Ratings are personal: never return another reader's ratings.
func (s *Store) Ratings(ctx context.Context, userID string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT b.slug, r.stars FROM book_ratings r
 JOIN books b ON b.id = r.book_id WHERE r.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ratings := map[string]int{}
	for rows.Next() {
		var slug string
		var stars int
		if err := rows.Scan(&slug, &stars); err != nil {
			return nil, err
		}
		ratings[slug] = stars
	}
	return ratings, rows.Err()
}

func (s *Store) RateBook(ctx context.Context, userID, bookID string, stars int) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO book_ratings (user_id, book_id, stars)
 VALUES ($1, $2, $3) ON CONFLICT (user_id, book_id) DO UPDATE
 SET stars = EXCLUDED.stars, updated_at = now()`, userID, bookID, stars)
	return err
}
