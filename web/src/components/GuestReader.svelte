<script lang="ts">
  import { onMount } from 'svelte';
  import { books, type GuestBook } from '../lib/guest-books';
  import Reader from './Reader.svelte';
  let book = $state<GuestBook | null>(null);
  let ready = $state(false);
  onMount(() => {
    const slug = new URLSearchParams(window.location.search).get('book');
    book = books.find(b => b.slug === slug) ?? null;
    ready = true;
  });
</script>

{#if book}
  <Reader slug={book.slug} title={book.title} author={book.author} preview={book} />
{:else if ready}
  <main><h1>Choose a book to try</h1>
    {#each books as sample}<p><a href={`/try/?book=${sample.slug}`}>{sample.title} →</a></p>{/each}
    <a href="/#try-it-now">Back to the books</a>
  </main>
{/if}

<style>main { max-width: 760px; padding: 24px 16px; margin: auto; }</style>
