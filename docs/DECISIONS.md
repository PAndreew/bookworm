# Bookworm — Decision Record (v1)

## Locked by the user
- **Klarna** = slick subscription UX only. Stripe subscriptions, no ledger, no BNPL.
- **Paid catalog** = seeded demo/licensed titles, publisher uploads filling it over time.
- **Age** = 13+ gate, declared birthdate at signup. No COPPA/GDPR-Art.8 consent flow.
- **Frontend islands** = Svelte.

## Decided by me (defaults — say the word and I'll change any of them)

### The page unit — the most load-bearing decision
A "page" is a **paragraph-aware chunk targeting ~900 characters**. It never splits a
paragraph unless that paragraph alone exceeds 1400 chars, in which case it splits on
sentence boundaries.

Why this matters more than it looks: the page is simultaneously the UI card, the
billing unit, and the anchor for every like and highlight. **Chunks are immutable once
an edition is published.** Re-chunking would orphan every like and highlight ever made.
So:
- `book_editions` carries a `chunker_version`. Changing the algorithm creates a *new*
  edition; the old one keeps serving existing readers and their annotations.
- Every page stores a `content_sha256`, so ingest is idempotent and verifiable.

### Reader
- Vertical snap-swipe, one page per screen — the TikTok feel.
- Font scale is user-settable. If a page overflows at a large scale that single card
  scrolls internally rather than being re-chunked. Accessibility beats pixel purity.
- A discovery "For You" feed across books is **v2**. The schema supports it already
  (`page_likes` is global, not per-reader), but the UI is not built.

### Social
- Friends = **mutual accept**. Fits "friends + streaks" better than asymmetric follow.
- Streak = **3+ pages read in a day**, day boundary in the **user's own timezone**
  (stored per user as an IANA name). Freezes are a Plus perk: the column exists,
  the UI is v2.
- Account visibility: `public` | `friends` | `private`, default **friends**.
  Per-book override via `reading_progress.is_private`.
- Share = a quote page with Web Share API in v1. The server-rendered PNG card is left
  as a clean seam at `/api/quotes/{id}/card.png` — that's the growth loop, but it needs
  a headless renderer and shouldn't block v1.

### Tiers
|                      | Free      | Plus      | Pro       |
|----------------------|-----------|-----------|-----------|
| Public-domain books  | unlimited | unlimited | unlimited |
| Licensed pages/day   | 0         | 30        | 150       |
| Price                | €0        | €4.99/mo  | €9.99/mo  |

Resets at midnight in the user's timezone. No rollover. At the cap you get an
interstitial ("resumes in 4h / upgrade"), never a mid-sentence cut — the page you are
on always completes.

### Stack
- Go 1.27, stdlib `net/http` with `chi`, `pgx/v5`, SQL migrations embedded via `go:embed`.
- Astro static/prerendered + Svelte islands, talking to the Go API directly.
  No Node server in production.
- Postgres 17. Sessions are opaque 32-byte tokens, SHA-256 hashed at rest, in HttpOnly
  SameSite=Lax cookies. 2FA is TOTP (RFC 6238) with single-use recovery codes.
- Argon2id for passwords (not bcrypt — no 72-byte truncation, memory-hard).
- Everything runs in Docker under WSL.

## Legal posture
The uploader-warranty clause is in the terms as you asked. Stated plainly in
`docs/LEGAL.md`: in the EU that clause does **not** transfer platform liability — DSA
notice-and-action and CDSM Art. 17 still bind you as the host. So v1 includes a takedown
endpoint and a repeat-infringer strike counter. They're cheap now and expensive to
retrofit.
