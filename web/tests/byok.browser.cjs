const assert = require('node:assert/strict');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try {
  for(const width of [320,390,1280]) {
   const page=await browser.newPage({viewport:{width,height:900},serviceWorkers:'block'});
   let used=0,tier='free',failProvider=false,failQuota=false;
   const metadata=[],providerCalls=[],paidCalls=[],errors=[];
   const source='A curious phrase from an original reading card. '.repeat(4);
   page.on('pageerror',e=>errors.push(e.message));
   await page.route('**/v1/**',async route=>{
    const req=route.request(),u=new URL(req.url());let body={};
    if(u.pathname==='/v1/me')body={user:{id:'test-user',tier,handle:'test',display_name:'Tester',visibility:'friends'},reading_activity:[]};
    else if(u.pathname==='/v1/byok/quota')body={used,limit:50,resets_at:'2027-01-01T00:00:00Z'};
    else if(u.pathname==='/v1/byok/reservations'){
     metadata.push(req.postDataJSON());assert.deepEqual(req.postDataJSON(),{});
     if(failQuota || used>=50)return route.fulfill({status:429,json:{error:'byok_limit',message:'Monthly limit reached.'}});
     body={used:++used,limit:50,resets_at:'2027-01-01T00:00:00Z'};
    }else if(u.pathname==='/v1/assistant'){paidCalls.push(req.postDataJSON());body={answer:'Membership answer.',model:'test'}}
    else if(u.pathname.endsWith('/position'))body={position:null};
    else if(u.pathname.endsWith('/cards'))body={page_count:1,pages:[{idx:0,kind:'text',id:'source-0',card_id:'card-0',content:source,char_count:source.length,like_count:0,liked_by_me:false}]};
    else if(u.pathname.endsWith('/translations')){assert.equal(req.method(),'GET','BYOK must never upload a translation');body={translations:[]};}
    else if(u.pathname.startsWith('/v1/books/'))body={book:{title:'Test book',author:'Tester'}};
    await route.fulfill({json:body});
   });
   for(const host of ['api.openai.com','api.anthropic.com','openrouter.ai'])await page.route(`https://${host}/**`,async route=>{
    const req=route.request();
    if(req.method()==='OPTIONS')return route.fulfill({status:204,headers:{'access-control-allow-origin':'*','access-control-allow-headers':'authorization,content-type,x-api-key,anthropic-version,anthropic-dangerous-direct-browser-access'}});
    const h=req.headers(),b=req.postDataJSON();providerCalls.push({host,headers:h,body:b});
    assert.equal(h.cookie,undefined);
    if(failProvider)return route.fulfill({status:401,json:{error:{message:'sensitive provider error must not be reflected'}}});
    return route.fulfill({json:host==='api.anthropic.com'?{content:[{type:'text',text:'Direct provider answer.'}]}:{choices:[{message:{content:'Direct provider answer.'}}]}});
   });
   await page.route('**/read/byok-test/**',async route=>route.fulfill({response:await page.request.get(base+'/read/')}));
   async function open(){await page.getByRole('button',{name:/Ask Puri/}).click();await page.getByLabel('AI access',{exact:true}).waitFor();}
   await page.goto(base+'/read/byok-test/');await page.waitForSelector('[data-body]');await open();
   assert.equal(await page.getByLabel('AI access',{exact:true}).inputValue(),'byok');
   await page.getByLabel('Help me',{exact:true}).selectOption('card_translate');
   const submit=page.getByRole('button',{name:'Translate card',exact:true});assert(await submit.isDisabled());
   const keys={openai:['sk','test-only-openai-not-real'].join('-'),anthropic:['sk','ant','test-only-not-real'].join('-'),openrouter:['sk','or','test-only-not-real'].join('-')};
   for(const id of ['openai','anthropic','openrouter']) {
    await page.getByLabel('Provider',{exact:true}).selectOption(id);
    await page.getByLabel('API key',{exact:true}).fill(keys[id]);
    await page.getByRole('button',{name:'Use key for this page',exact:true}).click();
    assert.equal(await page.getByLabel('API key',{exact:true}).count(),0);
    assert(await submit.isDisabled());await page.getByRole('checkbox').check();
    // Different language avoids the deliberately local translation cache.
    await page.getByLabel('Answer in',{exact:true}).selectOption({openai:'French',anthropic:'Spanish',openrouter:'German'}[id]);
    await submit.click();await page.locator('dialog[aria-labelledby="sunny-title"]').waitFor({state:'hidden'});
    assert.equal(providerCalls.length,used);
    const call=providerCalls.at(-1);
    assert.equal(call.headers[id==='anthropic'?'x-api-key':'authorization'],id==='anthropic'?keys[id]:`Bearer ${keys[id]}`);
    assert.equal(call.body.max_tokens ?? call.body.max_completion_tokens,800);
    assert(JSON.stringify(call.body).includes(source.trim()));
    if(id==='anthropic'){assert.equal(call.headers['anthropic-dangerous-direct-browser-access'],'true');assert.equal(call.headers['anthropic-version'],'2023-06-01');}
    if(id==='openrouter')assert.deepEqual(call.body.provider,{data_collection:'deny',zdr:true});
    const storage=await page.evaluate(()=>JSON.stringify({local:{...localStorage},session:{...sessionStorage}}));
    assert(!storage.includes(keys[id]));
    await open();
   }
   assert.equal(used,3);assert.equal(paidCalls.length,0);
   await submit.click();await page.locator('dialog[aria-labelledby="sunny-title"]').waitFor({state:'hidden'});assert.equal(used,3,'local cached translation must not reserve again');await open();
   // A fresh phrase goes to the provider; failure still consumes its reservation.
   await page.getByRole('button',{name:'Close reading companion',exact:true}).click();
   await page.locator('[data-body]').first().evaluate(el=>{const r=document.createRange();r.selectNodeContents(el);const s=getSelection();s.removeAllRanges();s.addRange(r);document.dispatchEvent(new Event('selectionchange'));});
   await open();await page.getByLabel('Help me',{exact:true}).selectOption('explain');
   failProvider=true;await page.getByRole('button',{name:'Explain phrase',exact:true}).click();await page.getByRole('alert').waitFor();
   assert((await page.getByRole('alert').textContent()).includes('rejected'));assert(!(await page.getByRole('alert').textContent()).includes('sensitive provider'));
   assert.equal(used,4);const before=providerCalls.length;
   failProvider=false;failQuota=true;await page.getByRole('button',{name:'Explain phrase',exact:true}).click();
   await page.getByText('Monthly limit reached.',{exact:true}).waitFor();assert.equal(providerCalls.length,before,'quota rejection must prevent provider call');
   await page.getByRole('button',{name:'Forget key',exact:true}).click();assert(await page.getByRole('button',{name:'Explain phrase',exact:true}).isDisabled());
   await page.getByLabel('Provider',{exact:true}).selectOption('openai');await page.getByLabel('API key',{exact:true}).fill('eyJ-codex-token-not-an-api-key');await page.getByRole('button',{name:'Use key for this page',exact:true}).click();
   assert((await page.getByRole('alert').textContent()).includes('not a password'));
   // Paid users retain the existing membership route and default.
   tier='plus';failQuota=false;await page.reload();await page.waitForSelector('[data-body]');await open();assert.equal(await page.getByLabel('AI access',{exact:true}).inputValue(),'paid');
   await page.getByLabel('AI access',{exact:true}).selectOption('byok');assert.equal(await page.getByLabel('API key',{exact:true}).inputValue(),'','reload must clear key');
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log('BYOK browser checks passed: three direct providers, metadata-only quota, consent, free/paid separation, page-local translations/cache, failures, forget/reload and token rejection (320/390/1280px).');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
