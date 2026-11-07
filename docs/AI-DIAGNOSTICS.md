# Isolated iOS AI diagnostics

URL: **https://potpuri.cc/ai-diagnostics/**. Within the installed PWA, open Puri,
select on-device execution if needed, then follow **Open isolated diagnostic page**.
The normal reader remains paused; the diagnostic does not call account, book,
quota or hosted-model APIs. It uses fixed synthetic prompts only.

## Suggested first run on iPhone 16 / iOS 26

1. Open this page inside the same installed PWA that downloaded the weights.
   The page checks local cache metadata/keys automatically, without reading the
   large weight bodies, requesting a GPU adapter or starting a model.
2. If cached weights are present, tap **Run / retry test** at the top. This explicit,
   risk-labelled action starts a fresh report after failure, inspects capabilities,
   loads the existing cached model and runs the selected test. **No export or
   separate reset/load sequence is required.** It never downloads missing weights.
3. Only if files are genuinely missing, use Download/check weights once. Existing
   shards are skipped. Cache presence is not an integrity verification; FP32 files
   cannot silently substitute for the pinned FP16 model.
4. Keep the initial **512-token context, 768 MiB GPU-buffer budget, 32 output tokens**.
   Advanced controls remain below, including separate inspect/load/test actions.
   Failed runs no longer lock the configuration selectors.
5. Reports are optional to export. If iOS reloads the page, the previous phase is
   retained and nothing restarts until another explicit Run / retry click. Closing,
   hiding or stopping during the combined flow cancels its remaining steps too.

**This test may still crash the tab.** The buffer budget tracks requested
GPUBuffer allocations, not CPU/WASM memory, GPU textures, driver memory, or total
process RAM. It is not a browser memory entitlement or a guarantee against OOM.
A GPU-buffer over-budget allocation is rejected before calling native createBuffer.

## What is and is not configurable

The pinned model is Qwen3.5-0.8B-q4f16_1-MLC at revision
`0ec138972555613c1d7812a821778ad0398c8790`, WebLLM 0.2.87.
Context, requested-buffer budget and output cap are explicit test settings.
`max_history_size=1` is retained for Qwen3.5's hybrid state.

**Prefill size comes from compiled WASM metadata in this WebLLM version.** It is not
an ordinary ChatOptions override. The page therefore displays the actual pipeline
prefill/context after loading using read-only inspection of the pinned version's
pipeline. It never changes private runtime fields or claims that a JSON setting
rebuilt a 128-token-prefill kernel. A smaller-prefill comparison needs a compatible
compiled artifact or a separately validated runtime, not a misleading dropdown.

## Lifecycle, report and privacy

- No worker or weight request starts on page open/reload. Each heavy step is explicit.
- Download/cache is separate from GPU initialization; initialization contains no
  hidden one-token inference. Tokenizer/config/WASM can still load in that stage.
- One worker/model is reused between explicit prompt tests. There is no per-prompt
  worker recreation and no automatic retries or precision changes.
- Load/generation have 60-second deadlines. Idle GPU workers unload after 60 seconds.
  Stop, page hiding, navigation and component teardown terminate the worker.
- Before each risky step the current phase is synchronously saved under
  `bw:ai-diagnostics:v1`. Known failures and interrupted/resident prior sessions
  require an explicit action before another test. Run / retry creates the fresh
  report itself; exporting the previous report is never required. Model testing is disabled
  when diagnostic storage cannot be written.
- Report events are bounded. JSON includes browser/GPU metadata, configuration,
  observed buffer counts, timings, errors and synthetic output. Nothing is uploaded;
  the user chooses Download or Copy. Original reading/annotation keys are untouched.
- New report resets only the diagnostic record, not model caches or reading data.
- The page is noindex and excluded by the existing sitemap filter.

## Recorder v2: preserve the actual failure

The iPhone report successfully initialized the model in 4,695 ms, then failed
before first output with approximately 749 MiB tracked live buffers against the
768 MiB guard. Its `[object Object]` failure summary discarded the thrown runtime
object. It does not establish whether iOS, the runtime or our guard caused failure.

Recorder v2 keeps bounded structured error fields, stack, code and nested causes,
including plain/cross-realm error objects. Independently, a guard rejection records
requested bytes, already-live bytes, the proposed total, budget, usage and buffer
label **before** the error can be wrapped by WASM. This is saved immediately without
replacing the active phase, so `failedAt` still identifies prefill/load correctly.
Native GPU errors and device loss have separate source labels. No limits, precision,
context, prompts or automatic-retry behavior were relaxed.

The regression runs the real diagnostic worker/core with a stub model runtime.
It starts at the report's 784,869,392 live bytes and attempts a **synthetic** 32 MiB
allocation, confirming that native allocation is blocked and both the independent
rejection and runtime-wrapped cause survive. This is not evidence that the phone's
actual next allocation was 32 MiB. A separate native-error case verifies that it
is not mislabeled as a budget rejection.

Refresh the diagnostic page and use Run / retry test to repeat the default short
test directly. Exporting the old record is optional. Cached model URLs are unchanged. New
reports include `recorderVersion: 2`; existing reports remain readable unchanged.

## Verification

- `node web/tests/ai-diagnostics-errors.cjs`: actual worker error forwarding,
  independently captured rejected allocation, wrapped causes and native-error distinction.
- `node --experimental-strip-types web/tests/ai-diagnostics-core.cjs`: configuration,
  report bounds and crash flags, pinned shard URLs, fixed prompts and allocation
  budget enforced before native createBuffer.
- `TEST_BASE_URL=http://127.0.0.1:4338 node web/tests/ai-diagnostics.browser.cjs`:
  390/1280px, no automatic model/API activity, separate cache/load/generate,
  cache reuse, one worker per active session, report download, plain-text output,
  failure lock, crash-marker recovery, and worker termination on backgrounding.
  These tests use a synthetic worker and tiny weight fixtures, not a real model.
- Svelte check introduces no new errors; the existing unrelated GuestReader implicit-any
  remains. Diagnostic worker/session pass a separate TypeScript no-emit check.

A successful real iPhone generation is still required. Do not interpret mocked
worker timings, buffer-count tests or this diagnostic deployment as proof that
the production reader AI is ready to re-enable.
