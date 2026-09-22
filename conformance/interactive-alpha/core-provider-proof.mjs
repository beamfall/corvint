import { readFile, writeFile, readdir, mkdir, cp, symlink, stat } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { createServer } from 'node:net';
import { coreBrowserExecutable, Child, run, cleanup, until, delay, sha, fileSHA, treeSHA, save, assert, mcp, processTable, fixtureIdentity } from './core-process.mjs';

const [bin, source, root, attachment, output, mode] = process.argv.slice(2);
for (const path of [bin, source, root, attachment, output]) assert(path?.startsWith('/'), 'core provider needs absolute paths');
assert(['sessions', 'interrupt'].includes(mode), 'unknown core provider mode');
const mcpPath = join(bin, 'corvint-test-validity-mcp');
const evidence = async (name, raw) => writeFile(join(output, name + '.json'), raw, { flag: 'wx', mode: 0o600 });
const fixtureSource = join(source, 'conformance/interactive-alpha/fixture');
const configs = new Map();
async function port() { const server = createServer(); await new Promise(resolve => server.listen(0, '127.0.0.1', resolve)); const value = server.address().port; await new Promise(resolve => server.close(resolve)); return value; }
async function git(dir, ...args) { return run(['/usr/bin/git', '-C', dir, ...args], dir); }
async function prepare(kind) {
  const dir = join(root, kind); await mkdir(dir, { recursive: true });
  if (mode === 'sessions') {
    if (kind === 'go') { await cp(join(fixtureSource, 'go.mod'), join(dir, 'go.mod')); await cp(join(fixtureSource, 'go'), join(dir, 'go'), { recursive: true }); }
    else {
      for (const file of ['package.json', 'package-lock.json', 'vitest.config.js', 'playwright.config.js', 'unit.test.js', 'e2e.spec.js', 'math.js']) await cp(join(fixtureSource, file), join(dir, file));
      await cp(join(fixtureSource, 'app'), join(dir, 'app'), { recursive: true });
      if (kind === 'e2e') {
        const config = join(dir, 'playwright.config.js'), original = await readFile(config, 'utf8');
        const executable = coreBrowserExecutable(fixtureSource);
        const pinned = original.replace('use: { headless: true }', `use: { headless: true, launchOptions: { executablePath: ${JSON.stringify(executable)} } }`);
        assert(pinned !== original, 'pinned browser fixture seam missing'); await writeFile(config, pinned);
      }
      await symlink(join(fixtureSource, 'node_modules'), join(dir, 'node_modules'), 'dir');
      const unit = join(dir, 'unit.test.js'); await writeFile(unit, (await readFile(unit, 'utf8')).replace('it("adds two numbers", () => {', 'it("adds two numbers", async () => { await new Promise(resolve => setTimeout(resolve, 1200));'));
      const test = join(dir, 'e2e.spec.js'); await writeFile(test, (await readFile(test, 'utf8')).replace('await page.goto', 'await page.waitForTimeout(1200);\n  await page.goto'));
    }
    await git(dir, 'init', '-q'); await git(dir, 'add', '.'); await git(dir, '-c', 'user.name=Core qualification', '-c', 'user.email=core@example.invalid', 'commit', '-qm', 'Frozen provider fixture');
  }
  let argv, watched;
  if (kind === 'go') {
    const a = JSON.parse(await readFile(attachment)); assert(a.repositoryRoot === dir && a.verifierExecutable === join(bin, 'corvint-go-test-provider'), 'Go attachment does not select exact installed fixture/verifier');
    for (const directory of [a.temporaryParent, a.moduleCacheDirectory]) await mkdir(directory, { recursive: true });
    argv = [join(bin, 'corvint-go-test-provider'), 'session', '--experimental', '--trusted-local', '--foreground', '--retain', '--authority-bundle', attachment, '--scope', 'example.test/corvint-interactive-alpha/go', '--watch', 'go/calc.go', '--watch', 'go/calc_test.go']; watched = join(dir, 'go/calc.go');
  } else {
    const e2e = kind === 'e2e'; const number = await port();
    if (e2e) {
      const server = join(dir, 'app/server.js'); await writeFile(server, (await readFile(server, 'utf8')).replace(/server.listen\(\d+/, `server.listen(${number}`));
      const test = join(dir, 'e2e.spec.js'); await writeFile(test, (await readFile(test, 'utf8')).replace(/127\.0\.0\.1:\d+/, `127.0.0.1:${number}`));
    }
    argv = [join(bin, 'corvint-js-test-provider'), kind, '--dir', dir, '--package-json', join(dir, 'package.json'), '--lockfile', join(dir, 'package-lock.json'), '--config', join(dir, e2e ? 'playwright.config.js' : 'vitest.config.js'), '--test-file', join(dir, e2e ? 'e2e.spec.js' : 'unit.test.js'), '--runner-version', e2e ? '1.63.0' : '5.0.0', '--timeout', '60s', '--retain', '--foreground', '--experimental', '--trusted-local', '--watch', e2e ? 'app/index.html' : 'math.js'];
    if (e2e) argv.push('--watch', 'app/server.js', '--app-build-dir', join(dir, 'app'), '--server-ready-url', `http://127.0.0.1:${number}/`, '--server-arg', 'node', '--server-arg', 'app/server.js', '--test-arg', 'npx', '--test-arg', 'playwright', '--test-arg', 'test', '--test-arg', '--reporter=json');
    watched = join(dir, e2e ? 'app/index.html' : 'math.js');
  }
  const original = await readFile(watched); const broken = Buffer.from(original.toString().replace(kind === 'e2e' ? ' + 1' : 'left + right', kind === 'e2e' ? ' + 2' : 'left - right')); assert(!broken.equals(original), 'fixture mutation missing');
  const config = { kind, dir, argv, watched, original, broken }; configs.set(kind, config); return config;
}
async function documents(dir) {
  const path = join(dir, '.corvint/test-evidence'); let names; try { names = await readdir(path); } catch (error) { if (error.code === 'ENOENT') return []; throw error; }
  const rows = await Promise.all(names.filter(n => n.endsWith('.json')).map(async name => ({ name, raw: await readFile(join(path, name)), time: (await stat(join(path, name))).mtimeMs })));
  return rows.sort((a, b) => a.time - b.time || a.name.localeCompare(b.name));
}
const providerState = row => { const doc = JSON.parse(row.raw); return doc.receipt ? doc.receipt.tests[0]?.state : doc.state; };
async function selected(config, child, name, expectedRow, expectedState) {
  const response = await mcp(mcpPath, config.dir); const doc = response.result.structuredContent.document;
  assert(!response.result.isError && doc.discovery.evidence.endsWith('/' + expectedRow.name), 'MCP did not select original retained document');
  assert(doc.discovery.freshness.state === 'UNKNOWN', 'MCP promoted unmeasured freshness');
  assert(doc.tests.length > 0 && doc.tests.every(t => t.projection.execution.state === expectedState.toUpperCase() && t.projection.freshness.state === 'UNKNOWN' && t.projection.strength.state === 'NOT_MEASURED'), 'MCP axes differ from actual execution');
  if (config.kind === 'go') assert(doc.tier === 'preview' && doc.promotable === false, 'Go tier promoted');
  assert(child.stdout.includes(expectedRow.raw), 'retention did not preserve stdout bytes'); if (name) await evidence(name, response.raw); return sha(response.raw);
}
async function active(child, kind) {
  return until(() => { child.observe(); const current = processTable(); const rows = [...child.observed.values()].filter(x => current.get(x.pid)?.startTime === x.startTime);
    const ok = kind === 'e2e' ? rows.some(r => /chromium/i.test(r.command)) && rows.some(r => r.command.includes('app/server.js')) : kind === 'go' ? rows.some(r => /go-build.*\.test|\bgo test\b/.test(r.command)) : rows.some(r => /vitest/.test(r.command));
    return ok ? rows : false;
  }, child, 90000);
}
async function sessions() {
  const transitions = [];
  for (const kind of ['unit', 'e2e', 'go']) {
    const c = await prepare(kind), prefix = kind === 'go' ? 'go' : 'js-' + kind;
    await save(join(output, 'fixture-' + kind + '.json'), await fixtureIdentity('js-' + kind === 'js-go' ? 'go' : 'js-' + kind, c.dir, kind === 'go' ? { package: 'go.mod', test: 'go/calc_test.go' } : { package: 'package.json', lock: 'package-lock.json', config: kind === 'unit' ? 'vitest.config.js' : 'playwright.config.js', test: kind === 'unit' ? 'unit.test.js' : 'e2e.spec.js' }));
    const fixtureBeforeSha256 = await fileSHA(c.watched); const child = new Child(c.argv, c.dir, 240000);
    try {
      const initial = await until(async () => (await documents(c.dir))[0], child, 120000); assert(providerState(initial) === 'passed', 'initial provider did not pass');
      await evidence(prefix + '-initial-provider', initial.raw); await selected(c, child, prefix + '-initial-mcp', initial, 'passed');
      if (kind === 'unit') for (const [method, name] of [['server/discover', 'test-validity-discover'], ['tools/list', 'test-validity-list']]) await evidence(name, (await mcp(mcpPath, c.dir, method, {})).raw);
      await writeFile(c.watched, c.broken); const pendingFailSha256 = await selected(c, child, prefix + '-pending-fail-mcp', initial, 'passed');
      const failed = await until(async () => (await documents(c.dir))[1], child, 90000); assert(providerState(failed) === 'failed', 'saved failing source did not fail');
      await evidence(prefix + '-fail-provider', failed.raw); await selected(c, child, prefix + '-fail-mcp', failed, 'failed');
      await writeFile(c.watched, c.original); const pendingFixSha256 = await selected(c, child, prefix + '-pending-fix-mcp', failed, 'failed');
      const fixed = await until(async () => (await documents(c.dir))[2], child, 90000); assert(providerState(fixed) === 'passed', 'saved correction did not pass');
      await evidence(prefix + '-fix-provider', fixed.raw); await selected(c, child, prefix + '-fix-mcp', fixed, 'passed');
      await writeFile(c.watched, c.broken); const old = await active(child, kind); const streamBefore = child.stdout.length; const diagnosticsBefore = child.stderr.length;
      await writeFile(c.watched, c.original);
      let replacement = await until(async () => (await documents(c.dir))[3], child, 90000); let supersededProviderRaw = '', supersededMcpRaw = '';
      if (kind === 'go') {
        const stale = JSON.parse(replacement.raw); assert(stale.state === 'stale' && stale.projection.freshness.state === 'STALE' && stale.projection.execution.state !== 'PASSED' && stale.testProjections.length > 0 && stale.testProjections.every(t => t.projection.freshness.state === 'STALE' && t.projection.execution.state !== 'PASSED'), 'Go supersession did not preserve explicit stale negative');
        const staleMCP = await mcp(mcpPath, c.dir), projected = staleMCP.result.structuredContent.document;
        assert(projected.discovery.evidence.endsWith('/' + replacement.name) && projected.run.freshness.state === 'STALE' && projected.run.execution.state !== 'PASSED' && projected.tests.every(t => t.projection.freshness.state === 'STALE' && t.projection.execution.state !== 'PASSED'), 'MCP lost Go stale negative');
        supersededProviderRaw = replacement.raw.toString(); supersededMcpRaw = staleMCP.raw.toString();
        replacement = await until(async () => (await documents(c.dir))[4], child, 90000); assert(JSON.parse(replacement.raw).sequence > stale.sequence, 'Go replacement sequence did not advance');
      }
      assert(providerState(replacement) === 'passed', 'superseded success boundary failed');
      await selected(c, child, null, replacement, 'passed');
      if (kind === 'go') assert(JSON.parse(replacement.raw).identity === JSON.parse(fixed.raw).identity, 'Go replacement identity differs from restored fixture');
      assert((await documents(c.dir)).length === (kind === 'go' ? 5 : 4), 'extra late result published');
      const newer = kind === 'go' ? child.stdout.subarray(streamBefore).toString().includes('"state":"running"') : child.stderr.subarray(diagnosticsBefore).toString().includes('source changed'); assert(newer, 'no newer generation observed');
      await until(() => { const now = processTable(); return old.every(row => now.get(row.pid)?.startTime !== row.startTime); }, child, 10000);
      transitions.push({ kind, commandSha256: sha(JSON.stringify(c.argv)), fixtureBeforeSha256, fixtureFailedSha256: sha(c.broken), fixtureFixedSha256: sha(c.original), initialSha256: sha(initial.raw), failedSha256: sha(failed.raw), fixedSha256: sha(fixed.raw), pendingFailSha256, pendingFixSha256, retainedBytesMatch: true, supersededSuccessAbsent: true, supersededProviderRaw, supersededMcpRaw });
      console.log(JSON.stringify({ kind, superseded: { observedPrior: old, replacementSha256: sha(replacement.raw), rawStream: child.stdout.toString(), diagnostics: child.stderr.toString() } }));
    } finally { await child.stop(); }
  }
  await save(join(output, 'provider-transitions.json'), { profile: 'corvint-core-provider-transitions/0', status: 'PASS', sessions: transitions });
}
async function interruptions() {
  const runs = [];
  for (const kind of ['unit', 'e2e', 'go']) for (const signal of ['SIGTERM', 'SIGINT']) {
    const c = await prepare(kind), before = (await documents(c.dir)).length, child = new Child(c.argv, c.dir, 120000);
    try { await active(child, kind); await child.stop(signal); assert((await documents(c.dir)).length === before, 'cancelled receipt published');
      runs.push({ kind, signal: signal.slice(3), exitCode: child.code, observedIdentities: [...child.observed.values()].map(({ pid, startTime, observation }) => ({ pid, startTime, observation })), remainingIdentities: [], cancelledReceiptPublished: false });
    } finally { await child.stop(); }
  }
  await save(join(output, 'provider-interruption.json'), { profile: 'corvint-core-provider-interruption/0', status: 'PASS', runs });
}
try { await mkdir(root, { recursive: true }); await mkdir(output, { recursive: true }); if (mode === 'sessions') await sessions(); else await interruptions(); } finally { await cleanup(); }
