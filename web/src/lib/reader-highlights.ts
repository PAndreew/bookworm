import type { Highlight, Page } from './api';

// Annotation coordinates remain on immutable source pages. The rendered card
// can join multiple source pages and be split into Unicode continuation screens.
export function highlightRuns(page:Page|undefined,marks:Highlight[],start:number,end:number) {
  if(!page)return [];
  const chars=Array.from(page.content),ranges:{start:number;end:number}[]=[];
  const parts=page.parts?.length?page.parts:[{page_id:page.id,start_off:page.start_off??0,end_off:(page.start_off??0)+chars.length,card_start:0}];
  for(const part of parts)for(const mark of marks) {
    if(mark.page_id!==part.page_id)continue;
    const a=Math.max(mark.start_off,part.start_off),b=Math.min(mark.end_off,part.end_off);
    const left=Math.max(start,part.card_start+a-part.start_off),right=Math.min(end,part.card_start+b-part.start_off);
    if(a<b&&left<right)ranges.push({start:left,end:right});
  }
  const boundaries=[...new Set([start,end,...ranges.flatMap(r=>[r.start,r.end])])].sort((a,b)=>a-b);
  return boundaries.slice(0,-1).map((left,i)=>({text:chars.slice(left,boundaries[i+1]).join(''),marked:ranges.some(r=>r.start<=left&&r.end>=boundaries[i+1])}));
}
