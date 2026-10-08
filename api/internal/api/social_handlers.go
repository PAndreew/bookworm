package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
)

func (s *Server) handleListFriends(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	friends, err := s.store.ListFriends(r.Context(), currentUser(r).ID)
	if err != nil {
		httpx.InternalError(w, r, "handleListFriends", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"friends": friends})
}

func (s *Server) handlePendingFriends(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	pending, err := s.store.ListPendingRequests(r.Context(), currentUser(r).ID)
	if err != nil {
		httpx.InternalError(w, r, "handlePendingFriends", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"pending": pending})
}

// resolveTarget turns a :handle route param into a user id.
func (s *Server) resolveTarget(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	handle := strings.ToLower(chi.URLParam(r, "handle"))

	target, err := s.store.UserByHandle(r.Context(), handle)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such user.")
		return nil, false
	}
	if err != nil {
		httpx.InternalError(w, r, "resolveTarget", err)
		return nil, false
	}
	return target, true
}

func (s *Server) handleRequestFriend(w http.ResponseWriter, r *http.Request) {
	target, ok := s.resolveTarget(w, r)
	if !ok {
		return
	}

	err := s.store.RequestFriend(r.Context(), currentUser(r).ID, target.ID)
	if errors.Is(err, store.ErrForbidden) {
		// Same response whether they blocked you or you asked yourself: never
		// confirm a block to the person who was blocked.
		httpx.Error(w, http.StatusForbidden, "forbidden", "You cannot add that person.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleRequestFriend", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "requested"})
}

func (s *Server) handleAcceptFriend(w http.ResponseWriter, r *http.Request) {
	target, ok := s.resolveTarget(w, r)
	if !ok {
		return
	}

	err := s.store.AcceptFriend(r.Context(), currentUser(r).ID, target.ID)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "no_request", "There is no request from that person.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleAcceptFriend", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (s *Server) handleRemoveFriend(w http.ResponseWriter, r *http.Request) {
	target, ok := s.resolveTarget(w, r)
	if !ok {
		return
	}
	if err := s.store.RemoveFriend(r.Context(), currentUser(r).ID, target.ID); err != nil {
		httpx.InternalError(w, r, "handleRemoveFriend", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBlockUser(w http.ResponseWriter, r *http.Request) {
	target, ok := s.resolveTarget(w, r)
	if !ok {
		return
	}

	err := s.store.BlockUser(r.Context(), currentUser(r).ID, target.ID)
	if errors.Is(err, store.ErrForbidden) {
		httpx.Error(w, http.StatusBadRequest, "forbidden", "You cannot block yourself.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleBlockUser", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

func (s *Server) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		httpx.JSON(w, http.StatusOK, map[string]any{"users": []store.Friend{}})
		return
	}

	users, err := s.store.SearchUsers(r.Context(), currentUser(r).ID, q,
		httpx.QueryInt(r, "limit", 20, 1, 50))
	if err != nil {
		httpx.InternalError(w, r, "handleSearchUsers", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"users": users})
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	handle := strings.ToLower(chi.URLParam(r, "handle"))

	profile, err := s.store.ProfileFor(r.Context(), currentUser(r).ID, handle)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such user.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleGetProfile", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"profile": profile})
}

func (s *Server) handleFriendFeed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	activity, err := s.store.FriendActivity(r.Context(), currentUser(r).ID,
		httpx.QueryInt(r, "limit", 30, 1, 100))
	if err != nil {
		httpx.InternalError(w, r, "handleFriendFeed", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"activity": activity})
}

// handleTakedown accepts an infringement notice. Public and unauthenticated on
// purpose: a rights holder must be able to file one without an account. See
// docs/LEGAL.md for why this exists in v1 rather than later.
func (s *Server) handleTakedown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		BookSlug string `json:"book_slug"`
		Claim    string `json:"claim"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}
	if !emailRe.MatchString(req.Email) || strings.TrimSpace(req.Claim) == "" {
		httpx.Error(w, http.StatusBadRequest, "incomplete",
			"Please give a contact email and describe the claim.")
		return
	}

	if err := s.store.FileTakedown(r.Context(), req.Email, req.BookSlug, req.Claim); err != nil {
		httpx.InternalError(w, r, "handleTakedown", err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{
		"status":  "received",
		"message": "Thank you. We will review this notice and respond to the address you gave.",
	})
}
