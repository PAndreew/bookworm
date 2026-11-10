// Potpuri service worker.
//
// Deliberately conservative about what it caches:
//   - the app shell: network-first with offline fallback; static assets: cache-first;
//   - PUBLIC-DOMAIN page content: cached, so a free book stays readable on a train;
//   - everything else, including licensed content and every authenticated call:
//     network-only, never stored.
//
// That last rule matters. Caching licensed page text would put publisher content
// in the browser's cache in plaintext, which is exactly what a licensor would
// object to. The API marks what is safe to keep with `Cache-Control: public` on
// public-domain responses; we do not guess.

const VERSION = 'potpuri-reader-v16';
const SHELL = `${VERSION}-shell`;
const BOOKS = `${VERSION}-books`;

const SHELL_URLS = ['/', '/app', '/manifest.webmanifest'];

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(SHELL).then((c) => c.addAll(SHELL_URLS)).then(() => self.skipWaiting()),
  );
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(keys.filter((k) => !k.startsWith(VERSION)).map((k) => caches.delete(k))),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (e) => {
  const { request } = e;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;

  // Never cache anything that depends on who is asking.
  const isPrivateApi =
    url.pathname.startsWith('/v1/') &&
    !url.pathname.startsWith('/v1/books') &&
    !url.pathname.startsWith('/v1/quotes');

  // The shelf and per-book bookmarks now include authenticated reading history.
  if (isPrivateApi || url.pathname === '/v1/books' || /^\/v1\/books\/[^/]+\/position$/.test(url.pathname)) return; // network-only

  // Book content: stale-while-revalidate, but only if the API says it is cacheable.
  if (url.pathname.startsWith('/v1/books')) {
    e.respondWith(
      caches.open(BOOKS).then(async (cache) => {
        const hit = await cache.match(request);
        const live = fetch(request)
          .then((res) => {
            const cc = res.headers.get('Cache-Control') ?? '';
            if (res.ok && cc.includes('public')) cache.put(request, res.clone());
            return res;
          })
          .catch(() => hit);
        return hit ?? live;
      }),
    );
    return;
  }

  // Always load current HTML online; old HTML references old UI bundles forever.
  if (request.mode === 'navigate') {
    e.respondWith(
      fetch(request)
        .then(async (res) => {
          if (res.ok) {
            const cache = await caches.open(SHELL);
            await cache.put(request, res.clone());
          }
          return res;
        })
        .catch(async () => (await caches.match(request)) ?? caches.match('/app')),
    );
    return;
  }

  // Static assets: cache first, fall back to the network.
  e.respondWith(
    caches.match(request).then(
      (hit) =>
        hit ??
        fetch(request)
          .then((res) => {
            if (res.ok && res.type === 'basic') {
              const copy = res.clone();
              caches.open(SHELL).then((c) => c.put(request, copy));
            }
            return res;
          })
          .catch(() => caches.match('/app')),
    ),
  );
});
