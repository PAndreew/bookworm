package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

// QueueAuthEmail serializes per-account requests and enforces a persistent cooldown.
func (s *Store) QueueAuthEmail(ctx context.Context, userID, purpose string, hash []byte, ttl time.Duration, recipient, subject, body string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	if err = tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&id); err != nil {
		return err
	}
	var recent bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM email_tokens WHERE user_id=$1 AND purpose=$2 AND expires_at > now() + ($3 * interval '1 second') - interval '1 minute')`, userID, purpose, ttl.Seconds()).Scan(&recent)
	if err != nil {
		return err
	}
	if recent {
		return nil
	}
	if _, err = tx.Exec(ctx, `DELETE FROM email_tokens WHERE user_id=$1 AND purpose=$2`, userID, purpose); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO email_tokens VALUES ($1,$2,$3,$4)`, hash, userID, purpose, time.Now().Add(ttl)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO email_outbox(recipient,subject,body,expires_at) VALUES ($1,$2,$3,$4)`, recipient, subject, body, time.Now().Add(ttl)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ConsumeEmailToken(ctx context.Context, hash []byte, purpose, passwordHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Lock the user first, matching issuance lock order and serializing concurrent redemptions.
	var id string
	err = tx.QueryRow(ctx, `SELECT u.id FROM users u JOIN email_tokens t ON t.user_id=u.id WHERE t.token_hash=$1 AND t.purpose=$2 AND t.expires_at>now() AND u.deleted_at IS NULL FOR UPDATE OF u`, hash, purpose).Scan(&id)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM email_tokens WHERE token_hash=$1 AND purpose=$2 AND expires_at>now()`, hash, purpose)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if purpose == "verify" {
		_, err = tx.Exec(ctx, `UPDATE users SET email_verified=true WHERE id=$1`, id)
	} else {
		_, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, id, passwordHash)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, id)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO email_outbox(recipient,subject,body) SELECT email,'Potpuri password changed','Your password was changed. If this was not you, contact support immediately.' FROM users WHERE id=$1`, id)
		}
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM email_tokens WHERE user_id=$1 AND purpose=$2`, id, purpose); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) QueueSignupNotice(ctx context.Context, recipient, handle string, created time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO email_outbox(recipient,subject,body) VALUES ($1,'New Potpuri registration',$2)`, recipient, "Handle: "+handle+"\nRegistered: "+created.UTC().Format(time.RFC3339))
	return err
}
