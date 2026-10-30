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
// page content, typed values or call parameters other than the selector, and it redacts URLs before
// they reach disk. The runner process removes the previous run's documents from outputDir.
const fs = require('node:fs');
const path = require('node:path');

const schema = 'playwright-navigation-events/0';
const maxEvents = 4096;
const maxURLBytes = 4096;
const maxSelectorBytes = 1024;
const maxDocumentBytes = 4 << 20; // observe's reporter file limit
const docPrefix = '{"schema":"' + schema + '"';
const tmpSuffix = '.corvint-navigation-events.tmp'; // marks a temporary file as this producer's
const methodPattern = /^[A-Za-z][A-Za-z0-9]{0,63}$/;
const testKeyPattern = /^[^\s\p{Cc}]{1,256}$/u;

// The ANL-V0-007 redaction observe applies, applied before writing so no secret-shaped query value
// or URL credential is persisted (ANL-V0-044). It mirrors how observe's Go code parses the URL:
// byte-wise unescaping, Unicode simple case folding, url.Query dropping a pair holding ';' or a bad
// escape, a hash-route query decoded once with the fragment and replaced by `_=REDACTED` when any of
// its pairs is bad, and a URL url.Parse refuses read as REDACTED.
const secretKey = /token|secret|passw|pwd|auth|session|sig|key|code|credential|bearer|jwt|cookie/iu;
const opaqueValue = /^[A-Za-z0-9_\-+/=.]{24,}$/;
const redacted = 'REDACTED';
const isHex = b => (b >= 0x30 && b <= 0x39) || (b >= 0x41 && b <= 0x46) || (b >= 0x61 && b <= 0x66);

// unescape decodes %XX escapes byte-wise like Go's url unescaping; a bad escape throws.
function unescape(bytes, plusIsSpace) {
  const out = [];
  for (let i = 0; i < bytes.length; i++) {
    const b = bytes[i];
    if (b === 0x25) {
      if (i + 2 >= bytes.length || !isHex(bytes[i + 1]) || !isHex(bytes[i + 2])) throw new Error('bad escape');
      out.push(parseInt(String.fromCharCode(bytes[i + 1], bytes[i + 2]), 16));
      i += 2;
    } else {
      out.push(plusIsSpace && b === 0x2b ? 0x20 : b);
    }
  }
  return Buffer.from(out);
}

// redactQuery redacts a query given as a binary string (one char per byte). It returns null when
// a pair holds ';' or a bad escape and strict is set; otherwise such a pair is dropped.
function redactQuery(query, strict) {
  const kept = [];
  for (const pair of query.split('&')) {
    if (pair === '') continue;
    const eq = pair.indexOf('=');
    const k = eq < 0 ? pair : pair.slice(0, eq);
    const v = eq < 0 ? '' : pair.slice(eq + 1);
    let key, value;
    try {
      if (pair.includes(';')) throw new Error('semicolon');
      key = unescape(Buffer.from(k, 'latin1'), true).toString('utf8');
      value = unescape(Buffer.from(v, 'latin1'), true).toString('utf8');
    } catch {
      if (strict) return null;
      continue;
    }
    kept.push(secretKey.test(key) || opaqueValue.test(value) ? k + '=' + redacted : pair);
  }
  return kept.join('&');
}

// escapeFragment writes fragment bytes so that one unescape returns exactly them.
function escapeFragment(bytes) {
  let out = '';
  for (const b of bytes) {
    out += b === 0x25 || b < 0x21 || b >= 0x7f ? '%' + b.toString(16).toUpperCase().padStart(2, '0') : String.fromCharCode(b);
  }
  return out;
}

// bareToken reports whether a fragment without '?' or '=' is an opaque token rather than a route.
function bareToken(decoded) {
  return !decoded.startsWith('/') && opaqueValue.test(decoded);
}

// redactBareFragment redacts a fragment holding no decoded '?' (ANL-V0-073). An OAuth implicit-flow
// callback carries credentials as a bare fragment query (#access_token=...): one holding '=' is
// redacted like a hash-route query, any bad pair replacing it with `_=REDACTED`, and an opaque
// token becomes `REDACTED`. An unchanged fragment, such as a route, keeps its original bytes.
function redactBareFragment(fragment, decoded) {
  if (!decoded.includes('=')) return bareToken(decoded) ? redacted : fragment;
  const query = redactQuery(decoded, true);
  const rewritten = query === null ? '_=' + redacted : query;
  return rewritten === decoded ? fragment : escapeFragment(Buffer.from(rewritten, 'latin1'));
}

// redactRawQuery redacts the URL's own query, keeping every other byte (url.Query semantics).
function redactRawQuery(query) {
  return Buffer.from(redactQuery(Buffer.from(query, 'utf8').toString('latin1'), false), 'latin1').toString('utf8');
}

function redactURL(raw) {
  let url = raw.replace(/^([A-Za-z][A-Za-z0-9+.-]*:\/\/)[^/?#]*@/, '$1');
  const hash = url.indexOf('#');
  const fragment = hash < 0 ? null : url.slice(hash + 1);
  if (hash >= 0) url = url.slice(0, hash);
  const q = url.indexOf('?');
  const path = q < 0 ? url : url.slice(0, q);
  try {
    unescape(Buffer.from(path, 'utf8'), false); // url.Parse checks the path
    if (fragment === null) {
      return q < 0 ? url : url.slice(0, q + 1) + redactRawQuery(url.slice(q + 1));
    }
    const decoded = unescape(Buffer.from(fragment, 'utf8'), false).toString('latin1');
    if (q >= 0) url = url.slice(0, q + 1) + redactRawQuery(url.slice(q + 1));
    const fq = decoded.indexOf('?');
    if (fq < 0) return url + '#' + redactBareFragment(fragment, decoded);
    const query = redactQuery(decoded.slice(fq + 1), true);
    const rewritten = decoded.slice(0, fq + 1) + (query === null ? '_=' + redacted : query);
    // Like Go's EscapedFragment, an unchanged fragment keeps its original encoding byte for byte.
    if (rewritten === decoded) return url + '#' + fragment;
    // observe matches screens on the route's literal bytes, so only the query is re-escaped. A route
    // whose first decoded '?' is not its first literal '?' cannot keep its bytes: null withholds it.
    const rq = fragment.indexOf('?');
    if (rq < 0 || unescape(Buffer.from(fragment.slice(0, rq), 'utf8'), false).toString('latin1') !== decoded.slice(0, fq)) return null;
    return url + '#' + fragment.slice(0, rq + 1) + escapeFragment(Buffer.from(rewritten.slice(fq + 1), 'latin1'));
  } catch {
    // url.Parse refuses a bad escape, so observe writes the visit as REDACTED but still matches the
    // screen on the raw path and hash route. Only those bytes are kept, and a trailing bad escape
    // keeps the URL unparseable; a route with an encoded '?' may hold query bytes and is withheld.
    // An emptied query keeps its '?', which decides whether the hash prefix applies.
    const base = q < 0 ? path : path + '?';
    if (fragment === null) return base;
    const route = fragment.slice(0, fragment.search(/[?#]|$/));
    // A route that is key=value pairs or an opaque token may be a credential: withheld (ANL-V0-073).
    if (route.includes('=') || bareToken(route)) return null;
    return /%3f/i.test(route) ? null : base + '#' + route + '?%';
  }
}

// clearPrevious removes earlier producer documents and this producer's interrupted temporary files
// directly in dir, so a run's directory holds only that run's tests. Other files, including another
// tool's *.json.tmp, are never touched (ANL-V0-046).
function clearPrevious(dir) {
  let names;
  try {
    names = fs.readdirSync(dir);
  } catch {
    return;
  }
  for (const name of names) {
    const full = path.join(dir, name);
    try {
      if (name.endsWith('.json' + tmpSuffix)) {
        if (fs.lstatSync(full).isFile()) fs.rmSync(full, {force: true});
        continue;
      }
      // An earlier producer wrote `.json.tmp`; such a file is its own only when it holds docPrefix.
      if (!(name.endsWith('.json') || name.endsWith('.json.tmp')) || !fs.lstatSync(full).isFile()) continue;
      const fd = fs.openSync(full, 'r');
      const head = Buffer.alloc(docPrefix.length);
      const n = fs.readSync(fd, head, 0, head.length, 0);
      fs.closeSync(fd);
      if (head.subarray(0, n).toString('utf8') === docPrefix) fs.rmSync(full, {force: true});
    } catch {
      // A file that vanished or cannot be read is left alone.
    }
  }
}

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
      // The committed URL is bounded before redaction, as the trace zip reader bounds it, and the
      // written URL after it.
      const committedBytes = Buffer.byteLength(ev.url);
      ev.url = redactURL(ev.url);
      if (ev.url === null) {
        s.opaque = true;
        return;
      }
      if (committedBytes > maxURLBytes || Buffer.byteLength(ev.url) > maxURLBytes || s.events.length >= maxEvents) {
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
    if (s.closed) s.reopened = true; // a second context of this attempt closed after the first
    const doc = s.pages > 1 || s.overflow || s.opaque || s.reopened ? '' : JSON.stringify({schema, test_key: testId, events: s.events}) + '\n';
    // Only the attempt's retry and page count outlive its write; the event buffer is released.
    s.events = [];
    s.closed = true;
    const reason = s.reopened ? 'more than one browser context' : s.pages > 1 ? 'more than one page'
      : s.overflow ? 'more than 4096 events or a URL over 4096 bytes'
        : s.opaque ? "a hash route with an encoded '?' and a redacted query"
        : Buffer.byteLength(doc) > maxDocumentBytes ? 'document over 4 MiB' : '';
    if (reason) {
      fs.rmSync(target, {force: true});
      warn(`corvint navigation-events: ${testId} not written: ${reason}`);
      return;
    }
    fs.mkdirSync(dir, {recursive: true});
    const tmp = target + tmpSuffix;
    fs.writeFileSync(tmp, doc);
    fs.renameSync(tmp, target);
  }

  function flush(context) {
    const testId = contexts.get(context);
    if (testId === undefined) return;
    contexts.delete(context);
    write(testId);
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
        // A new attempt drops the earlier attempt's document before recording its own.
        s = {retry: t.retry, events: [], pages: 0, overflow: false, opaque: false, last: ''};
        tests.set(t.testId, s);
        fs.rmSync(file(t.testId), {force: true});
      }
      contexts.set(context, t.testId);
      for (const page of context.pages()) watchPage(s, page);
      context.on('page', page => watchPage(s, page));
      // Browser.close() and a browser crash close the context without the before-close hook.
      context.on('close', () => flush(context));
    },
    async runBeforeCloseBrowserContext(context) {
      flush(context);
    },
  };
}

// install wires the recorder into the suite's own Playwright copy, resolved from the working
// directory. When the private instrumentation hook is missing it warns once and records nothing.
function install({outputDir} = {}) {
  if (typeof outputDir !== 'string' || outputDir === '') throw new Error('navigation-events: outputDir is required');
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
  // Playwright sets TEST_WORKER_INDEX in workers; the runner loads the config once before them.
  if (!process.env.TEST_WORKER_INDEX) clearPrevious(path.resolve(outputDir));
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

module.exports = {install, createRecorder, clearPrevious, redactURL, schema};
