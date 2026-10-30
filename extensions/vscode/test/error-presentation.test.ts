import "./vscode-stub.js";
import * as assert from "node:assert/strict";
import { test } from "node:test";
import { inertError } from "../src/extension.js";
import { ProcessFailure } from "../src/process.js";

test("VSC-V0-077: process errors display only the first non-empty sanitized stderr line", () => {
  const stderr = Buffer.from("\r\n \t\r\n\u001b[31mCLI detail\u001b[0m\u202e\u0000\nsecond line");
  assert.equal(inertError(new ProcessFailure("nonzero-exit", "ignored", stderr)), "CLI_FAILED: CLI detail��");
  assert.equal(inertError(new ProcessFailure("timed-out", "ignored", Buffer.from("\rreason\rlater"))), "TIMEOUT: reason");
});

test("VSC-V0-077: malformed UTF-8 is replaced and stderr is bounded by Unicode scalars", () => {
  assert.equal(inertError(new ProcessFailure("nonzero-exit", "ignored", new Uint8Array([0xff, 0xe2, 0x82]))), "CLI_FAILED: ��");
  const detail = inertError(new ProcessFailure("nonzero-exit", "ignored", Buffer.from("😀".repeat(201))));
  assert.equal(detail, `CLI_FAILED: ${"😀".repeat(199)}…`);
});

test("VSC-V0-077: absent or blank stderr retains the stable fallback", () => {
  for (const stderr of [undefined, Buffer.from(""), Buffer.from(" \t\r\n\n")]) {
    assert.equal(inertError(new ProcessFailure("nonzero-exit", "ignored", stderr)), "CLI_FAILED: local Corvint process failed");
  }
});
