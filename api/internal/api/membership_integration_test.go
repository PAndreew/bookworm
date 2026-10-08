package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bookworm/api/internal/assistant"
	"github.com/bookworm/api/internal/auth"
	"github.com/bookworm/api/internal/billing"
	"github.com/bookworm/api/internal/chunk"
	"github.com/bookworm/api/internal/config"
	"github.com/bookworm/api/internal/db"
	"github.com/bookworm/api/internal/ingest"
	"github.com/bookworm/api/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeAssistant struct{ calls atomic.Int32 }

func (f *fakeAssistant) Ask(ctx context.Context, req assistant.Request) (string, error) {
	f.calls.Add(1)
	if req.Mode != "card_translate" {
		return "An explanation.", nil
	}
	return "Une traduction littéraire de longueur semblable.", nil
}
func TestFoundingMembershipAndTranslations(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable TEST_DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	suffix := fmt.Sprint(time.Now().UnixNano())
	users := []string{}
	tokens := []string{}
	for i := 0; i < 3; i++ {
		handle := fmt.Sprintf("m%d%s", i, suffix)
		u, err := st.CreateUser(ctx, handle+"@example.invalid", handle, "Membership test", "unused", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "UTC")
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
		defer pool.Exec(ctx, `UPDATE founding_slots SET user_id=NULL,paid=false,expires_at=now(),checkout_id=NULL,checkout_url=NULL WHERE user_id=$1`, u.ID)
		token, hash, _ := auth.NewSessionToken()
		if err = st.CreateSession(ctx, u.ID, hash, false, "test", "", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		users = append(users, u.ID)
		tokens = append(tokens, token)
	}
	fake := &fakeAssistant{}
	server := New(&config.Config{StripeWebhookSecret: "test-webhook", PublicURL: "https://potpuri.cc", OpenRouterModel: "google/gemini-3.8-flash"}, st)
	server.assistant = fake
	var subscription billing.Subscription
	stripe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/coupons/"):
			fmt.Fprint(w, `{"id":"coupon","max_redemptions":50,"times_redeemed":0,"valid":true}`)
		case r.URL.Path == "/checkout/sessions":
			r.ParseForm()
			if r.Form.Get("discounts[0][coupon]") != "coupon" {
				t.Error("founding discount missing")
			}
			fmt.Fprint(w, `{"id":"cs_test","url":"https://checkout.stripe.com/c/pay/fake"}`)
		case strings.HasPrefix(r.URL.Path, "/subscriptions/"):
			if r.Method == "POST" {
				r.ParseForm()
				if r.Form.Get("cancel_at_period_end") != "true" {
					t.Error("cancel ends access immediately")
				}
				subscription.CancelAtPeriodEnd = true
			}
			json.NewEncoder(w).Encode(subscription)
		default:
			t.Errorf("unexpected Stripe path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer stripe.Close()
	server.stripe = billing.New("fake", "price", "coupon")
	server.stripe.BaseURL = stripe.URL
	router := server.Routes()
	call := func(method, path, body, token string, status int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s status %d want %d: %s", method, path, rec.Code, status, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	call("POST", "/v1/billing/checkout", "{}", "", 401)
	call("POST", "/v1/billing/checkout", "{}", tokens[0], 200)
	lease, err := st.ReserveOffer(ctx, users[0])
	if err != nil {
		t.Fatal(err)
	}
	same, err := st.ReserveOffer(ctx, users[0])
	if err != nil || same.ID != lease.ID {
		t.Fatal("checkout retries reserved multiple places")
	}
	subscription.ID = "sub_test"
	subscription.Customer = "cus_test"
	subscription.Status = "active"
	subscription.CurrentPeriodEnd = time.Now().Add(365 * 24 * time.Hour).Unix()
	subscription.Metadata = map[string]string{"app": billing.App, "user_id": users[0], "lease_id": lease.ID}
	subscription.Items.Data = append(subscription.Items.Data, struct {
		CurrentPeriodEnd int64 `json:"current_period_end"`
		Quantity         int   `json:"quantity"`
		Price            struct {
			ID string `json:"id"`
		} `json:"price"`
	}{Quantity: 1})
	subscription.Items.Data[0].Price.ID = "price"
	webhook := func(eventID string) {
		t.Helper()
		body := fmt.Sprintf(`{"id":%q,"type":"customer.subscription.updated","data":{"object":{"id":"sub_test","metadata":{"app":"potpuri-reader"}}}}`+"\n", eventID)
		stamp := fmt.Sprint(time.Now().Unix())
		mac := hmac.New(sha256.New, []byte("test-webhook"))
		mac.Write([]byte(stamp + "." + body))
		req := httptest.NewRequest("POST", "/v1/billing/webhook", strings.NewReader(body))
		req.Header.Set("Stripe-Signature", "t="+stamp+",v1="+hex.EncodeToString(mac.Sum(nil)))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != 204 {
			t.Fatalf("webhook %d: %s", rec.Code, rec.Body.String())
		}
	}
	webhook("evt_" + suffix)
	webhook("evt_" + suffix)
	call("POST", "/v1/billing/checkout", "{}", tokens[0], 409)
	var status struct {
		Tier  string `json:"tier"`
		Limit int    `json:"limit"`
	}
	json.Unmarshal(call("GET", "/v1/billing", "", tokens[0], 200), &status)
	if status.Tier != "plus" || status.Limit != 100 {
		t.Fatal("founding membership not granted")
	}
	book := ingest.Book{Slug: "translation-" + suffix, Title: "Translation fixture", Author: "Tester", License: "public_domain", Pages: chunk.Split(strings.Repeat("A literary passage with an unfamiliar word. ", 30))}
	if _, err = ingest.New(pool).Publish(ctx, book); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM books WHERE slug=$1`, book.Slug)
	stored, err := st.BookBySlug(ctx, book.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureReaderCards(ctx, stored.EditionID); err != nil {
		t.Fatal(err)
	}
	cards, _, err := st.CardWindow(ctx, stored.EditionID, users[0], 1, 0, 1)
	if err != nil || len(cards) != 1 {
		t.Fatal(err)
	}
	endpoint := "/v1/cards/" + cards[0].CardID + "/translations"
	call("POST", endpoint, `{"language":"French"}`, tokens[1], 402)
	call("POST", endpoint, `{"language":"Italian"}`, tokens[0], 400)
	call("POST", endpoint, `{"language":"French"}`, tokens[0], 200)
	if fake.calls.Load() != 1 {
		t.Fatal("generation did not call provider exactly once")
	}
	if _, err = pool.Exec(ctx, `UPDATE users SET tier='plus' WHERE id=$1`, users[1]); err != nil {
		t.Fatal(err)
	}
	var cached struct {
		Cached bool `json:"cached"`
	}
	json.Unmarshal(call("POST", endpoint, `{"language":"French"}`, tokens[1], 200), &cached)
	if !cached.Cached || fake.calls.Load() != 1 {
		t.Fatal("shared cached translation charged AI again")
	}
	used, err := st.AssistantUsage(ctx, users[1])
	if err != nil || used != 0 {
		t.Fatal("cached translation consumed quota")
	}
	call("GET", endpoint, "", tokens[1], 200)
	if _, err = pool.Exec(ctx, `UPDATE assistant_monthly_usage SET requests=100,last_request_at=now()-interval '10 seconds' WHERE user_id=$1`, users[0]); err != nil {
		t.Fatal(err)
	}
	call("POST", endpoint, `{"language":"German"}`, tokens[0], 429)
	if fake.calls.Load() != 1 {
		t.Fatal("exhausted allowance called provider")
	}
	call("POST", "/v1/billing/cancel", "{}", tokens[0], 200)
	sub, err := st.Subscription(ctx, users[0])
	if err != nil || !sub.CancelAtPeriodEnd || sub.Status != "active" {
		t.Fatal("cancellation revoked paid year")
	}
	call("GET", "/v1/billing", "", tokens[0], 200)
	subscription.Status = "canceled"
	webhook("evt_end_" + suffix)
	json.Unmarshal(call("GET", "/v1/billing", "", tokens[0], 200), &status)
	if status.Tier != "free" {
		t.Fatal("expired subscription retained AI access")
	}
	defer pool.Exec(ctx, `DELETE FROM reader_stripe_events WHERE id LIKE $1`, "%"+suffix)
	defer pool.Exec(ctx, `DELETE FROM assistant_global_usage WHERE day=(now() AT TIME ZONE 'UTC')::date`)
	// Fifty seats, including temporary holds, cannot be oversold by the app.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE founding_slots SET expires_at=now()+interval '1 hour' WHERE NOT paid`); err != nil {
		t.Fatal(err)
	}
	// Verify cap with the committed store after temporarily reserving all free rows.
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `UPDATE founding_slots SET expires_at=now() WHERE user_id IS NULL AND NOT paid`)
	if _, err = st.ReserveOffer(ctx, users[2]); !errors.Is(err, store.ErrOfferFull) {
		t.Fatal("51st seat accepted", err)
	}
}
