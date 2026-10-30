'use strict';
// ANL-V0-037..044: drives the navigation-events producer with synthetic Playwright instrumentation
// callbacks and page events. Node built-ins only; no Playwright install is needed.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {EventEmitter} = require('node:events');
const {createRecorder, install, clearPrevious, redactURL} = require('./navigation-events.cjs');

class Frame { constructor() { this.href = 'about:blank'; } url() { return this.href; } }
class Page extends EventEmitter { constructor() { super(); this.main = new Frame(); } mainFrame() { return this.main; } }
class Context extends EventEmitter { pages() { return []; } }
class Request {
  constructor(page, url, from) { this.page = page; this.href = url; this.from = from; }
  url() { return this.href; }
  redirectedFrom() { return this.from || null; }
  isNavigationRequest() { return true; }
  frame() { return this.page.mainFrame(); }
}

// navigate emits the document request chain (each HTTP hop, then the final URL) and the commit.
function navigate(page, url, hops = [], frame = page.mainFrame()) {
  let req = null;
  for (const u of [...hops, url]) {
    req = new Request(page, u.split('#')[0], req);
    page.emit('request', req);
  }
  frame.href = url;
  page.emit('framenavigated', frame);
}

function sameDocument(page, url) {
  page.mainFrame().href = url;
  page.emit('framenavigated', page.mainFrame());
}

function harness(testId = 'synthetic-walk', retry = 0) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-nav-'));
  const warnings = [];
  const current = {testId, retry};
  const rec = createRecorder({outputDir: dir, currentTest: () => current, warn: m => warnings.push(m)});
  const call = async (method, params, fn) => {
    const data = {};
    rec.onApiCallBegin(data, {type: 'Frame', method, params});
    try { await fn?.(); } finally { rec.onApiCallEnd(data); }
  };
  const open = async () => {
    const context = new Context();
    await rec.runAfterCreateBrowserContext(context);
    const page = new Page();
    context.emit('page', page);
    return {context, page};
  };
  const read = id => fs.readFileSync(path.join(dir, (id || testId) + '.json'), 'utf8');
  return {dir, rec, call, open, read, warnings, current};
}

test('ANL-V0-039..041: a walk writes the closed document byte-for-byte', async () => {
  const h = harness();
  const {context, page} = await h.open();
  const base = 'http://127.0.0.1:4817';
  await h.call('goto', {url: base + '/login?token=zz'}, () => navigate(page, base + '/login?token=zz'));
  await h.call('fill', {selector: '#user', value: 'typed-secret'});
  await h.call('click', {selector: '#enter'}, () => navigate(page, base + '/home'));
  await h.call('click', {selector: '#item'}, () => { navigate(page, base + '/items/7'); sameDocument(page, base + '/items/7#tab'); });
  await h.call('goBack', {}, () => navigate(page, base + '/home'));
  await h.call('__waitInfo__', {}, () => navigate(page, base + '/login'));
  const child = new Frame();
  navigate(page, base + '/frame', [], child);
  await h.call('reload', {}, () => navigate(page, base + '/login'));
  await h.call('goto', {url: base + '/old'}, () => navigate(page, base + '/home#top', [base + '/old', base + '/older']));
  await h.call('click', {selector: 'x'.repeat(1025)}, () => navigate(page, base + '/items/8'));
  await h.rec.runBeforeCloseBrowserContext(context);
  const golden = fs.readFileSync(path.join(__dirname, 'testdata/navigation-events/synthetic.json'), 'utf8');
  assert.equal(h.read(), golden);
  assert.ok(!h.read().includes('typed-secret'));
  assert.ok(!h.read().includes('token=zz'));
  assert.deepEqual(h.warnings, []);
});

test('ANL-V0-038: listener state is per attempt; a retry replaces the earlier file', async () => {
  const h = harness('k', 0);
  let {context, page} = await h.open();
  await h.call('goto', {}, () => navigate(page, 'https://x.test/a'));
  await h.rec.runBeforeCloseBrowserContext(context);
  h.current.retry = 1;
  ({context, page} = await h.open());
  await h.call('goto', {}, () => navigate(page, 'https://x.test/b'));
  await h.rec.runBeforeCloseBrowserContext(context);
  assert.equal(JSON.parse(h.read('k')).events.map(e => e.url).join(), 'https://x.test/b');
  h.current.retry = 2;
  await h.open();
  assert.ok(!fs.existsSync(path.join(h.dir, 'k.json')), 'a new attempt drops the earlier document');
});

test('ANL-V0-039: a context closed by Browser.close() still writes once', async () => {
  const h = harness('browser-close');
  const {context, page} = await h.open();
  await h.call('goto', {}, () => navigate(page, 'https://x.test/a'));
  context.emit('close');
  assert.equal(JSON.parse(h.read()).events.length, 1);
  fs.rmSync(path.join(h.dir, 'browser-close.json'));
  await h.rec.runBeforeCloseBrowserContext(context);
  assert.ok(!fs.existsSync(path.join(h.dir, 'browser-close.json')), 'already flushed');
});

test('ANL-V0-043: a second browser context in one attempt withholds; buffers are released', async () => {
  const h = harness('two-contexts');
  const a = await h.open();
  await h.call('goto', {}, () => navigate(a.page, 'https://x.test/a'));
  const b = new Context();
  await h.rec.runAfterCreateBrowserContext(b);
  await h.rec.runBeforeCloseBrowserContext(a.context);
  assert.equal(JSON.parse(h.read()).events.length, 1);
  await h.rec.runBeforeCloseBrowserContext(b);
  assert.ok(!fs.existsSync(path.join(h.dir, 'two-contexts.json')));
  assert.match(h.warnings[0], /two-contexts not written: more than one browser context/);
});

test('ANL-V0-044: URLs are redacted before they reach disk', () => {
  assert.equal(redactURL('https://u:p@x.test/a?token=zz&x=1#t'), 'https://x.test/a?token=REDACTED&x=1#t');
  assert.equal(redactURL('http://h/p?a=b&code=1&v=abcdefghijklmnopqrstuvwxyz'), 'http://h/p?a=b&code=REDACTED&v=REDACTED');
  assert.equal(redactURL('http://h/#/r?session=9&q=1'), 'http://h/#/r?session=REDACTED&q=1');
  assert.equal(redactURL('http://h/p?a=1;token=zz&b=2&q=%zz'), 'http://h/p?b=2');
  assert.equal(redactURL('http://h/#!/home?%2574oken=sekret1&a=1'), 'http://h/#!/home?%2574oken=REDACTED&a=1');
  // A redacted hash query keeps the route's literal bytes; an encoded '?' in the route cannot.
  assert.equal(redactURL('http://h/#!/%68ome?token=sekret'), 'http://h/#!/%68ome?token=REDACTED');
  assert.equal(redactURL('http://h/#/r%3Fsession%3D9'), null);
  // An unparseable URL keeps only the path and hash route observe matches on, and stays unparseable.
  assert.equal(redactURL('http://h/#/r?q=%zz&token=zz'), 'http://h/#/r?%');
  assert.equal(redactURL('http://u:p@h/a%zz?token=zz#/r?x=1'), 'http://h/a%zz?#/r?%');
  assert.equal(redactURL('http://h/?x=1#!/home?q=%zz'), 'http://h/?#!/home?%');
  assert.equal(redactURL('http://h/#/r%3Ftoken%3Dzz%zz'), null);
  assert.equal(redactURL('http://h/#!/%68ome?x=1'), 'http://h/#!/%68ome?x=1');
  assert.equal(redactURL('http://h/#!/home?x=%41'), 'http://h/#!/home?x=%41');
  assert.equal(redactURL('http://h/items/7#tab'), 'http://h/items/7#tab');
});

test('ANL-V0-073: a bare fragment query or token is redacted; a route keeps its bytes', () => {
  assert.equal(redactURL('https://x.test/cb#access_token=SECRETVALUE1&state=1'), 'https://x.test/cb#access_token=REDACTED&state=1');
  assert.equal(redactURL('https://x.test/cb#' + 'c'.repeat(40)), 'https://x.test/cb#REDACTED');
  assert.equal(redactURL('https://x.test/cb#a=1;b=SECRETVALUE2'), 'https://x.test/cb#_=REDACTED');
  assert.equal(redactURL('https://x.test/cb#access_token=SECRETVALUE3%zz'), null);
  assert.equal(redactURL('https://x.test/#/home'), 'https://x.test/#/home');
  assert.equal(redactURL('https://x.test/#/settings/profile/security-keys'), 'https://x.test/#/settings/profile/security-keys');
  assert.equal(redactURL('https://x.test/#/r=1'), 'https://x.test/#/r=1');
});

test('ANL-V0-046: the runner clears only earlier producer documents', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-nav-'));
  fs.writeFileSync(path.join(dir, 'old.json'), '{"schema":"playwright-navigation-events/0","test_key":"old","events":[]}\n');
  fs.writeFileSync(path.join(dir, 'half.json.corvint-navigation-events.tmp'), '{');
  fs.writeFileSync(path.join(dir, 'legacy.json.tmp'), '{"schema":"playwright-navigation-events/0","test_key":"l"');
  fs.writeFileSync(path.join(dir, 'settings.json.tmp'), '{"theme":"dark"}');
  fs.writeFileSync(path.join(dir, 'other.json'), '{"schema":"something-else"}');
  fs.writeFileSync(path.join(dir, 'notes.txt'), 'keep');
  clearPrevious(dir);
  assert.deepEqual(fs.readdirSync(dir).sort(), ['notes.txt', 'other.json', 'settings.json.tmp']);
  clearPrevious(path.join(dir, 'missing'));
});

test('ANL-V0-043: a second page, too many events, an oversized URL or document withholds the document', async () => {
  const h = harness('two-pages');
  const {context, page} = await h.open();
  await h.call('goto', {}, () => navigate(page, 'https://x.test/a'));
  context.emit('page', new Page());
  await h.rec.runBeforeCloseBrowserContext(context);
  assert.ok(!fs.existsSync(path.join(h.dir, 'two-pages.json')));
  assert.match(h.warnings[0], /two-pages not written: more than one page/);

  const big = harness('big');
  const b = await big.open();
  for (let i = 0; i <= 4096; i++) navigate(b.page, 'https://x.test/p' + i);
  await big.rec.runBeforeCloseBrowserContext(b.context);
  assert.ok(!fs.existsSync(path.join(big.dir, 'big.json')));

  const huge = harness('huge');
  const g = await huge.open();
  for (let i = 0; i < 1100; i++) navigate(g.page, 'https://x.test/' + i + 'a'.repeat(4000));
  await huge.rec.runBeforeCloseBrowserContext(g.context);
  assert.ok(!fs.existsSync(path.join(huge.dir, 'huge.json')));
  assert.match(huge.warnings[0], /huge not written: document over 4 MiB/);

  const long = harness('long');
  const l = await long.open();
  navigate(l.page, 'https://x.test/' + 'a'.repeat(4096));
  await long.rec.runBeforeCloseBrowserContext(l.context);
  assert.ok(!fs.existsSync(path.join(long.dir, 'long.json')));
  assert.match(long.warnings[0], /long not written/);

  // A committed URL over 4096 bytes is withheld even when redaction would shorten it.
  const shrunk = harness('shrunk');
  const r = await shrunk.open();
  navigate(r.page, 'http://h/home?token=' + 't'.repeat(5000));
  await shrunk.rec.runBeforeCloseBrowserContext(r.context);
  assert.ok(!fs.existsSync(path.join(shrunk.dir, 'shrunk.json')));
  assert.match(shrunk.warnings[0], /shrunk not written/);

  // So is a redirect hop over 4096 bytes.
  const hop = harness('hop');
  const o = await hop.open();
  navigate(o.page, 'http://h/home', ['http://h/old?code=' + 'c'.repeat(5000)]);
  await hop.rec.runBeforeCloseBrowserContext(o.context);
  assert.ok(!fs.existsSync(path.join(hop.dir, 'hop.json')));

  // So is a hash route whose redaction cannot keep its literal bytes.
  const opaque = harness('opaque');
  const q = await opaque.open();
  navigate(q.page, 'http://h/#/r%3Fsession%3D9');
  await opaque.rec.runBeforeCloseBrowserContext(q.context);
  assert.ok(!fs.existsSync(path.join(opaque.dir, 'opaque.json')));
  assert.match(opaque.warnings[0], /opaque not written: a hash route/);
});

test('ANL-V0-037/038: outside a test nothing is recorded; outputDir is required', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-nav-'));
  const rec = createRecorder({outputDir: dir, currentTest: () => null});
  const context = new Context();
  await rec.runAfterCreateBrowserContext(context);
  await rec.runBeforeCloseBrowserContext(context);
  assert.deepEqual(fs.readdirSync(dir), []);
  assert.throws(() => createRecorder({currentTest: () => null}), /outputDir is required/);
  assert.throws(() => install({}), /outputDir is required/);
});

test('ANL-V0-038: without a resolvable Playwright install warns once and records nothing', () => {
  const cwd = process.cwd();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'corvint-nav-'));
  const write = process.stderr.write;
  let seen = '';
  process.chdir(dir);
  process.stderr.write = chunk => { seen += chunk; return true; };
  try {
    install({outputDir: 'out'});
  } finally {
    process.stderr.write = write;
    process.chdir(cwd);
  }
  assert.match(seen, /Playwright not found/);
  assert.ok(!fs.existsSync(path.join(dir, 'out')));
});
