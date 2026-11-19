# Source removal B0: workflow repairs and retained blockers

Date: 2026-11-18
Base: `55decc6a041a1df3f8ae43e95ba21cea669e7d80`
Scope: source-removal task 3, RMV-V1-002 and RMV-V1-006. Status: PARTIAL.
Authority: the lane charter records owner acceptance under G14 on 2026-11-18, PR #235.
Ticket: V1-5693. Task 3 remains unticked. The .github change ships as PR #237.

## Changes and already satisfied steps

Removed the retired `cce-live` job from `ci.yml`. Replaced the stale `tools/qualification` comments
in `ci.yml` and `go-macos.yml` with the actual trusted documentation classifier fallback.
No script, Go package, assertion, frozen fixture, wire contract or core-freeze output changed.

Ancestor `62177f1df8399cae973b73ed4ccb7c153218a26d` already removed the local-authority job,
the intake workflow, the macOS references to missing packages and the companion-release step.
Those B0 steps need no second edit. The supplied base contains the cut and its workflow repair.

## Ruleset evidence: RMV-V1-002

Read `gh api repos/beamfall/corvint/rules/branches/main` before and after the workflow edits.
Both reads require exactly `go-product`, `ci-control-plane` and `doc-gates`, in that order.
Neither read requires any B0 removal. The ruleset was not modified.
Both raw JSON responses are 1,156 bytes, with this SHA-256:

`e0bb770385b51e0d5ce258470fc6c07d7fd71a65d0c15cd367c342e46b31eab8`

## Open item 7: stale shard entries

Resolved: stale package-cost, allowlist and slice entries do not add missing packages to CI.
`ci.yml` supplies a universe from `go list ./...`. `PlanFromParallel` places only that universe;
`SplitPackages` ignores entries outside it, and selection intersects it. Stale cost values can
still influence the conservative estimate for an unpriced package; this is not package membership.

The existing `TestAFPV0022MembershipDerivedFromUniverse` and
`TestAFPV0022RepositoryUniverseNeedsNoTableEdit` both PASS in a whole-package verbose run.
They exercise synthetic stale entries and simulated removal against the repository universe.
The current repository run logged zero stale cost entries. The analyzer rows in
`walltime_test.go` are the frozen `run41568676566` replay, not packages sent to `go test`.
The replay test also passes. Preserve those fixtures and defer table cleanup as B0 permits.

## Blocked deletions and proof

1. CEM workflow deletion would fail `make ci-least-privilege-check`: the exception for
   `job:cem-release-reuse` in `script/ci-least-privilege-exceptions.tsv` becomes stale.
   A temporary index containing only that deletion reproduced exit 2. Its exception file is
   outside this lane's allowed scope. Existing CEM tests in `tools/ci-reuse-plan` and
   `.corvint/shell-foreign-callers.json` also require coordination. Keep the workflow intact.
2. VS Code workflow deletion conflicts with the existing
   `TestVSCV0080ExtensionWorkflowRunsOnContractChanges` in `tools/ci-reuse-plan`.
   It unconditionally reads that workflow and asserts its jobs and commands. This lane cannot
   change that package or weaken its assertions. Keep the workflow intact.
3. RMV-V1-003's literal zero-reference proof is not met for the removed spec-toolchain workflow.
   Four references survive the named history exclusions: the active affected-plan and
   spec-toolchain-adapters specs, a historical example in ticket V1-1803, and the glob-reader
   fixture in `internal/liveverify/affected/golang/globs_test.go`. Those files are outside scope;
   the ticket store is explicitly forbidden. Do not remove historical or fixture evidence.
4. The whole `cishards` test package fails its isolated Windows builds with
   `error obtaining VCS status: exit status 128`. Its verbose rerun records 49 top-level PASS
   outcomes and one FAIL, `TestIsolatedHelperBuildsCopyWhatTheyNeed_AFPV02726`.
   The four isolated module builds fail alike. Root cause is NOT_OBSERVED; host replay is needed.

## Verification and limits

All commands use `TMPDIR=/home/russell/lanes/sr-b0.tmp`; Go tests use `script/heavy.sh`, one
package per command, without an exported commit identity or a lock-directory override.

- Go version equals `go1.27.4`: PASS.
- `corvint affected --base 55decc6a041a1df3f8ae43e95ba21cea669e7d80`: exit 0. Three dirty paths,
  321 selected units, 447 excluded, one `LANGUAGE_FRONTIER` unknown, 33 unresolved packages.
  Verdict FULL because `ci.yml` is a never-narrowed path. Both tested packages are explicit
  path readers. The broad fallback was retained, not executed, under the lane instruction.
- `script/heavy.sh go test -count=1 -timeout 30m ./tools/ci-reuse-plan`: PASS, 29.488 seconds.
  Existing CCE tests deliberately skip when the job is absent; no assertion was changed.
- `script/heavy.sh go test -count=1 -timeout 30m ./.github/cishards`: FAIL, 12.210 seconds.
  Rerun with `-v` to retain named open-item evidence: FAIL, 6.236 seconds, as detailed above.
- PyYAML 6.0.3 parses both changed surviving workflows, and all job dependencies resolve:
  PASS, 21 jobs in `ci.yml` and five in `go-macos.yml`. `actionlint` is not installed.
- `make ci-least-privilege-check`: PASS on the staged change; the deletion probe is separate.
- `script/check-line-citations.sh --repin`: PASS, no moved citation to rewrite.
- `make -s spec-requirements-check line-citations-check openspec-check long-lines-check`: PASS.
  OpenSpec retains 65 baseline findings, including source-removal's existing task-order debt.
- `go vet` on touched packages and `make windows-vet`: NOT_RUN; no Go package or module changed.
- `make spec-requirements`: NOT_RUN; no spec, requirement or task checkbox changed.
- Exhaustive gates and their recursive dry run: NOT_RUN under the lane's focused-only override.
- Hosted green CI and independent review: NOT_OBSERVED; publication is forbidden and review is
  reserved for the orchestrator. Dogfood, CEM and seal are retired and were not run.

Raw evidence is outside the repository in `/home/russell/lanes/sr-b0-evidence/`.
The verbose shard log is 22,182 bytes; SHA-256:

`d49e0115ac9abc39cf2cca2b0fed2814aa3bae9a461f3928c3f22153098d7dbf`

The CEM deletion probe log is 278 bytes; SHA-256:

`4b7da22858e609ca50361cb776cfb0c30a29ffba0f92131fde90acdb39227991`

Rollback: revert this batch's workflow changes from its parent. No runtime or data migration.
Completion requires the out-of-scope deletion dependencies, host test replay, required review
and integrated evidence. This partial batch does not claim completion of task 3 or all RMV-V1-006.

## Independent review repair

The independent review (gpt-6.1-sol, high) returned a P1: the deleted
`spec-toolchain-conformance.yml` was not OCM-only. At the base its script had already dropped the
OCM comparison; what remains validates the upstream OpenSpec fixtures and integration packaging,
and STA-V0-011 still requires that CI job. The workflow is restored unchanged. Its retirement needs
a coordinated change to STA-V0-011 and stays open, so item 3 above no longer applies. The review's
P2 (no regression test rejects a reintroduced `cce-live` job) is not fixed here under the
P0/P1-only repair rule. Ticket: V1-5693.
