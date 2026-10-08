const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const handlers = {};
const stale = { body: 'old blue cards' };
const fresh = { body: 'new text UI', ok: true, clone() { return this; } };
let offline = false;
vm.runInNewContext(fs.readFileSync('public/sw.js', 'utf8'), {
  self: { location: { origin: 'https://potpuri.cc' }, addEventListener: (name, fn) => handlers[name] = fn },
  URL,
  caches: { match: async () => stale, open: async () => ({ put: async () => {} }) },
  fetch: async () => { if (offline) throw new Error('offline'); return fresh; },
});
(async () => {
  for (const path of ['/', '/app', '/app/me/']) {
    let result;
    handlers.fetch({ request: { method: 'GET', mode: 'navigate', url: `https://potpuri.cc${path}` }, respondWith: p => result = p });
    assert.equal(await result, fresh, `${path} must not serve stale UI online`);
  }
  offline = true;
  let result;
  handlers.fetch({ request: { method: 'GET', mode: 'navigate', url: 'https://potpuri.cc/app' }, respondWith: p => result = p });
  assert.equal(await result, stale, 'offline shell remains available');
  for (const path of ['/v1/books?mine=true', '/v1/books/example/position']) {
    let intercepted = false;
    handlers.fetch({ request: { method: 'GET', url: `https://potpuri.cc${path}` }, respondWith: () => intercepted = true });
    assert.equal(intercepted, false, 'personal library metadata must bypass caches');
  }
  console.log('Navigation freshness, offline fallback and private library requests passed');
})();
