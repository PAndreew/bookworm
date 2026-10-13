package store

import "context"

// BookCompletion uses the same active text-card coverage as the full-book
// achievement. Reads on any device or date, including inherited coverage, count.
type BookCompletion struct {
	TotalCards         int   `json:"total_cards"`
	ReadCards          int   `json:"read_cards"`
	MissingCardIndices []int `json:"missing_card_indices"`
}

func (s *Store) BookCompletion(ctx context.Context, userID, editionID string) (BookCompletion, error) {
	result := BookCompletion{MissingCardIndices: []int{}}
	rows, err := s.pool.Query(ctx, `SELECT c.idx, EXISTS(SELECT 1 FROM card_reads r WHERE r.card_id=c.id AND r.user_id=$1) FROM reader_cards c WHERE c.edition_id=$2 AND c.kind='text' AND NOT c.archived ORDER BY c.idx`, userID, editionID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var idx int
		var read bool
		if err = rows.Scan(&idx, &read); err != nil {
			return result, err
		}
		result.TotalCards++
		if read {
			result.ReadCards++
		} else {
			result.MissingCardIndices = append(result.MissingCardIndices, idx)
		}
	}
	return result, rows.Err()
}
