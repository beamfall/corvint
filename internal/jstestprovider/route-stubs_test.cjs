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
  async unroute(url, handler) { this.routes = this.routes.filter(r => !(_internal.sameURL(r.url, url) && (!handler || r.handler === handler))); }
  async unrouteAll() { this.routes = []; }
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

class Browser {
  constructor() { this.contexts = []; }
  async newContext() { const context = new Context(); this.contexts.push(context); return context; }
  async newPage() { const context = await this.newContext(); return context.newPage().then(page => { page.context = () => context; return page; }); }
}

// fixturesOf returns the fixtures extend registers on a stand-in Playwright test.
function fixturesOf(warn) {
  let fixtures;
  extend({ extend(f) { fixtures = f; return {}; } }, { warn });
  return fixtures;
}

// attempt runs one test the way Playwright does: the automatic session fixture wraps the body,
// which receives the `context` fixture only when it asks for one (`context: null` means it does not).
async function attempt(fixtures, testInfo, context, body) {
  const [session, sessionOptions] = fixtures._corvintRouteStubs;
  assert.deepEqual(sessionOptions, { auto: true });
  await session({}, async observed => {
    if (context === null) return body();
    return fixtures.context({ context, _corvintRouteStubs: observed }, async ctx => body(ctx));
  }, testInfo);
}

// run executes one attempt body against the extended `context` fixture and returns the annotations.
async function run(body, { annotations = [], rootDir = __dirname, context = new Context(), browser } = {}) {
  const warnings = [];
  const fixtures = fixturesOf(message => warnings.push(message));
  const testInfo = { annotations: annotations.slice(), config: { rootDir } };
  if (!browser) {
    await attempt(fixtures, testInfo, context, body);
    return { annotations: testInfo.annotations, warnings, context };
  }
  const [worker, workerOptions] = fixtures.browser;
  assert.deepEqual(workerOptions, { scope: 'worker' });
  await worker({ browser }, async observed => attempt(fixtures, testInfo, context, ctx => body(ctx, observed)));
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
  assert.deepEqual(stubs.map(s => [s.target, s.kind, s.pattern, s.hits, s.unrouted]), [
    ['page', 'predicate', 'isImage', 1, undefined],
    ['page', 'other', 'pattern-object', 0, undefined],
    ['context', 'glob', '**/gone', 0, true],
  ]);
  assert.ok(!Object.hasOwn(context, 'route') && !Object.hasOwn(context, 'newPage'), 'teardown restores the context');
});

// LPCV-V0-214: a route stub that unroute or unrouteAll removes is marked unrouted, matched the way
// Playwright matches it: same URL (a RegExp by source and flags) and, when given, same handler, on
// the same instance only.
test('unroute and unrouteAll mark the stubs they remove', async () => {
  const { annotations } = await run(async context => {
    const page = await context.newPage();
    const kept = route => route.fulfill();
    const gone = route => route.fulfill();
    await page.route('**/api/a', kept);
    await page.route('**/api/a', gone);
    await page.unroute('**/api/a', gone);
    await page.route(/\/api\/b/i, kept);
    await page.request('http://127.0.0.1/api/B');
    await page.unroute(/\/api\/b/i);
    await page.route(/\/api\/c/, kept);
    await page.unroute(/\/api\/c/i);
    await context.route('**/api/d', kept);
    await page.unroute('**/api/d');
    const other = await context.newPage();
    await other.route('**/api/e', kept);
    await other.unrouteAll({ behavior: 'ignoreErrors' });
  });
  const { stubs } = record(annotations);
  assert.deepEqual(stubs.map(s => [s.target, s.pattern, s.hits, s.unrouted]), [
    ['page', '**/api/a', 0, undefined],
    ['page', '**/api/a', 0, true],
    ['page', '/\\/api\\/b/i', 1, true],
    ['page', '/\\/api\\/c/', 0, undefined],
    ['context', '**/api/d', 0, undefined],
    ['page', '**/api/e', 0, true],
  ]);
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
  const fixtures = fixturesOf(() => {});
  const testInfo = { annotations: [], config: { rootDir: __dirname } };
  await assert.rejects(attempt(fixtures, testInfo, new Context(), async ctx => { await ctx.route('**/y', () => {}); throw new Error('boom'); }));
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
  assert.deepEqual(frameLocation('    at async /tmp/spec.cjs:9:3'), { file: '/tmp/spec.cjs', line: 9 });
});

// LPCV-V0-076: contexts the test creates, routeWebSocket and routeFromHAR.
test('contexts created with browser.newContext and browser.newPage are recorded', async () => {
  const browser = new Browser();
  const { annotations } = await run(async (_, observed) => {
    const own = await observed.newContext();
    await own.route('**/own', route => route.fulfill());
    const page = await own.newPage();
    await page.route('**/own-page', route => route.fulfill());
    await page.request('http://127.0.0.1/own-page');
    const solo = await observed.newPage();
    await solo.route('**/solo', route => route.fulfill());
  }, { context: null, browser });
  assert.deepEqual(record(annotations).stubs.map(s => [s.target, s.pattern, s.hits]), [
    ['context', '**/own', 0],
    ['page', '**/own-page', 1],
    ['page', '**/solo', 0],
  ]);
  assert.ok(!Object.hasOwn(browser, 'newContext') && !Object.hasOwn(browser, 'newPage'), 'worker teardown restores the browser');
  for (const context of browser.contexts) assert.ok(!Object.hasOwn(context, 'route'), 'test teardown restores created contexts');
});

test('a browser whose newPage bypasses its own newContext is still recorded', async () => {
  const browser = new Browser();
  browser.newPage = async function () {
    const context = new Context();
    this.contexts.push(context);
    const page = await context.newPage();
    page.context = () => context;
    return page;
  };
  const { annotations } = await run(async (_, observed) => {
    const solo = await observed.newPage();
    await solo.route('**/solo', route => route.fulfill());
  }, { context: null, browser });
  assert.deepEqual(record(annotations).stubs.map(s => [s.target, s.pattern, s.hits]), [['page', '**/solo', 0]]);
  assert.equal(typeof browser.newPage, 'function');
  assert.ok(Object.hasOwn(browser, 'newPage') && !Object.hasOwn(browser, 'newContext'), 'teardown restores the own newPage');
});

test('a test without a browser fixture or created context publishes no record', async () => {
  const { annotations } = await run(async () => {}, { context: null });
  assert.equal(annotations.length, 0);
});

test('a context that resolves after its test ended is not adopted by the next test', async () => {
  let release;
  const gate = new Promise(resolve => { release = resolve; });
  const browser = new Browser();
  browser.newContext = async function () { await gate; return Browser.prototype.newContext.call(this); };
  const fixtures = fixturesOf(() => {});
  const [worker] = fixtures.browser;
  const first = { annotations: [], config: { rootDir: __dirname } };
  const second = { annotations: [], config: { rootDir: __dirname } };
  let late;
  await worker({ browser }, async observed => {
    await attempt(fixtures, first, null, async () => { late = observed.newContext(); });
    await attempt(fixtures, second, null, async () => {
      release();
      const context = await late;
      await context.route('**/late', route => route.fulfill());
    });
  });
  assert.equal(first.annotations.length, 0);
  assert.equal(second.annotations.length, 0);
});

test('a context created outside a running test is not recorded', async () => {
  const browser = new Browser();
  const restore = _internal.observeBrowser(browser);
  const outside = await browser.newContext();
  restore();
  assert.ok(!Object.hasOwn(outside, 'route'));
  assert.ok(!Object.hasOwn(browser, 'newContext'));
});

test('routeWebSocket registrations are recorded with their api and hits', async () => {
  class SocketPage extends Page {
    async routeWebSocket(url, handler) { this.sockets = (this.sockets || []).concat({ url, handler }); }
    async open(href) { for (const s of this.sockets || []) if (href.includes(s.url.replaceAll('*', ''))) s.handler({ url: () => href }); }
  }
  const context = new Context();
  context.newPage = async function () { const page = new SocketPage(); this.list.push(page); this.emit('page', page); return page; };
  const { annotations } = await run(async ctx => {
    const page = await ctx.newPage();
    await page.routeWebSocket('**/live', ws => ws);
    await page.routeWebSocket('**/never', ws => ws);
    await page.routeWebSocket('**/no-handler');
    await page.open('ws://127.0.0.1/live');
  }, { context });
  assert.deepEqual(record(annotations).stubs.map(s => [s.target, s.api, s.pattern, s.hits]), [
    ['page', 'routeWebSocket', '**/live', 1],
    ['page', 'routeWebSocket', '**/never', 0],
  ]);
});

test('a route registered from inside Playwright routeFromHAR is named routeFromHAR', () => {
  const stack = [
    'Error',
    `    at callStack (${require.resolve('./route-stubs.cjs')}:1:1)`,
    '    at HarRouter.addPageRoute (/p/node_modules/playwright-core/lib/coreBundle.js:10:5)',
    '    at Page.routeFromHAR (/p/node_modules/playwright-core/lib/coreBundle.js:20:5)',
    '    at async /repo/tests/har.spec.cjs:7:3',
  ].join('\n');
  assert.deepEqual(_internal.callSite('/repo', stack), { file: 'tests/har.spec.cjs', line: 7, har: true });
  const plain = stack.replace(' at Page.routeFromHAR', ' at Page.goto');
  assert.deepEqual(_internal.callSite('/repo', plain), { file: 'tests/har.spec.cjs', line: 7 });
  const ownHelper = stack.replace('/p/node_modules/playwright-core/lib/coreBundle.js:20:5', '/repo/helpers.cjs:20:5');
  assert.deepEqual(_internal.callSite('/repo', ownHelper), { file: 'helpers.cjs', line: 20 }, 'a routeFromHAR outside node_modules is the call site, not Playwright');
});

test('extend refuses a non-Playwright test', () => {
  assert.throws(() => extend({}), /needs a Playwright test/);
});
