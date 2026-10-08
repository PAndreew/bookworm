// Package billing implements bounded Stripe calls and signed webhook verification.
package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const App = "potpuri-reader"

type Client struct {
	Key, Price, Coupon, BaseURL string
	HTTP                        *http.Client
}

func New(key, price, coupon string) *Client {
	return &Client{key, price, coupon, "https://api.stripe.com/v1", &http.Client{Timeout: 15 * time.Second}}
}

type Coupon struct {
	ID             string `json:"id"`
	TimesRedeemed  int    `json:"times_redeemed"`
	MaxRedemptions int    `json:"max_redemptions"`
	Valid          bool   `json:"valid"`
}

func (c *Client) OfferRemaining(ctx context.Context) (int, error) {
	var out Coupon
	err := c.call(ctx, "GET", "/coupons/"+url.PathEscape(c.Coupon), nil, "", &out)
	if err != nil {
		return 0, err
	}
	if out.MaxRedemptions != 50 {
		return 0, errors.New("billing: invalid offer cap")
	}
	n := 50 - out.TimesRedeemed
	if !out.Valid || n < 0 {
		n = 0
	}
	return n, nil
}
func (c *Client) call(ctx context.Context, method, path string, form url.Values, idempotency string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Key, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Stripe-Version", "2024-06-20")
	if idempotency != "" {
		req.Header.Set("Idempotency-Key", idempotency)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errors.New("billing: Stripe unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("billing: Stripe status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

type Checkout struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
}

func (c *Client) Checkout(ctx context.Context, user, email, lease, origin string, expires time.Time) (Checkout, error) {
	f := url.Values{"mode": {"subscription"}, "line_items[0][price]": {c.Price}, "line_items[0][quantity]": {"1"}, "success_url": {origin + "/app/me?subscription=success"}, "cancel_url": {origin + "/app/upgrade?subscription=cancelled"}, "client_reference_id": {user}, "customer_email": {email}, "expires_at": {strconv.FormatInt(expires.Unix(), 10)}}
	f.Set("payment_method_types[0]", "card")
	f.Set("discounts[0][coupon]", c.Coupon) // Stripe atomically caps this offer at fifty redemptions.
	for _, prefix := range []string{"metadata", "subscription_data[metadata]"} {
		f.Set(prefix+"[app]", App)
		f.Set(prefix+"[user_id]", user)
		f.Set(prefix+"[lease_id]", lease)
	}
	var out Checkout
	err := c.call(ctx, "POST", "/checkout/sessions", f, "reader-checkout-"+lease, &out)
	if err == nil && (out.ID == "" || !strings.HasPrefix(out.URL, "https://checkout.stripe.com/")) {
		err = errors.New("billing: invalid checkout response")
	}
	return out, err
}

type Subscription struct {
	ID                string            `json:"id"`
	Customer          string            `json:"customer"`
	Status            string            `json:"status"`
	CurrentPeriodEnd  int64             `json:"current_period_end"`
	CancelAtPeriodEnd bool              `json:"cancel_at_period_end"`
	Metadata          map[string]string `json:"metadata"`
	Items             struct {
		Data []struct {
			CurrentPeriodEnd int64 `json:"current_period_end"`
			Quantity         int   `json:"quantity"`
			Price            struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

func (c *Client) Subscription(ctx context.Context, id string) (Subscription, error) {
	var s Subscription
	err := c.call(ctx, "GET", "/subscriptions/"+url.PathEscape(id), nil, "", &s)
	return s, err
}
func (c *Client) Cancel(ctx context.Context, id string) (Subscription, error) {
	var s Subscription
	err := c.call(ctx, "POST", "/subscriptions/"+url.PathEscape(id), url.Values{"cancel_at_period_end": {"true"}}, "reader-cancel-"+id, &s)
	return s, err
}
func (s Subscription) PeriodEnd() time.Time {
	n := s.CurrentPeriodEnd
	if n == 0 && len(s.Items.Data) > 0 {
		n = s.Items.Data[0].CurrentPeriodEnd
	}
	return time.Unix(n, 0).UTC()
}
func (c *Client) Valid(s Subscription) bool {
	return s.ID != "" && s.Customer != "" && s.Metadata["app"] == App && s.Metadata["user_id"] != "" && s.Metadata["lease_id"] != "" && len(s.Items.Data) == 1 && s.Items.Data[0].Price.ID == c.Price && s.Items.Data[0].Quantity == 1 && s.PeriodEnd().Unix() > 0
}
func Verify(body []byte, header, secret string, now time.Time) error {
	var timestamp string
	signatures := []string{}
	for _, p := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "t" {
			timestamp = kv[1]
		}
		if kv[0] == "v1" {
			signatures = append(signatures, kv[1])
		}
	}
	n, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || secret == "" || now.Unix()-n > 300 || n-now.Unix() > 300 {
		return errors.New("billing: invalid signature timestamp")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	for _, signature := range signatures {
		decoded, err := hex.DecodeString(signature)
		if err == nil && hmac.Equal(decoded, mac.Sum(nil)) {
			return nil
		}
	}
	return errors.New("billing: invalid signature")
}
