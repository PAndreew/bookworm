<script lang="ts">
  import { onMount } from 'svelte';
  import { Flame, UserPlus, Check, X, Search } from '@lucide/svelte';
  import { api, ApiError } from '../lib/api';

  let friends = $state<any[]>([]);
  let pending = $state<any[]>([]);
  let activity = $state<any[]>([]);
  let loading = $state(true);
  let needsAuth = $state(false);

  let query = $state('');
  let results = $state<any[]>([]);
  let searching = $state(false);
  let toast = $state<string | null>(null);

  onMount(load);

  async function load() {
    try {
      const [f, p, a] = await Promise.all([api.friends(), api.pendingFriends(), api.friendFeed()]);
      friends = f.friends;
      pending = p.pending;
      activity = a.activity;
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

  function ago(iso: string) {
    const mins = Math.floor((Date.now() - new Date(iso).getTime()) / 60000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins}m ago`;
    const h = Math.floor(mins / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.floor(h / 24)}d ago`;
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
        {#each friends as f (f.id)}
          <a class="row" href={`/app/u/${f.handle}`}>
            <div class="who">
              <div class="name">{f.display_name}</div>
              <div class="handle">@{f.handle}</div>
            </div>
            {#if f.streak_days > 0}
              <div class="streak"><Flame size={14} /> {f.streak_days}</div>
            {/if}
          </a>
        {/each}
      </section>
    {/if}

    {#if activity.length > 0}
      <section>
        <h2>Reading now</h2>
        {#each activity as a}
          <a class="act" href={`/read/${a.book_slug}`}>
            <div class="act-top">
              <strong>{a.display_name}</strong>
              <span class="when">{ago(a.last_read_at)}</span>
            </div>
            <div class="act-book">{a.book_title} <span class="dim">· {a.author}</span></div>
            <div class="bar"><div class="fill" style="width:{Math.round(a.progress * 100)}%"></div></div>
          </a>
        {/each}
      </section>
    {:else if friends.length === 0 && pending.length === 0}
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
  .name {
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

  .streak {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 5px 11px;
    border-radius: var(--radius-pill);
    background: var(--grad-warm);
    color: #fff;
    font-size: 0.8rem;
    font-weight: 650;
  }

  .act {
    display: block;
    padding: 13px 15px;
    border-radius: var(--radius-sm);
    background: var(--grad-card);
    border: 1px solid var(--border);
    margin-bottom: 8px;
  }
  .act-top {
    display: flex;
    justify-content: space-between;
    font-size: 0.84rem;
    margin-bottom: 4px;
  }
  .when {
    color: var(--text-faint);
    font-size: 0.74rem;
  }
  .act-book {
    font-size: 0.86rem;
    color: var(--text-dim);
    margin-bottom: 9px;
  }
  .dim {
    color: var(--text-faint);
  }
  .bar {
    height: 3px;
    border-radius: 2px;
    background: var(--border);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    background: var(--grad-brand);
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
