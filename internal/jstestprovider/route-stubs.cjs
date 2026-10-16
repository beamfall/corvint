'use strict';
// Corvint route-stubs producer (LPCV-V0-072). It records every request stub a test registers
// through `page.route` or `context.route` on the test's `context` fixture: the stub's target, URL
// pattern kind and text, how often its handler ran in the attempt, and the file and line that
// registered it. At fixture teardown it publishes one `corvint-route-stubs` annotation whose
// description is the closed JSON record `{"stubs":[...]}` that `ParsePlaywrightJSON` reads into
// the attempt's `routeStubs`. Install it in a fixtures module:
//
//   const { test: base } = require('@playwright/test');
//   const test = require('/path/to/corvint/internal/jstestprovider/route-stubs.cjs').extend(base);
//
// The producer is optional and additive. A test that never takes `context` or `page` publishes no
// record, and neither does an attempt with more than 64 stubs; the consumer reads a missing record
// as NOT_PRODUCED, never as zero hits. Routes on contexts the test creates itself
// (`browser.newContext()`) and `routeWebSocket` are not observed.
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
  let site = line.trim().replace(/^at\s+/, '');
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
// test, hook or helper line that registered the stub.
function callSite(rootDir) {
  const limit = Error.stackTraceLimit;
  Error.stackTraceLimit = 32;
  const stack = new Error().stack || '';
  Error.stackTraceLimit = limit;
  for (const line of stack.split('\n').slice(1)) {
    const site = frameLocation(line);
    if (!site || site.file === __filename || /[\\/]node_modules[\\/]/.test(site.file)) continue;
    if (site.line < 1 || site.line > 10_000_000) return {};
    let file = site.file;
    if (rootDir) {
      const relative = path.relative(rootDir, file);
      if (relative && !relative.startsWith('..') && !path.isAbsolute(relative)) file = relative;
    }
    return { file: clip(file.split(path.sep).join('/')), line: site.line };
  }
  return {};
}

function warner(sink) {
  const seen = new Set();
  return (key, message) => {
    if (seen.has(key)) return;
    seen.add(key);
    sink(message);
  };
}

// session instruments one attempt's context and its pages.
function session({ context, testInfo, warn }) {
  const rootDir = testInfo && testInfo.config && typeof testInfo.config.rootDir === 'string' ? testInfo.config.rootDir : '';
  const stubs = [];
  const restore = [];
  const pages = new WeakSet();
  let closed = false;

  // override replaces one method on the instance; undo restores an own property the instance
  // already had (another fixture's override) and otherwise deletes the producer's own.
  const override = (target, name, replacement) => {
    const own = Object.getOwnPropertyDescriptor(target, name);
    target[name] = replacement;
    restore.push(() => {
      if (own) Object.defineProperty(target, name, own);
      else delete target[name];
    });
  };

  // instrument replaces target.route and target.unroute on the instance. Each handler is wrapped
  // so a call counts as a hit; unroute(url, handler) still finds the wrappers of that handler.
  const instrument = (target, kind) => {
    const route = target.route;
    const unroute = target.unroute;
    if (typeof route !== 'function') {
      warn(`no-route-${kind}`, `${kind}.route is absent in this Playwright; its stubs are not recorded`);
      return;
    }
    const wrappers = new Map();
    override(target, 'route', function (url, handler, options) {
      if (typeof handler !== 'function') return route.call(this, url, handler, options);
      const stub = { target: kind, ...describe(url), hits: 0, ...callSite(rootDir) };
      stubs.push(stub);
      const wrapper = function (...args) {
        if (stub.hits < maxHits) stub.hits += 1;
        return handler.apply(this, args);
      };
      if (!wrappers.has(handler)) wrappers.set(handler, []);
      wrappers.get(handler).push(wrapper);
      return route.call(this, url, wrapper, options);
    });
    if (typeof unroute === 'function') {
      // unroute(url, handler) removes the producer's wrappers of handler and then any registration
      // of handler itself made before instrumentation.
      override(target, 'unroute', async function (url, handler) {
        if (typeof handler === 'function' && wrappers.has(handler)) {
          for (const wrapper of wrappers.get(handler)) await unroute.call(this, url, wrapper);
        }
        return unroute.call(this, url, handler);
      });
    }
  };

  const track = page => {
    if (closed || !page || pages.has(page)) return;
    pages.add(page);
    instrument(page, 'page');
  };
  instrument(context, 'context');
  const newPage = context.newPage;
  if (typeof newPage === 'function') {
    override(context, 'newPage', async function (...args) {
      const page = await newPage.apply(this, args);
      track(page);
      return page;
    });
  }
  if (typeof context.on === 'function') {
    context.on('page', track);
    restore.push(() => context.off('page', track));
  }
  if (typeof context.pages === 'function') for (const page of context.pages()) track(page);

  return {
    finish() {
      closed = true;
      for (const undo of restore.reverse()) undo();
      if (stubs.length > maxStubs) {
        warn('overflow', `more than ${maxStubs} request stubs; no route-stub record is published`);
        return;
      }
      if (testInfo.annotations.some(a => a && a.type === annotationType)) {
        // The producer's own record is still added: two annotations are refused by the consumer, so
        // the attempt reads as NOT_PRODUCED instead of trusting the foreign record.
        warn('foreign', `a ${annotationType} annotation was published outside the producer; the attempt's record is refused`);
      }
      const record = stubs.map(({ target, kind, pattern, hits, file, line }) => (file ? { target, kind, pattern, hits, file, line } : { target, kind, pattern, hits }));
      testInfo.annotations.push({ type: annotationType, description: JSON.stringify({ stubs: record }) });
    },
  };
}

// extend overrides the `context` fixture of the project's base test so every page.route and
// context.route on it is recorded. `warn(message)` replaces the once-per-kind stderr warning.
function extend(baseTest, { warn: sink } = {}) {
  if (!baseTest || typeof baseTest.extend !== 'function') {
    throw new TypeError('corvint route-stubs: extend needs a Playwright test');
  }
  const warn = warner(typeof sink === 'function' ? sink : message => process.stderr.write(`corvint route-stubs: ${message}\n`));
  return baseTest.extend({
    context: async ({ context }, use, testInfo) => {
      const observed = session({ context, testInfo, warn });
      try {
        await use(context);
      } finally {
        observed.finish();
      }
    },
  });
}

module.exports = { extend, annotationType, maxStubs, _internal: { clip, describe, frameLocation, session } };
