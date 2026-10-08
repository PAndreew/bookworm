# Guest reading onboarding

Above the landing-page footer, **Try it now** links to `/try/?book=SLUG` for
A Christmas Carol, Beyond Good and Evil, The Yellow Wallpaper, and
A Connecticut Yankee in King Arthur’s Court.

## Shared interfaces, not duplicate implementations

Landing tiles and the library both render `BookCard.svelte`, with the same
stable pastel cover, title, author, original page/card counts, genre tags,
review summary and five rating stars. Both grids use the same column sizing,
gap and 760px container/16px side padding. Landing tiles omit reading-history
labels and link to previews. Card metadata is included in the snapshot.

`GuestReader.svelte` is only a book chooser/loader. It passes a preview book to
**the real `Reader.svelte`**. That component owns the layout, pagination, text
size, progress, controls and gestures in both modes. Preview mode loads exactly
ten frozen cards, including the title, and disables like/highlight/share/report
actions. It never loads more cards, awards credit, saves progress or calls APIs.
Finishing the preview offers signup; normal public-domain reading remains free.

All pages use **the real `ReadingCompanion.svelte`** for Puri's position, scroll
hops, card-change hops, three-frame blink and reduced-motion behavior. Landing
`experience="facts"` uses a non-clickable image and five random turquoise trivia
bubbles. Preview `experience="demo"` uses a small `PuriCoach.svelte` bubble with
an exact suggested word, correction for other selections, and a prepared answer.
Only an exact case-insensitive selection from the book text enables Explain.
The bubble is anchored to Puri with a pointing tail and waits 550ms after the
last position/card/hop change (longer than the 450ms hop and 280ms transition).
Movement hides it immediately; pending hints are cancelled on rapid navigation
or unmount. Reduced-motion mode uses no settling delay. Neither mode opens the
paid AI dialog or calls `/assistant`. Full readers retain
the normal paid explanation/translation dialog.

Facts appear after a random 3.5–6 seconds, remain for ten seconds, and return
after a random 20–35-second pause. No immediate repeat. Reduced motion disables
hops and blink; timers and listeners are removed on unmount.

`ops/build-guest-samples.py` refreshes `web/src/lib/guest-books.json` from public
catalogue/card endpoints, validating all forty cards and their curated words.
Snapshots do not change automatically when the live card layout changes.
Unsupported selections never cause fallback AI requests.

## Fictional demo friend

`ops/seed-demo-friend.sql` transactionally seeds **Demo reader (fictional)**,
handle `puri_demo_reader`, and an accepted friendship with the requested
`test@test.com` account. Four bookmarks populate the friend's activity feed.
No fabricated read credits, streaks or community ratings are added.

The demo has a reserved `.invalid` email and a password hash generated from a
random discarded secret, so no known login credential. The script requires the
target account and four books, respects blocks, refuses identity conflicts,
and never resets existing bookmarks on reruns.

Tests: `web/tests/guest-onboarding.browser.cjs` compares actual library/preview
geometry at 320/390/1280px, checks exact-word coaching, disabled actions, the
limit and zero preview API calls. `web/tests/glim.browser.cjs` checks landing
hops, non-clickability, blink frames and reduced motion.
`web/tests/puri-bubble.browser.cjs` checks settling, nearby bubble geometry,
rapid-navigation hint cancellation and reduced motion.
