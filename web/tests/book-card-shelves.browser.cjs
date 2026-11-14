const assert = require('node:assert/strict');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const browser=await chromium.launch({args:['--no-sandbox']});
 try {
  const page=await browser.newPage({serviceWorkers:'block'});
  const books=[
   {id:'one',slug:'long',title:'A very long title for a collected edition',author:'An exceptionally long author name and additional historical contributors '.repeat(5),genres:['fiction','historical-fiction','philosophy','poetry','drama'],license:'public_domain',card_count:250,rating_count:12,rating_average:4.5,last_read_at:'2026-11-14T00:00:00Z',is_latest:true},
   {id:'two',slug:'short',title:'Short Title',author:'Author',genres:['fiction'],license:'public_domain',card_count:100}
  ];
  await page.route('**/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;
   const body=path==='/v1/me'?{user:{tier:'free'}}:path==='/v1/books'?{books,next_offset:null}:path==='/v1/genres'?{genres:[]}:path==='/v1/me/ratings'?{ratings:{}}:{current:0};
   await route.fulfill({json:body});
  });
  for(const width of [320,360,390,540,768,1024]) {
   await page.setViewportSize({width,height:900});
   await page.goto((process.env.TEST_BASE_URL||'http://127.0.0.1:4333')+'/app');
   await page.locator('.horizontal .card').first().waitFor();
   const dimensions=await page.locator('.horizontal .card,.grid .card').evaluateAll(cards=>cards.map(c=>({width:c.getBoundingClientRect().width,height:c.getBoundingClientRect().height})));
   console.log(width,dimensions);
   assert.equal(dimensions.length,4);
   for(const d of dimensions){
    assert.ok(Math.abs(d.height-dimensions[0].height)<1,'card heights must match across shelves and content');
    assert.ok(Math.abs(d.width-dimensions[0].width)<1,'card widths must match across shelves');
    assert.equal(d.height,270,'shared component owns the card height');
   }
   const stars=await page.locator('.card .rating').evaluateAll(els=>els.map(el=>{const r=el.getBoundingClientRect(),c=el.closest('.card').getBoundingClientRect();return r.bottom<=c.bottom&&r.top>=c.top;}));
   assert.ok(stars.every(Boolean),'ratings must remain visible');
   const rows=await page.locator('.rating').evaluateAll(els=>els.map(el=>new Set([...el.querySelectorAll('button')].map(b=>b.getBoundingClientRect().top)).size));
   assert.ok(rows.every(count=>count===1),'stars must stay on one row in narrow cards');
  }
  console.log('Shared BookCard dimensions match across My books and All books');
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exit(1);});
