// Exercises the real diagnostic worker/core with a model-runtime stub. No GPU or
// weights are loaded. Mirrors the iPhone report's exact live-byte measurements.
const assert=require('node:assert/strict');
const vm=require('node:vm');
const esbuild=require('esbuild');
const path=require('node:path');
const stub=`
export const prebuiltAppConfig={model_list:[{model_id:'Qwen3.5-0.8B-q4f16_1-MLC'}]};
export class MLCEngine {
 constructor(){this.chat={completions:{create:async()=>{
  if(globalThis.nativeFailure)throw {name:'NativeGPUError',message:'GPU command failed',code:17,details:{stage:'prefill'}};
  this.device.createBuffer({size:322712464,label:'accepted-prefill-buffer'});
  try{this.device.createBuffer({size:33554432,label:'next-prefill-buffer',usage:8})}
  catch(cause){throw {name:'RuntimeError',message:'GPU callback failed',cause}}
 }}}}
 async reload(){const a=await navigator.gpu.requestAdapter();this.device=await a.requestDevice();this.device.createBuffer({size:462156928,label:'loaded-model'});this.loadedModelIdToPipeline=new Map([['Qwen3.5-0.8B-q4f16_1-MLC',{prefillChunkSize:1024,contextWindowSize:512}]]);}
 async resetChat(){}
}`;
(async()=>{
 const compiled=await esbuild.build({entryPoints:[path.resolve(__dirname,'../src/lib/ai-diagnostics.worker.ts')],bundle:true,write:false,format:'iife',platform:'browser',plugins:[{name:'stub-model-runtime',setup(build){build.onResolve({filter:/^@mlc-ai\/web-llm$/},()=>({path:'stub',namespace:'diagnostic-test'}));build.onLoad({filter:/.*/,namespace:'diagnostic-test'},()=>({contents:stub,loader:'js'}));}}]});
 async function run(nativeFailure=false){
  const messages=[];let allocations=0;
  class Adapter{constructor(){this.features=new Set(['shader-f16']);this.info={vendor:'apple',architecture:'apple'};this.limits={maxBufferSize:1073741824,maxStorageBufferBindingSize:1073741824}}async requestDevice(){return {createBuffer(){allocations++;return {destroy(){}}},lost:new Promise(()=>{}),addEventListener(){}}}}
  const context={nativeFailure,navigator:{gpu:{requestAdapter:async()=>new Adapter()}},performance,console,self:{postMessage:message=>messages.push(structuredClone(message))}};
  vm.runInNewContext(compiled.outputFiles[0].text,context);
  await context.self.onmessage({data:{id:'load',kind:'load',config:{context:512,budgetMiB:768,maxTokens:32}}});
  await context.self.onmessage({data:{id:'run',kind:'run',test:'short',task:'explain',language:'English'}});
  return {messages,allocations};
 }
 const budget=await run();
 const failure=budget.messages.findLast(message=>message.kind==='failure');
 console.log('Worker failure summary:',failure?.error);
 assert(failure?.errorDetails?.cause?.message?.includes('budget exceeded'),'runtime-wrapped Error details were discarded as [object Object]');
 const rejection=budget.messages.find(message=>message.kind==='budget-rejected');
 assert(rejection,'budget rejection was not sent independently of the wrapped runtime error');
 assert.equal(rejection.allocation.requestedBytes,33554432);
 assert.equal(rejection.allocation.liveBytes,784869392);
 assert.equal(rejection.allocation.wouldBeBytes,818423824);
 assert.equal(rejection.allocation.budgetBytes,805306368);
 assert.equal(rejection.allocation.label,'next-prefill-buffer');
 assert.equal(budget.allocations,2,'guard allowed rejected native allocation');
 const native=await run(true);
 assert(!native.messages.some(message=>message.kind==='budget-rejected'),'native errors were misclassified as diagnostic budget failures');
 const nativeError=native.messages.findLast(message=>message.kind==='failure');
 assert.equal(nativeError.errorDetails.code,17);assert.equal(nativeError.errorDetails.details.stage,'prefill');
 console.log('Real diagnostic worker: wrapped exception preservation, exact rejection bytes and native-error distinction passed');
})().catch(error=>{console.error(error);process.exit(1)});
