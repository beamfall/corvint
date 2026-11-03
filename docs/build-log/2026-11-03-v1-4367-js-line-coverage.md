# JavaScript and TypeScript per-test line coverage (V1-4367)

Date: 2026-11-03. Ticket: V1-4367. Requirements: LPCV-V0-965..969 (new), LPCV-V0-955, LPCV-V0-962.

## Decision

The node runners (Mocha, Jasmine, `node:test`, Jest, Vitest, AVA, `bun test`, `deno test`) get one
collector, `internal/jslinecov`, registered as the typescript plugin's `linecov-js` Coverage
sub-plugin. It writes the shared `internal/linecov` record format and index (`linecov-index/2`)
rather than a JavaScript fork. The format change is one line: a profile line may now name any
extension. The converters live in `internal/linecov/v8`, so the browser E2E collector (V1-4368)
can reuse them.

Isolation (LPCV-V0-955) is one process per test under the runner's own anchored title filter,
after one in-order run per file. A run is trusted only when it reports exactly one passing test
with the requested title; anything else is `isolation-divergent`. Each runner keeps its native
coverage mode: `NODE_V8_COVERAGE` for node, Mocha, Jasmine and AVA; Istanbul JSON from the v8
provider for Jest and Vitest; and LCOV for Bun and Deno. LCOV from a generated file is mapped
through its `sourceMappingURL`, because Bun's report names `out/calc.js`, not `src/calc.ts`.

Selection (LPCV-V0-966) uses a lexical body scanner, not a parser. It refuses input it cannot
read, such as JSX, unbalanced braces or unterminated literals, and refusal falls back to the static
plan, so the scanner can only lose precision, never soundness.

## Evidence

- `go test ./internal/jslinecov` passes for all eight runners against `testdata/repo`, which uses
  CommonJS, a TypeScript source behind an esbuild source map, and ESM for AVA.
  - On WSL, node-test, Mocha, Jasmine, Jest, Vitest and AVA ran from `JSLINECOV_NODE_MODULES`.
  - On the Mac, node-test, Mocha, Bun and Deno ran.
- `go test ./internal/linecov/v8` passes, including `TestAddLCOVSourceMap`.

## Limits

- Bun marks an uncalled function's closing line executed. This over-approximation only widens
  selection, so the fixture's negative line checks are relaxed for Bun.
- When `-t` narrows a file, Jest reports the suite status `focused`, which the shared dynamic parser
  does not know. The collector drops that one problem on filtered Jest runs only.
- No command builds a JavaScript index yet, and the family has no LPCV-V0-963 qualification, so
  the static plan still decides every JavaScript change (LPCV-V0-969).

Rollback: delete the `linecov-js` Coverage entry and restore `Absent: {Coverage: noCollector}`.
