const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try {
  for(const width of [320,390,1280]) {
   const page=await browser.newPage({viewport:{width,height:800},serviceWorkers:'block'}),errors=[];
   page.on('pageerror',e=>errors.push(e.message));
   const friends=[{id:'one',handle:'alex',display_name:'Alex',latest_book_title:'The Yellow Wallpaper',common_books:3,streak_days:1,pages_30_days:9},{id:'two',handle:'beth',display_name:'Beth',common_books:1,streak_days:10,pages_30_days:20},{id:'three',handle:'cory',display_name:'Cory',common_books:2,streak_days:4,pages_30_days:40}];
   await page.route('**/v1/**',async route=>{
    const path=new URL(route.request().url()).pathname;
    let body={};
    if(path==='/v1/friends')body={friends};
    else if(path==='/v1/friends/pending')body={pending:[]};
    else if(path==='/v1/feed/friends')body={activity:[]};
    else if(path.startsWith('/v1/users/'))body={profile:{id:'one',handle:'alex',display_name:'Alex',visibility:'friends',streak_days:1,longest_days:15,books_read:4,reading:[{book_slug:'a-christmas-carol',book_title:'A Christmas Carol',author:'Charles Dickens',progress:.4}]}};
    await route.fulfill({json:body});
   });
   const base=process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
   await page.route('**/app/u/alex',async route=>route.fulfill({response:await page.request.get(base+'/app/u/')}));
   await page.goto(base+'/app/friends/');await page.locator('.friend').first().waitFor();
   const names=()=>page.locator('.friend .name').allTextContents();
   assert.deepEqual(await names(),['Alex','Cory','Beth']);
   assert.equal(await page.locator('.friend .avatar').count(),3);
   const pattern=()=>page.locator('.friend').first().locator('.avatar rect').evaluateAll(els=>els.map(el=>[el.getAttribute('x'),el.getAttribute('y'),el.getAttribute('fill')]));
   const avatar=await pattern();
   await page.reload();await page.locator('.friend').first().waitFor();assert.deepEqual(await pattern(),avatar);
   assert.equal(await page.getByRole('heading',{name:'Reading now',exact:true}).count(),0);
   assert.equal(await page.locator('.rankings').count(),0);
   assert.equal(await page.locator('.friend').first().locator('.metric strong').textContent(),'3');
   assert.equal(await page.locator('.friend').first().locator('.latest-book').textContent(),'The Yellow Wallpaper');
   assert.equal(await page.locator('.friend').first().locator('.identity .handle').textContent(),'@alex');
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.locator('.friend').first().locator('.avatar').click();
   await page.getByRole('heading',{name:'Alex',exact:true}).waitFor();
   assert.equal(await page.locator('header .avatar').count(),1);
   assert.deepEqual(await page.locator('header .avatar rect').evaluateAll(els=>els.map(el=>[el.getAttribute('x'),el.getAttribute('y'),el.getAttribute('fill')])),avatar);
   assert.equal(await page.locator('.stats dd').count(),3);
   assert(await page.locator('.stats div').evaluateAll(els=>els.every(el=>getComputedStyle(el).backgroundColor==='rgba(0, 0, 0, 0)' && getComputedStyle(el).borderTopWidth==='0px')));
   assert.equal(await page.getByRole('link',{name:/A Christmas Carol/}).getAttribute('href'),'/read/a-christmas-carol');
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log('Compact friend list, common books, latest title, avatar profile navigation and detailed reading passed');
 }finally{await browser.close()}
})();
