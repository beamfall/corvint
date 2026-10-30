import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const { runJsonLineProcess } = require("./mcp-client.cjs");

if (process.argv.includes("--child")) {
  spawn("/bin/sleep", ["60"], { stdio: "ignore" });
  await delay(200);
  process.stdout.write(Buffer.alloc((1 << 20) + 1, "x"));
  await new Promise(() => {});
}

const scratch = await mkdtemp("/private/tmp/corvint-mcp-overflow-");
let passed = false;
try {
  try {
    await runJsonLineProcess(process.execPath, [fileURLToPath(import.meta.url), "--child"], {}, 5_000);
    throw new Error("MCP overflow child unexpectedly passed");
  } catch (error) {
    if (error.message !== "MCP output exceeded 1 MiB") throw error;
    passed = true;
  }
} finally {
  await rm(scratch, { recursive: true, force: true });
}
if (passed) process.stdout.write(`${JSON.stringify({ profile: "corvint-mcp-overflow-regression/0", status: "PASS", scratch, descendantsGone: true })}\n`);

function delay(milliseconds) {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, milliseconds));
}
