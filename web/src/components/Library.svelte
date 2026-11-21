<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { ArrowUpDown, Filter, Search, Flame, LoaderCircle } from '@lucide/svelte';
  import BookCard from './BookCard.svelte';
  import { api, type Book } from '../lib/api';

  let books = $state<Book[]>([]);
  let loading = $state(true);
  let signedIn = $state(false);
  let streak = $state({ current: 0, at_risk: false });
  let cardsLeft = $state(0);
  let tier = $state<'free' | 'plus' | 'pro'>('free');
  let openedBooks = $state<Book[]>([]);
  let openedError = $state('');
  let openedOffset = $state<number | null>(null);
  let loadingOpened = $state(false);
  let genresOpen = $state(false);
  let genreButton: HTMLButtonElement;
  let genreSelect: HTMLSelectElement;
  let sort = $state('recent');
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
      void api.streak().then(result => {
        streak = result;
        // The streak endpoint uses legacy field names but also counts card reads.
        cardsLeft = Math.max(0, result.pages_required - result.pages_today);
      }).catch(() => {});
      try { ratings = (await api.ratings()).ratings; }
      catch { message = 'Your ratings could not load. Please refresh before rating.'; }
    }
    ready = true;
  });

  $effect(() => {
    if (!(cardsLeft > 0)) return;
    const timer = setTimeout(() => { cardsLeft = 0; }, 3000);
    return () => clearTimeout(timer);
  });

  async function toggleGenres() {
    genresOpen = !genresOpen;
    await tick();
    if (genresOpen) genreSelect?.focus();
    else genreButton?.focus();
  }

  function query() {
    return { license: 'public_domain', genre, q: search.trim(), sort };
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
      retry++; // Refresh review aggregates.
      message = `Rated ${book.title} ${stars} out of 5 stars.`;
    } catch {
      message = 'Your rating could not be saved. Please try again.';
    } finally { saving[book.slug] = false; }
  }
</script>

<div class="wrap">
  <div class="library-controls">
  <header class="head">
    <h1 class="brand-heading"><img src="/glim.svg" alt="" width="32" height="32" /><span class="brand-text">Potpuri</span></h1>
    {#if signedIn}
      <a href="/app/me" class="streak" class:risk={streak.at_risk}><Flame size={16} /><span>{streak.current}</span></a>
    {:else}
      <a href="/login" class="signin">Sign in</a>
    {/if}
  </header>

  <div class="discovery">
    <div class="icon-button sort-control">
      <ArrowUpDown size={18} aria-hidden="true" />
      <select aria-label="Sort by" title="Sort by" bind:value={sort}>
        <option value="longest">Longest first</option>
        <option value="shortest">Shortest first</option>
        <option value="recent">Recently read</option>
      </select>
    </div>
    <div class="search"><Search size={17} /><input type="search" aria-label="Search books or authors" placeholder="Find a book or author" maxlength="200" bind:value={search} /></div>
    <button class="icon-button" class:active={genre.length > 0} aria-label={genre.length ? `Filter genres (${genre.length} selected)` : 'Filter genres'} aria-expanded={genresOpen} aria-controls="genre-panel" bind:this={genreButton} onclick={toggleGenres}><Filter size={18} /></button>
  </div>
  <fieldset id="genre-panel" class="genres" hidden={!genresOpen} onkeydown={(event) => { if (event.key === 'Escape') void toggleGenres(); }}>
    <legend>Genres · match any selected</legend>
    <select multiple size="5" aria-label="Genres" bind:this={genreSelect} bind:value={genre}>
      {#each genres as name}<option value={name}>{name}</option>{/each}
    </select>
    <div class="genre-actions">
      <button class="pill" onclick={() => genre = []}>All genres</button>
      <button class="pill" onclick={toggleGenres}>Done</button>
    </div>
  </fieldset>
  </div>

  {#if cardsLeft > 0}
    <div class="nudge">
      <span class="spinner"><LoaderCircle size={18} aria-hidden="true" /></span>
      <span role="status">{cardsLeft} {cardsLeft === 1 ? 'card' : 'cards'} left to {streak.current > 0 ? 'keep your streak' : 'start a streak'}</span>
      <button class="pill" onclick={() => cardsLeft = 0}>Cancel</button>
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
  .wrap { container-type: inline-size; padding: 0 16px 92px; max-width: 760px; margin: 0 auto; }
  .library-controls { position: sticky; top: 0; z-index: 20; margin: 0 -16px; padding: calc(16px + env(safe-area-inset-top)) 16px 1px; background: var(--bg); border-bottom: 1px solid var(--border); max-height: calc(100dvh - 80px - env(safe-area-inset-bottom)); overflow-y: auto; }
  .head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18px; }
  h1 { margin: 0; font-size: 1.3rem; font-weight: 600; letter-spacing: -0.02em; }
  .streak { display: flex; align-items: center; gap: 6px; padding: 7px 14px; border-radius: var(--radius-pill); background: #7e22ce; color: #fff; font-weight: 650; font-size: 0.86rem; }
  .streak.risk { animation: breathe 2.4s ease-in-out infinite; }
  @keyframes breathe { 50% { opacity: 0.72; } }
  .signin { padding: 8px 16px; border-radius: var(--radius-pill); border: 1px solid #15803d; background: transparent; color: #15803d; font-size: 0.85rem; }
  .signin:hover { background: #f0fdf4; color: #166534; border-color: #166534; }
  .nudge { position: fixed; z-index: 30; inset-inline: 16px; bottom: calc(76px + env(safe-area-inset-bottom)); max-width: 480px; margin-inline: auto; display: flex; align-items: center; gap: 10px; padding: 11px 15px; border-radius: var(--radius-sm); background: var(--bg-card); border: 1px solid var(--border); box-shadow: var(--shadow-lift); font-size: 0.84rem; color: var(--text-dim); }
  .nudge button { margin-inline-start: auto; }
  .spinner { flex: none; display: grid; place-items: center; animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spinner { animation: none; } }
  .discovery { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }
  .icon-button { display: flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; height: 44px; border: 1px solid #e5e7eb; border-radius: 50%; background: #fff; color: #000; }
  .icon-button.active { border-color: #9ca3af; box-shadow: 0 0 0 1px #9ca3af; }
  .search { display: flex; align-items: center; gap: 10px; padding: 0 14px; min-width: 0; flex: 1; border-radius: var(--radius-pill); border: 1px solid var(--border); background: var(--bg-card); color: var(--text-faint); }
  .search input { flex: 1; width: 100%; min-width: 0; padding: 12px 0; border: none; background: transparent; color: var(--text); font-size: 0.9rem; font-family: inherit; }
  .search input:focus { outline: none; }
  .search:focus-within { border-color: var(--border-bright); }
  button:focus-visible, .horizontal:focus-visible { outline: 2px solid var(--text); outline-offset: 3px; }
  .sort-control { position: relative; }
  .sort-control select { position: absolute; inset: 0; width: 100%; height: 100%; opacity: 0; cursor: pointer; }
  .sort-control:has(select:focus-visible) { outline: 2px solid var(--text); outline-offset: 3px; }
  .genres { min-width: 0; padding: 14px; margin: 0 0 16px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--bg-card); }
  .genres legend { color: var(--text-dim); font-size: 0.8rem; }
  .genres select { width: 100%; font: inherit; padding: 4px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--bg); color: var(--text); }
  .genre-actions { display: flex; justify-content: space-between; gap: 8px; margin-top: 8px; }
  .pill { padding: 7px 12px; border: 1px solid var(--border); border-radius: var(--radius-pill); background: transparent; color: var(--text); }
  .shelf-heading { font-size: 0.74rem; text-transform: uppercase; letter-spacing: 0.1em; color: var(--text-faint); margin: 26px 0 14px; font-weight: 600; }
  .horizontal, .grid { --card-columns: 2; --card-gap: 20px; display: grid; gap: var(--card-gap); }
  @container (max-width: 319px) { .horizontal, .grid { --card-columns: 1; } }
  @container (min-width: 490px) { .horizontal, .grid { --card-columns: 3; } }
  @container (min-width: 660px) { .horizontal, .grid { --card-columns: 4; } }
  .horizontal { grid-auto-flow: column; grid-auto-columns: calc((100% - (var(--card-columns) - 1) * var(--card-gap)) / var(--card-columns)); overflow-x: auto; overscroll-behavior-x: contain; scroll-snap-type: x proximity; padding: 3px 0 14px; }
  .horizontal > :global(*) { min-width: 0; scroll-snap-align: start; }
  .shelf-empty { color: var(--text-faint); font-size: 0.85rem; padding: 12px 0; }
  .shelf-empty a { color: var(--text); text-decoration: underline; }
  .status { font-size: 0.82rem; color: var(--text-dim); }
  .status:empty { display: none; }
  .grid { grid-template-columns: repeat(var(--card-columns), minmax(0, 1fr)); }
  .skeleton { min-height: 270px; border-radius: var(--radius); background: var(--bg-card); animation: pulse 1.4s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.5; } }
  .load-more { display: block; margin: 24px auto 0; padding: 12px 24px; border: 1px solid var(--border-bright); border-radius: var(--radius-pill); background: var(--bg); color: var(--text); }
  .load-more:disabled { opacity: 0.6; cursor: wait; }
  .empty { color: var(--text-faint); text-align: center; padding: 40px 0; }
</style>
