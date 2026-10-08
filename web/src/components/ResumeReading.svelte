<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  let position = $state<{ slug: string; title: string; page_idx: number; card_idx: number | null } | null>(null);
  onMount(async () => {
    try { position = (await api.me()).last_read; }
    catch { /* Signed-out readers and network failures leave the shelf usable. */ }
  });
</script>

{#if position}
  <a class="resume" href={`/read/${encodeURIComponent(position.slug)}?${position.card_idx == null ? 'from=' + position.page_idx : 'card=' + position.card_idx}`}>
    <strong>Resume reading</strong>
    <span>{position.title} · {position.card_idx == null ? 'Saved position' : 'Card ' + (position.card_idx + 1)}</span>
  </a>
{/if}

<style>
  .resume { display: flex; flex-direction: column; gap: 4px; padding: 14px 0; margin: 16px 0; color: var(--text); }
  .resume:hover strong { text-decoration: underline; text-underline-offset: 3px; }
  strong { color: var(--accent); }
  span { font-size: 0.85rem; color: var(--text-dim); }
</style>
