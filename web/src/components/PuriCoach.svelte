<script lang="ts">
  import { onMount } from 'svelte';
  let { word = '', definition = '', selection = '', facts = false, expanded = false }: {
    word?: string; definition?: string; selection?: string; facts?: boolean; expanded?: boolean;
  } = $props();
  const trivia = [
    'A “bookworm” can be a real insect! Several kinds of beetle larvae nibble on old books.',
    'Many printed books are made from folded groups of pages called “signatures.”',
    'The word “novel” comes from the Italian “novella,” meaning a new story or piece of news.',
    'Gutenberg’s famous Bible was printed in the 1450s—more than 500 years ago!',
    'A book with pages bound along one edge is called a “codex.” It gradually replaced the scroll.',
  ];
  let fact = $state('');
  let explained = $state(false);
  const correct = $derived(Boolean(word && selection.trim().toLowerCase() === word.toLowerCase()));
  $effect(() => { word; definition; selection; explained = false; });
  onMount(() => {
    if (!facts) return;
    let timer: ReturnType<typeof setTimeout>, last = -1;
    function show() {
      const choices = trivia.map((_, i) => i).filter(i => i !== last);
      last = choices[Math.floor(Math.random() * choices.length)];
      fact = trivia[last];
      timer = setTimeout(() => { fact = ''; timer = setTimeout(show, 20000 + Math.random() * 15000); }, 10000);
    }
    timer = setTimeout(show, 3500 + Math.random() * 2500);
    return () => clearTimeout(timer);
  });
</script>

{#if !facts || fact}
  <div class="bubble" class:fact={facts} role="status">
    {#if facts}{fact}
    {:else if explained && correct}
      <strong>“{word}”</strong><p class="definition">{definition}</p>
      <small>A prepared explanation for this demo. No AI used.</small>
    {:else if !word}Turn to the next card. I’ll show you a word to highlight!
    {:else if selection && !correct}
      You highlighted “{selection.slice(0, 60)}”. For this demo, highlight <strong>“{word}”</strong> instead, then click me and choose Explain.
    {:else if expanded && correct}Nice! Click Explain to discover what <strong>“{word}”</strong> means.
    {:else}Highlight <strong>“{word}”</strong> in the text, then click me and choose Explain.
    {/if}
    {#if !facts && expanded && word && !explained}
      <button class="explain" disabled={!correct} onclick={() => explained = correct}>Explain</button>
    {/if}
  </div>
{/if}

<style>
  .bubble { position: relative; padding: 12px 16px; border-radius: 18px 18px 4px 18px; background: #b8eee8; border: 1px solid #75cfc5; color: #164e4a; font-size: 0.85rem; line-height: 1.5; text-align: left; }
  .bubble::after { content: ''; position: absolute; left: clamp(12px, var(--tail-x, 90%), calc(100% - 20px)); top: var(--tail-top, auto); bottom: var(--tail-bottom, -7px); width: 12px; height: 12px; background: #b8eee8; border-right: 1px solid #75cfc5; border-bottom: 1px solid #75cfc5; transform: translateX(-50%) rotate(var(--tail-rotation, 45deg)); }
  .definition { margin: 6px 0; }
  small { display: block; font-size: 0.7rem; color: #356660; }
  .explain { display: block; margin-top: 10px; padding: 7px 16px; border-radius: 999px; border: 1px solid #519d94; background: #fff; color: #164e4a; font-weight: 600; cursor: pointer; }
  .explain:disabled { opacity: 0.5; cursor: default; }
</style>
