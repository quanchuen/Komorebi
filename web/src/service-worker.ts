/// <reference lib="webworker" />

import { build, files, version } from '$service-worker';

const worker = self as unknown as ServiceWorkerGlobalScope;
const cacheName = `komorebi-shell-${version}`;
const shellAssets = [...build, ...files];
const shellAssetPaths = new Set(shellAssets);

worker.addEventListener('install', (event) => {
  event.waitUntil(caches.open(cacheName).then((cache) => cache.addAll(shellAssets)));
  worker.skipWaiting();
});

worker.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(keys.filter((key) => key !== cacheName).map((key) => caches.delete(key)))
      )
      .then(() => worker.clients.claim())
  );
});

worker.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;
  const url = new URL(event.request.url);
  if (url.origin !== worker.location.origin || url.pathname.startsWith('/api/')) return;

  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request)
        .then((response) => {
          if (response.ok) {
            const copy = response.clone();
            void caches.open(cacheName).then((cache) => cache.put(event.request, copy));
          }
          return response;
        })
        .catch(async () => {
          return (
            (await caches.match(event.request)) ?? (await caches.match('/')) ?? Response.error()
          );
        })
    );
    return;
  }

  // Only the precached app shell (immutable build output + static files) is
  // served cache-first. Everything else same-origin — SvelteKit `__data.json`
  // load payloads, dev proxies such as /nominatim — is dynamic and must always
  // hit the network so conditions, ETAs, and suggestions never go stale.
  if (!shellAssetPaths.has(url.pathname)) return;

  event.respondWith(
    caches.match(event.request).then((cached) => {
      if (cached) return cached;
      return fetch(event.request).then((response) => {
        if (response.ok) {
          const copy = response.clone();
          void caches.open(cacheName).then((cache) => cache.put(event.request, copy));
        }
        return response;
      });
    })
  );
});
