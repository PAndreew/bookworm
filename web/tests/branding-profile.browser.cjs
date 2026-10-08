const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_EXECUTABLE, args: ['--no-sandbox'] });
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 900 }, serviceWorkers: 'block' });
    const base = process.env.TEST_BASE_URL || 'http://localhost:4333';
    for (const path of ['/login', '/signup']) {
      await page.goto(base + path);
      const brand = page.locator('.brand-heading');
      assert.equal(await brand.locator('img').getAttribute('src'), '/glim.svg');
      const box = await brand.evaluate(el => {
        const img = el.querySelector('img').getBoundingClientRect();
        const text = el.querySelector('span').getBoundingClientRect();
        return (img.left + text.right) / 2;
      });
      assert(Math.abs(box - 195) < 2, 'auth branding must be centered');
    }
    let tier = 'free', totp = false;
    await page.route('**/v1/me', route => route.fulfill({ json: { user: { display_name: 'Reader', handle: 'reader', tier, visibility: 'friends', totp_enabled: totp } } }));
    for (const plan of ['free', 'plus', 'pro']) {
      tier = plan; totp = plan === 'pro';
      await page.goto(base + '/app/me');
      await page.waitForSelector('.plans');
      assert.equal(await page.locator('.plans .on').count(), plan === 'free' ? 0 : 1);
      const inactive = page.locator('.plans a:not(.on)').first();
      assert.equal(await inactive.evaluate(el => getComputedStyle(el).backgroundColor), 'rgb(255, 255, 255)');
      assert.equal(await inactive.evaluate(el => getComputedStyle(el).borderRadius), '999px');
      assert.equal(await page.locator('.stats').count(), 0);
      assert.equal(await page.locator('.box').evaluateAll(els => els.every(el => getComputedStyle(el).backgroundColor === 'rgba(0, 0, 0, 0)')), true);
      if (totp) assert.equal(await page.locator('.two-factor').evaluate(el => getComputedStyle(el).backgroundColor), 'rgb(232, 237, 243)');
      else assert.equal(await page.getByRole('button', { name: 'Set up', exact: true }).evaluate(el => getComputedStyle(el).backgroundColor), 'rgb(255, 255, 255)');
    }
    console.log('Logo, centered auth branding, plan states, and 2FA styles passed');
  } finally { await browser.close(); }
})();
