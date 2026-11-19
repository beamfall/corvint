// SPDX-License-Identifier: Apache-2.0
// Qualification-only reader for the retained foreground fixture providers.
const assert = require('node:assert/strict');
const { readdir, open } = require('node:fs/promises');
const { constants } = require('node:fs');
const { join } = require('node:path');

function closed(value, keys) {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value), 'expected provider object');
  assert.ok(Object.keys(value).every(key => keys.includes(key)), 'unsupported provider member');
}

function projection(value, execution, stale = false) {
  closed(value, ['association', 'hygiene', 'execution', 'freshness', 'strength']);
  assert.ok(value && value.execution && value.freshness && value.strength, 'missing provider axes');
  assert.equal(value.execution.state, execution, 'provider execution disagrees with observation');
  assert.equal(value.strength.state, 'NOT_MEASURED', 'provider promoted unmeasured strength');
  assert.ok((stale ? ['STALE'] : ['CURRENT', 'UNKNOWN']).includes(value.freshness.state), 'unsupported provider freshness');
  // Reading retained bytes does not establish currency against current inputs.
  return { ...value, freshness: { ...value.freshness, state: stale ? 'STALE' : 'UNKNOWN' } };
}

function decode(raw) {
  const value = JSON.parse(raw);
  assert.ok(value && !Array.isArray(value), 'provider must be an object');
  let kind, source, run, tests, extra = {};
  if (value.profile === 'corvint-go-live-session-event/0' && value.receipt === undefined) {
    closed(value, ['profile', 'state', 'identity', 'sequence', 'scope', 'detail', 'projection', 'testProjections', 'testProjectionsOmitted']);
    assert.ok(['passed', 'failed', 'stale'].includes(value.state), 'unsupported Go terminal state');
    assert.ok(typeof value.identity === 'string' && value.identity.length > 0, 'missing Go identity');
    assert.ok(Array.isArray(value.scope) && value.scope.length > 0 && value.scope.every(s => typeof s === 'string'), 'missing Go scope');
    assert.equal(value.testProjectionsOmitted, 0, 'Go tests omitted');
    const stale = value.state === 'stale';
    run = projection(value.projection, stale ? 'INFRASTRUCTURE' : value.state.toUpperCase(), stale);
    assert.ok(Array.isArray(value.testProjections) && value.testProjections.length > 0, 'missing Go tests');
    tests = value.testProjections.map(test => {
      closed(test, ['package', 'name', 'action', 'projection']);
      const execution = new Map([['pass', 'PASSED'], ['fail', 'FAILED'], ['skip', 'SKIPPED'], ['none', 'UNSUPPORTED'], ['', 'UNSUPPORTED']]).get(test.action);
      assert.ok(execution && typeof test.package === 'string' && typeof test.name === 'string', 'unsupported Go test observation');
      return { ...test, projection: projection(test.projection, execution, stale) };
    });
    kind = 'go-session'; source = 'corvint-go-test-provider'; extra = { tier: 'preview', promotable: false };
  } else if (value.profile === undefined && value.receipt &&
      (value.receipt.profile === undefined || value.receipt.profile === '') &&
      ['unit', 'e2e'].includes(value.receipt.kind)) {
    closed(value, ['receipt', 'testProjections', 'runProjection']);
    closed(value.receipt, ['profile', 'kind', 'identity', 'appBuildAtStart', 'appBuildAtPublish', 'staleAppBuild', 'tests', 'cancelled', 'infrastructure', 'serverDescendantsGone', 'descendantObservation', 'runnerResources', 'schedule', 'testRepositoryAtStart', 'testRepositoryAtPublish']);
    assert.equal(value.receipt.cancelled, false, 'cancelled JS receipt');
    assert.equal(value.receipt.staleAppBuild, false, 'stale JS receipt');
    assert.ok(value.receipt.infrastructure == null, 'JS infrastructure failure');
    kind = value.receipt.kind; source = 'corvint-js-test-provider';
    assert.ok(Array.isArray(value.receipt.tests) && value.receipt.tests.length > 0, 'missing JS observations');
    assert.ok(Array.isArray(value.testProjections), 'missing JS projections');
    assert.equal(value.testProjections.length, value.receipt.tests.length, 'JS tests omitted');
    tests = value.testProjections.map((test, i) => {
      closed(test, ['name', 'state', 'projection']);
      const observed = value.receipt.tests[i];
      closed(observed, ['id', 'project', 'attempts', 'name', 'fullName', 'state', 'retries', 'durationMs', 'durationMissing', 'anchor', 'failureMessage', 'artifacts', 'attemptDetails']);
      assert.ok(['passed', 'failed', 'skipped'].includes(observed.state), 'unsupported JS observation');
      assert.equal(test.name, observed.name, 'JS test identity differs');
      assert.equal(test.state, observed.state, 'JS test state differs');
      return { ...test, projection: projection(test.projection, observed.state.toUpperCase()) };
    });
    // An ordinary JS receipt has no run-level execution observation.
    closed(value.runProjection, ['association', 'hygiene', 'freshness', 'execution', 'strength']);
    for (const axis of ['association', 'hygiene', 'freshness', 'execution', 'strength']) {
      assert.equal(value.runProjection[axis]?.state, 'UNSUPPORTED', 'JS run promoted absent observation');
    }
    run = value.runProjection;
  } else {
    throw new Error('unsupported retained provider form');
  }
  assert.equal(new Set(tests.map(t => `${t.package ?? ''}#${t.name}`)).size, tests.length, 'duplicate provider tests');
  return { source, kind, run, tests, ...extra };
}

async function readRetainedEvidence(root) {
  const directory = join(root, '.corvint/test-evidence');
  const names = await readdir(directory);
  assert.ok(names.length <= 256, 'retained evidence exceeds discovery bound');
  const rows = [];
  for (const name of names) {
    // Retain writes this exact private temporary form before its atomic rename.
    if (/^\.corvint-(?:js|go)-test-provider-[0-9]+-[0-9a-f]+\.tmp$/.test(name)) continue;
    assert.match(name, /^corvint-(?:js|go)-test-provider-[0-9]+-[0-9a-f]+\.json$/, 'unsupported retained evidence name');
    const file = await open(join(directory, name), constants.O_RDONLY | constants.O_NOFOLLOW);
    try {
      const info = await file.stat();
      assert.ok(info.isFile() && info.size > 0 && info.size <= 262144, 'retained evidence must be a bounded regular file');
      const buffer = Buffer.alloc(262145);
      const { bytesRead } = await file.read(buffer, 0, buffer.length, 0);
      assert.ok(bytesRead === info.size && bytesRead <= 262144, 'retained evidence changed or exceeds byte bound');
      const raw = buffer.subarray(0, bytesRead);
      const document = decode(raw);
      rows.push({ raw, document, name, time: info.mtimeMs });
    } finally { await file.close(); }
  }
  rows.sort((a, b) => a.time - b.time || a.name.localeCompare(b.name));
  assert.ok(rows.length > 0, 'no retained evidence');
  const selected = rows.at(-1);
  selected.document.discovery = { evidence: '.corvint/test-evidence/' + selected.name, freshness: { state: 'UNKNOWN' } };
  return selected;
}

async function selectedEvidence(config, child, expectedRow, expectedState) {
  const response = await readRetainedEvidence(config.dir), doc = response.document;
  assert.ok(doc.discovery.evidence.endsWith('/' + expectedRow.name), 'reader did not select original retained document');
  assert.equal(doc.discovery.freshness.state, 'UNKNOWN', 'reader promoted unmeasured freshness');
  assert.ok(doc.tests.length > 0 && doc.tests.every(t => t.projection.execution.state === expectedState.toUpperCase() && t.projection.freshness.state === 'UNKNOWN' && t.projection.strength.state === 'NOT_MEASURED'), 'reader axes differ from actual execution');
  if (config.kind === 'go') assert.ok(doc.tier === 'preview' && doc.promotable === false, 'Go tier promoted');
  assert.ok(child.stdout.includes(expectedRow.raw), 'retention did not preserve stdout bytes');
  assert.deepEqual(response.raw, expectedRow.raw, 'retained bytes changed after observation');
  return response;
}

module.exports = { readRetainedEvidence, selectedEvidence, decode };
