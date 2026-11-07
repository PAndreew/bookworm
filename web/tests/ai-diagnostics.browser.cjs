// Product tests only: synthetic worker and tiny model-cache fixtures, no real GPU
// or large weight downloads. Physical iOS results must come from the diagnostic.
const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const base=process.env.TEST_BASE_URL||'http://127.0.0.1:4338';
const key='bw:ai-diagnostics:v1';
const model='https://huggingface.co/mlc-ai/Qwen3.5-0.8B-q4f16_1-MLC/resolve/0ec138972555613c1d7812a821778ad0398c8790/';
const normalWorker=`let context=512;self.onmessage=({data:c})=>{
 if(c.kind==='load'){
  context=c.config.context;
  postMessage({id:c.id,kind:'phase',phase:'gpu-device-created',detail:'Mock GPU device'});
  postMessage({id:c.id,kind:'memory',memory:{liveBytes:64*1024*1024,peakBytes:80*1024*1024,largestBufferBytes:32*1024*1024,budgetBytes:c.config.budgetMiB*1024*1024}});
  postMessage({id:c.id,kind:'complete',operation:'load',loadMs:12,compiledPrefill:1024,actualContext:context,memory:{peakBytes:80*1024*1024,largestBufferBytes:32*1024*1024}});
 }else{
  postMessage({id:c.id,kind:'phase',phase:'prompt-prefill',detail:c.test+' synthetic test'});
  const output='A peaceful refusal to obey an unjust rule. <img src="https://never-load.invalid/">';
  postMessage({id:c.id,kind:'output',text:output,firstTextMs:8});
  postMessage({id:c.id,kind:'complete',operation:'run',test:c.test,task:c.task,language:c.language,firstTextMs:8,totalMs:18,finishReason:'stop',output});
 }
};`;
async function waitWorkers(page,active){for(let i=0;i<40&&active.size;i++)await page.waitForTimeout(25);assert.equal(active.size,0,'diagnostic left a worker resident');}
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try{
  for(const width of [390,1280]){
   const context=await browser.newContext({viewport:{width,height:844},serviceWorkers:'block'}),page=await context.newPage();
   const workers=[],files=[],api=[],errors=[],active=new Set();let mode='normal';
   page.on('pageerror',e=>errors.push(e.message));
   page.on('worker',w=>{active.add(w);w.on('close',()=>active.delete(w))});
   await page.addInitScript(()=>{
    Object.defineProperty(navigator,'standalone',{value:true});
    Object.defineProperty(navigator,'gpu',{value:{requestAdapter:async()=>{if(window.__diagDelayAdapter)await new Promise(resolve=>setTimeout(resolve,250));return {features:new Set(['shader-f16']),info:{vendor:'mock',architecture:'apple-test'},limits:{maxBufferSize:1024**3,maxStorageBufferBindingSize:128*1024**2}}}}});
   });
   await page.route('**/v1/**',route=>{api.push(route.request().url());return route.fulfill({status:500,json:{error:'unexpected API'}})});
   await page.route(model+'**',route=>{
    const u=route.request().url();files.push(u);
    return route.fulfill(u.endsWith('tensor-cache.json')?{json:{records:[{dataPath:'params_shard_0.bin',nbytes:4},{dataPath:'params_shard_1.bin',nbytes:4}]}}:{contentType:'application/octet-stream',body:Buffer.from([1,2,3,4])});
   });
   await page.route('**/ai-diagnostics.worker-*.js',route=>{
    workers.push(route.request().url());
    const body=mode==='fail'?`self.onmessage=({data:c})=>{postMessage({id:c.id,kind:'phase',phase:'gpu-device-created',detail:'Mock load'});postMessage({id:c.id,kind:'budget-rejected',allocation:{reason:'budget-exceeded',requestedBytes:33554432,liveBytes:784869392,wouldBeBytes:818423824,budgetBytes:805306368,label:'prefill-scratch',usage:8}});postMessage({id:c.id,kind:'failure',error:'Diagnostic GPU buffer budget exceeded (test).',errorSource:'runtime',errorDetails:{name:'RuntimeError',message:'GPU callback failed',cause:{message:'budget exceeded',code:'DIAGNOSTIC_GPU_BUFFER_BUDGET'}}});};`:mode==='stall'?'self.onmessage=()=>{};':normalWorker;
    return route.fulfill({contentType:'text/javascript',body});
   });
   await page.goto(base+'/ai-diagnostics/',{waitUntil:'networkidle'});
   await page.getByRole('button',{name:'Inspect device',exact:true}).waitFor();
   assert.equal(workers.length,0);assert.equal(files.length,0);assert.equal(api.length,0);
   assert(await page.getByRole('button',{name:'Load GPU model',exact:true}).isDisabled());
   assert(await page.getByRole('button',{name:'Download/check weights',exact:true}).isDisabled());
   assert.equal(await page.locator('meta[name="robots"]').getAttribute('content'),'noindex, follow');
   await page.evaluate(()=>localStorage.setItem('bw:reading-marker','keep'));
   await page.getByLabel('Device / OS label (optional)',{exact:true}).fill('iPhone 16 · iOS 26 · test PWA');
   await page.getByRole('button',{name:'Inspect device',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('[data-diag-phase]').textContent==='device-inspected');
   assert.equal(files.length,0,'inspect fetched model weights');assert.equal(workers.length,0);
   await page.getByLabel('GPU buffer budget',{exact:true}).selectOption('2048');
   await page.getByRole('checkbox').check();
   await page.getByRole('button',{name:'Download/check weights',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('[data-diag-phase]').textContent==='weights-cached');
   assert.equal(files.length,3);assert.equal(workers.length,0,'download allocated model worker');
   await page.getByRole('button',{name:'Load GPU model',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('[data-diag-phase]').textContent==='model-ready');
   assert.equal(workers.length,1);
   const loaded=await page.evaluate(k=>JSON.parse(localStorage.getItem(k)),key);
   assert.equal(loaded.config.budgetMiB,2048);
   assert.equal(loaded.resident,true);assert.equal(loaded.active,false);assert.equal(loaded.measurements.runs,undefined,'load secretly ran a prompt');
   assert.equal(loaded.measurements.load.actualContext,512);assert.equal(loaded.measurements.load.compiledPrefill,1024);
   await page.getByRole('button',{name:'Run selected test',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('[data-diag-phase]').textContent==='test-completed');
   assert((await page.getByLabel('Diagnostic model output',{exact:true}).textContent()).includes('<img'));
   assert.equal(await page.locator('main img').count(),0,'model output interpreted as HTML');
   await page.getByLabel('Input size',{exact:true}).selectOption('card');
   await page.getByLabel('Diagnostic task',{exact:true}).selectOption('translate');
   await page.getByLabel('Output language',{exact:true}).selectOption('Romanian');
   await page.getByRole('button',{name:'Run selected test',exact:true}).click();
   await page.waitForFunction(k=>JSON.parse(localStorage.getItem(k)).measurements.runs?.length===2,key);
   assert.equal(workers.length,1,'each prompt unnecessarily recreated the model');
   const downloadPromise=page.waitForEvent('download');await page.getByRole('button',{name:'Download report',exact:true}).click();
   const downloaded=await downloadPromise;assert(downloaded.suggestedFilename().endsWith('.json'));
   const stream=await downloaded.createReadStream();let raw='';for await(const chunk of stream)raw+=chunk.toString();
   const exported=JSON.parse(raw);assert.equal(exported.environment.standalone,true);assert.equal(exported.measurements.runs[1].language,'Romanian');
   await page.getByRole('button',{name:'Stop and release model',exact:true}).click();await waitWorkers(page,active);
   let report=await page.evaluate(k=>JSON.parse(localStorage.getItem(k)),key);assert.equal(report.active,false);assert.equal(report.resident,false);
   await page.reload({waitUntil:'networkidle'});assert.equal(workers.length,1,'reload started a worker');assert.equal(files.length,3);
   await page.getByRole('button',{name:'Inspect device',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('[data-diag-phase]').textContent==='device-inspected');
   assert.equal(files.length,3,'cached weights were needlessly redownloaded');
   await page.getByRole('checkbox').check();mode='fail';
   await page.getByRole('button',{name:'Load GPU model',exact:true}).click();
   await page.getByRole('alert').filter({hasText:'budget exceeded'}).waitFor();await waitWorkers(page,active);
   report=await page.evaluate(k=>JSON.parse(localStorage.getItem(k)),key);assert.equal(report.phase,'failure');assert.equal(report.measurements.failedAt,'gpu-device-created');
   assert.equal(report.measurements.lastBudgetRejection.requestedBytes,33554432);assert.equal(report.measurements.lastBudgetRejection.rejectedDuring,'gpu-device-created');
   assert.equal(report.measurements.errorDetails.cause.code,'DIAGNOSTIC_GPU_BUFFER_BUDGET');assert.equal(report.measurements.errorSource,'runtime');
   await page.getByLabel('GPU allocation rejection',{exact:true}).waitFor();assert((await page.getByLabel('Structured diagnostic error',{exact:true}).textContent()).includes('GPU callback failed'));
   await page.reload({waitUntil:'networkidle'});assert.equal(workers.length,2,'failure automatically retried after reload');
   assert(await page.getByRole('button',{name:'Inspect device',exact:true}).isDisabled());
   // Saved failures can be retried directly: no export, manual reset, cache
   // redownload or separate GPU-load click is required.
   mode='normal';
   const oldId=report.id;
   await page.getByRole('button',{name:'Run / retry test',exact:true}).click();
   await page.waitForFunction(k=>JSON.parse(localStorage.getItem(k)).phase==='test-completed',key);
   report=await page.evaluate(k=>JSON.parse(localStorage.getItem(k)),key);
   assert.notEqual(report.id,oldId);assert.equal(report.measurements.runs.length,1);
   assert.equal(files.length,3,'retry downloaded already-cached weights');assert.equal(workers.length,3);
   await page.getByRole('button',{name:'Stop test',exact:true}).click();await waitWorkers(page,active);
   await page.getByRole('button',{name:'New report / reset test lock',exact:true}).click();
   assert.equal(await page.evaluate(()=>localStorage.getItem('bw:reading-marker')),'keep');
   // Simulate a process kill: there is no live worker to run pagehide cleanup.
   await page.evaluate(k=>{const r=JSON.parse(localStorage.getItem(k));r.phase='prompt-prefill';r.active=true;r.resident=true;localStorage.setItem(k,JSON.stringify(r))},key);
   await page.reload({waitUntil:'networkidle'});await page.getByText('A previous test failed or did not finish cleanly.',{exact:true}).waitFor();
   assert.equal(workers.length,3);assert.equal(files.length,3);assert.equal(api.length,0);
   report=await page.evaluate(k=>JSON.parse(localStorage.getItem(k)),key);assert.equal(report.phase,'prompt-prefill','reopening overwrote the crash marker');
   await page.getByRole('button',{name:'New report / reset test lock',exact:true}).click();
   await page.getByRole('button',{name:'Inspect device',exact:true}).click();await page.waitForFunction(()=>document.querySelector('[data-diag-phase]').textContent==='device-inspected');
   await page.getByRole('checkbox').check();mode='stall';
   await page.getByRole('button',{name:'Load GPU model',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('[data-diag-phase]').textContent==='worker-start');
   await page.waitForTimeout(100);
   await page.evaluate(()=>{Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'))});
   await waitWorkers(page,active);
   report=await page.evaluate(k=>JSON.parse(localStorage.getItem(k)),key);assert.equal(report.phase,'stopped');assert.equal(report.resident,false);
   const beforeCancelledRetry=workers.length;
   await page.evaluate(()=>{Object.defineProperty(document,'hidden',{configurable:true,value:false});window.__diagDelayAdapter=true});
   await page.getByRole('button',{name:'Run / retry test',exact:true}).click();
   await page.evaluate(()=>{Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'))});
   await page.waitForTimeout(400);
   assert.equal(workers.length,beforeCancelledRetry,'retry started a model after the page was hidden during inspection');
   assert.deepEqual(errors,[]);assert.deepEqual(api,[]);
   console.log(`${width}px: no automatic model/API activity, staged cached download/load, explicit prompts, report export, budget failure, crash lock and background termination passed`);
   await context.close();
  }
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
