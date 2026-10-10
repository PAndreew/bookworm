package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type Achievement struct {
	ID         string     `json:"id"`
	UnlockedAt *time.Time `json:"unlocked_at"`
}

var achievementIDs = []string{"first-card", "hundred-cards", "full-book"}

// SyncAchievements is idempotent. Only non-inherited text reads count toward
// first/100; old archived reads are retained in the ledger across rechunking.
// Completion requires every active text card in one edition to have read coverage.
// On a first qualifying request, the database locks in the award; later layout
// changes cannot revoke it. Each award is a one-time snapshot, so inherited
// coverage added after an award never creates extra 100-card progress.
func (s *Store) SyncAchievements(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `WITH genuine AS (
 SELECT r.card_id,min(r.read_at) AS at FROM card_reads r
 JOIN reader_cards c ON c.id=r.card_id AND c.kind='text'
 WHERE r.user_id=$1 AND NOT r.inherited GROUP BY r.card_id
 ), hundred AS (
 SELECT at FROM (SELECT at,row_number() OVER (ORDER BY at,card_id) n FROM genuine) ranked WHERE n=100
 ), complete AS (
 SELECT max(first_read) AS at FROM (
  SELECT c.edition_id,c.id, min(r.read_at) AS first_read FROM reader_cards c
  LEFT JOIN card_reads r ON r.card_id=c.id AND r.user_id=$1
  WHERE c.kind='text' AND NOT c.archived
  GROUP BY c.edition_id,c.id
 ) covered GROUP BY edition_id
 HAVING count(*)>0 AND count(first_read)=count(*)
 ) INSERT INTO achievements(user_id,code,earned_at)
 SELECT $1,'first-card',min(at) FROM genuine HAVING count(*)>0
 UNION ALL SELECT $1,'hundred-cards',at FROM hundred
 UNION ALL SELECT $1,'full-book',min(at) FROM complete HAVING count(*)>0
 ON CONFLICT(user_id,code) DO NOTHING`, userID)
	return err
}

type AchievementReader struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
}

// Only the milestone is published. A share URL never reveals which book was
// read, and cannot be used to browse private reading history.
func (s *Store) SharedAchievement(ctx context.Context, handle, id string) (Achievement, AchievementReader, error) {
	var a Achievement
	var reader AchievementReader
	if id != "first-card" && id != "hundred-cards" && id != "full-book" {
		return a, reader, ErrNotFound
	}
	var userID string
	err := s.pool.QueryRow(ctx, `SELECT id,handle,display_name FROM users WHERE handle=$1`, handle).Scan(&userID, &reader.Handle, &reader.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, reader, ErrNotFound
	}
	if err != nil {
		return a, reader, err
	}
	if err = s.SyncAchievements(ctx, userID); err != nil {
		return a, reader, err
	}
	a.ID = id
	err = s.pool.QueryRow(ctx, `SELECT earned_at FROM achievements WHERE user_id=$1 AND code=$2`, userID, id).Scan(&a.UnlockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Achievement{}, AchievementReader{}, ErrNotFound
	}
	return a, reader, err
}

func (s *Store) Achievements(ctx context.Context, userID string) ([]Achievement, error) {
	if err := s.SyncAchievements(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT code,earned_at FROM achievements WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	earned := map[string]time.Time{}
	for rows.Next() {
		var id string
		var at time.Time
		if err = rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		earned[id] = at
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result := make([]Achievement, 0, len(achievementIDs))
	for _, id := range achievementIDs {
		var date *time.Time
		if at, ok := earned[id]; ok {
			date = &at
		}
		result = append(result, Achievement{ID: id, UnlockedAt: date})
	}
	return result, nil
}
