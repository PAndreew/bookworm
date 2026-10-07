<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  let position = $state<{ slug: string; title: string; page_idx: number } | null>(null);
  onMount(async () => {
    try { position = (await api.me()).last_read; }
    catch { /* Signed-out readers and network failures leave the shelf usable. */ }
  });
</script>

{#if position}
  <a class="resume" href={`/read/${encodeURIComponent(position.slug)}?from=${position.page_idx}`}>
    <strong>Resume reading</strong>
    <span>{position.title} · Page {position.page_idx + 1}</span>
  </a>
{/if}

<style>
  .resume { display: flex; flex-direction: column; gap: 4px; padding: 14px 20px; margin: 16px 0; border-radius: var(--radius-sm); background: var(--card-blue); border: 1px solid var(--border); color: var(--text); }
  .resume:hover { border-color: var(--accent); }
  strong { color: var(--accent); }
  span { font-size: 0.85rem; color: var(--text-dim); }
</style>
