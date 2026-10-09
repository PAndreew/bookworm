const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/home/pi/pi-web-local/node_modules/playwright');
(async () => {
 const browser = await chromium.launch({executablePath:'/usr/bin/chromium-browser',args:['--no-sandbox']});
 const page = await browser.newPage({viewport:{width:320,height:568}});
 await page.goto((process.env.TEST_BASE_URL || 'http://localhost:4322')+'/try/?book=gutenberg-46');
 await page.getByRole('button',{name:'Next card',exact:true}).click();
 await page.waitForTimeout(900);
 assert(!(await page.locator('.page-num').innerText()).includes('part'), 'Christmas Carol second card splits on SE');
 await page.getByRole('button',{name:'Close reading companion',exact:true}).filter({visible:true}).click();
 await page.getByRole('button',{name:'Click Puri',exact:true}).click();
 await page.waitForTimeout(2300);
 assert.equal(await page.locator('.coach').count(),0,'demo hint did not disappear');
 await browser.close();
})().catch(e=>{console.error(e);process.exit(1)});
