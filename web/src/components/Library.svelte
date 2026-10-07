<script lang="ts">
  import { onMount } from 'svelte';
  import ResumeReading from './ResumeReading.svelte';
  import { Flame, Lock } from '@lucide/svelte';
  import { api, ApiError, type Book } from '../lib/api';

  let books = $state<Book[]>([]);
  let loading = $state(true);
  let signedIn = $state(false);
  let streak = $state({ current: 0, at_risk: false, pages_today: 0, pages_required: 3 });
  let tier = $state<'free' | 'plus' | 'pro'>('free');
  let filter = $state<'all' | 'public_domain' | 'licensed'>('all');

  onMount(async () => {
    // The catalogue is public; the streak header only appears when signed in.
    const [booksRes, meRes] = await Promise.allSettled([api.books(), api.me()]);

    if (booksRes.status === 'fulfilled') books = booksRes.value.books;

    if (meRes.status === 'fulfilled') {
      signedIn = true;
      tier = meRes.value.user.tier;
      try {
        const s = await api.streak();
        streak = s;
      } catch {
        // A failed streak fetch must not blank the library.
      }
    }
    loading = false;
  });

  const shown = $derived(filter === 'all' ? books : books.filter((b) => b.license === filter));
  const locked = (b: Book) => b.license === 'licensed' && tier === 'free';

  /** Gutenberg ships almost no cover art, so generate one. The hue is derived
   * from the slug, which keeps a book's cover stable across sessions and
   * devices without storing anything. */
  function cover(slug: string) {
    let h = 0;
    for (let i = 0; i < slug.length; i++) h = (h * 31 + slug.charCodeAt(i)) >>> 0;
    // Narrow bands on purpose: wider ranges wander into yellow and blue, which
    // reads as a different product. Orange 20-44, purple 268-292, through pink.
    const a = 20 + (h % 24);
    const b = 268 + ((h >> 8) % 24);
    return `linear-gradient(145deg, hsl(${a} 94% 56%) 0%, hsl(${b} 78% 60%) 100%)`;
  }

  /** First letters of the title: a cheap, legible cover mark. */
  function initials(title: string) {
    return title
      .replace(/^(the|a|an)\s+/i, '')
      .split(/\s+/)
      .slice(0, 2)
      .map((w) => w[0])
      .join('')
      .toUpperCase();
  }
</script>

<div class="wrap">
  <header class="head">
    <h1><span class="brand-text">Bookworm</span></h1>

    {#if signedIn}
      <a href="/app/me" class="streak" class:risk={streak.at_risk}>
        <Flame size={16} />
        <span>{streak.current}</span>
      </a>
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

  <div class="filters">
    <button class:on={filter === 'all'} onclick={() => (filter = 'all')}>All</button>
    <button class:on={filter === 'public_domain'} onclick={() => (filter = 'public_domain')}>Free</button>
    <button class:on={filter === 'licensed'} onclick={() => (filter = 'licensed')}>Plus</button>
  </div>

  {#if loading}
    <div class="grid">
      {#each Array(4) as _}<div class="skeleton"></div>{/each}
    </div>
  {:else if shown.length === 0}
    <p class="empty">Nothing here yet.</p>
  {:else}
    <div class="grid">
      {#each shown as book (book.id)}
        <a class="card" href={`/read/${book.slug}`}>
          <div class="cover">
            {#if book.cover_url}
              <img src={book.cover_url} alt="" loading="lazy" />
            {:else}
              <div class="generated" style="background:{cover(book.slug)}">
                <span class="initials">{initials(book.title)}</span>
                <span class="spine"></span>
              </div>
            {/if}
            {#if locked(book)}
              <div class="lock"><Lock size={13} /> Plus</div>
            {/if}
          </div>
          <div class="meta">
            <div class="t">{book.title}</div>
            <div class="a">{book.author}</div>
            <div class="p">{book.page_count} pages</div>
          </div>
        </a>
      {/each}
    </div>
  {/if}
</div>

<style>
  .wrap {
    padding: 16px 16px 92px;
    padding-top: calc(16px + env(safe-area-inset-top));
    max-width: 760px;
    margin: 0 auto;
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 18px;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 750;
    letter-spacing: -0.02em;
  }

  .streak {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 7px 14px;
    border-radius: var(--radius-pill);
    background: var(--grad-warm);
    color: #fff;
    font-weight: 650;
    font-size: 0.86rem;
  }
  .streak.risk {
    animation: breathe 2.4s ease-in-out infinite;
  }
  @keyframes breathe {
    50% {
      opacity: 0.72;
    }
  }

  .signin {
    padding: 8px 16px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--border-bright);
    font-size: 0.85rem;
  }

  .nudge {
    padding: 11px 15px;
    margin-bottom: 16px;
    border-radius: var(--radius-sm);
    background: var(--grad-card);
    border: 1px solid var(--border);
    font-size: 0.84rem;
    color: var(--text-dim);
  }

  .filters {
    display: flex;
    gap: 8px;
    margin-bottom: 18px;
  }
  .filters button {
    padding: 7px 16px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-dim);
    font-size: 0.82rem;
  }
  .filters button.on {
    background: var(--text);
    color: var(--bg);
    border-color: var(--text);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 16px;
  }

  .card {
    display: block;
  }
  .cover {
    position: relative;
    aspect-ratio: 2 / 3;
    border-radius: var(--radius);
    overflow: hidden;
    background: var(--grad-card);
    border: 1px solid var(--border);
    margin-bottom: 9px;
  }
  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .generated {
    position: relative;
    width: 100%;
    height: 100%;
    display: grid;
    place-items: center;
    overflow: hidden;
  }
  .initials {
    font-size: 2.1rem;
    font-weight: 750;
    color: rgba(255, 255, 255, 0.94);
    letter-spacing: -0.03em;
    text-shadow: 0 2px 12px rgba(0, 0, 0, 0.22);
  }
  /* A hint of a book spine, so the tile reads as a cover and not a swatch. */
  .spine {
    position: absolute;
    left: 10%;
    top: 0;
    bottom: 0;
    width: 2px;
    background: rgba(0, 0, 0, 0.16);
  }

  .lock {
    position: absolute;
    top: 8px;
    right: 8px;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 9px;
    border-radius: var(--radius-pill);
    background: rgba(10, 10, 12, 0.82);
    backdrop-filter: blur(6px);
    font-size: 0.66rem;
    color: var(--orange);
    font-weight: 600;
  }

  .t {
    font-weight: 600;
    font-size: 0.88rem;
    line-height: 1.3;
  }
  .a {
    font-size: 0.78rem;
    color: var(--text-dim);
    margin-top: 2px;
  }
  .p {
    font-size: 0.7rem;
    color: var(--text-faint);
    margin-top: 3px;
  }

  .skeleton {
    aspect-ratio: 2 / 3;
    border-radius: var(--radius);
    background: var(--bg-card);
    animation: pulse 1.4s ease-in-out infinite;
  }
  @keyframes pulse {
    50% {
      opacity: 0.5;
    }
  }

  .empty {
    color: var(--text-faint);
    text-align: center;
    padding: 40px 0;
  }
</style>
