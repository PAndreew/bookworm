// Package store is the data access layer. Handlers call these methods; they never
// write SQL themselves.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("store: not found")
	ErrConflict  = errors.New("store: conflict")
	ErrForbidden = errors.New("store: forbidden")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the underlying pool for the few places that need a transaction.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// ------------------------------------------------------------------- models

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Handle        string    `json:"handle"`
	DisplayName   string    `json:"display_name"`
	Birthdate     time.Time `json:"-"`
	Timezone      string    `json:"timezone"`
	Role          string    `json:"role"`
	Tier          Tier      `json:"tier"`
	Visibility    string    `json:"visibility"`
	EmailVerified bool      `json:"email_verified"`
	TOTPEnabled   bool      `json:"totp_enabled"`
	CreatedAt     time.Time `json:"created_at"`
	passwordHash  string
	totpSecret    []byte
	TOTPEnabledAt *time.Time `json:"-"`
}

func (u *User) PasswordHash() string { return u.passwordHash }
func (u *User) TOTPSecret() []byte   { return u.totpSecret }

type Book struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	CoverURL    *string  `json:"cover_url"`
	Language    string   `json:"language"`
	License     string   `json:"license"`
	EditionID   string   `json:"edition_id"`
	PageCount   int      `json:"page_count"`
	CardCount   int      `json:"card_count"`
	WordCount   int      `json:"word_count"`
	Genres      []string `json:"genres"`
}

type Page struct {
	ID           string  `json:"id"`
	Idx          int     `json:"idx"`
	ChapterTitle *string `json:"chapter_title"`
	Content      string  `json:"content"`
	CharCount    int     `json:"char_count"`
	LikeCount    int     `json:"like_count"`
	LikedByMe    bool    `json:"liked_by_me"`
}

// --------------------------------------------------------------------- users

func (s *Store) CreateUser(ctx context.Context, email, handle, displayName, passwordHash string, birthdate time.Time, tz string) (*User, error) {
	const q = `
		INSERT INTO users (email, handle, display_name, password_hash, birthdate, timezone)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, handle, display_name, birthdate, timezone, role, tier,
		          visibility, email_verified, created_at`

	var u User
	err := s.pool.QueryRow(ctx, q, email, handle, displayName, passwordHash, birthdate, tz).
		Scan(&u.ID, &u.Email, &u.Handle, &u.DisplayName, &u.Birthdate, &u.Timezone,
			&u.Role, &u.Tier, &u.Visibility, &u.EmailVerified, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}

	// Every user gets a streak row up front, so later updates are a plain UPDATE.
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO streaks (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, u.ID); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
		SELECT id, email, handle, display_name, password_hash, birthdate, timezone,
		       role, tier, visibility, email_verified, totp_secret, totp_enabled_at, created_at
		FROM users WHERE email = $1 AND deleted_at IS NULL`
	return s.scanUser(s.pool.QueryRow(ctx, q, email))
}

func (s *Store) UserByID(ctx context.Context, id string) (*User, error) {
	const q = `
		SELECT id, email, handle, display_name, password_hash, birthdate, timezone,
		       role, tier, visibility, email_verified, totp_secret, totp_enabled_at, created_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`
	return s.scanUser(s.pool.QueryRow(ctx, q, id))
}

func (s *Store) UserByHandle(ctx context.Context, handle string) (*User, error) {
	const q = `
		SELECT id, email, handle, display_name, password_hash, birthdate, timezone,
		       role, tier, visibility, email_verified, totp_secret, totp_enabled_at, created_at
		FROM users WHERE handle = $1 AND deleted_at IS NULL`
	return s.scanUser(s.pool.QueryRow(ctx, q, handle))
}

func (s *Store) scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Handle, &u.DisplayName, &u.passwordHash,
		&u.Birthdate, &u.Timezone, &u.Role, &u.Tier, &u.Visibility,
		&u.EmailVerified, &u.totpSecret, &u.TOTPEnabledAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.TOTPEnabled = u.TOTPEnabledAt != nil
	return &u, nil
}

func (s *Store) UpdateProfile(ctx context.Context, userID, displayName, visibility, tz string) error {
	const q = `
		UPDATE users SET display_name = $2, visibility = $3, timezone = $4
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, userID, displayName, visibility, tz)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetTOTPSecret(ctx context.Context, userID string, secret []byte) error {
	// Staged, not enabled: totp_enabled_at stays NULL until a code is confirmed.
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET totp_secret = $2, totp_enabled_at = NULL WHERE id = $1`,
		userID, secret)
	return err
}

func (s *Store) EnableTOTP(ctx context.Context, userID string, codeHashes [][]byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE users SET totp_enabled_at = now() WHERE id = $1 AND totp_secret IS NOT NULL`,
		userID); err != nil {
		return err
	}
	// Re-enrolling invalidates the previous recovery codes.
	if _, err := tx.Exec(ctx,
		`DELETE FROM totp_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, h := range codeHashes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO totp_recovery_codes (user_id, code_hash) VALUES ($1, $2)`,
			userID, h); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) DisableTOTP(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE users SET totp_secret = NULL, totp_enabled_at = NULL WHERE id = $1`,
		userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM totp_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConsumeRecoveryCode marks a code used and reports whether it was valid. Codes are
// single use: the UPDATE only matches rows that have not been used.
func (s *Store) ConsumeRecoveryCode(ctx context.Context, userID string, codeHash []byte) (bool, error) {
	const q = `
		UPDATE totp_recovery_codes SET used_at = now()
		WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, userID, codeHash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ------------------------------------------------------------------ sessions

// CreateSession stores a session. An empty ip is written as NULL: the audit column
// is a nicety and must never be the reason a sign-in fails.
func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash []byte, mfaPending bool, ua, ip string, expires time.Time) error {
	var ipArg *string
	if ip != "" {
		ipArg = &ip
	}

	const q = `
		INSERT INTO sessions (user_id, token_hash, mfa_pending, user_agent, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5::inet, $6)`
	_, err := s.pool.Exec(ctx, q, userID, tokenHash, mfaPending, ua, ipArg, expires)
	return err
}

type Session struct {
	ID         string
	UserID     string
	MFAPending bool
}

func (s *Store) SessionByToken(ctx context.Context, tokenHash []byte) (*Session, error) {
	const q = `
		SELECT id, user_id, mfa_pending FROM sessions
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`

	var sess Session
	err := s.pool.QueryRow(ctx, q, tokenHash).Scan(&sess.ID, &sess.UserID, &sess.MFAPending)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// PromoteSession clears mfa_pending once the second factor is verified.
func (s *Store) PromoteSession(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET mfa_pending = false WHERE id = $1`, sessionID)
	return err
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) RevokeAllSessions(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID)
	return err
}

func isUniqueViolation(err error) bool {
	// 23505 = unique_violation
	return err != nil && (contains(err.Error(), "23505") || contains(err.Error(), "duplicate key"))
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
