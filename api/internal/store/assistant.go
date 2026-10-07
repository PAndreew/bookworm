package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrAssistantLimit = errors.New("store: assistant limit reached")

// ReserveAssistantAttempt caps paid calls across all replicas and restarts.
// Always lock global then user; rollback the global reservation if user is capped.
func (s *Store) ReserveAssistantAttempt(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var count int
	err = tx.QueryRow(ctx, `INSERT INTO assistant_global_usage (day)
 VALUES ((now() AT TIME ZONE 'UTC')::date)
 ON CONFLICT (day) DO UPDATE SET requests=assistant_global_usage.requests+1
 WHERE assistant_global_usage.requests < 200 RETURNING requests`).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAssistantLimit
	}
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `INSERT INTO assistant_daily_usage (user_id, day)
 VALUES ($1, (now() AT TIME ZONE 'UTC')::date)
 ON CONFLICT (user_id,day) DO UPDATE SET requests=assistant_daily_usage.requests+1, last_request_at=now()
 WHERE assistant_daily_usage.requests < 30
 AND assistant_daily_usage.last_request_at <= now()-interval '5 seconds'
 RETURNING requests`, userID).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAssistantLimit
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
