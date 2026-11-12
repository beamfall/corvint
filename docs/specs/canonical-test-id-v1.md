# Canonical test ID V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: partial
Health: KEEP
Authoritative inputs: change folder
`docs/specs/openspec/changes/archive/2026-11-12-canonical-test-id/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/canonical-test-id/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-005, PROD-TEST-006, PROD-TEST-010, PROD-TEST-018, PROD-TEST-020) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R11, R32 and R38).

## Agent digest
- Claim: One canonical ID names every test of the seven blocking runners, so plan, receipt and watch output join on the same string.
- Status: accepted / partial. Change `canonical-test-id` auto-accepted under G14 (decision 0596); delivered under V1-5398.
- Exists: `internal/testid`; `testId` on `corvint-test-run/0` outcomes; `tiers.testIds` in `affected-summary/1` and `corvint.affected`; `TestIDs` in `corvint watch`.
- Blocked on: run evidence, line evidence and flow links adopt the ID in their own R38 changes; RSpec test-level names need the group chain.
- Read next: Surfaces; Traceability.

## User and boundary

An agent or reviewer joins a plan with a receipt, run evidence, line evidence, flow links and watch
output. Before this change every surface had its own key: Go units by import path, JavaScript and
TypeScript units by `typescript:<runner>:<path>`, Ruby units by `ruby:<kind>:<stem>`, the Go line
index by package plus name, and the Ruby hook by `Class#method` or the RSpec example id. No rule
said how titles, parametrised or nested tests, or duplicates are named, or how a file-level
selection matches test-level evidence.

This spec is the canonical home of requirements CTI-V1-001 to CTI-V1-012, copied from the change's
`specs/canonical-test-id/spec.md`. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds only the delivery record (Surfaces, Traceability).
RNC-V1-012 in `runner-contract` points here and adds no rule of its own.

## Requirements

- `CTI-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) One ID form for every test

Every test of the seven blocking runners MUST have one canonical ID of the form
`<runner>:<file>::<segment>[::<segment>...]`, where `<runner>` is one of `go`, `vitest`, `jest`,
`playwright`, `pytest`, `rspec`, `minitest`, `<file>` is a clean repository-relative slash path (the
package directory for Go), and the segments are the runner-native name path. A file-level ID MUST be
`<runner>:<file>` with no segments.

**Scenario: Jest test inside a describe block**
- GIVEN `web/sum.test.js` has `describe("sum")` containing `test("adds")`
- WHEN Corvint names that test
- THEN its ID is `jest:web/sum.test.js::sum::adds`

**Scenario: Go subtest**
- GIVEN package directory `internal/calc` has `TestAdd` with subtest `negative`
- WHEN Corvint names that subtest
- THEN its ID is `go:internal/calc::TestAdd::negative`

- `CTI-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) Escape rule

In `<file>` and in every segment, `%` MUST be written `%25`, `:` MUST be written `%3A`, and
every control character or invalid UTF-8 byte MUST be written `%XX` in upper-case hexadecimal. No other
character is escaped. Parsing an ID and formatting it again MUST give the same bytes.

**Scenario: title containing a colon**
- GIVEN a Vitest test titled `GET /a:b`
- WHEN Corvint names it
- THEN the last segment is `GET /a%3Ab` and the ID parses back to the same title

- `CTI-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) Go name path

For Go, `<file>` MUST be the package directory relative to the repository root, and the segments
MUST be the top-level `Test`, `Fuzz` or `Example` function name followed by each subtest name that
`go test -json` reports, split on `/`.

**Scenario: nested subtests**
- GIVEN `go test -json` reports `TestParse/empty/utf8` in package directory `pkg/p`
- WHEN Corvint names it
- THEN its ID is `go:pkg/p::TestParse::empty::utf8`

- `CTI-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) Vitest and Jest name path

For Vitest and Jest, `<file>` MUST be the test file and the segments MUST be the report's
`ancestorTitles` in order followed by its `title`, after the runner's own interpolation of
`test.each` titles.

**Scenario: parametrised Vitest test**
- GIVEN `test.each([1, 2])("case %i", ...)` in `src/a.test.ts`
- WHEN Corvint names the two tests
- THEN their IDs are `vitest:src/a.test.ts::case 1` and `vitest:src/a.test.ts::case 2`

- `CTI-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) Playwright name path

For Playwright, `<file>` MUST be the spec file and the segments MUST be the project name (an empty
segment when the configuration defines no projects), then each `describe` title, then the test
title.

**Scenario: same test in two projects**
- GIVEN `e2e/login.spec.ts` has `test("signs in")` and the config defines projects `chromium`
  and `webkit`
- WHEN Corvint names the tests
- THEN the IDs are `playwright:e2e/login.spec.ts::chromium::signs in` and
  `playwright:e2e/login.spec.ts::webkit::signs in`

- `CTI-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) pytest name path

For pytest, `<file>` MUST be the file part of the node ID and the segments MUST be the remaining
`::`-separated parts of the node ID. A `::` inside a trailing `[...]` parameter part MUST NOT split
a segment.

**Scenario: parametrised method in a class**
- GIVEN the node ID `tests/test_a.py::TestA::test_b[x::y]`
- WHEN Corvint names it
- THEN its ID is `pytest:tests/test_a.py::TestA::test_b[x%3A%3Ay]`

- `CTI-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) RSpec and Minitest name path

For RSpec, `<file>` MUST be the example's rerun file path and the segments MUST be each example group
description followed by the example description. For Minitest, `<file>` MUST be the source file of
the test method and the segments MUST be the class name followed by the method name.

**Scenario: Minitest method**
- GIVEN `test/calc_test.rb` defines `CalcTest#test_add`
- WHEN Corvint names it
- THEN its ID is `minitest:test/calc_test.rb::CalcTest::test_add`

- `CTI-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) File-level selection matches every test under the file

A selection of a file-level ID `<runner>:<file>` MUST match every ID that starts with
`<runner>:<file>::`. A selection of a runner instance MUST match every ID of that runner under the
instance root. Matching MUST compare escaped forms.

**Scenario: evidence for one test, selection by file**
- GIVEN a plan selects `rspec:spec/user_spec.rb` and the receipt has outcomes for three examples
  in that file
- WHEN a report joins the plan with the receipt
- THEN all three outcomes are counted as selected

- `CTI-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) One ID everywhere

The plan from `corvint affected`, the `corvint-test-run/0` receipt (as an added `testId` field per
outcome under settled D25), run evidence, line evidence, flow link records, `corvint watch` output
and MCP results MUST use the same ID string for the same test. No surface may define its own key.

**Scenario: plan, receipt and watch agree**
- GIVEN a change that selects one Jest test
- WHEN the plan, the receipt and watch output are compared
- THEN each names the test with the identical ID

- `CTI-V1-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) Duplicate IDs widen to the file

When two or more tests in one listing or report produce the same ID, the second and later ones MUST
get a final segment `#2`, `#3` in report order and every ID of the group MUST be marked
`duplicate`. A selection that names a duplicated ID MUST select the whole file and print reason
`duplicate-test-id`.

**Scenario: two equal titles**
- GIVEN `a.test.js` has two top-level tests both titled `works`
- WHEN a change selects one of them
- THEN the plan selects `jest:a.test.js` with reason `duplicate-test-id`

- `CTI-V1-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) Unmappable tests run with a named reason

An ID with an unknown runner token, an ID that does not parse, or a selection that cannot be
expressed in IDs MUST NOT be matched by guess. It MUST widen to the file when the file is known,
otherwise to every test of the runner, and print reason `test-id-unmapped`. A record keyed by an ID
that no longer exists (for example after a rename) MUST be reported as dangling and MUST NOT narrow
any selection.

**Scenario: renamed test**
- GIVEN line evidence keyed by `vitest:a.test.ts::old name` and the test is renamed `new name`
- WHEN a change touches a line that evidence covered
- THEN the dangling record is reported and `a.test.ts` is selected at file level

- `CTI-V1-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5398) Legacy keys are read at their own granularity

A receipt outcome with unit `go:<package>` MUST be read as the Go package-level ID, a unit
`typescript:<runner>:<path>` MUST be read as the file-level ID `<runner>:<path>`, and an outcome
without `testId` MUST be read at the granularity of its unit. Stored records MUST NOT be rewritten.

**Scenario: receipt written before this change**
- GIVEN a receipt from integ 9ec3a29d0 with unit `typescript:vitest:src/a.test.ts`
- WHEN a report reads it
- THEN the outcome is joined as `vitest:src/a.test.ts` at file granularity

## Surfaces

Delivered by ticket V1-5398 (build-log `docs/build-log/2026-11-11-canonical-test-id.md`):

- `internal/testid` (layer 0, no Corvint imports) formats, parses, escapes and matches IDs
  (CTI-V1-001, CTI-V1-002, CTI-V1-008), maps each runner's machine report to IDs (CTI-V1-003 to
  CTI-V1-007), disambiguates duplicates and resolves a requested ID with its widening reason
  (CTI-V1-010, CTI-V1-011), and reads legacy unit keys (CTI-V1-012).
- The `corvint-test-run/0` receipt: each outcome carries `testId`, the ID its unit names, an added
  omitempty field (settled D25). The unit of `corvint test` is a Go package or a test file, so the
  ID is the package-level or file-level one; a selected Go slice keeps its package ID and its
  `slice`. A Go unit resolves through the root `go.mod`; a `python:<module>` unit and a
  nested-module `go-module:<dir>` unit name no file and carry no `testId`. `Outcome.ReadTestID`
  reads a receipt without the field at its unit's granularity (CTI-V1-012).
- The plan: `corvint affected --summary` (`affected-summary/1`) and the `corvint.affected` MCP tool,
  which returns that verdict, carry `tiers.testIds` for the tier-0 and tier-1 units, derived as the
  receipt derives `testId`. The frozen `affected-plan/0` and `affected-plan/1` documents are not
  extended: CCF-V1-006 admits an optional member only with a regenerated Core golden.
- `corvint watch`: each planned invocation carries `TestIDs`, the package-level ID of a whole
  package or one ID per kept top-level test; a compile-only run names none.

Per runner, where a surface exists on this tree:

| Runner | Report mapping | Plan and MCP | Receipt | Watch |
| --- | --- | --- | --- | --- |
| Go | `go test -json` | package ID | package ID | package or test IDs |
| Vitest | JSON reporter | file ID | file ID (provider adapter) | NOT_APPLICABLE: `corvint watch` refuses a non-Go index |
| Jest | `--json` | file ID | file ID (declared adapter) | NOT_APPLICABLE: as Vitest |
| Playwright | JSON reporter | file ID | file ID (provider adapter) | NOT_APPLICABLE: as Vitest |
| pytest | pytest-json-report node IDs | NOT_APPLICABLE: a `python:<module>` unit names no file | NOT_APPLICABLE: as plan | NOT_APPLICABLE: as Vitest |
| RSpec | file ID with `test-id-unmapped`: the JSON report carries no example group chain | file ID | file ID (declared adapter) | NOT_APPLICABLE: as Vitest |
| Minitest | Corvint hook records | file ID | file ID (declared adapter) | NOT_APPLICABLE: as Vitest |

Run evidence, line evidence and flow link records do not exist on this tree; the later changes of
the R38 order key them by this ID. RSpec examples are named at test level once a report carries the
group chain (`RSpecExample` implements the rule).

## Traceability

| Requirement | Evidence |
|---|---|
| CTI-V1-001 | `TestCTIV1001FormatParseRoundTrip`, `TestCTIV1001ParseRefusesMalformed` (`internal/testid`) |
| CTI-V1-002 | `TestCTIV1002EscapeRoundTrip` (`internal/testid`) |
| CTI-V1-003 | `TestCTIV1003GoTestJSON` (`internal/testid/testdata/go-test.jsonl`, a live `go test -json` recording) |
| CTI-V1-004 | `TestCTIV1004VitestAndJestJSON` (`vitest-mixed.json`, a recorded Vitest run; `jest.json`) |
| CTI-V1-005 | `TestCTIV1005PlaywrightJSON` (`playwright-mixed.json`, a recorded run; `playwright-projects.json`) |
| CTI-V1-006 | `TestCTIV1006PytestNodeIDs` (`pytest.json`) |
| CTI-V1-007 | `TestCTIV1007RSpecAndMinitest` (`rspec.json`, `minitest.jsonl`) |
| CTI-V1-008 | `TestCTIV1008FileLevelMatch` (`internal/testid`) |
| CTI-V1-009 | `TestCTIV1009GoPlanReceiptAndWatchAgree`, `TestCTIV1009PlanAndReceiptAgreePerRunner` (`cmd/corvint`); `TestCTIV1009GoUnitDirFromRootModule` (`internal/sessiontest`); `TestSTRV0010CLIRunWritesReceipt`, `TestSTRV08704CLIMixedGoAndPlaywrightRun` read `testId` back from written receipts |
| CTI-V1-010 | `TestCTIV1010DuplicateJestTitlesWidenToFile` (`internal/testid`) |
| CTI-V1-011 | `TestCTIV1011UnknownAndMalformedWidenWithReason`, `TestCTIV1007RSpecAndMinitest` (`internal/testid`) |
| CTI-V1-012 | `TestCTIV1012LegacyUnitKeys` (`internal/testid`), `TestCTIV1012ReceiptWithAndWithoutTestID` (`internal/sessiontest`) |

## Non-goals

No change to how any runner selects or runs tests (`runner-contract`); no inventory file,
run-evidence store, line index or flow link schema (the later changes in the R38 order); no rename
tracking across commits; no reuse of AFU-V1-140 to AFU-V1-145 or AFU-V1-150 to AFU-V1-154 (owner
decision R11); no runner beyond the seven blocking runners (G26, owner decision R6).

## Rollback

The ID is an added field (`testId`) and an added package. Reverting the change's commits returns
every reader to the `unit` key, which is unchanged; no stored record has to be rewritten. CTI-V1-001
to CTI-V1-012 stay listed as retired if the change is withdrawn and are never reused.
