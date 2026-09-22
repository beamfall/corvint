import { isJsonObject, parseBoundedJson, type JsonObject, type JsonValue } from "./json.js";
import { createHash } from "node:crypto";

const HEX_REVISION = /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;
const HEX_BLOB = /^[0-9a-f]{40,64}$/;
const MAX_RESULTS = 20;
const MAX_EVIDENCE_PER_RESULT = 16;

export type Operation = "query" | "impact";

export interface EvidenceItem {
  readonly resultId: string;
  readonly resultKind: string;
  readonly summary: string;
  readonly path: string;
  readonly line: number;
  readonly blobHash: string;
  readonly reason: string;
  readonly confidence: string;
  readonly authority: string;
  readonly resultState?: string;
}

export interface ResultItem {
  readonly id: string;
  readonly kind: string;
  readonly summary: string;
  readonly ordinal?: number;
  readonly score?: number;
  readonly state?: string;
  readonly evidence: readonly EvidenceItem[];
}

export interface TransportRepositoryIdentity {
  readonly commitRevision: string;
  readonly treeRevision: string;
  readonly objectFormat: "sha1" | "sha256";
  readonly profileId: "generic" | "beamfall";
  readonly worktreeState: "CLEAN" | "MIXED";
  readonly dirtyPathCount: number;
  readonly dirtyPathsSha256: string;
}

export interface TransportAuthority {
  readonly profile: "corvint-mcp-bridge-result/0";
  readonly tool: "corvint.query" | "corvint.impact";
  readonly state: "READY" | "ABSTAINED";
  readonly epistemicClass: "OBSERVED" | "NOT_OBSERVED";
  readonly authorityClass: "REPOSITORY_EVIDENCE" | "NONE";
  readonly abstentionReason: string;
  readonly repository?: TransportRepositoryIdentity;
  readonly serverVersion: string;
  readonly executableSha256: string;
  readonly protocol: "2026-07-28";
}

export interface CorvintSnapshot {
  readonly snapshotKey: string;
  readonly adapterProfile: "unprofiled context schema_version 1" | "corvint-mcp-bridge-result/0";
  readonly operation: Operation;
  readonly schemaVersion: 1;
  readonly revision: string;
  readonly state: string;
  readonly freshness: string;
  readonly worktreeMixedPathCount: number;
  readonly results: readonly ResultItem[];
  readonly evidence: readonly EvidenceItem[];
  readonly uncertainty: readonly string[];
  readonly verification: readonly string[];
  readonly transportAuthority?: TransportAuthority;
}

export type ProjectionClass = "evidence" | "candidate" | "unknown" | "informational";
export type DiagnosticClass = "error" | "warning" | "information" | "none";

export interface ContextReceiptSource {
  readonly context: JsonObject;
  readonly snapshotBytes: Uint8Array;
}

export function decodeCorvintReceipt(
  bytes: Uint8Array,
  operation: Operation,
  maxBytes: number,
  cliKind: "corvint",
  expected: string | readonly string[],
): CorvintSnapshot {
  if (bytes.byteLength < 2 || bytes[bytes.byteLength - 1] !== 0x0a || bytes[bytes.byteLength - 2] === 0x0a) {
    throw new Error("INVALID_JSON: stdout must be one JSON object followed by one LF");
  }
  const value = parseBoundedJson(bytes.subarray(0, bytes.byteLength - 1), {
    maxBytes,
    maxDepth: 32,
    maxNodes: 65_536,
  });
  const envelope = object(value, "response");
  exactKeys(envelope, ["context", "mutates", "ok", "tool"], "response");
  if (envelope.ok !== true || envelope.mutates !== false || string(envelope.tool, "tool", 16) !== operation) {
    throw new Error("UNSUPPORTED_PROFILE: response envelope does not match the requested read operation");
  }
  const context = object(envelope.context, "context");
  return decodeContextReceipt(context, operation, maxBytes, cliKind, expected, bytes);
}

export function decodeContextReceipt(
  context: JsonObject,
  operation: Operation,
  maxBytes: number,
  cliKind: "corvint",
  expected: string | readonly string[],
  snapshotBytes: Uint8Array,
): CorvintSnapshot {
  exactAllowedKeys(context, ["schema_version", "mode", "request", "revision", "freshness", "state", "results", "exclusions", "verification", "coverage"],
    operation === "query" ? ["abstention", "intent", "learning"] : [], "context");
  if (context.schema_version !== 1 || string(context.mode, "context.mode", 16) !== operation) {
    throw new Error("UNSUPPORTED_PROFILE: only Corvint context schema_version 1 is supported");
  }
  const revision = string(context.revision, "context.revision", 64);
  if (!HEX_REVISION.test(revision)) {
    throw new Error("INVALID_JSON: invalid Corvint revision");
  }
  const state = enumLike(context.state, "context.state");
  if (!["READY", "NEEDS_WIDENING", "OUT_OF_SCOPE", "STALE_INDEX", "BUDGETED", "CRITICAL_EVIDENCE_OVERFLOW"].includes(state)) {
    throw new Error("INVALID_JSON: unsupported context state");
  }
  validateEchoedRequest(object(context.request, "context.request"), operation, cliKind, expected);
  const freshnessObject = object(context.freshness, "context.freshness");
  const freshness = enumLike(freshnessObject.state, "context.freshness.state");
  if (freshness !== "fresh" && freshness !== "mixed-worktree") {
    throw new Error("INVALID_JSON: unsupported freshness state");
  }
  if (freshnessObject.revision !== undefined && freshnessObject.revision !== revision) {
    throw new Error("INVALID_JSON: freshness revision does not match context revision");
  }
  const mixedCount = integer(freshnessObject.mixed_path_count, "context.freshness.mixed_path_count", 1_000_000);
  const rawResults = array(context.results, "context.results", MAX_RESULTS);
  const results = rawResults.map((entry, index) => resultItem(entry, index));
  const evidence = results.flatMap((result) => result.evidence);
  const verification = stringArray(context.verification, "context.verification", 64, 1_000);
  const uncertainty: string[] = [];
  const coverage = object(context.coverage, "context.coverage");
  if (coverage.within_budget !== true || integer(coverage.packet_bytes, "context.coverage.packet_bytes", maxBytes) > maxBytes) {
    throw new Error("INVALID_JSON: receipt exceeds or rejects its packet budget");
  }
  uncertainty.push(...stringArray(coverage.uncertainty, "context.coverage.uncertainty", 256, 2_000));
  uncertainty.push(...stringArray(coverage.critical_missing, "context.coverage.critical_missing", 256, 2_000));
  const exclusions = object(context.exclusions, "context.exclusions");
  const exclusionCount = integer(exclusions.count, "context.exclusions.count", 1_000_000);
  if (exclusionCount > 0) {
    uncertainty.push(`Excluded source count: ${exclusionCount}`);
  }
  if (context.abstention !== undefined) {
    const abstention = object(context.abstention, "context.abstention");
    const reason = abstention.reason;
    if (typeof reason === "string") {
      uncertainty.push(display(reason, 2_000));
    }
  }
  if (state !== "READY") {
    uncertainty.unshift(`Corvint state: ${state}`);
  }
  return Object.freeze({
    snapshotKey: createHash("sha256").update(snapshotBytes).digest("hex"),
    adapterProfile: "unprofiled context schema_version 1" as const,
    operation,
    schemaVersion: 1 as const,
    revision,
    state,
    freshness,
    worktreeMixedPathCount: mixedCount,
    results: Object.freeze(results),
    evidence: Object.freeze(evidence),
    uncertainty: Object.freeze(unique(uncertainty).slice(0, 512)),
    verification: Object.freeze(verification),
  });
}

function resultItem(value: JsonValue, index: number): ResultItem {
  const item = object(value, `context.results[${index}]`);
  const id = identityString(item.id, "result.id", 4_096);
  const kind = string(item.kind, "result.kind", 128);
  const summary = display(string(item.summary, "result.summary", 4_096), 300);
  const state = item.status === undefined ? undefined : enumLike(item.status, "result.status");
  const rawEvidence = array(item.evidence, "result.evidence", MAX_EVIDENCE_PER_RESULT);
  const evidence = rawEvidence.map((entry, evidenceIndex) => evidenceItem(entry, evidenceIndex, id, kind, summary, state));
  const ordinal = item.ordinal === undefined ? undefined : integer(item.ordinal, "result.ordinal", Number.MAX_SAFE_INTEGER);
  const score = item.score === undefined ? undefined : integer(item.score, "result.score", Number.MAX_SAFE_INTEGER);
  return Object.freeze({ id, kind, summary, ...(ordinal === undefined ? {} : { ordinal }), ...(score === undefined ? {} : { score }),
    ...(state === undefined ? {} : { state }), evidence: Object.freeze(evidence) });
}

function evidenceItem(
  value: JsonValue,
  index: number,
  resultId: string,
  resultKind: string,
  summary: string,
  resultState: string | undefined,
): EvidenceItem {
  const item = object(value, `result.evidence[${index}]`);
  exactKeys(item, ["path", "line", "blob_hash", "reason", "confidence", "authority"], "evidence");
  const path = string(item.path, "evidence.path", 4_096);
  if (!isRepositoryRelativePath(path)) {
    throw new Error("PATH_REJECTED: Corvint evidence path is not normalized repository-relative text");
  }
  const blobHash = string(item.blob_hash, "evidence.blob_hash", 64);
  if (!HEX_BLOB.test(blobHash) || (blobHash.length !== 40 && blobHash.length !== 64)) {
    throw new Error("INVALID_JSON: invalid evidence blob identity");
  }
  return Object.freeze({
    resultId,
    resultKind,
    summary,
    path,
    line: integer(item.line, "evidence.line", 10_000_000, 1),
    blobHash,
    reason: display(string(item.reason, "evidence.reason", 4_096), 2_000),
    confidence: display(string(item.confidence, "evidence.confidence", 128), 128),
    authority: display(string(item.authority, "evidence.authority", 128), 128),
    ...(resultState === undefined ? {} : { resultState }),
  });
}

export function isRepositoryRelativePath(value: string): boolean {
  if (value.length < 1 || value.length > 4_096 || value === "." || value.startsWith("/") || value.includes("\\")) {
    return false;
  }
  if (/[\u0000-\u001f\u007f]/u.test(value)) {
    return false;
  }
  const parts = value.split("/");
  return parts.every((part) => part.length > 0 && part !== "." && part !== "..");
}

export function display(value: string, maximumScalars: number): string {
  // Mirrors internal/workqueue/grammar.go's forbiddenIdentifierRune (C0/C1,
  // U+061C, U+200E/F, U+202A-E, U+2066-9) plus U+FEFF, which that Go set
  // already covers but this class previously omitted, extended with the
  // line/paragraph separators (U+2028/9) and the invisible-format block
  // (U+200B-D zero-width space/non-joiner/joiner, U+2060-4 word joiner and
  // invisible operators) that neither set covered.
  const stripped = value
    .replace(/\u001b\[[0-?]*[ -/]*[@-~]/gu, "")
    .replace(/[\u0000-\u001f\u007f-\u009f\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069\u2028\u2029\u200b-\u200d\u2060-\u2064\ufeff]/gu, "�");
  const scalars = [...stripped];
  return scalars.length <= maximumScalars ? stripped : `${scalars.slice(0, maximumScalars - 1).join("")}…`;
}

export function attachTransportAuthority(snapshot: CorvintSnapshot, authority: TransportAuthority): CorvintSnapshot {
  const repository = authority.repository === undefined ? undefined : Object.freeze({ ...authority.repository });
  const transportAuthority = Object.freeze({
    ...authority,
    ...(repository === undefined ? {} : { repository }),
  });
  return Object.freeze({ ...snapshot, transportAuthority });
}

export function createTransportAbstentionSnapshot(
  operation: Operation,
  expected: string | readonly string[],
  snapshotBytes: Uint8Array,
  authority: TransportAuthority,
): CorvintSnapshot {
  if (authority.tool !== `corvint.${operation}` || authority.state !== "ABSTAINED" ||
    authority.epistemicClass !== "NOT_OBSERVED" || authority.authorityClass !== "NONE" ||
    authority.abstentionReason === "NONE") {
    throw new Error("MCP_PROTOCOL: inconsistent null-receipt transport authority");
  }
  const snapshotKey = createHash("sha256")
    .update("corvint-vscode-null-receipt-v0\0")
    .update(operation)
    .update("\0")
    .update(JSON.stringify(expected))
    .update("\0")
    .update(snapshotBytes)
    .digest("hex");
  return attachTransportAuthority(Object.freeze({
    snapshotKey,
    adapterProfile: "corvint-mcp-bridge-result/0" as const,
    operation,
    schemaVersion: 1 as const,
    revision: authority.repository?.treeRevision ?? "not-observed",
    state: "ABSTAINED",
    freshness: "not-observed",
    worktreeMixedPathCount: 0,
    results: Object.freeze([]),
    evidence: Object.freeze([]),
    uncertainty: Object.freeze([authority.abstentionReason]),
    verification: Object.freeze([]),
  }), authority);
}

export function projectionClass(state: string | undefined): ProjectionClass {
  switch (state) {
    case "CONFLICTED":
    case "REFUTED":
    case "PROVEN":
    case "AFFECTED":
    case "OBSERVED":
    case "DIRECT":
    case "INCLUDED":
    case "READY":
      return "evidence";
    case "CANDIDATE":
    case "BOUNDED_CANDIDATE":
    case "POTENTIAL":
    case "POSSIBLE":
      return "candidate";
    case "UNKNOWN":
    case "STALE":
    case "NOT_OBSERVED":
    case "UNOBSERVED":
    case "OUT_OF_SCOPE":
      return "unknown";
    case "INFORMATIONAL":
    case "INFO":
      return "informational";
    default:
      return "unknown";
  }
}

export function diagnosticClass(state: string | undefined): DiagnosticClass {
  switch (state) {
    case "CONFLICTED":
    case "REFUTED":
      return "error";
    case "UNKNOWN":
    case "STALE":
      return "warning";
    case "CANDIDATE":
    case "BOUNDED_CANDIDATE":
    case "INFORMATIONAL":
    case "INFO":
      return "information";
    default:
      return "none";
  }
}

export function orderedResults(results: readonly ResultItem[]): ResultItem[] {
  return [...results].sort((left, right) => {
    if (left.ordinal !== undefined || right.ordinal !== undefined) {
      if (left.ordinal === undefined) {
        return 1;
      }
      if (right.ordinal === undefined) {
        return -1;
      }
      if (left.ordinal !== right.ordinal) {
        return left.ordinal - right.ordinal;
      }
    }
    return compareProjectionTuple(resultTuple(left), resultTuple(right));
  });
}

export function orderedEvidence(evidence: readonly EvidenceItem[]): EvidenceItem[] {
  return [...evidence].sort((left, right) => compareProjectionTuple([
    stateRank(left.resultState),
    left.path,
    left.line,
    stableEvidenceId(left),
  ], [
    stateRank(right.resultState),
    right.path,
    right.line,
    stableEvidenceId(right),
  ]));
}

export function stableResultId(result: ResultItem): string {
  return stableProjectionId(["result", result.kind, result.id]);
}

export function stableEvidenceId(evidence: EvidenceItem): string {
  return stableProjectionId([
    "evidence",
    evidence.resultKind,
    evidence.resultId,
    evidence.path,
    evidence.line,
    evidence.blobHash,
    evidence.reason,
    evidence.confidence,
    evidence.authority,
    evidence.resultState ?? "",
  ]);
}

export function stableProjectionId(parts: readonly (string | number)[]): string {
  return createHash("sha256").update(JSON.stringify(parts)).digest("hex");
}

function resultTuple(result: ResultItem): readonly [number, string, number, string] {
  const evidence = orderedEvidence(result.evidence)[0];
  const repositoryPath = evidence?.path ?? (isRepositoryRelativePath(result.id) ? result.id : "");
  return [stateRank(result.state), repositoryPath, evidence?.line ?? 0, stableResultId(result)];
}

function stateRank(state: string | undefined): number {
  switch (projectionClass(state)) {
    case "evidence":
      return state === "CONFLICTED" || state === "REFUTED" ? 0 : 2;
    case "unknown":
      return 1;
    case "candidate":
      return 3;
    case "informational":
      return 4;
  }
}

function compareProjectionTuple(
  left: readonly [number, string, number, string],
  right: readonly [number, string, number, string],
): number {
  return left[0] - right[0] || compareUtf8(left[1], right[1]) || left[2] - right[2] || compareUtf8(left[3], right[3]);
}

function compareUtf8(left: string, right: string): number {
  return Buffer.compare(Buffer.from(left, "utf8"), Buffer.from(right, "utf8"));
}

function object(value: JsonValue | undefined, name: string): JsonObject {
  if (value === undefined || !isJsonObject(value)) {
    throw new Error(`INVALID_JSON: ${name} must be an object`);
  }
  return value;
}

function array(value: JsonValue | undefined, name: string, maximum: number): readonly JsonValue[] {
  if (!Array.isArray(value) || value.length > maximum) {
    throw new Error(`INVALID_JSON: ${name} must be a bounded array`);
  }
  return value;
}

function string(value: JsonValue | undefined, name: string, maximumBytes: number): string {
  if (typeof value !== "string" || Buffer.byteLength(value, "utf8") > maximumBytes) {
    throw new Error(`INVALID_JSON: ${name} must be a bounded string`);
  }
  return value;
}

function identityString(value: JsonValue | undefined, name: string, maximumBytes: number): string {
  const result = string(value, name, maximumBytes);
  if (/[\u0000-\u001f\u007f-\u009f\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]/u.test(result)) {
    throw new Error(`INVALID_JSON: ${name} contains unsafe identity text`);
  }
  return result;
}

function enumLike(value: JsonValue | undefined, name: string): string {
  const result = string(value, name, 64);
  if (!/^[A-Za-z][A-Za-z0-9_-]*$/.test(result)) {
    throw new Error(`INVALID_JSON: ${name} is not an enum token`);
  }
  return result;
}

function integer(value: JsonValue | undefined, name: string, maximum: number, minimum = 0): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < minimum || value > maximum) {
    throw new Error(`INVALID_JSON: ${name} must be a bounded integer`);
  }
  return value;
}

function stringArray(value: JsonValue | undefined, name: string, maximum: number, maximumBytes: number): string[] {
  if (value === undefined) {
    return [];
  }
  return array(value, name, maximum).map((item) => display(string(item, name, maximumBytes), 2_000));
}

function unique(values: readonly string[]): string[] {
  return [...new Set(values)];
}

function validateEchoedRequest(
  request: JsonObject,
  operation: Operation,
  _cliKind: "corvint",
  expected: string | readonly string[],
): void {
  if (operation === "query") {
    if (typeof expected !== "string") {
      throw new Error("INVALID_JSON: query expectation is invalid");
    }
    exactKeys(request, ["limit", "text"], "context.request");
    if (request.text !== expected || request.limit !== 1) {
      throw new Error("INVALID_JSON: query request echo mismatch");
    }
    return;
  }
  if (typeof expected === "string") {
    throw new Error("INVALID_JSON: impact expectation is invalid");
  }
  exactKeys(request, ["limit", "paths"], "context.request");
  const paths = stringArray(request.paths, "context.request.paths", 256, 4_096);
  if (request.limit !== 20 ||
    paths.length !== expected.length || paths.some((entry, index) => entry !== expected[index])) {
    throw new Error("INVALID_JSON: impact request echo mismatch");
  }
}

function exactKeys(value: JsonObject, keys: readonly string[], name: string): void {
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) {
    throw new Error(`INVALID_JSON: ${name} keys do not match the admitted schema`);
  }
}

function exactAllowedKeys(value: JsonObject, required: readonly string[], optional: readonly string[], name: string): void {
  for (const key of required) {
    if (!(key in value)) {
      throw new Error(`INVALID_JSON: ${name} is missing ${key}`);
    }
  }
  const allowed = new Set([...required, ...optional]);
  if (Object.keys(value).some((key) => !allowed.has(key))) {
    throw new Error(`INVALID_JSON: ${name} contains an unsupported key`);
  }
}
