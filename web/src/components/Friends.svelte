<script lang="ts">
  import { onMount } from 'svelte';
  import { UserPlus, Check, X, Search } from '@lucide/svelte';
  import { api, ApiError } from '../lib/api';
  import Avatar from './Avatar.svelte';

  let friends = $state<any[]>([]);
  const rankedFriends = $derived([...friends].sort((a, b) => (b.common_books ?? 0) - (a.common_books ?? 0) || a.handle.localeCompare(b.handle)));
  let pending = $state<any[]>([]);
  let loading = $state(true);
  let needsAuth = $state(false);

  let query = $state('');
  let results = $state<any[]>([]);
  let searching = $state(false);
  let toast = $state<string | null>(null);

  onMount(load);

  async function load() {
    try {
      const [f, p] = await Promise.all([api.friends(), api.pendingFriends()]);
      friends = f.friends;
      pending = p.pending;
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) needsAuth = true;
    } finally {
      loading = false;
    }
  }

  let searchTimer: number;
  function onSearch() {
    clearTimeout(searchTimer);
    if (query.trim().length < 2) {
      results = [];
      return;
    }
    // Debounced: a request per keystroke would hammer the endpoint.
    searchTimer = window.setTimeout(async () => {
      searching = true;
      try {
        results = (await api.searchUsers(query.trim())).users;
      } catch {
        results = [];
      } finally {
        searching = false;
      }
    }, 280);
  }

  async function act(fn: () => Promise<unknown>, msg: string) {
    try {
      await fn();
      show(msg);
      await load();
    } catch (e) {
      show(e instanceof ApiError ? e.message : 'Something went wrong.');
    }
  }

  function show(m: string) {
    toast = m;
    setTimeout(() => (toast = null), 2400);
  }

</script>

<div class="wrap">
  <h1>Friends</h1>

  {#if needsAuth}
    <p class="empty">
      <a href="/login" class="link">Sign in</a> to add friends and compare streaks.
    </p>
  {:else if loading}
    <p class="empty">Loading…</p>
  {:else}
    <div class="search">
      <Search size={17} />
      <input bind:value={query} oninput={onSearch} placeholder="Find people by handle" />
    </div>

    {#if results.length > 0}
      <section>
        <h2>Results</h2>
        {#each results as u (u.id)}
          <div class="row">
            <Avatar seed={u.id ?? u.handle} />
            <div class="who">
              <div class="name">{u.display_name}</div>
              <div class="handle">@{u.handle}</div>
            </div>
            <button class="pill" onclick={() => act(() => api.requestFriend(u.handle), 'Request sent.')}>
              <UserPlus size={15} /> Add
            </button>
          </div>
        {/each}
      </section>
    {:else if searching}
      <p class="empty">Searching…</p>
    {/if}

    {#if pending.length > 0}
      <section>
        <h2>Requests</h2>
        {#each pending as p (p.id)}
          <div class="row">
            <Avatar seed={p.id ?? p.handle} />
            <div class="who">
              <div class="name">{p.display_name}</div>
              <div class="handle">@{p.handle} · {p.incoming ? 'wants to be friends' : 'request sent'}</div>
            </div>
            {#if p.incoming}
              <div class="btns">
                <button class="pill ok" onclick={() => act(() => api.acceptFriend(p.handle), 'Friends now.')}>
                  <Check size={15} />
                </button>
                <button class="pill no" onclick={() => act(() => api.removeFriend(p.handle), 'Declined.')}>
                  <X size={15} />
                </button>
              </div>
            {:else}
              <button class="pill no" onclick={() => act(() => api.removeFriend(p.handle), 'Cancelled.')}>
                Cancel
              </button>
            {/if}
          </div>
        {/each}
      </section>
    {/if}

    {#if friends.length > 0}
      <section>
        <h2>Your friends</h2>
        {#each rankedFriends as f (f.id)}
          <a class="row friend" href={`/app/u/${encodeURIComponent(f.handle)}`}>
            <Avatar seed={f.id ?? f.handle} />
            <div class="who">
              <div class="identity"><span class="name">{f.display_name}</span><span class="handle">@{f.handle}</span></div>
              {#if f.latest_book_title}<div class="latest-book">{f.latest_book_title}</div>{/if}
            </div>
            <div class="metric">
              <strong>{f.common_books ?? 0}</strong><span>books in common</span>
            </div>
          </a>
        {/each}
      </section>
    {/if}

    {#if friends.length === 0 && pending.length === 0}
      <p class="empty">Find a friend and see what they are reading.</p>
    {/if}
  {/if}

  {#if toast}<div class="toast">{toast}</div>{/if}
</div>

<style>
  .wrap {
    padding: 16px 16px 92px;
    padding-top: calc(16px + env(safe-area-inset-top));
    max-width: 620px;
    margin: 0 auto;
  }
  h1 {
    margin: 0 0 18px;
    font-size: 1.5rem;
    font-weight: 740;
    letter-spacing: -0.02em;
  }
  h2 {
    font-size: 0.74rem;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-faint);
    margin: 26px 0 10px;
    font-weight: 600;
  }

  .search {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 14px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--border);
    background: var(--bg-card);
    color: var(--text-faint);
  }
  .search input {
    flex: 1;
    padding: 12px 0;
    border: none;
    background: transparent;
    color: var(--text);
    font-size: 0.9rem;
    font-family: inherit;
  }
  .search input:focus {
    outline: none;
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 12px 14px;
    border-radius: var(--radius-sm);
    background: var(--bg-card);
    border: 1px solid var(--border);
    margin-bottom: 8px;
  }
  .row.friend {
    padding: 12px 0;
    border: none;
    border-radius: 0;
    background: transparent;
  }
  .who { flex: 1; min-width: 0; }
  .identity { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 8px; }
  .handle { overflow-wrap: anywhere; }
  .latest-book { margin-top: 4px; font-size: .8rem; color: var(--text-dim); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .metric { text-align: right; flex-shrink: 0; }
  .metric strong { display: flex; justify-content: flex-end; align-items: center; gap: 4px; font-size: 1rem; }
  .metric span { display: block; font-size: .65rem; color: var(--text-faint); }
  .name {
    overflow-wrap: anywhere;
    font-weight: 600;
    font-size: 0.9rem;
  }
  .handle {
    font-size: 0.76rem;
    color: var(--text-faint);
  }

  .btns {
    display: flex;
    gap: 7px;
  }
  .pill {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 7px 13px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--border-bright);
    background: transparent;
    color: var(--text);
    font-size: 0.8rem;
  }
  .pill.ok {
    border-color: var(--success);
    color: var(--success);
  }
  .pill.no {
    border-color: var(--border-bright);
    color: var(--text-faint);
  }

  .empty {
    color: var(--text-faint);
    text-align: center;
    padding: 36px 0;
    font-size: 0.9rem;
  }
  .link {
    color: var(--orange);
  }

  .toast {
    position: fixed;
    bottom: calc(84px + env(safe-area-inset-bottom));
    left: 50%;
    transform: translateX(-50%);
    padding: 11px 18px;
    border-radius: var(--radius-pill);
    background: var(--bg-raised);
    border: 1px solid var(--border-bright);
    font-size: 0.84rem;
    box-shadow: var(--shadow-lift);
  }
</style>
