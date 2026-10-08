<script lang="ts">
  import { onMount } from 'svelte';
  let { word = '', definition = '', selection = '', landing = false }: {
    word?: string; definition?: string; selection?: string; landing?: boolean;
  } = $props();
  const facts = [
    'A “bookworm” can be a real insect! Several kinds of beetle larvae nibble on old books.',
    'Many printed books are made from folded groups of pages called “signatures.”',
    'The word “novel” comes from the Italian “novella,” meaning a new story or piece of news.',
    'Gutenberg’s famous Bible was printed in the 1450s—more than 500 years ago!',
    'A book with pages bound along one edge is called a “codex.” It gradually replaced the scroll.',
  ];
  let fact = $state('');
  let expanded = $state(false);
  let explained = $state(false);
  const correct = $derived(Boolean(word && selection.trim().toLowerCase() === word.toLowerCase()));
  $effect(() => { word; definition; expanded = false; explained = false; });
  $effect(() => { selection; explained = false; });
  onMount(() => {
    if (!landing) return;
    let timer: ReturnType<typeof setTimeout>;
    let last = -1;
    const show = () => {
      const choices = facts.map((_, i) => i).filter(i => i !== last);
      last = choices[Math.floor(Math.random() * choices.length)];
      fact = facts[last];
      timer = setTimeout(() => {
        fact = '';
        timer = setTimeout(show, 20000 + Math.random() * 15000);
      }, 10000);
    };
    timer = setTimeout(show, 3500 + Math.random() * 2500);
    return () => clearTimeout(timer);
  });
</script>

<div class="guest-puri" class:landing>
  {#if landing}
    {#if fact}<div class="bubble fact" role="status">{fact}</div>{/if}
  {:else}
    <div class="bubble" role="status">
      {#if explained && correct}
        <strong>“{word}”</strong>
        <p class="definition">{definition}</p>
        <small>A prepared explanation for this demo. No AI used.</small>
      {:else if !word}
        Turn to the next card. I’ll show you a word to highlight!
      {:else if selection && !correct}
        You highlighted “{selection.slice(0, 60)}”. For this demo, highlight <strong>“{word}”</strong> instead, then click me and choose Explain.
      {:else if expanded && correct}
        Nice! Click Explain to discover what <strong>“{word}”</strong> means.
      {:else}
        Highlight <strong>“{word}”</strong> in the text, then click me and choose Explain.
      {/if}
      {#if expanded && word && !explained}
        <button class="explain" disabled={!correct} onclick={() => explained = correct}>Explain</button>
      {/if}
    </div>
  {/if}
  {#if landing}
    <span class="puri-mascot" aria-label="Puri, your reading companion" role="img">
      <img class="open" src="/glim.svg" alt="" width="32" height="32" />
      <img class="half" src="/glim_blink.svg" alt="" width="32" height="32" />
      <img class="closed" src="/glim_blink_two.svg" alt="" width="32" height="32" />
    </span>
  {:else}
    <button class="puri-mascot" aria-label="Click Puri" aria-expanded={expanded} onclick={() => expanded = !expanded}>
      <img class="open" src="/glim.svg" alt="" width="32" height="32" />
      <img class="half" src="/glim_blink.svg" alt="" width="32" height="32" />
      <img class="closed" src="/glim_blink_two.svg" alt="" width="32" height="32" />
    </button>
  {/if}
</div>

<style>
  .guest-puri { display: flex; align-items: flex-end; justify-content: flex-end; gap: 12px; width: 100%; text-align: left; }
  .guest-puri.landing { position: fixed; right: 20px; bottom: 20px; width: min(360px, calc(100vw - 40px)); z-index: 20; pointer-events: none; }
  .bubble { position: relative; max-width: 310px; flex: 1; padding: 12px 16px; border-radius: 18px 18px 4px 18px; background: #b8eee8; border: 1px solid #75cfc5; color: #164e4a; font-size: 0.85rem; line-height: 1.5; }
  .definition { margin: 6px 0; }
  small { display: block; font-size: 0.7rem; color: #356660; }
  .explain { display: block; margin-top: 10px; padding: 7px 16px; border-radius: 999px; border: 1px solid #519d94; background: #fff; color: #164e4a; font-weight: 600; cursor: pointer; }
  .explain:disabled { opacity: 0.5; cursor: default; }
  .puri-mascot { position: relative; display: block; flex: 0 0 32px; width: 32px; height: 32px; border: 0; padding: 0; background: transparent; }
  button.puri-mascot { cursor: pointer; }
  .puri-mascot img { position: absolute; inset: 0; width: 32px; height: 32px; }
  .open { animation: open 5s steps(1, end) infinite; }
  .half { opacity: 0; animation: half 5s steps(1, end) infinite; }
  .closed { opacity: 0; animation: closed 5s steps(1, end) infinite; }
  @keyframes open { 0%, 98%, 100% { opacity: 1; } 94% { opacity: 0; } }
  @keyframes half { 0%, 95%, 98%, 100% { opacity: 0; } 94%, 97% { opacity: 1; } }
  @keyframes closed { 0%, 97%, 100% { opacity: 0; } 95% { opacity: 1; } }
  @media (prefers-reduced-motion: reduce) { .open { animation: none; opacity: 1; } .half, .closed { animation: none; opacity: 0; } }
</style>
