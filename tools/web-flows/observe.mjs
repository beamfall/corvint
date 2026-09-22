import { spawn } from 'node:child_process';
import { ownedLifecycle } from './lifecycle.mjs';
import { scan, digest } from './scan.mjs';
import { createRequire } from 'node:module';
const require=createRequire(import.meta.url);

const MAX_BYTES = 5 * 1024 * 1024;
const safeID = /^[A-Za-z][A-Za-z0-9_-]{0,63}$/;
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

async function input() {
  const chunks = [];
  let bytes = 0;
  for await (const chunk of process.stdin) {
    bytes += chunk.length;
    if (bytes > MAX_BYTES + 65536) throw new Error('input-budget');
    chunks.push(chunk);
  }
  return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}

async function boundedBody(response, max = MAX_BYTES) {
  const chunks = [];
  let bytes = 0;
  for await (const chunk of response.body ?? []) {
    bytes += chunk.length;
    if (bytes > max) throw new Error('response-budget');
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}

function localURL(origin, path) {
  const u = new URL(path, origin);
  if (u.origin !== origin || u.username || u.password || u.protocol !== 'http:') throw new Error('origin-denied');
  return u;
}

async function jsonRequest(m, path, method = 'GET') {
  const response = await fetch(localURL(m.origin, path), { method, redirect: 'manual', signal: AbortSignal.timeout(2000) });
  if (response.status !== 200) throw new Error('probe-status');
  return JSON.parse((await boundedBody(response, 65536)).toString('utf8'));
}

async function identity(incoming) {
  const value = await jsonRequest(incoming.manifest, incoming.manifest.identityPath);
  return value.runId === incoming.runId && value.frontendDigest === incoming.binding.frontendDigest &&
    value.backendDigest === incoming.binding.backendDigest && value.fixture === incoming.binding.fixture;
}

async function ready(incoming, child) {
  for (let i = 0; i < 100; i++) {
    if (child.exitCode !== null || child.signalCode !== null) return false;
    try { return await identity(incoming); } catch { await sleep(50); }
  }
  return false;
}

function gap(e, value) {
  if (!e.gaps.includes(value) && e.gaps.length < 128) e.gaps.push(value);
}

async function routes(context, m, e) {
  await context.route('**/*', async route => {
    try {
      const request = route.request();
      const url = localURL(m.origin, request.url());
      const body = request.postDataBuffer();
      if (body?.length > 8192) throw new Error('request-budget');
      // Proxy without following redirects; even the first redirect cannot escape this origin.
      const response = await fetch(url, {
        method: request.method(), body: ['GET', 'HEAD'].includes(request.method()) ? undefined : body,
        headers: request.headers(), redirect: 'manual', signal: AbortSignal.timeout(3000),
      });
      if (response.status >= 300 && response.status < 400) throw new Error('redirect-denied');
      const bytes = await boundedBody(response);
      const headers = Object.fromEntries(response.headers);
      delete headers['content-encoding'];
      delete headers['content-length'];
      await route.fulfill({ status: response.status, headers, body: bytes });
    } catch {
      gap(e, 'request-blocked-or-unavailable');
      await route.abort().catch(() => {});
    }
  });
  await context.routeWebSocket('**/*', ws => { gap(e, 'websocket-blocked'); ws.close(); });
}

async function state(page, e, run, states) {
  const snapshot = await page.evaluate(() => {
    const nodes = [...document.querySelectorAll('a,form,input,button,select,textarea,canvas,[role="button"]')].slice(0, 101);
    return nodes.map(el => ({ tag: el.tagName.toLowerCase(), id: el.id,
      visible: !!el.getClientRects().length, disabled: !!el.disabled, children: el.childElementCount }));
  });
  const safe = snapshot.map(c => ({ kind: ({ a:'link', form:'form', input:'input', button:'button' })[c.tag] ?? 'unsupported',
    selector: safeID.test(c.id) ? `#${c.id}` : '', visible:c.visible, disabled:c.disabled, children:Math.min(c.children, 10000) }));
  const hash = digest(JSON.stringify({ path: new URL(page.url()).pathname, controls: safe }));
  states.add(hash);
  if (states.size > 50 || run.states.length >= 50 || e.controls.length >= 100 || snapshot.length > 100) throw new Error('exploration-budget');
  run.states.push(hash);
  for (let i = 0; i < safe.length; i++) {
    const c = safe[i], id = digest(`${hash}\0${i}`);
    if (e.controls.some(x => x.id === id)) continue;
    if (e.controls.length >= 100) throw new Error('exploration-budget');
    e.controls.push({ id, kind:c.kind, selector:c.selector, state:hash, exercised:false });
    if (!c.selector || c.kind === 'unsupported') gap(e, 'unaddressable-or-visual-control');
  }
  return hash;
}

async function act(page, a) {
  if (a.kind === 'reload') { await page.reload({ waitUntil:'domcontentloaded' }); return; }
  const target = page.locator(a.selector);
  if (a.kind === 'fill') { await target.fill(a.value); return; }
  await target.click();
  await page.waitForLoadState('networkidle', { timeout:1500 });
}

function pointer(value, path) {
  for (const raw of path.slice(1).split('/')) {
    const key = raw.replace(/~1/g, '/').replace(/~0/g, '~');
    if (value === null || typeof value !== 'object' || !Object.hasOwn(value, key)) throw new Error('missing-probe-field');
    value = value[key];
  }
  return value;
}

async function check(page, m, c) {
  const result = { id:c.id, outcome:'unknown', layer:c.kind === 'json' ? 'backend' : 'ui' };
  try {
    if (c.kind === 'json') {
      const value = pointer(await jsonRequest(m, c.path), c.pointer);
      result.outcome = value === c.want ? 'matched' : 'contradicted';
      return result;
    }
    const end = Date.now() + 1200;
    let matches = false;
    do {
      const target = page.locator(c.selector);
      if (c.kind === 'count') matches = await target.count() === c.want;
      if (c.kind === 'visible') matches = await target.isVisible();
      if (c.kind === 'text') matches = await target.textContent({ timeout:500 }) === c.want;
      if (c.kind === 'contains') matches = (await target.textContent({ timeout:500 }))?.includes(c.want) === true;
      if (matches) break;
      await sleep(25);
    } while (Date.now() < end);
    result.outcome = matches ? 'matched' : 'contradicted';
  } catch { result.outcome = 'unknown'; }
  return result;
}

async function scenario(browser, incoming, e, s, states) {
  const m = incoming.manifest;
  const run = { scenario:s.id, outcome:'inconclusive', checks:[], states:[], identityMatched:false };
  e.runs.push(run);
  const context = await browser.newContext({ serviceWorkers:'block', acceptDownloads:false });
  context.setDefaultTimeout(1500);
  try {
    if (!await identity(incoming)) throw new Error('identity-drift');
    await jsonRequest(m, m.resetPath, 'POST');
    await routes(context, m, e);
    const page = await context.newPage();
    page.on('dialog', dialog => dialog.dismiss().catch(() => {}));
    page.on('download', download => download.cancel().catch(() => {}));
    await page.goto(localURL(m.origin, s.path).href, { waitUntil:'domcontentloaded' });
    let current = await state(page, e, run, states);
    for (const a of s.actions) {
      await act(page, a);
      for (const c of e.controls) {
        if (c.state === current && c.selector === a.selector) c.exercised = true;
      }
      current = await state(page, e, run, states);
    }
    for (const c of s.checks) run.checks.push(await check(page, m, c));
    run.identityMatched = await identity(incoming);
    if (!run.identityMatched) throw new Error('identity-drift');
    run.outcome = run.checks.some(c => c.outcome === 'unknown') ? 'inconclusive' : 'matched';
    if (run.checks.some(c => c.outcome === 'contradicted')) run.outcome = 'contradicted';
  } catch {
    gap(e, 'scenario-incomplete-or-budgeted');
    run.outcome = 'inconclusive';
  } finally { await context.close(); }
}

async function observe(incoming, e) {
  const { chromium } = await import('playwright');
  e.playwrightVersion=require('playwright/package.json').version;
  if(e.playwrightVersion!=='1.63.0') throw new Error('unsupported-playwright-version');
  const m = incoming.manifest;
  const lifecycle = ownedLifecycle(e, gap);
  try {
    const child = spawn(m.server[0], m.server.slice(1), { cwd:incoming.root,
      env:{ PATH:process.env.PATH, HOME:process.env.HOME, TMPDIR:process.env.TMPDIR,
        CORVINT_FLOW_RUN_ID:incoming.runId }, stdio:['ignore','ignore','ignore'] });
    lifecycle.ownServer(child);
    child.on('error', () => gap(e, 'server-unavailable'));
    if (!await ready(incoming, child)) { gap(e, 'served-identity-unavailable'); return; }
    const browser = await lifecycle.launch(() => chromium.launch({ headless:true, args:['--force-webrtc-ip-handling-policy=disable_non_proxied_udp'] }));
    if (lifecycle.stopping) return;
    e.browser = `chromium-${browser.version()}`;
    const states = new Set();
    for (const s of m.scenarios) await scenario(browser, incoming, e, s, states);
    if (!await identity(incoming)) {
      gap(e, 'served-identity-drift');
      for (const run of e.runs) { run.identityMatched = false; run.outcome = 'inconclusive'; }
    }
  } finally {
    await lifecycle.close();
    lifecycle.dispose();
  }
}

async function main() {
  const incoming = await input();
  const parsed = scan(incoming.sources, incoming.binding.sources);
  const e = { profile:'application-flow-evidence/0', authority:'CALLER_REPORTED', binding:incoming.binding,
    runId:incoming.runId, mode:incoming.observe ? 'observe' : 'scan', parser:parsed.parser, browser:'',
    nodeVersion:process.version, playwrightVersion:'', providerDigest:'',
    tests:parsed.tests, inventoryComplete:parsed.inventoryComplete, controls:[], runs:[], gaps:parsed.gaps, cleanup:false,
    cleanupScope:'owned-process-group-and-owned-browser', browserClosed:false, serverExited:false };
  gap(e, 'guided-exploration-only');
  if (incoming.observe) {
    gap(e, 'non-http-browser-transports-unqualified');
    gap(e, 'escaped-daemon-descendants-unqualified');
    try { await observe(incoming, e); } catch { gap(e, 'observer-infrastructure-unavailable'); }
  }
  const output = JSON.stringify(e);
  if (Buffer.byteLength(output) > MAX_BYTES) throw new Error('evidence-budget');
  process.stdout.write(output + '\n');
}

main().catch(() => { process.stderr.write('flow observer failed\n'); process.exitCode = 2; });
