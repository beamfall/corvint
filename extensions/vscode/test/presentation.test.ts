import { stub } from "./vscode-stub.js";
import * as assert from "node:assert/strict";
import { mkdtemp, realpath, rm, writeFile } from "node:fs/promises";
import * as os from "node:os";
import * as path from "node:path";
import { test } from "node:test";
import * as vscode from "vscode";
import { CorvintPresentation } from "../src/presentation.js";
import type { CorvintSnapshot, EvidenceItem } from "../src/model.js";

test("VSC-V0-078: diagnostics for unopened files never open documents or reject out-of-file lines", async () => {
  const root = await realpath(await mkdtemp(path.join(os.tmpdir(), "corvint-presentation-")));
  const file = path.join(root, "source.ts");
  const presentation = new CorvintPresentation({ dispose() {} } as vscode.StatusBarItem);
  stub.documentOpenCalls.length = 0;
  const evidence = (line: number): EvidenceItem => ({
    resultId: `result-${line}`, resultKind: "test", summary: "summary", path: "source.ts", line,
    blobHash: "a".repeat(40), reason: "conflicting evidence", confidence: "high", authority: "test",
    resultState: "CONFLICTED",
  });
  const snapshot: CorvintSnapshot = {
    snapshotKey: "snapshot", adapterProfile: "unprofiled context schema_version 1", operation: "query",
    schemaVersion: 1, revision: "a".repeat(40), state: "READY", freshness: "fresh", worktreeMixedPathCount: 0,
    results: [], evidence: [1, 200, 0, -1, 1.5, Number.NaN, Number.POSITIVE_INFINITY].map(evidence),
    uncertainty: [], verification: [],
  };
  try {
    await writeFile(file, "short");
    await presentation.apply(snapshot, root);
    const diagnostics = stub.diagnostics.get(file) as vscode.Diagnostic[] | undefined;
    assert.ok(diagnostics);
    assert.equal(diagnostics.length, 2);
    assert.deepEqual(diagnostics.map((item) => item.range.start.line).sort((a, b) => a - b), [0, 199]);
    for (const diagnostic of diagnostics) {
      assert.equal(diagnostic.range.start.character, 0);
      assert.equal(diagnostic.range.end.line, diagnostic.range.start.line);
      assert.equal(diagnostic.range.end.character, Number.MAX_SAFE_INTEGER);
      assert.equal(diagnostic.source, "Corvint");
      assert.equal(diagnostic.severity, vscode.DiagnosticSeverity.Error);
      assert.equal(diagnostic.code, `result-${diagnostic.range.start.line + 1}`);
    }
    assert.deepEqual(stub.documentOpenCalls, []);
  } finally {
    presentation.dispose();
    await rm(root, { recursive: true, force: true });
  }
});
