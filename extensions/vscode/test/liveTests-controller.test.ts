import { stub } from "./vscode-stub.js";
import * as assert from "node:assert/strict";
import { mkdtemp, rm, writeFile, readFile, symlink, mkdir } from "node:fs/promises";
import { tmpdir } from "node:os";
import * as path from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { test } from "node:test";
import * as vscode from "vscode";
import type { ProcessGroupHandle } from "../src/process.js";
import { LiveTestController } from "../src/liveTests.js";

const running = (identity: string): string =>
  JSON.stringify({ profile: "corvint-go-live-session-event/0", state: "running", identity, sequence: 1, scope: ["s"], detail: "" });

test("a record the provider prints after stop() changes no test run (VSC-V0-066)", { skip: process.platform === "win32" }, async () => {
  const script = `late='${running("late")}'\ntrap 'printf "%s\\n" "$late"; exit 0' TERM\nprintf '%s\\n' '${running("early")}'\nwhile :; do sleep 1; done\n`;
  await withProvider(script, async (controller) => {
    await withHangDetector(until(() => stub.runs.length === 1), "provider never announced its first run");
    await controller.stop();
    assert.deepEqual(stub.runs.map((run) => run.name), ["Corvint live tests · early"]);
  });
});

test("shutdown() settles only after a SIGTERM-ignoring provider was killed (VSC-V0-059)", { skip: process.platform === "win32" }, async () => {
  await withProvider(`trap '' TERM\nprintf '${running("%s")}\\n' "$$"\nwhile :; do sleep 1; done\n`, async (controller) => {
    await withHangDetector(until(() => stub.runs.length === 1), "provider never reported its pid");
    const pid = Number(stub.runs[0]?.name.split(" · ")[1]);
    assert.ok(Number.isInteger(pid) && alive(pid), "provider pid was not reported alive");
    await withHangDetector(controller.shutdown(), "shutdown never settled");
    assert.equal(alive(pid), false, "shutdown settled while the provider was still alive");
  });
});

function jsResult(state: "PASSED" | "FAILED"): string {
  const axis = { state: "UNSUPPORTED", reason: "no-association-input", anchors: null };
  const projection = { association: axis, hygiene: axis,
    freshness: { state: "CURRENT", reason: "", anchors: ["input-identity:fixture"] },
    execution: { state, reason: state === "FAILED" ? "ASSERTION_OR_TEST" : "", anchors: ["fixture.test.js:1"] },
    strength: { state: "NOT_MEASURED", reason: "no-mutation-run", anchors: null } };
  return JSON.stringify({ receipt: { kind: "unit" }, testProjections: [{ name: "addition", state: state.toLowerCase(), projection }], runProjection: projection });
}

for (const state of ["PASSED", "FAILED"] as const) {
  test(`exit-zero JS ${state} result persists without a newline (VSC-V0-071)`, { skip: process.platform === "win32" }, async () => {
    await withProvider(`printf '%s' '${jsResult(state)}'`, async () => {
      await withHangDetector(until(() => stub.runs.some((run) => run.ended)), "provider never completed");
      // Let close and its cleanup settle before checking the retained projection.
      await delay(150);
      assert.equal(stub.runs.at(-1)?.name, "Corvint live tests · js-unit");
      assert.equal(stub.runs.at(-1)?.results[0]?.state, state.toLowerCase());
      assert.ok(stub.items.has("addition"));
      assert.equal(stub.diagnostics.values().next().value?.length, state === "FAILED" ? 1 : 0);
    });
  });
}

for (const [name, output, exit] of [
  ["empty", "", 0], ["malformed", "not json\n", 0], ["trailing", `${jsResult("PASSED")}garbage`, 0],
  ["duplicate", jsResult("PASSED") + jsResult("FAILED"), 0],
  ["mixed", `${running("mixed")}\n${jsResult("PASSED")}`, 0],
  ["invalid leading whitespace", `\u00a0${jsResult("PASSED")}\n`, 0],
  ["truncated", jsResult("PASSED").slice(0, -1), 0],
  ["overflow", `${"x".repeat(262_145)}\n${jsResult("PASSED")}`, 0],
  ["nonzero", jsResult("PASSED"), 7],
] as const) {
  test(`JS ${name} output cannot retain a passing result (VSC-V0-071)`, { skip: process.platform === "win32" }, async () => {
    // Write output separately so large framing cases are not shell command arguments.
    await withProvider(`while [ ! -f output ]; do sleep 0.01; done; cat output; exit ${exit}`, async (controller, directory) => {
      await writeFile(path.join(directory, "output"), output);
      await withHangDetector(until(() => controller.snapshot().phase === "unavailable"), "invalid provider output was accepted");
      assert.equal(controller.snapshot().documentDigest, null);
      assert.equal(stub.items.has("addition"), false);
    });
  });
}

test("normal exit-zero leader is cleaned up before its result is retained (VSC-V0-074)", { skip: process.platform === "win32" }, async () => {
  const script = `/bin/sh -c 'trap "" TERM; echo $$ > descendant.pid; exec >/dev/null; sleep 300' &
while [ ! -s descendant.pid ]; do sleep 0.01; done
printf '%s' '${jsResult("PASSED")}'`;
  await withProvider(script, async (controller, directory) => {
    await withHangDetector(until(() => controller.snapshot().phase === "completed"), "normal provider completion did not settle");
    const pid = Number(await readFile(path.join(directory, "descendant.pid"), "utf8"));
    assert.ok(Number.isInteger(pid) && pid > 1);
    assert.equal(alive(pid), false, "result published while a descendant still lived");
    const copy = controller.snapshot();
    (copy.items as { id: string; state: string }[]).length = 0;
    assert.ok(controller.snapshot().items.some((item) => item.id === "addition"));
    assert.match(copy.documentDigest ?? "", /^sha256:[a-f0-9]{64}$/);
  });
});

const countedProvider = `echo run >> run-count
if [ "$(cat source.js)" = fail ]; then printf '%s' '${jsResult("FAILED")}'; else printf '%s' '${jsResult("PASSED")}'; fi`;

test("rapid saves clear current state immediately and rerun once with latest inputs (VSC-V0-072/073)", { skip: process.platform === "win32" }, async () => {
  await withProvider(countedProvider, async (controller, directory) => {
    await completed(controller);
    await writeFile(path.join(directory, "source.js"), "fail");
    const uri = vscode.Uri.file(path.join(directory, "source.js"));
    const changes = [controller.onSavedDocument(uri), controller.onSavedDocument(uri), controller.onSavedDocument(uri)];
    assert.equal(controller.snapshot().phase, "pending");
    assert.equal(controller.snapshot().documentDigest, null);
    assert.equal(stub.items.has("addition"), false);
    await Promise.all(changes);
    await completed(controller);
    assert.equal(controller.snapshot().items.find((item) => item.id === "addition")?.state, "failed");
    assert.equal(await runCount(directory), 2);
    await writeFile(uri.fsPath, "pass");
    await controller.onSavedDocument(uri);
    await completed(controller);
    assert.equal(controller.snapshot().items.find((item) => item.id === "addition")?.state, "passed");
    assert.equal(await runCount(directory), 3);
  }, true);
});

test("save cancels an active provider, ignores its final output and launches only the replacement (VSC-V0-073)", { skip: process.platform === "win32" }, async () => {
  const script = `echo run >> run-count
echo $$ > provider.pid
if [ "$(cat source.js)" = pass ]; then
  trap 'printf "%s" \x27${jsResult("FAILED")}\x27; exit 0' TERM
  while :; do sleep 1; done
fi
printf '%s' '${jsResult("PASSED")}'`;
  await withProvider(script, async (controller, directory) => {
    await withHangDetector(until(() => { try { return stub.runs.length > 0; } catch { return false; } }), "provider did not start");
    await withHangDetector(until(async () => await runCount(directory) === 1), "provider never announced start");
    const pid = Number(await readFile(path.join(directory, "provider.pid"), "utf8"));
    await writeFile(path.join(directory, "source.js"), "replacement");
    await controller.onSavedDocument(vscode.Uri.file(path.join(directory, "source.js")));
    await completed(controller);
    assert.equal(alive(pid), false);
    assert.equal(controller.snapshot().items.find((item) => item.id === "addition")?.state, "passed");
    assert.equal(await runCount(directory), 2);
  }, true);
});

test("disable cancels a pending debounce and configuration changes use newest argv (VSC-V0-073)", { skip: process.platform === "win32" }, async () => {
  await withProvider(countedProvider, async (controller, directory) => {
    await completed(controller);
    await controller.onSavedDocument(vscode.Uri.file(path.join(directory, "source.js")));
    stub.settings.set("corvint.liveTests.enabled", false);
    await controller.applyConfiguration();
    await delay(350);
    assert.equal(await runCount(directory), 1);
    assert.equal(controller.snapshot().phase, "disabled");
    await writeFile(path.join(directory, "replacement.sh"), `printf '%s' '${jsResult("FAILED")}'`);
    stub.settings.set("corvint.liveTests.enabled", true);
    const older = controller.applyConfiguration();
    stub.settings.set("corvint.liveTests.command", ["/bin/sh", path.join(directory, "replacement.sh")]);
    await Promise.all([older, controller.applyConfiguration()]);
    await completed(controller);
    assert.equal(controller.snapshot().items.find((item) => item.id === "addition")?.state, "failed");
    assert.equal(await runCount(directory), 1);
    await controller.shutdown();
    await controller.onSavedDocument(vscode.Uri.file(path.join(directory, "source.js")));
    await delay(350);
    assert.equal(await runCount(directory), 1);
  }, true);
});

test("watch scope, trust and canonical containment exclude unrelated saves (VSC-V0-072)", { skip: process.platform === "win32" }, async () => {
  await withProvider(countedProvider, async (controller, directory) => {
    await completed(controller);
    await mkdir(path.join(directory, "node_modules"));
    await writeFile(path.join(directory, "node_modules", "generated.js"), "generated");
    await controller.onSavedDocument(vscode.Uri.file(path.join(directory, "node_modules", "generated.js")));
    await controller.onSavedDocument(vscode.Uri.file(path.join(directory, "..", "outside.js")));
    stub.trusted = false;
    await controller.onSavedDocument(vscode.Uri.file(path.join(directory, "source.js")));
    stub.trusted = true;
    await delay(350);
    assert.equal(await runCount(directory), 1);
    const external = await mkdtemp(path.join(tmpdir(), "corvint-watch-outside-"));
    try {
      await writeFile(path.join(external, "file.js"), "outside");
      await symlink(external, path.join(directory, "escape"));
      await controller.onSavedDocument(vscode.Uri.file(path.join(directory, "escape", "file.js")));
      assert.equal(controller.snapshot().phase, "unavailable");
      await delay(350);
      assert.equal(await runCount(directory), 1);
    } finally { await rm(external, { recursive: true, force: true }); }
  }, true);
});

test("toolchain paths are explicit, bounded and limited to enabled trusted providers (VSC-V0-076)", { skip: process.platform === "win32" }, async () => {
  await withProvider(countedProvider, async (controller, directory) => {
    await completed(controller);
    for (const invalid of [["relative"], ["/tmp/../bin"], ["/tmp:/bin"], ["/tmp\n"], new Array(17).fill("/bin"), "wrong type"]) {
      stub.settings.set("corvint.liveTests.toolchainPaths", invalid);
      await controller.applyConfiguration();
      assert.equal(controller.snapshot().phase, "unavailable");
    }
    assert.equal(await runCount(directory), 1);
    stub.settings.set("corvint.liveTests.enabled", false);
    await controller.applyConfiguration();
    assert.equal(controller.snapshot().phase, "disabled");
    stub.settings.set("corvint.liveTests.enabled", true);
    stub.trusted = false;
    await controller.applyConfiguration();
    assert.equal(controller.snapshot().phase, "disabled");
    stub.trusted = true;
    const tools = path.join(directory, "tools");
    await mkdir(tools);
    await writeFile(path.join(tools, "fixture-tool"), "#!/bin/sh\nprintf found", { mode: 0o700 });
    const script = path.join(directory, "path-provider.sh");
    await writeFile(script, `fixture-tool > tool-result\nprintf '%s' "$PATH" > tool-path\nprintf '%s' '${jsResult("PASSED")}'`);
    stub.settings.set("corvint.liveTests.command", ["/bin/sh", script]);
    stub.settings.set("corvint.liveTests.toolchainPaths", [tools, tools]);
    await controller.applyConfiguration();
    await completed(controller);
    assert.equal(await readFile(path.join(directory, "tool-result"), "utf8"), "found");
    assert.equal(await readFile(path.join(directory, "tool-path"), "utf8"), `${tools}:/usr/bin:/bin:/usr/sbin:/sbin`);
  }, true);
});

test("a provider basename cannot be discovered through toolchainPaths (VSC-V0-058/076)", { skip: process.platform === "win32" }, async () => {
  await withProvider(countedProvider, async (controller, directory) => {
    await completed(controller);
    stub.settings.set("corvint.liveTests.command", ["corvint-js-test-provider", "unit"]);
    stub.settings.set("corvint.liveTests.toolchainPaths", [directory]);
    await controller.applyConfiguration();
    assert.equal(controller.snapshot().phase, "unavailable");
    await delay(100);
    assert.equal(await runCount(directory), 1);
  }, true);
});

test("Go idle is enabled pending state rather than disabled (VSC-V0-075)", { skip: process.platform === "win32" }, async () => {
  const idle = JSON.stringify({ profile: "corvint-go-live-session-event/0", state: "idle", identity: "initial", sequence: 0, scope: [], detail: "" });
  await withProvider(`printf '%s\\n' '${idle}'; read input`, async (controller) => {
    await withHangDetector(until(() => controller.snapshot().provider === "go-session"), "idle not observed");
    assert.equal(controller.snapshot().phase, "pending");
    assert.equal(controller.snapshot().documentDigest, null);
  });
});

test("exit cleanup failure is visible even while descendant stdout remains open (VSC-V0-074)", { skip: process.platform === "win32" }, async () => {
  await assert.rejects(withProvider("sleep 300 &\nexit 0", async (controller) => {
    // Inject only the cleanup failure, retaining the real group handle for unconditional test cleanup.
    const handle = (controller as unknown as { handle: ProcessGroupHandle }).handle;
    assert.ok(handle);
    const realCleanup = handle.killGroup;
    (handle as { killGroup: () => Promise<void> }).killGroup = () => Promise.reject(new Error("injected cleanup refusal"));
    try {
      await withHangDetector(until(() => controller.snapshot().phase === "unavailable"), "exit cleanup failure stayed silent");
      assert.equal(controller.snapshot().documentDigest, null);
      await assert.rejects(controller.applyConfiguration(), /injected cleanup refusal/);
    } finally {
      (handle as { killGroup: () => Promise<void> }).killGroup = realCleanup;
      await realCleanup();
    }
  }), /injected cleanup refusal/);
});

function completed(controller: LiveTestController): Promise<void> {
  return withHangDetector(until(() => controller.snapshot().phase === "completed"), "provider never completed");
}

async function runCount(directory: string): Promise<number> {
  try { return (await readFile(path.join(directory, "run-count"), "utf8")).trim().split("\n").length; }
  catch { return 0; }
}

async function withProvider(script: string, body: (controller: LiveTestController, directory: string) => Promise<void>, automatic = false): Promise<void> {
  const directory = await mkdtemp(path.join(tmpdir(), "corvint-vscode-controller-"));
  const scriptPath = path.join(directory, automatic ? "corvint-js-test-provider" : "provider.sh");
  await writeFile(scriptPath, `#!/bin/sh\n${script}`, { encoding: "utf8", mode: 0o700 });
  stub.runs.length = 0;
  stub.items.clear();
  stub.diagnostics.clear();
  stub.settings.clear();
  stub.trusted = true;
  stub.settings.set("corvint.liveTests.enabled", true);
  stub.settings.set("corvint.liveTests.command", automatic ? [scriptPath, "unit"] : ["/bin/sh", scriptPath]);
  await writeFile(path.join(directory, "source.js"), "pass");
  const controller = new LiveTestController(directory);
  try {
    await controller.applyConfiguration();
    await body(controller, directory);
  } finally {
    try { await controller.shutdown(); }
    finally { await rm(directory, { recursive: true, force: true }); }
  }
}

// A hang detector, not a latency budget: each wait is event-driven.
const HANG_DETECTOR_MS = 60_000;

async function withHangDetector<T>(work: Promise<T>, message: string): Promise<T> {
  let timer: NodeJS.Timeout | undefined;
  const hang = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${message} (hang detector ${HANG_DETECTOR_MS} ms)`)), HANG_DETECTOR_MS);
  });
  try {
    return await Promise.race([work, hang]);
  } finally {
    clearTimeout(timer);
  }
}

async function until(predicate: () => boolean | Promise<boolean>): Promise<void> {
  const deadline = Date.now() + HANG_DETECTOR_MS;
  while (!await predicate()) {
    if (Date.now() >= deadline) throw new Error("condition did not settle before hang detector");
    await delay(20);
  }
}

function alive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}
