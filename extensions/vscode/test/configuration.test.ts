import * as assert from "node:assert/strict";
import { test } from "node:test";
import { stub } from "./vscode-stub.js";
import { configurationValue } from "../src/configuration.js";

const resource = { fsPath: "/workspace", scheme: "file" } as never;

test("configuration precedence reads the single corvint namespace (CRB-V0-013 VSC-V0-011 VSC-V0-046)", () => {
  stub.settings.clear();
  stub.defaults.clear();
  stub.defaults.set("corvint.transport", "cli");
  assert.equal(configurationValue("transport", "cli", resource), "cli");

  stub.settings.set("corvint.transport", "mcp-stdio");
  assert.equal(configurationValue("transport", "cli", resource), "mcp-stdio");

  stub.settings.set("corvint.transport", "cli");
  assert.equal(configurationValue("transport", "mcp-stdio", resource), "cli");

  stub.settings.clear();
  assert.equal(configurationValue("transport", "mcp-stdio", resource), "mcp-stdio");
  stub.defaults.clear();
});

test("array configuration is returned verbatim or falls back (CRB-V0-013 VSC-V0-058)", () => {
  stub.settings.clear();
  assert.deepEqual(configurationValue<readonly string[]>("liveTests.command", [], resource), []);
  stub.settings.set("corvint.liveTests.command", ["/bin/provider", "unit"]);
  assert.deepEqual(configurationValue<readonly string[]>("liveTests.command", [], resource), ["/bin/provider", "unit"]);
  stub.settings.set("corvint.liveTests.command", ["/bin/provider", "e2e"]);
  assert.deepEqual(configurationValue<readonly string[]>("liveTests.command", [], resource), ["/bin/provider", "e2e"]);
  stub.settings.clear();
});
