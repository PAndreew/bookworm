package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bookworm/api/internal/assistant"
	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
)

func (s *Server) handleAssistant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	// Keep the entire operation inside the HTTP server's 30-second write timeout.
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if currentUser(r).Tier == store.TierFree {
		httpx.Error(w, http.StatusPaymentRequired, "upgrade_required", "Subscribe to Plus for Puri's AI explanations and card translations.")
		return
	}
	var req assistant.Request
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, 400, "bad_request", "Could not read that phrase request.")
		return
	}
	if req.Mode == "card_translate" {
		httpx.Error(w, 400, "bad_request", "Use a book card to request a saved translation.")
		return
	}
	if err := req.Validate(); err != nil {
		httpx.Error(w, 400, "bad_request", err.Error())
		return
	}
	if req.CardID == "" {
		httpx.Error(w, 400, "selection_required", "Highlight a passage on a reading card first.")
		return
	}
	card, _, sourceErr := s.store.TranslationSource(r.Context(), req.CardID, currentUser(r).ID)
	if sourceErr != nil || !strings.Contains(card.Content, strings.TrimSpace(req.Phrase)) {
		httpx.Error(w, 400, "invalid_selection", "Choose a passage from an accessible reading card.")
		return
	}
	if s.assistant == nil {
		httpx.Error(w, 503, "assistant_unavailable", "The reading companion is not configured yet.")
		return
	}
	err := s.store.ReserveAssistantAttempt(r.Context(), currentUser(r).ID)
	if assistantQuotaError(w, err) {
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleAssistant quota", err)
		return
	}
	answer, err := s.assistant.Ask(r.Context(), req)
	if err != nil {
		// Errors contain status codes only, never credentials, phrases or provider bodies.
		slog.Warn("reading companion unavailable", "err", err)
		httpx.Error(w, 503, "assistant_unavailable", "The reading companion could not answer. Please try again shortly.")
		return
	}
	if err := s.store.SavePuriResult(r.Context(), currentUser(r).ID, req.CardID, sourceHash(card.Content), store.PuriResult{Mode: req.Mode, Language: req.Language, Phrase: strings.TrimSpace(req.Phrase), Content: answer, Model: s.cfg.OpenRouterModel}); err != nil {
		httpx.InternalError(w, r, "save explanation", err)
		return
	}
	httpx.JSON(w, 200, map[string]string{"answer": answer, "model": s.cfg.OpenRouterModel})
}
