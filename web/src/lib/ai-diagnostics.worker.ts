import { MLCEngine, prebuiltAppConfig } from '@mlc-ai/web-llm';
import { DIAG_MODEL, DIAG_MODEL_URL, MIB, diagnosticPrompt, installBufferBudget, validateConfig, type DiagnosticConfig, type BufferStats } from './ai-diagnostics-core';

let currentId = '';
let busy = false;
let engine: MLCEngine | undefined;
let config: DiagnosticConfig;
let memory: BufferStats | undefined;
const send = (kind: string, data: Record<string, unknown> = {}) => self.postMessage({ id: currentId, kind, ...data });
const phase = (name: string, detail = '') => send('phase', { phase: name, detail });

async function instrumentGPU() {
  phase('adapter-check', 'Checking a real WebGPU adapter with shader-f16; FP32 fallback is disabled.');
  const adapter = await (navigator as any).gpu?.requestAdapter();
  if (!adapter || adapter.isFallbackAdapter || adapter.info?.isFallbackAdapter || /swiftshader|llvmpipe|software/i.test(`${adapter.info?.description ?? ''} ${adapter.info?.architecture ?? ''}`)) throw new Error('No supported hardware WebGPU adapter. Software GPUs are intentionally rejected.');
  if (!adapter.features.has('shader-f16')) throw new Error('This adapter does not expose shader-f16. The diagnostic will not silently load the higher-memory FP32 variant.');
  send('adapter', { features: [...adapter.features], vendor: adapter.info?.vendor ?? '', architecture: adapter.info?.architecture ?? '', description: adapter.info?.description ?? '', maxBufferSize: adapter.limits.maxBufferSize, maxStorageBufferBindingSize: adapter.limits.maxStorageBufferBindingSize });
  const prototype = Object.getPrototypeOf(adapter);
  const requestDevice = prototype.requestDevice;
  let devices = 0;
  let lastMemoryMessage = 0;
  prototype.requestDevice = async function (...args: any[]) {
    if (++devices !== 1) throw new Error('Diagnostic refused a second GPU device.');
    const device = await requestDevice.apply(this, args);
    phase('gpu-device-created', 'Installing a requested-buffer allocation budget. This does not cap total process RAM.');
    installBufferBudget(device, config.budgetMiB * MIB, stats => {
      memory = stats;
      if (performance.now() - lastMemoryMessage > 100) { lastMemoryMessage = performance.now(); send('memory', { memory }); }
    });
    device.lost.then((info: any) => { if (info.reason !== 'destroyed') send('failure', { error: `GPU device lost: ${String(info.message || info.reason).slice(0, 300)}`, memory }); });
    device.addEventListener('uncapturederror', (event: any) => send('failure', { error: `GPU error: ${String(event.error?.message ?? 'Unknown validation error').slice(0, 300)}`, memory }));
    return device;
  };
}

async function load() {
  await instrumentGPU();
  const record = prebuiltAppConfig.model_list.find(item => item.model_id === DIAG_MODEL);
  if (!record) throw new Error('Pinned diagnostic model is not in this runtime.');
  phase('model-initialize', 'Loading cached or remote model files, WASM and GPU state. No inference warm-up is hidden in this step.');
  const start = performance.now();
  engine = new MLCEngine({
    appConfig: { model_list: [{ ...record, model: DIAG_MODEL_URL, required_features: [...new Set([...(record.required_features ?? []), 'shader-f16'])] }] },
    logLevel: 'ERROR',
    initProgressCallback: progress => send('progress', { text: progress.text.slice(0, 400), progress: progress.progress }),
  });
  await engine.reload(DIAG_MODEL, { context_window_size: config.context, max_history_size: 1 });
  // Read-only inspection of the pinned version: never pretend a JSON option
  // changed the compiled prefill limit. No private runtime fields are mutated.
  const pipeline = (engine as unknown as { loadedModelIdToPipeline?: Map<string, { prefillChunkSize?: number; contextWindowSize?: number }> }).loadedModelIdToPipeline?.get(DIAG_MODEL);
  send('complete', { operation: 'load', loadMs: Math.round(performance.now() - start), memory, compiledPrefill: pipeline?.prefillChunkSize ?? null, actualContext: pipeline?.contextWindowSize ?? null });
}

async function run(command: any) {
  if (!engine) throw new Error('Load the model first.');
  const messages = diagnosticPrompt(command.test, command.task, command.language);
  phase('reset-chat', 'Resetting model state; every diagnostic prompt is independent.');
  await engine.resetChat();
  phase('prompt-prefill', `${command.test} ${command.task}; ${messages.reduce((n, item) => n + item.content.length, 0)} input characters. Waiting for the first output. Large prefill can fail even if model initialization succeeded.`);
  const start = performance.now();
  let firstTextMs: number | null = null;
  let output = '';
  let usage: unknown = null;
  let finishReason: string | null = null;
  const stream = await engine.chat.completions.create({ messages, max_tokens: config.maxTokens, temperature: 0.2, stream: true, stream_options: { include_usage: true }, extra_body: { enable_thinking: false } });
  for await (const chunk of stream) {
    const text = chunk.choices[0]?.delta.content ?? '';
    if (text) {
      if (firstTextMs === null) { firstTextMs = Math.round(performance.now() - start); phase('decode', 'First visible output received.'); }
      output = (output + text).slice(0, 4000);
      send('output', { text: output, firstTextMs });
    }
    if (chunk.usage) usage = chunk.usage;
    if (chunk.choices[0]?.finish_reason) finishReason = chunk.choices[0].finish_reason;
  }
  if (!output.trim()) throw new Error('The model completed without visible output.');
  send('complete', { operation: 'run', test: command.test, task: command.task, language: command.language, firstTextMs, totalMs: Math.round(performance.now() - start), finishReason, usage, memory, output });
}

self.onmessage = async ({ data: command }: MessageEvent) => {
  if (!command || typeof command.id !== 'string' || !['load', 'run'].includes(command.kind)) return;
  if (busy) { self.postMessage({ id: command.id, kind: 'failure', error: 'Diagnostic worker is busy.' }); return; }
  currentId = command.id;
  busy = true;
  try {
    if (command.kind === 'load') {
      if (engine) throw new Error('Stop before changing diagnostic configuration.');
      config = validateConfig(command.config);
      await load();
    } else await run(command);
  } catch (error) { send('failure', { error: (error instanceof Error ? error.message : String(error)).slice(0, 500), memory }); }
  finally { busy = false; }
};
