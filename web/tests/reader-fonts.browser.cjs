const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_EXECUTABLE, args: ['--no-sandbox'] });
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 }, serviceWorkers: 'block' });
    const url = (process.env.TEST_BASE_URL || 'http://localhost:4333') + '/try/?book=gutenberg-46';
    await page.goto(url);
    await page.locator('.page').waitFor();
    await page.getByRole('button', { name: 'Next card', exact: true }).click();
    await page.locator('.page[data-kind="text"]').waitFor();
    const card = await page.locator('.page').getAttribute('data-card-id');
    await page.getByRole('button', { name: 'Text appearance' }).click();
    // Simulate a notched device's safe-area top. The menu must remain below T.
    await page.locator('.reader').evaluate(el => el.style.setProperty('--reader-safe-top', '44px'));
    const controls = await page.locator('.bar').boundingBox();
    const panel = await page.locator('.type-panel').boundingBox();
    assert(controls.y >= 47, 'toolbar must respect the safe area');
    assert(panel.y >= controls.y + controls.height + 4, 'menu must not cover T');
    assert.equal(await page.getByRole('button', { name: 'Text appearance' }).evaluate(el => {
      const rect = el.getBoundingClientRect();
      return el.contains(document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2));
    }), true, 'T remains clickable with the menu open');
    for (const [id, family] of [['fondamento', 'Fondamento'], ['space-mono', 'Space Mono'], ['unifrakturcook', 'UnifrakturCook'], ['fraunces', 'Fraunces'], ['geist', 'Geist']]) {
      await page.locator('#reader-font').selectOption(id);
      await page.evaluate(() => document.fonts.ready);
      await page.waitForTimeout(150);
      assert((await page.locator('.body').evaluate(el => getComputedStyle(el).fontFamily)).includes(family));
      assert.equal(await page.locator('.page').getAttribute('data-card-id'), card);
      const fits = await page.locator('.body').evaluate(el => el.getBoundingClientRect().bottom <= el.closest('.page').getBoundingClientRect().bottom - 65);
      assert(fits, `${family} must fit above navigation`);
    }
    await page.locator('#reader-font').selectOption('space-mono');
    await page.getByRole('button', { name: '145%', exact: true }).click();
    await page.reload();
    await page.getByRole('button', { name: 'Text appearance' }).click();
    assert.equal(await page.locator('#reader-font').inputValue(), 'space-mono');
    assert.equal(await page.getByRole('button', { name: '145%', exact: true }).getAttribute('aria-pressed'), 'true');
    await page.evaluate(() => localStorage.setItem('bw:readerFont', 'invalid-font'));
    await page.reload();
    await page.getByRole('button', { name: 'Text appearance' }).click();
    assert.equal(await page.locator('#reader-font').inputValue(), 'geist');
    console.log('Reader font selection, fit, card identity, persistence and invalid-value fallback passed');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
