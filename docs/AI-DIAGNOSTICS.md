# Isolated iOS AI diagnostics

URL: **https://potpuri.cc/ai-diagnostics/**. Within the installed PWA, open Puri,
select on-device execution if needed, then follow **Open isolated diagnostic page**.
The normal reader remains paused; the diagnostic does not call account, book,
quota or hosted-model APIs. It uses fixed synthetic prompts only.

## Suggested first run on iPhone 16 / iOS 26

1. Close other Potpuri tabs. Open this page in the installed PWA.
2. Inspect device. This reads adapter features and cached shard presence, not model
   weights into RAM. It records standalone/browser mode and optional device label.
3. Read/check the risk acknowledgement, then Download/check weights. Complete
   cached FP16 MLC shards are reused, without allocating a GPU model. Cache presence
   is not an integrity verification. Existing FP32 weights are not silently reused.
4. Keep the initial **512-token context, 768 MiB GPU-buffer budget, 32 output tokens**.
   Click Load GPU model. FP16 hardware support is required; no FP32 fallback.
5. Once model-ready, run the short Explain prompt first. Then explicitly choose
   the card-sized sample / Translate test. A low output cap can truncate translation;
   this page is for stability/timing, not a quality certification.
6. Stop and export the report. If iOS reloads the page, reopen the diagnostic URL:
   the last saved active/resident phase is shown, with no automatic retry.

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
  require an explicit new report before another test. Model testing is disabled
  when diagnostic storage cannot be written.
- Report events are bounded. JSON includes browser/GPU metadata, configuration,
  observed buffer counts, timings, errors and synthetic output. Nothing is uploaded;
  the user chooses Download or Copy. Original reading/annotation keys are untouched.
- New report resets only the diagnostic record, not model caches or reading data.
- The page is noindex and excluded by the existing sitemap filter.

## Verification

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
