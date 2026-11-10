# BYOK initial release — 2026-11-10

Historical record: the subsequent reader/settings update supersedes memory-only keys with explicit device-local storage, one-time terms and live model selection under You. See [BYOK.md](BYOK.md) for current behavior.

## Shipped

- Direct-browser OpenAI Platform, Anthropic and OpenRouter API-key requests.
- Free BYOK plan: 50 reserved attempts/account/UTC calendar month; independent of Plus/Pro inference, subscription routes and paid allowances.
- Keys held in page memory only; explicit provider consent, forget control, no persistence/server upload. BYOK card translations stay on-page.
- Explicit funding selection; no automatic provider fallback or silent funding change on reopening the companion.
- AGPL-3.0-only license, third-party notices and visible source links. Repository made public: https://github.com/PAndreew/bookworm.
- Existing deployed email, branding and reader-font changes preserved and committed separately so published source matches the deployed application. Marketing drafts and the unrelated bot directory were not published.
- Existing browser-local LLM experiment remains reverted and preserved on `spike/browser-local-llm`.

Everything remains hosted on the existing VPS and domain, https://potpuri.cc. No Vercel migration, domain cutover or Yara rename was performed.

## Important limits

**Codex/ChatGPT subscription credentials are not supported.** OpenAI's documented general API uses Platform keys. Supported subscription-based product integration uses a separate Codex app-server/local-client architecture; it was not replaced with an unofficial browser token-import flow. The UI rejects subscription tokens/auth.json and explains API billing.

OpenRouter uses pasted API keys in this release; browser OAuth PKCE is not shipped. No encrypted persistent vault or OS password-manager API is claimed. Provider requests expose the text and user's IP to that provider. App JavaScript/extensions can access an active key. The app quota is bypassable by modified clients and is not a provider spending cap. Failed/cancelled reserved attempts count.

## Verification and deployment

- Go test suite passed against disposable PostgreSQL, including BYOK concurrent ceiling/reset and existing email-store tests.
- Production frontend build passed.
- Mocked-provider Playwright passed at 320/390/1280px, plus paid membership/translation and reader-font regressions, SEO and service-worker tests.
- Gitleaks scanned all Git refs/history and the working release: no secrets detected. This is automated screening, not a guarantee that no sensitive information exists. Deployment configuration and public operational documentation were reviewed separately; actual mounted secrets and `.env` files are not tracked.
- Non-breaking dependency fixes removed all high-severity npm advisories present before this work. Three low-severity build-tool dependency findings remain in the Svelte plugin/deepmerge chain; npm's suggested forced downgrade is breaking and was not applied.
- Database and source backed up privately on the VPS before deployment; previous API/web images retained for rollback. No database volumes or previously applied local-AI tables were dropped.
- Production API/web rebuilt and restarted; health, CSP, service-worker freshness, authenticated BYOK quota/reservation, rejection of secret fields, free-vs-paid gating, reader hydration/provider choices and public source link checked.
- Temporary production QA accounts and their associated records removed after checks. No real API key, provider inference or billable model call was used by testing. Real provider authentication/model access remains subject to the user's account and provider availability/CORS.

Operationally, existing installed clients need to receive service-worker v15 before old experimental bundles disappear from their browser caches. Server-side local-LLM endpoints and diagnostic pages are no longer deployed. No TOTP, Stripe, OpenRouter or Resend credentials were rotated or published.
