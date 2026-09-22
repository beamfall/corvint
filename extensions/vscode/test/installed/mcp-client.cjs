const { spawn } = require("node:child_process");

const outputLimit = 1 << 20;

exports.callMcpDiscover = function callMcpDiscover(executable, root, timeoutMilliseconds = 15_000) {
  const request = { jsonrpc: "2.0", id: 1, method: "tools/call", params: { _meta: { "io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": {} }, name: "corvint.test_validity", arguments: { discover: true } } };
  return runJsonLineProcess(executable, ["--root", root], request, timeoutMilliseconds);
};

exports.runJsonLineProcess = runJsonLineProcess;

function runJsonLineProcess(executable, args, request, timeoutMilliseconds) {
  return new Promise((resolvePromise, reject) => {
    const child = spawn(executable, args, { detached: true, stdio: ["pipe", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    let failure;
    let closed = false;
    let forced = false;
    let force;
    const fail = (error) => {
      if (failure === undefined) failure = error;
      if (child.pid !== undefined) {
        safeTerminate(child.pid, "SIGTERM", (terminationError) => { failure = new AggregateError([failure, terminationError], "MCP failure and termination failure"); });
        force ??= setTimeout(() => {
          if (!closed) {
            forced = true;
            safeTerminate(child.pid, "SIGKILL", (terminationError) => { failure = new AggregateError([failure, terminationError], "MCP failure and forced termination failure"); });
          }
        }, 1_000);
      }
    };
    const append = (current, chunk) => {
      if (failure !== undefined) return current;
      const next = current + chunk.toString("utf8");
      if (Buffer.byteLength(next) > outputLimit) {
        fail(new Error("MCP output exceeded 1 MiB"));
        return current;
      }
      return next;
    };
    const timeout = setTimeout(() => fail(new Error("test-validity MCP timed out")), timeoutMilliseconds);
    child.stdout.on("data", (chunk) => { stdout = append(stdout, chunk); });
    child.stderr.on("data", (chunk) => { stderr = append(stderr, chunk); });
    child.once("error", fail);
    child.once("close", async (code) => {
      closed = true;
      clearTimeout(timeout);
      if (force !== undefined) clearTimeout(force);
      try {
        if (child.pid !== undefined && !await waitForProcessGroupExit(child.pid, 2_000)) {
          forced = true;
          terminate(child.pid, "SIGKILL");
          if (!await waitForProcessGroupExit(child.pid, 2_000)) throw new Error(`MCP process group ${child.pid} survived SIGKILL`);
        }
        if (forced && failure === undefined) failure = new Error("MCP process group required fallback containment");
        if (failure !== undefined) return reject(failure);
        if (code !== 0) return reject(new Error(`test-validity MCP exited ${code}: ${stderr}`));
        try { resolvePromise(JSON.parse(stdout.trim())); } catch (error) { reject(new Error(`invalid MCP response: ${error}; ${stdout}`)); }
      } catch (error) {
        reject(error);
      }
    });
    child.stdin.on("error", fail);
    child.stdin.end(`${JSON.stringify(request)}\n`);
  });
}

async function waitForProcessGroupExit(pid, timeoutMilliseconds) {
  const deadline = Date.now() + timeoutMilliseconds;
  while (Date.now() < deadline) {
    if (!processGroupExists(pid)) return true;
    await delay(25);
  }
  return !processGroupExists(pid);
}

function terminate(pid, signal) {
  try { process.kill(-pid, signal); } catch (error) { if (error.code !== "ESRCH") throw error; }
}

function safeTerminate(pid, signal, onError) {
  try { terminate(pid, signal); } catch (error) { onError(error); }
}

function processGroupExists(pid) {
  try { process.kill(-pid, 0); return true; } catch (error) { if (error.code === "ESRCH") return false; throw error; }
}

function delay(milliseconds) {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, milliseconds));
}
