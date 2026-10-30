import * as assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile, chmod } from "node:fs/promises";
import { tmpdir } from "node:os";
import * as path from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { test } from "node:test";
import { spawnProcessGroup } from "../src/process.js";

test("corvint.liveTests is disabled by default in the extension manifest", async () => {
  const manifestPath = path.join(__dirname, "..", "..", "package.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf-8")) as {
    contributes: { configuration: { properties: Record<string, { default: unknown }> } };
  };
  const properties = manifest.contributes.configuration.properties;
  assert.equal(properties["corvint.liveTests.enabled"]?.default, false);
  assert.deepEqual(properties["corvint.liveTests.command"]?.default, []);
  assert.equal(properties["corvint.liveTests.retainEvidence"]?.default, false);
});

test("killGroup terminates the provider and every descendant it started, on POSIX", { skip: process.platform === "win32" }, async () => {
  const directory = await mkdtemp(path.join(tmpdir(), "corvint-vscode-livetests-"));
  const script = path.join(directory, "provider.sh");
  let handle: ReturnType<typeof spawnProcessGroup> | undefined;
  try {
    await writeFile(script, "#!/bin/sh\nsleep 300 &\necho $!\nwait\n", { encoding: "utf8", mode: 0o700 });
    await chmod(script, 0o700);
    const provider = spawnProcessGroup("/bin/sh", [script], { cwd: directory, env: { PATH: "/usr/bin:/bin" } });
    handle = provider;
    // "close" fires only after the provider exits and every holder of its stdout
    // pipe, including the backgrounded descendant, has closed it.
    const closed = new Promise<void>((resolve) => provider.child.once("close", () => resolve()));

    const descendantPid = Number(await withHangDetector(firstLine(provider.child.stdout), "provider never reported its descendant pid"));
    assert.ok(Number.isInteger(descendantPid), "provider reported a non-integer descendant pid");
    const providerPid = provider.child.pid;
    assert.ok(providerPid !== undefined);
    assert.equal(alive(providerPid), true);
    assert.equal(alive(descendantPid), true);

    await provider.killGroup();
    await withHangDetector(closed, "provider or descendant kept the provider stdout open after killGroup");
    // The reparented descendant can remain a zombie briefly after it closes the pipe.
    await withHangDetector(until(() => !alive(providerPid) && !alive(descendantPid)), "provider or descendant was never reaped after killGroup");

    assert.equal(alive(providerPid), false, "provider process survived killGroup");
    assert.equal(alive(descendantPid), false, "descendant process survived killGroup");
  } finally {
    // The descendant outlives the hang detector so a no-op killGroup cannot pass;
    // this idempotent call cleans up only when the test body never reached killGroup.
    await handle?.killGroup();
    await rm(directory, { recursive: true, force: true });
  }
});

test("killGroup kills a SIGTERM-ignoring descendant after the provider itself exited on SIGTERM, on POSIX", { skip: process.platform === "win32" }, async () => {
  // The descendant reports its pid only once it ignores SIGTERM, then releases the provider stdout pipe,
  // so the provider's "close" fires as soon as the provider itself exits.
  const provider = spawnProcessGroup("/bin/sh", ["-c", "/bin/sh -c 'trap \"\" TERM; echo $$; exec sleep 300 >/dev/null 2>&1' & wait"], {
    cwd: tmpdir(),
    env: { PATH: "/usr/bin:/bin" },
  });
  let descendantPid: number | undefined;
  try {
    descendantPid = Number(await withHangDetector(firstLine(provider.child.stdout), "provider never reported its descendant pid"));
    assert.ok(Number.isInteger(descendantPid) && alive(descendantPid), "descendant was not reported alive");
    await provider.killGroup();
    const pid = descendantPid;
    await withHangDetector(until(() => !alive(pid)), "SIGTERM-ignoring descendant survived killGroup");
  } finally {
    if (descendantPid !== undefined && alive(descendantPid)) process.kill(descendantPid, "SIGKILL");
    await provider.killGroup();
  }
});

test("a provider that floods stderr past the pipe buffer still delivers its stdout records", { skip: process.platform === "win32" }, async () => {
  const provider = spawnProcessGroup("/bin/sh", ["-c", "head -c 1048576 /dev/zero >&2; echo ready; exec sleep 300"], {
    cwd: tmpdir(),
    env: { PATH: "/usr/bin:/bin" },
  });
  try {
    assert.equal(await withHangDetector(firstLine(provider.child.stdout), "provider stdout stalled behind an undrained stderr pipe"), "ready");
  } finally {
    await provider.killGroup();
  }
});

// A hang detector, not a latency budget: every wait above is event-driven and
// normally settles in milliseconds; this only turns a genuine hang into a failure.
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

function firstLine(stream: NodeJS.ReadableStream): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    let text = "";
    stream.setEncoding("utf8");
    stream.on("data", (chunk: string) => {
      text += chunk;
      const newline = text.indexOf("\n");
      if (newline >= 0) resolve(text.slice(0, newline).trim());
    });
    stream.once("end", () => reject(new Error("provider stdout ended before a full line")));
  });
}

async function until(predicate: () => boolean): Promise<void> {
  while (!predicate()) {
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

test("foreground stdin stays open and concurrent cleanup waits for EOF shutdown (VSC-V0-076)", { skip: process.platform === "win32" }, async () => {
  const directory = await mkdtemp(path.join(tmpdir(), "corvint-vscode-eof-"));
  const handle = spawnProcessGroup("/bin/sh", ["-c", "echo $$; read input; echo eof > stopped"], { cwd: directory, env: { PATH: "/usr/bin:/bin" } });
  try {
    const pid = Number(await withHangDetector(firstLine(handle.child.stdout), "EOF helper never started"));
    await delay(50);
    assert.equal(alive(pid), true, "stdin was closed before explicit cleanup");
    const first = handle.killGroup();
    assert.equal(handle.killGroup(), first, "concurrent cleanup must share one promise");
    await first;
    assert.equal(alive(pid), false);
    assert.equal((await readFile(path.join(directory, "stopped"), "utf8")).trim(), "eof");
  } finally {
    await handle.killGroup();
    await rm(directory, { recursive: true, force: true });
  }
});
