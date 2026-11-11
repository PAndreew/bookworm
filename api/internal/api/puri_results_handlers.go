package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func sourceHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// BYOK output is untrusted, private reader content, never a shared translation.
func (s *Server) handlePuriResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	id := chi.URLParam(r, "id")
	var uuid pgtype.UUID
	if uuid.Scan(id) != nil || !uuid.Valid {
		httpx.Error(w, 400, "bad_request", "Invalid reading card.")
		return
	}
	card, book, err := s.store.TranslationSource(r.Context(), id, currentUser(r).ID)
	if err != nil || book.License != "public_domain" {
		httpx.Error(w, 404, "not_found", "That text card is unavailable.")
		return
	}
	hash := sourceHash(card.Content)
	if r.Method == "GET" {
		results, err := s.store.PuriResults(r.Context(), currentUser(r).ID, id, hash)
		if err != nil {
			httpx.InternalError(w, r, "puri results", err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"results": results})
		return
	}
	var req struct {
		store.PuriResult
		Source string `json:"source"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if httpx.DecodeJSON(w, r, &req) != nil || !utf8.ValidString(req.Content) || len(req.Content) > 16000 || strings.TrimSpace(req.Content) == "" || len(req.Model) > 200 || req.Model == "" || len(req.Language) > 40 || strings.TrimSpace(req.Language) == "" || len(req.Phrase) > 4800 || req.Source != card.Content {
		httpx.Error(w, 400, "bad_request", "Invalid result or changed source card.")
		return
	}
	if req.Mode == "card_translate" {
		req.Phrase = ""
		if req.Language != "French" && req.Language != "Spanish" && req.Language != "German" {
			httpx.Error(w, 400, "bad_request", "Choose French, Spanish or German.")
			return
		}
	} else if (req.Mode != "explain" && req.Mode != "translate") || strings.TrimSpace(req.Phrase) == "" || !strings.Contains(card.Content, req.Phrase) {
		httpx.Error(w, 400, "bad_request", "Choose a passage from this card.")
		return
	}
	if err = s.store.SavePuriResult(r.Context(), currentUser(r).ID, id, hash, req.PuriResult); err != nil {
		httpx.InternalError(w, r, "save puri result", err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"saved": true})
}
