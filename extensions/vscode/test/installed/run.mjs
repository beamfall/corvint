import { constants } from "node:fs";
import { access, chmod, cp, mkdtemp, mkdir, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";
import { dirname, isAbsolute, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const execFileAsync = promisify(execFile);
const here = dirname(fileURLToPath(import.meta.url));
const repository = resolve(here, "../../../..");
const fixtureSource = join(repository, "conformance/interactive-alpha/fixture");
const extensionDevelopmentPath = join(here, "driver");
const vscodeExecutablePath = "/Applications/Visual Studio Code.app/Contents/MacOS/Code";
const vscodeCLIPath = "/Applications/Visual Studio Code.app/Contents/Resources/app/out/cli.js";
const expectedCLISHA256 = "ee3e08d632e08abf83574d41bdaebf10e88d0d71bd17bf49e4688c1cc782e7ed";
const expectedCodeSHA256 = "72f6b27260b64923278468d94a0ac83f0ce32038803e0b6c05072eddf7d046a9";
const expectedBrowserSHA256 = "a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282";
const expectedNpmSHA256 = "8e5f6f3429f8cdbe693cdc29904e9d5a7b127a494bd15c804bd54c7403bfcbe7";
const expectedVersions = { node: "v22.23.2", npm: "10.9.8", vscode: "1.137.0", vitest: "5.0.0", playwright: "1.63.0", testElectron: "3.1.0" };
const isolatedProfileSettings = `${JSON.stringify({
  "telemetry.telemetryLevel": "off",
  "update.mode": "none",
  "extensions.autoCheckUpdates": false,
  "extensions.autoUpdate": "off",
}, null, 2)}\n`;
const interruptWitness = process.argv.includes("--interrupt-witness");
const overflowRegression = process.argv.includes("--overflow-regression");
const overflowRegressionChild = process.argv.includes("--overflow-regression-child");

if (overflowRegressionChild) {
  spawn("/bin/sleep", ["60"], { stdio: "ignore" });
  await delay(200);
  process.stdout.write(Buffer.alloc((1 << 20) + 1, "x"));
  await new Promise(() => {});
}

let activeGroup;
let observedDescendants = new Map();
let interrupted = false;
for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.once(signal, () => {
    interrupted = true;
    if (activeGroup !== undefined) terminateGroup(activeGroup, signal === "SIGINT" ? "SIGINT" : "SIGTERM");
  });
}

const scratch = await mkdtemp("/private/tmp/corvint-interactive-alpha-");
const workspace = join(scratch, "workspace");
const userData = join(scratch, "vscode-user-data");
const extensions = join(scratch, "vscode-extensions");
const authorityTemporary = join(scratch, "go-authority");
const moduleCache = join(scratch, "module-cache");
const marker = join(scratch, "interrupt-ready");
let overflowRegressionPassed = false;
try {
  if (overflowRegression) {
    try {
      await runCommand(process.execPath, [fileURLToPath(import.meta.url), "--overflow-regression-child"], scratch);
      throw new Error("overflow regression child unexpectedly passed");
    } catch (error) {
      assertEqual(error.message, "child output exceeded 1 MiB", "overflow regression failure");
      overflowRegressionPassed = true;
    }
  } else {
  await requireExecutable(vscodeExecutablePath);
  const extractedRoot = await realpath(requiredEnvironment("CORVINT_RELEASE_EXTRACTED_ROOT"));
  const releaseArchive = await realpath(requiredEnvironment("CORVINT_RELEASE_ARCHIVE"));
  const vsix = await realpath(requiredEnvironment("CORVINT_VSIX"));
  for (const name of ["CORVINT_JS_PROVIDER", "CORVINT_GO_PROVIDER", "CORVINT_TEST_VALIDITY_MCP"]) {
    const candidate = requiredEnvironment(name);
    assertEqual(candidate, await realpath(candidate), `${name} canonical path`);
    await requireExecutable(candidate);
    await requireInside(extractedRoot, candidate, name);
  }
  await requireInside(extractedRoot, vsix, "CORVINT_VSIX");
  assertEqual(process.version, expectedVersions.node, "Node version");
  const nodeInstallation = resolve(dirname(process.execPath), "..");
  const npmCli = join(nodeInstallation, "lib/node_modules/npm/bin/npm-cli.js");
  assertEqual(npmCli, await realpath(npmCli), "npm CLI canonical path");
  await requireInside(nodeInstallation, npmCli, "npm CLI");
  assertEqual(await sha256File(npmCli), expectedNpmSHA256, "npm CLI digest");
  assertEqual((await runCommand(process.execPath, [npmCli, "--version"], scratch)).trim(), expectedVersions.npm, "npm version");
  assertEqual(await sha256File(vscodeExecutablePath), expectedCodeSHA256, "VS Code executable digest");
  assertEqual(vscodeCLIPath, await realpath(vscodeCLIPath), "VS Code CLI canonical path");
  await requireInside("/Applications/Visual Studio Code.app", vscodeCLIPath, "VS Code CLI");
  assertEqual(await sha256File(vscodeCLIPath), expectedCLISHA256, "VS Code CLI digest");
  const app = JSON.parse(await readFile("/Applications/Visual Studio Code.app/Contents/Resources/app/package.json", "utf8"));
  assertEqual(app.version, expectedVersions.vscode, "VS Code version");

  await cp(fixtureSource, workspace, { recursive: true, errorOnExist: true });
  await Promise.all([mkdir(userData), mkdir(extensions), mkdir(authorityTemporary), mkdir(moduleCache)]);
  await mkdir(join(userData, "User"));
  const profileSettingsPath = join(userData, "User/settings.json");
  await writeFile(profileSettingsPath, isolatedProfileSettings, { mode: 0o600, flag: "wx" });
  const profileSettingsSHA256 = sha256Bytes(isolatedProfileSettings);
  await runCommand("/usr/bin/git", ["init", "-q"], workspace);
  await runCommand("/usr/bin/git", ["add", "."], workspace);
  await runCommand("/usr/bin/env", ["GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z", "/usr/bin/git", "-c", "user.name=Corvint Qualification", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "frozen fixture"], workspace);
  const fixtureCommit = (await runCommand("/usr/bin/git", ["rev-parse", "HEAD"], workspace)).trim();
  const fixtureTree = (await runCommand("/usr/bin/git", ["rev-parse", "HEAD^{tree}"], workspace)).trim();
  await runCommand(process.execPath, [npmCli, "ci", "--offline", "--ignore-scripts", "--no-audit", "--no-fund"], workspace);
  assertEqual((await runCommand(join(workspace, "node_modules/.bin/vitest"), ["--version"], workspace)).trim().split("/")[1]?.split(" ")[0], expectedVersions.vitest, "Vitest version");
  assertEqual((await runCommand(join(workspace, "node_modules/.bin/playwright"), ["--version"], workspace)).trim(), `Version ${expectedVersions.playwright}`, "Playwright version");
  // Default headless Playwright launches the shell; executablePath() names full Chrome.
  assertEqual(process.platform, "darwin", "installed fixture platform");
  assertEqual(process.arch, "arm64", "installed fixture architecture");
  const browserCachePath = requiredEnvironment("PLAYWRIGHT_BROWSERS_PATH");
  if (!isAbsolute(browserCachePath)) throw new Error("PLAYWRIGHT_BROWSERS_PATH must be absolute");
  const browserCacheRoot = await realpath(browserCachePath);
  const browserExecutable = join(browserCacheRoot, "chromium_headless_shell-1243", "chrome-headless-shell-mac-arm64", "chrome-headless-shell");
  assertEqual(browserExecutable, await realpath(browserExecutable), "Playwright browser canonical path");
  await requireInside(browserCacheRoot, browserExecutable, "Playwright headless browser");
  await requireExecutable(browserExecutable);
  assertEqual(await sha256File(browserExecutable), expectedBrowserSHA256, "Playwright browser digest");
  const browserVersion = (await runCommand(browserExecutable, ["--version"], workspace)).trim();
  assertEqual(browserVersion, "Google Chrome for Testing 153.0.8010.12", "Playwright browser version");

  const extensionPackageRoot = resolve(here, "../..");
  const testElectron = JSON.parse(await readFile(join(extensionPackageRoot, "node_modules/@vscode/test-electron/package.json"), "utf8"));
  assertEqual(testElectron.version, expectedVersions.testElectron, "@vscode/test-electron version");
  await chmod(moduleCache, 0o555);
  const authorityBundlePath = join(scratch, "authority-bundle.json");
  await writeFile(authorityBundlePath, JSON.stringify(await measuredCallerAttachment(workspace, authorityTemporary, moduleCache)), { mode: 0o600 });
  const isolatedCodeArgs = ["--user-data-dir", userData, "--extensions-dir", extensions, "--disable-updates", "--disable-telemetry", "--use-inmemory-secretstorage", "--sync", "off"];
  await runCodeCLI(["--install-extension", vsix, "--force", ...isolatedCodeArgs], scratch);
  const installed = await runCodeCLI(["--list-extensions", "--show-versions", ...isolatedCodeArgs], scratch);
  if (!installed.split(/\r?\n/).includes("corvint.corvint-vscode@0.1.0")) throw new Error(`installed VSIX identity missing: ${installed}`);
  const toolIdentities = await captureToolIdentities(browserExecutable, browserVersion, releaseArchive, vsix, npmCli);

  const worker = spawn(process.execPath, [join(here, "worker.mjs")], {
    detached: true,
    stdio: ["ignore", "pipe", "pipe"],
    env: {
      ...process.env,
      CORVINT_FIXTURE_ROOT: workspace,
      CORVINT_EXTENSION_DEVELOPMENT_PATH: extensionDevelopmentPath,
      CORVINT_EXTENSION_TESTS_PATH: join(here, "index.cjs"),
      CORVINT_VSCODE_EXECUTABLE: vscodeExecutablePath,
      CORVINT_VSCODE_USER_DATA: userData,
      CORVINT_VSCODE_EXTENSIONS: extensions,
      CORVINT_GO_AUTHORITY_BUNDLE: authorityBundlePath,
      CORVINT_NODE_EXECUTABLE: process.execPath,
      CORVINT_GO_EXECUTABLE: "/opt/homebrew/Cellar/go/1.27.1/bin/go",
      CORVINT_TOOL_IDENTITIES: JSON.stringify(publicToolIdentities(toolIdentities)),
      CORVINT_FIXTURE_COMMIT: fixtureCommit,
      CORVINT_FIXTURE_TREE: fixtureTree,
      CORVINT_PROFILE_SETTINGS_SHA256: profileSettingsSHA256,
      CORVINT_INTERRUPT_MARKER: marker,
      CORVINT_INTERRUPT_WITNESS: interruptWitness ? "1" : "0",
    },
  });
  activeGroup = worker.pid;
  let tracking = true;
  const tracker = trackDescendants(worker.pid, observedDescendants, () => tracking);
  let stdout = "";
  let stderr = "";
  let outputFailure;
  const captureWorkerOutput = (stream, chunk) => {
    if (outputFailure !== undefined) return;
    try {
      if (stream === "stdout") stdout = boundedAppend(stdout, chunk);
      else stderr = boundedAppend(stderr, chunk);
      (stream === "stdout" ? process.stdout : process.stderr).write(chunk);
    } catch (error) {
      outputFailure = error;
      try { terminateGroup(worker.pid, "SIGTERM"); } catch (terminationError) { outputFailure = new AggregateError([error, terminationError], "worker output and termination failure"); }
    }
  };
  worker.stdout.on("data", (chunk) => captureWorkerOutput("stdout", chunk));
  worker.stderr.on("data", (chunk) => captureWorkerOutput("stderr", chunk));

  let code;
  let workerFailure;
  try {
    if (interruptWitness) {
      await waitForFile(marker, 180_000);
      await waitForObserved(observedDescendants, [/corvint-js-test-provider/, /app\/server\.js/, /chrome-headless-shell/], 120_000);
      terminateGroup(worker.pid, "SIGINT");
    }
    code = await waitForExit(worker, 360_000);
  } catch (error) {
    workerFailure = error;
  } finally {
    tracking = false;
    await tracker;
  }
  const survivors = await waitForTrackedExit(observedDescendants, 5_000);
  const residue = survivors.length > 0;
  if (residue) await containTracked(survivors);
  activeGroup = undefined;
  if (residue) throw new Error(`observed VS Code descendant tree survived completion and required fallback containment: ${survivors.map((entry) => entry.pid).join(",")}`);
  if (workerFailure !== undefined) throw workerFailure;
  if (outputFailure !== undefined) throw outputFailure;
  assertEqual(await readFile(profileSettingsPath, "utf8"), isolatedProfileSettings, "isolated VS Code profile settings");
  await revalidateToolIdentities(toolIdentities);
  if (interruptWitness) {
    if (code !== 1 && code !== 130 && code !== null) throw new Error(`interruption worker exited ${code}; expected test-runner interrupt exit`);
    process.stdout.write(`${JSON.stringify({ profile: "corvint-interactive-alpha-interruption/0", status: "PASS", workerExit: code, descendantsGone: true })}\n`);
  } else if (code !== 0) {
    throw new Error(`installed-host worker exited ${code}: ${stderr.slice(-2000) || stdout.slice(-2000)}`);
  }
  }
} finally {
  if (activeGroup !== undefined) {
    terminateGroup(activeGroup, "SIGTERM");
    await delay(400);
    if (processGroupExists(activeGroup)) terminateGroup(activeGroup, "SIGKILL");
  }
  const survivors = await currentTracked(observedDescendants);
  if (survivors.length > 0) await containTracked(survivors);
  await rm(scratch, { recursive: true, force: true });
}
if (overflowRegressionPassed) process.stdout.write(`${JSON.stringify({ profile: "corvint-installed-process-overflow-regression/0", status: "PASS", scratch, descendantsGone: true })}\n`);
if (interrupted) process.exitCode = 130;

function requiredEnvironment(name) {
  const value = process.env[name];
  if (!value || !value.startsWith("/")) throw new Error(`${name} must be an absolute path`);
  return value;
}

async function measuredCallerAttachment(worktree, temporaryParent, moduleCacheDirectory) {
  const verifierExecutable = await realpath(requiredEnvironment("CORVINT_GO_PROVIDER"));
  const gitExecutable = await realpath("/usr/bin/git");
  const goExecutable = await realpath("/opt/homebrew/Cellar/go/1.27.1/bin/go");
  const goos = await runCommand(goExecutable, ["env", "GOOS"], worktree);
  const goarch = await runCommand(goExecutable, ["env", "GOARCH"], worktree);
  const goroot = await runCommand(goExecutable, ["env", "GOROOT"], worktree);
  return {
    profile: "corvint-go-live-authority-attachment/0",
    verifierExecutable,
    verifierExecutableRawSha256: await sha256File(verifierExecutable),
    gitExecutable,
    gitExecutableRawSha256: await sha256File(gitExecutable),
    goExecutable,
    goWorkPath: "",
    goarch: goarch.trim(),
    goos: goos.trim(),
    goroot: await realpath(goroot.trim()),
    moduleCacheDirectory,
    moduleMode: "MODULE_READONLY",
    outputLimitBytes: 8 << 20,
    packages: ["example.test/corvint-interactive-alpha/go"],
    repositoryRoot: worktree,
    temporaryParent,
    timeoutMilliseconds: 1_800_000,
  };
}

async function captureToolIdentities(browserExecutable, browserVersion, releaseArchive, vsix, npmCli) {
  const paths = {
    vscode: vscodeExecutablePath,
    vscodeCLI: vscodeCLIPath,
    node: process.execPath,
    npm: npmCli,
    jsProvider: requiredEnvironment("CORVINT_JS_PROVIDER"),
    goProvider: requiredEnvironment("CORVINT_GO_PROVIDER"),
    testValidityMcp: requiredEnvironment("CORVINT_TEST_VALIDITY_MCP"),
    browser: browserExecutable,
    releaseArchive,
    vsix,
  };
  const identities = {};
  for (const [name, file] of Object.entries(paths)) identities[name] = { path: file, sha256: await sha256File(file) };
  identities.vscode.version = expectedVersions.vscode;
  identities.node.version = expectedVersions.node;
  identities.npm.version = expectedVersions.npm;
  identities.browser.version = browserVersion;
  identities.vitest = { version: expectedVersions.vitest };
  identities.playwright = { version: expectedVersions.playwright };
  identities.testElectron = { version: expectedVersions.testElectron };
  return identities;
}

async function revalidateToolIdentities(identities) {
  for (const identity of Object.values(identities)) {
    if (identity.path !== undefined) assertEqual(await sha256File(identity.path), identity.sha256, `tool identity ${identity.path}`);
  }
}

function publicToolIdentities(identities) {
  return Object.fromEntries(Object.entries(identities).map(([name, identity]) => {
    const { path: _privatePath, ...published } = identity;
    return [name, published];
  }));
}

async function requireInside(root, candidate, name) {
  const resolved = await realpath(candidate);
  const inside = relative(root, resolved);
  if (inside === "" || inside.startsWith(`..${process.platform === "win32" ? "\\" : "/"}`) || isAbsolute(inside)) throw new Error(`${name} is not a file inside CORVINT_RELEASE_EXTRACTED_ROOT`);
}

async function requireExecutable(file) {
  if (!file.startsWith("/")) throw new Error(`executable path is not absolute: ${file}`);
  await access(file, constants.X_OK);
}

async function sha256File(file) {
  return createHash("sha256").update(await readFile(file)).digest("hex");
}

function sha256Bytes(value) {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

async function runCodeCLI(args, cwd) {
  const env = { ...process.env, ELECTRON_RUN_AS_NODE: "1" };
  for (const name of ["NODE_OPTIONS", "NODE_REPL_EXTERNAL_MODULE", "VSCODE_NODE_OPTIONS", "VSCODE_NODE_REPL_EXTERNAL_MODULE", "VSCODE_IPC_HOOK_CLI"]) delete env[name];
  return runCommand(vscodeExecutablePath, [vscodeCLIPath, ...args], cwd, env);
}

async function runCommand(command, args, cwd, env = process.env) {
  const child = spawn(command, args, { cwd, env, detached: true, stdio: ["ignore", "pipe", "pipe"] });
  activeGroup = child.pid;
  const observed = new Map();
  let tracking = true;
  const tracker = trackDescendants(child.pid, observed, () => tracking);
  let stdout = "";
  let stderr = "";
  let outputFailure;
  const captureOutput = (stream, chunk) => {
    if (outputFailure !== undefined) return;
    try {
      if (stream === "stdout") stdout = boundedAppend(stdout, chunk);
      else stderr = boundedAppend(stderr, chunk);
    } catch (error) {
      outputFailure = error;
      try { terminateGroup(child.pid, "SIGTERM"); } catch (terminationError) { outputFailure = new AggregateError([error, terminationError], "child output and termination failure"); }
    }
  };
  child.stdout.on("data", (chunk) => captureOutput("stdout", chunk));
  child.stderr.on("data", (chunk) => captureOutput("stderr", chunk));
  let code;
  let failure;
  try {
    code = await waitForExit(child, 180_000);
  } catch (error) {
    failure = error;
  } finally {
    tracking = false;
    await tracker;
  }
  const survivors = await waitForTrackedExit(observed, 2_000);
  if (survivors.length > 0) {
    await containTracked(survivors);
    throw new Error(`${command} left descendants and required fallback containment: ${survivors.map((entry) => entry.pid).join(",")}`);
  }
  activeGroup = undefined;
  if (failure !== undefined) throw failure;
  if (outputFailure !== undefined) throw outputFailure;
  if (code !== 0) throw new Error(`${command} exited ${code}: ${stderr.slice(-2000)}`);
  return stdout;
}

function boundedAppend(current, chunk) {
  const next = current + chunk.toString("utf8");
  if (Buffer.byteLength(next) > 1 << 20) throw new Error("child output exceeded 1 MiB");
  return next;
}

function waitForExit(child, timeoutMilliseconds) {
  return new Promise((resolvePromise, reject) => {
    const timeout = setTimeout(() => reject(new Error("VS Code worker timed out")), timeoutMilliseconds);
    child.once("error", (error) => { clearTimeout(timeout); reject(error); });
    child.once("close", (code) => { clearTimeout(timeout); resolvePromise(code); });
  });
}

async function waitForFile(file, timeoutMilliseconds) {
  const deadline = Date.now() + timeoutMilliseconds;
  while (Date.now() < deadline) {
    try { await access(file); return; } catch { await delay(50); }
  }
  throw new Error("interruption witness did not reach a live provider");
}

async function waitForObserved(observed, patterns, timeoutMilliseconds) {
  const deadline = Date.now() + timeoutMilliseconds;
  while (Date.now() < deadline) {
    if (patterns.every((pattern) => [...observed.values()].some((row) => pattern.test(row.command)))) return;
    await delay(50);
  }
  throw new Error(`interruption witness did not observe full provider tree: ${[...observed.values()].map((row) => row.command).join("\n")}`);
}

async function trackDescendants(rootPid, observed, active) {
  while (active()) {
    const table = await processTable();
    const descendants = descendantRows(table, rootPid);
    for (const row of descendants) observed.set(row.pid, row);
    await delay(100);
  }
  const table = await processTable();
  for (const row of descendantRows(table, rootPid)) observed.set(row.pid, row);
}

async function processTable() {
  const { stdout } = await execFileAsync("/bin/ps", ["-axo", "pid=,ppid=,pgid=,lstart=,command="], { maxBuffer: 4 << 20 });
  const rows = [];
  for (const line of stdout.split("\n")) {
    const match = line.match(/^\s*(\d+)\s+(\d+)\s+(\d+)\s+(.{24})\s+(.*)$/);
    if (match) rows.push({ pid: Number(match[1]), ppid: Number(match[2]), pgid: Number(match[3]), started: match[4], command: match[5] });
  }
  return rows;
}

function descendantRows(table, rootPid) {
  const pids = new Set([rootPid]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const row of table) {
      if (pids.has(row.ppid) && !pids.has(row.pid)) { pids.add(row.pid); changed = true; }
    }
  }
  return table.filter((row) => pids.has(row.pid));
}

async function currentTracked(observed) {
  if (observed.size === 0) return [];
  const table = await processTable();
  const current = new Map(table.map((row) => [row.pid, row]));
  return [...observed.values()].filter((row) => {
    const value = current.get(row.pid);
    return value !== undefined && value.started === row.started && value.command === row.command;
  });
}

async function waitForTrackedExit(observed, timeoutMilliseconds) {
  const deadline = Date.now() + timeoutMilliseconds;
  while (Date.now() < deadline) {
    const survivors = await currentTracked(observed);
    if (survivors.length === 0) return [];
    await delay(50);
  }
  return currentTracked(observed);
}

async function containTracked(rows) {
  for (const pgid of new Set(rows.map((row) => row.pgid))) terminateGroup(pgid, "SIGTERM");
  await delay(400);
  for (const row of await currentTracked(new Map(rows.map((entry) => [entry.pid, entry])))) {
    try { process.kill(row.pid, "SIGKILL"); } catch (error) { if (error.code !== "ESRCH") throw error; }
  }
}

function terminateGroup(pid, signal) {
  try { process.kill(-pid, signal); } catch (error) { if (error.code !== "ESRCH") throw error; }
}

function processGroupExists(pid) {
  try { process.kill(-pid, 0); return true; } catch (error) { if (error.code === "ESRCH") return false; throw error; }
}

function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(`${label} ${JSON.stringify(actual)} != ${JSON.stringify(expected)}`);
}

function delay(milliseconds) {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, milliseconds));
}
