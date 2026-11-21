const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
 const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_EXECUTABLE, args: ['--no-sandbox'] });
 try {
  const page = await browser.newPage({ viewport: { width: 390, height: 900 }, serviceWorkers: 'block' });
  const base = process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
  const queries = [], saves = [], windows = [];
  const books = [
   {id:'one',slug:'recent',title:'Latest Book',author:'Tester',page_count:90,card_count:100,genres:['fiction'],license:'public_domain',last_read_at:'2026-10-08T00:00:00Z',last_page_idx:2,last_card_idx:5,is_latest:true,read_percent:42,rating_average:4.5,rating_count:2},
   {id:'two',slug:'new',title:'New Book',author:'Tester',page_count:20,card_count:25,genres:['mystery'],license:'public_domain',last_read_at:null,last_page_idx:null,last_card_idx:null,is_latest:false,rating_average:5,rating_count:1},
   {id:'three',slug:'finished',title:'Finished Book',author:'Tester',page_count:50,card_count:60,genres:['poetry'],license:'public_domain',last_read_at:'2026-10-07T00:00:00Z',read_percent:100},
  ];
  await page.route('**/v1/**', async route => {
   const url = new URL(route.request().url()), path = url.pathname;
   let body = {};
   if (path === '/v1/me') body = {user:{tier:'pro'},last_read:null};
   else if (path === '/v1/genres') body = {genres:['fiction','mystery','poetry']};
   else if (path === '/v1/streak') body = {current:2,pages_today:1,pages_required:3};
   else if (path === '/v1/me/ratings') body = {ratings:{}};
   else if (path === '/v1/books') {
    queries.push(url.searchParams);
    let result = url.searchParams.get('mine') === 'true' ? [books[0]] : [...books];
    const genres = url.searchParams.getAll('genre');
    if (genres.length) result = result.filter(b => b.genres.some(g => genres.includes(g)));
    const search = url.searchParams.get('q');
    if (search) result = result.filter(b => `${b.title} ${b.author}`.toLowerCase().includes(search.toLowerCase()));
    const sort = url.searchParams.get('sort'), ascending = sort === 'shortest';
    const value = b => sort === 'recent' ? Date.parse(b.last_read_at) || 0 : b.page_count;
    result.sort((a,b) => (value(a)-value(b)) * (ascending ? 1 : -1));
    body = {books:result,next_offset:null};
   } else if (path.endsWith('/position')) {
    if (route.request().method() === 'POST') { saves.push(route.request().postDataJSON().card_id); body = {status:'saved'}; }
    else body = {position:{page_idx:2,card_idx:5}};
   } else if (path === '/v1/books/recent') body = {book:books[0]};
   else if (path.endsWith('/cards')) {
    const start = Number(url.searchParams.get('card_from')); windows.push(start);
    body = {page_count:9,pages:Array.from({length:9-start},(_,i)=>({idx:start+i,card_id:`card-${start+i}`,id:`page-${start+i}`,kind:'text',source_idx:2,start_off:0,content:`Saved card ${start+i}. This is a short reading test.`,parts:[],char_count:60,like_count:0,liked_by_me:false}))};
   } else if (path === '/v1/card-reads') body = {streak_days:2,streak_extended:false};
   await route.fulfill({json:body});
  });
  await page.goto(base+'/app');
  const notice = page.locator('.nudge');
  await notice.getByText('2 cards left to keep your streak').waitFor();
  assert.equal(await notice.locator('.spinner svg').count(),1);
  await notice.getByRole('button',{name:'Cancel',exact:true}).click();
  await notice.waitFor({state:'hidden'});
  // Record the DOM transitions in the page; reload can resolve after the notice appears.
  await page.addInitScript(() => {
    new MutationObserver(() => {
      const isShown = !!document.querySelector('.nudge');
      if (isShown && !performance.getEntriesByName('streak-shown').length) performance.mark('streak-shown');
      if (!isShown && performance.getEntriesByName('streak-shown').length && !performance.getEntriesByName('streak-hidden').length) performance.mark('streak-hidden');
    }).observe(document, {childList:true, subtree:true});
  });
  await page.reload();
  await notice.waitFor({state:'visible'});
  await page.waitForSelector('.card.latest');
  const footerIcon = await page.getByRole('link',{name:'Library',exact:true}).locator('path').getAttribute('d');
  assert.equal(await page.locator('.resume').count(),0,'library must not contain the old resume button');
  const controlStyle = await page.getByRole('button',{name:'Filter genres',exact:true}).evaluate(el => {
    const style = getComputedStyle(el);
    return {color:style.color, background:style.backgroundColor, border:style.borderTopColor};
  });
  assert.equal(controlStyle.border, 'rgb(229, 231, 235)');
  assert.equal(controlStyle.color, 'rgb(0, 0, 0)');
  assert.equal(controlStyle.background, 'rgb(255, 255, 255)');
  assert.equal(await page.locator('.card').first().locator('.p').first().textContent(), '100 cards', 'book cards show card counts without page counts');
  assert(!/\bpages\b/.test(await page.locator('.card').first().textContent()), 'book cards must not show page counts');
  assert.equal(await page.locator('.card').first().locator('h2').textContent(),'Latest Book');
  assert.equal(await page.locator('.card.latest').first().evaluate(el=>getComputedStyle(el).borderTopColor),'rgb(22, 163, 74)');
  assert.equal(await page.locator('.book-link').first().getAttribute('href'),'/read/recent?card=5');
  const mine = page.getByRole('region', {name:'Opened books',exact:true});
  const all = page.locator('section[aria-labelledby="all-books-heading"]');
  assert.equal(await mine.locator('.card').count(),1);
  assert.equal(await all.locator('.card').count(),3);
  assert.equal(await mine.getByText('42% read',{exact:true}).count(),1);
  assert.equal(await all.getByText('0% read',{exact:true}).count(),1);
  assert.equal(await all.getByText('Already read',{exact:true}).locator('svg.lucide-check').count(),1);
  assert.deepEqual(await page.locator('.discovery').evaluate(el => [...el.children].map(child => child.tagName === 'BUTTON' ? 'filter' : child.querySelector('select') ? 'sort' : 'search')), ['sort','search','filter']);
  assert(queries.some(q => q.get('mine') === 'true'));
  assert.equal(await mine.evaluate(el => getComputedStyle(el).gridAutoFlow), 'column');
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'no page-level horizontal overflow');
  await page.getByRole('button',{name:'Filter genres',exact:true}).click();
  const genres = page.getByRole('listbox',{name:'Genres',exact:true});
  assert.equal(await genres.getAttribute('multiple'),'');
  await genres.selectOption('fiction');
  await page.waitForFunction(()=>document.querySelectorAll('.grid .card').length===1);
  await genres.selectOption(['fiction','mystery']);
  await page.waitForFunction(()=>document.querySelectorAll('.grid .card').length===2);
  assert.deepEqual(queries.at(-1).getAll('genre'),['fiction','mystery']);
  await page.getByRole('button',{name:'All genres',exact:true}).click();
  await page.getByRole('button',{name:'Done',exact:true}).click();
  await page.getByRole('searchbox').fill('New Book');
  await page.waitForFunction(()=>document.querySelectorAll('.grid .card').length===1 && !document.querySelector('.horizontal .card'));
  await page.getByRole('searchbox').fill('');
  await page.waitForFunction(()=>document.querySelectorAll('.grid .card').length===3 && document.querySelectorAll('.horizontal .card').length===1);
  const sorter = page.getByRole('combobox',{name:'Sort by',exact:true});
  assert.deepEqual(await sorter.locator('option').allTextContents(),['Longest first','Shortest first','Recently read']);
  for (const [sort, title] of [['shortest','New Book'], ['longest','Latest Book'], ['recent','Latest Book']]) {
    await Promise.all([
      page.waitForResponse(r => r.url().includes('/v1/books?') && new URL(r.url()).searchParams.get('sort') === sort),
      sorter.selectOption(sort),
    ]);
    await page.waitForFunction(expected => document.querySelector('.grid .card h2')?.textContent === expected, title);
  }
  await notice.waitFor({state:'hidden',timeout:4000});
  const duration = await page.evaluate(() => performance.measure('streak-duration', 'streak-shown', 'streak-hidden').duration);
  assert(duration >= 2750 && duration <= 4000, `expected a three-second streak notice, got ${duration}ms`);
  // Astro's preview lacks production Nginx's /read/:slug fallback.
  await page.route('**/read/recent', async route => {
    const response = await page.request.get(base + '/read/');
    await route.fulfill({ response });
  });
  await page.goto(base+'/read/recent');
  await page.waitForSelector('[data-body]');
  assert.equal(await page.getByRole('link',{name:'Back to library'}).locator('path').getAttribute('d'),footerIcon);
  assert.equal(windows[0],5,'direct book opening must load its saved card');
  await page.waitForTimeout(150);
  assert(saves.includes('card-5'),'opening a saved book must update its position');
  if (await page.getByRole('button', { name: 'Next card', exact: true }).count()) {
    await page.getByRole('button', { name: 'Next card', exact: true }).click();
  } else {
    await page.locator('[data-idx="6"]').scrollIntoViewIfNeeded();
  }
  for (let i = 0; i < 10 && saves.at(-1) !== 'card-6'; i++) await page.waitForTimeout(50);
  assert.equal(saves.at(-1),'card-6','navigation must save before dwell credit');
  await page.route('**/v1/me', route => route.fulfill({status:401,json:{error:'unauthorized'}}));
  await page.goto(base+'/app');
  await page.waitForSelector('.grid .card');
  assert.equal(await page.locator('.horizontal .card').count(),0);
  assert(await page.getByText('to see your opened books.',{exact:false}).count());
  console.log('Library shelves, multi-genre filtering, search, sorting, signed-out state and reader bookmarks passed');
 } finally { await browser.close(); }
})();
