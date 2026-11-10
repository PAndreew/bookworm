<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import { Heart, Highlighter, Share2, ChevronLeft, ChevronRight, Flag, Type, Flame, Lock } from '@lucide/svelte';
  import { api, ApiError, type Page, type Translation } from '../lib/api';
  import ReadingCompanion from './ReadingCompanion.svelte';
  import { askBYOK } from '../lib/byok';
  import { fitTextCard, type Fragment } from '../lib/card-fit';

  import { guestPages, guestTranslation, type GuestBook } from '../lib/guest-books';
  let { slug, title, author, preview }: { slug: string; title: string; author: string; preview?: GuestBook } = $props();
  let previewFinished = $state(false);
  let pages = $state<Page[]>([]);
  let pageCount = $state(0);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let blocked = $state<{ message: string; resetsAt: string | null } | null>(null);
  let gated = $state<string | null>(null);
  let activeIdx = $state(0);
  let streak = $state(0);
  let streakPulse = $state(false);
  let fontScale = $state(1);
  let showType = $state(false);
  let toast = $state<string | null>(null);
  let fragments = $state<Fragment[]>([]);
  let fragmentIdx = $state(0);
  let fittedCard = $state('');
  let navigating = $state(false);
  let enterFromEnd = false;
  let reportDialog = $state<HTMLDialogElement | null>(null);
  let reportPage = $state<Page | null>(null);
  let reportCategory = $state<'layout' | 'text' | 'break'>('layout');
  let reportNote = $state('');
  let reportError = $state('');
  let reportBusy = $state(false);
  let touchStart: { x: number; y: number } | null = null;
  let documentVisible = $state(true);
  const DWELL_MS = 1800;
  const recorded = new Set<string>();
  let completionDialog = $state<HTMLDialogElement | null>(null);
  let completionOpen = $state(false);
  let completion = $state<{ total_cards: number; read_cards: number; missing_card_indices: number[] } | null>(null);
  let completionLoading = $state(false);
  let completionError = $state('');
  let missingLimit = $state(50);
  async function refreshCompletion() {
    if (completionLoading) return;
    completionLoading = true; completionError = '';
    try { completion = (await api.bookPosition(slug)).completion; }
    catch { completionError = 'Could not load reading completion. Please try again.'; }
    finally { completionLoading = false; }
  }
  function openCompletion() {
    completionOpen = true; missingLimit = 50;
    completionDialog?.showModal();
    void refreshCompletion();
  }
  async function jumpToMissing(idx: number) {
    completionDialog?.close();
    if (navigating || blocked) return;
    navigating = true;
    try {
      if (!pages.some(p => p.idx === idx)) {
        const res = await api.cards(slug, idx, 12);
        pages = [...pages, ...res.pages.filter(p => !pages.some(old => old.idx === p.idx))].sort((a, b) => a.idx - b.idx);
      }
      if (pages.some(p => p.idx === idx)) { enterFromEnd = false; activeIdx = idx; fragmentIdx = 0; fragments = []; fittedCard = ''; }
    } catch { showToast('Could not load that card. Try again.'); }
    finally { navigating = false; }
  }
  let canSavePosition = $state(false);
  let pendingPosition: string | null = null;
  let savingPosition = false;
  async function savePosition(cardID: string) {
    if (preview) return;
    pendingPosition = cardID;
    if (savingPosition) return;
    savingPosition = true;
    try {
      while (pendingPosition) {
        const next = pendingPosition;
        pendingPosition = null;
        await api.saveBookPosition(slug, next);
      }
    } catch { showToast('Your position could not be saved.'); }
    finally { savingPosition = false; }
  }
  $effect(() => {
    const cardID = activePage?.card_id;
    if (canSavePosition && cardID && !loading && documentVisible) void savePosition(cardID);
  });
  const activePage = $derived(pages.find(p => p.idx === activeIdx));
  let translationPaid = $state(false);
  let translations = $state<Record<string, Translation[]>>({});
  let preferredLanguage = $state('Original');
  const loadedTranslations = new Set<string>();
  const activeTranslation = $derived((translations[activePage?.card_id ?? ''] ?? []).find(t => t.language === preferredLanguage));
  const renderKey = $derived((activePage?.card_id ?? '') + (activeTranslation ? ':' + activeTranslation.language : ''));
  const displayPage = $derived(activePage ? { ...activePage, card_id: renderKey, content: activeTranslation?.content ?? activePage.content } : undefined);
  const fragment = $derived(fragments[fragmentIdx]);
  const displayedText = $derived(displayPage && fragment && fittedCard === renderKey
    ? Array.from(displayPage.content).slice(fragment.start, fragment.end).join('') : displayPage?.content ?? '');
  $effect(() => {
    const id = activePage?.card_id;
    if (preview || !translationPaid || activePage?.kind !== 'text' || !id || loadedTranslations.has(id)) return;
    loadedTranslations.add(id);
    api.cardTranslations(id).then(result => translations[id] = [...result.translations, ...(translations[id] ?? []).filter(t => t.model.startsWith('BYOK · '))]).catch(() => loadedTranslations.delete(id));
  });
  async function translateCurrentCard(language: string, signal?: AbortSignal, byok = false) {
    if (language !== 'French' && language !== 'Spanish' && language !== 'German') throw new Error('Choose French, Spanish or German.');
    const id = activePage?.card_id;
    if (!id || activePage?.kind !== 'text') throw new Error('Choose a text card to translate.');
    const prepared = guestTranslation(preview, activeIdx);
    if (preview && (!prepared || language !== 'French')) return;
    let result;
    if (byok && !preview) {
      const existing = (translations[id] ?? []).find(t => t.language === language && t.model.startsWith('BYOK · '));
      if (existing) result = {cached:true,translation:existing};
      else {
        const answer = await askBYOK({mode:'card_translate',phrase:activePage.content,language},signal);
        result = {cached:false,translation:{language,content:answer.answer,model:answer.model}};
      }
    } else result = preview
      ? { cached: true, translation: { language: 'French' as const, content: prepared, model: 'Prepared demo — no AI' } }
      : await api.translateCard(id, language, signal);
    translations[id] = [...(translations[id] ?? []).filter(t => t.language !== language), result.translation];
    if (activePage?.card_id === id) { preferredLanguage = language; fragmentIdx = 0; fragments = []; fittedCard = ''; }
    showToast(byok ? 'BYOK translation · this page only.' : result.cached ? 'Showing saved translation.' : 'AI translation saved.');
  }
  const progress = $derived(pageCount > 0 ? ((activeIdx + 1) / pageCount) * 100 : 0);

  onMount(() => {
    const key = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement;
      if (document.querySelector('dialog[open]') || target.closest('input,textarea,select,button,a')) return;
      if (['ArrowRight', 'ArrowDown', 'PageDown', ' '].includes(event.key)) { event.preventDefault(); move(1); }
      if (['ArrowLeft', 'ArrowUp', 'PageUp'].includes(event.key)) { event.preventDefault(); move(-1); }
    };
    const visibility = () => { documentVisible = !document.hidden; if (documentVisible && completionOpen) void refreshCompletion(); };
    window.addEventListener('keydown', key);
    document.addEventListener('visibilitychange', visibility);
    visibility();
    const saved = Number(localStorage.getItem('bw:fontScale'));
    if ([0.85, 1, 1.2, 1.45].includes(saved)) fontScale = saved;
    openBook();
    if (!preview) api.me().then(result => translationPaid = result.user.tier !== 'free').catch(() => {});
    return () => { window.removeEventListener('keydown', key); document.removeEventListener('visibilitychange', visibility); };
  });
  let alive = true;
  onDestroy(() => { alive = false; });
  async function openBook() {
    try {
      if (preview) {
        pages = guestPages(preview); pageCount = pages.length; activeIdx = 0;
        return;
      }
      if (!slug) {
        slug = decodeURIComponent(window.location.pathname.split('/')[2] ?? '');
        const { book } = await api.book(slug); title = book.title; author = book.author;
      }
      const params = new URLSearchParams(window.location.search);
      let savedPosition: { page_idx: number; card_idx: number | null } | null = null;
      try {
        savedPosition = (await api.bookPosition(slug)).position;
        canSavePosition = true;
      } catch { /* Signed-out readers still have access to free books. */ }
      const explicitPosition = params.has('from') || params.has('card');
      const requested = Number(params.get('from') ?? (!explicitPosition ? savedPosition?.page_idx ?? 0 : 0));
      const from = Number.isSafeInteger(requested) && requested >= 0 ? requested : 0;
      const requestedCard = Number(params.get('card') ?? (!explicitPosition ? savedPosition?.card_idx ?? -1 : -1));
      const hasCard = Number.isSafeInteger(requestedCard) && requestedCard >= 0;
      const res = await api.cards(slug, hasCard ? requestedCard : 0, 12, hasCard ? 0 : from);
      if (!alive) return;
      pages = res.pages; pageCount = res.page_count; activeIdx = pages[0]?.idx ?? 0;
    } catch (e) {
      if (e instanceof ApiError && e.isPaywall) gated = e.message;
      else error = e instanceof ApiError ? e.message : 'Could not load this book.';
    } finally { loading = false; }
  }

  // Font size is never reduced to force a fit. Unusual screens, poetry and
  // accessibility sizes get word-boundary continuation screens instead.
  function fitCard(node: HTMLElement, params: { page: Page; scale: number }) {
    let frame = 0;
    function fit() {
      if (params.page.kind !== 'text') return;
      const spans = fitTextCard(node, params.page.content);
      const offset = fittedCard === params.page.card_id ? fragments[fragmentIdx]?.start ?? 0 : 0;
      fragments = spans;
      fragmentIdx = enterFromEnd ? spans.length - 1 : Math.max(0, spans.findIndex(s => offset >= s.start && offset < s.end));
      enterFromEnd = false;
      fittedCard = params.page.card_id ?? '';
    }
    const schedule = () => { cancelAnimationFrame(frame); frame = requestAnimationFrame(fit); };
    const resize = new ResizeObserver(schedule); resize.observe(node);
    document.fonts.addEventListener('loadingdone', schedule); schedule();
    return { update(next: typeof params) { params = next; schedule(); }, destroy() { cancelAnimationFrame(frame); resize.disconnect(); document.fonts.removeEventListener('loadingdone', schedule); } };
  }
  $effect(() => {
    const page = activePage;
    const part = fragmentIdx; // changing continuation resets dwell
    if (preview || !page || page.kind !== 'text' || fittedCard !== renderKey || !documentVisible || reportPage || completionOpen || blocked) return;
    if (recorded.has(page.card_id!)) return;
    const timer = window.setTimeout(() => markRead(page.card_id!), DWELL_MS);
    return () => clearTimeout(timer);
  });
  $effect(() => { if (activeIdx >= (pages.at(-1)?.idx ?? 0) - 3 && !loading) loadMore(); });
  let loadingMore = false;
  async function loadMore() {
    const next = (pages.at(-1)?.idx ?? -1) + 1;
    if (preview || loadingMore || next >= pageCount) return;
    loadingMore = true;
    try {
      const res = await api.cards(slug, next, 10);
      if (alive) pages = [...pages, ...res.pages.filter(p => !pages.some(old => old.idx === p.idx))].sort((a, b) => a.idx - b.idx);
    } catch { /* The current card remains readable. */ }
    finally { loadingMore = false; }
  }
  async function move(direction: number) {
    if (previewFinished) { if (direction < 0) previewFinished = false; return; }
    if (navigating || blocked || document.querySelector('dialog[open]')) return;
    if (activePage?.kind === 'text' && fittedCard === renderKey) {
      const nextPart = fragmentIdx + direction;
      if (nextPart >= 0 && nextPart < fragments.length) { fragmentIdx = nextPart; return; }
    }
    const next = activeIdx + direction;
    if (preview && next >= pageCount) { previewFinished = true; return; }
    if (next < 0 || next >= pageCount) return;
    navigating = true;
    try {
      if (!pages.some(p => p.idx === next)) {
        const res = await api.cards(slug, Math.max(0, next - (direction < 0 ? 9 : 0)), 12);
        pages = [...pages, ...res.pages.filter(p => !pages.some(old => old.idx === p.idx))].sort((a, b) => a.idx - b.idx);
      }
      if (pages.some(p => p.idx === next)) { enterFromEnd = direction < 0; activeIdx = next; fragmentIdx = 0; fragments = []; fittedCard = ''; }
    } catch { showToast('Could not load that card. Try again.'); }
    finally { navigating = false; }
  }
  function swipe(event: TouchEvent) {
    if (!touchStart || window.getSelection()?.toString()) return;
    const touch = event.changedTouches[0];
    const dx = touch.clientX - touchStart.x, dy = touch.clientY - touchStart.y;
    touchStart = null;
    if (Math.max(Math.abs(dx), Math.abs(dy)) > 50) move((Math.abs(dy) > Math.abs(dx) ? dy : dx) < 0 ? 1 : -1);
  }
  async function markRead(cardID: string) {
    if (preview || recorded.has(cardID)) return;
    recorded.add(cardID);
    try {
      const res = await api.recordCardRead(cardID);
      if (res.streak_extended) { streak = res.streak_days; streakPulse = true; setTimeout(() => (streakPulse = false), 2200); }
      // A dwell request for the previous card may finish after navigation.
      if (canSavePosition && activePage?.card_id) void savePosition(activePage.card_id);
    } catch (e) {
      recorded.delete(cardID);
      if (e instanceof ApiError && e.isPaywall) blocked = { message: e.message, resetsAt: e.body?.resets_at ?? null };
    }
  }
  async function toggleLike(page: Page) {
    if (preview) return;
    const wasLiked = page.liked_by_me;
    const before = pages.filter(p => p.id === page.id).map(p => ({ p, liked: p.liked_by_me, count: p.like_count }));
    const nextCount = Math.max(0, page.like_count + (wasLiked ? -1 : 1));
    for (const { p } of before) { p.liked_by_me = !wasLiked; p.like_count = nextCount; }
    try { wasLiked ? await api.unlike(page.id) : await api.like(page.id); }
    catch { for (const s of before) { s.p.liked_by_me = s.liked; s.p.like_count = s.count; } showToast('Could not save that like.'); }
  }
  function selectionOffsets(pageEl: HTMLElement) {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) return null;
    const body = pageEl.querySelector('[data-body]');
    const range = sel.getRangeAt(0);
    if (!body || !body.contains(range.commonAncestorContainer)) return null;
    const pre = range.cloneRange(); pre.selectNodeContents(body); pre.setEnd(range.startContainer, range.startOffset);
    const offset = fittedCard === renderKey ? fragment?.start ?? 0 : 0;
    const start = offset + Array.from(pre.toString()).length;
    const text = range.toString();
    return { start, end: start + Array.from(text).length, text };
  }
  async function highlight(page: Page, pageEl: HTMLElement, share: boolean) {
    if (preview || activeTranslation) return;
    const sel = selectionOffsets(pageEl);
    if (!sel || sel.text.trim().length < 4) { showToast('Select the words you want to keep first.'); return; }
    try {
      const parts = page.parts?.length ? page.parts : [{ page_id: page.id, start_off: page.start_off ?? 0, end_off: (page.start_off ?? 0) + Array.from(page.content).length, card_start: 0 }];
      const results = [];
      for (const part of parts) {
        const start = Math.max(sel.start, part.card_start), end = Math.min(sel.end, part.card_start + part.end_off - part.start_off);
        if (end <= start) continue;
        results.push(await api.createHighlight({ page_id: part.page_id, start_off: part.start_off + start - part.card_start, end_off: part.start_off + end - part.card_start, quote: Array.from(page.content).slice(start, end).join(''), share }));
      }
      if (!results.length) { showToast('Select book text first.'); return; }
      if (share) {
        const urls = results.filter(res => res.highlight.share_slug).map(res => `${location.origin}/q/${res.highlight.share_slug}`);
        if (navigator.share) await navigator.share({ title: `${title} — ${author}`, text: urls.length > 1 ? `${sel.text}\n${urls.slice(1).join('\n')}` : sel.text, url: urls[0] });
        else { await navigator.clipboard.writeText(urls.join('\n')); showToast(urls.length > 1 ? 'Quote links copied.' : 'Link copied.'); }
      } else showToast('Saved to your highlights.');
      window.getSelection()?.removeAllRanges();
    } catch (e) { showToast(e instanceof ApiError ? e.message : 'Could not save that highlight.'); }
  }
  function showToast(msg: string) { toast = msg; setTimeout(() => (toast = null), 2600); }
  function setScale(v: number) { fontScale = v; localStorage.setItem('bw:fontScale', String(v)); }
  async function openReport(page: Page) {
    if (preview) return;
    reportPage = page; reportCategory = 'layout'; reportNote = ''; reportError = '';
    await tick(); reportDialog?.showModal();
  }
  async function submitReport(event: SubmitEvent) {
    event.preventDefault(); if (preview || !reportPage?.card_id || reportBusy) return;
    reportBusy = true; reportError = '';
    try { await api.reportCard(reportPage.card_id, reportCategory, reportNote); reportDialog?.close(); showToast('Thanks—your report was saved.'); }
    catch (e) { reportError = e instanceof ApiError ? e.message : 'Could not save your report. Please try again.'; }
    finally { reportBusy = false; }
  }
</script>

<div class="reader" style="--scale:{fontScale}">
  <header class="bar">
    <a href={preview ? '/#try-it-now' : '/app'} class="icon-btn" aria-label="Back to library"><ChevronLeft size={22} /></a>
    {#if canSavePosition && !preview}<button class="completion-control" onclick={openCompletion} aria-label="Reading completion">Read cards</button>{/if}
    <button class="icon-btn" onclick={() => (showType = !showType)} aria-label="Text size"><Type size={20} /></button>
  </header>
  <div class="progress" role="progressbar" aria-valuenow={Math.round(progress)} aria-valuemin={0} aria-valuemax={100}><div class="progress-fill" style="width:{progress}%"></div></div>
  {#if showType}
    <div class="type-panel"><span>Text size</span><div class="scale-row">{#each [0.85, 1, 1.2, 1.45] as s}<button class="scale-btn" class:on={fontScale === s} onclick={() => setScale(s)}>{Math.round(s * 100)}%</button>{/each}</div></div>
  {/if}
  {#if loading}
    <div class="state"><div class="spinner"></div></div>
  {:else if previewFinished}
    <div class="state"><div class="gate"><h2>That’s your first 10 cards!</h2><p>Keep reading for free. Create an account to save your place.</p><a class="cta signup" href="/signup">Create an account</a></div></div>
  {:else if gated}
    <div class="state"><div class="gate"><div class="gate-badge"><Lock size={22} /></div><h2>{title}</h2><p class="gate-author">{author}</p><p>{gated}</p><a class="cta" href="/app/upgrade">See Plus and Pro</a><a class="ghost" href="/app">Browse free books</a><p class="dim">Every public-domain book is free, with no daily limit.</p></div></div>
  {:else if error}
    <div class="state"><div class="gate"><h2>Could not open this book</h2><p>{error}</p><a class="cta" href="/app">Back to the library</a></div></div>
  {:else}
    <div class="feed" ontouchstart={(e) => { touchStart = { x: e.touches[0].clientX, y: e.touches[0].clientY }; }} ontouchend={swipe}>
      {#each activePage ? [activePage] : [] as page (page.card_id)}
        <article class="page" use:fitCard={{ page: displayPage ?? page, scale: fontScale }} data-page-id={page.id} data-card-id={page.card_id} data-kind={page.kind} data-idx={page.idx}>
          {#if activeTranslation}<span class="ai-badge">AI translation</span>{/if}
          {#if page.kind === 'text' && translations[page.card_id ?? '']?.length}
            <select class="card-language" aria-label="Card language" value={activeTranslation?.language ?? 'Original'} onchange={(e) => { preferredLanguage = e.currentTarget.value; fragmentIdx = 0; fragments = []; fittedCard = ''; }}>
              <option value="Original">Original</option>
              {#each translations[page.card_id ?? ''] as t}<option value={t.language}>{t.language}</option>{/each}
            </select>
          {/if}
          <button class="report-icon" disabled={!!preview} onclick={() => openReport(page)} aria-label="Report text or layout issue" title="Report an issue"><Flag size={13} /></button>
          {#if page.kind === 'text'}
            <div class="body" data-body>{displayedText}<span class="sunny-anchor" data-sunny-anchor aria-hidden="true"></span></div>
            <div class="actions">
              <button class="action" disabled={!!preview} class:liked={page.liked_by_me} onclick={() => toggleLike(page)} aria-label={page.liked_by_me ? 'Unlike this page' : 'Like this page'}><Heart size={24} fill={page.liked_by_me ? 'currentColor' : 'none'} />{#if page.like_count > 0}<span class="count">{page.like_count}</span>{/if}</button>
              <button class="action" disabled={!!preview || !!activeTranslation} onclick={(e) => highlight(page, e.currentTarget.closest('.page')!, false)} aria-label="Highlight the selected text"><Highlighter size={24} /></button>
              <button class="action" disabled={!!preview || !!activeTranslation} onclick={(e) => highlight(page, e.currentTarget.closest('.page')!, true)} aria-label="Share the selected quote"><Share2 size={24} /></button>
            </div>
          {:else}
            <div class="heading-card" class:book-title={page.kind === 'title'}><p class="heading-label">{page.kind === 'title' ? 'Your next read' : 'Chapter'}</p><h2 data-body={preview ? true : undefined}>{page.content}<span class="sunny-anchor" data-sunny-anchor aria-hidden="true"></span></h2><p class="heading-note">This card doesn’t count toward your allowance.</p></div>
          {/if}
          <div class="page-num">{page.idx + 1} / {pageCount}{#if page.kind === 'text' && fragments.length > 1} · part {fragmentIdx + 1}/{fragments.length}{/if}</div>
          <button class="turn previous" onclick={() => move(-1)} disabled={navigating || (activeIdx === 0 && fragmentIdx === 0)} aria-label="Previous card"><ChevronLeft size={20} /></button>
          <button class="turn next" onclick={() => move(1)} disabled={navigating || (!preview && activeIdx === pageCount - 1 && fragmentIdx >= fragments.length - 1)} aria-label="Next card"><ChevronRight size={20} /></button>
        </article>
      {/each}
    </div>
  {/if}
  {#if blocked}<div class="paywall" role="alertdialog" aria-labelledby="pw-title"><div class="paywall-card"><h2 id="pw-title">Your licensed reading allowance</h2><p>{blocked.message}</p>{#if blocked.resetsAt}<p class="dim">Your licensed cards reset on {new Date(blocked.resetsAt).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}.</p>{/if}<a class="cta" href="/app/upgrade">See Plus and Pro</a><a class="ghost" href="/app">Browse free books</a></div></div>{/if}
  {#if streakPulse}<div class="streak-pop"><Flame size={20} /> {streak} day streak</div>{/if}
  {#if toast}<div class="toast" role="status">{toast}</div>{/if}
</div>
<dialog class="completion-dialog" bind:this={completionDialog} aria-labelledby="completion-title" onclose={() => { completionOpen = false; }}>
  <header><h2 id="completion-title">Reading completion</h2><button class="icon-btn" onclick={() => completionDialog?.close()} aria-label="Close reading completion">×</button></header>
  <p>Reading on every device signed into your account counts. Only text cards count toward a whole book.</p>
  {#if completionLoading}<p role="status">Checking your reading…</p>
  {:else if completionError}<p role="alert">{completionError}</p>
  {:else if completion}
    <p>{completion.read_cards} of {completion.total_cards} text cards read</p>
    {#if completion.total_cards > 0 && completion.missing_card_indices.length === 0}
      <p>Every text card is read. Your whole-book achievement is unlocked.</p><a href="/app/me">View achievements</a>
    {:else if completion.missing_card_indices.length}
      <p>These cards have no recorded read yet. Open each one and pause to read it.</p>
      <div class="missing-cards">{#each completion.missing_card_indices.slice(0, missingLimit) as idx}<button onclick={() => jumpToMissing(idx)}>Read card {idx + 1}</button>{/each}</div>
      {#if completion.missing_card_indices.length > missingLimit}<button onclick={() => missingLimit += 50}>Show more missing cards</button>{/if}
    {/if}
  {/if}
  <button onclick={() => refreshCompletion()} disabled={completionLoading}>Refresh reading completion</button>
</dialog>
<dialog class="report-dialog" bind:this={reportDialog} aria-labelledby="report-title" onclose={() => { reportPage = null; }}>
  <header><h2 id="report-title">Report an issue</h2><button class="icon-btn" onclick={() => reportDialog?.close()} aria-label="Close report">×</button></header>
  <form onsubmit={submitReport}>
    <label for="report-category">What’s wrong?</label>
    <select id="report-category" bind:value={reportCategory}><option value="layout">Text doesn’t fit or spacing looks wrong</option><option value="break">Awkward card break</option><option value="text">Missing or incorrect text</option></select>
    <label for="report-note">Details (optional)</label><textarea id="report-note" bind:value={reportNote} maxlength="1000" rows="3" placeholder="Tell us what you noticed…"></textarea>
    <p class="report-note">The book and card are included automatically. Please don’t include personal information.</p>
    {#if reportError}<p role="alert">{reportError}{#if reportError === 'Please sign in.'} <a href="/login">Sign in</a> to report this card.{/if}</p>{/if}
    <button class="report-submit" disabled={reportBusy}>{reportBusy ? 'Sending…' : 'Send report'}</button>
  </form>
</dialog>
{#if !previewFinished}
<ReadingCompanion {progress} cardIndex={activeIdx} positionLabel={pageCount ? `Card ${activeIdx + 1} of ${pageCount}` : 'Opening your book'}
  onTranslateCard={activePage?.kind === 'text' ? translateCurrentCard : undefined}
  demoTranslation={!!guestTranslation(preview, activeIdx)} demoTranslated={!!activeTranslation}
  experience={preview ? 'demo' : 'live'} demoWord={preview?.pages[activeIdx]?.word ?? ''} demoDefinition={preview?.pages[activeIdx]?.definition ?? ''} />
{/if}

<style>
  .reader { height: 100dvh; overflow: hidden; display: flex; flex-direction: column; background: var(--bg); }
  .bar { position: absolute; top: calc(3px + env(safe-area-inset-top)); left: 4px; right: 4px; display: flex; justify-content: space-between; z-index: 10; pointer-events: none; }
  .bar .icon-btn { pointer-events: auto; }
  .completion-control { pointer-events: auto; border: none; border-radius: var(--radius-pill); background: var(--bg-card); color: var(--text); padding: 6px 12px; font-size: 13px; align-self: center; }
  .completion-dialog { max-width: min(480px, 90vw); max-height: 80dvh; overflow: auto; border: 1px solid var(--border); border-radius: 16px; padding: 20px; background: var(--bg-card); color: var(--text); }
  .completion-dialog::backdrop { background: #0008; }
  .completion-dialog header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .completion-dialog p { margin: 12px 0; }
  .missing-cards { display: flex; flex-wrap: wrap; gap: 8px; margin: 16px 0; }
  .completion-dialog button:not(.icon-btn) { padding: 8px 12px; border-radius: 8px; border: 1px solid var(--border); background: var(--bg); color: var(--text); }
  .icon-btn { display: grid; place-items: center; width: 40px; height: 40px; border: none; border-radius: var(--radius-pill); background: transparent; color: var(--text); }
  .icon-btn:hover { background: var(--bg-card); }
  .progress { height: calc(3px + env(safe-area-inset-top)); flex-shrink: 0; background: #b9d7ee; }
  .progress-fill { height: 100%; background: #3189c9; transition: width 0.3s ease; }
  .type-panel { position: absolute; top: 46px; right: 8px; z-index: 11; padding: 12px 16px; background: var(--bg-raised); border-bottom: 1px solid var(--border); font-size: 0.8rem; color: var(--text-dim); flex-shrink: 0; }
  .scale-row { display: flex; gap: 8px; margin-top: 8px; }
  .scale-btn { flex: 1; padding: 8px; border-radius: var(--radius-sm); border: 1px solid var(--border); background: var(--bg-card); color: var(--text-dim); font-size: 0.8rem; }
  .scale-btn.on { border-color: var(--orange); color: var(--orange); }
  .feed { flex: 1; min-height: 0; overflow: hidden; touch-action: pan-x; }
  .page { position: relative; height: 100%; min-height: 0; display: flex; flex-direction: column; justify-content: center; padding: 44px 12px calc(78px + env(safe-area-inset-bottom)); background: var(--bg); }
  .ai-badge { position: absolute; left: 48px; top: 12px; padding: 4px 9px; border-radius: 999px; background: #e8edf3; color: #334155; font-size: .65rem; }
  .card-language { position: absolute; right: 48px; top: 12px; max-width: 45%; border: 1px solid var(--border); background: var(--bg-raised); border-radius: 999px; padding: 4px 9px; font-size: .7rem; }
  .body { font-family: var(--font-read); font-size: calc(clamp(16px, 2.7dvh, 18px) * var(--scale)); line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; overflow: visible; flex-shrink: 0; width: 100%; }
  .body::selection { background: var(--selection); }
  .sunny-anchor { display: inline-block; width: 36px; height: 32px; vertical-align: middle; }
  .heading-card { width: 100%; max-width: 38rem; margin: auto; text-align: center; }
  .heading-card h2 { overflow-wrap: anywhere; white-space: pre-line; font-size: clamp(1.3rem, 4vw, 2rem); line-height: 1.45; }
  .heading-label { color: var(--accent); font-size: 0.75rem; letter-spacing: 0.12em; text-transform: uppercase; }
  .heading-note { font-size: 0.75rem; color: var(--text-faint); margin-top: 24px; }
  .actions { position: absolute; left: 50%; bottom: calc(8px + env(safe-area-inset-bottom)); transform: translateX(-50%); display: flex; gap: 14px; }
  .action { position: relative; display: grid; place-items: center; width: 48px; height: 48px; border: 1px solid var(--border); border-radius: var(--radius-pill); background: var(--bg-raised); color: var(--text); transition: transform 0.15s ease; }
  .action:disabled, .report-icon:disabled { opacity: 0.35; cursor: default; }
  .action:active { transform: scale(0.9); }
  .action.liked { color: var(--like); border-color: var(--like); }
  .count { position: absolute; bottom: -4px; font-size: 0.62rem; color: var(--text-dim); }
  .page-num { position: absolute; bottom: calc(62px + env(safe-area-inset-bottom)); left: 16px; font-size: 0.72rem; color: var(--text-faint); }
  .report-icon { position: absolute; right: 12px; bottom: calc(52px + env(safe-area-inset-bottom)); width: 32px; height: 32px; display: grid; place-items: center; padding: 0; border: 0; background: transparent; color: var(--text-faint); opacity: 0.45; }
  .report-icon::before { content: ''; position: absolute; inset: -6px; }
  .report-icon:hover, .report-icon:focus-visible { opacity: 1; }
  .report-icon:focus-visible { outline: 2px solid var(--accent); border-radius: 50%; }
  .turn { position: absolute; bottom: calc(10px + env(safe-area-inset-bottom)); width: 44px; height: 44px; display: grid; place-items: center; background: transparent; color: var(--text-dim); border: 0; border-radius: 50%; }
  .turn:disabled { opacity: 0.2; }
  .previous { left: 12px; } .next { right: 12px; }
  .state { flex: 1; display: grid; place-items: center; color: var(--text-dim); }
  .spinner { width: 28px; height: 28px; border: 2px solid var(--border); border-top-color: var(--orange); border-radius: 50%; animation: spin 0.8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .gate { max-width: 340px; padding: 24px; text-align: center; }
  .gate-badge { display: grid; place-items: center; width: 56px; height: 56px; margin: 0 auto 18px; border-radius: var(--radius-pill); background: var(--grad-brand); color: white; }
  .gate-author, .dim { color: var(--text-faint); font-size: 0.8rem; }
  .cta, .ghost { display: block; padding: 12px; margin-top: 12px; border-radius: var(--radius-pill); }
  .cta { background: var(--accent); color: white; }
  .cta.signup { background: #0f766e; }
  .cta.signup:hover { background: #115e59; }
  .ghost { color: var(--text-dim); }
  .paywall { position: fixed; inset: 0; display: grid; place-items: center; background: rgba(10,10,12,0.82); backdrop-filter: blur(6px); z-index: 50; padding: 24px; }
  .paywall-card { background: var(--bg-card); border: 1px solid var(--border-bright); border-radius: var(--radius); padding: 28px 24px; max-width: 340px; text-align: center; }
  .streak-pop, .toast { position: fixed; left: 50%; transform: translateX(-50%); padding: 12px 18px; border-radius: var(--radius-pill); background: var(--bg-raised); box-shadow: var(--shadow-lift); z-index: 60; }
  .streak-pop { top: 90px; display: flex; gap: 8px; } .toast { bottom: 26px; font-size: 0.85rem; }
  .report-dialog { width: min(420px, calc(100vw - 32px)); max-height: calc(100dvh - 32px); overflow: auto; padding: 20px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--bg); color: var(--text); }
  .report-dialog::backdrop { background: rgba(17,24,39,0.35); }
  .report-dialog header { display: flex; align-items: center; justify-content: space-between; }
  .report-dialog h2 { margin: 0; font-size: 1.1rem; }
  .report-dialog label { display: block; margin: 14px 0 6px; font-size: 0.85rem; }
  .report-dialog select, .report-dialog textarea { width: 100%; padding: 10px; border: 1px solid var(--border); border-radius: var(--radius-sm); color: var(--text); background: var(--bg); }
  .report-dialog textarea { resize: vertical; }
  .report-note { color: var(--text-faint); font-size: 0.75rem; }
  .report-submit { width: 100%; padding: 12px; border: 0; border-radius: var(--radius-pill); color: white; background: var(--accent); }
  .report-submit:disabled { opacity: 0.5; }
  @media (min-width: 680px) { .page { padding: 44px 24px calc(78px + env(safe-area-inset-bottom)); } .body { max-width: 34em; margin: 0 auto; } }
  @media (prefers-reduced-motion: reduce) { .progress-fill, .action { transition: none; } }
</style>
