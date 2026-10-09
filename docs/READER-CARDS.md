# Reader cards and paid usage

## Compact reader

The reader has a narrow blue progress line, a top-left back caret and a top-right
text-size control, without a title navbar. Reduced side/footer padding preserves
readable type. Demo hints have a close button, dismiss after two seconds, and
can be reopened by clicking Puri. Text starts just below the corner controls,
not vertically centred. Shorter cards leave whitespace below; larger screens do
not stretch the font or paragraph spacing. Font controls offer 100% and 110%.
All four demo opening prose cards fit at 320×568.

## Non-destructive pagination

Original `pages`, edition IDs, content hashes, highlights, shared quotes, likes and
original-page bookmarks stay immutable. Reader cards are a separate presentation
layer; replacing their layouts never re-ingests or deletes original text.

Layout **v5** uses **pySBD 0.3.4**, with cleaning disabled and exact Unicode
character spans. Python runs only during layout construction, batching the
edition's chapters; regular card reads use persisted `sentence_ends` offsets.
Failures abort the layout transaction rather than silently guessing boundaries.
Standalone non-punctuated headings stay attached to the following prose.

Go balances each whole chapter using dynamic programming over those boundaries.
Target about **360 characters**, with a soft preference for four sentences;
allow five only below 360 characters / 60 words. Grouped cards stay under 480
characters / 85 words. Penalising squared length variation avoids greedy packing
that leaves a tiny final card. Short and long outliers are intentional. A single
long sentence stays intact regardless of the estimated budget: only actual
viewport measurements can require a word-boundary continuation. No ML/provider
call is involved. The current English catalogue uses pySBD's English rules.
An old source boundary inside a sentence joins with a space, fixing the artificial
blank paragraph between `Mr.` and `Bingley`.

`reader_card_parts` maps each card's rune ranges back to one or more exact source
pages. Highlight selection uses this mapping, including viewport-continuation
offsets. Cross-source highlights save the selected ranges on each original page;
cross-source sharing supplies the resulting quote links. Existing highlights and
shared quotes stay unchanged. Likes retain the first original-page anchor and
update its loaded sibling cards together.

Prepare layouts after deployment/ingestion:

```sh
cd /opt/bookworm
docker compose -f compose.production.yml exec -T api cards -slug pride-and-prejudice
# After checking one book, replace all published layouts:
docker compose -f compose.production.yml exec -T api cards
```

Replacement is idempotent, atomic and protected by per-edition advisory locks.
V1/v2/v3/v4 layouts are replaced with exact-bookmark remapping and inherited,
unmetered read coverage. Unreferenced obsolete rows are deleted. Minimal anchors
referenced by historical reads/reports keep their exact quota/streak ledger but
contain no presentation text and are never served. Source parts are retained
for accurate coverage across subsequent migrations. V5 layouts stay
stable. New editions build lazily on their first card request. Catalogue requests
are bounded metadata-only queries using cached `reader_card_count`.

`GET /v1/books/{slug}/cards?card_from=0&limit=12` returns at most 30 cards.
`source_from=N` maps old original-page bookmarks into a card containing that
passage. Legacy `?from=N` links use this mapping. `?card=N` and resume buttons
restore exact presentation-card positions; replacement remaps stored bookmarks.
Old manually copied card-index URLs may point at a different card after replacement.
Source-page APIs remain available for existing annotations/integrations.

## Layout and Sunny

Default responsive type remains **16–18px**, with at most a 10% in-app increase.
No card/feed/document scrolling is needed or allowed. Previous/next arrows,
keyboard arrows/PageUp/PageDown/Space and swipes turn cards. Real font/viewport
measurements paginate overflow at the last complete sentence that fits (with a
part counter), retaining every character and annotation offset. Only a sentence
larger than the available viewport falls back to a word break. Source cards and
demos use persisted pySBD ends; translations without stored ends use a browser
fallback. Fonts are never reduced to force fit. Paragraph-heavy front matter
uses the same continuation mechanism. A viewport continuation remains part of one
persistent card/quota unit. Only an unbroken token longer than a whole screen may
need a rune-level continuation.

Sunny is 32px with a 44px effective touch target, following an inline marker after
the final visible text character. Layout/font/card changes move its marker.
Reduced motion disables hopping/sliding. Its native dialog remains keyboard
accessible; only explicit user-selected phrases may be sent to paid AI.

## Quiet issue reporting

A muted **13px flag** sits in the lower-right card corner, with an invisible 44px
touch target, tooltip, accessible label and visible keyboard focus. It opens a
native dialog for layout, awkward-break or text issues with optional details.

`POST /v1/cards/{id}/reports` requires completed sign-in, validates published-book
access, UUID/category and a 1,000-character note limit. Reports deduplicate per
reader/card/type, with an atomic **20-new-reports/day** cap. They never call AI or
modify the book automatically. Reports persist privately in `card_reports`.

Operator queue (database access only):

```sql
SELECT r.id, r.created_at, b.slug, c.idx, r.category, r.note
FROM card_reports r JOIN reader_cards c ON c.id=r.card_id
JOIN book_editions e ON e.id=c.edition_id JOIN books b ON b.id=e.book_id
ORDER BY r.created_at DESC;
```

## Membership and counting

- **Every tier:** public-domain text cards are unlimited.
- **Free:** no AI explanations/translations and no licensed reading.
- **Plus:** Sunny AI and 30 licensed text cards per local calendar day.
- **Pro:** Sunny AI and **1,000 licensed text cards per local calendar month**.
- Title/chapter cards never consume allowance or count toward streaks.
- Re-reading the same licensed card within the tier's quota period is free.
- Pro resets on the first of the next month in the profile timezone.
- Original historical metered reads remain included; nothing is erased.

`POST /v1/card-reads` accepts `{"card_id":"uuid"}` after actual text-card dwell.
Quota checking, read recording and both original/exact bookmarks are one atomic
transaction, serialized by a user-row lock. Edition locks protect reads/reports
against simultaneous layout replacement. Legacy `/v1/reads` remains compatible
for public-domain source pages; licensed legacy reads require the card reader.

`GET /v1/me` returns `quota.limit`, `used`, `period`, `resets_at` and an optional
exact `last_read.card_idx` alongside the original `page_idx`.

AI membership is enforced **server-side**. Existing consent, phrase/output bounds,
30 attempts/paid reader/day, 200 shared attempts/day and five-second cooldown
remain. The OpenRouter key stays in the encrypted VPS vault; see ASSISTANT.md.

`test@test.com` remains manually granted Pro. No other account tier is changed.
Handles retain their case-insensitive `citext UNIQUE` constraint.

## Verification

- Install the pinned segmenter first: `python3 -m venv /tmp/bookworm-pysbd &&
  /tmp/bookworm-pysbd/bin/pip install -r api/requirements-sentences.txt`.
  Export `PYSBD_PYTHON=/tmp/bookworm-pysbd/bin/python` for Go tests and demo generation.
  The production Docker image includes this runtime.
- `go test ./...`: balancing, pySBD boundaries, Unicode preservation and
  exact source-part mappings.
- `TEST_DATABASE_URL=postgres://... go test -p 1 -parallel 1 ./...` on a disposable DB: replacement,
  original page preservation, legacy/exact resume, quota/history/inherited reads,
  unmetered headings, monthly limits, report authentication including 2FA, access,
  validation, deduplication and daily cap.
- `web/tests/companion.browser.cjs`: mocked APIs/no AI spend; 320×568, 390×600 and
  desktop geometry, no scrolling, large-type continuations, tiny flag/report
  submission, paid AI/consent, original annotation offsets and reduced motion.
- Optional real-book preview: export source pages as JSON objects with `content`
  and `chapter`, set `CARD_TEST_SOURCE` and `CARD_TEST_PREVIEW`, and run
  `go test ./internal/store -run TestRealBookLayout -v`. Then run
  `web/tests/cards.browser.cjs` with that `CARD_TEST_PREVIEW`, `TEST_BASE_URL`,
  `PLAYWRIGHT_MODULE`/`PLAYWRIGHT_EXECUTABLE` and optional `SCREENSHOT_DIR`.
  It checks real front matter, prose and long-sentence continuations without
  mutating production. The v5 Pride and Prejudice preview has 1,884 text cards;
  90.7% fit the grouping budgets, with whole long sentences retained as outliers.
- `web/tests/sentence-layout.browser.cjs`: top alignment at phone/desktop sizes,
  sentence-ending screen breaks, intact continuation text, font controls, all four
  demo openings and matching prepared translations.
- Regenerate static demo cards with `GO=go python3 ops/build-guest-samples.py`;
  this uses the same pySBD/Go pipeline, not frozen legacy card boundaries.
  Update prepared translations whenever their source card changes.

Historical v3 rollout verified all **150** published editions: **298,264**
text cards, **295,347 (99.0%)** within 340 ±5%. All **129,739** original pages
retain the same aggregate ID/content checksum. Every persisted source-part range
matches its exact original text; obsolete historical anchors contain no layout
text/parts. Real 320×568, 390×600 and desktop screenshots and report persistence
were verified; temporary QA reports/accounts were removed. See
[screenshot](screenshots/rechunk-320.png).
