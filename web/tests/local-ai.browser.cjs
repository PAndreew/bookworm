// Mocked worker verifies product boundaries without AI/provider spend.
// REAL_LOCAL_AI=1 instead runs the actual pinned Qwen model through WebGPU.
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.TEST_BASE_URL || 'http://127.0.0.1:4338';
const real = process.env.REAL_LOCAL_AI === '1';
const workerMock = `
let next = 0;
self.onmessage = ({data: msg}) => {
 if(['setLogLevel','setAppConfig','keepAlive'].includes(msg.kind))return;
 let content = null;
 if(msg.kind === 'reload') self.postMessage({kind:'initProgressCallback',content:{progress:1,text:'Ready (test worker)'}});
 if(msg.kind === 'chatCompletionNonStreaming') content={choices:[{message:{content:'Hello'},finish_reason:'stop'}]};
 if(msg.kind === 'chatCompletionStreamInit') next=0;
 if(msg.kind === 'completionStreamNextChunk') { content = next++ === 0 ? {choices:[{delta:{content:'Bonjour, mon ami.'},finish_reason:'stop'}]} : undefined; }
 self.postMessage({kind:'return',uuid:msg.uuid,content});
};`;
(async () => {
 const browser = await chromium.launch({executablePath: process.env.PLAYWRIGHT_EXECUTABLE, args:['--no-sandbox','--enable-unsafe-webgpu']});
 try {
  for (const width of real ? [1280] : [320,390,1280]) {
   const context = await browser.newContext({viewport:{width,height:800},serviceWorkers:'block'});
   const page = await context.newPage(); const errors=[],reservations=[],cloud=[],downloads=[];
   const monitor = real ? setInterval(async()=>{try{console.log('progress',await page.locator('.local-progress').allTextContents(),await page.locator('.error').allTextContents())}catch{}},15000) : null;
   page.on('pageerror',e=>errors.push(e.message));
   page.on('request',r=>{if(/huggingface|githubusercontent/.test(r.url()))downloads.push(r.url())});
   page.on('console',msg=>{if(real && /error|warn/.test(msg.type()))console.log(msg.type(),msg.text().slice(0,600))});
   let used=0,limit=10,workerBehavior='normal';
   if(!real) {
    await page.addInitScript(()=>Object.defineProperty(navigator,'gpu',{value:{requestAdapter:async()=>({features:new Set(['shader-f16'])})}}));
    await page.route('**/local-ai.worker-*.js',route=>route.fulfill({contentType:'text/javascript',body:workerBehavior==='stall' ? 'self.onmessage=()=>{};' : workerMock}));
   }
   await page.route('**/v1/**',async route=>{
    const r=route.request(),u=new URL(r.url());let body={};let status=200;
    if(u.pathname==='/v1/me')body={user:{tier:'free',handle:'test'}};
    else if(u.pathname==='/v1/assistant/local') {
     if(r.method()==='POST') {reservations.push(r.postDataJSON());if(used>=limit)status=429;else used++;}
     body=status===200?{used,limit,resets_at:'2027-01-01T00:00:00Z'}:{error:{code:'local_assistant_limit',message:'Monthly allowance used up.'}};
    }else if(u.pathname==='/v1/assistant') {cloud.push(r.postDataJSON());body={answer:'Unexpected cloud call'};}
    else if(u.pathname.endsWith('/position'))body={position:null};
    else if(u.pathname.endsWith('/cards'))body={page_count:1,pages:[{idx:0,kind:'text',id:'source-0',card_id:'card-0',content:'Bonjour, mon ami. The curious traveller greeted his friend warmly.',char_count:67,like_count:0,liked_by_me:false}]};
    else if(u.pathname.startsWith('/v1/books/'))body={book:{title:'Local AI test',author:'Tester'}};
    await route.fulfill({status,json:body});
   });
   await page.route('**/read/local-ai-test/**',async route=>route.fulfill({response:await page.request.get(base+'/read/')}));
   await page.goto(base+'/read/local-ai-test/',{waitUntil:'networkidle'});
   await page.waitForSelector('[data-body]');
   assert.equal(downloads.length,0,'model downloads before opt-in');
   async function selectPhrase() {
    await page.evaluate(()=>{const n=document.querySelector('[data-body]').firstChild;const r=document.createRange();r.setStart(n,0);r.setEnd(n,16);const s=getSelection();s.removeAllRanges();s.addRange(r);document.dispatchEvent(new Event('selectionchange'));});
    await page.getByRole('button',{name:/Ask Puri/}).click();
   }
   await selectPhrase();
   await page.getByRole('button',{name:'Enable on-device AI',exact:true}).waitFor();
   assert(await page.getByRole('button',{name:'Explain phrase',exact:true}).isDisabled());
   if(!real){
    workerBehavior='stall';
    await page.getByRole('button',{name:'Enable on-device AI',exact:true}).click();
    await page.getByRole('button',{name:'Cancel download / warm-up',exact:true}).click();
    await page.getByRole('button',{name:'Enable on-device AI',exact:true}).waitFor();
    assert.equal(reservations.length,0,'cancelled load consumed quota');
    workerBehavior='normal';
   }
   const warm=Date.now();
   await page.getByRole('button',{name:'Enable on-device AI',exact:true}).click();
   await page.getByRole('button',{name:'Explain phrase',exact:true}).waitFor();
   try {await page.waitForFunction(()=>!document.querySelector('.submit').disabled,null,{timeout:real?600000:30000});}
   catch(e){console.log('LOAD UI:',await page.locator('dialog[open]').textContent());throw e;}
   const coldMs=Date.now()-warm;
   assert.equal(reservations.length,0,'warm-up consumed quota');
   const start=Date.now();
   await page.getByRole('button',{name:'Explain phrase',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('.answer').textContent.trim().length>0,null,{timeout:real?180000:30000});
   const firstMs=Date.now()-start;
   await page.waitForFunction(()=>!document.querySelector('.submit').disabled,null,{timeout:180000});
   console.log(JSON.stringify({width,real,coldWarmMs:coldMs,firstTextMs:firstMs,answer:await page.locator('.answer').textContent(),error:await page.locator('.error').allTextContents()}));
   assert.deepEqual(reservations,[{card_id:'card-0'}]);
   assert.equal(cloud.length,0,'free local generation contacted cloud');
   if(real){clearInterval(monitor);assert.equal(await page.locator('.error').count(),0);await context.close();continue;}
   await page.getByLabel('Help me',{exact:true}).selectOption('card_translate');
   await page.getByRole('button',{name:'Translate card',exact:true}).click();
   await page.waitForSelector('.ai-badge');
   assert.equal(await page.locator('.ai-badge').textContent(),'On-device AI translation');
   assert((await page.locator('[data-body]').textContent()).includes('Bonjour'));
   assert(await page.getByRole('button',{name:'Highlight the selected text',exact:true}).isDisabled());
   await page.getByLabel('Card language',{exact:true}).selectOption('Original');
   assert.equal(await page.locator('.ai-badge').count(),0);
   const before=reservations.length;
   await selectPhrase();
   assert.equal(await page.getByRole('button',{name:'Enable on-device AI',exact:true}).count(),0,'warm model lost on close/reopen');
   await page.getByLabel('Help me',{exact:true}).selectOption('explain');
   await page.waitForFunction(()=>!document.querySelector('.submit').disabled);
   limit=used;
   await page.getByRole('button',{name:'Explain phrase',exact:true}).click();
   await page.getByRole('alert').waitFor();
   assert.equal(reservations.length,before+1);assert.equal(cloud.length,0);
   assert.deepEqual(errors,[]);
   await context.close();
  }
  if(!real)for(const software of [false,true]){
   const page=await browser.newPage({serviceWorkers:'block'});
   await page.addInitScript(flag=>Object.defineProperty(navigator,'gpu',{value:flag?{requestAdapter:async()=>({features:new Set(),info:{description:'SwiftShader software GPU'}})}:undefined}),software);
   await page.route('**/v1/**',r=>r.fulfill({json:r.request().url().endsWith('/me')?{user:{tier:'free'}}:r.request().url().endsWith('/local')?{used:0,limit:10,resets_at:'2027-01-01T00:00:00Z'}:r.request().url().includes('/cards?')?{page_count:1,pages:[{idx:0,kind:'text',id:'source',card_id:'card',content:'A short reading passage.',char_count:24}]}:{book:{title:'Test',author:'Tester'}}}));
   await page.goto(base+'/read/');await page.getByRole('button',{name:/Ask Puri/}).click();await page.getByText(/cannot run this model/).waitFor();
   assert.equal(await page.getByRole('button',{name:'Enable on-device AI',exact:true}).count(),0);
   await page.close();
  }
  if(!real){
   const page=await browser.newPage({serviceWorkers:'block'});const cloud=[];
   await page.route('**/v1/**',async route=>{
    const req=route.request(),u=new URL(req.url());let body={};
    if(u.pathname==='/v1/me')body={user:{tier:'plus'}};
    else if(u.pathname==='/v1/assistant/local')body={used:0,limit:100,resets_at:'2027-01-01T00:00:00Z'};
    else if(u.pathname==='/v1/assistant'){cloud.push(req.postDataJSON());body={answer:'A trusted cloud explanation.',model:'cloud'}}
    else if(u.pathname.endsWith('/cards'))body={page_count:1,pages:[{idx:0,kind:'text',id:'source',card_id:'card',content:'A curious phrase in a reading passage.',char_count:37}]};
    else if(u.pathname.endsWith('/position'))body={position:null};
    else if(u.pathname.endsWith('/translations'))body={translations:[]};
    else if(u.pathname.startsWith('/v1/books/'))body={book:{title:'Test',author:'Tester'}};
    await route.fulfill({json:body});
   });
   await page.goto(base+'/read/');await page.waitForSelector('[data-body]');
   await page.evaluate(()=>{const n=document.querySelector('[data-body]').firstChild,r=document.createRange();r.setStart(n,2);r.setEnd(n,9);const s=getSelection();s.removeAllRanges();s.addRange(r);document.dispatchEvent(new Event('selectionchange'));});
   await page.getByRole('button',{name:/Ask Puri/}).click();
   await page.getByLabel('Run with',{exact:true}).waitFor();
   assert.equal(await page.getByLabel('Run with',{exact:true}).inputValue(),'cloud');
   await page.getByRole('button',{name:'Explain phrase',exact:true}).click();
   await page.getByText('A trusted cloud explanation.',{exact:true}).waitFor();
   assert.equal(cloud.length,1);assert.equal(cloud[0].card_id,'card');
   await page.close();
  }
  console.log(real?'Real Qwen WebGPU smoke passed':'Local AI: opt-in, quota-only metadata, streaming, warm reuse, card translation, exhaustion and unsupported-device paths passed');
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exit(1)});
