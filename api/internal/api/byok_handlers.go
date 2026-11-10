package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
)

func (s *Server) handleBYOKQuota(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	q, err := s.store.BYOKQuota(r.Context(), currentUser(r).ID)
	if err != nil {
		httpx.InternalError(w, r, "BYOK quota", err)
		return
	}
	httpx.JSON(w, 200, q)
}

func (s *Server) handleBYOKReserve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	// Empty object only: reject accidentally attached keys, prompts or results.
	r.Body = http.MaxBytesReader(w, r.Body, 128)
	var body *struct{}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&body)
	if err != nil || body == nil || dec.Decode(new(any)) != io.EOF {
		httpx.Error(w, 400, "bad_request", "Send an empty object only. Never send API keys or reading text to this endpoint.")
		return
	}
	q, err := s.store.ReserveBYOKAttempt(r.Context(), currentUser(r).ID)
	if errors.Is(err, store.ErrBYOKLimit) {
		httpx.Error(w, 429, "byok_limit", "Your 50 BYOK requests for this calendar month have been used. Resets on the first of next month (UTC).")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "BYOK reservation", err)
		return
	}
	httpx.JSON(w, 200, q)
}
