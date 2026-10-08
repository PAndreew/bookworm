# Guest reading onboarding

Above the landing-page footer, **Try it now** links to `/try/?book=SLUG` for
A Christmas Carol, Beyond Good and Evil, The Yellow Wallpaper, and
A Connecticut Yankee in King Arthur’s Court.

These previews import a frozen public-domain snapshot (`web/src/lib/guest-books.json`)
with exactly ten reader cards each, including the title card. They work without a
session or backend requests. Completing card ten leads to a signup prompt, not an
eleventh card. The ordinary public-domain reader remains free; this is a bounded
onboarding experience, not a new restriction on book access.

On each text card Puri names a specific word and asks the visitor to highlight it,
click Puri, then choose **Explain**. The suggested word has a subtle dotted underline.
Only an exact, case-insensitive selection from the current card enables Explain.
A different selection triggers a correction naming the intended word. Explanations
come directly from the snapshot and are labeled as prepared demo explanations.
No assistant endpoint, AI credential, usage quota or progress/streak writes are involved.

On the landing page Puri is a non-clickable 32px image, not an AI-dialog button.
A turquoise bubble appears after a random 3.5–6-second delay, remains for ten
seconds, then reappears after a random 20–35-second pause. Five prepared facts
rotate randomly without immediate repeats. Both modes blink every five seconds;
reduced motion disables the blink. Timers/listeners are removed on unmount.

`ops/build-guest-samples.py` refreshes the snapshots from public book-card endpoints,
validating all forty cards and the presence of every prepared word before writing.
The small glossary is deliberately curated: unsupported selections never cause
fallback AI requests. A future card-layout change can require glossary adjustments
before a snapshot refresh; deployed snapshots do not change automatically.

## Fictional demo friend

`ops/seed-demo-friend.sql` transactionally seeds **Demo reader (fictional)**,
handle `puri_demo_reader`, and an accepted friendship with the specifically requested
`test@test.com` account. Four book bookmarks populate the friend's activity feed.
There are no fabricated read-credit events, streaks or community ratings.

The demo uses a reserved `.invalid` email and a valid password hash generated from
a random discarded secret. It has no known login credential. The script requires
the target account and all four books, respects existing blocks, refuses identity
conflicts, and does not reset existing demo bookmarks on subsequent runs.

Test: `web/tests/guest-onboarding.browser.cjs` covers all books at 320/390/1280px,
wrong/right selections, fixed answers, ten-card completion, and zero preview API calls.
`web/tests/glim.browser.cjs` covers blink frames, size and reduced-motion behavior.
