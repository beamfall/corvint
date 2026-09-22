import { execFile, spawn, type ChildProcessByStdio, type ChildProcessWithoutNullStreams } from "node:child_process";
import type { Readable, Writable } from "node:stream";

export interface CancellationSubscription {
  dispose(): void;
}

export interface CancellationLike {
  readonly isCancellationRequested: boolean;
  onCancellationRequested(listener: () => void): CancellationSubscription;
}

export interface ProcessOptions {
  readonly cwd: string;
  readonly env: Readonly<Record<string, string>>;
  readonly maxStdoutBytes: number;
  readonly maxStderrBytes: number;
  readonly timeoutMilliseconds: number;
  readonly cancellation?: CancellationLike;
}

export interface ProcessResult {
  readonly stdout: Uint8Array;
  readonly stderr: Uint8Array;
  readonly exitCode: number;
}

export type ProcessFailureCode =
  | "cancelled"
  | "output-limit"
  | "process-residue"
  | "spawn-failed"
  | "timed-out"
  | "nonzero-exit";

export class ProcessFailure extends Error {
  constructor(
    readonly code: ProcessFailureCode,
    message: string,
    readonly stderr: Uint8Array = new Uint8Array(),
  ) {
    super(message);
    this.name = "ProcessFailure";
  }
}

export async function runBoundedProcess(
  executable: string,
  args: readonly string[],
  options: ProcessOptions,
): Promise<ProcessResult> {
  validateOptions(executable, args, options);
  if (options.cancellation?.isCancellationRequested === true) {
    throw new ProcessFailure("cancelled", "Corvint command was cancelled before spawn");
  }
  return await new Promise<ProcessResult>((resolve, reject) => {
    let child: ChildProcessWithoutNullStreams;
    try {
      child = spawn(executable, [...args], {
        cwd: options.cwd,
        env: { ...options.env },
        shell: false,
        windowsHide: true,
        detached: false,
        stdio: ["pipe", "pipe", "pipe"],
      });
    } catch {
      reject(new ProcessFailure("spawn-failed", "Corvint process spawn failed"));
      return;
    }

    child.stdin.end();
    const stdout: Buffer[] = [];
    const stderr: Buffer[] = [];
    let stdoutBytes = 0;
    let stderrBytes = 0;
    let settled = false;
    let terminalFailure: ProcessFailure | undefined;
    let forceKillTimer: NodeJS.Timeout | undefined;

    const terminate = (failure: ProcessFailure): void => {
      if (settled || terminalFailure !== undefined) {
        return;
      }
      terminalFailure = failure;
      signalDirectChild(child, "SIGTERM");
      forceKillTimer = setTimeout(() => {
        signalDirectChild(child, "SIGKILL");
        // A descendant that inherited stdout/stderr keeps "close" from firing after the
        // direct child is gone; settle as observed residue instead of waiting on it.
        forceKillTimer = setTimeout(settleResidue, 250);
        forceKillTimer.unref();
      }, 250);
      forceKillTimer.unref();
    };

    const settleResidue = (): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timeout);
      subscription?.dispose();
      child.stdout.destroy();
      child.stderr.destroy();
      reject(new ProcessFailure("process-residue", "Corvint process output stayed open after termination", Buffer.concat(stderr)));
    };

    const collect = (
      target: Buffer[],
      chunk: Buffer | string,
      stream: "stdout" | "stderr",
    ): void => {
      const bytes = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
      if (stream === "stdout") {
        stdoutBytes += bytes.byteLength;
      } else {
        stderrBytes += bytes.byteLength;
      }
      const limit = stream === "stdout" ? options.maxStdoutBytes : options.maxStderrBytes;
      const total = stream === "stdout" ? stdoutBytes : stderrBytes;
      if (total > limit) {
        terminate(new ProcessFailure("output-limit", `Corvint ${stream} exceeded its byte bound`));
        return;
      }
      target.push(bytes);
    };

    child.stdout.on("data", (chunk: Buffer | string) => collect(stdout, chunk, "stdout"));
    child.stderr.on("data", (chunk: Buffer | string) => collect(stderr, chunk, "stderr"));
    child.on("error", () => {
      terminate(new ProcessFailure("spawn-failed", "Corvint process spawn failed"));
    });

    const timeout = setTimeout(() => {
      terminate(new ProcessFailure("timed-out", "Corvint command timed out"));
    }, options.timeoutMilliseconds);
    timeout.unref();

    const subscription = options.cancellation?.onCancellationRequested(() => {
      terminate(new ProcessFailure("cancelled", "Corvint command was cancelled"));
    });
    if (options.cancellation?.isCancellationRequested === true) {
      terminate(new ProcessFailure("cancelled", "Corvint command was cancelled"));
    }

    child.on("close", (code) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timeout);
      if (forceKillTimer !== undefined) {
        clearTimeout(forceKillTimer);
      }
      subscription?.dispose();
      const stderrBytes = Buffer.concat(stderr);
      if (terminalFailure !== undefined) {
        reject(new ProcessFailure(terminalFailure.code, terminalFailure.message, stderrBytes));
        return;
      }
      if (code === null || code !== 0) {
        reject(new ProcessFailure("nonzero-exit", `Corvint exited with code ${String(code)}`, stderrBytes));
        return;
      }
      resolve({ stdout: Buffer.concat(stdout), stderr: stderrBytes, exitCode: code });
    });
  });
}

function signalDirectChild(child: ChildProcessWithoutNullStreams, signal: NodeJS.Signals): void {
  try {
    child.kill(signal);
  } catch {
    // The direct child already exited. This is signalling only, not descendant containment.
  }
}

export interface ProcessGroupOptions {
  readonly cwd: string;
  readonly env: Readonly<Record<string, string>>;
}

export interface ProcessGroupHandle {
  /**
   * The foreground provider process. Reads JSON records from its stdout.
   * Its stderr is not piped back: an undrained pipe would stall the provider
   * once the OS pipe buffer fills, and provider stderr is never displayed.
   */
  readonly child: ChildProcessByStdio<Writable, Readable, null>;
  /**
   * Terminates the whole process group the provider started (itself plus any
   * descendants it spawned), not only the direct child. On POSIX this sends
   * the signal to the negated pid (the process group id, since the child is
   * spawned detached as its own group leader); on Windows it runs
   * `taskkill /t /f` over the process tree. Concurrent and repeated calls await the same cleanup promise.
   */
  killGroup(): Promise<void>;
}

/**
 * Spawns a long-running foreground provider process as its own process
 * group leader so its whole descendant tree can be terminated at once (see
 * killGroup). Unlike runBoundedProcess, stdout is not buffered or bounded
 * here — the caller streams NDJSON lines off `child.stdout` as they arrive.
 */
export function spawnProcessGroup(
  executable: string,
  args: readonly string[],
  options: ProcessGroupOptions,
): ProcessGroupHandle {
  if (executable.length === 0 || executable.includes("\0")) {
    throw new ProcessFailure("spawn-failed", "invalid executable path");
  }
  if (args.length > 512 || args.some((argument) => argument.includes("\0") || argument.length > 16_384)) {
    throw new ProcessFailure("spawn-failed", "invalid provider argument vector");
  }
  const child = spawn(executable, [...args], {
    cwd: options.cwd,
    env: { ...options.env },
    shell: false,
    windowsHide: true,
    detached: process.platform !== "win32",
    stdio: ["pipe", "pipe", "ignore"],
  });
  // An EOF-aware foreground provider owns this pipe until cleanup; errors never bypass group inspection.
  child.stdin.on("error", () => undefined);
  let cleanup: Promise<void> | undefined;
  const killGroup = (): Promise<void> => {
    cleanup ??= (async () => {
      const pid = child.pid;
      if (pid === undefined) return;
      child.stdin.end();
      await waitForChildExit(child, 200);
      if (process.platform === "win32") {
        await new Promise<void>((resolve, reject) => {
          execFile("taskkill", ["/pid", String(pid), "/t", "/f"], { timeout: SIGTERM_GRACE_MS + SIGKILL_GRACE_MS }, (error) => {
            if (error !== null) reject(new ProcessFailure("process-residue", "provider tree cleanup could not be verified"));
            else resolve();
          });
        });
        return;
      }
      signalGroup(pid, "SIGTERM");
      await waitForChildExit(child, SIGTERM_GRACE_MS);
      // Sent even after a normal leader exit; descendants may have closed stdout or ignored TERM.
      signalGroup(pid, "SIGKILL");
      const deadline = Date.now() + SIGKILL_GRACE_MS;
      while (groupPresent(pid)) {
        if (Date.now() >= deadline) throw new ProcessFailure("process-residue", "provider process group survived cleanup deadline");
        await new Promise<void>((resolve) => setTimeout(resolve, 20));
      }
    })();
    return cleanup;
  };
  return { child, killGroup };
}

const SIGTERM_GRACE_MS = 2_000;
const SIGKILL_GRACE_MS = 1_000;

/**
 * Waits for the child's actual `close` event (fired once its stdio streams
 * have finished, after `exit`) instead of a fixed sleep, so killGroup gives
 * the provider up to `timeoutMs` to exit on SIGTERM before it signals the
 * group with SIGKILL. Resolves true if the child had already exited or
 * exits before the bound; false if the bound elapses first.
 */
function waitForChildExit(child: ChildProcessByStdio<Writable, Readable, null>, timeoutMs: number): Promise<boolean> {
  if (child.exitCode !== null || child.signalCode !== null) {
    return Promise.resolve(true);
  }
  return new Promise<boolean>((resolve) => {
    let settled = false;
    const onClose = (): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      resolve(true);
    };
    const timer = setTimeout(() => {
      if (settled) {
        return;
      }
      settled = true;
      child.removeListener("close", onClose);
      resolve(false);
    }, timeoutMs);
    timer.unref();
    child.once("close", onClose);
  });
}

function groupPresent(pid: number): boolean {
  try {
    process.kill(-pid, 0);
    return true;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ESRCH") return false;
    throw new ProcessFailure("process-residue", "provider process group could not be inspected");
  }
}

function signalGroup(pid: number, signal: NodeJS.Signals): void {
  try {
    process.kill(-pid, signal);
  } catch {
    // The group already exited. This is signalling only.
  }
}

function validateOptions(
  executable: string,
  args: readonly string[],
  options: ProcessOptions,
): void {
  if (executable.length === 0 || executable.includes("\0")) {
    throw new ProcessFailure("spawn-failed", "invalid executable path");
  }
  if (args.length > 512 || args.some((argument) => argument.includes("\0") || argument.length > 16_384)) {
    throw new ProcessFailure("spawn-failed", "invalid Corvint argument vector");
  }
  if (!Number.isSafeInteger(options.maxStdoutBytes) || options.maxStdoutBytes < 1) {
    throw new ProcessFailure("spawn-failed", "invalid stdout byte bound");
  }
  if (!Number.isSafeInteger(options.maxStderrBytes) || options.maxStderrBytes < 1) {
    throw new ProcessFailure("spawn-failed", "invalid stderr byte bound");
  }
  if (!Number.isSafeInteger(options.timeoutMilliseconds) || options.timeoutMilliseconds < 1) {
    throw new ProcessFailure("spawn-failed", "invalid timeout");
  }
}
