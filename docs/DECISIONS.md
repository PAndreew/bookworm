# Bookworm — Decision Record

Current reader-card refinements and quota semantics: [READER-CARDS.md](READER-CARDS.md).

## Locked by the user
- **Klarna** = slick subscription UX only. Stripe subscriptions, no ledger, no BNPL.
- **Paid catalog** = seeded demo/licensed titles, publisher uploads filling it over time.
- **Age** = 13+ gate, declared birthdate at signup. No COPPA/GDPR-Art.8 consent flow.
- **Frontend islands** = Svelte.

## Decided by me (defaults — say the word and I'll change any of them)

### The page unit — the most load-bearing decision
An immutable **source page** is a paragraph-aware chunk targeting ~900 characters. It never splits a
paragraph unless that paragraph alone exceeds 1400 chars, in which case it splits on
sentence boundaries.

The source page remains the anchor for every existing like and highlight. The
separate v5 reader-card layer balances pySBD sentences across whole chapters,
targeting ~360 characters with precise multi-source offset maps. Four sentences
is a preference; whole-sentence outliers and variable whitespace are accepted. Title/chapter cards are separate
and unmetered; source pages themselves are no longer the UI/billing unit. **Chunks are immutable once
an edition is published.** Re-chunking would orphan every like and highlight ever made.
So:
- `book_editions` carries a `chunker_version`. Changing the algorithm creates a *new*
  edition; the old one keeps serving existing readers and their annotations.
- Every page stores a `content_sha256`, so ingest is idempotent and verifiable.

### Reader
- One card per screen, turned with buttons, keyboard or swipes. No scrolling.
- Text is top-aligned; whitespace below absorbs card-length differences. Font
  controls offer 100%/110%. Overflow prefers persisted sentence endings; only a
  sentence larger than the actual viewport may continue at a word boundary.
- A muted 13px corner flag opens private text/layout reports; no large report button.
- pySBD 0.3.4 segments unchanged English source text during layout construction.
  Go balances its sentence spans; normal reads need no Python execution.
- A discovery "For You" feed across books is **v2**. The schema supports it already
  (`page_likes` is global, not per-reader), but the UI is not built.

### Social
- Friends = **mutual accept**. Fits "friends + streaks" better than asymmetric follow.
- Streak = **3+ text cards read in a day**, day boundary in the **user's own timezone**
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
| Licensed text cards  | 0         | 30/day    | 1,000/month |
| Sunny AI              | no        | yes       | yes       |
| Price                | €0        | €4.99/mo  | €9.99/mo  |

Plus resets at local midnight; Pro resets on the first of the next calendar month
in the user's timezone. Public-domain books stay unlimited. Title/chapter cards do
not consume allowance or extend streaks. Re-reading within a quota period is free.
Sunny sits at paragraph/card endings, hops vertically, and requires Plus/Pro for AI.
No rollover. At the cap you get an
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
