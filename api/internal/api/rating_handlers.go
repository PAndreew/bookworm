package api

import (
	"errors"
	"net/http"

	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleRatings(w http.ResponseWriter, r *http.Request) {
	ratings, err := s.store.Ratings(r.Context(), currentUser(r).ID)
	if err != nil {
		httpx.InternalError(w, r, "handleRatings", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ratings": ratings})
}

func (s *Server) handleRateBook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Stars int `json:"stars"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil || req.Stars < 1 || req.Stars > 5 {
		httpx.Error(w, http.StatusBadRequest, "bad_rating", "Choose 1 to 5 stars.")
		return
	}
	book, err := s.store.BookBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such book.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleRateBook", err)
		return
	}
	if err := s.store.RateBook(r.Context(), currentUser(r).ID, book.ID, req.Stars); err != nil {
		httpx.InternalError(w, r, "handleRateBook", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"stars": req.Stars})
}
