const assert = require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'/home/pi/pi-web-local/node_modules/playwright');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE||'/usr/bin/chromium-browser',args:['--no-sandbox']});
 const ctx=await browser.newContext({viewport:{width:320,height:568},serviceWorkers:'block',permissions:['clipboard-read','clipboard-write']});
 const page=await ctx.newPage();
 await page.route('**/v1/**',r=>{
  const path=new URL(r.request().url()).pathname;
  const data=path==='/v1/me'?{user:{id:'uid',handle:'reader',display_name:'Reader',tier:'free',visibility:'private'},reading_activity:[]}
   :path==='/v1/me/achievements'?{achievements:[{id:'first-card',unlocked_at:'2026-01-01T00:00:00Z'},{id:'hundred-cards',unlocked_at:null},{id:'full-book',unlocked_at:null}]}
   :path==='/v1/achievements/reader/first-card'?{achievement:{id:'first-card',unlocked_at:'2026-01-01T00:00:00Z'},reader:{handle:'reader',display_name:'Reader'}}
   :{status:'ok'};
  return r.fulfill({status:path==='/v1/achievements/reader/hundred-cards'?404:200,json:data});
 });
 await page.goto((process.env.TEST_BASE_URL||'http://localhost:4333')+'/app/me/');
 await page.getByRole('heading',{name:'Achievements'}).waitFor();
 assert.equal(await page.locator('.award').count(),3);
 assert.equal(await page.locator('.award img').nth(2).getAttribute('src'),'/achievements/full-book-locked.webp');
 assert.equal(await page.getByRole('button',{name:'Share A whole book'}).count(),0);
 assert.equal(await page.locator('.award img').first().getAttribute('src'),'/achievements/first-card.webp');
 assert.equal(await page.locator('.award').first().evaluate(el=>getComputedStyle(el).backgroundColor),'rgba(0, 0, 0, 0)');
 await page.route('**/v1/me/achievements',r=>r.fulfill({json:{achievements:[{id:'first-card',unlocked_at:null},{id:'full-book',unlocked_at:'2026-01-01T00:00:00Z'}]}}));
 await page.reload();
 await page.getByRole('heading',{name:'Achievements'}).waitFor();
 assert.equal(await page.locator('.award img').first().getAttribute('src'),'/achievements/first-card-locked.webp');
 assert.equal(await page.locator('.award img').nth(1).getAttribute('src'),'/achievements/full-book.webp');
 await page.unroute('**/v1/me/achievements');
 await page.reload();
 await page.getByRole('button',{name:'Share First card'}).waitFor();
 assert.equal(await page.getByRole('button',{name:'Share First card'}).count(),1);
 assert.equal(await page.getByRole('button',{name:'Share 100 cards'}).count(),0);
 await page.getByRole('button',{name:'Share First card'}).click();
 assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),(process.env.TEST_BASE_URL||'http://localhost:4333')+'/a/reader/first-card');
 await page.route('**/a/reader/**', async route => {
  const response = await route.fetch({url:(process.env.TEST_BASE_URL||'http://localhost:4333')+'/a/'});
  await route.fulfill({response});
 });
 await page.goto((process.env.TEST_BASE_URL||'http://localhost:4333')+'/a/reader/first-card/');
 await page.getByRole('heading',{name:'First card'}).waitFor();
 assert((await page.locator('#award').innerText()).includes('@reader'));
 await page.goto((process.env.TEST_BASE_URL||'http://localhost:4333')+'/a/reader/hundred-cards/');
 await page.getByText('This achievement is not available.').waitFor();
 await browser.close();
})().catch(e=>{console.error(e);process.exit(1)});
