# Change proposal: one canonical test ID for the seven blocking runners

Change name: `canonical-test-id`
Ticket: none filed yet; to be filed in `.taskman/` before dispatch (not filed by this proposal).
Product requirement served: PROD-TEST-005, PROD-TEST-006, PROD-TEST-010, PROD-TEST-018 and
PROD-TEST-020 in `docs/specs/PRODUCT-PLAN-1.0.md` (every join between plan, receipt, run evidence,
line evidence, flow links and watch).
Owning capability spec: a new spec `docs/specs/canonical-test-id-v1.md` (prefix `CTI-V1`). It does
not exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: CTI-V1-001 to CTI-V1-012. `CTI-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and appears in no other folder under `docs/specs/openspec/changes/`
(checked 2026-11-09).
Relation to RNC-V1-012: RNC-V1-012 in `runner-contract` is the canonical test ID. Its definition is
CTI-V1-001 to CTI-V1-012 in this change; RNC-V1-012 points here and adds no rule of its own. This
folder does not edit `runner-contract`.
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.

## Needs first

Nothing. This change is first in the testing build order fixed by owner decision R38 (2026-11-09):
canonical test ID, then inventory, then run results, then line evidence, then flow links. Every
later change in that order keys its records by the ID defined here.

## Why

Owner decision R11 (2026-11-09) approves one canonical test ID across runners, with fresh
requirement IDs, and forbids reusing AFU-V1-140 to AFU-V1-145 or AFU-V1-150 to AFU-V1-154 with a
changed meaning. Without one ID, the plan, the receipt, run evidence, line evidence, flow links,
watch and MCP cannot be joined (review finding testing T-B4).

### Today's keys, from the code at integ 9ec3a29d0

- Go units in `corvint test` are keyed by package: the receipt outcome loop trims the `go:` prefix
  from the unit and looks the result up per package (`internal/sessiontest/run.go`, lines 757-774,
  integ 9ec3a29d0). The receipt `Outcome` holds only `Outcome`, `Slice` and `Unit`
  (`internal/sessiontest/run.go`, lines 70-75).
- JavaScript and TypeScript units are keyed by file as `typescript:<runner>:<path>`
  (`internal/sessiontest/provider.go`, lines 19-39, integ 9ec3a29d0).
- The Go line index records a test as package plus name (`internal/linecov/compact.go`, lines
  341-351, integ 9ec3a29d0).
- The Minitest hook records `Class#method` with the method's source file, and the RSpec hook keys
  examples by the rerun file path (`internal/rubylinecov/linecov.rb`, lines 300-318, integ
  9ec3a29d0).
- The run-evidence ingest of `flows ingest` builds its own test keys per report format
  (`internal/appflows/runingest.go`, integ 9ec3a29d0).
- The flow link record of PROD-DOCS-012 names a test by file path plus title path.
- pytest node IDs (`file::Class::test[param]`) are named by `pytest-provider`; no code in integ
  9ec3a29d0 keys pytest results by node ID in `corvint test`.

No rule defines how titles are normalised, how parametrised or nested tests are named, how
duplicates are handled, or how a file-level selection matches test-level evidence.

## What changes

- One ID form: runner, repository-relative file (the package directory for Go), and the
  runner-native name path, with one escape rule (CTI-V1-001, CTI-V1-002).
- A mapping from each runner's report to the name path: Go, Vitest, Jest, Playwright, pytest, RSpec
  and Minitest (CTI-V1-003 to CTI-V1-007).
- A file-level selection matches every ID under that file (CTI-V1-008).
- The same ID string is used in the plan, the receipt, run evidence, line evidence, flow links,
  watch output and MCP results (CTI-V1-009).
- Duplicate IDs are disambiguated by an ordinal and widen selection to the file (CTI-V1-010).
- Anything that cannot be mapped runs with a named reason, never a guess (CTI-V1-011).
- Today's keys are read as file-level or package-level selections; nothing old is rewritten
  (CTI-V1-012).

## Non-goals

- No change to how any runner selects or runs tests; that is `runner-contract`.
- No inventory file, run-evidence store, line index or flow link schema; those are the later changes
  in the R38 order, which use this ID.
- No rename tracking across commits. A renamed test gets a new ID; records keyed by the old ID become
  dangling and are reported (CTI-V1-011).
- No reuse of AFU-V1-140 to AFU-V1-145 or AFU-V1-150 to AFU-V1-154 (owner decision R11).
- No runner beyond the seven blocking runners (G26, owner decision R6).

## Impact

- New spec `docs/specs/canonical-test-id-v1.md`; new rows in `docs/specs/REQUIREMENTS.tsv` from the
  generator.
- New code in one small package that formats, parses and matches IDs, used by `corvint test`,
  `corvint affected`, `corvint watch`, the flow reports and MCP.
- The `corvint-test-run/0` receipt gains a `testId` field per outcome under the add-only rule for
  receipts (settled D25). Existing fields keep their meaning.
- `runner-contract` (RNC-V1-005, RNC-V1-012), `test-inventory`, `run-evidence`, `line-evidence`,
  `flow-link-proposals`, `jest-provider`, `per-test-durations`, `pytest-provider` (PYR-V1-004),
  `ruby-providers` (RBR-V1-005), `watch-seven-runners` (WCH-V1-002), `flows-status-report`
  (AFL-V1-001) and `fault-injection-harness` (FIH-V1-008) cite this ID instead of their own keys.
