<script lang="ts">
  import { onMount } from 'svelte';
  let purpose = $state('verify');
  let token = $state('');
  let email = $state('');
  let password = $state('');
  let busy = $state(false);
  let message = $state('');
  let done = $state(false);
  onMount(() => {
    const params = new URLSearchParams(location.hash.slice(1));
    purpose = params.get('purpose') === 'reset' ? 'reset' : 'verify';
    token = params.get('token') || '';
    history.replaceState(null, '', location.pathname);
  });
  async function submit(event: Event) {
    event.preventDefault(); busy = true; message = '';
    const action = token ? (purpose === 'reset' ? 'reset-password' : 'verify-email') : (purpose === 'reset' ? 'forgot-password' : 'resend-verification');
    try {
      const response = await fetch('/v1/auth/' + action, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(token ? { token, password } : { email }),
      });
      const data = await response.json();
      message = data.message || data.error?.message || 'Unable to complete this request. Please try again.';
      if (response.ok && token) { done = true; token = ''; }
    } catch { message = 'Unable to connect. Please try again.'; }
    finally { busy = false; }
  }
</script>
<main>
  <h1>{purpose === 'reset' ? 'Reset your password' : 'Confirm your email'}</h1>
  {#if !done}
    {#if !token}<p>{purpose === 'verify' ? 'Check your inbox for your confirmation link. Need another email? Enter your address below.' : 'Enter your email to receive a password-reset link.'}</p>{/if}
    <form onsubmit={submit}>
      {#if !token}<label>Email <input type="email" bind:value={email} autocomplete="email" required /></label>
      {:else if purpose === 'reset'}<label>New password <input type="password" bind:value={password} minlength="10" autocomplete="new-password" required /></label>{/if}
      <button disabled={busy}>{busy ? 'Please wait…' : token ? (purpose === 'reset' ? 'Change password' : 'Confirm email') : 'Send email'}</button>
    </form>
  {/if}
  <p role="status">{message}</p>
  <a href="/login">Sign in</a> · <a href="/app">Continue reading</a>
</main>
<style>
 main { max-width: 420px; margin: 12vh auto; padding: 24px; }
 form, label { display: flex; flex-direction: column; gap: 12px; }
 input, button { padding: 14px; border-radius: 8px; border: 1px solid var(--border); }
 button { cursor: pointer; }
</style>
