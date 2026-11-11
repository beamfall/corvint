# Canonical test ID V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: proposed
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/canonical-test-id/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/canonical-test-id/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-005, PROD-TEST-006, PROD-TEST-010, PROD-TEST-018, PROD-TEST-020) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R11, R32 and R38).

## Agent digest
- Claim: One canonical ID names every test of the seven blocking runners, so plan, receipt and watch output join on the same string.
- Status: proposed / not-started. Change `canonical-test-id` is first in the R38 testing order.
- Exists: the change folder; ticket V1-5398 delivers it.
- Blocked on: nothing (R38 puts this change first).
- Read next: Requirements; Surfaces; Traceability.

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

- `CTI-V1-001`: (proposed 2026-11-09; delivery not-started) One ID form for every test

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

- `CTI-V1-002`: (proposed 2026-11-09; delivery not-started) Escape rule

In `<file>` and in every segment, `%` MUST be written `%25`, `:` MUST be written `%3A`, and
every control character or invalid UTF-8 byte MUST be written `%XX` in upper-case hexadecimal. No other
character is escaped. Parsing an ID and formatting it again MUST give the same bytes.

**Scenario: title containing a colon**
- GIVEN a Vitest test titled `GET /a:b`
- WHEN Corvint names it
- THEN the last segment is `GET /a%3Ab` and the ID parses back to the same title

- `CTI-V1-003`: (proposed 2026-11-09; delivery not-started) Go name path

For Go, `<file>` MUST be the package directory relative to the repository root, and the segments
MUST be the top-level `Test`, `Fuzz` or `Example` function name followed by each subtest name that
`go test -json` reports, split on `/`.

**Scenario: nested subtests**
- GIVEN `go test -json` reports `TestParse/empty/utf8` in package directory `pkg/p`
- WHEN Corvint names it
- THEN its ID is `go:pkg/p::TestParse::empty::utf8`

- `CTI-V1-004`: (proposed 2026-11-09; delivery not-started) Vitest and Jest name path

For Vitest and Jest, `<file>` MUST be the test file and the segments MUST be the report's
`ancestorTitles` in order followed by its `title`, after the runner's own interpolation of
`test.each` titles.

**Scenario: parametrised Vitest test**
- GIVEN `test.each([1, 2])("case %i", ...)` in `src/a.test.ts`
- WHEN Corvint names the two tests
- THEN their IDs are `vitest:src/a.test.ts::case 1` and `vitest:src/a.test.ts::case 2`

- `CTI-V1-005`: (proposed 2026-11-09; delivery not-started) Playwright name path

For Playwright, `<file>` MUST be the spec file and the segments MUST be the project name (an empty
segment when the configuration defines no projects), then each `describe` title, then the test
title.

**Scenario: same test in two projects**
- GIVEN `e2e/login.spec.ts` has `test("signs in")` and the config defines projects `chromium`
  and `webkit`
- WHEN Corvint names the tests
- THEN the IDs are `playwright:e2e/login.spec.ts::chromium::signs in` and
  `playwright:e2e/login.spec.ts::webkit::signs in`

- `CTI-V1-006`: (proposed 2026-11-09; delivery not-started) pytest name path

For pytest, `<file>` MUST be the file part of the node ID and the segments MUST be the remaining
`::`-separated parts of the node ID. A `::` inside a trailing `[...]` parameter part MUST NOT split
a segment.

**Scenario: parametrised method in a class**
- GIVEN the node ID `tests/test_a.py::TestA::test_b[x::y]`
- WHEN Corvint names it
- THEN its ID is `pytest:tests/test_a.py::TestA::test_b[x%3A%3Ay]`

- `CTI-V1-007`: (proposed 2026-11-09; delivery not-started) RSpec and Minitest name path

For RSpec, `<file>` MUST be the example's rerun file path and the segments MUST be each example group
description followed by the example description. For Minitest, `<file>` MUST be the source file of
the test method and the segments MUST be the class name followed by the method name.

**Scenario: Minitest method**
- GIVEN `test/calc_test.rb` defines `CalcTest#test_add`
- WHEN Corvint names it
- THEN its ID is `minitest:test/calc_test.rb::CalcTest::test_add`

- `CTI-V1-008`: (proposed 2026-11-09; delivery not-started) File-level selection matches every test under the file

A selection of a file-level ID `<runner>:<file>` MUST match every ID that starts with
`<runner>:<file>::`. A selection of a runner instance MUST match every ID of that runner under the
instance root. Matching MUST compare escaped forms.

**Scenario: evidence for one test, selection by file**
- GIVEN a plan selects `rspec:spec/user_spec.rb` and the receipt has outcomes for three examples
  in that file
- WHEN a report joins the plan with the receipt
- THEN all three outcomes are counted as selected

- `CTI-V1-009`: (proposed 2026-11-09; delivery not-started) One ID everywhere

The plan from `corvint affected`, the `corvint-test-run/0` receipt (as an added `testId` field per
outcome under settled D25), run evidence, line evidence, flow link records, `corvint watch` output
and MCP results MUST use the same ID string for the same test. No surface may define its own key.

**Scenario: plan, receipt and watch agree**
- GIVEN a change that selects one Jest test
- WHEN the plan, the receipt and watch output are compared
- THEN each names the test with the identical ID

- `CTI-V1-010`: (proposed 2026-11-09; delivery not-started) Duplicate IDs widen to the file

When two or more tests in one listing or report produce the same ID, the second and later ones MUST
get a final segment `#2`, `#3` in report order and every ID of the group MUST be marked
`duplicate`. A selection that names a duplicated ID MUST select the whole file and print reason
`duplicate-test-id`.

**Scenario: two equal titles**
- GIVEN `a.test.js` has two top-level tests both titled `works`
- WHEN a change selects one of them
- THEN the plan selects `jest:a.test.js` with reason `duplicate-test-id`

- `CTI-V1-011`: (proposed 2026-11-09; delivery not-started) Unmappable tests run with a named reason

An ID with an unknown runner token, an ID that does not parse, or a selection that cannot be
expressed in IDs MUST NOT be matched by guess. It MUST widen to the file when the file is known,
otherwise to every test of the runner, and print reason `test-id-unmapped`. A record keyed by an ID
that no longer exists (for example after a rename) MUST be reported as dangling and MUST NOT narrow
any selection.

**Scenario: renamed test**
- GIVEN line evidence keyed by `vitest:a.test.ts::old name` and the test is renamed `new name`
- WHEN a change touches a line that evidence covered
- THEN the dangling record is reported and `a.test.ts` is selected at file level

- `CTI-V1-012`: (proposed 2026-11-09; delivery not-started) Legacy keys are read at their own granularity

A receipt outcome with unit `go:<package>` MUST be read as the Go package-level ID, a unit
`typescript:<runner>:<path>` MUST be read as the file-level ID `<runner>:<path>`, and an outcome
without `testId` MUST be read at the granularity of its unit. Stored records MUST NOT be rewritten.

**Scenario: receipt written before this change**
- GIVEN a receipt from integ 9ec3a29d0 with unit `typescript:vitest:src/a.test.ts`
- WHEN a report reads it
- THEN the outcome is joined as `vitest:src/a.test.ts` at file granularity

## Non-goals

No change to how any runner selects or runs tests (`runner-contract`); no inventory file,
run-evidence store, line index or flow link schema (the later changes in the R38 order); no rename
tracking across commits; no reuse of AFU-V1-140 to AFU-V1-145 or AFU-V1-150 to AFU-V1-154 (owner
decision R11); no runner beyond the seven blocking runners (G26, owner decision R6).

## Rollback

The ID is an added field (`testId`) and an added package. Reverting the change's commits returns
every reader to the `unit` key, which is unchanged; no stored record has to be rewritten. CTI-V1-001
to CTI-V1-012 stay listed as retired if the change is withdrawn and are never reused.
