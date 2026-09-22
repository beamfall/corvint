import * as assert from "node:assert/strict";
import { test } from "node:test";
import { JsonValidationError, parseBoundedJson } from "../src/json.js";
import {
  attachTransportAuthority,
  createTransportAbstentionSnapshot,
  decodeCorvintReceipt,
  diagnosticClass,
  display,
  orderedEvidence,
  orderedResults,
  projectionClass,
  stableEvidenceId,
  stableResultId,
  type EvidenceItem,
  type ResultItem,
  type TransportAuthority,
} from "../src/model.js";
import { decodeObservation } from "../src/observations.js";

test("display strips line/paragraph separators, zero-width and bidi controls, and BOM", () => {
  const hostile = "a\u2028b\u2029c\u200bd\u200ce\u200df\u2060g\u2061h\u2062i\u2063j\u2064k\ufeffl"
    + "m\u202an\u2066o";
  const cleaned = display(hostile, 1_000);
  for (const forbidden of ["\u2028", "\u2029", "\u200b", "\u200c", "\u200d", "\u2060", "\u2061", "\u2062", "\u2063", "\u2064", "\ufeff", "\u202a", "\u2066"]) {
    assert.ok(!cleaned.includes(forbidden), `expected ${JSON.stringify(forbidden)} to be stripped`);
  }
  assert.equal([...cleaned].filter((scalar) => scalar === "\ufffd").length, 13);
});

test("bounded JSON rejects duplicate, dangerous, invalid-Unicode, and over-depth input", () => {
  for (const source of [
    '{"a":1,"a":2}',
    '{"__proto__":1}',
    '{"a":"\\ud800"}',
    '{"a":{"b":{"c":1}}}',
  ]) {
    assert.throws(
      () => parseBoundedJson(Buffer.from(source), { maxBytes: 1_024, maxDepth: source.includes('"c"') ? 1 : 32 }),
      JsonValidationError,
    );
  }
});

test("strict CLI decoder produces deterministic snapshot identity", () => {
  const bytes = receiptBytes("result-id");
  const first = decodeCorvintReceipt(bytes, "query", 262_144, "corvint", "project operations");
  const second = decodeCorvintReceipt(bytes, "query", 262_144, "corvint", "project operations");
  assert.equal(first.snapshotKey, second.snapshotKey);
  assert.equal(first.results[0]?.id, "result-id");
  assert.equal(first.evidence[0]?.resultId, "result-id");
});

test("strict CLI decoder rejects unsafe identities and non-Git blob widths", () => {
  assert.throws(
    () => decodeCorvintReceipt(receiptBytes("bad\u202eid"), "query", 262_144, "corvint", "project operations"),
    /unsafe identity text/,
  );
  const object = JSON.parse(Buffer.from(receiptBytes("result-id")).toString("utf8")) as {
    context: { results: Array<{ evidence: Array<{ blob_hash: string }> }> };
  };
  const result = object.context.results[0];
  const evidence = result?.evidence[0];
  assert.ok(evidence);
  evidence.blob_hash = "a".repeat(41);
  assert.throws(
    () => decodeCorvintReceipt(Buffer.from(`${JSON.stringify(object)}\n`), "query", 262_144, "corvint", "project operations"),
    /blob identity/,
  );
});

test("observation projection fails closed without the shared Corvint verifier", () => {
  assert.throws(() => decodeObservation(Buffer.from('{}\n')), /UNVERIFIED_IMPORT/);
});

test("projection ordering prefers ordinals then exact state, UTF-8 path, range, and immutable ID", () => {
  const candidate = result("candidate", "CANDIDATE", "src/z.go", 9);
  const unknown = result("unknown", "UNKNOWN", "src/b.go", 7);
  const proven = result("proven", "PROVEN", "src/a.go", 3);
  const ordinal = { ...result("ordinal", "PROVEN", "src/y.go", 4), ordinal: 2 };
  const candidateEvidence = candidate.evidence[0];
  const provenEvidence = proven.evidence[0];
  const unknownEvidence = unknown.evidence[0];
  assert.ok(candidateEvidence);
  assert.ok(provenEvidence);
  assert.ok(unknownEvidence);
  assert.deepEqual(orderedResults([candidate, proven, ordinal, unknown]).map((item) => item.id), [
    "ordinal",
    "unknown",
    "proven",
    "candidate",
  ]);
  assert.deepEqual(orderedEvidence([candidateEvidence, provenEvidence, unknownEvidence])
    .map((item) => item.resultId), ["unknown", "proven", "candidate"]);
});

test("stable projection IDs do not depend on input array positions", () => {
  const first = result("stable", "PROVEN", "src/a.go", 3);
  const second = result("other", "UNKNOWN", "src/b.go", 7);
  const firstEvidence = first.evidence[0];
  const secondEvidence = second.evidence[0];
  const reorderedResult = orderedResults([first, second]).find((item) => item.id === "stable");
  assert.ok(firstEvidence);
  assert.ok(secondEvidence);
  assert.ok(reorderedResult);
  const reorderedEvidence = orderedEvidence([secondEvidence, firstEvidence]).find((item) => item.resultId === "stable");
  assert.ok(reorderedEvidence);
  assert.equal(stableResultId(reorderedResult), stableResultId(first));
  assert.equal(stableEvidenceId(reorderedEvidence), stableEvidenceId(firstEvidence));
});

test("projection classes fail unknown and absent states closed", () => {
  assert.equal(projectionClass("PROVEN"), "evidence");
  assert.equal(projectionClass("CANDIDATE"), "candidate");
  assert.equal(projectionClass("UNKNOWN"), "unknown");
  assert.equal(projectionClass(undefined), "unknown");
  assert.equal(projectionClass("unexpected"), "unknown");
  assert.equal(diagnosticClass("CONFLICTED"), "error");
  assert.equal(diagnosticClass("UNKNOWN"), "warning");
  assert.equal(diagnosticClass("CANDIDATE"), "information");
  assert.equal(diagnosticClass("PROVEN"), "none");
  assert.equal(diagnosticClass(undefined), "none");
});

test("MCP authority attachment is immutable and null receipt preserves only the bounded gap", () => {
  const decoded = decodeCorvintReceipt(receiptBytes("result-id"), "query", 262_144, "corvint", "project operations");
  const authority = transportAuthority();
  const attached = attachTransportAuthority(decoded, authority);
  assert.notEqual(attached, decoded);
  assert.equal(attached.transportAuthority?.executableSha256, "d".repeat(64));
  assert.ok(Object.isFrozen(attached));
  assert.ok(Object.isFrozen(attached.transportAuthority));
  assert.ok(Object.isFrozen(attached.transportAuthority?.repository));

  const abstentionAuthority: TransportAuthority = {
    profile: authority.profile,
    tool: authority.tool,
    state: "ABSTAINED",
    epistemicClass: "NOT_OBSERVED",
    authorityClass: "NONE",
    abstentionReason: "UNSUPPORTED_INTENT",
    serverVersion: authority.serverVersion,
    executableSha256: authority.executableSha256,
    protocol: authority.protocol,
  };
  const abstained = createTransportAbstentionSnapshot(
    "query",
    "unsupported task",
    Buffer.from('{"bridge":"bounded"}'),
    abstentionAuthority,
  );
  assert.equal(abstained.state, "ABSTAINED");
  assert.deepEqual(abstained.results, []);
  assert.deepEqual(abstained.evidence, []);
  assert.deepEqual(abstained.uncertainty, ["UNSUPPORTED_INTENT"]);
  assert.equal(abstained.transportAuthority?.authorityClass, "NONE");
});

function result(id: string, state: string, path: string, line: number): ResultItem {
  const evidence: EvidenceItem = Object.freeze({
    resultId: id,
    resultKind: "path",
    summary: id,
    path,
    line,
    blobHash: "b".repeat(40),
    reason: `reason-${id}`,
    confidence: "high",
    authority: "repository",
    resultState: state,
  });
  return Object.freeze({ id, kind: "path", summary: id, state, evidence: Object.freeze([evidence]) });
}

function transportAuthority(): TransportAuthority {
  return Object.freeze({
    profile: "corvint-mcp-bridge-result/0",
    tool: "corvint.query",
    state: "READY",
    epistemicClass: "OBSERVED",
    authorityClass: "REPOSITORY_EVIDENCE",
    abstentionReason: "NONE",
    repository: Object.freeze({
      commitRevision: "a".repeat(40),
      treeRevision: "b".repeat(40),
      objectFormat: "sha1",
      profileId: "generic",
      worktreeState: "CLEAN",
      dirtyPathCount: 0,
      dirtyPathsSha256: "c".repeat(64),
    }),
    serverVersion: "display-only",
    executableSha256: "d".repeat(64),
    protocol: "2026-07-28",
  });
}

function receiptBytes(resultId: string): Uint8Array {
  const revision = "a".repeat(40);
  return Buffer.from(`${JSON.stringify({
    context: {
      schema_version: 1,
      mode: "query",
      request: { limit: 1, text: "project operations" },
      revision,
      freshness: { state: "fresh", revision, mixed_path_count: 0 },
      state: "READY",
      results: [{
        id: resultId,
        kind: "path",
        summary: "bounded result",
        status: "UNKNOWN",
        evidence: [{
          path: "src/a.go",
          line: 1,
          blob_hash: "b".repeat(40),
          reason: "matched authority",
          confidence: "high",
          authority: "repository",
        }],
      }],
      exclusions: { count: 0 },
      verification: [],
      coverage: { within_budget: true, packet_bytes: 1, uncertainty: [], critical_missing: [] },
      abstention: { active: false, reason: "none" },
      intent: {},
      learning: {},
    },
    mutates: false,
    ok: true,
    tool: "query",
  })}\n`, "utf8");
}
