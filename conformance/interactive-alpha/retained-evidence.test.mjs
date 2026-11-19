// SPDX-License-Identifier: Apache-2.0
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, rm, readFile, utimes } from 'node:fs/promises';
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
  return { receipt: { kind, cancelled: false, staleAppBuild: false, tests: [{ name: 'adds', state }] }, testProjections: [{ name: 'adds', state, projection: projection(state.toUpperCase()) }], runProjection: unsupported };
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
