<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { api, ApiError } from '../lib/api';

  let { progress, positionLabel, cardIndex }: { progress?: number; positionLabel?: string; cardIndex?: number } = $props();
  let documentProgress = $state(0);
  let x = $state(0);
  let y = $state(0);
  let visible = $state(false);
  let remeasure = () => {};
  $effect(() => {
    progress; cardIndex;
    const frame = requestAnimationFrame(() => remeasure());
    return () => cancelAnimationFrame(frame);
  });
  let reducedMotion = $state(false);
  let icon = $state<SVGSVGElement | null>(null);
  let dialog = $state<HTMLDialogElement | null>(null);
  let selected = '';
  let phrase = $state('');
  let mode = $state<'explain' | 'translate'>('explain');
  let language = $state('English');
  let consent = $state(false);
  let answer = $state('');
  let error = $state('');
  let busy = $state(false);
  let auth = $state<'checking' | 'signed-in' | 'signed-out' | 'upgrade'>('checking');
  let controller: AbortController | null = null;
  let lastHop = 0;
  let hop: Animation | null = null;
  const percent = $derived(Math.max(0, Math.min(100, progress ?? documentProgress)));

  function jump() {
    if (reducedMotion || !icon || performance.now() - lastHop < 450) return;
    lastHop = performance.now();
    hop?.cancel();
    hop = icon.animate([
      { transform: 'translateY(0) rotate(0deg)' },
      { transform: 'translateY(-10px) rotate(-5deg)', offset: 0.45 },
      { transform: 'translateY(0) rotate(0deg)' },
    ], { duration: 450, easing: 'ease-in-out' });
  }

  onMount(() => {
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const updateMotion = () => { reducedMotion = motion.matches; if (reducedMotion) hop?.cancel(); };
    updateMotion();
    motion.addEventListener('change', updateMotion);
    let frame = 0;
    function update() {
      const total = document.documentElement.scrollHeight - window.innerHeight;
      documentProgress = total > 0 ? (window.scrollY / total) * 100 : 0;
      const markers = progress !== undefined
        ? Array.from(document.querySelectorAll(`.feed [data-idx="${cardIndex ?? 0}"] [data-sunny-anchor]`))
        : Array.from(document.querySelectorAll('main [data-sunny-anchor]'));
      const rects = markers.filter(el => {
        const box = el.closest('[data-body]')?.getBoundingClientRect();
        const rect = el.getBoundingClientRect();
        return !box || (rect.top >= box.top - 1 && rect.bottom <= box.bottom + 1);
      }).map(el => el.getBoundingClientRect()).filter(r => r.top >= 60 && r.bottom <= window.innerHeight - 12);
      rects.sort((a, b) => Math.abs(a.top - window.innerHeight * 0.68) - Math.abs(b.top - window.innerHeight * 0.68));
      visible = rects.length > 0;
      if (visible) { x = rects[0].left + 2; y = rects[0].top; }
    }
    remeasure = update;
    function schedule() { if (!frame) frame = requestAnimationFrame(() => { frame = 0; update(); }); }
    function scroll() { jump(); schedule(); }
    function selection() {
      const sel = window.getSelection();
      if (!sel || sel.isCollapsed || !sel.rangeCount) return;
      const node = sel.getRangeAt(0).commonAncestorContainer;
      const el = node instanceof Element ? node : node.parentElement;
      if (el?.closest('dialog')) return;
      if (progress !== undefined && !el?.closest('[data-body]')) return;
      selected = sel.toString().trim().slice(0, 1200);
    }
    update();
    const resize = new ResizeObserver(update);
    resize.observe(document.documentElement);
    const mutations = new MutationObserver(schedule);
    const surface = document.querySelector('.reader') ?? document.querySelector('main');
    if (surface) mutations.observe(surface, { childList: true, subtree: true, attributes: true });
    window.addEventListener('scroll', scroll, { capture: true, passive: true });
    window.addEventListener('resize', update, { passive: true });
    document.fonts.addEventListener('loadingdone', schedule);
    document.addEventListener('selectionchange', selection);
    return () => {
      cancelAnimationFrame(frame);
      hop?.cancel();
      controller?.abort();
      resize.disconnect();
      mutations.disconnect();
      window.removeEventListener('scroll', scroll, true);
      window.removeEventListener('resize', update);
      document.fonts.removeEventListener('loadingdone', schedule);
      document.removeEventListener('selectionchange', selection);
      motion.removeEventListener('change', updateMotion);
    };
  });

  async function open() {
    if (selected) { phrase = selected; answer = ''; selected = ''; }
    error = '';
    auth = 'checking';
    await tick();
    dialog?.showModal();
    try { const me = await api.me(); auth = me.user.tier === 'free' ? 'upgrade' : 'signed-in'; }
    catch { auth = 'signed-out'; }
  }

  function close() { controller?.abort(); dialog?.close(); }

  async function ask(event: SubmitEvent) {
    event.preventDefault();
    if (busy || auth !== 'signed-in') return;
    busy = true;
    error = '';
    answer = '';
    controller = new AbortController();
    try {
      const result = await api.assistant({ mode, phrase, language, consent }, controller.signal);
      answer = result.answer;
    } catch (e) {
      if (!controller.signal.aborted) error = e instanceof ApiError ? e.message : 'Could not connect. Please try again.';
    } finally { busy = false; }
  }
</script>

<button class="mascot" style="left:{x}px;top:{y}px;visibility:{visible ? 'visible' : 'hidden'}" onclick={open}
  aria-label={`Ask Sunny, your reading companion. ${positionLabel ? positionLabel + '. ' : ''}${Math.round(percent)}% through ${progress === undefined ? 'this page' : 'this book'}.`}
  title={`Ask Sunny · ${positionLabel ?? Math.round(percent) + '% page progress'}`}>
  <svg bind:this={icon} width="32" height="32" viewBox="0 0 64 64" aria-hidden="true">
    <circle cx="32" cy="32" r="30" fill="#facc15" />
    <ellipse cx="23" cy="26" rx="3.5" ry="6" fill="#111" />
    <ellipse cx="41" cy="26" rx="3.5" ry="6" fill="#111" />
    <path d="M22 38H42A10 10 0 0 1 22 38Z" fill="#111" />
  </svg>
</button>

<dialog bind:this={dialog} aria-labelledby="sunny-title" onclose={() => { controller?.abort(); }}>
  <header><h2 id="sunny-title">Ask Sunny</h2><button class="close" type="button" onclick={close} aria-label="Close reading companion">×</button></header>
  <p class="intro">Select a phrase, then tap Sunny—or paste one below.</p>
  {#if auth === 'checking'}<p role="status">Checking sign-in…</p>{/if}
  {#if auth === 'signed-out'}<p class="sign-in"><a href="/login">Sign in</a> with Plus or Pro to explain or translate phrases.</p>{/if}
  {#if auth === 'upgrade'}<p class="sign-in">AI explanations and translations are a paid feature. <a href="/app/upgrade">See Plus and Pro</a>.</p>{/if}
  <form onsubmit={ask}>
    <label for="sunny-phrase">Phrase</label>
    <textarea id="sunny-phrase" bind:value={phrase} maxlength="1200" rows="3" required placeholder="For example: c’est la vie" />
    <div class="options">
      <div><label for="sunny-mode">Help me</label><select id="sunny-mode" bind:value={mode}><option value="explain">Explain this phrase</option><option value="translate">Translate this phrase</option></select></div>
      <div><label for="sunny-language">Answer in</label><select id="sunny-language" bind:value={language}>{#each ['English', 'Romanian', 'French', 'German', 'Spanish', 'Italian', 'Portuguese', 'Dutch', 'Polish', 'Japanese', 'Chinese', 'Arabic'] as name}<option>{name}</option>{/each}</select></div>
    </div>
    <label class="consent"><input type="checkbox" bind:checked={consent} required /> <span>Send this phrase to OpenRouter / Google Gemini. Only the text above and your chosen language are sent, not the whole book.</span></label>
    <button class="submit" type="submit" disabled={busy || auth !== 'signed-in' || !consent || !phrase.trim()}>{busy ? 'Thinking…' : mode === 'explain' ? 'Explain phrase' : 'Translate phrase'}</button>
  </form>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="answer" role="status" aria-live="polite">{answer}</div>
  <p class="note">Gemini 3.8 Flash · Plus and Pro only · AI can make mistakes. 30 requests per reader/day, with a shared daily limit. Requests—including failed attempts—count. Please don’t send sensitive information.</p>
</dialog>

<style>
  .mascot { position: fixed; width: 32px; height: 32px; padding: 0; border: 0; border-radius: 50%; background: transparent; z-index: 36; transition: top 280ms ease, left 180ms ease; touch-action: manipulation; }
  .mascot::before { content: ''; position: absolute; inset: -6px; border-radius: 50%; }
  .mascot svg { display: block; filter: drop-shadow(0 3px 3px rgba(0,0,0,0.1)); }
  .mascot:hover svg { filter: drop-shadow(0 3px 5px rgba(0,0,0,0.18)); }
  dialog { width: min(460px, calc(100vw - 32px)); max-height: calc(100dvh - 48px); padding: 24px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--bg); color: var(--text); overflow-y: auto; box-shadow: var(--shadow-lift); }
  dialog::backdrop { background: rgba(17,24,39,0.35); }
  header { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
  h2 { font-size: 1.4rem; margin: 0; }
  .close { width: 36px; height: 36px; border: none; border-radius: 50%; background: var(--bg-raised); color: var(--text); font-size: 1.5rem; }
  .intro, .sign-in { font-size: 0.86rem; line-height: 1.6; color: var(--text-dim); }
  .sign-in a { color: var(--accent); text-decoration: underline; }
  label { display: block; margin-bottom: 6px; font-size: 0.82rem; font-weight: 500; }
  textarea, select { width: 100%; padding: 10px; border: 1px solid var(--border-bright); border-radius: var(--radius-sm); background: var(--bg); color: var(--text); font-size: 0.88rem; }
  textarea { resize: vertical; }
  .options { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 14px; }
  .options > div { min-width: 0; }
  .consent { display: flex; align-items: flex-start; gap: 8px; margin: 16px 0; font-size: 0.76rem; line-height: 1.6; color: var(--text-dim); }
  .consent input { margin-top: 4px; flex-shrink: 0; }
  .submit { width: 100%; padding: 12px; border: 0; border-radius: var(--radius-pill); background: var(--accent); color: white; font-weight: 600; }
  .submit:disabled { opacity: 0.55; cursor: default; }
  .answer { margin-top: 18px; white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.7; font-size: 0.9rem; }
  .answer:empty { display: none; }
  .error { color: #be123c; font-size: 0.85rem; line-height: 1.5; }
  .note { font-size: 0.7rem; color: var(--text-faint); line-height: 1.6; margin-bottom: 0; }
  @media (prefers-reduced-motion: reduce) { .mascot { transition: none; } }
</style>
