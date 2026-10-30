'use strict';
// Corvint route-stubs producer (LPCV-V0-072, LPCV-V0-076). It records every request stub a test
// registers through `page.route`, `context.route`, `routeWebSocket` or `routeFromHAR` on the test's
// `context` fixture, on a context the test creates with `browser.newContext()` or
// `browser.newPage()`, and on those contexts' pages: the stub's target, API, URL pattern kind and
// text, how often its handler ran in the attempt, and the file and line that registered it. At test
// teardown it publishes one `corvint-route-stubs` annotation whose description is the closed JSON
// record `{"stubs":[...]}` that `ParsePlaywrightJSON` reads into the attempt's `routeStubs`.
// Install it in a fixtures module:
//
//   const { test: base } = require('@playwright/test');
//   const test = require('/path/to/corvint/internal/jstestprovider/route-stubs.cjs').extend(base);
//
// The producer is optional and additive. A test that neither takes `context` or `page` nor creates
// a context publishes no record, and neither does an attempt with more than 64 stubs; the consumer
// reads a missing record as NOT_PRODUCED, never as zero hits. Contexts created outside a test body
// and its test-scoped fixtures (`beforeAll`, worker fixtures) and contexts from a browser the test
// launches itself are not observed.
const path = require('node:path');
const { fileURLToPath } = require('node:url');

const annotationType = 'corvint-route-stubs';
const maxStubs = 64; // jstestprovider.MaxRouteStubs
const maxTextBytes = 1024; // jstestprovider.maxRouteStubTextBytes
const maxHits = 2 ** 31 - 1;

// clip keeps a pattern or path inside the record's text bound: well-formed UTF-8 without NUL, at
// most 1024 bytes, cut on a code point and marked with "..." when it was longer.
function clip(value) {
  const clean = value.toWellFormed().replaceAll('\u0000', '�');
  if (Buffer.byteLength(clean) <= maxTextBytes) return clean;
  let out = '';
  let bytes = 0;
  for (const point of clean) {
    const size = Buffer.byteLength(point);
    if (bytes + size > maxTextBytes - 3) break;
    out += point;
    bytes += size;
  }
  return `${out}...`;
}

// describe names a route URL argument: a glob string, a RegExp, a predicate function (by its
// name), or anything else by its string form.
function describe(url) {
  if (typeof url === 'string') return { kind: 'glob', pattern: clip(url || '(empty)') };
  if (url instanceof RegExp) return { kind: 'regexp', pattern: clip(String(url)) };
  if (typeof url === 'function') return { kind: 'predicate', pattern: clip(url.name || 'anonymous') };
  let text = '';
  try { text = String(url); } catch { text = ''; }
  return { kind: 'other', pattern: clip(text || 'unknown') };
}

// frameLocation returns the file and line of one V8 stack frame, or null.
function frameLocation(line) {
  let site = line.trim().replace(/^at\s+/, '').replace(/^async\s+/, '');
  if (site.endsWith(')') && site.includes('(')) site = site.slice(site.lastIndexOf('(') + 1, -1);
  const match = /^(.*):(\d+):\d+$/.exec(site);
  if (!match) return null;
  let file = match[1];
  if (file.startsWith('file://')) {
    try { file = fileURLToPath(file); } catch { return null; }
  }
  if (!path.isAbsolute(file)) return null;
  return { file, line: Number(match[2]) };
}

// callSite is the first stack frame outside this module, Node internals and node_modules: the
// test, hook or helper line that registered the stub. `har` is set when a node_modules frame before
// it is Playwright's routeFromHAR, whose HAR router registers its stub through the instance route.
function callSite(rootDir, stack = callStack()) {
  let har = false;
  for (const line of stack.split('\n').slice(1)) {
    const site = frameLocation(line);
    if (site && site.file !== __filename && /[\\/]node_modules[\\/]/.test(site.file) && /\brouteFromHAR\b/.test(line)) har = true;
    if (!site || site.file === __filename || /[\\/]node_modules[\\/]/.test(site.file)) continue;
    if (site.line < 1 || site.line > 10_000_000) return har ? { har } : {};
    let file = site.file;
    if (rootDir) {
      const relative = path.relative(rootDir, file);
      if (relative && !relative.startsWith('..') && !path.isAbsolute(relative)) file = relative;
    }
    return { file: clip(file.split(path.sep).join('/')), line: site.line, ...(har ? { har } : {}) };
  }
  return har ? { har } : {};
}

function callStack() {
  const limit = Error.stackTraceLimit;
  Error.stackTraceLimit = 32;
  const stack = new Error().stack || '';
  Error.stackTraceLimit = limit;
  return stack;
}

// stubFor builds one stub entry; `api` is omitted for a plain route registration.
function stubFor(target, api, url, rootDir) {
  const { har, ...site } = callSite(rootDir);
  const named = api === 'route' && har ? 'routeFromHAR' : api;
  return { target, ...(named === 'route' ? {} : { api: named }), ...describe(url), hits: 0, ...site };
}

function warner(sink) {
  const seen = new Set();
  return (key, message) => {
    if (seen.has(key)) return;
    seen.add(key);
    sink(message);
  };
}

// override replaces one method on an instance and pushes its undo onto restore: undo restores an
// own property the instance already had (another fixture's override) and otherwise deletes the
// producer's own.
function override(restore, target, name, replacement) {
  const own = Object.getOwnPropertyDescriptor(target, name);
  target[name] = replacement;
  restore.push(() => {
    if (own) Object.defineProperty(target, name, own);
    else delete target[name];
  });
}

// sameURL is Playwright's own unroute match: a string or predicate by identity, a RegExp by its
// source and flags.
function sameURL(a, b) {
  if (a instanceof RegExp && b instanceof RegExp) return a.source === b.source && a.flags === b.flags;
  return a === b;
}

// counted wraps a handler so each call counts as one hit of stub.
function counted(stub, handler) {
  return function (...args) {
    if (stub.hits < maxHits) stub.hits += 1;
    return handler.apply(this, args);
  };
}

// session instruments the contexts one attempt adopts and their pages. It publishes a record only
// when it adopted at least one context.
function session({ context, testInfo, warn }) {
  const rootDir = testInfo && testInfo.config && typeof testInfo.config.rootDir === 'string' ? testInfo.config.rootDir : '';
  const stubs = [];
  const restore = [];
  const seen = new WeakSet();
  let closed = false;
  let engaged = false;

  // instrument replaces route, unroute, unrouteAll and routeWebSocket on the instance. Each handler
  // is wrapped so a call counts as a hit; unroute(url, handler) still finds the wrappers of that
  // handler. A route stub that unroute or unrouteAll removes is marked unrouted (LPCV-V0-214).
  // routeFromHAR registers through the instance route and is named by its call site.
  const instrument = (target, kind) => {
    const route = target.route;
    const unroute = target.unroute;
    const unrouteAll = target.unrouteAll;
    const routeWebSocket = target.routeWebSocket;
    // live holds this instance's registered route stubs with their URL and handler, unserialized.
    const live = [];
    if (typeof route !== 'function') {
      warn(`no-route-${kind}`, `${kind}.route is absent in this Playwright; its stubs are not recorded`);
    } else {
      const wrappers = new Map();
      override(restore, target, 'route', function (url, handler, options) {
        if (typeof handler !== 'function') return route.call(this, url, handler, options);
        const stub = stubFor(kind, 'route', url, rootDir);
        stubs.push(stub);
        live.push({ stub, url, handler });
        const wrapper = counted(stub, handler);
        if (!wrappers.has(handler)) wrappers.set(handler, []);
        wrappers.get(handler).push(wrapper);
        return route.call(this, url, wrapper, options);
      });
      if (typeof unroute === 'function') {
        // unroute(url, handler) removes the producer's wrappers of handler and then any
        // registration of handler itself made before instrumentation.
        override(restore, target, 'unroute', async function (url, handler) {
          for (const entry of live) {
            if (sameURL(entry.url, url) && (typeof handler !== 'function' || entry.handler === handler)) entry.stub.unrouted = true;
          }
          if (typeof handler === 'function' && wrappers.has(handler)) {
            for (const wrapper of wrappers.get(handler)) await unroute.call(this, url, wrapper);
          }
          return unroute.call(this, url, handler);
        });
      }
      if (typeof unrouteAll === 'function') {
        override(restore, target, 'unrouteAll', function (...args) {
          for (const entry of live) entry.stub.unrouted = true;
          return unrouteAll.apply(this, args);
        });
      }
    }
    if (typeof routeWebSocket === 'function') {
      override(restore, target, 'routeWebSocket', function (url, handler) {
        if (typeof handler !== 'function') return routeWebSocket.call(this, url, handler);
        const stub = stubFor(kind, 'routeWebSocket', url, rootDir);
        stubs.push(stub);
        return routeWebSocket.call(this, url, counted(stub, handler));
      });
    }
  };

  const track = page => {
    if (closed || !page || seen.has(page)) return;
    seen.add(page);
    instrument(page, 'page');
  };
  // adopt instruments one context once: its route methods, every page it already has and every
  // page it opens until finish.
  const adopt = observed => {
    if (closed || !observed || seen.has(observed)) return;
    seen.add(observed);
    engaged = true;
    instrument(observed, 'context');
    const newPage = observed.newPage;
    if (typeof newPage === 'function') {
      override(restore, observed, 'newPage', async function (...args) {
        const page = await newPage.apply(this, args);
        track(page);
        return page;
      });
    }
    if (typeof observed.on === 'function') {
      observed.on('page', track);
      restore.push(() => observed.off('page', track));
    }
    if (typeof observed.pages === 'function') for (const page of observed.pages()) track(page);
  };
  if (context) adopt(context);

  return {
    adopt,
    finish() {
      closed = true;
      for (const undo of restore.reverse()) undo();
      if (!engaged) return;
      if (stubs.length > maxStubs) {
        warn('overflow', `more than ${maxStubs} request stubs; no route-stub record is published`);
        return;
      }
      if (testInfo.annotations.some(a => a && a.type === annotationType)) {
        // The producer's own record is still added: two annotations are refused by the consumer, so
        // the attempt reads as NOT_PRODUCED instead of trusting the foreign record.
        warn('foreign', `a ${annotationType} annotation was published outside the producer; the attempt's record is refused`);
      }
      const record = stubs.map(({ target, api, kind, pattern, hits, file, line, unrouted }) => ({ target, ...(api ? { api } : {}), kind, pattern, hits, ...(file ? { file, line } : {}), ...(unrouted ? { unrouted: true } : {}) }));
      testInfo.annotations.push({ type: annotationType, description: JSON.stringify({ stubs: record }) });
    },
  };
}

// active is the session of the test running in this worker; a worker runs one test at a time.
let active = null;

// observeBrowser hands every context the browser creates while a test runs to that test's session,
// and returns the undo that restores the browser instance.
function observeBrowser(browser) {
  const restore = [];
  const newContext = browser && browser.newContext;
  const newPage = browser && browser.newPage;
  if (typeof newContext === 'function') {
    override(restore, browser, 'newContext', async function (...args) {
      // The session is taken before the await, so a context that resolves after its test ended is
      // refused by that session's closed guard instead of adopted by the next test.
      const session = active;
      const context = await newContext.apply(this, args);
      if (session) session.adopt(context);
      return context;
    });
  }
  if (typeof newPage === 'function') {
    override(restore, browser, 'newPage', async function (...args) {
      const session = active;
      const page = await newPage.apply(this, args);
      if (session && page && typeof page.context === 'function') session.adopt(page.context());
      return page;
    });
  }
  return () => { for (const undo of restore.reverse()) undo(); };
}

// extend adds an automatic test fixture holding the attempt's session, overrides the `context`
// fixture so its routes are recorded, and overrides the worker `browser` fixture so contexts the
// test creates with browser.newContext() or browser.newPage() are recorded too. `warn(message)`
// replaces the once-per-kind stderr warning.
function extend(baseTest, { warn: sink } = {}) {
  if (!baseTest || typeof baseTest.extend !== 'function') {
    throw new TypeError('corvint route-stubs: extend needs a Playwright test');
  }
  const warn = warner(typeof sink === 'function' ? sink : message => process.stderr.write(`corvint route-stubs: ${message}\n`));
  return baseTest.extend({
    _corvintRouteStubs: [async ({}, use, testInfo) => {
      const observed = session({ testInfo, warn });
      const previous = active;
      active = observed;
      try {
        await use(observed);
      } finally {
        if (active === observed) active = previous;
        observed.finish();
      }
    }, { auto: true }],
    context: async ({ context, _corvintRouteStubs }, use) => {
      _corvintRouteStubs.adopt(context);
      await use(context);
    },
    browser: [async ({ browser }, use) => {
      const undo = observeBrowser(browser);
      try {
        await use(browser);
      } finally {
        undo();
      }
    }, { scope: 'worker' }],
  });
}

module.exports = { extend, annotationType, maxStubs, _internal: { callSite, clip, describe, frameLocation, observeBrowser, sameURL, session } };
