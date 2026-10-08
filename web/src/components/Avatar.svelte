<script lang="ts">
  let { seed, size = 40 }: { seed: string; size?: number } = $props();
  // Stable local identicon: five columns mirrored about the centre, no tracking
  // service, remote images, random state, or personal information in the SVG.
  const hash = $derived.by(() => {
    let value = 2166136261;
    for (const char of seed) value = Math.imul(value ^ char.charCodeAt(0), 16777619);
    return value >>> 0;
  });
  const cells = $derived.by(() => {
    let value = hash || 1;
    const result: { x: number; y: number }[] = [];
    for (let y = 0; y < 5; y++) for (let x = 0; x < 3; x++) {
      value ^= value << 13; value ^= value >>> 17; value ^= value << 5;
      if (value & 1) { result.push({ x, y }); if (x !== 2) result.push({ x: 4 - x, y }); }
    }
    return result;
  });
</script>

<svg class="avatar" width={size} height={size} viewBox="0 0 7 7" aria-hidden="true" style="color:hsl({hash % 360} 48% 43%)" shape-rendering="crispEdges">
  <rect width="7" height="7" fill="#f1f0ee" />
  {#each cells as cell}<rect x={cell.x + 1} y={cell.y + 1} width="1" height="1" fill="currentColor" />{/each}
</svg>
<style>.avatar { display: block; flex-shrink: 0; border-radius: 8px; overflow: hidden; }</style>
