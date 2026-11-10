# BYOK settings and reader fixes

## Changes

- Setup moved to **You → Your AI**. Save on this device persists the selected provider/key/model in account-scoped browser local storage. Provider credentials are never stored in cookies or sent to the VPS. Forget key removes the credential; one-time terms acceptance is retained. This is not encrypted storage and remains accessible to same-origin scripts/extensions or browser-profile access.
- Puri no longer repeats credential setup or consent. It shows the selected model, allowance and a settings link. Plus/Pro and the separate 50/month BYOK allowance remain unchanged.
- Models are fetched directly from provider catalogs, not hardcoded: OpenAI `/v1/models`, Anthropic `/v1/models` with cursor pagination, OpenRouter `/api/v1/models`. Current text-model candidates are sorted newest first; users choose explicitly. OpenAI inference now uses `/v1/responses` with `store:false`. Catalog access does not consume the app's inference allowance.
- Puri uses a top-layer modal positioned within the visual viewport and safe-area insets. The close control stays outside the scrolling content. Outside taps and Escape dismiss it and cancel outstanding inference.
- Highlight selection is retained across native focus/selection-menu clearing. Even one-character selections can be saved. Successful highlights visibly mark the original text and reload from account-private, book-scoped storage. Source-page rune offsets, joined-card mappings and continuation-screen offsets are preserved; no book re-chunking or annotation migration.

## Diagnosis

The old dialog used its anchor's raw Y coordinate without the iPhone safe area, had no outside-dismiss handler and scrolled its close button with a tall settings panel. The old highlighter read only the live DOM selection after the action gained focus; native selection clearing could erase it before save. Successful annotations also had no reader rendering/restoration path. A failing browser regression reproduced all of these before fixes.

## Verification

- Chromium and WebKit browser regressions: 320/390/1280px plus short-height/safe-area cases; closing, outside dismissal, selection retention, visible/reloaded highlights.
- Both engines: live-model request formats/filtering/Anthropic pagination, explicit model choice, local-storage persistence, one-time terms, account isolation, key deletion, all three inference formats, quota/failure gates and private translation cache.
- Pure Unicode/joined-source/continuation annotation mapping tests; existing paid AI, reader-font, SEO and service-worker regressions.
- Disposable-PostgreSQL API tests include canonical server-side quotes, book/account scope, private caching and existing BYOK concurrency limits.
- One pre-existing email test fixture was made deterministic: base64url '-' was occasionally generating an invalid handle.
- Real OpenRouter public catalog fetched successfully. Authenticated OpenAI/Anthropic catalog and inference paths are tested with mocked provider responses, not customer keys. No billable inference calls made. WebKit/mobile viewport checks are not physical iPhone testing.

No schema migration, database-volume removal, model download, domain change or Vercel migration. Service worker v16 refreshes the UI. Unrelated marketing drafts and bot files remain uncommitted.
