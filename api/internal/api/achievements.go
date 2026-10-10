package api

import (
	"github.com/bookworm/api/internal/httpx"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s *Server) handleAchievements(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	achievements, err := s.store.Achievements(r.Context(), currentUser(r).ID)
	if err != nil {
		httpx.InternalError(w, r, "achievements", err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"achievements": achievements})
}

func (s *Server) handleSharedAchievement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	handle := chi.URLParam(r, "handle")
	achievement, profile, err := s.store.SharedAchievement(r.Context(), handle, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "No such achievement.")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	httpx.JSON(w, 200, map[string]any{"achievement": achievement, "reader": profile})
}
