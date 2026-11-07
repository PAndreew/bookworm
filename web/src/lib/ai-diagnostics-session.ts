import { DIAG_MODEL_URL, DIAG_STORAGE_KEY, createReport, diagnosticError, parseReport, recordPhase, shardURLs, validateConfig, type DiagnosticConfig, type DiagnosticReport, type DiagnosticCase, type DiagnosticTask } from './ai-diagnostics-core';

export type DiagnosticView = { report: DiagnosticReport; ready: boolean; busy: boolean; weightsReady: boolean; cacheChecked: boolean; needsReset: boolean; error: string; progress: string; output: string; storageWarning: string };
const DEFAULT: DiagnosticConfig = { context: 512, budgetMiB: 768, maxTokens: 32 };

export class DiagnosticSession {
  private view: DiagnosticView;
  private worker?: Worker;
  private download?: AbortController;
  private id = '';
  private deadline?: ReturnType<typeof setTimeout>;
  private idle?: ReturnType<typeof setTimeout>;
  private lastSave = 0;
  private environment: Record<string, unknown>;
  private actionEpoch = 0;
  private queuedTest?: { test: DiagnosticCase; task: DiagnosticTask; language: string };
  constructor(private changed: (view: DiagnosticView) => void) {
    let previous: DiagnosticReport | null = null;
    let storageWarning = '';
    try { previous = parseReport(localStorage.getItem(DIAG_STORAGE_KEY)); }
    catch { storageWarning = 'Diagnostic storage is unavailable; model tests are disabled because crash markers cannot be saved.'; }
    const environment = { userAgent: navigator.userAgent, standalone: !!(navigator as any).standalone || window.matchMedia('(display-mode: standalone)').matches, hardwareConcurrency: navigator.hardwareConcurrency, deviceMemoryGiB: (navigator as any).deviceMemory ?? null, viewport: { width: window.innerWidth, height: window.innerHeight } };
    this.environment = environment;
    this.view = { report: previous ?? createReport(DEFAULT, environment), ready: false, busy: false, weightsReady: false, cacheChecked: false, needsReset: !!previous && (previous.active || previous.resident || previous.phase === 'failure'), error: '', progress: '', output: '', storageWarning };
    if (!previous) this.publish(true);
    else this.changed({ ...this.view });
    // Metadata/keys only: no adapter, worker, weight body or network request.
    void this.checkSavedWeights();
  }

  private async checkSavedWeights() {
    try {
      const cache = await this.cacheStatus();
      if (!this.view.busy) this.view.weightsReady = cache.complete;
    } catch { /* A manual check can explain missing/unavailable cache later. */ }
    this.view.cacheChecked = true;
    this.publish();
  }

  async retry(config: DiagnosticConfig, test: DiagnosticCase, task: DiagnosticTask, language: string, deviceLabel = '') {
    if (this.view.busy) throw new Error('A diagnostic step is already running.');
    if (this.view.ready && JSON.stringify(config) === JSON.stringify(this.view.report.config)) { this.run(test, task, language); return; }
    if (this.view.ready) this.stop('Stopped to change diagnostic configuration.');
    // This explicit click replaces the failed report. Export is never required.
    this.newReport(config);
    const epoch = this.actionEpoch;
    await this.inspect(deviceLabel);
    if (epoch !== this.actionEpoch || document.hidden || this.view.needsReset) return;
    if (!this.view.weightsReady) throw new Error('Some weight files are not cached. Use Download/check weights once; retry never downloads them automatically.');
    this.queuedTest = { test, task, language };
    try { this.load(); } catch (error) { this.queuedTest = undefined; throw error; }
  }

  private publish(save = false) {
    if (save) {
      try { localStorage.setItem(DIAG_STORAGE_KEY, JSON.stringify(this.view.report)); this.lastSave = performance.now(); }
      catch { this.view.storageWarning = 'Could not save the diagnostic crash marker. Stop and export the report; further model tests are disabled.'; }
    }
    this.changed({ ...this.view });
  }
  private phase(name: string, detail: string, active = this.view.report.active, resident = this.view.report.resident) {
    this.view.report = recordPhase(this.view.report, name, detail, active, resident);
    this.publish(true);
  }
  private measurements(values: Record<string, unknown>, save = false) {
    this.view.report = { ...this.view.report, measurements: { ...this.view.report.measurements, ...values } };
    this.publish(save);
  }
  private allowed() {
    if (this.view.busy) throw new Error('A diagnostic step is already running.');
    if (this.view.needsReset) throw new Error('Export the previous report and start a new report first. Nothing will resume automatically.');
    if (this.view.storageWarning) throw new Error(this.view.storageWarning);
  }

  newReport(config: DiagnosticConfig = DEFAULT) {
    if (this.view.busy || this.view.ready) throw new Error('Stop and release the model first.');
    this.actionEpoch++;
    this.view = { ...this.view, report: createReport(validateConfig(config), this.environment), needsReset: false, error: '', progress: '', output: '', storageWarning: '' };
    this.publish(true);
  }
  configure(config: DiagnosticConfig) {
    this.allowed();
    if (this.view.ready) throw new Error('Stop before changing the loaded model configuration.');
    if (JSON.stringify(config) !== JSON.stringify(this.view.report.config)) this.newReport(config);
  }

  private async cacheStatus() {
    const cache = await caches.open('webllm/model');
    const response = await cache.match(DIAG_MODEL_URL + 'tensor-cache.json');
    if (!response) return { complete: false, present: 0, total: null, expectedBytes: null };
    const shards = shardURLs(await response.json());
    const keys = new Set((await cache.keys()).map(request => request.url));
    const present = shards.filter(shard => keys.has(shard.url)).length;
    return { complete: present === shards.length, present, total: shards.length, expectedBytes: shards.reduce((sum, shard) => sum + shard.bytes, 0) };
  }

  async inspect(deviceLabel = '') {
    this.allowed();
    const epoch = this.actionEpoch;
    this.view.report = { ...this.view.report, environment: { ...this.view.report.environment, userProvidedDeviceLabel: deviceLabel.slice(0, 80) } };
    this.view.busy = true;
    this.view.error = '';
    this.phase('inspect-device', 'Checking adapter features and cached file presence. No GPU device, weights or inference worker is created.', true, false);
    try {
      const adapter = await (navigator as any).gpu?.requestAdapter();
      const info = adapter?.info;
      const cache = await this.cacheStatus();
      if (epoch !== this.actionEpoch) return;
      this.view.weightsReady = cache.complete;
      this.view.cacheChecked = true;
      let storage: unknown = null;
      try { storage = await navigator.storage?.estimate(); } catch { /* Optional disk-space information. */ }
      if (epoch !== this.actionEpoch) return;
      this.measurements({ adapter: adapter ? { hardwareAdapter: !adapter.isFallbackAdapter && !info?.isFallbackAdapter && !/swiftshader|llvmpipe|software/i.test(`${info?.description ?? ''} ${info?.architecture ?? ''}`), shaderF16: adapter.features.has('shader-f16'), features: [...adapter.features], vendor: info?.vendor ?? '', architecture: info?.architecture ?? '', description: info?.description ?? '', maxBufferSize: adapter.limits?.maxBufferSize, maxStorageBufferBindingSize: adapter.limits?.maxStorageBufferBindingSize } : null, cache, diskStorageEstimate: storage });
      this.phase('device-inspected', cache.complete ? 'Cached weight files are present. Presence is not a cryptographic integrity check.' : 'Some or all weights are missing; use Download/check weights before GPU loading.', false, false);
    } catch (error) { this.fail(error); }
    finally { if (epoch === this.actionEpoch) { this.view.busy = false; this.publish(true); } }
  }

  async downloadWeights() {
    this.allowed();
    this.view.busy = true;
    this.view.error = '';
    const controller = new AbortController();
    this.download = controller;
    this.phase('weight-cache-check', 'Download/cache step only. GPU initialization and inference are not started.', true, false);
    this.deadline = setTimeout(() => this.stop('Download timed out after ten minutes.'), 10 * 60_000);
    try {
      if (this.view.storageWarning) throw new Error(this.view.storageWarning);
      const cache = await caches.open('webllm/model');
      const manifestURL = DIAG_MODEL_URL + 'tensor-cache.json';
      let response = await cache.match(manifestURL);
      if (!response) {
        response = await fetch(manifestURL, { signal: controller.signal, credentials: 'omit', cache: 'no-store' });
        if (!response.ok) throw new Error(`Model manifest HTTP ${response.status}.`);
        // Small metadata only; large shard bodies are never arrayBuffer()'d here.
        const manifest = await response.json();
        shardURLs(manifest);
        await cache.put(manifestURL, new Response(JSON.stringify(manifest), { headers: { 'Content-Type': 'application/json' } }));
        response = await cache.match(manifestURL);
      }
      const shards = shardURLs(await response!.json());
      const keys = new Set((await cache.keys()).map(request => request.url));
      let cached = 0;
      for (let i = 0; i < shards.length; i++) {
        controller.signal.throwIfAborted();
        const shard = shards[i];
        const hit = keys.has(shard.url);
        this.view.progress = `${i + 1}/${shards.length} · ${Math.round(shard.bytes / 1024 / 1024)} MiB · ${hit ? 'already cached' : 'downloading'}`;
        this.phase(hit ? 'weight-cache-hit' : 'weight-download', `Shard ${i + 1}/${shards.length}; ${shard.bytes} expected bytes. Files are processed sequentially.`, true, false);
        if (hit) cached++;
        else {
          const file = await fetch(shard.url, { signal: controller.signal, credentials: 'omit', cache: 'no-store' });
          if (!file.ok) throw new Error(`Weight shard ${i + 1} HTTP ${file.status}.`);
          await cache.put(shard.url, file);
        }
      }
      controller.signal.throwIfAborted();
      const status = await this.cacheStatus();
      controller.signal.throwIfAborted();
      if (!status.complete) throw new Error('The browser did not retain every weight shard.');
      this.view.weightsReady = true;
      this.view.cacheChecked = true;
      this.measurements({ cache: status, download: { reusedShards: cached, fetchedShards: shards.length - cached } });
      this.phase('weights-cached', 'Weights are cached. Tokenizer/config/WASM files may still be fetched during GPU initialization. No model is loaded.', false, false);
    } catch (error) { if (!controller.signal.aborted) this.fail(error); }
    finally {
      if (this.download === controller) { this.download = undefined; clearTimeout(this.deadline); this.view.busy = false; this.publish(true); }
    }
  }

  load() {
    this.allowed();
    if (this.view.ready || this.worker) throw new Error('A model worker already exists. Stop it first.');
    const adapter = this.view.report.measurements.adapter as { shaderF16?: boolean; hardwareAdapter?: boolean } | undefined;
    if (!adapter?.shaderF16 || !adapter.hardwareAdapter) throw new Error('Inspect this configuration first; a hardware adapter with shader-f16 is required.');
    if (!this.view.weightsReady) throw new Error('Download/check weights first so cache loading and GPU initialization are separate tests.');
    this.id = crypto.randomUUID();
    this.view.busy = true;
    this.view.error = '';
    this.view.output = '';
    this.phase('worker-start', 'Explicit diagnostic activation. If the tab disappears, this report records the last phase. The reader AI remains paused.', true, false);
    try {
      if (this.view.storageWarning) throw new Error(this.view.storageWarning);
      this.worker = new Worker(new URL('./ai-diagnostics.worker.ts', import.meta.url), { type: 'module' });
      this.worker.onmessage = event => this.message(event.data);
      this.worker.onerror = event => { event.preventDefault(); this.fail(event.error ?? { name: 'WorkerError', message: event.message || 'Diagnostic worker crashed.', filename: event.filename, line: event.lineno, column: event.colno }, undefined, 'worker-error-event'); };
      this.worker.onmessageerror = () => this.fail(new Error('Could not read a diagnostic worker response.'));
      this.deadline = setTimeout(() => this.fail(new Error('GPU initialization exceeded 60 seconds. No retry was started.')), 60_000);
      this.worker.postMessage({ id: this.id, kind: 'load', config: this.view.report.config });
    } catch (error) { this.fail(error); }
  }

  run(test: DiagnosticCase, task: DiagnosticTask, language: string) {
    this.allowed();
    if (!this.worker || !this.view.ready) throw new Error('Load the model first.');
    clearTimeout(this.idle);
    this.id = crypto.randomUUID();
    this.view.busy = true;
    this.view.error = '';
    this.view.output = '';
    this.phase('test-start', `Explicit ${test} ${task} test; output cap ${this.view.report.config.maxTokens} tokens.`, true, true);
    if (this.view.storageWarning) { this.fail(new Error(this.view.storageWarning)); return; }
    this.deadline = setTimeout(() => this.fail(new Error('Generation exceeded 60 seconds. Worker terminated; no retry.')), 60_000);
    this.worker.postMessage({ id: this.id, kind: 'run', test, task, language });
  }

  private message(message: any) {
    if (!message || message.id !== this.id) return;
    if (message.kind === 'phase') this.phase(String(message.phase), String(message.detail ?? ''));
    else if (message.kind === 'progress') {
      this.view.progress = String(message.text).slice(0, 400);
      this.measurements({ lastRuntimeProgress: this.view.progress }, performance.now() - this.lastSave > 1000);
    } else if (message.kind === 'adapter') this.measurements({ workerAdapter: message }, true);
    else if (message.kind === 'memory') this.measurements({ lastGPUBufferStats: message.memory }, performance.now() - this.lastSave > 1000);
    else if (message.kind === 'output') { this.view.output = String(message.text).slice(0, 4000); this.publish(); }
    else if (message.kind === 'budget-rejected') {
      // Persist before the runtime handles the thrown guard error. Do not change
      // the active phase or terminate early and lose the ensuing runtime exception.
      if (message.memory) this.measurements({ lastGPUBufferStats: message.memory });
      this.measurements({ lastBudgetRejection: { ...message.allocation, rejectedDuring: this.view.report.phase } }, true);
    }
    else if (message.kind === 'failure') { if (message.memory) this.measurements({ lastGPUBufferStats: message.memory }); this.fail(message.error, message.errorDetails, message.errorSource ?? 'worker'); }
    else if (message.kind === 'complete') {
      clearTimeout(this.deadline);
      this.view.busy = false;
      this.view.ready = true;
      if (message.operation === 'load') this.measurements({ load: message, lastGPUBufferStats: message.memory });
      else {
        const runs = (this.view.report.measurements.runs as unknown[] | undefined) ?? [];
        this.measurements({ runs: [...runs.slice(-9), message], lastGPUBufferStats: message.memory });
        this.view.output = String(message.output ?? '').slice(0, 4000);
      }
      this.phase(message.operation === 'load' ? 'model-ready' : 'test-completed', message.operation === 'load' ? 'GPU initialization completed, but no prompt has been tested. Idle model unloads after 60 seconds.' : `Generation finished (${message.finishReason ?? 'unknown finish reason'}). Idle model unloads after 60 seconds.`, false, true);
      if (message.operation === 'load' && this.queuedTest) {
        const next = this.queuedTest;
        this.queuedTest = undefined;
        try { this.run(next.test, next.task, next.language); } catch (error) { this.fail(error); }
        return;
      }
      this.idle = setTimeout(() => this.stop('Idle model released after 60 seconds.'), 60_000);
    }
  }

  private fail(error: unknown, details?: unknown, source = 'client') {
    const failedAt = this.view.report.phase;
    const formatted = diagnosticError(error);
    this.disposeResources();
    this.view.error = formatted.message;
    this.view.needsReset = true;
    this.measurements({ failedAt, errorDetails: details ?? formatted.details, errorSource: source });
    this.phase('failure', this.view.error, false, false);
  }
  private disposeResources() {
    this.actionEpoch++;
    clearTimeout(this.deadline); clearTimeout(this.idle);
    this.id = '';
    this.queuedTest = undefined;
    this.download?.abort(); this.download = undefined;
    this.worker?.terminate(); this.worker = undefined;
    this.view.ready = false; this.view.busy = false;
  }
  stop(reason = 'Stopped by the user. Worker terminated; browser resource release may be asynchronous.') {
    if (!this.worker && !this.download && !this.view.busy) return;
    this.disposeResources();
    this.phase('stopped', reason, false, false);
  }
}
