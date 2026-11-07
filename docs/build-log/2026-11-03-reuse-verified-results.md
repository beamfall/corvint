# reuse: verified per-package test reuse and job-level input predicates

Base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3` (`origin/main`). Tickets: V1-1097 and V1-1099.
Requirements: AFP-V0-2320 to AFP-V0-2346 in `docs/specs/affected-plan-reuse-v0.md`, a companion
under the parent's `AFP-V0` prefix because `affected-plan-v0.md` holds 256 of 256 obligations (the
charter assigned the AFP-V0-2320..2359 block, so the companion keeps the prefix, as
`affected-plan-selection-v0.md` does). AFP-V0-2344 amends AFP-V0-013.

## Decisions

1. **What bounds a package's inputs.** A digest is sound only if it names every repository file the
   test can read. The planner's literal-reader analysis is inference; the `.github/testconfine`
   Landlock wrapper is enforcement: a declared package's test binary, and every process it starts,
   can read below the root only its own directory and its declared entries. So eligibility is a
   declared read scope (218 of 440 packages at base), and the digest is that directory, those
   entries, the compiled closure from `go list -deps -test`, and the module files, each bound to
   its HEAD blob. Undeclared packages always run (`UNDECLARED_READ_SCOPE`). The ticket's
   `UNBOUNDED_READER` rule is kept as defense in depth; at base no declared package is one.
2. **No planner range check.** Digest equality is the whole proof, so a record needs no old commit
   and an UNKNOWN plan scope does not block reuse. A planner-gated design was rejected: the plan
   scope was UNKNOWN for all 100 ranges of the job replay below, so it would have reused nothing.
3. **Identity.** `go env` values, the argv (must carry `-count=1`, no `-run`/`-skip`/`-short`...) and
   harness blobs. CI's harness is `test-commands.json`, `.github/testconfine` and `ci.yml`, so a
   workflow edit invalidates every record (919 `IDENTITY_CHANGED` units in the replay).
4. **Trust.** The caller is the boundary: CI lists runs of this repository by event and branch;
   `decide` refuses a record of another pull-request number, an edited identity or another kind.
   Merge-queue and push runs never consume records, and a reused pull-request run writes no
   AFP-V0-170 or AFP-V0-734 record, so landing always rests on a full queue run.
5. **Local runner.** No `corvint-test-selected` binary exists at base; the charter's local path is
   `script/heavy.sh --plan`, which now honours a decision through `HEAVY_REUSE`.
6. **Job predicates as a subcommand.** `corvint affected jobs` writes its own
   `corvint-ci-job-plan/0` instead of adding a member to the closed `affected-plan/1` wire. The
   declaration lives in `.corvint/ci-job-inputs.json` (project authority, ships with the code);
   the `.github` wiring is a separate commit.
7. **artifact-integrity inputs.** The archive gate exports every tracked file (`git ls-tree -r`)
   and refuses non-regular or oversized blobs, so it is declared `exportedTree` beside its legal
   files, scripts, manifest and the `./cmd/corvint` and `./conformance/release-artifact-v0/...`
   closure on its five target platforms (each with cgo off and on).
8. **containment not wired.** Its `goTests` predicate needs a BOUNDED plan, which no replayed range
   had, and building `corvint` for its plan costs about the job's two minutes. It stays declared
   and evaluated.

## Measurements (same build, real ranges)

Data: `tools/ci-reuse-replay/fetch.py` over every `ci.yml` and `intake-containment.yml` run of
`beamfall/corvint` (381 and 290 runs), all retained `ci-outcomes-*` artifacts (2428 shard
documents of 302 runs) and job listings. The branch's own `corvint` (built at the lane head)
replayed them with `tools/ci-reuse-replay/replay.py` in a detached worktree checked out at each
historical commit.

**V1-1097 reuse (`replay.py tests`)**, pull-request runs 2026-10-30 to 2026-11-03: 81 re-push pairs
over 47 merged pull requests; each later run decided against a record of the same pull request's
previous run (made at that run's tested merge) and, for 79 pairs, of the newest successful
merge-queue run before it.

| measure | value |
|---|---|
| package runs in the later runs | 30094 |
| decided REUSE | 10125 (34%) |
| packages that failed in the later run | 22 |
| failed packages decided REUSE (violations) | **0** |
| failed packages by reason | 13 UNDECLARED_READ_SCOPE, 5 IDENTITY_CHANGED, 2 DIGEST_CHANGED, 2 NO_RECORD |
| runner minutes of the later runs (committed cost table) | 13219.8 |
| runner minutes reused | 2830.5 (21%) |
| modelled shard wall, 16 shards, LPT, split packages as 16 slices | 849.2 to 723.5 min over 81 pushes (10.5 to 8.9 min each, -15%) |

All units: 15719 UNDECLARED_READ_SCOPE, 10125 RECORD_DIGEST_EQUAL, 2608 DIGEST_CHANGED, 919
IDENTITY_CHANGED, 577 NO_RECORD, 146 SLICED_UNIT, 0 DIGEST_ERROR. The wall figure is a model from
`.github/cishards/package-costs.json`, not an observed shard time; hosted reuse is NOT_OBSERVED.

**V1-1099 job predicates (`replay.py jobs`)**, the last 100 completed `pull_request` (72) and
`push` (28) runs, 2026-11-03T01:47Z to 23:37Z, each planned at the run's head over its change:

| job | SKIP of 100 | runs observed (success/failure) | failures | SKIP on failure | runner minutes observed | saved |
|---|---|---|---|---|---|---|
| artifact-integrity | 36 | 48 | 0 | **0** | 701.0 | 200.5 (29%) |
| go-interop | 88 | 71 | 0 | **0** | 35.3 | 32.4 (92%) |
| containment | 0 (PLAN_SCOPE_UNKNOWN) | 99 | 0 | **0** | 54.4 | 0 |

No declared job failed in the window, so the replay shows no unsound SKIP but holds no failing
positive case; the fault injections below supply those.

## Soundness controls

Negative controls (`TestDigestNegativeControls`): a change to a declared scope file, a dependency's
compiled source, the package's own testdata, `go.mod`, the declaration's entry for the package or a
newly created declared file each runs the package; a documentation edit, an undeclared data file
and a dependency's test file reuse. Fault injection (`TestFaultInjectionIsSelectedAndFails`): a
record from a real passing `go test -json` run, then a committed edit of the declared file the test
asserts on: the decision runs the package and the real test fails, while the untouched dependency
is reused. Job predicates: `TestGoDirectoriesUnionsPlatforms` injects a compile error into a
package only the windows build imports (RUN, and `GOOS=windows go build` fails);
`TestInteropFormattingFaultIsRun` injects an unformatted interop file (RUN, and `gofmt -l` reports
it). `TestEvaluateFailsClosed`, `TestDirtyOrUntrackedInputRuns` and `TestIdentityAndTrust` cover
the fail-closed paths.

## Limits

The `>= 50 merged pull requests with re-pushes` criterion: the repository has 82 merged pull
requests since its public history began on 2026-10-30; 54 have re-pushes, and 47 of them have two
runs with retained shard outcomes (the rest re-pushed through documentation-only or cancelled
runs). The replay covers all 47; the shortfall is the history that exists, not a sample choice.
Hosted behaviour of both wirings is NOT_OBSERVED until the owner's `ci-control-plane` review and
the `CORVINT_PR_REUSE` / `CORVINT_PR_JOB_PLAN` variables.
