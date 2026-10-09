<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Heart, Highlighter, Share2, ChevronLeft, Type, Flame, Lock } from '@lucide/svelte';
  import { api, ApiError, type Page, type Translation } from '../lib/api';
  import ReadingCompanion from './ReadingCompanion.svelte';

  import { guestPages, guestTranslation, type GuestBook } from '../lib/guest-books';
  let { slug, title, author, preview }: { slug: string; title: string; author: string; preview?: GuestBook } = $props();
  let previewFinished = $state(false);

  let pages = $state<Page[]>([]);
  let pageCount = $state(0);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Paywall state, shown as an interstitial rather than cutting off mid-sentence.
  let blocked = $state<{ message: string; resetsAt: string | null } | null>(null);
  // Set when the whole book is above this reader's tier, as opposed to `blocked`,
  // which is hitting the daily allowance part-way through a book you can read.
  let gated = $state<string | null>(null);

  let activeIdx = $state(0);
  let streak = $state(0);
  let streakPulse = $state(false);
  let fontScale = $state(1);
  let showType = $state(false);
  let toast = $state<string | null>(null);

  let container = $state<HTMLElement | null>(null);
  let observer: IntersectionObserver | null = null;

  // A page counts as read once it has held the screen briefly. Firing on first
  // paint would bill a user for pages they flicked past.
  const DWELL_MS = 1800;
  const dwellTimers = new Map<string, number>();
  const recorded = new Set<string>();
  const activePage = $derived(pages.find(p => p.idx === activeIdx));
  let translationPaid = $state(false);
  let translations = $state<Record<string, Translation[]>>({});
  let preferredLanguage = $state('Original');
  const loadedTranslations = new Set<string>();
  function cardTranslation(page: Page) { return (translations[page.card_id ?? ''] ?? []).find(t => t.language === preferredLanguage); }
  $effect(() => {
    const id = activePage?.card_id;
    if (preview || !translationPaid || activePage?.kind !== 'text' || !id || loadedTranslations.has(id)) return;
    loadedTranslations.add(id);
    api.cardTranslations(id).then(result => translations[id] = result.translations).catch(() => loadedTranslations.delete(id));
  });
  async function translateCurrentCard(language: string, signal?: AbortSignal) {
    const id = activePage?.card_id;
    if (!id || activePage?.kind !== 'text') throw new Error('Choose a text card to translate.');
    const prepared = guestTranslation(preview, activeIdx);
    if (preview && (!prepared || language !== 'French')) return;
    const result = preview
      ? { cached: true, translation: { language: 'French' as const, content: prepared, model: 'Prepared demo — no AI' } }
      : await api.translateCard(id, language, signal);
    translations[id] = [...(translations[id] ?? []).filter(t => t.language !== language), result.translation];
    if (activePage?.card_id === id) preferredLanguage = language;
    showToast(result.cached ? 'Showing saved translation.' : 'AI translation saved.');
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
    const cardID = pages.find(p => p.idx === activeIdx)?.card_id;
    if (canSavePosition && cardID && !loading) void savePosition(cardID);
  });

  onMount(async () => {
    const saved = localStorage.getItem('bw:fontScale');
    if (saved) fontScale = parseFloat(saved);

    try {
      if (preview) {
        pages = guestPages(preview); pageCount = pages.length; activeIdx = 0;
        return;
      }
      if (!slug) {
        slug = decodeURIComponent(window.location.pathname.split('/')[2] ?? '');
        const { book } = await api.book(slug);
        title = book.title;
        author = book.author;
      }
      api.me().then(result => translationPaid = result.user.tier !== 'free').catch(() => {});
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
      activeIdx = res.pages[0]?.idx ?? 0;
      pages = res.pages;
      pageCount = res.page_count;
    } catch (e) {
      // A 402 here is not a failure, it is the product working: this reader is
      // on a tier that does not include this book. It deserves a real screen,
      // not an error message.
      if (e instanceof ApiError && e.isPaywall) {
        gated = e.message;
      } else {
        error = e instanceof ApiError ? e.message : 'Could not load this book.';
      }
    } finally {
      loading = false;
    }
  });

  onDestroy(() => {
    observer?.disconnect();
    dwellTimers.forEach((t) => clearTimeout(t));
  });

  // Wire up the observer once the cards exist.
  $effect(() => {
    if (!container || pages.length === 0) return;

    observer?.disconnect();
    observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          const el = entry.target as HTMLElement;
          const id = el.dataset.cardId!;
          const idx = Number(el.dataset.idx);

          if (entry.isIntersecting && entry.intersectionRatio > 0.6) {
            activeIdx = idx;
            if (el.dataset.kind === 'text' && !recorded.has(id) && !dwellTimers.has(id)) {
              dwellTimers.set(id, window.setTimeout(() => markRead(id), DWELL_MS));
            }
            // Prefetch before the reader reaches the end of the window.
            if (idx >= (pages.at(-1)?.idx ?? 0) - 3) loadMore();
          } else {
            const t = dwellTimers.get(id);
            if (t) {
              clearTimeout(t);
              dwellTimers.delete(id);
            }
          }
        }
      },
      { threshold: [0.6] },
    );

    container.querySelectorAll('[data-card-id]').forEach((el) => observer!.observe(el));
  });

  let loadingMore = false;
  async function loadMore() {
    const next = (pages.at(-1)?.idx ?? -1) + 1;
    if (preview || loadingMore || next >= pageCount) return;
    loadingMore = true;
    try {
      const res = await api.cards(slug, next, 10);
      pages = [...pages, ...res.pages];
    } catch {
      // Silent: the reader still has pages in hand, and a failed prefetch
      // should not interrupt reading.
    } finally {
      loadingMore = false;
    }
  }

  async function markRead(pageId: string) {
    dwellTimers.delete(pageId);
    if (preview || recorded.has(pageId)) return;
    recorded.add(pageId);

    try {
      const res = await api.recordCardRead(pageId);
      // A dwell request for the previous card may finish after navigation.
      const currentCardID = pages.find(p => p.idx === activeIdx)?.card_id;
      if (canSavePosition && currentCardID) void savePosition(currentCardID);
      if (res.streak_extended) {
        streak = res.streak_days;
        streakPulse = true;
        setTimeout(() => (streakPulse = false), 2200);
      }
    } catch (e) {
      recorded.delete(pageId);
      if (e instanceof ApiError && e.isPaywall) {
        blocked = { message: e.message, resetsAt: e.body?.resets_at ?? null };
      }
    }
  }

  async function toggleLike(page: Page) {
    if (preview) return;
    // Optimistic: the heart must answer instantly, or it feels broken.
    const wasLiked = page.liked_by_me;
    const siblings = pages.filter(p => p.id === page.id);
    const before = siblings.map(p => ({ p, liked: p.liked_by_me, count: p.like_count }));
    const nextCount = Math.max(0, page.like_count + (wasLiked ? -1 : 1));
    for (const p of siblings) { p.liked_by_me = !wasLiked; p.like_count = nextCount; }
    pages = pages;

    try {
      wasLiked ? await api.unlike(page.id) : await api.like(page.id);
    } catch {
      for (const saved of before) { saved.p.liked_by_me = saved.liked; saved.p.like_count = saved.count; }
      pages = pages;
      showToast('Could not save that like.');
    }
  }

  /** Character offsets of the current selection within this page's text. */
  function selectionOffsets(pageEl: HTMLElement): { start: number; end: number; text: string } | null {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) return null;

    const body = pageEl.querySelector('[data-body]') as HTMLElement;
    if (!body) return null;

    const range = sel.getRangeAt(0);
    if (!body.contains(range.commonAncestorContainer)) return null;

    const pre = range.cloneRange();
    pre.selectNodeContents(body);
    pre.setEnd(range.startContainer, range.startOffset);

    const start = Array.from(pre.toString()).length;
    const text = range.toString();
    return { start, end: start + Array.from(text).length, text };
  }

  async function highlight(page: Page, pageEl: HTMLElement, share: boolean) {
    if (preview || cardTranslation(page)) return;
    const sel = selectionOffsets(pageEl);
    if (!sel || sel.text.trim().length < 4) {
      showToast('Select the words you want to keep first.');
      return;
    }

    try {
      const res = await api.createHighlight({
        page_id: page.id,
        start_off: (page.start_off ?? 0) + sel.start,
        end_off: (page.start_off ?? 0) + sel.end,
        quote: sel.text,
        share,
      });

      if (share && res.highlight.share_slug) {
        const url = `${location.origin}/q/${res.highlight.share_slug}`;
        // Web Share where available: on mobile this opens the native sheet,
        // which is the whole point of a share button on a phone.
        if (navigator.share) {
          await navigator.share({ title: `${title} — ${author}`, text: sel.text, url });
        } else {
          await navigator.clipboard.writeText(url);
          showToast('Link copied.');
        }
      } else {
        showToast('Saved to your highlights.');
      }
      window.getSelection()?.removeAllRanges();
    } catch (e) {
      showToast(e instanceof ApiError ? e.message : 'Could not save that highlight.');
    }
  }

  function showToast(msg: string) {
    toast = msg;
    setTimeout(() => (toast = null), 2600);
  }

  function setScale(v: number) {
    fontScale = v;
    localStorage.setItem('bw:fontScale', String(v));
  }

  const progress = $derived(pageCount > 0 ? ((activeIdx + 1) / pageCount) * 100 : 0);
</script>

<div class="reader" style="--scale:{fontScale}">
  <header class="bar">
    <a href={preview ? '/#try-it-now' : '/app'} class="icon-btn" aria-label="Back to library"><ChevronLeft size={22} /></a>

    <div class="title-wrap">
      <div class="title">{title}</div>
      <div class="author">{author}</div>
    </div>

    <button class="icon-btn" onclick={() => (showType = !showType)} aria-label="Text size">
      <Type size={20} />
    </button>
  </header>

  <div class="progress" role="progressbar" aria-valuenow={Math.round(progress)} aria-valuemin={0} aria-valuemax={100}>
    <div class="progress-fill" style="width:{progress}%"></div>
  </div>

  {#if showType}
    <div class="type-panel">
      <span>Text size</span>
      <div class="scale-row">
        {#each [0.85, 1, 1.2, 1.45] as s}
          <button class="scale-btn" class:on={fontScale === s} onclick={() => setScale(s)}>
            {Math.round(s * 100)}%
          </button>
        {/each}
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="state"><div class="spinner"></div></div>
  {:else if previewFinished}
    <div class="state"><div class="gate"><h2>That’s your first 10 cards!</h2><p>Keep reading for free. Create an account to save your place.</p><a class="gate-cta signup" href="/signup">Create an account</a></div></div>
  {:else if gated}
    <div class="state">
      <div class="gate">
        <div class="gate-badge"><Lock size={22} /></div>
        <h2>{title}</h2>
        <p class="gate-author">{author}</p>
        <p class="gate-msg">{gated}</p>
        <a class="gate-cta" href="/app/upgrade">See Plus and Pro</a>
        <a class="gate-alt" href="/app">Browse free books</a>
        <p class="gate-note">Every public-domain book is free, with no daily limit.</p>
      </div>
    </div>
  {:else if error}
    <div class="state">
      <div class="gate">
        <h2>Could not open this book</h2>
        <p class="gate-msg">{error}</p>
        <a class="gate-cta" href="/app">Back to the library</a>
      </div>
    </div>
  {:else}
    <div class="feed" bind:this={container}>
      {#each pages as page (page.card_id)}
        <article class="page" data-page-id={page.id} data-card-id={page.card_id} data-kind={page.kind} data-idx={page.idx}>
          {#if page.kind === 'text'}
          {#if cardTranslation(page)}<span class="ai-badge">AI translation</span>{/if}
          {#if translations[page.card_id ?? '']?.length}
            <select class="card-language" aria-label="Card language" value={cardTranslation(page)?.language ?? 'Original'} onchange={(e) => preferredLanguage = e.currentTarget.value}>
              <option value="Original">Original</option>
              {#each translations[page.card_id ?? ''] as t}<option value={t.language}>{t.language}</option>{/each}
            </select>
          {/if}
          <div class="body" data-body>{cardTranslation(page)?.content ?? page.content}<span class="sunny-anchor" data-sunny-anchor aria-hidden="true"></span></div>

          <div class="actions">
            <button
              class="action"
              disabled={!!preview}
              class:liked={page.liked_by_me}
              onclick={() => toggleLike(page)}
              aria-label={page.liked_by_me ? 'Unlike this page' : 'Like this page'}
            >
              <Heart size={24} fill={page.liked_by_me ? 'currentColor' : 'none'} />
              {#if page.like_count > 0}<span class="count">{page.like_count}</span>{/if}
            </button>

            <button
              class="action"
              disabled={!!preview || !!cardTranslation(page)}
              onclick={(e) => highlight(page, e.currentTarget.closest('.page')!, false)}
              aria-label="Highlight the selected text"
            >
              <Highlighter size={24} />
            </button>

            <button
              class="action"
              disabled={!!preview || !!cardTranslation(page)}
              onclick={(e) => highlight(page, e.currentTarget.closest('.page')!, true)}
              aria-label="Share the selected quote"
            >
              <Share2 size={24} />
            </button>
          </div>
          {:else}
            <div class="heading-card" class:book-title={page.kind === 'title'}>
              <p class="heading-label">{page.kind === 'title' ? 'Your next read' : 'Chapter'}</p>
              <h2 data-body={preview ? true : undefined}>{page.content}<span class="sunny-anchor" data-sunny-anchor aria-hidden="true"></span></h2>
              <p class="heading-note">This card doesn’t count toward your allowance.</p>
            </div>
          {/if}
          <div class="page-num">{page.idx + 1} / {pageCount}</div>
          {#if preview && page.idx === pageCount - 1}<button class="preview-finish" onclick={() => previewFinished = true}>Finish preview</button>{/if}
        </article>
      {/each}
    </div>
  {/if}

  {#if blocked}
    <div class="paywall" role="alertdialog" aria-labelledby="pw-title">
      <div class="paywall-card">
        <h2 id="pw-title">Your licensed reading allowance</h2>
        <p>{blocked.message}</p>
        {#if blocked.resetsAt}
          <p class="dim">
            Your licensed cards reset on
            {new Date(blocked.resetsAt).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}.
          </p>
        {/if}
        <a class="cta" href="/app/upgrade">See Plus and Pro</a>
        <a class="ghost" href="/app">Browse free books</a>
      </div>
    </div>
  {/if}

  {#if streakPulse}
    <div class="streak-pop"><Flame size={20} /> {streak} day streak</div>
  {/if}

  {#if toast}<div class="toast">{toast}</div>{/if}
</div>
{#if !previewFinished}
<ReadingCompanion {progress} cardIndex={activeIdx} positionLabel={pageCount ? `Card ${activeIdx + 1} of ${pageCount}` : 'Opening your book'}
  onTranslateCard={activePage?.kind === 'text' ? translateCurrentCard : undefined}
  demoTranslation={!!guestTranslation(preview, activeIdx)} demoTranslated={!!activePage && !!cardTranslation(activePage)}
  experience={preview ? 'demo' : 'live'} demoWord={preview?.pages[activeIdx]?.word ?? ''} demoDefinition={preview?.pages[activeIdx]?.definition ?? ''} />
{/if}

<style>
  .ai-badge { position: absolute; left: 16px; top: 12px; padding: 4px 9px; border-radius: 999px; background: #e8edf3; color: #334155; font-size: .65rem; }
  .card-language { position: absolute; right: 16px; top: 12px; max-width: 45%; border: 1px solid var(--border); background: var(--bg-raised); border-radius: 999px; padding: 4px 9px; font-size: .7rem; }
  .reader {
    height: 100dvh;
    display: flex;
    flex-direction: column;
    background: var(--bg);
  }

  .bar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    padding-top: calc(10px + env(safe-area-inset-top));
    border-bottom: 1px solid var(--border);
    background: var(--bg-raised);
  }

  .title-wrap {
    flex: 1;
    min-width: 0;
  }
  .title {
    font-weight: 650;
    font-size: 0.95rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .author {
    font-size: 0.78rem;
    color: var(--text-faint);
  }

  .icon-btn {
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    border: none;
    border-radius: var(--radius-pill);
    background: transparent;
    color: var(--text);
  }
  .icon-btn:hover {
    background: var(--bg-card);
  }

  .progress {
    height: 2px;
    background: var(--border);
  }
  .progress-fill {
    height: 100%;
    background: var(--grad-brand);
    transition: width 0.3s ease;
  }

  .type-panel {
    padding: 12px 16px;
    background: var(--bg-raised);
    border-bottom: 1px solid var(--border);
    font-size: 0.8rem;
    color: var(--text-dim);
  }
  .scale-row {
    display: flex;
    gap: 8px;
    margin-top: 8px;
  }
  .scale-btn {
    flex: 1;
    padding: 8px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg-card);
    color: var(--text-dim);
    font-size: 0.8rem;
  }
  .scale-btn.on {
    border-color: var(--orange);
    color: var(--orange);
  }

  /* The feed: one page per screen, snapped. */
  .feed {
    flex: 1;
    overflow-y: auto;
    scroll-snap-type: y mandatory;
    scrollbar-width: none;
  }
  .feed::-webkit-scrollbar {
    display: none;
  }

  .page {
    position: relative;
    height: 100%;
    scroll-snap-align: start;
    scroll-snap-stop: always;
    display: flex;
    flex-direction: column;
    justify-content: center;
    padding: 18px 22px 90px;
    background: var(--bg);
  }

  /* Full-width text above a horizontal bottom action bar. */
  @media (min-width: 680px) {
    .page {
      padding: 28px 40px 90px;
    }
    .body {
      max-width: 34em;
      margin: 0 auto;
    }
  }

  .heading-card { width: 100%; max-width: 38rem; margin: auto; text-align: center; }
  .heading-card h2 { overflow-wrap: anywhere; white-space: pre-line; font-size: clamp(1.3rem, 4vw, 2rem); line-height: 1.45; }
  .heading-label { color: var(--accent); font-size: 0.75rem; letter-spacing: 0.12em; text-transform: uppercase; }
  .heading-note { font-size: 0.75rem; color: var(--text-faint); margin-top: 24px; }
  .sunny-anchor { display: inline-block; width: 36px; height: 32px; vertical-align: middle; }

  .chapter {
    font-size: 0.72rem;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--orange);
    margin-bottom: 18px;
    font-weight: 600;
  }

  .body {
    font-family: var(--font-read);
    font-size: calc(clamp(16px, 2.7dvh, 18px) * var(--scale));
    line-height: 1.55;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    /* A long page at a large text size scrolls inside its own card rather than
     * being re-chunked: accessibility wins over a perfect single screen. */
    overflow-y: auto;
    max-height: 100%;
    scrollbar-width: none;
  }
  .body::-webkit-scrollbar {
    display: none;
  }
  .body::selection {
    background: var(--selection);
  }

  .actions {
    position: absolute;
    left: 50%;
    bottom: calc(18px + env(safe-area-inset-bottom));
    transform: translateX(-50%);
    display: flex;
    flex-direction: row;
    gap: 14px;
  }

  .action {
    position: relative;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    border: 1px solid var(--border);
    border-radius: var(--radius-pill);
    background: var(--bg-raised);
    backdrop-filter: blur(10px);
    color: var(--text);
    transition: transform 0.15s ease, color 0.15s ease;
  }
  .preview-finish { position: absolute; right: 12px; bottom: 20px; padding: 10px; border: 1px solid var(--border); border-radius: var(--radius-pill); background: var(--bg-raised); }
  .action:disabled { opacity: 0.35; cursor: default; }
  .action:active {
    transform: scale(0.9);
  }
  .action.liked {
    color: var(--like);
    border-color: var(--like);
  }
  .count {
    position: absolute;
    bottom: -4px;
    font-size: 0.62rem;
    color: var(--text-dim);
  }

  .page-num {
    position: absolute;
    bottom: calc(74px + env(safe-area-inset-bottom));
    left: 16px;
    font-size: 0.72rem;
    color: var(--text-faint);
  }

  .state {
    flex: 1;
    display: grid;
    place-items: center;
    color: var(--text-dim);
  }
  .spinner {
    width: 28px;
    height: 28px;
    border: 2px solid var(--border);
    border-top-color: var(--orange);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .gate {
    max-width: 320px;
    padding: 0 24px;
    text-align: center;
  }
  .gate-badge {
    display: grid;
    place-items: center;
    width: 56px;
    height: 56px;
    margin: 0 auto 18px;
    border-radius: var(--radius-pill);
    background: var(--grad-brand);
    color: #fff;
  }
  .gate h2 {
    margin: 0 0 4px;
    font-size: 1.2rem;
    font-weight: 680;
    color: var(--text);
    line-height: 1.3;
  }
  .gate-author {
    margin: 0 0 16px;
    font-size: 0.85rem;
    color: var(--text-faint);
  }
  .gate-msg {
    margin: 0 0 22px;
    font-size: 0.92rem;
    color: var(--text-dim);
    line-height: 1.55;
  }
  .gate-cta {
    display: block;
    padding: 13px;
    border-radius: var(--radius-pill);
    background: var(--grad-brand);
    color: #fff;
    font-weight: 640;
    font-size: 0.92rem;
  }
  .gate-alt {
    display: block;
    margin-top: 11px;
    padding: 11px;
    font-size: 0.86rem;
    color: var(--text-dim);
  }
  .gate-note {
    margin: 18px 0 0;
    font-size: 0.78rem;
    color: var(--text-faint);
    line-height: 1.5;
  }

  .paywall {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    background: rgba(10, 10, 12, 0.82);
    backdrop-filter: blur(6px);
    z-index: 50;
    padding: 24px;
  }
  .paywall-card {
    background: var(--bg-card);
    border: 1px solid var(--border-bright);
    border-radius: var(--radius);
    padding: 28px 24px;
    max-width: 340px;
    text-align: center;
    box-shadow: var(--shadow-lift);
  }
  .paywall-card h2 {
    margin: 0 0 10px;
    font-size: 1.15rem;
  }
  .paywall-card p {
    margin: 0 0 8px;
    color: var(--text-dim);
    font-size: 0.9rem;
  }
  .dim {
    color: var(--text-faint) !important;
    font-size: 0.82rem !important;
  }
  .cta {
    display: block;
    margin-top: 18px;
    padding: 13px;
    border-radius: var(--radius-pill);
    background: var(--grad-brand);
    color: #fff;
    font-weight: 600;
  }
  .ghost {
    display: block;
    width: 100%;
    margin-top: 10px;
    padding: 11px;
    border: none;
    background: transparent;
    color: var(--text-faint);
    font-size: 0.85rem;
  }

  .streak-pop {
    position: fixed;
    top: calc(72px + env(safe-area-inset-top));
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 18px;
    border-radius: var(--radius-pill);
    background: var(--grad-warm);
    color: #fff;
    font-weight: 650;
    font-size: 0.88rem;
    box-shadow: var(--shadow-lift);
    animation: pop 2.2s ease forwards;
    z-index: 40;
  }
  @keyframes pop {
    0% {
      opacity: 0;
      transform: translate(-50%, -12px) scale(0.9);
    }
    12%,
    78% {
      opacity: 1;
      transform: translate(-50%, 0) scale(1);
    }
    100% {
      opacity: 0;
      transform: translate(-50%, -8px) scale(0.97);
    }
  }

  .toast {
    position: fixed;
    bottom: calc(26px + env(safe-area-inset-bottom));
    left: 50%;
    transform: translateX(-50%);
    padding: 11px 18px;
    border-radius: var(--radius-pill);
    background: var(--bg-raised);
    border: 1px solid var(--border-bright);
    font-size: 0.84rem;
    box-shadow: var(--shadow-lift);
    z-index: 60;
  }
  .gate-cta.signup { background: #0f766e; }
  .gate-cta.signup:hover { background: #115e59; }
</style>
