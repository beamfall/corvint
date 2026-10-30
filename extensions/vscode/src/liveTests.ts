/**
 * VS Code wiring for roadmap IPR-09 (editor half): a TestController fed by
 * an explicitly enabled foreground live test-provider process. Parsing and
 * mapping logic lives in the pure liveTestProtocol.ts adapter; this module
 * only spawns the process, reads bounded JSON records, and projects the
 * decoded records onto the vscode.tests / vscode.languages APIs.
 *
 * Disabled by default: nothing spawns until `corvint.liveTests.enabled` is
 * explicitly set true for a workspace resource, matching the rest of this
 * extension's explicit-execution-only posture.
 */

import * as path from "node:path";
import { realpath } from "node:fs/promises";
import { createHash } from "node:crypto";
import * as vscode from "vscode";
import { minimumEnvironment } from "./executable.js";
import { configurationValue } from "./configuration.js";
import { spawnProcessGroup, type ProcessGroupHandle } from "./process.js";
import {
  classifyGoSessionEvent,
  classifyGoTest,
  classifyProjection,
  describeOmittedGoTests,
  describeLiveTestUnavailability,
  DiagnosticsLedger,
  extractJsonRecords,
  parseProviderRecord,
  ProtocolError,
  beginsNewRun,
  shouldApplyRecord,
  withEvidenceRetention,
  type GoSessionEvent,
  type JsProviderOutput,
  type JsTestResult,
  type LiveTestUnavailableCause,
  type ProviderRecord,
  type RunOutcome,
} from "./liveTestProtocol.js";

const DIAGNOSTIC_SOURCE = "corvint-live-tests";
let controllerSuffix = 0;

export interface LiveTestSnapshot {
  readonly profile: "corvint-vscode-live-tests/0";
  readonly phase: "disabled" | "pending" | "running" | "completed" | "unavailable";
  readonly generation: number;
  readonly provider: "unknown" | "go-session" | "js-unit" | "js-e2e";
  readonly inputIdentity: string | null;
  /** SHA-256 of the exact complete provider JSON value, excluding surrounding JSON whitespace. */
  readonly documentDigest: string | null;
  readonly items: readonly { readonly id: string; readonly state: "started" | "passed" | "failed" | "skipped" | "errored" }[];
  readonly omitted: number;
}

export class LiveTestController implements vscode.Disposable {
  private readonly controller = vscode.tests.createTestController(`corvint.liveTests.${controllerSuffix++}`, "Corvint Live Tests");
  private readonly diagnostics = vscode.languages.createDiagnosticCollection("corvint-live-tests");
  private readonly ledger = new DiagnosticsLedger();
  private readonly items = new Map<string, vscode.TestItem>();
  private handle: ProcessGroupHandle | undefined;
  private run: vscode.TestRun | undefined;
  private activeInputIdentity: string | undefined;
  private activeScopeItems: readonly vscode.TestItem[] = [];
  private buffer = "";
  private discardingOverflow = false;
  private disposed = false;
  private transition: Promise<void> = Promise.resolve();
  private generation = 0;
  private debounce: NodeJS.Timeout | undefined;
  private configured: { argv: readonly string[]; watchPaths: readonly string[]; env: Readonly<Record<string, string>> } | undefined;
  private cleanupFailure: string | undefined;
  private jsRecord: JsProviderOutput | undefined;
  private protocolFault = false;
  private sawGo = false;
  private jsDigest: string | undefined;
  private phase: LiveTestSnapshot["phase"] = "disabled";
  private provider: LiveTestSnapshot["provider"] = "unknown";
  private documentDigest: string | null = null;
  private snapshotIdentity: string | null = null;
  private readonly observed = new Map<string, LiveTestSnapshot["items"][number]>();
  private omitted = 0;
  private shutdownPromise: Promise<void> | undefined;

  constructor(private readonly root: string) {}

  /** A bounded copy of values submitted to VS Code, not independent UI or evidence verification. */
  snapshot(): LiveTestSnapshot {
    const items = [...this.observed.values()];
    return structuredClone({ profile: "corvint-vscode-live-tests/0", phase: this.phase, generation: this.generation,
      provider: this.provider, inputIdentity: this.snapshotIdentity, documentDigest: this.documentDigest,
      items: items.slice(0, 2048), omitted: this.omitted + Math.max(0, items.length - 2048) });
  }

  private outcome(run: vscode.TestRun, item: vscode.TestItem, outcome: RunOutcome): void {
    applyOutcome(run, item, outcome);
    this.observed.set(item.id, { id: item.id, state: outcome.kind === "stale" ? "errored" : outcome.kind });
  }

  /** Starts or stops the provider to match the latest configuration for this root. */
  applyConfiguration(): Promise<void> {
    if (this.disposed) return this.transition;
    const previous = this.supersede();
    const generation = this.generation;
    this.configured = undefined;
    // Trust precedes reading resource configuration and filesystem paths.
    if (!vscode.workspace.isTrusted) return this.enqueue(() => this.cleanup(previous));
    const resource = vscode.Uri.file(this.root);
    const enabled = configurationValue<boolean>("liveTests.enabled", false, resource);
    if (!enabled) return this.enqueue(() => this.cleanup(previous));
    if (process.platform === "win32") return this.enqueue(async () => {
      await this.cleanup(previous);
      if (this.current(generation)) this.reportUnavailable({ kind: "process-error", detail: "live providers are unsupported on Windows in this alpha" });
    });
    const argv = configurationValue<readonly string[]>("liveTests.command", [], resource);
    const retainEvidence = configurationValue<boolean>("liveTests.retainEvidence", false, resource);
    const watchPaths = configurationValue<readonly string[]>("liveTests.watchPaths", ["."], resource);
    const toolchainPaths = configurationValue<unknown>("liveTests.toolchainPaths", [], resource);
    let cause: LiveTestUnavailableCause | undefined;
    if (!Array.isArray(argv) || argv.length === 0 || !argv.every((entry) => typeof entry === "string" && entry.length > 0) || typeof argv[0] !== "string" || !path.isAbsolute(argv[0]) || path.normalize(argv[0]) !== argv[0]) {
      cause = { kind: "invalid-command" };
    } else {
      const retention = withEvidenceRetention(argv, retainEvidence === true);
      if (retention.kind === "unavailable") cause = retention.cause;
      else if (!validWatchPaths(watchPaths)) cause = { kind: "process-error", detail: "invalid corvint.liveTests.watchPaths: expected at most 64 normalized repository-relative paths" };
      else if (!validToolchainPaths(toolchainPaths)) cause = { kind: "process-error", detail: "invalid corvint.liveTests.toolchainPaths: expected at most 16 absolute normalized directories within 8192 bytes" };
      else {
        const env = { ...minimumEnvironment() };
        const directories = [...new Set(toolchainPaths)];
        if (directories.length > 0) env.PATH = [...directories, ...(env.PATH === undefined ? [] : [env.PATH])].join(path.delimiter);
        this.configured = { argv: retention.argv, watchPaths, env };
      }
    }
    const configured = this.configured;
    return this.enqueue(async () => {
      await this.cleanup(previous);
      if (!this.current(generation)) return;
      if (cause !== undefined) this.reportUnavailable(cause);
      else if (configured !== undefined) this.start(configured.argv, generation, configured.env);
    });
  }

  /** Called only for a saved document's containing workspace folder. */
  onSavedDocument(uri: vscode.Uri): Promise<void> {
    const configured = this.configured;
    if (this.disposed || !vscode.workspace.isTrusted || uri.scheme !== "file" || configured === undefined || !isAutomaticJs(configured.argv)) return Promise.resolve();
    const relative = path.relative(this.root, uri.fsPath);
    if (!contained(this.root, uri.fsPath) || excluded(relative) || !configured.watchPaths.some((entry) => contained(path.resolve(this.root, entry), uri.fsPath))) return Promise.resolve();
    // Invalidate synchronously; filesystem validation and group cleanup must not admit late green output.
    const previous = this.supersede();
    const generation = this.generation;
    this.pending();
    return this.enqueue(async () => {
      await this.cleanup(previous);
      if (!this.current(generation)) return;
      if (!await canonicalWatchMatch(this.root, uri.fsPath, configured.watchPaths)) {
        if (this.current(generation)) this.reportUnavailable({ kind: "process-error", detail: "saved path or watch scope is unavailable or escapes the workspace" });
        return;
      }
      if (!this.current(generation)) return;
      this.debounce = setTimeout(() => {
        this.debounce = undefined;
        void this.enqueue(async () => {
          if (this.current(generation)) this.start(configured.argv, generation, configured.env);
        }).catch(() => undefined);
      }, 250);
    });
  }

  private current(generation: number): boolean {
    return !this.disposed && vscode.workspace.isTrusted && this.generation === generation;
  }

  private enqueue(action: () => Promise<void>): Promise<void> {
    const next = this.transition.then(action);
    this.transition = next.catch((error: unknown) => {
      this.cleanupFailure = String(error);
      if (!this.disposed) this.reportUnavailable({ kind: "process-error", detail: this.cleanupFailure });
    });
    return next;
  }

  private async cleanup(handle: ProcessGroupHandle | undefined): Promise<void> {
    await handle?.killGroup();
    if (this.cleanupFailure !== undefined) throw new Error(this.cleanupFailure);
  }

  private pending(): void {
    const run = this.beginSession(runName("pending"));
    this.outcome(run, this.itemFor("corvint-live-tests:pending"), { kind: "errored", message: "UNKNOWN: inputs superseded prior results; waiting for the configured test suite", notes: [] });
    this.endRun();
    this.phase = "pending";
  }

  private start(argv: readonly string[], generation: number, env: Readonly<Record<string, string>>): void {
    if (this.cleanupFailure !== undefined) throw new Error(this.cleanupFailure);
    const [executable, ...args] = argv;
    if (executable === undefined) return;
    let handle: ProcessGroupHandle;
    try {
      handle = spawnProcessGroup(executable, args, { cwd: this.root, env });
    } catch (error) {
      this.reportUnavailable({ kind: "spawn-failed", detail: String(error) });
      return;
    }
    this.handle = handle;
    this.buffer = "";
    this.discardingOverflow = false;
    this.jsRecord = undefined;
    this.jsDigest = undefined;
    this.protocolFault = false;
    this.sawGo = false;
    if (isAutomaticJs(argv)) this.pending();
    handle.child.stdout.setEncoding("utf8");
    handle.child.stdout.on("data", (chunk: string) => {
      if (this.handle === handle && this.current(generation)) this.onData(chunk);
    });
    // A descendant holding stdout can otherwise prevent close forever after its leader exits.
    handle.child.once("exit", () => {
      // Record failure even if a surviving descendant keeps stdout open and close never arrives.
      void this.enqueue(() => this.cleanup(handle)).catch(() => undefined);
    });
    let processError: string | undefined;
    handle.child.once("error", (error) => { processError = String(error); });
    handle.child.once("close", (code, signal) => {
      if (this.handle !== handle || !this.current(generation)) return;
      const record = this.jsRecord;
      const digest = this.jsDigest;
      const qualified = record !== undefined && !this.protocolFault && !this.sawGo && !this.discardingOverflow && !/[^ \t\r\n]/.test(this.buffer);
      void this.enqueue(async () => {
        await this.cleanup(handle);
        if (this.handle !== handle || !this.current(generation)) return;
        this.handle = undefined;
        if (processError !== undefined) this.reportUnavailable({ kind: "process-error", detail: processError });
        else if (code === 0 && signal === null && qualified && record !== undefined) this.onJsProviderOutput(record, digest ?? null);
        else this.reportUnavailable({ kind: "process-exited", code, signal });
      }).catch(() => undefined);
    });
  }

  /** Surfaces a cause-naming explanation (VSC-V0-066) instead of a silent return, and clears any stale session state. */
  private reportUnavailable(cause: LiveTestUnavailableCause): void {
    const run = this.beginSession(runName("unavailable"));
    this.outcome(run, this.itemFor("corvint-live-tests:unavailable"), { kind: "errored", message: `Corvint live tests unavailable: ${describeLiveTestUnavailability(cause)}`, notes: [] });
    this.endRun();
    this.phase = "unavailable";
    this.documentDigest = null;
    this.snapshotIdentity = null;
  }

  /** Detaches stale callbacks before any asynchronous cleanup. */
  private supersede(): ProcessGroupHandle | undefined {
    this.generation += 1;
    this.phase = "disabled";
    this.provider = "unknown";
    this.documentDigest = null;
    this.snapshotIdentity = null;
    this.omitted = 0;
    clearTimeout(this.debounce);
    this.debounce = undefined;
    this.endRun();
    this.clearItems();
    this.ledger.reset();
    this.diagnostics.clear();
    this.activeInputIdentity = undefined;
    this.activeScopeItems = [];
    const previous = this.handle;
    this.handle = undefined;
    return previous;
  }

  /** Stops the provider and any scheduled replacement, awaiting process-group absence. */
  stop(): Promise<void> {
    this.configured = undefined;
    const previous = this.supersede();
    return this.enqueue(() => this.cleanup(previous));
  }

  private onData(chunk: string): void {
    this.buffer += chunk;
    // An unterminated record past the bound is dropped by the scanner, which
    // resyncs after the next LF; records completed before it in the same
    // buffer are still delivered.
    const extracted = extractJsonRecords(this.buffer, this.discardingOverflow);
    this.buffer = extracted.rest;
    this.discardingOverflow = extracted.overflow;
    this.protocolFault ||= extracted.overflow;
    for (const text of extracted.records) {
      this.onRecordText(text);
    }
  }

  private onRecordText(text: string): void {
    let record: ProviderRecord;
    try {
      record = parseProviderRecord(text);
    } catch (error) {
      if (error instanceof ProtocolError) {
        this.protocolFault = true;
        return;
      }
      throw error;
    }
    const digest = `sha256:${createHash("sha256").update(text, "utf8").digest("hex")}`;
    if (record.kind === "go-session") {
      this.sawGo = true;
      if (this.jsRecord !== undefined) this.protocolFault = true;
      else this.onGoSessionEvent(record, digest);
      return;
    }
    if (this.jsRecord !== undefined || this.sawGo) this.protocolFault = true;
    else { this.jsRecord = record; this.jsDigest = digest; }
  }

  private onGoSessionEvent(event: GoSessionEvent, digest: string): void {
    this.provider = "go-session";
    if (event.state === "running") {
      if (beginsNewRun(this.activeInputIdentity, this.run !== undefined, event.identity)) {
        this.beginSession(runName(event.identity));
        this.activeInputIdentity = event.identity;
      }
      this.activeScopeItems = event.scope.map((scope) => this.itemFor(scope));
      for (const item of this.activeScopeItems) {
        this.run?.started(item);
        this.observed.set(item.id, { id: item.id, state: "started" });
      }
      this.phase = "running";
      this.documentDigest = null;
      this.snapshotIdentity = event.identity;
      return;
    }
    if (event.state === "idle") {
      // The foreground process is enabled, but has not announced an execution yet.
      this.endRun();
      this.phase = "pending";
      this.documentDigest = null;
      this.snapshotIdentity = null;
      return;
    }
    // Terminal: passed/failed/stale/infrastructure/cancelled close every
    // item the matching "running" event announced.
    if (!shouldApplyRecord(this.activeInputIdentity, event.identity) || this.run === undefined) {
      return;
    }
    const run = this.run;
    const outcome = classifyGoSessionEvent(event);
    for (const item of this.activeScopeItems) {
      this.outcome(run, item, { ...outcome, notes: [] });
    }
    for (const test of event.tests) {
      const item = test.file !== undefined && test.line !== undefined ? this.itemForFile(test.testId, test.file, test.line) : this.itemFor(test.testId);
      const testOutcome = classifyGoTest(test);
      if (test.file !== undefined) {
        this.applyDiagnostics(test.file, test.testId, test.line ?? 1, testOutcome.kind === "failed", testOutcome.message);
      }
      this.outcome(run, item, testOutcome);
    }
    if (event.testsOmitted > 0) {
      this.outcome(run, this.itemFor("corvint-live-tests:go-tests-omitted"), { kind: "errored", message: describeOmittedGoTests(event.testsOmitted), notes: [] });
    }
    this.endRun();
    this.phase = "completed";
    this.documentDigest = digest;
    this.snapshotIdentity = event.identity;
    this.omitted = event.testsOmitted;
  }

  private onJsProviderOutput(record: JsProviderOutput, digest: string | null): void {
    // A JS provider invocation runs to completion once and prints exactly
    // one receipt: that receipt is unambiguously a fresh session, so it
    // always starts a new run rather than joining a Go session's run.
    const run = this.beginSession(runName(`js-${record.runKind}`));
    for (const test of record.tests) {
      this.applyJsTestResult(run, test);
    }
    const runOutcome = classifyProjection(record.runProjection);
    if (runOutcome.kind === "errored" || runOutcome.kind === "stale") {
      const runItem = this.itemFor(`js-run:${record.runKind}`);
      const message = [runOutcome.message, ...runOutcome.notes].filter((text) => text.length > 0).join("\n");
      this.outcome(run, runItem, { kind: "errored", message, notes: [] });
    }
    this.endRun();
    this.phase = "completed";
    this.provider = record.runKind === "unit" ? "js-unit" : "js-e2e";
    this.documentDigest = digest;
    this.snapshotIdentity = null;
  }

  private applyJsTestResult(run: vscode.TestRun, test: JsTestResult): void {
    const item = test.file !== undefined && test.line !== undefined ? this.itemForFile(test.testId, test.file, test.line) : this.itemFor(test.testId);
    const outcome = classifyProjection(test.projection);
    if (test.file !== undefined) {
      this.applyDiagnostics(test.file, test.testId, test.line ?? 1, outcome.kind === "failed", outcome.message);
    }
    this.outcome(run, item, outcome);
  }

  /** Ends the current run, drops every tracked item/diagnostic, and starts a fresh run: called whenever a newer session identity begins. */
  private beginSession(name: string): vscode.TestRun {
    this.endRun();
    this.clearItems();
    this.ledger.reset();
    this.diagnostics.clear();
    const run = this.controller.createTestRun(new vscode.TestRunRequest(), name, false);
    this.run = run;
    return run;
  }

  private clearItems(): void {
    for (const item of this.items.values()) {
      this.controller.items.delete(item.id);
    }
    this.items.clear();
    this.observed.clear();
  }

  private itemFor(testId: string, uri?: vscode.Uri): vscode.TestItem {
    let item = this.items.get(testId);
    if (item === undefined) {
      // vscode.TestItem.uri is read-only after creation; a provider report
      // of the same testId under a different file would need a new item
      // (testId is expected to be stable per test, per the stream contract).
      item = this.controller.createTestItem(testId, testId, uri);
      this.items.set(testId, item);
      this.controller.items.add(item);
    }
    return item;
  }

  private itemForFile(testId: string, file: string, line: number): vscode.TestItem {
    const uri = vscode.Uri.file(path.isAbsolute(file) ? file : path.join(this.root, file));
    const item = this.itemFor(testId, uri);
    const zeroBasedLine = Math.max(line - 1, 0);
    item.range = new vscode.Range(zeroBasedLine, 0, zeroBasedLine, 0);
    return item;
  }

  private applyDiagnostics(file: string, testId: string, line: number, failing: boolean, message: string): void {
    const uri = vscode.Uri.file(path.isAbsolute(file) ? file : path.join(this.root, file));
    const snapshot = this.ledger.apply(file, testId, failing, line, message);
    const diagnostics = snapshot.entries.map((entry) => {
      const entryLine = Math.max(entry.line - 1, 0);
      const diagnostic = new vscode.Diagnostic(new vscode.Range(entryLine, 0, entryLine, 200), entry.message, vscode.DiagnosticSeverity.Error);
      diagnostic.source = DIAGNOSTIC_SOURCE;
      diagnostic.code = entry.testId;
      return diagnostic;
    });
    this.diagnostics.set(uri, diagnostics);
  }

  private endRun(): void {
    this.run?.end();
    this.run = undefined;
  }

  /** Disposes the controller and settles once the provider's process group has been terminated (VSC-V0-059). */
  shutdown(): Promise<void> {
    this.shutdownPromise ??= this.performShutdown();
    return this.shutdownPromise;
  }

  private async performShutdown(): Promise<void> {
    this.disposed = true;
    try {
      await this.stop();
    } finally {
      this.diagnostics.dispose();
      this.controller.dispose();
    }
  }

  dispose(): void {
    void this.shutdown().catch(() => undefined);
  }
}

/** Applies one projection-derived outcome (VSC-V0-060..062) to its TestItem. */
function applyOutcome(run: vscode.TestRun, item: vscode.TestItem, outcome: RunOutcome): void {
  const message = [outcome.message, ...outcome.notes].filter((text) => text.length > 0).join("\n");
  switch (outcome.kind) {
    case "passed":
      run.passed(item);
      break;
    case "failed":
      run.failed(item, new vscode.TestMessage(message));
      break;
    case "skipped":
      run.skipped(item);
      break;
    case "errored":
    case "stale":
      run.errored(item, new vscode.TestMessage(message));
      break;
  }
  if (outcome.notes.length > 0 && outcome.kind !== "failed" && outcome.kind !== "errored" && outcome.kind !== "stale") {
    run.appendOutput(`${outcome.notes.join(" · ")}\r\n`, undefined, item);
  }
}

function runName(inputIdentity: string): string {
  return `Corvint live tests · ${inputIdentity}`;
}

function isAutomaticJs(argv: readonly string[]): boolean {
  return ["corvint-js-test-provider"].includes(path.basename(argv[0] ?? "")) &&
    (argv[1] === "unit" || argv[1] === "e2e");
}

function contained(root: string, file: string): boolean {
  const relative = path.relative(root, file);
  return relative === "" || (!path.isAbsolute(relative) && relative !== ".." && !relative.startsWith(`..${path.sep}`));
}

function excluded(relative: string): boolean {
  return relative.split(path.sep).some((segment) => [".git", ".corvint", "node_modules", "dist", "build", "out", "coverage"].includes(segment));
}

function validWatchPaths(value: unknown): value is readonly string[] {
  return Array.isArray(value) && value.length > 0 && value.length <= 64 && value.every((entry: unknown) =>
    typeof entry === "string" && entry.length > 0 && entry.length <= 512 && !/[\\\x00-\x1f\x7f]/.test(entry) &&
    !path.posix.isAbsolute(entry) && path.posix.normalize(entry) === entry && entry !== ".." && !entry.startsWith("../"));
}

async function canonicalWatchMatch(root: string, file: string, watchPaths: readonly string[]): Promise<boolean> {
  try {
    const canonicalRoot = await realpath(root);
    const canonicalFile = await realpath(file);
    if (!contained(canonicalRoot, canonicalFile) || excluded(path.relative(canonicalRoot, canonicalFile))) return false;
    let matches = false;
    for (const entry of watchPaths) {
      const watched = await realpath(path.resolve(root, entry));
      if (!contained(canonicalRoot, watched)) return false;
      if (contained(watched, canonicalFile)) matches = true;
    }
    return matches;
  } catch {
    return false;
  }
}

function validToolchainPaths(value: unknown): value is readonly string[] {
  return Array.isArray(value) && value.length <= 16 && value.every((entry: unknown) =>
    typeof entry === "string" && entry.length > 0 && entry.length <= 4096 && path.isAbsolute(entry) &&
    path.normalize(entry) === entry && !/[\x00-\x1f\x7f]/.test(entry) && !entry.includes(path.delimiter)) &&
    Buffer.byteLength(value.join(path.delimiter), "utf8") <= 8192;
}
