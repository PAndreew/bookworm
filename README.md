# Bookworm

Books rendered as a social feed. Go API, Astro + Svelte PWA, Postgres.

Every public-domain book is free and unmetered. Plus adds 30 licensed text cards/day; Pro adds 1,000/month. Both retain included paid AI. Free accounts can instead use their own OpenAI, Anthropic or OpenRouter API key for 50 app-enforced AI requests per UTC calendar month (provider charges apply).
Reading three text cards a day keeps a streak; friends
can see what you are reading if you let them; any passage can be highlighted and
shared as a quote link.

## BYOK — free, for the brave

Open a book and ask Puri, then choose **Free BYOK**. Keys go directly from your browser to your provider over HTTPS and live only in page memory; reload/navigation clears them. BYOK card translations also stay on the page, not in the shared server cache. Our API stores only a monthly reservation counter; failed/cancelled attempts count. The 50-request app allowance is not a tamper-proof spending cap. Use a dedicated provider key with a low credit limit. Plus/Pro inference remains separate with no automatic fallback.

OpenAI requires a Platform API key and API billing. ChatGPT/Codex subscription tokens and `auth.json` are **not** general API keys. Supported Codex subscription integration needs a separate local Codex app-server client, not this direct-browser API flow; see [OpenAI's authentication docs](https://developers.openai.com/codex/auth/).

## License and source

Original project code: **GNU Affero General Public License v3.0 only** ([LICENSE](LICENSE)). Third-party libraries, fonts and book content retain their own terms; see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Source: <https://github.com/PAndreew/bookworm>.

## Running it

Requires Docker. Everything else is in containers.

```bash
docker compose up -d                    # Postgres + API on :8080, migrations run at startup
docker compose exec api ingest -all     # the full local catalogue (needs internet)
```

`-all` loads nine public-domain classics from Project Gutenberg and six invented
commercial titles across three fake publishers. Without internet access, use
`-seed-local` for the offline samples plus `-commercial` for the fake catalogue,
which needs no network at all.

Then the web app:

```bash
cd web
npm install
npm run dev                   # http://localhost:4321, proxies /v1 to the API
```

Open <http://localhost:4321>, create an account, and read.

To preview the *built* site instead of the dev server — same single origin as
production, so cookies behave identically:

```bash
cd web && npm run build
python3 .scratch/serve.py     # serves web/dist, proxies /v1 to the API
```

### The seeded catalogue

**Public domain** (real texts, fetched from Project Gutenberg):

| | pages |
|---|---|
| Moby-Dick · Melville | 1536 |
| Dracula · Stoker | 1060 |
| Pride and Prejudice · Austen | 935 |
| The Adventures of Sherlock Holmes · Conan Doyle | 712 |
| The Picture of Dorian Gray · Wilde | 560 |
| Frankenstein · Shelley | 524 |
| The Great Gatsby · Fitzgerald | 349 |
| Alice's Adventures in Wonderland · Carroll | 194 |
| The Yellow Wallpaper · Gilman | 40 |

**Commercial** — three invented publishers, six invented books:

| publisher | titles | state |
|---|---|---|
| Northwind Press | *The Salt Road*, *The Glasshouse Winter* | verified |
| Meridian Books | *Quiet Machines*, *The Borrowed Hour* | verified |
| Brightfolio | *The Last Exhibit*, *Sixteen Winters* | **unverified** (pending review) |

These are original texts written for this repository, not real books. That is
deliberate: the paid tier needs content that genuinely is *not* public domain in
order to exercise the meter, and using a real licensed book to test a licensing
system would be the exact failure the product is built to avoid. Brightfolio is
left unverified on purpose, so the catalogue contains the pre-review state a real
upload would pass through.

### Loading options

```bash
ingest -all            # everything below
ingest -seed           # the public-domain shelf (needs internet)
ingest -commercial     # fake publishers + their catalogue (no network needed)
ingest -seed-local     # the two offline sample excerpts
ingest -gutenberg 1342 # one book by Gutenberg id
ingest -file book.txt -slug x -title "X" -author "Y"   # a local text file
```

> On a network with a TLS-inspecting proxy, fetches to Gutenberg may be blocked.
> Everything except `-seed` and `-gutenberg` works offline.

**Re-running is safe.** An edition at the current `chunk.Version` is never
rewritten, and the tool prints `unchanged` rather than claiming a publish. To
genuinely swap a book's text — promoting a seed excerpt to the full work, say —
pass `-replace`, which refuses if anyone has already liked or highlighted a page
of that edition.

## Layout

```
api/                      Go 1.27, stdlib net/http + chi
  cmd/server              the API
  cmd/ingest              catalogue loader
  internal/chunk          text -> immutable pages   <- the load-bearing piece
  internal/auth           argon2id, TOTP, sessions
  internal/store          data access + the tier and streak rules
  internal/api            HTTP handlers
  internal/db/migrations  embedded SQL
web/                      Astro 7 static + Svelte 5 islands
docs/DECISIONS.md         what was decided and why
docs/LEGAL.md             the honest version of the legal position
```

## The one design decision to understand

**A source page is immutable once published.** It anchors every existing like and
highlight. The separate `reader_cards` presentation layer balances smaller screen
cards without changing source text or losing annotations. Title/chapter cards are
unmetered; only text cards count. See [docs/READER-CARDS.md](docs/READER-CARDS.md).

So `chunk.Version` is pinned into each `book_editions` row. Changing the chunking
algorithm means bumping that constant, which creates a *new* edition and leaves the
old one serving its existing readers. Ingest is idempotent: re-running it with the
same version touches metadata only, never pages.

If you change the source `chunk.Split` algorithm, bump `chunk.Version`.
Presentation-only `Cards`/`FitLines` changes use a separately versioned layout with
bookmark/coverage preservation, not destructive source re-chunking. The tests enforce determinism
but cannot catch a silent re-chunk.

## Tiers

|                     | Free      | Plus      | Pro       |
|---------------------|-----------|-----------|-----------|
| Public-domain books | unlimited | unlimited | unlimited |
| Licensed text cards | 0         | 30/day    | 1,000/month |
| Sunny AI            | no        | yes       | yes       |

Allowances reset **in the user's own timezone**: Plus daily, Pro on the first of
each calendar month. Re-reading within the quota period is free; title/chapter
cards never count toward allowance or streaks.

Metering happens on `POST /v1/card-reads` when a text card has held the screen for
~1.8s, not on fetch. Metering on fetch would bill people for prefetch and for swiping back.

## Tests

```bash
cd api && go test ./...            # 60+ unit tests
cd .scratch && node e2e.mjs        # 13 browser checks against the running stack
```

The Go tests cover the parts where a bug is expensive and invisible: chunk
determinism, Argon2id (including that it does *not* truncate at 72 bytes the way
bcrypt does), TOTP against the RFC 4226 vectors, the tier rules, and streak
arithmetic across timezones and freezes.

## What is built, and what is not

**Working, verified end to end:** signup with a 13+ gate, login, optional TOTP 2FA
with single-use recovery codes, the reader feed, likes, highlights, public quote
links, page metering with the paywall interstitial, streaks, mutual-accept friends
with visibility rules, user search, the takedown endpoint, and the PWA shell.

**Deliberately not built:**
- **Payments.** The meter, limits, and paywall are live; Stripe is not wired up. The pricing page says so rather than pretending.
- **Publisher portal.** No upload UI and no review queue. The data path is real and exercised — `ingest -commercial` creates publisher accounts (one deliberately unverified) and publishes against them — but a publisher cannot yet do any of that themselves.
- **Email delivery requires configuration.** Verification, password reset, and private signup alerts use Resend; provision its secret before production deployment. See [EMAIL.md](docs/EMAIL.md).
- **Discovery feed.** The cross-book "For You" page is v2. The schema already supports it.
- **Quote card images.** `/q/<slug>` is a real page; the server-rendered PNG is a seam, not an implementation.
- **GDPR export/erasure endpoints.** `deleted_at` exists; the actual flows do not. See `docs/LEGAL.md`.

## Two things worth your attention

1. **The uploader-warranty clause is in the terms as you asked**, but in the EU it
   does not transfer platform liability — DSA notice-and-action and CDSM Art. 17
   still bind you as the host. That is why the takedown endpoint and a repeat-infringer
   counter are in v1 rather than later. `docs/LEGAL.md` has the reasoning.

2. **"Alpha" means under-13s, and the age gate is 13+.** That keeps you clear of
   COPPA entirely, but Germany sets the GDPR Art. 8 digital-consent age at **16**,
   so a 13-year-old in Germany still needs parental consent for consent-based
   processing. The design leans on *contract* as the lawful basis and ships no
   profiling and no public-by-default profiles, which is what makes that workable.
   Adding behavioural recommendations later would change the analysis.
