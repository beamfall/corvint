/**
 * A minimal in-process stand-in for the `vscode` module, covering only the
 * API surface liveTests.ts touches. Importing this file before any module
 * that imports `vscode` resolves that import to the stub; no VS Code or
 * Electron process is ever started.
 */
import * as nodeModule from "node:module";

export interface RecordedRun {
  readonly name: string;
  ended: boolean;
  results: { id: string; state: string; message?: unknown }[];
}

export const stub = {
  settings: new Map<string, unknown>(),
  defaults: new Map<string, unknown>(),
  runs: [] as RecordedRun[],
  items: new Map<string, { id: string }>(),
  diagnostics: new Map<string, unknown[]>(),
  trusted: true,
};

const api = {
  tests: {
    createTestController: () => ({
      items: { add: (item: { id: string }) => stub.items.set(item.id, item), delete: (id: string) => stub.items.delete(id) },
      createTestItem: (id: string, label: string, uri?: unknown) => ({ id, label, uri, range: undefined }),
      createTestRun: (_request: unknown, name: string) => {
        const run: RecordedRun = { name, ended: false, results: [] };
        stub.runs.push(run);
        const ignore = (): void => undefined;
        const result = (state: string) => (item: { id: string }, message?: unknown): void => { run.results.push({ id: item.id, state, message }); };
        return { started: result("started"), passed: result("passed"), failed: result("failed"), errored: result("errored"), skipped: result("skipped"), appendOutput: ignore, end: () => { run.ended = true; } };
      },
      dispose: () => undefined,
    }),
  },
  languages: {
    createDiagnosticCollection: () => ({ set: (uri: { fsPath: string }, values: unknown[]) => stub.diagnostics.set(uri.fsPath, values), clear: () => stub.diagnostics.clear(), dispose: () => undefined }),
  },
  workspace: {
    get isTrusted() { return stub.trusted; },
    getConfiguration: (namespace: string) => ({
      get: (key: string, fallback: unknown) => {
        const qualified = `${namespace}.${key}`;
        if (stub.settings.has(qualified)) return stub.settings.get(qualified);
        return stub.defaults.has(qualified) ? stub.defaults.get(qualified) : fallback;
      },
      inspect: (key: string) => {
        const qualified = `${namespace}.${key}`;
        return {
          key: qualified,
          ...(stub.defaults.has(qualified) ? { defaultValue: stub.defaults.get(qualified) } : {}),
          ...(stub.settings.has(qualified) ? { workspaceFolderValue: stub.settings.get(qualified) } : {}),
        };
      },
    }),
  },
  Uri: { file: (fsPath: string) => ({ fsPath, scheme: "file" }) },
  DiagnosticSeverity: { Error: 0 },
  TestRunRequest: class {},
  TestMessage: class {
    constructor(readonly message: string) {}
  },
  Range: class {},
  Diagnostic: class {},
};

const STUB_ID = "corvint-vscode-test-stub";
const loader = nodeModule.Module as unknown as { _resolveFilename: (request: string, ...rest: unknown[]) => string };
const resolve = loader._resolveFilename;
loader._resolveFilename = (request: string, ...rest: unknown[]) => (request === "vscode" ? STUB_ID : resolve(request, ...rest));
const stubModule = new nodeModule.Module(STUB_ID);
stubModule.exports = api;
stubModule.loaded = true;
require.cache[STUB_ID] = stubModule;
