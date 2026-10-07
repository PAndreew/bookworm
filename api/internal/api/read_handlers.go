package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
)

func (s *Server) handleListBooks(w http.ResponseWriter, r *http.Request) {
	license := r.URL.Query().Get("license")
	if license != "" && license != "public_domain" && license != "licensed" {
		httpx.Error(w, http.StatusBadRequest, "bad_filter", "Unknown licence filter.")
		return
	}
	limit := httpx.QueryInt(r, "limit", 50, 1, 100)
	offset := httpx.QueryInt(r, "offset", 0, 0, 100000)

	books, err := s.store.ListBooks(r.Context(), license, limit, offset)
	if err != nil {
		httpx.InternalError(w, r, "handleListBooks", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"books": books})
}

func (s *Server) handleGetBook(w http.ResponseWriter, r *http.Request) {
	book, err := s.store.BookBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such book.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleGetBook", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"book": book})
}

// handleGetPages returns a window of pages for the feed.
//
// Note what this does NOT do: it does not meter. Fetching pages is free; the meter
// runs on POST /reads when a page is actually read. Metering on fetch would charge
// users for prefetch and for swiping back.
func (s *Server) handleGetPages(w http.ResponseWriter, r *http.Request) {
	book, err := s.store.BookBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such book.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleGetPages", err)
		return
	}

	// Licensed books need a paid tier even to open.
	if book.License == "licensed" {
		user := currentUser(r)
		if user == nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Please sign in.")
			return
		}
		if user.Tier.DailyLicensedPageLimit() == 0 {
			httpx.Error(w, http.StatusPaymentRequired, "upgrade_required",
				"This book is available on Plus and Pro.")
			return
		}
	}

	from := httpx.QueryInt(r, "from", 0, 0, 1000000)
	limit := httpx.QueryInt(r, "limit", 10, 1, 30)

	pages, err := s.store.PageWindow(r.Context(), book.EditionID, viewerID(r), from, limit)
	if err != nil {
		httpx.InternalError(w, r, "handleGetPages", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"pages":      pages,
		"page_count": book.PageCount,
	})
}

type readRequest struct {
	PageID string `json:"page_id"`
}

// handleRecordRead is the meter and the streak in one call. The client posts this
// when a page has actually been on screen long enough to count.
func (s *Server) handleRecordRead(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req readRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}

	meta, err := s.store.PageMeta(r.Context(), req.PageID)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such page.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleRecordRead", err)
		return
	}

	now := time.Now()
	localDay := store.LocalDay(user.Timezone, now)

	alreadyRead, err := s.store.HasReadToday(r.Context(), user.ID, meta.PageID, localDay)
	if err != nil {
		httpx.InternalError(w, r, "handleRecordRead", err)
		return
	}
	usedToday, err := s.store.MeteredPagesToday(r.Context(), user.ID, localDay)
	if err != nil {
		httpx.InternalError(w, r, "handleRecordRead", err)
		return
	}

	decision := store.DecideMeter(user.Tier, meta.License == "public_domain", alreadyRead, usedToday)
	if !decision.Allowed {
		httpx.JSON(w, http.StatusPaymentRequired, map[string]any{
			"error":     decision.Reason,
			"message":   limitMessage(decision.Reason),
			"resets_at": store.NextResetAt(user.Timezone, now),
		})
		return
	}

	if err := s.store.RecordRead(r.Context(), user.ID, meta.PageID, meta.EditionID,
		meta.Idx, localDay, decision.Metered); err != nil {
		httpx.InternalError(w, r, "handleRecordRead", err)
		return
	}

	// Fold today's reading into the streak.
	pagesToday, err := s.store.PagesReadToday(r.Context(), user.ID, localDay)
	if err != nil {
		httpx.InternalError(w, r, "handleRecordRead", err)
		return
	}
	current, err := s.store.GetStreak(r.Context(), user.ID)
	if err != nil {
		httpx.InternalError(w, r, "handleRecordRead", err)
		return
	}

	update := store.ApplyRead(current, localDay, pagesToday)
	if update.Extended {
		if err := s.store.SaveStreak(r.Context(), user.ID, update.Streak); err != nil {
			httpx.InternalError(w, r, "handleRecordRead", err)
			return
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"metered":         decision.Metered,
		"remaining":       decision.Remaining,
		"pages_today":     pagesToday,
		"streak_days":     update.CurrentDays,
		"streak_extended": update.Extended,
		"freeze_used":     update.FreezeUsed,
	})
}

func limitMessage(reason string) string {
	switch reason {
	case "upgrade_required":
		return "This book is available on Plus and Pro."
	case "daily_limit_reached":
		return "That is your reading for today. Your pages reset at midnight."
	default:
		return "You cannot read that right now."
	}
}

// ------------------------------------------------------------ likes & quotes

func (s *Server) handleLike(w http.ResponseWriter, r *http.Request) {
	if err := s.store.LikePage(r.Context(), currentUser(r).ID, chi.URLParam(r, "id")); err != nil {
		httpx.InternalError(w, r, "handleLike", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"liked": true})
}

func (s *Server) handleUnlike(w http.ResponseWriter, r *http.Request) {
	if err := s.store.UnlikePage(r.Context(), currentUser(r).ID, chi.URLParam(r, "id")); err != nil {
		httpx.InternalError(w, r, "handleUnlike", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"liked": false})
}

type highlightRequest struct {
	PageID   string `json:"page_id"`
	StartOff int    `json:"start_off"`
	EndOff   int    `json:"end_off"`
	Quote    string `json:"quote"`
	Share    bool   `json:"share"`
}

func (s *Server) handleCreateHighlight(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req highlightRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}
	if req.EndOff <= req.StartOff || req.StartOff < 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_range", "That selection is not valid.")
		return
	}

	// Trust the offsets, not the client's quote text: re-slice from the stored page
	// so a doctored request cannot attribute invented words to a book.
	quote, err := s.store.QuoteFromPage(r.Context(), req.PageID, req.StartOff, req.EndOff)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such page.")
		return
	}
	if errors.Is(err, store.ErrForbidden) {
		httpx.Error(w, http.StatusBadRequest, "invalid_range",
			"That selection does not fit on the page.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleCreateHighlight", err)
		return
	}

	var slug string
	if req.Share {
		slug = newShareSlug()
	}

	h, err := s.store.CreateHighlight(r.Context(), user.ID, req.PageID,
		req.StartOff, req.EndOff, quote, slug)
	if err != nil {
		httpx.InternalError(w, r, "handleCreateHighlight", err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"highlight": h})
}

func (s *Server) handleListHighlights(w http.ResponseWriter, r *http.Request) {
	limit := httpx.QueryInt(r, "limit", 50, 1, 100)
	offset := httpx.QueryInt(r, "offset", 0, 0, 100000)

	hs, err := s.store.ListHighlights(r.Context(), currentUser(r).ID, limit, offset)
	if err != nil {
		httpx.InternalError(w, r, "handleListHighlights", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"highlights": hs})
}

func (s *Server) handleDeleteHighlight(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteHighlight(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such highlight.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleDeleteHighlight", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetQuote serves a shared quote. Public on purpose: this is the link that
// travels outside the app, and it must open for someone with no account.
func (s *Server) handleGetQuote(w http.ResponseWriter, r *http.Request) {
	h, err := s.store.HighlightBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "That quote is no longer available.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "handleGetQuote", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"quote": h})
}

func newShareSlug() string {
	buf := make([]byte, 9)
	_, _ = rand.Read(buf)
	return strings.ToLower(base64.RawURLEncoding.EncodeToString(buf))
}

// --------------------------------------------------------------------- me

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	now := time.Now()
	localDay := store.LocalDay(user.Timezone, now)

	used, err := s.store.MeteredPagesToday(r.Context(), user.ID, localDay)
	if err != nil {
		httpx.InternalError(w, r, "handleMe", err)
		return
	}
	st, err := s.store.GetStreak(r.Context(), user.ID)
	if err != nil {
		httpx.InternalError(w, r, "handleMe", err)
		return
	}

	// Report the streak as the user would see it today, not the stored number.
	shown := st.CurrentDays
	if !st.IsAlive(localDay) {
		shown = 0
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"user": user,
		"quota": map[string]any{
			"limit":     user.Tier.DailyLicensedPageLimit(),
			"used":      used,
			"resets_at": store.NextResetAt(user.Timezone, now),
		},
		"streak": map[string]any{
			"current": shown,
			"longest": st.LongestDays,
			"at_risk": st.AtRisk(localDay),
		},
	})
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	var req struct {
		DisplayName string `json:"display_name"`
		Visibility  string `json:"visibility"`
		Timezone    string `json:"timezone"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Could not read that request.")
		return
	}

	if req.DisplayName == "" {
		req.DisplayName = user.DisplayName
	}
	switch req.Visibility {
	case "public", "friends", "private":
	case "":
		req.Visibility = user.Visibility
	default:
		httpx.Error(w, http.StatusBadRequest, "invalid_visibility", "Unknown visibility setting.")
		return
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil || req.Timezone == "" {
		req.Timezone = user.Timezone
	}

	if err := s.store.UpdateProfile(r.Context(), user.ID,
		req.DisplayName, req.Visibility, req.Timezone); err != nil {
		httpx.InternalError(w, r, "handleUpdateMe", err)
		return
	}

	updated, err := s.store.UserByID(r.Context(), user.ID)
	if err != nil {
		httpx.InternalError(w, r, "handleUpdateMe", err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": updated})
}

func (s *Server) handleGetStreak(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	localDay := store.LocalDay(user.Timezone, time.Now())

	st, err := s.store.GetStreak(r.Context(), user.ID)
	if err != nil {
		httpx.InternalError(w, r, "handleGetStreak", err)
		return
	}
	pagesToday, err := s.store.PagesReadToday(r.Context(), user.ID, localDay)
	if err != nil {
		httpx.InternalError(w, r, "handleGetStreak", err)
		return
	}

	shown := st.CurrentDays
	if !st.IsAlive(localDay) {
		shown = 0
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"current":        shown,
		"longest":        st.LongestDays,
		"at_risk":        st.AtRisk(localDay),
		"pages_today":    pagesToday,
		"pages_required": store.PagesPerStreakDay,
		"freezes_left":   st.FreezesLeft,
	})
}
