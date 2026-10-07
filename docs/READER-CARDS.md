# Reader cards and paid usage

## Non-destructive pagination

The original `pages` rows, edition IDs, content hashes, highlights, shared quote
links, likes and original-page bookmarks stay immutable. Migration 0006 adds a
separate `reader_cards` presentation layer; it does **not** re-ingest source text,
replace editions, or wipe reading data.

Each published edition gets a title card, chapter cards when the original chapter
metadata changes, and text cards balanced within their original source page.
Text cards target roughly 300–360 Unicode characters, with a 400-character hard
cap and a conservative 13-line phone budget. Prefer nearby sentence endings, then
whitespace; even enormous words remain bounded. Paragraph-heavy front matter,
poetry and small original chapter tails can be shorter. Real Pride and Prejudice
front matter showed why a character cap alone cannot guarantee screen fit. Offsets are rune positions
into the exact original text, not newly normalized prose.

A card references its original page and a start/end range. Likes remain anchored
to that original passage and update all visible sibling cards together. Highlight
selection adds the card's starting rune offset to the local selection; shared
quotes retain their original anchors. No annotation migration or deletion occurs.

Prepare layouts after deployment/ingestion:

```sh
cd /opt/bookworm
docker compose -f compose.production.yml exec -T api cards
```

The operation is idempotent and protected by per-edition advisory locks. Existing
version-2 card layouts remain stable. The one-time v1→v2 line-budget refinement
preserves each existing card ID for its first piece, adds only necessary pieces,
and remaps exact-card bookmarks atomically. Previously read coverage carries to
new pieces without additional quota or artificial historical streak counts;
actual later visits become normal reads. New editions also build lazily on their
first card request. Catalogue calls remain bounded metadata-only requests and
read the cached `reader_card_count` field; they never build layouts or fetch text.

`GET /v1/books/{slug}/cards?card_from=0&limit=12` fetches at most 30 cards, ordered
by persistent card index. `source_from=N` maps an old original-page bookmark into
the corresponding first presentation card. Existing `?from=N` reader links use
that mapping. New reading stores both original-page and exact-card bookmarks;
`?card=N` and resume buttons restore the exact new position. Source-page API
responses remain available for old annotations/integrations.

## Layout and Sunny

Text uses the full available width above a horizontal bottom bar. Cards are sized
for common phone viewports at the default responsive 16–18px type size. Extremely
small screens or explicitly enlarged accessibility text may still scroll inside
the card; text is never clipped or silently discarded to force a fit.

Sunny is 32px with a 44px effective touch target. An empty inline marker reserves
space after the final text character; Sunny follows that exact marker, hopping
vertically on scroll rather than moving along a horizontal bottom track. Landing
paragraph endings supply similar markers. The mascot follows font/viewport/layout
changes and the current card, without obstructing text or bottom controls.
Reduced motion disables hops/sliding. The native dialog remains keyboard accessible.

## Membership and counting

- **Every tier:** public-domain text cards are unlimited.
- **Free:** no AI explanations/translations and no licensed reading.
- **Plus:** Sunny AI and 30 licensed text cards per local calendar day.
- **Pro:** Sunny AI and **1,000 licensed text cards per local calendar month**.
- Title/chapter cards never consume allowance and do not count toward streaks.
- Re-reading the same licensed card within the tier's quota period is free.
- Pro resets at midnight on the first of the next month in the profile timezone.
- Original historical metered reads remain included in usage; nothing is erased.

`POST /v1/card-reads` accepts `{"card_id":"uuid"}` after actual text-card dwell.
Quota checking, read recording and bookmark advancement are one transaction,
serialized by a user-row lock so concurrent tabs cannot exceed the allowance.
Legacy `/v1/reads` remains compatible for public-domain source pages; licensed
legacy reads ask the client to reload into the current card reader.

`GET /v1/me` reports `quota.limit`, `used`, `period` (day/month), `resets_at` and an
optional exact `last_read.card_idx` alongside the original `page_idx`.

AI membership is checked **server-side before any provider request**, not merely
hidden in the UI. Existing consent, phrase-size/output bounds, 30 AI attempts per
paid reader/day, 200 shared attempts/day and five-second cooldown still apply.
The OpenRouter key remains in the encrypted VPS credential vault; see ASSISTANT.md.

`test@test.com` is manually granted Pro on this deployment. No other account tier
is changed. Handles already have a case-insensitive `citext UNIQUE` constraint;
a regression test confirms duplicate case variants are rejected.

## Verification

Disposable PostgreSQL tests cover original page preservation, rune ranges,
legacy/exact resume, unmetered headings/streaks, free/paid access, monthly cap,
free rereads, rollover, public reading at the cap and unique handles. Optional
`web/tests/companion.browser.cjs` mocks all APIs without spending credits and checks
320×568, 390×600 and desktop layout, paragraph-end Sunny, bottom controls, default
text overflow, paid AI, consent, annotation offset mapping and reduced motion.
