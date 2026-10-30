import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fulfillHeaders } from '../proxy-headers.mjs';

// V1-3026: separate session and CSRF Set-Cookie headers both survive the proxy.
test('fulfillHeaders keeps every Set-Cookie and joins other repeated headers', () => {
  const h = new Headers();
  h.append('Set-Cookie', 'session=abc; Path=/; HttpOnly');
  h.append('Set-Cookie', 'csrf=xyz; Path=/');
  h.append('Vary', 'Accept');
  h.append('Vary', 'Cookie');
  h.set('Content-Length', '10');
  h.set('Content-Encoding', 'gzip');
  h.set('Content-Type', 'text/html');
  assert.deepEqual(fulfillHeaders(h), {
    'set-cookie': 'session=abc; Path=/; HttpOnly\ncsrf=xyz; Path=/',
    vary: 'Accept, Cookie',
    'content-type': 'text/html',
  });
  assert.deepEqual(fulfillHeaders(new Headers({ 'content-type': 'text/plain' })), { 'content-type': 'text/plain' });
});

test('fulfillHeaders round-trips a two-cookie fetch response', async () => {
  const r = new Response('ok', { headers: [['set-cookie', 'session=abc'], ['set-cookie', 'csrf=xyz']] });
  assert.deepEqual(fulfillHeaders(r.headers)['set-cookie'].split('\n'), ['session=abc', 'csrf=xyz']);
});

test('both browser proxies fulfil through fulfillHeaders', () => {
  for (const file of ['navigation.mjs', 'observe.mjs']) {
    const src = readFileSync(new URL('../' + file, import.meta.url), 'utf8');
    assert.match(src, /import \{ fulfillHeaders \} from '\.\/proxy-headers\.mjs';/, file);
    assert.match(src, /fulfillHeaders\((r|response)\.headers\)/, file);
    assert.doesNotMatch(src, /Object\.fromEntries\((r|response)\.headers\)/, file);
  }
});
