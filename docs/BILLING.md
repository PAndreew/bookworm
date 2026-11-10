# AI-only founding membership

The only public paid offer is **Plus: €20/year, 100 new AI requests per calendar
month, limited to 50 founding subscriptions**. Regular monthly Plus/Pro plans are
not offered. Existing complimentary Pro accounts retain their 1,000-request limit;
ending their access on You does not initiate or cancel a Stripe charge.

The library requests only `public_domain` books and has no Plus filter. Licensing
records and legacy entitlements are preserved, not deleted. Plus pays for Puri's
AI explanations and translations, not book access.

## Stripe

`ops/setup-reader-stripe.py` runs as root on the VPS and reuses the old Potpuri's
Stripe account and live €24/year price. The recurring €4 founding coupon makes
the actual charge **€20/year**. Setup requests product scoping; checkout always
fixes the verified annual price server-side (some Stripe responses omit scope).
The coupon lasts forever while
the subscription continues, and has **max_redemptions=50**. Stripe enforces this
cap atomically even if concurrent checkouts or delayed webhooks race. No checkout,
subscription or charge is created by setup. The product is renamed Potpuri Plus.

The app also serialises fifty temporary 35-minute checkout reservations in
PostgreSQL. Checkout retries reuse the same lease/idempotency key. A successful
founding seat is never recycled into a new founding place after cancellation.
Stripe's redemption cap remains authoritative; temporary holds may make the
UI show fewer available places until abandoned sessions expire.

Checkout requires an authenticated, completed-2FA session when applicable. It
uses the configured price/coupon, server-owned user/lease metadata and a fixed
return origin, not a client-provided amount or redirect. Only card payment methods
are enabled. Renewal cancellation sets `cancel_at_period_end`; paid access remains
until the annual period ends. No subscription is granted from a browser return URL.

A separate `/v1/billing/webhook` endpoint verifies the exact raw body, HMAC
signature and five-minute timestamp window. After acquiring a per-subscription
lock, it fetches the authoritative Stripe subscription, validates app/price/user
metadata, then commits event receipt, seat, subscription state and tier together.
Duplicate and out-of-order events cannot regrant expired access. Cancellation
uses the same lock. Subscription expiry is checked by profile/AI access paths
as a fallback if a Stripe event is delayed. Pool-consuming AI and billing work is
bounded to avoid exhausting PostgreSQL while waiting on external services.

Stripe key/webhook secrets are encrypted with the host systemd credential key in
`/opt/potpuri-reader-vault`, decrypted into volatile storage at boot, and mounted
read-only into the API. Only price/coupon identifiers appear in the production
`.env`. Never print credentials or commit them.

## AI translations and quotas

Gemini remains `google/gemini-3.8-flash` with `data_collection: deny` and `zdr: true`.
Mistral Large 4 was evaluated but has no eligible zero-retention route, so it is
not used. Never silently relax provider privacy requirements.

One new explanation, phrase translation or card-translation attempt consumes one
monthly request. Failures also count, as the provider may already have incurred
cost. Requests are spaced at least five seconds; global daily capacity is 1,000.
Cached card translations do not consume another request, even for another paid
reader. UTC monthly allowances do not roll over.

Full-card translations support French, Spanish and German. The server loads the
canonical published text card, not arbitrary client text, and asks for similar
length while preserving meaning, tone and paragraphs. A per-card/language lock
prevents duplicate generation. Stored output is keyed by card ID, language and
source hash, so rechunking or text changes cannot serve a stale translation.
The reader offers Original and the languages cached for that card, with an AI
translation badge at top-left. Existing large-text continuation logic fits longer
translations without shrinking the user's chosen type size. Share/highlight
controls are disabled in translated views because source annotation offsets do
not correspond to translated words.

## Verification

- `api/internal/billing/stripe_test.go`: fixed checkout identity/price/coupon,
  idempotency and signature/timestamp/body tampering.
- `api/internal/api/membership_integration_test.go`: fake Stripe/provider plus
  disposable PostgreSQL; paid webhook activation, retries, period-end cancellation,
  expiry, first-fifty cap, monthly allowance and shared cache reuse.
- `web/tests/ai-membership.browser.cjs`: subscribe/unsubscribe, spacing, hidden
  licensed titles/buttons, full-card translation, saved-language switching and badge.

Live verification must not create a subscription or charge. Read price/coupon/
webhook configuration and offer availability; payment flows are exercised with
fakes. Stripe customer/subscription IDs are not returned to the browser.
