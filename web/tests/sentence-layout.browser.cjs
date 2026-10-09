const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/home/pi/pi-web-local/node_modules/playwright');
const base = process.env.TEST_BASE_URL || 'http://localhost:4322';
const samples = require('../src/lib/guest-books.json');
const translations = require('../src/lib/guest-translations.json');
for (const book of samples) {
 for (const card of book.pages.slice(1)) assert(/[.!?][”’"']?\s*$/.test(card.content),`${book.slug}: stale mid-sentence demo chunk`);
 assert.equal(translations[book.slug].source,book.pages[2].content,`${book.slug}: stale demo translation`);
}
(async () => {
 const browser = await chromium.launch({executablePath: process.env.PLAYWRIGHT_EXECUTABLE || '/usr/bin/chromium-browser', args:['--no-sandbox']});
 try {
 for (const viewport of [{width:320,height:568},{width:1280,height:900}]) {
  const page = await browser.newPage({viewport, serviceWorkers:'block'});
  page.on('pageerror', error => console.error(error));
  const sentence = 'Mr. Bingley listened carefully while the visitors described their journey through the countryside and the quiet villages near his new home.';
  let content = 'A short complete sentence.';
  await page.route('**/v1/**', route => {
   const path = new URL(route.request().url()).pathname;
   const body = path.endsWith('/cards') ? {pages:[{id:'source',card_id:'card',idx:0,kind:'text',content}],page_count:1}
    : path === '/v1/me' ? {user:{tier:'free'}} : {book:{title:'Sentence fixture',author:'Tester'}};
   return route.fulfill({json:body});
  });
  await page.goto(base+'/read/pride-and-prejudice/',{waitUntil:'networkidle'});
  let top = await page.locator('[data-body]').evaluate(el=>el.getBoundingClientRect().top);
  assert(top <= 48,`text starts at ${top}px on ${viewport.width}px screen`);
  await page.getByRole('button',{name:'Text size',exact:true}).click();
  assert.deepEqual(await page.locator('.scale-btn').allTextContents(),['100%','110%']);
  content = Array(6).fill(sentence).join(' ');
  await page.reload({waitUntil:'networkidle'});
  let collected = '';
  for(let n=0;n<10;n++) {
   const text = await page.locator('[data-body]').textContent(); collected += text;
   assert(/[.!?][”’"']?\s*$/.test(text),'screen ends mid-sentence: '+text.slice(-70));
   const label = await page.locator('.page-num').textContent();
   const part = label.match(/part (\d+)\/(\d+)/);
   if(!part || part[1]===part[2])break;
   await page.getByRole('button',{name:'Next card',exact:true}).click();
  }
  assert.equal(collected,content,'continuations changed text');
  await page.close();
 }
 // All guest books use the real reader, not only the mocked fitting fixture.
 const demo = await browser.newPage({viewport:{width:320,height:568},serviceWorkers:'block'});
 for (const book of samples) {
  await demo.goto(base+'/try/?book='+book.slug,{waitUntil:'networkidle'});
  await demo.getByRole('button',{name:'Next card',exact:true}).click();
  await demo.waitForTimeout(100);
  const top = await demo.locator('[data-body]').evaluate(el=>el.getBoundingClientRect().top);
  assert(top <= 48, `${book.slug}: demo is not top aligned`);
  assert(!(await demo.locator('.page-num').textContent()).includes('part'),`${book.slug}: opening prose card split`);
 }
 await demo.close();
 } finally { await browser.close(); }
})().catch(e=>{console.error(e);process.exit(1)});
