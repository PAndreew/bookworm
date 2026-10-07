package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetCards(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	book, err := s.store.BookBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, 404, "not_found", "No such book.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleGetCards", err)
		return
	}
	if book.License != "public_domain" {
		user := currentUser(r)
		if user == nil {
			httpx.Error(w, 401, "unauthorized", "Please sign in.")
			return
		}
		if user.Tier == store.TierFree {
			httpx.Error(w, 402, "upgrade_required", "This book is available on Plus and Pro.")
			return
		}
	}
	cards, count, err := s.store.CardWindow(r.Context(), book.EditionID, viewerID(r), httpx.QueryInt(r, "card_from", 0, 0, 1000000), httpx.QueryInt(r, "source_from", 0, 0, 1000000), httpx.QueryInt(r, "limit", 12, 1, 30))
	if err != nil {
		httpx.InternalError(w, r, "handleGetCards", err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"pages": cards, "page_count": count})
}

func (s *Server) handleCardRead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CardID string `json:"card_id"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, 400, "bad_request", "Could not read that request.")
		return
	}
	user := currentUser(r)
	now := time.Now()
	decision, day, err := s.store.RecordCard(r.Context(), user.ID, req.CardID, now)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, 404, "not_found", "No such card.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleCardRead", err)
		return
	}
	if !decision.Allowed {
		_, reset := user.Tier.QuotaPeriod(user.Timezone, now)
		httpx.JSON(w, 402, map[string]any{"error": decision.Reason, "message": limitMessage(decision.Reason), "resets_at": reset})
		return
	}
	s.respondRead(w, r, user, day, decision)
}

func (s *Server) respondRead(w http.ResponseWriter, r *http.Request, user *store.User, day time.Time, decision store.MeterDecision) {
	pages, err := s.store.PagesReadToday(r.Context(), user.ID, day)
	if err != nil {
		httpx.InternalError(w, r, "read streak", err)
		return
	}
	current, err := s.store.GetStreak(r.Context(), user.ID)
	if err != nil {
		httpx.InternalError(w, r, "read streak", err)
		return
	}
	update := store.ApplyRead(current, day, pages)
	if update.Extended {
		if err := s.store.SaveStreak(r.Context(), user.ID, update.Streak); err != nil {
			httpx.InternalError(w, r, "read streak", err)
			return
		}
	}
	httpx.JSON(w, 200, map[string]any{"metered": decision.Metered, "remaining": decision.Remaining, "pages_today": pages, "streak_days": update.CurrentDays, "streak_extended": update.Extended, "freeze_used": update.FreezeUsed})
}
