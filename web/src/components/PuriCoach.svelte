<script lang="ts">
  let { word = '', definition = '', selection = '', expanded = false, translation = false, translated = false, onTranslate, onClose }: {
    word?: string; definition?: string; selection?: string; expanded?: boolean;
    translation?: boolean; translated?: boolean; onTranslate?: () => Promise<void>; onClose?: () => void;
  } = $props();
  let explained = $state(false);
  const correct = $derived(Boolean(word && selection.trim().toLowerCase() === word.toLowerCase()));
  $effect(() => { word; definition; selection; explained = false; });
</script>

<div class="bubble" role="status">
  <button class="close" onclick={onClose} aria-label="Close reading companion">×</button>
  {#if translation}
    {#if translated}
      Here’s the card in French! Use <strong>Card language</strong> to switch back to Original.
      <small>A prepared translation for this demo. No live AI request. In Plus, saved translations don’t use another AI request.</small>
    {:else}
      Now let’s translate a whole card! Click me and choose <strong>Translate card</strong> to see it in French.
      {#if expanded}<button class="explain" onclick={() => onTranslate?.()}>Translate card</button>{/if}
    {/if}
  {:else if explained && correct}
    <strong>“{word}”</strong><p class="definition">{definition}</p>
    <small>A prepared explanation for this demo. No AI used.</small>
  {:else if !word}Turn to the next card. I’ll show you a word to highlight!
  {:else if selection && !correct}
    You highlighted “{selection.slice(0, 60)}”. For this demo, highlight <strong>“{word}”</strong> instead, then click me and choose Explain.
  {:else if expanded && correct}Nice! Click Explain to discover what <strong>“{word}”</strong> means.
  {:else}Highlight <strong>“{word}”</strong> in the text, then click me and choose Explain.
  {/if}
  {#if !translation && expanded && word && !explained}
    <button class="explain" disabled={!correct} onclick={() => explained = correct}>Explain</button>
  {/if}
</div>

<style>
  .bubble { position: relative; padding: 12px 36px 12px 16px; border-radius: 18px 18px 4px 18px; background: #b8eee8; border: 1px solid #75cfc5; color: #164e4a; font-size: 0.85rem; line-height: 1.5; text-align: left; }
  .bubble::after { content: ''; position: absolute; left: clamp(12px, var(--tail-x, 90%), calc(100% - 20px)); top: var(--tail-top, auto); bottom: var(--tail-bottom, -7px); width: 12px; height: 12px; background: #b8eee8; border-right: 1px solid #75cfc5; border-bottom: 1px solid #75cfc5; transform: translateX(-50%) rotate(var(--tail-rotation, 45deg)); }
  .close { position: absolute; top: 4px; right: 4px; width: 32px; height: 32px; border: 0; background: transparent; color: inherit; font-size: 1.25rem; cursor: pointer; }
  .definition { margin: 6px 0; }
  small { display: block; margin-top: 6px; font-size: 0.7rem; color: #356660; }
  .explain { display: block; margin-top: 10px; padding: 7px 16px; border-radius: 999px; border: 1px solid #519d94; background: #fff; color: #164e4a; font-weight: 600; cursor: pointer; }
  .explain:disabled { opacity: 0.5; cursor: default; }
</style>
