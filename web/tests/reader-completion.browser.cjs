const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
 const browser = await chromium.launch({ args: ['--no-sandbox'] });
 try {
 const context = await browser.newContext({ serviceWorkers: 'block' });
 const page = await context.newPage();
 let missing = [1, 3], positionRequests = 0;
 await page.route('**/v1/**', route => {
  const path = new URL(route.request().url()).pathname;
  let body = {};
  if (path.endsWith('/position')) {
   if (route.request().method() === 'GET') { positionRequests++; body = {position: null, completion: {total_cards: 3, read_cards: 3 - missing.length, missing_card_indices: missing}}; }
  } else if (path.endsWith('/cards')) body = {page_count: 4, pages: [0,1,2,3].map(idx => ({idx, id: 'page-'+idx, card_id: 'card-'+idx, kind: idx === 0 ? 'title' : 'text', content: idx === 0 ? 'Test book' : 'Text card '+idx+'.', like_count: 0, liked_by_me: false}))};
  else if (path === '/v1/me') body = {user: {tier:'pro'}};
  else if (path.startsWith('/v1/books/')) body = {book:{title:'Test book', author:'Test'}};
  return route.fulfill({json:body});
 });
 await page.goto((process.env.TEST_BASE_URL || 'http://localhost:4321') + '/read/the-yellow-wallpaper/');
 await page.getByRole('button', {name:'Reading completion', exact:true}).click();
 await page.getByText('1 of 3 text cards read').waitFor();
 assert.equal(await page.getByRole('button', {name:'Read card 2', exact:true}).count(), 1);
 assert.equal(await page.getByRole('button', {name:'Read card 4', exact:true}).count(), 1);
 assert.equal(await page.getByRole('button', {name:'Read card 1', exact:true}).count(), 0);
 await page.getByRole('button', {name:'Read card 4', exact:true}).click();
 await page.locator('[data-idx="3"]').waitFor();
 // Simulate server coverage updated by another device. Reopening must refetch.
 missing = [];
 const before = positionRequests;
 await page.getByRole('button', {name:'Reading completion', exact:true}).click();
 await page.getByText('Every text card is read. Your whole-book achievement is unlocked.').waitFor();
 assert(positionRequests > before);
 await context.close();
 console.log('Missing cards, exact-card jumps and account-wide refresh passed');
 } finally { await browser.close(); }
})().catch(err => {console.error(err); process.exit(1)});
