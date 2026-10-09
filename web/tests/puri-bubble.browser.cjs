const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE,args:['--no-sandbox']});
 try{
  for(const reducedMotion of ['no-preference','reduce']){
   const page=await browser.newPage({viewport:{width:390,height:800},serviceWorkers:'block',reducedMotion});
   await page.route('**/v1/**',()=>{throw Error('Preview called backend')});
   await page.goto((process.env.TEST_BASE_URL || 'http://127.0.0.1:4333')+'/try/?book=gutenberg-46');
   await page.waitForSelector('.mascot');
   assert.equal(await page.locator('.coach').count(),0,'title card has no coaching');
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
   assert((await page.locator('.bubble').textContent()).includes('endeavoured'),'first non-title card teaches explanation');
   await turn(2); await page.waitForSelector('.coach');
   assert((await page.locator('.bubble').textContent()).includes('Translate'),'second non-title card teaches translation');
   for(let i=3;i<10;i++) { await turn(i); assert.equal(await page.locator('.coach').count(),0,'remaining cards have no hints'); }
   await turn(10);
   await page.getByRole('link',{name:'Create an account',exact:true}).waitFor();
   assert.equal(await page.locator('.coach').count(),0);
   await page.close();
  }
  console.log('Puri hints stay attached, skip the title and appear on only the first two non-title cards and end at signup');
 }finally{await browser.close()}
})();
