import * as assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import * as path from "node:path";
import { test } from "node:test";
import {
  classifyGoSessionEvent,
  classifyGoSessionState,
  classifyGoTest,
  classifyProjection,
  describeLiveTestUnavailability,
  describeOmittedGoTests,
  DiagnosticsLedger,
  extractJsonRecords,
  beginsNewRun,
  MAX_RECORD_BYTES,
  ProtocolError,
  parseProviderRecord,
  shouldApplyRecord,
  withEvidenceRetention,
} from "../src/liveTestProtocol.js";
import type { Projection } from "../src/testvalidity.js";

const VECTORS_PATH = path.join(__dirname, "..", "..", "..", "..", "conformance", "test-validity-v0", "vectors.json");

interface VectorCase {
  readonly name: string;
  readonly expected: Projection;
}

interface VectorFile {
  readonly cases: readonly VectorCase[];
}

test("parseProviderRecord decodes a Go session event", () => {
  const record = parseProviderRecord(
    JSON.stringify({
      profile: "corvint-go-live-session-event/0",
      state: "running",
      identity: "abc123",
      sequence: 1,
      scope: ["github.com/corvint-context/corvint/internal/testvalidity"],
      detail: "",
    }),
  );
  assert.equal(record.kind, "go-session");
  if (record.kind !== "go-session") throw new Error("unreachable");
  assert.equal(record.state, "running");
  assert.equal(record.identity, "abc123");
  assert.equal(record.sequence, 1);
  assert.deepEqual(record.scope, ["github.com/corvint-context/corvint/internal/testvalidity"]);
});

test("parseProviderRecord decodes a Go session idle event with a null scope", () => {
  const record = parseProviderRecord(JSON.stringify({ profile: "corvint-go-live-session-event/0", state: "idle", identity: "abc123", sequence: 0, scope: null, detail: "" }));
  assert.equal(record.kind, "go-session");
  if (record.kind !== "go-session") throw new Error("unreachable");
  assert.deepEqual(record.scope, []);
});

function goTestProjection(execution: string, reason: string, freshness: string): Projection {
  const unsupported = { state: "UNSUPPORTED", reason: "no-association-input", anchors: null };
  return {
    association: unsupported,
    hygiene: unsupported,
    freshness: { state: freshness, reason: freshness === "STALE" ? "workspace-execution-identity-mismatch" : "", anchors: ["input-identity:abc"] },
    execution: { state: execution, reason, anchors: execution === "UNSUPPORTED" ? null : ["input-identity:abc"] },
    strength: { state: "NOT_MEASURED", reason: "no-mutation-run", anchors: null },
  };
}

test("parseProviderRecord decodes a Go session event's per-test projections and omitted count (GLTP-V0-051)", () => {
  const record = parseProviderRecord(
    JSON.stringify({
      profile: "corvint-go-live-session-event/0",
      state: "failed",
      identity: "abc",
      sequence: 2,
      scope: ["fixture"],
      detail: "TestFail",
      testProjections: [{ package: "fixture", name: "TestFail", action: "fail", projection: goTestProjection("FAILED", "ASSERTION_OR_TEST", "CURRENT") }],
      testProjectionsOmitted: 3,
    }),
  );
  if (record.kind !== "go-session") throw new Error("unreachable");
  assert.equal(record.tests.length, 1);
  assert.equal(record.tests[0]?.testId, "fixture#TestFail");
  assert.equal(record.tests[0]?.projection.execution.state, "FAILED");
  assert.equal(record.testsOmitted, 3);
  const bare = parseProviderRecord(JSON.stringify({ profile: "corvint-go-live-session-event/0", state: "passed", identity: "abc", sequence: 1, scope: null, detail: "" }));
  if (bare.kind !== "go-session") throw new Error("unreachable");
  assert.deepEqual(bare.tests, []);
  assert.equal(bare.testsOmitted, 0);
  assert.throws(
    () => parseProviderRecord(JSON.stringify({ profile: "corvint-go-live-session-event/0", state: "passed", identity: "abc", sequence: 1, scope: null, detail: "", testProjections: [{ package: "fixture", name: "TestX", action: "pass" }] })),
    ProtocolError,
  );
});

test("classifyGoTest states pass/fail/stale from the projection and keeps an abstained result an explicit unknown", () => {
  const test = (action: string, projection: Projection) =>
    classifyGoTest({ testId: `fixture#${action}`, package: "fixture", name: action, action, file: undefined, line: undefined, projection });
  assert.equal(test("pass", goTestProjection("PASSED", "", "CURRENT")).kind, "passed");
  assert.equal(test("fail", goTestProjection("FAILED", "ASSERTION_OR_TEST", "CURRENT")).kind, "failed");
  assert.equal(test("pass", goTestProjection("PASSED", "", "STALE")).kind, "stale");
  assert.equal(test("skip", goTestProjection("SKIPPED", "test-skipped", "CURRENT")).kind, "skipped");
  const never = test("none", goTestProjection("UNSUPPORTED", "no-execution-input", "CURRENT"));
  assert.equal(never.kind, "errored");
  assert.equal(never.message, "UNSUPPORTED: no-execution-input (go test action: none)");
  assert.match(describeOmittedGoTests(3), /^UNKNOWN: 3 /);
});

test("parseProviderRecord decodes a JS provider receipt document, recovering file:line from the execution anchor", () => {
  const projection: Projection = {
    association: { state: "ASSOCIATED", reason: "", anchors: null },
    hygiene: { state: "ELIGIBLE", reason: "", anchors: null },
    freshness: { state: "CURRENT", reason: "", anchors: null },
    execution: { state: "FAILED", reason: "assertion failed", anchors: ["pkg/foo.test.ts:42"] },
    strength: { state: "NOT_MEASURED", reason: "no-mutation-run", anchors: null },
  };
  const record = parseProviderRecord(
    JSON.stringify({
      receipt: { kind: "unit" },
      testProjections: [{ name: "foo > bar", state: "failed", projection }],
      runProjection: { ...projection, execution: { state: "PASSED", reason: "", anchors: null } },
    }),
  );
  assert.equal(record.kind, "js-provider");
  if (record.kind !== "js-provider") throw new Error("unreachable");
  assert.equal(record.runKind, "unit");
  assert.equal(record.tests.length, 1);
  assert.equal(record.tests[0]?.testId, "foo > bar");
  assert.equal(record.tests[0]?.file, "pkg/foo.test.ts");
  assert.equal(record.tests[0]?.line, 42);
});

test("parseProviderRecord recovers a bare-file anchor (no line) as line 1", () => {
  const projection: Projection = {
    association: { state: "ASSOCIATED", reason: "", anchors: null },
    hygiene: { state: "ELIGIBLE", reason: "", anchors: null },
    freshness: { state: "CURRENT", reason: "", anchors: null },
    execution: { state: "INFRASTRUCTURE", reason: "missing browser", anchors: ["pkg/foo.test.ts"] },
    strength: { state: "NOT_MEASURED", reason: "no-mutation-run", anchors: null },
  };
  const record = parseProviderRecord(
    JSON.stringify({ receipt: { kind: "e2e" }, testProjections: [{ name: "t", state: "infrastructure", projection }], runProjection: projection }),
  );
  if (record.kind !== "js-provider") throw new Error("unreachable");
  assert.equal(record.tests[0]?.file, "pkg/foo.test.ts");
  assert.equal(record.tests[0]?.line, 1);
});

test("parseProviderRecord leaves a JS test with no anchor as file/line undefined", () => {
  const projection: Projection = {
    association: { state: "ASSOCIATED", reason: "", anchors: null },
    hygiene: { state: "ELIGIBLE", reason: "", anchors: null },
    freshness: { state: "CURRENT", reason: "", anchors: null },
    execution: { state: "PASSED", reason: "", anchors: null },
    strength: { state: "NOT_MEASURED", reason: "no-mutation-run", anchors: null },
  };
  const record = parseProviderRecord(JSON.stringify({ receipt: { kind: "unit" }, testProjections: [{ name: "t", state: "passed", projection }], runProjection: projection }));
  if (record.kind !== "js-provider") throw new Error("unreachable");
  assert.equal(record.tests[0]?.file, undefined);
  assert.equal(record.tests[0]?.line, undefined);
});

test("parseProviderRecord rejects malformed, empty, unsupported-shape, and unsupported-state input", () => {
  assert.throws(() => parseProviderRecord(""), ProtocolError);
  assert.throws(() => parseProviderRecord("not json"), ProtocolError);
  assert.throws(() => parseProviderRecord(JSON.stringify({ kind: "validity" })), ProtocolError, "the retired kind:validity shape is not carried by any real producer");
  assert.throws(() => parseProviderRecord(JSON.stringify({ profile: "corvint-go-live-session-event/0", state: "bogus", identity: "i", sequence: 0, scope: null, detail: "" })), ProtocolError);
});

test("extractJsonRecords splits compact NDJSON lines (the Go session's framing)", () => {
  const buffer = '{"profile":"corvint-go-live-session-event/0","state":"idle","identity":"a","sequence":0,"scope":null,"detail":""}\n{"profile":"corvint-go-live-session-event/0","state":"running","identity":"a","sequence":1,"scope":["x"],"detail":""}\n';
  const { records, rest } = extractJsonRecords(buffer);
  assert.equal(records.length, 2);
  assert.equal(rest, "");
  assert.equal(JSON.parse(records.at(0) ?? "null").state, "idle");
  assert.equal(JSON.parse(records.at(1) ?? "null").state, "running");
});

test("extractJsonRecords assembles one pretty-printed multi-line JSON document (the JS provider's framing)", () => {
  const buffer = '{\n  "receipt": {\n    "kind": "unit"\n  },\n  "testProjections": []\n}\n';
  const { records, rest } = extractJsonRecords(buffer);
  assert.equal(records.length, 1);
  assert.equal(rest, "");
  assert.equal(JSON.parse(records.at(0) ?? "null").receipt.kind, "unit");
});

test("extractJsonRecords holds back an incomplete trailing record for the next chunk", () => {
  const { records, rest } = extractJsonRecords('{"profile":"corvint-go-live-session-event/0","state":"idle"');
  assert.equal(records.length, 0);
  assert.equal(rest, '{"profile":"corvint-go-live-session-event/0","state":"idle"');
});

test("classifyProjection matches every shared test-validity vector's execution/freshness/strength/hygiene mapping", async () => {
  const raw = await readFile(VECTORS_PATH, "utf-8");
  const parsed = JSON.parse(raw) as VectorFile;
  for (const testCase of parsed.cases) {
    const outcome = classifyProjection(testCase.expected);
    const execution = testCase.expected.execution.state;
    const freshness = testCase.expected.freshness.state;
    if (execution === "INFRASTRUCTURE" || execution === "CANCELLED") {
      assert.equal(outcome.kind, "errored", testCase.name);
    } else if (freshness === "STALE") {
      assert.equal(outcome.kind, "stale", testCase.name);
    } else if (execution === "PASSED") {
      assert.equal(outcome.kind, "passed", testCase.name);
    } else if (execution === "FAILED") {
      assert.equal(outcome.kind, "failed", testCase.name);
    } else if (execution === "SKIPPED") {
      assert.equal(outcome.kind, "skipped", testCase.name);
    } else {
      assert.equal(outcome.kind, "errored", testCase.name);
    }
    const strength = testCase.expected.strength.state;
    const hasStrengthNote = outcome.notes.some((note) => note.startsWith("Strength:"));
    assert.equal(hasStrengthNote, strength !== "NOT_MEASURED" && strength !== "UNSUPPORTED", `${testCase.name} strength note`);
    const hygiene = testCase.expected.hygiene.state;
    const hasHygieneNote = outcome.notes.some((note) => note.startsWith("Hygiene warning:"));
    assert.equal(hasHygieneNote, hygiene === "INELIGIBLE" || hygiene === "ABSTAINED", `${testCase.name} hygiene note`);
  }
});

test("classifyProjection: freshness STALE overrides a reported PASSED execution", () => {
  const projection: Projection = {
    association: { state: "ASSOCIATED", reason: "", anchors: null },
    hygiene: { state: "ELIGIBLE", reason: "", anchors: null },
    freshness: { state: "STALE", reason: "workspace-execution-identity-mismatch", anchors: null },
    execution: { state: "PASSED", reason: "", anchors: null },
    strength: { state: "NOT_MEASURED", reason: "no-mutation-run", anchors: null },
  };
  const outcome = classifyProjection(projection);
  assert.equal(outcome.kind, "stale");
  assert.match(outcome.message, /^STALE: workspace-execution-identity-mismatch$/);
});

test("parseProviderRecord recovers a Go per-test source anchor only from a leading file:line execution anchor (VSC-V0-069)", () => {
  const anchored = goTestProjection("FAILED", "ASSERTION_OR_TEST", "CURRENT");
  const entry = (projection: Projection) => ({ package: "fixture", name: "TestFail", action: "fail", projection });
  const record = parseProviderRecord(
    JSON.stringify({
      profile: "corvint-go-live-session-event/0",
      state: "failed",
      identity: "abc",
      sequence: 2,
      scope: ["fixture"],
      detail: "TestFail",
      testProjections: [
        entry({ ...anchored, execution: { ...anchored.execution, anchors: ["pkg/fixture_test.go:12", "input-identity:abc"] } }),
        entry(anchored),
        entry({ ...anchored, execution: { ...anchored.execution, anchors: ["pkg/fixture_test.go", "input-identity:abc"] } }),
      ],
    }),
  );
  if (record.kind !== "go-session") throw new Error("unreachable");
  assert.deepEqual(
    record.tests.map((t) => [t.file, t.line]),
    [["pkg/fixture_test.go", 12], [undefined, undefined], [undefined, undefined]],
  );
});

test("classifyGoSessionEvent decodes the session projection and never lets it override the state (VSC-V0-068)", () => {
  const event = (state: string, projection: Projection | undefined) => {
    const record = parseProviderRecord(JSON.stringify({ profile: "corvint-go-live-session-event/0", state, identity: "abc", sequence: 1, scope: ["fixture"], detail: "TestFail", projection }));
    if (record.kind !== "go-session") throw new Error("unreachable");
    return record;
  };
  const failed = event("failed", goTestProjection("FAILED", "ASSERTION_OR_TEST", "CURRENT"));
  assert.equal(failed.projection?.execution.state, "FAILED");
  assert.deepEqual(classifyGoSessionEvent(failed), { kind: "failed", message: "TestFail" });
  assert.equal(classifyGoSessionEvent(event("stale", goTestProjection("INFRASTRUCTURE", "STALE", "STALE"))).kind, "errored");
  assert.equal(classifyGoSessionEvent(event("passed", undefined)).kind, "passed");
  const contradicted = classifyGoSessionEvent(event("passed", goTestProjection("FAILED", "ASSERTION_OR_TEST", "CURRENT")));
  assert.equal(contradicted.kind, "errored");
  assert.match(contradicted.message, /^UNKNOWN: session state passed contradicts its projection \(failed/);
  assert.throws(
    () => parseProviderRecord(JSON.stringify({ profile: "corvint-go-live-session-event/0", state: "passed", identity: "abc", sequence: 1, scope: null, detail: "", projection: "PASSED" })),
    ProtocolError,
  );
});

test("classifyGoSessionState: passed/failed/stale/infrastructure/cancelled map to exactly one TestRun call, never failed for infra/cancelled", () => {
  assert.equal(classifyGoSessionState("passed", "").kind, "passed");
  assert.equal(classifyGoSessionState("failed", "assertion failed").kind, "failed");
  assert.equal(classifyGoSessionState("stale", "").kind, "errored");
  assert.match(classifyGoSessionState("stale", "").message, /^STALE/);
  assert.equal(classifyGoSessionState("infrastructure", "build failed").kind, "errored");
  assert.equal(classifyGoSessionState("cancelled", "").kind, "errored");
});

test("shouldApplyRecord: no active identity admits any record; a mismatched identity is dropped", () => {
  assert.equal(shouldApplyRecord(undefined, "wei:1"), true);
  assert.equal(shouldApplyRecord("wei:2", "wei:2"), true);
  assert.equal(shouldApplyRecord("wei:2", "wei:1"), false, "an older identity arriving after a newer session started is superseded");
});

test("DiagnosticsLedger clears diagnostics from a superseded identity on reset", () => {
  const ledger = new DiagnosticsLedger();
  ledger.apply("pkg/foo_test.go", "unit:TestFoo", true, 10, "assertion failed");
  assert.deepEqual(ledger.snapshot("pkg/foo_test.go").entries, [{ testId: "unit:TestFoo", line: 10, message: "assertion failed" }]);
  ledger.reset();
  assert.deepEqual(ledger.snapshot("pkg/foo_test.go").entries, []);
});

test("DiagnosticsLedger clears one test's own diagnostic once it stops failing", () => {
  const ledger = new DiagnosticsLedger();
  ledger.apply("pkg/foo_test.go", "unit:TestFoo", true, 10, "assertion failed");
  ledger.apply("pkg/foo_test.go", "unit:TestFoo", false, 10, "");
  assert.deepEqual(ledger.snapshot("pkg/foo_test.go").entries, []);
});

test("extractJsonRecords keeps the complete records that precede an unbounded trailing value", () => {
  const good = '{"profile":"corvint-go-live-session-event/0","state":"idle"}';
  const { records, rest, overflow } = extractJsonRecords(`${good}\n{${"x".repeat(MAX_RECORD_BYTES + 1)}`);
  assert.deepEqual(records, [good]);
  assert.equal(rest, "");
  assert.equal(overflow, true);
});

test("extractJsonRecords discards the rest of an overflowed record through its LF before scanning again", () => {
  // The next chunk can begin at a "{" inside the dropped record's string; scanned as a record start, its
  // inverted quote parity would swallow the complete records behind it.
  const good = '{"profile":"corvint-go-live-session-event/0","state":"idle"}';
  assert.equal(extractJsonRecords(`{"pad":"${"x".repeat(MAX_RECORD_BYTES + 1)}`).overflow, true);
  assert.deepEqual(extractJsonRecords('{"a"', true), { records: [], rest: "", overflow: true });
  assert.deepEqual(extractJsonRecords(`{inside"}\n${good}\n`, true), { records: [good], rest: "", overflow: false });
});

test("a repeated running event with the active identity opens a run once the previous one closed", () => {
  assert.equal(beginsNewRun(undefined, false, "a"), true);
  assert.equal(beginsNewRun("a", true, "a"), false);
  assert.equal(beginsNewRun("a", false, "a"), true);
  assert.equal(beginsNewRun("a", true, "b"), true);
});

test("extractJsonRecords resyncs after a non-JSON line and still delivers the record behind it", () => {
  // A provider that writes a stray non-JSON line (a toolchain warning, a
  // `go: downloading ...` notice) must not cost the caller every result that
  // follows it: the bad line is surfaced as one record, dropped by
  // parseProviderRecord per VSC-V0-065, and the next complete record is still
  // extracted from the same buffer.
  const good = '{"profile":"corvint-go-live-session-event/0","state":"running","identity":"wei:1","sequence":1,"scope":["./internal/x"],"detail":""}';
  const { records, rest, overflow } = extractJsonRecords(`go: downloading example.com/m v1.2.3\n${good}\n`);
  assert.deepEqual(records, ["go: downloading example.com/m v1.2.3", good]);
  assert.equal(rest, "");
  assert.equal(overflow, false);
  assert.throws(() => parseProviderRecord(records.at(0) ?? ""), ProtocolError);
  const record = parseProviderRecord(records.at(1) ?? "");
  assert.equal(record.kind, "go-session");
  assert.equal(record.kind === "go-session" ? record.identity : undefined, "wei:1");
});

test("describeLiveTestUnavailability names a distinct cause for each unavailability case (VSC-V0-066)", () => {
  const messages = [
    describeLiveTestUnavailability({ kind: "invalid-command" }),
    describeLiveTestUnavailability({ kind: "spawn-failed", detail: "ENOENT" }),
    describeLiveTestUnavailability({ kind: "process-error", detail: "EACCES" }),
    describeLiveTestUnavailability({ kind: "process-exited", code: 1, signal: null }),
  ];
  assert.equal(new Set(messages).size, messages.length, "each cause must produce a distinct message");
  assert.match(messages[0] ?? "", /corvint\.liveTests\.command/);
  assert.match(messages[1] ?? "", /could not be started/);
  assert.match(messages[1] ?? "", /ENOENT/);
  assert.match(messages[2] ?? "", /failed/);
  assert.match(messages[2] ?? "", /EACCES/);
  assert.match(messages[3] ?? "", /exited unexpectedly/);
  assert.match(messages[3] ?? "", /code 1/);
  assert.match(messages[3] ?? "", /signal null/);
});

test("Corvint provider retention preserves legacy aliases (CRB-V0-013 VSC-V0-070)", () => {
  const go = ["/opt/corvint/bin/corvint-go-test-provider", "session", "--experimental", "--foreground"];
  assert.deepEqual(withEvidenceRetention(go, false), { kind: "argv", argv: go });
  assert.deepEqual(withEvidenceRetention(go, true), {
    kind: "argv",
    argv: ["/opt/corvint/bin/corvint-go-test-provider", "session", "--retain", "--experimental", "--foreground"],
  });
  assert.deepEqual(withEvidenceRetention(["corvint-js-test-provider", "unit", "--dir", "web"], true), {
    kind: "argv",
    argv: ["corvint-js-test-provider", "unit", "--retain", "--dir", "web"],
  });
  assert.deepEqual(withEvidenceRetention(["corvint-js-test-provider", "e2e"], true), { kind: "argv", argv: ["corvint-js-test-provider", "e2e", "--retain"] });

  const refusedCommands = [
    ["corvint-go-test-provider", "--experimental", "--trusted-local", "--attachment"],
    ["sh", "-c", "corvint-js-test-provider unit"],
    ["corvint-js-test-provider", "unit", "--retain=false"],
    ["corvint-go-test-provider", "session", "-retain"],
  ];
  for (const command of refusedCommands) {
    const refused = withEvidenceRetention(command, true);
    assert.equal(refused.kind, "unavailable", command.join(" "));
    if (refused.kind !== "unavailable") throw new Error("unreachable");
    assert.match(describeLiveTestUnavailability(refused.cause), /corvint\.liveTests\.retainEvidence/);
  }
});
