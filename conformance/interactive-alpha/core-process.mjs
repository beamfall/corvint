import { readFileSync, lstatSync, openSync, fstatSync, readSync, closeSync, realpathSync } from 'node:fs';
import { createRequire } from 'node:module';
import { spawn, execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readFile, readdir, lstat, readlink, writeFile, open } from 'node:fs/promises';
import { join, relative, dirname, resolve, isAbsolute } from 'node:path';

// Playwright 1.63 selects this registry entry for headless Chromium without a channel.
// Keep the executed browser and the offline runtime identity on the same closed resolver.
export function coreBrowserExecutable(dependencyRoot) {
  const require = createRequire(join(dependencyRoot, 'package.json'));
  assert(require('playwright/package.json').version === '1.63.0', 'unsupported core Playwright version');
  assert(require('playwright-core/package.json').version === '1.63.0', 'unsupported core Playwright layout');
  const registry = require('playwright-core/lib/coreBundle').registry?.registry;
  assert(typeof registry?.findExecutable === 'function', 'unsupported core browser registry');
  const entry = registry.findExecutable('chromium-headless-shell');
  assert(entry?.name === 'chromium-headless-shell' && entry.browserName === 'chromium' &&
    entry.revision === '1243' && entry.browserVersion === '153.0.8010.12' &&
    typeof entry.executablePath === 'function', 'unsupported core headless browser metadata');
  const executable = entry.executablePath();
  assert(typeof executable === 'string' && isAbsolute(executable) && realpathSync(executable) === executable,
    'core browser executable is not canonical');
  assert(lstatSync(executable).isFile(), 'core browser executable is not a regular file');
  return executable;
}

function verifyPins() {
  const path = process.env.CORVINT_CORE_PINS; if (!path) return;
  assert(lstatSync(path).size <= 1 << 20, 'runtime pin manifest exceeds bound');
  const pins = JSON.parse(readFileSync(path, 'utf8')); assert(Array.isArray(pins) && pins.length > 0 && pins.length <= 32, 'invalid runtime pin count');
  for (const pin of pins) {
    const info = lstatSync(pin.path, { bigint: true }); assert(info.isFile() && info.size <= 512n << 20n && /^[a-f0-9]{64}$/.test(pin.sha256), 'invalid pinned runtime');
    const fd = openSync(pin.path, 'r');
    try {
      const start = fstatSync(fd, { bigint: true }); assert(start.dev === info.dev && start.ino === info.ino, 'pinned runtime replaced');
      const hash = createHash('sha256'), buffer = Buffer.alloc(65536); let total = 0, length;
      while ((length = readSync(fd, buffer, 0, buffer.length, null)) > 0) { total += length; assert(total <= 512 << 20, 'runtime grew beyond bound'); hash.update(buffer.subarray(0, length)); }
      const end = fstatSync(fd, { bigint: true }), named = lstatSync(pin.path, { bigint: true });
      assert(hash.digest('hex') === pin.sha256 && start.ino === named.ino && start.dev === named.dev && start.size === end.size && start.mtimeNs === end.mtimeNs, 'pinned runtime drift');
    } finally { closeSync(fd); }
  }
}

export const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
export const sha = bytes => createHash('sha256').update(bytes).digest('hex');
export async function fileSHA(path) {
  const original = await lstat(path, { bigint: true }); assert(original.isFile() && original.size <= 512n << 20n, 'hash input exceeds regular-file bound');
  const file = await open(path, 'r');
  try {
    const start = await file.stat({ bigint: true }); assert(start.dev === original.dev && start.ino === original.ino, 'hash input replaced while opening');
    const hash = createHash('sha256'); let bytes = 0;
    for await (const chunk of file.createReadStream({ autoClose: false })) { bytes += chunk.length; assert(bytes <= 512 << 20, 'hash input grew beyond bound'); hash.update(chunk); }
    const end = await file.stat({ bigint: true }), named = await lstat(path, { bigint: true });
    assert(start.dev === named.dev && start.ino === named.ino && start.size === end.size && start.mtimeNs === end.mtimeNs && start.size === BigInt(bytes), 'hash input changed');
    return hash.digest('hex');
  } finally { await file.close(); }
}
export const save = async (path, value) => writeFile(path, JSON.stringify(value) + '\n', { flag: 'wx', mode: 0o600 });
export function assert(condition, message) { if (!condition) throw new Error(message); }
export async function treeSHA(root, ignored = []) {
  const rows = []; let count = 0, bytes = 0;
  const rootInfo = await lstat(root); assert(rootInfo.isDirectory() && !rootInfo.isSymbolicLink(), "inventory root is not a real directory");
  async function visit(path) {
    for (const name of (await readdir(path)).sort()) {
      if (ignored.includes(name)) continue;
      const file = join(path, name), info = await lstat(file), logical = relative(root, file);
      assert(++count <= 200000, 'inventory file count exceeded'); bytes += info.isFile() ? info.size : 0; assert(bytes <= 4 * (1 << 30), 'inventory byte limit exceeded');
      if (info.isDirectory()) await visit(file);
      else if (info.isSymbolicLink()) { const target = await readlink(file), confined = relative(root, resolve(dirname(file), target)); assert(!isAbsolute(target) && confined !== '..' && !confined.startsWith('../'), 'inventory symlink escapes root'); rows.push([logical, 'link', target]); }
      else { assert(info.isFile(), 'nonregular inventory member'); rows.push([logical, info.mode & 0o111 ? 'executable' : 'file', await fileSHA(file)]); }
    }
  }
  await visit(root);
  return sha(JSON.stringify(rows));
}
export function processTable() {
  const text = execFileSync('/bin/ps', ['-axo', 'pid=,ppid=,lstart=,command='], { encoding: 'utf8', timeout: 2000, maxBuffer: 8 << 20 });
  const rows = new Map();
  for (const line of text.split('\n')) {
    const match = line.trim().match(/^(\d+)\s+(\d+)\s+(\S+\s+\S+\s+\d+\s+\S+\s+\d+)\s+(.*)$/);
    if (match) rows.set(+match[1], { pid: +match[1], parent: +match[2], startTime: match[3], command: match[4], observation: 'ps-lstart-seconds' });
  }
  return rows;
}
export function descendants(pid, rows) {
  const found = new Set([pid]);
  let changed = true;
  while (changed) { changed = false; for (const row of rows.values()) if (found.has(row.parent) && !found.has(row.pid)) { found.add(row.pid); changed = true; } }
  found.delete(pid); return [...found].map(id => rows.get(id));
}
const owned = new Set();
const cleanupHooks = new Set();
export function registerCleanup(fn) { cleanupHooks.add(fn); return () => cleanupHooks.delete(fn); }
let ending = false;
for (const [signal, code] of [['SIGINT', 130], ['SIGTERM', 143]]) process.on(signal, () => {
  if (ending) return; ending = true;
  cleanup().then(() => process.exit(code), error => { console.error(error); process.exit(1); });
});
export class Child {
  constructor(argv, cwd, timeout = 120000) {
    verifyPins();
    this.timeout = timeout; this.argv = argv; this.stdout = Buffer.alloc(0); this.stderr = Buffer.alloc(0); this.observed = new Map(); this.failure = undefined; this.closed = false;
    this.process = spawn(argv[0], argv.slice(1), { cwd, env: process.env, detached: true, stdio: ['pipe', 'pipe', 'pipe'] });
    owned.add(this);
    const append = (key, bytes) => {
      if (this[key].length + bytes.length > 1 << 20) { this.failure ??= new Error('child output exceeded 1 MiB'); this.signal('SIGTERM'); return; }
      this[key] = Buffer.concat([this[key], bytes]);
    };
    this.process.stdout.on('data', bytes => append('stdout', bytes)); this.process.stderr.on('data', bytes => append('stderr', bytes));
    this.process.stdin.on('error', error => { if (error.code !== 'EPIPE') this.failure ??= error; });
    this.done = new Promise(resolve => {
      this.process.once('error', error => { this.failure = error; });
      this.process.once('close', (code, signal) => { try { verifyPins(); } catch (error) { this.failure ??= error; } this.closed = true; this.code = code; this.exitSignal = signal; clearTimeout(this.timer); resolve(); });
    });
    this.tracker = setInterval(() => { try { this.observe(); } catch (error) { this.failure ??= error; this.signal('SIGTERM'); } }, 100);
    this.timer = setTimeout(() => { this.failure ??= new Error('child timeout'); this.signal('SIGTERM'); }, timeout);
  }
  observe() { if (!this.process.pid) return; for (const row of descendants(this.process.pid, processTable())) this.observed.set(`${row.pid}:${row.startTime}`, row); }
  signal(signal) { if (this.process.pid) try { process.kill(-this.process.pid, signal); } catch (error) { if (error.code !== 'ESRCH') throw error; } }
  current() { const rows = processTable(); return [...this.observed.values()].filter(row => { const now = rows.get(row.pid); return now?.startTime === row.startTime && now.command === row.command; }); }
  async waitClosed(milliseconds) { let timer; try { return await Promise.race([this.done.then(() => true), new Promise(resolve => { timer = setTimeout(() => resolve(false), milliseconds); })]); } finally { clearTimeout(timer); } }
  signalObserved(signal) {
    for (const row of this.current()) try { process.kill(row.pid, signal); } catch (error) { if (error.code !== 'ESRCH') throw error; }
  }
  async retire() {
    let forced = false;
    if (!this.closed || this.current().length) {
      forced = true; this.signal('SIGTERM'); this.signalObserved('SIGTERM'); await this.waitClosed(1000); await delay(100);
      if (!this.closed || this.current().length) { this.signal('SIGKILL'); this.signalObserved('SIGKILL'); await this.waitClosed(2000); }
    }
    if (!this.closed) {
      this.process.stdin.destroy(); this.process.stdout.destroy(); this.process.stderr.destroy();
      await this.waitClosed(500);
    }
    const deadline = Date.now() + 3000;
    while (this.current().length && Date.now() < deadline) await delay(50);
    this.remaining = this.current(); clearInterval(this.tracker); clearTimeout(this.timer); owned.delete(this);
    assert(this.closed && !this.remaining.length, 'owned child or observed escaped descendant survived bounded cleanup');
    return forced;
  }
  async join(expected = 0) {
    const closed = await this.waitClosed(this.timeout + 5000);
    const forced = await this.retire();
    if (!closed || forced) throw new Error('normal child completion required fallback containment');
    if (this.failure) throw this.failure;
    assert(this.code === expected, `child exit ${this.code}, want ${expected}: ${this.stderr.toString().slice(-2000)}`);
    return this.stdout;
  }
  async stop(signal = 'SIGTERM') {
    if (!this.closed) { this.observe(); this.process.kill(signal); }
    const closed = await this.waitClosed(5000); const forced = await this.retire();
    if (!closed || forced) throw new Error('child interruption required fallback containment');
    if (this.failure) throw this.failure;
  }
}
let cleaning;
export function cleanup() {
  if (cleaning) return cleaning;
  cleaning = cleanupAll().finally(() => { cleaning = undefined; });
  return cleaning;
}
async function cleanupAll() {
  const failures = [], observed = new Map();
  const observe = () => { try { for (const row of descendants(process.pid, processTable())) observed.set(`${row.pid}:${row.startTime}`, row); } catch (error) { failures.push(error); } };
  observe(); const tracker = setInterval(observe, 100);
  const current = () => { const rows = processTable(); return [...observed.values()].filter(row => { const now = rows.get(row.pid); return now?.startTime === row.startTime && now.command === row.command; }); };
  const signal = name => { for (const row of current()) try { process.kill(row.pid, name); } catch (error) { if (error.code !== 'ESRCH') failures.push(error); } };
  try {
    for (const hook of cleanupHooks) {
      let timer;
      try { await Promise.race([Promise.resolve().then(hook), new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('cleanup hook exceeded 5s')), 5000); })]); }
      catch (error) { failures.push(error); }
      finally { clearTimeout(timer); }
    }
    for (const child of [...owned]) try { await child.stop(); } catch (error) { failures.push(error); }
    if (current().length) {
      failures.push(new Error('helper descendant required fallback containment'));
      signal('SIGTERM'); let deadline = Date.now() + 1000;
      while (current().length && Date.now() < deadline) await delay(50);
      if (current().length) { signal('SIGKILL'); deadline = Date.now() + 3000; while (current().length && Date.now() < deadline) await delay(50); }
      if (current().length) failures.push(new Error('observed helper descendant survived bounded cleanup'));
    }
  } finally { clearInterval(tracker); }
  if (failures.length) throw new AggregateError(failures, 'child cleanup failed');
}
export async function run(argv, cwd, expected = 0, timeout = 120000) { const child = new Child(argv, cwd, timeout); child.process.stdin.end(); return child.join(expected); }
export async function until(predicate, child, timeout = 60000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) { const result = await predicate(); if (result) return result; if (child?.closed) throw new Error(`child exited while waiting: ${child.stdout} ${child.stderr}`); await delay(25); }
  throw new Error('bounded observation timeout');
}
export const meta = { 'io.modelcontextprotocol/protocolVersion': '2026-07-28', 'io.modelcontextprotocol/clientCapabilities': {} };
export async function mcp(executable, root, method = 'tools/call', params = { name: 'corvint.test_validity', arguments: { discover: true } }) {
  const child = new Child([executable, '--root', root], root, 15000);
  child.process.stdin.end(JSON.stringify({ jsonrpc: '2.0', id: 1, method, params: { _meta: meta, ...params } }) + '\n');
  const raw = await child.join(); const parsed = JSON.parse(raw); assert(parsed.jsonrpc === '2.0' && parsed.id === 1 && !parsed.error, 'MCP envelope failure'); return { raw, result: parsed.result };
}

export async function fixtureIdentity(name, root, bindings) {
  const value = { name, treeSha256: await treeSHA(root, ['.git', '.corvint', 'node_modules']) };
  for (const key of ['package', 'lock', 'config', 'test']) value[key + 'Sha256'] = bindings[key] ? await fileSHA(join(root, bindings[key])) : '';
  return value;
}
