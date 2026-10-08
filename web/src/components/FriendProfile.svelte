<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import Avatar from './Avatar.svelte';
  let profile = $state<any>(null);
  let loading = $state(true);
  let error = $state('');
  let needsAuth = $state(false);
  onMount(async () => {
    const handle = new URLSearchParams(location.search).get('handle') ?? location.pathname.split('/')[3] ?? '';
    try {
      if (!/^[a-z0-9_]{3,24}$/.test(handle)) throw new Error('Choose a friend to view their profile.');
      profile = (await api.profile(handle)).profile;
    } catch (e) {
      needsAuth = e instanceof ApiError && e.status === 401;
      error = e instanceof Error ? e.message : 'Could not load this profile.';
    } finally { loading = false; }
  });
</script>

<main class="wrap">
  <a class="back" href="/app/friends">← Friends</a>
  {#if loading}<p>Loading…</p>
  {:else if needsAuth}<p><a href="/login">Sign in</a> to view this profile.</p>
  {:else if error}<p role="alert">{error}</p>
  {:else if profile}
    <header><Avatar seed={profile.id ?? profile.handle} size={64} /><div><h1>{profile.display_name}</h1><p>@{profile.handle}</p></div></header>
    <dl class="stats">
      <div><dt>Current streak</dt><dd>{profile.streak_days} <small>days</small></dd></div>
      <div><dt>Longest streak</dt><dd>{profile.longest_days} <small>days</small></dd></div>
      <div><dt>Books finished</dt><dd>{profile.books_read}</dd></div>
    </dl>
    <h2>Reading</h2>
    {#each profile.reading as book}
      <a class="book" href={`/read/${encodeURIComponent(book.book_slug)}`}>
        <strong>{book.book_title}</strong><span>{book.author}</span>
        <div class="progress" role="progressbar" aria-label={`${book.book_title} progress`} aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(Math.max(0,Math.min(1,book.progress))*100)}><div style={`width:${Math.max(0,Math.min(1,book.progress))*100}%`}></div></div>
      </a>
    {:else}<p class="muted">{profile.visibility === 'private' ? 'This reading list is private.' : 'No shared reading yet.'}</p>{/each}
  {/if}
</main>
<style>
  .wrap { max-width: 620px; margin: 0 auto; padding: 24px 16px 100px; }
  .back { color: var(--text-dim); font-size: .85rem; }
  header { display: flex; align-items: center; gap: 16px; margin: 28px 0; }
  h1 { margin: 0; font-size: 1.5rem; overflow-wrap: anywhere; }
  header p { margin: 4px 0 0; color: var(--text-faint); }
  .stats { display: flex; flex-wrap: wrap; gap: 24px; margin: 0 0 32px; }
  .stats div { background: none; border: none; padding: 0; }
  dt { font-size: .75rem; color: var(--text-faint); }
  dd { margin: 6px 0 0; font-size: 1.4rem; font-weight: 650; }
  small { font-size: .75rem; font-weight: 400; color: var(--text-dim); }
  h2 { font-size: 1rem; }
  .book { display: block; padding: 16px 0; border-bottom: 1px solid var(--border); }
  .book span { display: block; margin-top: 4px; color: var(--text-dim); font-size: .85rem; }
  .progress { height: 3px; background: var(--border); margin-top: 12px; border-radius: 2px; overflow: hidden; }
  .progress div { height: 100%; background: #76b49b; }
  .muted { color: var(--text-faint); font-size: .9rem; }
</style>
