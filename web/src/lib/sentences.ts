/** Rune offsets, matching stored annotation offsets. Abbreviations are not ends. */
export function sentenceEnds(runes: string[]): number[] {
  const ends: number[] = [];
  const abbreviations = /^(?:mr|mrs|ms|dr|prof|rev|st|sr|jr|vs|etc|e\.g|i\.e|a\.m|p\.m|no|fig|vol|mt|messrs|mme|mlle)$/iu;
  for (let cut = 1; cut <= runes.length; cut++) {
    if (cut < runes.length && (!/\s/u.test(runes[cut]) || /\s/u.test(runes[cut - 1]))) continue;
    let end = cut;
    while (end > 0 && /[\s"'”’\])}]/u.test(runes[end - 1])) end--;
    const mark = runes[end - 1];
    if (/[!?。！？]/u.test(mark ?? '')) { ends.push(cut); continue; }
    if (mark !== '.') continue;
    let start = end - 1;
    while (start > 0 && /[\p{L}.]/u.test(runes[start - 1])) start--;
    const token = runes.slice(start, end - 1).join('');
    if (abbreviations.test(token) || /^\p{Lu}$/u.test(token) || token.includes('.')) continue;
    ends.push(cut);
  }
  return ends;
}
