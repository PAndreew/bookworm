<script lang="ts">
  import { onMount } from 'svelte';
  import { Share2, Trash2 } from '@lucide/svelte';
  import { api, ApiError } from '../lib/api';

  let items = $state<any[]>([]);
  let loading = $state(true);
  let needsAuth = $state(false);
  let toast = $state<string | null>(null);

  onMount(load);

  async function load() {
    try {
      items = (await api.highlights()).highlights;
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) needsAuth = true;
    } finally {
      loading = false;
    }
  }

  async function share(h: any) {
    if (!h.share_slug) {
      show('This quote is private. Re-highlight with share to get a link.');
      return;
    }
    const url = `${location.origin}/q/${h.share_slug}`;
    if (navigator.share) {
      await navigator.share({ title: h.book_title, text: h.quote, url }).catch(() => {});
    } else {
      await navigator.clipboard.writeText(url);
      show('Link copied.');
    }
  }

  async function remove(h: any) {
    try {
      await api.deleteHighlight(h.id);
      items = items.filter((x) => x.id !== h.id);
    } catch {
      show('Could not delete that.');
    }
  }

  function show(m: string) {
    toast = m;
    setTimeout(() => (toast = null), 2400);
  }
</script>

<div class="wrap">
  <h1>Your quotes</h1>

  {#if needsAuth}
    <p class="empty"><a href="/login" class="link">Sign in</a> to keep quotes.</p>
  {:else if loading}
    <p class="empty">Loading…</p>
  {:else if items.length === 0}
    <p class="empty">Select text while reading to keep it here.</p>
  {:else}
    {#each items as h (h.id)}
      <article class="q">
        <blockquote>{h.quote}</blockquote>
        <div class="src">{h.book_title} <span class="dim">· {h.author}</span></div>
        <div class="acts">
          <button onclick={() => share(h)} aria-label="Share"><Share2 size={16} /></button>
          <button onclick={() => remove(h)} aria-label="Delete"><Trash2 size={16} /></button>
        </div>
      </article>
    {/each}
  {/if}

  {#if toast}<div class="toast">{toast}</div>{/if}
</div>

<style>
  .wrap { padding: 16px 16px 92px; padding-top: calc(16px + env(safe-area-inset-top)); max-width: 620px; margin: 0 auto; }
  h1 { margin: 0 0 18px; font-size: 1.5rem; font-weight: 740; letter-spacing: -0.02em; }
  .q { padding: 17px; border-radius: var(--radius); background: var(--grad-card); border: 1px solid var(--border); margin-bottom: 11px; }
  blockquote { margin: 0 0 11px; font-family: var(--font-read); font-size: 1.02rem; line-height: 1.6; border-left: 2px solid var(--orange); padding-left: 13px; }
  .src { font-size: 0.8rem; color: var(--text-dim); }
  .dim { color: var(--text-faint); }
  .acts { display: flex; gap: 8px; margin-top: 12px; }
  .acts button { display: grid; place-items: center; width: 34px; height: 34px; border-radius: var(--radius-pill); border: 1px solid var(--border); background: transparent; color: var(--text-faint); }
  .empty { color: var(--text-faint); text-align: center; padding: 46px 0; font-size: 0.9rem; }
  .link { color: var(--orange); }
  .toast { position: fixed; bottom: calc(84px + env(safe-area-inset-bottom)); left: 50%; transform: translateX(-50%); padding: 11px 18px; border-radius: var(--radius-pill); background: var(--bg-raised); border: 1px solid var(--border-bright); font-size: 0.84rem; }
</style>
