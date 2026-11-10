# Change proposal: line evidence for the seven blocking runners

Change name: `line-evidence`
Ticket: none filed yet; to be filed in `.taskman/` before dispatch (not filed by this proposal).
Product requirement served: PROD-TEST-008 in `docs/specs/PRODUCT-PLAN-1.0.md` (Go, Vitest and Jest
line-level at RC), PROD-TEST-017 and PROD-TEST-018 (tiered selection with a reason per selection).
Owning capability spec: a new spec `docs/specs/line-evidence-v1.md` (prefix `LEV-V1`). It does not
exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: LEV-V1-001 to LEV-V1-015. `LEV-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and appears in no other folder under `docs/specs/openspec/changes/`
(checked 2026-11-09).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.

## Needs first

- `canonical-test-id`: evidence rows are keyed by canonical test ID.
- `test-inventory`: evidence is built per runner instance and checked against the inventory.
- `run-evidence`: line evidence collection runs record their results and durations there.

This change is fourth in the build order fixed by owner decision R38 (2026-11-09).

## Why

Owner decision R8 (2026-11-09): line-exact selection in 1.0 for Go, RSpec, Minitest, Vitest and
Jest; pytest selects by file with an explicit tier label. Owner decision R9 (2026-11-09): pytest
file-level is a release exception recorded in the acceptance record, and the Python line collector is
a 1.0.x ticket. Owner decision R16 (2026-11-09): one tier vocabulary `line | file | flow | full` plus
`exact`; `flow` does not count as narrowed for the no-missed-test claim.

G2 (owner answer), recorded as: "in 1.0 a browser E2E test is removed from the selection only by
accepted flow links plus static reachability (file and symbol level). Line evidence may add or rank
tests, never remove one. Per-test line narrowing for E2E ships as an opt-in profile labelled
experimental; promotion needs fault-injected zero-miss proof and a measured net wall-clock win.
Every selection prints a one-line reason." Condition carried: ranking by line evidence is never
presented as narrowing.

Review finding testing T-B1 asks this change to define the writing verb, storage and tree binding,
freshness, the consumers (`affected`, `test`, `watch`, MCP), the CI and Action refresh, and the
fault-injection proof per runner before tier `line` is used. NEW-testing-3 asks for a product
writing verb whose output is private and tree-bound, built by the Action on the default branch and
cached. T-M9 asks for a rule when the selected tests exceed argument limits. T-M16 asks that the G4
risk of file-level pytest be stated.

### What exists at integ 9ec3a29d0

- Go only in the product. `corvint affected --base SHA --line-index FILE` narrows Go units and
  refuses every other form with `unsupported-affected-line-index-profile`; a narrowed selection must
  pass `linecov.CheckQualification` (`cmd/corvint/affected_linecov.go`, lines 19-30, integ
  9ec3a29d0; GLS-V0-010, GLS-V0-034). `corvint test --line-index FILE` is experimental (GLS-V0-020).
- `corvint watch` refuses an index that records a language other than Go with
  `watch-index-language` (`cmd/corvint/watch.go`, lines 128-130, integ 9ec3a29d0).
- The index format `linecov-index/4` binds module, commit and runner, and records a blob OID per
  instrumented file (GLS-V0-001). Its extension already records `Language`, `Command`,
  `RunnerFamily` and `Config` (`internal/linecov/compact.go`, lines 60 and 74, integ 9ec3a29d0).
- Collectors exist outside `corvint`: `internal/jslinecov` lists runners mocha, jasmine, node-test,
  jest, vitest, ava, bun-test and deno-test (`internal/jslinecov/collect.go`, line 26, integ
  9ec3a29d0), and `internal/rubylinecov` holds RSpec and Minitest hooks. Both are used by the
  separate `tools/linecov` program and the registry plugins, not by `corvint affected`, `test` or
  `watch`. `tools/release-binary/binaries.json` (integ 9ec3a29d0) ships no linecov binary.
- There is no Python line collector.

## What changes

- One writing verb, `corvint line-evidence refresh` (name proposed), that collects per-test line
  evidence for each runner instance and writes it privately under `<git-dir>/corvint/line-evidence/`
  (LEV-V1-001, LEV-V1-002).
- Evidence is bound to the tree; stale, missing or unqualified evidence widens and names the reason
  (LEV-V1-003, LEV-V1-004).
- Collectors: Go, Vitest, Jest, RSpec and Minitest produce line evidence; pytest has none in 1.0 and
  selects by file (LEV-V1-005, LEV-V1-006).
- Playwright follows G2 exactly: line evidence may add or rank E2E tests and never removes one; the
  per-test E2E line profile is opt-in and experimental (LEV-V1-007, LEV-V1-008).
- One tier vocabulary in every plan, with `exact` separate and a one-line reason per selection
  (LEV-V1-009, LEV-V1-010).
- Tier `line` is used for a runner only after that runner passes the fault-injection proof
  (LEV-V1-011).
- One reader used by `corvint affected`, `corvint test`, `corvint watch` and MCP; watch reads every
  supported language (LEV-V1-012).
- The GitHub Action builds evidence on the default branch and caches it (LEV-V1-013).
- Argument limits use argument files or widen to file level (LEV-V1-014).
- One-shot `affected`, `test` and the MCP plan tool plan the working tree as `watch` does
  (LEV-V1-015; R42 (panel pick) A7).

## Non-goals

- No Python line collector in 1.0 (owner decision R9; 1.0.x ticket).
- No removal of a browser E2E test by line evidence (G2).
- No change to the explicit `--line-index FILE` flag or to GLS-V0-010; this change adds the default
  reader beside it.
- No committed line evidence; the store is private.
- No ranking presented as narrowing.
- No collection run by a read command (product invariant 4).

## Impact

- New spec `docs/specs/line-evidence-v1.md`; generated rows in `docs/specs/REQUIREMENTS.tsv`.
- `internal/jslinecov` and `internal/rubylinecov` are called from `corvint`; the `tools/linecov`
  program is folded into the new verb.
- `cmd/corvint/watch.go` drops the Go-only refusal in favour of the shared reader.
- G4 risk: G4 requires pytest to narrow at least 60 percent of PRs with median selected time at most
  50 percent. File-level selection selects every test in a reached file and may miss that bar on
  repositories with large test files; the measured result is reported either way, and a miss is a
  release finding for the owner, not a reason to narrow below file level.
- Changes `fault-injection-harness` (FIH-V1-015), `narrowing-measurement`, `watch-seven-runners`,
  `github-action`, `pytest-provider` and `ruby-providers` cite these tiers and reasons.
