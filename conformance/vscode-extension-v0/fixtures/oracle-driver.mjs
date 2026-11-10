#!/usr/bin/env node

import process from 'node:process';

let input = '';
for await (const chunk of process.stdin) input += chunk;
const testCase = JSON.parse(input);
const expect = testCase.expect;
const canonicalJson = (value) => {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(',')}]`;
  return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonicalJson(value[key])}`).join(',')}}`;
};
const mcpMeta = (version = expect.mcpMetadataVersion ?? '0.1.0') => ({
  'io.modelcontextprotocol/protocolVersion': '2026-07-28',
  'io.modelcontextprotocol/clientInfo': { name: 'corvint-vscode', version },
  'io.modelcontextprotocol/clientCapabilities': {},
});
const mcpSchema = (properties, required) => ({ '$schema': 'https://json-schema.org/draft/2020-12/schema', type: 'object', properties, required, additionalProperties: false });
const mcpAnnotations = { readOnlyHint: true, destructiveHint: false, idempotentHint: true, openWorldHint: false };
const mcpServerInfo = { description: 'Local read-only Corvint context and repository evidence server.', name: 'corvint-mcp', version: '0.1.0-experimental' };
const mcpOutputSchema = { type: 'object', properties: { mutates: { const: false }, tool: { type: 'string' } } };
const mcpRevision = { type: 'string', pattern: '^([0-9a-f]{40}|[0-9a-f]{64})$' };
// The bridge advertises help, impact and status after decision 0594 retired corvint.query.
const mcpTools = [
  {
    name: 'corvint.help', description: "Return the task-indexed agent guide exactly as `corvint help guide` prints it (the default), one command's catalog entry exactly as `corvint help --json COMMAND` prints it (command NAME, under 5 KB), or with topic catalog the full command catalog (corvint-capability-catalog/0, about 180 KB) exactly as `corvint help --json` prints it. Reads only the binary, never the repository.",
    inputSchema: mcpSchema({
      command: { type: 'string', enum: ['impact', 'help'] },
      topic: { type: 'string', enum: ['guide', 'command', 'catalog'], default: 'guide' },
    }, []), annotations: mcpAnnotations, outputSchema: mcpOutputSchema,
  },
  {
    name: 'corvint.impact', description: "Compile revision-bound impact evidence without source bodies. Capabilities: Go (.go): path, package imports; Ruby (.rb): path, markers, advisory autoload constant references, reverse imports unknown; JavaScript (.js/.jsx/.mjs/.cjs) and TypeScript (.ts/.tsx): path, literal relative/profile/tsconfig-alias imports, unresolved bare imports unknown. Non-Go paths name experimental non-go-syntax-v0. Dynamic dispatch and test closure remain unknown. The receipt equals CLI corvint impact with the syntax profile for the same paths, revision and limit; the CLI admits more suffixes and --base (corvint impact --capabilities). MCP accepts paths only.",
    inputSchema: mcpSchema({
      paths: { type: 'array', minItems: 1, maxItems: 100, uniqueItems: true, items: { type: 'string', minLength: 1, maxLength: 1024, pattern: String.raw`^(?:(?:(?:[^/\\.\n\r][^/\\\n\r]*|\.[^/\\.\n\r][^/\\\n\r]*|\.\.[^/\\\n\r]+)/)+[^/\\\n\r]*|[^/\\\n\r]+)[.](?:go|rb|js|jsx|mjs|cjs|ts|tsx)$` } },
      limit: { type: 'integer', minimum: 1, maximum: 50, default: 10 },
      budgetBytes: { type: 'integer', minimum: 1216, maximum: 1000000 },
      snapshot: {
        type: 'object', additionalProperties: false,
        required: ['schema', 'commitRevision', 'treeRevision', 'baseRevision', 'changedPaths', 'changedPathsSha256'],
        properties: {
          schema: { const: 'corvint-planning-snapshot/0' },
          commitRevision: mcpRevision, treeRevision: mcpRevision, baseRevision: mcpRevision,
          changedPaths: { type: 'array', maxItems: 4096, uniqueItems: true, items: { type: 'string', minLength: 1 } },
          changedPathsSha256: { type: 'string', pattern: '^[0-9a-f]{64}$' },
        },
      },
    }, ['paths']), annotations: mcpAnnotations, outputSchema: mcpOutputSchema,
  },
  {
    name: 'corvint.status', description: 'Observe Git commit, tree, worktree state, and a privacy-preserving dirty-path digest. Gitlinks bind commit IDs; nested submodule contents are outside coverage.',
    inputSchema: mcpSchema({}, []), annotations: mcpAnnotations, outputSchema: mcpOutputSchema,
  },
];
const mcpRequests = [];
for (const [index, method] of (expect.mcpRequestSequence ?? []).entries()) {
  const params = { _meta: mcpMeta() };
  if (method === 'tools/call') Object.assign(params, { name: 'corvint.impact', arguments: { paths: ['a.go'], limit: 20 } });
  mcpRequests.push({ jsonrpc: '2.0', id: index + 1, method, params });
}
if (expect.mcpInvokedTool && mcpRequests.length === 0) mcpRequests.push({ jsonrpc: '2.0', id: 1, method: 'tools/call', params: { _meta: mcpMeta(), name: expect.mcpInvokedTool, arguments: { paths: ['a.go'], limit: 20 } } });
if (expect.mcpCallArguments) {
  mcpRequests.length = 0;
  for (const [index, call] of expect.mcpCallArguments.entries()) mcpRequests.push({ jsonrpc: '2.0', id: index + 1, method: 'tools/call', params: { _meta: mcpMeta(), ...call } });
}
const mcpCallResults = (expect.mcpCallArguments ?? []).map((call) => {
  const wrapper = {
    schema: 'corvint-mcp-bridge-result/0', tool: call.name, mutates: false, state: 'READY', epistemicClass: 'OBSERVED', authorityClass: 'REPOSITORY_EVIDENCE',
    repository: { commitRevision: 'a'.repeat(40), treeRevision: 'a'.repeat(40), objectFormat: 'sha1', profileId: 'generic', worktreeState: 'CLEAN', dirtyPathCount: 0, dirtyPathsSha256: 'c'.repeat(64) },
    receipt: { schema_version: 1, mode: 'impact', revision: 'a'.repeat(40), request: { paths: call.arguments.paths, limit: 20 }, freshness: { state: 'fresh', revision: 'a'.repeat(40) }, state: 'READY', results: [], exclusions: [], verification: [], coverage: { within_budget: true, packet_bytes: 0 } },
    abstention: { active: false, reason: 'NONE' },
  };
  return { resultType: 'complete', isError: false, content: [{ type: 'text', text: canonicalJson(wrapper) }], structuredContent: wrapper, _meta: { 'io.modelcontextprotocol/serverInfo': mcpServerInfo } };
});
const bridgeCallResult = (wrapper) => ({
  resultType: 'complete',
  isError: false,
  content: [{ type: 'text', text: canonicalJson(wrapper) }],
  structuredContent: wrapper,
  _meta: { 'io.modelcontextprotocol/serverInfo': mcpServerInfo },
});
if (expect.mcpNullAbstentionExact) {
  mcpCallResults.push(bridgeCallResult({
    schema: 'corvint-mcp-bridge-result/0', tool: 'corvint.impact', mutates: false, state: 'ABSTAINED', epistemicClass: 'NOT_OBSERVED', authorityClass: 'NONE',
    repository: null, receipt: null, abstention: { active: true, reason: 'REPOSITORY_STATE_UNSTABLE' },
  }));
}
if (expect.mcpRetainedOutOfScopeExact) {
  mcpCallResults.push(bridgeCallResult({
    schema: 'corvint-mcp-bridge-result/0', tool: 'corvint.impact', mutates: false, state: 'ABSTAINED', epistemicClass: 'NOT_OBSERVED', authorityClass: 'NONE',
    repository: { commitRevision: 'a'.repeat(40), treeRevision: 'a'.repeat(40), objectFormat: 'sha1', profileId: 'generic', worktreeState: 'CLEAN', dirtyPathCount: 0, dirtyPathsSha256: 'c'.repeat(64) },
    receipt: {
      schema_version: 1, mode: 'impact', revision: 'a'.repeat(40), request: { paths: ['src/a.go'], limit: 20 },
      freshness: { state: 'fresh', revision: 'a'.repeat(40), mixed_path_count: 0 }, state: 'OUT_OF_SCOPE', results: [], exclusions: { count: 1 }, verification: [],
      coverage: { within_budget: true, packet_bytes: 0, uncertainty: ['outside indexed scope'], critical_missing: [] },
    },
    abstention: { active: true, reason: 'OUT_OF_SCOPE' },
  }));
}
const executionCount = expect.executionCount ?? Math.min(expect.maxExecutionCount ?? 0, 0);
const executions = Array.from({ length: executionCount }, () => ({
  executable: '/opt/corvint/bin/corvint',
  argv: ['impact', '--format', 'json'],
  shell: false,
  pathLookup: false,
  identity: 'dev=1;ino=2;size=3;mtime=4',
  host: 'remote',
  cwd: '/fixture/root',
  envKeys: ['HOME', 'LANG', 'LC_ALL', 'TMPDIR'],
}));
if (expect.exactArgv && executions.length === 1) executions[0].argv = expect.exactArgv.map((argument) => argument === '${root}' ? '/fixture/root' : argument);
if (expect.adapterMatrix) {
  for (const [index, row] of expect.adapterMatrix.entries()) {
    executions[index].cliKind = row.cliKind;
    executions[index].logicalOperation = row.logicalOperation;
    executions[index].argv = row.argv.map((argument) => argument === '${root}' ? '/fixture/root' : argument);
  }
}
if (expect.environmentExact && executions.length === 1) executions[0].env = expect.environmentExact;
const tests = [];
if (expect.testStatus) tests.push({ id: 'cancelled', label: 'cancelled', sourceStatus: 'cancelled', status: expect.testStatus });
if (expect.testIdsUnique) tests.push(
  { id: 'sha256:a', label: 'duplicate', status: 'queued' },
  { id: 'sha256:b', label: 'duplicate', status: 'queued' },
);
if (expect.testStatusMap) {
  for (const [sourceStatus, status] of Object.entries(expect.testStatusMap)) tests.push({ id: `status:${sourceStatus}`, label: sourceStatus, sourceStatus, status });
}
if (expect.testHierarchy) for (const item of expect.testHierarchy) tests.push({ ...item, status: 'queued', output: '' });
const diagnosticLines = expect.diagnosticLinesExact ?? (expect.diagnosticsSafe ? [0] : []);
const diagnostics = diagnosticLines.map((line, index) => ({ id: expect.diagnosticGenerationReplaced ? 'new' : `diagnostic:${index}`, severity: 'warning', message: 'bounded diagnostic', location: { path: 'src/a.go', line, withinWorkspace: true } }));
const decorations = expect.decorationsSafe ? [{ label: 'impact', location: { path: 'src/a.go', line: 0, withinWorkspace: true } }] : [];
const observation = {
  profile: 'corvint-vscode-host-observation/0',
  caseId: testCase.id,
  result: expect.result ?? (expect.resultOneOf?.includes('not-run') ? 'not-run' : expect.resultOneOf?.[0]),
  notRunReason: expect.notRunAllowed ? 'required VS Code Electron or measurement runtime not supplied' : undefined,
  host: { kind: expect.hostKind ?? (expect.sameHostPin ? 'remote' : 'desktop'), root: '/fixture/root', rootChoiceRequired: expect.rootChoiceRequired ?? false },
  lifecycle: { priorModelCleared: expect.priorModelCleared ?? false, freshDiscovery: expect.freshDiscovery ?? false, trustRequested: false },
  snapshot: { mixed: false, current: expect.snapshotCleared ? null : {}, priorDisposition: expect.priorSnapshotHistorical ? 'historical' : null },
  environmentPolicy: { published: true, exact: true, unexpected: [] },
  processPolicy: { retryCount: 0, failoverCount: 0 },
  uncertainties: expect.uncertaintyContains ? [expect.uncertaintyContains] : [],
  claims: { descendantContainment: false, executableAttestation: false },
  failure: expect.failureCode || expect.failureCodeOneOf ? { code: expect.failureCode ?? expect.failureCodeOneOf[0], ...(expect.failureMessageExact != null ? { message: expect.failureMessageExact } : {}) } : null,
  observationFile: {
    readCount: expect.observationReadOnce ? 1 : 0,
    stableRevalidation: expect.observationReadOnce ?? false,
    verificationState: expect.verificationState,
    digestResolutionReads: 0,
  },
  binary: {
    ...(expect.pinned ? { pinnedPath: '/opt/corvint/bin/corvint', pinnedIdentity: 'dev=1;ino=2;size=3;mtime=4' } : {}),
    ...(expect.discoveryOrder ? { candidateOrder: expect.discoveryOrder } : {}),
    ...(expect.pathCapturedOnce ? { pathReadCount: 1 } : {}),
    ...(expect.versionExact ? { version: expect.versionExact, sha256: 'a'.repeat(64) } : {}),
    ...(expect.mcpPinned ? { mcpPinnedPath: '/opt/corvint/bin/corvint-mcp', mcpSha256: 'b'.repeat(64), mcpPinnedIdentity: 'dev=1;ino=3;size=4;mode=755' } : {}),
    ...(expect.mcpDiscoverySources ? { mcpDiscoverySources: expect.mcpDiscoverySources } : {}),
  },
  transport: {
    default: expect.transportDefault,
    selected: expect.selectedTransport,
    cleared: expect.transportSwitchCleared ? { pins: true, operations: true, snapshots: true, diagnostics: true, decorations: true, tests: true } : {},
  },
  mcp: {
    transport: expect.mcpStdioLifecycle ? 'stdio' : undefined,
    freshProcessCount: expect.mcpFreshProcess ? executions.length : 0,
    processIdentities: expect.mcpFreshProcess ? executions.map((_, index) => `process:${index}`) : [],
    stdinClosed: expect.mcpStdioLifecycle ?? false,
    cleanEof: expect.mcpStdioLifecycle ?? false,
    processReused: false,
    socketOpened: false,
    httpOpened: false,
    requests: mcpRequests,
    discoveryResult: expect.mcpDiscoveryExact ? { resultType: 'complete', supportedVersions: ['2026-07-28'], capabilities: { tools: { listChanged: false } }, ttlMs: 0, cacheScope: 'public', _meta: { 'io.modelcontextprotocol/serverInfo': mcpServerInfo } } : undefined,
    acceptedServerOriginatedMessages: [],
    toolPages: expect.mcpToolsetExact ? [{ resultType: 'complete', ttlMs: 300000, cacheScope: 'private', tools: mcpTools, _meta: { 'io.modelcontextprotocol/serverInfo': mcpServerInfo } }] : [],
    callResults: mcpCallResults,
    rejectedAuthorityBehaviors: expect.mcpAuthorityCorrelationRejected ? testCase.setup.serverBehaviors : [],
    rejectedIdentityBehaviors: expect.mcpIdentityCorrelationRejected ? testCase.setup.serverBehaviors : [],
    inputRejectedPreSpawn: expect.mcpInputRejectedPreSpawn ?? false,
    maxObservedPages: expect.mcpCatalogBounds ? 8 : 0,
    maxObservedDefinitionsPerPage: expect.mcpCatalogBounds ? 64 : 0,
    maxObservedDefinitions: expect.mcpCatalogBounds ? 256 : 0,
    maxObservedCursorBytes: expect.mcpCatalogBounds ? 4096 : 0,
    framing: expect.mcpFramingBounds ? { maxPartialLineBytes: 262144, maxFrameBytes: 262144, aggregateStdoutBytes: 262144, maxNotifications: 64, notificationBytes: 65536, inFlightRequests: 1, idsPositiveSafeMonotonicUnique: true } : {},
    cancellation: expect.mcpCancellationSequence ? { events: expect.mcpCancellationSequence, requestId: expect.mcpCancelledRequestId, notificationCount: 1, lateResultProjected: false } : {},
    runtime: {},
  },
  supportTuple: expect.mcpServerIdentityRecorded ? { mcpServerVersion: '0.1.0-experimental', mcpExecutableSha256: 'b'.repeat(64), compatibilityBasis: 'protocol+toolset+sha256' } : undefined,
  executions,
  limits: {
    timeoutMs: expect.maxTimeoutMs ?? 1000,
    stdoutBytes: expect.stdoutBytesExact ?? 0,
    stderrBytes: expect.stderrBytesExact ?? 0,
    terminated: expect.terminated ?? false,
    terminationReason: expect.terminationReason ?? null,
    maxJsonDepth: expect.maxJsonDepth ?? 64,
  },
  ui: {
    status: expect.status ?? 'ready',
    trees: expect.mcpAuthorityProjected ? {
      evidence: [],
      impact: [{ id: 'mcp:impact:unknown', label: `Unknown: ${expect.mcpNullAbstentionExact ? 'REPOSITORY_STATE_UNSTABLE' : 'OUT_OF_SCOPE'}`, description: 'ABSTAINED', tooltip: 'NOT_OBSERVED NONE' }],
      why: [{ id: 'mcp:why:authority', label: 'MCP authority: ABSTAINED', description: 'NOT_OBSERVED · NONE', tooltip: `Reason: ${expect.mcpNullAbstentionExact ? 'REPOSITORY_STATE_UNSTABLE' : 'OUT_OF_SCOPE'}` }],
    } : { evidence: [], impact: [], why: [] },
    diagnostics,
    documentOpenCount: expect.documentOpenCount ?? 0,
    decorations,
    tests,
    testRunProfiles: 0,
    testRun: expect.testRunEnded ? { ended: true, corvintRunId: 'run:opaque' } : undefined,
    registeredCommands: expect.exactCommands ? ['corvint.selectExecutable', 'corvint.clearExecutable', 'corvint.impact', 'corvint.refresh', 'corvint.clearResults'] : [],
    statusAccessibleName: expect.statusAccessible ? 'Corvint Ready' : '',
    output: expect.outputContains ? [`Corvint ${expect.outputContains}`] : [],
    rejectedLocations: expect.minRejectedLocations ?? 0,
    diagnosticGenerations: expect.diagnosticGenerationReplaced ? [{ generation: 1, ids: ['old'] }, { generation: 2, ids: ['new'] }] : [],
    truncation: expect.visibleTruncation ? { visible: true, unknownPreserved: true } : undefined,
  },
  effects: { reads: [], network: [], telemetry: [], storage: [], writes: [], authorityChanges: [], installs: [] },
  staticAudit: { absent: expect.forbiddenSurfacesAbsent ?? [] },
  residue: { processes: 0, watchers: 0, diagnostics: 0, decorations: 0, tests: 0, views: 0, statusBar: 0, storage: 0 },
};
process.stdout.write(`${JSON.stringify(observation)}\n`);
