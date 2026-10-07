# Design foundation

The reader borrows Datamat's design language, not its decorative effects:

- Self-hosted Geist body text and Quicksand headings/wordmark.
- White and neutral gray surfaces, dark ink, blue (`#2563eb`) actions.
- Single pastel book cards with black text, no nested covers or monograms.
  Colors use a stable slug-derived hue at 65% saturation and 91% lightness.
- 12px card corners, subtle borders/shadows, generous card padding and grid gaps.
- Centered landing page with a small wordmark below the rose logo.
- Individual benefit words accented in blue (free), purple (streak), and green (Highlight).
- No gradient backgrounds or gradient text. Reading interactions remain unchanged.

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
