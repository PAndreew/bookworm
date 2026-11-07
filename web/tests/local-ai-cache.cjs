const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const handlers = {};
const deleted = [];
const names = ['potpuri-reader-v14-shell', 'potpuri-reader-v14-books', 'potpuri-reader-v17-shell', 'potpuri-reader-v17-books', 'webllm/model', 'webllm/wasm', 'another-app-cache'];
vm.runInNewContext(fs.readFileSync(new URL('../public/sw.js', `file://${__filename}`), 'utf8'), {
  URL,
  self: { location: { origin: 'https://example.test' }, addEventListener: (name, fn) => handlers[name] = fn, clients: { claim: async () => {} } },
  caches: { keys: async () => names, delete: async key => deleted.push(key) },
});
let finished;
handlers.activate({ waitUntil: promise => finished = promise });
finished.then(() => {
  assert.deepEqual(deleted.sort(), ['potpuri-reader-v14-books', 'potpuri-reader-v14-shell']);
  let intercepted = false;
  handlers.fetch({ request: { method: 'GET', url: 'https://example.test/local-ai-policy.json' }, respondWith: () => intercepted = true });
  assert.equal(intercepted, false, 'Safety policy must bypass the service-worker cache');
  console.log('Service-worker upgrades preserve model caches; safety policy stays network-only');
}).catch(error => { console.error(error); process.exit(1); });
