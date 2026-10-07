package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

// SessionCookieName is the cookie holding the opaque session token.
const SessionCookieName = "bw_session"

// SessionTTL is how long a session lives. Deliberately long for a reading app --
// being logged out breaks a streak, which is the one thing users care about.
const SessionTTL = 90 * 24 * time.Hour

// NewSessionToken returns a 32-byte opaque token and its SHA-256 hash. The plaintext
// goes in the cookie; only the hash is stored, so a database leak does not hand over
// live sessions.
//
// These are opaque random tokens rather than JWTs on purpose: revocation must be
// immediate and server-side. A stolen JWT stays valid until it expires.
func NewSessionToken() (token string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("auth: generating session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

// HashSessionToken hashes a token from a request cookie for database lookup.
func HashSessionToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SetSessionCookie writes the session cookie.
//
// SameSite=Lax rather than Strict: Strict would drop the cookie when a user arrives
// from a shared quote link, logging them out at exactly the moment the growth loop
// is trying to convert them.
func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(SessionTTL),
		MaxAge:   int(SessionTTL.Seconds()),
	})
}

// ClearSessionCookie expires the cookie on logout.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
