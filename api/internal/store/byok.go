package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const BYOKMonthlyLimit = 50

var ErrBYOKLimit = errors.New("store: monthly BYOK allowance reached")

type BYOKQuota struct {
	Used     int       `json:"used"`
	Limit    int       `json:"limit"`
	ResetsAt time.Time `json:"resets_at"`
}

// BYOKQuota holds metadata only, never credentials, text or provider responses.
func (s *Store) BYOKQuota(ctx context.Context, userID string) (BYOKQuota, error) {
	q := BYOKQuota{Limit: BYOKMonthlyLimit}
	err := s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT requests FROM byok_monthly_usage WHERE user_id=$1 AND month=date_trunc('month',now() AT TIME ZONE 'UTC')::date),0), (date_trunc('month',now() AT TIME ZONE 'UTC')+interval '1 month') AT TIME ZONE 'UTC'`, userID).Scan(&q.Used, &q.ResetsAt)
	return q, err
}

// Atomic across tabs/replicas. Reservations count even when a direct provider
// request fails or is cancelled; the server cannot observe that request.
func (s *Store) ReserveBYOKAttempt(ctx context.Context, userID string) (BYOKQuota, error) {
	q := BYOKQuota{Limit: BYOKMonthlyLimit}
	err := s.pool.QueryRow(ctx, `INSERT INTO byok_monthly_usage(user_id,month,requests) VALUES($1,date_trunc('month',now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(user_id,month) DO UPDATE SET requests=byok_monthly_usage.requests+1 WHERE byok_monthly_usage.requests<$2 RETURNING requests,(month+interval '1 month') AT TIME ZONE 'UTC'`, userID, BYOKMonthlyLimit).Scan(&q.Used, &q.ResetsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return q, ErrBYOKLimit
	}
	return q, err
}
