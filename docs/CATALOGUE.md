# Genres, ratings and the expanded shelf

## Data and API

- `books.genres` is a non-null text array, capped at five tags by PostgreSQL.
  Ingest accepts unique lowercase hyphenated names (`historical-fiction`).
- `book_ratings` stores one personal integer rating (1–5) per user/book.
  Updating a rating replaces that user's previous choice, not anyone else's.
- `GET /v1/me/ratings` returns the signed-in user's slug-to-stars map.
- `PUT /v1/books/{slug}/rating` accepts `{"stars": 1}` through `{"stars": 5}`.
  Authentication is mandatory; invalid stars return 400, unknown books 404.
- Public book responses include `genres`, never book text. The shelf requests
  24 metadata records at a time, followed by explicit **Load more books**.
- `GET /v1/books` accepts `q`, `genre`, `license`, `limit` (maximum 100) and
  `offset`. Responses include a nullable `next_offset`. Search is literal,
  case-insensitive title/author substring matching; filters run in PostgreSQL.
- `GET /v1/genres` returns the independent full genre list, loaded once.
- Migration 0004 adds stable title/ID ordering, trigram title/author search,
  and GIN genre-array indexes. At small catalogue sizes PostgreSQL may choose
  a cheaper sequential scan. No catalogue query touches page content.
- Search is debounced; obsolete filter requests are cancelled and stale results
  ignored. Reader text remains separately fetched in bounded page windows.

Ratings are personal, not community averages. They are never included in public
book responses or cached by the service worker. The shelf supports rating,
search, access filters and genre filters; hashtags show each book's genres.

## Public-domain expansion

`api/cmd/ingest/catalogue.csv` curates 127 additional Gutenberg sources across
fiction, poetry, drama, philosophy, science, history, biography, travel and more.
Run from `/opt/bookworm`:

```sh
docker compose -f compose.production.yml exec api ingest -expand
```

The importer downloads sequentially with a pause between publications, verifies
Gutenberg text markers and source title metadata, strips boilerplate, and publishes
immutable pages. It skips already-published source IDs. Retry is safe: it never
replaces existing editions or resets annotations/progress. A run fails if fewer
than 100 curated expansion sources are available.

The initial source audit also retained seven additional genuine Gutenberg works
with corrected titles, authors, genres and (for the Spanish Don Quijote) language.
Migration 0003 fixes metadata only; its source texts/page anchors stay intact.
The deployed shelf contains 134 more public-domain entries than the starter shelf.
Some sources are alternative editions of the same work.

Project Gutenberg's public-domain determinations apply to the United States;
readers must check copyright law in their own jurisdiction. Source references
(`gutenberg:<id>`) remain attached to every imported book for provenance.

## Verification

- `go test ./...` covers the curated manifest and genre validation.
- Run database/HTTP regression tests with a **disposable** PostgreSQL database:
  `TEST_DATABASE_URL=postgres://... go test ./...`.
  Tests cover migrations, database limits, all five rating values, updates,
  authentication, user isolation, invalid requests, and immutable page anchors.
- Chromium checks at 390px/1280px verify font/layout, hashtags, all catalogue
  pagination, server-side filters, and ratings surviving reload and logout/login.

A full production SQL backup was taken under `/opt/backups/` before migrations.
