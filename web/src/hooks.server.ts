import { env } from '$env/dynamic/private';
import type { Handle } from '@sveltejs/kit';

const hopByHopHeaders = new Set([
  'connection',
  'keep-alive',
  'proxy-authenticate',
  'proxy-authorization',
  'te',
  'trailer',
  'transfer-encoding',
  'upgrade'
]);

// Nominatim's usage policy requires a User-Agent that identifies the
// application (https://operations.osmfoundation.org/policies/nominatim/).
const NOMINATIM_USER_AGENT = 'komorebi-web/0.1 (cycling route planner; self-hosted)';

// Same-origin proxies so the browser never needs to know where the API, the
// tile server or the geocoders live. Mirrors the Vite dev proxy in
// vite.config.ts:
//   /api/*       -> API_URL   (path kept as-is)
//   /tiles/*     -> TILES_URL (the /tiles prefix is stripped)
//   /nominatim/* -> nominatim.openstreetmap.org (prefix stripped)
//   /gsi/*       -> msearch.gsi.go.jp, Japan's GSI address search (prefix stripped)
const proxies: Array<{
  prefix: string;
  target: () => string;
  strip: boolean;
  headers?: Record<string, string>;
}> = [
  { prefix: '/api/', target: () => env.API_URL ?? 'http://127.0.0.1:8080', strip: false },
  { prefix: '/tiles/', target: () => env.TILES_URL ?? 'http://127.0.0.1:3000', strip: true },
  {
    prefix: '/nominatim/',
    target: () => 'https://nominatim.openstreetmap.org',
    strip: true,
    headers: { 'user-agent': NOMINATIM_USER_AGENT }
  },
  { prefix: '/gsi/', target: () => 'https://msearch.gsi.go.jp', strip: true }
];

export const handle: Handle = async ({ event, resolve }) => {
  const proxy = proxies.find((p) => event.url.pathname.startsWith(p.prefix));
  if (!proxy) return resolve(event);

  const pathname = proxy.strip
    ? event.url.pathname.slice(proxy.prefix.length - 1)
    : event.url.pathname;
  const upstream = new URL(`${pathname}${event.url.search}`, proxy.target());
  const requestHeaders = new Headers(event.request.headers);
  requestHeaders.delete('host');
  // Node fetch decompresses the body but keeps content-encoding/length, so never ask for compression.
  requestHeaders.delete('accept-encoding');
  for (const [name, value] of Object.entries(proxy.headers ?? {})) requestHeaders.set(name, value);

  const response = await fetch(upstream, {
    method: event.request.method,
    headers: requestHeaders,
    body:
      event.request.method === 'GET' || event.request.method === 'HEAD'
        ? undefined
        : await event.request.arrayBuffer(),
    redirect: 'manual'
  });

  const responseHeaders = new Headers(response.headers);
  for (const header of hopByHopHeaders) responseHeaders.delete(header);

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders
  });
};
