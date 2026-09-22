import { createHash } from "node:crypto";
import { lstat, open, realpath } from "node:fs/promises";
import * as path from "node:path";
import { isJsonObject, parseBoundedJson, type JsonObject, type JsonValue } from "./json.js";
import { isRepositoryRelativePath } from "./model.js";

const MAX_OBSERVATION_BYTES = 4 * 1024 * 1024;
const ID_PATTERN = /^[a-z][a-z0-9-]*:sha256:[0-9a-f]{64}$/;
const DIGEST_PATTERN = /^[0-9a-f]{64}$/;

export type ObservedStatus = "PASSED" | "FAILED" | "SKIPPED" | "ERRORED";

export interface ObservedTest {
  readonly id: string;
  readonly status: ObservedStatus;
}

export interface ImportedObservation {
  readonly runId: string;
  readonly wei: string;
  readonly executionStatus: string;
  readonly sourceCurrency: string;
  readonly toolchainCurrency: string;
  readonly cancelled: boolean;
  readonly tests: readonly ObservedTest[];
  readonly verification: "UNVERIFIED_IMPORT";
}

export async function readStableObservation(root: string, relativePath: string): Promise<Uint8Array> {
  if (!isRepositoryRelativePath(relativePath)) {
    throw new Error("PATH_REJECTED: observation path must be normalized and workspace-relative");
  }
  const lexical = path.resolve(root, ...relativePath.split("/"));
  const relative = path.relative(root, lexical);
  if (relative.startsWith("..") || path.isAbsolute(relative)) {
    throw new Error("PATH_REJECTED: observation path escapes the workspace root");
  }
  const link = await lstat(lexical, { bigint: true });
  if (!link.isFile() || link.isSymbolicLink() || link.size > MAX_OBSERVATION_BYTES) {
    throw new Error("PATH_REJECTED: observation must be one bounded regular non-symlink file");
  }
  const resolved = await realpath(lexical);
  if (resolved !== lexical) {
    throw new Error("PATH_REJECTED: observation real path differs from its lexical path");
  }
  const handle = await open(resolved, "r");
  let bytes: Uint8Array;
  let openedIdentity: Awaited<ReturnType<typeof handleStat>>;
  try {
    const before = await handleStat(handle);
    openedIdentity = before;
    if (!before.isFile() || before.size > BigInt(MAX_OBSERVATION_BYTES)) {
      throw new Error("PATH_REJECTED: observation is not a bounded regular file");
    }
    const buffer = Buffer.alloc(Number(before.size));
    let offset = 0;
    while (offset < buffer.byteLength) {
      const read = await handle.read(buffer, offset, buffer.byteLength - offset, offset);
      if (read.bytesRead === 0) {
        break;
      }
      offset += read.bytesRead;
    }
    if (offset !== buffer.byteLength) {
      throw new Error("OBSERVATION_CHANGED: observation ended before its recorded size");
    }
    bytes = buffer;
    const after = await handleStat(handle);
    if (!sameIdentity(before, after)) {
      throw new Error("OBSERVATION_CHANGED: observation identity changed during read");
    }
  } finally {
    await handle.close();
  }
  const afterPath = await lstat(resolved, { bigint: true });
  if (!afterPath.isFile() || afterPath.isSymbolicLink() || !sameIdentity(openedIdentity, afterPath)) {
    throw new Error("OBSERVATION_CHANGED: observation changed after read");
  }
  // Hash the exact admitted immutable handoff. No second pathname read may
  // substitute different bytes after the stable-read proof.
  digest(bytes);
  return bytes;
}

export function decodeObservation(bytes: Uint8Array): ImportedObservation {
  try {
    decodeObservationStructure(bytes);
  } catch {
    throw new Error("UNVERIFIED_IMPORT: no shared Corvint go-live verifier is linked; Test Explorer projection is disabled");
  }
  throw new Error("UNVERIFIED_IMPORT: no shared Corvint go-live verifier is linked; Test Explorer projection is disabled");
}

function decodeObservationStructure(bytes: Uint8Array): ImportedObservation {
  if (bytes.byteLength === 0 || bytes.byteLength > MAX_OBSERVATION_BYTES || bytes[bytes.byteLength - 1] !== 0x0a) {
    throw new Error("INVALID_JSON: observation must be bounded LF-terminated JSONL");
  }
  const lines = splitLines(bytes);
  if (lines.length < 1 || lines.length > 65_536 || lines.some((line) => line.byteLength === 0)) {
    throw new Error("INVALID_JSON: observation has invalid JSONL framing");
  }
  const documents = lines.map((line) => parseBoundedJson(line, {
    maxBytes: MAX_OBSERVATION_BYTES,
    maxDepth: 16,
    maxNodes: 65_536,
    maxStringBytes: 65_536,
  }));
  const terminal = object(documents[documents.length - 1], "terminal");
  if (terminal.profile !== "go-live-run/0") {
    throw new Error("UNSUPPORTED_PROFILE: terminal record must be go-live-run/0");
  }
  const runId = id(terminal.runId, "terminal.runId");
  const wei = id(terminal.wei, "terminal.wei");
  for (let index = 0; index < documents.length - 1; index += 1) {
    const event = object(documents[index], `event[${index}]`);
    if (event.profile !== "go-live-event/0" || event.runId !== runId || event.wei !== wei || event.sequence !== String(index)) {
      throw new Error("UNVERIFIED_IMPORT: event sequence or identity is inconsistent");
    }
    id(event.id, "event.id");
    object(event.fact, "event.fact");
  }
  const eventCount = decimal(terminal.eventCount, "terminal.eventCount");
  if (eventCount !== documents.length - 1 || !DIGEST_PATTERN.test(text(terminal.eventRootSha256, "terminal.eventRootSha256", 64))) {
    throw new Error("UNVERIFIED_IMPORT: terminal event count/root is inconsistent");
  }
  const execution = object(terminal.execution, "terminal.execution");
  const executionStatus = token(execution.status, "terminal.execution.status");
  if (typeof execution.cancelled !== "boolean") {
    throw new Error("INVALID_JSON: terminal.execution.cancelled must be boolean");
  }
  const cancelled = execution.cancelled;
  const sourceCurrency = token(terminal.sourceCurrency, "terminal.sourceCurrency");
  const toolchainCurrency = token(terminal.toolchainCurrency, "terminal.toolchainCurrency");
  const scope = object(terminal.scope, "terminal.scope");
  const rawTests = list(scope.testTerminals, "terminal.scope.testTerminals", 20_000);
  const tests = rawTests.map((value, index) => {
    const test = object(value, `testTerminals[${index}]`);
    const testId = id(test.test, "test terminal id");
    const rawStatus = token(test.status, "test terminal status");
    let status: ObservedStatus;
    if (cancelled) {
      status = "SKIPPED";
    } else if (rawStatus === "PASSED") {
      // No shared Corvint verifier is linked in V0. Structural imports cannot create a passing claim.
      status = "ERRORED";
    } else if (rawStatus === "FAILED") {
      status = "FAILED";
    } else if (rawStatus === "SKIPPED") {
      status = "SKIPPED";
    } else {
      status = "ERRORED";
    }
    return Object.freeze({ id: testId, status });
  });
  if (new Set(tests.map((test) => test.id)).size !== tests.length) {
    throw new Error("UNVERIFIED_IMPORT: duplicate test terminal identity");
  }
  return Object.freeze({
    runId,
    wei,
    executionStatus,
    sourceCurrency,
    toolchainCurrency,
    cancelled,
    tests: Object.freeze(tests),
    verification: "UNVERIFIED_IMPORT" as const,
  });
}

async function handleStat(handle: Awaited<ReturnType<typeof open>>) {
  return await handle.stat({ bigint: true });
}

function digest(bytes: Uint8Array): string {
  return createHash("sha256").update(bytes).digest("hex");
}

function splitLines(bytes: Uint8Array): Uint8Array[] {
  const lines: Uint8Array[] = [];
  let start = 0;
  for (let index = 0; index < bytes.byteLength; index += 1) {
    if (bytes[index] === 0x0a) {
      lines.push(bytes.subarray(start, index));
      start = index + 1;
    }
  }
  if (start !== bytes.byteLength) {
    throw new Error("INVALID_JSON: observation has an unterminated JSONL frame");
  }
  return lines;
}

function sameIdentity(
  left: Awaited<ReturnType<typeof handleStat>>,
  right: Awaited<ReturnType<typeof handleStat>>,
): boolean {
  return left.dev === right.dev && left.ino === right.ino && left.mode === right.mode && left.size === right.size &&
    left.mtimeNs === right.mtimeNs && left.ctimeNs === right.ctimeNs;
}

function object(value: JsonValue | undefined, name: string): JsonObject {
  if (value === undefined || !isJsonObject(value)) {
    throw new Error(`INVALID_JSON: ${name} must be an object`);
  }
  return value;
}

function list(value: JsonValue | undefined, name: string, maximum: number): readonly JsonValue[] {
  if (!Array.isArray(value) || value.length > maximum) {
    throw new Error(`INVALID_JSON: ${name} must be a bounded array`);
  }
  return value;
}

function text(value: JsonValue | undefined, name: string, maximumBytes: number): string {
  if (typeof value !== "string" || Buffer.byteLength(value, "utf8") > maximumBytes) {
    throw new Error(`INVALID_JSON: ${name} must be a bounded string`);
  }
  return value;
}

function id(value: JsonValue | undefined, name: string): string {
  const result = text(value, name, 160);
  if (!ID_PATTERN.test(result)) {
    throw new Error(`INVALID_JSON: ${name} must be a Corvint identity`);
  }
  return result;
}

function token(value: JsonValue | undefined, name: string): string {
  const result = text(value, name, 64);
  if (!/^[A-Z][A-Z0-9_]*$/.test(result)) {
    throw new Error(`INVALID_JSON: ${name} must be an enum token`);
  }
  return result;
}

function decimal(value: JsonValue | undefined, name: string): number {
  const result = text(value, name, 20);
  if (!/^(?:0|[1-9][0-9]*)$/.test(result)) {
    throw new Error(`INVALID_JSON: ${name} must be canonical decimal text`);
  }
  const number = Number(result);
  if (!Number.isSafeInteger(number)) {
    throw new Error(`INVALID_JSON: ${name} is too large`);
  }
  return number;
}
