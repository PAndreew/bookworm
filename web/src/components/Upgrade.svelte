<script lang="ts">
 import {onMount} from 'svelte';
 import {api,type User} from '../lib/api';
 import Membership from './Membership.svelte';
 let user=$state<User|null>(null),ready=$state(false);
 async function load(){try{user=(await api.me()).user}catch{}finally{ready=true}}
 onMount(load);
</script>
<main>
 <h1>Founding Plus</h1>
 <p class="price">€20 <span>per year</span></p>
 <p>For the first 50 subscribers. 100 AI explanations or translations every month.</p>
 <p>Ask Puri about a word or phrase, or translate a reading card into French, Spanish or German. Saved translations can be reused without another AI request.</p>
 <p>All books currently on the shelf are free. Plus pays for the AI companion, not access to books.</p>
 {#if user}<Membership {user} onchange={load} />{:else if ready}<a class="cta" href="/login">Sign in to subscribe</a>{/if}
 <p class="fine">Annual renewal at €20 until cancelled. Unsubscribe on You at any time; access continues until the paid year ends. The monthly AI allowance resets on the first of each month, UTC. Unused requests do not roll over.</p>
 <a href="/app">← Back to the library</a>
</main>
<style>
 main{max-width:560px;margin:auto;padding:32px 16px 100px;}
 h1{font-size:1.6rem;margin:0 0 20px;}
 p{line-height:1.7;color:var(--text-dim);font-size:.9rem;}
 .price{font-size:2rem;color:var(--text);font-weight:700;}.price span{font-size:.9rem;font-weight:400;}
 .cta{display:inline-block;padding:9px 18px;background:#fff;border:1px solid var(--border);border-radius:999px;}
 .fine{font-size:.75rem;margin:24px 0;}
</style>
