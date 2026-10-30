/**
 * Pure adapter for the two real live test-provider stream shapes (roadmap
 * IPR-09):
 *
 *  - `corvint-go-test-provider session --foreground ...` (internal/liveverify/
 *    session/session.go + cmd/corvint-go-test-provider/session.go) emits one
 *    compact NDJSON line per session-state transition:
 *    `{"profile":"corvint-go-live-session-event/0","state":"idle|running|
 *    passed|failed|stale|infrastructure|cancelled","identity":string,
 *    "sequence":number,"scope":string[]|null,"detail":string,"projection":
 *    <testvalidity.Projection JSON>,"testProjections":[{"package":string,
 *    "name":string,"action":"pass|fail|skip|none","projection":<testvalidity.
 *    Projection JSON>}]|null,"testProjectionsOmitted":number}`. Each `scope`
 *    entry becomes one `TestItem`, and a terminal state closes every item
 *    announced by the matching `running` event from `state`/`detail`.
 *    `projection` (GLTP-V0-050) restates that session-level transition; it
 *    is decoded and cross-checked against `state` by `classifyGoSessionEvent`
 *    (VSC-V0-068), which never lets it upgrade the state. `testProjections`
 *    (GLTP-V0-051) carries one
 *    per-test projection for a run that completed; each entry becomes its
 *    own `TestItem` classified through `classifyGoTest`, which is
 *    `classifyProjection` plus the observed go test action on an abstained
 *    result. A capped remainder (`testProjectionsOmitted`) is shown as an
 *    explicit unknown, never as passing. A Go entry whose execution anchors
 *    lead with a `file:line` (GLTP-V0-052) is anchored there (VSC-V0-069).
 *  - `corvint-js-test-provider unit|e2e` (internal/jstestprovider,
 *    cmd/corvint-js-test-provider/main.go) runs to completion once and prints
 *    a single (pretty-printed, so multi-line) JSON document:
 *    `{"receipt":{"kind":"unit"|"e2e",...},"testProjections":[{"name":
 *    string,"state":string,"projection":<testvalidity.Projection JSON>}],
 *    "runProjection":<testvalidity.Projection JSON>}`. This is per-test:
 *    each `testProjections` entry becomes one `TestItem`, with diagnostics
 *    anchored at the `file:line` carried in `projection.execution.anchors[0]`
 *    when the projection has one (see internal/jstestprovider/projection.go
 *    anchorString).
 *
 * There is no producer in this repository for a `{"kind":"validity"|
 * "state",...}` record; that shape does not exist on either wire and is not
 * carried here.
 *
 * Records are discriminated by the `profile` field (Go) vs. the `receipt`
 * field (JS), not by transport framing, because the two producers use
 * different framing: the Go session is genuinely NDJSON (one line, one
 * record), while the JS provider's single document spans many lines.
 * `extractJsonRecords` finds complete top-level JSON values across either
 * framing by tracking brace/bracket depth, so callers feed it raw stdout
 * chunks rather than assuming a record ends at a newline.
 *
 * This module holds no VS Code, filesystem, or process dependency so a
 * provider field-name change is a one-file edit, and so the mapping and
 * supersession rules are testable under plain node:test without mocking the
 * `vscode` module (the convention already used by testvalidity.ts and the
 * rest of this extension's pure modules).
 */

import { isJsonObject, parseBoundedJson, type JsonObject, type JsonValue } from "./json.js";
import type { Axis, Projection } from "./testvalidity.js";

export const MAX_RECORD_BYTES = 262_144;
const GO_SESSION_PROFILE = "corvint-go-live-session-event/0";

export class ProtocolError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ProtocolError";
  }
}

export type GoSessionState = "idle" | "running" | "passed" | "failed" | "stale" | "infrastructure" | "cancelled";
const GO_SESSION_STATES: readonly GoSessionState[] = ["idle", "running", "passed", "failed", "stale", "infrastructure", "cancelled"];

/** One `corvint-go-live-session-event/0` transition line. */
export interface GoSessionEvent {
  readonly kind: "go-session";
  readonly state: GoSessionState;
  readonly identity: string;
  readonly sequence: number;
  readonly scope: readonly string[];
  readonly detail: string;
  /** The session-level projection (GLTP-V0-050); undefined when the producer sent none. */
  readonly projection: Projection | undefined;
  /** Per-test projections (GLTP-V0-051); empty when the producer sent none. */
  readonly tests: readonly GoTestResult[];
  /** Tests the producer reported but dropped at its per-event bound. */
  readonly testsOmitted: number;
}

/** One per-test entry from a Go session event's `testProjections` array. */
export interface GoTestResult {
  readonly testId: string;
  readonly package: string;
  readonly name: string;
  readonly action: string;
  /** Source anchor from the leading `file:line` execution anchor (GLTP-V0-052), if any. */
  readonly file: string | undefined;
  readonly line: number | undefined;
  readonly projection: Projection;
}

/** One per-test entry from a JS provider's `testProjections` array. */
export interface JsTestResult {
  readonly testId: string;
  readonly state: string;
  readonly file: string | undefined;
  readonly line: number | undefined;
  readonly projection: Projection;
}

/** The JS provider's one-shot receipt document. */
export interface JsProviderOutput {
  readonly kind: "js-provider";
  readonly runKind: string;
  readonly tests: readonly JsTestResult[];
  readonly runProjection: Projection;
}

export type ProviderRecord = GoSessionEvent | JsProviderOutput;

export interface ExtractResult {
  readonly records: readonly string[];
  readonly rest: string;
  /** True when an unterminated trailing value grew past MAX_RECORD_BYTES and
   * was dropped (the records before it are still returned), or when the scan
   * is still discarding such a value. Pass it back as `discardingOverflow`
   * with the next buffer. */
  readonly overflow: boolean;
}

/**
 * Splits a buffer of accumulated provider stdout into complete top-level
 * JSON values (by brace/bracket depth, respecting quoted strings), so it
 * handles both one-object-per-line NDJSON and a single pretty-printed
 * multi-line JSON document uniformly. A leading byte that cannot start a
 * JSON value is treated as one bad record spanning to the next newline (or
 * end of buffer), so a garbage line is still surfaced to the caller — and
 * dropped by it — the same way a malformed NDJSON line always was, instead
 * of desynchronizing the stream. A value still incomplete after growing past
 * MAX_RECORD_BYTES is dropped and reported through `overflow`; complete
 * records that preceded it in the same buffer are kept. With
 * `discardingOverflow`, the buffer is skipped through its first LF first, so
 * a chunk that begins inside the dropped record cannot swallow the records
 * behind it.
 */
/**
 * Whether a `running` event opens a new test run: always for an identity
 * other than the active one, and also for the same identity when no run is
 * open (a re-trigger on unchanged content after the previous run closed),
 * so its terminal event has a run to close.
 */
export function beginsNewRun(activeIdentity: string | undefined, runOpen: boolean, identity: string): boolean {
  return activeIdentity !== identity || !runOpen;
}

export function extractJsonRecords(buffer: string, discardingOverflow = false): ExtractResult {
  const records: string[] = [];
  const newline = discardingOverflow ? buffer.indexOf("\n") : -1;
  if (discardingOverflow && newline === -1) {
    return { records, rest: "", overflow: true };
  }
  let offset = newline + 1;
  while (offset < buffer.length) {
    while (offset < buffer.length && isJsonWhitespace(buffer.charAt(offset))) {
      offset += 1;
    }
    if (offset >= buffer.length) {
      break;
    }
    const start = offset;
    const end = findRecordEnd(buffer, start);
    if (end === undefined) {
      if (buffer.length - start > MAX_RECORD_BYTES) {
        return { records, rest: "", overflow: true };
      }
      return { records, rest: buffer.slice(start), overflow: false };
    }
    records.push(buffer.slice(start, end).replace(/^[ \t\r\n]+|[ \t\r\n]+$/g, ""));
    offset = end;
  }
  return { records, rest: "", overflow: false };
}

function isJsonWhitespace(character: string): boolean {
  return character === " " || character === "\n" || character === "\r" || character === "\t";
}

function findRecordEnd(text: string, start: number): number | undefined {
  const first = text.charAt(start);
  if (first !== "{" && first !== "[") {
    const newlineIndex = text.indexOf("\n", start);
    return newlineIndex === -1 ? undefined : newlineIndex + 1;
  }
  let depth = 0;
  let inString = false;
  let escaped = false;
  for (let index = start; index < text.length; index += 1) {
    const character = text.charAt(index);
    if (inString) {
      if (escaped) {
        escaped = false;
      } else if (character === "\\") {
        escaped = true;
      } else if (character === '"') {
        inString = false;
      }
      continue;
    }
    if (character === '"') {
      inString = true;
    } else if (character === "{" || character === "[") {
      depth += 1;
    } else if (character === "}" || character === "]") {
      depth -= 1;
      if (depth === 0) {
        return index + 1;
      }
    }
  }
  return undefined;
}

/** Parses one extracted record's text into a ProviderRecord. */
export function parseProviderRecord(text: string): ProviderRecord {
  const bytes = Buffer.from(text, "utf8");
  if (bytes.byteLength === 0 || bytes.byteLength > MAX_RECORD_BYTES) {
    throw new ProtocolError("provider record must be a bounded non-empty JSON value");
  }
  let value: JsonValue;
  try {
    value = parseBoundedJson(bytes, { maxBytes: MAX_RECORD_BYTES, maxDepth: 16, maxNodes: 16_384, maxStringBytes: 65_536 });
  } catch (error) {
    throw new ProtocolError(`invalid provider JSON: ${error instanceof Error ? error.message : String(error)}`);
  }
  if (!isJsonObject(value)) {
    throw new ProtocolError("provider record must be a JSON object");
  }
  if (value.profile === GO_SESSION_PROFILE) {
    return parseGoSessionEvent(value);
  }
  if (value.receipt !== undefined) {
    return parseJsProviderOutput(value);
  }
  throw new ProtocolError("unsupported provider record shape");
}

function parseGoSessionEvent(value: JsonObject): GoSessionEvent {
  const state = str(value.state, "state");
  if (!GO_SESSION_STATES.includes(state as GoSessionState)) {
    throw new ProtocolError(`unsupported session state ${JSON.stringify(state)}`);
  }
  const scopeValue = value.scope;
  let scope: readonly string[] = [];
  if (scopeValue !== undefined && scopeValue !== null) {
    if (!Array.isArray(scopeValue) || !scopeValue.every((entry) => typeof entry === "string")) {
      throw new ProtocolError("scope must be an array of strings or null");
    }
    scope = scopeValue as readonly string[];
  }
  const detailValue = value.detail;
  const projectionValue = value.projection;
  if (projectionValue !== undefined && projectionValue !== null && !isJsonObject(projectionValue)) {
    throw new ProtocolError("projection must be an object");
  }
  return {
    kind: "go-session",
    state: state as GoSessionState,
    identity: str(value.identity, "identity"),
    sequence: nonNegativeInt(value.sequence, "sequence"),
    scope,
    detail: typeof detailValue === "string" ? detailValue : "",
    projection: projectionValue === undefined || projectionValue === null ? undefined : parseProjection(projectionValue),
    tests: parseGoTests(value.testProjections),
    testsOmitted: value.testProjectionsOmitted === undefined ? 0 : nonNegativeInt(value.testProjectionsOmitted, "testProjectionsOmitted"),
  };
}

function parseGoTests(value: JsonValue | undefined): readonly GoTestResult[] {
  if (value === undefined || value === null) {
    return [];
  }
  if (!Array.isArray(value)) {
    throw new ProtocolError("testProjections must be an array or null");
  }
  return value.map((entry) => {
    if (!isJsonObject(entry)) {
      throw new ProtocolError("testProjections entry must be an object");
    }
    const projectionValue = entry.projection;
    if (projectionValue === undefined || !isJsonObject(projectionValue)) {
      throw new ProtocolError("testProjections[].projection must be an object");
    }
    const pkg = str(entry.package, "testProjections[].package");
    const name = str(entry.name, "testProjections[].name");
    const projection = parseProjection(projectionValue);
    const anchor = goSourceAnchor(projection);
    return { testId: `${pkg}#${name}`, package: pkg, name, action: str(entry.action, "testProjections[].action"), file: anchor?.file, line: anchor?.line, projection };
  });
}

function parseJsProviderOutput(value: JsonObject): JsProviderOutput {
  const receipt = value.receipt;
  if (receipt === undefined || !isJsonObject(receipt)) {
    throw new ProtocolError("receipt must be an object");
  }
  const runKind = str(receipt.kind, "receipt.kind");
  const runProjectionValue = value.runProjection;
  if (runProjectionValue === undefined || !isJsonObject(runProjectionValue)) {
    throw new ProtocolError("runProjection must be an object");
  }
  const runProjection = parseProjection(runProjectionValue);
  const tests: JsTestResult[] = [];
  const testProjectionsValue = value.testProjections;
  if (testProjectionsValue !== undefined && testProjectionsValue !== null) {
    if (!Array.isArray(testProjectionsValue)) {
      throw new ProtocolError("testProjections must be an array");
    }
    for (const entry of testProjectionsValue) {
      if (!isJsonObject(entry)) {
        throw new ProtocolError("testProjections entry must be an object");
      }
      const name = str(entry.name, "testProjections[].name");
      const state = str(entry.state, "testProjections[].state");
      const projectionValue = entry.projection;
      if (projectionValue === undefined || !isJsonObject(projectionValue)) {
        throw new ProtocolError("testProjections[].projection must be an object");
      }
      const projection = parseProjection(projectionValue);
      const anchor = anchorFromProjection(projection);
      tests.push({ testId: name, state, file: anchor?.file, line: anchor?.line, projection });
    }
  }
  return { kind: "js-provider", runKind, tests, runProjection };
}

/**
 * Recovers `file`/`line` from `projection.execution.anchors[0]`, the one
 * anchor string internal/jstestprovider/projection.go's `anchorString`
 * writes (either bare `file` or `file:line`). Execution-based facts
 * (timedOut/interrupted/infrastructure) and claim-based facts (passed/
 * failed/skipped/flaky) both surface their anchor on the execution axis —
 * see internal/testvalidity/projection.go's Project. Returns undefined when
 * no anchor was reported, so the caller renders that item with no
 * diagnostic instead of a fabricated line.
 */
function anchorFromProjection(projection: Projection): { file: string; line: number } | undefined {
  const anchor = projection.execution.anchors?.[0];
  if (anchor === undefined) {
    return undefined;
  }
  const separatorIndex = anchor.lastIndexOf(":");
  if (separatorIndex === -1) {
    return { file: anchor, line: 1 };
  }
  const line = Number(anchor.slice(separatorIndex + 1));
  if (!Number.isInteger(line) || line < 1) {
    return { file: anchor, line: 1 };
  }
  return { file: anchor.slice(0, separatorIndex), line };
}

/**
 * Recovers a Go per-test source anchor (GLTP-V0-052): the session puts
 * `<repo-relative path>:<line>` ahead of its `input-identity:<id>` anchor
 * only when one declaration was located. Anything else — no leading
 * anchor, an identity anchor, or no positive integer line — is unanchored.
 */
function goSourceAnchor(projection: Projection): { file: string; line: number } | undefined {
  const anchor = projection.execution.anchors?.[0];
  if (anchor === undefined || anchor.startsWith("input-identity:")) {
    return undefined;
  }
  const separatorIndex = anchor.lastIndexOf(":");
  const line = Number(anchor.slice(separatorIndex + 1));
  if (separatorIndex < 1 || !Number.isInteger(line) || line < 1) {
    return undefined;
  }
  return { file: anchor.slice(0, separatorIndex), line };
}

function parseProjection(value: JsonObject): Projection {
  return {
    association: parseAxis(value.association, "projection.association"),
    hygiene: parseAxis(value.hygiene, "projection.hygiene"),
    freshness: parseAxis(value.freshness, "projection.freshness"),
    execution: parseAxis(value.execution, "projection.execution"),
    strength: parseAxis(value.strength, "projection.strength"),
  };
}

function parseAxis(value: JsonValue | undefined, name: string): Axis {
  if (value === undefined || !isJsonObject(value)) {
    throw new ProtocolError(`${name} must be an object`);
  }
  const state = str(value.state, `${name}.state`);
  const reasonValue = value.reason;
  if (reasonValue !== undefined && typeof reasonValue !== "string") {
    throw new ProtocolError(`${name}.reason must be a string`);
  }
  const anchorsValue = value.anchors;
  let anchors: readonly string[] | null = null;
  if (anchorsValue !== undefined && anchorsValue !== null) {
    if (!Array.isArray(anchorsValue) || !anchorsValue.every((entry) => typeof entry === "string")) {
      throw new ProtocolError(`${name}.anchors must be an array of strings or null`);
    }
    anchors = anchorsValue as readonly string[];
  }
  return { state, reason: reasonValue ?? "", anchors };
}

function str(value: JsonValue | undefined, name: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new ProtocolError(`${name} must be a non-empty string`);
  }
  return value;
}

function nonNegativeInt(value: JsonValue | undefined, name: string): number {
  if (typeof value !== "number" || !Number.isInteger(value) || value < 0) {
    throw new ProtocolError(`${name} must be a non-negative integer`);
  }
  return value;
}

/**
 * Stale supersession: a per-test record's identity is only applied while it
 * matches the session's currently active identity (last announced by a Go
 * session "running" event). A record tagged with a prior identity that
 * arrives late — out of order after a newer session already started — is
 * dropped rather than allowed to resurrect or downgrade a newer result.
 * `activeInputIdentity` of `undefined` means no session has announced an
 * identity yet, so the record is admitted provisionally.
 */
export function shouldApplyRecord(activeInputIdentity: string | undefined, recordInputIdentity: string): boolean {
  return activeInputIdentity === undefined || activeInputIdentity === recordInputIdentity;
}

export interface GoSessionOutcome {
  readonly kind: "passed" | "failed" | "errored";
  readonly message: string;
}

/**
 * Maps one Go session terminal state to a `vscode.TestRun` outcome for
 * every `TestItem` the matching `running` event announced. `infrastructure`
 * and `cancelled` are always `errored`, never `failed` (they are not a
 * report about the test's own assertions); `stale` overrides whatever a
 * prior `passed`/`failed` for this identity would have meant, but the
 * producer (internal/liveverify/session) already resolves that override
 * before publishing — a `stale` event is simply rendered as `errored` here,
 * matching VSC-V0-061's "no dedicated stale TestRun state" display choice.
 */
export function classifyGoSessionState(state: GoSessionState, detail: string): GoSessionOutcome {
  const withDetail = (label: string): string => `${label}${suffix(detail)}`;
  switch (state) {
    case "passed":
      return { kind: "passed", message: "" };
    case "failed":
      return { kind: "failed", message: detail.length > 0 ? detail : "test failed" };
    case "stale":
      return { kind: "errored", message: withDetail("STALE") };
    case "infrastructure":
      return { kind: "errored", message: withDetail("INFRASTRUCTURE") };
    case "cancelled":
      return { kind: "errored", message: withDetail("CANCELLED") };
    default:
      return { kind: "errored", message: withDetail(state) };
  }
}

/**
 * Classifies a terminal Go session event for its `scope` items (VSC-V0-068).
 * The outcome is always `classifyGoSessionState`'s; the decoded session
 * `projection` only cross-checks it. When `classifyProjection` of that
 * projection chooses a different kind, the event is shown `errored` with an
 * explicit contradiction rather than either reading, so the projection can
 * never upgrade a state. An event with no projection is classified from its
 * state alone.
 */
export function classifyGoSessionEvent(event: GoSessionEvent): GoSessionOutcome {
  const outcome = classifyGoSessionState(event.state, event.detail);
  if (event.projection === undefined) {
    return outcome;
  }
  const projected = classifyProjection(event.projection);
  if (projected.kind === outcome.kind) {
    return outcome;
  }
  return { kind: "errored", message: `UNKNOWN: session state ${event.state} contradicts its projection (${projected.kind}${suffix(projected.message)})` };
}

export type RunOutcomeKind = "passed" | "failed" | "skipped" | "errored" | "stale";

export interface RunOutcome {
  readonly kind: RunOutcomeKind;
  /** Primary message for failed/errored/stale; empty for passed/skipped. */
  readonly message: string;
  /** Strength/hygiene advisory notes. These never change `kind`. */
  readonly notes: readonly string[];
}

/**
 * Maps one Projection to a VS Code TestRun outcome. Mapping decisions
 * (see docs/specs/vscode-extension-v0.md VSC-V0-058..):
 *  - execution INFRASTRUCTURE/CANCELLED always wins: errored, with the
 *    execution reason.
 *  - otherwise freshness STALE wins over whatever execution reports: a
 *    passing/failing result against superseded source is never presented as
 *    a fresh pass or fail. TestRun has no dedicated "stale" run state and
 *    `TestRun.enqueued()` accepts no message, so a stale result surfaces as
 *    `errored` carrying an explicit "STALE: <reason>" message rather than a
 *    silent requeue.
 *  - otherwise execution PASSED/FAILED/SKIPPED map to the matching TestRun
 *    call; any other execution state (ERROR, NOT_MATCHED, UNSUPPORTED) maps
 *    to errored.
 *  - strength is always surfaced as an advisory note when measured; it never
 *    changes `kind` (a killed/survived mutation witnesses one distinction,
 *    never general adequacy).
 *  - hygiene INELIGIBLE/ABSTAINED is always surfaced as an advisory warning
 *    note; it never changes `kind` either.
 */
export function classifyProjection(projection: Projection): RunOutcome {
  const notes: string[] = [];
  if (projection.strength.state !== "NOT_MEASURED" && projection.strength.state !== "UNSUPPORTED") {
    notes.push(`Strength: ${projection.strength.state}${suffix(projection.strength.reason)}`);
  }
  if (projection.hygiene.state === "INELIGIBLE" || projection.hygiene.state === "ABSTAINED") {
    notes.push(`Hygiene warning: ${projection.hygiene.state}${suffix(projection.hygiene.reason)}`);
  }
  const execution = projection.execution.state;
  if (execution === "INFRASTRUCTURE" || execution === "CANCELLED") {
    return { kind: "errored", message: `${execution}${suffix(projection.execution.reason)}`, notes };
  }
  if (projection.freshness.state === "STALE") {
    return { kind: "stale", message: `STALE${suffix(projection.freshness.reason)}`, notes };
  }
  switch (execution) {
    case "PASSED":
      return { kind: "passed", message: "", notes };
    case "FAILED":
      return { kind: "failed", message: projection.execution.reason.length > 0 ? projection.execution.reason : "test failed", notes };
    case "SKIPPED":
      return { kind: "skipped", message: "", notes };
    default:
      return { kind: "errored", message: `${execution}${suffix(projection.execution.reason)}`, notes };
  }
}

/**
 * Classifies one Go per-test projection (GLTP-V0-051) exactly as
 * `classifyProjection` does, so a `SKIPPED` execution (GLTP-V0-052) is
 * `skipped`. When the projection abstains on execution (`UNSUPPORTED`: a
 * test with no terminal action has no stated outcome), the errored message also names the go test action observed, so
 * the unknown stays explicit and legible rather than looking like a crash.
 */
export function classifyGoTest(test: GoTestResult): RunOutcome {
  const outcome = classifyProjection(test.projection);
  if (outcome.kind !== "errored" || test.projection.execution.state !== "UNSUPPORTED") {
    return outcome;
  }
  return { ...outcome, message: `${outcome.message} (go test action: ${test.action})` };
}

/** The explicit-unknown message for per-test rows the Go producer dropped at its bound. */
export function describeOmittedGoTests(count: number): string {
  return `UNKNOWN: ${count} further test result(s) were omitted by the provider's per-event bound; their per-test state is not shown`;
}

function suffix(reason: string): string {
  return reason.length > 0 ? `: ${reason}` : "";
}

interface DiagnosticEntry {
  readonly line: number;
  readonly message: string;
}

export interface FileDiagnostics {
  readonly file: string;
  readonly entries: readonly { readonly testId: string; readonly line: number; readonly message: string }[];
}

/**
 * Pure bookkeeping for "failing assertions become Diagnostics at file:line;
 * a newer inputIdentity clears diagnostics from older identities." Kept
 * free of the `vscode` module so the clearing rule is directly testable;
 * LiveTestController mirrors each returned FileDiagnostics snapshot into a
 * real vscode.DiagnosticCollection entry for `file`.
 */
export class DiagnosticsLedger {
  private byFile = new Map<string, Map<string, DiagnosticEntry>>();

  /** Drops every tracked diagnostic. Call when a new session identity begins. */
  reset(): void {
    this.byFile.clear();
  }

  /** Records (or clears) one test's diagnostic in `file` and returns the file's current snapshot. */
  apply(file: string, testId: string, failing: boolean, line: number, message: string): FileDiagnostics {
    let entries = this.byFile.get(file);
    if (entries === undefined) {
      entries = new Map();
      this.byFile.set(file, entries);
    }
    if (failing) {
      entries.set(testId, { line, message });
    } else {
      entries.delete(testId);
    }
    return this.snapshot(file);
  }

  snapshot(file: string): FileDiagnostics {
    const entries = this.byFile.get(file);
    return {
      file,
      entries: entries === undefined ? [] : [...entries.entries()].map(([testId, entry]) => ({ testId, line: entry.line, message: entry.message })),
    };
  }

  files(): readonly string[] {
    return [...this.byFile.keys()];
  }
}

/** Why the live test-provider process could not be started or exited abnormally (VSC-V0-066). */
export type LiveTestUnavailableCause =
  | { readonly kind: "invalid-command" }
  | { readonly kind: "spawn-failed"; readonly detail: string }
  | { readonly kind: "process-error"; readonly detail: string }
  | { readonly kind: "process-exited"; readonly code: number | null; readonly signal: string | null }
  | { readonly kind: "retention-refused"; readonly detail: string };

/** Pure decision: maps an unavailability cause to the cause-naming message liveTests.ts reports. */
export function describeLiveTestUnavailability(cause: LiveTestUnavailableCause): string {
  switch (cause.kind) {
    case "invalid-command":
      return "corvint.liveTests.command is empty or is not a list of non-empty strings";
    case "spawn-failed":
      return `the live test-provider process could not be started (${cause.detail})`;
    case "process-error":
      return `the live test-provider process failed (${cause.detail})`;
    case "process-exited":
      return `the live test-provider process exited unexpectedly (code ${cause.code ?? "null"}, signal ${cause.signal ?? "null"})`;
    case "retention-refused":
      return `corvint.liveTests.retainEvidence is true but ${cause.detail}`;
  }
}

/** The subcommands after which each provider accepts `--retain` (LPCV-V0-055). */
const RETAINING_SUBCOMMANDS: Readonly<Record<string, readonly string[]>> = {
  "corvint-js-test-provider": ["unit", "e2e"],
  "corvint-go-test-provider": ["session"],
};

const RETAIN_TOKEN = /^--?retain(=|$)/;

/** The argv to spawn, or the cause that refuses it. */
export type RetentionArgv =
  | { readonly kind: "argv"; readonly argv: readonly string[] }
  | { readonly kind: "unavailable"; readonly cause: LiveTestUnavailableCause };

/**
 * Pure decision (VSC-V0-070): with retention off the argv is unchanged; with it on, `--retain` is
 * inserted directly after a supported provider subcommand, and any other command, or a command
 * that already names a retain flag, is refused rather than run without the retention it asked for.
 */
export function withEvidenceRetention(argv: readonly string[], retain: boolean): RetentionArgv {
  if (!retain) {
    return { kind: "argv", argv };
  }
  const [executable = "", subcommand = "", ...rest] = argv;
  const program = executable.split(/[\\/]/).pop() ?? "";
  const subcommands = RETAINING_SUBCOMMANDS[program] ?? [];
  if (!subcommands.includes(subcommand)) {
    return { kind: "unavailable", cause: { kind: "retention-refused", detail: "corvint.liveTests.command is not a supported Corvint provider subcommand" } };
  }
  if (rest.some((token) => RETAIN_TOKEN.test(token))) {
    return { kind: "unavailable", cause: { kind: "retention-refused", detail: "corvint.liveTests.command already names a retain flag" } };
  }
  return { kind: "argv", argv: [executable, subcommand, "--retain", ...rest] };
}
