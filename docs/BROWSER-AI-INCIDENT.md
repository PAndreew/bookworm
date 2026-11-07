# Browser AI crash report and containment

## Report and scope

A reader reported repeated browser problems and a site/tab crash after downloading
Qwen weights while on card 26. Device/browser and the exact Explain/Translate stage
are not yet confirmed. The VPS, databases and reader endpoints remained healthy.
No server error was found in the request window checked.

**The cause of the original browser crash is unresolved.** GPU/memory pressure is
plausible, not established. This is not a claim that a physical-device crash was
reproduced or that model generation has been validated.

## Confirmed defect

An opt-in flag plus complete cached model metadata caused reader mount to call
warm-up without a new click. A reload could repeat the same GPU allocation even
when the prior session had failed. The worker remained resident during reading
after dismissing Puri.

A deterministic Playwright test seeds `webllm/model` metadata and remembered opt-in,
then opens/reloads card 26 through the real Reader/ReadingCompanion code. The worker
is stalled rather than allocating real weights:

- Before the fix: **two workers started without user action**; regression failed.
- After the fix: **zero workers**, both with test policy enabled and disabled;
  card rendering/reload remain functional with no browser JS errors.

This reproduces the unsafe repeated activation pattern, not the device's memory
failure. The initial real model smoke had already failed to complete warm-up on
software WebGPU; shipping even an opt-in beta before successful real-GPU validation
was premature. Mocked browser and quota tests did not establish resource safety.

## Containment and fix

1. Temporarily rolled the web image back to the pre-local-AI reader, preserving the
   current cache-preserving service worker. Cloud AI, reading data and APIs stayed up.
2. Added an edge 503 for `/v1/assistant/local`. Old clients fetch allowance before
   automatic warm-up, so unsuccessful allowance stops that old reload path too.
   These failures do not reserve additional requests.
3. Ship `local-ai-policy.json` with `enabled: false`. Policy is no-store and bypasses
   service-worker caching; missing, malformed or unavailable policy also disables AI.
4. Enforce policy before worker/model load and before generation, not only in UI.
5. Removed all automatic warming and retired its localStorage flag without deleting
   downloaded weights, reading positions or annotations. Cached files are not consent
   to allocate GPU memory when the reader opens.
6. If ever enabled again, activation is explicit per Puri session. Dismissal, card
   change, tab hiding and reader teardown terminate the worker. `?ai=off` provides
   an independent safe-mode override.

A currently running old worker cannot be remotely terminated by a server flag:
readers must close all Potpuri tabs, then reopen online to load the safe frontend.
Clearing site data is not needed or advised.

## Verification and release gate

Tests cover no automatic worker on cached reload, paused UI, fail-closed policy
at execution boundaries, safe-mode override, real worker termination on dismissal,
cache-policy bypass, and paid cloud operation while local AI is paused. Existing
mocked local workflows remain testable under explicitly mocked enabled policy.
No reading/database data was erased, and no cloud fallback or paid call was used
for local recovery tests.

Do not remove either production guard until a supported real device produces a
complete answer and representative explanation/translation checks pass. Measure
cold/cached startup, process/tab memory pressure where observable, repeat usage,
cancellation and recovery after device loss. Test the reported device/browser and
card when available. No instant-startup claim is justified.

What would have prevented this: a fail-closed rollout gate before resource-heavy
work, successful real-device validation before deployment, and a test asserting
that cached reader reload cannot create GPU workers without an explicit action.
