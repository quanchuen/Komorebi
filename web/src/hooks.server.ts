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

export const handle: Handle = async ({ event, resolve }) => {
  if (!event.url.pathname.startsWith('/api/')) return resolve(event);

  const apiURL = env.API_URL ?? 'http://127.0.0.1:8080';
  const upstream = new URL(`${event.url.pathname}${event.url.search}`, apiURL);
  const requestHeaders = new Headers(event.request.headers);
  requestHeaders.delete('host');

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
