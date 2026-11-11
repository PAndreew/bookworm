const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
 const browser = await chromium.launch({ args: ['--no-sandbox'] });
 try {
  const page = await browser.newPage({serviceWorkers:'block'});
  await page.route('**/v1/**', route => {
   const path = new URL(route.request().url()).pathname;
   return route.fulfill(path === '/v1/me' ? {status:401,json:{message:'Please sign in.'}} : {json:{books:[],genres:[],next_offset:null}});
  });
  for (const path of ['/', '/app', '/login', '/signup']) {
   await page.goto((process.env.TEST_BASE_URL || 'http://127.0.0.1:4333') + path);
   await page.locator('.brand-text').waitFor();
   const style = await page.locator('.brand-text').evaluate(async el => {
    await document.fonts.load('400 32px "Momo Trust Display"');
    const s = getComputedStyle(el);
    return {family:s.fontFamily,weight:s.fontWeight,style:s.fontStyle,synthesis:s.fontSynthesis,loaded:document.fonts.check('400 32px "Momo Trust Display"')};
   });
   assert(style.family.startsWith('"Momo Trust Display"'));
   assert.equal(style.weight,'400');assert.equal(style.style,'normal');assert.equal(style.synthesis,'none');assert(style.loaded);
  }
  console.log('Regular Momo Trust Display wordmark loaded on landing, library, login and signup');
 } finally { await browser.close(); }
})().catch(e=>{console.error(e);process.exitCode=1;});
