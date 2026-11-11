<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { api, ApiError, type PuriResult } from '../lib/api';
  import { renderMarkdown } from '../lib/markdown';
  import { askBYOK, guestBYOKOwner, loadBYOK, providers } from '../lib/byok';

  import BYOKSettings from './BYOKSettings.svelte';
  import PuriCoach from './PuriCoach.svelte';
  let { progress, positionLabel, cardIndex, currentCardId = '', currentCardSource = '', experience = 'live', demoWord = '', demoDefinition = '', demoTranslation = false, demoTranslated = false, onTranslateCard }: {
    progress?: number; positionLabel?: string; cardIndex?: number; currentCardId?: string; currentCardSource?: string;
    experience?: 'live' | 'facts' | 'demo'; demoWord?: string; demoDefinition?: string;
    demoTranslation?: boolean; demoTranslated?: boolean;
    onTranslateCard?: (language: string, signal?: AbortSignal, byok?: boolean) => Promise<void>;
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
    controller?.abort();
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
  let mode = $state<'explain' | 'translate' | 'card_translate'>('explain');
  $effect(() => {
    if (mode === 'card_translate' && !onTranslateCard) mode = 'explain';
    if (mode === 'card_translate' && !['French','Spanish','German'].includes(language)) language = 'French';
  });
  let language = $state('English');
  let answer = $state('');
  let error = $state('');
  let busy = $state(false);
  let auth = $state<'checking' | 'signed-in' | 'signed-out'>('checking');
  let controller: AbortController | null = null;
  let funding = $state<'paid' | 'byok'>('byok');
  let paid = $state(false);
  let fundingInitialized = false;
  let keyReady = $state(false);
  let modelLabel = $state('');
  let viewportTop = $state(0);
  let viewportLeft = $state(0);
  let visualWidth = $state(390);
  let visualHeight = $state(800);
  let savedResults = $state<PuriResult[]>([]);
  function refreshGuestKey() {
    const config = loadBYOK(guestBYOKOwner).config;
    keyReady = !!config;
    modelLabel = config ? `${providers[config.provider].name} · ${config.model}` : '';
  }
  let quota = $state<{used:number;limit:number;resets_at:string} | null>(null);
  async function refreshBYOKQuota() { try { quota = await api.byokQuota(); } catch { quota = null; } }
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
      viewportTop = window.visualViewport?.offsetTop ?? 0;
      viewportLeft = window.visualViewport?.offsetLeft ?? 0;
      visualWidth = window.visualViewport?.width ?? viewportWidth;
      visualHeight = window.visualViewport?.height ?? viewportHeight;
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
    window.visualViewport?.addEventListener('resize', update);
    window.visualViewport?.addEventListener('scroll', update);
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
      window.visualViewport?.removeEventListener('resize', update);
      window.visualViewport?.removeEventListener('scroll', update);
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
    keyReady = false; paid = false; quota = null; savedResults = [];
    await tick();
    dialog?.showModal();
    try {
      const me = await api.me();
      paid = me.user.tier !== 'free';
      const config = loadBYOK(me.user.id ?? me.user.handle).config;
      keyReady = !!config;
      modelLabel = config ? `${providers[config.provider].name} · ${config.model}` : '';
      if (!fundingInitialized) { funding = paid ? 'paid' : 'byok'; fundingInitialized = true; }
      else if (!paid) funding = 'byok';
      auth = 'signed-in';
      await refreshBYOKQuota();
      if (currentCardId) {
        try { savedResults = (await api.puriResults(currentCardId)).results.filter(r => r.mode !== 'card_translate'); }
        catch { /* Saving remains available if history could not load. */ }
      }
    }
    catch (e) {
      if (e instanceof ApiError && e.status === 401) { auth = 'signed-out'; funding = 'byok'; refreshGuestKey(); }
      else { error = 'Could not check your account. Please try again.'; }
    }
  }

  function close() { demoDismissed = true; coaching = false; controller?.abort(); dialog?.close(); }

  async function ask(event: SubmitEvent) {
    event.preventDefault();
    if (experience !== 'live' || busy || auth === 'checking') return;
    if (funding === 'byok' && (!keyReady || (auth === 'signed-in' && (!quota || quota.used >= quota.limit)))) return;
    if (funding === 'paid' && !paid) return;
    if (mode !== 'card_translate' && (!phrase || !phraseCardId)) return;
    busy = true;
    error = '';
    answer = '';
    controller = new AbortController();
    try {
      if (mode === 'card_translate') {
        if (!onTranslateCard) { error = 'Choose a text card to translate.'; return; }
        await onTranslateCard(language, controller.signal, funding === 'byok');
        close();
        return;
      }
      const result = funding === 'byok'
        ? await askBYOK({mode,phrase,language}, controller.signal)
        : await api.assistant({ mode, phrase, card_id: phraseCardId, language, consent: true }, controller.signal);
      answer = result.answer;
      if (funding === 'byok' && auth === 'signed-in') {
        try { await api.savePuriResult(phraseCardId,{mode,phrase,language,source:currentCardSource,content:result.answer,model:result.model}); }
        catch { error = 'Your answer is shown below, but could not be saved. Keep a copy before leaving.'; }
      }
    } catch (e) {
      if (!controller.signal.aborted) error = e instanceof Error ? e.message : 'Could not connect. Please try again.';
    } finally { busy = false; if (funding === 'byok' && auth === 'signed-in') await refreshBYOKQuota(); }
  }
</script>

<svelte:element this={experience === 'facts' ? 'span' : 'button'} class="mascot" class:facts={experience === 'facts'}
  style="left:{x}px;top:{y}px;visibility:{visible && !landingScrolling ? 'visible' : 'hidden'}" onclick={experience === 'facts' ? undefined : open}
  onpointerdown={(event) => { if (experience === 'live') event.preventDefault(); }}
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
<dialog bind:this={dialog}
  style="--viewport-top:{viewportTop}px;--viewport-left:{viewportLeft}px;--viewport-width:{visualWidth}px;--viewport-height:{visualHeight}px"
  aria-labelledby="sunny-title" onclose={() => { controller?.abort(); }}
  onclick={(event) => { if (event.target === dialog && dialog) { const r = dialog.getBoundingClientRect(); if (event.clientX < r.left || event.clientX > r.right || event.clientY < r.top || event.clientY > r.bottom) close(); } }}>
  <header><h2 id="sunny-title">Ask Puri</h2><button class="close" type="button" onclick={close} aria-label="Close reading companion">×</button></header>
  <div class="bubble-content">
  <p class="intro">Explain a selected phrase, or translate this reading card.</p>
  {#if auth === 'checking'}<p role="status">Checking sign-in…</p>{/if}
  {#if auth === 'signed-out'}
    <p class="sign-in">Use your own API key without an account. Your provider bills usage. Explanations and translations are not saved—<a href="/login">sign in</a> to keep future results.</p>
    <BYOKSettings userId={guestBYOKOwner} onSaved={refreshGuestKey} />
    {#if keyReady}<p class="intro">{modelLabel}</p>{/if}
  {/if}
  {#if auth === 'signed-in'}
    <label for="ai-funding">AI access</label>
    <select id="ai-funding" bind:value={funding} disabled={busy}>
      <option value="byok">Free BYOK · your API credits</option>
      {#if paid}<option value="paid">Membership · included AI</option>{/if}
    </select>
    {#if funding === 'byok'}
      {#if keyReady}<p class="intro">{modelLabel} · {quota ? `${quota.used} / ${quota.limit} this month` : 'Checking allowance…'}</p>{/if}
      <p class="sign-in"><a href="/app/me#byok">{keyReady ? 'Change AI settings' : 'Set up your AI key on You'}</a></p>
      {#if !paid && !keyReady}<p class="sign-in">Or <a href="/app/upgrade">use included AI with Plus</a>.</p>{/if}
    {/if}
  {/if}
  <form onsubmit={ask}>
    {#if mode !== 'card_translate'}
      {#if phrase}<blockquote class="selected-phrase">{phrase}</blockquote>
      {:else}<p class="intro">Highlight a word or passage on the card, then click Puri.</p>{/if}
    {:else}<p class="intro">{auth === 'signed-out' ? 'Translate the original current card. Guest results are not saved.' : 'Translate the original current card. Results are saved privately to your account, including BYOK translations.'}</p>{/if}
    <div class="options">
      <div><label for="sunny-mode">Help me</label><select id="sunny-mode" bind:value={mode}><option value="explain">Explain this phrase</option>{#if onTranslateCard}<option value="card_translate">Translate current card</option>{/if}<option value="translate">Translate this phrase</option></select></div>
      <div><label for="sunny-language">Answer in</label><select id="sunny-language" bind:value={language}>{#each (mode === 'card_translate' ? ['French', 'Spanish', 'German'] : ['English', 'Romanian', 'French', 'German', 'Spanish', 'Italian', 'Portuguese', 'Dutch', 'Polish', 'Japanese', 'Chinese', 'Arabic']) as name}<option>{name}</option>{/each}</select></div>
    </div>
    <button class="submit" type="submit" disabled={busy || auth === 'checking' || (funding === 'byok' && (!keyReady || (auth === 'signed-in' && (!quota || quota.used >= quota.limit)))) || (mode !== 'card_translate' && (!phrase.trim() || !phraseCardId))}>{busy ? 'Thinking…' : mode === 'card_translate' ? 'Translate card' : mode === 'explain' ? 'Explain phrase' : 'Translate phrase'}</button>
  </form>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if savedResults.length}<details><summary>Saved explanations and phrase translations</summary>{#each savedResults as result}<button type="button" class="saved-result" onclick={() => answer = result.content}>{result.mode === 'explain' ? 'Explain' : 'Translate'}: {result.phrase} · {result.language}</button>{/each}</details>{/if}
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
  dialog { position: fixed; inset: auto; margin: 0; left: calc(var(--viewport-left) + (var(--viewport-width) - min(360px, var(--viewport-width) - 32px)) / 2); top: calc(var(--viewport-top) + max(env(safe-area-inset-top, 0px), var(--reader-safe-top, 0px)) + 12px); width: min(360px, calc(var(--viewport-width) - 32px)); max-height: calc(var(--viewport-height) - max(env(safe-area-inset-top, 0px), var(--reader-safe-top, 0px)) - env(safe-area-inset-bottom, 0px) - 24px); padding: 0; border: 1px solid #75cfc5; border-radius: 18px; background: #b8eee8; color: #164e4a; overflow: hidden; font-size: .85rem; }
  dialog[open] { display: flex; flex-direction: column; }
  dialog::backdrop { background: #0003; }
  dialog > header { flex: none; padding: 6px 10px 6px 16px; }
  .bubble-content { min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 0 16px 16px; }
  .saved-result { display: block; margin: 8px 0; text-align: left; background: #fff; border: 1px solid #75cfc5; border-radius: 8px; padding: 8px; color: #164e4a; }
  .selected-phrase { margin: 10px 0; padding: 10px; border-radius: 10px; background: #fff; overflow-wrap: anywhere; }
  header { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
  h2 { font-size: 1rem; margin: 0; }
  .close { width: 44px; height: 44px; flex-shrink: 0; padding: 0; border: none; border-radius: 0; background: transparent; color: #164e4a; font-size: 1.1rem; box-shadow: none; cursor: pointer; }
  .close:focus { outline: none; }
  .close:focus-visible { outline: none; color: #0f766e; background: #ccfbf1; border-radius: 2px; }
  .intro, .sign-in { font-size: 0.86rem; line-height: 1.6; color: var(--text-dim); }
  .sign-in a { color: #164e4a; text-decoration: underline; }
  label { display: block; margin-bottom: 6px; font-size: 0.82rem; font-weight: 500; }
  select { width: 100%; padding: 10px; border: 1px solid #75cfc5; border-radius: var(--radius-sm); background: #fff; color: #164e4a; font-size: 0.88rem; }
  .options { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 14px; }
  .options > div { min-width: 0; }
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
