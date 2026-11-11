const assert = require('node:assert/strict');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
(async()=>{
 const browser=await chromium.launch({args:['--no-sandbox']});
 try { for(const width of [320,390,1280]) {
  const page=await browser.newPage({viewport:{width,height:850},serviceWorkers:'block'});
  const calls=[],errors=[];let signedIn=false;
  page.on('pageerror',e=>errors.push(e.message));
  const source='A curious phrase from a classic book.';
  await page.route('**/v1/**',async route=>{
   const u=new URL(route.request().url());calls.push(u.pathname);let json={};
   if(u.pathname==='/v1/me')return route.fulfill(signedIn?{json:{user:{id:'new-account',tier:'free',handle:'reader'}}}:{status:401,json:{message:'Please sign in.'}});
   if(u.pathname.endsWith('/cards')) json={page_count:30,pages:Array.from({length:12},(_,i)=>({idx:i,kind:'text',id:'source-'+i,card_id:'card-'+i,content:source,char_count:source.length,like_count:0}))};
   if(u.pathname.endsWith('/position'))return route.fulfill({status:401,json:{message:'Please sign in.'}});
   if(u.pathname.endsWith('/puri-results'))json={results:[]};
   if(u.pathname==='/v1/byok/quota')json={used:0,limit:50};
   await route.fulfill({json});
  });
  await page.route('https://api.openai.com/**',async route=>{
   if(route.request().url().endsWith('/models'))return route.fulfill({json:{data:[{id:'gpt-test',created:1}]}});
   await route.fulfill({json:{output:[{type:'message',content:[{type:'output_text',text:'Guest AI answer.'}]}]}});
  });
  await page.goto(base+'/read/gutenberg-46/');await page.waitForSelector('[data-body]');
  await page.locator('[data-body]').evaluate(el=>{const r=document.createRange();r.selectNodeContents(el);const s=getSelection();s.removeAllRanges();s.addRange(r);document.dispatchEvent(new Event('selectionchange'));});
  await page.getByRole('button',{name:/Ask Puri/}).click();
  await page.getByLabel('API key',{exact:true}).fill('sk-test-only-guest-not-real');
  await page.getByRole('button',{name:'Refresh models',exact:true}).click();
  await page.getByLabel('Model · newest first',{exact:true}).selectOption('gpt-test');
  await page.getByRole('checkbox',{name:/I accept the BYOK terms/}).check();
  await page.getByRole('button',{name:'Save on this device',exact:true}).click();
  await page.getByRole('button',{name:'Explain phrase',exact:true}).click();
  await page.getByText('Guest AI answer.',{exact:true}).waitFor();
  await page.getByLabel('Help me',{exact:true}).selectOption('card_translate');
  await page.getByRole('button',{name:'Translate card',exact:true}).click();
  await page.getByLabel('Card language').waitFor();
  assert.equal(await page.getByLabel('Card language').inputValue(),'French');
  assert(!calls.includes('/v1/byok/reservations'),'guests use only their own provider credits');
  assert(!calls.some(p=>p.endsWith('/puri-results')),'guest outputs must not be uploaded');
  await page.reload();await page.waitForSelector('[data-body]');
  assert.equal(await page.getByLabel('Card language').count(),0,'guest translations disappear on reload');
  await page.getByRole('button',{name:/Ask Puri/}).click();
  await page.getByLabel('API key',{exact:true}).waitFor();
  assert.equal(await page.getByLabel('API key',{exact:true}).inputValue(),'sk-test-only-guest-not-real');
  signedIn=true;await page.reload();await page.waitForSelector('[data-body]');
  await page.getByRole('button',{name:/Ask Puri/}).click();await page.getByLabel('AI access',{exact:true}).waitFor();
  assert.equal(await page.getByLabel('API key',{exact:true}).count(),0,'signed-in accounts must not inherit a guest key');
  assert(await page.getByRole('button',{name:'Explain phrase',exact:true}).isDisabled());
  assert.deepEqual(errors,[]);await page.close();
 }
 console.log('Anonymous BYOK explanations/translations, reload behavior and guest credential isolation passed');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
