// Ripple PWA Service Worker · P12-E
// 策略：
//   - 静态资源（HTML/JS/CSS/图标/manifest）：stale-while-revalidate
//   - API GET /api/v1/lakes /nodes /perma_nodes /recommendations /search：network-first（超时 5s 回落缓存）
//   - 其他请求：network-only（不缓存 POST/PUT/DELETE/SSE/WS）
//
// 版本号变化时旧 cache 自动清理。
const CACHE_VERSION = 'ripple-v3';
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

const API_GET_PREFIXES = [
  '/api/v1/lakes',
  '/api/v1/nodes',
  '/api/v1/perma_nodes',
  '/api/v1/recommendations',
  '/api/v1/search',
];

const NETWORK_TIMEOUT_MS = 5000;

function isCacheableAPI(url) {
  return API_GET_PREFIXES.some((p) => url.pathname.startsWith(p));
}

/** network-first with timeout — 超时或离线时回落缓存 */
function networkFirstWithTimeout(req, cacheName) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), NETWORK_TIMEOUT_MS);
  return fetch(req, { signal: controller.signal })
    .then((resp) => {
      clearTimeout(timer);
      const copy = resp.clone();
      caches.open(cacheName).then((c) => c.put(req, copy)).catch(() => {});
      return resp;
    })
    .catch(() => {
      clearTimeout(timer);
      return caches.match(req).then((r) => r ||
        new Response('{"offline":true}', { status: 503, headers: { 'Content-Type': 'application/json' } }));
    });
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
    event.respondWith(networkFirstWithTimeout(req, API_CACHE));
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
