// Package httpx holds small HTTP helpers shared by the API handlers.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// JSON writes v as a JSON response.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writing json response", "err", err)
	}
}

// ErrorBody is the single error shape the API returns, so the client never has to
// guess at the format.
type ErrorBody struct {
	Error   string `json:"error"`             // machine-readable code
	Message string `json:"message,omitempty"` // human-readable, safe to display
}

// Error writes a structured error. The message must be safe to show a user: never
// put internal detail here.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorBody{Error: code, Message: message})
}

// InternalError logs the real cause and returns an opaque 500 to the caller.
//
// The client must never see internal detail, but a bare 500 with nothing in the log
// is undebuggable -- which is exactly how an IPv6 address parsing bug stayed hidden
// behind "Something went wrong."
func InternalError(w http.ResponseWriter, r *http.Request, op string, err error) {
	slog.Error("request failed", "op", op, "path", r.URL.Path, "err", err)
	Error(w, http.StatusInternalServerError, "internal", "Something went wrong.")
}

// DecodeJSON reads a JSON body with a size limit, rejecting unknown fields so that
// a typo in a client payload surfaces as an error instead of being silently dropped.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// QueryInt reads an integer query parameter, clamped to [min, max].
func QueryInt(r *http.Request, key string, fallback, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
