package api

import (
	"errors"
	"net/http"
	"regexp"
	"unicode/utf8"

	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
	"github.com/go-chi/chi/v5"
)

var cardUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type cardReportRequest struct {
	Category string `json:"category"`
	Note     string `json:"note"`
}

func validCardReport(id string, req cardReportRequest) bool {
	return cardUUID.MatchString(id) && (req.Category == "layout" || req.Category == "text" || req.Category == "break") && utf8.ValidString(req.Note) && utf8.RuneCountInString(req.Note) <= 1000
}
func (s *Server) handleCardReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var req cardReportRequest
	id := chi.URLParam(r, "id")
	if err := httpx.DecodeJSON(w, r, &req); err != nil || !validCardReport(id, req) {
		httpx.Error(w, 400, "bad_request", "Choose an issue type and keep your description under 1,000 characters.")
		return
	}
	err := s.store.ReportCard(r.Context(), currentUser(r).ID, id, req.Category, req.Note)
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpx.Error(w, 404, "not_found", "No such card.")
	case errors.Is(err, store.ErrForbidden):
		httpx.Error(w, 403, "forbidden", "This book is not available on your plan.")
	case errors.Is(err, store.ErrReportLimit):
		httpx.Error(w, 429, "report_limit", "You can submit up to 20 reports per day.")
	case err != nil:
		httpx.InternalError(w, r, "handleCardReport", err)
	default:
		httpx.JSON(w, 201, map[string]string{"status": "received"})
	}
}
