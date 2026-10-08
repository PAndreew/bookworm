package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

var ErrOfferFull = errors.New("store: founding offer full")

type OfferLease struct {
	Slot    int
	ID      string
	Expires time.Time
	URL     string
}
type Subscription struct {
	Customer          string    `json:"-"`
	ID                string    `json:"-"`
	Status            string    `json:"status"`
	PeriodEnd         time.Time `json:"period_end"`
	CancelAtPeriodEnd bool      `json:"cancel_at_period_end"`
}

func (s *Store) Subscription(ctx context.Context, user string) (*Subscription, error) {
	var sub Subscription
	err := s.pool.QueryRow(ctx, `SELECT customer_id,subscription_id,status,period_end,cancel_at_period_end FROM reader_subscriptions WHERE user_id=$1`, user).Scan(&sub.Customer, &sub.ID, &sub.Status, &sub.PeriodEnd, &sub.CancelAtPeriodEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &sub, err
}
func (s *Store) ReconcilePaidTier(ctx context.Context, user string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET tier='free' WHERE id=$1 AND tier<>'free' AND EXISTS(SELECT 1 FROM reader_subscriptions WHERE user_id=$1 AND (status<>'active' OR period_end<=now()))`, user)
	return tag.RowsAffected() > 0, err
}
func (s *Store) DropComplimentaryTier(ctx context.Context, user string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET tier='free' WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM reader_subscriptions WHERE user_id=$1)`, user)
	return err
}
func (s *Store) OfferRemaining(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM founding_slots WHERE NOT paid AND expires_at<=now()`).Scan(&n)
	return n, err
}
func (s *Store) ReserveOffer(ctx context.Context, user string) (OfferLease, error) {
	var lease OfferLease
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return lease, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(821,50)`); err != nil {
		return lease, err
	}
	err = tx.QueryRow(ctx, `SELECT slot,lease_id,expires_at,COALESCE(checkout_url,'') FROM founding_slots WHERE user_id=$1`, user).Scan(&lease.Slot, &lease.ID, &lease.Expires, &lease.URL)
	if err == nil && lease.Expires.After(time.Now()) && lease.URL != "" {
		return lease, tx.Commit(ctx)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return lease, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT slot FROM founding_slots WHERE NOT paid AND expires_at<=now() ORDER BY slot LIMIT 1`).Scan(&lease.Slot)
		if errors.Is(err, pgx.ErrNoRows) {
			return lease, ErrOfferFull
		}
		if err != nil {
			return lease, err
		}
	}
	// Keep a lease's expiry stable across retries: Stripe idempotency parameters must match.
	err = tx.QueryRow(ctx, `UPDATE founding_slots SET user_id=$1,lease_id=CASE WHEN user_id=$1 AND expires_at>now() THEN lease_id ELSE gen_random_uuid() END,expires_at=CASE WHEN user_id=$1 AND expires_at>now() THEN expires_at ELSE now()+interval '35 minutes' END,checkout_id=NULL,checkout_url=NULL WHERE slot=$2 RETURNING lease_id,expires_at`, user, lease.Slot).Scan(&lease.ID, &lease.Expires)
	if err != nil {
		return lease, err
	}
	lease.URL = ""
	return lease, tx.Commit(ctx)
}
func (s *Store) SaveCheckout(ctx context.Context, user, lease, id, url string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE founding_slots SET checkout_id=$3,checkout_url=$4 WHERE user_id=$1 AND lease_id=$2`, user, lease, id, url)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrOfferFull
	}
	return err
}

// Webhooks read the latest subscription from Stripe, not stale event snapshots.
// Event receipt, founding seat, subscription and tier changes commit together.
func (s *Store) ApplySubscription(ctx context.Context, event, user, lease string, sub Subscription) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(821,50)`); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO reader_stripe_events(id) VALUES($1) ON CONFLICT DO NOTHING`, event)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL)`, user).Scan(&exists); err != nil || !exists {
		return errors.New("billing: subscription identity unavailable")
	}
	var oldID string
	err = tx.QueryRow(ctx, `SELECT subscription_id FROM reader_subscriptions WHERE user_id=$1`, user).Scan(&oldID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if oldID != "" && oldID != sub.ID {
		var active bool
		if err = tx.QueryRow(ctx, `SELECT status='active' AND period_end>now() FROM reader_subscriptions WHERE user_id=$1`, user).Scan(&active); err != nil {
			return err
		}
		if active {
			return errors.New("billing: duplicate active subscription")
		}
		// Ignore inactive events from a replaced subscription.
		if sub.Status != "active" {
			return tx.Commit(ctx)
		}
	}
	if sub.Status == "active" && sub.PeriodEnd.After(time.Now()) {
		// Stripe's coupon max_redemptions is the authoritative fifty-purchase cap.
		// A delayed paid webhook may reclaim an unpaid lease; never a paid seat.
		tag, err = tx.Exec(ctx, `UPDATE founding_slots SET paid=true,expires_at=now(),checkout_id=NULL,checkout_url=NULL WHERE user_id=$1`, user)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			tag, err = tx.Exec(ctx, `UPDATE founding_slots SET user_id=$1,lease_id=$2,paid=true,expires_at=now(),checkout_id=NULL,checkout_url=NULL WHERE slot=(SELECT slot FROM founding_slots WHERE NOT paid ORDER BY slot LIMIT 1)`, user, lease)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrOfferFull
			}
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO reader_subscriptions(user_id,customer_id,subscription_id,status,period_end,cancel_at_period_end) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(user_id) DO UPDATE SET customer_id=excluded.customer_id,subscription_id=excluded.subscription_id,status=excluded.status,period_end=excluded.period_end,cancel_at_period_end=excluded.cancel_at_period_end,updated_at=now()`, user, sub.Customer, sub.ID, sub.Status, sub.PeriodEnd, sub.CancelAtPeriodEnd)
	if err != nil {
		return err
	}
	tier := TierFree
	if sub.Status == "active" && sub.PeriodEnd.After(time.Now()) {
		tier = TierPlus
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET tier=$2 WHERE id=$1`, user, tier); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) AssistantUsage(ctx context.Context, user string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT requests FROM assistant_monthly_usage WHERE user_id=$1 AND month=date_trunc('month',now() AT TIME ZONE 'UTC')::date),0)`, user).Scan(&n)
	return n, err
}
