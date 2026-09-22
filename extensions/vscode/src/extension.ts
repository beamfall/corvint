import { realpath } from "node:fs/promises";
import * as path from "node:path";
import * as vscode from "vscode";
import {
  configuredCandidate,
  configuredMcpCandidate,
  discoverPathCandidates,
  fixedArguments,
  minimumEnvironment,
  pinExecutable,
  pinMcpExecutable,
  revalidatePin,
  revalidateMcpPin,
  type ExecutablePin,
  type McpExecutablePin,
} from "./executable.js";
import { runMcpOperation, McpFailure } from "./mcp.js";
import { AsyncTaskTracker } from "./lifecycle.js";
import {
  attachTransportAuthority,
  createTransportAbstentionSnapshot,
  decodeCorvintReceipt,
  decodeContextReceipt,
  display,
  isRepositoryRelativePath,
  type Operation,
  type TransportAuthority,
} from "./model.js";
import { decodeObservation, readStableObservation } from "./observations.js";
import { LiveTestController, type LiveTestSnapshot } from "./liveTests.js";
import { CorvintPresentation } from "./presentation.js";
import { ProcessFailure, runBoundedProcess } from "./process.js";
import { ObservationTestingBridge } from "./testing.js";
import { configurationValue } from "./configuration.js";

const STDERR_LIMIT = 65_536;
let runtime: CorvintRuntime | undefined;

type CorvintPin =
  | { readonly transport: "cli"; readonly executable: ExecutablePin }
  | { readonly transport: "mcp-stdio"; readonly executable: McpExecutablePin };

export function activate(context: vscode.ExtensionContext): { liveTestSnapshot(root: string): LiveTestSnapshot | undefined } {
  runtime = new CorvintRuntime(context);
  context.subscriptions.push(runtime);
  const active = runtime;
  return { liveTestSnapshot: (root) => active.liveTestSnapshot(root) };
}

export async function deactivate(): Promise<void> {
  await runtime?.shutdown();
  runtime = undefined;
}

class CorvintRuntime implements vscode.Disposable {
  private readonly presentation: CorvintPresentation;
  private readonly testing = new ObservationTestingBridge();
  private readonly liveTests = new Map<string, LiveTestController>();
  private readonly extensionVersion: string;
  private readonly tasks = new AsyncTaskTracker();
  private pin: CorvintPin | undefined;
  private lastOperation: { operation: Operation; root: string; value: string | readonly string[] } | undefined;
  private activeCancellation: vscode.CancellationTokenSource | undefined;
  private operationChain: Promise<void> = Promise.resolve();
  private operationGeneration = 0;
  private disposed = false;
  private shutdownPromise: Promise<void> | undefined;

  constructor(context: vscode.ExtensionContext) {
    this.extensionVersion = manifestVersion(context);
    const status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    status.command = "corvint.selectExecutable";
    this.presentation = new CorvintPresentation(status);
    context.subscriptions.push(
      this.testing,
      vscode.window.registerTreeDataProvider("corvint.evidence", this.presentation.evidence),
      vscode.window.registerTreeDataProvider("corvint.impact", this.presentation.impact),
      vscode.window.registerTreeDataProvider("corvint.why", this.presentation.why),
      vscode.window.onDidChangeVisibleTextEditors(() => this.presentation.applyDecorations()),
      vscode.workspace.onDidGrantWorkspaceTrust(() => this.onTrustGranted()),
      vscode.workspace.onDidChangeWorkspaceFolders(() => this.onRootsChanged()),
      vscode.workspace.onDidChangeConfiguration((event) => this.onConfigurationChanged(event)),
      vscode.workspace.onDidSaveTextDocument((document) => {
        if (this.disposed || !vscode.workspace.isTrusted || document.uri.scheme !== "file") return;
        const folder = vscode.workspace.getWorkspaceFolder(document.uri);
        const controller = folder === undefined ? undefined : this.liveTests.get(folder.uri.fsPath);
        if (controller !== undefined) this.tasks.track(controller.onSavedDocument(document.uri)).catch(() => undefined);
      }),
      vscode.commands.registerCommand("corvint.selectExecutable", () => this.tasks.track(this.selectExecutable())),
      vscode.commands.registerCommand("corvint.clearExecutable", () => this.clearExecutable()),
      vscode.commands.registerCommand("corvint.query", () => this.tasks.track(this.query())),
      vscode.commands.registerCommand("corvint.impact", () => this.tasks.track(this.impact())),
      vscode.commands.registerCommand("corvint.refresh", () => this.tasks.track(this.refresh())),
      vscode.commands.registerCommand("corvint.importTestObservation", () => this.tasks.track(this.importObservation())),
      vscode.commands.registerCommand("corvint.clearResults", () => this.clearResults()),
    );
    this.resetInitialState();
    this.syncLiveTests();
    this.tasks.track(this.applyLiveTestsConfiguration()).catch(() => undefined);
  }

  liveTestSnapshot(root: string): LiveTestSnapshot | undefined {
    if (this.disposed || !vscode.workspace.isTrusted || !vscode.workspace.workspaceFolders?.some((folder) => folder.uri.scheme === "file" && folder.uri.fsPath === root)) return undefined;
    return this.liveTests.get(root)?.snapshot();
  }

  private syncLiveTests(): void {
    const roots = new Set((vscode.workspace.workspaceFolders ?? []).filter((folder) => folder.uri.scheme === "file").map((folder) => folder.uri.fsPath));
    for (const [root, controller] of this.liveTests) {
      if (!roots.has(root)) {
        this.tasks.track(controller.shutdown()).catch(() => undefined);
        this.liveTests.delete(root);
      }
    }
    for (const root of roots) {
      if (!this.liveTests.has(root)) {
        this.liveTests.set(root, new LiveTestController(root));
      }
    }
  }

  private async applyLiveTestsConfiguration(): Promise<void> {
    if (this.disposed) {
      return;
    }
    await Promise.all([...this.liveTests.values()].map((controller) => controller.applyConfiguration()));
  }

  dispose(): void {
    void this.shutdown();
  }

  shutdown(): Promise<void> {
    this.shutdownPromise ??= this.performShutdown();
    return this.shutdownPromise;
  }

  private async performShutdown(): Promise<void> {
    this.disposed = true;
    this.activeCancellation?.cancel();
    this.operationGeneration += 1;
    try {
      await this.tasks.close();
      await this.operationChain.catch(() => undefined);
    } catch {
      // User-visible failure was already reported by the owning operation.
    }
    const liveTests = [...this.liveTests.values()];
    this.liveTests.clear();
    await Promise.all(liveTests.map((controller) => controller.shutdown()));
    this.pin = undefined;
    this.lastOperation = undefined;
    this.presentation.dispose();
  }

  private resetInitialState(): void {
    this.presentation.clear(vscode.workspace.isTrusted ? "No Corvint snapshot" : "Workspace Trust required");
    if (!vscode.workspace.isTrusted) {
      this.presentation.setStatus("Restricted", "Workspace Trust is required");
    } else if (!supportedExecutionPlatform()) {
      this.presentation.setStatus("Unsupported", "Corvint execution is limited to Darwin and Linux in V0");
    } else {
      this.presentation.setStatus("No CLI", "Select and pin a local Corvint executable");
    }
  }

  private onTrustGranted(): void {
    if (this.disposed) return;
    this.invalidateOperations();
    this.pin = undefined;
    this.lastOperation = undefined;
    this.presentation.clear("Trust granted; select a Corvint executable");
    this.presentation.setStatus(supportedExecutionPlatform() ? "No CLI" : "Unsupported");
    this.tasks.track(this.applyLiveTestsConfiguration()).catch(() => undefined);
  }

  private onRootsChanged(): void {
    if (this.disposed) return;
    this.invalidateOperations();
    this.pin = undefined;
    this.lastOperation = undefined;
    this.testing.clear();
    this.presentation.clear("Workspace roots changed");
    this.resetInitialState();
    this.syncLiveTests();
    this.tasks.track(this.applyLiveTestsConfiguration()).catch(() => undefined);
  }

  private onConfigurationChanged(event: vscode.ConfigurationChangeEvent): void {
    if (this.disposed) return;
    if (event.affectsConfiguration("corvint.liveTests")) {
      this.tasks.track(this.applyLiveTestsConfiguration()).catch(() => undefined);
    }
    if (!event.affectsConfiguration("corvint.transport") &&
      !event.affectsConfiguration("corvint.executablePath") &&
      !event.affectsConfiguration("corvint.mcpExecutablePath")) {
      return;
    }
    this.invalidateOperations();
    this.pin = undefined;
    this.lastOperation = undefined;
    this.testing.clear();
    this.presentation.clear("Corvint transport or executable configuration changed");
    this.resetInitialState();
  }

  private async selectExecutable(): Promise<void> {
    if (!this.guardTrusted() || !this.guardPlatform()) {
      return;
    }
    const root = await this.selectRoot();
    if (root === undefined || this.disposed || !vscode.workspace.isTrusted) {
      return;
    }
    this.invalidateOperations();
    const generation = this.operationGeneration;
    const cancellation = new vscode.CancellationTokenSource();
    this.activeCancellation = cancellation;
    let transport: "cli" | "mcp-stdio" | undefined;
    try {
      transport = transportFor(root);
    } catch (error) {
      cancellation.dispose();
      this.activeCancellation = undefined;
      this.fail("Incompatible", error);
      return;
    }
    if (transport === undefined) {
      cancellation.dispose();
      this.activeCancellation = undefined;
      this.fail("Incompatible", new Error("CLI_INCOMPATIBLE: corvint.transport must be cli or mcp-stdio"));
      return;
    }
    if (transport === "mcp-stdio") {
      try {
        const configured = configurationValue("mcpExecutablePath", "", vscode.Uri.file(root));
        if (configured.length === 0) {
          throw new Error("MCP_MISSING: corvint.mcpExecutablePath is required for mcp-stdio");
        }
        const candidate = configuredMcpCandidate(configured);
        this.presentation.setStatus("Busy", "Validating Corvint MCP executable identity");
        const pin = await pinMcpExecutable(candidate.path, candidate.cliKind, root, vscode.env.remoteName ?? "local");
        if (!this.operationCurrent(generation, cancellation)) {
          return;
        }
        this.pin = { transport, executable: pin };
        this.presentation.setStatus("Ready", `${pin.cliKind} pinned; protocol compatibility is checked per operation`);
      } catch (error) {
        if (this.operationCurrent(generation, cancellation)) {
          this.fail("Incompatible", mcpQualificationError(error));
        }
      } finally {
        if (this.activeCancellation === cancellation) {
          this.activeCancellation = undefined;
        }
        cancellation.dispose();
      }
      return;
    }
    try {
      const configured = configurationValue("executablePath", "", vscode.Uri.file(root));
      const candidates = configured.length > 0
        ? [{ ...configuredCandidate(configured), source: "configuration" as const }]
        : (await discoverPathCandidates(process.env.PATH)).map((candidate) => ({ ...candidate, source: "path" as const }));
      if (!this.operationCurrent(generation, cancellation)) {
        return;
      }
      if (candidates.length === 0) {
        this.fail("No CLI", new Error("CLI_MISSING: no Corvint executable candidate was found"));
        return;
      }
      this.presentation.setStatus("Busy", "Validating Corvint executable identity");
      let lastError: unknown;
      for (const candidate of candidates) {
        try {
          const pin = await pinExecutable(candidate.path, candidate.cliKind, root, vscode.env.remoteName ?? "local", candidate.source, cancellation.token);
          if (!this.operationCurrent(generation, cancellation)) {
            return;
          }
          this.pin = { transport, executable: pin };
          this.presentation.setStatus("Ready", `${pin.cliKind} ${pin.version}; substitution detection only`);
          return;
        } catch (error) {
          lastError = error;
          if (candidate.source === "configuration") {
            break;
          }
        }
      }
      if (this.operationCurrent(generation, cancellation)) {
        this.fail("Incompatible", lastError ?? new Error("CLI_INCOMPATIBLE"));
      }
    } catch (error) {
      if (this.operationCurrent(generation, cancellation)) {
        this.fail("Incompatible", error);
      }
    } finally {
      if (this.activeCancellation === cancellation) {
        this.activeCancellation = undefined;
      }
      cancellation.dispose();
    }
  }

  private clearExecutable(): void {
    if (!this.guardTrusted()) {
      return;
    }
    this.invalidateOperations();
    this.pin = undefined;
    this.lastOperation = undefined;
    this.presentation.clear("Pinned executable cleared");
    this.presentation.setStatus(supportedExecutionPlatform() ? "No CLI" : "Unsupported");
  }

  private async query(): Promise<void> {
    if (!this.guardTrusted() || !this.guardPlatform()) {
      return;
    }
    const task = await vscode.window.showInputBox({
      title: "Corvint query",
      prompt: "Describe the evidence you need. The task is sent only to the local pinned Corvint process and is not stored by this extension.",
      ignoreFocusOut: true,
      validateInput: validateTask,
    });
    if (task === undefined || this.disposed || !vscode.workspace.isTrusted) {
      return;
    }
    const root = await this.selectRoot(vscode.window.activeTextEditor?.document.uri);
    if (root !== undefined && !this.disposed && vscode.workspace.isTrusted) {
      await this.startOperation("query", root, task);
    }
  }

  private async impact(): Promise<void> {
    if (!this.guardTrusted() || !this.guardPlatform()) {
      return;
    }
    const uri = vscode.window.activeTextEditor?.document.uri;
    const root = await this.selectRoot(uri);
    if (this.disposed || !vscode.workspace.isTrusted) {
      return;
    }
    if (root === undefined || uri === undefined || uri.scheme !== "file") {
      this.fail("Degraded", new Error("PATH_REJECTED: an active file in the selected workspace is required"));
      return;
    }
    let resolvedFile: string;
    try {
      resolvedFile = await realpath(path.resolve(uri.fsPath));
    } catch {
      if (!this.disposed) this.fail("Degraded", new Error("PATH_REJECTED: active file could not be resolved"));
      return;
    }
    if (this.disposed || !vscode.workspace.isTrusted) {
      return;
    }
    const relationToRoot = path.relative(root, resolvedFile);
    if (relationToRoot.startsWith("..") || path.isAbsolute(relationToRoot)) {
      this.fail("Degraded", new Error("PATH_REJECTED: active file real path escapes the selected workspace"));
      return;
    }
    const relative = relationToRoot.split(path.sep).join("/");
    if (!isRepositoryRelativePath(relative)) {
      this.fail("Degraded", new Error("PATH_REJECTED: active file is outside the selected workspace"));
      return;
    }
    await this.startOperation("impact", root, [relative]);
  }

  private async refresh(): Promise<void> {
    if (!this.guardTrusted() || !this.guardPlatform()) {
      return;
    }
    const last = this.lastOperation;
    if (last === undefined) {
      this.fail("Degraded", new Error("CLI_FAILED: no explicit query or impact is available to refresh"));
      return;
    }
    await this.startOperation(last.operation, last.root, last.value);
  }

  private async startOperation(operation: Operation, root: string, value: string | readonly string[]): Promise<void> {
    if (this.disposed || !vscode.workspace.isTrusted) {
      return;
    }
    const generation = ++this.operationGeneration;
    this.activeCancellation?.cancel();
    const run = this.operationChain.catch(() => undefined).then(async () => {
      if (!this.disposed && generation === this.operationGeneration) {
        await this.executeOperation(operation, root, value, generation);
      }
    });
    this.operationChain = run;
    await run;
  }

  private async executeOperation(operation: Operation, root: string, value: string | readonly string[], generation: number): Promise<void> {
    const pin = this.pin;
    const cancellation = new vscode.CancellationTokenSource();
    this.activeCancellation = cancellation;
    const pinStable = pin !== undefined && pin.executable.workspaceRoot === root && this.operationCurrent(generation, cancellation) &&
      await revalidateCorvintPin(pin);
    if (!pinStable || !this.operationCurrent(generation, cancellation) || pin === undefined) {
      if (this.operationCurrent(generation, cancellation)) {
        if (this.pin === pin) {
          this.pin = undefined;
        }
        this.fail("Incompatible", new Error("PIN_DRIFT: executable is not pinned unchanged for this root"));
      }
      if (this.activeCancellation === cancellation) {
        this.activeCancellation = undefined;
      }
      cancellation.dispose();
      return;
    }
    let settings: { maxOutputBytes: number; timeoutMilliseconds: number } | undefined;
    try {
      settings = settingsFor(root);
    } catch (error) {
      this.fail("Incompatible", error);
      if (this.activeCancellation === cancellation) this.activeCancellation = undefined;
      cancellation.dispose();
      return;
    }
    if (settings === undefined) {
      this.fail("Incompatible", new Error("CLI_INCOMPATIBLE: invalid process bounds configuration"));
      if (this.activeCancellation === cancellation) {
        this.activeCancellation = undefined;
      }
      cancellation.dispose();
      return;
    }
    this.presentation.clear("Corvint operation in progress; prior snapshot cleared");
    this.presentation.setStatus("Busy", `${operation}; containment UNKNOWN`);
    let postCloseChecked = false;
    try {
      await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: `Corvint ${operation}`, cancellable: true }, async (_progress, token) => {
        const listener = token.onCancellationRequested(() => cancellation.cancel());
        try {
          if (!this.operationCurrent(generation, cancellation)) {
            return;
          }
          const snapshot = pin.transport === "cli"
            ? await this.runCli(pin.executable, operation, root, value, settings, cancellation)
            : await this.runMcp(pin.executable, operation, root, value, settings, cancellation);
          if (!this.operationCurrent(generation, cancellation)) {
            return;
          }
          const stableAfterClose = await revalidateCorvintPin(pin);
          postCloseChecked = true;
          if (!this.operationCurrent(generation, cancellation)) {
            return;
          }
          if (!stableAfterClose) {
            if (this.pin === pin) {
              this.pin = undefined;
            }
            throw new Error("PIN_DRIFT: executable changed while Corvint was running");
          }
          if (!this.operationCurrent(generation, cancellation)) {
            return;
          }
          await this.presentation.apply(snapshot, root);
          if (!this.operationCurrent(generation, cancellation)) {
            return;
          }
          this.lastOperation = { operation, root, value };
          const identity = pin.transport === "cli"
            ? `${pin.executable.cliKind} ${pin.executable.version}`
            : `${pin.executable.cliKind} ${snapshot.transportAuthority?.serverVersion ?? "unknown"} · 2026-07-28 · sha256:${pin.executable.sha256.slice(0, 12)}`;
          this.presentation.setStatus("Ready", `${identity}; receipt ${snapshot.state}; containment UNKNOWN`);
        } finally {
          listener.dispose();
        }
      });
    } catch (error) {
      let reported = error;
      if (!postCloseChecked) {
        const stableAfterFailure = await revalidateCorvintPin(pin);
        postCloseChecked = true;
        if (!stableAfterFailure) {
          if (this.pin === pin) this.pin = undefined;
          reported = new Error("PIN_DRIFT: executable identity changed during the failed operation");
        }
      }
      if (this.uiCurrent(generation)) {
        this.fail("Degraded", reported);
      }
    } finally {
      if (this.activeCancellation === cancellation) {
        this.activeCancellation = undefined;
      }
      cancellation.dispose();
    }
  }

  private async runCli(
    pin: ExecutablePin,
    operation: Operation,
    root: string,
    value: string | readonly string[],
    settings: { maxOutputBytes: number; timeoutMilliseconds: number },
    cancellation: vscode.CancellationTokenSource,
  ) {
    const result = await runBoundedProcess(pin.realPath, fixedArguments(pin.cliKind, operation, root, value), {
      cwd: root,
      env: minimumEnvironment(),
      maxStdoutBytes: settings.maxOutputBytes,
      maxStderrBytes: STDERR_LIMIT,
      timeoutMilliseconds: settings.timeoutMilliseconds,
      cancellation: cancellation.token,
    });
    return decodeCorvintReceipt(result.stdout, operation, settings.maxOutputBytes, pin.cliKind, value);
  }

  private async runMcp(
    pin: McpExecutablePin,
    operation: Operation,
    root: string,
    value: string | readonly string[],
    settings: { maxOutputBytes: number; timeoutMilliseconds: number },
    cancellation: vscode.CancellationTokenSource,
  ) {
    const source = await runMcpOperation(pin.realPath, root, operation, value, {
      cwd: root,
      env: minimumEnvironment(),
      maxMessageBytes: settings.maxOutputBytes,
      timeoutMilliseconds: settings.timeoutMilliseconds,
      clientVersion: this.extensionVersion,
      cancellation: cancellation.token,
    });
    const authority: TransportAuthority = Object.freeze({
      profile: "corvint-mcp-bridge-result/0",
      tool: `corvint.${operation}`,
      state: source.authority.state,
      epistemicClass: source.authority.epistemicClass,
      authorityClass: source.authority.authorityClass,
      abstentionReason: source.authority.abstention.reason,
      ...(source.authority.repository === undefined ? {} : { repository: source.authority.repository }),
      serverVersion: source.authority.serverVersion,
      executableSha256: pin.sha256,
      protocol: source.authority.protocolVersion,
    });
    if (source.context === undefined) {
      return createTransportAbstentionSnapshot(operation, value, source.snapshotBytes, authority);
    }
    return attachTransportAuthority(
      decodeContextReceipt(source.context, operation, settings.maxOutputBytes, "corvint", value, source.snapshotBytes),
      authority,
    );
  }

  private async importObservation(): Promise<void> {
    if (!this.guardTrusted()) {
      return;
    }
    const root = await this.selectRoot();
    if (root === undefined || this.disposed || !vscode.workspace.isTrusted) {
      return;
    }
    this.invalidateOperations();
    const generation = this.operationGeneration;
    let configured: string;
    try {
      configured = configurationValue("testObservationPath", "", vscode.Uri.file(root));
    } catch (error) {
      this.fail("Incompatible", error);
      return;
    }
    const relativePath = configured.length > 0 ? configured : await vscode.window.showInputBox({
      title: "Import Corvint test observation",
      prompt: "Workspace-relative path to a complete go-live-event/0 JSONL sequence and go-live-run/0 terminal.",
      ignoreFocusOut: true,
    });
    if (relativePath === undefined || !this.uiCurrent(generation)) {
      return;
    }
    try {
      const bytes = await readStableObservation(root, relativePath);
      if (!this.uiCurrent(generation)) return;
      const observation = decodeObservation(bytes);
      if (!this.uiCurrent(generation)) return;
      this.testing.import(observation);
      this.presentation.setStatus("Degraded", `UNVERIFIED_IMPORT ${observation.runId}; observation only, not current`);
    } catch (error) {
      if (this.uiCurrent(generation)) {
        this.testing.clear();
        this.fail("Degraded", error);
      }
    }
  }

  private clearResults(): void {
    if (this.disposed) return;
    this.invalidateOperations();
    this.lastOperation = undefined;
    this.testing.clear();
    this.presentation.clear();
    if (!vscode.workspace.isTrusted) {
      this.presentation.setStatus("Restricted");
    } else if (this.pin !== undefined) {
      const identity = this.pin.transport === "cli"
        ? `${this.pin.executable.cliKind} ${this.pin.executable.version}`
        : `${this.pin.executable.cliKind} pinned`;
      this.presentation.setStatus("Ready", identity);
    } else {
      this.presentation.setStatus(supportedExecutionPlatform() ? "No CLI" : "Unsupported");
    }
  }

  private guardTrusted(): boolean {
    if (vscode.workspace.isTrusted) {
      return true;
    }
    this.invalidateOperations();
    this.pin = undefined;
    this.lastOperation = undefined;
    this.testing.clear();
    this.presentation.clear("Workspace Trust required");
    this.presentation.setStatus("Restricted", "Workspace Trust is required");
    return false;
  }

  private guardPlatform(): boolean {
    if (supportedExecutionPlatform()) {
      return true;
    }
    this.presentation.setStatus("Unsupported", "Corvint execution is limited to Darwin and Linux in V0");
    return false;
  }

  private async selectRoot(preferred?: vscode.Uri): Promise<string | undefined> {
    if (this.disposed) return undefined;
    const folders = vscode.workspace.workspaceFolders?.filter((folder) => folder.uri.scheme === "file") ?? [];
    if (folders.length === 0) {
      this.fail("Unsupported", new Error("ROOT_UNSUPPORTED: a file workspace folder is required"));
      return undefined;
    }
    let selected = preferred === undefined ? undefined : vscode.workspace.getWorkspaceFolder(preferred);
    if (selected === undefined && folders.length === 1) {
      selected = folders[0];
    }
    if (selected === undefined) {
      selected = await vscode.window.showWorkspaceFolderPick({ placeHolder: "Select the exact workspace folder for this Corvint operation" });
    }
    if (this.disposed || !vscode.workspace.isTrusted) return undefined;
    if (selected === undefined || selected.uri.scheme !== "file") {
      return undefined;
    }
    const lexical = path.resolve(selected.uri.fsPath);
    try {
      const resolved = await realpath(lexical);
      if (this.disposed || !vscode.workspace.isTrusted) return undefined;
      if (resolved !== lexical) {
        throw new Error("ROOT_UNSUPPORTED: workspace root must not resolve through a different path");
      }
      return resolved;
    } catch {
      if (!this.disposed) this.fail("Unsupported", new Error("ROOT_UNSUPPORTED: workspace root could not be resolved safely"));
      return undefined;
    }
  }

  private fail(status: "No CLI" | "Incompatible" | "Degraded" | "Unsupported", error: unknown): void {
    const message = inertError(error);
    this.presentation.clear(message);
    this.presentation.setStatus(status, message);
    void vscode.window.showWarningMessage(`Corvint ${status}: ${message}`);
  }

  private invalidateOperations(): void {
    this.operationGeneration += 1;
    this.activeCancellation?.cancel();
  }

  private operationCurrent(generation: number, cancellation: vscode.CancellationTokenSource): boolean {
    return !this.disposed && vscode.workspace.isTrusted && generation === this.operationGeneration &&
      !cancellation.token.isCancellationRequested;
  }

  private uiCurrent(generation: number): boolean {
    return !this.disposed && vscode.workspace.isTrusted && generation === this.operationGeneration;
  }
}

function settingsFor(root: string): { maxOutputBytes: number; timeoutMilliseconds: number } | undefined {
  const resource = vscode.Uri.file(root);
  const maxOutputBytes = configurationValue("maxOutputBytes", 262_144, resource);
  const timeoutMilliseconds = configurationValue("timeoutMilliseconds", 15_000, resource);
  if (!Number.isSafeInteger(maxOutputBytes) || maxOutputBytes < 4_096 || maxOutputBytes > 1_048_576 ||
    !Number.isSafeInteger(timeoutMilliseconds) || timeoutMilliseconds < 1_000 || timeoutMilliseconds > 30_000) {
    return undefined;
  }
  return { maxOutputBytes, timeoutMilliseconds };
}

function transportFor(root: string): "cli" | "mcp-stdio" | undefined {
  const value = configurationValue("transport", "cli", vscode.Uri.file(root));
  return value === "cli" || value === "mcp-stdio" ? value : undefined;
}

async function revalidateCorvintPin(pin: CorvintPin): Promise<boolean> {
  return pin.transport === "cli"
    ? await revalidatePin(pin.executable)
    : await revalidateMcpPin(pin.executable);
}

function manifestVersion(context: vscode.ExtensionContext): string {
  const value = (context.extension.packageJSON as { readonly version?: unknown }).version;
  if (typeof value !== "string" || value.length === 0 || Buffer.byteLength(value, "utf8") > 128) {
    return "0.0.0-invalid";
  }
  return value;
}

function validateTask(value: string): string | undefined {
  if (value.length === 0 || [...value].length > 2_000 || Buffer.byteLength(value, "utf8") > 16_384 || /[\u0000-\u001f\u007f]/u.test(value)) {
    return "Task must be 1–2,000 Unicode scalars, at most 16,384 UTF-8 bytes, without control characters.";
  }
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) {
        return "Task contains an unpaired surrogate.";
      }
      index += 1;
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      return "Task contains an unpaired surrogate.";
    }
  }
  return undefined;
}

function supportedExecutionPlatform(): boolean {
  return process.platform === "darwin" || process.platform === "linux";
}

function inertError(error: unknown): string {
  if (error instanceof ProcessFailure) {
    return `${failureCode(error)}: local Corvint process failed`;
  }
  if (error instanceof McpFailure) {
    return display(`${mcpFailureCode(error)}: ${error.message}`, 500);
  }
  const message = error instanceof Error ? error.message : "";
  const code = /^([A-Z][A-Z0-9_]*):/.exec(message)?.[1] ?? "CLI_FAILED";
  return stableLocalFailure(code);
}

function stableLocalFailure(code: string): string {
  const descriptions: Readonly<Record<string, string>> = {
    CLI_FAILED: "local Corvint operation failed",
    CLI_INCOMPATIBLE: "local Corvint executable or configuration is incompatible",
    CLI_MISSING: "no local Corvint executable was admitted",
    INVALID_JSON: "local Corvint output did not match the admitted schema",
    MCP_MISSING: "no configured Corvint MCP executable was admitted",
    OBSERVATION_CHANGED: "local observation changed during bounded acquisition",
    PATH_REJECTED: "local path did not satisfy the workspace boundary",
    PIN_DRIFT: "pinned executable identity changed",
    ROOT_UNSUPPORTED: "workspace root did not satisfy the local execution profile",
    UNVERIFIED_IMPORT: "test observation is not independently verified",
    UNSUPPORTED_PROFILE: "local Corvint output profile is unsupported",
  };
  const admitted = Object.hasOwn(descriptions, code) ? code : "CLI_FAILED";
  return `${admitted}: ${descriptions[admitted]}`;
}

function mcpQualificationError(error: unknown): Error | McpFailure {
  if (error instanceof Error && error.message.startsWith("MCP_MISSING:")) {
    return error;
  }
  return new McpFailure("incompatible", "configured MCP executable failed local qualification");
}

function mcpFailureCode(error: McpFailure): string {
  switch (error.code) {
    case "cancelled": return "CANCELLED";
    case "timed-out": return "TIMEOUT";
    case "output-limit": return "OUTPUT_LIMIT";
    case "spawn-failed": return "SPAWN_DENIED";
    case "process-residue": return "PROCESS_RESIDUE";
    case "incompatible": return "MCP_INCOMPATIBLE";
    case "input-unsupported": return "MCP_INPUT_UNSUPPORTED";
    case "toolset-mismatch": return "MCP_TOOLSET_MISMATCH";
    case "tool-error": return "MCP_TOOL_ERROR";
    case "interaction-required": return "MCP_INTERACTION_REQUIRED";
    case "protocol": return "MCP_PROTOCOL";
  }
}

function failureCode(error: ProcessFailure): string {
  switch (error.code) {
    case "cancelled": return "CANCELLED";
    case "timed-out": return "TIMEOUT";
    case "output-limit": return "OUTPUT_LIMIT";
    case "spawn-failed": return "SPAWN_DENIED";
    case "process-residue": return "PROCESS_RESIDUE";
    case "nonzero-exit": return "CLI_FAILED";
  }
}
