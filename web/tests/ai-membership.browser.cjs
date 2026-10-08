const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try{
  for(const width of [320,390,1280]){
   const page=await browser.newPage({viewport:{width,height:800},serviceWorkers:'block'});
   const errors=[],posts=[];page.on('pageerror',e=>errors.push(e.message));let tier='free',cancelled=false;
   const source='An original English reading card with a curious unfamiliar word. '.repeat(4);
   const french='Un texte français de lecture avec un mot curieux. '.repeat(4);
   await page.route('**/v1/**',async route=>{
    const req=route.request(),u=new URL(req.url());let body={};
    if(u.pathname==='/v1/me')body={user:{tier,handle:'test',display_name:'Tester',visibility:'friends',totp_enabled:false},last_read:null};
    else if(u.pathname==='/v1/billing/offer')body={available:true,remaining:50,limit:50,price_eur:20,requests_per_month:100};
    else if(u.pathname==='/v1/billing')body={tier,used:2,limit:tier==='free'?0:100,subscription:tier==='free'?null:{status:'active',period_end:'2027-01-01T00:00:00Z',cancel_at_period_end:cancelled}};
    else if(u.pathname==='/v1/billing/cancel'){cancelled=true;body={message:'Renewal cancelled. Access continues until next year.'}}
    else if(u.pathname==='/v1/books'){assert.equal(u.searchParams.get('license'),'public_domain');body={books:[],next_offset:null}}
    else if(u.pathname==='/v1/genres')body={genres:[]};
    else if(u.pathname.endsWith('/position'))body={position:null};
    else if(u.pathname.endsWith('/cards'))body={page_count:3,pages:[0,1,2].map(idx=>({idx,kind:'text',id:`source-${idx}`,card_id:`card-${idx}`,content:source,char_count:source.length,like_count:0,liked_by_me:false}))};
    else if(u.pathname.endsWith('/translations')){
     const id=u.pathname.split('/')[3];
     if(req.method()==='POST'){posts.push(req.postDataJSON());body={cached:false,translation:{language:'French',content:french,model:'google/gemini-3.8-flash'}}}
     else body={translations:[{language:id==='card-0'?'German':'Spanish',content:id==='card-0'?'Ein deutscher Text.':'Un texto español guardado.',model:'google/gemini-3.8-flash'}]};
    }else if(u.pathname.startsWith('/v1/books/'))body={book:{title:'Test book',author:'Tester'}};
    await route.fulfill({json:body});
   });
   const base=process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
   await page.goto(base+'/app');await page.getByRole('button',{name:'My books',exact:true}).waitFor();
   assert.equal(await page.getByRole('button',{name:'Plus',exact:true}).count(),0);
   await page.goto(base+'/app/me');await page.getByRole('button',{name:'Subscribe',exact:true}).waitFor();
   assert((await page.locator('.plans').textContent()).includes('€20/year'));
   assert.equal(await page.getByRole('link',{name:'Get Pro',exact:true}).count(),0);
   const spacing=await page.locator('.box').evaluateAll(els=>els.map(el=>({padding:getComputedStyle(el).padding,gap:getComputedStyle(el).gap})));
   assert.deepEqual(spacing[0],spacing[1]);assert.equal(spacing[0].gap,'12px');
   tier='plus';await page.goto(base+'/app/me');await page.getByRole('button',{name:'Unsubscribe',exact:true}).waitFor();
   page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Unsubscribe',exact:true}).click();await page.getByRole('button',{name:'Renewal cancelled',exact:true}).waitFor();
   assert(await page.getByRole('button',{name:'Renewal cancelled',exact:true}).isDisabled());
   await page.route('**/read/ai-demo/**',async route=>route.fulfill({response:await page.request.get(base+'/read/')}));
   await page.goto(base+'/read/ai-demo/');await page.waitForSelector('[data-body]');
   await page.locator('[data-idx="0"]').getByLabel('Card language',{exact:true}).waitFor();
   await page.getByRole('button',{name:/Ask Puri/}).click();await page.getByLabel('Help me',{exact:true}).selectOption('card_translate');
   assert.deepEqual(await page.getByLabel('Answer in',{exact:true}).locator('option').allTextContents(),['French','Spanish','German']);
   await page.getByRole('button',{name:'Translate card',exact:true}).click();await page.waitForSelector('.ai-badge');
   assert.equal(posts.length,1);assert.equal(posts[0].language,'French');
   assert((await page.locator('[data-idx="0"] [data-body]').textContent()).includes('Un texte français'));
   const badge=await page.locator('.ai-badge').boundingBox(),card=await page.locator('[data-idx="0"]').boundingBox();
   assert(Math.abs(badge.x-card.x-16)<1&&Math.abs(badge.y-card.y-12)<1,'AI badge must be top-left');
   assert(await page.locator('[data-idx="0"]').getByRole('button',{name:'Share the selected quote',exact:true}).isDisabled());
   await page.locator('[data-idx="0"]').getByLabel('Card language',{exact:true}).selectOption('German');assert((await page.locator('[data-body]').first().textContent()).includes('Ein deutscher Text'));
   await page.locator('[data-idx="0"]').getByLabel('Card language',{exact:true}).selectOption('Original');assert((await page.locator('[data-body]').first().textContent()).includes('An original English'));
   assert.equal(await page.locator('.ai-badge').count(),0);assert.equal(posts.length,1,'language switching must not regenerate translations');
   if(await page.getByRole('button',{name:'Next card',exact:true}).count())await page.getByRole('button',{name:'Next card',exact:true}).click();else await page.locator('[data-idx="1"]').scrollIntoViewIfNeeded();
   await page.locator('[data-idx="1"]').getByLabel('Card language',{exact:true}).selectOption('Spanish');assert((await page.locator('[data-idx="1"] [data-body]').textContent()).includes('Un texto español'));
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log('Only founding offer, subscription/cancellation, equal profile spacing, saved full-card translations, language switching and AI badge passed');
 }finally{await browser.close()}
})();
