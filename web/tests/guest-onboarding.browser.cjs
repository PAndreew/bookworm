const assert = require('node:assert/strict');
const samples = require('../src/lib/guest-books.json');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
 const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_EXECUTABLE, args: ['--no-sandbox'] });
 try {
  for (const width of [320, 390, 1280]) {
   const page = await browser.newPage({viewport:{width,height:800},serviceWorkers:'block'});
   const errors = [], calls = [];
   page.on('pageerror',error=>errors.push(error.message));
   await page.route('**/v1/**',async route=>{
    calls.push(new URL(route.request().url()).pathname);
    await route.fulfill({status:401,json:{error:'Sign in'}});
   });
   const base=process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
   await page.goto(base);
   assert.equal(await page.locator('#try-it-now .sample-book').count(),4);
   assert.equal(await page.locator('.guest-puri.landing button').count(),0,'landing Puri must not be a button');
   assert.equal(await page.locator('.guest-puri.landing .puri-mascot').evaluate(el=>Math.round(el.getBoundingClientRect().width)),32);
   assert(await page.locator('#try-it-now').evaluate(el=>Boolean(el.compareDocumentPosition(document.querySelector('footer'))&Node.DOCUMENT_POSITION_FOLLOWING)));
   await page.waitForSelector('.guest-puri.landing .fact',{timeout:10000});
   assert.equal(await page.locator('.fact').evaluate(el=>getComputedStyle(el).backgroundColor),'rgb(184, 238, 232)');
   assert(!calls.includes('/v1/assistant'),'landing must never request AI');
   for (const book of samples) {
    calls.length=0;
    await page.goto(`${base}/try/?book=${book.slug}`);
    await page.getByRole('heading',{name:book.title,exact:true}).waitFor();
    await page.getByRole('button',{name:'Next card',exact:true}).click();
    await page.locator('[data-demo-word]').waitFor();
    const card=book.pages[1];
    assert((await page.locator('.bubble').textContent()).includes(card.word));
    // Highlight a different word in the actual text, not a UI label.
    await page.locator('.book-text').evaluate(el=>{
     const node=el.firstChild;
     const range=document.createRange();range.setStart(node,0);range.setEnd(node,Math.min(5,node.textContent.length));
     const selection=window.getSelection();selection.removeAllRanges();selection.addRange(range);
    });
    await page.getByRole('button',{name:'Click Puri'}).click();
    await page.waitForFunction(()=>document.querySelector('.bubble')?.textContent.includes('instead'));
    assert(await page.getByRole('button',{name:'Explain',exact:true}).isDisabled());
    await page.locator('[data-demo-word]').evaluate(el=>{
     const range=document.createRange();range.selectNodeContents(el);
     const selection=window.getSelection();selection.removeAllRanges();selection.addRange(range);
    });
    await page.getByRole('button',{name:'Explain',exact:true}).waitFor();
    await page.waitForFunction(()=>!document.querySelector('.explain')?.disabled);
    await page.getByRole('button',{name:'Explain',exact:true}).click();
    assert.equal(await page.locator('.definition').textContent(),card.definition);
    // The complete preview contains exactly the ten frozen cards.
    for (let i=2;i<10;i++) {
     await page.getByRole('button',{name:'Next card',exact:true}).click();
     await page.getByRole('article',{name:`Reading card ${i+1}`,exact:true}).waitFor();
     assert((await page.locator('.book-text').textContent()).trim().toLowerCase().includes(book.pages[i].word.toLowerCase()));
    }
    await page.getByRole('button',{name:'Finish preview',exact:true}).click();
    await page.getByRole('heading',{name:'That’s your first 10 cards!'}).waitFor();
    assert.equal(await page.getByRole('link',{name:'Create an account',exact:true}).getAttribute('href'),'/signup');
    assert.deepEqual(calls,[],'guest reading/explanations must not call any backend or AI');
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth),'horizontal overflow');
   }
   assert.deepEqual(errors,[]);
   await page.close();
  }
  console.log('Guest previews, exact-word coaching, prepared definitions, non-clickable landing Puri and zero AI/API calls passed');
 } finally { await browser.close(); }
})();
