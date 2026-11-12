# Change proposal: a pinned dead-function report with a shrinking baseline, then delete what it lists

Change name: `deadcode-ratchet`
Tickets: V1-4801 (the report and its baseline) and V1-4827 (delete the listed functions). Both are
milestone v1-0, P1, steps of the 2026-11-06 expert-panel restructuring plan.
Product requirement served: no product requirement names dead functions. The nearest authority is
owner decision R33 (`docs/specs/PRODUCT-PLAN-1.0.md` checklist row O42: orphaned code is deleted
and tracked) and the source line of PROD-INT-001 ("perfectly integrated and structured").
Owning capability spec: new, `docs/specs/deadcode-ratchet-v0.md` (prefix `DCR-V0`). The lane
creates it from the spec delta in this folder.
Requirement IDs this change adds: DCR-V0-001 to DCR-V0-008. No `DCR-V0` ID exists in `docs` on
`origin/main` at `4c3c20333`, on the fetched lane branches `claude/affstatus` and
`claude/affmeasure`, or on the unpushed batch branches 1111a, 1111b and 1111c (checked
2026-11-11).
Intent status: proposed (written 2026-11-11; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/deadcode-ratchet/spec.md`).

## Why

The 2026-11-06 panel measured 139 functions (about 1.4k lines) that no binary and no test can
reach. Nothing stops that number from growing: `go vet` does not report an unused exported
function or an unused method, and no target in the Makefile runs a reachability analysis.

Two facts from `origin/main` at `4c3c20333` (2026-11-11):

- No dead-code tool, target or baseline exists. `go.mod` has no `golang.org/x/tools` requirement
  and no `tool` directive.
- The 139 figure is stale. Decision 0594 and the orphan deletions removed whole packages after it
  was taken. The lane measures the current number and records it; this proposal does not assume
  it.

V1-4801 asks for the report and a baseline that only shrinks. V1-4827 asks for the listed
functions to be deleted. They are one change because the second is checked by the first.

## What Changes

- A `make deadcode` target installs one exact version of `golang.org/x/tools/cmd/deadcode` into a
  scratch directory and runs it with `-test` for six `GOOS`/`GOARCH` configurations
  (DCR-V0-001, DCR-V0-002). `go.mod` and `go.sum` do not change.
- A small Go tool, `tools/deadcode-ratchet`, reduces each run to function identities, keeps the
  identities present in every run, and compares them with a checked-in baseline (DCR-V0-003 to
  DCR-V0-005).
- The comparison fails on a function that is unreachable and not in the baseline, and on a
  baseline entry that is no longer reported. The baseline can only shrink.
- Hermetic tests cover the comparison on fixture data, without loading the module (DCR-V0-006).
- In a second commit, the lane deletes the functions the report lists, under fail-closed
  exclusions, and proves the tree still builds and vets in every configuration (DCR-V0-007,
  DCR-V0-008). A function that an exclusion keeps stays in the baseline with a written reason.

## Non-goals

- Code that only tests reach (the panel counted about 22k lines). `-test` treats it as live.
- Packages linked into no binary. Decision R33 and the `source-removal` change own those.
- A CI job. Workflow files change in their own pull request. The lane reports the measured run
  time so that a later change can decide where the target runs.
- Functions that exist on one operating system only. The report is an intersection, so a function
  declared and unreachable on Linux alone is not listed (V1-4801 criterion 2 asks for this).
- Unused constants, variables, types and struct fields.
- Any change to `go.mod`, `go.sum`, `protocol/**`, `.github/**` or the module `interop/cem01-go`.

## Failure modes

- **A configuration fails to load or type-check.** The target fails and names the configuration.
  It never intersects a partial set of runs, because a missing run would shrink the report and
  make the baseline look stale.
- **The pinned version cannot be fetched or verified.** The target fails. It does not fall back to
  another version or to a tool already on `PATH`.
- **The analyzer misses a use it cannot see** (a `//go:linkname` target, an assembly reference, a
  build tag outside the six configurations). The ratchet only gains a harmless baseline entry.
  The deletion step is where this matters, so DCR-V0-007 excludes those cases and DCR-V0-008
  vets every custom build tag found in the tree.
- **A deleted function was cited by a spec as evidence.** DCR-V0-007 keeps any function whose name
  a spec or a top-level document cites, and the lane reports it instead of deleting it.
- **A deletion shifts lines that documents cite.** The lane runs the line-citation gate and fixes
  what its own change moved.
- **Residual entries remain after the deletions.** V1-4827 criterion 1 (zero baseline functions)
  is then not met. The lane reports each residual entry with its reason and the ticket stays
  open for an owner decision.

## Acceptance evidence

- V1-4801 criterion 1: `make deadcode` exits 0 on the committed baseline, and exits non-zero with
  the function named after a scratch commit adds one unreachable function. Both runs are recorded
  in the build-log entry with their exit codes and elapsed time.
- V1-4801 criterion 2: the hermetic test in which a function reported in one configuration only is
  absent from the report.
- V1-4801 criterion 3: the test that asserts the baseline entry count. The ticket says 139; the
  asserted number is the one measured at the lane base, and the build-log entry records both.
- V1-4827 criterion 1: `make deadcode` after the deletion commit, with the baseline count and each
  residual entry's reason.
- V1-4827 criterion 2: `go build ./...` and `go vet ./...` for each of the six configurations and
  each custom build tag, plus the tests of every package that lost a function.
- V1-4827 criterion 3: a check that every removed declaration is in the regenerated list, recorded
  as a command and its output.

## Rollback

Revert the deletion commit to restore the functions; the ratchet commit stands alone. Revert the
ratchet commit to remove the target, the tool, the baseline and the spec. Nothing persists outside
the repository: the analyzer binary lives in a scratch directory that the target removes.
