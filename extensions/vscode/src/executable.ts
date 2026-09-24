import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { access, lstat, realpath, stat } from "node:fs/promises";
import * as path from "node:path";
import { constants } from "node:fs";
import { runBoundedProcess, type CancellationLike } from "./process.js";
import { isRepositoryRelativePath } from "./model.js";

const MAX_EXECUTABLE_BYTES = 256 * 1024 * 1024;
const STDERR_LIMIT = 65_536;
const VERSION_TOKEN = "((?:0|[1-9][0-9]*)\\.(?:0|[1-9][0-9]*)\\.(?:0|[1-9][0-9]*)(?:(?:a|b|rc)(?:0|[1-9][0-9]*)|-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?(?:\\+[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?)";
const VERSION_PATTERNS: Readonly<Record<CliKind, RegExp>> = {
  corvint: new RegExp(`^Corvint ${VERSION_TOKEN} \\(build (?:0|[1-9][0-9]*)\\)\\n$`),
};

export type CliKind = "corvint";
export type McpCliKind = "corvint-mcp";
export type ExecutableKind = CliKind | McpCliKind;
export type CandidateSource = "configuration" | "path";

export interface ExecutablePin {
  readonly cliKind: CliKind;
  readonly candidatePath: string;
  readonly realPath: string;
  readonly sha256: string;
  readonly size: number;
  readonly mode: number;
  readonly device: string;
  readonly fileId: string;
  readonly version: string;
  readonly workspaceRoot: string;
  readonly extensionHost: string;
  readonly source: CandidateSource;
}

interface Fingerprint<Kind extends ExecutableKind = ExecutableKind> {
  readonly cliKind: Kind;
  readonly candidatePath: string;
  readonly realPath: string;
  readonly sha256: string;
  readonly size: number;
  readonly mode: number;
  readonly device: string;
  readonly fileId: string;
}

export interface McpExecutablePin {
  readonly cliKind: McpCliKind;
  readonly candidatePath: string;
  readonly realPath: string;
  readonly sha256: string;
  readonly size: number;
  readonly mode: number;
  readonly device: string;
  readonly fileId: string;
  readonly workspaceRoot: string;
  readonly extensionHost: string;
  readonly source: "configuration";
}

export function configuredCandidate(value: string): { path: string; cliKind: CliKind } {
  if (value.length === 0 || value.includes("\0") || value.includes("\n") || value.includes("\r")) {
    throw new Error("CLI_INCOMPATIBLE: executable path is empty or contains a control character");
  }
  if (!path.isAbsolute(value)) {
    throw new Error("CLI_INCOMPATIBLE: executable path must be absolute");
  }
  const base = path.basename(value).replace(/\.exe$/i, "");
  if (base !== "corvint") {
    throw new Error("CLI_INCOMPATIBLE: executable basename must be corvint");
  }
  return { path: value, cliKind: base };
}

export function configuredMcpCandidate(value: string): { path: string; cliKind: McpCliKind } {
  if (value.length === 0 || value.includes("\0") || value.includes("\n") || value.includes("\r")) {
    throw new Error("CLI_INCOMPATIBLE: MCP executable path is empty or contains a control character");
  }
  if (!path.isAbsolute(value)) {
    throw new Error("CLI_INCOMPATIBLE: MCP executable path must be absolute");
  }
  const base = path.basename(value).replace(/\.exe$/i, "");
  if (base !== "corvint-mcp") {
    throw new Error("CLI_INCOMPATIBLE: MCP executable basename must be corvint-mcp");
  }
  return { path: value, cliKind: base };
}

export async function discoverPathCandidates(pathValue: string | undefined): Promise<readonly { path: string; cliKind: CliKind }[]> {
  if (pathValue === undefined || pathValue.length > 32_768) {
    return [];
  }
  const directories = pathValue.split(path.delimiter).filter((entry) => entry.length > 0 && path.isAbsolute(entry)).slice(0, 128);
  const suffix = process.platform === "win32" ? ".exe" : "";
  const found: { path: string; cliKind: CliKind }[] = [];
  for (const cliKind of ["corvint"] as const) {
    for (const directory of directories) {
      const candidate = path.join(directory, cliKind + suffix);
      try {
        await lstat(candidate);
        found.push({ path: candidate, cliKind });
      } catch {
        // Missing candidates are expected during the single advisory PATH inspection.
      }
    }
  }
  return found;
}

export async function pinExecutable(
  candidatePath: string,
  expectedKind: CliKind,
  workspaceRoot: string,
  extensionHost: string,
  source: CandidateSource,
  cancellation?: CancellationLike,
): Promise<ExecutablePin> {
  const before = await fingerprint(candidatePath, expectedKind);
  const result = await runBoundedProcess(before.realPath, ["--version"], {
    cwd: workspaceRoot,
    env: minimumEnvironment(),
    maxStdoutBytes: 4_096,
    maxStderrBytes: STDERR_LIMIT,
    timeoutMilliseconds: 2_000,
    ...(cancellation === undefined ? {} : { cancellation }),
  });
  const versionOutput = new TextDecoder("utf-8", { fatal: true }).decode(result.stdout);
  const matched = VERSION_PATTERNS[expectedKind].exec(versionOutput);
  if (matched?.[1] === undefined || matched[1].length > 64 || matched[1] !== "0.8.1" || result.stderr.byteLength !== 0) {
    throw new Error(`CLI_INCOMPATIBLE: version probe did not match the ${expectedKind} identity`);
  }
  const after = await fingerprint(candidatePath, expectedKind);
  if (!equalFingerprint(before, after)) {
    throw new Error("PIN_DRIFT: executable changed during version probe");
  }
  return { ...after, version: matched[1], workspaceRoot, extensionHost, source };
}

export async function revalidatePin(pin: ExecutablePin): Promise<boolean> {
  try {
    return equalFingerprint(pin, await fingerprint(pin.candidatePath, pin.cliKind));
  } catch {
    return false;
  }
}

export function fixedArguments(_cliKind: CliKind, operation: "query" | "impact", root: string, value: string | readonly string[]): string[] {
  if (operation === "query") {
    if (typeof value !== "string") throw new Error("CLI_FAILED: invalid query input");
    return ["--root", root, "query", "--task", value, "--limit", "1"];
  }
  if (typeof value === "string" || value.length < 1 || value.length > 256) {
    throw new Error("PATH_REJECTED: impact requires bounded repository-relative paths");
  }
  const paths = [...new Set(value)].sort((left, right) => Buffer.from(left).compare(Buffer.from(right)));
  if (paths.some((entry) => !isRepositoryRelativePath(entry))) {
    throw new Error("PATH_REJECTED: invalid impact path");
  }
  return ["--root", root, "impact", "--limit", "20", "--", ...paths];
}

export async function pinMcpExecutable(
  candidatePath: string,
  expectedKind: McpCliKind,
  workspaceRoot: string,
  extensionHost: string,
): Promise<McpExecutablePin> {
  const fingerprinted = await fingerprint(candidatePath, expectedKind);
  return { ...fingerprinted, workspaceRoot, extensionHost, source: "configuration" };
}

export async function revalidateMcpPin(pin: McpExecutablePin): Promise<boolean> {
  try {
    return equalFingerprint(pin, await fingerprint(pin.candidatePath, pin.cliKind));
  } catch {
    return false;
  }
}

export function minimumEnvironment(source: NodeJS.ProcessEnv = process.env): Readonly<Record<string, string>> {
  const result: Record<string, string> = {
    GIT_TERMINAL_PROMPT: "0",
    LANG: "C.UTF-8",
    LC_ALL: "C.UTF-8",
  };
  if (process.platform === "win32") {
    const systemRoot = source.SystemRoot;
    if (systemRoot !== undefined && path.isAbsolute(systemRoot)) {
      result.SystemRoot = systemRoot;
    }
  } else {
    result.PATH = "/usr/bin:/bin:/usr/sbin:/sbin";
  }
  for (const name of ["HOME", "TMPDIR"] as const) {
    const value = source[name];
    if (value !== undefined && path.isAbsolute(value) && !value.includes("\0")) {
      result[name] = value;
    }
  }
  return Object.freeze(result);
}

async function fingerprint<Kind extends ExecutableKind>(candidatePath: string, expectedKind: Kind): Promise<Fingerprint<Kind>> {
  const link = await lstat(candidatePath, { bigint: true });
  if (!link.isFile() && !link.isSymbolicLink()) {
    throw new Error("CLI_INCOMPATIBLE: candidate is not a file or symlink");
  }
  const resolved = await realpath(candidatePath);
  if (!path.isAbsolute(resolved) || !path.isAbsolute(path.dirname(resolved))) {
    throw new Error("CLI_INCOMPATIBLE: real executable path is not absolute");
  }
  const info = await stat(resolved, { bigint: true });
  if (!info.isFile() || info.size > BigInt(MAX_EXECUTABLE_BYTES)) {
    throw new Error("CLI_INCOMPATIBLE: executable is not a bounded regular file");
  }
  if (process.platform !== "win32") {
    await access(resolved, constants.R_OK | constants.X_OK);
  } else {
    await access(resolved, constants.R_OK);
  }
  const base = path.basename(candidatePath).replace(/\.exe$/i, "");
  if (base !== expectedKind) {
    throw new Error("CLI_INCOMPATIBLE: executable kind changed");
  }
  return {
    cliKind: expectedKind,
    candidatePath,
    realPath: resolved,
    sha256: await hashFile(resolved),
    size: Number(info.size),
    mode: Number(info.mode),
    device: info.dev.toString(10),
    fileId: info.ino.toString(10),
  };
}

async function hashFile(filePath: string): Promise<string> {
  const hash = createHash("sha256");
  await new Promise<void>((resolve, reject) => {
    const stream = createReadStream(filePath, { highWaterMark: 64 * 1024 });
    stream.on("data", (chunk: Buffer | string) => hash.update(chunk));
    stream.on("error", reject);
    stream.on("end", resolve);
  });
  return hash.digest("hex");
}

function equalFingerprint(left: Fingerprint, right: Fingerprint): boolean {
  return left.cliKind === right.cliKind && left.candidatePath === right.candidatePath && left.realPath === right.realPath && left.sha256 === right.sha256 &&
    left.size === right.size && left.mode === right.mode && left.device === right.device && left.fileId === right.fileId;
}
