# Direct-browser BYOK

Free accounts can use OpenAI Platform, Anthropic or OpenRouter API keys for 50 app-enforced attempts per UTC calendar month. Paid accounts can also explicitly choose BYOK; Plus/Pro included inference and allowances are unchanged and separate. No automatic fallback.

## User flow

Open **You → Your AI**, paste a dedicated API key, refresh the current model list, explicitly choose a model and accept the BYOK terms once. **Save on this device** stores the provider/key/model and acceptance in account-scoped `localStorage`; cookies are not used, so credentials are never automatically sent to the VPS. Reload/navigation retains saved settings. **Forget key** removes the credential but preserves one-time acceptance. A different account cannot load that account's key through the UI; this is application isolation, not encryption against same-origin scripts or someone accessing the browser profile. Shared-device users should forget the key before leaving.

Puri contains only a compact model/allowance summary and a link back to You, not repeated consent/setup controls. Paid users retain a separate membership mode with no automatic funding switch. BYOK translations stay in page memory and disappear on navigation/reload.

Provider requests use fixed HTTPS endpoints, authorization headers, omitted cookies, rejected redirects, no caching, bounded input/output and a 40-second timeout. OpenAI uses the Responses API (`store:false`); Anthropic uses Messages; OpenRouter uses chat completions. Model lists come directly from each provider's Models endpoint, without consuming inference allowance; Anthropic pagination is followed by cursor on the fixed endpoint. Retired/non-text models are filtered where metadata permits, newest first, with no hardcoded default or automatic expensive model selection. A selected model may still have account-specific restrictions. OpenRouter requests deny data collection and require ZDR; model availability can be narrower as a result. Direct OpenAI/Anthropic requests use those providers' own privacy policies. Browser scripts/extensions can still steal active keys; no OS-keychain or malicious-operator protection is claimed.

OpenAI **Platform API keys only**. [OpenAI's Codex authentication documentation](https://developers.openai.com/codex/auth/) distinguishes API keys from ChatGPT subscription credentials and says general OpenAI API calls use Platform keys. [Codex app-server](https://developers.openai.com/codex/app-server/) supports product integrations but requires a local process/client; this site does not implement that separate architecture. Never paste `auth.json`, a subscription access/refresh token, account password or cookie into BYOK. No unofficial Codex OAuth client impersonation or private ChatGPT endpoint is used.

OpenRouter API keys are supported in this release; browser OAuth PKCE is researched but not yet implemented. Gemini, Meta and direct xAI are not in this release.

## Server boundary

- Authenticated `GET /v1/byok/quota`: `{used,limit,resets_at}`; `Cache-Control: no-store`.
- Authenticated `POST /v1/byok/reservations`: **empty JSON object only**. No provider, key, model, card, phrase or result. Strict decoding rejects extra fields, null and trailing JSON.
- Migration `0013_byok.sql` stores account/month/count only. Atomic UPSERT stops at 50 across tabs and replicas. New UTC months start with a fresh row.
- A reservation is made immediately before each new provider attempt. Errors/cancellation after reservation count; locally cached card translations do not reserve again. BYOK never consumes the paid assistant counter or paid global capacity.
- The server cannot prove inference occurred or prevent modified clients calling providers independently. This is an application allowance, not a security boundary or provider billing cap.
- BYOK card translations remain in Reader state only. They are not POSTed to `/cards/{id}/translations` or synchronized between readers. Full-card generation uses original card text, not an existing translation.

## License and publication

Original project code is AGPL-3.0-only; the app links to its source. Third-party fonts, dependencies and reading content retain their own terms (THIRD_PARTY_NOTICES.md). No infrastructure secrets belong in the repository. The old browser-local LLM experiment remains on `spike/browser-local-llm` and is not restored by BYOK.

## Verification

- Go tests with a disposable PostgreSQL instance, including concurrent reservations, exact 50 ceiling, UTC reset, rejection of accidental secret/text fields, unauthenticated rejection and paid-counter separation.
- Mocked-provider Playwright at 320/390/1280px: You-page setup, live catalogs/filtering/Anthropic pagination, one-time terms, account-scoped persistence/forget, all three inference formats, metadata-only usage calls, paid-mode separation, private translation cache and failure/quota gates.
- Reader regression at 320/390/1280px plus a short viewport: safe-area-visible close control, outside dismissal, retained selections, visible saved highlights and reload restoration. Source offsets remain Unicode code-point based across joined pages and continuation screens.
- Existing paid membership/translation browser regression, SEO and service-worker tests.
- No real customer keys or billable provider calls are used by tests. Authenticated real-provider/model access remains dependent on the user's provider account; CORS and model availability can change.
