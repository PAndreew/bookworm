package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bookworm/api/internal/auth"
	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
)

// MinAge is the floor for an account. Set to 13 to stay clear of COPPA's
// under-13 parental-consent regime and GDPR Art. 8 in the member states that
// set their digital-consent age at 13. See docs/LEGAL.md.
const MinAge = 13

var (
	handleRe = regexp.MustCompile(`^[a-z0-9_]{3,24}$`)
	emailRe  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

type signupRequest struct {
	Email       string `json:"email"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Birthdate   string `json:"birthdate"` // YYYY-MM-DD
	Timezone    string `json:"timezone"`  // IANA name from the browser
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Handle = strings.TrimSpace(strings.ToLower(req.Handle))
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if !emailRe.MatchString(req.Email) {
		httpx.Error(w, http.StatusBadRequest, "invalid_email", "That email address does not look right.")
		return
	}
	if !handleRe.MatchString(req.Handle) {
		httpx.Error(w, http.StatusBadRequest, "invalid_handle",
			"Handles are 3-24 characters: lowercase letters, numbers and underscores.")
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Handle
	}

	birthdate, err := time.Parse("2006-01-02", req.Birthdate)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_birthdate", "Please give your date of birth.")
		return
	}
	if ageOn(birthdate, time.Now()) < MinAge {
		// Deliberately not "come back when you are older": that just teaches people
		// to retype a different year. We simply decline.
		httpx.Error(w, http.StatusForbidden, "too_young",
			"You need to be at least 13 to use Bookworm.")
		return
	}

	tz := req.Timezone
	if _, err := time.LoadLocation(tz); err != nil || tz == "" {
		tz = "UTC"
	}

	hash, err := auth.HashPassword(req.Password)
	if errors.Is(err, auth.ErrPasswordTooShort) {
		httpx.Error(w, http.StatusBadRequest, "weak_password",
			"Please use a password of at least 10 characters.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleSignup", err)
		return
	}

	user, err := s.store.CreateUser(r.Context(), req.Email, req.Handle, req.DisplayName, hash, birthdate, tz)
	if errors.Is(err, store.ErrConflict) {
		httpx.Error(w, http.StatusConflict, "already_exists",
			"That email or handle is already taken.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleSignup", err)
		return
	}

	if err := s.issueSession(w, r, user.ID, false); err != nil {
		httpx.InternalError(w, r, "handleSignup", err)
		return
	}
	if s.cfg.ResendKey != "" {
		if err := s.queueAuthEmail(r.Context(), user, "verify"); err != nil {
			slog.Error("signup verification queue failed")
		}
		if s.cfg.SignupNotify != "" {
			if err := s.store.QueueSignupNotice(r.Context(), s.cfg.SignupNotify, user.Handle, user.CreatedAt); err != nil {
				slog.Error("signup notification queue failed")
			}
		}
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"user": user})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}

	user, err := s.store.UserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		// Spend the time anyway. Returning immediately on an unknown email turns
		// login into an account-enumeration oracle via response timing.
		_, _ = auth.HashPassword("decoy password to equalise timing")
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials",
			"That email or password is not right.")
		return
	}

	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash())
	if err != nil || !ok {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials",
			"That email or password is not right.")
		return
	}

	// With 2FA on, the cookie we issue is pending: it unlocks nothing except the
	// verify endpoint until a valid code arrives.
	if user.TOTPEnabled {
		if err := s.issueSession(w, r, user.ID, true); err != nil {
			httpx.InternalError(w, r, "handleLogin", err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"mfa_required": true})
		return
	}

	if err := s.issueSession(w, r, user.ID, false); err != nil {
		httpx.InternalError(w, r, "handleLogin", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": user})
}

type verifyRequest struct {
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

func (s *Server) handle2FAVerify(w http.ResponseWriter, r *http.Request) {
	sess, user, err := s.requirePendingSession(r)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Please sign in again.")
		return
	}

	var req verifyRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}

	secret, err := s.decryptSecret(user.TOTPSecret())
	if err != nil {
		httpx.InternalError(w, r, "handle2FAVerify", err)
		return
	}

	verified := false
	switch {
	case req.Code != "":
		verified = auth.VerifyTOTP(secret, req.Code, time.Now())
	case req.RecoveryCode != "":
		verified, err = s.store.ConsumeRecoveryCode(r.Context(), user.ID,
			auth.HashRecoveryCode(req.RecoveryCode))
		if err != nil {
			httpx.InternalError(w, r, "handle2FAVerify", err)
			return
		}
	}

	if !verified {
		httpx.Error(w, http.StatusUnauthorized, "invalid_code", "That code is not right.")
		return
	}

	if err := s.store.PromoteSession(r.Context(), sess.ID); err != nil {
		httpx.InternalError(w, r, "handle2FAVerify", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookieName); err == nil {
		_ = s.store.RevokeSession(r.Context(), auth.HashSessionToken(c.Value))
	}
	auth.ClearSessionCookie(w, s.cfg.SecureCookies)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "signed_out"})
}

// ---------------------------------------------------------------- 2FA setup

func (s *Server) handle2FASetup(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	secret, err := auth.NewTOTPSecret()
	if err != nil {
		httpx.InternalError(w, r, "handle2FASetup", err)
		return
	}
	enc, err := s.encryptSecret(secret)
	if err != nil {
		httpx.InternalError(w, r, "handle2FASetup", err)
		return
	}
	// Staged only: 2FA is not active until handle2FAEnable confirms a real code.
	// Otherwise a failed enrolment would lock the user out of their own account.
	if err := s.store.SetTOTPSecret(r.Context(), user.ID, enc); err != nil {
		httpx.InternalError(w, r, "handle2FASetup", err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"provisioning_uri": auth.TOTPProvisioningURI(secret, user.Email),
	})
}

func (s *Server) handle2FAEnable(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req verifyRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}
	if user.TOTPSecret() == nil {
		httpx.Error(w, http.StatusBadRequest, "no_pending_setup", "Start two-factor setup first.")
		return
	}

	secret, err := s.decryptSecret(user.TOTPSecret())
	if err != nil {
		httpx.InternalError(w, r, "handle2FAEnable", err)
		return
	}
	if !auth.VerifyTOTP(secret, req.Code, time.Now()) {
		httpx.Error(w, http.StatusBadRequest, "invalid_code", "That code is not right.")
		return
	}

	plain, hashes, err := auth.NewRecoveryCodes(10)
	if err != nil {
		httpx.InternalError(w, r, "handle2FAEnable", err)
		return
	}
	if err := s.store.EnableTOTP(r.Context(), user.ID, hashes); err != nil {
		httpx.InternalError(w, r, "handle2FAEnable", err)
		return
	}

	// The only time these are ever visible. They are stored hashed.
	httpx.JSON(w, http.StatusOK, map[string]any{"recovery_codes": plain})
}

func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req struct {
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}

	// Turning off a security control requires re-proving who you are: otherwise a
	// borrowed unlocked phone is enough to strip the second factor.
	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash())
	if err != nil || !ok {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "That password is not right.")
		return
	}

	if err := s.store.DisableTOTP(r.Context(), user.ID); err != nil {
		httpx.InternalError(w, r, "handle2FADisable", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// ------------------------------------------------------------------ helpers

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, userID string, mfaPending bool) error {
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	err = s.store.CreateSession(r.Context(), userID, hash, mfaPending,
		r.UserAgent(), clientIP(r), time.Now().Add(auth.SessionTTL))
	if err != nil {
		return err
	}
	auth.SetSessionCookie(w, token, s.cfg.SecureCookies)
	return nil
}

// encryptSecret wraps the TOTP secret with AES-GCM before it touches the database,
// so a dumped users table does not hand over everyone's second factor.
func (s *Server) encryptSecret(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.cfg.TOTPEncryptKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func (s *Server) decryptSecret(enc []byte) ([]byte, error) {
	if len(enc) == 0 {
		return nil, errors.New("api: no totp secret")
	}
	block, err := aes.NewCipher(s.cfg.TOTPEncryptKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(enc) < gcm.NonceSize() {
		return nil, errors.New("api: totp secret is malformed")
	}
	nonce, body := enc[:gcm.NonceSize()], enc[gcm.NonceSize():]
	return gcm.Open(nil, nonce, body, nil)
}

// ageOn returns the age in whole years at a given instant.
func ageOn(birthdate, at time.Time) int {
	years := at.Year() - birthdate.Year()
	// Not yet had this year's birthday.
	if at.YearDay() < birthdate.YearDay() {
		years--
	}
	return years
}

// clientIP extracts the caller's address as a bare IP string.
//
// net.SplitHostPort rather than a split on the last colon: an IPv6 RemoteAddr looks
// like "[::1]:54321", and slicing at the colon leaves "[::1]", which Postgres rejects
// as an inet. That failure showed up as a 500 on signup.
//
// Returns "" when there is no usable address, which the store maps to NULL rather
// than letting a malformed value fail the insert.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := xff
		if i := strings.IndexByte(xff, ','); i >= 0 {
			first = xff[:i]
		}
		if ip := net.ParseIP(strings.TrimSpace(first)); ip != nil {
			return ip.String()
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}
