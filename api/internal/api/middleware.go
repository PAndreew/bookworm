package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/bookworm/api/internal/auth"
	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
)

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxSession
)

// requireAuth rejects anonymous requests and, critically, rejects sessions that
// have passed the password but not yet the second factor. Without that check,
// 2FA would be decorative: the cookie issued before the TOTP step would already
// grant full access.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, user, err := s.resolveSession(r)
		if err != nil || user == nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Please sign in.")
			return
		}
		if sess.MFAPending {
			httpx.Error(w, http.StatusUnauthorized, "mfa_required",
				"Enter your authentication code to continue.")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxSession, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// optionalAuth attaches the user when a valid session exists but never rejects.
// Used by public endpoints that personalise their output.
func (s *Server) optionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, user, err := s.resolveSession(r)
		if err == nil && user != nil && !sess.MFAPending {
			ctx := context.WithValue(r.Context(), ctxUser, user)
			ctx = context.WithValue(ctx, ctxSession, sess)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// requirePendingSession allows exactly one thing: completing the second factor.
func (s *Server) requirePendingSession(r *http.Request) (*store.Session, *store.User, error) {
	sess, user, err := s.resolveSession(r)
	if err != nil || user == nil {
		return nil, nil, errors.New("no session")
	}
	if !sess.MFAPending {
		return nil, nil, errors.New("session is not awaiting a second factor")
	}
	return sess, user, nil
}

func (s *Server) resolveSession(r *http.Request) (*store.Session, *store.User, error) {
	c, err := r.Cookie(auth.SessionCookieName)
	if err != nil || c.Value == "" {
		return nil, nil, errors.New("no cookie")
	}

	sess, err := s.store.SessionByToken(r.Context(), auth.HashSessionToken(c.Value))
	if err != nil {
		return nil, nil, err
	}
	user, err := s.store.UserByID(r.Context(), sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	return sess, user, nil
}

// currentUser returns the authenticated user, or nil on a public endpoint.
func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(ctxUser).(*store.User)
	return u
}

func currentSession(r *http.Request) *store.Session {
	s, _ := r.Context().Value(ctxSession).(*store.Session)
	return s
}

// viewerID returns the signed-in user's id, or "" when anonymous.
func viewerID(r *http.Request) string {
	if u := currentUser(r); u != nil {
		return u.ID
	}
	return ""
}
