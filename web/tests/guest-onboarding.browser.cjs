const assert=require('node:assert/strict');
const samples=require('../src/lib/guest-books.json');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try {
  for(const width of [320,390,1280]) {
   const page=await browser.newPage({viewport:{width,height:800},serviceWorkers:'block'});
   const calls=[],errors=[];page.on('pageerror',e=>errors.push(e.message));
   let live=samples[0];
   await page.route('**/v1/**',async route=>{
    const url=new URL(route.request().url());calls.push(url.pathname);
    let body={};let status=200;
    if(url.pathname==='/v1/me'){status=401;body={error:'Sign in'}}
    else if(url.pathname==='/v1/books')body={books:samples.map(b=>({...b,id:b.slug})),next_offset:null};
    else if(url.pathname==='/v1/genres')body={genres:[]};
    else if(url.pathname.endsWith('/position'))body={position:null};
    else if(url.pathname.endsWith('/cards'))body={page_count:10,pages:live.pages.map((p,idx)=>({...p,idx,id:`source-${idx}`,card_id:`card-${idx}`,char_count:p.content.length,liked_by_me:false,like_count:0}))};
    else if(url.pathname.startsWith('/v1/books/'))body={book:live};
    await route.fulfill({status,json:body});
   });
   const base=process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
   await page.route('**/read/demo/**',async route=>route.fulfill({response:await page.request.get(base+'/read/')}));
   const geometry=async selector=>page.locator(selector).evaluateAll(els=>els.map(el=>{
    const rect=el.getBoundingClientRect(),style=getComputedStyle(el);
    return {width:rect.width,height:rect.height,padding:style.padding,bg:style.backgroundColor,radius:style.borderRadius,text:el.textContent.trim()};
   }));
   await page.goto(base);await page.waitForSelector('#try-it-now .card');
   assert.equal(await page.locator('#try-it-now .card').count(),4);
   assert.equal(await page.locator('button.mascot').count(),0);
   assert.equal(await page.locator('#try-it-now .resume-label').count(),0);
   assert.equal(await page.locator('#try-it-now .rating button').count(),20);
   const landingCards=await geometry('#try-it-now .card');
   assert.equal(await page.locator('.coach').count(),0,'landing has no fun facts');
   assert.equal(await page.locator('.tag').textContent(),'Read timeless classics, bit by bit.');
   assert(!calls.includes('/v1/assistant'));
   await page.goto(base+'/app');await page.waitForSelector('.grid .card');
   assert.deepEqual(await geometry('.grid .card'),landingCards,'sample cards must be identical to library cards');
   for(const book of samples) {
    live=book;
    await page.goto(base+'/read/demo/');await page.waitForSelector('[data-idx="0"]');
    const realTitle=await geometry('[data-idx="0"]');
    calls.length=0;
    await page.goto(`${base}/try/?book=${book.slug}`);await page.waitForSelector('[data-idx="0"]');
    assert.deepEqual(await geometry('[data-idx="0"]'),realTitle,'preview must use the same Reader interface');
    const turn=async index=>{
     if(await page.getByRole('button',{name:'Next card',exact:true}).count())await page.getByRole('button',{name:'Next card',exact:true}).click();
     else await page.locator(`[data-idx="${index}"]`).scrollIntoViewIfNeeded();
     await page.waitForTimeout(120);
    };
    await turn(1);
    const body=page.locator('[data-idx="1"] [data-body]');await body.waitFor();
    const card=book.pages[1];
    assert((await page.locator('.bubble').textContent()).includes(card.word));
    assert(await page.locator('[data-idx="1"] .action').evaluateAll(buttons=>buttons.every(b=>b.disabled)));
    if(await page.locator('.report-icon').count())assert(await page.locator('.report-icon').isDisabled());
    async function select(correct){
     await body.evaluate((el,word)=>{
      const node=el.firstChild,text=node.textContent,match=new RegExp(`\\b${word}\\b`,'i').exec(text);
      const start=word?match.index:0,length=word?match[0].length:5;
      const range=document.createRange();range.setStart(node,start);range.setEnd(node,start+length);
      const selection=window.getSelection();selection.removeAllRanges();selection.addRange(range);
      document.dispatchEvent(new Event('selectionchange'));
     },correct?card.word:'');
    }
    await select(false);await page.getByRole('button',{name:'Click Puri'}).click();
    await page.waitForFunction(()=>document.querySelector('.bubble')?.textContent.includes('instead'));
    assert(await page.getByRole('button',{name:'Explain',exact:true}).isDisabled());
    await select(true);await page.getByRole('button',{name:'Explain',exact:true}).click();
    assert.equal(await page.locator('.definition').textContent(),card.definition);
    await turn(2);
    await page.getByRole('button',{name:'Click Puri'}).click();
    await page.getByRole('button',{name:'Translate card',exact:true}).click();
    await page.getByLabel('Card language').waitFor();
    assert.equal(await page.getByLabel('Card language').inputValue(),'French');
    assert.equal(await page.locator('.ai-badge').textContent(),'AI translation');
    assert((await page.locator('.bubble').textContent()).includes('prepared translation'));
    await page.getByLabel('Card language').selectOption('Original');
    assert.equal(await page.locator('.ai-badge').count(),0);
    assert((await page.locator('[data-idx="2"] [data-body]').textContent()).startsWith(book.pages[2].content));
    for(let i=3;i<10;i++)await turn(i);
    if(await page.getByRole('button',{name:'Next card',exact:true}).count())await page.getByRole('button',{name:'Next card',exact:true}).click();
    else await page.getByRole('button',{name:'Finish preview',exact:true}).click();
    await page.getByRole('heading',{name:'That’s your first 10 cards!'}).waitFor();
    assert.equal(await page.getByRole('link',{name:'Create an account',exact:true}).getAttribute('href'),'/signup');
    assert.deepEqual(calls,[],'demo must never request AI, reading credit or any API');
   }
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log('Shared library cards and Reader/Puri, disabled demo actions, ten-card limit and no-AI coaching passed');
 }finally{await browser.close()}
})();
