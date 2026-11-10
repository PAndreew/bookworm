<script lang="ts">
 import {api,ApiError,type User,type BillingStatus,type Offer} from '../lib/api';
 let {user,onchange}: {user:Pick<User,'tier'>;onchange?:()=>void}=$props();
 let status=$state<BillingStatus|null>(null),offer=$state<Offer|null>(null),busy=$state(false),message=$state('');
 async function refresh(){
  const results=await Promise.allSettled([api.billing(),api.offer()]);
  if(results[0].status==='fulfilled')status=results[0].value;
  if(results[1].status==='fulfilled')offer=results[1].value;
 }
 $effect(()=>{user.tier;void refresh()});
 async function subscribe(){busy=true;message='';try{const result=await api.checkout();window.location.assign(result.url)}catch(e){message=e instanceof ApiError?e.message:'Could not open checkout.';busy=false}}
 async function unsubscribe(){
  if(!window.confirm(status?.subscription?'Cancel renewal? Your Plus access continues until the end of the paid year.':'End your complimentary paid access?'))return;
  busy=true;message='';try{message=(await api.cancelSubscription()).message;await refresh();onchange?.()}catch(e){message=e instanceof ApiError?e.message:'Could not cancel renewal.'}finally{busy=false}
 }
</script>

<nav class="plans" aria-label="Membership">
 {#if user.tier==='free'}
  <p class="offer"><strong>Free BYOK · for the brave</strong><br/>50 app-enforced AI requests per calendar month with your OpenAI, Anthropic or OpenRouter API key. You pay the provider; keys stay in this page’s memory. Open a book and ask Puri to set it up.</p>
  <p class="offer"><strong>Founding Plus · €20/year</strong><br/>100 AI explanations or translations per month. First 50 subscribers only.</p>
  <button class="cta" disabled={busy||!offer?.available} onclick={subscribe}>{busy?'Opening Stripe…':'Subscribe'}</button>
  {#if offer}<p class="dim">{offer.available?`${offer.remaining} founding places available.`:'The founding offer is currently unavailable.'}</p>{/if}
 {:else}
  <p class="offer"><strong>{user.tier==='pro'?'Pro':'Plus'} membership</strong>{#if status}<br/>{status.used} / {status.limit} AI requests this month{/if}</p>
  <button class="cta on" disabled={busy||status?.subscription?.cancel_at_period_end} onclick={unsubscribe}>{status?.subscription?.cancel_at_period_end?'Renewal cancelled':busy?'Cancelling…':'Unsubscribe'}</button>
  {#if status?.subscription}<p class="dim">{status.subscription.cancel_at_period_end?'Access until':'Renews on'} {new Date(status.subscription.period_end).toLocaleDateString()}.</p>{/if}
 {/if}
 {#if message}<p class="message" role="status">{message}</p>{/if}
</nav>
<style>
 .plans{display:flex;align-items:flex-start;flex-direction:column;gap:12px;padding:16px 0;margin:0 0 12px;}
 .offer,.dim,.message{margin:0;line-height:1.6;font-size:.84rem;}
 .dim{color:var(--text-dim);font-size:.76rem;}
 .cta{padding:7px 16px;min-height:36px;border:1px solid var(--border);border-radius:var(--radius-pill);background:#fff;color:var(--text);font-size:.82rem;cursor:pointer;}
 .cta.on{background:#fef3c7;border-color:#fde68a;color:#713f12;}
 .cta:disabled{opacity:.6;cursor:default;}
</style>
