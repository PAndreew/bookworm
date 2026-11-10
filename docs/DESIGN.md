# Design foundation

The reader borrows Datamat's design language, not its decorative effects:

- Self-hosted Geist body text and Fraunces headings/wordmark: a friendly serif.
- White and neutral gray surfaces, dark ink, blue (`#2563eb`) actions.
- Single pastel book cards with black text, no nested covers or monograms.
  Colors use a stable slug-derived hue at 65% saturation and 91% lightness.
- 12px card corners, subtle borders/shadows, generous card padding and grid gaps.
- Book titles begin at the top, genre hashtags follow the metadata, personal
  1–5-star controls sit below, and locked Plus links sit at the bottom right.
- Server-side search and genre/access filters support the expanded catalogue;
  explicit loading adds 24 metadata-only cards at a time.
- Puri, the circular green mascot (`web/public/glim.svg`), is the brand icon.
  The centered landing page places a small wordmark below Puri; elsewhere
  the wordmark sits beside Puri.
- “Reading tailored to our times. Read awesome/classic/serious books as a social
  feed.” The adjective gently fades between words without changing its width.
  Reduced-motion preference disables rotation; screen readers receive static copy.
- An “or” separates the two landing actions.
- Individual benefit words accented in blue (free), purple (streak), and green (Highlight).
- No gradient backgrounds or gradient text.
- The reader T menu controls both text size and card font. Choices: Geist,
  Fraunces, calligraphic Fondamento, brutalist Space Mono and gothic
  UnifrakturCook. Google Fonts are self-hosted with SIL OFL licenses. Selection
  persists locally and refits continuation cards without changing source text
  or annotation anchors; decorative styles include a readability reminder.
- Reader text spans the available width above horizontal bottom actions. Balanced
  text cards fit common phone screens at default type size; separate title/chapter
  cards are unmetered. Original annotations and positions retain their anchors.
- Sunny is a small 32px yellow circle with black oval eyes and a semicircle smile,
  sitting after the last text character and hopping vertically as cards/paragraphs
  change. There is no horizontal mascot track. Reduced motion disables movement.
- Sunny AI requires Plus/Pro, explicit consent, and server-side membership checks.
  Pro includes 1,000 licensed text cards/calendar month; public books stay unlimited.
  Detailed behavior and regression coverage: [READER-CARDS.md](READER-CARDS.md).

Tokens live in `web/src/styles/global.css`. Existing `--orange`, `--purple`,
and `--grad-*` component tokens are compatibility aliases for solid colors,
not gradients. New styles should use the semantic `--accent`, `--bg`, and
`--card-*` tokens instead. Font licenses ship alongside the files in
`web/public/fonts/`.

The service-worker cache version was bumped with the restyle so installed apps
receive the updated shell. PWA icon backgrounds and theme metadata are white.

Verified in Chromium at 390px and 1280px viewport widths: landing, library,
login, and reader at page index 100. Checked fonts, horizontal overflow, computed
gradient backgrounds, landing centering and removed benefit borders; no runtime
JavaScript errors. Resume pagination still starts at the requested index.
