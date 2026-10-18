# 2026-10-18 V1-1858: JS/TS per-example hygiene and strength in test-validity

Ticket: V1-1858 (`corvint tests validity` gives JS/TS no per-example facts; RSpec has them since
V1-1798). GitHub #1159.

## Decision

`TSA-V0-073` to `TSA-V0-083` are added to `docs/specs/test-source-analysis-v0.md`. They are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11). They mirror the
RSpec readings `TSA-V0-062..072` for Jest/Vitest/Mocha/Playwright-shaped JS/TS test files.

- One reader (`internal/testsource/validity_js_examples.go`) reuses `parseSource`, the
  `appmap.LexJS` token stream and the existing helper definitions. It adds no second parser. It
  returns one `ExampleSource` per example, keyed by its callee's start line.
- Run state (TSA-V0-074..076):
  - `JS-SKIPPED` covers `skip`, `xit`/`xtest`, `xdescribe` and `describe.skip` ancestors.
  - `JS-TODO` covers `.todo` and bodyless examples.
  - `JS-FOCUSED` covers `only`/`fit`/`fdescribe`, which narrows the run to itself.
  - All three are invalid, so the example is `INELIGIBLE`. A focused example in the file does not
    relax its siblings.
  - `.each`/`.for` tables give the advisory `JS-EACH-ROW-BINDING`, with the row count when the
    table is a literal array.
- Assertions (TSA-V0-077/078): `JS-NO-ASSERTION` follows helpers defined in the file, up to depth
  3, as `MaxHelperDepth` does for RSpec.
  - The finding is withheld, not guessed, when the body calls a name imported from a relative,
    non-subject module. That helper may assert in a file this reader does not read.
  - `JS-CONSTANT-EXPECTATION` covers every expectation whose subject and matcher arguments are
    literals or constant-bound names.
- Mocks (TSA-V0-079):
  - `jest.mock`/`vi.mock` of a subject module (the spec-stem match, or the module the test
    imports names from) gives `JS-SUBJECT-MOCKED`.
  - Network, filesystem, clock and process modules, globals and timers give `JS-BOUNDARY-MOCKED`.
  - Both are advisory.
- Strength (TSA-V0-080) is always `NOT_MEASURED`, with the first reason in this order: subject
  mocked, no expectation, constant only, boundary mocked, else the body's expectation anchors.
- The CLI (TSA-V0-081) lists every JS/TS example in line order with hygiene and a `strength`
  member. Findings that name no example keep their TSA-V0-071 test.
- Receipt discovery (TSA-V0-082) reads the anchored file through the confined `os.Root` reader.
  It uses the file only when its SHA-256 equals the retained `testFileDigests` entry, else
  `js-source-not-bound`.
- Unsupported shapes abstain with a named reason (TSA-V0-073/083): `js-shape-unsupported`,
  `js-body-not-literal`, `js-line-ambiguous` and `js-example-not-located`. An uncertain file keeps
  its TSA-V0-071 behaviour.

Implementation notes:

- The first file name, `source_rules_js.go`, is a `GOOS=js` build constraint. It was renamed
  `source_rules_jsts.go`.
- `RubySourceReader` became `ExampleSourceReader`, and `testsource.ExampleSources` dispatches on
  `_spec.rb`.
- `ExampleSource.EndLine` (not serialized) lets discovery place an anchor that points inside an
  example.

Requirement IDs: TSA-V0 on origin/main and the open PRs stopped at `TSA-V0-072`.

## Evidence

Real CLI built from this branch, against a scratch repository with one committed `src/cart.test.ts`
(`jest.mock("axios")`, then `it`, `it.skip`, `it.todo`, a constant `expect(1).toBe(1)`, an
assertion-free example and an `it.each` table). `corvint tests validity --files src/cart.test.ts`
exits 1 and gives:

| Line | Hygiene | Findings | Strength |
|---|---|---|---|
| 5 | ELIGIBLE | | NOT_MEASURED `boundary-mocked` (anchor `:3`) |
| 6 | INELIGIBLE `JS-SKIPPED` | | |
| 7 | INELIGIBLE `JS-TODO` | | |
| 8 | | `JS-CONSTANT-EXPECTATION` | `constant-expectations-only` |
| 9 | | `JS-NO-ASSERTION` | `no-expectation-in-example-body` |
| 10 | | `JS-EACH-ROW-BINDING` (2 rows) | |

Focused tests:

- `./internal/testsource`, `./internal/testvalidity`, `./internal/testvaliditydoc` and
  `./internal/mcp/testvaliditybridge`: the whole packages.
- `./cmd/corvint`: `-run 'TestTestsValidityCLI|TestTestValidity|TestTestsCLI|TestTestsLint'`.

## Residuals

- Helpers imported from other files are not followed. A call to one withholds `JS-NO-ASSERTION`
  rather than reading the other file.
- Conditional skips (`test.skipIf`, `test.skip(cond)` inside a body) abstain or are not reported.
- Mocks set up in helpers, `__mocks__` directories or setup files are not seen.
- Subject detection does not follow re-exports or aliases.
- Discovery binds only absolute `testFileDigests` keys inside the root.
- `corvint tests validity` lists every example for JS/TS but only finding-named tests for RSpec.
