const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const handlers = {};
const deleted = [];
const names = ['potpuri-reader-v14-shell', 'potpuri-reader-v14-books', 'potpuri-reader-v15-shell', 'potpuri-reader-v15-books', 'webllm/model', 'webllm/wasm', 'another-app-cache'];
vm.runInNewContext(fs.readFileSync(new URL('../public/sw.js', `file://${__filename}`), 'utf8'), {
  self: { addEventListener: (name, fn) => handlers[name] = fn, clients: { claim: async () => {} } },
  caches: { keys: async () => names, delete: async key => deleted.push(key) },
});
let finished;
handlers.activate({ waitUntil: promise => finished = promise });
finished.then(() => {
  assert.deepEqual(deleted.sort(), ['potpuri-reader-v14-books', 'potpuri-reader-v14-shell']);
  console.log('Service-worker upgrades preserve model/runtime and unrelated caches');
}).catch(error => { console.error(error); process.exit(1); });
