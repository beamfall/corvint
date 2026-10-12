'use strict';
// Corvint behavior-events producer (DCP-V1-081). It publishes the closed annotations that
// `corvint docs corpus witness-playwright` reads (DCP-V1-049 to DCP-V1-052): one
// `corvint-behavior-event` per observed page commit, passing registered assertion and passing
// negative control, then `corvint-behavior-cleanup`, `corvint-behavior-fixture` and
// `corvint-behavior-role`. Install it in a fixtures module; specs then take the `behavior` fixture:
//
//   const { test: base } = require('@playwright/test');
//   const test = require('/path/to/corvint/internal/doccorpus/behavior-events.cjs').extend(base);
//   test.use({ behaviorFixtures: ['seeded-items'] });
//   test('lists items', async ({ page, behavior }) => {
//     behavior.cleanup(async () => { /* undo the seeded rows */ });
//     await page.goto('/items');
//     await behavior.expect({ id: 'items-visible', behavior: 'list-items', criterion: 'items-visible' })(page.locator('#count')).toHaveText('2');
//     await behavior.negativeControl('empty-hidden')(page.locator('#empty')).toBeHidden();
//   });
//
// The test author never supplies an event, a matcher result or an expect implementation: the
// producer resolves Playwright's own `expect` from the project, runs the matcher itself, and records
// an assertion only after that matcher resolved and the attempt gained no error. The matcher name,
// locator selector and expected value of the event are the call's own arguments, so the event equals
// the registered assertion tuple byte for byte or names a different one; a valueless state matcher
// (toBeVisible, toBeHidden, toBeEnabled, toBeChecked) records its state name as the value
// (DCP-V1-083). Anything the producer cannot state exactly (a RegExp or array expectation, extra
// matcher options, a frame or unobserved-page locator) runs the matcher and records nothing. Page events come only from
// main-frame commits of http(s) URLs in pages of the test's `context` fixture.
//
// Private Playwright fields it reads (qualified on 1.61.1): Locator `_frame` and `_selector`, Page
// `_opener`. When one is absent the producer warns once on stderr and records nothing for that page
// or locator, so a later Playwright release degrades to unwitnessed rather than to a guessed event.
const fs = require('node:fs');
const path = require('node:path');

const maxEvents = 4096; // doccorpus.MaxRecords
const maxTextBytes = 64 << 10; // doccorpus textOK
const prefix = 'corvint-behavior-';
const annotation = {
  event: 'corvint-behavior-event',
  cleanup: 'corvint-behavior-cleanup',
  fixture: 'corvint-behavior-fixture',
  role: 'corvint-behavior-role',
};
// Assertion matchers whose single expected argument is the event value. String matchers need a
// non-empty string; toHaveCount needs a non-negative safe integer, written in decimal.
const assertionMatchers = Object.freeze({
  toHaveText: 'string',
  toContainText: 'string',
  toHaveValue: 'string',
  toHaveClass: 'string',
  toHaveId: 'string',
  toHaveAccessibleName: 'string',
  toHaveAccessibleDescription: 'string',
  toHaveRole: 'string',
  toHaveCount: 'count',
});
// Valueless state matchers (DCP-V1-083): each is recorded with its canonical state name as the
// event value, only when called with no argument or a timeout-only options object. `.not` is not
// offered on behavior.expect, and a state option (visible, enabled, checked, indeterminate) runs
// the matcher and records nothing, so a recorded event always means the positive state held.
const stateMatchers = Object.freeze({
  toBeVisible: 'visible',
  toBeHidden: 'hidden',
  toBeEnabled: 'enabled',
  toBeChecked: 'checked',
});
const negativeMatcher = /^to[A-Z][A-Za-z]{0,63}$/;

// text mirrors doccorpus textOK: non-empty, at most 64 KiB of well-formed UTF-8 without NUL.
function text(value) {
  return typeof value === 'string' && value !== '' && value.isWellFormed() && !value.includes('\u0000') && Buffer.byteLength(value) <= maxTextBytes;
}

function names(value, option) {
  if (!Array.isArray(value) || !value.every(text)) {
    throw new TypeError(`corvint behavior-events: ${option} must be an array of non-empty strings`);
  }
  return value.slice();
}

// resolveExpect loads Playwright's own expect from the project, never from the caller, so no test
// author can hand the producer a matcher that always passes.
function resolveExpect(from) {
  for (const id of ['playwright/test', '@playwright/test']) {
    let resolved;
    try {
      resolved = require.resolve(id, { paths: [from] });
    } catch {
      continue;
    }
    const expect = require(resolved).expect;
    if (typeof expect === 'function') return expect;
  }
  throw new Error(`corvint behavior-events: cannot resolve playwright/test from ${from}`);
}

// event writes one closed BehaviorEvent with the key order of the committed report fixture.
function event(fields) {
  return JSON.stringify({
    browser_context: fields.context,
    page: fields.page,
    frame: 'main',
    navigation: fields.navigation || '',
    parent_page: fields.parent || '',
    behavior: fields.behavior || '',
    criterion: fields.criterion || '',
    matcher: fields.matcher || '',
    locator: fields.locator || '',
    value: fields.value || '',
    passed: true,
    sequence: fields.sequence,
    kind: fields.kind,
    id: fields.id,
  });
}

function httpPath(url) {
  let parsed;
  try {
    parsed = new URL(url);
  } catch {
    return '';
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return '';
  return text(parsed.pathname) ? parsed.pathname : '';
}

// session observes one test attempt. Only its frozen `api` reaches the test; the recorder stays in
// this closure.
function session({ context, testInfo, expect, fixtures, roles, warn }) {
  const contextID = 'context-1';
  const events = [];
  const cleanups = [];
  const pages = new Map();
  const detach = [];
  let pageCount = 0;
  let overflow = false;
  let closed = false;

  const record = fields => {
    if (closed) return;
    if (events.length >= maxEvents) {
      overflow = true;
      return;
    }
    events.push(event({ ...fields, context: contextID, sequence: events.length + 1 }));
  };

  const commit = (state, url, request) => {
    const id = httpPath(url);
    if (id === '' || id === state.last) return;
    let navigation = 'main-frame';
    if (!state.committed && state.parent !== '') navigation = 'popup';
    else if (request && request.url() === url && request.redirectedFrom()) navigation = 'redirect';
    state.committed = true;
    state.last = id;
    record({ page: state.id, navigation, parent: navigation === 'popup' ? state.parent : '', kind: 'page', id });
  };

  const track = page => {
    if (closed || pages.has(page)) return;
    if (!('_opener' in page)) {
      warn('page-opener', 'Page._opener is absent in this Playwright; pages are not observed');
      return;
    }
    let parent = '';
    if (page._opener) {
      const opener = pages.get(page._opener);
      if (!opener) {
        warn('popup-unobserved', 'a popup of an unobserved page is not observed');
        return;
      }
      parent = opener.id;
    }
    const state = { id: `page-${++pageCount}`, parent, committed: false, last: '', request: null };
    pages.set(page, state);
    const onRequest = request => {
      try {
        if (request.isNavigationRequest() && request.frame() === page.mainFrame()) state.request = request;
      } catch {
        // A service-worker navigation has no frame and cannot commit this page.
      }
    };
    const onNavigated = frame => {
      if (frame !== page.mainFrame()) return;
      const request = state.request;
      state.request = null;
      commit(state, frame.url(), request);
    };
    page.on('request', onRequest);
    page.on('framenavigated', onNavigated);
    detach.push(() => {
      page.off('request', onRequest);
      page.off('framenavigated', onNavigated);
    });
    commit(state, page.url(), null);
  };

  // target names the observed page and selector of a main-frame Locator, or why it has none.
  const target = actual => {
    if (actual === null || typeof actual !== 'object' || !('_frame' in actual) || !('_selector' in actual)) {
      return { reason: 'the target is not a Playwright Locator with _frame and _selector' };
    }
    for (const [page, state] of pages) {
      if (page.mainFrame() !== actual._frame) continue;
      if (actual.constructor !== page.locator(':root').constructor) break;
      const selector = actual._selector;
      if (!text(selector) || selector.includes('internal:control=enter-frame')) {
        return { reason: 'the locator selector is not one main-frame selector' };
      }
      return { page: state.id, selector };
    }
    return { reason: 'the locator is not a Locator on the main frame of an observed page' };
  };

  // run executes the genuine matcher and records only after it resolved without a new error.
  const run = async (actual, assertion, invoke, fields) => {
    const where = target(actual);
    const errors = testInfo.errors.length;
    const result = await invoke(assertion);
    if (closed) {
      warn('late', 'a behavior matcher resolved after the test body ended; no event');
    } else if (testInfo.errors.length !== errors) {
      warn('errors', 'the attempt gained an error while a behavior matcher ran; no event');
    } else if (where.reason) {
      warn(where.reason, `${where.reason}; no event`);
    } else if (fields) {
      record({ ...fields, page: where.page, locator: fields.kind === 'assertion' ? where.selector : '' });
    }
    return result;
  };

  // timeoutOnly admits no options or a plain object whose only own enumerable key is timeout.
  const timeoutOnly = options => options === undefined || (options !== null && typeof options === 'object' && !Array.isArray(options) && Object.keys(options).every(key => key === 'timeout'));
  // snapshot is the fresh options object a recorded matcher receives: only an own timeout, so an
  // inherited or non-enumerable option (for example ignoreCase) never reaches the matcher.
  const snapshot = options => (Object.hasOwn(options, 'timeout') ? { timeout: options.timeout } : {});

  const assertionValue = (kind, expected, options) => {
    if (!timeoutOnly(options)) return '';
    if (kind === 'count') return Number.isSafeInteger(expected) && expected >= 0 ? String(expected) : '';
    return text(expected) ? expected : '';
  };

  const api = {
    expect(meta) {
      if (meta === null || typeof meta !== 'object' || !text(meta.id) || !text(meta.behavior) || !text(meta.criterion)) {
        throw new TypeError('corvint behavior-events: behavior.expect needs {id, behavior, criterion} strings');
      }
      const { id, behavior, criterion } = meta;
      return actual => {
        const matchers = {};
        for (const [matcher, kind] of Object.entries(assertionMatchers)) {
          matchers[matcher] = (expected, options) => {
            const value = assertionValue(kind, expected, options);
            if (value === '') warn(`value-${matcher}`, `${matcher} with a non-literal expectation or extra options records no event`);
            const fields = value === '' ? null : { kind: 'assertion', id, behavior, criterion, matcher, value };
            const passed = value === '' || options === undefined ? options : snapshot(options);
            return run(actual, expect(actual), a => a[matcher](...(passed === undefined ? [expected] : [expected, passed])), fields);
          };
        }
        for (const [matcher, value] of Object.entries(stateMatchers)) {
          matchers[matcher] = (...args) => {
            const exact = args.length <= 1 && timeoutOnly(args[0]);
            if (!exact) warn(`state-${matcher}`, `${matcher} with an argument other than a timeout-only options object records no event`);
            const fields = exact ? { kind: 'assertion', id, behavior, criterion, matcher, value } : null;
            const passed = !exact ? args : args[0] === undefined ? [] : [snapshot(args[0])];
            return run(actual, expect(actual), a => a[matcher](...passed), fields);
          };
        }
        return Object.freeze(matchers);
      };
    },
    negativeControl(id) {
      if (!text(id)) throw new TypeError('corvint behavior-events: behavior.negativeControl needs an id string');
      return actual => {
        const wrap = negated => new Proxy(Object.freeze({}), {
          get(_, matcher) {
            if (matcher === 'not' && !negated) return wrap(true);
            if (typeof matcher !== 'string' || !negativeMatcher.test(matcher)) return undefined;
            return (...args) => run(actual, expect(actual), a => (negated ? a.not : a)[matcher](...args), { kind: 'negative-control', id });
          },
        });
        return wrap(false);
      };
    },
    cleanup(fn) {
      if (typeof fn !== 'function') throw new TypeError('corvint behavior-events: behavior.cleanup needs a function');
      if (closed) throw new Error('corvint behavior-events: behavior.cleanup after the test body ended');
      cleanups.push(fn);
    },
  };

  const listen = page => track(page);
  context.on('page', listen);
  detach.push(() => context.off('page', listen));
  for (const page of context.pages()) track(page);

  const finish = async () => {
    closed = true;
    for (const off of detach) off();
    let failure = null;
    for (const fn of cleanups.reverse()) {
      try {
        await fn();
      } catch (error) {
        failure = failure || { error };
      }
    }
    const foreign = testInfo.annotations.some(a => a && typeof a.type === 'string' && a.type.startsWith(prefix));
    if (foreign) warn('foreign', 'corvint-behavior-* annotations were published outside the producer; the events are poisoned');
    if (overflow) warn('overflow', `more than ${maxEvents} behavior events; the events are poisoned`);
    for (const name of fixtures) testInfo.annotations.push({ type: annotation.fixture, description: name });
    for (const name of roles) testInfo.annotations.push({ type: annotation.role, description: name });
    if (foreign || overflow) {
      testInfo.annotations.push({ type: annotation.event, description: 'corvint behavior-events producer refused this attempt' });
    } else {
      for (const description of events) testInfo.annotations.push({ type: annotation.event, description });
    }
    if (cleanups.length > 0) testInfo.annotations.push({ type: annotation.cleanup, description: failure ? 'failed' : 'passed' });
    if (failure) throw failure.error;
  };

  return { api: Object.freeze(api), finish };
}

function warner(sink) {
  const seen = new Set();
  return (key, message) => {
    if (seen.has(key)) return;
    seen.add(key);
    sink(message);
  };
}

// extend adds the `behavior` fixture and the `behaviorFixtures` and `behaviorRoles` options to the
// project's base test. `from` is where Playwright is resolved (default: the working directory);
// `warn(message)` replaces the once-per-kind stderr warning.
function extend(baseTest, { from = process.cwd(), warn: sink } = {}) {
  if (!baseTest || typeof baseTest.extend !== 'function') {
    throw new TypeError('corvint behavior-events: extend needs a Playwright test');
  }
  const expect = resolveExpect(fs.realpathSync(path.resolve(from)));
  const warn = warner(typeof sink === 'function' ? sink : message => process.stderr.write(`corvint behavior-events: ${message}\n`));
  return baseTest.extend({
    behaviorFixtures: [[], { option: true }],
    behaviorRoles: [[], { option: true }],
    behavior: async ({ context, behaviorFixtures, behaviorRoles }, use, testInfo) => {
      const fixtures = names(behaviorFixtures, 'behaviorFixtures');
      const roles = names(behaviorRoles, 'behaviorRoles');
      const observed = session({ context, testInfo, expect, fixtures, roles, warn });
      await use(observed.api);
      await observed.finish();
    },
  });
}

module.exports = { extend };
