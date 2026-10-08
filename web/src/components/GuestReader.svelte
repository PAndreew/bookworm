<script lang="ts">
  import { onMount } from 'svelte';
  import books from '../lib/guest-books.json';
  import GuestPuri from './GuestPuri.svelte';
  let book = $state<(typeof books)[number] | null>(null);
  let index = $state(0);
  let finished = $state(false);
  let selection = $state('');
  let text = $state<HTMLElement | null>(null);
  const card = $derived(book?.pages[index]);
  const match = $derived(card?.word ? new RegExp(`\\b${card.word}\\b`, 'i').exec(card.content) : null);
  function move(direction: number) {
    if (!book) return;
    if (finished && direction < 0) { finished = false; return; }
    if (direction > 0 && index === book.pages.length - 1) { finished = true; return; }
    index = Math.max(0, Math.min(book.pages.length - 1, index + direction));
    selection = '';
    window.getSelection()?.removeAllRanges();
    text?.scrollTo(0, 0);
  }
  onMount(() => {
    const slug = new URLSearchParams(window.location.search).get('book');
    book = books.find(b => b.slug === slug) ?? null;
    function capture() {
      const selected = window.getSelection();
      if (!selected?.rangeCount || selected.isCollapsed || !text) return;
      const range = selected.getRangeAt(0);
      if (text.contains(range.startContainer) && text.contains(range.endContainer)) selection = selected.toString().trim().slice(0, 200);
    }
    function key(event: KeyboardEvent) {
      if (event.shiftKey || event.ctrlKey || event.altKey || event.metaKey) return;
      if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
        event.preventDefault();
        move(event.key === 'ArrowRight' ? 1 : -1);
      }
    }
    document.addEventListener('selectionchange', capture);
    window.addEventListener('keydown', key);
    return () => {
      document.removeEventListener('selectionchange', capture);
      window.removeEventListener('keydown', key);
    };
  });
</script>

<main class="guest-reader">
  <a class="back" href="/#try-it-now">← Back to the books</a>
  {#if book && card}
    <header>
      <h1>{book.title}</h1>
      <p>{book.author} · Free preview · No account needed</p>
    </header>
    {#if finished}
      <section class="finish" aria-label="Preview complete">
        <h2>That’s your first 10 cards!</h2>
        <p>Keep reading for free. Create an account to save your place and discover more books.</p>
        <a class="primary" href="/signup">Create an account</a>
        <a href="/login">Already have an account? Sign in</a>
        <button onclick={() => move(-1)}>Back to the last card</button>
      </section>
    {:else}
      <article class:title-card={card.kind === 'title'} aria-label={`Reading card ${index + 1}`}>
        <div class="book-text" bind:this={text}>
          {#if match}
            {card.content.slice(0, match.index)}<span data-demo-word>{match[0]}</span>{card.content.slice(match.index + match[0].length)}
          {:else}
            {card.content}
          {/if}
        </div>
      </article>
      <nav aria-label="Reading navigation">
        <button aria-label="Previous card" disabled={index === 0} onclick={() => move(-1)}>←</button>
        <span aria-live="polite">{index + 1} / {book.pages.length}</span>
        <button aria-label={index === book.pages.length - 1 ? 'Finish preview' : 'Next card'} onclick={() => move(1)}>→</button>
      </nav>
      <GuestPuri word={card.word} definition={card.definition} {selection} />
    {/if}
  {:else}
    <h1>Choose a book to try</h1>
    <p>Ten cards, a word to discover, and no account needed.</p>
    {#each books as sample}<a class="sample-link" href={`/try/?book=${sample.slug}`}>{sample.title} →</a>{/each}
  {/if}
</main>

<style>
  .guest-reader { max-width: 620px; min-height: 100dvh; margin: 0 auto; padding: 24px 20px 32px; }
  .back { font-size: 0.85rem; color: var(--text-dim); }
  header { margin: 24px 0 18px; }
  h1 { margin: 0 0 8px; font-size: 1.2rem; line-height: 1.4; }
  header p { margin: 0; font-size: 0.75rem; color: var(--text-faint); }
  article { min-height: 230px; padding: 24px; border: 1px solid var(--border); border-radius: 20px; background: #fff; }
  .book-text { white-space: pre-wrap; font-family: Georgia, serif; font-size: 1.12rem; line-height: 1.75; overflow-wrap: anywhere; }
  .title-card { display: flex; align-items: center; justify-content: center; text-align: center; }
  .title-card .book-text { font-size: 1.45rem; }
  [data-demo-word] { text-decoration: underline dotted; text-decoration-color: #75cfc5; text-underline-offset: 4px; }
  nav { display: flex; align-items: center; justify-content: center; gap: 32px; margin: 16px 0 22px; }
  nav span { color: var(--text-dim); font-size: 0.85rem; }
  nav button, .finish button { border: 1px solid var(--border); background: #fff; border-radius: 999px; padding: 8px 18px; cursor: pointer; }
  nav button:disabled { opacity: 0.35; cursor: default; }
  .finish { text-align: center; padding: 40px 12px; }
  .finish h2 { font-size: 1.5rem; }
  .finish p { color: var(--text-dim); line-height: 1.6; }
  .finish a, .finish button { display: block; width: fit-content; margin: 20px auto; }
  .primary { padding: 12px 24px; border-radius: 999px; background: var(--accent); color: #fff; font-weight: 600; }
  .sample-link { display: block; margin-top: 24px; }
  @media (max-width: 380px) { article { padding: 18px; } .book-text { font-size: 1rem; } }
</style>
