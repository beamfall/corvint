// SPDX-License-Identifier: Apache-2.0
//
// Corvint browser-coverage hook (LPCV-V0-970..976). Loaded into every Node
// process of one collection run through NODE_OPTIONS=--require. It writes
// nothing outside CORVINT_BROWSERCOV_DIR.
//
// Every process writes role-<pid>.json naming what it is, so the collector
// can tell the test runner's own processes from the processes under test.
// A process under test answers SIGUSR2 by writing its NODE_V8_COVERAGE
// (v8.takeCoverage) and then taken-<pid>-<n>, so its coverage is on disk
// before a runner stops it with SIGKILL (LPCV-V0-954, LPCV-V0-974).
//
// A Playwright worker starts precise JS coverage on every page its contexts
// open; at each context close it flushes the processes under test and writes
// browser-<pid>-<n>.json. Cypress loads this file again from the
// collector's generated config and calls cypressConfig, which reads the
// browser's coverage over the Chrome DevTools Protocol.
"use strict";

const fs = require("fs");
const path = require("path");

const dir = process.env.CORVINT_BROWSERCOV_DIR;
let role = "";
// Preloads also run in worker threads (Playwright's ESM loader thread), where
// process.argv is only the node binary; only the main thread is the process.
if (dir && !global.__corvintBrowsercov && require("worker_threads").isMainThread) {
  global.__corvintBrowsercov = true;
  try {
    main(dir);
  } catch (e) {
    note("hook-error", String((e && e.stack) || e));
  }
}

function note(kind, detail) {
  try {
    fs.appendFileSync(path.join(dir, `incomplete-${process.pid}.jsonl`), JSON.stringify({ kind, detail }) + "\n");
  } catch {}
}

function main(dir) {
  const argv = process.argv.slice(1).join(" ");
  // Playwright forks every worker (and its loader) as lib/common/process.js;
  // only a worker creates browser contexts, so instrumenting all is harmless.
  const worker = /[\\/]playwright[\\/]lib[\\/]common[\\/]process\.js|workerProcessEntry/.test(argv);
  // The runners' own processes run test code, never code under test.
  const runner =
    !worker &&
    /[\\/](playwright|@playwright[\\/]test|cypress|Cypress)[\\/]|[\\/]\.bin[\\/](playwright|cypress)\b/.test(argv);
  role = worker ? "worker" : runner ? "runner" : "process";
  fs.writeFileSync(
    path.join(dir, `role-${process.pid}.json`),
    JSON.stringify({ pid: process.pid, ppid: process.ppid, role, argv: process.argv, cwd: process.cwd() })
  );
  if (role === "process") {
    let taken = 0;
    process.on("SIGUSR2", () => {
      try {
        require("v8").takeCoverage();
      } catch (e) {
        note("take-coverage", String(e));
      }
      fs.writeFileSync(path.join(dir, `taken-${process.pid}-${taken++}`), "");
    });
    for (const sig of ["SIGTERM", "SIGINT"]) {
      process.on(sig, () => {
        // Leave a server's own shutdown handler to run; exit only when ours is
        // the sole listener, which would otherwise suppress the default exit.
        if (process.listenerCount(sig) === 1) process.exit(sig === "SIGTERM" ? 143 : 130);
      });
    }
    return;
  }
  if (role === "worker") instrumentPlaywright();
}

// flushServers asks every live process under test to write its coverage and
// waits until each has; a process that does not answer makes the run
// incomplete.
async function flushServers() {
  await Promise.all(serverPids().map((pid) => flushOne(pid, (ms) => new Promise((r) => setTimeout(r, ms)))));
}

// flushServersSync is flushServers for a process that is exiting, where no
// timer runs again: it is the last chance to take what the processes under
// test ran after the last context closed.
function flushServersSync() {
  const cell = new Int32Array(new SharedArrayBuffer(4));
  for (const pid of serverPids()) flushOne(pid, (ms) => Atomics.wait(cell, 0, 0, ms));
}

function serverPids() {
  const pids = [];
  for (const f of fs.readdirSync(dir)) {
    const m = /^role-(\d+)\.json$/.exec(f);
    if (!m) continue;
    try {
      if (JSON.parse(fs.readFileSync(path.join(dir, f), "utf8")).role === "process") pids.push(Number(m[1]));
    } catch {}
  }
  return pids;
}

// flushOne signals one process and waits through sleep, which returns a
// promise or blocks, until it has written its coverage.
function flushOne(pid, sleep) {
  const count = () => fs.readdirSync(dir).filter((f) => f.startsWith(`taken-${pid}-`)).length;
  const before = count();
  try {
    process.kill(pid, "SIGUSR2");
  } catch {
    return; // exited; NODE_V8_COVERAGE wrote at exit
  }
  const timeout = () => {
    try {
      process.kill(pid, 0);
      note("flush-timeout", `process ${pid} did not write its coverage`);
    } catch {}
  };
  let i = 0;
  const step = () => {
    for (; i < 500; i++) {
      if (count() > before) return;
      const p = sleep(20);
      if (p && typeof p.then === "function") {
        i++;
        return p.then(step);
      }
    }
    timeout();
  };
  return step();
}

async function writeBrowser(browser, entries, extra) {
  const out = [];
  for (const e of entries) {
    out.push({ url: e.url, source: e.source || "", functions: e.functions, sourceMap: await fetchMap(e) });
  }
  const file = path.join(dir, `browser-${process.pid}-${writeBrowser.seq++}.json`);
  fs.writeFileSync(file, JSON.stringify(Object.assign({ browser, scripts: out }, extra || {})));
}
writeBrowser.seq = 0;

// fetchMap returns the text of an external source map, fetched while the
// server is still up; an inline data: map stays in the source. A runner's
// own script is never bound, so its map is not fetched: that request would
// reach the server under test.
async function fetchMap(e) {
  if (!mayBeAppCode(e.url)) return undefined;
  const ref = e.sourceMapURL || (/\/\/[#@] sourceMappingURL=([^\s'"]+)\s*$/.exec(e.source || "") || [])[1];
  if (!ref || ref.startsWith("data:")) return undefined;
  let url;
  try {
    url = new URL(ref, e.url).href;
  } catch {
    return { url: ref, error: "unresolvable source map URL" };
  }
  try {
    const r = await fetch(url);
    if (!r.ok) return { url, error: `HTTP ${r.status}` };
    return { url, text: await r.text() };
  } catch (err) {
    return { url, error: String(err) };
  }
}

function instrumentPlaywright() {
  let core;
  try {
    const pw = path.dirname(require.resolve("playwright/package.json", { paths: [process.cwd()] }));
    core = require(require.resolve("playwright-core", { paths: [pw] }));
  } catch (e) {
    note("no-playwright-core", String(e));
    return;
  }
  const inst = core._instrumentation;
  if (!inst || typeof inst.addListener !== "function") {
    note("no-instrumentation", "playwright-core has no _instrumentation listener API");
    return;
  }
  const started = new WeakSet();
  const watched = new WeakSet(); // every page coverage was started on
  const opened = new WeakMap(); // context -> every page it opened
  const stash = new WeakMap(); // context -> coverage entries of pages closed early
  const live = new Set(); // contexts with coverage started and not yet closed
  const worker = (w) => {
    // A worker's scripts run outside the page's coverage.
    if (mayBeAppCode(w.url())) note("unobserved-worker", w.url());
  };
  process.on("exit", () => {
    if (live.size > 0) note("unclosed-context", `${live.size} browser context(s) closed without a coverage take`);
    flushServersSync();
  });

  async function start(page) {
    if (!page.coverage || typeof page.coverage.startJSCoverage !== "function") {
      note("no-js-coverage", "browser has no JS coverage (only Chromium reports it)");
      return;
    }
    await page.coverage.startJSCoverage({ resetOnNavigation: false, reportAnonymousScripts: false });
    started.add(page);
    watched.add(page);
    live.add(page.context());
    // A page the app or browser closes (window.close, a crash) loses its
    // coverage: only page.close() and the context close reach stop().
    page.on("close", () => {
      if (!started.delete(page)) return;
      note("closed-page", `${page.url()} closed before its coverage was taken`);
    });
    const close = page.close.bind(page);
    page.close = async (...args) => {
      await stop(page);
      return close(...args);
    };
  }

  async function stop(page) {
    if (!started.has(page)) return;
    started.delete(page);
    let entries = [];
    try {
      entries = await page.coverage.stopJSCoverage();
    } catch (e) {
      note("stop-failed", String(e));
      return;
    }
    const ctx = page.context();
    if (!stash.has(ctx)) stash.set(ctx, []);
    stash.get(ctx).push(...entries);
  }

  inst.addListener({
    runAfterCreateBrowserContext: async (context) => {
      const newPage = context.newPage.bind(context);
      context.newPage = async (...args) => {
        const page = await newPage(...args);
        await start(page);
        return page;
      };
      opened.set(context, []);
      context.on("page", (page) => {
        opened.get(context).push(page);
        page.on("worker", worker);
      });
      context.on("serviceworker", worker);
    },
    runBeforeCloseBrowserContext: async (context) => {
      // A page opened by the app (a popup) starts after its first scripts
      // may have run, so its coverage is incomplete.
      for (const page of opened.get(context) || []) {
        if (!watched.has(page)) note("unwatched-page", page.url());
      }
      for (const page of context.pages()) await stop(page);
      const entries = stash.get(context) || [];
      stash.delete(context);
      live.delete(context);
      const browser = context.browser();
      await writeBrowser(browser ? `${browser.browserType().name()}@${browser.version()}` : "", entries);
      await flushServers();
    },
  });
}

// cdpCoverage connects to a Chromium browser's remote-debugging port and
// starts precise coverage in every page and frame target, existing or new.
async function cdpCoverage(port) {
  let info;
  for (let i = 0; ; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${port}/json/version`);
      info = await r.json();
      break;
    } catch (e) {
      if (i >= 300) throw new Error(`no DevTools endpoint on port ${port}: ${e}`);
      await new Promise((r) => setTimeout(r, 100));
    }
  }
  const ws = new WebSocket(info.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    ws.onopen = resolve;
    ws.onerror = () => reject(new Error("DevTools WebSocket failed"));
  });
  let id = 0;
  const pending = new Map();
  const sessions = new Map(); // sessionId -> Map scriptId -> {url, sourceMapURL}
  const untaken = new Set(); // sessions that parsed possible repository code since their last take
  // LPCV-V0-8301: a closed transport stays closed; only close() is intentional.
  let closed = false;
  let closing = false;
  const send = (method, params, sessionId) =>
    new Promise((resolve, reject) => {
      if (closed) return reject(new Error("DevTools connection closed"));
      const n = ++id;
      pending.set(n, { resolve, reject, sessionId });
      ws.send(JSON.stringify({ id: n, method, params: params || {}, sessionId }));
    });
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data);
    if (m.id) {
      const p = pending.get(m.id);
      pending.delete(m.id);
      if (p) m.error ? p.reject(new Error(`${m.error.message}`)) : p.resolve(m.result);
      return;
    }
    if (m.method === "Target.attachedToTarget") attached(m.params).catch((e) => note("cdp-attach", String(e)));
    else if (m.method === "Target.detachedFromTarget") {
      // A target that closes before a take loses its coverage; that matters
      // only when it parsed a script that may be repository code.
      if (untaken.has(m.params.sessionId)) note("cdp-detached", "a page closed before its coverage was taken");
      untaken.delete(m.params.sessionId);
      for (const [id, command] of pending) if (command.sessionId === m.params.sessionId) {
        pending.delete(id); command.reject(new Error("DevTools session detached"));
      }
      sessions.delete(m.params.sessionId);
    } else if (m.method === "Debugger.scriptParsed" && sessions.has(m.sessionId)) {
      sessions.get(m.sessionId).set(m.params.scriptId, { url: m.params.url, sourceMapURL: m.params.sourceMapURL });
      if (mayBeAppCode(m.params.url)) untaken.add(m.sessionId);
    }
  };
  ws.onclose = () => {
    closed = true;
    // Already-parsed code can run again after a successful take without a new
    // scriptParsed, so an unexpected loss is incomplete even with untaken empty.
    if (untaken.size) note("cdp-detached", "a page closed before its coverage was taken");
    else if (!closing) note("cdp-detached", "the DevTools connection closed unexpectedly; executions after the last take are lost");
    for (const command of pending.values()) command.reject(new Error("DevTools connection closed"));
    pending.clear(); untaken.clear(); sessions.clear();
  };
  async function attached({ sessionId, targetInfo, waitingForDebugger }) {
    // A worker's scripts run outside the page's coverage.
    if (targetInfo.type.includes("worker") && mayBeAppCode(targetInfo.url)) note("unobserved-worker", targetInfo.url);
    if (targetInfo.type === "page" || targetInfo.type === "iframe") {
      sessions.set(sessionId, new Map());
      await send("Profiler.enable", {}, sessionId);
      await send("Profiler.startPreciseCoverage", { callCount: true, detailed: true }, sessionId);
      await send("Debugger.enable", {}, sessionId);
      await send("Target.setAutoAttach", { autoAttach: true, waitForDebuggerOnStart: true, flatten: true }, sessionId);
    }
    if (waitingForDebugger) await send("Runtime.runIfWaitingForDebugger", {}, sessionId);
  }
  await send("Target.setAutoAttach", { autoAttach: true, waitForDebuggerOnStart: true, flatten: true });
  const version = await send("Browser.getVersion");
  return {
    browser: version.product,
    // take returns every target's coverage since the last take and resets it.
    async take() {
      if (closed) throw new Error("DevTools connection closed");
      const out = [];
      for (const [sessionId, scripts] of sessions) {
        let res;
        try {
          res = await send("Profiler.takePreciseCoverage", {}, sessionId);
        } catch (e) {
          note("cdp-take", String(e));
          continue;
        }
        untaken.delete(sessionId);
        for (const s of res.result) {
          if (!s.url) continue;
          let source = "";
          try {
            source = (await send("Debugger.getScriptSource", { scriptId: s.scriptId }, sessionId)).scriptSource;
          } catch (e) {
            note("cdp-source", `${s.url}: ${e}`);
          }
          out.push({ url: s.url, source, functions: s.functions, sourceMapURL: (scripts.get(s.scriptId) || {}).sourceMapURL });
        }
      }
      return out;
    },
    close() {
      closing = true;
      ws.close();
    },
  };
}

// mayBeAppCode mirrors the collector's rule for a script it must bind
// (coverage.go Attribute): a loopback origin outside Cypress's own routes.
function mayBeAppCode(u) {
  let p;
  try {
    p = new URL(u);
  } catch {
    return false;
  }
  if (p.protocol === "file:") return true;
  const h = p.hostname.replace(/^\[|\]$/g, "");
  const loopback = h === "localhost" || h === "127.0.0.1" || h === "::1" || h === "0.0.0.0" || h.endsWith(".localhost");
  return loopback && !p.pathname.startsWith("/__cypress") && !p.pathname.startsWith("/__/");
}

// cypressConfig wraps a Cypress configuration: it isolates one test named by
// the CORVINT_ONLY env (a URL-encoded JSON title path), skips every test
// under CORVINT_LIST, records each spec's results, and
// reads the browser's coverage at each spec's end over the DevTools port
// Cypress opened (LPCV-V0-975).
function cypressConfig(user, supportPath) {
  user = (user && user.default) || user;
  const e2e = Object.assign({}, user.e2e);
  const root = process.cwd();
  let support = e2e.supportFile;
  if (support === undefined) {
    support = ["js", "jsx", "ts", "tsx"].map((x) => `cypress/support/e2e.${x}`).find((p) => fs.existsSync(path.join(root, p)));
  }
  const lines = [];
  if (support) lines.push(`require(${JSON.stringify("./" + path.relative(path.dirname(supportPath), path.resolve(root, support)).split(path.sep).join("/"))});`);
  lines.push(
    "beforeEach(function () {",
    '  const only = Cypress.env("CORVINT_ONLY");',
    '  if (Cypress.env("CORVINT_LIST") || (only && JSON.stringify(this.currentTest.titlePath()) !== decodeURIComponent(String(only).replace(/\\+/g, " ")))) this.skip();',
    // Coverage must be running in the browser before the test loads a page.
    '  cy.task("corvintBrowsercovReady", null, { log: false });',
    "});",
    // Cypress closes the spec's tab before after:spec, so take the coverage
    // from inside the spec while its page is alive.
    'after(() => cy.task("corvintBrowsercovTake", null, { log: false }));'
  );
  fs.writeFileSync(supportPath, lines.join("\n") + "\n");
  e2e.supportFile = supportPath;
  const userSetup = e2e.setupNodeEvents;
  e2e.setupNodeEvents = async (on, config) => {
    const handlers = {};
    const wrapped = (event, fn) => {
      if (["before:browser:launch", "after:spec", "before:run"].includes(event)) handlers[event] = fn;
      else on(event, fn);
    };
    let result = config;
    if (userSetup) result = (await userSetup(wrapped, config)) || config;
    let cdp;
    let taken = []; // coverage taken inside the current spec
    const take = async () => {
      const c = await cdp;
      taken.push(...(await c.take()));
      return c.browser;
    };
    on("task", {
      corvintBrowsercovReady: async () => {
        if (cdp) await cdp.catch(() => {});
        return null;
      },
      corvintBrowsercovTake: async () => {
        if (cdp) await take().catch((e) => note("cdp-take", String(e)));
        return null;
      },
    });
    on("before:browser:launch", async (browser, launchOptions) => {
      if (handlers["before:browser:launch"]) launchOptions = (await handlers["before:browser:launch"](browser, launchOptions)) || launchOptions;
      const arg = (launchOptions.args || []).find((a) => a.startsWith("--remote-debugging-port="));
      if (browser.family !== "chromium" || !arg || browser.name === "electron") {
        note("cypress-browser", `browser ${browser.name} has no remote debugging port; collect with a Chromium browser`);
        return launchOptions;
      }
      cdp = cdpCoverage(Number(arg.split("=")[1]));
      cdp.catch((e) => note("cdp-connect", String(e)));
      return launchOptions;
    });
    on("before:run", async (details) => {
      fs.writeFileSync(path.join(dir, "specs.json"), JSON.stringify((details.specs || []).map((s) => s.absolute)));
      if (handlers["before:run"]) return handlers["before:run"](details);
    });
    on("after:spec", async (spec, results) => {
      const tests = ((results && results.tests) || []).map((t) => ({ title: t.title, state: t.state }));
      let browser = "";
      if (cdp) {
        try {
          browser = await take();
        } catch (e) {
          note("cdp-take", String(e));
        }
      } else if (tests.some((t) => t.state === "passed" || t.state === "failed")) {
        note("cypress-browser", "no browser coverage connection");
      }
      const entries = taken;
      taken = [];
      await writeBrowser(browser, entries, { spec: spec.absolute, tests });
      if (handlers["after:spec"]) return handlers["after:spec"](spec, results);
    });
    return result;
  };
  return Object.assign({}, user, { e2e });
}

module.exports = { cypressConfig };
