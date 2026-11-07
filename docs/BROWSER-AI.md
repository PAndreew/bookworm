# Experimental browser AI

## Offer and boundaries

Puri can run Qwen3.5-0.8B locally using WebLLM 0.2.87 in a dedicated Web Worker.
Free gets 10 local attempts per UTC calendar month, Plus 100, existing Pro 1,000.
These are separate from unchanged paid cloud allowances. Five-second spacing and
monthly reservations are atomic in PostgreSQL. Local use does not consume the
hosted daily/global pool and never calls OpenRouter. Plus/Pro default to cloud;
Free defaults to on-device execution. There is no automatic provider fallback.

`GET /v1/assistant/local` returns used, limit and UTC reset. `POST` with only
`{"card_id":"…"}` validates accessible published text-card access and reserves
one local attempt. Authentication/completed 2FA requirements are inherited from
the normal authenticated routes. Selected phrases and generated output are not
sent to this endpoint. Failed/aborted generations count after reservation;
downloads, warming and switching saved local translations do not.

**Limits on public local models are not DRM.** The official UI observes server
allowances across devices, but modified clients can bypass them and execute public
weights. Cloud API quota/security boundaries remain authoritative and unchanged.

## Startup and making repeated use fast

Initial activation is opt-in. The model repository is approximately **447 MB**,
plus the JS/WASM runtime. At 20 Mbps the model alone takes roughly 179 seconds;
at 100 Mbps roughly 36 seconds, ignoring overhead. Those are bandwidth calculations,
not measured promises. The browser must also initialize GPU resources and compile
shaders. Software-emulated WebGPU can be dramatically slower than a real GPU.

- No model weights are fetched before Enable is chosen.
- Weights/runtime are cached by WebLLM; the app service worker must preserve those
  caches when retiring its own versioned shell/book caches.
- A one-token warm-up exercises the actual model before declaring it ready.
- The engine remains loaded through Puri dismissal and card changes in the reader.
- After a successful opt-in, later reader visits automatically warm an already
  cached model in the background. This is skipped when data-saving/2G is reported.
  The cache availability check is not a browser guarantee against partial eviction;
  the original download consent remains relevant for restoring incomplete files.
- Leaving the reader releases the worker; an unloaded/reopened reader still has
  GPU initialization even if weights are cached. No promise of instant startup.
- Remove downloaded AI clears both supported quantization variants and disables
  remembered automatic warm-up. Browser storage settings are the recovery path
  if cache deletion fails.

The browser can evict caches or discard tabs. Keeping a model loaded indefinitely
would trade faster requests for memory/battery usage; this implementation retains
it only during the reader component's lifetime, not globally in a service worker.

## Runtime and models

Use `Qwen3.5-0.8B-q4f16_1-MLC` when `shader-f16` is available, otherwise
`Qwen3.5-0.8B-q4f32_1-MLC`. WebLLM catalogue GPU estimates at 4K context are
1,629 MB and 1,894 MB respectively; these are not total browser RAM measurements.
Our runtime context is 2,048 tokens. Output caps are 160 explanation / 512 translation
tokens. Source selections/cards are bounded to 1,200 characters. Oversized cards
ask the reader to select a phrase rather than silently truncate the source.

Model revisions are pinned in `web/src/lib/local-ai.ts`, and the npm dependency
is exact-pinned. Qwen3.5's thinking mode is explicitly disabled. Every task resets
chat state; no previous phrase or reader history is supplied. Explain receives
only the current source card as limited contextual help. Book content is treated
as untrusted data, not instructions; small-model prompt adherence is not guaranteed.

Downloading/loading has a ten-minute deadline, the one-token GPU warm-up has a
45-second deadline, and generation has a two-minute deadline. Known software/fallback
adapters (including SwiftShader and llvmpipe) are rejected before any model download.
Cancellation/timeout terminates active generation and frees the worker so the UI
is not held hostage by a stalled device. Aborted jobs may require warming again.
A model/device failure offers a retry/upgrade, never a hidden cloud request.

Source licenses: Qwen3.5 Apache 2.0; WebLLM Apache 2.0. Download hosts receive
connection metadata. We send neither reading text nor generated output to those
hosts for inference. The model operates on-device after loading.

## Translation safety

Local answers are experimental, not equivalent to the hosted model. Full-card
local translations support the current French/Spanish/German reader menu and
remain in memory in the current tab, identified as on-device AI. They are never
uploaded into the canonical shared translation cache. Annotations/sharing remain
disabled on translated text to preserve original offsets. A token-limit cutoff
is an error, not a saved translation. Check meaning, negation and names against
the original. Other phrase-output languages are not quality-certified.

## Verification

- `go test ./...`: normal Go checks (integration tests skip without a DB).
- `TEST_DATABASE_URL=postgres://… go test -p 1 ./...`: use a disposable PostgreSQL
  database. Existing suites race initial migration when run in parallel against
  one pristine schema; `-p 1` avoids that existing test-harness limitation.
- New integration coverage: auth, source validation, no-store status, concurrent
  reservations, cooldown/exhaustion, upgrade limits, UTC monthly reset, no provider
  calls, and unchanged paid-only cloud boundary.
- Build frontend with `cd web && npm ci && npm run build`.
- Serve the build, then run `TEST_BASE_URL=http://127.0.0.1:4338 node
  web/tests/local-ai.browser.cjs`. `PLAYWRIGHT_MODULE` and
  `PLAYWRIGHT_EXECUTABLE` can override the browser installation. Default tests mock
  the worker, not generation quality; cover 320/390/1280px, opt-in, streamed output,
  metadata-only accounting, warm reuse, translation, exhaustion and unsupported GPU.
- `REAL_LOCAL_AI=1` runs the actual pinned model and reports cold warm-up and
  first-text latency. It downloads the model and requires sufficient device RAM.
  Software WebGPU results must not be presented as representative laptop/phone GPU
  benchmarks. A successful mocked test is not a model-quality evaluation.

The older full-project Svelte check currently reports an unrelated implicit-any
in `GuestReader.svelte`; new local-AI files should introduce no additional errors.
A real pinned-model browser smoke downloaded/loaded its weights (approximately
30 seconds in that particular run), but software-emulated WebGPU on the development
Raspberry Pi did not finish the one-token warm-up after more than four minutes and
used over 4.5 GB GPU-process RSS. The run was stopped to protect the host; **no actual
generated answer or successful real-GPU inference was obtained**. This finding is
why production rejects known software adapters and bounds GPU warm-up. Mocked UI
latencies are not model benchmarks. Actual supported-GPU performance and language
quality remain unverified; deployment must be described as an opt-in experimental
beta, not a validated cloud replacement.

A representative language/device quality study is still needed before promoting
this experimental option as a dependable replacement for cloud AI.
