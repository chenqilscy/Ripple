// Ripple PWA Service Worker · M4-T4
// 策略：
//   - 静态资源（HTML/JS/CSS/图标/manifest）：stale-while-revalidate
//   - API GET /api/v1/lakes /nodes /perma_nodes / recommendations：network-first，失败回落缓存
//   - 其他请求：network-only（不缓存 POST/PUT/DELETE/SSE/WS）
//
// 版本号变化时旧 cache 自动清理。
const CACHE_VERSION = 'ripple-v1';
const STATIC_CACHE = `${CACHE_VERSION}-static`;
const API_CACHE = `${CACHE_VERSION}-api`;

const STATIC_PRECACHE = [
  '/',
  '/manifest.webmanifest',
  '/icon-192.svg',
  '/icon-512.svg',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(STATIC_CACHE).then((c) => c.addAll(STATIC_PRECACHE)).then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => !k.startsWith(CACHE_VERSION)).map((k) => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

const API_GET_PREFIXES = ['/api/v1/lakes', '/api/v1/nodes', '/api/v1/perma_nodes', '/api/v1/recommendations'];

function isCacheableAPI(url) {
  return API_GET_PREFIXES.some((p) => url.pathname.startsWith(p));
}

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return; // 不缓存写请求
  const url = new URL(req.url);

  // SSE/WS 不要拦截
  const accept = req.headers.get('accept') || '';
  if (accept.includes('text/event-stream') || req.headers.get('upgrade')) return;

  // 同源 API
  if (isCacheableAPI(url) && url.origin === self.location.origin) {
    event.respondWith(
      fetch(req)
        .then((resp) => {
          const copy = resp.clone();
          caches.open(API_CACHE).then((c) => c.put(req, copy)).catch(() => {});
          return resp;
        })
        .catch(() => caches.match(req).then((r) => r || new Response('{"offline":true}', { status: 503, headers: { 'Content-Type': 'application/json' } })))
    );
    return;
  }

  // 同源静态：stale-while-revalidate
  if (url.origin === self.location.origin) {
    event.respondWith(
      caches.match(req).then((cached) => {
        const fetchPromise = fetch(req).then((resp) => {
          const copy = resp.clone();
          caches.open(STATIC_CACHE).then((c) => c.put(req, copy)).catch(() => {});
          return resp;
        }).catch(() => cached);
        return cached || fetchPromise;
      })
    );
  }
});
