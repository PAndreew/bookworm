const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
 const browser = await chromium.launch({args:['--no-sandbox']});
 try {
  const page = await browser.newPage({serviceWorkers:'block'});
  const title = 'A very long book title\nwith a lengthy subtitle '.repeat(15);
  await page.route('**/v1/**',async route => {
   const path = new URL(route.request().url()).pathname;
   if(path === '/v1/me') return route.fulfill({status:401,json:{error:'unauthorized'}});
   await route.fulfill({json:path === '/v1/books' ? {books:[
    {id:'long',slug:'long',title,author:'Author',genres:[],license:'public_domain',card_count:200},
    {id:'short',slug:'short',title:'Short Title',author:'Author',genres:[],license:'public_domain',card_count:200}
   ],next_offset:null} : path === '/v1/genres' ? {genres:[]} : {}});
  });
  for(const width of [390,1024]) {
   await page.setViewportSize({width,height:900});
   await page.goto((process.env.TEST_BASE_URL || 'http://127.0.0.1:4333')+'/app');
   await page.waitForSelector('.card .t');
   const heading=page.locator('.card .t').first();
   assert.equal(await heading.textContent(),title.replace(/\s+/g,' ').trim().slice(0,30).trimEnd()+'…');
   assert.equal(await heading.getAttribute('title'),title);
   assert.equal(await heading.getAttribute('aria-label'),title);
   assert.equal(await page.locator('.card .t').nth(1).textContent(),'Short Title');
   const geometry=await page.locator('.card').evaluateAll(cards=>cards.map(card=>{
    const heading=card.querySelector('.t'),style=getComputedStyle(heading);
    return {height:card.getBoundingClientRect().height,titleHeight:heading.getBoundingClientRect().height,lineHeight:parseFloat(style.lineHeight)};
   }));
   assert.ok(geometry[0].titleHeight <= geometry[0].lineHeight*2+1);
   assert.ok(Math.abs(geometry[0].height-geometry[1].height)<1,'long title must not elongate the card');
  }
  console.log('Long titles truncated, full titles accessible, and equal card heights on mobile and desktop');
 } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exit(1);});
