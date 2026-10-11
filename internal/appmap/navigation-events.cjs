'use strict';
// Corvint navigation-events producer (ANL-V0-037..046). It writes one closed
// `playwright-navigation-events/0` document per test for `corvint flows appmap observe --trace DIR`.
// Install it with one line in playwright.config; no spec file changes:
//
//   require('/path/to/corvint/internal/appmap/navigation-events.cjs').install({outputDir: 'corvint-navigation'});
//
// The config file is loaded by the runner and by every worker. In a worker the producer adds a
// listener to Playwright's client instrumentation, so it runs with any trace setting and records
// only main-frame URL commits and the Playwright call in flight when each committed. It never reads
// page content, typed values or call parameters other than the selector.
const fs = require('node:fs');
const path = require('node:path');

const schema = 'playwright-navigation-events/0';
const maxEvents = 4096;
const maxURLBytes = 4096;
const maxSelectorBytes = 1024;
const methodPattern = /^[A-Za-z][A-Za-z0-9]{0,63}$/;
const testKeyPattern = /^[^\s\p{Cc}]{1,256}$/u;

// createRecorder returns the instrumentation listener. currentTest() returns the running test's
// {testId, retry} or null; warn receives one-line notices. Exported for tests.
function createRecorder({outputDir, currentTest, warn = () => {}}) {
  if (typeof outputDir !== 'string' || outputDir === '') throw new Error('navigation-events: outputDir is required');
  const dir = path.resolve(outputDir);
  const tests = new Map(); // testId -> {retry, events, pages, overflow, last}
  const contexts = new Map(); // context -> testId
  let inFlight = []; // Playwright calls begun and not yet ended: {method, selector, committed}

  function file(testId) { return path.join(dir, testId + '.json'); }

  // onCommit records one main-frame URL commit. req is the page's latest main-frame document
  // request; its redirect chain, when it ended at this URL, becomes the leading events.
  function onCommit(s, url, req) {
    if (typeof url !== 'string' || url === s.last) return;
    s.last = url;
    if (url.startsWith('about:')) return;
    const hops = [];
    if (req && req.url() === url.split('#')[0]) {
      for (let r = req.redirectedFrom(); r && hops.length < maxEvents; r = r.redirectedFrom()) hops.unshift(r.url());
    }
    const call = inFlight.length ? inFlight[inFlight.length - 1] : null;
    const first = {kind: 'navigate', url: hops.length ? hops[0] : url};
    if (call && !call.committed) {
      call.committed = true;
      first.action = call.selector === undefined ? {kind: call.method} : {kind: call.method, selector: call.selector};
    }
    const add = [first, ...hops.slice(1).map(u => ({kind: 'redirect', url: u}))];
    if (hops.length) add.push({kind: 'redirect', url});
    for (const ev of add) {
      if (Buffer.byteLength(ev.url) > maxURLBytes || s.events.length >= maxEvents) {
        s.overflow = true;
        return;
      }
      s.events.push(ev);
    }
  }

  function watchPage(s, page) {
    s.pages++;
    if (s.pages > 1) return;
    let navRequest = null;
    page.on('request', req => {
      if (req.isNavigationRequest() && req.frame() === page.mainFrame()) navRequest = req;
    });
    page.on('framenavigated', frame => {
      if (frame !== page.mainFrame()) return;
      const req = navRequest;
      navRequest = null;
      onCommit(s, frame.url(), req);
    });
  }

  function write(testId) {
    const s = tests.get(testId);
    if (!s) return;
    const target = file(testId);
    if (s.pages > 1 || s.overflow) {
      fs.rmSync(target, {force: true});
      warn(`corvint navigation-events: ${testId} not written: ${s.pages > 1 ? 'more than one page' : 'more than 4096 events or a URL over 4096 bytes'}`);
      return;
    }
    fs.mkdirSync(dir, {recursive: true});
    const tmp = target + '.tmp';
    fs.writeFileSync(tmp, JSON.stringify({schema, test_key: testId, events: s.events}) + '\n');
    fs.renameSync(tmp, target);
  }

  return {
    onApiCallBegin(data, channel) {
      if (!channel || !methodPattern.test(channel.method || '')) return;
      const selector = channel.params && typeof channel.params.selector === 'string' && Buffer.byteLength(channel.params.selector) <= maxSelectorBytes ? channel.params.selector : undefined;
      const call = {method: channel.method, selector, committed: false};
      inFlight.push(call);
      if (data && typeof data === 'object') data.__corvintNavCall = call;
    },
    onApiCallEnd(data) {
      const call = data && data.__corvintNavCall;
      if (call) inFlight = inFlight.filter(c => c !== call);
    },
    async runAfterCreateBrowserContext(context) {
      const t = currentTest();
      if (!t || !testKeyPattern.test(t.testId)) return;
      let s = tests.get(t.testId);
      if (!s || s.retry !== t.retry) {
        s = {retry: t.retry, events: [], pages: 0, overflow: false, last: ''};
        tests.set(t.testId, s);
      }
      contexts.set(context, t.testId);
      for (const page of context.pages()) watchPage(s, page);
      context.on('page', page => watchPage(s, page));
    },
    async runBeforeCloseBrowserContext(context) {
      const testId = contexts.get(context);
      if (testId === undefined) return;
      contexts.delete(context);
      write(testId);
    },
  };
}

// install wires the recorder into the suite's own Playwright copy, resolved from the working
// directory. When the private instrumentation hook is missing it warns once and records nothing.
function install({outputDir} = {}) {
  const warn = msg => process.stderr.write(msg + '\n');
  const fromCwd = {paths: [process.cwd()]};
  let core, test;
  try {
    const pw = path.dirname(require.resolve('playwright/package.json', fromCwd));
    core = require(require.resolve('playwright-core', {paths: [pw]}));
    test = require(require.resolve('playwright/test', fromCwd)).test;
  } catch {
    warn('corvint navigation-events: Playwright not found from the working directory; nothing recorded');
    return;
  }
  const inst = core._instrumentation;
  if (!inst || typeof inst.addListener !== 'function') {
    warn('corvint navigation-events: this Playwright has no client instrumentation hook; nothing recorded');
    return;
  }
  const currentTest = () => {
    try {
      const info = test.info();
      return {testId: info.testId, retry: info.retry};
    } catch {
      return null;
    }
  };
  inst.addListener(createRecorder({outputDir, currentTest, warn}));
}

module.exports = {install, createRecorder, schema};
