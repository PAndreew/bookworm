// Run with Node 22+: node --experimental-strip-types tests/local-ai-policy.cjs
const assert=require('node:assert/strict');
(async()=>{
 const ai=await import('../src/lib/local-ai.ts');
 global.window={location:{href:'https://example.test/read/book/'}};
 let gpuCalls=0,workerCalls=0,fetchCalls=0;
 Object.defineProperty(global,'navigator',{configurable:true,value:{gpu:{requestAdapter:async()=>{gpuCalls++;throw new Error('GPU must not be touched while paused')}}}});
 global.Worker=class {constructor(){workerCalls++;throw new Error('Worker must not start while paused')}};
 for(const policy of [{enabled:false},{enabled:'true'},{},null]){
  global.fetch=async()=>{fetchCalls++;return {ok:true,json:async()=>policy}};
  assert.equal(await ai.localPolicyEnabled(),false);
  await assert.rejects(ai.warmLocal(()=>{},new AbortController().signal),/temporarily paused/);
  await assert.rejects(ai.generateLocal({mode:'explain',phrase:'A phrase',language:'English'},()=>{},new AbortController().signal),/temporarily paused/);
 }
 global.fetch=async()=>{throw new Error('offline')};
 assert.equal(await ai.localPolicyEnabled(),false);
 global.fetch=async()=>({ok:false,json:async()=>({enabled:true})});
 assert.equal(await ai.localPolicyEnabled(),false);
 global.fetch=async()=>({ok:true,json:async()=>({enabled:true})});
 assert.equal(await ai.localPolicyEnabled(),true);
 window.location.href='https://example.test/read/book/?ai=off';
 assert.equal(await ai.localPolicyEnabled(),false);
 assert.equal(gpuCalls,0);assert.equal(workerCalls,0);
 console.log('Local AI policy: fail-closed execution guards, unavailable/malformed policy and safe-mode override passed');
})().catch(e=>{console.error(e);process.exit(1)});
