package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bookworm/api/internal/billing"
	"github.com/bookworm/api/internal/httpx"
	"github.com/bookworm/api/internal/store"
	"io"
	"net/http"
	"time"
)

func (s *Server) handleOffer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.stripe == nil {
		httpx.JSON(w, 200, map[string]any{"available": false, "remaining": 0})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	n, err := s.stripe.OfferRemaining(ctx)
	if err != nil {
		httpx.Error(w, 503, "billing_unavailable", "Subscriptions are temporarily unavailable.")
		return
	}
	pending, err := s.store.OfferRemaining(ctx)
	if err != nil {
		httpx.InternalError(w, r, "offer capacity", err)
		return
	}
	if pending < n {
		n = pending
	}
	httpx.JSON(w, 200, map[string]any{"available": n > 0, "remaining": n, "limit": 50, "price_eur": 20, "cadence": "year", "requests_per_month": 100})
}
func (s *Server) handleBilling(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	u := currentUser(r)
	expired, err := s.store.ReconcilePaidTier(r.Context(), u.ID)
	if err != nil {
		httpx.InternalError(w, r, "billing access", err)
		return
	}
	if expired {
		u.Tier = store.TierFree
	}
	sub, err := s.store.Subscription(r.Context(), u.ID)
	if err != nil {
		httpx.InternalError(w, r, "billing status", err)
		return
	}
	used, err := s.store.AssistantUsage(r.Context(), u.ID)
	if err != nil {
		httpx.InternalError(w, r, "billing usage", err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"subscription": sub, "tier": u.Tier, "used": used, "limit": u.Tier.AssistantLimit(), "renews_monthly_quota_at": time.Date(time.Now().UTC().Year(), time.Now().UTC().Month()+1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")})
}
func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if s.stripe == nil {
		httpx.Error(w, 503, "billing_unavailable", "Subscriptions are not available yet.")
		return
	}
	u := currentUser(r)
	sub, err := s.store.Subscription(r.Context(), u.ID)
	if err != nil {
		httpx.InternalError(w, r, "checkout subscription", err)
		return
	}
	if sub != nil && sub.Status == "active" && sub.PeriodEnd.After(time.Now()) {
		httpx.Error(w, 409, "already_subscribed", "You already have an active subscription.")
		return
	}
	n, err := s.stripe.OfferRemaining(r.Context())
	if err != nil {
		httpx.Error(w, 503, "billing_unavailable", "Could not contact Stripe.")
		return
	}
	if n == 0 {
		httpx.Error(w, 409, "offer_full", "The fifty founding places have been taken.")
		return
	}
	lease, err := s.store.ReserveOffer(r.Context(), u.ID)
	if errors.Is(err, store.ErrOfferFull) {
		httpx.Error(w, 409, "offer_full", "All founding places are taken or temporarily reserved. Please try later.")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, "checkout lease", err)
		return
	}
	if lease.URL != "" {
		httpx.JSON(w, 200, map[string]string{"url": lease.URL})
		return
	}
	checkout, err := s.stripe.Checkout(r.Context(), u.ID, u.Email, lease.ID, s.cfg.PublicURL, lease.Expires)
	if err != nil {
		httpx.Error(w, 503, "billing_unavailable", "Could not open Stripe Checkout. Please try again.")
		return
	}
	if err = s.store.SaveCheckout(r.Context(), u.ID, lease.ID, checkout.ID, checkout.URL); err != nil {
		httpx.InternalError(w, r, "checkout save", err)
		return
	}
	httpx.JSON(w, 200, map[string]string{"url": checkout.URL})
}
func (s *Server) handleCancelSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	select {
	case s.billingWork <- struct{}{}:
		defer func() { <-s.billingWork }()
	default:
		httpx.Error(w, 503, "billing_busy", "Billing is busy. Please try again shortly.")
		return
	}
	u := currentUser(r)
	sub, err := s.store.Subscription(r.Context(), u.ID)
	if err != nil {
		httpx.InternalError(w, r, "cancel subscription", err)
		return
	}
	if sub == nil {
		if err = s.store.DropComplimentaryTier(r.Context(), u.ID); err != nil {
			httpx.InternalError(w, r, "cancel complimentary access", err)
			return
		}
		httpx.JSON(w, 200, map[string]string{"message": "Your complimentary paid access has ended. There was no Stripe renewal."})
		return
	}
	if s.stripe == nil {
		httpx.Error(w, 503, "billing_unavailable", "Could not contact Stripe.")
		return
	}
	lock, err := s.store.LockSubscription(r.Context(), sub.ID)
	if err != nil {
		httpx.InternalError(w, r, "cancel lock", err)
		return
	}
	defer lock.Rollback(r.Context())
	current, err := s.stripe.Cancel(r.Context(), sub.ID)
	if err != nil {
		httpx.Error(w, 503, "billing_unavailable", "Could not cancel renewal. Your subscription is unchanged; please try again.")
		return
	}
	if !s.stripe.Valid(current) || current.Metadata["user_id"] != u.ID {
		httpx.Error(w, 503, "billing_unavailable", "Could not verify the subscription.")
		return
	}
	err = s.store.ApplySubscription(r.Context(), "cancel-"+current.ID+"-"+time.Now().UTC().Format(time.RFC3339Nano), u.ID, current.Metadata["lease_id"], store.Subscription{Customer: current.Customer, ID: current.ID, Status: current.Status, PeriodEnd: current.PeriodEnd(), CancelAtPeriodEnd: current.CancelAtPeriodEnd})
	if err != nil {
		httpx.InternalError(w, r, "cancel save", err)
		return
	}
	httpx.JSON(w, 200, map[string]string{"message": "Renewal cancelled. Your access continues until " + current.PeriodEnd().Format("2 January 2006") + "."})
}
func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if s.stripe == nil {
		httpx.Error(w, 503, "billing_unavailable", "Billing unavailable.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	raw, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		httpx.Error(w, 400, "bad_request", "Invalid event.")
		return
	}
	if err := billing.Verify(raw, r.Header.Get("Stripe-Signature"), s.cfg.StripeWebhookSecret, time.Now()); err != nil {
		httpx.Error(w, 400, "bad_signature", "Invalid signature.")
		return
	}
	select {
	case s.billingWork <- struct{}{}:
		defer func() { <-s.billingWork }()
	default:
		httpx.Error(w, 503, "billing_busy", "Please retry this event.")
		return
	}
	var event struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID           string            `json:"id"`
				Subscription string            `json:"subscription"`
				Metadata     map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &event) != nil || event.ID == "" {
		httpx.Error(w, 400, "bad_request", "Invalid event.")
		return
	}
	object := event.Data.Object
	if object.Metadata["app"] != billing.App {
		w.WriteHeader(204)
		return
	}
	var id string
	switch event.Type {
	case "checkout.session.completed":
		id = object.Subscription
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		id = object.ID
	default:
		w.WriteHeader(204)
		return
	}
	if id == "" {
		w.WriteHeader(204)
		return
	}
	// Serialise processing across replicas, then fetch the authoritative Stripe state.
	// Out-of-order deliveries cannot undo a newer cancellation or payment update.
	lock, err := s.store.LockSubscription(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, r, "webhook lock", err)
		return
	}
	defer lock.Rollback(r.Context())
	current, err := s.stripe.Subscription(r.Context(), id)
	if err != nil {
		httpx.Error(w, 503, "billing_unavailable", "Could not verify subscription.")
		return
	}
	if !s.stripe.Valid(current) {
		w.WriteHeader(204)
		return
	}
	err = s.store.ApplySubscription(r.Context(), event.ID, current.Metadata["user_id"], current.Metadata["lease_id"], store.Subscription{Customer: current.Customer, ID: current.ID, Status: current.Status, PeriodEnd: current.PeriodEnd(), CancelAtPeriodEnd: current.CancelAtPeriodEnd})
	if err != nil {
		httpx.InternalError(w, r, "webhook subscription", err)
		return
	}
	w.WriteHeader(204)
}
