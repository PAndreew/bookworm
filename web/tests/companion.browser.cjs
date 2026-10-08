// No credentials/AI spend: mock every API against a served frontend build.
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.TEST_BASE_URL || 'http://localhost:4321';
(async () => {
 const browser = await chromium.launch({ ...(process.env.PLAYWRIGHT_EXECUTABLE ? {executablePath:process.env.PLAYWRIGHT_EXECUTABLE} : {}), args:['--no-sandbox'] });
 for (const [width,height] of [[320,568],[390,600],[1280,720]]) {
  const ctx=await browser.newContext({viewport:{width,height},serviceWorkers:'block'});
  const page=await ctx.newPage();const errors=[];const ai=[];const reads=[];const marks=[];
  let tier='free';page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/v1/**',async route=>{
   const request=route.request(),url=new URL(request.url());let body={};
   if(url.pathname==='/v1/me')body={user:{tier},last_read:null};
   else if(url.pathname.endsWith('/cards')){
    const from=Number(url.searchParams.get('card_from')||url.searchParams.get('source_from')||0);
    const prose=('Bonjour, mon ami. '+ 'This is a balanced sentence with familiar words. '.repeat(7)).slice(0,360);
    body={page_count:1536,pages:Array.from({length:12},(_,i)=>({card_id:`card-${from+i}`,id:`source-${Math.floor((from+i)/3)}`,idx:from+i,source_idx:Math.floor((from+i)/3),start_off:50,kind:from+i===0?'title':from+i===1?'chapter':'text',content:from+i===0?'Test Book\nTest Author':from+i===1?'CHAPTER 1.':prose,char_count:360,like_count:0,liked_by_me:false}))};
   }else if(url.pathname==='/v1/assistant'){ai.push(request.postDataJSON());body={answer:'Bonjour means hello.',model:'google/gemini-3.8-flash'};}
   else if(url.pathname==='/v1/card-reads'){reads.push(request.postDataJSON());body={streak_extended:false};}
   else if(url.pathname==='/v1/highlights'){marks.push(request.postDataJSON());body={highlight:{id:'mark'}};}
   else if(url.pathname.startsWith('/v1/books/'))body={book:{title:'Test Book',author:'Tester'}};
   await route.fulfill({json:body});
  });
  await page.route('**/read/moby-dick/**', async route => {
   const response = await page.request.get(base + '/read/');
   await route.fulfill({response});
  });
  await page.goto(base,{waitUntil:'networkidle'});
  await page.locator('.puri-mascot').waitFor({state:'visible'});
  assert.equal((await page.locator('.puri-mascot').boundingBox()).width,32);
  assert.equal(await page.locator('.guest-puri button').count(),0,'landing Puri must not open AI');
  assert.equal(await page.locator('.journey').count(),0,'old horizontal track remained');
  await page.goto(base+'/read/moby-dick/?card=0',{waitUntil:'networkidle'});
  await page.locator('.mascot').waitFor({state:'visible'});
  await page.locator('.mascot').click();await page.getByRole('link',{name:'See Plus and Pro',exact:true}).waitFor();
  assert(await page.getByRole('button',{name:'Explain phrase',exact:true}).isDisabled());
  await page.keyboard.press('Escape');tier='pro';
  await page.goto(base+'/read/moby-dick/?card=0',{waitUntil:'networkidle'});
  await page.waitForSelector('[data-kind="title"]');await page.waitForTimeout(1900);
  assert.equal(reads.length,0,'title card counted as read');
  await page.evaluate(()=>{let f=document.querySelector('.feed');f.scrollTop=f.clientHeight;});await page.waitForTimeout(2100);
  assert.equal(reads.length,0,'chapter card counted as read');
  await page.evaluate(()=>{let f=document.querySelector('.feed');f.scrollTop=f.clientHeight*2;});await page.waitForTimeout(900);
  const geometry=await page.evaluate(()=>{
   const article=document.querySelector('[data-idx="2"]'),body=article.querySelector('[data-body]'),marker=article.querySelector('[data-sunny-anchor]').getBoundingClientRect(),sunny=document.querySelector('.mascot').getBoundingClientRect(),actions=article.querySelector('.actions').getBoundingClientRect();
   return {overflow:body.scrollHeight-body.clientHeight,marker:{x:marker.x,y:marker.y},sunny:{x:sunny.x,y:sunny.y},actions:{x:actions.x,y:actions.y,width:actions.width,height:actions.height},bodyBottom:body.getBoundingClientRect().bottom};
  });
  assert(geometry.overflow<=1,`default text overflows at ${width}px: ${geometry.overflow}`);
  assert(Math.abs(geometry.sunny.y-geometry.marker.y)<2,'Sunny not sitting on final line');
  assert(Math.abs(geometry.sunny.x-geometry.marker.x-2)<2,'Sunny not following sentence end');
  assert(geometry.actions.width>geometry.actions.height*2,'controls not horizontal');
  assert(geometry.actions.y>geometry.bodyBottom,'controls cover text');
  const selected=await page.evaluate(()=>{const node=document.querySelector('[data-idx="2"] [data-body]').firstChild;const range=document.createRange();range.setStart(node,0);range.setEnd(node,16);const sel=window.getSelection();sel.removeAllRanges();sel.addRange(range);document.dispatchEvent(new Event('selectionchange'));return sel.toString().trim();});
  await page.locator('.mascot').click();assert.equal(await page.getByLabel('Phrase',{exact:true}).inputValue(),selected);
  assert(!(await page.getByRole('button',{name:'Explain phrase',exact:true}).isDisabled()));
  assert.equal(await page.getByRole('checkbox').count(),0,'paid reader must not have the old consent checkbox');
  await page.getByRole('button',{name:'Explain phrase',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.answer').textContent==='Bonjour means hello.');
  assert.deepEqual(ai[0],{mode:'explain',phrase:selected,language:'English',consent:true});
  await page.getByLabel('Help me',{exact:true}).selectOption('translate');await page.getByRole('button',{name:'Translate phrase',exact:true}).click();await page.waitForTimeout(200);assert.equal(ai[1].mode,'translate');await page.keyboard.press('Escape');
  // Re-select and verify absolute original-page offsets, not local card offsets.
  await page.evaluate(()=>{const node=document.querySelector('[data-idx="2"] [data-body]').firstChild;const r=document.createRange();r.setStart(node,0);r.setEnd(node,16);const s=window.getSelection();s.removeAllRanges();s.addRange(r);});
  await page.locator('[data-idx="2"]').getByRole('button',{name:'Highlight the selected text',exact:true}).click();await page.waitForTimeout(250);
  assert.equal(marks[0].start_off,50);assert.equal(marks[0].end_off,66);assert.equal(marks[0].page_id,'source-0');
  assert.deepEqual(errors,[]);console.log(`${width}×${height}: inline 32px Sunny, no overflow, bottom controls, unmetered headings, paid AI, original annotation offsets passed`);
  if(process.env.SCREENSHOT_DIR)await page.screenshot({path:`${process.env.SCREENSHOT_DIR}/sunny-${width}.png`});
  await ctx.close();
 }
 const ctx=await browser.newContext({reducedMotion:'reduce',viewport:{width:390,height:600},serviceWorkers:'block'});const p=await ctx.newPage();await p.goto(base,{waitUntil:'networkidle'});await p.evaluate(()=>window.dispatchEvent(new Event('scroll')));await p.waitForTimeout(100);assert.equal(await p.locator('.puri-mascot').evaluate(e=>e.getAnimations({subtree:true}).length),0);await browser.close();console.log('Reduced motion: no hopping');
})().catch(e=>{console.error(e);process.exit(1)});
