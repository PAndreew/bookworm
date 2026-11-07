// Diagnostic-only primitives. No account API, reader state, or production AI switch.
export const DIAG_MODEL = 'Qwen3.5-0.8B-q4f16_1-MLC';
export const DIAG_REVISION = '0ec138972555613c1d7812a821778ad0398c8790';
export const DIAG_MODEL_URL = `https://huggingface.co/mlc-ai/${DIAG_MODEL}/resolve/${DIAG_REVISION}/`;
export const DIAG_STORAGE_KEY = 'bw:ai-diagnostics:v1';
export const MIB = 1024 * 1024;

export type DiagnosticConfig = { context: 512 | 1024 | 2048; budgetMiB: 512 | 768 | 1024 | 2048; maxTokens: 16 | 32 | 64 | 96 };
export type DiagnosticCase = 'short' | 'card';
export type DiagnosticTask = 'explain' | 'translate';
export type BufferStats = { liveBytes: number; peakBytes: number; largestBufferBytes: number; allocatedBuffers: number; budgetBytes: number };
export type BufferRejection = { reason: 'budget-exceeded' | 'invalid-size'; requestedBytes: number | null; liveBytes: number; wouldBeBytes: number | null; budgetBytes: number; label: string; usage: number | null };

// Errors thrown through WASM/runtime callbacks may be plain objects, not Error
// instances. Keep non-enumerable fields and nested causes, with bounded output.
export function diagnosticError(error: unknown): { message: string; details: any } {
  const seen = new WeakSet<object>();
  let nodes = 0;
  let characters = 12_000;
  const text = (value: string) => { const result = value.slice(0, Math.min(4000, characters)); characters -= result.length; return result; };
  const read = (value: object, key: string) => { try { return (value as any)[key]; } catch { return '[Unreadable property]'; } };
  const copy = (value: any, depth: number): any => {
    if (++nodes > 80 || characters <= 0 || depth > 5) return '[Truncated]';
    if (value === null || typeof value === 'boolean') return value;
    if (typeof value === 'string') return text(value);
    if (typeof value === 'number') return Number.isFinite(value) ? value : String(value);
    if (typeof value !== 'object') { try { return text(String(value)); } catch { return '[Unprintable]'; } }
    if (seen.has(value)) return '[Circular or repeated reference]';
    seen.add(value);
    if (ArrayBuffer.isView(value) || value instanceof ArrayBuffer) return '[Binary data omitted]';
    if (Array.isArray(value)) return value.slice(0, 8).map(item => copy(item, depth + 1));
    let own: string[] = [];
    try { own = Object.getOwnPropertyNames(value); } catch { return '[Unreadable object]'; }
    const keys = [...new Set(['name', 'message', 'stack', 'cause', 'code', 'reason', ...own])].slice(0, 20);
    const result: Record<string, unknown> = {};
    for (const key of keys) { const item = read(value, key); if (item !== undefined) Object.defineProperty(result, key, { value: copy(item, depth + 1), enumerable: true }); }
    return result;
  };
  const details = copy(error, 0);
  let message: string;
  if (typeof details === 'string') message = details;
  else if (typeof details?.message === 'string' && details.message) message = details.message;
  else { try { message = JSON.stringify(details) ?? 'Unknown diagnostic error'; } catch { message = 'Unserializable diagnostic error'; } }
  return { message: message.slice(0, 600), details };
}
export type DiagnosticReport = {
  version: 1; recorderVersion?: number; id: string; model: string; revision: string; runtime: string;
  environment: Record<string, unknown>; config: DiagnosticConfig;
  phase: string; active: boolean; resident: boolean;
  events: Array<{ at: string; phase: string; detail: string }>;
  measurements: Record<string, unknown>;
};

export function validateConfig(value: DiagnosticConfig): DiagnosticConfig {
  if (![512, 1024, 2048].includes(value.context) || ![512, 768, 1024, 2048].includes(value.budgetMiB) || ![16, 32, 64, 96].includes(value.maxTokens)) throw new Error('Unsupported diagnostic configuration.');
  return { context: value.context, budgetMiB: value.budgetMiB, maxTokens: value.maxTokens };
}

export function createReport(config: DiagnosticConfig, environment: Record<string, unknown>): DiagnosticReport {
  return { version: 1, recorderVersion: 2, id: crypto.randomUUID(), model: DIAG_MODEL, revision: DIAG_REVISION, runtime: '@mlc-ai/web-llm@0.2.87', environment, config: validateConfig(config), phase: 'idle', active: false, resident: false, events: [], measurements: {} };
}

export function recordPhase(report: DiagnosticReport, phase: string, detail = '', active = report.active, resident = report.resident): DiagnosticReport {
  return { ...report, phase, active, resident, events: [...report.events.slice(-159), { at: new Date().toISOString(), phase, detail: detail.slice(0, 400) }] };
}

export function parseReport(raw: string | null): DiagnosticReport | null {
  if (!raw || raw.length > 200_000) return null;
  try {
    const report = JSON.parse(raw);
    validateConfig(report.config);
    return report.version === 1 && typeof report.id === 'string' && typeof report.active === 'boolean' && typeof report.resident === 'boolean' && Array.isArray(report.events) && report.events.length <= 160 && report.measurements && typeof report.measurements === 'object' ? report : null;
  } catch { return null; }
}

export function diagnosticPrompt(test: DiagnosticCase, task: DiagnosticTask, language: string) {
  if (!['short', 'card'].includes(test) || !['explain', 'translate'].includes(task) || !['English', 'French', 'German', 'Spanish', 'Romanian'].includes(language)) throw new Error('Unsupported diagnostic test.');
  const phrase = 'civil disobedience';
  const card = 'A traveller was asked to obey a rule he believed was unjust. He refused peacefully and accepted the consequences, hoping that his example would persuade others to question the rule. His neighbours disagreed about whether obedience was always a duty. Some feared that refusing any law would weaken every law. Others argued that a person should not help enforce injustice merely because it had been written into a statute. The traveller did not claim that every personal preference should outweigh public decisions. He asked instead whether conscience sometimes required a person to withhold cooperation. This invented reading sample is used only to compare a short prompt with a card-sized prompt; it is not taken from the reader’s account.';
  const passage = test === 'short' ? (task === 'translate' ? 'He refused peacefully to obey an unjust rule.' : phrase) : card;
  return [
    { role: 'system' as const, content: task === 'translate' ? `Translate the text into ${language}. Output only the translation.` : `In ${language}, explain the phrase "${phrase}" in one short sentence. Use the supplied context if present. Do not invent facts.` },
    { role: 'user' as const, content: passage },
  ];
}

export function shardURLs(manifest: unknown): Array<{ url: string; bytes: number }> {
  const records = (manifest as { records?: Array<{ dataPath: string; nbytes: number }> })?.records;
  if (!Array.isArray(records) || !records.length || records.length > 128) throw new Error('Invalid model-shard manifest.');
  const seen = new Set<string>();
  return records.map(record => {
    const url = new URL(record.dataPath, DIAG_MODEL_URL);
    if (!url.href.startsWith(DIAG_MODEL_URL) || url.search || url.hash || !Number.isSafeInteger(record.nbytes) || record.nbytes <= 0 || record.nbytes > 160 * MIB || seen.has(url.href)) throw new Error('Invalid or unexpectedly large model shard.');
    seen.add(url.href);
    return { url: url.href, bytes: record.nbytes };
  });
}

// Requested GPUBuffer bytes only: not total process RAM, GPU textures or driver memory.
export function installBufferBudget(device: { createBuffer: (descriptor: any) => any }, budgetBytes: number, update: (stats: BufferStats) => void, rejected: (allocation: BufferRejection) => void = () => {}) {
  if (!Number.isSafeInteger(budgetBytes) || budgetBytes <= 0) throw new Error('Invalid GPU buffer budget.');
  const stats: BufferStats = { liveBytes: 0, peakBytes: 0, largestBufferBytes: 0, allocatedBuffers: 0, budgetBytes };
  const create = device.createBuffer.bind(device);
  device.createBuffer = descriptor => {
    const bytes = Number(descriptor.size);
    if (!Number.isSafeInteger(bytes) || bytes < 0 || stats.liveBytes + bytes > budgetBytes) {
      const valid = Number.isSafeInteger(bytes) && bytes >= 0;
      const allocation: BufferRejection = { reason: valid ? 'budget-exceeded' : 'invalid-size', requestedBytes: Number.isFinite(bytes) ? bytes : null, liveBytes: stats.liveBytes, wouldBeBytes: Number.isSafeInteger(stats.liveBytes + bytes) ? stats.liveBytes + bytes : null, budgetBytes, label: typeof descriptor.label === 'string' ? descriptor.label.slice(0, 160) : '', usage: typeof descriptor.usage === 'number' ? descriptor.usage : null };
      // Send evidence before throwing: a WASM layer may replace/wrap this Error.
      try { rejected(allocation); } catch { /* Telemetry must not bypass the guard. */ }
      const message = valid ? `Diagnostic GPU buffer budget exceeded: next allocation ${bytes} bytes; live ${stats.liveBytes}; would become ${stats.liveBytes + bytes}; budget ${budgetBytes}. No native allocation was made. This is not total RAM.` : 'Diagnostic refused an invalid GPU buffer size before native allocation.';
      throw Object.assign(new Error(message), { name: 'DiagnosticBufferBudgetError', code: valid ? 'DIAGNOSTIC_GPU_BUFFER_BUDGET' : 'DIAGNOSTIC_GPU_BUFFER_SIZE', allocation });
    }
    const buffer = create(descriptor);
    const destroy = buffer.destroy.bind(buffer);
    let released = false;
    buffer.destroy = () => {
      destroy();
      if (!released) { released = true; stats.liveBytes -= bytes; update({ ...stats }); }
    };
    stats.liveBytes += bytes;
    stats.peakBytes = Math.max(stats.peakBytes, stats.liveBytes);
    stats.largestBufferBytes = Math.max(stats.largestBufferBytes, bytes);
    stats.allocatedBuffers++;
    update({ ...stats });
    return buffer;
  };
  return () => ({ ...stats });
}
