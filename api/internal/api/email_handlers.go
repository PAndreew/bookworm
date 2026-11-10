package api

import (
	"context"
	"github.com/bookworm/api/internal/auth"
	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func (s *Server) queueAuthEmail(ctx context.Context, u *store.User, purpose string) error {
	if s.cfg.ResendKey == "" {
		return nil
	}
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	ttl, subject := 24*time.Hour, "Confirm your Potpuri email"
	if purpose == "reset" {
		ttl, subject = 30*time.Minute, "Reset your Potpuri password"
	}
	link := strings.TrimRight(s.cfg.PublicURL, "/") + "/email-action#purpose=" + purpose + "&token=" + token
	return s.store.QueueAuthEmail(ctx, u.ID, purpose, hash, ttl, u.Email, subject, subject+"\n\n"+link+"\n\nIf you did not request this, ignore this email.")
}

func (s *Server) handleEmailRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, 400, "bad_request", "Invalid request.")
		return
	}
	if s.cfg.ResendKey == "" {
		httpx.Error(w, 503, "email_unavailable", "Email is temporarily unavailable.")
		return
	}
	purpose := "reset"
	if strings.HasSuffix(r.URL.Path, "resend-verification") {
		purpose = "verify"
	}
	u, err := s.store.UserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if err == nil && (purpose != "verify" || !u.EmailVerified) {
		if err = s.queueAuthEmail(r.Context(), u, purpose); err != nil {
			slog.Error("queue auth email failed")
		}
	} else if err != nil && err != store.ErrNotFound {
		slog.Error("email account lookup failed")
	}
	httpx.JSON(w, 200, map[string]string{"message": "If an eligible account exists, we have queued an email. Check your inbox."})
}

func (s *Server) handleEmailConsume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil || len(req.Token) != 43 {
		httpx.Error(w, 400, "invalid_token", "This link is invalid or expired.")
		return
	}
	purpose, hash := "verify", ""
	if strings.HasSuffix(r.URL.Path, "reset-password") {
		purpose = "reset"
		var err error
		hash, err = auth.HashPassword(req.Password)
		if err != nil {
			httpx.Error(w, 400, "weak_password", "Use a password of at least 10 characters.")
			return
		}
	}
	err := s.store.ConsumeEmailToken(r.Context(), auth.HashSessionToken(req.Token), purpose, hash)
	if err == store.ErrNotFound {
		httpx.Error(w, 400, "invalid_token", "This link is invalid or expired.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "consume email token", err)
		return
	}
	if purpose == "reset" {
		auth.ClearSessionCookie(w, s.cfg.SecureCookies)
	}
	httpx.JSON(w, 200, map[string]string{"message": "Success. You can now sign in."})
}
