package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignature(t *testing.T) {
	now := time.Now()
	body := []byte("{\"id\":\"evt_test\"}\n")
	stamp := fmt.Sprint(now.Unix())
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte(stamp + "."))
	mac.Write(body)
	header := "t=" + stamp + ",v1=00,v1=" + hex.EncodeToString(mac.Sum(nil))
	if err := Verify(body, header, "test-secret", now); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		body   []byte
		secret string
		at     time.Time
	}{{[]byte("{}"), "test-secret", now}, {body, "wrong", now}, {body, "test-secret", now.Add(301 * time.Second)}} {
		if Verify(bad.body, header, bad.secret, bad.at) == nil {
			t.Fatal("invalid signature accepted")
		}
	}
}
func TestCheckoutUsesFixedOfferAndIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(raw))
		if form.Get("mode") != "subscription" || form.Get("line_items[0][price]") != "price_annual" || form.Get("discounts[0][coupon]") != "coupon_founding" || form.Get("subscription_data[metadata][app]") != App || form.Get("subscription_data[metadata][user_id]") != "user" || r.Header.Get("Idempotency-Key") != "reader-checkout-lease" {
			t.Fatalf("wrong checkout: %v", form)
		}
		if strings.Contains(string(raw), "unit_amount") {
			t.Fatal("caller can set price")
		}
		fmt.Fprint(w, `{"id":"cs_test","url":"https://checkout.stripe.com/c/pay/test"}`)
	}))
	defer server.Close()
	c := New("fake", "price_annual", "coupon_founding")
	c.BaseURL = server.URL
	if _, err := c.Checkout(context.Background(), "user", "user@example.invalid", "lease", "https://potpuri.cc", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}
