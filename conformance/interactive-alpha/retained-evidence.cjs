// SPDX-License-Identifier: Apache-2.0
// Qualification-only reader for the retained foreground fixture providers.
const assert = require('node:assert/strict');
const { opendir, open } = require('node:fs/promises');
const { constants } = require('node:fs');
const { join } = require('node:path');

function closed(value, keys) {
  assert.ok(value && typeof value === 'object' && !Array.isArray(value), 'expected provider object');
  assert.ok(Object.keys(value).every(key => keys.includes(key)), 'unsupported provider member');
}

const nonempty = value => typeof value === 'string' && value.trim().length > 0;
const digest = value => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);

function validateJSIdentity(identity, kind) {
  closed(identity, ['testFileDigests', 'configFile', 'configDigest', 'packageDigest', 'nodeVersion',
    'runnerName', 'runnerVersion', 'environment', 'argv', 'configInputDigests']);
  assert.ok(nonempty(identity.configFile) && digest(identity.configDigest) && digest(identity.packageDigest),
    'missing JS config or package identity');
  assert.equal(identity.runnerName, kind === 'unit' ? 'vitest' : 'playwright', 'unexpected JS runner');
  assert.ok(nonempty(identity.nodeVersion) && nonempty(identity.runnerVersion), 'missing JS runtime identity');
  assert.ok(Array.isArray(identity.argv) && identity.argv.length > 0 && identity.argv.every(nonempty), 'missing JS argv');
  assert.ok(identity.environment && typeof identity.environment === 'object' && !Array.isArray(identity.environment) &&
    Object.entries(identity.environment).every(([key, value]) => nonempty(key) && typeof value === 'string'),
    'invalid JS environment');
  validateFileDigests(identity.testFileDigests);
  if (Object.hasOwn(identity, 'configInputDigests')) validateFileDigests(identity.configInputDigests);
}

function validateFileDigests(files) {
  assert.ok(files && typeof files === 'object' && !Array.isArray(files) && Object.keys(files).length > 0 &&
    Object.entries(files).every(([path, value]) => nonempty(path) && digest(value)), 'missing JS file digests');
}

// Only the Vitest and provider-owned Playwright JSON reporters run in these fixtures.
// Other receipt profiles and their optional instrumentation need their own validation.
const terminalStates = ['passed', 'failed', 'skipped'];
const count = value => Number.isSafeInteger(value) && value >= 0;
const duration = value => Number.isFinite(value) && value >= 0;

function validateBuild(build) {
  closed(build, ['digest', 'unknown', 'reason']);
  assert.ok(typeof build.unknown === 'boolean' && (build.digest === '' || digest(build.digest)), 'invalid app build');
  if (Object.hasOwn(build, 'reason')) assert.equal(typeof build.reason, 'string');
}

function validateFailure(row) {
  if (Object.hasOwn(row, 'failureMessage')) assert.equal(typeof row.failureMessage, 'string');
  if (Object.hasOwn(row, 'anchor')) {
    closed(row.anchor, ['file', 'line']);
    assert.ok(nonempty(row.anchor.file) && count(row.anchor.line) && row.anchor.line > 0, 'invalid source anchor');
  }
  if (Object.hasOwn(row, 'artifacts')) {
    assert.ok(Array.isArray(row.artifacts), 'invalid failure artifacts');
    for (const artifact of row.artifacts) {
      closed(artifact, ['name', 'path']);
      assert.ok(nonempty(artifact.name) && nonempty(artifact.path), 'invalid failure artifact');
    }
  }
}

function validateObservation(row, kind) {
  const keys = ['name', 'fullName', 'state', 'retries', 'durationMs', 'anchor', 'failureMessage'];
  closed(row, keys.concat(kind === 'unit' ? ['durationMissing'] : ['artifacts', 'attemptDetails']));
  assert.ok(nonempty(row.name) && typeof row.fullName === 'string', 'invalid JS test name');
  assert.ok(terminalStates.includes(row.state), 'unsupported JS observation');
  assert.ok(count(row.retries) && duration(row.durationMs), 'invalid JS test retries or duration');
  if (Object.hasOwn(row, 'durationMissing')) {
    assert.ok(row.durationMissing === true && row.durationMs === 0, 'invalid missing duration');
  }
  validateFailure(row);
  if (Object.hasOwn(row, 'attemptDetails')) {
    assert.ok(Array.isArray(row.attemptDetails) && row.attemptDetails.length > 0, 'invalid JS attempts');
    for (const [index, attempt] of row.attemptDetails.entries()) {
      closed(attempt, ['state', 'retry', 'durationMs', 'failureMessage', 'anchor', 'artifacts']);
      assert.ok(terminalStates.includes(attempt.state) && attempt.retry === index && duration(attempt.durationMs),
        'invalid JS attempt');
      validateFailure(attempt);
    }
    const last = row.attemptDetails.at(-1);
    assert.ok(last.retry === row.retries && last.durationMs === row.durationMs && last.state === row.state,
      'JS attempt differs from observed result');
    assert.ok(row.state !== 'passed' || row.attemptDetails.every(a => a.state === 'passed'),
      'JS pass after an unsuccessful attempt');
  }
}

// Each fixture provider emits these five native Axes, including null anchors when unbound.
function validateAxes(value, states) {
  closed(value, Object.keys(states));
  for (const [name, expected] of Object.entries(states)) {
    const axis = value[name];
    closed(axis, ['state', 'reason', 'anchors']);
    assert.equal(axis.state, expected, 'unsupported provider ' + name);
    assert.equal(typeof axis.reason, 'string', 'missing provider axis reason');
    assert.ok(axis.anchors === null || (Array.isArray(axis.anchors) && axis.anchors.every(a => typeof a === 'string')),
      'unsupported provider axis anchors');
  }
}

function projection(value, execution, provider, stale = false) {
  const js = provider === 'js';
  validateAxes(value, {
    association: js ? 'ASSOCIATED' : 'UNSUPPORTED',
    hygiene: js ? 'ELIGIBLE' : 'UNSUPPORTED',
    execution,
    freshness: js ? 'UNKNOWN' : stale ? 'STALE' : 'CURRENT',
    strength: 'NOT_MEASURED',
  });
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
    assert.ok(nonempty(value.identity), 'missing Go identity');
    assert.ok(Array.isArray(value.scope) && value.scope.length > 0 && value.scope.every(nonempty), 'missing Go scope');
    assert.equal(value.testProjectionsOmitted, 0, 'Go tests omitted');
    assert.ok(count(value.sequence) && typeof value.detail === 'string', 'invalid Go event sequence or detail');
    const stale = value.state === 'stale';
    run = projection(value.projection, stale ? 'INFRASTRUCTURE' : value.state.toUpperCase(), 'go', stale);
    assert.ok(Array.isArray(value.testProjections) && value.testProjections.length > 0, 'missing Go tests');
    tests = value.testProjections.map(test => {
      closed(test, ['package', 'name', 'action', 'projection']);
      const execution = new Map([['pass', 'PASSED'], ['fail', 'FAILED'], ['skip', 'SKIPPED'], ['none', 'UNSUPPORTED'], ['', 'UNSUPPORTED']]).get(test.action);
      assert.ok(execution && nonempty(test.package) && nonempty(test.name), 'unsupported Go test observation');
      return { ...test, projection: projection(test.projection, execution, 'go', stale) };
    });
    kind = 'go-session'; source = 'corvint-go-test-provider'; extra = { tier: 'preview', promotable: false };
  } else if (value.profile === undefined && value.receipt &&
      (value.receipt.profile === undefined || value.receipt.profile === '') &&
      ['unit', 'e2e'].includes(value.receipt.kind)) {
    closed(value, ['receipt', 'testProjections', 'runProjection']);
    const keys = ['profile', 'kind', 'identity', 'appBuildAtStart', 'appBuildAtPublish', 'staleAppBuild', 'tests', 'cancelled'];
    closed(value.receipt, value.receipt.kind === 'e2e' ? [...keys, 'serverDescendantsGone'] : keys);
    validateBuild(value.receipt.appBuildAtStart);
    validateBuild(value.receipt.appBuildAtPublish);
    assert.deepEqual(value.receipt.appBuildAtStart, value.receipt.appBuildAtPublish, 'JS build changed');
    if (value.receipt.kind === 'e2e') assert.equal(value.receipt.serverDescendantsGone, true, 'unproven server cleanup');
    validateJSIdentity(value.receipt.identity, value.receipt.kind);
    assert.equal(value.receipt.cancelled, false, 'cancelled JS receipt');
    assert.equal(value.receipt.staleAppBuild, false, 'stale JS receipt');
    kind = value.receipt.kind; source = 'corvint-js-test-provider';
    assert.ok(Array.isArray(value.receipt.tests) && value.receipt.tests.length > 0, 'missing JS observations');
    assert.ok(Array.isArray(value.testProjections), 'missing JS projections');
    assert.equal(value.testProjections.length, value.receipt.tests.length, 'JS tests omitted');
    tests = value.testProjections.map((test, i) => {
      closed(test, ['name', 'state', 'projection']);
      const observed = value.receipt.tests[i];
      validateObservation(observed, kind);
      assert.equal(test.name, observed.name, 'JS test identity differs');
      assert.equal(test.state, observed.state, 'JS test state differs');
      return { ...test, projection: projection(test.projection, observed.state.toUpperCase(), 'js') };
    });
    // An ordinary JS receipt has no run-level execution observation.
    validateAxes(value.runProjection, {
      association: 'UNSUPPORTED', hygiene: 'UNSUPPORTED', execution: 'UNSUPPORTED',
      freshness: 'UNSUPPORTED', strength: 'UNSUPPORTED',
    });
    run = value.runProjection;
  } else {
    throw new Error('unsupported retained provider form');
  }
  assert.equal(new Set(tests.map(t => `${t.package ?? ''}#${t.name}`)).size, tests.length, 'duplicate provider tests');
  return { source, kind, run, tests, ...extra };
}

async function retainedDocuments(root) {
  const directory = join(root, '.corvint/test-evidence');
  let entries;
  try { entries = await opendir(directory); } catch (error) {
    if (error.code === 'ENOENT') return [];
    throw error;
  }
  let scanned = 0;
  const rows = [];
  for await (const { name } of entries) {
    assert.ok(++scanned <= 256, 'retained evidence exceeds discovery bound');
    if (name === '.gitignore') continue; // IgnoreRetained writes this when .corvint already exists.
    // Retain writes this exact private temporary form before its atomic rename.
    if (/^\.corvint-(?:js|go)-test-provider-[0-9]+-[0-9a-f]+\.tmp$/.test(name)) continue;
    assert.match(name, /^corvint-(?:js|go)-test-provider-[0-9]+-[0-9a-f]+\.json$/, 'unsupported retained evidence name');
    const file = await open(join(directory, name), constants.O_RDONLY | constants.O_NONBLOCK | constants.O_NOFOLLOW);
    try {
      const info = await file.stat();
      assert.ok(info.isFile() && info.size > 0 && info.size <= 262144, 'retained evidence must be a bounded regular file');
      const buffer = Buffer.alloc(info.size + 1);
      const { bytesRead } = await file.read(buffer, 0, buffer.length, 0);
      assert.ok(bytesRead === info.size && bytesRead <= 262144, 'retained evidence changed or exceeds byte bound');
      const raw = buffer.subarray(0, bytesRead);
      const document = decode(raw);
      assert.ok(name.startsWith(document.source + '-'), 'retained name differs from provider');
      rows.push({ raw, document, name, time: info.mtimeMs });
    } finally { await file.close(); }
  }
  return rows.sort((a, b) => a.time - b.time || a.name.localeCompare(b.name));
}

async function readRetainedEvidence(root) {
  const rows = await retainedDocuments(root);
  assert.ok(rows.length > 0, 'no retained evidence');
  const selected = rows.at(-1);
  selected.document.discovery = { evidence: '.corvint/test-evidence/' + selected.name, freshness: { state: 'UNKNOWN' } };
  return selected;
}

async function selectedEvidence(config, child, expectedRow, expectedState) {
  const response = await readRetainedEvidence(config.dir), doc = response.document;
  const expectedKind = new Map([['go', 'go-session'], ['unit', 'unit'], ['e2e', 'e2e']]).get(config.kind);
  assert.ok(expectedKind, 'unsupported invocation kind');
  assert.equal(doc.kind, expectedKind, 'receipt kind differs from invocation');
  assert.ok(doc.discovery.evidence.endsWith('/' + expectedRow.name), 'reader did not select original retained document');
  assert.equal(doc.discovery.freshness.state, 'UNKNOWN', 'reader promoted unmeasured freshness');
  assert.ok(doc.tests.length > 0 && doc.tests.every(t => t.projection.execution.state === expectedState.toUpperCase() && t.projection.freshness.state === 'UNKNOWN' && t.projection.strength.state === 'NOT_MEASURED'), 'reader axes differ from actual execution');
  if (config.kind === 'go') assert.ok(doc.tier === 'preview' && doc.promotable === false, 'Go tier promoted');
  assert.ok(child.stdout.includes(expectedRow.raw), 'retention did not preserve stdout bytes');
  assert.deepEqual(response.raw, expectedRow.raw, 'retained bytes changed after observation');
  return response;
}

module.exports = { retainedDocuments, readRetainedEvidence, selectedEvidence, decode };
