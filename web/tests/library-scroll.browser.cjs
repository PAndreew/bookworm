const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const browser=await chromium.launch({args:['--no-sandbox']});
 try {
  const page=await browser.newPage({serviceWorkers:'block'});
  const books=Array.from({length:30},(_,i)=>({id:String(i),slug:'book-'+i,title:'Book '+i,author:'Author',genres:['fiction'],license:'public_domain',card_count:100,last_read_at:'2026-11-14T00:00:00Z'}));
  await page.route('**/v1/**',async route=>{
   const url=new URL(route.request().url()),path=url.pathname;
   await route.fulfill({json:path==='/v1/me'?{user:{tier:'free'}}:path==='/v1/books'?{books:url.searchParams.has('mine')?books.slice(0,8):books,next_offset:null}:path==='/v1/genres'?{genres:['fiction']}:path==='/v1/me/ratings'?{ratings:{}}:{current:0}});
  });
  const bounds=()=>page.evaluate(()=>{
   const rect=selector=>{const r=document.querySelector(selector).getBoundingClientRect();return {top:r.top,left:r.left,bottom:r.bottom,width:r.width};};
   return {controls:rect('.discovery'),footer:rect('.tabs'),viewport:innerHeight,overflow:document.documentElement.scrollWidth-innerWidth};
  });
  for(const width of [390,1024]){
   await page.setViewportSize({width,height:850});
   await page.goto((process.env.TEST_BASE_URL||'http://127.0.0.1:4333')+'/app');
   await page.locator('.horizontal .card').first().waitFor();
   const before=await bounds();
   await page.evaluate(()=>{window.scrollTo(0,500);document.querySelector('.horizontal').scrollLeft=400;});
   await page.waitForTimeout(100);
   const after=await bounds();
   console.log(width,{before,after});
   assert.deepEqual(after.controls,before.controls,'search/filter navbar must stay in place');
   assert.deepEqual(after.footer,before.footer,'bottom navigation must stay in place');
   assert.equal(after.footer.bottom,after.viewport);
   assert.equal(after.overflow,0,'only book shelf should scroll horizontally');
   await page.getByRole('button',{name:'Sorting options',exact:true}).click();
   assert.ok(await page.getByRole('group',{name:'Book sorting'}).isVisible(),'sort controls remain usable after scrolling');
   await page.evaluate(()=>window.scrollBy(0,500));
   assert.deepEqual((await bounds()).footer,before.footer);
   await page.setViewportSize({width,height:700});
   const resized=await bounds();
   assert.equal(resized.footer.bottom,700,'footer follows viewport height changes');
   assert.deepEqual(resized.controls,before.controls,'controls stay pinned when viewport resizes');
   const footerClickable=await page.locator('.tabs a').first().evaluate(el=>{
    const r=el.getBoundingClientRect();return el.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));
   });
   assert.ok(footerClickable,'cards must not cover the fixed navigation');
  }
  console.log('Library controls and footer stay anchored during vertical and shelf scrolling');
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exit(1);});
