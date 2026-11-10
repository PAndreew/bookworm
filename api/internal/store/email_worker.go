package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"net/http"
	"time"
)

// RunEmailWorker uses row locks for multiple replicas and Resend idempotency for crash retries.
// Outbox bodies contain bearer links: restrict database/backups and delete after delivery/expiry.
func (s *Store) RunEmailWorker(ctx context.Context, key, from string) {
	if key == "" {
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		for i := 0; i < 20; i++ {
			if err := s.sendNextEmail(ctx, client, key, from); err != nil {
				if err != pgx.ErrNoRows {
					slog.Error("email delivery deferred")
				}
				break
			}
		}
	}
}
func (s *Store) sendNextEmail(ctx context.Context, client *http.Client, key, from string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM email_outbox WHERE expires_at<now(); DELETE FROM email_tokens WHERE expires_at<now()`); err != nil {
		return err
	}
	var id, to, subject, body string
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id,recipient,subject,body,attempts FROM email_outbox WHERE next_attempt<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &to, &subject, &body, &attempts)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"from": from, "to": []string{to}, "subject": subject, "text": body})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", id)
	res, sendErr := client.Do(req)
	success := false
	if res != nil {
		success = res.StatusCode >= 200 && res.StatusCode < 300
		res.Body.Close()
	}
	if success {
		_, err = tx.Exec(ctx, `DELETE FROM email_outbox WHERE id=$1`, id)
	} else {
		delay := time.Minute * time.Duration(1<<min(attempts, 8))
		_, err = tx.Exec(ctx, `UPDATE email_outbox SET attempts=attempts+1,next_attempt=$2 WHERE id=$1`, id, time.Now().Add(delay))
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if !success || sendErr != nil {
		return fmt.Errorf("delivery deferred")
	}
	return nil
}
