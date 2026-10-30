import * as vscode from "vscode";
import type { ImportedObservation, ObservedTest } from "./observations.js";

export class ObservationTestingBridge implements vscode.Disposable {
  private readonly controller = vscode.tests.createTestController("corvint.observations", "Corvint Observations");

  import(observation: ImportedObservation): void {
    this.controller.items.replace([]);
    const items = observation.tests.map((test) => this.item(test, observation));
    this.controller.items.replace(items);
    const run = this.controller.createTestRun(
      new vscode.TestRunRequest(items),
      `Corvint ${observation.runId} · UNVERIFIED_IMPORT · not current`,
      false,
    );
    run.appendOutput(`Corvint observation only; authority is not upgraded. Source currency at capture: ${observation.sourceCurrency}. Toolchain currency at capture: ${observation.toolchainCurrency}.\r\n`);
    if (observation.cancelled) {
      run.appendOutput("Corvint observed cancellation; imported items are skipped and are not passing results.\r\n");
    }
    for (let index = 0; index < observation.tests.length; index += 1) {
      const test = observation.tests[index];
      const item = items[index];
      if (test === undefined || item === undefined) {
        continue;
      }
      switch (test.status) {
        case "PASSED":
          run.passed(item);
          break;
        case "FAILED":
          run.failed(item, new vscode.TestMessage("Corvint observed a failed terminal result; runner output is not retained."));
          break;
        case "SKIPPED":
          run.skipped(item);
          break;
        case "ERRORED":
          run.errored(item, new vscode.TestMessage("The imported result is incomplete, stale, unknown, or inconsistent; it is not a passing result."));
          break;
      }
    }
    run.end();
  }

  clear(): void {
    this.controller.items.replace([]);
  }

  dispose(): void {
    this.clear();
    this.controller.dispose();
  }

  private item(test: ObservedTest, observation: ImportedObservation): vscode.TestItem {
    const item = this.controller.createTestItem(test.id, test.id);
    item.description = `${test.status} observed · ${observation.verification} · not current`;
    return item;
  }
}
