import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import * as path from "node:path";
import { isJsonObject, parseBoundedJson, type JsonObject, type JsonValue } from "./json.js";
import type { Operation } from "./model.js";
import type { CancellationLike } from "./process.js";

const PROTOCOL_VERSION = "2026-07-28";
const MAX_TOOLS_PER_PAGE = 64;
const MAX_TOTAL_TOOLS = 256;
const STDERR_LIMIT = 65_536;
const CURRENT_SERVER = Object.freeze({
  name: "corvint-mcp",
  description: "Local read-only Corvint context and repository evidence server.",
} as const);

export type McpFailureCode =
  | "cancelled"
  | "incompatible"
  | "input-unsupported"
  | "interaction-required"
  | "output-limit"
  | "process-residue"
  | "protocol"
  | "spawn-failed"
  | "timed-out"
  | "tool-error"
  | "toolset-mismatch";

export class McpFailure extends Error {
  constructor(readonly code: McpFailureCode, message: string) {
    super(message);
    this.name = "McpFailure";
  }
}

export interface McpOperationOptions {
  readonly cwd: string;
  readonly env: Readonly<Record<string, string>>;
  readonly maxMessageBytes: number;
  readonly timeoutMilliseconds: number;
  readonly clientVersion: string;
  readonly cancellation?: CancellationLike;
}

export interface McpRepositoryBinding {
  readonly commitRevision: string;
  readonly treeRevision: string;
  readonly objectFormat: "sha1" | "sha256";
  readonly profileId: "generic" | "beamfall";
  readonly worktreeState: "CLEAN" | "MIXED";
  readonly dirtyPathCount: number;
  readonly dirtyPathsSha256: string;
}

export interface McpAuthority {
  readonly protocolVersion: typeof PROTOCOL_VERSION;
  readonly serverVersion: string;
  readonly state: "READY" | "ABSTAINED";
  readonly epistemicClass: "OBSERVED" | "NOT_OBSERVED";
  readonly authorityClass: "REPOSITORY_EVIDENCE" | "NONE";
  readonly repository?: McpRepositoryBinding;
  readonly abstention: { readonly active: boolean; readonly reason: string };
}

export interface McpOperationResult {
  readonly context?: JsonObject;
  readonly snapshotBytes: Uint8Array;
  readonly authority: McpAuthority;
}

type ServerInfo = typeof CURRENT_SERVER & { readonly version: string };

/**
 * Runs one explicit Corvint operation in one root-bound modern MCP stdio session.
 * The native context receipt remains unaltered and is handed to the shared
 * strict context decoder; the bridge is never rewritten as a CLI envelope.
 */
export async function runMcpOperation(
  executable: string,
  root: string,
  operation: Operation,
  value: string | readonly string[],
  options: McpOperationOptions,
): Promise<McpOperationResult> {
  validateInputs(executable, root, options);
  if (options.cancellation?.isCancellationRequested === true) {
    throw new McpFailure("cancelled", "MCP operation was cancelled before spawn");
  }
  const tool = `corvint.${operation}`;
  const args = operation === "query"
    ? queryArguments(value)
    : impactArguments(value);
  const transport = new McpTransport(executable, root, options);
  let operationFailure: unknown;
  let receipt: McpOperationResult | undefined;
  try {
    let serverInfo: ServerInfo;
    try {
      serverInfo = validateDiscovery(await transport.request("server/discover", {}));
    } catch (error) {
      throw preserveLifecycleOr(error, "incompatible", "MCP discovery failed");
    }
    try {
      await listAndValidateTools(transport, serverInfo);
    } catch (error) {
      if (error instanceof McpFailure && error.code === "incompatible") {
        throw error;
      }
      throw preserveLifecycleOr(error, "toolset-mismatch", "MCP tool registry is incompatible");
    }
    const call = await transport.request("tools/call", { name: tool, arguments: args });
    receipt = decodeCallResult(call, operation, serverInfo);
  } catch (error) {
    operationFailure = error;
  }
  const shutdownFailure = await transport.shutdown();
  if (shutdownFailure?.code === "process-residue" || operationFailure === undefined && shutdownFailure !== undefined) {
    operationFailure = shutdownFailure;
  }
  if (operationFailure !== undefined) {
    throw operationFailure;
  }
  if (receipt === undefined) {
    throw new McpFailure("protocol", "MCP operation produced no receipt");
  }
  return receipt;
}

class McpTransport {
  private readonly child: ChildProcessWithoutNullStreams;
  private readonly maxMessageBytes: number;
  private readonly closePromise: Promise<void>;
  private readonly cancellationSubscription;
  private readonly deadline: NodeJS.Timeout;
  private buffer = Buffer.alloc(0);
  private stderrBytes = 0;
  private totalStdoutBytes = 0;
  private nextId = 1;
  private current: { id: number; resolve(value: JsonObject): void; reject(error: Error): void } | undefined;
  private terminalFailure: McpFailure | undefined;
  private closed = false;
  private inputEnded = false;
  private cancelled = false;
  private readonly clientVersion: string;

  constructor(executable: string, root: string, options: McpOperationOptions) {
    this.maxMessageBytes = options.maxMessageBytes;
    this.clientVersion = options.clientVersion;
    try {
      this.child = spawn(executable, ["--root", root], {
        cwd: options.cwd,
        env: { ...options.env },
        shell: false,
        windowsHide: true,
        detached: false,
        stdio: ["pipe", "pipe", "pipe"],
      });
    } catch {
      throw new McpFailure("spawn-failed", "MCP process could not be started");
    }
    this.closePromise = new Promise<void>((resolve) => {
      this.child.once("close", (code, terminationSignal) => {
        this.closed = true;
        if (!this.cancelled && this.buffer.byteLength !== 0 && this.terminalFailure === undefined) {
          this.terminalFailure = new McpFailure("protocol", "MCP closed with an unterminated frame");
        }
        if (!this.cancelled && (code !== 0 || terminationSignal !== null) && this.terminalFailure === undefined) {
          this.terminalFailure = new McpFailure("protocol", "MCP process did not close successfully");
        }
        this.rejectCurrent(this.terminalFailure ?? new McpFailure("protocol", "MCP process closed before a complete response"));
        resolve();
      });
    });
    this.child.once("error", () => this.abort(new McpFailure("spawn-failed", "MCP process failed")));
    // A server that closes its stdin makes the next write fail with EPIPE; without a
    // listener that stream error is an uncaught exception in the extension host.
    this.child.stdin.on("error", () => this.abort(new McpFailure("spawn-failed", "MCP stdin write failed")));
    this.child.stdout.on("data", (chunk: Buffer | string) => this.onStdout(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk)));
    this.child.stderr.on("data", (chunk: Buffer | string) => {
      this.stderrBytes += Buffer.byteLength(chunk);
      if (this.stderrBytes > STDERR_LIMIT) {
        this.abort(new McpFailure("output-limit", "MCP stderr exceeded its byte bound"));
      }
    });
    this.deadline = setTimeout(() => this.cancel("timed-out", "MCP operation timed out"), options.timeoutMilliseconds);
    this.deadline.unref();
    this.cancellationSubscription = options.cancellation?.onCancellationRequested(() => {
      this.cancel("cancelled", "MCP operation was cancelled");
    });
    if (options.cancellation?.isCancellationRequested === true) {
      this.cancel("cancelled", "MCP operation was cancelled");
    }
  }

  async request(method: string, params: Readonly<Record<string, JsonValue>>): Promise<JsonObject> {
    if (this.terminalFailure !== undefined) {
      throw this.terminalFailure;
    }
    if (this.current !== undefined || this.closed || this.inputEnded) {
      throw new McpFailure("protocol", "MCP request lifecycle is invalid");
    }
    const id = this.nextId++;
    const frame = canonicalJson({
      jsonrpc: "2.0",
      id,
      method,
      params: { ...params, _meta: requestMeta(this.clientVersion) },
    });
    if (Buffer.byteLength(frame, "utf8") > this.maxMessageBytes) {
      throw new McpFailure("output-limit", "MCP request exceeded its byte bound");
    }
    const response = new Promise<JsonObject>((resolve, reject) => {
      this.current = { id, resolve, reject };
    });
    this.child.stdin.write(`${frame}\n`, "utf8", (error) => {
      if (error !== null && error !== undefined) {
        this.abort(new McpFailure("spawn-failed", "MCP stdin write failed"));
      }
    });
    return await response;
  }

  async shutdown(): Promise<McpFailure | undefined> {
    clearTimeout(this.deadline);
    this.cancellationSubscription?.dispose();
    this.endInput();
    if (await settlesWithin(this.closePromise, 250)) {
      return this.terminalFailure;
    }
    signal(this.child, "SIGTERM");
    if (await settlesWithin(this.closePromise, 250)) {
      return this.terminalFailure;
    }
    signal(this.child, "SIGKILL");
    if (await settlesWithin(this.closePromise, 250)) {
      return this.terminalFailure;
    }
    return new McpFailure("process-residue", "MCP direct child did not exit after shutdown escalation");
  }

  private onStdout(chunk: Buffer): void {
    this.totalStdoutBytes += chunk.byteLength;
    if (this.totalStdoutBytes > this.maxMessageBytes) {
      const failure = new McpFailure("output-limit", "MCP session output exceeded its aggregate byte bound");
      if (this.cancelled) {
        this.terminalFailure = failure;
      } else {
        this.abort(failure);
      }
      return;
    }
    if (this.cancelled) {
      return;
    }
    this.buffer = Buffer.concat([this.buffer, chunk]);
    if (this.buffer.byteLength > this.maxMessageBytes && this.buffer.indexOf(0x0a) === -1) {
      this.abort(new McpFailure("output-limit", "MCP partial frame exceeded its byte bound"));
      return;
    }
    while (true) {
      const newline = this.buffer.indexOf(0x0a);
      if (newline < 0) {
        return;
      }
      const frame = this.buffer.subarray(0, newline);
      this.buffer = this.buffer.subarray(newline + 1);
      if (frame.byteLength === 0 || frame.byteLength > this.maxMessageBytes) {
        this.abort(new McpFailure("output-limit", "MCP frame exceeded its byte bound"));
        return;
      }
      this.onFrame(frame);
      if (this.terminalFailure !== undefined) {
        return;
      }
    }
  }

  private onFrame(frame: Uint8Array): void {
    let value: JsonValue;
    try {
      value = parseBoundedJson(frame, {
        maxBytes: this.maxMessageBytes,
        maxDepth: 64,
        maxNodes: 65_536,
        maxStringBytes: 65_536,
      });
    } catch {
      this.abort(new McpFailure("protocol", "MCP emitted invalid bounded JSON"));
      return;
    }
    if (!isJsonObject(value)) {
      this.abort(new McpFailure("protocol", "MCP emitted a non-object frame"));
      return;
    }
    if (value.method !== undefined) {
      // V0 advertises no server requests or notifications. Progress, logging,
      // input-required, sampling, elicitation, and resource traffic all fail closed.
      this.abort(new McpFailure("protocol", "MCP emitted an unadvertised request or notification"));
      return;
    }
    const active = this.current;
    if (active === undefined || value.jsonrpc !== "2.0" || value.id !== active.id) {
      this.abort(new McpFailure("protocol", "MCP response identity is inconsistent"));
      return;
    }
    this.current = undefined;
    const keys = Object.keys(value).sort();
    if (value.error !== undefined) {
      if (!sameStrings(keys, ["error", "id", "jsonrpc"])) {
        active.reject(new McpFailure("protocol", "MCP error envelope is not closed"));
        return;
      }
      const error = object(value.error, "MCP error");
      if (!allowedExactKeys(error, ["code", "message"], ["data"]) ||
        typeof error.code !== "number" || !Number.isSafeInteger(error.code) || typeof error.message !== "string") {
        active.reject(new McpFailure("protocol", "MCP error is malformed"));
        return;
      }
      active.reject(new McpFailure("protocol", `MCP request failed with code ${error.code}`));
      return;
    }
    const result = value.result;
    if (!sameStrings(keys, ["id", "jsonrpc", "result"]) || result === undefined || !isJsonObject(result)) {
      active.reject(new McpFailure("protocol", "MCP success envelope is not closed"));
      return;
    }
    active.resolve(result);
  }

  private cancel(code: "cancelled" | "timed-out", message: string): void {
    if (this.terminalFailure !== undefined || this.closed) {
      return;
    }
    this.cancelled = true;
    const failure = new McpFailure(code, message);
    this.terminalFailure = failure;
    const currentId = this.current?.id;
    this.rejectCurrent(failure);
    if (currentId === undefined || this.inputEnded) {
      this.endInput();
      return;
    }
    const notification = canonicalJson({
      jsonrpc: "2.0",
      method: "notifications/cancelled",
      params: { _meta: requestMeta(this.clientVersion), requestId: currentId, reason: "client-cancelled" },
    });
    this.inputEnded = true;
    this.child.stdin.end(`${notification}\n`, "utf8");
  }

  private abort(failure: McpFailure): void {
    if (this.terminalFailure !== undefined) {
      return;
    }
    this.terminalFailure = failure;
    this.rejectCurrent(failure);
    this.endInput();
  }

  private rejectCurrent(error: Error): void {
    const active = this.current;
    this.current = undefined;
    active?.reject(error);
  }

  private endInput(): void {
    if (!this.inputEnded) {
      this.inputEnded = true;
      this.child.stdin.end();
    }
  }
}

function requestMeta(clientVersion: string): JsonObject {
  return {
    "io.modelcontextprotocol/protocolVersion": PROTOCOL_VERSION,
    "io.modelcontextprotocol/clientCapabilities": {},
    "io.modelcontextprotocol/clientInfo": { name: "corvint-vscode", version: clientVersion },
  };
}

function validateDiscovery(result: JsonObject): ServerInfo {
  exactKeys(result, ["_meta", "cacheScope", "capabilities", "resultType", "supportedVersions", "ttlMs"], "discovery");
  if (result.resultType !== "complete" || result.cacheScope !== "public" || result.ttlMs !== 0 ||
    !stringArrayEquals(result.supportedVersions, [PROTOCOL_VERSION])) {
    throw new McpFailure("protocol", "MCP discovery contract is incompatible");
  }
  const capabilities = object(result.capabilities, "discovery capabilities");
  exactKeys(capabilities, ["tools"], "discovery capabilities");
  const tools = object(capabilities.tools, "tools capability");
  exactKeys(tools, ["listChanged"], "tools capability");
  if (tools.listChanged !== false) {
    throw new McpFailure("protocol", "MCP tools capability is not stable");
  }
  return validateServerMeta(result._meta);
}

async function listAndValidateTools(transport: McpTransport, expectedServerInfo: ServerInfo): Promise<void> {
  const result = await transport.request("tools/list", {});
  if (!allowedExactKeys(result, ["_meta", "cacheScope", "resultType", "tools", "ttlMs"], [])) {
    throw new McpFailure("toolset-mismatch", "MCP tool-list result is not closed");
  }
  if (result.resultType !== "complete" || result.cacheScope !== "private" || result.ttlMs !== 300_000) {
    throw new McpFailure("toolset-mismatch", "MCP tool-list metadata is incompatible");
  }
  requireSameServerInfo(validateServerMeta(result._meta), expectedServerInfo);
  const tools = list(result.tools, "MCP tools", MAX_TOOLS_PER_PAGE).map((tool) => object(tool, "MCP tool descriptor"));
  if (tools.length > MAX_TOTAL_TOOLS) {
    throw new McpFailure("toolset-mismatch", "MCP advertised too many tools");
  }
  validateToolDescriptors(tools);
}

function validateToolDescriptors(actual: readonly JsonObject[]): void {
  const expected = expectedTools();
  if (canonicalJson([...actual]) !== canonicalJson([...expected])) {
    throw new McpFailure("toolset-mismatch", "MCP advertised a non-conforming tool registry");
  }
}

function expectedTools(): readonly JsonObject[] {
  const annotations = { destructiveHint: false, idempotentHint: true, openWorldHint: false, readOnlyHint: true };
  return [
    {
      name: "corvint.impact",
      description: "Compile revision-bound impact evidence for tracked Go files without returning source bodies.",
      annotations,
      inputSchema: schema({
        paths: { type: "array", minItems: 1, maxItems: 100, uniqueItems: true, items: { type: "string", minLength: 1, maxLength: 1024, pattern: "^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\\\).+[.]go$" } },
        limit: { type: "integer", minimum: 1, maximum: 50, default: 10 },
      }, ["paths"]),
    },
    {
      name: "corvint.query",
      description: "Compile the narrow project-operations authority-start receipt (limit is fixed at one).",
      annotations,
      inputSchema: schema({ task: { type: "string", minLength: 1, maxLength: 2000, pattern: "^[ -~]*[!-~][ -~]*$" } }, ["task"]),
    },
    {
      name: "corvint.status",
      description: "Observe Git commit, tree, worktree state, and a privacy-preserving dirty-path digest.",
      annotations,
      inputSchema: schema({}, []),
    },
  ];
}

function schema(properties: JsonObject, required: readonly string[]): JsonObject {
  return { "$schema": "https://json-schema.org/draft/2020-12/schema", type: "object", properties, required: [...required], additionalProperties: false };
}

function queryArguments(value: string | readonly string[]): JsonObject {
  if (typeof value !== "string" || [...value].length > 2_000 || !/^[ -~]*[!-~][ -~]*$/u.test(value)) {
    throw new McpFailure("input-unsupported", "MCP query task is outside the admitted profile");
  }
  return { task: value };
}

function impactArguments(value: string | readonly string[]): JsonObject {
  if (typeof value === "string" || value.length < 1 || value.length > 100) {
    throw new McpFailure("input-unsupported", "MCP impact paths are outside the admitted profile");
  }
  const paths = [...value];
  if (new Set(paths).size !== paths.length || paths.some((entry) =>
    entry.length === 0 || [...entry].length > 1_024 || Buffer.byteLength(entry, "utf8") > 4_096 ||
    !inertUnicode(entry) || /[\u0000-\u001f\u007f]/u.test(entry) || !entry.endsWith(".go") || entry.includes("\\") || entry.startsWith("/") ||
    entry.split("/").some((part) => part.length === 0 || part === "." || part === ".."))) {
    throw new McpFailure("input-unsupported", "MCP impact path is outside the admitted profile");
  }
  return { paths, limit: 20 };
}

export function decodeCallResult(result: JsonObject, operation: Operation, expectedServerInfo?: ServerInfo): McpOperationResult {
  if (!allowedExactKeys(result, ["_meta", "content", "isError", "resultType", "structuredContent"], [])) {
    throw new McpFailure("protocol", "MCP call result is not closed");
  }
  if (result.resultType === "input_required") {
    throw new McpFailure("interaction-required", "MCP requested an unsupported interaction");
  }
  if (result.resultType !== "complete" || typeof result.isError !== "boolean") {
    throw new McpFailure("protocol", "MCP call did not complete");
  }
  const serverInfo = validateServerMeta(result._meta);
  if (expectedServerInfo !== undefined) {
    requireSameServerInfo(serverInfo, expectedServerInfo);
  }
  const structured = object(result.structuredContent, "MCP structuredContent");
  validateCanonicalContent(result.content, structured);
  if (result.isError) {
    validateToolError(structured, operation);
    throw new McpFailure("tool-error", "MCP tool returned a closed operational failure");
  }
  const bridge = validateBridge(structured, operation);
  return Object.freeze({
    ...(bridge.receipt === undefined ? {} : { context: bridge.receipt }),
    snapshotBytes: Buffer.from(canonicalJson(structured), "utf8"),
    authority: Object.freeze({
      protocolVersion: PROTOCOL_VERSION,
      serverVersion: serverInfo.version,
      state: bridge.state,
      epistemicClass: bridge.epistemicClass,
      authorityClass: bridge.authorityClass,
      ...(bridge.repository === undefined ? {} : { repository: bridge.repository }),
      abstention: Object.freeze(bridge.abstention),
    }),
  });
}

function validateCanonicalContent(value: JsonValue | undefined, structured: JsonObject): void {
  const content = list(value, "MCP content", 1);
  if (content.length !== 1) {
    throw new McpFailure("protocol", "MCP call content duplicate is missing");
  }
  const block = object(content[0], "MCP text content");
  exactKeys(block, ["text", "type"], "MCP text content");
  if (block.type !== "text" || typeof block.text !== "string" || Buffer.byteLength(block.text, "utf8") > 384 * 1024 ||
    block.text !== canonicalJson(structured)) {
    throw new McpFailure("protocol", "MCP text and structured content are not canonical duplicates");
  }
}

function validateBridge(value: JsonObject, operation: Operation): {
  readonly receipt?: JsonObject;
  readonly state: "READY" | "ABSTAINED";
  readonly epistemicClass: "OBSERVED" | "NOT_OBSERVED";
  readonly authorityClass: "REPOSITORY_EVIDENCE" | "NONE";
  readonly repository?: McpRepositoryBinding;
  readonly abstention: { readonly active: boolean; readonly reason: string };
} {
  exactKeys(value, ["abstention", "authorityClass", "epistemicClass", "mutates", "receipt", "repository", "schema", "state", "tool"], "MCP bridge");
  const expectedTool = `corvint.${operation}`;
  if (value.schema !== "corvint-mcp-bridge-result/0" || value.tool !== expectedTool || value.mutates !== false ||
    (value.state !== "READY" && value.state !== "ABSTAINED") ||
    (value.epistemicClass !== "OBSERVED" && value.epistemicClass !== "NOT_OBSERVED") ||
    (value.authorityClass !== "REPOSITORY_EVIDENCE" && value.authorityClass !== "NONE")) {
    throw new McpFailure("protocol", "MCP bridge authority envelope is invalid");
  }
  const abstention = object(value.abstention, "MCP abstention");
  exactKeys(abstention, ["active", "reason"], "MCP abstention");
  if (typeof abstention.active !== "boolean" || typeof abstention.reason !== "string" || !/^[A-Z][A-Z0-9_]*$/.test(abstention.reason)) {
    throw new McpFailure("protocol", "MCP abstention is invalid");
  }
  const repository = validateRepository(value.repository);
  if (value.state === "READY") {
    if (value.epistemicClass !== "OBSERVED" || value.authorityClass !== "REPOSITORY_EVIDENCE" || abstention.active || abstention.reason !== "NONE" || repository === undefined) {
      throw new McpFailure("protocol", "MCP ready authority is inconsistent");
    }
  } else if (!abstention.active || abstention.reason === "NONE" || value.epistemicClass !== "NOT_OBSERVED" || value.authorityClass !== "NONE") {
    throw new McpFailure("protocol", "MCP abstention authority is inconsistent");
  }
  const authority = {
    state: value.state,
    epistemicClass: value.epistemicClass,
    authorityClass: value.authorityClass,
    ...(repository === undefined ? {} : { repository }),
    abstention: Object.freeze({ active: abstention.active, reason: abstention.reason }),
  } as const;
  if (value.receipt === null) {
    if (value.state !== "ABSTAINED") {
      throw new McpFailure("protocol", "MCP ready result omitted its native receipt");
    }
    return authority;
  }
  const receipt = object(value.receipt, "MCP native receipt");
  if (repository !== undefined && receipt.revision !== repository.treeRevision) {
    throw new McpFailure("protocol", "MCP receipt and repository revision differ");
  }
  if (value.state === "READY" && receipt.state === "OUT_OF_SCOPE") {
    throw new McpFailure("protocol", "MCP ready wrapper contradicts the native receipt state");
  }
  if (value.state === "ABSTAINED" &&
    (operation !== "impact" || abstention.reason !== "OUT_OF_SCOPE" || receipt.state !== "OUT_OF_SCOPE")) {
    throw new McpFailure("protocol", "MCP retained abstention receipt is inconsistent");
  }
  return { ...authority, receipt };
}

function validateRepository(value: JsonValue | undefined): McpRepositoryBinding | undefined {
  if (value === null) {
    return undefined;
  }
  const repository = object(value, "MCP repository binding");
  exactKeys(repository, ["commitRevision", "dirtyPathCount", "dirtyPathsSha256", "objectFormat", "profileId", "treeRevision", "worktreeState"], "MCP repository binding");
  const revisionWidth = repository.objectFormat === "sha1" ? 40 : repository.objectFormat === "sha256" ? 64 : 0;
  if (revisionWidth === 0 || !hex(repository.commitRevision, revisionWidth, revisionWidth) || !hex(repository.treeRevision, revisionWidth, revisionWidth) || !hex(repository.dirtyPathsSha256, 64, 64) ||
    typeof repository.dirtyPathCount !== "number" || !Number.isSafeInteger(repository.dirtyPathCount) || repository.dirtyPathCount < 0 || repository.dirtyPathCount > 1_000_000 ||
    (repository.profileId !== "generic" && repository.profileId !== "beamfall") ||
    (repository.worktreeState !== "CLEAN" && repository.worktreeState !== "MIXED")) {
    throw new McpFailure("protocol", "MCP repository binding is invalid");
  }
  return Object.freeze({
    commitRevision: repository.commitRevision as string,
    treeRevision: repository.treeRevision as string,
    objectFormat: repository.objectFormat as "sha1" | "sha256",
    profileId: repository.profileId as "generic" | "beamfall",
    worktreeState: repository.worktreeState as "CLEAN" | "MIXED",
    dirtyPathCount: repository.dirtyPathCount as number,
    dirtyPathsSha256: repository.dirtyPathsSha256 as string,
  });
}

function validateToolError(value: JsonObject, operation: Operation): void {
  exactKeys(value, ["abstention", "code", "mutates", "profile", "tool"], "MCP tool error");
  const abstention = object(value.abstention, "MCP tool-error abstention");
  exactKeys(abstention, ["active", "reason"], "MCP tool-error abstention");
  if (value.profile !== "corvint-mcp-tool-error/0" || value.tool !== `corvint.${operation}` || value.mutates !== false ||
    value.code === undefined || typeof value.code !== "string" || !/^[a-z][a-z0-9-]*$/.test(value.code) ||
    abstention.active !== true || abstention.reason !== "OPERATION_FAILED") {
    throw new McpFailure("protocol", "MCP tool error is malformed");
  }
}

function validateServerMeta(value: JsonValue | undefined): ServerInfo {
  const meta = object(value, "MCP server metadata");
  exactKeys(meta, ["io.modelcontextprotocol/serverInfo"], "MCP server metadata");
  const info = object(meta["io.modelcontextprotocol/serverInfo"], "MCP server info");
  exactKeys(info, ["description", "name", "version"], "MCP server info");
  const presentation = [CURRENT_SERVER].find((candidate) =>
    info.name === candidate.name && info.description === candidate.description);
  if (presentation === undefined || typeof info.version !== "string" ||
    info.version.length === 0 || Buffer.byteLength(info.version, "utf8") > 128 || !inertProtocolText(info.version)) {
    throw new McpFailure("protocol", "MCP server identity is incompatible");
  }
  return Object.freeze({ ...presentation, version: info.version });
}

function requireSameServerInfo(actual: ServerInfo, expected: ServerInfo): void {
  if (actual.name !== expected.name || actual.description !== expected.description || actual.version !== expected.version) {
    throw new McpFailure("incompatible", "MCP server identity changed during the session");
  }
}

function validateInputs(executable: string, root: string, options: McpOperationOptions): void {
  if (!path.isAbsolute(executable) || !path.isAbsolute(root) || executable.includes("\0") || root.includes("\0") ||
    !Number.isSafeInteger(options.maxMessageBytes) || options.maxMessageBytes < 4_096 || options.maxMessageBytes > 1_048_576 ||
    !Number.isSafeInteger(options.timeoutMilliseconds) || options.timeoutMilliseconds < 1_000 || options.timeoutMilliseconds > 30_000 ||
    options.clientVersion.length === 0 || Buffer.byteLength(options.clientVersion, "utf8") > 128) {
    throw new McpFailure("spawn-failed", "MCP execution parameters are invalid");
  }
}

function object(value: JsonValue | undefined, name: string): JsonObject {
  if (value === undefined || !isJsonObject(value)) {
    throw new McpFailure("protocol", `${name} must be an object`);
  }
  return value;
}

function list(value: JsonValue | undefined, name: string, maximum: number): readonly JsonValue[] {
  if (!Array.isArray(value) || value.length > maximum) {
    throw new McpFailure("protocol", `${name} must be a bounded array`);
  }
  return value;
}

function exactKeys(value: JsonObject, keys: readonly string[], name: string): void {
  if (!allowedExactKeys(value, keys, [])) {
    throw new McpFailure("protocol", `${name} keys do not match the admitted contract`);
  }
}

function allowedExactKeys(value: JsonObject, required: readonly string[], optional: readonly string[]): boolean {
  const keys = Object.keys(value);
  const allowed = new Set([...required, ...optional]);
  return required.every((key) => key in value) && keys.every((key) => allowed.has(key));
}

function canonicalJson(value: JsonValue | Readonly<Record<string, unknown>>): string {
  return JSON.stringify(sortJson(value));
}

function sortJson(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sortJson);
  }
  if (value !== null && typeof value === "object") {
    const source = value as Readonly<Record<string, unknown>>;
    const sorted: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
    for (const key of Object.keys(source).sort((left, right) => Buffer.from(left).compare(Buffer.from(right)))) {
      sorted[key] = sortJson(source[key]);
    }
    return sorted;
  }
  return value;
}

function stringArrayEquals(value: JsonValue | undefined, expected: readonly string[]): boolean {
  return Array.isArray(value) && value.length === expected.length && value.every((entry, index) => entry === expected[index]);
}

function sameStrings(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function hex(value: JsonValue | undefined, minimum: number, maximum: number): boolean {
  return typeof value === "string" && value.length >= minimum && value.length <= maximum && /^[0-9a-f]+$/.test(value);
}

function inertProtocolText(value: string): boolean {
  if (/[\u0000-\u001f\u007f-\u009f\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]/u.test(value)) {
    return false;
  }
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) {
        return false;
      }
      index += 1;
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      return false;
    }
  }
  return true;
}

function inertUnicode(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) {
        return false;
      }
      index += 1;
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      return false;
    }
  }
  return true;
}

function signal(child: ChildProcessWithoutNullStreams, name: NodeJS.Signals): void {
  try {
    child.kill(name);
  } catch {
    // Direct-child signalling only; descendant containment remains unknown.
  }
}

async function settlesWithin(promise: Promise<void>, milliseconds: number): Promise<boolean> {
  return await Promise.race([
    promise.then(() => true),
    new Promise<boolean>((resolve) => {
      const timer = setTimeout(() => resolve(false), milliseconds);
      timer.unref();
    }),
  ]);
}

function preserveLifecycleOr(error: unknown, code: McpFailureCode, message: string): McpFailure {
  if (error instanceof McpFailure && ["cancelled", "timed-out", "output-limit", "process-residue", "spawn-failed"].includes(error.code)) {
    return error;
  }
  return new McpFailure(code, message);
}
