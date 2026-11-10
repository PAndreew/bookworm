# Device-only API credentials: research, not implementation

Researched 2026-11-10. This is the original pre-implementation research; see [BYOK.md](BYOK.md) for the subsequently shipped OpenAI/Anthropic/OpenRouter feature. Persistent credential storage and OpenRouter OAuth remain unimplemented.

## Conclusion

Users can enter their own API keys on-device and, where the provider permits browser requests, send them **directly from the browser to that provider over HTTPS**. Potpuri's API need not receive the keys or inference requests. Attaching a user key to a request to Potpuri's backend would still expose it to our server, even if we never persist it; that does not meet the stated requirement.

However, a web app cannot promise that credentials are inaccessible to its operator: the operator serves JavaScript that can read entered/unlocked keys. XSS, compromised dependencies, malicious extensions and compromised devices remain threats. A PWA does not change this trust boundary.

Recommended starting point: OpenRouter browser OAuth PKCE, then direct OpenAI, Anthropic and Gemini with explicit BYOK risk consent. Investigate xAI CORS before promising direct Grok support. “Muse” needs clarification; Mistral is researched below only as a possible intended provider.

## Provider documentation findings

| Provider | Authentication and API | Browser finding / caveat |
| --- | --- | --- |
| OpenAI | `Authorization: Bearer <key>`; `https://api.openai.com/v1/responses` | Official JS SDK supports browsers with `dangerouslyAllowBrowser: true`, disabled by default because of key exposure. Direct requests are technically plausible; not a secure browser vault endorsement. |
| Anthropic / Claude | `x-api-key`, `anthropic-version`; `https://api.anthropic.com/v1/messages` | Official SDK documents browser opt-in via `dangerouslyAllowBrowser: true`. SDK sends `anthropic-dangerous-direct-browser-access: true`. Use the SDK's supported API version when implementing. |
| Google Gemini Developer API | `x-goog-api-key`; `https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent` | Official Google Gen AI SDK documents browser support but explicitly cautions against exposing API keys in production clients. This is user-supplied BYOK, not permission to embed Potpuri's key. Ephemeral Live API credentials should not be assumed to authorize ordinary text generation. |
| xAI / Grok | `Authorization: Bearer <key>`; `https://api.x.ai/v1/responses`; OpenAI-compatible API documented | REST/OpenAI compatibility does not establish browser CORS compatibility. Observed preflight uses wildcard allowed headers rather than explicitly naming `Authorization`; see below. Direct-browser support remains unresolved. |
| OpenRouter | Bearer key; `https://openrouter.ai/api/v1/chat/completions` or `/responses` | Explicitly documents user-paid browser BYOK via OAuth PKCE, exchanging the code in the browser and receiving a user-controlled API key. Prefer S256 PKCE, unpredictable verifier and validated state. This is an API key, not a short-lived OAuth access token with an assumed refresh flow. Set a credit limit. |
| Mistral — only if “Muse” meant Mistral | Bearer key; `https://api.mistral.ai/v1/chat/completions` | Official TypeScript SDK and chat API documented; preflight permits the necessary headers. Still requires authenticated browser verification before claiming supported integration. |
| Muse | Unknown | Name alone does not identify a unique inference API. Need the provider URL or exact product name before concluding anything about authentication or safe integration. |

ChatGPT, Claude, Gemini consumer subscriptions and X subscriptions must not be presented as interchangeable with API access/credits. Do not collect account passwords, session cookies or consumer-login tokens. OpenRouter's documented authorization is a distinct supported flow.

### Primary sources actually read

- [OpenAI JS SDK: browser runtime warnings](https://github.com/openai/openai-node#requirements)
- [Anthropic TypeScript SDK: browser opt-in](https://github.com/anthropics/anthropic-sdk-typescript#requirements) and [header implementation](https://github.com/anthropics/anthropic-sdk-typescript/blob/main/src/client.ts)
- [Gemini API key documentation](https://ai.google.dev/gemini-api/docs/api-key) (redirected to AI Studio documentation) and [Google Gen AI SDK initialization/browser warnings](https://github.com/googleapis/js-genai#initialization)
- [xAI Responses API reference](https://docs.x.ai/developers/rest-api-reference/inference/responses) and [quickstart](https://docs.x.ai/developers/quickstart)
- [OpenRouter API authentication](https://openrouter.ai/docs/api_reference/authentication) and [OAuth PKCE browser BYOK guide](https://openrouter.ai/docs/guides/overview/auth/oauth)
- [Mistral SDK documentation](https://docs.mistral.ai/resources/sdks) and [official TypeScript SDK](https://github.com/mistralai/client-ts)
- [OWASP HTML5 security: storage and IndexedDB](https://cheatsheetseries.owasp.org/cheatsheets/HTML5_Security_Cheat_Sheet.html)
- [MDN Web Crypto / SubtleCrypto](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto)

### Limited live CORS observations

Sent unauthenticated OPTIONS requests with `Origin: https://potpuri.cc`, method POST and the minimal auth/content headers. No real credentials, prompts, billable calls or authenticated browser tests were used.

| Endpoint | Status | Observed allowed origin/headers |
| --- | --- | --- |
| OpenAI `/v1/responses` | 200 | `*`; explicitly echoed `authorization,content-type` |
| Anthropic `/v1/messages` | 200 | `*`; echoed API key, version, dangerous-browser-access and content type headers |
| Gemini `generateContent` | 200 | `https://potpuri.cc`; echoed `x-goog-api-key,content-type` |
| xAI `/v1/responses` | 200 | `*`; allowed headers `*`, methods `*` |
| OpenRouter `/api/v1/chat/completions` | 204 | `*`; explicit Authorization and Content-Type in allowed headers |
| Mistral `/v1/chat/completions` | 200 | `*`; explicit Authorization and Content-Type in allowed headers |

Important: `Authorization` is a CORS non-wildcard request-header name; a wildcard header allowance alone is not sufficient for it. Therefore the xAI result is a warning, **not a passing browser test**. Confirm in an actual supported browser with a user-owned test key; ask xAI for explicit Authorization CORS support if blocked. Do not silently proxy keys via Potpuri or bypass this by placing keys in URLs. Other results are encouraging preflight evidence only, not guarantees of authenticated response, streaming or future CORS behavior. Use fetch `credentials: 'omit'`, not Potpuri session cookies.

OpenRouter can provide Grok models without direct xAI authentication if available, but that uses OpenRouter billing/credentials, not the user's existing xAI key. Its separately documented provider-BYOK feature must not be confused with device-only storage: uploading provider keys to OpenRouter changes the trust boundary.

## What “safely stored on-device” can mean

| Approach | Persistence | Security trade-off |
| --- | --- | --- |
| In-memory only | Until tab/app lifecycle clears it | Best simple default; no persisted key, but still readable by hostile JavaScript while active. |
| Plain localStorage, sessionStorage or IndexedDB | Depends on store | Not recommended for secrets. Same-origin scripts can read them; local browser-profile access is another risk. sessionStorage is not encryption. |
| Encrypted IndexedDB vault, unlocked with a separate local passphrase | Across visits; unlock after restart | Recommended opt-in persistence if wanted. Derive an encryption key locally with a reviewed password KDF; authenticated AES-GCM encryption, fresh salt and unique nonce; keep passphrase/decryption key in memory only. Store only ciphertext and non-secret KDF parameters. Weak passphrases remain vulnerable to offline guessing. |
| Ciphertext plus an automatically available persisted CryptoKey | Across visits without unlock | Convenient but materially weaker protection against same-origin code: malicious scripts can invoke decryption even if the key is non-extractable. Not equivalent to an OS keychain. |
| Native application using Keychain/Keystore | Across visits | Stronger platform-managed storage option, but different product/distribution work; cannot be claimed for the current website/PWA. |

Encryption protects a locked vault against some storage/profile exposure. It does **not** stop malicious code capturing the unlock passphrase or stealing decrypted keys during use. Automatic unlock without any independent secret cannot provide the same protection as a separately unlocked vault. JavaScript memory clearing is best-effort, not guaranteed secure erasure.

No key sync, recovery, telemetry or server backup by default. Browser data deletion/eviction can lose the vault; provider key rotation/revocation is the recovery path. Let users choose session-only use, lock, delete a provider, or delete all stored keys. Explain shared-device risk.

## Proposed constraints for a later implementation

These are recommendations, not implemented changes:

1. Explicit mode choice between existing Potpuri-funded inference and user-funded direct inference. No automatic cloud fallback, provider switching or unannounced charges.
2. A fixed provider/HTTPS-host allowlist. Never send keys to arbitrary base URLs or follow redirects carrying secrets. Keys in auth headers, never query strings, analytics events or error messages.
3. Isolated BYOK request path in the browser. No Potpuri API payload, server log, service-worker cache, crash report, session replay or shared translation cache should receive raw keys. Strip authorization data from diagnostics and errors.
4. Strong CSP with explicit `connect-src`, minimized third-party scripts, reviewed dependencies, safe Markdown rendering and no unsafe HTML injection. Audit current CSP before extending it; hardening reduces risk but cannot remove operator trust.
5. Local passphrase unrelated to the account password if persistent storage is offered. Do not reuse the server login password for vault encryption. Do not advertise non-extractable keys as hardware-backed protection.
6. Bounded selected-phrase/card context, token caps and explicit consent. Show provider/model, who pays, and that the provider receives the text and user's IP. Existing Potpuri OpenRouter privacy-routing settings do not automatically apply to direct-provider calls.
7. Keep BYOK results private/local by default. Publishing/syncing translations would separately send content to Potpuri and needs an explicit design decision.
8. Respect API credits, model access, quotas and provider restrictions. Handle 401, insufficient credit, 429 and network/CORS failures without exposing keys or silently retrying billable requests.
9. For OpenRouter OAuth, use a static client-handled callback and S256 PKCE with validated state; no secret in URL parameters. The authorization code may reach callback access logs, so minimize logging/referrers and clear callback query parameters. Test the code exchange's own CORS separately.
10. Test real Safari/iOS and Chromium authenticated calls, streaming/cancellation, lock/reload, storage clearing, account switching, redacted errors and network capture proving no credentials reach Potpuri. No authenticated provider calls were made during this research.

## Revert completed

- Preserved all seven experiment commits through `55202c3` on remote `spike/browser-local-llm` **before** reverting.
- Pushed revert `4df1a2e` to remote `main` (also `revert/browser-local-llm`), without force push. Its tracked tree matches pre-experiment `86c86b8`.
- Fast-forwarded this worktree's local `main`; unrelated uncommitted email, font, branding and marketing work was retained.
- Frontend clean install/build passed. Go tests passed using `/usr/local/go/bin/go` (the default older Go binary could not parse the module version). Database-backed integration coverage may require its configured test database.
- npm reported six dependency vulnerabilities (three low, three high); no dependency fixes were made as part of the revert.
- No production deployment, remote database rollback or existing browser-cache cleanup performed. Removed migration 0012 was additive experiment metadata; do not destructively drop already-applied tables/data as an implicit rollback. Old installed clients may retain cached experimental assets until their service worker updates; rollout/cache retirement is a separate operational check.
