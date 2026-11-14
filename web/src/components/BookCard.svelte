<script lang="ts">
  import { Lock, Star } from '@lucide/svelte';
  import type { Book } from '../lib/api';
  type CardBook = Pick<Book, 'slug' | 'title' | 'author' | 'page_count' | 'genres' | 'license'> & Partial<Book>;
  let { book, href, rating = 0, saving = false, signedIn = false, locked = false, showHistory = true, onrate }: {
    book: CardBook; href: string; rating?: number; saving?: boolean; signedIn?: boolean;
    locked?: boolean; showHistory?: boolean; onrate?: (stars: number) => void;
  } = $props();
  function cardTitle(title: string) {
    const characters = Array.from(title.replace(/\s+/g, ' ').trim());
    return characters.length > 30 ? `${characters.slice(0, 30).join('').trimEnd()}…` : characters.join('');
  }
  function cover(slug: string) {
    let h = 0;
    for (let i = 0; i < slug.length; i++) h = (h * 31 + slug.charCodeAt(i)) >>> 0;
    return `hsl(${h % 360} 65% 91%)`;
  }
</script>

<article class="card" class:latest={showHistory && book.is_latest} style="background:{cover(book.slug)}">
  <a class="book-link" {href}>
    <h2 class="t" title={book.title} aria-label={book.title}>{cardTitle(book.title)}</h2>
    <div class="a">{book.author}</div>
    {#if book.card_count}<div class="p">{book.card_count} {book.card_count === 1 ? 'card' : 'cards'}</div>{/if}
    {#if showHistory && book.last_read_at}<div class="resume-label">{book.is_latest ? 'Continue reading' : 'Opened before'}</div>{/if}
    {#if book.rating_count}<div class="p">★ {(book.rating_average ?? 0).toFixed(1)} · {book.rating_count} {book.rating_count === 1 ? 'review' : 'reviews'}</div>{/if}
    <div class="genres">{#each book.genres as tag}<span>#{tag}</span>{/each}</div>
  </a>
  <div class="rating" role="group" aria-label={`Your rating for ${book.title}`}>
    {#each [1, 2, 3, 4, 5] as stars}
      <button class:filled={rating >= stars} disabled={saving}
        aria-label={`${signedIn ? 'Rate' : 'Sign in to rate'} ${book.title} ${stars} out of 5 stars`}
        aria-pressed={rating === stars}
        onclick={() => { if (onrate) onrate(stars); else window.location.href = '/login'; }}>
        <Star size={18} fill={rating >= stars ? 'currentColor' : 'none'} />
      </button>
    {/each}
  </div>
  <div class="card-footer">
    {#if locked}<a href="/app/upgrade" class="lock"><Lock size={12} /> Plus</a>{/if}
  </div>
</article>

<style>
  .card { display: flex; flex-direction: column; min-height: 270px; padding: 18px; color: #000; border: 1px solid transparent; border-radius: var(--radius); box-shadow: 0 2px 6px rgba(17,24,39,0.04); transition: box-shadow 0.2s, border-color 0.2s; }
  .card:hover { border-color: var(--border-bright); box-shadow: var(--shadow-lift); }
  .card.latest, .card.latest:hover { border: 2px solid #16a34a; padding: 17px; }
  .book-link { flex: 1; display: block; overflow-wrap: anywhere; }
  .t { margin: 0; font-family: var(--font-heading); font-weight: 400; font-synthesis: none; font-size: 1.05rem; line-height: 1.35; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden; }
  .a { font-size: 0.78rem; margin-top: 9px; }
  .p { font-size: 0.7rem; margin-top: 6px; }
  .genres { display: flex; flex-wrap: wrap; gap: 5px 8px; margin: 14px 0; font-size: 0.68rem; line-height: 1.5; }
  .rating { display: flex; flex-wrap: wrap; gap: 0; }
  .rating button { display: grid; place-items: center; min-width: 24px; min-height: 44px; padding: 0; border: 0; background: transparent; color: #374151; }
  .rating button.filled, .rating button:hover { color: #854d0e; }
  .rating button:disabled { cursor: wait; opacity: 0.6; }
  .card-footer { display: flex; justify-content: flex-end; align-items: flex-end; min-height: 26px; }
  .lock { display: flex; align-items: center; gap: 4px; padding: 4px 9px; border-radius: var(--radius-pill); background: var(--bg); font-size: 0.66rem; color: var(--accent); font-weight: 600; }
  .resume-label { margin-top: 8px; font-size: 0.72rem; color: #166534; font-weight: 600; }
</style>
