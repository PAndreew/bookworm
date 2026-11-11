<script lang="ts">
  import { onMount } from 'svelte';
  import { clearBYOK, fetchBYOKModels, loadBYOK, providers, saveBYOK, type BYOKModel, type BYOKProvider } from '../lib/byok';
  let { userId, onSaved }: {userId:string; onSaved?: () => void} = $props();
  let provider=$state<BYOKProvider>('openai');
  let key=$state(''),model=$state(''),search=$state(''),message=$state('');
  let accepted=$state(false),agree=$state(false),saved=$state(false),busy=$state(false);
  let models=$state<BYOKModel[]>([]);
  const filtered=$derived(models.filter(m=>(m.name+' '+m.id).toLowerCase().includes(search.toLowerCase())));
  let controller:AbortController|null=null;
  onMount(()=>{
    const settings=loadBYOK(userId);accepted=settings.accepted;
    if(settings.config){provider=settings.config.provider;key=settings.config.key;model=settings.config.model;saved=true;void refresh();}
    return()=>controller?.abort();
  });
  function changedProvider(){controller?.abort();key='';model='';models=[];search='';message='';saved=false;busy=false;}
  function changedKey(){controller?.abort();models=[];model='';saved=false;busy=false;message='';}
  async function refresh(){
    controller?.abort();const request=new AbortController();controller=request;busy=true;message='';
    try {const list=await fetchBYOKModels(provider,key,request.signal);if(request.signal.aborted)return;models=list;if(!list.some(m=>m.id===model))model='';}
    catch(e){if(!request.signal.aborted)message=e instanceof Error?e.message:'Could not load models.';}
    finally{if(controller===request)busy=false;}
  }
  function save(){
    message='';
    try{saveBYOK(userId,{provider,key,model},accepted||agree);accepted=true;agree=false;saved=true;message='Saved on this device.';onSaved?.();}
    catch(e){message=e instanceof Error?e.message:'Could not save.';}
  }
  function forget(){
    controller?.abort();
    try{clearBYOK(userId);key='';model='';models=[];saved=false;message='Saved key removed.';busy=false;onSaved?.();}
    catch(e){message=e instanceof Error?e.message:'Could not remove key.';}
  }
</script>

<section class="settings" id="byok" aria-labelledby="byok-title">
  <h2 id="byok-title">Your AI · bring your own key</h2>
  <p>{userId === 'anonymous-device' ? 'No account needed. Answers are not saved; your provider’s limits apply.' : '50 requests/month on Free. Answers are saved privately to your account.'} Your provider bills the API usage.</p>
  <label for="byok-provider">Provider</label>
  <select id="byok-provider" bind:value={provider} onchange={changedProvider}>
    {#each Object.entries(providers) as [id,info]}<option value={id}>{info.name}</option>{/each}
  </select>
  <label for="byok-key">API key</label>
  <input id="byok-key" type="password" bind:value={key} oninput={changedKey} autocomplete="off" autocapitalize="off" spellcheck="false" maxlength="512" />
  <div class="row"><button type="button" disabled={busy||!key.trim()} onclick={refresh}>{busy?'Loading models…':'Refresh models'}</button>{#if saved}<span>Key saved</span>{/if}</div>
  {#if models.length}
    <label for="byok-search">Find a model</label>
    <input id="byok-search" type="search" bind:value={search} placeholder="Search current models" autocomplete="off" />
    <label for="byok-model">Model · newest first</label>
    <select id="byok-model" bind:value={model} onchange={()=>saved=false}>
      <option value="" disabled>Choose a model</option>
      {#each filtered as item (item.id)}<option value={item.id}>{item.name===item.id?item.id:`${item.name} · ${item.id}`}</option>{/each}
    </select>
    <small>{models.length} text models from {providers[provider].name}. Availability and pricing depend on your API account.</small>
  {/if}
  {#if !accepted}
    <label class="terms"><input type="checkbox" bind:checked={agree} />I accept the <a href="/legal/privacy#byok">BYOK terms</a>: my API charges, text sent directly to my provider, key saved in this browser.</label>
    <small>Browser scripts/extensions can access saved keys. Use a dedicated key with a spending cap. OpenAI needs API billing, not a ChatGPT/Codex subscription.</small>
  {:else}<small>Saved only in this browser. <a href="/legal/privacy#byok">BYOK terms</a></small>{/if}
  <div class="row">
    <button type="button" onclick={save} disabled={busy||!model||!models.some(m=>m.id===model)||(!accepted&&!agree)}>Save on this device</button>
    {#if key||saved}<button type="button" onclick={forget}>Forget key</button>{/if}
  </div>
  {#if message}<p role="status">{message}</p>{/if}
</section>

<style>
  .settings{display:flex;flex-direction:column;gap:10px;padding:16px 0;margin-bottom:12px;scroll-margin-top:24px;}
  h2,p{margin:0;}h2{font-size:.9rem;font-weight:620;}p,small,.terms{font-size:.8rem;line-height:1.6;color:var(--text-dim);}
  label{font-size:.82rem;font-weight:500;}select,input:not([type=checkbox]){width:100%;min-width:0;padding:10px;border:1px solid var(--border);border-radius:8px;background:var(--bg);color:var(--text);font:inherit;}
  .row{display:flex;align-items:center;gap:10px;flex-wrap:wrap;}button{padding:8px 14px;border:1px solid var(--border);border-radius:var(--radius-pill);background:var(--bg);color:var(--text);cursor:pointer;font-size:.82rem;}button:disabled{opacity:.5;cursor:default;}
  .row span{font-size:.75rem;color:var(--text-dim);}.terms{display:flex;align-items:flex-start;gap:7px;flex-wrap:wrap;font-weight:400;}.terms input{margin-top:5px;}a{color:var(--accent);text-decoration:underline;}
</style>
