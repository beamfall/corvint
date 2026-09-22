import * as assert from "node:assert/strict";
import { mkdtemp, chmod, realpath, rm, symlink, writeFile, unlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import * as path from "node:path";
import { test } from "node:test";
import { fixedArguments, pinExecutable, revalidatePin } from "../src/executable.js";
import { decodeCorvintReceipt } from "../src/model.js";
import { ProcessFailure, runBoundedProcess } from "../src/process.js";

test("pin executes the real path but revalidates the configured symlink identity", async () => {
  const directory = await mkdtemp(path.join(tmpdir(), "corvint-vscode-pin-"));
  const first = path.join(directory, "build-one");
  const second = path.join(directory, "build-two");
  const candidate = path.join(directory, "corvint");
  try {
    await executable(first, "#!/bin/sh\nprintf 'Corvint 0.5.0a3 (build 12)\\n'\n");
    await executable(second, "#!/bin/sh\nprintf 'Corvint 0.5.0a3 (build 12)\\n'\n# replacement\n");
    await symlink(first, candidate);
    const pin = await pinExecutable(candidate, "corvint", directory, "test", "configuration");
    assert.equal(pin.candidatePath, candidate);
    assert.equal(pin.realPath, await realpath(first));
    assert.equal(await revalidatePin(pin), true);
    await unlink(candidate);
    await symlink(second, candidate);
    assert.equal(await revalidatePin(pin), false);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("version probe requires the build number (VSC-V0-007 PUB-V0-021)", async () => {
  const directory = await mkdtemp(path.join(tmpdir(), "corvint-vscode-build-"));
  const candidate = path.join(directory, "corvint");
  try {
    await executable(candidate, "#!/bin/sh\nprintf 'Corvint 0.5.0a3\\n'\n");
    await assert.rejects(pinExecutable(candidate, "corvint", directory, "test", "configuration"), /CLI_INCOMPATIBLE/);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("Corvint native argv and receipt (CRB-V0-013 VSC-V0-006 VSC-V0-007)", async () => {
  const directory = await mkdtemp(path.join(tmpdir(), "corvint-vscode-native-"));
  const candidate = path.join(directory, "corvint");
  const task = "project operations";
  const receipt = JSON.stringify(nativeReceipt(task));
  try {
    await executable(candidate, `#!/bin/sh
if [ "$1" = "--version" ]; then printf 'Corvint 0.5.0a3 (build 12)\\n'; exit 0; fi
[ "$1" = "--root" ] && [ "$2" = "${directory}" ] && [ "$3" = "query" ] && [ "$4" = "--task" ] && [ "$5" = "${task}" ] && [ "$6" = "--limit" ] && [ "$7" = "1" ] && [ "$#" = "7" ] || exit 19
printf '%s\\n' '${receipt}'
`);
    const pin = await pinExecutable(candidate, "corvint", directory, "test", "configuration");
    const argv = fixedArguments(pin.cliKind, "query", directory, task);
    assert.deepEqual(argv, ["--root", directory, "query", "--task", task, "--limit", "1"]);
    const result = await runBoundedProcess(pin.realPath, argv, {
      cwd: directory, env: {}, maxStdoutBytes: 262_144, maxStderrBytes: 65_536, timeoutMilliseconds: 2_000,
    });
    const snapshot = decodeCorvintReceipt(result.stdout, "query", 262_144, pin.cliKind, task);
    assert.equal(snapshot.revision, "a".repeat(40));
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("native argv retains repository-path rejection (VSC-V0-016 VSC-V0-040)", () => {
  for (const rejected of ["../escape.go", "./relative.go", "src//empty.go", "src\\windows.go", "/absolute.go"]) {
    assert.throws(() => fixedArguments("corvint", "impact", "/workspace", [rejected]), /PATH_REJECTED/);
  }
});

test("bounded process admits exact stdout bound and rejects one byte over", async () => {
  const exact = await runBoundedProcess(process.execPath, ["-e", "process.stdout.write('1234')"], {
    cwd: tmpdir(), env: {}, maxStdoutBytes: 4, maxStderrBytes: 16, timeoutMilliseconds: 2_000,
  });
  assert.equal(Buffer.from(exact.stdout).toString(), "1234");
  await assert.rejects(
    runBoundedProcess(process.execPath, ["-e", "process.stdout.write('12345')"], {
      cwd: tmpdir(), env: {}, maxStdoutBytes: 4, maxStderrBytes: 16, timeoutMilliseconds: 2_000,
    }),
    (error: unknown) => error instanceof ProcessFailure && error.code === "output-limit",
  );
});

test("pre-cancelled process is rejected without a successful result", async () => {
  const cancellation = {
    isCancellationRequested: true,
    onCancellationRequested: (_listener: () => void) => ({ dispose() {} }),
  };
  await assert.rejects(
    runBoundedProcess(process.execPath, ["-e", "process.stdout.write('unexpected')"], {
      cwd: tmpdir(), env: {}, maxStdoutBytes: 64, maxStderrBytes: 64, timeoutMilliseconds: 2_000, cancellation,
    }),
    (error: unknown) => error instanceof ProcessFailure && error.code === "cancelled",
  );
});

test("a descendant holding the output pipes open after termination settles as process residue", { skip: process.platform === "win32" }, async () => {
  await assert.rejects(
    runBoundedProcess("/bin/sh", ["-c", "sleep 10 & printf '{}\\n'"], {
      cwd: tmpdir(), env: { PATH: "/usr/bin:/bin" }, maxStdoutBytes: 64, maxStderrBytes: 64, timeoutMilliseconds: 1_000,
    }),
    (error: unknown) => error instanceof ProcessFailure && error.code === "process-residue",
  );
});

test("spawn failures never retain configured filesystem paths", async () => {
  const secretPath = path.join(tmpdir(), "secret-corvint-location", "corvint");
  await assert.rejects(
    runBoundedProcess(secretPath, [], {
      cwd: tmpdir(), env: {}, maxStdoutBytes: 64, maxStderrBytes: 64, timeoutMilliseconds: 2_000,
    }),
    (error: unknown) => error instanceof ProcessFailure && error.code === "spawn-failed" &&
      !error.message.includes(secretPath) && error.message === "Corvint process spawn failed",
  );
});

async function executable(file: string, source: string): Promise<void> {
  await writeFile(file, source, { encoding: "utf8", mode: 0o700 });
  await chmod(file, 0o700);
}

function nativeReceipt(task: string): object {
  const revision = "a".repeat(40);
  return {
    context: {
      schema_version: 1, mode: "query", request: { limit: 1, text: task }, revision,
      freshness: { state: "fresh", revision, mixed_path_count: 0 }, state: "READY",
      results: [], exclusions: { count: 0 }, verification: [],
      coverage: { within_budget: true, packet_bytes: 1, uncertainty: [], critical_missing: [] },
      abstention: { active: false, reason: "none" }, intent: {}, learning: {},
    },
    mutates: false, ok: true, tool: "query",
  };
}
