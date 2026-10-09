const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const dist = path.resolve(__dirname, '../dist');
const read = file => fs.readFileSync(path.join(dist, file), 'utf8');
const locations = xml => [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map(match => match[1]);
const index = locations(read('sitemap-index.xml'));
assert(index.length > 0);
const urls = index.flatMap(url => {
  assert(url.startsWith('https://potpuri.cc/'));
  return locations(read(new URL(url).pathname.slice(1)));
});
assert.equal(new Set(urls).size, urls.length);
assert(urls.includes('https://potpuri.cc/'));
assert(urls.includes('https://potpuri.cc/read/gutenberg-46/'));
for (const url of urls) {
  const pathname = new URL(url).pathname;
  assert(pathname === '/' || pathname.startsWith('/legal/') || (pathname.startsWith('/read/') && pathname !== '/read/'));
  const html = read(path.join(pathname.slice(1), 'index.html'));
  assert(html.includes(`<link rel="canonical" href="${url}">`), `canonical mismatch: ${url}`);
  assert(html.includes('name="robots" content="index, follow"'));
  assert(html.includes(`property="og:url" content="${url}"`));
  assert(html.includes('name="twitter:card" content="summary"'));
  assert(html.includes('rel="describedby" href="/llms.txt"'));
}
for (const file of ['app/index.html', 'app/me/index.html', 'app/friends/index.html', 'app/u/index.html', 'login/index.html', 'signup/index.html', 'try/index.html', 'q/index.html', 'read/index.html']) {
  assert(read(file).includes('name="robots" content="noindex, follow"'), `${file} must not be indexed`);
}
const book = read('read/gutenberg-46/index.html');
assert(book.includes('A Christmas Carol by Charles Dickens — Read free on Potpuri'));
assert(book.includes('Read A Christmas Carol by Charles Dickens for free on Potpuri'));
const schemas = html => [...html.matchAll(/<script type="application\/ld\+json">([^<]+)<\/script>/g)].map(match => JSON.parse(match[1]));
assert(schemas(book).some(schema => schema['@type'] === 'Book' && schema.name === 'A Christmas Carol' && schema.author.name === 'Charles Dickens'));
assert(schemas(read('index.html')).some(schema => schema['@type'] === 'WebSite' && schema.url === 'https://potpuri.cc/'));
assert(read('robots.txt').includes('Sitemap: https://potpuri.cc/sitemap-index.xml'));
assert(read('llms.txt').startsWith('# Potpuri\n'));
assert(read('product-guide.md').startsWith('# Potpuri: product guide'));
console.log(`SEO checks passed: ${urls.length} canonical sitemap URLs, social metadata, structured data, and utility-page noindex`);
