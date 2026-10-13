# Reading achievements (not deployed)

`main` uses the live v4 reader-card layout. The pySBD experiment remains on
`preserve/pysbd-sentence-balancing`; this feature does not include it.

The You page lists three milestones, including locked placeholders:

- **First card:** one genuine text-card read.
- **100 cards:** 100 distinct genuinely read text-card IDs. Title cards,
  rereads of the same card, and inherited layout-migration coverage do not
  increment the count. Archived original reads still count.
- **A whole book:** every active text card in any one edition has read
  coverage. Title/chapter cards do not count; inherited coverage preserves
  progress through layout migrations. Once earned, an achievement stays earned.

The signed-in reader has a **Read cards** control. Its reading-completion
panel lists the exact missing card numbers with buttons to jump back to them.
`GET /v1/books/{slug}/position` includes `completion` (`total_cards`,
`read_cards`, zero-based `missing_card_indices`). Coverage comes from the
account-wide database ledger, not browser storage; headings and archived cards
are excluded and inherited reads count. Opening/refreshing the panel fetches
current coverage, including reads from another device. The list renders 50
missing cards at a time. Opening a missing card does not itself award credit:
the normal visible text-card dwell still applies.

An open profile refreshes achievements and activity when its tab becomes visible
or its window regains focus. This is refresh-on-return, not live push sync.

On first request the API computes qualifying awards and records them in
`achievements`; old card reads have a local date, so their award timestamp is
backfilled at noon on that date. No text, edition, or streak record is changed.

Each award has a circular placeholder SVG in `web/public/achievements/`. Replace
those three files with final artwork later (retain paths, round cropping and
accessible accompanying text). Locked achievements cannot be shared. Unlocked
awards have a Share action using native share or copy-link fallback. Public
`/a/{handle}/{award}` links verify the award server-side and disclose only
handle, display name, award and date; they do not disclose a book title or
private reading list. A missing/locked award returns 404.

Run `go test -p 1 -parallel 1 ./...` against a disposable `TEST_DATABASE_URL`;
`web/tests/achievements.browser.cjs` checks the You page, sharing and public
shell. Deployment requires backing up the database before migration
`0010_achievements.sql` and the Nginx `/a/` fallback; do not deploy this work
until requested.
