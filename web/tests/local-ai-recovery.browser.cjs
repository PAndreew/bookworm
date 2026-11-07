// Regression at the real reader/component/cache seam: remembered opt-in + cached
// weights must not allocate a GPU worker merely by opening/reloading card 26.
const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const base=process.env.TEST_BASE_URL||'http://127.0.0.1:4338';
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({serviceWorkers:'block'}),page=await context.newPage();
  const workers=[],downloads=[],errors=[];
  await page.addInitScript(()=>Object.defineProperty(navigator,'gpu',{value:{requestAdapter:async()=>({features:new Set(['shader-f16'])})}}));
  page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/local-ai.worker-*.js',route=>{workers.push(route.request().url());return route.fulfill({contentType:'text/javascript',body:'self.onmessage=()=>{};'})});
  await page.route(/https:\/\/(huggingface.co|raw.githubusercontent.com)\//,route=>{downloads.push(route.request().url());return route.abort()});
  await page.route('**/local-ai-policy.json',route=>route.fulfill({json:{enabled:process.env.POLICY_ENABLED==='1'}}));
  await page.route('**/v1/**',route=>{
   const u=new URL(route.request().url());let body={};
   if(u.pathname==='/v1/me')body={user:{tier:'free'}};
   else if(u.pathname==='/v1/assistant/local')body={used:2,limit:10,resets_at:'2027-01-01T00:00:00Z'};
   else if(u.pathname.endsWith('/position'))body={position:{page_idx:25,card_idx:25}};
   else if(u.pathname.endsWith('/cards'))body={page_count:26,pages:[{idx:25,kind:'text',id:'source-26',card_id:'card-26',content:'The reader must remain available even when on-device AI is not safe to run.',char_count:74}]};
   else if(u.pathname.startsWith('/v1/books/'))body={book:{title:'Card 26 recovery fixture',author:'Tester'}};
   return route.fulfill({json:body});
  });
  await page.goto(base+'/',{waitUntil:'domcontentloaded'});
  await page.evaluate(async()=>{
   localStorage.setItem('bw:local-ai-enabled','yes');
   const cache=await caches.open('webllm/model');
   await cache.put('https://huggingface.co/mlc-ai/Qwen3.5-0.8B-q4f16_1-MLC/resolve/0ec138972555613c1d7812a821778ad0398c8790/tensor-cache.json',new Response(JSON.stringify({records:[]})));
  });
  await page.route('**/read/recovery-test/**',async route=>route.fulfill({response:await page.request.get(base+'/read/')}));
  await page.goto(base+'/read/recovery-test/?card=25',{waitUntil:'networkidle'});
  await page.locator('[data-idx="25"] [data-body]').waitFor();
  await page.waitForTimeout(2500);
  await page.reload({waitUntil:'networkidle'});
  await page.locator('[data-idx="25"] [data-body]').waitFor();
  await page.waitForTimeout(2500);
  console.log({workersStartedWithoutUserAction:workers.length,modelDownloads:downloads.length,errors});
  assert.equal(workers.length,0,'Opening/reloading card 26 started GPU warm-up from remembered opt-in, creating a potential crash/reload loop');
  assert.deepEqual(downloads,[]);assert.deepEqual(errors,[]);
  await page.getByRole('button',{name:/Ask Puri/}).click();
  if(process.env.POLICY_ENABLED!=='1'){
   await page.getByText(/On-device AI is temporarily paused/).waitFor();
   assert.equal(await page.getByRole('button',{name:'Enable on-device AI',exact:true}).count(),0);
  }
  console.log('Cached-model crash-loop recovery: reading and reload allocate no worker; paused mode cannot start a model');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
