'use strict';
// DCP-V1-081: drives the behavior-events producer with synthetic Playwright pages, locators, a
// stand-in `playwright/test` expect and a test-info object. Node built-ins only; no Playwright
// install is needed. The clean walk must reproduce, byte for byte, the annotations of the committed
// report fixture that the Go importer witnesses (testdata/playwright-behavior/report.json).
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { EventEmitter } = require('node:events');
const { extend } = require('./behavior-events.cjs');

class Frame { constructor() { this.href = 'about:blank'; } url() { return this.href; } }
class Locator { constructor(frame, selector) { this._frame = frame; this._selector = selector; } }
class Page extends EventEmitter {
  constructor(opener = null) { super(); this.main = new Frame(); this._opener = opener; }
  mainFrame() { return this.main; }
  url() { return this.main.href; }
  locator(selector) { return new Locator(this.main, selector); }
}
class Context extends EventEmitter {
  constructor() { super(); this.list = []; }
  pages() { return this.list.slice(); }
  open(page) { this.list.push(page); this.emit('page', page); return page; }
}
class Request {
  constructor(page, url, from) { this.page = page; this.href = url; this.from = from; }
  url() { return this.href; }
  redirectedFrom() { return this.from || null; }
  isNavigationRequest() { return true; }
  frame() { return this.page.mainFrame(); }
}

// navigate emits the document request chain (each HTTP hop, then the final URL) and the commit.
function navigate(page, url, hops = [], frame = page.mainFrame()) {
  let request = null;
  for (const hop of [...hops, url]) {
    request = new Request(page, hop, request);
    page.emit('request', request);
  }
  frame.href = url;
  page.emit('framenavigated', frame);
}

// The stand-in project resolves `playwright/test` to an expect that asks `outcome` whether each
// matcher passes, so a test can make the genuine-matcher path pass or fail on demand.
const calls = [];
let outcome = () => true;
globalThis.__corvintBehaviorExpect = actual => {
  const matchers = negated => new Proxy({}, {
    get(_, name) {
      if (name === 'not') return matchers(true);
      if (typeof name !== 'string' || !name.startsWith('to')) return undefined;
      return async (...args) => {
        calls.push({ actual, name, args, negated });
        const result = await outcome({ actual, name, args, negated });
        if (!result) throw new Error(`${name} failed`);
      };
    },
  });
  return matchers(false);
};
const project = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-behavior-'));
fs.mkdirSync(path.join(project, 'node_modules', 'playwright'), { recursive: true });
fs.writeFileSync(path.join(project, 'node_modules', 'playwright', 'package.json'), '{"name":"playwright","version":"0.0.0-test"}');
fs.writeFileSync(path.join(project, 'node_modules', 'playwright', 'test.js'), 'module.exports = { expect: (...a) => globalThis.__corvintBehaviorExpect(...a) };\n');

// harness builds the extended test and returns a runner for one attempt of the behavior fixture.
function harness({ fixtures = [], roles = [] } = {}) {
  const warnings = [];
  let defined;
  extend({ extend: f => { defined = f; return {}; } }, { from: project, warn: m => warnings.push(m) });
  const context = new Context();
  const testInfo = { annotations: [], errors: [] };
  const attempt = async body => {
    let api;
    await defined.behavior({ context, behaviorFixtures: fixtures, behaviorRoles: roles }, async a => { api = a; await body(api); }, testInfo);
    return api;
  };
  const events = () => testInfo.annotations.filter(a => a.type === 'corvint-behavior-event').map(a => a.description);
  return { warnings, context, testInfo, attempt, events };
}

function fixtureAnnotations() {
  const report = JSON.parse(fs.readFileSync(path.join(__dirname, 'testdata', 'playwright-behavior', 'report.json'), 'utf8'));
  const find = suites => {
    for (const suite of suites || []) {
      for (const spec of suite.specs || []) {
        for (const t of spec.tests) if (spec.title === 'lists items' && t.projectName === 'alpha') return t.results[0].annotations;
      }
      const found = find(suite.suites);
      if (found) return found;
    }
    return null;
  };
  return find(report.suites);
}

// walk is the items flow the committed report fixture declares.
async function walk(h, page) {
  await h.attempt(async behavior => {
    behavior.cleanup(async () => {});
    navigate(page, 'http://127.0.0.1:4173/items');
    await behavior.expect({ id: 'items-visible', behavior: 'list-items', criterion: 'items-visible' })(page.locator('#count')).toHaveText('2');
    await behavior.negativeControl('empty-hidden')(page.locator('#empty')).toBeHidden();
    navigate(page, 'http://127.0.0.1:4173/done');
  });
}

test.beforeEach(() => { calls.length = 0; outcome = () => true; });

test('DCP-V1-081: a clean walk publishes the committed fixture annotations byte for byte', async () => {
  const h = harness({ fixtures: ['seeded-items'] });
  const page = h.context.open(new Page());
  await walk(h, page);
  assert.deepEqual(h.testInfo.annotations, fixtureAnnotations());
  assert.deepEqual(calls.map(c => [c.name, c.args, c.negated]), [['toHaveText', ['2'], false], ['toBeHidden', [], false]]);
  assert.deepEqual(h.warnings, []);
});

test('DCP-V1-081: a failing or error-raising matcher records no event', async () => {
  const h = harness();
  const page = h.context.open(new Page());
  outcome = ({ name }) => name !== 'toHaveText';
  await assert.rejects(h.attempt(async behavior => {
    await behavior.expect({ id: 'a', behavior: 'b', criterion: 'c' })(page.locator('#count')).toHaveText('2');
  }), /toHaveText failed/);
  assert.deepEqual(h.events(), []);

  const soft = harness();
  const softPage = soft.context.open(new Page());
  outcome = () => { soft.testInfo.errors.push(new Error('soft')); return true; };
  await soft.attempt(async behavior => {
    await behavior.expect({ id: 'a', behavior: 'b', criterion: 'c' })(softPage.locator('#count')).toHaveText('2');
    await behavior.negativeControl('n')(softPage.locator('#x')).toBeHidden();
  });
  assert.deepEqual(soft.events(), []);
});

test('DCP-V1-081: inexact tuples run the matcher and record nothing', async () => {
  const h = harness();
  const page = h.context.open(new Page());
  const other = new Page();
  class Impostor { constructor() { this._frame = page.mainFrame(); this._selector = '#count'; } }
  await h.attempt(async behavior => {
    const meta = { id: 'a', behavior: 'b', criterion: 'c' };
    await behavior.expect(meta)(page.locator('#count')).toHaveText(/2/);
    await behavior.expect(meta)(page.locator('#count')).toHaveText(['2']);
    await behavior.expect(meta)(page.locator('#count')).toHaveText('');
    await behavior.expect(meta)(page.locator('#count')).toHaveText('2', { ignoreCase: true });
    await behavior.expect(meta)(page.locator('#count')).toHaveCount(2.5);
    await behavior.expect(meta)(page.locator('internal:control=enter-frame >> #count')).toHaveText('2');
    await behavior.expect(meta)(other.locator('#count')).toHaveText('2');
    await behavior.expect(meta)(new Impostor()).toHaveText('2');
    await behavior.expect(meta)({ textContent: '2' }).toHaveText('2');
  });
  assert.equal(calls.length, 9);
  assert.deepEqual(h.events(), []);
  assert.ok(h.warnings.length >= 4);
});

test('DCP-V1-081: inherited or hidden matcher options never reach a recorded matcher', async () => {
  const h = harness();
  const page = h.context.open(new Page());
  const inherited = Object.create({ ignoreCase: true });
  const hidden = { timeout: 50 };
  Object.defineProperty(hidden, 'ignoreCase', { value: true, enumerable: false });
  const inheritedTimeout = Object.create({ timeout: 5 });
  await h.attempt(async behavior => {
    const meta = { id: 'a', behavior: 'b', criterion: 'c' };
    await behavior.expect(meta)(page.locator('#count')).toHaveText('2', inherited);
    await behavior.expect(meta)(page.locator('#count')).toHaveText('2', hidden);
    await behavior.expect(meta)(page.locator('#count')).toHaveText('2', inheritedTimeout);
  });
  assert.deepEqual(calls.map(c => c.args), [['2', {}], ['2', { timeout: 50 }], ['2', {}]]);
  for (const call of calls) assert.deepEqual(Object.getOwnPropertyNames(call.args[1]).filter(k => k !== 'timeout'), []);
  for (const call of calls) assert.equal(Object.getPrototypeOf(call.args[1]), Object.prototype);
  assert.equal(h.events().length, 3);
});

test('DCP-V1-081: the recorded tuple is the call itself, including timeout-only options and counts', async () => {
  const h = harness();
  const page = h.context.open(new Page());
  await h.attempt(async behavior => {
    navigate(page, 'http://127.0.0.1/list?token=zz#x');
    await behavior.expect({ id: 'n', behavior: 'b', criterion: 'c' })(page.locator('li')).toHaveCount(3, { timeout: 100 });
    await behavior.negativeControl('gone')(page.locator('#gone')).not.toBeVisible();
  });
  const events = h.events().map(e => JSON.parse(e));
  assert.deepEqual(events.map(e => [e.kind, e.id, e.matcher, e.locator, e.value, e.sequence]), [
    ['page', '/list', '', '', '', 1],
    ['assertion', 'n', 'toHaveCount', 'li', '3', 2],
    ['negative-control', 'gone', '', '', '', 3],
  ]);
  assert.equal(calls[1].negated, true);
  assert.ok(!h.events().join('').includes('token'));
});

test('DCP-V1-081: the API exposes no recorder and refuses ambiguous use', async () => {
  const h = harness();
  const page = h.context.open(new Page());
  await h.attempt(async behavior => {
    assert.ok(Object.isFrozen(behavior));
    assert.deepEqual(Object.keys(behavior).sort(), ['cleanup', 'expect', 'negativeControl']);
    const matchers = behavior.expect({ id: 'a', behavior: 'b', criterion: 'c' })(page.locator('#x'));
    assert.ok(Object.isFrozen(matchers));
    assert.equal(matchers.not, undefined);
    assert.equal(matchers.toBe, undefined);
    assert.equal(behavior.negativeControl('n')(page.locator('#x')).then, undefined);
    assert.equal(behavior.negativeControl('n')(page.locator('#x')).not.not, undefined);
    assert.throws(() => behavior.expect({ id: 'a', behavior: 'b' }), TypeError);
    assert.throws(() => behavior.expect({ id: 'a', behavior: 'b', criterion: 'c\u0000' }), TypeError);
    assert.throws(() => behavior.negativeControl(''), TypeError);
    assert.throws(() => behavior.cleanup('x'), TypeError);
  });
  assert.throws(() => extend({}), TypeError);
  assert.throws(() => extend({ extend() {} }, { from: os.tmpdir() }), /cannot resolve playwright\/test/);
  await assert.rejects(harness({ fixtures: [''] }).attempt(async () => {}), TypeError);
});

test('DCP-V1-083: valueless state matchers record their canonical state name after they pass', async () => {
  const h = harness();
  const page = h.context.open(new Page());
  await h.attempt(async behavior => {
    navigate(page, 'http://127.0.0.1/form');
    const meta = id => ({ id, behavior: 'b', criterion: 'c' });
    await behavior.expect(meta('v'))(page.locator('#panel')).toBeVisible();
    await behavior.expect(meta('h'))(page.locator('#empty')).toBeHidden({ timeout: 100 });
    await behavior.expect(meta('e'))(page.locator('#save')).toBeEnabled({});
    await behavior.expect(meta('k'))(page.locator('#agree')).toBeChecked(undefined);
  });
  const events = h.events().map(e => JSON.parse(e));
  assert.deepEqual(events.map(e => [e.kind, e.id, e.matcher, e.locator, e.value]), [
    ['page', '/form', '', '', ''],
    ['assertion', 'v', 'toBeVisible', '#panel', 'visible'],
    ['assertion', 'h', 'toBeHidden', '#empty', 'hidden'],
    ['assertion', 'e', 'toBeEnabled', '#save', 'enabled'],
    ['assertion', 'k', 'toBeChecked', '#agree', 'checked'],
  ]);
  assert.deepEqual(calls.map(c => [c.name, c.args, c.negated]), [
    ['toBeVisible', [], false], ['toBeHidden', [{ timeout: 100 }], false], ['toBeEnabled', [{}], false], ['toBeChecked', [], false],
  ]);
  assert.deepEqual(h.warnings, []);
});

test('DCP-V1-083: state options, extra arguments, .not and failures record nothing; hidden options never reach a recorded matcher', async () => {
  const h = harness();
  const page = h.context.open(new Page());
  const hidden = { timeout: 50 };
  Object.defineProperty(hidden, 'visible', { value: false, enumerable: false });
  await h.attempt(async behavior => {
    const meta = { id: 'a', behavior: 'b', criterion: 'c' };
    const matchers = behavior.expect(meta)(page.locator('#x'));
    assert.equal(matchers.not, undefined);
    await matchers.toBeVisible({ visible: false });
    await matchers.toBeVisible({ visible: true });
    await matchers.toBeEnabled({ enabled: false, timeout: 5 });
    await matchers.toBeChecked({ checked: false });
    await matchers.toBeChecked({ indeterminate: true });
    await matchers.toBeHidden('x');
    await matchers.toBeHidden(null);
    await matchers.toBeVisible({ timeout: 5 }, 'extra');
    await matchers.toBeVisible(hidden);
  });
  assert.equal(calls.length, 9);
  assert.deepEqual(calls[0].args, [{ visible: false }]);
  assert.deepEqual(calls[8].args, [{ timeout: 50 }]);
  assert.equal(Object.getOwnPropertyNames(calls[8].args[0]).includes('visible'), false);
  // Only the last call records: its non-enumerable visible:false never reached the matcher, which
  // received the fresh timeout-only snapshot, so the event states what actually ran.
  assert.deepEqual(h.events().map(e => [JSON.parse(e).matcher, JSON.parse(e).value]), [['toBeVisible', 'visible']]);
  assert.ok(h.warnings.length >= 4);

  const failing = harness();
  const failingPage = failing.context.open(new Page());
  outcome = ({ name }) => name !== 'toBeChecked';
  await assert.rejects(failing.attempt(async behavior => {
    await behavior.expect({ id: 'a', behavior: 'b', criterion: 'c' })(failingPage.locator('#agree')).toBeChecked();
  }), /toBeChecked failed/);
  assert.deepEqual(failing.events(), []);
});

test('DCP-V1-081: page events follow real main-frame commits only', async () => {
  const h = harness();
  const first = new Page();
  first.main.href = 'http://127.0.0.1/start';
  h.context.list.push(first); // already open and committed when the fixture starts
  await h.attempt(async () => {
    navigate(first, 'http://127.0.0.1/start#again'); // same path: not a new page
    navigate(first, 'data:text/html,x'); // not http(s)
    const child = new Frame();
    navigate(first, 'http://127.0.0.1/child', [], child); // a subframe commit
    navigate(first, 'http://127.0.0.1/final', ['http://127.0.0.1/old']);
    const popup = h.context.open(new Page(first));
    navigate(popup, 'about:blank');
    navigate(popup, 'http://127.0.0.1/popup');
    navigate(popup, 'http://127.0.0.1/popup-next');
    const orphan = h.context.open(new Page(new Page()));
    navigate(orphan, 'http://127.0.0.1/orphan');
    const second = h.context.open(new Page());
    navigate(second, 'http://127.0.0.1/second');
    navigate(first, 'http://127.0.0.1/start');
  });
  const events = h.events().map(e => JSON.parse(e));
  assert.deepEqual(events.map(e => [e.page, e.id, e.navigation, e.parent_page, e.sequence]), [
    ['page-1', '/start', 'main-frame', '', 1],
    ['page-1', '/final', 'redirect', '', 2],
    ['page-2', '/popup', 'popup', 'page-1', 3],
    ['page-2', '/popup-next', 'main-frame', '', 4],
    ['page-3', '/second', 'main-frame', '', 5],
    ['page-1', '/start', 'main-frame', '', 6],
  ]);
  assert.ok(events.every(e => e.browser_context === 'context-1' && e.frame === 'main' && e.kind === 'page' && e.passed === true));
  assert.ok(h.warnings.some(w => w.includes('popup of an unobserved page')));
  // Listeners are detached at teardown: a later commit is not recorded.
  navigate(first, 'http://127.0.0.1/after');
  assert.equal(h.events().length, 6);
});

test('DCP-V1-081: a Playwright without Page._opener observes no pages', async () => {
  const h = harness();
  const page = new Page();
  delete page._opener;
  h.context.open(page);
  await h.attempt(async behavior => {
    navigate(page, 'http://127.0.0.1/items');
    await behavior.expect({ id: 'a', behavior: 'b', criterion: 'c' })(page.locator('#count')).toHaveText('2');
  });
  assert.deepEqual(h.events(), []);
  assert.ok(h.warnings.some(w => w.includes('_opener')));
});

test('DCP-V1-081: cleanup is published only after registered callbacks ran, in reverse order', async () => {
  const none = harness();
  await none.attempt(async () => {});
  assert.deepEqual(none.testInfo.annotations, []);

  const order = [];
  const ok = harness();
  await ok.attempt(async behavior => {
    behavior.cleanup(() => order.push(1));
    behavior.cleanup(async () => order.push(2));
  });
  assert.deepEqual(order, [2, 1]);
  assert.deepEqual(ok.testInfo.annotations, [{ type: 'corvint-behavior-cleanup', description: 'passed' }]);

  const failed = harness();
  const ran = [];
  await assert.rejects(failed.attempt(async behavior => {
    behavior.cleanup(() => ran.push('first'));
    behavior.cleanup(() => { throw new Error('undo failed'); });
  }), /undo failed/);
  assert.deepEqual(ran, ['first']);
  assert.deepEqual(failed.testInfo.annotations, [{ type: 'corvint-behavior-cleanup', description: 'failed' }]);

  let late;
  const closed = harness();
  await closed.attempt(async behavior => { late = behavior; });
  assert.throws(() => late.cleanup(() => {}), /after the test body ended/);
});

test('DCP-V1-081: hand-published behavior annotations or an overflow poison the events', async () => {
  const h = harness({ fixtures: ['f'], roles: ['admin'] });
  const page = h.context.open(new Page());
  await h.attempt(async behavior => {
    behavior.cleanup(() => {});
    navigate(page, 'http://127.0.0.1/items');
    h.testInfo.annotations.push({ type: 'corvint-behavior-event', description: '{"sequence":1}' });
  });
  assert.deepEqual(h.testInfo.annotations.map(a => a.type), ['corvint-behavior-event', 'corvint-behavior-fixture', 'corvint-behavior-role', 'corvint-behavior-event', 'corvint-behavior-cleanup']);
  assert.throws(() => JSON.parse(h.testInfo.annotations[3].description));

  const big = harness();
  const bigPage = big.context.open(new Page());
  await big.attempt(async () => {
    for (let i = 0; i <= 4096; i++) navigate(bigPage, `http://127.0.0.1/p${i}`);
  });
  assert.equal(big.events().length, 1);
  assert.throws(() => JSON.parse(big.events()[0]));
});

test('DCP-V1-081: a matcher resolving after teardown records nothing', async () => {
  const h = harness();
  const page = h.context.open(new Page());
  let release;
  outcome = () => new Promise(resolve => { release = resolve; });
  let pending;
  await h.attempt(async behavior => {
    pending = behavior.expect({ id: 'a', behavior: 'b', criterion: 'c' })(page.locator('#count')).toHaveText('2');
  });
  release(true);
  await pending;
  assert.deepEqual(h.events(), []);
});
