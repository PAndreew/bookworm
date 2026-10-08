const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_EXECUTABLE, args: ['--no-sandbox'] });
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 900 }, serviceWorkers: 'block' });
    await page.route('**/v1/me', route => route.fulfill({ json: { user: { tier: 'pro' } } }));
    await page.goto(process.env.TEST_BASE_URL || 'http://localhost:4321');
    await page.waitForSelector('.puri-mascot');
    assert.deepEqual(await page.locator('.puri-mascot').evaluate(el => [el.offsetWidth, el.offsetHeight]), [32, 32]);
    assert.equal(await page.locator('.puri-mascot img').evaluateAll(images => images.every(img => img.complete && img.naturalWidth > 0)), true);
    for (const [time, expected] of [[0, [1, 0, 0]], [4725, [0, 1, 0]], [4800, [0, 0, 1]], [4875, [0, 1, 0]], [4950, [1, 0, 0]]]) {
      const opacity = await page.locator('.puri-mascot').evaluate((el, time) => {
        el.getAnimations({ subtree: true }).forEach(animation => { animation.pause(); animation.currentTime = time; });
        return [...el.querySelectorAll('img')].map(img => Number(getComputedStyle(img).opacity));
      }, time);
      assert.deepEqual(opacity, expected);
    }
    assert.equal(await page.locator('.guest-puri button').count(), 0, 'landing Puri must not open the AI dialog');
    await page.emulateMedia({ reducedMotion: 'reduce' });
    assert.deepEqual(await page.locator('.puri-mascot img').evaluateAll(images => images.map(img => getComputedStyle(img).animationName)), ['none', 'none', 'none']);
    console.log('Puri frames, size, non-clickable landing behavior, and reduced motion passed');
  } finally { await browser.close(); }
})();
