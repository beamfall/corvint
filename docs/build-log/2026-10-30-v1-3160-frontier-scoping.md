# Scope the global plan frontiers — V1-3160

## Root cause

At `fd51f53a2` every non-empty Corvint plan was `UNKNOWN`, so CI ran FULL, because three unit
frontiers bore on every plan:

- `go:build-constraint-variants` on every constrained package a plan reached (AFP-V0-580). Most
  plans reach one, so it was named almost always.
- `go:nested-module-frontier` through the synthetic `go:<module>` unit for `tools/local-authority`.
  That unit inherits root locating from `internal/authoritystore`, whose working-directory walk
  climbs to `.git`. `graph.locatorReach` marks a root locator's dependents as followed on every
  dirty path (AFP-V0-581 (1), AFP-V0-012 rule (d)), so its DEPENDENTS frontier bears on all of them.
- `shell:foreign-invocation-unmodelled` through `script/heavy.sh`. The script locates the root
  (`git rev-parse --show-toplevel`, lines 201 and 363) and sources `"$w/meta.env"` (line 577), which
  makes it an unbounded reader (`shell:dynamic-source`) followed on every dirty path.

Main-delta paths measured before with the planner on base `fd51f53a2` (a temporary diagnostic test,
since deleted), and after with `corvint affected --no-cache --path PATH` from a binary built at the
lane commit `17f32d2` (the same code as this commit):

| Dirty path | Selected units | Frontiers before | After TSP-V0-001 |
|---|---|---|---|
| `docs/AUTOMATION.md` | 50 | all three | nested-module, shell |
| `internal/liveverify/affected/golang/golang.go` | 95 | all three | nested-module, shell |
| `script/heavy.sh` | 50 | all three | nested-module, shell |
| `cmd/corvint/main.go` | 83 | all three | all three (constrained package) |
| `internal/gitstatus/status.go` | 175 | all three | all three (constrained package) |
| `internal/tasks/cli/cli.go` | 61 | all three | all three (constrained package) |
| `go.mod`, `README.md` (root, an ancestor) | n/a | all three and others | build-constraint kept |

The selected sets are unchanged, because a frontier changes scope, not selection. Every plan is
still `UNKNOWN` because of the two frontiers below, so CI still runs FULL. TSP-V0-001 removes one of
the three causes; it does not change any CI run on its own.

## Decision: TSP-V0-001 (build constraints)

`go:build-constraint-variants` now bears only on a plan with a dirty path in the constrained
package's own directory or an ancestor (`frontierBears: OWN_FILES`). This is a narrowing of AFP-V0-580, which
said the frontier bears on every plan that reaches the package. The supporting model:

- the package's files all live in its directory, so the variant set changes only there;
- `go.mod`, `go.work` and the `Makefile`, which choose the toolchain and tags, lie in an ancestor
  (the ancestor rule was added after the lane's self-check found a `go.mod` change would
  otherwise drop the frontier);
- every variant's imports are already graph edges;
- one run compiles one host variant, whichever packages it selects.

Naming the frontier for an outside change therefore widened the run without testing any other
variant. A plain file in the directory still bears, which keeps AFP-V0-580's withdrawn-scoping
case (a variant that calls a plain file). Reviewers should check this argument directly.

## Finding: nested-module and shell frontiers need an owner decision

Neither can be narrowed soundly under existing project authority, so both are left as they were.

- An attempt to give the dependent-module unit `DEPENDENTS` still bore on every plan, because of
  `locatorReach`. It was reverted.
- `tools/local-authority`'s tests are run only by the Makefile gate, never by CI. The frontier is
  their only signal.
- `script/heavy.sh`'s dynamic source is a runtime file, but no model says so.

The options are listed in the spec's Open questions.

## Requirement IDs

`affected-plan-v0.md` is at its OCM limit, so the new requirement is in the companion spec
`test-selection-precision-v0.md` (prefix `TSP-V0`). The lane's assigned `AFP-V0-920` to `949`
block was not used.

## Measured outcome after TSP-V0-002 to 005

TSP-V0-002 closes a symlink gap in TSP-V0-001. TSP-V0-003 (V1-0872) removes the nested-module
cause through a verified gated-locator declaration. TSP-V0-004 (V1-3325) bounds heavy.sh's fresh
`mktemp -d` source. TSP-V0-005 (the orchestrator's option (c)) verifies a declaration of every
file that names `script/heavy.sh` and the three scripts that run it. It is verified rather than
trusted: the shell plugin re-scans every candidate file on each graph build, and
`TestRepositoryShellForeignCallersComplete_TSP_V0_005` scans the tracked tree, failing on an
unlisted file.

The measurements used `corvint affected --no-cache --path PATH` on a clean tree:

- before: a binary built from base `fd51f53a2`, run on a worktree at that base;
- after: a binary built from the lane commit that adds TSP-V0-005, run on the lane worktree.

Cost is the selected Go packages' share of the summed per-package test time in
`.github/cishards/package-costs.json`. Shell tests are counted in the units but not costed. An
`UNKNOWN` plan runs FULL in CI, so its effective cost there is 1.0.

| Dirty path | Before: scope, units, cost | After: scope, units, cost |
|---|---|---|
| `docs/AUTOMATION.md` | UNKNOWN, 50, 0.327 (CI 1.0) | **BOUNDED**, 50, 0.327 |
| `internal/liveverify/affected/golang/golang.go` | UNKNOWN, 95, 0.478 (CI 1.0) | **BOUNDED**, 95, 0.478 |
| `script/heavy.sh` | UNKNOWN, 50, 0.320 (CI 1.0) | **BOUNDED**, 64, 0.380 |
| `internal/capcatalog/capcatalog.go` | UNKNOWN, 74, 0.438 | **BOUNDED**, 74, 0.438 |
| `script/lane-guard.sh` | UNKNOWN, 50, 0.320 | **BOUNDED**, 50, 0.320 |
| `cmd/corvint/main.go` | UNKNOWN, 83, 0.385 | UNKNOWN, 83, 0.385 |
| `internal/gitstatus/status.go` | UNKNOWN, 175, 0.751 | UNKNOWN, 175, 0.751 |
| `internal/tasks/cli/cli.go` | UNKNOWN, 61, 0.531 | UNKNOWN, 61, 0.531 |
| `docs/specs/affected-plan-v0.md` | UNKNOWN, 59, 0.357 | UNKNOWN, 61, 0.366 |
| `README.md`, `go.mod` | UNKNOWN (root, many frontiers) | UNKNOWN |

`script/heavy.sh` selects 14 more units after the change: `internal/dogfoodflow` and 13 of its
importers' packages (`cmd/corvint-test`, `tools/corvint-pr-tests`, `tools/ci-forward-row` and
others). They arrive through `batch-regen.sh`, whose declared Go callers are in
`internal/dogfoodflow`. These are edges the plugin could not model before. In this repository,
`heavy.sh` is no longer followed on every dirty path once TSP-V0-004 bounds its source, so these
tests are selected only when `heavy.sh` or `batch-regen.sh` changes: `docs/AUTOMATION.md` selects
the same 50 units before and after. The `docs/specs` path gains two units, which were not
attributed.

Files that only name `heavy.sh`, and do not run it, are declared as `mentions`:

- the `corvint test` message in `cmd/corvint`;
- the deny text in `lane-guard.sh`;
- the transcript matcher;
- a comment in `tools/test-shards`.

Declaring them as callers would have added `tools/test-shards` to the units selected on every
change, which the unbounded-readers check refused. This split between callers and mentions is the
project's own claim, file by file. The rest of the declaration is verified.

## Residuals that still keep a plan UNKNOWN

The bearing units were found by a temporary diagnostic test (`ReachedBeyondOwnTests`), since
deleted. No CLI output names them.

- **Go source in a constrained package** (`cmd/corvint`, `internal/gitstatus`, `internal/tasks/cli`):
  `go:build-constraint-variants` bears on the package's own files. TSP-V0-001 intends this.
- **`cmd/corvint/**`**:
  - `go:nested-module-frontier` bears through `tools/local-authority`, which reads `cmd/corvint` by
    literal path.
  - `shell:foreign-invocation-unmodelled` bears through 14 undeclared non-test scripts that build
    or read `cmd/corvint` by literal path: `agent-preflight`, `check-analyzer-python-offline-build`,
    `check-install-lifecycle`, `check-sql-native-ratchets`, `dogfood-batch-check`,
    `dogfood-bind-range`, `dogfood-build-corvint`, `dogfood-change`, `dogfood-check`,
    `dogfood-finish`, `gate-affected-corvint`, `install-cem-verifiers`,
    `spec-toolchain-conformance` and `tools/check-analyzer-go`.
- **`docs/specs/**`**: `shell:foreign-invocation-unmodelled` bears through the undeclared spec
  checkers:
  - `check-error-code-ownership`
  - `check-line-citations`
  - `check-requirement-definitions`
  - `check-traceability-tests`
  - `dogfood-finish`
  - `gen-spec-requirements`
  - `spec-acceptance-audit`
  - `spec-requirements-resolve`
  - `spec-toolchain-conformance`

  The staged-CEM path set in `replay_v12941_test.go` has the same bearers, through
  `dogfood-seal.sh` and `check-install-lifecycle.sh`.
- **Root files** (`README.md`, `go.mod`): every language's global frontier bears.

Declaring the residual scripts is mechanical. The cost would be over-selection: several Go
packages (`internal/contextindex`, `internal/console`, `internal/lrfrepo`, `tools/docs-ci-plan`)
name them, and each one would have to be classified as a caller or a mention. That is left for a
follow-up.
