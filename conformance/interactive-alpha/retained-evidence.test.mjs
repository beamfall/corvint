// SPDX-License-Identifier: Apache-2.0
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, rm, readFile, realpath, utimes } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import vm from 'node:vm';
import { readRetainedEvidence, selectedEvidence, decode } from './retained-evidence.cjs';

const axis = state => ({ state, reason: '', anchors: [] });
const projection = (state, freshness = 'UNKNOWN') => ({ association: axis('UNSUPPORTED'), hygiene: axis('UNSUPPORTED'), execution: axis(state), freshness: axis(freshness), strength: axis('NOT_MEASURED') });
const unsupported = Object.fromEntries(['association', 'hygiene', 'execution', 'freshness', 'strength'].map(key => [key, axis('UNSUPPORTED')]));
function fixture(kind, state) {
  if (kind === 'go') return { profile: 'corvint-go-live-session-event/0', identity: 'fixture', sequence: 1, scope: ['example.test/fixture'], detail: '', state,
    projection: projection(state === 'stale' ? 'INFRASTRUCTURE' : state.toUpperCase(), state === 'stale' ? 'STALE' : 'CURRENT'),
    testProjections: [{ package: 'example.test/fixture', name: 'TestAdds', action: state === 'stale' ? 'none' : state === 'passed' ? 'pass' : 'fail', projection: projection(state === 'stale' ? 'UNSUPPORTED' : state.toUpperCase(), state === 'stale' ? 'STALE' : 'CURRENT') }], testProjectionsOmitted: 0 };
  const hash = 'a'.repeat(64);
  const runnerName = kind === 'unit' ? 'vitest' : 'playwright';
  const identity = { testFileDigests: { 'adds.js': hash }, configFile: runnerName + '.config.js',
    configDigest: hash, packageDigest: hash, nodeVersion: 'v22.0.0', runnerName,
    runnerVersion: kind === 'unit' ? '5.0.3' : '1.64.0', environment: {}, argv: ['npx', runnerName] };
  return { receipt: { kind, identity, cancelled: false, staleAppBuild: false, tests: [{ name: 'adds', state }] }, testProjections: [{ name: 'adds', state, projection: { ...projection(state.toUpperCase()), association: axis('ASSOCIATED'), hygiene: axis('ELIGIBLE') } }], runProjection: unsupported };
}

// RMV-V1-009: exercise the installed harness comparison without VS Code or any MCP binary.
test('retained unit, e2e and Go transitions agree with installed snapshots without MCP', async () => {
  const root = await mkdtemp(join(tmpdir(), 'retained-qualification-'));
  const directory = join(root, '.corvint/test-evidence');
  await mkdir(directory, { recursive: true });
  try {
    await writeFile(join(directory, '.corvint-js-test-provider-0-abc.tmp'), '{unfinished');
    const filename = resolve('extensions/vscode/test/installed/index.cjs');
    const require = createRequire(filename);
    const context = { exports: {}, Buffer, console, setTimeout, process: { env: { CORVINT_FIXTURE_ROOT: root } }, require: name => name === 'vscode' ? { Uri: { file: value => value } } : require(name) };
    vm.runInNewContext(await readFile(filename, 'utf8') + '\nexports.compareRetained = compareRetained;', context, { filename });
    let sequence = 0;
    for (const kind of ['unit', 'e2e', 'go']) for (const state of ['passed', 'failed', 'passed', ...(kind === 'go' ? ['stale'] : [])]) {
      const value = fixture(kind, state), raw = Buffer.from(JSON.stringify(value) + '\n');
      const name = `corvint-${kind === 'go' ? 'go' : 'js'}-test-provider-${++sequence}-abc.json`;
      await writeFile(join(directory, name), raw);
      await utimes(join(directory, name), sequence, sequence);
      const retained = await readRetainedEvidence(root);
      if (state !== 'stale') {
        await selectedEvidence({ dir: root, kind }, { stdout: raw }, { name, raw }, state);
        await assert.rejects(selectedEvidence({ dir: root, kind }, { stdout: Buffer.alloc(0) }, { name, raw }, state), /stdout bytes/);
      }
      assert.equal(retained.name, name);
      assert.deepEqual(retained.raw, raw);
      assert.equal(retained.document.discovery.freshness.state, 'UNKNOWN');
      assert.equal(retained.document.tests[0].projection.execution.state, state === 'stale' ? 'UNSUPPORTED' : state.toUpperCase());
      const itemState = state === 'stale' ? 'errored' : state;
      const items = kind === 'go' ? [{ id: 'example.test/fixture#TestAdds', state: itemState }, { id: 'example.test/fixture', state: itemState }] : [{ id: 'adds', state }, { id: `js-run:${kind}`, state: 'errored' }];
      // Arrays must belong to the installed module's realm for strict comparison.
      context.snapshotJSON = JSON.stringify({ provider: kind === 'go' ? 'go-session' : 'js-' + kind, inputIdentity: 'fixture', documentDigest: 'sha256:' + createHash('sha256').update(raw.toString().trim()).digest('hex'), omitted: 0, items });
      await assert.rejects(vm.runInNewContext('exports.compareRetained({ ...JSON.parse(snapshotJSON), provider: "future-provider" })', context), /unsupported editor provider/);
      const result = await vm.runInNewContext('exports.compareRetained(JSON.parse(snapshotJSON))', context);
      assert.equal(result.evidence, '.corvint/test-evidence/' + name);
    }
  } finally { await rm(root, { recursive: true, force: true }); }
});

test('selection binds the receipt to the invocation and requires every native axis', async t => {
  const root = await mkdtemp(join(tmpdir(), 'retained-invocation-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const directory = join(root, '.corvint/test-evidence');
  await mkdir(directory, { recursive: true });
  const name = 'corvint-js-test-provider-1-abc.json';
  async function select(value, kind) {
    const raw = Buffer.from(JSON.stringify(value));
    await writeFile(join(directory, name), raw);
    return selectedEvidence({ dir: root, kind }, { stdout: raw }, { name, raw }, 'passed');
  }
  await assert.rejects(select(fixture('e2e', 'passed'), 'unit'), /kind differs/);
  await assert.rejects(select(fixture('unit', 'passed'), 'future-runner'), /unsupported invocation/);
  for (const name of Object.keys(unsupported)) {
    const value = fixture('unit', 'passed');
    delete value.testProjections[0].projection[name];
    await assert.rejects(select(value, 'unit'), /expected provider object/);
  }
  const future = fixture('unit', 'passed');
  future.testProjections[0].projection.hygiene.state = 'FUTURE';
  await assert.rejects(select(future, 'unit'), /unsupported provider hygiene/);
  await writeFile(join(directory, '.gitignore.backup'), '');
  await assert.rejects(select(fixture('unit', 'passed'), 'unit'), /unsupported retained evidence name/);
});

test('retained JS qualification requires producer identity and matching nonempty names', () => {
  for (const kind of ['unit', 'e2e']) {
    function refuses(change) {
      const value = fixture(kind, 'passed');
      change(value.receipt, value.testProjections[0]);
      assert.throws(() => decode(JSON.stringify(value)));
    }
    refuses((r, t) => { delete r.identity; delete r.tests[0].name; delete t.name; });
    for (const key of Object.keys(fixture(kind, 'passed').receipt.identity)) refuses(r => { delete r.identity[key]; });
    refuses(r => { r.identity.futureBinding = {}; });
    refuses(r => { r.identity.testFileDigests = { 'adds.js': null }; });
    refuses(r => { r.identity.runnerName = 'future'; });
    for (const name of [undefined, null, '', ' ', 42]) refuses((r, t) => { r.tests[0].name = t.name = name; });
    refuses((r, t) => { t.name = 'another test'; });
  }
});

test('retained discovery refuses a FIFO without waiting for a writer', { skip: process.platform === 'win32' }, async t => {
  const root = await mkdtemp(join(tmpdir(), 'retained-fifo-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const directory = join(root, '.corvint/test-evidence');
  await mkdir(directory, { recursive: true });
  const path = join(directory, 'corvint-js-test-provider-1-abc.json');
  await mkdir(path);
  await assert.rejects(readRetainedEvidence(root), /bounded regular file/);
  await rm(path, { recursive: true });
  const fifo = spawnSync('mkfifo', [path]);
  assert.ifError(fifo.error);
  assert.equal(fifo.status, 0, fifo.stderr.toString());
  // Bound the process itself so a regression blocking inside open cannot hang the suite.
  const read = spawnSync(process.execPath, ['-e',
    'require(process.argv[1]).readRetainedEvidence(process.argv[2]).catch(e => { console.error(e.message); process.exitCode = 1; })',
    resolve('conformance/interactive-alpha/retained-evidence.cjs'), root], { timeout: 5000, encoding: 'utf8' });
  assert.ifError(read.error);
  assert.equal(read.status, 1);
  assert.match(read.stderr, /bounded regular file/);
});

test('unknown forms and observation disagreements fail closed', () => {
  for (const change of [
    v => { v.profile = 'future-provider/1'; },
    v => { v.receipt.kind = 'future-runner'; },
    v => { v.receipt.futureSemantics = true; },
    v => { v.receipt.tests[0].state = 'future-state'; },
    v => { v.testProjections[0].projection.execution.state = 'FAILED'; },
    v => { v.receipt.cancelled = true; },
    v => { v.testProjections = []; },
  ]) {
    const value = fixture('unit', 'passed'); change(value);
    assert.throws(() => decode(JSON.stringify(value)));
  }
  const unknownGo = fixture('go', 'passed'); unknownGo.testProjections[0].action = 'future-action';
  assert.throws(() => decode(JSON.stringify(unknownGo)), /unsupported Go/);
});

// RMV-V1-009: run the same qualification entry points that consume the release binaries.
test('core sessions capture the provider bytes without an MCP executable', async t => {
  const scratch = await mkdtemp(join(tmpdir(), 'core-retained-session-'));
  t.after(() => rm(scratch, { recursive: true, force: true }));
  const raw = JSON.stringify(fixture('unit', 'passed')) + '\n';
  const output = join(scratch, 'proof');
  await mkdir(output);
  // Stop the real session at its next exclusive capture, before it needs a second receipt.
  const nextCapture = 'js-unit-pending-fail-mcp.json';
  await writeFile(join(output, nextCapture), '');
  await writeFile(join(scratch, 'corvint-js-test-provider'), `#!/usr/bin/env node
const fs = require('node:fs');
const raw = ${JSON.stringify(raw)};
const dir = '.corvint/test-evidence';
fs.mkdirSync(dir, { recursive: true });
fs.writeSync(1, raw);
fs.writeFileSync(dir + '/.corvint-js-test-provider-1-abc.tmp', raw);
fs.renameSync(dir + '/.corvint-js-test-provider-1-abc.tmp', dir + '/corvint-js-test-provider-1-abc.json');
process.stdin.resume();
`, { mode: 0o700 });
  const result = spawnSync(process.execPath, [
    resolve('conformance/interactive-alpha/core-provider-proof.mjs'), scratch, resolve('.'),
    join(scratch, 'work'), join(scratch, 'attachment'), output, 'sessions',
  ], { encoding: 'utf8', timeout: 15000 });
  if (result.error) throw result.error;
  assert.equal(result.status, 1, result.stderr);
  assert.match(result.stderr, /EEXIST[^\n]*js-unit-pending-fail-mcp\.json/);
  assert.deepEqual(await readFile(join(output, 'js-unit-initial-mcp.json')), Buffer.from(raw));
});

test('installed runner accepts both provider paths without the MCP environment variable', async t => {
  const scratch = await realpath(await mkdtemp(join(tmpdir(), 'installed-retained-preflight-')));
  t.after(() => rm(scratch, { recursive: true, force: true }));
  const release = join(scratch, 'release');
  await mkdir(release);
  const env = {
    ...process.env,
    CORVINT_RELEASE_EXTRACTED_ROOT: release,
    CORVINT_JS_PROVIDER: join(release, 'js-provider'),
    CORVINT_GO_PROVIDER: join(release, 'go-provider'),
    CORVINT_RELEASE_ARCHIVE: join(scratch, 'release.tar'),
    CORVINT_VSIX: join(scratch, 'outside-release.vsix'),
  };
  delete env.CORVINT_TEST_VALIDITY_MCP;
  for (const file of [env.CORVINT_JS_PROVIDER, env.CORVINT_GO_PROVIDER, env.CORVINT_RELEASE_ARCHIVE, env.CORVINT_VSIX]) {
    await writeFile(file, '', { mode: 0o700 });
  }
  // Redirect the two macOS host paths. Keep all release and provider checks on the real filesystem.
  const preload = join(scratch, 'host-paths.cjs');
  await writeFile(preload, `
const fs = require('node:fs/promises');
const { access, mkdtemp } = fs;
fs.access = (file, mode) => access(file === '/Applications/Visual Studio Code.app/Contents/MacOS/Code' ? process.execPath : file, mode);
fs.mkdtemp = prefix => mkdtemp(prefix === '/private/tmp/corvint-interactive-alpha-' ? __dirname + '/profile-' : prefix);
require('node:module').syncBuiltinESMExports();
`);
  const result = spawnSync(process.execPath, ['--require', preload, resolve('extensions/vscode/test/installed/run.mjs')], {
    env, encoding: 'utf8', timeout: 10000,
  });
  if (result.error) throw result.error;
  assert.equal(result.status, 1, result.stderr);
  // The VSIX confinement check follows successful provider validation and precedes launching Code.
  assert.match(result.stderr, /Error: CORVINT_VSIX is not a file inside CORVINT_RELEASE_EXTRACTED_ROOT/);
});
