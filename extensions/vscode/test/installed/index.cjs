const assert = require("node:assert/strict");
const { createHash } = require("node:crypto");
const { readFile, readdir, writeFile } = require("node:fs/promises");
const path = require("node:path");
const vscode = require("vscode");
const { callMcpDiscover } = require("./mcp-client.cjs");

const root = process.env.CORVINT_FIXTURE_ROOT;
const rootUri = vscode.Uri.file(root);
const extensionId = "corvint.corvint-vscode";
const timeout = 180_000;

exports.run = async function run() {
  const extension = vscode.extensions.getExtension(extensionId);
  assert.ok(extension, `${extensionId} was not installed in the isolated host`);
  const api = await extension.activate();
  assert.equal(typeof api?.liveTestSnapshot, "function", "extension must export liveTestSnapshot(root)");
  assert.equal(vscode.workspace.isTrusted, true, "--disable-workspace-trust must yield a trusted isolated workspace");

  if (process.env.CORVINT_INTERRUPT_WITNESS === "1") {
    await configure(e2eCommand(), true);
    await waitSnapshot(api, (snapshot) => snapshot.phase === "pending", "interrupt E2E provider start");
    await writeFile(process.env.CORVINT_INTERRUPT_MARKER, "ready\n");
    await new Promise(() => {});
    return;
  }

  const observations = [];
  await configure(unitCommand(), true);
  observations.push(await qualifyCycle(api, "unit", "math.js", "return left + right;", "return left - right;", "unit.test.js"));

  await configure(e2eCommand(), true);
  observations.push(await qualifyCycle(api, "e2e", "app/index.html", '<output id="count">0</output>', '<output id="count">9</output>', "e2e.spec.js"));

  await configure(goCommand(), true);
  const initialGo = await completedAfter(api, undefined, "initial Go pass");
  assertSnapshotState(initialGo, "passed");
  observations.push(await compareMcp(initialGo));
  await editDocument("go/calc.go", "return left + right", "return left - right");
  const failedGo = await waitSnapshot(api, (snapshot) => snapshot.phase === "completed" && snapshot.inputIdentity !== initialGo.inputIdentity, "Go failure");
  assertSnapshotState(failedGo, "failed");
  await assertDiagnostics("go/calc_test.go", true);
  observations.push(await compareMcp(failedGo));
  await editDocument("go/calc.go", "return left - right", "return left + right");
  const fixedGo = await waitSnapshot(api, (snapshot) => snapshot.phase === "completed" && snapshot.inputIdentity !== failedGo.inputIdentity, "Go fix");
  assertSnapshotState(fixedGo, "passed");
  await assertDiagnostics("go/calc_test.go", false);
  observations.push(await compareMcp(fixedGo));

  const document = await editDocument("go/calc.go", "return left + right", "return left + right // stale trigger");
  const running = await waitSnapshot(api, (snapshot) => snapshot.phase === "running" && snapshot.inputIdentity !== fixedGo.inputIdentity, "Go running before stale edit");
  await replaceAndSave(document, "return left + right // stale trigger", "return left + right // changed during run");
  const stale = await waitSnapshot(api, (snapshot) => snapshot.phase === "completed" && snapshot.inputIdentity === running.inputIdentity && snapshot.items.some((item) => item.state === "errored"), "Go stale terminal");
  await configure([], false);
  observations.push(await compareMcp(stale));

  const disabled = await waitSnapshot(api, (snapshot) => snapshot.phase === "disabled", "disabled shutdown");
  assert.equal(disabled.items.length, 0);
  assert.equal(vscode.languages.getDiagnostics().flatMap(([, diagnostics]) => diagnostics).filter(isCorvintDiagnostic).length, 0);
  process.stdout.write(`${JSON.stringify({ profile: "corvint-interactive-alpha-installed/0", status: "PASS", fixture: { commit: process.env.CORVINT_FIXTURE_COMMIT, tree: process.env.CORVINT_FIXTURE_TREE }, host: { vscode: "1.137.0", isolatedUserData: true, isolatedExtensions: true, updates: "disabled", settingsSync: "off", telemetry: "off", profileSettingsSHA256: process.env.CORVINT_PROFILE_SETTINGS_SHA256 }, tools: JSON.parse(process.env.CORVINT_TOOL_IDENTITIES), observations })}\n`);
};

async function qualifyCycle(api, provider, source, passingText, failingText, diagnosticFile) {
  const initial = await completedAfter(api, undefined, `initial ${provider} pass`);
  assert.equal(initial.provider, provider === "unit" ? "js-unit" : "js-e2e");
  assertSnapshotState(initial, "passed");
  const initialMcp = await compareMcp(initial);
  const failed = await saveAndWait(api, source, passingText, failingText, initial.generation, `${provider} failure`);
  assertSnapshotState(failed, "failed");
  await assertDiagnostics(diagnosticFile, true);
  const failedMcp = await compareMcp(failed);
  const fixed = await saveAndWait(api, source, failingText, passingText, failed.generation, `${provider} fix`);
  assertSnapshotState(fixed, "passed");
  await assertDiagnostics(diagnosticFile, false);
  const fixedMcp = await compareMcp(fixed);
  return { provider, generations: [initial.generation, failed.generation, fixed.generation], mcp: [initialMcp, failedMcp, fixedMcp] };
}

async function configure(command, enabled) {
  const config = vscode.workspace.getConfiguration("corvint", rootUri);
  const target = vscode.ConfigurationTarget.WorkspaceFolder;
  await config.update("liveTests.enabled", false, target);
  await config.update("liveTests.command", command, target);
  await config.update("liveTests.watchPaths", ["."], target);
  await config.update("liveTests.toolchainPaths", [path.dirname(process.env.CORVINT_NODE_EXECUTABLE), path.dirname(process.env.CORVINT_GO_EXECUTABLE)], target);
  await config.update("liveTests.retainEvidence", enabled, target);
  if (enabled) await config.update("liveTests.enabled", true, target);
}

function unitCommand() {
  return [process.env.CORVINT_JS_PROVIDER, "unit", "--dir", root, "--config", path.join(root, "vitest.config.js"), "--package-json", "package.json", "--lockfile", "package-lock.json", "--runner-version", "5.0.0", "--test-file", "unit.test.js"];
}

function e2eCommand() {
  return [process.env.CORVINT_JS_PROVIDER, "e2e", "--dir", root, "--config", path.join(root, "playwright.config.js"), "--package-json", "package.json", "--lockfile", "package-lock.json", "--runner-version", "1.63.0", "--test-file", "e2e.spec.js", "--app-build-dir", "app", "--server-ready-url", "http://127.0.0.1:4173/", "--server-arg", process.env.CORVINT_NODE_EXECUTABLE, "--server-arg", "app/server.js", "--test-arg", path.join(root, "node_modules/.bin/playwright"), "--test-arg", "test", "--test-arg", "--config=playwright.config.js", "--test-arg", "--reporter=json"];
}

function goCommand() {
  return [process.env.CORVINT_GO_PROVIDER, "session", "--experimental", "--trusted-local", "--foreground", "--authority-bundle", process.env.CORVINT_GO_AUTHORITY_BUNDLE, "--interval", "50ms", "--debounce", "100ms", "--scope", "example.test/corvint-interactive-alpha/go", "--watch", path.join(root, "go/calc.go"), "--watch", path.join(root, "go/calc_test.go")];
}

async function saveAndWait(api, relative, before, after, generation, label) {
  const document = await editDocument(relative, before, after);
  return completedAfter(api, generation, label);
}

async function editDocument(relative, before, after) {
  const document = await vscode.workspace.openTextDocument(path.join(root, relative));
  await vscode.window.showTextDocument(document);
  await replaceAndSave(document, before, after);
  return document;
}

async function replaceAndSave(document, before, after) {
  const current = document.getText();
  assert.ok(current.includes(before), `saved source is missing ${JSON.stringify(before)}`);
  const changed = current.replace(before, after);
  const edit = new vscode.WorkspaceEdit();
  edit.replace(document.uri, new vscode.Range(document.positionAt(0), document.positionAt(current.length)), changed);
  assert.equal(await vscode.workspace.applyEdit(edit), true, "workspace edit failed");
  assert.equal(await document.save(), true, "document save failed");
}

function completedAfter(api, generation, label) {
  return waitSnapshot(api, (snapshot) => snapshot.phase === "completed" && (generation === undefined || snapshot.generation > generation), label);
}

async function waitSnapshot(api, predicate, label) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const snapshot = api.liveTestSnapshot(root);
    if (snapshot !== undefined && predicate(snapshot)) return snapshot;
    await delay(25);
  }
  throw new Error(`timed out waiting for ${label}: ${JSON.stringify(api.liveTestSnapshot(root))}`);
}

function assertSnapshotState(snapshot, expected) {
  assert.equal(snapshot.profile, "corvint-vscode-live-tests/0");
  assert.ok(snapshot.items.some((item) => item.state === expected), `snapshot has no ${expected} item: ${JSON.stringify(snapshot)}`);
}

async function assertDiagnostics(relative, expected) {
  const uri = vscode.Uri.file(path.join(root, relative));
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    const found = vscode.languages.getDiagnostics(uri).some(isCorvintDiagnostic);
    if (found === expected) return;
    await delay(25);
  }
  throw new Error(`Corvint diagnostic state for ${relative} did not become ${expected}`);
}

function isCorvintDiagnostic(diagnostic) {
  return diagnostic.source === "corvint-live-tests";
}

async function compareMcp(snapshot) {
  assert.match(snapshot.documentDigest ?? "", /^sha256:[0-9a-f]{64}$/);
  assert.equal(snapshot.omitted, 0, "editor snapshot omitted test items");
  assertUnique(snapshot.items.map((item) => item.id), "editor snapshot item IDs");
  await waitForRetainedDocument(snapshot.documentDigest);
  const response = await callMcpDiscover(process.env.CORVINT_TEST_VALIDITY_MCP, root);
  assert.equal(response.jsonrpc, "2.0");
  assert.equal(response.id, 1);
  assert.equal(response.error, undefined, JSON.stringify(response));
  const result = response.result;
  assert.equal(result.isError, false, JSON.stringify(result));
  const structured = result.structuredContent;
  assert.equal(structured.schema, "corvint-mcp-test-validity-result/0");
  const document = structured.document;
  const evidence = document.discovery?.evidence;
  assert.ok(evidence, "MCP discover did not select retained evidence");
  const retainedBytes = await readFile(path.join(root, evidence), "utf8");
  const retained = JSON.parse(retainedBytes);
  const valueSpan = retainedBytes.trim();
  const digest = `sha256:${createHash("sha256").update(valueSpan).digest("hex")}`;
  assert.equal(digest, snapshot.documentDigest, "editor terminal document differs from MCP-selected retained evidence");
  if (snapshot.provider === "go-session") assert.equal(retained.identity, snapshot.inputIdentity, "editor Go input identity differs from retained evidence");
  assert.equal(document.tests.length > 0, true, "MCP document has no test projections");
  const expectedKind = snapshot.provider === "go-session" ? "go-session" : snapshot.provider === "js-unit" ? "unit" : "e2e";
  assert.equal(document.kind, expectedKind, "editor/MCP provider kind differs");
  const projected = document.tests.map((test) => [document.kind === "go-session" ? `${test.package}#${test.name}` : test.name, classifyProjection(test.projection)]);
  const runState = classifyProjection(document.run);
  const supplemental = document.kind === "go-session"
    ? retained.scope.map((scope) => [scope, runState])
    : runState === "errored" ? [[`js-run:${document.kind}`, "errored"]] : [];
  const expected = [...projected, ...supplemental];
  assertUnique(expected.map(([name]) => name), "MCP test and run item IDs");
  const editor = snapshot.items.map((item) => [item.id, item.state]);
  assert.deepEqual(editor.sort(compareEntries), expected.sort(compareEntries), "complete editor/MCP test and run state sets differ");
  return { evidence, digest, source: document.source, kind: document.kind, runExecution: document.run.execution.state, runFreshness: document.run.freshness.state, freshness: document.discovery?.freshness?.state ?? null };
}

async function waitForRetainedDocument(digest) {
  const directory = path.join(root, ".corvint/test-evidence");
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    let entries;
    try { entries = await readdir(directory, { withFileTypes: true }); } catch (error) {
      if (error.code !== "ENOENT") throw error;
      entries = [];
    }
    assert.ok(entries.length <= 256, "retained evidence exceeds discovery bound");
    for (const entry of entries) {
      if (!entry.isFile() || !/^corvint-(?:js|go)-test-provider-[0-9]+-[0-9a-f]+\.json$/.test(entry.name)) continue;
      const bytes = await readFile(path.join(directory, entry.name), "utf8");
      if (`sha256:${createHash("sha256").update(bytes.trim()).digest("hex")}` === digest) return;
    }
    await delay(25);
  }
  throw new Error(`editor document ${digest} was not retained before timeout`);
}

function assertUnique(values, label) {
  assert.equal(new Set(values).size, values.length, `${label} contain duplicates`);
}

function compareEntries(left, right) {
  return left[0].localeCompare(right[0]) || left[1].localeCompare(right[1]);
}

function classifyProjection(projection) {
  if (projection.freshness.state === "STALE") return "errored";
  const execution = projection.execution.state;
  if (execution === "PASSED") return "passed";
  if (execution === "FAILED") return "failed";
  if (execution === "SKIPPED") return "skipped";
  return "errored";
}

const delay = (milliseconds) => new Promise((resolvePromise) => setTimeout(resolvePromise, milliseconds));
