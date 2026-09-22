import * as assert from "node:assert/strict";
import { chmod, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import * as path from "node:path";
import { test } from "node:test";
import type { JsonObject } from "../src/json.js";
import { McpFailure, decodeCallResult, runMcpOperation } from "../src/mcp.js";
import { decodeContextReceipt } from "../src/model.js";
import { configuredMcpCandidate, pinMcpExecutable, revalidateMcpPin } from "../src/executable.js";

test("Corvint MCP pin keeps Corvint wire tools (CRB-V0-013 VSC-V0-047 VSC-V0-052)", async () => {
  await withFakeServer("success", async ({ executable, root }) => {
    const candidate = configuredMcpCandidate(executable);
    assert.equal(candidate.cliKind, "corvint-mcp");
    const pin = await pinMcpExecutable(candidate.path, candidate.cliKind, root, "test");
    assert.equal(await revalidateMcpPin(pin), true);
    const source = await runMcpOperation(pin.realPath, root, "query", "project operations", options(root));
    assert.ok(source.context);
    const snapshot = decodeContextReceipt(source.context, "query", 65_536, "corvint", "project operations", source.snapshotBytes);
    assert.equal(snapshot.state, "READY");
    assert.equal(snapshot.results[0]?.id, "AGENTS.md");
    assert.equal(source.authority.serverVersion, "0.1.0-experimental");
    assert.equal(source.authority.repository?.profileId, "generic");
  });
});

test("CRB-V0-016 VSC-V0-050 accepts the exact current MCP presentation pair", async () => {
  for (const mode of ["success"] as const) {
    await withFakeServer(mode, async ({ executable, root }) => {
      const pin = await pinMcpExecutable(executable, "corvint-mcp", root, "test");
      const source = await runMcpOperation(pin.realPath, root, "query", "project operations", options(root));
      assert.equal(source.authority.serverVersion, "0.1.0-experimental");
      assert.equal(source.authority.state, "READY");
      assert.equal(source.context?.state, "READY");
    });
  }
});

test("CRB-V0-016 VSC-V0-050 rejects unknown and drifting MCP presentation", async () => {
  for (const [mode, codes] of [
    ["unknown-pair", ["protocol", "incompatible"]],
    ["presentation-drift", ["toolset-mismatch"]],
    ["call-presentation-drift", ["protocol", "incompatible"]],
  ] as const) {
    await withFakeServer(mode, async ({ executable, root }) => {
      await assert.rejects(runMcpOperation(executable, root, "query", "project operations", options(root)),
        (error: unknown) => error instanceof McpFailure && (codes as readonly string[]).includes(error.code), mode);
    });
  }
});

test("MCP rejects reordered tools, input-required, hostile server version, and nonzero close", async () => {
  for (const [mode, code] of [
    ["reordered-tools", "toolset-mismatch"],
    ["input-required", "interaction-required"],
    ["hostile-version", "incompatible"],
    ["version-drift", "incompatible"],
    ["nonzero-close", "protocol"],
    ["closed-stdin", "spawn-failed"],
  ] as const) {
    await withFakeServer(mode, async ({ executable, root }) => {
      await assert.rejects(
        runMcpOperation(executable, root, "query", "project operations", options(root)),
        (error: unknown) => error instanceof McpFailure && error.code === code,
        mode,
      );
    });
  }
});

test("MCP cancellation sends bounded metadata for the observed in-flight request", async () => {
  await withFakeServer("cancel", async ({ executable, root, cancellationLog, callMarker }) => {
    const token = new MutableCancellation();
    const running = runMcpOperation(executable, root, "query", "project operations", { ...options(root), cancellation: token });
    const call = JSON.parse(await waitForFile(callMarker)) as { id?: number };
    assert.equal(typeof call.id, "number");
    token.cancel();
    await assert.rejects(running, (error: unknown) => error instanceof McpFailure && error.code === "cancelled");
    const notification = JSON.parse(await readFile(cancellationLog, "utf8")) as {
      method?: string;
      params?: { requestId?: number; _meta?: Record<string, unknown> };
    };
    assert.equal(notification.method, "notifications/cancelled");
    assert.equal(notification.params?.requestId, call.id);
    assert.deepEqual(notification.params?._meta?.["io.modelcontextprotocol/clientCapabilities"], {});
    assert.equal(notification.params?._meta?.["io.modelcontextprotocol/protocolVersion"], "2026-07-28");
  });
});

test("call-result decoder rejects a canonical-copy mismatch", () => {
  const structured = bridgeResult();
  const result = callResult(structured);
  const content = result.content as Array<{ text: string }>;
  const first = content[0];
  assert.ok(first);
  first.text = "{}";
  assert.throws(() => decodeCallResult(result, "query"), /canonical duplicates/);
});

test("MCP admits a closed null-receipt abstention and rejects authority laundering", () => {
  const base = bridgeResult();
  const nullAbstention: JsonObject = {
    ...base,
    state: "ABSTAINED",
    epistemicClass: "NOT_OBSERVED",
    authorityClass: "NONE",
    receipt: null,
    abstention: { active: true, reason: "UNSUPPORTED_INTENT" },
  };
  const admitted = decodeCallResult(callResult(nullAbstention), "query");
  assert.equal(admitted.context, undefined);
  assert.equal(admitted.authority.state, "ABSTAINED");
  assert.equal(admitted.authority.abstention.reason, "UNSUPPORTED_INTENT");

  const retained: JsonObject = {
    ...base,
    state: "ABSTAINED",
    epistemicClass: "NOT_OBSERVED",
    authorityClass: "NONE",
    abstention: { active: true, reason: "OUT_OF_SCOPE" },
  };
  assert.throws(() => decodeCallResult(callResult(retained), "query"), /retained abstention receipt/);
  assert.throws(() => decodeCallResult(callResult({ ...nullAbstention, abstention: { active: true, reason: "NONE" } }), "query"), /abstention authority/);

  const native = retained.receipt as JsonObject;
  const impactRetained: JsonObject = {
    ...retained,
    tool: "corvint.impact",
    receipt: { ...native, mode: "impact", request: { paths: ["src/a.go"], limit: 20 } },
  };
  assert.throws(() => decodeCallResult(callResult(impactRetained), "impact"), /retained abstention receipt/);
  const validImpact: JsonObject = {
    ...impactRetained,
    receipt: { ...(impactRetained.receipt as JsonObject), state: "OUT_OF_SCOPE" },
  };
  const impact = decodeCallResult(callResult(validImpact), "impact");
  assert.equal(impact.context?.state, "OUT_OF_SCOPE");
  assert.throws(() => decodeCallResult(callResult({ ...base, receipt: { ...(base.receipt as JsonObject), state: "OUT_OF_SCOPE" } }), "query"), /ready wrapper contradicts/);
});

test("repository object format, digest widths, and profile are correlated", () => {
  for (const override of [
    { objectFormat: "sha256" },
    { profileId: "private" },
    { dirtyPathsSha256: "b".repeat(40) },
  ]) {
    const structured = bridgeResult();
    const repository = structured.repository as JsonObject;
    assert.throws(() => decodeCallResult(callResult({ ...structured, repository: { ...repository, ...override } }), "query"), /repository binding/);
  }
});

test("MCP-only input is rejected before any process spawn", async () => {
  const missing = path.join(tmpdir(), "corvint-vscode-must-not-spawn", "corvint-mcp");
  for (const [operation, value] of [
    ["query", "   "],
    ["query", "Unicode π"],
    ["impact", ["../escape.go"]],
    ["impact", ["src/not-go.ts"]],
    ["impact", ["src/\ud800.go"]],
  ] as const) {
    await assert.rejects(
      runMcpOperation(missing, tmpdir(), operation, value, options(tmpdir())),
      (error: unknown) => error instanceof McpFailure && error.code === "input-unsupported",
    );
  }
});

function options(root: string) {
  return {
    cwd: root,
    env: { PATH: "/usr/bin:/bin:/usr/sbin:/sbin" },
    maxMessageBytes: 65_536,
    timeoutMilliseconds: 2_000,
    clientVersion: "9.8.7-test",
  } as const;
}

async function withFakeServer(
  mode: "success" | "legacy-success" | "mixed-current" | "mixed-legacy" | "unknown-pair" | "presentation-drift" | "call-presentation-drift" | "reordered-tools" | "input-required" | "hostile-version" | "version-drift" | "nonzero-close" | "closed-stdin" | "cancel",
  action: (fixture: { executable: string; root: string; cancellationLog: string; callMarker: string }) => Promise<void>,
): Promise<void> {
  const root = await mkdtemp(path.join(tmpdir(), "corvint-vscode-mcp-"));
  const executable = path.join(root, "corvint-mcp");
  const cancellationLog = path.join(root, "cancel.json");
  const callMarker = path.join(root, "call.json");
  try {
    await writeFile(executable, fakeServerSource(mode, root, cancellationLog, callMarker), { encoding: "utf8", mode: 0o700 });
    await chmod(executable, 0o700);
    await action({ executable, root, cancellationLog, callMarker });
  } finally {
    await rm(root, { recursive: true, force: true });
  }
}

function fakeServerSource(mode: string, root: string, cancellationLog: string, callMarker: string): string {
  const version = mode === "hostile-version" ? "bad\nversion" : "0.1.0-experimental";
  const meta = serverMeta(version, mode);
  const listMeta = mode === "version-drift" ? serverMeta("0.1.1-drift", mode)
    : mode === "presentation-drift" ? serverMeta(version, "unknown-pair") : meta;
  const callMeta = mode === "call-presentation-drift" ? serverMeta(version, "unknown-pair") : meta;
  const tools = toolRegistry();
  if (mode === "reordered-tools") {
    tools.reverse();
  }
  const responses = {
    discovery: {
      jsonrpc: "2.0", id: 1,
      result: { _meta: meta, cacheScope: "public", capabilities: { tools: { listChanged: false } }, resultType: "complete", supportedVersions: ["2026-07-28"], ttlMs: 0 },
    },
    tools: { jsonrpc: "2.0", id: 2, result: { _meta: listMeta, cacheScope: "private", resultType: "complete", tools, ttlMs: 300000 } },
    call: { jsonrpc: "2.0", id: 3, result: mode === "input-required"
      ? { _meta: meta, content: [], isError: false, resultType: "input_required", structuredContent: {} }
      : callResult(bridgeResult(), callMeta) },
  };
  return `#!${process.execPath}
const fs = require("node:fs");
const readline = require("node:readline");
const expectedRoot = ${JSON.stringify(root)};
const mode = ${JSON.stringify(mode)};
const responses = ${JSON.stringify(responses)};
if (process.argv.length !== 4 || process.argv[2] !== "--root" || process.argv[3] !== expectedRoot) process.exit(31);
if (mode === "closed-stdin") {
  // Answer discovery only after closing stdin, so the client's next request write fails with EPIPE.
  let input = Buffer.alloc(0);
  const chunk = Buffer.alloc(65536);
  while (!input.includes(10)) {
    const read = fs.readSync(0, chunk);
    if (read === 0) process.exit(34);
    input = Buffer.concat([input, chunk.subarray(0, read)]);
  }
  fs.closeSync(0);
  fs.writeSync(1, JSON.stringify(responses.discovery) + "\\n");
  setTimeout(() => process.exit(0), 5000);
  return;
}
const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
let step = 0;
rl.on("line", (line) => {
  const value = JSON.parse(line);
  if (value.method === "notifications/cancelled") {
    fs.writeFileSync(${JSON.stringify(cancellationLog)}, JSON.stringify(value));
    process.exit(0);
  }
  const meta = value.params && value.params._meta;
  if (!meta || meta["io.modelcontextprotocol/protocolVersion"] !== "2026-07-28" ||
      meta["io.modelcontextprotocol/clientInfo"].name !== "corvint-vscode" ||
      meta["io.modelcontextprotocol/clientInfo"].version !== "9.8.7-test" ||
      Object.keys(meta["io.modelcontextprotocol/clientCapabilities"]).length !== 0) process.exit(32);
  if (step === 0 && value.method === "server/discover") process.stdout.write(JSON.stringify(responses.discovery) + "\\n");
  else if (step === 1 && value.method === "tools/list") process.stdout.write(JSON.stringify(responses.tools) + "\\n");
  else if (step === 2 && value.method === "tools/call" && mode !== "cancel") process.stdout.write(JSON.stringify(responses.call) + "\\n");
  else if (step === 2 && value.method === "tools/call" && mode === "cancel") fs.writeFileSync(${JSON.stringify(callMarker)}, JSON.stringify({ id: value.id }));
  else process.exit(33);
  step += 1;
});
rl.on("close", () => process.exit(mode === "nonzero-close" ? 7 : 0));
`;
}

function serverMeta(version = "0.1.0-experimental", pair = "success"): JsonObject {
  const current = { description: "Local read-only Corvint context and repository evidence server.", name: "corvint-mcp", version };
  const info = pair === "unknown-pair" ? { ...current, name: "other-mcp", description: "Other server." }
    : current;
  return { "io.modelcontextprotocol/serverInfo": info };
}

function toolRegistry(): Array<Record<string, unknown>> {
  const annotations = { destructiveHint: false, idempotentHint: true, openWorldHint: false, readOnlyHint: true };
  const schema = (properties: Record<string, unknown>, required: string[]) => ({ "$schema": "https://json-schema.org/draft/2020-12/schema", additionalProperties: false, properties, required, type: "object" });
  return [
    { name: "corvint.impact", description: "Compile revision-bound impact evidence for tracked Go files without returning source bodies.", annotations,
      inputSchema: schema({ paths: { type: "array", minItems: 1, maxItems: 100, uniqueItems: true, items: { type: "string", minLength: 1, maxLength: 1024, pattern: "^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\\\).+[.]go$" } }, limit: { type: "integer", minimum: 1, maximum: 50, default: 10 } }, ["paths"]) },
    { name: "corvint.query", description: "Compile the narrow project-operations authority-start receipt (limit is fixed at one).", annotations,
      inputSchema: schema({ task: { type: "string", minLength: 1, maxLength: 2000, pattern: "^[ -~]*[!-~][ -~]*$" } }, ["task"]) },
    { name: "corvint.status", description: "Observe Git commit, tree, worktree state, and a privacy-preserving dirty-path digest.", annotations, inputSchema: schema({}, []) },
  ];
}

function bridgeResult(): JsonObject {
  const revision = "a".repeat(40);
  return {
    schema: "corvint-mcp-bridge-result/0", tool: "corvint.query", mutates: false, state: "READY",
    epistemicClass: "OBSERVED", authorityClass: "REPOSITORY_EVIDENCE",
    repository: { commitRevision: revision, treeRevision: revision, objectFormat: "sha1", profileId: "generic", worktreeState: "CLEAN", dirtyPathCount: 0, dirtyPathsSha256: "b".repeat(64) },
    receipt: {
      schema_version: 1, mode: "query", request: { text: "project operations", limit: 1 }, revision,
      freshness: { state: "fresh", revision, mixed_path_count: 0 }, state: "READY",
      results: [{ id: "AGENTS.md", kind: "path", summary: "root authority", evidence: [{ path: "AGENTS.md", line: 1, blob_hash: "c".repeat(40), reason: "authority", confidence: "high", authority: "repository" }] }],
      exclusions: { count: 0 }, verification: [], coverage: { within_budget: true, packet_bytes: 1, uncertainty: [], critical_missing: [] },
      abstention: { active: false, reason: "none" }, intent: {}, learning: {},
    },
    abstention: { active: false, reason: "NONE" },
  };
}

function callResult(structured: JsonObject, meta = serverMeta()): JsonObject {
  return { _meta: meta, content: [{ type: "text", text: canonical(structured) }], isError: false, resultType: "complete", structuredContent: structured };
}

function canonical(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value !== null && typeof value === "object") {
    const record = value as Record<string, unknown>;
    return `{${Object.keys(record).sort().map((key) => `${JSON.stringify(key)}:${canonical(record[key])}`).join(",")}}`;
  }
  return JSON.stringify(value);
}

class MutableCancellation {
  isCancellationRequested = false;
  private readonly listeners = new Set<() => void>();

  onCancellationRequested(listener: () => void) {
    this.listeners.add(listener);
    return { dispose: () => this.listeners.delete(listener) };
  }

  cancel(): void {
    this.isCancellationRequested = true;
    for (const listener of this.listeners) listener();
  }
}

async function waitForFile(file: string): Promise<string> {
  const deadline = Date.now() + 2_000;
  while (Date.now() < deadline) {
    try {
      return await readFile(file, "utf8");
    } catch {
      await new Promise<void>((resolve) => setTimeout(resolve, 10));
    }
  }
  throw new Error("fake MCP server did not observe tools/call");
}
