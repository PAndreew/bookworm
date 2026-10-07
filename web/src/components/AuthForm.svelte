<script lang="ts">
  import { api, ApiError } from '../lib/api';

  let { mode }: { mode: 'login' | 'signup' } = $props();

  let email = $state('');
  let password = $state('');
  let handle = $state('');
  let displayName = $state('');
  let birthdate = $state('');
  let busy = $state(false);
  let error = $state<string | null>(null);

  // Set when the account has 2FA: the password step succeeded but the session is
  // still pending until a code is entered.
  let needs2FA = $state(false);
  let code = $state('');
  let useRecovery = $state(false);

  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

  async function submit(e: Event) {
    e.preventDefault();
    busy = true;
    error = null;

    try {
      if (mode === 'signup') {
        await api.signup({
          email,
          handle,
          display_name: displayName || handle,
          password,
          birthdate,
          timezone: tz,
        });
        location.href = '/app';
        return;
      }

      const res = await api.login(email, password);
      if (res.mfa_required) {
        needs2FA = true;
        return;
      }
      location.href = '/app';
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Something went wrong.';
    } finally {
      busy = false;
    }
  }

  async function submitCode(e: Event) {
    e.preventDefault();
    busy = true;
    error = null;
    try {
      useRecovery ? await api.verifyRecovery(code) : await api.verify2FA(code);
      location.href = '/app';
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Something went wrong.';
    } finally {
      busy = false;
    }
  }

  // The latest date that satisfies the 13+ rule, used to bound the date picker.
  const maxBirthdate = (() => {
    const d = new Date();
    d.setFullYear(d.getFullYear() - 13);
    return d.toISOString().slice(0, 10);
  })();
</script>

<div class="shell">
  <div class="card">
    <h1><span class="brand-text">Bookworm</span></h1>

    {#if needs2FA}
      <p class="sub">Enter the code from your authenticator app.</p>

      <form onsubmit={submitCode}>
        <input
          bind:value={code}
          inputmode={useRecovery ? 'text' : 'numeric'}
          autocomplete="one-time-code"
          placeholder={useRecovery ? 'Recovery code' : '000000'}
          class="code"
          required
        />

        {#if error}<p class="err">{error}</p>{/if}

        <button class="go" disabled={busy}>{busy ? 'Checking…' : 'Continue'}</button>
      </form>

      <button class="link" onclick={() => { useRecovery = !useRecovery; code = ''; error = null; }}>
        {useRecovery ? 'Use your authenticator app instead' : 'Use a recovery code'}
      </button>
    {:else}
      <p class="sub">
        {mode === 'signup' ? 'Read books like a feed.' : 'Welcome back.'}
      </p>

      <form onsubmit={submit}>
        <input bind:value={email} type="email" placeholder="Email" autocomplete="email" required />

        {#if mode === 'signup'}
          <input
            bind:value={handle}
            placeholder="Handle"
            pattern="[a-z0-9_]{'{'}3,24{'}'}"
            title="3-24 characters: lowercase letters, numbers and underscores"
            autocomplete="username"
            required
          />
          <input bind:value={displayName} placeholder="Display name (optional)" autocomplete="name" />

          <label class="field">
            <span>Date of birth</span>
            <input bind:value={birthdate} type="date" max={maxBirthdate} required />
          </label>
          <p class="hint">Bookworm is for ages 13 and over.</p>
        {/if}

        <input
          bind:value={password}
          type="password"
          placeholder="Password"
          minlength={mode === 'signup' ? 10 : undefined}
          autocomplete={mode === 'signup' ? 'new-password' : 'current-password'}
          required
        />
        {#if mode === 'signup'}
          <p class="hint">At least 10 characters. Length beats symbols.</p>
        {/if}

        {#if error}<p class="err">{error}</p>{/if}

        <button class="go" disabled={busy}>
          {busy ? 'One moment…' : mode === 'signup' ? 'Create account' : 'Sign in'}
        </button>
      </form>

      <p class="alt">
        {#if mode === 'signup'}
          Already have an account? <a href="/login">Sign in</a>
        {:else}
          New here? <a href="/signup">Create an account</a>
        {/if}
      </p>
    {/if}
  </div>
</div>

<style>
  .shell {
    min-height: 100dvh;
    display: grid;
    place-items: center;
    padding: 24px;
  }
  .card {
    width: 100%;
    max-width: 360px;
  }
  h1 {
    margin: 0 0 6px;
    font-size: 2rem;
    font-weight: 750;
    letter-spacing: -0.03em;
    text-align: center;
  }
  .sub {
    text-align: center;
    color: var(--text-dim);
    margin: 0 0 26px;
    font-size: 0.92rem;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 11px;
  }

  input {
    width: 100%;
    padding: 14px 15px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg-card);
    color: var(--text);
    font-size: 0.95rem;
    font-family: inherit;
  }
  input:focus {
    outline: none;
    border-color: var(--orange);
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .field span {
    font-size: 0.76rem;
    color: var(--text-faint);
    padding-left: 3px;
  }

  .code {
    text-align: center;
    font-size: 1.4rem;
    letter-spacing: 0.3em;
    font-variant-numeric: tabular-nums;
  }

  .hint {
    margin: -4px 0 0 3px;
    font-size: 0.74rem;
    color: var(--text-faint);
  }

  .err {
    margin: 2px 0 0;
    padding: 10px 13px;
    border-radius: var(--radius-sm);
    background: rgba(255, 77, 109, 0.1);
    border: 1px solid rgba(255, 77, 109, 0.3);
    color: #ff8fa3;
    font-size: 0.84rem;
  }

  .go {
    margin-top: 6px;
    padding: 14px;
    border: none;
    border-radius: var(--radius-pill);
    background: var(--grad-brand);
    color: #fff;
    font-weight: 650;
    font-size: 0.95rem;
  }
  .go:disabled {
    opacity: 0.6;
  }

  .link {
    display: block;
    width: 100%;
    margin-top: 14px;
    padding: 8px;
    border: none;
    background: transparent;
    color: var(--text-faint);
    font-size: 0.82rem;
    text-decoration: underline;
  }

  .alt {
    text-align: center;
    margin-top: 20px;
    font-size: 0.86rem;
    color: var(--text-dim);
  }
  .alt a {
    color: var(--orange);
    font-weight: 550;
  }
</style>
