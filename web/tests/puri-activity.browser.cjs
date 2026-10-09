const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.TEST_BASE_URL || 'http://localhost:4335';
(async () => {
 const browser = await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try {
  for (const [width,height] of [[320,568],[390,800],[1280,720]]) {
   const page=await browser.newPage({viewport:{width,height},serviceWorkers:'block'});
   const calls=[],errors=[];
   page.on('pageerror',e=>errors.push(e.message));
   const activity=Array.from({length:30},(_,i)=>({date:`2026-06-${String(i+1).padStart(2,'0')}`,cards:i===29?4:0}));
   await page.route('**/v1/**',async route=>{
    const path=new URL(route.request().url()).pathname; let body={};
    if(path==='/v1/me') body={user:{id:'reader',display_name:'Reader',handle:'reader',tier:'pro',visibility:'friends'},reading_activity:activity};
    else if(path.endsWith('/cards')) body={page_count:1,pages:[{id:'source',card_id:'card',idx:0,kind:'text',content:'Bonjour, mon ami. A short passage for reading.',like_count:0,liked_by_me:false}]};
    else if(path==='/v1/assistant') { calls.push(route.request().postDataJSON());body={answer:'**Bonjour**\n\n*Interjection*\n\n- Means hello.\n\n<script>alert(1)</script>'}; }
    else if(path==='/v1/billing') body={tier:'pro',used:0,limit:1000,subscription:null};
    else if(path==='/v1/billing/offer') body={available:true,remaining:50};
    else if(path.startsWith('/v1/books/')) body={book:{title:'Test',author:'Tester'}};
    await route.fulfill({json:body});
   });
   await page.route('**/read/test/**',async route=>route.fulfill({response:await page.request.get(base+'/read/')}));
   await page.goto(base+'/read/test/?card=0');
   await page.locator('.mascot').waitFor({state:'visible'});
   await page.locator('.mascot').click();
   await page.waitForFunction(()=>document.querySelector('dialog[open] .submit')?.disabled);
   assert.equal(await page.locator('dialog[open] textarea').count(),0);
   await page.keyboard.press('Escape');
   await page.evaluate(()=>{const n=document.querySelector('[data-body]').firstChild;const r=document.createRange();r.setStart(n,0);r.setEnd(n,7);const s=window.getSelection();s.removeAllRanges();s.addRange(r);document.dispatchEvent(new Event('selectionchange'));});
   await page.locator('.mascot').click();
   await page.waitForFunction(()=>!document.querySelector('dialog[open] .submit')?.disabled);
   const dialog=page.locator('dialog[open]');
   assert.equal(await dialog.locator('.selected-phrase').textContent(),'Bonjour');
   assert.equal(await dialog.locator('.submit').evaluate(el=>getComputedStyle(el).backgroundColor),'rgb(255, 255, 255)');
   assert((await dialog.locator('.submit').boundingBox()).width<(await dialog.boundingBox()).width-32);
   assert.equal(await dialog.locator('.close').evaluate(el=>getComputedStyle(el).backgroundColor),'rgba(0, 0, 0, 0)');
   await dialog.locator('.submit').click();
   await dialog.locator('.answer strong').waitFor();
   assert.equal(await dialog.locator('.answer strong').textContent(),'Bonjour');
   assert.equal(await dialog.locator('.answer em').textContent(),'Interjection');
   assert.equal(await dialog.locator('.answer script').count(),0);
   assert.equal(calls[0].card_id,'card');
   assert.equal(calls[0].phrase,'Bonjour');
   const box=await dialog.boundingBox();assert(box.x>=0&&box.x+box.width<=width+1&&box.y>=0&&box.y+box.height<=height+1,JSON.stringify(box));
   await page.goto(base+'/app/me/');
   await page.locator('.reading-day').last().waitFor();
   assert.equal(await page.locator('.reading-day').count(),30);
   assert.equal(await page.locator('.reading-day').last().getAttribute('aria-label'),'2026-06-30: 4 new cards');
   assert.equal(await page.locator('.reading-bar').last().evaluate(el=>el.getBoundingClientRect().height),129);
   assert.deepEqual(errors,[]);
   await page.close();
  }
  console.log('Puri selection, demo styling, safe Markdown, bubble bounds and 30-day chart passed');
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exit(1)});
