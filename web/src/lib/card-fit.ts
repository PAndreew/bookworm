export interface Fragment { start: number; end: number }

/** Measure real font/viewport geometry, retaining rune offsets for annotations.
 * Exceptions are continuation screens, never smaller type or hidden text. */
export function fitTextCard(article: HTMLElement, content: string): Fragment[] {
  const body = article.querySelector('[data-body]') as HTMLElement;
  if (!body) return [];
  const style = getComputedStyle(article);
  const available = article.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom);
  const probe = body.cloneNode(false) as HTMLElement;
  probe.removeAttribute('data-body');
  probe.setAttribute('aria-hidden', 'true');
  Object.assign(probe.style, {
    position: 'fixed', visibility: 'hidden', pointerEvents: 'none',
    width: `${body.getBoundingClientRect().width}px`, height: 'auto',
    maxHeight: 'none', overflow: 'visible', margin: '0',
  });
  article.appendChild(probe);
  const runes = Array.from(content);
  const spans: Fragment[] = [];
  const fits = (start: number, end: number) => {
    probe.textContent = runes.slice(start, end).join('');
    const marker = document.createElement('span');
    Object.assign(marker.style, { display: 'inline-block', width: '36px', height: '32px', verticalAlign: 'middle' });
    probe.appendChild(marker);
    return probe.getBoundingClientRect().height <= available + 0.5;
  };
  let start = 0;
  try {
    while (start < runes.length) {
      let end = runes.length;
      if (!fits(start, end)) {
        let lo = start + 1, hi = end;
        while (lo < hi) {
          const mid = Math.ceil((lo + hi) / 2);
          if (fits(start, mid)) lo = mid;
          else hi = mid - 1;
        }
        end = lo;
        while (end > start + 1 && !/\s/u.test(runes[end] ?? '')) end--;
        // Only a token longer than the whole screen requires rune continuation.
        if (end <= start + 1) end = lo;
      }
      spans.push({ start, end });
      start = end;
    }
    return spans.length ? spans : [{ start: 0, end: 0 }];
  } finally { probe.remove(); }
}
