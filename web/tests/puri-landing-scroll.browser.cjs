const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try{
  for(const reducedMotion of ['no-preference','reduce']){
   const page=await browser.newPage({viewport:{width:390,height:800},serviceWorkers:'block',reducedMotion});
   await page.route('**/v1/**',r=>r.fulfill({status:401,json:{error:'Sign in'}}));
   await page.goto(process.env.TEST_BASE_URL || 'http://127.0.0.1:4333');
   await page.waitForSelector('.mascot');await page.waitForTimeout(600);
   const initial=await page.locator('.mascot').evaluate(el=>[el.style.left,el.style.top]);
   for(let i=0;i<5;i++){
    await page.evaluate(()=>window.scrollBy(0,25));await page.waitForTimeout(60);
    assert.deepEqual(await page.locator('.mascot').evaluate(el=>[el.style.left,el.style.top]),initial,'Puri must not chase changing anchors during scrolling');
    assert.equal(await page.locator('.glim').evaluate(el=>el.getAnimations().length),0,'no hops during scrolling');
   }
   await page.waitForTimeout(150);
   const result=await page.evaluate(()=>{
    const p=document.querySelector('.mascot').getBoundingClientRect();
    const markers=[...document.querySelectorAll('main [data-sunny-anchor]')].map(el=>el.getBoundingClientRect()).filter(r=>r.top>=60&&r.bottom<=innerHeight-12);
    markers.sort((a,b)=>Math.abs(a.top-innerHeight*.68)-Math.abs(b.top-innerHeight*.68));
    return {errorX:Math.abs(p.left-markers[0].left-2),errorY:Math.abs(p.top-markers[0].top),transition:getComputedStyle(document.querySelector('.mascot')).transitionDuration,hops:document.querySelector('.glim').getAnimations().length};
   });
   assert(result.errorX<1&&result.errorY<1,JSON.stringify(result));
   assert(result.transition.split(',').every(t=>parseFloat(t)<=0.00001),'landing relocation must be immediate, not a CSS glide');
   assert.equal(result.hops,reducedMotion==='reduce'?0:1,'one hop after scroll settles');
   await page.close();
  }
  console.log('Landing Puri stays still during scrolling, then relocates precisely with one hop; reduced motion respected');
 }finally{await browser.close()}
})();
