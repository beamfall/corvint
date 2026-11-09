// fulfillHeaders turns a fetch Response's headers into the object route.fulfill takes. Each
// Set-Cookie header is kept: Playwright's Chromium fulfill splits a set-cookie value on "\n", so
// separate session and CSRF cookies both reach the browser. Fetch's Headers already joins every
// other repeated header with ", ", which HTTP defines as equivalent. The proxy re-sends a decoded,
// complete body, so content-encoding and content-length are dropped.
export function fulfillHeaders(headers) {
  const out = Object.fromEntries(headers);
  const cookies = headers.getSetCookie();
  if (cookies.length) out['set-cookie'] = cookies.join('\n');
  delete out['content-encoding'];
  delete out['content-length'];
  return out;
}
