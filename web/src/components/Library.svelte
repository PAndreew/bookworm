<script lang="ts">
  import { onMount } from 'svelte';
  import { ArrowDown, ArrowUp, ArrowUpDown, Filter, Search, Flame } from '@lucide/svelte';
  import BookCard from './BookCard.svelte';
  import { api, type Book } from '../lib/api';

  let books = $state<Book[]>([]);
  let loading = $state(true);
  let signedIn = $state(false);
  let streak = $state({ current: 0, at_risk: false, pages_today: 0, pages_required: 3 });
  let tier = $state<'free' | 'plus' | 'pro'>('free');
  let openedBooks = $state<Book[]>([]);
  let openedError = $state('');
  let openedOffset = $state<number | null>(null);
  let loadingOpened = $state(false);
  let panel = $state<'genres' | 'sort' | null>(null);
  let sort = $state('recent');
  let direction = $state<'asc' | 'desc'>('desc');
  let genre = $state<string[]>([]);
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
    const [genresRes, meRes] = await Promise.allSettled([api.genres(), api.me()]);
    if (genresRes.status === 'fulfilled') genres = genresRes.value.genres;
    if (meRes.status === 'fulfilled') {
      signedIn = true;
      tier = meRes.value.user.tier;
      ready = true;
      const [streakRes, ratingsRes] = await Promise.allSettled([api.streak(), api.ratings()]);
      if (streakRes.status === 'fulfilled') streak = streakRes.value;
      if (ratingsRes.status === 'fulfilled') ratings = ratingsRes.value.ratings;
      else message = 'Your ratings could not load. Please refresh before rating.';
    }
    ready = true;
  });

  function query() {
    return { license: 'public_domain', genre, q: search.trim(), sort, direction };
  }

  $effect(() => {
    if (!ready) return;
    const options = query();
    const includeOpened = signedIn;
    void retry;
    const id = ++revision;
    const controller = new AbortController();
    loading = true;
    loadingMore = false;
    catalogError = '';
    openedError = '';
    nextOffset = null;
    openedOffset = null;
    loadingOpened = false;
    const timer = setTimeout(async () => {
      const [all, opened] = await Promise.allSettled([
        api.books(options, controller.signal),
        includeOpened ? api.books({ ...options, mine: true }, controller.signal) : Promise.resolve(null),
      ]);
      if (id !== revision || controller.signal.aborted) return;
      if (all.status === 'fulfilled') {
        books = all.value.books;
        nextOffset = all.value.next_offset;
      } else catalogError = 'The shelf could not load. Please try again.';
      if (opened.status === 'fulfilled') {
        openedBooks = opened.value?.books ?? [];
        openedOffset = opened.value?.next_offset ?? null;
      } else openedError = 'Your books could not load. Please try again.';
      loading = false;
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
  async function loadMoreOpened() {
    if (loadingOpened || openedOffset === null) return;
    const id = revision;
    loadingOpened = true;
    try {
      const result = await api.books({ ...query(), mine: true, offset: openedOffset });
      if (id !== revision) return;
      openedBooks = [...openedBooks, ...result.books];
      openedOffset = result.next_offset;
    } catch {
      if (id === revision) message = 'More of your books could not load. Please try again.';
    } finally {
      if (id === revision) loadingOpened = false;
    }
  }
  const locked = (b: Book) => b.license === 'licensed' && tier === 'free';

  async function rate(book: Book, stars: number) {
    if (!signedIn) { window.location.href = '/login'; return; }
    if (saving[book.slug]) return;
    saving[book.slug] = true;
    try {
      const result = await api.rateBook(book.slug, stars);
      ratings[book.slug] = result.stars;
      retry++; // Refresh review aggregates and any rating-based ordering.
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

  <div class="discovery">
    <button class="icon-button" class:active={genre.length > 0} aria-label={genre.length ? `Filter genres (${genre.length} selected)` : 'Filter genres'} aria-expanded={panel === 'genres'} aria-controls="genre-panel" onclick={() => panel = panel === 'genres' ? null : 'genres'}><Filter size={18} /></button>
    <button class="icon-button" aria-label="Sorting options" aria-expanded={panel === 'sort'} aria-controls="sort-panel" onclick={() => panel = panel === 'sort' ? null : 'sort'}><ArrowUpDown size={18} /></button>
    <div class="search"><Search size={17} /><input type="search" aria-label="Search books or authors" placeholder="Find a book or author" maxlength="200" bind:value={search} /></div>
  </div>
  {#if panel === 'genres'}
    <fieldset id="genre-panel" class="panel genres" onkeydown={(event) => { if (event.key === 'Escape') panel = null; }}>
      <legend>Genres · match any selected</legend>
      <button class="clear" onclick={() => genre = []}>All genres</button>
      {#each genres as name}<label><input type="checkbox" value={name} bind:group={genre} /> {name}</label>{/each}
      <button class="clear" onclick={() => panel = null}>Done</button>
    </fieldset>
  {:else if panel === 'sort'}
    <div id="sort-panel" class="panel sorter" onkeydown={(event) => { if (event.key === 'Escape') panel = null; }} role="group" aria-label="Book sorting">
      <label for="book-sort">Sort by</label>
      <select id="book-sort" bind:value={sort}>
        <option value="recent">Recently read</option><option value="rating">Rating</option><option value="reviews">Reviews</option><option value="pages">Page count</option>
      </select>
      <button class="direction" aria-label={`Sort ${direction === 'asc' ? 'ascending' : 'descending'}; switch to ${direction === 'asc' ? 'descending' : 'ascending'}`} onclick={() => direction = direction === 'asc' ? 'desc' : 'asc'}>
        {#if direction === 'asc'}<ArrowUp size={16} />{:else}<ArrowDown size={16} />{/if}
      </button>
    </div>
  {/if}

  {#if signedIn && streak.pages_today < streak.pages_required}
    <div class="nudge">
      {streak.pages_required - streak.pages_today} more
      {streak.pages_required - streak.pages_today === 1 ? 'page' : 'pages'} today
      {streak.current > 0 ? `to keep your ${streak.current} day streak` : 'to start a streak'}
    </div>
  {/if}

  <p class="status" role="status">{message}</p>

  <section aria-labelledby="my-books-heading">
    <h2 id="my-books-heading" class="shelf-heading">My books</h2>
    {#if !signedIn && !loading}
      <p class="shelf-empty"><a href="/login">Sign in</a> to see your opened books.</p>
    {:else if loading}
      <div class="horizontal">{#each Array(3) as _}<div class="skeleton"></div>{/each}</div>
    {:else if openedError}
      <p role="alert">{openedError}</p><button class="load-more" onclick={() => retry++}>Try again</button>
    {:else if openedBooks.length === 0}
      <p class="shelf-empty">{search || genre.length ? 'No opened books match your filters.' : 'Open a book below and it will appear here.'}</p>
    {:else}
      <div class="horizontal" tabindex="0" role="region" aria-label="Opened books">
        {#each openedBooks as book (book.id)}{@render card(book)}{/each}
        {#if openedOffset !== null}<button class="load-more" disabled={loadingOpened} onclick={loadMoreOpened}>{loadingOpened ? 'Loading…' : 'More of my books'}</button>{/if}
      </div>
    {/if}
  </section>

  <section aria-labelledby="all-books-heading">
  <h2 id="all-books-heading" class="shelf-heading">All books</h2>

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
        {@render card(book)}
      {/each}
    </div>
    {#if nextOffset !== null}
      <button class="load-more" disabled={loadingMore} onclick={loadMore}>{loadingMore ? 'Loading…' : 'Load more books'}</button>
    {/if}
  {/if}
  </section>
</div>

{#snippet card(book: Book)}
  <BookCard {book}
    href={`/read/${encodeURIComponent(book.slug)}${book.last_card_idx != null ? '?card=' + book.last_card_idx : book.last_page_idx != null ? '?from=' + book.last_page_idx : ''}`}
    rating={ratings[book.slug] ?? 0} saving={saving[book.slug]} {signedIn}
    locked={locked(book)} onrate={(stars) => rate(book, stars)} />
{/snippet}

<style>
  .wrap { padding: calc(16px + env(safe-area-inset-top)) 16px 92px; max-width: 760px; margin: 0 auto; }
  .head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18px; }
  h1 { margin: 0; font-size: 1.3rem; font-weight: 600; letter-spacing: -0.02em; }
  .streak { display: flex; align-items: center; gap: 6px; padding: 7px 14px; border-radius: var(--radius-pill); background: #7e22ce; color: #fff; font-weight: 650; font-size: 0.86rem; }
  .streak.risk { animation: breathe 2.4s ease-in-out infinite; }
  @keyframes breathe { 50% { opacity: 0.72; } }
  .signin { padding: 8px 16px; border-radius: var(--radius-pill); border: 1px solid #15803d; background: transparent; color: #15803d; font-size: 0.85rem; }
  .signin:hover { background: #f0fdf4; color: #166534; border-color: #166534; }
  .nudge { padding: 11px 15px; margin-bottom: 16px; border-radius: var(--radius-sm); background: var(--bg-card); border: 1px solid var(--border); font-size: 0.84rem; color: var(--text-dim); }
  .discovery { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }
  .icon-button { display: flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; height: 44px; border: 1px solid #fff; border-radius: 50%; background: transparent; color: #fff; }
  .icon-button.active { background: #fff; color: #18181b; }
  .search { display: flex; align-items: center; gap: 10px; padding: 0 14px; min-width: 0; flex: 1; border-radius: var(--radius-pill); border: 1px solid var(--border); background: var(--bg-card); color: var(--text-faint); }
  .search input { flex: 1; width: 100%; min-width: 0; padding: 12px 0; border: none; background: transparent; color: var(--text); font-size: 0.9rem; font-family: inherit; }
  .search input:focus { outline: none; }
  .search:focus-within { border-color: var(--border-bright); }
  button:focus-visible, .horizontal:focus-visible { outline: 2px solid var(--text); outline-offset: 3px; }
  .panel { padding: 14px; margin: 0 0 16px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--bg-card); }
  .genres { display: flex; flex-wrap: wrap; gap: 12px 16px; max-height: 260px; overflow-y: auto; }
  .genres legend { color: var(--text-dim); font-size: 0.8rem; }
  .genres label { display: flex; align-items: center; gap: 6px; font-size: 0.85rem; }
  .clear { padding: 5px 10px; border: 1px solid var(--border); border-radius: var(--radius-pill); background: transparent; color: var(--text); }
  .sorter { display: flex; align-items: center; gap: 8px; font-size: 0.82rem; }
  .shelf-heading { font-size: 0.74rem; text-transform: uppercase; letter-spacing: 0.1em; color: var(--text-faint); margin: 26px 0 14px; font-weight: 600; }
  .horizontal { display: grid; grid-auto-flow: column; grid-auto-columns: 160px; gap: 20px; overflow-x: auto; overscroll-behavior-x: contain; scroll-snap-type: x proximity; padding: 3px 3px 14px; }
  .horizontal > :global(*) { min-width: 0; scroll-snap-align: start; }
  .shelf-empty { color: var(--text-faint); font-size: 0.85rem; padding: 12px 0; }
  .shelf-empty a { color: var(--text); text-decoration: underline; }
  .sorter select { min-width: 0; max-width: 145px; padding: 7px 8px; border: 1px solid var(--border); border-radius: var(--radius-pill); background: var(--bg); color: var(--text); font-size: 0.82rem; }
  .direction { display: flex; align-items: center; justify-content: center; width: 34px; height: 34px; border: 1px solid var(--border); border-radius: 50%; background: var(--bg); color: var(--text); }
  .direction:hover { border-color: var(--border-bright); }
  .status { font-size: 0.82rem; color: var(--text-dim); }
  .status:empty { display: none; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 20px; }
  .skeleton { min-height: 270px; border-radius: var(--radius); background: var(--bg-card); animation: pulse 1.4s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.5; } }
  .load-more { display: block; margin: 24px auto 0; padding: 12px 24px; border: 1px solid var(--border-bright); border-radius: var(--radius-pill); background: var(--bg); color: var(--text); }
  .load-more:disabled { opacity: 0.6; cursor: wait; }
  .empty { color: var(--text-faint); text-align: center; padding: 40px 0; }
</style>
