import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';

import { checkObservation, loadManifest, parseJsonStrict, runDriver, runSuite, validateManifest } from './run.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const manifest = await loadManifest(path.join(here, 'cases.json'));

test('manifest is well formed and covers every required hostile family', () => {
  assert.deepEqual(validateManifest(manifest), []);
  assert.ok(manifest.cases.length >= 20);
  const families = new Set(manifest.cases.map((item) => item.id.split('-')[2]));
  for (const family of ['ACTIVATION', 'DISCOVERY', 'PIN', 'TRUST', 'BINARY', 'CANCEL', 'TIMEOUT', 'PROCESS', 'OUTPUT', 'JSON', 'HOSTILE', 'REDACTION', 'DIAGNOSTIC', 'DECORATION', 'TESTING', 'AUTHORITY', 'PRIVACY', 'UPDATE', 'RESIDUE', 'MCP', 'ELECTRON']) {
    assert.ok(families.has(family), `missing ${family}`);
  }
});

test('manifest requirement references exactly cover the frozen VS Code specification', async () => {
  const specification = await readFile(path.join(here, '../../docs/specs/vscode-extension-v0.md'), 'utf8');
  const specificationRequirements = new Set([...specification.matchAll(/\*\*(VSC-V0-\d{3})\.\*\*/gu)].map((match) => match[1]));
  const caseRequirements = new Set(manifest.cases.flatMap((item) => item.requirements));
  assert.ok(specificationRequirements.size > 0, 'specification has no normative requirement IDs');
  assert.deepEqual([...caseRequirements].filter((id) => !specificationRequirements.has(id)), [], 'case references unknown requirement IDs');
  assert.deepEqual([...specificationRequirements].filter((id) => !caseRequirements.has(id)), [], 'specification requirement lacks a conformance vector');
});

test('every declarative expectation is enforced by the observation checker', async () => {
  const runner = await readFile(path.join(here, 'run.mjs'), 'utf8');
  const globallyEnforced = new Set(['noShell', 'noPathLookup']);
  const expectationKeys = new Set(manifest.cases.flatMap((item) => Object.keys(item.expect)));
  const unchecked = [...expectationKeys].filter((key) => !globallyEnforced.has(key) && !runner.includes(`expect.${key}`));
  assert.deepEqual(unchecked, []);
});

test('strict JSON parser rejects duplicate decoded member names and surplus values', () => {
  assert.throws(() => parseJsonStrict('{"a":1,"\\u0061":2}'), /duplicate object member/u);
  assert.throws(() => parseJsonStrict('{} {}'), /trailing JSON content/u);
  assert.deepEqual({ ...parseJsonStrict('{"a":[true,false,null,-1.5e2]}') }, { a: [true, false, null, -150] });
  assert.throws(() => parseJsonStrict('[[[0]]]', { maxDepth: 2 }), /JSON depth exceeds/u);
  assert.throws(() => parseJsonStrict('1e999'), /non-finite JSON number/u);
});

test('runner oracle passes every vector', async () => {
  const result = await runSuite({ manifest, driver: path.join(here, 'fixtures/oracle-driver.mjs') });
  assert.equal(result.status, 'NOT_RUN', JSON.stringify(result.failures, null, 2));
  assert.deepEqual(result.failures, []);
  assert.deepEqual(result.notRun.map((item) => item.id), manifest.cases.filter((item) => item.expect.notRunAllowed).map((item) => item.id));
  assert.equal(result.cases.length, manifest.cases.length);
});

test('runner reports NOT_RUN when no VS Code host driver is supplied', async () => {
  const result = await runSuite({ manifest });
  assert.equal(result.status, 'NOT_RUN');
});

test('global process and privacy invariants reject shell, PATH, effects, and secret leakage', () => {
  const testCase = manifest.cases.find((item) => item.id === 'VSC-CONF-REDACTION-001');
  const observation = {
    profile: 'corvint-vscode-host-observation/0',
    caseId: testCase.id,
    result: 'degraded',
    executions: [{ executable: 'corvint', argv: [], shell: true, pathLookup: true }],
    limits: { timeoutMs: 1000, stdoutBytes: 0, stderrBytes: 0, terminated: false, terminationReason: null },
    ui: { status: 'degraded', trees: { evidence: [], impact: [], why: [] }, diagnostics: [], decorations: [], tests: [], output: ['CORVINT-CONFORMANCE-SECRET-7f4c'] },
    effects: { reads: [], network: ['https://example.invalid'], telemetry: [], storage: [], writes: [], authorityChanges: [], installs: [] },
    residue: {},
  };
  const failures = checkObservation(testCase, observation, manifest.limits).join('\n');
  assert.match(failures, /absolute literal executable/u);
  assert.match(failures, /shell execution/u);
  assert.match(failures, /PATH lookup/u);
  assert.match(failures, /forbidden text leaked/u);
  assert.match(failures, /effect network/u);
});

test('Testing API oracle rejects false pass on cancellation', () => {
  const testCase = manifest.cases.find((item) => item.id === 'VSC-CONF-TESTING-002');
  const observation = {
    profile: 'corvint-vscode-host-observation/0',
    caseId: testCase.id,
    result: 'ready',
    executions: [],
    limits: { timeoutMs: 1000, stdoutBytes: 0, stderrBytes: 0, terminated: false, terminationReason: null },
    ui: { status: 'ready', trees: { evidence: [], impact: [], why: [] }, diagnostics: [], decorations: [], tests: [{ id: 'one', status: 'passed' }], testRunProfiles: 0, output: ['cancelled'] },
    effects: { reads: [], network: [], telemetry: [], storage: [], writes: [], authorityChanges: [], installs: [] },
    residue: {},
  };
  assert.ok(checkObservation(testCase, observation, manifest.limits).some((failure) => failure.includes('cancellation status')));
});

test('MCP checker rejects capability, registry, canonical-result, and cancellation weakening', async () => {
  const oracle = path.join(here, 'fixtures/oracle-driver.mjs');
  const canonicalJson = (value) => {
    if (value === null || typeof value !== 'object') return JSON.stringify(value);
    if (Array.isArray(value)) return `[${value.map(canonicalJson).join(',')}]`;
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonicalJson(value[key])}`).join(',')}}`;
  };
  const recanonicalize = (result) => {
    result.content[0].text = canonicalJson(result.structuredContent);
  };

  const discoveryCase = manifest.cases.find((item) => item.id === 'VSC-CONF-MCP-DISCOVERY-001');
  const discovery = await runDriver(oracle, discoveryCase, manifest.limits);
  discovery.mcp.requests[0].params._meta['io.modelcontextprotocol/clientCapabilities'] = { roots: {} };
  assert.ok(checkObservation(discoveryCase, discovery, manifest.limits).some((failure) => failure.includes('metadata')));

  const toolsetCase = manifest.cases.find((item) => item.id === 'VSC-CONF-MCP-TOOLSET-001');
  const toolset = await runDriver(oracle, toolsetCase, manifest.limits);
  toolset.mcp.toolPages[0].tools[0].annotations.openWorldHint = true;
  assert.ok(checkObservation(toolsetCase, toolset, manifest.limits).some((failure) => failure.includes('MCPV0 registry')));

  const callCase = manifest.cases.find((item) => item.id === 'VSC-CONF-MCP-CALL-001');
  const call = await runDriver(oracle, callCase, manifest.limits);
  call.mcp.callResults[0].content[0].text += ' ';
  assert.ok(checkObservation(callCase, call, manifest.limits).some((failure) => failure.includes('canonical duplicate')));

  const laundered = await runDriver(oracle, callCase, manifest.limits);
  laundered.mcp.callResults[0].structuredContent.state = 'ABSTAINED';
  recanonicalize(laundered.mcp.callResults[0]);
  assert.ok(checkObservation(callCase, laundered, manifest.limits).some((failure) => failure.includes('authority correlation')));

  const retainedQuery = await runDriver(oracle, callCase, manifest.limits);
  Object.assign(retainedQuery.mcp.callResults[0].structuredContent, {
    state: 'ABSTAINED', epistemicClass: 'NOT_OBSERVED', authorityClass: 'NONE', abstention: { active: true, reason: 'OUT_OF_SCOPE' },
  });
  recanonicalize(retainedQuery.mcp.callResults[0]);
  assert.ok(checkObservation(callCase, retainedQuery, manifest.limits).some((failure) => failure.includes('authority correlation')));

  const profileDrift = await runDriver(oracle, callCase, manifest.limits);
  profileDrift.mcp.callResults[0].structuredContent.repository.profileId = 'unknown';
  recanonicalize(profileDrift.mcp.callResults[0]);
  assert.ok(checkObservation(callCase, profileDrift, manifest.limits).some((failure) => failure.includes('authority correlation')));

  const digestDrift = await runDriver(oracle, callCase, manifest.limits);
  digestDrift.mcp.callResults[0].structuredContent.repository.dirtyPathsSha256 = 'c'.repeat(40);
  recanonicalize(digestDrift.mcp.callResults[0]);
  assert.ok(checkObservation(callCase, digestDrift, manifest.limits).some((failure) => failure.includes('authority correlation')));

  const serverVersionDrift = await runDriver(oracle, callCase, manifest.limits);
  serverVersionDrift.mcp.callResults[1]._meta['io.modelcontextprotocol/serverInfo'].version = 'different-display-version';
  assert.ok(checkObservation(callCase, serverVersionDrift, manifest.limits).some((failure) => failure.includes('server identity')));

  const cancelCase = manifest.cases.find((item) => item.id === 'VSC-CONF-MCP-CANCEL-001');
  const cancel = await runDriver(oracle, cancelCase, manifest.limits);
  cancel.mcp.cancellation.events.reverse();
  assert.ok(checkObservation(cancelCase, cancel, manifest.limits).some((failure) => failure.includes('cancellation/EOF/escalation')));
});

// These observations are synthetic checker witnesses, never installed-host evidence.
function liveOracle(testCase) {
  return {
    profile: 'corvint-vscode-host-observation/0', caseId: testCase.id, result: 'ready',
    executions: [], limits: {}, effects: {}, ui: { tests: [], diagnostics: [], output: [] },
    liveTests: {
      trusted: testCase.setup.trusted,
      checkpoints: testCase.expect.liveTests.checkpoints.map((point, index) => ({ ...(point.snapshotAbsent === true ? {} : { generation: index + 1 }), output: [], ...structuredClone(point) })),
      events: testCase.expect.liveTests.events.map((event, index) => ({ atMs: index * 250, ...structuredClone(event) })),
    },
  };
}

test('live vectors require every observed checkpoint field and event independently', () => {
  for (const testCase of manifest.cases.filter((item) => item.expect.liveTests)) {
    const observation = liveOracle(testCase);
    assert.deepEqual(checkObservation(testCase, observation, manifest.limits), [], testCase.id);
    for (const [index, point] of testCase.expect.liveTests.checkpoints.entries()) {
      for (const key of Object.keys(point)) {
        const changed = structuredClone(observation);
        delete changed.liveTests.checkpoints[index][key];
        assert.ok(checkObservation(testCase, changed, manifest.limits).length > 0, `${testCase.id} ignored checkpoint ${index}.${key}`);
      }
    }
    for (const [index, event] of testCase.expect.liveTests.events.entries()) {
      for (const key of Object.keys(event)) {
        const changed = structuredClone(observation);
        delete changed.liveTests.events[index][key];
        assert.ok(checkObservation(testCase, changed, manifest.limits).length > 0, `${testCase.id} ignored event ${index}.${key}`);
      }
    }
    const fabricated = structuredClone(observation);
    fabricated.result = 'not-run';
    fabricated.notRunReason = 'host unavailable';
    assert.ok(checkObservation(testCase, fabricated, manifest.limits).some((failure) => failure.includes('fabricated')));
  }
});

test('live conformance rejects false pass, surviving group and unchanged generation', () => {
  const completed = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-COMPLETION'));
  const falsePass = liveOracle(completed);
  falsePass.liveTests.checkpoints[0].items[0].state = 'passed';
  assert.ok(checkObservation(completed, falsePass, manifest.limits).some((failure) => failure.includes('items differs')));
  const residue = liveOracle(completed);
  residue.liveTests.events.find((event) => event.kind === 'group-probe').groupAbsent = false;
  assert.ok(checkObservation(completed, residue, manifest.limits).some((failure) => failure.includes('groupAbsent differs')));
  const save = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-SAVE'));
  const premature = liveOracle(save);
  premature.liveTests.events.at(-1).atMs = premature.liveTests.events.at(-2).atMs + 1;
  assert.ok(checkObservation(save, premature, manifest.limits).some((failure) => failure.includes('debounce')));
  const sameGeneration = liveOracle(save);
  sameGeneration.liveTests.checkpoints.forEach((point) => { point.generation = 1; });
  assert.ok(checkObservation(save, sameGeneration, manifest.limits).some((failure) => failure.includes('generation did not advance')));
});

test('live observation extension is closed and event ordering cannot be reversed', () => {
  const testCase = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-COMPLETION'));
  const changed = liveOracle(testCase);
  changed.liveTests.authority = 'verified';
  assert.ok(checkObservation(testCase, changed, manifest.limits).some((failure) => failure.includes('closed')));
  const reordered = liveOracle(testCase);
  reordered.liveTests.events.reverse();
  assert.ok(checkObservation(testCase, reordered, manifest.limits).some((failure) => failure.includes('monotonic')));
});

test('VSC-V0-077 and VSC-V0-078 checkers reject later stderr lines, lost fallbacks, document opens and dropped lines', async () => {
  const oracle = path.join(here, 'fixtures/oracle-driver.mjs');
  const detailCase = manifest.cases.find((item) => item.id === 'VSC-CONF-PROCESS-004');
  const fallbackCase = manifest.cases.find((item) => item.id === 'VSC-CONF-PROCESS-005');
  const diagnosticCase = manifest.cases.find((item) => item.id === 'VSC-CONF-DIAGNOSTIC-003');
  for (const testCase of [detailCase, fallbackCase, diagnosticCase]) {
    assert.deepEqual(checkObservation(testCase, await runDriver(oracle, testCase, manifest.limits), manifest.limits), []);
  }

  const secondLine = await runDriver(oracle, detailCase, manifest.limits);
  secondLine.failure.message = 'CLI_FAILED: second stderr line must not display';
  const secondLineFailures = checkObservation(detailCase, secondLine, manifest.limits);
  assert.ok(secondLineFailures.some((failure) => failure.includes('expected failure message')));
  assert.ok(secondLineFailures.some((failure) => failure.includes('forbidden text leaked')));
  const codeless = await runDriver(oracle, detailCase, manifest.limits);
  codeless.failure.message = 'CLI detail\ufffd';
  assert.ok(checkObservation(detailCase, codeless, manifest.limits).some((failure) => failure.includes('expected failure message')));
  const lostFallback = await runDriver(oracle, fallbackCase, manifest.limits);
  lostFallback.failure.message = 'CLI_FAILED: ';
  assert.ok(checkObservation(fallbackCase, lostFallback, manifest.limits).some((failure) => failure.includes('expected failure message')));

  const opened = await runDriver(oracle, diagnosticCase, manifest.limits);
  opened.ui.documentOpenCount = 1;
  assert.ok(checkObservation(diagnosticCase, opened, manifest.limits).some((failure) => failure.includes('text-document opens')));
  const clamped = await runDriver(oracle, diagnosticCase, manifest.limits);
  clamped.ui.diagnostics = clamped.ui.diagnostics.filter((item) => item.location.line < 3);
  assert.ok(checkObservation(diagnosticCase, clamped, manifest.limits).some((failure) => failure.includes('expected diagnostic lines')));
  const invalidKept = await runDriver(oracle, diagnosticCase, manifest.limits);
  invalidKept.ui.diagnostics.push({ ...invalidKept.ui.diagnostics[0], id: 'diagnostic:zero', location: { ...invalidKept.ui.diagnostics[0].location, line: 0 } });
  assert.ok(checkObservation(diagnosticCase, invalidKept, manifest.limits).some((failure) => failure.includes('expected diagnostic lines')));
});

test('manifest rejects missing new requirements and unknown numbered intents', () => {
  const changed = structuredClone(manifest);
  changed.cases = changed.cases.filter((item) => !item.requirements.includes('VSC-V0-076'));
  assert.ok(validateManifest(changed).includes('uncovered requirement VSC-V0-076'));
  changed.cases[0].requirements.push('VSC-V0-080');
  assert.ok(validateManifest(changed).includes('unknown requirement VSC-V0-080'));
  const withoutProcessDetail = structuredClone(manifest);
  withoutProcessDetail.cases = withoutProcessDetail.cases.filter((item) => !item.requirements.includes('VSC-V0-077') && !item.requirements.includes('VSC-V0-078'));
  assert.ok(validateManifest(withoutProcessDetail).includes('uncovered requirement VSC-V0-077'));
  assert.ok(validateManifest(withoutProcessDetail).includes('uncovered requirement VSC-V0-078'));
});

test('malformed live checkpoints and events fail without crashing the checker', () => {
  const testCase = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-COMPLETION'));
  for (const [member, value] of [['checkpoints', [null]], ['events', [null]], ['checkpoints', {}], ['events', {}]]) {
    const observation = liveOracle(testCase);
    observation.liveTests[member] = value;
    assert.ok(checkObservation(testCase, observation, manifest.limits).length > 0);
  }
});


test('untrusted live cases cannot pass with a trusted or unobserved host', () => {
  const testCase = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-UNTRUSTED'));
  for (const trusted of [true, undefined]) {
    const observation = liveOracle(testCase);
    observation.liveTests.trusted = trusted;
    assert.ok(checkObservation(testCase, observation, manifest.limits).some((failure) => failure.includes('trust')));
  }
});

test('live snapshot absence is exclusive and optional fields remain typed', () => {
  const completed = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-COMPLETION'));
  const absent = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-UNTRUSTED'));
  for (const [key, value] of [['snapshotAbsent', true], ['documentDigest', 'fake'], ['inputIdentity', 1], ['omitted', -1]]) {
    const observation = liveOracle(completed);
    observation.liveTests.checkpoints[0][key] = value;
    assert.ok(checkObservation(completed, observation, manifest.limits).length > 0, key);
  }
  for (const [key, value] of [['phase', 'completed'], ['generation', 1], ['documentDigest', null], ['inputIdentity', null], ['omitted', 0], ['items', [{ id: 'fake', state: 'passed' }]]]) {
    const observation = liveOracle(absent);
    observation.liveTests.checkpoints[0][key] = value;
    assert.ok(checkObservation(absent, observation, manifest.limits).length > 0, key);
  }
  for (const [key, value] of [['atMs', -0.5], ['argv', [1]], ['env', { PATH: 1 }], ['groupAbsent', 'true']]) {
    const observation = liveOracle(completed);
    observation.liveTests.events[0][key] = value;
    assert.ok(checkObservation(completed, observation, manifest.limits).length > 0, key);
  }
});

test('empty provider command refuses any observed execution', () => {
  const testCase = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-UNAVAILABLE'));
  const observation = liveOracle(testCase);
  observation.executions.push({ executable: '/fixture/provider', argv: [], shell: false, pathLookup: false });
  assert.ok(checkObservation(testCase, observation, manifest.limits).some((failure) => failure.includes('expected 0 executions')));
});

test('in-flight supersession rejects late resurrection and replacement before group cleanup', () => {
  const testCase = manifest.cases.find((item) => item.id.startsWith('VSC-CONF-LIVE-INFLIGHT'));
  const resurrected = liveOracle(testCase);
  resurrected.liveTests.checkpoints[1].items = [{ id: 'addition', state: 'passed' }];
  assert.ok(checkObservation(testCase, resurrected, manifest.limits).some((failure) => failure.includes('items differs')));
  const premature = liveOracle(testCase);
  [premature.liveTests.events[3], premature.liveTests.events[5]] = [premature.liveTests.events[5], premature.liveTests.events[3]];
  assert.ok(checkObservation(testCase, premature, manifest.limits).length > 0);
  const neverEmitted = liveOracle(testCase);
  neverEmitted.liveTests.events.splice(2, 1);
  assert.ok(checkObservation(testCase, neverEmitted, manifest.limits).some((failure) => failure.includes('event count')));
});
