package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Local allowances govern the official UI, not access to public model weights.
func (t Tier) LocalAssistantLimit() int {
	switch t {
	case TierPro:
		return 1000
	case TierPlus:
		return 100
	default:
		return 10
	}
}

type LocalAssistantUsage struct {
	Used     int       `json:"used"`
	Limit    int       `json:"limit"`
	ResetsAt time.Time `json:"resets_at"`
}

func (s *Store) LocalAssistantAllowance(ctx context.Context, userID string, reserve bool) (LocalAssistantUsage, error) {
	var usage LocalAssistantUsage
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return usage, err
	}
	defer tx.Rollback(ctx)
	var tier Tier
	// Serialize reservations and respect subscription expiry even before its webhook.
	err = tx.QueryRow(ctx, `SELECT CASE WHEN EXISTS(SELECT 1 FROM reader_subscriptions WHERE user_id=$1 AND (status<>'active' OR period_end<=now())) THEN 'free'::tier ELSE tier END FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&tier)
	if err != nil {
		return usage, err
	}
	usage.Limit = tier.LocalAssistantLimit()
	if err = tx.QueryRow(ctx, `SELECT (date_trunc('month',now() AT TIME ZONE 'UTC') + interval '1 month') AT TIME ZONE 'UTC'`).Scan(&usage.ResetsAt); err != nil {
		return usage, err
	}
	if reserve {
		err = tx.QueryRow(ctx, `INSERT INTO assistant_local_usage(user_id,month,requests,last_request_at) VALUES($1,date_trunc('month',now() AT TIME ZONE 'UTC')::date,1,now()) ON CONFLICT(user_id,month) DO UPDATE SET requests=assistant_local_usage.requests+1,last_request_at=now() WHERE assistant_local_usage.requests<$2 AND assistant_local_usage.last_request_at<=now()-interval '5 seconds' RETURNING requests`, userID, usage.Limit).Scan(&usage.Used)
		if errors.Is(err, pgx.ErrNoRows) {
			return usage, ErrAssistantLimit
		}
	} else {
		err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT requests FROM assistant_local_usage WHERE user_id=$1 AND month=date_trunc('month',now() AT TIME ZONE 'UTC')::date),0)`, userID).Scan(&usage.Used)
	}
	if err != nil {
		return usage, err
	}
	return usage, tx.Commit(ctx)
}
