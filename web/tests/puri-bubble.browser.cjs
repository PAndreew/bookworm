const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try{
  for(const reducedMotion of ['no-preference','reduce']){
   const page=await browser.newPage({viewport:{width:390,height:800},serviceWorkers:'block',reducedMotion});
   await page.route('**/v1/**',()=>{throw Error('Preview called backend')});
   await page.goto((process.env.TEST_BASE_URL || 'http://127.0.0.1:4333')+'/try/?book=gutenberg-46');
   await page.waitForSelector('.mascot');await page.waitForSelector('.coach');
   await page.waitForTimeout(650);
   async function turn(index) {
    if(await page.getByRole('button',{name:'Next card',exact:true}).count())await page.getByRole('button',{name:'Next card',exact:true}).click();
    else { await page.locator(`[data-idx="${index}"]`).scrollIntoViewIfNeeded(); await page.waitForTimeout(50); }
   }
   await turn(1);
   if(reducedMotion==='no-preference')assert.equal(await page.locator('.coach').isVisible(),false,'bubble must hide during relocation/hop');
   await page.waitForSelector('.coach');
   const geometry=await page.evaluate(()=>{
    const p=document.querySelector('.mascot').getBoundingClientRect(),b=document.querySelector('.bubble').getBoundingClientRect();
    return {gap:Math.min(Math.abs(p.top-b.bottom),Math.abs(b.top-p.bottom)),puriX:p.left+16,left:b.left,right:b.right};
   });
   assert(geometry.gap<=20,`bubble detached from Puri: ${JSON.stringify(geometry)}`);
   assert(geometry.puriX>=geometry.left&&geometry.puriX<=geometry.right,'bubble tail must align horizontally with Puri');
   for(let i=2;i<=4;i++)await turn(i);
   await page.waitForSelector('.coach');
   assert((await page.locator('.bubble').textContent()).includes('simile'),'rapid navigation must not leave an earlier hint');
   await page.close();
  }
  console.log('Puri bubbles wait for movement, stay attached and show only the current card hint');
 }finally{await browser.close()}
})();
