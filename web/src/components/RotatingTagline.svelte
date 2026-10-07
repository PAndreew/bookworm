<script lang="ts">
  import { onMount } from 'svelte';
  const words = ['awesome', 'classic', 'serious'];
  let active = $state(0);
  let fading = $state(false);

  onMount(() => {
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    let timer: ReturnType<typeof setInterval> | undefined;
    let swap: ReturnType<typeof setTimeout> | undefined;
    function start() {
      clearInterval(timer);
      clearTimeout(swap);
      fading = false;
      if (motion.matches) return;
      timer = setInterval(() => {
        fading = true;
        swap = setTimeout(() => {
          active = (active + 1) % words.length;
          fading = false;
        }, 350);
      }, 3200);
    }
    start();
    motion.addEventListener('change', start);
    return () => {
      clearInterval(timer);
      clearTimeout(swap);
      motion.removeEventListener('change', start);
    };
  });
</script>

<p class="tag">
  Reading tailored to our times.<br />
  <span aria-hidden="true">Read <span class="word" class:fading>{words[active]}</span> books as a social feed.</span>
  <span class="sr-only">Read awesome, classic, and serious books as a social feed.</span>
</p>

<style>
  .tag { margin: 0 0 32px; font-size: 1.15rem; color: var(--text-dim); line-height: 1.7; text-align: center; }
  .word { display: inline-block; min-width: 5.1em; color: var(--accent); font-weight: 600; opacity: 1; transform: translateY(0); transition: opacity 350ms ease, transform 350ms ease; }
  .word.fading { opacity: 0; transform: translateY(3px); }
  @media (prefers-reduced-motion: reduce) { .word { transition: none; } }
</style>
