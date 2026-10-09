# V1-1044: `corvint affected` already selects the fixture-maintenance guard (2026-10-08)

## Intent

`tools/fixture-maintenance-check` (`TestRepositoryGitFixturesDisableDetachedMaintenance`) caught new
fixtures in `internal/stepnegation` and `internal/testplan` that ran `git commit` without
`-c maintenance.auto=false -c gc.auto=0`, but only in CI. The ticket asks whether
`corvint affected --base SHA` selects the guard for a change that adds a `*_test.go` invoking
`git commit`, and to fix the selection or the documented focused-test route if it does not.

## Evidence

Throwaway clone `/private/tmp/claude-501/v1-1044-scratch` of `origin/main` `a4acc6f6`; no lane file
was touched for the experiment.

1. Committed a new `internal/stepnegation/zz_scratch_fixture_test.go` whose test runs
   `exec.Command("git", "-C", dir, "commit", ...)` unguarded. Installed Corvint 1.0.0-rc.2
   (build 360) `corvint affected --base a4acc6f6...` selected 43 units including
   `go:github.com/Beamfall/corvint/tools/fixture-maintenance-check` with witness
   `{"kind":"DECLARED_READ_SCOPE","dirtyPath":"internal/stepnegation/zz_scratch_fixture_test.go"}`,
   and listed the package in `provider.go.packages`.
2. The same file as an uncommitted edit to the existing package `internal/localcompletion`
   (`corvint affected`, dirty worktree) selected it with the same `DECLARED_READ_SCOPE` witness.
   The lane-built binary (`2411e0f8`) gave the same result for `--base a4acc6f6...`.
3. The witness comes from `.corvint/test-read-scopes.json`, which declares the guard reads `cmd/`,
   `conformance/`, `internal/`, `interop/` and `tools/` (the guard's walk roots). `git log -S`
   shows that entry and the guard itself were both added by `440d30c9` (V1-1030, 2026-10-08).
4. On a branch from `440d30c9^1` (`3e501e17`) the same commit yields no
   `fixture-maintenance-check` unit: the guard package does not exist there.

## Finding

The selector is correct: any change under the guard's walk roots on a tree that contains the
guard selects it through its declared read scope. The CI-only catch is explained (inference from
the dates; the stepnegation/testplan branches are not on `origin/main` to inspect) by those lanes
being based before `440d30c9`: a branch cannot select a guard its tree does not yet contain, and CI
tested the merge result. No selector, read-scope or route change is needed, so none was made;
adding the guard to the documented focused-test route would duplicate what `affected` already
selects.

## Tests run

- `corvint affected` experiments above (rc=0 each); no `go test` was needed for a no-code outcome.
- Lane doc gates run with the V1-1043 commit and again with this entry.

## Limits

- A lane based before a newly landed repository-wide guard still misses it locally until the lane
  takes `origin/main`; that is a base-freshness question, not a selection defect.
- Only Go `*_test.go` additions under the declared roots were exercised; a fixture outside
  `cmd/`, `conformance/`, `internal/`, `interop/` or `tools/` is outside the guard's walk as well.
