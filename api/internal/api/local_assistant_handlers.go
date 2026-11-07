package api

import (
	"errors"
	"net/http"

	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
)

// No selected text or generated answer is sent to this endpoint.
func (s *Server) handleLocalAssistant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	reserve := r.Method == http.MethodPost
	if reserve {
		var req struct {
			CardID string `json:"card_id"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if err := httpx.DecodeJSON(w, r, &req); err != nil || req.CardID == "" {
			httpx.Error(w, 400, "bad_request", "Choose a reading card first.")
			return
		}
		if _, _, err := s.store.TranslationSource(r.Context(), req.CardID, currentUser(r).ID); err != nil {
			httpx.Error(w, 400, "invalid_selection", "Choose an accessible reading card.")
			return
		}
	}
	usage, err := s.store.LocalAssistantAllowance(r.Context(), currentUser(r).ID, reserve)
	if errors.Is(err, store.ErrAssistantLimit) {
		httpx.Error(w, 429, "local_assistant_limit", "Your on-device allowance is used up, or a request was made less than five seconds ago. Allowances reset on the first of each month (UTC).")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "local assistant allowance", err)
		return
	}
	httpx.JSON(w, 200, usage)
}
