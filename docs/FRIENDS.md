# Friends and reader profiles

The Friends page lists friends by descending **books in common**, with equal
scores breaking by handle. Each row shows a deterministic GitHub-style mirrored
5×5 identicon, nickname beside handle, most recent visible book title underneath,
and common-book count. Reading lists appear only on the friend's profile. The same local SVG component
uses the immutable account ID on You and reader profiles; no avatar service,
external image request, uploaded picture or stored avatar record is needed.

Common books means distinct canonical books both readers have started (saved
reading progress) or earned real reading credits for, across editions and both
page/card ledgers. Starting a book does not grant streak or page credits.
Inherited card credits alone and bookmarks do not count. The viewer can
compare their own private books, but the friend's private books/profile never
contribute to common-book or recent-reading counts.

Recent pages means earned page/card credits during the reader's **today and
previous 29 local calendar dates**, consistent with the app's per-day ledgers.
Repeat reads on different days count; same-card same-day duplicates cannot.
Card history has no precise timestamp, so the UI must not imply an exact rolling
720-hour window. Streaks expire in each user's timezone rather than continuing
to display stale database values. None of these queries grant reading credit.

Click a friend to open `/app/u/HANDLE`. Nginx serves the static profile shell for
these dynamic paths; the authenticated `/v1/users/HANDLE` endpoint remains the
privacy authority. Detailed current/longest streak and finished-book totals are
plain text, not blue cards, followed by the existing shared reading list and
progress. Blocked profiles remain unavailable.

Tests: `api/internal/api/friends_integration_test.go` exercises common-book
ordering, real vs navigation/inherited credits, local-date boundaries, stale
streaks and private books/profiles in disposable PostgreSQL.
`web/tests/friends.browser.cjs` covers compact rows, latest titles, common counts,
stable/matching avatars, profile navigation, unboxed stats and no horizontal
overflow at 320/390/1280px.
