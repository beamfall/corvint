# Scope the build-constraint frontier to its package's own files — V1-3160

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
