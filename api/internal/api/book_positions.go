package api

import (
	"errors"
	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"net/http"
)

func (s *Server) handleBookPosition(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	book, err := s.store.BookBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, 404, "not_found", "No such book.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "book position", err)
		return
	}
	user := currentUser(r)
	if book.License != "public_domain" && user.Tier == store.TierFree {
		httpx.Error(w, 402, "upgrade_required", "This book is available on Plus and Pro.")
		return
	}
	if err = s.store.EnsureReaderCards(r.Context(), book.EditionID); err != nil {
		httpx.InternalError(w, r, "book position", err)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			CardID string `json:"card_id"`
		}
		var id pgtype.UUID
		if err = httpx.DecodeJSON(w, r, &req); err != nil || id.Scan(req.CardID) != nil || !id.Valid {
			httpx.Error(w, 400, "bad_request", "Choose a card position.")
			return
		}
		if err = s.store.SaveBookPosition(r.Context(), user.ID, book.EditionID, req.CardID); errors.Is(err, store.ErrNotFound) {
			httpx.Error(w, 404, "not_found", "No such card in this book.")
			return
		}
		if err != nil {
			httpx.InternalError(w, r, "save book position", err)
			return
		}
		httpx.JSON(w, 200, map[string]string{"status": "saved"})
		return
	}
	position, err := s.store.BookPosition(r.Context(), user.ID, book.EditionID)
	if err != nil {
		httpx.InternalError(w, r, "book position", err)
		return
	}
	completion, err := s.store.BookCompletion(r.Context(), user.ID, book.EditionID)
	if err != nil {
		httpx.InternalError(w, r, "book completion", err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"position": position, "completion": completion})
}
