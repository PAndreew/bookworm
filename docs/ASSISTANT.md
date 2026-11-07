# Puri: reading companion

## Experimental on-device AI

Signed-in Free readers can enable **Qwen3.5-0.8B** in their browser for explanations,
phrase translations and short full-card translations. Free allows **10 local
attempts/month**, Plus **100**, and existing Pro **1,000**; these are separate
from paid cloud allowances. WebGPU is required. See [BROWSER-AI.md](BROWSER-AI.md)
for downloads, warm-up, privacy, limits, tests and deployment details.

The sections below describe the existing **cloud** service. Free remains blocked
from provider calls: local errors never silently fall back to cloud AI.

Puri uses the Glim SVG artwork, shared with the Potpuri logo. On the landing
page, Puri is non-interactive and occasionally displays one of five prepared
book facts in a turquoise bubble. Guest previews also use prepared explanations,
never the assistant API; see [GUEST-ONBOARDING.md](GUEST-ONBOARDING.md).

In the full reader, its 32px circle follows an inline marker at the end
of the current paragraph/card, hopping vertically rather than along a bottom
track. See [READER-CARDS.md](READER-CARDS.md) for non-destructive pagination,
original annotation preservation and monthly licensed-card usage. Scroll events
trigger short hops, not continuous animation. Reduced-motion preference disables
hops, sliding transitions and the five-second three-frame blink. Its accessible label/tooltip includes the exact
reader page, so rounding a percentage never hides small progress changes.

Click Puri after selecting book text, paste a phrase, or choose Translate current
card. A card's dropdown offers Original plus its saved translations. The native dialog supports
keyboard focus trapping, Escape/close, explanation, translation and output language.
For cloud execution the user must sign in with Plus or Pro. Submitting a phrase
in cloud mode sends it to the provider by default, without a separate consent checkbox. Free accounts are blocked server-side before any provider call.
Results are plain text, never interpreted HTML. Closing cancels the browser request.

## API and privacy

`POST /v1/assistant` requires a paid Plus/Pro authenticated, completed-2FA session:

```json
{"mode":"explain","phrase":"c'est la vie","language":"English","consent":true}
```

Only the explicit phrase or current card text (maximum 1200 characters), desired
language and task instructions reach OpenRouter. No whole book, account details,
reading history, retrieval tools, or conversation history are supplied. Personal
phrase explanations are not persisted. Full-card French/Spanish/German translations
are stored by canonical card ID, language and source hash for reuse by paid readers.
They are labeled AI translations; annotations are disabled in translated text to
protect immutable original-text offsets. Provider response bodies are not logged. The response is `no-store`, and
the service worker never caches assistant requests. Provider routing requests
`data_collection: deny` and `zdr: true`; if no eligible route exists, the request
fails rather than silently relaxing those constraints. Provider billing/operational
metadata remains subject to the provider's policies.

Model: **`google/gemini-3.8-flash`**, confirmed in OpenRouter's model catalogue.
Output is limited to 800 tokens. A 25-second request deadline fits inside the API's
30-second HTTP write timeout. Failed responses/timeouts expose only safe errors.

Atomic PostgreSQL reservations cap new attempts at **100 per Plus reader/calendar
month** for the founding offer, **1,000 for existing Pro accounts**, **1,000 across the
app/day**, and **one per reader per five seconds**. Calendar-month allowances reset
on the first at midnight UTC and do not roll over. Cached translations do not reserve
another request. Limits survive restarts/replicas; failed provider attempts count.
Usage rows store only counts/timestamps. The only public paid offer is €20/year for
the first 50 subscriptions; see [BILLING.md](BILLING.md). OpenRouter account
budgets should also be configured, especially if the same key serves other apps.

## Encrypted VPS credential vault

The production key is not in Git, Compose environment values or browser bundles.
It is held in a root-only, encrypted systemd credential vault:

- `/opt/potpuri-reader-vault/`: mode 0700, root owned.
- `openrouter_api_key.cred`: mode 0600, encrypted with `systemd-creds --with-key=host`.
- Encryption host key: `/var/lib/systemd/credential.secret` (root-only).
- `ops/potpuri-reader-secrets.service` is installed/enabled in systemd. It decrypts
  before Docker starts on boot into `/run/potpuri-reader-secrets/` (0700).
- The runtime file is 0444 so the non-root API process can read its Docker-mounted
  secret; the host's enclosing 0700 directory prevents other local users traversing it.
- Compose mounts it only in the API at `/run/secrets/openrouter_api_key`;
  `BW_OPENROUTER_KEY_FILE` names the mount. `OPENROUTER_SECRET_FILE` can override
  the host path. No key value enters container environment metadata.

This is a machine-bound encrypted-file vault, **not HashiCorp Vault or a cloud KMS**.
It protects stored credentials from unprivileged users/accidental disclosure, not
an attacker with root access to the live VPS. Back up the encrypted credential and
host encryption key only to independently encrypted, access-controlled storage.
A credential backup without its host key cannot be recovered on another machine.

To rotate, securely pipe the new key into `systemd-creds encrypt --with-key=host
--name=openrouter_api_key - /opt/potpuri-reader-vault/openrouter_api_key.cred`, never
putting it in command arguments, shell history, logs or a tracked file. Then:

```sh
systemctl restart potpuri-reader-secrets.service
cd /opt/bookworm
docker compose -f compose.production.yml up -d --force-recreate api
```

Force recreation remounts the decrypted file after replacement. Keep the vault,
service and runtime directory intact during app deployment. Verify secret service
health after boot; do not expose the mounted file through any HTTP route.

Development may leave `BW_OPENROUTER_KEY_FILE` unset; the endpoint then returns 503.
The production Compose stack requires its configured secret source to exist.

## Tests

- `go test ./...`: payload validation, both tasks, privacy/provider boundaries,
  output limits, cancellation, safe failures.
- `TEST_DATABASE_URL=postgres://... go test ./...` on a disposable DB: auth/consent,
  missing configuration, atomic cooldown, daily caps and rollback of reservations.
- Optional no-credit browser regression:
  install Playwright locally, serve a built frontend, then run
  `TEST_BASE_URL=http://localhost:4321 node web/tests/companion.browser.cjs`.
  `PLAYWRIGHT_EXECUTABLE` and `PLAYWRIGHT_MODULE` may override browser/module paths.
  Tests mock all APIs and check mobile/desktop scroll movement, exact page labels,
  selection capture, consent, explain/translate, dismissal and reduced motion.
- Real production explanation and Romanian translation were also verified with
  Gemini 3.8 Flash. Temporary QA accounts were removed afterwards.
