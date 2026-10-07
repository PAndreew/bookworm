<script lang="ts">
  import { onMount } from 'svelte';
  import { Flame, Shield, ShieldCheck, LogOut, Eye } from '@lucide/svelte';
  import { api, ApiError, type User } from '../lib/api';

  let user = $state<User | null>(null);
  let quota = $state({ limit: 0, used: 0, resets_at: '' });
  let streak = $state({ current: 0, longest: 0, at_risk: false, pages_today: 0, pages_required: 3 });
  let loading = $state(true);
  let needsAuth = $state(false);
  let toast = $state<string | null>(null);

  // 2FA enrolment state.
  let setupUri = $state<string | null>(null);
  let setupCode = $state('');
  let recoveryCodes = $state<string[] | null>(null);
  let disablePassword = $state('');
  let showDisable = $state(false);

  onMount(load);

  async function load() {
    try {
      const me = await api.me();
      user = me.user;
      quota = me.quota;
      const s = await api.streak();
      streak = s;
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) needsAuth = true;
    } finally {
      loading = false;
    }
  }

  async function begin2FA() {
    try {
      const res = await api.setup2FA();
      setupUri = res.provisioning_uri;
    } catch (e) {
      show(e instanceof ApiError ? e.message : 'Could not start setup.');
    }
  }

  async function confirm2FA() {
    try {
      const res = await api.enable2FA(setupCode);
      recoveryCodes = res.recovery_codes;
      setupUri = null;
      setupCode = '';
      await load();
    } catch (e) {
      show(e instanceof ApiError ? e.message : 'That code did not work.');
    }
  }

  async function turnOff2FA() {
    try {
      await api.disable2FA(disablePassword);
      disablePassword = '';
      showDisable = false;
      show('Two-factor is off.');
      await load();
    } catch (e) {
      show(e instanceof ApiError ? e.message : 'Could not turn it off.');
    }
  }

  async function setVisibility(v: string) {
    try {
      const res = await api.updateMe({ visibility: v });
      user = res.user;
      show('Saved.');
    } catch (e) {
      show(e instanceof ApiError ? e.message : 'Could not save.');
    }
  }

  async function signOut() {
    await api.logout().catch(() => {});
    location.href = '/';
  }

  function show(m: string) {
    toast = m;
    setTimeout(() => (toast = null), 2400);
  }

  /** The otpauth URI rendered as a QR via a public chart service is a privacy leak,
   * so the secret is shown as text for manual entry instead. */
  const secretFromUri = $derived(
    setupUri ? (new URL(setupUri).searchParams.get('secret') ?? '') : '',
  );
</script>

<div class="wrap">
  {#if needsAuth}
    <p class="empty"><a href="/login" class="link">Sign in</a> to see your profile.</p>
  {:else if loading}
    <p class="empty">Loading…</p>
  {:else if user}
    <header class="head">
      <div>
        <h1>{user.display_name}</h1>
        <div class="handle">@{user.handle}</div>
      </div>
      <div class="tier">{user.tier}</div>
    </header>

    <div class="stats">
      <div class="stat">
        <div class="v"><Flame size={17} /> {streak.current}</div>
        <div class="l">day streak</div>
      </div>
      <div class="stat">
        <div class="v">{streak.longest}</div>
        <div class="l">longest</div>
      </div>
      <div class="stat">
        <div class="v">{streak.pages_today}</div>
        <div class="l">pages today</div>
      </div>
    </div>

    {#if quota.limit > 0}
      <section class="box">
        <h2>Today's Plus pages</h2>
        <div class="bar"><div class="fill" style="width:{(quota.used / quota.limit) * 100}%"></div></div>
        <p class="dim">
          {quota.used} of {quota.limit} used · resets at
          {new Date(quota.resets_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
        </p>
      </section>
    {:else}
      <section class="box">
        <h2>Free plan</h2>
        <p class="dim">Every public-domain book, with no limit. Plus adds 30 pages a day of newer titles.</p>
        <a class="cta" href="/app/upgrade">See Plus and Pro</a>
      </section>
    {/if}

    <section class="box">
      <h2><Eye size={15} /> Who can see your reading</h2>
      <div class="seg">
        {#each [['public', 'Anyone'], ['friends', 'Friends'], ['private', 'Only me']] as [v, label]}
          <button class:on={user.visibility === v} onclick={() => setVisibility(v)}>{label}</button>
        {/each}
      </div>
    </section>

    <section class="box">
      <h2>
        {#if user.totp_enabled}<ShieldCheck size={15} />{:else}<Shield size={15} />{/if}
        Two-factor authentication
      </h2>

      {#if recoveryCodes}
        <p class="warn">
          Save these recovery codes now. Each works once, and you will not see them again.
        </p>
        <div class="codes">
          {#each recoveryCodes as c}<code>{c}</code>{/each}
        </div>
        <button class="ghost" onclick={() => (recoveryCodes = null)}>Done</button>
      {:else if setupUri}
        <p class="dim">Add this key to your authenticator app, then enter the code it shows.</p>
        <code class="secret">{secretFromUri}</code>
        <input bind:value={setupCode} inputmode="numeric" placeholder="000000" class="code-in" />
        <button class="cta" onclick={confirm2FA}>Turn on</button>
      {:else if user.totp_enabled}
        <p class="dim">Two-factor is on.</p>
        {#if showDisable}
          <input bind:value={disablePassword} type="password" placeholder="Your password" class="code-in" />
          <button class="danger" onclick={turnOff2FA}>Confirm turn off</button>
        {:else}
          <button class="ghost" onclick={() => (showDisable = true)}>Turn off</button>
        {/if}
      {:else}
        <p class="dim">Add a second step when signing in.</p>
        <button class="cta" onclick={begin2FA}>Set up</button>
      {/if}
    </section>

    <button class="signout" onclick={signOut}><LogOut size={16} /> Sign out</button>

    <footer>
      <a href="/legal/terms">Terms</a>
      <a href="/legal/privacy">Privacy</a>
      <a href="/legal/takedown">Report a work</a>
    </footer>
  {/if}

  {#if toast}<div class="toast">{toast}</div>{/if}
</div>

<style>
  .wrap {
    padding: 16px 16px 92px;
    padding-top: calc(16px + env(safe-area-inset-top));
    max-width: 560px;
    margin: 0 auto;
  }
  .head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: 20px;
  }
  h1 {
    margin: 0;
    font-size: 1.45rem;
    font-weight: 740;
    letter-spacing: -0.02em;
  }
  .handle {
    color: var(--text-faint);
    font-size: 0.84rem;
  }
  .tier {
    padding: 5px 13px;
    border-radius: var(--radius-pill);
    background: var(--grad-brand);
    color: #fff;
    font-size: 0.72rem;
    font-weight: 650;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .stats {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    margin-bottom: 20px;
  }
  .stat {
    padding: 15px 10px;
    border-radius: var(--radius-sm);
    background: var(--grad-card);
    border: 1px solid var(--border);
    text-align: center;
  }
  .v {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    font-size: 1.3rem;
    font-weight: 700;
    color: var(--orange);
  }
  .l {
    font-size: 0.7rem;
    color: var(--text-faint);
    margin-top: 3px;
  }

  .box {
    padding: 16px;
    border-radius: var(--radius-sm);
    background: var(--bg-card);
    border: 1px solid var(--border);
    margin-bottom: 12px;
  }
  h2 {
    display: flex;
    align-items: center;
    gap: 7px;
    margin: 0 0 11px;
    font-size: 0.88rem;
    font-weight: 620;
  }
  .dim {
    color: var(--text-dim);
    font-size: 0.84rem;
    margin: 0 0 11px;
    line-height: 1.5;
  }

  .bar {
    height: 6px;
    border-radius: 3px;
    background: var(--border);
    overflow: hidden;
    margin-bottom: 9px;
  }
  .fill {
    height: 100%;
    background: var(--grad-brand);
  }

  .seg {
    display: flex;
    gap: 7px;
  }
  .seg button {
    flex: 1;
    padding: 9px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-dim);
    font-size: 0.82rem;
  }
  .seg button.on {
    border-color: var(--orange);
    color: var(--orange);
  }

  .cta {
    display: inline-block;
    padding: 11px 20px;
    border: none;
    border-radius: var(--radius-pill);
    background: var(--grad-brand);
    color: #fff;
    font-weight: 620;
    font-size: 0.87rem;
  }
  .ghost {
    padding: 10px 18px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--border-bright);
    background: transparent;
    color: var(--text-dim);
    font-size: 0.85rem;
  }
  .danger {
    padding: 11px 20px;
    border: none;
    border-radius: var(--radius-pill);
    background: rgba(255, 77, 109, 0.15);
    border: 1px solid rgba(255, 77, 109, 0.4);
    color: #ff8fa3;
    font-size: 0.85rem;
  }

  .secret {
    display: block;
    padding: 12px;
    border-radius: var(--radius-sm);
    background: var(--bg);
    border: 1px solid var(--border);
    font-size: 0.86rem;
    letter-spacing: 0.08em;
    word-break: break-all;
    margin-bottom: 11px;
  }
  .code-in {
    width: 100%;
    padding: 12px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg);
    color: var(--text);
    font-family: inherit;
    font-size: 0.95rem;
    margin-bottom: 11px;
  }

  .warn {
    padding: 11px 13px;
    border-radius: var(--radius-sm);
    background: rgba(255, 176, 32, 0.1);
    border: 1px solid rgba(255, 176, 32, 0.3);
    color: var(--warn);
    font-size: 0.82rem;
    margin: 0 0 12px;
  }
  .codes {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 7px;
    margin-bottom: 13px;
  }
  .codes code {
    padding: 9px;
    border-radius: 6px;
    background: var(--bg);
    border: 1px solid var(--border);
    font-size: 0.82rem;
    text-align: center;
    letter-spacing: 0.04em;
  }

  .signout {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    width: 100%;
    padding: 13px;
    margin-top: 8px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-faint);
    font-size: 0.88rem;
  }

  footer {
    display: flex;
    gap: 18px;
    justify-content: center;
    padding-top: 26px;
    font-size: 0.76rem;
    color: var(--text-faint);
  }

  .empty {
    color: var(--text-faint);
    text-align: center;
    padding: 50px 0;
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
