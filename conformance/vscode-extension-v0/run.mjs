#!/usr/bin/env node

import { spawn } from 'node:child_process';
import { checkLiveObservation } from './live-observation.mjs';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const DEFAULT_CASES = path.join(HERE, 'cases.json');
const DRIVER_PROFILE = 'corvint-vscode-host-observation/0';
const activeProcessGroups = new Set();

function terminateProcessGroup(pid, signal = 'SIGTERM') {
  if (!pid) return;
  try {
    process.kill(process.platform === 'win32' ? pid : -pid, signal);
  } catch (error) {
    if (error?.code !== 'ESRCH') throw error;
  }
}

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.once(signal, () => {
    for (const pid of activeProcessGroups) terminateProcessGroup(pid, 'SIGTERM');
    process.exit(128 + (signal === 'SIGINT' ? 2 : 15));
  });
}

export function parseJsonStrict(source, { maxBytes = Number.POSITIVE_INFINITY, maxDepth = 64, maxNodes = 100_000 } = {}) {
  if (Buffer.byteLength(source, 'utf8') > maxBytes) throw new Error(`JSON exceeds ${maxBytes} bytes`);
  let offset = 0;
  let nodes = 0;
  const fail = (message) => {
    throw new Error(`${message} at byte ${Buffer.byteLength(source.slice(0, offset), 'utf8')}`);
  };
  const whitespace = () => {
    while ([' ', '\n', '\r', '\t'].includes(source[offset])) offset += 1;
  };
  const string = () => {
    if (source[offset] !== '"') fail('expected string');
    const start = offset++;
    let escaped = false;
    while (offset < source.length) {
      const character = source[offset++];
      if (escaped) {
        escaped = false;
      } else if (character === '\\') {
        escaped = true;
      } else if (character === '"') {
        return JSON.parse(source.slice(start, offset));
      } else if (character.charCodeAt(0) < 0x20) {
        fail('unescaped control character');
      }
    }
    fail('unterminated string');
  };
  const value = (depth = 0) => {
    if (depth > maxDepth) fail(`JSON depth exceeds ${maxDepth}`);
    nodes += 1;
    if (nodes > maxNodes) fail(`JSON nodes exceed ${maxNodes}`);
    whitespace();
    const character = source[offset];
    if (character === '"') return string();
    if (character === '{') {
      offset += 1;
      const object = Object.create(null);
      const keys = new Set();
      whitespace();
      if (source[offset] === '}') {
        offset += 1;
        return object;
      }
      while (offset < source.length) {
        whitespace();
        const key = string();
        if (keys.has(key)) fail(`duplicate object member ${JSON.stringify(key)}`);
        keys.add(key);
        whitespace();
        if (source[offset++] !== ':') fail('expected colon');
        object[key] = value(depth + 1);
        whitespace();
        const delimiter = source[offset++];
        if (delimiter === '}') return object;
        if (delimiter !== ',') fail('expected comma or object end');
      }
      fail('unterminated object');
    }
    if (character === '[') {
      offset += 1;
      const array = [];
      whitespace();
      if (source[offset] === ']') {
        offset += 1;
        return array;
      }
      while (offset < source.length) {
        array.push(value(depth + 1));
        whitespace();
        const delimiter = source[offset++];
        if (delimiter === ']') return array;
        if (delimiter !== ',') fail('expected comma or array end');
      }
      fail('unterminated array');
    }
    for (const [literal, parsed] of [['true', true], ['false', false], ['null', null]]) {
      if (source.startsWith(literal, offset)) {
        offset += literal.length;
        return parsed;
      }
    }
    const number = source.slice(offset).match(/^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/u)?.[0];
    if (number) {
      offset += number.length;
      const parsed = Number(number);
      if (!Number.isFinite(parsed)) fail('non-finite JSON number');
      return parsed;
    }
    fail('invalid JSON value');
  };
  const parsed = value();
  whitespace();
  if (offset !== source.length) fail('trailing JSON content');
  return parsed;
}

export async function loadManifest(filename = DEFAULT_CASES) {
  const raw = await readFile(filename, 'utf8');
  return parseJsonStrict(raw, { maxBytes: 1024 * 1024 });
}

export function validateManifest(manifest) {
  const failures = [];
  if (manifest?.profile !== 'corvint-vscode-conformance-cases/0') failures.push('invalid case profile');
  if (!Array.isArray(manifest?.cases) || manifest.cases.length === 0) failures.push('cases must be non-empty');
  const ids = new Set();
  const requirements = new Set();
  for (const testCase of manifest?.cases ?? []) {
    if (!/^VSC-CONF-[A-Z]+(?:-[A-Z]+)*-\d{3}$/u.test(testCase.id ?? '')) failures.push(`invalid case id ${testCase.id}`);
    if (ids.has(testCase.id)) failures.push(`duplicate case id ${testCase.id}`);
    ids.add(testCase.id);
    if (!Array.isArray(testCase.requirements) || testCase.requirements.length === 0) failures.push(`${testCase.id}: missing requirements`);
    for (const requirement of testCase.requirements ?? []) {
      requirements.add(requirement);
      if (!/^VSC-V0-(?:00[1-9]|0[1-6][0-9]|07[0-6])$/u.test(requirement)) failures.push(`unknown requirement ${requirement}`);
    }
    if (!testCase.setup || !testCase.action || !testCase.expect) failures.push(`${testCase.id}: incomplete vector`);
    if (testCase.action?.kind === 'import-observation' && testCase.expect?.result === 'ready' && testCase.setup?.sharedCorvintVerifier !== 'available') failures.push(`${testCase.id}: positive observation projection lacks an explicit shared Corvint verifier`);
  }
  for (let ordinal = 1; ordinal <= 76; ordinal += 1) {
    const requirement = `VSC-V0-${String(ordinal).padStart(3, '0')}`;
    if (!requirements.has(requirement)) failures.push(`uncovered requirement ${requirement}`);
  }
  for (const key of ['automaticTimeoutMs', 'maxStdoutBytes', 'maxStderrBytes', 'maxUiTextBytes']) {
    if (!Number.isSafeInteger(manifest?.limits?.[key]) || manifest.limits[key] <= 0) failures.push(`invalid limit ${key}`);
  }
  return failures;
}

function appendBounded(chunks, chunk, state, maximum, streamName, child) {
  state.bytes += chunk.length;
  if (state.bytes <= maximum) {
    chunks.push(chunk);
    return;
  }
  state.overflow = `${streamName} exceeded ${maximum} bytes`;
  terminateProcessGroup(child.pid, 'SIGTERM');
}

export async function runDriver(driver, testCase, limits, { timeoutMs = 5000 } = {}) {
  const absoluteDriver = path.resolve(driver);
  const javascript = /\.(?:c|m)?js$/u.test(absoluteDriver);
  const executable = javascript ? process.execPath : absoluteDriver;
  const argv = javascript ? [absoluteDriver] : [];
  const child = spawn(executable, argv, {
    detached: process.platform !== 'win32',
    env: { ...process.env, CORVINT_VSCODE_CONFORMANCE: '1' },
    shell: false,
    stdio: ['pipe', 'pipe', 'pipe'],
  });
  activeProcessGroups.add(child.pid);
  const stdout = [];
  const stderr = [];
  const stdoutState = { bytes: 0, overflow: null };
  const stderrState = { bytes: 0, overflow: null };
  child.stdout.on('data', (chunk) => appendBounded(stdout, chunk, stdoutState, limits.maxStdoutBytes, 'driver stdout', child));
  child.stderr.on('data', (chunk) => appendBounded(stderr, chunk, stderrState, limits.maxStderrBytes, 'driver stderr', child));
  child.stdin.end(`${JSON.stringify(testCase)}\n`);
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    terminateProcessGroup(child.pid, 'SIGTERM');
    setTimeout(() => terminateProcessGroup(child.pid, 'SIGKILL'), 250).unref();
  }, timeoutMs);
  const exit = await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('close', (code, signal) => resolve({ code, signal }));
  }).finally(() => {
    clearTimeout(timer);
    activeProcessGroups.delete(child.pid);
  });
  const stderrText = Buffer.concat(stderr).toString('utf8');
  if (timedOut) throw new Error(`driver deadline exceeded ${timeoutMs} ms`);
  if (stdoutState.overflow) throw new Error(stdoutState.overflow);
  if (stderrState.overflow) throw new Error(stderrState.overflow);
  if (exit.code !== 0) throw new Error(`driver exited ${exit.code ?? exit.signal}: ${stderrText.slice(0, 1000)}`);
  return parseJsonStrict(Buffer.concat(stdout).toString('utf8'), { maxBytes: limits.maxStdoutBytes });
}

function allStrings(value, result = []) {
  if (typeof value === 'string') result.push(value);
  else if (Array.isArray(value)) value.forEach((item) => allStrings(item, result));
  else if (value && typeof value === 'object') Object.values(value).forEach((item) => allStrings(item, result));
  return result;
}

function entriesWithLocations(ui) {
  return ['diagnostics', 'decorations', 'tests']
    .flatMap((key) => Array.isArray(ui?.[key]) ? ui[key] : [])
    .filter((entry) => entry?.location != null);
}

function isSafeUiText(text) {
  return !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/u.test(text)
    && !/\u001b\[[0-?]*[ -/]*[@-~]/u.test(text);
}

function assertCondition(failures, condition, message) {
  if (!condition) failures.push(message);
}

const MCP_PROTOCOL_VERSION = '2026-07-28';
const MCP_SERVER_INFO = Object.freeze({
  description: 'Local read-only Corvint context and repository evidence server.',
  name: 'corvint-mcp',
});

function validMcpServerInfo(info) {
  return info?.name === MCP_SERVER_INFO.name
    && info?.description === MCP_SERVER_INFO.description
    && typeof info?.version === 'string'
    && Buffer.byteLength(info.version, 'utf8') >= 1
    && Buffer.byteLength(info.version, 'utf8') <= 64
    && /^[\x20-\x7e]+$/u.test(info.version);
}
const MCP_ANNOTATIONS = Object.freeze({
  readOnlyHint: true,
  destructiveHint: false,
  idempotentHint: true,
  openWorldHint: false,
});

function canonicalJson(value) {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(',')}]`;
  return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonicalJson(value[key])}`).join(',')}}`;
}

function expectedMcpTools() {
  const schema = (properties, required) => ({
    '$schema': 'https://json-schema.org/draft/2020-12/schema',
    type: 'object', properties, required, additionalProperties: false,
  });
  return [
    {
      name: 'corvint.impact',
      description: 'Compile revision-bound impact evidence for tracked Go files without returning source bodies.',
      inputSchema: schema({
        paths: {
          type: 'array', minItems: 1, maxItems: 100, uniqueItems: true,
          items: { type: 'string', minLength: 1, maxLength: 1024, pattern: '^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\\\).+[.]go$' },
        },
        limit: { type: 'integer', minimum: 1, maximum: 50, default: 10 },
      }, ['paths']),
      annotations: MCP_ANNOTATIONS,
    },
    {
      name: 'corvint.query',
      description: 'Compile the narrow project-operations authority-start receipt (limit is fixed at one).',
      inputSchema: schema({ task: { type: 'string', minLength: 1, maxLength: 2000, pattern: '^[ -~]+$' } }, ['task']),
      annotations: MCP_ANNOTATIONS,
    },
    {
      name: 'corvint.status',
      description: 'Observe Git commit, tree, worktree state, and a privacy-preserving dirty-path digest.',
      inputSchema: schema({}, []),
      annotations: MCP_ANNOTATIONS,
    },
  ];
}

function validMcpMeta(params, extensionVersion) {
  const meta = params?._meta;
  return canonicalJson(meta) === canonicalJson({
    'io.modelcontextprotocol/protocolVersion': MCP_PROTOCOL_VERSION,
    'io.modelcontextprotocol/clientInfo': { name: 'corvint-vscode', version: extensionVersion },
    'io.modelcontextprotocol/clientCapabilities': {},
  });
}

function validMcpDiscovery(result) {
  return result?.resultType === 'complete'
    && canonicalJson(result.supportedVersions) === canonicalJson([MCP_PROTOCOL_VERSION])
    && canonicalJson(result.capabilities) === canonicalJson({ tools: { listChanged: false } })
    && result.ttlMs === 0
    && result.cacheScope === 'public'
    && validMcpServerInfo(result?._meta?.['io.modelcontextprotocol/serverInfo'])
    && Object.keys(result).sort().join(',') === '_meta,cacheScope,capabilities,resultType,supportedVersions,ttlMs';
}

function validMcpToolPages(pages) {
  if (!Array.isArray(pages) || pages.length !== 1) return false;
  const cursors = new Set();
  const tools = [];
  for (const page of pages) {
    if (page?.resultType !== 'complete' || page?.ttlMs !== 300000 || page?.cacheScope !== 'private' || !Array.isArray(page.tools) || page.tools.length > 64) return false;
    if (!validMcpServerInfo(page?._meta?.['io.modelcontextprotocol/serverInfo'])) return false;
    tools.push(...page.tools);
    if (page.nextCursor != null) return false;
  }
  return tools.length <= 256 && canonicalJson(tools) === canonicalJson(expectedMcpTools());
}

function validMcpRepository(repository) {
  if (repository == null || typeof repository !== 'object' || Array.isArray(repository)) return false;
  if (Object.keys(repository).sort().join(',') !== 'commitRevision,dirtyPathCount,dirtyPathsSha256,objectFormat,profileId,treeRevision,worktreeState') return false;
  if (!['generic', 'beamfall'].includes(repository.profileId) || !['sha1', 'sha256'].includes(repository.objectFormat) || !['CLEAN', 'MIXED'].includes(repository.worktreeState)) return false;
  const revisionWidth = repository.objectFormat === 'sha1' ? 40 : 64;
  if (!new RegExp(`^[a-f0-9]{${revisionWidth}}$`, 'u').test(repository.commitRevision ?? '') ||
    !new RegExp(`^[a-f0-9]{${revisionWidth}}$`, 'u').test(repository.treeRevision ?? '') ||
    !/^[a-f0-9]{64}$/u.test(repository.dirtyPathsSha256 ?? '')) return false;
  if (!Number.isSafeInteger(repository.dirtyPathCount) || repository.dirtyPathCount < 0 || repository.dirtyPathCount > 1000000) return false;
  return repository.worktreeState !== 'CLEAN' || repository.dirtyPathCount === 0;
}

function validMcpBridgeResult(callResult, expectedCall) {
  if (callResult?.resultType !== 'complete' || callResult?.isError !== false || !Array.isArray(callResult.content) || callResult.content.length !== 1) return false;
  if (!validMcpServerInfo(callResult?._meta?.['io.modelcontextprotocol/serverInfo'])) return false;
  const block = callResult.content[0];
  if (block?.type !== 'text' || typeof block.text !== 'string' || callResult.structuredContent == null) return false;
  let textObject;
  try {
    textObject = parseJsonStrict(block.text, { maxBytes: 393216, maxDepth: 32, maxNodes: 65536 });
  } catch {
    return false;
  }
  if (canonicalJson(textObject) !== canonicalJson(callResult.structuredContent) || canonicalJson(callResult.structuredContent) !== block.text) return false;
  const wrapper = callResult.structuredContent;
  if (Object.keys(wrapper).sort().join(',') !== 'abstention,authorityClass,epistemicClass,mutates,receipt,repository,schema,state,tool') return false;
  if (wrapper.schema !== 'corvint-mcp-bridge-result/0' || wrapper.mutates !== false || wrapper.tool !== expectedCall?.name || !['corvint.query', 'corvint.impact'].includes(wrapper.tool)) return false;
  if (!['READY', 'ABSTAINED'].includes(wrapper.state) || !['OBSERVED', 'NOT_OBSERVED'].includes(wrapper.epistemicClass) || !['REPOSITORY_EVIDENCE', 'NONE'].includes(wrapper.authorityClass)) return false;
  const abstention = wrapper.abstention;
  if (abstention == null || typeof abstention !== 'object' || Array.isArray(abstention) || Object.keys(abstention).sort().join(',') !== 'active,reason' ||
    typeof abstention.active !== 'boolean' || typeof abstention.reason !== 'string' || !/^[A-Z][A-Z0-9_]*$/u.test(abstention.reason)) return false;
  const repositoryValid = validMcpRepository(wrapper.repository);
  if (wrapper.state === 'READY') {
    if (wrapper.epistemicClass !== 'OBSERVED' || wrapper.authorityClass !== 'REPOSITORY_EVIDENCE' || abstention.active || abstention.reason !== 'NONE' || !repositoryValid || wrapper.receipt == null) return false;
  } else if (wrapper.epistemicClass !== 'NOT_OBSERVED' || wrapper.authorityClass !== 'NONE' || !abstention.active || abstention.reason === 'NONE') {
    return false;
  }
  if (wrapper.receipt == null) return wrapper.state === 'ABSTAINED';
  if (wrapper.receipt != null) {
    const receipt = wrapper.receipt;
    const required = ['schema_version', 'mode', 'request', 'revision', 'freshness', 'state', 'results', 'exclusions', 'verification', 'coverage'];
    const allowed = new Set([...required, ...(receipt.mode === 'query' ? ['abstention', 'intent', 'learning'] : [])]);
    if (receipt.context != null || required.some((key) => !(key in receipt)) || Object.keys(receipt).some((key) => !allowed.has(key))) return false;
    if (receipt.schema_version !== 1 || receipt.mode !== (expectedCall?.name === 'corvint.query' ? 'query' : 'impact')) return false;
    const expectedRequest = receipt.mode === 'query' ? { text: expectedCall?.arguments?.task, limit: 1 } : { paths: expectedCall?.arguments?.paths, limit: 20 };
    if (canonicalJson(receipt.request) !== canonicalJson(expectedRequest)) return false;
    if (typeof receipt.revision !== 'string' || !/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/u.test(receipt.revision)) return false;
    if (repositoryValid && receipt.revision !== wrapper.repository.treeRevision) return false;
    if (receipt.freshness?.revision !== receipt.revision || !['fresh', 'mixed-worktree'].includes(receipt.freshness?.state)) return false;
    if (!['READY', 'NEEDS_WIDENING', 'OUT_OF_SCOPE', 'STALE_INDEX', 'BUDGETED', 'CRITICAL_EVIDENCE_OVERFLOW'].includes(receipt.state)) return false;
    if (!Array.isArray(receipt.results) || receipt.results.length > (receipt.mode === 'query' ? 1 : 20) || !Array.isArray(receipt.verification)) return false;
    if (receipt.coverage?.within_budget !== true || !Number.isSafeInteger(receipt.coverage?.packet_bytes) || receipt.coverage.packet_bytes < 0 || receipt.coverage.packet_bytes > 393216) return false;
    if (wrapper.state === 'ABSTAINED' && (wrapper.tool !== 'corvint.impact' || abstention.reason !== 'OUT_OF_SCOPE' || receipt.state !== 'OUT_OF_SCOPE')) return false;
  }
  return true;
}

function validMcpBridgeResults(results, expectedCalls) {
  if (!Array.isArray(results) || results.length !== expectedCalls.length || !results.every((result, index) => validMcpBridgeResult(result, expectedCalls[index]))) return false;
  const versions = results.map((result) => result?._meta?.['io.modelcontextprotocol/serverInfo']?.version);
  return new Set(versions).size <= 1;
}

export function checkObservation(testCase, observation, limits) {
  const failures = [];
  const prefix = testCase.id;
  assertCondition(failures, observation?.profile === DRIVER_PROFILE, `${prefix}: invalid observation profile`);
  assertCondition(failures, observation?.caseId === testCase.id, `${prefix}: caseId mismatch`);
  assertCondition(failures, Array.isArray(observation?.executions), `${prefix}: executions must be an array`);
  assertCondition(failures, observation?.ui && typeof observation.ui === 'object', `${prefix}: missing ui observation`);
  assertCondition(failures, observation?.effects && typeof observation.effects === 'object', `${prefix}: missing effects observation`);
  assertCondition(failures, observation?.limits && typeof observation.limits === 'object', `${prefix}: missing limits observation`);
  const executions = Array.isArray(observation?.executions) ? observation.executions : [];
  const expect = testCase.expect;
  if (expect.liveTests) failures.push(...checkLiveObservation(testCase, observation).map((message) => `${prefix}: ${message}`));

  const admittedResults = expect.resultOneOf ?? [expect.result];
  assertCondition(failures, admittedResults.includes(observation?.result), `${prefix}: expected result in ${admittedResults.join('|')}, got ${observation?.result}`);
  if (expect.executionCount != null) assertCondition(failures, executions.length === expect.executionCount, `${prefix}: expected ${expect.executionCount} executions, got ${executions.length}`);
  if (expect.maxExecutionCount != null) assertCondition(failures, executions.length <= expect.maxExecutionCount, `${prefix}: expected at most ${expect.maxExecutionCount} executions`);
  for (const execution of executions) {
    assertCondition(failures, typeof execution.executable === 'string' && path.isAbsolute(execution.executable), `${prefix}: execution is not an absolute literal executable`);
    assertCondition(failures, execution.shell === false, `${prefix}: shell execution observed`);
    assertCondition(failures, execution.pathLookup === false, `${prefix}: PATH lookup observed after pin`);
    assertCondition(failures, Array.isArray(execution.argv), `${prefix}: execution argv missing`);
  }
  if (expect.sameExecutable && executions.length > 1) assertCondition(failures, new Set(executions.map((item) => item.executable)).size === 1, `${prefix}: executable changed after pin`);
  if (expect.sameIdentity && executions.length > 1) assertCondition(failures, executions.every((item) => item.identity) && new Set(executions.map((item) => item.identity)).size === 1, `${prefix}: file identity changed after pin`);
  if (expect.pinned) assertCondition(failures, path.isAbsolute(observation?.binary?.pinnedPath ?? '') && Boolean(observation?.binary?.pinnedIdentity), `${prefix}: binary was not canonically pinned`);
  if (expect.status) assertCondition(failures, observation?.ui?.status === expect.status, `${prefix}: expected UI status ${expect.status}, got ${observation?.ui?.status}`);
  if (expect.failureCode) assertCondition(failures, observation?.failure?.code === expect.failureCode, `${prefix}: expected failure code ${expect.failureCode}, got ${observation?.failure?.code}`);
  if (expect.failureCodeOneOf) assertCondition(failures, expect.failureCodeOneOf.includes(observation?.failure?.code), `${prefix}: failure code ${observation?.failure?.code} is not one of ${expect.failureCodeOneOf.join('|')}`);
  if (expect.hostKind) assertCondition(failures, observation?.host?.kind === expect.hostKind, `${prefix}: expected host kind ${expect.hostKind}`);
  if (expect.sameHostPin) {
    assertCondition(failures, Boolean(observation?.host?.kind), `${prefix}: extension host was not observed`);
    assertCondition(failures, executions.every((execution) => execution.host === observation?.host?.kind), `${prefix}: executable did not run in the pinning extension host`);
  }
  if (expect.uncertaintyContains) assertCondition(failures, (observation?.uncertainties ?? []).includes(expect.uncertaintyContains), `${prefix}: missing explicit uncertainty ${expect.uncertaintyContains}`);
  if (expect.priorModelCleared) assertCondition(failures, observation?.lifecycle?.priorModelCleared === true, `${prefix}: prior untrusted model was reused`);
  if (expect.freshDiscovery) assertCondition(failures, observation?.lifecycle?.freshDiscovery === true, `${prefix}: trust grant did not perform fresh discovery`);
  if (expect.rootChoiceRequired) assertCondition(failures, observation?.host?.rootChoiceRequired === true, `${prefix}: multi-root choice was silently guessed`);
  if (expect.noMixedSnapshot) assertCondition(failures, observation?.snapshot?.mixed === false, `${prefix}: mixed identities reached a snapshot`);
  if (expect.terminated != null) assertCondition(failures, observation?.limits?.terminated === expect.terminated, `${prefix}: termination mismatch`);
  if (expect.terminationReason) assertCondition(failures, observation?.limits?.terminationReason === expect.terminationReason, `${prefix}: expected termination reason ${expect.terminationReason}`);
  if (expect.maxTimeoutMs != null) assertCondition(failures, Number.isFinite(observation?.limits?.timeoutMs) && observation.limits.timeoutMs <= expect.maxTimeoutMs, `${prefix}: automatic timeout exceeds ${expect.maxTimeoutMs} ms`);
  if (expect.maxJsonDepth != null) assertCondition(failures, Number.isSafeInteger(observation?.limits?.maxJsonDepth) && observation.limits.maxJsonDepth <= expect.maxJsonDepth, `${prefix}: JSON depth bound missing or exceeds ${expect.maxJsonDepth}`);
  if (expect.noDescendantContainmentClaim) assertCondition(failures, observation?.claims?.descendantContainment === false, `${prefix}: direct-child control was advertised as descendant containment`);
  if (expect.noAttestationClaim) assertCondition(failures, observation?.claims?.executableAttestation === false, `${prefix}: substitution detection was advertised as executable attestation`);
  if (expect.activationInert) {
    assertCondition(failures, executions.length === 0, `${prefix}: activation spawned a process`);
    assertCondition(failures, (observation?.effects?.reads ?? []).length === 0, `${prefix}: activation read workspace or executable state`);
    assertCondition(failures, observation?.lifecycle?.trustRequested === false, `${prefix}: activation requested Workspace Trust`);
  }
  if (expect.discoveryOrder) assertCondition(failures, JSON.stringify(observation?.binary?.candidateOrder) === JSON.stringify(expect.discoveryOrder), `${prefix}: candidate discovery order differs from contract`);
  if (expect.pathCapturedOnce) assertCondition(failures, observation?.binary?.pathReadCount === 1, `${prefix}: extension-host PATH was not captured exactly once`);
  if (expect.versionExact) {
    assertCondition(failures, observation?.binary?.version === expect.versionExact, `${prefix}: pinned version token mismatch`);
    assertCondition(failures, /^[a-f0-9]{64}$/u.test(observation?.binary?.sha256 ?? ''), `${prefix}: executable SHA-256 missing or malformed`);
  }
  if (expect.exactArgv) {
    const resolvedExpected = expect.exactArgv.map((argument) => argument === '${root}' ? observation?.host?.root : argument);
    assertCondition(failures, executions.length === 1 && JSON.stringify(executions[0].argv) === JSON.stringify(resolvedExpected), `${prefix}: fixed argv mismatch`);
  }
  if (expect.adapterMatrix) {
    const expected = expect.adapterMatrix.map((row) => ({ ...row, argv: row.argv.map((argument) => argument === '${root}' ? observation?.host?.root : argument) }));
    const actual = executions.map(({ cliKind, logicalOperation, argv }) => ({ cliKind, logicalOperation, argv }));
    assertCondition(failures, JSON.stringify(actual) === JSON.stringify(expected), `${prefix}: CLI adapter matrix differs from the frozen argv contract`);
  }
  if (expect.noRetryOrFailover) {
    assertCondition(failures, observation?.processPolicy?.retryCount === 0, `${prefix}: operation was silently retried`);
    assertCondition(failures, observation?.processPolicy?.failoverCount === 0, `${prefix}: operation silently failed over to another transport or CLI kind`);
  }
  if (expect.actualElectronProfiles && observation?.result !== 'not-run') {
    assertCondition(failures, observation?.host?.actualElectron === true, `${prefix}: observation did not come from a real VS Code Extension Host`);
    assertCondition(failures, JSON.stringify(observation?.host?.profiles) === JSON.stringify(expect.actualElectronProfiles), `${prefix}: trusted/untrusted Electron profiles were not both exercised`);
    assertCondition(failures, typeof observation?.host?.vscodeVersion === 'string' && observation.host.vscodeVersion.length > 0, `${prefix}: pinned VS Code version missing`);
  }
  if (expect.transportDefault) assertCondition(failures, observation?.transport?.default === expect.transportDefault, `${prefix}: default transport mismatch`);
  if (expect.selectedTransport) assertCondition(failures, observation?.transport?.selected === expect.selectedTransport, `${prefix}: selected transport mismatch`);
  if (expect.transportSwitchCleared) {
    const cleared = observation?.transport?.cleared ?? {};
    for (const key of ['pins', 'operations', 'snapshots', 'diagnostics', 'decorations', 'tests']) assertCondition(failures, cleared[key] === true, `${prefix}: transport change did not clear ${key}`);
  }
  if (expect.mcpPinned) {
    assertCondition(failures, path.basename(observation?.binary?.mcpPinnedPath ?? '') === 'corvint-mcp', `${prefix}: MCP executable was not pinned by exact basename`);
    assertCondition(failures, /^[a-f0-9]{64}$/u.test(observation?.binary?.mcpSha256 ?? ''), `${prefix}: MCP executable SHA-256 missing or malformed`);
    assertCondition(failures, Boolean(observation?.binary?.mcpPinnedIdentity), `${prefix}: MCP filesystem identity missing`);
  }
  if (expect.mcpDiscoverySources) assertCondition(failures, canonicalJson(observation?.binary?.mcpDiscoverySources) === canonicalJson(expect.mcpDiscoverySources), `${prefix}: MCP executable used an unapproved discovery source`);
  if (expect.mcpFreshProcess) {
    assertCondition(failures, observation?.mcp?.freshProcessCount === executions.length && new Set(observation?.mcp?.processIdentities ?? []).size === executions.length, `${prefix}: MCP child was pooled or reused`);
  }
  if (expect.mcpStdioLifecycle) assertCondition(failures, observation?.mcp?.transport === 'stdio' && observation?.mcp?.stdinClosed === true && observation?.mcp?.cleanEof === true && observation?.mcp?.processReused === false, `${prefix}: MCP stdio lifecycle differs from one-operation/one-child contract`);
  if (expect.noNetworkTransport) assertCondition(failures, observation?.mcp?.socketOpened === false && observation?.mcp?.httpOpened === false && (observation?.effects?.network ?? []).length === 0, `${prefix}: MCP operation opened a network transport`);
  if (expect.mcpRequestSequence) {
    const requests = observation?.mcp?.requests ?? [];
    assertCondition(failures, canonicalJson(requests.map((request) => request.method)) === canonicalJson(expect.mcpRequestSequence), `${prefix}: MCP request order mismatch`);
    assertCondition(failures, requests.every((request, index) => request.jsonrpc === '2.0' && request.id === index + 1 && validMcpMeta(request.params, expect.mcpMetadataVersion)), `${prefix}: MCP request ID or reserved metadata mismatch`);
  }
  if (expect.mcpMetadataVersion) assertCondition(failures, (observation?.mcp?.requests ?? []).every((request) => validMcpMeta(request.params, expect.mcpMetadataVersion)), `${prefix}: one or more MCP requests omitted exact metadata`);
  if (expect.mcpDiscoveryExact) assertCondition(failures, validMcpDiscovery(observation?.mcp?.discoveryResult), `${prefix}: server/discover result differs from MCPV0-006`);
  if (expect.mcpServerIdentityRecorded) {
    const reportedVersion = observation?.mcp?.discoveryResult?._meta?.['io.modelcontextprotocol/serverInfo']?.version;
    assertCondition(failures, observation?.supportTuple?.mcpServerVersion === reportedVersion, `${prefix}: self-reported server version was not retained as display/debug identity`);
    assertCondition(failures, observation?.supportTuple?.mcpExecutableSha256 === observation?.binary?.mcpSha256, `${prefix}: MCP support tuple omitted pinned executable SHA-256`);
    assertCondition(failures, observation?.supportTuple?.compatibilityBasis === 'protocol+toolset+sha256', `${prefix}: server version was incorrectly used as the compatibility basis`);
  }
  if (expect.noServerOriginatedMessages) assertCondition(failures, (observation?.mcp?.acceptedServerOriginatedMessages ?? []).length === 0, `${prefix}: server-originated request or notification was accepted`);
  if (expect.mcpNoLegacyInitialize) {
    const methods = (observation?.mcp?.requests ?? []).map((request) => request.method);
    assertCondition(failures, !methods.includes('initialize') && !methods.includes('notifications/initialized'), `${prefix}: client fell back to legacy initialization`);
  }
  if (expect.mcpToolsetExact) assertCondition(failures, validMcpToolPages(observation?.mcp?.toolPages), `${prefix}: tools/list differs from exact MCPV0 registry`);
  if (expect.mcpInvokedTool) assertCondition(failures, (observation?.mcp?.requests ?? []).filter((request) => request.method === 'tools/call').every((request) => request.params?.name === expect.mcpInvokedTool), `${prefix}: unexpected MCP tool invoked`);
  if (expect.mcpStatusCallCount != null) assertCondition(failures, (observation?.mcp?.requests ?? []).filter((request) => request.method === 'tools/call' && request.params?.name === 'corvint.status').length === expect.mcpStatusCallCount, `${prefix}: corvint.status was invoked by the editor client`);
  if (expect.mcpCatalogBounds) {
    assertCondition(failures, (observation?.mcp?.maxObservedPages ?? 0) <= 8, `${prefix}: MCP catalog exceeded eight pages`);
    assertCondition(failures, (observation?.mcp?.maxObservedDefinitionsPerPage ?? 0) <= 64, `${prefix}: MCP catalog page exceeded 64 definitions`);
    assertCondition(failures, (observation?.mcp?.maxObservedDefinitions ?? 0) <= 256, `${prefix}: MCP catalog exceeded 256 definitions`);
    assertCondition(failures, (observation?.mcp?.maxObservedCursorBytes ?? 0) <= 4096, `${prefix}: MCP cursor exceeded 4096 bytes`);
  }
  if (expect.mcpCallArguments) {
    const actual = (observation?.mcp?.requests ?? []).filter((request) => request.method === 'tools/call').map((request) => ({ name: request.params?.name, arguments: request.params?.arguments }));
    assertCondition(failures, canonicalJson(actual) === canonicalJson(expect.mcpCallArguments), `${prefix}: MCP tool arguments differ from the closed root-free adapter contract`);
    assertCondition(failures, actual.every((call) => call.arguments?.root == null && call.arguments?._meta == null), `${prefix}: root or metadata leaked into MCP tool arguments`);
  }
  if (expect.mcpBridgeResultExact) assertCondition(failures, validMcpBridgeResults(observation?.mcp?.callResults, expect.mcpCallArguments), `${prefix}: MCP bridge result, authority correlation, server identity, or canonical duplicate differs from MCPV0-008`);
  if (expect.mcpNullAbstentionExact) {
    const results = observation?.mcp?.callResults ?? [];
    const wrapper = results[0]?.structuredContent;
    assertCondition(failures, validMcpBridgeResults(results, [{ name: 'corvint.query', arguments: { task: 'unsupported task' } }]) &&
      wrapper?.state === 'ABSTAINED' && wrapper?.receipt === null && wrapper?.repository === null && wrapper?.abstention?.reason === 'UNSUPPORTED_INTENT', `${prefix}: null-receipt abstention lost or weakened its closed bridge authority`);
  }
  if (expect.mcpRetainedOutOfScopeExact) {
    const results = observation?.mcp?.callResults ?? [];
    const wrapper = results[0]?.structuredContent;
    assertCondition(failures, validMcpBridgeResults(results, [{ name: 'corvint.impact', arguments: { paths: ['src/a.go'], limit: 20 } }]) &&
      wrapper?.state === 'ABSTAINED' && wrapper?.receipt?.state === 'OUT_OF_SCOPE' && wrapper?.abstention?.reason === 'OUT_OF_SCOPE', `${prefix}: retained impact OUT_OF_SCOPE receipt is not exactly correlated`);
  }
  if (expect.mcpAuthorityProjected) {
    const projection = allStrings(observation?.ui?.trees).join('\n');
    assertCondition(failures, /ABSTAINED/u.test(projection) && /NOT_OBSERVED/u.test(projection) && /NONE/u.test(projection) && /(?:UNSUPPORTED_INTENT|OUT_OF_SCOPE)/u.test(projection), `${prefix}: bounded MCP authority/gap was not projected in Impact and Why`);
  }
  if (expect.mcpAuthorityCorrelationRejected) assertCondition(failures, canonicalJson(observation?.mcp?.rejectedAuthorityBehaviors) === canonicalJson(testCase.setup.serverBehaviors) && (observation?.mcp?.callResults ?? []).length === 0, `${prefix}: hostile bridge authority correlation was admitted or not exercised`);
  if (expect.mcpIdentityCorrelationRejected) assertCondition(failures, canonicalJson(observation?.mcp?.rejectedIdentityBehaviors) === canonicalJson(testCase.setup.serverBehaviors) && (observation?.mcp?.callResults ?? []).length === 0, `${prefix}: repository/server identity drift was admitted or not exercised`);
  if (expect.mcpInputRejectedPreSpawn) assertCondition(failures, executions.length === 0 && (observation?.mcp?.requests ?? []).length === 0 && observation?.mcp?.inputRejectedPreSpawn === true, `${prefix}: hostile surrogate input reached MCP spawn or framing`);
  if (expect.mcpFramingBounds) {
    const framing = observation?.mcp?.framing ?? {};
    assertCondition(failures, framing.maxPartialLineBytes <= limits.maxStdoutBytes && framing.maxFrameBytes <= limits.maxStdoutBytes && framing.aggregateStdoutBytes <= limits.maxStdoutBytes, `${prefix}: MCP stdout framing exceeded configured bounds`);
    assertCondition(failures, framing.maxNotifications <= 64 && framing.notificationBytes <= 65536, `${prefix}: MCP notification bounds exceeded`);
    assertCondition(failures, framing.inFlightRequests <= 1 && framing.idsPositiveSafeMonotonicUnique === true, `${prefix}: MCP request IDs or concurrency violate V0`);
  }
  if (expect.mcpCancellationSequence) assertCondition(failures, canonicalJson(observation?.mcp?.cancellation?.events) === canonicalJson(expect.mcpCancellationSequence), `${prefix}: MCP cancellation/EOF/escalation order mismatch`);
  if (expect.mcpCancelledRequestId != null) assertCondition(failures, observation?.mcp?.cancellation?.requestId === expect.mcpCancelledRequestId && observation?.mcp?.cancellation?.notificationCount === 1, `${prefix}: cancellation did not target the exact in-flight ID once`);
  if (expect.mcpLateResultIgnored) assertCondition(failures, observation?.mcp?.cancellation?.lateResultProjected === false, `${prefix}: post-cancellation result reached UI`);
  if (expect.mcpRuntimeEvidence && observation?.result !== 'not-run') assertCondition(failures, observation?.mcp?.runtime?.packagedExtension === true && observation?.mcp?.runtime?.publishableCorvintMcp === true && observation?.mcp?.runtime?.blackBox === true, `${prefix}: MCP runtime evidence is not packaged black-box evidence`);
  if (expect.cwdIsRoot) assertCondition(failures, executions.every((execution) => execution.cwd === observation?.host?.root), `${prefix}: child cwd differs from selected root`);
  if (expect.environmentAllowlisted) {
    const keys = executions.flatMap((execution) => execution.envKeys ?? []);
    const forbidden = /(?:TOKEN|SECRET|PASSWORD|CREDENTIAL|PROXY|SSH_|ASKPASS|TELEMETRY|TRACE|VSCODE_IPC)/iu;
    assertCondition(failures, observation?.environmentPolicy?.published === true && observation?.environmentPolicy?.exact === true, `${prefix}: exact environment allowlist is not published`);
    assertCondition(failures, keys.every((key) => typeof key === 'string' && !forbidden.test(key)), `${prefix}: sensitive environment variable reached child`);
    assertCondition(failures, (observation?.environmentPolicy?.unexpected ?? []).length === 0, `${prefix}: unexpected child environment keys observed`);
  }
  if (expect.environmentExact) assertCondition(failures, JSON.stringify(executions[0]?.env) === JSON.stringify(expect.environmentExact), `${prefix}: child environment values differ from the frozen profile`);
  assertCondition(failures, (observation?.limits?.stdoutBytes ?? 0) <= limits.maxStdoutBytes, `${prefix}: stdout bound exceeded`);
  assertCondition(failures, (observation?.limits?.stderrBytes ?? 0) <= limits.maxStderrBytes, `${prefix}: stderr bound exceeded`);
  if (expect.stdoutBytesExact != null) assertCondition(failures, observation?.limits?.stdoutBytes === expect.stdoutBytesExact, `${prefix}: stdout exact-bound observation mismatch`);
  if (expect.stderrBytesExact != null) assertCondition(failures, observation?.limits?.stderrBytes === expect.stderrBytesExact, `${prefix}: stderr exact-bound observation mismatch`);

  const uiStrings = allStrings(observation?.ui);
  const uiBytes = Buffer.byteLength(uiStrings.join('\n'), 'utf8');
  assertCondition(failures, uiBytes <= limits.maxUiTextBytes, `${prefix}: UI text exceeds ${limits.maxUiTextBytes} bytes`);
  if (expect.outputContains) assertCondition(failures, (observation?.ui?.output ?? []).some((line) => String(line).toLowerCase().includes(expect.outputContains.toLowerCase())), `${prefix}: output does not contain ${expect.outputContains}`);
  if (expect.uiTextSafe) assertCondition(failures, uiStrings.every(isSafeUiText), `${prefix}: unsafe control or ANSI text reached UI`);
  if (expect.noCommandUris) assertCondition(failures, uiStrings.every((text) => !/command:/iu.test(text)), `${prefix}: command URI reached UI`);
  for (const forbidden of expect.forbiddenText ?? []) assertCondition(failures, !allStrings(observation).some((text) => text.includes(forbidden)), `${prefix}: forbidden text leaked`);

  const trees = observation?.ui?.trees ?? {};
  if (expect.noProjectedEvidence) {
    const projected = ['evidence', 'impact', 'why'].flatMap((key) => Array.isArray(trees[key]) ? trees[key] : []);
    assertCondition(failures, projected.length === 0 && (observation?.ui?.diagnostics ?? []).length === 0 && (observation?.ui?.decorations ?? []).length === 0 && (observation?.ui?.tests ?? []).length === 0, `${prefix}: malformed output was partially projected`);
  }
  if (expect.snapshotCleared) assertCondition(failures, observation?.snapshot?.current === null, `${prefix}: failed operation left a current snapshot`);
  if (expect.priorSnapshotHistorical) assertCondition(failures, observation?.snapshot?.priorDisposition === 'historical', `${prefix}: prior snapshot was not visibly labelled historical`);
  if (expect.locationsWithinWorkspace) assertCondition(failures, entriesWithLocations(observation?.ui).every((entry) => entry.location?.withinWorkspace === true), `${prefix}: navigable location escaped workspace`);
  if (expect.minRejectedLocations != null) assertCondition(failures, Number.isSafeInteger(observation?.ui?.rejectedLocations) && observation.ui.rejectedLocations >= expect.minRejectedLocations, `${prefix}: hostile locations were not observably rejected`);
  if (expect.testIdsUnique) {
    const ids = (observation?.ui?.tests ?? []).map((item) => item.id);
    assertCondition(failures, ids.every((id) => typeof id === 'string' && id.length > 0) && new Set(ids).size === ids.length, `${prefix}: test IDs are empty or duplicated`);
  }
  if (expect.diagnosticsSafe) {
    for (const diagnostic of observation?.ui?.diagnostics ?? []) {
      assertCondition(failures, ['error', 'warning', 'information', 'hint'].includes(diagnostic.severity), `${prefix}: invalid diagnostic severity`);
      assertCondition(failures, typeof diagnostic.message === 'string' && isSafeUiText(diagnostic.message), `${prefix}: unsafe diagnostic message`);
      assertCondition(failures, Number.isSafeInteger(diagnostic.location?.line) && diagnostic.location.line >= 0, `${prefix}: invalid diagnostic line`);
    }
  }
  if (expect.diagnosticGenerationReplaced) {
    const generations = observation?.ui?.diagnosticGenerations ?? [];
    const currentIds = new Set((observation?.ui?.diagnostics ?? []).map((item) => item.id));
    const priorIds = new Set((generations[0]?.ids ?? []));
    const latestIds = new Set((generations.at(-1)?.ids ?? []));
    assertCondition(failures, generations.length >= 2, `${prefix}: fewer than two diagnostic generations observed`);
    assertCondition(failures, [...priorIds].every((id) => !currentIds.has(id)), `${prefix}: stale diagnostics remained visible`);
    assertCondition(failures, currentIds.size === latestIds.size && [...currentIds].every((id) => latestIds.has(id)), `${prefix}: visible diagnostics do not equal latest generation`);
  }
  if (expect.decorationsSafe) {
    for (const decoration of observation?.ui?.decorations ?? []) {
      assertCondition(failures, typeof decoration.label === 'string' && isSafeUiText(decoration.label), `${prefix}: unsafe decoration label`);
      assertCondition(failures, Number.isSafeInteger(decoration.location?.line) && decoration.location.line >= 0, `${prefix}: invalid decoration line`);
    }
  }
  if (expect.testStatus) {
    const statuses = (observation?.ui?.tests ?? []).map((item) => item.status);
    assertCondition(failures, statuses.includes(expect.testStatus) && !statuses.includes('passed'), `${prefix}: cancellation status was not preserved`);
  }
  if (expect.testStatusMap) {
    const actual = Object.fromEntries((observation?.ui?.tests ?? []).map((item) => [item.sourceStatus, item.status]));
    for (const [sourceStatus, status] of Object.entries(expect.testStatusMap)) assertCondition(failures, actual[sourceStatus] === status, `${prefix}: ${sourceStatus} mapped to ${actual[sourceStatus]}, expected ${status}`);
  }
  if (expect.noPassedTests) assertCondition(failures, (observation?.ui?.tests ?? []).every((item) => item.status !== 'passed'), `${prefix}: rejected or incomplete observation displayed a passing test`);
  if (expect.observationReadOnce) {
    assertCondition(failures, observation?.observationFile?.readCount === 1, `${prefix}: observation was not read exactly once`);
    assertCondition(failures, observation?.observationFile?.stableRevalidation === true, `${prefix}: observation identity was not revalidated after EOF`);
    assertCondition(failures, observation?.residue?.watchers === 0, `${prefix}: observation import installed a watcher`);
  }
  if (expect.verificationState) assertCondition(failures, observation?.observationFile?.verificationState === expect.verificationState, `${prefix}: import verification state mismatch`);
  if (expect.digestNotResolved) assertCondition(failures, observation?.observationFile?.digestResolutionReads === 0, `${prefix}: output digest triggered an additional file read`);
  if (expect.testOutputBounded) {
    const outputs = (observation?.ui?.tests ?? []).map((item) => String(item.output ?? ''));
    assertCondition(failures, outputs.every((output) => Buffer.byteLength(output, 'utf8') <= 8192), `${prefix}: per-test output exceeds 8 KiB`);
    assertCondition(failures, Buffer.byteLength(outputs.join(''), 'utf8') <= 262144, `${prefix}: imported run output exceeds 256 KiB`);
  }
  if (expect.testHierarchy) {
    const actual = (observation?.ui?.tests ?? []).map(({ id, parentId = null }) => ({ id, parentId }));
    assertCondition(failures, JSON.stringify(actual) === JSON.stringify(expect.testHierarchy), `${prefix}: Test Explorer hierarchy differs from immutable Corvint IDs`);
  }
  if (expect.testRunEnded) assertCondition(failures, observation?.ui?.testRun?.ended === true && typeof observation?.ui?.testRun?.corvintRunId === 'string', `${prefix}: imported observation did not create one finite ended Corvint TestRun`);
  if (expect.noRunProfile) assertCondition(failures, observation?.ui?.testRunProfiles === 0, `${prefix}: Testing API RunProfile was registered`);
  if (expect.exactCommands) {
    const expected = ['corvint.selectExecutable', 'corvint.clearExecutable', 'corvint.query', 'corvint.impact', 'corvint.refresh', 'corvint.importTestObservation', 'corvint.clearResults'].sort();
    assertCondition(failures, JSON.stringify([...(observation?.ui?.registeredCommands ?? [])].sort()) === JSON.stringify(expected), `${prefix}: command surface differs from contract`);
  }
  if (expect.statusAccessible) assertCondition(failures, typeof observation?.ui?.statusAccessibleName === 'string' && /Corvint/u.test(observation.ui.statusAccessibleName) && /Ready/u.test(observation.ui.statusAccessibleName), `${prefix}: status accessible name omits Corvint or state`);
  if (expect.exactTreeViews) assertCondition(failures, JSON.stringify(Object.keys(observation?.ui?.trees ?? {}).sort()) === JSON.stringify(['evidence', 'impact', 'why']), `${prefix}: TreeView surface differs from contract`);
  if (expect.treeBounds) {
    const nodes = ['evidence', 'impact', 'why'].flatMap((key) => observation?.ui?.trees?.[key] ?? []);
    assertCondition(failures, nodes.length <= 2000 && nodes.every((node) => [...(node.label ?? '')].length <= 200 && [...(node.description ?? '')].length <= 300 && [...(node.tooltip ?? '')].length <= 2000 && (node.depth ?? 0) <= 16), `${prefix}: TreeView bounds exceeded`);
  }
  if (expect.treeOrder) {
    const actualOrder = ['evidence', 'impact', 'why'].flatMap((key) => observation?.ui?.trees?.[key] ?? []).map((node) => node.id);
    assertCondition(failures, JSON.stringify(actualOrder) === JSON.stringify(expect.treeOrder), `${prefix}: deterministic TreeView order mismatch`);
  }
  if (expect.visibleTruncation) assertCondition(failures, observation?.ui?.truncation?.visible === true && observation?.ui?.truncation?.unknownPreserved === true, `${prefix}: truncation was silent or discarded unknown state`);
  if (expect.noWhyExecution) assertCondition(failures, executions.every((execution) => !execution.argv.includes('why')), `${prefix}: nonexistent Corvint why command executed`);
  if (expect.forbiddenSurfacesAbsent) {
    const absent = new Set(observation?.staticAudit?.absent ?? []);
    for (const surface of expect.forbiddenSurfacesAbsent) assertCondition(failures, absent.has(surface), `${prefix}: static audit did not prove ${surface} absent`);
  }

  const effectKeys = ['network', 'telemetry', 'storage', 'writes', 'authorityChanges', 'installs'];
  if (expect.effectsEmpty) for (const key of effectKeys) assertCondition(failures, Array.isArray(observation?.effects?.[key]) && observation.effects[key].length === 0, `${prefix}: non-empty or missing effect ${key}`);
  if (expect.noReads) assertCondition(failures, Array.isArray(observation?.effects?.reads) && observation.effects.reads.length === 0, `${prefix}: untrusted mode performed file/configuration/repository reads`);
  if (expect.authorityUnchanged) assertCondition(failures, Array.isArray(observation?.effects?.authorityChanges) && observation.effects.authorityChanges.length === 0, `${prefix}: projection upgraded authority`);
  if (expect.residueZero) for (const key of ['processes', 'watchers', 'diagnostics', 'decorations', 'tests', 'views', 'statusBar', 'storage']) assertCondition(failures, observation?.residue?.[key] === 0, `${prefix}: residue ${key}=${observation?.residue?.[key]}`);
  return failures;
}

export async function runSuite({ manifest, driver, selectedIds = [], driverTimeoutMs = 5000 }) {
  const manifestFailures = validateManifest(manifest);
  if (manifestFailures.length) return { status: 'FAIL', failures: manifestFailures, cases: [] };
  const selected = selectedIds.length ? manifest.cases.filter((item) => selectedIds.includes(item.id)) : manifest.cases;
  const unknown = selectedIds.filter((id) => !manifest.cases.some((item) => item.id === id));
  if (unknown.length) return { status: 'FAIL', failures: unknown.map((id) => `unknown case ${id}`), cases: [] };
  if (!driver) return { status: 'NOT_RUN', failures: ['actual VS Code host driver not supplied'], cases: [] };
  const results = [];
  const failures = [];
  const notRun = [];
  for (const testCase of selected) {
    try {
      const observation = await runDriver(driver, testCase, manifest.limits, { timeoutMs: driverTimeoutMs });
      const caseFailures = checkObservation(testCase, observation, manifest.limits);
      const caseStatus = caseFailures.length ? 'FAIL' : observation.result === 'not-run' ? 'NOT_RUN' : 'PASS';
      if (caseStatus === 'NOT_RUN') {
        if (!testCase.expect.notRunAllowed || typeof observation.notRunReason !== 'string' || !observation.notRunReason) {
          caseFailures.push(`${testCase.id}: NOT_RUN lacks an allowed explicit reason`);
        } else {
          notRun.push({ id: testCase.id, reason: observation.notRunReason });
        }
      }
      results.push({ id: testCase.id, status: caseFailures.length ? 'FAIL' : caseStatus, failures: caseFailures });
      failures.push(...caseFailures);
    } catch (error) {
      const message = `${testCase.id}: driver error: ${error.message}`;
      results.push({ id: testCase.id, status: 'FAIL', failures: [message] });
      failures.push(message);
    }
  }
  return { status: failures.length ? 'FAIL' : notRun.length ? 'NOT_RUN' : 'PASS', failures, notRun, cases: results };
}

function parseArguments(argv) {
  const options = { selectedIds: [] };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === '--driver') options.driver = argv[++index];
    else if (argument === '--cases') options.cases = argv[++index];
    else if (argument === '--case') options.selectedIds.push(argv[++index]);
    else if (argument === '--driver-timeout-ms') options.driverTimeoutMs = Number(argv[++index]);
    else if (argument === '--list') options.list = true;
    else throw new Error(`unknown argument ${argument}`);
  }
  return options;
}

async function main() {
  const options = parseArguments(process.argv.slice(2));
  const manifest = await loadManifest(options.cases ?? DEFAULT_CASES);
  if (options.list) {
    for (const item of manifest.cases) process.stdout.write(`${item.id}\t${item.title}\n`);
    return;
  }
  const result = await runSuite({ manifest, ...options });
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  process.exitCode = result.status === 'PASS' ? 0 : result.status === 'NOT_RUN' ? 2 : 1;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    process.stderr.write(`${error.stack ?? error}\n`);
    process.exitCode = 1;
  });
}
