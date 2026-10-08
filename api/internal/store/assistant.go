package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrAssistantLimit = errors.New("store: monthly assistant allowance or request spacing reached")
var ErrAssistantCapacity = errors.New("store: daily assistant service capacity reached")
var ErrAssistantUnpaid = errors.New("store: paid assistant access required")

func (t Tier) AssistantLimit() int {
	if t == TierPro {
		return 1000
	}
	if t == TierPlus {
		return 100
	}
	return 0
}

// Count new provider attempts, not cached translations. UTC monthly quotas and
// five-second spacing are atomic across replicas; provider failures still count.
func (s *Store) ReserveAssistantAttempt(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var count int
	err = tx.QueryRow(ctx, `INSERT INTO assistant_global_usage(day) VALUES((now() AT TIME ZONE 'UTC')::date) ON CONFLICT(day) DO UPDATE SET requests=assistant_global_usage.requests+1 WHERE assistant_global_usage.requests<1000 RETURNING requests`).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAssistantCapacity
	}
	if err != nil {
		return err
	}
	var tier Tier
	err = tx.QueryRow(ctx, `SELECT CASE WHEN EXISTS(SELECT 1 FROM reader_subscriptions WHERE user_id=$1 AND (status<>'active' OR period_end<=now())) THEN 'free'::tier ELSE tier END FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&tier)
	if err != nil {
		return err
	}
	if tier.AssistantLimit() == 0 {
		return ErrAssistantUnpaid
	}
	err = tx.QueryRow(ctx, `INSERT INTO assistant_monthly_usage(user_id,month,requests,last_request_at) VALUES($1,date_trunc('month',now() AT TIME ZONE 'UTC')::date,1,now()) ON CONFLICT(user_id,month) DO UPDATE SET requests=assistant_monthly_usage.requests+1,last_request_at=now() WHERE assistant_monthly_usage.requests<$2 AND (assistant_monthly_usage.last_request_at IS NULL OR assistant_monthly_usage.last_request_at<=now()-interval '5 seconds') RETURNING requests`, userID, tier.AssistantLimit()).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAssistantLimit
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
