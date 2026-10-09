<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { renderMarkdown } from '../lib/markdown';

  import PuriCoach from './PuriCoach.svelte';
  let { progress, positionLabel, cardIndex, experience = 'live', demoWord = '', demoDefinition = '', demoTranslation = false, demoTranslated = false, onTranslateCard }: {
    progress?: number; positionLabel?: string; cardIndex?: number;
    experience?: 'live' | 'facts' | 'demo'; demoWord?: string; demoDefinition?: string;
    demoTranslation?: boolean; demoTranslated?: boolean;
    onTranslateCard?: (language: string, signal?: AbortSignal) => Promise<void>;
  } = $props();
  let coaching = $state(false);
  let demoDismissed = $state(false);
  $effect(() => { cardIndex; demoTranslated; demoDismissed = false; });
  $effect(() => {
    if (experience !== 'demo' || !settled || demoDismissed) return;
    const timer = setTimeout(() => { demoDismissed = true; coaching = false; }, 2000);
    return () => clearTimeout(timer);
  });
  $effect(() => {
    cardIndex; demoWord;
    selected = ''; selectedCardId = ''; phrase = ''; phraseCardId = ''; answer = ''; coaching = false;
    dialog?.close();
  });
  let documentProgress = $state(0);
  let x = $state(0);
  let y = $state(0);
  let viewportWidth = $state(390);
  let settled = $state(false);
  let hopRevision = $state(0);
  let landingScrolling = $state(false);
  const coachLeft = $derived(Math.max(16, Math.min(x - 278, viewportWidth - 326)));
  const below = $derived(y < 200);
  $effect(() => {
    x; y; cardIndex; demoWord; demoTranslated; hopRevision;
    settled = false;
    if (!visible || experience === 'live' || landingScrolling) return;
    // The hop lasts 450ms; position transitions last at most 280ms.
    // Restart after every layout/position change so stale hints never flash.
    const timer = setTimeout(() => settled = true, reducedMotion ? 0 : 550);
    return () => clearTimeout(timer);
  });
  let visible = $state(false);
  let remeasure = () => {};
  $effect(() => {
    progress; cardIndex;
    const frame = requestAnimationFrame(() => { if (experience !== 'facts') jump(); remeasure(); });
    return () => cancelAnimationFrame(frame);
  });
  let reducedMotion = $state(false);
  let icon = $state<HTMLSpanElement | null>(null);
  let dialog = $state<HTMLDialogElement | null>(null);
  let selected = $state('');
  let phrase = $state('');
  let selectedCardId = $state('');
  let phraseCardId = $state('');
  let viewportHeight = $state(800);
  const liveBelow = $derived(y < viewportHeight / 2);
  let mode = $state<'explain' | 'translate' | 'card_translate'>('explain');
  $effect(() => {
    if (mode === 'card_translate' && !onTranslateCard) mode = 'explain';
    if (mode === 'card_translate' && !['French','Spanish','German'].includes(language)) language = 'French';
  });
  let language = $state('English');
  let answer = $state('');
  let error = $state('');
  let busy = $state(false);
  let auth = $state<'checking' | 'signed-in' | 'signed-out' | 'upgrade'>('checking');
  let controller: AbortController | null = null;
  let lastHop = 0;
  let hop: Animation | null = null;
  const percent = $derived(Math.max(0, Math.min(100, progress ?? documentProgress)));

  function jump() {
    if (reducedMotion || !icon || (experience !== 'facts' && performance.now() - lastHop < 450)) return;
    lastHop = performance.now();
    hopRevision++;
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
    let scrollTimer: ReturnType<typeof setTimeout> | null = null;
    function update() {
      // Font/resize observers must not bypass the landing scroll debounce.
      if (experience === 'facts' && scrollTimer !== null) return;
      viewportWidth = window.innerWidth;
      viewportHeight = window.innerHeight;
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
    function scroll() {
      if (experience !== 'facts') { jump(); schedule(); return; }
      if (scrollTimer !== null) clearTimeout(scrollTimer);
      landingScrolling = true;
      settled = false;
      hop?.cancel();
      scrollTimer = setTimeout(() => {
        scrollTimer = null;
        landingScrolling = false;
        update(); // Measure the final anchor, not an intermediate scroll position.
        jump();
      }, 180);
    }
    function selection() {
      const sel = window.getSelection();
      if (!sel || sel.isCollapsed || !sel.rangeCount) { selected = ''; selectedCardId = ''; return; }
      const node = sel.getRangeAt(0).commonAncestorContainer;
      const el = node instanceof Element ? node : node.parentElement;
      if (el?.closest('dialog')) return;
      const body = el?.closest('[data-body]');
      if (experience === 'live' && !body) return;
      if (progress !== undefined && !body) return;
      selected = sel.toString().trim().slice(0, 1200);
      selectedCardId = body?.closest('[data-card-id]')?.getAttribute('data-card-id') ?? '';
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
      if (scrollTimer !== null) clearTimeout(scrollTimer);
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
    if (experience !== 'live') { coaching = true; demoDismissed = false; return; }
    phrase = selected;
    phraseCardId = selectedCardId;
    answer = '';
    selected = '';
    error = '';
    auth = 'checking';
    await tick();
    dialog?.show();
    try { const me = await api.me(); auth = me.user.tier === 'free' ? 'upgrade' : 'signed-in'; }
    catch { auth = 'signed-out'; }
  }

  function close() { demoDismissed = true; coaching = false; controller?.abort(); dialog?.close(); }

  async function ask(event: SubmitEvent) {
    event.preventDefault();
    if (experience !== 'live' || busy || auth !== 'signed-in') return;
    if (mode !== 'card_translate' && (!phrase || !phraseCardId)) return;
    busy = true;
    error = '';
    answer = '';
    controller = new AbortController();
    try {
      if (mode === 'card_translate') {
        if (!onTranslateCard) { error = 'Choose a text card to translate.'; return; }
        await onTranslateCard(language, controller.signal);
        close();
        return;
      }
      const result = await api.assistant({ mode, phrase, card_id: phraseCardId, language, consent: true }, controller.signal);
      answer = result.answer;
    } catch (e) {
      if (!controller.signal.aborted) error = e instanceof ApiError ? e.message : 'Could not connect. Please try again.';
    } finally { busy = false; }
  }
</script>

<svelte:element this={experience === 'facts' ? 'span' : 'button'} class="mascot" class:facts={experience === 'facts'}
  style="left:{x}px;top:{y}px;visibility:{visible && !landingScrolling ? 'visible' : 'hidden'}" onclick={experience === 'facts' ? undefined : open}
  role={experience === 'facts' ? 'img' : undefined}
  aria-label={experience === 'facts' ? 'Puri, your reading companion' : experience === 'demo' ? 'Click Puri' : `Ask Puri, your reading companion. ${positionLabel ? positionLabel + '. ' : ''}${Math.round(percent)}% through ${progress === undefined ? 'this page' : 'this book'}.`}
  title={experience === 'live' ? `Ask Puri · ${positionLabel ?? Math.round(percent) + '% page progress'}` : 'Puri'}>
  <span class="glim" bind:this={icon} aria-hidden="true">
    <img class="eyes-open" src="/glim.svg" width="32" height="32" alt="" draggable="false" />
    <img class="eyes-half" src="/glim_blink.svg" width="32" height="32" alt="" draggable="false" />
    <img class="eyes-closed" src="/glim_blink_two.svg" width="32" height="32" alt="" draggable="false" />
  </span>
</svelte:element>

{#if experience === 'demo' && !demoDismissed && (cardIndex === 1 || cardIndex === 2)}
  <div class="coach" class:below
    style="left:{coachLeft}px;top:{below ? y + 44 : y - 12}px;--tail-x:{x + 16 - coachLeft}px;visibility:{visible && settled ? 'visible' : 'hidden'}">
    <PuriCoach onClose={close} word={demoWord} definition={demoDefinition} selection={selected} expanded={coaching}
      translation={demoTranslation} translated={demoTranslated} onTranslate={() => onTranslateCard?.('French') ?? Promise.resolve()} />
  </div>
{/if}

<svelte:window onkeydown={(event) => { if (event.key === 'Escape') close(); }} />
<dialog bind:this={dialog} class:live-below={liveBelow}
  style="left:{coachLeft}px;top:{liveBelow ? y + 44 : y - 12}px;--tail-x:{x + 16 - coachLeft}px;--bubble-height:{Math.max(120, liveBelow ? viewportHeight - y - 64 : y - 28)}px"
  aria-labelledby="sunny-title" onclose={() => { controller?.abort(); }}>
  <div class="bubble-content">
  <header><h2 id="sunny-title">Ask Puri</h2><button class="close" type="button" onclick={close} aria-label="Close reading companion">×</button></header>
  <p class="intro">Explain a selected phrase, or translate this reading card and save it for later.</p>
  {#if auth === 'checking'}<p role="status">Checking sign-in…</p>{/if}
  {#if auth === 'signed-out'}<p class="sign-in"><a href="/login">Sign in</a> with Plus to explain or translate phrases.</p>{/if}
  {#if auth === 'upgrade'}<p class="sign-in">AI explanations and translations are a paid feature. <a href="/app/upgrade">Subscribe to Plus</a>.</p>{/if}
  <form onsubmit={ask}>
    {#if mode !== 'card_translate'}
      {#if phrase}<blockquote class="selected-phrase">{phrase}</blockquote>
      {:else}<p class="intro">Highlight a word or passage on the card, then click Puri.</p>{/if}
    {:else}<p class="intro">Translate the entire current card. Saved translations do not use another request.</p>{/if}
    <div class="options">
      <div><label for="sunny-mode">Help me</label><select id="sunny-mode" bind:value={mode}><option value="explain">Explain this phrase</option>{#if onTranslateCard}<option value="card_translate">Translate current card</option>{/if}<option value="translate">Translate this phrase</option></select></div>
      <div><label for="sunny-language">Answer in</label><select id="sunny-language" bind:value={language}>{#each (mode === 'card_translate' ? ['French', 'Spanish', 'German'] : ['English', 'Romanian', 'French', 'German', 'Spanish', 'Italian', 'Portuguese', 'Dutch', 'Polish', 'Japanese', 'Chinese', 'Arabic']) as name}<option>{name}</option>{/each}</select></div>
    </div>
    <button class="submit" type="submit" disabled={busy || auth !== 'signed-in' || (mode !== 'card_translate' && (!phrase.trim() || !phraseCardId))}>{busy ? 'Thinking…' : mode === 'card_translate' ? 'Translate card' : mode === 'explain' ? 'Explain phrase' : 'Translate phrase'}</button>
  </form>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="answer" role="status" aria-live="polite">{@html renderMarkdown(answer)}</div>
  </div>
</dialog>

<style>
  .mascot { position: fixed; width: 32px; height: 32px; padding: 0; border: 0; border-radius: 50%; background: transparent; z-index: 36; transition: top 280ms ease, left 180ms ease; touch-action: manipulation; }
  .mascot.facts { transition: none; }
  .coach { position: fixed; width: min(310px, calc(100vw - 32px)); transform: translateY(-100%); z-index: 37; }
  .coach.below { transform: none; --tail-top: -7px; --tail-bottom: auto; --tail-rotation: 225deg; }
  .facts { pointer-events: none; }
  .mascot::before { content: ''; position: absolute; inset: -6px; border-radius: 50%; }
  .glim { position: relative; display: block; width: 32px; height: 32px; filter: drop-shadow(0 3px 3px rgba(0,0,0,0.1)); }
  .mascot:hover .glim { filter: drop-shadow(0 3px 5px rgba(0,0,0,0.18)); }
  .glim img { position: absolute; inset: 0; width: 32px; height: 32px; pointer-events: none; }
  .eyes-open { animation: eyes-open 5s step-end infinite; }
  .eyes-half { opacity: 0; animation: eyes-half 5s step-end infinite; }
  .eyes-closed { opacity: 0; animation: eyes-closed 5s step-end infinite; }
  @keyframes eyes-open { 0%, 98%, 100% { opacity: 1; } 94% { opacity: 0; } }
  @keyframes eyes-half { 0%, 95%, 98%, 100% { opacity: 0; } 94%, 97% { opacity: 1; } }
  @keyframes eyes-closed { 0%, 97%, 100% { opacity: 0; } 95% { opacity: 1; } }
  dialog { position: fixed; inset: auto; margin: 0; width: min(310px, calc(100vw - 32px)); max-height: var(--bubble-height); padding: 12px 16px; border: 1px solid #75cfc5; border-radius: 18px 18px 4px 18px; background: #b8eee8; color: #164e4a; overflow: visible; transform: translateY(-100%); z-index: 37; font-size: .85rem; }
  dialog.live-below { transform: none; }
  dialog::after { content: ''; position: absolute; left: clamp(12px, var(--tail-x, 90%), calc(100% - 20px)); bottom: -7px; width: 12px; height: 12px; background: #b8eee8; border-right: 1px solid #75cfc5; border-bottom: 1px solid #75cfc5; transform: translateX(-50%) rotate(45deg); }
  dialog.live-below::after { top: -7px; bottom: auto; transform: translateX(-50%) rotate(225deg); }
  .bubble-content { max-height: calc(var(--bubble-height) - 26px); overflow-y: auto; }
  .selected-phrase { margin: 10px 0; padding: 10px; border-radius: 10px; background: #fff; overflow-wrap: anywhere; }
  header { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
  h2 { font-size: 1rem; margin: 0; }
  .close { width: 22px; height: 22px; padding: 0; border: none; border-radius: 0; background: transparent; color: #164e4a; font-size: 1.1rem; box-shadow: none; cursor: pointer; }
  .close:focus { outline: none; }
  .close:focus-visible { outline: none; color: #0f766e; background: #ccfbf1; border-radius: 2px; }
  .intro, .sign-in { font-size: 0.86rem; line-height: 1.6; color: var(--text-dim); }
  .sign-in a { color: #164e4a; text-decoration: underline; }
  label { display: block; margin-bottom: 6px; font-size: 0.82rem; font-weight: 500; }
  textarea, select { width: 100%; padding: 10px; border: 1px solid #75cfc5; border-radius: var(--radius-sm); background: #fff; color: #164e4a; font-size: 0.88rem; }
  textarea { resize: vertical; }
  .options { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 14px; }
  .options > div { min-width: 0; }
  .consent { display: flex; align-items: flex-start; gap: 8px; margin: 16px 0; font-size: 0.76rem; line-height: 1.6; color: var(--text-dim); }
  .consent input { margin-top: 4px; flex-shrink: 0; }
  .submit { display: block; width: auto; padding: 7px 16px; border: 1px solid #519d94; border-radius: var(--radius-pill); background: #fff; color: #164e4a; font-weight: 600; cursor: pointer; margin-top: 12px; }
  .submit:disabled { opacity: 0.55; cursor: default; }
  .answer { margin-top: 18px; padding: 14px 16px; border: 1px solid #75cfc5; border-radius: 12px; background: #fff; color: #164e4a; overflow-wrap: anywhere; line-height: 1.7; font-size: 0.9rem; }
  .answer :global(p) { margin: 0 0 10px; }
  .answer :global(> :last-child) { margin-bottom: 0; }
  .answer :global(h1), .answer :global(h2), .answer :global(h3) { font-size: 1.05rem; margin: 14px 0 8px; }
  .answer :global(ul), .answer :global(ol) { padding-left: 22px; }
  .answer :global(pre) { overflow-x: auto; padding: 10px; background: #f0fdfa; border-radius: 8px; }
  .answer :global(a) { color: #0f766e; text-decoration: underline; }
  .answer :global(blockquote) { margin-left: 0; padding-left: 12px; border-left: 3px solid #75cfc5; }
  .answer:empty { display: none; }
  .error { color: #be123c; font-size: 0.85rem; line-height: 1.5; }
  .note { font-size: 0.7rem; color: var(--text-faint); line-height: 1.6; margin-bottom: 0; }
  @media (prefers-reduced-motion: reduce) {
    .mascot { transition: none; }
    .glim img { animation: none; }
  }
</style>
