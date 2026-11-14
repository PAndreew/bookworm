const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
 const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_EXECUTABLE, args: ['--no-sandbox'] });
 try {
  const page = await browser.newPage({ viewport: { width: 390, height: 900 }, serviceWorkers: 'block' });
  const base = process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
  const queries = [], saves = [], windows = [];
  const books = [
   {id:'one',slug:'recent',title:'Latest Book',author:'Tester',page_count:90,card_count:100,genres:['fiction'],license:'public_domain',last_read_at:'2026-10-08T00:00:00Z',last_page_idx:2,last_card_idx:5,is_latest:true,rating_average:4.5,rating_count:2},
   {id:'two',slug:'new',title:'New Book',author:'Tester',page_count:20,card_count:25,genres:['mystery'],license:'public_domain',last_read_at:null,last_page_idx:null,last_card_idx:null,is_latest:false,rating_average:5,rating_count:1},
  ];
  await page.route('**/v1/**', async route => {
   const url = new URL(route.request().url()), path = url.pathname;
   let body = {};
   if (path === '/v1/me') body = {user:{tier:'pro'},last_read:null};
   else if (path === '/v1/genres') body = {genres:['fiction','mystery','poetry']};
   else if (path === '/v1/streak') body = {current:2,pages_today:3,pages_required:3};
   else if (path === '/v1/me/ratings') body = {ratings:{}};
   else if (path === '/v1/books') {
    queries.push(url.searchParams);
    let result = url.searchParams.get('mine') === 'true' ? [books[0]] : [...books];
    const genres = url.searchParams.getAll('genre');
    if (genres.length) result = result.filter(b => b.genres.some(g => genres.includes(g)));
    const search = url.searchParams.get('q');
    if (search) result = result.filter(b => `${b.title} ${b.author}`.toLowerCase().includes(search.toLowerCase()));
    const sort = url.searchParams.get('sort'), ascending = url.searchParams.get('direction') === 'asc';
    const value = b => sort === 'pages' ? b.page_count : sort === 'rating' ? b.rating_average : sort === 'reviews' ? b.rating_count : Date.parse(b.last_read_at) || 0;
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
  await page.waitForSelector('.card.latest');
  assert.equal(await page.locator('.resume').count(),0,'library must not contain the old resume button');
  const controlStyle = await page.getByRole('button',{name:'Filter genres',exact:true}).evaluate(el => {
    const style = getComputedStyle(el);
    return {color:style.color, background:style.backgroundColor, border:style.borderTopColor};
  });
  assert.equal(controlStyle.border, 'rgb(255, 255, 255)');
  assert.notEqual(controlStyle.color, controlStyle.background, 'white icons need a contrasting fill on the light page');
  assert.notEqual(controlStyle.background, 'rgba(0, 0, 0, 0)', 'controls must remain visible on the light page');
  assert.equal(await page.locator('.card').first().locator('.p').first().textContent(), '100 cards', 'book cards show card counts without page counts');
  assert(!/\bpages\b/.test(await page.locator('.card').first().textContent()), 'book cards must not show page counts');
  assert.equal(await page.locator('.card').first().locator('h2').textContent(),'Latest Book');
  assert.equal(await page.locator('.card.latest').first().evaluate(el=>getComputedStyle(el).borderTopColor),'rgb(22, 163, 74)');
  assert.equal(await page.locator('.book-link').first().getAttribute('href'),'/read/recent?card=5');
  const mine = page.getByRole('region', {name:'Opened books',exact:true});
  const all = page.locator('section[aria-labelledby="all-books-heading"]');
  assert.equal(await mine.locator('.card').count(),1);
  assert.equal(await all.locator('.card').count(),2);
  assert(queries.some(q => q.get('mine') === 'true'));
  assert.equal(await mine.evaluate(el => getComputedStyle(el).gridAutoFlow), 'column');
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'no page-level horizontal overflow');
  await page.getByRole('button',{name:'Filter genres',exact:true}).click();
  await page.getByLabel('fiction',{exact:true}).check();
  await page.waitForFunction(()=>document.querySelectorAll('.grid .card').length===1);
  await page.getByLabel('mystery',{exact:true}).check();
  await page.waitForFunction(()=>document.querySelectorAll('.grid .card').length===2);
  assert.deepEqual(queries.at(-1).getAll('genre'),['fiction','mystery']);
  await page.getByRole('button',{name:'All genres',exact:true}).click();
  await page.getByRole('button',{name:'Done',exact:true}).click();
  await page.getByRole('searchbox').fill('New Book');
  await page.waitForFunction(()=>document.querySelectorAll('.grid .card').length===1 && !document.querySelector('.horizontal .card'));
  await page.getByRole('searchbox').fill('');
  await page.waitForFunction(()=>document.querySelectorAll('.grid .card').length===2 && document.querySelectorAll('.horizontal .card').length===1);
  await page.getByRole('button',{name:'Sorting options',exact:true}).click();
  for (const sort of ['pages', 'rating', 'reviews', 'recent']) {
    await page.getByLabel('Sort by', {exact:true}).selectOption(sort);
    await page.waitForResponse(r => r.url().includes('/v1/books?') && new URL(r.url()).searchParams.get('sort') === sort);
    assert.equal(queries.at(-1).get('direction'), 'desc');
    await page.getByRole('button', {name:'Sort descending; switch to ascending',exact:true}).click();
    await page.waitForResponse(r => r.url().includes('/v1/books?') && new URL(r.url()).searchParams.get('direction') === 'asc');
    assert.equal(queries.at(-1).get('sort'), sort);
    await page.waitForFunction(title => document.querySelector('.grid .card h2')?.textContent === title, sort === 'rating' ? 'Latest Book' : 'New Book');
    await page.getByRole('button', {name:'Sort ascending; switch to descending',exact:true}).click();
    await page.waitForResponse(r => r.url().includes('/v1/books?') && new URL(r.url()).searchParams.get('direction') === 'desc');
    await page.waitForFunction(title => document.querySelector('.grid .card h2')?.textContent === title, sort === 'rating' ? 'New Book' : 'Latest Book');
  }
  // Astro's preview lacks production Nginx's /read/:slug fallback.
  await page.route('**/read/recent', async route => {
    const response = await page.request.get(base + '/read/');
    await route.fulfill({ response });
  });
  await page.goto(base+'/read/recent');
  await page.waitForSelector('[data-body]');
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
