// Optional browser regression: install Playwright, then run against a served web build.
// All API calls are mocked: this test needs no credentials and spends no AI credits.
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.TEST_BASE_URL || 'http://localhost:4321';

(async () => {
  const browser = await chromium.launch({
    ...(process.env.PLAYWRIGHT_EXECUTABLE ? { executablePath: process.env.PLAYWRIGHT_EXECUTABLE } : {}),
    args: ['--no-sandbox'],
  });
  for (const width of [390, 1280]) {
    const context = await browser.newContext({ viewport: { width, height: 600 }, serviceWorkers: 'block' });
    const page = await context.newPage();
    const errors = [];
    const requests = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.route('**/v1/**', async route => {
      const request = route.request();
      const url = new URL(request.url());
      let body = {};
      if (url.pathname === '/v1/me') body = { user: { id: 'test', tier: 'free' } };
      else if (url.pathname.endsWith('/pages')) {
        const from = Number(url.searchParams.get('from') || 0);
        body = { page_count: 1536, pages: Array.from({ length: Math.min(12, 1536 - from) }, (_, i) => ({
          id: `page-${from+i}`, idx: from+i, content: 'Bonjour, mon ami. This is a deterministic book page for testing the reading companion.', chapter_title: null, like_count: 0, liked_by_me: false,
        })) };
      } else if (url.pathname === '/v1/assistant') {
        requests.push(request.postDataJSON());
        body = { answer: 'Bonjour means hello.', model: 'google/gemini-3.8-flash' };
      } else if (url.pathname === '/v1/reads') body = { streak_extended: false };
      else if (url.pathname.startsWith('/v1/books/')) body = { book: { title: 'Test Book', author: 'Test Author' } };
      await route.fulfill({ json: body });
    });
    await page.goto(base, { waitUntil: 'networkidle' });
    await page.waitForSelector('.mascot');
    assert.equal(await page.locator('.mascot ellipse').count(), 2);
    await page.setViewportSize({ width, height: 360 }); // force a scrollable landing
    const first = await page.locator('.mascot').boundingBox();
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    await page.waitForTimeout(650);
    const last = await page.locator('.mascot').boundingBox();
    assert(last.x > first.x + 20, 'landing mascot did not track scroll');
    await page.locator('.mascot').click();
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('dialog').isVisible(), false);

    await page.setViewportSize({ width, height: 600 });
    await page.goto(base + '/read/moby-dick/?from=100', { waitUntil: 'networkidle' });
    await page.waitForSelector('[data-body]');
    const before = await page.locator('.mascot').getAttribute('aria-label');
    const xBefore = (await page.locator('.mascot').boundingBox()).x;
    await page.evaluate(() => { const feed = document.querySelector('.feed'); feed.scrollTop = feed.clientHeight * 8; });
    await page.waitForTimeout(800);
    assert.notEqual(await page.locator('.mascot').getAttribute('aria-label'), before, 'exact page label did not advance');
    assert((await page.locator('.mascot').boundingBox()).x > xBefore, 'book progress position did not advance');
    const selected = await page.evaluate(() => {
      const feed = document.querySelector('.feed');
      const node = feed.querySelectorAll('[data-body]')[Math.round(feed.scrollTop / feed.clientHeight)].firstChild;
      const range = document.createRange(); range.setStart(node, 0); range.setEnd(node, 16);
      const selection = window.getSelection(); selection.removeAllRanges(); selection.addRange(range);
      document.dispatchEvent(new Event('selectionchange'));
      return selection.toString().trim();
    });
    await page.locator('.mascot').click();
    assert.equal(await page.getByLabel('Phrase', { exact: true }).inputValue(), selected);
    assert(await page.getByRole('button', { name: 'Explain phrase', exact: true }).isDisabled(), 'consent was bypassed');
    await page.getByRole('checkbox').check();
    await page.getByRole('button', { name: 'Explain phrase', exact: true }).click();
    await page.waitForFunction(() => document.querySelector('.answer').textContent === 'Bonjour means hello.');
    assert.deepEqual(requests[0], { mode: 'explain', phrase: selected, language: 'English', consent: true });
    await page.getByLabel('Help me', { exact: true }).selectOption('translate');
    await page.getByRole('button', { name: 'Translate phrase', exact: true }).click();
    await page.waitForTimeout(200);
    assert.equal(requests[1].mode, 'translate');
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('dialog').isVisible(), false);
    assert.deepEqual(errors, []);
    console.log(`${width}px: scroll progress, exact page label, selection, consent, explain/translate, Escape passed`);
    await context.close();
  }
  const reduced = await browser.newContext({ reducedMotion: 'reduce', viewport: { width: 390, height: 360 }, serviceWorkers: 'block' });
  const page = await reduced.newPage();
  await page.goto(base, { waitUntil: 'networkidle' });
  await page.evaluate(() => window.scrollTo(0, 9999));
  await page.waitForTimeout(150);
  assert.equal(await page.locator('.mascot svg').evaluate(el => el.getAnimations().length), 0);
  await browser.close();
  console.log('Reduced motion: no hopping');
})().catch(err => { console.error(err); process.exit(1); });
