package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/bookworm/api/internal/assistant"
	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"net/http"
	"time"
	"unicode/utf8"
)

func assistantQuotaError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, store.ErrAssistantUnpaid):
		httpx.Error(w, 402, "upgrade_required", "Subscribe to Plus to use Puri's AI companion.")
	case errors.Is(err, store.ErrAssistantLimit):
		w.Header().Set("Retry-After", "5")
		httpx.Error(w, 429, "assistant_limit", "Your monthly AI allowance is used up, or a request was made less than five seconds ago. Allowances reset on the first of each month (UTC).")
	case errors.Is(err, store.ErrAssistantCapacity):
		httpx.Error(w, 429, "assistant_capacity", "Today's AI service capacity has been reached. Please try tomorrow.")
	default:
		return false
	}
	return true
}
func (s *Server) handleCardTranslations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	u := currentUser(r)
	expired, err := s.store.ReconcilePaidTier(ctx, u.ID)
	if err != nil {
		httpx.InternalError(w, r, "translation access", err)
		return
	}
	if u.Tier == store.TierFree || expired {
		httpx.Error(w, 402, "upgrade_required", "Subscribe to Plus for saved card translations.")
		return
	}
	id := chi.URLParam(r, "id")
	var uuid pgtype.UUID
	if uuid.Scan(id) != nil || !uuid.Valid {
		httpx.Error(w, 400, "bad_request", "Invalid reading card.")
		return
	}
	card, book, err := s.store.TranslationSource(ctx, id, u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, 404, "not_found", "That text card is unavailable.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "translation source", err)
		return
	}
	if book.License != "public_domain" {
		httpx.Error(w, 404, "not_found", "Translations currently support free books only.")
		return
	}
	sum := sha256.Sum256([]byte(card.Content))
	hash := hex.EncodeToString(sum[:])
	if r.Method == "GET" {
		translations, err := s.store.Translations(ctx, id, hash)
		if err != nil {
			httpx.InternalError(w, r, "translations", err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"translations": translations})
		return
	}
	var req struct {
		Language string `json:"language"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if httpx.DecodeJSON(w, r, &req) != nil || (req.Language != "French" && req.Language != "Spanish" && req.Language != "German") {
		httpx.Error(w, 400, "bad_request", "Choose French, Spanish or German.")
		return
	}
	select {
	case s.translationWork <- struct{}{}:
		defer func() { <-s.translationWork }()
	default:
		w.Header().Set("Retry-After", "2")
		httpx.Error(w, 429, "translation_busy", "The AI service is busy. Please try again shortly.")
		return
	}
	tx, cached, err := s.store.LockTranslation(ctx, id, req.Language, hash)
	if errors.Is(err, store.ErrTranslationBusy) {
		w.Header().Set("Retry-After", "2")
		httpx.Error(w, 409, "translation_busy", "This translation is being prepared. Please try again shortly.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "translation lock", err)
		return
	}
	defer tx.Rollback(ctx)
	if cached != nil {
		httpx.JSON(w, 200, map[string]any{"translation": cached, "cached": true})
		return
	}
	if s.assistant == nil {
		httpx.Error(w, 503, "assistant_unavailable", "The reading companion is unavailable.")
		return
	}
	request := assistant.Request{Mode: "card_translate", Phrase: card.Content, Language: req.Language, Consent: true}
	if request.Validate() != nil {
		httpx.Error(w, 400, "bad_request", "This card is too long to translate.")
		return
	}
	err = s.store.ReserveAssistantAttempt(ctx, u.ID)
	if assistantQuotaError(w, err) {
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "translation quota", err)
		return
	}
	answer, err := s.assistant.Ask(ctx, request)
	if err != nil || !utf8.ValidString(answer) || utf8.RuneCountInString(answer) > 4000 {
		httpx.Error(w, 503, "assistant_unavailable", "The translation could not be completed. Please try again shortly.")
		return
	}
	translation := store.Translation{Language: req.Language, Content: answer, Model: s.cfg.OpenRouterModel}
	if err = store.SaveTranslation(ctx, tx, id, hash, translation); err != nil {
		httpx.InternalError(w, r, "translation save", err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"translation": translation, "cached": false})
}
