# Guest reading onboarding

Above the landing-page footer, **Try it now** links to `/try/?book=SLUG` for
one book: A Christmas Carol. Old preview query parameters also open this demo.

## Shared interfaces, not duplicate implementations

Landing tiles and the library both render `BookCard.svelte`, with the same
stable pastel cover, title, author, card counts, genre tags,
review summary and five rating stars. The landing demo tile uses the same
responsive grid column sizing as library cards, not a wider standalone tile. Landing tiles omit reading-history
labels and link to previews. Card metadata is included in the snapshot.

`GuestReader.svelte` loads the single demo book. It passes a preview book to
**the real `Reader.svelte`**. That component owns the layout, pagination, text
size, progress, controls and gestures in both modes. Preview mode loads exactly
three frozen cards: title, explanation, and translation, and disables like/highlight/share/report
actions. It never loads more cards, awards credit, saves progress or calls APIs.
A Continue reading link opens `/read/gutenberg-46`, the unrestricted full book, without signup. The end screen offers the same continuation, not a ten-card cutoff. Signup remains optional.

All pages use **the real `ReadingCompanion.svelte`** for Puri's position, scroll
hops, card-change hops, three-frame blink and reduced-motion behavior. Landing
`experience="facts"` retains the non-clickable mascot and scroll behavior, but
has no trivia bubbles or timers. The headline is “Read timeless classics, bit by
bit.” with a locally hosted, OFL-licensed Pacifico font for “timeless classics.”
Preview `experience="demo"` uses a small `PuriCoach.svelte` bubble with
an exact suggested word, correction for other selections, and a prepared answer.
Only an exact case-insensitive selection from the book text enables Explain.
The bubble is anchored to Puri with a pointing tail and waits 550ms after the
last position/card/hop change (longer than the 450ms hop and 280ms transition).
Movement hides it immediately; pending hints are cancelled on rapid navigation
or unmount. Reduced-motion mode uses no settling delay. Neither mode opens the
paid AI dialog or calls `/assistant`. Full readers retain
the normal explanation/translation dialog with membership AI or BYOK, including anonymous BYOK. Signed-out visitors see only Library in the tab bar; Friends, Quotes and You appear after authentication.

On the landing page, scroll events immediately hide Puri.
After 180ms without scrolling, Puri measures the final marker and relocates
immediately with one hop—no CSS glide or hops during momentum scrolling.
Resize/font observers cannot bypass a pending scroll pause; timers are cancelled
on unmount. The full reader's movement is unchanged.

The title card (index 0) has no coaching bubble. Card 2 (index 1), the first
non-title card, teaches word explanation. Card 3 (index 2), the second non-title
card, teaches full-card translation. Click Puri, then Translate card, to show a prepared French version
in the same reader with its AI translation badge and Original/French dropdown.
This uses the actual translation state/rendering, not a separate demo interface,
and makes no API calls. `guest-translations.json` stores the exact source beside
the translation; a changed snapshot disables mismatched demo translations rather
than translating the wrong text. Continue reading opens the live full book. Reduced motion
disables hops and blink; movement timers and listeners are removed on unmount.

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
geometry at 320/390/1280px, checks exact-word coaching, prepared full-card
translation/Original switching, disabled actions, full-book continuation and zero preview API calls. `web/tests/glim.browser.cjs` checks landing
hops, non-clickability, blink frames and reduced motion.
`web/tests/puri-bubble.browser.cjs` checks settling, nearby bubble geometry,
rapid-navigation hint cancellation and reduced motion.
`web/tests/puri-landing-scroll.browser.cjs` verifies scroll-burst stability,
exact final placement, one delayed hop, no glide, and reduced motion.
