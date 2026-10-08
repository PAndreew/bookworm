<script lang="ts">
  import { onMount } from 'svelte';
  import ResumeReading from './ResumeReading.svelte';
  import { Flame, Lock, Star } from '@lucide/svelte';
  import { api, type Book } from '../lib/api';

  let books = $state<Book[]>([]);
  let loading = $state(true);
  let signedIn = $state(false);
  let streak = $state({ current: 0, at_risk: false, pages_today: 0, pages_required: 3 });
  let tier = $state<'free' | 'plus' | 'pro'>('free');
  let filter = $state<'all' | 'public_domain' | 'licensed'>('all');
  let genre = $state('');
  let search = $state('');
  let ratings = $state<Record<string, number>>({});
  let saving = $state<Record<string, boolean>>({});
  let message = $state('');
  let catalogError = $state('');

  let genres = $state<string[]>([]);
  let ready = $state(false);
  let retry = $state(0);
  let nextOffset = $state<number | null>(null);
  let loadingMore = $state(false);
  let revision = 0;

  onMount(async () => {
    ready = true;
    const [genresRes, meRes] = await Promise.allSettled([api.genres(), api.me()]);
    if (genresRes.status === 'fulfilled') genres = genresRes.value.genres;
    if (meRes.status === 'fulfilled') {
      signedIn = true;
      tier = meRes.value.user.tier;
      const [streakRes, ratingsRes] = await Promise.allSettled([api.streak(), api.ratings()]);
      if (streakRes.status === 'fulfilled') streak = streakRes.value;
      if (ratingsRes.status === 'fulfilled') ratings = ratingsRes.value.ratings;
      else message = 'Your ratings could not load. Please refresh before rating.';
    }
  });

  function query() {
    return { license: filter === 'all' ? undefined : filter, genre, q: search.trim() };
  }

  $effect(() => {
    if (!ready) return;
    const options = query();
    void retry;
    const id = ++revision;
    const controller = new AbortController();
    loading = true;
    loadingMore = false;
    catalogError = '';
    nextOffset = null;
    const timer = setTimeout(async () => {
      try {
        const result = await api.books(options, controller.signal);
        if (id !== revision) return;
        books = result.books;
        nextOffset = result.next_offset;
      } catch {
        if (id === revision && !controller.signal.aborted) catalogError = 'The shelf could not load. Please try again.';
      } finally {
        if (id === revision && !controller.signal.aborted) loading = false;
      }
    }, options.q ? 250 : 0);
    return () => { clearTimeout(timer); controller.abort(); };
  });

  async function loadMore() {
    if (loadingMore || nextOffset === null) return;
    const id = revision;
    loadingMore = true;
    try {
      const result = await api.books({ ...query(), offset: nextOffset });
      if (id !== revision) return;
      books = [...books, ...result.books];
      nextOffset = result.next_offset;
    } catch {
      if (id === revision) message = 'More books could not load. Please try again.';
    } finally {
      if (id === revision) loadingMore = false;
    }
  }
  const locked = (b: Book) => b.license === 'licensed' && tier === 'free';

  /** Pseudo-random, stable, high-contrast pastels. */
  function cover(slug: string) {
    let h = 0;
    for (let i = 0; i < slug.length; i++) h = (h * 31 + slug.charCodeAt(i)) >>> 0;
    return `hsl(${h % 360} 65% 91%)`;
  }

  async function rate(book: Book, stars: number) {
    if (!signedIn) { window.location.href = '/login'; return; }
    if (saving[book.slug]) return;
    saving[book.slug] = true;
    try {
      const result = await api.rateBook(book.slug, stars);
      ratings[book.slug] = result.stars;
      message = `Rated ${book.title} ${stars} out of 5 stars.`;
    } catch {
      message = 'Your rating could not be saved. Please try again.';
    } finally { saving[book.slug] = false; }
  }
</script>

<div class="wrap">
  <header class="head">
    <h1 class="brand-heading"><img src="/glim.svg" alt="" width="32" height="32" /><span class="brand-text">Potpuri</span></h1>
    {#if signedIn}
      <a href="/app/me" class="streak" class:risk={streak.at_risk}><Flame size={16} /><span>{streak.current}</span></a>
    {:else}
      <a href="/login" class="signin">Sign in</a>
    {/if}
  </header>

  {#if signedIn && streak.pages_today < streak.pages_required}
    <div class="nudge">
      {streak.pages_required - streak.pages_today} more
      {streak.pages_required - streak.pages_today === 1 ? 'page' : 'pages'} today
      {streak.current > 0 ? `to keep your ${streak.current} day streak` : 'to start a streak'}
    </div>
  {/if}
  <ResumeReading />

  <div class="filters" aria-label="Book access">
    <button class:on={filter === 'all'} aria-pressed={filter === 'all'} onclick={() => (filter = 'all')}>All</button>
    <button class:on={filter === 'public_domain'} aria-pressed={filter === 'public_domain'} onclick={() => (filter = 'public_domain')}>Free</button>
    <button class:on={filter === 'licensed'} aria-pressed={filter === 'licensed'} onclick={() => (filter = 'licensed')}>Plus</button>
  </div>
  <div class="discovery">
    <input type="search" aria-label="Search books or authors" placeholder="Find a book or author" maxlength="200" bind:value={search} />
    <select aria-label="Filter by genre" bind:value={genre}>
      <option value="">All genres</option>
      {#each genres as name}<option value={name}>#{name}</option>{/each}
    </select>
  </div>
  <p class="status" role="status">{message}</p>

  {#if loading}
    <div class="grid">{#each Array(4) as _}<div class="skeleton"></div>{/each}</div>
  {:else if catalogError}
    <p class="empty" role="alert">{catalogError}</p>
    <button class="load-more" onclick={() => retry++}>Try again</button>
  {:else if books.length === 0}
    <p class="empty">No books match. Try another genre or search.</p>
  {:else}
    <div class="grid">
      {#each books as book (book.id)}
        <article class="card" style="background:{cover(book.slug)}">
          <a class="book-link" href={`/read/${book.slug}`}>
            <h2 class="t">{book.title}</h2>
            <div class="a">{book.author}</div>
            <div class="p">{book.card_count || book.page_count} cards</div>
            <div class="genres">{#each book.genres as tag}<span>#{tag}</span>{/each}</div>
          </a>
          <div class="rating" role="group" aria-label={`Your rating for ${book.title}`}>
            {#each [1, 2, 3, 4, 5] as stars}
              <button class:filled={(ratings[book.slug] ?? 0) >= stars}
                disabled={saving[book.slug]}
                aria-label={`${signedIn ? 'Rate' : 'Sign in to rate'} ${book.title} ${stars} out of 5 stars`}
                aria-pressed={ratings[book.slug] === stars}
                onclick={() => rate(book, stars)}>
                <Star size={18} fill={(ratings[book.slug] ?? 0) >= stars ? 'currentColor' : 'none'} />
              </button>
            {/each}
          </div>
          <div class="card-footer">
            {#if locked(book)}<a href="/app/upgrade" class="lock"><Lock size={12} /> Plus</a>{/if}
          </div>
        </article>
      {/each}
    </div>
    {#if nextOffset !== null}
      <button class="load-more" disabled={loadingMore} onclick={loadMore}>{loadingMore ? 'Loading…' : 'Load more books'}</button>
    {/if}
  {/if}
</div>

<style>
  .wrap { padding: calc(16px + env(safe-area-inset-top)) 16px 92px; max-width: 760px; margin: 0 auto; }
  .head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18px; }
  h1 { margin: 0; font-size: 1.3rem; font-weight: 600; letter-spacing: -0.02em; }
  .streak { display: flex; align-items: center; gap: 6px; padding: 7px 14px; border-radius: var(--radius-pill); background: #7e22ce; color: #fff; font-weight: 650; font-size: 0.86rem; }
  .streak.risk { animation: breathe 2.4s ease-in-out infinite; }
  @keyframes breathe { 50% { opacity: 0.72; } }
  .signin { padding: 8px 16px; border-radius: var(--radius-pill); border: 1px solid var(--border-bright); font-size: 0.85rem; }
  .nudge { padding: 11px 15px; margin-bottom: 16px; border-radius: var(--radius-sm); background: var(--bg-card); border: 1px solid var(--border); font-size: 0.84rem; color: var(--text-dim); }
  .filters { display: flex; gap: 8px; margin-bottom: 14px; }
  .filters button { padding: 7px 16px; border-radius: var(--radius-pill); border: 1px solid var(--border); background: transparent; color: var(--text-dim); font-size: 0.82rem; }
  .filters button.on { background: var(--text); color: var(--bg); border-color: var(--text); }
  .discovery { display: flex; gap: 12px; margin-bottom: 14px; }
  .discovery input, .discovery select { min-width: 0; padding: 10px 12px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--bg); color: var(--text); font-size: 0.85rem; }
  .discovery input { flex: 1; width: 100%; }
  .discovery select { max-width: 45%; }
  .status { font-size: 0.82rem; color: var(--text-dim); }
  .status:empty { display: none; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 20px; }
  .card { display: flex; flex-direction: column; min-height: 270px; padding: 18px; color: #000; border: 1px solid transparent; border-radius: var(--radius); box-shadow: 0 2px 6px rgba(17,24,39,0.04); transition: box-shadow 0.2s, border-color 0.2s; }
  .card:hover { border-color: var(--border-bright); box-shadow: var(--shadow-lift); }
  .book-link { flex: 1; display: block; overflow-wrap: anywhere; }
  .t { margin: 0; font-family: var(--font-heading); font-weight: 600; font-size: 1.05rem; line-height: 1.35; }
  .a { font-size: 0.78rem; margin-top: 9px; }
  .p { font-size: 0.7rem; margin-top: 6px; }
  .genres { display: flex; flex-wrap: wrap; gap: 5px 8px; margin: 14px 0; font-size: 0.68rem; line-height: 1.5; }
  .rating { display: flex; flex-wrap: wrap; gap: 0; }
  .rating button { display: grid; place-items: center; min-width: 24px; min-height: 44px; padding: 0; border: 0; background: transparent; color: #374151; }
  .rating button.filled, .rating button:hover { color: #854d0e; }
  .rating button:disabled { cursor: wait; opacity: 0.6; }
  .card-footer { display: flex; justify-content: flex-end; align-items: flex-end; min-height: 26px; }
  .lock { display: flex; align-items: center; gap: 4px; padding: 4px 9px; border-radius: var(--radius-pill); background: var(--bg); font-size: 0.66rem; color: var(--accent); font-weight: 600; }
  .skeleton { min-height: 270px; border-radius: var(--radius); background: var(--bg-card); animation: pulse 1.4s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.5; } }
  .load-more { display: block; margin: 24px auto 0; padding: 12px 24px; border: 1px solid var(--border-bright); border-radius: var(--radius-pill); background: var(--bg); color: var(--text); }
  .load-more:disabled { opacity: 0.6; cursor: wait; }
  .empty { color: var(--text-faint); text-align: center; padding: 40px 0; }
</style>
