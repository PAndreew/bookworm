const assert=require('node:assert/strict');
const {chromium,webkit}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base=process.env.TEST_BASE_URL || 'http://127.0.0.1:4340';
(async()=>{
 const browser=await (process.env.TEST_WEBKIT?webkit:chromium).launch(process.env.TEST_WEBKIT?{}:{executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try{for(const width of [320,390,1280]){
  const page=await browser.newPage({viewport:{width,height:850},serviceWorkers:'block'});
  let used=0,tier='free',account='byok-test',failProvider=false,failQuota=false;
  const savedByAccount={};
  const calls=[],catalogs=[],metadata=[],paid=[],errors=[];const source='A curious phrase from an original reading card. '.repeat(4);
  page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/v1/**',async route=>{
   const r=route.request(),u=new URL(r.url());let json={};
   if(u.pathname==='/v1/me')json={user:{id:account,tier,handle:'tester',display_name:'Tester',visibility:'friends',email_verified:true},reading_activity:[]};
   else if(u.pathname==='/v1/byok/quota')json={used,limit:50,resets_at:'2027-01-01T00:00:00Z'};
   else if(u.pathname==='/v1/byok/reservations'){
    metadata.push(r.postDataJSON());assert.deepEqual(r.postDataJSON(),{});
    if(failQuota||used>=50)return route.fulfill({status:429,json:{error:'byok_limit',message:'Monthly limit reached.'}});
    json={used:++used,limit:50,resets_at:'2027-01-01T00:00:00Z'};
   }else if(u.pathname==='/v1/assistant'){paid.push(r.postDataJSON());json={answer:'Membership answer.',model:'test'}}
   else if(u.pathname.endsWith('/position'))json={position:null};
   else if(u.pathname.endsWith('/cards'))json={page_count:1,pages:[{idx:0,kind:'text',id:'source-0',card_id:'card-0',content:source,char_count:source.length,like_count:0}]};
   else if(u.pathname.endsWith('/puri-results')){
    if(r.method()==='POST'){
     const value=r.postDataJSON();assert.equal(value.source,source);assert(!JSON.stringify(value).includes('sk-test-only'));
     savedByAccount[account]=[value];json={saved:true};
    }else json={results:savedByAccount[account]||[]};
   }
   else if(u.pathname.endsWith('/translations')){assert.equal(r.method(),'GET');json={translations:[]};}
   else if(u.pathname==='/v1/highlights')json={highlights:[]};
   else if(u.pathname.startsWith('/v1/books/'))json={book:{title:'Test',author:'Tester'}};
   await route.fulfill({json});
  });
  for(const host of ['api.openai.com','api.anthropic.com','openrouter.ai'])await page.route(`https://${host}/**`,async route=>{
   const r=route.request(),url=new URL(r.url()),headers=r.headers();assert.equal(headers.cookie,undefined);
   if(r.method()==='OPTIONS')return route.fulfill({status:204,headers:{'access-control-allow-origin':'*','access-control-allow-headers':'authorization,content-type,x-api-key,anthropic-version,anthropic-dangerous-direct-browser-access'}});
   if(url.pathname.endsWith('/models')){
    catalogs.push({host,headers});
    const data=host==='api.openai.com'?[{id:'gpt-older-example',created:1},{id:'gpt-current-example',created:9999999999},{id:'text-embedding-example',created:9999999999},{id:'gpt-retired-example',shutdown_date:'2020-01-01',created:9999999999}]
     :host==='api.anthropic.com'?url.searchParams.has('after_id')?[{id:'claude-older-example',created_at:'2000-01-01'}]:[{id:'claude-current-example',display_name:'Current Claude example',created_at:'2030-01-01'}]
     :[{id:'example/current-text',name:'Current text example',created:9999999999,architecture:{input_modalities:['text'],output_modalities:['text']}},{id:'example/image-only',architecture:{output_modalities:['image']}}];
    return route.fulfill({json:{data,has_more:host==='api.anthropic.com'&&!url.searchParams.has('after_id'),last_id:'claude-current-example'}});
   }
   calls.push({host,headers,body:r.postDataJSON(),url:r.url()});
   if(failProvider)return route.fulfill({status:401,json:{error:{message:'provider sensitive details must not be shown'}}});
   return route.fulfill({json:host==='api.openai.com'?{output:[{type:'reasoning',summary:[]},{type:'message',content:[{type:'output_text',text:'Direct answer.'}]}]}:host==='api.anthropic.com'?{content:[{type:'text',text:'Direct answer.'}]}:{choices:[{message:{content:'Direct answer.'}}]}});
  });
  await page.route('**/read/byok-test/**',async route=>route.fulfill({response:await page.request.get(base+'/read/')}));
  const keys={openai:['sk','test-only-openai-not-real'].join('-'),anthropic:['sk','ant','test-only-not-real'].join('-'),openrouter:['sk','or','test-only-not-real'].join('-')};
  async function setup(id,first){
   await page.goto(base+'/app/me#byok');await page.getByLabel('Provider',{exact:true}).waitFor();
   await page.getByLabel('Provider',{exact:true}).selectOption(id);await page.getByLabel('API key',{exact:true}).fill(keys[id]);
   await page.getByRole('button',{name:'Refresh models',exact:true}).click();await page.getByLabel('Model · newest first',{exact:true}).waitFor();
   const choices=await page.getByLabel('Model · newest first',{exact:true}).locator('option').allTextContents();
   assert(!choices.join(' ').includes('embedding'));assert(!choices.join(' ').includes('retired'));assert(!choices.join(' ').includes('image-only'));
   assert(choices[1].includes({openai:'gpt-current-example',anthropic:'claude-current-example',openrouter:'example/current-text'}[id]),'newest text model should be first');
   assert.equal(await page.getByLabel('Model · newest first',{exact:true}).inputValue(),'','do not silently select a model');
   await page.getByLabel('Model · newest first',{exact:true}).selectOption({openai:'gpt-current-example',anthropic:'claude-current-example',openrouter:'example/current-text'}[id]);
   if(first){assert(await page.getByRole('button',{name:'Save on this device',exact:true}).isDisabled());await page.getByRole('checkbox',{name:/I accept the BYOK terms/}).check();}
   else assert.equal(await page.getByRole('checkbox',{name:/I accept the BYOK terms/}).count(),0,'terms should be accepted once');
   await page.getByRole('button',{name:'Save on this device',exact:true}).click();await page.getByText('Saved on this device.',{exact:true}).waitFor();
  }
  async function open(){await page.getByRole('button',{name:/Ask Puri/}).click();await page.getByLabel('AI access',{exact:true}).waitFor();}
  for(const [i,id] of ['openai','anthropic','openrouter'].entries()){
   savedByAccount[account]=[];
   await setup(id,i===0);assert.equal(used,i,'catalog fetches must not consume inference allowance');
   await page.reload();await page.getByLabel('API key',{exact:true}).waitFor();assert.equal(await page.getByLabel('API key',{exact:true}).inputValue(),keys[id]);assert.equal(await page.getByRole('checkbox',{name:/I accept/}).count(),0);
   await page.goto(base+'/read/byok-test/');await page.waitForSelector('[data-body]');await open();
   assert.equal(await page.getByRole('checkbox').count(),0);assert.equal(await page.getByLabel('API key',{exact:true}).count(),0,'reader must not contain credential setup');
   await page.getByLabel('Help me',{exact:true}).selectOption('card_translate');await page.getByLabel('Answer in',{exact:true}).selectOption('French');
   await page.getByRole('button',{name:'Translate card',exact:true}).click();await page.locator('dialog[aria-labelledby="sunny-title"]').waitFor({state:'hidden'});
   const call=calls.at(-1);assert.equal(calls.length,i+1);assert(JSON.stringify(call.body).includes(source.trim()));assert.equal(call.headers[id==='anthropic'?'x-api-key':'authorization'],id==='anthropic'?keys[id]:`Bearer ${keys[id]}`);
   if(id==='openai'){assert(call.url.endsWith('/responses'));assert.equal(call.body.store,false);assert.equal(call.body.model,'gpt-current-example');assert.equal(call.body.max_output_tokens,2000);}
   if(id==='anthropic')assert.equal(call.headers['anthropic-dangerous-direct-browser-access'],'true');
   if(id==='openrouter')assert.deepEqual(call.body.provider,{data_collection:'deny',zdr:true});
   await open();await page.getByRole('button',{name:'Translate card',exact:true}).click();await page.locator('dialog[aria-labelledby="sunny-title"]').waitFor({state:'hidden'});assert.equal(used,i+1,'cached translation should not reserve');
  }
  assert.equal(savedByAccount[account][0].content,'Direct answer.');
  await page.reload();await page.waitForSelector('[data-body]');await page.getByLabel('Card language').waitFor();
  await page.getByLabel('Card language').selectOption('French');
  assert((await page.locator('[data-body]').textContent()).includes('Direct answer.'));
  await page.getByLabel('Card language').selectOption('Original');
  await page.locator('[data-body]').evaluate(el=>{const r=document.createRange();r.selectNodeContents(el);const s=getSelection();s.removeAllRanges();s.addRange(r);document.dispatchEvent(new Event('selectionchange'));});
  await open();await page.getByLabel('Help me',{exact:true}).selectOption('explain');failProvider=true;await page.getByRole('button',{name:'Explain phrase',exact:true}).click();await page.getByRole('alert').waitFor();assert((await page.getByRole('alert').textContent()).includes('rejected'));assert(!(await page.getByRole('alert').textContent()).includes('sensitive'));assert.equal(used,4);
  const before=calls.length;failProvider=false;failQuota=true;await page.getByRole('button',{name:'Explain phrase',exact:true}).click();await page.getByText('Monthly limit reached.',{exact:true}).waitFor();assert.equal(calls.length,before);await page.keyboard.press('Escape');
  account='different-user';await page.goto(base+'/app/me');await page.getByLabel('API key',{exact:true}).waitFor();assert.equal(await page.getByLabel('API key',{exact:true}).inputValue(),'');assert.equal(await page.getByRole('checkbox',{name:/I accept/}).count(),1);
  account='byok-test';await page.goto(base+'/app/me');await page.getByLabel('API key',{exact:true}).waitFor();await page.getByRole('button',{name:'Forget key',exact:true}).click();await page.reload();await page.getByLabel('API key',{exact:true}).waitFor();assert.equal(await page.getByLabel('API key',{exact:true}).inputValue(),'');assert.equal(await page.getByRole('checkbox',{name:/I accept/}).count(),0,'forget preserves one-time terms');
  tier='plus';await page.goto(base+'/read/byok-test/');await page.waitForSelector('[data-body]');await open();assert.equal(await page.getByLabel('AI access',{exact:true}).inputValue(),'paid');await page.getByLabel('AI access',{exact:true}).selectOption('byok');await page.getByRole('button',{name:'Close reading companion',exact:true}).click();await open();assert.equal(await page.getByLabel('AI access',{exact:true}).inputValue(),'byok');
  assert(catalogs.length>=6);assert.deepEqual(errors,[]);await page.close();
 }
 console.log('BYOK You-page setup, live model lists/pagination/filtering, persistent account-scoped keys, one-time terms, direct inference, quota/failures, private translations and forget passed.');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
