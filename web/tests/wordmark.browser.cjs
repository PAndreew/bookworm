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
   const headings = await page.locator('h1, h2, h3').evaluateAll(els => els.map(el => ({family:getComputedStyle(el).fontFamily,weight:getComputedStyle(el).fontWeight})));
   for (const heading of headings) {
    assert(heading.family.startsWith('"Momo Trust Display"'), `${path}: heading font`);
    assert.equal(heading.weight,'400', `${path}: heading weight`);
   }
   if (path === '/app') {
    await page.locator('.signin').waitFor();
    const button = await page.locator('.signin').evaluate(el => {const s=getComputedStyle(el);return {color:s.color,border:s.borderTopColor,background:s.backgroundColor};});
    assert.equal(button.color,'rgb(21, 128, 61)');
    assert.equal(button.border,button.color);
    assert.equal(button.background,'rgba(0, 0, 0, 0)');
   }
  }
  console.log('Regular Momo Trust Display wordmarks and headings; green ghost library sign-in passed');
 } finally { await browser.close(); }
})().catch(e=>{console.error(e);process.exitCode=1;});
