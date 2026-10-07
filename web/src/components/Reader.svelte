<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Heart, Highlighter, Share2, ChevronLeft, Type, Flame, Lock } from '@lucide/svelte';
  import { api, ApiError, type Page } from '../lib/api';
  import ReadingCompanion from './ReadingCompanion.svelte';

  let { slug, title, author }: { slug: string; title: string; author: string } = $props();

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

  onMount(async () => {
    const saved = localStorage.getItem('bw:fontScale');
    if (saved) fontScale = parseFloat(saved);

    try {
      if (!slug) {
        slug = decodeURIComponent(window.location.pathname.split('/')[2] ?? '');
        const { book } = await api.book(slug);
        title = book.title;
        author = book.author;
      }
      const requested = Number(new URLSearchParams(window.location.search).get('from') ?? 0);
      const from = Number.isSafeInteger(requested) && requested >= 0 ? requested : 0;
      const res = await api.pages(slug, from, 12);
      activeIdx = from;
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
          const id = el.dataset.pageId!;
          const idx = Number(el.dataset.idx);

          if (entry.isIntersecting && entry.intersectionRatio > 0.6) {
            activeIdx = idx;
            if (!recorded.has(id) && !dwellTimers.has(id)) {
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

    container.querySelectorAll('[data-page-id]').forEach((el) => observer!.observe(el));
  });

  let loadingMore = false;
  async function loadMore() {
    const next = (pages.at(-1)?.idx ?? -1) + 1;
    if (loadingMore || next >= pageCount) return;
    loadingMore = true;
    try {
      const res = await api.pages(slug, next, 10);
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
    if (recorded.has(pageId)) return;
    recorded.add(pageId);

    try {
      const res = await api.recordRead(pageId);
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
    // Optimistic: the heart must answer instantly, or it feels broken.
    const wasLiked = page.liked_by_me;
    page.liked_by_me = !wasLiked;
    page.like_count += wasLiked ? -1 : 1;
    pages = pages;

    try {
      wasLiked ? await api.unlike(page.id) : await api.like(page.id);
    } catch {
      page.liked_by_me = wasLiked;
      page.like_count += wasLiked ? 1 : -1;
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

    const start = pre.toString().length;
    const text = range.toString();
    return { start, end: start + text.length, text };
  }

  async function highlight(page: Page, pageEl: HTMLElement, share: boolean) {
    const sel = selectionOffsets(pageEl);
    if (!sel || sel.text.trim().length < 4) {
      showToast('Select the words you want to keep first.');
      return;
    }

    try {
      const res = await api.createHighlight({
        page_id: page.id,
        start_off: sel.start,
        end_off: sel.end,
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
    <a href="/app" class="icon-btn" aria-label="Back to library"><ChevronLeft size={22} /></a>

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
      {#each pages as page (page.id)}
        <article class="page" data-page-id={page.id} data-idx={page.idx}>
          {#if page.chapter_title}
            <div class="chapter">{page.chapter_title}</div>
          {/if}

          <div class="body" data-body>{page.content}</div>

          <div class="actions">
            <button
              class="action"
              class:liked={page.liked_by_me}
              onclick={() => toggleLike(page)}
              aria-label={page.liked_by_me ? 'Unlike this page' : 'Like this page'}
            >
              <Heart size={24} fill={page.liked_by_me ? 'currentColor' : 'none'} />
              {#if page.like_count > 0}<span class="count">{page.like_count}</span>{/if}
            </button>

            <button
              class="action"
              onclick={(e) => highlight(page, e.currentTarget.closest('.page')!, false)}
              aria-label="Highlight the selected text"
            >
              <Highlighter size={24} />
            </button>

            <button
              class="action"
              onclick={(e) => highlight(page, e.currentTarget.closest('.page')!, true)}
              aria-label="Share the selected quote"
            >
              <Share2 size={24} />
            </button>
          </div>

          <div class="page-num">{page.idx + 1} / {pageCount}</div>
        </article>
      {/each}
    </div>
  {/if}

  {#if blocked}
    <div class="paywall" role="alertdialog" aria-labelledby="pw-title">
      <div class="paywall-card">
        <h2 id="pw-title">That is today's reading</h2>
        <p>{blocked.message}</p>
        {#if blocked.resetsAt}
          <p class="dim">
            Your pages come back at
            {new Date(blocked.resetsAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}.
          </p>
        {/if}
        <a class="cta" href="/app/upgrade">See Plus and Pro</a>
        <button class="ghost" onclick={() => (blocked = null)}>Keep browsing</button>
      </div>
    </div>
  {/if}

  {#if streakPulse}
    <div class="streak-pop"><Flame size={20} /> {streak} day streak</div>
  {/if}

  {#if toast}<div class="toast">{toast}</div>{/if}
</div>
<ReadingCompanion {progress} positionLabel={pageCount ? `Page ${activeIdx + 1} of ${pageCount}` : 'Opening your book'} />

<style>
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
    /* The right padding clears the action rail: text must never run under the
     * like and share buttons. */
    padding: 28px 78px 92px 22px;
    background: var(--bg);
  }

  /* On a wide screen the rail sits beside the column rather than over it. */
  @media (min-width: 680px) {
    .page {
      padding: 40px 96px 92px 40px;
    }
    .body {
      max-width: 34em;
      margin: 0 auto;
    }
  }

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
    font-size: calc(var(--reader-size) * var(--scale));
    line-height: var(--reader-leading);
    white-space: pre-wrap;
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
    right: 14px;
    bottom: 50%;
    transform: translateY(50%);
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .action {
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
    bottom: 60px;
    left: 22px;
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
</style>
