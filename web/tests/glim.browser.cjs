const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
 const browser = await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try {
  const page=await browser.newPage({viewport:{width:390,height:900},serviceWorkers:'block'});
  await page.route('**/v1/me',route=>route.fulfill({status:401,json:{error:'Sign in'}}));
  await page.goto(process.env.TEST_BASE_URL || 'http://localhost:4333');
  await page.waitForSelector('.mascot');
  assert.deepEqual(await page.locator('.glim').evaluate(el=>[el.offsetWidth,el.offsetHeight]),[32,32]);
  assert(await page.locator('.glim img').evaluateAll(imgs=>imgs.every(img=>img.complete&&img.naturalWidth>0)));
  for(const [time,expected] of [[0,[1,0,0]],[4725,[0,1,0]],[4800,[0,0,1]],[4875,[0,1,0]],[4950,[1,0,0]]]) {
   const opacity=await page.locator('.glim').evaluate((el,time)=>{
    el.getAnimations({subtree:true}).forEach(a=>{a.pause();a.currentTime=time});
    return [...el.querySelectorAll('img')].map(img=>Number(getComputedStyle(img).opacity));
   },time);
   assert.deepEqual(opacity,expected);
  }
  assert.equal(await page.locator('button.mascot').count(),0);
  await page.waitForTimeout(500);
  await page.evaluate(()=>window.dispatchEvent(new Event('scroll')));
  await page.waitForTimeout(220);
  assert(await page.locator('.glim').evaluate(el=>el.getAnimations().some(a=>a.effect.getKeyframes().some(f=>String(f.transform).includes('translateY')))),'landing Puri must hop on scroll');
  await page.emulateMedia({reducedMotion:'reduce'});
  assert.deepEqual(await page.locator('.glim img').evaluateAll(imgs=>imgs.map(img=>getComputedStyle(img).animationName)),['none','none','none']);
  assert.equal(await page.locator('.glim').evaluate(el=>el.getAnimations().length),0);
  console.log('Shared Puri: blink, 32px size, landing hops, non-clickable facts and reduced motion passed');
 } finally {await browser.close()}
})();
