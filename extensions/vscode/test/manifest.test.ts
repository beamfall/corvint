import * as assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import * as path from "node:path";
import { test } from "node:test";

// VSC-V0-079 (V1-0438): observation import is not offered until a shared
// Corvint go-live verifier is linked.
test("manifest offers no Test Observation import command, activation or setting", async () => {
  const manifestPath = path.join(__dirname, "..", "..", "package.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf-8")) as {
    activationEvents: string[];
    capabilities: { untrustedWorkspaces: { restrictedConfigurations: string[] } };
    contributes: { commands: { command: string }[]; configuration: { properties: Record<string, unknown> } };
  };
  assert.deepEqual(manifest.contributes.commands.map((entry) => entry.command), [
    "corvint.selectExecutable",
    "corvint.clearExecutable",
    "corvint.query",
    "corvint.impact",
    "corvint.refresh",
    "corvint.clearResults",
  ]);
  assert.ok(!manifest.activationEvents.includes("onCommand:corvint.importTestObservation"));
  assert.ok(!("corvint.testObservationPath" in manifest.contributes.configuration.properties));
  assert.ok(!manifest.capabilities.untrustedWorkspaces.restrictedConfigurations.includes("corvint.testObservationPath"));
});
