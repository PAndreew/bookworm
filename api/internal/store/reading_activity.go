package store

import (
	"context"
	"time"
)

type ReadingDay struct {
	Date  string `json:"date"`
	Cards int    `json:"cards"`
}

// Count each text card on its first-ever local read date, not on rereads.
// Inherited coverage participates in first-seen detection but never adds activity.
func (s *Store) ReadingActivity(ctx context.Context, userID, timezone string, now time.Time) ([]ReadingDay, error) {
	today := LocalDay(timezone, now)
	start := today.AddDate(0, 0, -29)
	rows, err := s.pool.Query(ctx, `WITH first_reads AS (
 SELECT r.card_id,min(r.local_day) AS day FROM card_reads r
 JOIN reader_cards c ON c.id=r.card_id AND c.kind='text'
 WHERE r.user_id=$1 GROUP BY r.card_id
 HAVING min(r.local_day)=min(r.local_day) FILTER (WHERE NOT r.inherited)
 ) SELECT day::text,count(*) FROM first_reads WHERE day BETWEEN $2::date AND $3::date GROUP BY day`, userID, start.Format("2006-01-02"), today.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var day string
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		counts[day] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	days := make([]ReadingDay, 0, 30)
	for i := 0; i < 30; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		days = append(days, ReadingDay{Date: date, Cards: counts[date]})
	}
	return days, nil
}
