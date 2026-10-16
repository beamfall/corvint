'use strict';
// LPCV-V0-072: drives the route-stubs producer with a synthetic Playwright context and pages.
// Node built-ins only; no Playwright install is needed.
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { EventEmitter } = require('node:events');
const { extend, _internal } = require('./route-stubs.cjs');

// Router stands in for Playwright's route registry: the last matching registration runs first.
class Router extends EventEmitter {
  constructor() { super(); this.routes = []; }
  async route(url, handler, options) { this.routes.push({ url, handler, options }); }
  async unroute(url, handler) { this.routes = this.routes.filter(r => !(r.url === url && (!handler || r.handler === handler))); }
  async request(href) {
    for (const r of [...this.routes].reverse()) {
      const hit = typeof r.url === 'string' ? href.includes(r.url.replaceAll('*', '')) : r.url instanceof RegExp ? r.url.test(href) : typeof r.url === 'function' ? r.url(new URL(href)) : false;
      if (hit) { await r.handler({ fulfill() {} }, { url: () => href }); return true; }
    }
    return false;
  }
}
class Page extends Router {}
class Context extends Router {
  constructor() { super(); this.list = []; }
  pages() { return this.list.slice(); }
  async newPage() { const page = new Page(); this.list.push(page); this.emit('page', page); return page; }
}

// run executes one attempt body against the extended `context` fixture and returns the annotations.
async function run(body, { annotations = [], rootDir = __dirname, context = new Context() } = {}) {
  let fixture;
  const base = { extend(fixtures) { fixture = fixtures.context; return {}; } };
  const warnings = [];
  extend(base, { warn: message => warnings.push(message) });
  const testInfo = { annotations: annotations.slice(), config: { rootDir } };
  await fixture({ context }, async ctx => body(ctx), testInfo);
  return { annotations: testInfo.annotations, warnings, context };
}

function record(annotations) {
  const found = annotations.filter(a => a.type === 'corvint-route-stubs');
  assert.equal(found.length, 1);
  return JSON.parse(found[0].description);
}

test('records hit and never-hit stubs with their registration line', async () => {
  const { annotations } = await run(async context => {
    const page = await context.newPage();
    await page.route('**/api/items', route => route.fulfill()); const hitLine = new Error().stack.split('\n')[1];
    await context.route(/\/api\/never/, route => route.fulfill());
    await page.request('http://127.0.0.1/api/items');
    await page.request('http://127.0.0.1/api/items');
    assert.match(hitLine, /route-stubs_test\.cjs:\d+/);
  });
  const { stubs } = record(annotations);
  assert.equal(stubs.length, 2);
  assert.deepEqual(stubs.map(s => [s.target, s.kind, s.pattern, s.hits, s.file]), [
    ['page', 'glob', '**/api/items', 2, 'route-stubs_test.cjs'],
    ['context', 'regexp', '/\\/api\\/never/', 0, 'route-stubs_test.cjs'],
  ]);
  assert.ok(Number.isInteger(stubs[0].line) && stubs[1].line === stubs[0].line + 1);
});

test('a producer that saw no stubs publishes an empty record', async () => {
  const { annotations } = await run(async () => {});
  assert.deepEqual(record(annotations), { stubs: [] });
});

test('predicate and other patterns, existing pages and unroute by handler', async () => {
  const { annotations, context } = await run(async context => {
    const existing = await context.newPage();
    const handler = route => route.fulfill();
    await existing.route(function isImage(url) { return url.pathname.endsWith('.png'); }, handler);
    await existing.route({ toString: () => 'pattern-object' }, handler);
    await existing.unroute('missing', handler);
    await existing.request('http://127.0.0.1/a.png');
    await context.route('**/gone', handler);
    await context.unroute('**/gone', handler);
    assert.equal(await context.request('http://127.0.0.1/gone'), false);
  });
  const { stubs } = record(annotations);
  assert.deepEqual(stubs.map(s => [s.target, s.kind, s.pattern, s.hits]), [
    ['page', 'predicate', 'isImage', 1],
    ['page', 'other', 'pattern-object', 0],
    ['context', 'glob', '**/gone', 0],
  ]);
  assert.ok(!Object.hasOwn(context, 'route') && !Object.hasOwn(context, 'newPage'), 'teardown restores the context');
});

test('more than 64 stubs publish no record', async () => {
  const { annotations, warnings } = await run(async context => {
    for (let i = 0; i < 65; i++) await context.route(`**/s${i}`, route => route.fulfill());
  });
  assert.equal(annotations.length, 0);
  assert.match(warnings.join('\n'), /more than 64/);
});

test('a foreign record is refused by adding the producer record beside it', async () => {
  const foreign = { type: 'corvint-route-stubs', description: '{"stubs":[]}' };
  const { annotations, warnings } = await run(async context => { await context.route('**/x', () => {}); }, { annotations: [foreign] });
  assert.equal(annotations.filter(a => a.type === 'corvint-route-stubs').length, 2, 'two records make the consumer report NOT_PRODUCED');
  assert.match(warnings.join('\n'), /outside the producer/);
});

test('teardown restores own-property overrides another fixture installed', async () => {
  const context = new Context();
  const page = await context.newPage();
  const own = { context: { route: context.route.bind(context), unroute: context.unroute.bind(context), newPage: context.newPage.bind(context) }, page: { route: page.route.bind(page), unroute: page.unroute.bind(page) } };
  Object.assign(context, own.context);
  Object.assign(page, own.page);
  await run(async ctx => { await ctx.route('**/a', () => {}); await page.route('**/b', () => {}); }, { context });
  for (const name of ['route', 'unroute', 'newPage']) assert.equal(context[name], own.context[name], `context.${name}`);
  for (const name of ['route', 'unroute']) assert.equal(page[name], own.page[name], `page.${name}`);
});

test('unroute also removes a registration made before instrumentation', async () => {
  const context = new Context();
  const handler = route => route.fulfill();
  await context.route('**/before', handler);
  const { annotations } = await run(async ctx => {
    await ctx.route('**/after', handler);
    await ctx.unroute('**/before', handler);
    await ctx.unroute('**/after', handler);
  }, { context });
  assert.equal(context.routes.length, 0);
  assert.equal(record(annotations).stubs.length, 1);
});

test('a page that resolves after teardown is not instrumented', async () => {
  const context = new Context();
  let release;
  const gate = new Promise(resolve => { release = resolve; });
  const original = context.newPage;
  context.newPage = async function () { await gate; return original.call(this); };
  let pending;
  await run(async ctx => { pending = ctx.newPage(); }, { context });
  release();
  const late = await pending;
  assert.ok(!Object.hasOwn(late, 'route') && !Object.hasOwn(late, 'unroute'), 'late page left untouched');
});

test('the record is published even when the test body throws', async () => {
  let fixture;
  extend({ extend(f) { fixture = f.context; } }, { warn() {} });
  const testInfo = { annotations: [], config: { rootDir: __dirname } };
  await assert.rejects(fixture({ context: new Context() }, async ctx => { await ctx.route('**/y', () => {}); throw new Error('boom'); }, testInfo));
  assert.equal(record(testInfo.annotations).stubs[0].hits, 0);
});

test('long patterns are clipped inside the bound and stack frames resolve', () => {
  const { clip, frameLocation } = _internal;
  const long = 'é'.repeat(800);
  const clipped = clip(long);
  assert.ok(Buffer.byteLength(clipped) <= 1024 && clipped.endsWith('...'));
  assert.equal(clip('a\u0000b'), 'a�b');
  assert.deepEqual(frameLocation('    at Object.<anonymous> (/tmp/x y/spec.ts:12:5)'), { file: '/tmp/x y/spec.ts', line: 12 });
  assert.deepEqual(frameLocation(`    at ${new URL('file:///tmp/spec.mjs')}:7:1`), { file: path.resolve('/tmp/spec.mjs'), line: 7 });
  assert.equal(frameLocation('    at node:internal/process/task_queues:95:5'), null);
});

test('extend refuses a non-Playwright test', () => {
  assert.throws(() => extend({}), /needs a Playwright test/);
});
