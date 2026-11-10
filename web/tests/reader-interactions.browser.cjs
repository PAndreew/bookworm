const assert=require('node:assert/strict');
const {chromium,webkit}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base=process.env.TEST_BASE_URL || 'http://127.0.0.1:4333';
(async()=>{
 const engine=process.env.TEST_WEBKIT?webkit:chromium;
 const browser=await engine.launch(process.env.TEST_WEBKIT?{}:{executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 const failures=[];
 try{
 for(const [width,height] of [[320,568],[390,600],[390,420],[1280,720]]){
  const page=await browser.newPage({viewport:{width,height},serviceWorkers:'block'});const marks=[],errors=[];
  const text='A familiar sentence with an unfamiliar word. '.repeat(6);
  page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/v1/**',async route=>{
   const r=route.request(),u=new URL(r.url());let json={};
   if(u.pathname==='/v1/me')json={user:{id:'reader-qa',tier:'free',handle:'reader'},reading_activity:[]};
   else if(u.pathname==='/v1/byok/quota')json={used:0,limit:50};
   else if(u.pathname.endsWith('/position'))json={position:null};
   else if(u.pathname.endsWith('/cards'))json={page_count:2,pages:[0,1].map(idx=>({id:'source-'+idx,card_id:'card-'+idx,idx,kind:'text',start_off:50,content:text,char_count:text.length,like_count:0}))};
   else if(u.pathname==='/v1/highlights'){
    if(r.method()==='POST'){const h={id:'h'+marks.length,...r.postDataJSON()};marks.push(h);json={highlight:h};}
    else json={highlights:marks};
   }else if(u.pathname.startsWith('/v1/books/'))json={book:{title:'Test',author:'Tester'}};
   await route.fulfill({json});
  });
  await page.route('**/read/interaction-test/**',async route=>route.fulfill({response:await page.request.get(base+'/read/')}));
  await page.goto(base+'/read/interaction-test/');await page.waitForSelector('[data-body]');
  await page.evaluate(()=>document.documentElement.style.setProperty('--reader-safe-top','80px'));
  await page.getByRole('button',{name:/Ask Puri/}).click();await page.getByLabel('AI access',{exact:true}).waitFor();
  const close=page.getByRole('button',{name:'Close reading companion',exact:true});const box=await close.boundingBox();
  if(box.y<80 || box.y+box.height>height-12)failures.push(`${width}x${height}: close button outside safe viewport at y=${box.y}`);
  await page.mouse.click(width-2,height-2);await page.waitForTimeout(80);
  if(await page.locator('dialog[aria-labelledby="sunny-title"]').isVisible())failures.push(`${width}x${height}: outside tap did not dismiss companion`);
  await page.keyboard.press('Escape');
  // Simulate the native selection menu/focus clearing the range on action tap.
  await page.locator('[data-body]').first().evaluate(el=>{const t=document.createTreeWalker(el,NodeFilter.SHOW_TEXT).nextNode(),r=document.createRange();r.setStart(t,2);r.setEnd(t,20);const s=getSelection();s.removeAllRanges();s.addRange(r);document.dispatchEvent(new Event('selectionchange'));});
  await page.getByRole('button',{name:'Highlight the selected text',exact:true}).evaluate(el=>el.addEventListener('pointerdown',()=>{getSelection().removeAllRanges();document.dispatchEvent(new Event('selectionchange'));},{once:true}));
  await page.getByRole('button',{name:'Highlight the selected text',exact:true}).click();await page.waitForTimeout(150);
  if(marks.length!==1)failures.push(`${width}x${height}: selected text lost before highlight save`);
  else {assert.equal(marks[0].start_off,52);assert.equal(marks[0].end_off,70);}
  if(await page.locator('[data-body] mark').count()===0)failures.push(`${width}x${height}: saved highlight has no visible mark`);
  await page.locator('[data-body]').first().evaluate(el=>{const t=document.createTreeWalker(el,NodeFilter.SHOW_TEXT).nextNode(),r=document.createRange();r.setStart(t,0);r.setEnd(t,1);const s=getSelection();s.removeAllRanges();s.addRange(r);document.dispatchEvent(new Event('selectionchange'));});
  await page.getByRole('button',{name:'Highlight the selected text',exact:true}).click();await page.waitForTimeout(100);
  if(marks.length!==2)failures.push(`${width}x${height}: one-character selection could not be highlighted`);
  await page.reload();await page.waitForSelector('[data-body]');await page.waitForTimeout(150);
  if(marks.length && await page.locator('[data-body] mark').count()===0)failures.push(`${width}x${height}: saved highlight not restored after reload`);
  assert.deepEqual(errors,[]);await page.close();
 }
 assert.deepEqual(failures,[]);console.log('Safe-area close, outside-dismiss, selection retention and visible/restored highlights passed.');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1});
