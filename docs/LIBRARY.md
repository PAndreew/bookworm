# Personal library

The library no longer has a separate Resume Reading button. By default, published
books the reader opened within the past rolling calendar month lead the shelf,
ordered newest first. Other titles follow alphabetically. The latest recent book
has a green border. Explicit review/length sorting overrides this default order.

**All / My books** sits beside the sort dropdown and its up/down direction
button. My books is an independent toggle, combined with genre and search filters. It includes all opened books, even those older than a month.
Signed-out users are directed to sign in. Turning it off restores the full shelf.

`GET /v1/books` accepts `sort=recent|rating|reviews|pages`,
`direction=asc|desc` (default descending), and `mine=true`. The dropdown offers
Recently read, Rating, Reviews, and Page count; the adjacent arrow toggles the
ordering for every attribute and retains the direction when switching attributes.
Rating uses community average stars, breaking ties by review count in the same
direction; Reviews uses review count. Unrated books count as zero. Page count
uses original `page_count`. Recent ordering keeps unopened/out-of-window books
last in either direction (My books includes older opened books). Legacy
`shortest`/`longest` sort URLs remain supported.
Filtering and ordering happen in PostgreSQL before pagination. Personal bookmarks
are joined only for the authenticated viewer. Responses are private/no-store and
the service worker never caches the catalogue or position endpoints.

Each opened edition already has a `reading_progress` row keyed by user/edition.
`GET /v1/books/:slug/position` returns that reader's saved position after ensuring
the current card layout; `POST` saves a validated card in that book. Title/chapter
cards and navigation are saved too, without charging quota or earning streaks.
Only eligible readers can save licensed-book positions. Existing card-layout
refinement preserves/remaps these bookmarks.

Opening a book without an explicit `card`/`from` link restores its saved position.
Explicit deep links take precedence. Navigation writes are serialized/coalesced
so a slower older request cannot overwrite a newer position in the same reader.
After a dwell-credit request completes, the client re-saves its current position
in case that read finished after navigation. Continuation-screen offsets inside
a card are not separately saved.

Tests: `api/internal/api/library_integration_test.go` (disposable PostgreSQL),
`web/tests/library.browser.cjs`, and `web/tests/sw.cjs`.
