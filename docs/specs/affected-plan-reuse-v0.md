# Affected Plan Verified Reuse and Job Predicates V0

Owner: Russell Lewis
Date: 2026-11-03
Requirement prefix: `AFP-V0`
Intent status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11)
Delivery status: experimental
Authoritative inputs: `docs/specs/affected-plan-v0.md` (parent contract: AFP-V0-013, AFP-V0-023,
AFP-V0-024, AFP-V0-028, AFP-V0-041, AFP-V0-047, AFP-V0-080, AFP-V0-163, AFP-V0-170, AFP-V0-590 and
AFP-V0-591), `.github/testconfine` (the confinement that bounds a declared package's reads),
`internal/liveverify/affected`, `AGENTS.md` invariants 1, 2, 4 and 8. The parent's Requirements
section is at the 256-obligation OCM limit, so lane reuse (2026-11-03; native tickets V1-1097 and
V1-1099) puts its requirements here under the parent's `AFP-V0` prefix, in the block
AFP-V0-2320 to AFP-V0-2359 its charter assigned, as `affected-plan-selection-v0.md` does; see
build-log entry `2026-11-03-reuse-verified-results`.

## Agent digest
- Claim: `corvint test reuse` reuses a Go pass whose input-closure digest is unchanged; `corvint affected jobs` decides RUN or SKIP for declared CI jobs.
- Status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11); delivery experimental; the CI wiring stays dark until the owner's `ci-control-plane` review and the `CORVINT_PR_REUSE` / `CORVINT_PR_JOB_PLAN` variables.
- Exists: `internal/testreuse` (`DigestPackage`, `Observe`, `BuildRecord`, `Decide`), `cmd/corvint/test_reuse.go`, `HEAVY_REUSE` in `script/heavy.sh`, `internal/jobinputs` (`Parse`, `Evaluate`, `GoDirectories`), `cmd/corvint/affected_jobs.go`, `.corvint/ci-job-inputs.json`, `tools/ci-reuse-replay`
- Blocked on: the owner's `ci-control-plane` review of the `.github` commit; hosted reuse and skips are `NOT_OBSERVED`.
- Read next: Requirements; Failure modes; Rollback.

## User and current state

The affected user is the parent's: an engineer, agent or CI run that wants to run only what a
change needs. Before this slice a pull-request re-push reran every package its range selected
even when the previous push of the same pull request had just passed it on identical inputs,
and `go-interop` and `artifact-integrity` ran on every pull request, documentation-only ones
included (the 2026-10-10 CI audit: PR wall median 39.8 minutes, `artifact-integrity` 9.3 minutes
per run). AFP-V0-024 reuses a result only for an identical whole tree; these requirements reuse
per package, and skip per job, only where the inputs the result depends on are proven unchanged.

## Requirements

- **AFP-V0-2320:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097)
  A Go package is eligible for result reuse only when (1) `.corvint/test-read-scopes.json`
  declares a read scope for its directory (AFP-V0-023), so CI runs its tests under
  `.github/testconfine`, which lets them read below the repository root only the package's own
  directory and the declared entries; (2) the affected graph of the same checkout does not name
  it an `UNBOUNDED_READER` unit; (3) `.github/cishards/test-split-allow.json` does not name it, so
  no CI run splits it into slices (AFP-V0-041); and (4) its input-closure digest (AFP-V0-2321)
  resolves. Otherwise it runs with reason `UNDECLARED_READ_SCOPE`, `UNBOUNDED_READER`,
  `SLICED_UNIT` or `DIGEST_ERROR`. A dirty or invalid read-scope or split declaration leaves no
  package eligible.
- **AFP-V0-2321:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097)
  The input-closure digest of package P is the SHA-256 of the canonical JSON
  `corvint-test-input-digest/0` document naming P, the identity digest (AFP-V0-2322), P's declared
  entries, and the sorted `(path, mode, blob)` of: every tracked file below P's directory; every
  tracked file below each declared `DIR/` entry (with a `dir` marker) and each declared file, or an
  `absent` marker for a declared path the tree lacks; `go.mod`, `go.sum`, `go.work` and
  `go.work.sum`, each present or `absent`; and the compiled and embedded files (`GoFiles`,
  `CFiles`, `CXXFiles`, `HFiles`, `SFiles`, `SysoFiles`, `EmbedFiles`) of every non-standard
  package `go list -deps -test` puts in P's test binary (P itself when it has no test files).
  Blob ids come from HEAD's tree. The digest is a `DIGEST_ERROR` with a named detail when an input
  is untracked (`untracked-input`), not a regular file blob (`irregular-input`), dirty, or has a
  modified, untracked or ignored worktree path at or below it (`dirty-input`); when a closure
  package is outside the repository (`external-package`) or uses cgo (`cgo-input`); when
  `go list` reports an error for any closure package (`go-list-error`, `closure-unlisted`,
  `package-unlisted`); or when a declared entry's form disagrees with the tree (`scope-form`).
- **AFP-V0-2322:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097)
  The identity is the canonical `corvint-test-reuse-identity/0` of the `go env` values of `CC`,
  `CGO_CFLAGS`, `CGO_CPPFLAGS`, `CGO_CXXFLAGS`, `CGO_ENABLED`, `CGO_LDFLAGS`, `CXX`, `GOAMD64`,
  `GOARCH`, `GOARM64`, `GODEBUG`, `GOEXPERIMENT`, `GOFLAGS`, `GOOS`, `GOTOOLCHAIN`, `GOVERSION` and
  `GOWORK`;
  the caller's `--argv` words; and the `(path, mode, blob)` of every tracked file of each
  `--harness` path, which must be clean. The argv MUST carry `-count=1` and MUST NOT carry `-run`,
  `-skip`, `-list`, `-short`, `-failfast`, `-shuffle`, `-bench` or `-fuzz` (with or without the
  `test.` prefix): a partial or cached run is never a package pass. A record matches only an equal
  identity digest; otherwise the unit runs with `IDENTITY_CHANGED`.
- **AFP-V0-2323:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097)
  `corvint test reuse record` writes one `corvint-test-reuse-record/0` to `--out`: the source
  (`kind` `main`, `pull_request` with its number, or `local`; run id and attempt; commit and tree,
  which MUST be HEAD's), the identity and its digest, and each package whose outcome was `pass`
  with its digest. Outcomes come from `corvint-ci-outcomes/0` documents (AFP-V0-047) whose target
  and tree are HEAD's, first-run outcomes only, a package any document did not pass being not
  passed; or from a `go test -json` stream, where a package passes when its package-level action
  is `pass` and none of its actions failed. Every other package of the run is listed in `skipped`
  with `NOT_PASSED` or its AFP-V0-2320 reason. It writes nothing else.
- **AFP-V0-2324:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097)
  `corvint test reuse decide` admits a record only when it parses strictly, its identity digest
  equals the digest of its own identity (`IDENTITY_DIGEST_MISMATCH` otherwise), its source kind
  is in `--trust` (default `local,main`), and a `pull_request` source names the `--pull-request`
  number; any other record is listed with `UNTRUSTED_SOURCE` or `RECORD_INVALID` and never
  reused. The caller that chose the record files is the trust boundary: in CI only the workflow
  lists runs of this repository by event, branch and pull-request number (AFP-V0-2327).
- **AFP-V0-2325:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097)
  The decision is one `corvint-test-reuse-decision/0`: HEAD, tree, identity, every record's
  verdict, and per package (`--packages`, default `go list ./...`) `REUSE` with
  `RECORD_DIGEST_EQUAL` and a witness naming the record, its source and commit, only when the
  package is eligible and an admitted record passed it with the same identity and digest; else
  `RUN` with exactly one of `UNDECLARED_READ_SCOPE`, `UNBOUNDED_READER`, `SLICED_UNIT`,
  `DIGEST_ERROR` (with detail), `NO_RECORD`, `IDENTITY_CHANGED` or `DIGEST_CHANGED`. `--run-list`
  writes the RUN packages. When no decision can be produced (no snapshot, no identity, an
  unwritable output) it exits 3, `test-reuse-incomplete`, and the caller runs its full plan.
- **AFP-V0-2326:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097)
  The local runner honours the same decisions: `script/heavy.sh --plan M` with `HEAVY_REUSE=D`
  prints `# reused go:P: passed at COMMIT (KIND record NAME), digest DIGEST (AFP-V0-2326)` in
  place of every command of a package D decides `REUSE` with `RECORD_DIGEST_EQUAL` and a witness,
  and counts them on its summary line. A decision whose head is not M's `source.head`, an
  unreadable one or one of another profile is not applied: one `heavy.sh: HEAVY_REUSE=PATH not
  applied: REASON` line and the full plan. `corvint test reuse record --go-test-json` records a
  local run (`--source-kind local`).
- **AFP-V0-2327:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097;
  effective only after the owner's `ci-control-plane` review) In `.github/workflows/ci.yml`, only
  while the repository variable `CORVINT_PR_REUSE` is `on`: (1) a `pull_request` or `merge_group`
  shard that ran its whole partition writes `ci-test-reuse-record-SHARD` from its own outcomes
  with the CI argv `-race -count=1 -timeout 50m` and harness `.github/cishards/test-commands.json`,
  `.github/testconfine` and `.github/workflows/ci.yml`, with `corvint` built from the tested
  merge's first parent; (2) a `test-reuse-plan` job of a `pull_request` run reads the records of
  earlier runs of the same pull request in this repository and of successful `merge_group` runs
  whose tested commit is an ancestor of the first parent, decides with `--trust
  main,pull_request --pull-request N`, and outputs `mode=REUSE` only when it reused a package; (3)
  each shard of that run whose narrowing gate did not narrow it then runs the cost-balanced
  placement (AFP-V0-590) of the decision's run list, so the remaining packages spread across the
  shards, and its coverage marker is `BALANCED` over that list (AFP-V0-591); (4) such a shard's
  `fullN` output stays empty, so it never writes an AFP-V0-170 tested-tree record and the merge
  queue, which never reads reuse records, runs in full; (5) any failure in (1) to (3) leaves the
  shard on its existing path.
- **AFP-V0-2333:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097)
  Acceptance replay: `tools/ci-reuse-replay` decides every pull-request re-push of at least 50
  merged pull requests that have retained outcomes against records of the previous push and of
  main, each made by the branch's own `corvint` at that run's tested merge commit, and reports zero
  packages decided `REUSE` whose outcome in the later run was `fail`, with the runner minutes and
  modelled shard wall time the reuse removes.
- **AFP-V0-2340:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1099)
  The project-owned `.corvint/ci-job-inputs.json` (`corvint-ci-job-inputs/0`) declares, per job of
  a workflow under `.github/workflows/`, its inputs: `paths` (sorted repository files and `DIR/`
  subtrees), `goPackages` (`./DIR` or `./DIR/...` patterns and the `GOOS/GOARCH` platforms they
  are built for), `goTests` (`./DIR` packages a job tests) and `exportedTree` (the job exports
  every tracked file). Unknown members, unsorted or escaping paths, duplicate jobs, a job with no
  input, more than 64 jobs or 256 entries are invalid.
- **AFP-V0-2341:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1099)
  `corvint affected jobs --base FULL_COMMIT_ID [--declaration PATH]` evaluates the declaration
  over the paths of the range and the dirty worktree, exactly the parent plan's `dirty` set, and
  writes one `corvint-ci-job-plan/0`: base, revision, declaration path and digest, changed-path
  count, the plan scope, and per declared job `RUN` or `SKIP`, its reason and its witnesses (at
  most 16, with `omitted`). It is a read command. An unreadable or invalid declaration exits 3
  with `declarationError` `DECLARATION_INVALID` and no job; a planning failure exits 2, as does a
  tree it cannot list (`unsupported-affected-jobs`); either way
  the caller runs every job.
- **AFP-V0-2342:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1099)
  A job is `RUN` when any of these holds, named by the first that holds, each holding one
  witnessed: `WORKFLOW_CHANGED` (its workflow file), `DECLARATION_CHANGED` (the declaration),
  `INPUT_PATH_CHANGED` (a declared file or a path below a declared `DIR/`, or the directory
  itself), `GO_MODULE_CHANGED` (`go.mod`, `go.sum`, `go.work` or `go.work.sum`, for a job with
  `goPackages` or `goTests`), `GO_PACKAGE_DIRECTORY_CHANGED` (a changed path, deleted ones
  included, directly in the directory of a non-standard package `go list -deps` lists for any
  declared platform with cgo off or on), `GO_TEST_SELECTED` (the plan selects a `goTests` unit),
  `PLAN_SCOPE_UNKNOWN` (a `goTests` job whose plan scope is not `BOUNDED`), the AFP-V0-2343 tree
  reasons, or `EVALUATION_ERROR` (a failed `go list`, a package outside the repository, a failed
  plan or no root module). Otherwise it is `SKIP` with `NO_DECLARED_INPUT_CHANGED` and one
  `INPUTS_EVALUATED` witness counting what was evaluated. SKIP is never a pass of the job.
- **AFP-V0-2343:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1099)
  An `exportedTree` job also runs on `IRREGULAR_TREE_ENTRY` (a changed path HEAD holds as other
  than a `100644` or `100755` blob), `OVERSIZED_TREE_ENTRY` (a changed blob over 128 MiB) and
  `TREE_LISTING_LARGE` (HEAD's `git ls-tree -r -z --full-tree` listing over 4 MiB, half the
  export's 8 MiB capture bound), the conditions the AFP-V0-163 archive export refuses.
- **AFP-V0-2344:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1099;
  amends AFP-V0-013; effective only after the owner's `ci-control-plane` review) AFP-V0-013's
  "build/interop checks MUST remain full" admits one exception: only while the repository variable
  `CORVINT_PR_JOB_PLAN` is `on`, a `pull_request` run of `ci.yml` MAY skip `go-interop` and
  `artifact-integrity` when a `job-plan` job, running `corvint affected jobs` built from the tested
  merge's first parent over `FIRST_PARENT..MERGE`, decided that job `SKIP`. A missing, failed or
  unparsable plan, or any decision other than SKIP, runs the job. `merge_group`, `push` and
  release runs stay full, so AFP-V0-080 and AFP-V0-163 keep their inputs: a pull request whose
  `artifact-integrity` was skipped writes no integrity record and its queue entry runs the gate.
  The `containment` job of `intake-containment.yml` is declared and evaluated but not wired: its
  run (about two minutes) costs about what building `corvint` for its plan would.
- **AFP-V0-2346:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1099)
  Acceptance replay: `tools/ci-reuse-replay jobs` evaluates the declaration with the branch's own
  `corvint` over each of the last 100 completed `pull_request` and `push` runs of `ci.yml` at the
  run's head, and reports no `SKIP` of a job whose conclusion in that run was `failure`, with the
  runner minutes the observed SKIPs would have saved.

## Non-goals

No reuse for an undeclared package: without the confinement its reads are bounded only by the
planner's inference, which is not enforced. No reuse of a sliced package or of a slice outcome.
No reuse in a `merge_group` or `push` run, which stay full (AFP-V0-026, AFP-V0-024). No reuse
across identities: a local record never stands for a CI run or the reverse. No hosted-runner
image, host package, network or clock identity: those stay residual, as in AFP-V0-024. The job
plan does not skip a required check, a test job (`go-product`, `go-static`) or a release job, and
does not infer a job's inputs from its steps: the declaration is the owner's claim, checked by the
replay and by the fault injections below.

## Failure modes

Every failure runs the work. An unreadable record is `RECORD_INVALID`; an edited identity is
`IDENTITY_DIGEST_MISMATCH`; a record of another pull request is `UNTRUSTED_SOURCE`. A test that
passed only on its AFP-V0-441 rerun is not recorded. A flaky package that passed on an earlier push
can be reused on unchanged inputs; this is the residual every result cache carries, and the
replay counts it. A test that tolerates a denied read under the confinement could pass while
depending on an undeclared file; such a dependency is outside the digest, as it is outside the
AFP-V0-023 scope the planner already trusts. Locally the confinement is not enforced, so a local
record is trusted only for local decisions. A wrong job declaration skips a job that should run;
the declaration is project authority, reviewed like code, and every change to it runs every job.
A `go list` that cannot list a declared platform runs the job. `heavy.sh` prints the full plan for
any decision it cannot apply.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-2320 | `DigestPackage` eligibility checks in `internal/testreuse/testreuse.go`; `readDeclarations` in `observe.go`; `testReuseObserve` in `cmd/corvint/test_reuse.go` | `TestRecordCarriesOnlyEligiblePasses`, `TestUnboundedAndSlicedUnitsRun`, `TestDirtyOrUntrackedInputRuns` in `internal/testreuse/testreuse_test.go` |
| AFP-V0-2321 | `DigestPackage`, `Snapshot.closure`, `Snapshot.dirtyWithin`, `Observe` in `internal/testreuse` | `TestDigestNegativeControls` (ten controls), `TestDirtyOrUntrackedInputRuns`, `TestFaultInjectionIsSelectedAndFails` |
| AFP-V0-2322 | `Identity`, `CheckArgv`, `ObserveIdentity` in `internal/testreuse` | `TestCheckArgv`, `TestIdentityAndTrust` |
| AFP-V0-2323 | `BuildRecord`, `ParseOutcomes`, `MergeOutcomes`, `ParseTestJSON` in `internal/testreuse/record.go`; `testReuseRecord` | `TestRecordCarriesOnlyEligiblePasses`, `TestMergeOutcomesRefusesAnotherCommit`, `TestParseTestJSON` |
| AFP-V0-2324 | `ParseRecord` in `record.go`; record admission in `Decide` (`decide.go`) | `TestIdentityAndTrust` |
| AFP-V0-2325 | `Decide`; `testReuseDecide` in `cmd/corvint/test_reuse.go` | `TestDigestNegativeControls`, `TestFaultInjectionIsSelectedAndFails` |
| AFP-V0-2326 | `HEAVY_REUSE` in `manifest_perl` and `print_plan` of `script/heavy.sh` | the AFP-V0-2326 group of `script/lane-scripts_test.sh` |
| AFP-V0-2327 | the reuse steps and `test-reuse-plan` job of `.github/workflows/ci.yml` (separate commit) | `NOT_OBSERVED` hosted; structural review |
| AFP-V0-2333 | `tools/ci-reuse-replay/fetch.py`, `replay.py tests` | build-log entry `2026-11-03-reuse-verified-results` |
| AFP-V0-2340 | `Parse` in `internal/jobinputs/jobinputs.go`; `.corvint/ci-job-inputs.json` | `TestParseRefusesInvalidDeclarations`, `TestRepositoryDeclarationParses` |
| AFP-V0-2341 | `parseAffectedJobsOptions`, `runAffectedJobs`, `affectedJobsTree` in `cmd/corvint/affected_jobs.go` | `TestAffectedJobsCLI` in `cmd/corvint/affected_jobs_test.go` |
| AFP-V0-2342 | `Evaluate`, `GoDirectories` in `internal/jobinputs` | `TestEvaluateSkipsOnlyWithoutDeclaredInputChange`, `TestEvaluateFailsClosed`, `TestGoDirectoriesUnionsPlatforms` (fault injection), `TestInteropFormattingFaultIsRun` (fault injection) |
| AFP-V0-2343 | the `ExportedTree` branch of `Evaluate` | `TestEvaluateExportedTree` |
| AFP-V0-2344 | the `job-plan` job and `if:` conditions of `.github/workflows/ci.yml` (separate commit) | `NOT_OBSERVED` hosted; structural review |
| AFP-V0-2346 | `replay.py jobs` | build-log entry `2026-11-03-reuse-verified-results` |

## Rollback

For the CI paths, unset `CORVINT_PR_REUSE` or `CORVINT_PR_JOB_PLAN` (every shard and job then
runs as before), or revert the `.github` commit. For the product, revert the lane's commits:
delete `internal/testreuse`, `internal/jobinputs`, `cmd/corvint/test_reuse.go`,
`cmd/corvint/affected_jobs.go`, `.corvint/ci-job-inputs.json` and `tools/ci-reuse-replay`, drop
the `reuse` and `jobs` dispatch, help and catalog rows and the `HEAVY_REUSE` branch of
`script/heavy.sh`, restore AFP-V0-013's status line, and run `make spec-requirements` after
deleting this file.
