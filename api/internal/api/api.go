// Package api wires the HTTP routes.
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bookworm/api/internal/assistant"
	"github.com/bookworm/api/internal/config"
	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
)

type Server struct {
	cfg       *config.Config
	store     *store.Store
	assistant *assistant.Client
}

func New(cfg *config.Config, st *store.Store) *Server {
	s := &Server{cfg: cfg, store: st}
	if cfg.OpenRouterKey != "" {
		s.assistant = assistant.New(cfg.OpenRouterKey, cfg.OpenRouterModel)
	}
	return s
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(httpx.Recoverer)
	r.Use(httpx.Logger)
	r.Use(httpx.SecurityHeaders)
	r.Use(httpx.CORS(s.cfg.AllowOrigins))

	// Credential endpoints get a tighter limit than the rest of the API.
	authLimit := httpx.NewRateLimiter(10, time.Minute)
	readLimit := httpx.NewRateLimiter(300, time.Minute)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/v1", func(r chi.Router) {
		// ---------------------------------------------------------- public
		r.Group(func(r chi.Router) {
			r.Use(authLimit.Middleware)
			r.Post("/auth/signup", s.handleSignup)
			r.Post("/auth/login", s.handleLogin)
			r.Post("/auth/2fa/verify", s.handle2FAVerify) // needs a pending session
		})

		r.Group(func(r chi.Router) {
			r.Use(readLimit.Middleware)
			r.Use(s.optionalAuth) // likes show "liked_by_me" when signed in
			r.Get("/books", s.handleListBooks)
			r.Get("/genres", s.handleGenres)
			r.Get("/books/{slug}", s.handleGetBook)
			r.Get("/books/{slug}/pages", s.handleGetPages)
			r.Get("/books/{slug}/cards", s.handleGetCards)
			r.Get("/quotes/{slug}", s.handleGetQuote)
		})

		r.Post("/legal/takedown", s.handleTakedown)

		// ------------------------------------------------- authenticated
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Post("/auth/logout", s.handleLogout)
			r.Get("/me", s.handleMe)
			r.Get("/me/ratings", s.handleRatings)
			r.Put("/books/{slug}/rating", s.handleRateBook)
			r.Get("/books/{slug}/position", s.handleBookPosition)
			r.Post("/books/{slug}/position", s.handleBookPosition)
			r.Patch("/me", s.handleUpdateMe)

			r.Post("/auth/2fa/setup", s.handle2FASetup)
			r.Post("/auth/2fa/enable", s.handle2FAEnable)
			r.Post("/auth/2fa/disable", s.handle2FADisable)

			r.Post("/assistant", s.handleAssistant)
			r.Post("/reads", s.handleRecordRead)
			r.Post("/card-reads", s.handleCardRead)
			r.Post("/pages/{id}/like", s.handleLike)
			r.Delete("/pages/{id}/like", s.handleUnlike)

			r.Get("/highlights", s.handleListHighlights)
			r.Post("/highlights", s.handleCreateHighlight)
			r.Delete("/highlights/{id}", s.handleDeleteHighlight)

			r.Get("/streak", s.handleGetStreak)
			r.Get("/feed/friends", s.handleFriendFeed)

			r.Get("/friends", s.handleListFriends)
			r.Get("/friends/pending", s.handlePendingFriends)
			r.Post("/friends/{handle}", s.handleRequestFriend)
			r.Post("/friends/{handle}/accept", s.handleAcceptFriend)
			r.Delete("/friends/{handle}", s.handleRemoveFriend)
			r.Post("/friends/{handle}/block", s.handleBlockUser)

			r.Get("/users/search", s.handleSearchUsers)
			r.Get("/users/{handle}", s.handleGetProfile)
		})
	})

	return r
}
