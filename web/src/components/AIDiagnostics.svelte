<script lang="ts">
  import { onMount } from 'svelte';
  import { DiagnosticSession, type DiagnosticView } from '../lib/ai-diagnostics-session';
  import { DIAG_MODEL, type DiagnosticConfig, type DiagnosticCase, type DiagnosticTask, type BufferStats, type BufferRejection } from '../lib/ai-diagnostics-core';

  let session: DiagnosticSession | undefined;
  let view = $state<DiagnosticView | null>(null);
  let consent = $state(false);
  let deviceLabel = $state('');
  let context = $state<DiagnosticConfig['context']>(512);
  let budgetMiB = $state<DiagnosticConfig['budgetMiB']>(768);
  let maxTokens = $state<DiagnosticConfig['maxTokens']>(32);
  let test = $state<DiagnosticCase>('short');
  let task = $state<DiagnosticTask>('explain');
  let language = $state('English');
  let actionError = $state('');
  const config = $derived({ context, budgetMiB, maxTokens });
  const reportJSON = $derived(view ? JSON.stringify(view.report, null, 2) : '');
  const memory = $derived(view?.report.measurements.lastGPUBufferStats as BufferStats | undefined);
  const load = $derived(view?.report.measurements.load as { loadMs?: number; compiledPrefill?: number; actualContext?: number } | undefined);
  const runs = $derived((view?.report.measurements.runs ?? []) as Array<{ firstTextMs?: number; totalMs?: number; finishReason?: string }>);
  const lastRun = $derived(runs.at(-1));
  const rejection = $derived(view?.report.measurements.lastBudgetRejection as BufferRejection | undefined);
  const blocked = $derived(!view || view.busy || view.needsReset || !!view.storageWarning);
  const settingsLocked = $derived(!view || view.busy || view.ready || !!view.storageWarning);
  const locked = $derived(blocked || !!view?.ready);
  const gpuAllowed = $derived((view?.report.measurements.adapter as { shaderF16?: boolean; hardwareAdapter?: boolean } | undefined)?.shaderF16 === true && (view?.report.measurements.adapter as { hardwareAdapter?: boolean } | undefined)?.hardwareAdapter === true);
  const bytes = (value?: number) => typeof value === 'number' ? `${(value / 1024 / 1024).toFixed(1)} MiB` : 'Not measured';
  const seconds = (value?: number | null) => typeof value === 'number' ? `${(value / 1000).toFixed(2)} s` : 'Not measured';

  onMount(() => {
    let first = true;
    session = new DiagnosticSession(next => {
      view = next;
      if (first) { context = next.report.config.context; budgetMiB = next.report.config.budgetMiB; maxTokens = next.report.config.maxTokens; first = false; }
    });
    const hidden = () => { if (document.hidden) session?.stop('Tab hidden; diagnostic worker terminated. No background inference or automatic resume.'); };
    const pagehide = () => session?.stop('Page left; diagnostic worker terminated.');
    document.addEventListener('visibilitychange', hidden);
    window.addEventListener('pagehide', pagehide);
    return () => { document.removeEventListener('visibilitychange', hidden); window.removeEventListener('pagehide', pagehide); session?.stop('Diagnostic page unmounted.'); };
  });

  async function perform(action: () => void | Promise<void>) {
    actionError = '';
    try { await action(); } catch (error) { actionError = error instanceof Error ? error.message : String(error); }
  }
  async function inspect() {
    if (!session) return;
    session.configure(config);
    await session.inspect(deviceLabel);
  }
  function newReport() { session?.newReport(config); consent = false; }
  async function retry() {
    if (!session) return;
    consent = true; // Explicit risk-labelled Run/retry click, never an automatic retry.
    await session.retry(config, test, task, language, deviceLabel);
  }
  function exportReport() {
    const blob = new Blob([reportJSON], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url; link.download = `potpuri-ai-diagnostic-${view?.report.id ?? 'report'}.json`;
    document.body.appendChild(link); link.click(); link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 60_000);
  }
  async function copyReport() { await navigator.clipboard.writeText(reportJSON); }
</script>

<main class="diagnostics">
  <header>
    <a href="/app">← Back to library</a>
    <p class="eyebrow">Potpuri · isolated test lab · recorder v2</p>
    <h1>iPhone AI diagnostics</h1>
    <p>Test Qwen3.5 0.8B one stage at a time. The reader’s local AI stays paused. This page never uses cloud AI, your account, reading history or monthly allowance.</p>
  </header>

  {#if view}
    <section class="quick" aria-label="Quick diagnostic test">
      <h2>Run / retry — no export needed</h2>
      <p>{test === 'short' ? 'Short phrase' : 'Card-sized sample'} · {task} · {language}. Uses the settings below.</p>
      <p><strong>This explicit test may crash the tab.</strong> Cached weights are reused. Retry never downloads missing weights automatically.</p>
      <p role="status">{!view.cacheChecked ? 'Checking saved weight entries…' : view.weightsReady ? '✓ All weight shards are already on this device.' : 'Some weights are missing. Use Download/check weights below once.'}</p>
      <div class="buttons"><button onclick={() => perform(retry)} disabled={view.busy || !!view.storageWarning || !view.cacheChecked || !view.weightsReady}>Run / retry test</button><button class="stop" onclick={() => session?.stop()} disabled={!view.busy && !view.ready}>Stop test</button></div>
      <p>Starts a fresh report after failure and runs the selected test. Export is optional. No model starts until you click.</p>
    </section>
  {/if}

  <aside class="warning" aria-label="Diagnostic safety warning">
    <strong>This experiment can crash or reload this tab.</strong>
    <p>Close other Potpuri tabs first. Nothing runs automatically, even with cached weights. Stop is always available, and hiding or leaving this page terminates its worker.</p>
    <p>A small GPU-buffer budget is a guardrail, not a guarantee: CPU, WASM, driver and temporary memory can still exceed iOS limits.</p>
  </aside>

  {#if !view}<p role="status">Preparing the diagnostic controls. No model is loading.</p>
  {:else}
    {#if view.needsReset}
      <aside class="recovery" role="alert">
        <strong>A previous test failed or did not finish cleanly.</strong>
        <p>Saved phase: <code>{view.report.measurements.failedAt ?? view.report.phase}</code>. Nothing restarted automatically. Use Run / retry test above; exporting first is optional.</p>
      </aside>
    {/if}
    {#if view.storageWarning}<p class="recovery" role="alert">{view.storageWarning}</p>{/if}
    {#if actionError || view.error}<p class="recovery" role="alert">{actionError || view.error}</p>{/if}
    {#if rejection}
      <aside class="recovery" aria-label="GPU allocation rejection">
        <strong>The diagnostic guard refused a GPU allocation before calling the native allocator.</strong>
        <p>Next allocation: {bytes(rejection.requestedBytes ?? undefined)}. Already live: {bytes(rejection.liveBytes)}. Would become: {bytes(rejection.wouldBeBytes ?? undefined)}. Budget: {bytes(rejection.budgetBytes)}.</p>
        <p>This records our guard’s decision, not proof that iOS ran out of memory. The limit has not been raised automatically.</p>
      </aside>
    {/if}

    <section class="status" aria-label="Diagnostic status">
      <div><span>Last phase</span><strong data-diag-phase>{view.report.phase}</strong></div>
      <div><span>Model worker</span><strong>{view.busy ? 'Step running' : view.ready ? 'Loaded · idle unload in 60 s' : 'Not loaded'}</strong></div>
      <p role="status">{view.progress || 'Start with Inspect device. Opening this page does not request model files or start a GPU worker.'}</p>
    </section>

    <section>
      <h2>1. Inspect device and cached files</h2>
      <label for="diag-device">Device / OS label (optional)</label>
      <input id="diag-device" placeholder="iPhone 16 · iOS 26 · installed PWA" maxlength="80" bind:value={deviceLabel} disabled={settingsLocked} />
      <button onclick={() => perform(inspect)} disabled={locked}>Inspect device</button>
      <p>Checks WebGPU features and existing weight-cache entries without allocating a model. Reported storage quota is disk space, not available RAM.</p>
      {#if view.report.measurements.adapter !== undefined}
        <p class="fact">GPU adapter: {view.report.measurements.adapter ? ((view.report.measurements.adapter as { shaderF16: boolean }).shaderF16 ? 'FP16 available' : 'FP16 missing — GPU load will be refused') : 'Unavailable'} · Weights: {view.weightsReady ? 'all shard entries present' : 'missing or not yet checked'}</p>
      {/if}
    </section>

    <section>
      <h2>2. Download/check weights — no GPU load</h2>
      <p>The weight shards total about 424 MB; the full model files are about 447 MB, plus the runtime. Already cached FP16 shards are reused. This step uses the existing MLC Cache API format, not a new OPFS loader.</p>
      <label class="consent"><input type="checkbox" bind:checked={consent} disabled={view.busy} /><span>I understand that these explicit tests may download large files and may crash this tab.</span></label>
      <button onclick={() => perform(() => session!.downloadWeights())} disabled={locked || !consent}>Download/check weights</button>
      <p>Sequential files, no inference. If interrupted, complete cached shards remain; nothing resumes automatically.</p>
    </section>

    <section>
      <h2>3. Load the model, then send a prompt</h2>
      <p><code>{DIAG_MODEL}</code> · FP16 only, with no silent FP32 fallback.</p>
      <div class="settings">
        <label>Context tokens<select aria-label="Context tokens" bind:value={context} disabled={settingsLocked}><option value={512}>512 · start here</option><option value={1024}>1,024</option><option value={2048}>2,048 · previous reader setting</option></select></label>
        <label>GPU buffer budget<select aria-label="GPU buffer budget" bind:value={budgetMiB} disabled={settingsLocked}><option value={512}>512 MiB</option><option value={768}>768 MiB · default</option><option value={1024}>1,024 MiB · higher risk</option><option value={2048}>2,048 MiB · high crash risk</option></select></label>
        <label>Output cap<select aria-label="Output cap" bind:value={maxTokens} disabled={settingsLocked}><option value={16}>16 tokens</option><option value={32}>32 tokens</option><option value={64}>64 tokens</option><option value={96}>96 tokens</option></select></label>
      </div>
      <p><strong>Prefill is compiled into this MLC artifact (expected 1,024).</strong> This page does not pretend that an unsupported “128” option changes it. Actual runtime metadata is recorded after loading. The smaller context reduces one variable; it is not a proven fix.</p>
      <button onclick={() => perform(() => { session!.configure(config); session!.load(); })} disabled={locked || !consent || !view.weightsReady || !gpuAllowed}>Load GPU model</button>
      <p>Model loading is separate from inference: there is no hidden hello/warm-up prompt. This step may fetch missing tokenizer/config/WASM files. Changing profiles starts a new report. Exporting the previous result is optional.</p>
      <div class="settings">
        <label>Input size<select aria-label="Input size" bind:value={test} disabled={view.busy}><option value="short">Short phrase · try first</option><option value="card">Card-sized synthetic sample</option></select></label>
        <label>Task<select aria-label="Diagnostic task" bind:value={task} disabled={view.busy}><option value="explain">Explain</option><option value="translate">Translate</option></select></label>
        <label>Output language<select aria-label="Output language" bind:value={language} disabled={view.busy}>{#each ['English', 'French', 'German', 'Spanish', 'Romanian'] as item}<option>{item}</option>{/each}</select></label>
      </div>
      <button onclick={() => perform(() => session!.run(test, task, language))} disabled={blocked || !consent || !view.ready}>Run selected test</button>
      <p>Fixed synthetic prompts only. Generation resets chat state and has a 60-second deadline. A short output cap may truncate the response; this is a stability test, not a translation-quality verdict.</p>
      {#if view.output}<pre class="output" aria-label="Diagnostic model output">{view.output}</pre>{/if}
    </section>

    <section>
      <h2>Results and crash report</h2>
      <dl class="metrics">
        <div><dt>Model initialization</dt><dd>{seconds(load?.loadMs)}</dd></div>
        <div><dt>First visible text</dt><dd>{seconds(lastRun?.firstTextMs)}</dd></div>
        <div><dt>Last generation</dt><dd>{seconds(lastRun?.totalMs)}</dd></div>
        <div><dt>Peak requested GPU buffers</dt><dd>{bytes(memory?.peakBytes)}</dd></div>
        <div><dt>Largest requested buffer</dt><dd>{bytes(memory?.largestBufferBytes)}</dd></div>
        <div><dt>Actual context / compiled prefill</dt><dd>{load?.actualContext ?? '—'} / {load?.compiledPrefill ?? '—'}</dd></div>
      </dl>
      <p>GPU numbers track requested buffers, <strong>not total browser memory</strong>. The report stores phase markers on this device. If iOS kills the page, reopen this same diagnostic URL to recover the last saved phase—never an automatic retry.</p>
      <div class="buttons">
        <button onclick={exportReport}>Download report</button>
        <button onclick={() => perform(copyReport)}>Copy report</button>
        <button onclick={() => perform(newReport)} disabled={view.busy || view.ready}>New report / reset test lock</button>
      </div>
      <p>Reports contain browser/device information, timing and synthetic model output—not cookies, account data or book selections. Nothing is uploaded automatically. New report resets only this diagnostic record, never books or cached weights.</p>
      <ol class="events" aria-label="Recent diagnostic phases">{#each view.report.events.slice(-12).reverse() as event}<li><code>{event.phase}</code><span>{event.detail}</span></li>{/each}</ol>
      {#if view.report.measurements.errorDetails !== undefined}<details open><summary>Full captured error ({view.report.measurements.errorSource ?? 'unknown source'})</summary><pre class="output" aria-label="Structured diagnostic error">{JSON.stringify(view.report.measurements.errorDetails, null, 2)}</pre></details>{/if}
      <details><summary>Report preview / manual copy</summary><textarea aria-label="Diagnostic report JSON" readonly value={reportJSON} rows="12"></textarea></details>
    </section>
    <div class="stop-bar"><button class="stop" onclick={() => session?.stop()} disabled={!view.busy && !view.ready}>Stop and release model</button><a href="/app">Return to library</a></div>
  {/if}
</main>

<style>
  .diagnostics { max-width: 850px; margin: 0 auto; padding: max(24px, env(safe-area-inset-top)) 18px calc(120px + env(safe-area-inset-bottom)); color: var(--text, #17212d); }
  header, section, aside { margin-bottom: 20px; }
  h1 { font-size: clamp(1.8rem, 6vw, 2.8rem); line-height: 1.1; margin: 10px 0 16px; }
  h2 { font-size: 1.15rem; margin: 0 0 14px; }
  p { font-size: .9rem; line-height: 1.6; margin: 10px 0; }
  .eyebrow { margin-top: 22px; text-transform: uppercase; letter-spacing: .09em; font-size: .7rem; }
  a { color: #155f67; text-decoration: underline; text-underline-offset: 3px; }
  section { padding: 20px; border: 1px solid #d7dce0; border-radius: 14px; background: var(--bg, white); }
  aside { padding: 18px; border-radius: 12px; }
  .warning { background: #fff4d7; color: #5f4200; border: 1px solid #e9cd81; }
  .recovery { padding: 16px; background: #fff0eb; color: #7d2d19; border: 1px solid #efbaaa; border-radius: 12px; }
  .quick { background: #eff7f5; border-color: #94b9af; }
  .status { background: #eff7f5; display: grid; gap: 12px; }
  .status div { display: flex; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
  .status span, .metrics dt { font-size: .8rem; color: #52606c; }
  .status strong { overflow-wrap: anywhere; }
  label { display: grid; gap: 7px; font-size: .82rem; }
  input, select, textarea { width: 100%; box-sizing: border-box; min-height: 44px; padding: 10px; border: 1px solid #b8c2ca; border-radius: 7px; background: white; color: #17212d; font: inherit; }
  input { margin-bottom: 10px; }
  .consent { display: flex; gap: 12px; align-items: start; margin: 18px 0; line-height: 1.5; }
  .consent input { width: 22px; height: 22px; min-height: 22px; flex: 0 0 22px; margin: 0; }
  button { min-height: 44px; padding: 10px 15px; border-radius: 8px; border: 1px solid #173e41; background: #173e41; color: white; font: inherit; font-size: .85rem; cursor: pointer; }
  button:disabled { opacity: .4; cursor: not-allowed; }
  .settings { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 12px; margin: 18px 0; }
  .buttons { display: flex; gap: 10px; flex-wrap: wrap; }
  .fact { font-weight: 600; }
  code { font-size: .78rem; overflow-wrap: anywhere; }
  .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: 16px; }
  .metrics dd { margin: 5px 0 0; font-weight: 650; }
  .output { padding: 14px; border-radius: 8px; background: #f0f3f6; color: #17212d; white-space: pre-wrap; overflow-wrap: anywhere; font: inherit; font-size: .9rem; }
  .events { padding-left: 20px; font-size: .8rem; }
  .events li { margin: 12px 0; }
  .events span { display: block; margin-top: 4px; line-height: 1.5; overflow-wrap: anywhere; }
  details { margin-top: 18px; }
  summary { cursor: pointer; min-height: 44px; padding-top: 10px; }
  textarea { font: .7rem/1.6 monospace; }
  .stop-bar { position: fixed; z-index: 50; bottom: 0; left: 0; right: 0; display: flex; justify-content: center; align-items: center; flex-wrap: wrap; gap: 16px; padding: 12px 18px calc(12px + env(safe-area-inset-bottom)); background: #fff; border-top: 1px solid #ccd4db; box-shadow: 0 -4px 18px #0000000a; }
  .stop { background: #8c321b; border-color: #8c321b; }
  .stop-bar a { font-size: .8rem; }
  @media (max-width: 420px) { section { padding: 16px; } .metrics { grid-template-columns: 1fr; } }
</style>
