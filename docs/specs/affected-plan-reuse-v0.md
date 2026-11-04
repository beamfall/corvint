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
- Status: per requirement, each experimental under the owner's standing lane auto-accept (2026-10-11); delivery experimental; CI reuse is hard-disabled pending a protected execution-evidence collector; job skips stay dark until control-plane review and opt-in.
- Exists: `internal/testreuse` (`DigestPackage`, `Observe`, `BuildRecord`, `Decide`), `cmd/corvint/test_reuse.go`, `HEAVY_REUSE` in `script/heavy.sh`, `internal/jobinputs` (`Parse`, `Evaluate`, `GoDirectories`), `cmd/corvint/affected_jobs.go`, `.corvint/ci-job-inputs.json`, `tools/ci-reuse-replay`
- Blocked on: a protected execution-evidence collector, confined (`--confine`) record and decide steps, and the owner's `ci-control-plane` review; hosted reuse and skips are `NOT_OBSERVED`. Unconfined local runs (`heavy.sh`) never reuse.
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

- **AFP-V0-2320:** (experimental under the owner's standing lane auto-accept (2026-10-11); amended 2026-11-04 (rounds 3 and 4) for review soundness; V1-1097)
  A Go package is eligible for result reuse only when (1) `.corvint/test-read-scopes.json`
  declares a read scope for its directory (AFP-V0-023), so CI runs its tests under
  `.github/testconfine`, which lets them read below the repository root only the package's own
  directory and the declared entries; (2) the affected graph of the same checkout does not name
  it an `UNBOUNDED_READER` unit; (2a) `.corvint/existence-readers.json` (AFP-V0-980) does not
  declare its directory an existence reader, because such tests read the existence or mode of
  paths outside every digest and the confinement does not deny those reads (amended 2026-11-04,
  round 4); (3) `.github/cishards/test-split-allow.json` does not name it, so
  no CI run splits it into slices (AFP-V0-041); (4) the identity binds the confinement wrapper
  the tests ran under as `go test -exec` (`--confine PATH`, AFP-V0-2322), because a declaration
  alone enforces nothing and an unconfined test's out-of-scope read is in no digest; and (5) its
  input-closure digest (AFP-V0-2321) resolves. Otherwise it runs with reason
  `UNDECLARED_READ_SCOPE`, `UNBOUNDED_READER`, `EXISTENCE_READER`, `SLICED_UNIT`,
  `UNCONFINED_EXECUTION` or `DIGEST_ERROR`, checked in that order. An existence-reader
  declaration that cannot be read fails the affected graph, so no snapshot is produced and the
  caller runs its full plan (`test-reuse-incomplete`). A record of an unconfined run therefore carries no
  package, and a decision without `--confine` reuses none. A dirty or invalid read-scope or
  split declaration leaves no package eligible.
- **AFP-V0-2321:** (experimental under the owner's standing lane auto-accept (2026-10-11); amended 2026-11-04 for review soundness, and in round 4; V1-1097)
  The input-closure digest of package P is the SHA-256 of the canonical JSON
  `corvint-test-input-digest/0` document naming P, the identity digest (AFP-V0-2322), P's declared
  entries, and the sorted `(path, mode, blob)` of: every tracked file below P's directory; every
  tracked file below each declared `DIR/` entry (with a `dir` marker) and each declared file, or an
  `absent` marker for a declared path the tree lacks; `go.mod`, `go.sum`, `go.work` and
  `go.work.sum`, each present or `absent`; every closure module's effective `go.mod`
  and adjacent `go.sum` (including in-repository replacements, checksum present or `absent`); and the compiled and embedded files (`GoFiles`,
  `CFiles`, `CXXFiles`, `HFiles`, `SFiles`, `SysoFiles`, `EmbedFiles`) of every non-standard
  package `go list -deps -test` puts in P's test binary (P itself when it has no test files).
  Blob ids come from HEAD's tree. The digest is a `DIGEST_ERROR` with a named detail when an input
  is untracked (`untracked-input`), not a regular file blob (`irregular-input`), dirty, or has a
  modified, untracked or ignored worktree path at or below it (`dirty-input`), including inputs hidden by assume-unchanged or skip-worktree index flags; when a closure
  package is outside the repository (`external-package`), uses cgo (`cgo-input`), or has
  assembly sources (`assembly-input`, because `go list` omits escaping includes); when a
  non-standard closure member lacks module metadata or its effective module directory/manifest
  is outside the repository or not the directory's `go.mod` (`module-input`); when
  `go list` reports an error for any closure package (`go-list-error`, `closure-unlisted`,
  `package-unlisted`); or when a declared entry's form disagrees with the tree (`scope-form`).
  A clean status does not prove the worktree holds each blob's bytes: a checkout attribute
  (`eol`, `text`, `filter`, `ident`, `working-tree-encoding`, from any `.gitattributes` or
  `info/attributes`), `core.autocrlf`, or `core.fileMode=false` can change what a test reads while
  the blob and the status stay unchanged. So after the dirty check every regular-file input is
  read raw from the worktree, as a regular file (not followed through a link), and must hash, as a
  Git blob of HEAD's object format, to its tree blob, with an owner executable bit that matches its
  mode outside Windows; otherwise the digest is a `DIGEST_ERROR` with `worktree-bytes` naming the
  first such path (amended 2026-11-04, round 4). One observation reads HEAD, its tree, the status
  and the index flags first and rereads them after its last read of the checkout (in the CLI,
  after the affected graph and the identity); any change refuses the observation with
  `snapshot-drift`, exit 3 `test-reuse-incomplete` (amended 2026-11-04, round 4).
- **AFP-V0-2322:** (experimental under the owner's standing lane auto-accept (2026-10-11); amended 2026-11-04 for review soundness, and in rounds 3 and 4; V1-1097)
  The identity is the canonical `corvint-test-reuse-identity/0` of the `go env` values of `CC`,
  `CGO_CFLAGS`, `CGO_CPPFLAGS`, `CGO_CXXFLAGS`, `CGO_ENABLED`, `CGO_LDFLAGS`, `CXX`, `GO386`, `GOAMD64`,
  `GOARCH`, `GOARM`, `GOARM64`, `GODEBUG`, `GOEXPERIMENT`, `GOFLAGS`, `GOMIPS`, `GOMIPS64`, `GOOS`,
  `GOPPC64`, `GORISCV64`, `GOTOOLCHAIN`, `GOVERSION`, `GOWASM` and `GOWORK`;
  the SHA-256 `runtimeEnvDigest` of the canonical full supplied process environment
  (all names and values, not an allowlist, including test qualification controls and child-process
  configuration); the caller's `--argv` words; the `(path, mode, blob)` of every tracked file of each
  `--harness` path, which must be clean and, as for AFP-V0-2321, hold its blob's raw bytes
  (`worktree-bytes` refuses the identity otherwise; amended 2026-11-04, round 4); and `confine`, the `sha256:` digest of the content of the
  `--confine` wrapper (a regular executable file; anything else is refused with exit 3), or empty
  when the run was unconfined (amended 2026-11-04, round 3). The wrapper's path is not bound, since
  it varies between runners; a changed wrapper is `IDENTITY_CHANGED`. `--argv` still refuses
  `-exec`: confinement is named only through `--confine`. Harness directories carry a `dir` marker so local
  re-evaluation also discovers added files. The runtime environment digest uses a name/value
  map with last-value-wins duplicates (case-folded names on Windows), independent of entry order,
  distinguishing unset from an empty value. Arbitrary environment names/values are never written
  into the identity; only their digest is retained, alongside the existing resolved Go values.
  Older records without the runtime environment binding cannot authorize reuse. The argv MUST carry `-count=1` and MUST NOT carry `-run`,
  `-skip`, `-list`, `-short`, `-failfast`, `-shuffle`, `-bench` or `-fuzz` (with or without the
  `test.` prefix): a partial or cached run is never a package pass. A record matches only an equal
  identity digest; otherwise the unit runs with `IDENTITY_CHANGED`. Closure discovery receives the
  argv's build flags. Supported boolean build flags are `race`, `msan`, `asan`, `cover` and
  `trimpath`; supported valued build flags are `tags`, `covermode` and `coverpkg`; supported
  valued execution flags are `timeout`, `p`, `parallel` and `cpu`; supported boolean execution
  flags are `v` and `json`. `GOFLAGS` is checked against the same allowlist. Unknown flags,
  file-valued compiler overrides, partial execution flags and invalid values refuse reuse.
  `GOWORK` must be empty, `off`, or the repository root's `go.work`, whose bytes AFP-V0-2321
  pins; any other workspace refuses the observation with `unmodeled-workspace` because its
  contents (including runtime default directives) are not part of the digest.
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
- **AFP-V0-2325:** (experimental under the owner's standing lane auto-accept (2026-10-11); amended 2026-11-04 (round 4); V1-1097)
  The decision is one `corvint-test-reuse-decision/0`: HEAD, tree, identity, every record's
  verdict, and per package (`--packages`, default `go list ./...`) `REUSE` with
  `RECORD_DIGEST_EQUAL` and a witness naming the record, its source and commit, only when the
  package is eligible and an admitted record passed it with the same identity and digest; else
  `RUN` with exactly one of `UNDECLARED_READ_SCOPE`, `UNBOUNDED_READER`, `EXISTENCE_READER`,
  `SLICED_UNIT`, `UNCONFINED_EXECUTION`, `DIGEST_ERROR` (with detail), `NO_RECORD`, `IDENTITY_CHANGED` or `DIGEST_CHANGED`. `--run-list`
  writes the RUN packages. When no decision can be produced (no snapshot, no identity, an
  unwritable output) it exits 3, `test-reuse-incomplete`, and the caller runs its full plan.
- **AFP-V0-2326:** (experimental under the owner's standing lane auto-accept (2026-10-11); amended 2026-11-04 (round 3); V1-1097)
  The local runner re-evaluates the saved decision against current inputs and identity: `script/heavy.sh --plan M` with `HEAVY_REUSE=D`
  prints `# reused go:P: passed at COMMIT (KIND record NAME), digest DIGEST (AFP-V0-2326)` in
  place of every command of a package D decides `REUSE` with `RECORD_DIGEST_EQUAL` and a witness,
  and counts them on its summary line. Before suppressing commands it invokes
  `${CORVINT_BIN:-corvint} test reuse decide` with the saved decision's admitted record paths and
  harness paths (record names preserve the caller's paths), and the printed plan's argv `-count=1 -timeout 30m`. The caller trusts these local
  paths as it trusts the original decision. The printed plan runs plain `go test` with no
  confinement wrapper and the re-evaluation passes no `--confine`, so under AFP-V0-2320 (4) it
  reuses no package until local runs are confined (amended 2026-11-04, round 3): the suppression
  path stays wired and fails closed. Missing records run the package; a failed re-evaluation
  prints `reuse re-evaluation failed` and the full plan. A decision whose head is not M's `source.head`, an
  unreadable one or one of another profile is not applied: one `heavy.sh: HEAVY_REUSE=PATH not
  applied: REASON` line and the full plan. `corvint test reuse record --go-test-json` records a
  local run (`--source-kind local`).
- **AFP-V0-2327:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1097;
  effective only after the owner's `ci-control-plane` review and a protected execution-evidence
  collector) The CI reuse producer and consumer are hard-disabled (`if: false && ...`), with hold
  `UNAUTHENTICATED_EXECUTION`: PR test processes can write the recorder and outcomes in their
  runner, so those artifacts do not establish trusted execution. A repository variable alone MUST
  NOT enable this path. Removing the hold requires a separately reviewed collector boundary that
  tested code cannot write and its falsification evidence, and record and decide steps that pass
  `--confine "$RUNNER_TEMP/test-confine"` (AFP-V0-2320 (4)), which the dormant steps do not yet
  (amended 2026-11-04, round 3). The dormant wiring is as follows. In `.github/workflows/ci.yml`, only
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
- **AFP-V0-2333:** (experimental under the owner's standing lane auto-accept (2026-10-11); amended 2026-11-04 for review soundness; V1-1097)
  Acceptance replay: `tools/ci-reuse-replay` decides every pull-request re-push of at least 50
  merged pull requests that have retained outcomes against records of the previous push and of
  main, each made by the branch's own `corvint` at that run's tested merge commit, and reports zero
  packages decided `REUSE` whose outcome in the later run was `fail`, with the runner minutes and
  modelled shard wall time the reuse removes. Every invocation MUST recompute records and
  decisions from the supplied binary, environment, source outcomes and historical checkout;
  existing output files never authorize loading old results and are removed before each
  recomputation, so a failed one leaves none behind. Only records freshly computed
  within that same invocation may be memoized. A recomputation failure remains `NOT_PRODUCED`
  and cannot fall back to an older file or enter the decided/savings denominator.
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
- **AFP-V0-2342:** (experimental under the owner's standing lane auto-accept (2026-10-11); amended 2026-11-04 (rounds 3 and 4); V1-1099)
  A job is `RUN` when any of these holds, named by the first that holds, each holding one
  witnessed: `WORKFLOW_CHANGED` (its workflow file), `DECLARATION_CHANGED` (the declaration),
  `CHECKOUT_ATTRIBUTES_CHANGED` (a changed `.gitattributes` at any depth, for every job, because a
  checkout or export attribute can change a declared input's worktree bytes or exported tree
  without changing its blob; a superset of the ancestor files of the declared inputs, chosen fail
  closed; amended 2026-11-04, round 4),
  `INPUT_PATH_CHANGED` (a declared file or a path below a declared `DIR/`, or the directory
  itself), `GO_MODULE_CHANGED` (`go.mod`, `go.sum`, `go.work` or `go.work.sum`, for a job with
  `goPackages` or `goTests`, and the `go.mod` or `go.sum` of the module of any package of the
  job's closure (amended 2026-11-04, round 3), witnessed with that package and platform),
  `GO_PACKAGE_DIRECTORY_CHANGED` (a changed path, deleted ones
  included, at or below the directory of a non-standard package `go list -deps` lists for any
  declared platform with cgo off or on, so an embedded subtree counts), `GO_TEST_SELECTED` (the plan selects a `goTests` unit),
  `PLAN_SCOPE_UNKNOWN` (a `goTests` job whose plan scope is not `BOUNDED`), the AFP-V0-2343 tree
  reasons, or `EVALUATION_ERROR` (a failed `go list`, a package outside the repository, a failed
  plan, no root module, or, amended 2026-11-04 (round 3), a closure package that has cgo files
  (`cgo-input`) or assembly files (`assembly-input`), whose `#include`s may name files outside
  every package directory and go list does not name them, or whose module metadata is missing or
  names a module directory or `go.mod` outside the repository (`module-input`)). Otherwise it is `SKIP` with `NO_DECLARED_INPUT_CHANGED` and one
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
- **AFP-V0-2346:** (experimental under the owner's standing lane auto-accept (2026-10-11); amended 2026-11-04 for review soundness; V1-1099)
  Acceptance replay: `tools/ci-reuse-replay jobs` evaluates the declaration with the branch's own
  `corvint` over each of the last 100 completed `pull_request` and `push` runs of `ci.yml` at the
  run's head, and reports no `SKIP` of a job whose conclusion in that run was `failure`, with the
  runner minutes the observed SKIPs would have saved. Every invocation MUST regenerate job
  plans using the supplied binary, environment, declaration and source evidence, irrespective
  of existing output files, each removed before its plan is regenerated. A planning failure stays
  `NOT_PRODUCED`, never a stale plan or savings.

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
run without `--confine` records and reuses nothing (`UNCONFINED_EXECUTION`). `--confine` binds a
wrapper by content and trusts the caller that the tests ran under it, as the argv is trusted; a
wrapper that does not enforce the scope defeats the binding. A job closure package with cgo or
assembly files, whose includes the planner cannot bound, runs the job (`EVALUATION_ERROR`). A test that reads the existence or mode of an undeclared path outside its scope is not in any
digest and the confinement does not deny such reads; a declared existence reader runs
(`EXISTENCE_READER`), an undeclared one is the residual the AFP-V0-980 declaration already carries
for the planner. Checkout configuration outside the tree (`info/attributes`, `core.autocrlf`,
`core.fileMode`) is caught by the raw worktree check, not by a job plan, which reads only the
tree; it can therefore still skip a job whose runner checkout differs, as for any other runner
configuration. A wrong job declaration skips a job that should run;
the declaration is project authority, reviewed like code, and every change to it runs every job.
A `go list` that cannot list a declared platform runs the job. `heavy.sh` prints the full plan for
any decision it cannot apply.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-2320 | `DigestPackage` eligibility checks in `internal/testreuse/testreuse.go`; `readDeclarations` in `observe.go`; `testReuseObserve` in `cmd/corvint/test_reuse.go` | `TestRecordCarriesOnlyEligiblePasses`, `TestUnboundedAndSlicedUnitsRun`, `TestDirtyOrUntrackedInputRuns` in `internal/testreuse/testreuse_test.go`; `TestUnconfinedOutOfScopeReadRuns` (fault injection: an out-of-scope read changes and the rerun fails) in `internal/testreuse/review_test.go`; the `UNCONFINED_EXECUTION` step of `TestTestReuseCLI`; `TestTestReuseExistenceReaderRuns` (fault injection: a confined existence reader's stat target is deleted and the rerun fails; an undeclared sibling is the negative control) in `cmd/corvint/test_reuse_test.go` |
| AFP-V0-2321 | `DigestPackage`, `Snapshot.closure`, `Snapshot.dirtyWithin`, `Observe` in `internal/testreuse` | `TestDigestNegativeControls` (ten controls), `TestDirtyOrUntrackedInputRuns`, `TestFaultInjectionIsSelectedAndFails`, `TestBuildFlagsReachTheClosure`, `TestHiddenIndexFlagsAreDirty`, `TestDependencyModuleLanguageFault`, `TestAssemblyIncludedHeaderFault` in `internal/testreuse/review_test.go`; `TestCheckoutAttributeTransformationRuns` (fault injection: a CRLF attribute keeps a clean status and the rerun fails; a non-transforming attribute is the negative control), `TestHiddenModeChangeRuns`, `TestConcurrentEditDuringListingIsDrift` (fault injection: an edit during go list fails the rerun), `TestRecheckRefusesLaterDrift` in `internal/testreuse/worktree_test.go` (`Snapshot.Recheck`, `Snapshot.worktreeObject`) |
| AFP-V0-2322 | `Identity`, `CheckArgv`, `ObserveIdentity` in `internal/testreuse` | `TestCheckArgv`, `TestIdentityAndTrust`, `TestArgvFlagsAreClassified`, `TestHarnessDirectoryReevaluation`, `TestUnsupportedGOFLAGSFailClosed`, `TestWorkspaceInputsFailClosed`, `TestRuntimeEnvironmentEnablesSkippedFailure`, `TestRuntimeEnvironmentCanonicalization`, `TestConfinementWrapperBindsIdentity` in `internal/testreuse/review_test.go`; `ConfineDigest`; the harness step of `TestCheckoutAttributeTransformationRuns` |
| AFP-V0-2323 | `BuildRecord`, `ParseOutcomes`, `MergeOutcomes`, `ParseTestJSON` in `internal/testreuse/record.go`; `testReuseRecord` | `TestRecordCarriesOnlyEligiblePasses`, `TestMergeOutcomesRefusesAnotherCommit`, `TestParseTestJSON` |
| AFP-V0-2324 | `ParseRecord` in `record.go`; record admission in `Decide` (`decide.go`) | `TestIdentityAndTrust` |
| AFP-V0-2325 | `Decide`; `testReuseDecide` in `cmd/corvint/test_reuse.go` | `TestDigestNegativeControls`, `TestFaultInjectionIsSelectedAndFails` |
| AFP-V0-2326 | `HEAVY_REUSE` in `manifest_perl` and `print_plan` of `script/heavy.sh` | the AFP-V0-2326 group of `script/lane-scripts_test.sh` |
| AFP-V0-2327 | the reuse steps and `test-reuse-plan` job of `.github/workflows/ci.yml` (separate commit) | `NOT_OBSERVED` hosted; structural review |
| AFP-V0-2333 | `tools/ci-reuse-replay/fetch.py`, `replay.py tests` | build-log entry `2026-11-03-reuse-verified-results`; `ReplayFreshnessTest.test_records_and_decisions_regenerated` in `tools/ci-reuse-replay/replay_test.py` |
| AFP-V0-2340 | `Parse` in `internal/jobinputs/jobinputs.go`; `.corvint/ci-job-inputs.json` | `TestParseRefusesInvalidDeclarations`, `TestRepositoryDeclarationParses` |
| AFP-V0-2341 | `parseAffectedJobsOptions`, `runAffectedJobs`, `affectedJobsTree` in `cmd/corvint/affected_jobs.go` | `TestAffectedJobsCLI` in `cmd/corvint/affected_jobs_test.go` |
| AFP-V0-2342 | `Evaluate`, `GoDirectories` in `internal/jobinputs` | `TestEvaluateSkipsOnlyWithoutDeclaredInputChange`, `TestEvaluateFailsClosed`, `TestGoDirectoriesUnionsPlatforms` (fault injection), `TestInteropFormattingFaultIsRun` (fault injection), `TestJobDependencyModuleLanguageFault`, `TestJobEscapingIncludeFault` (fault injection), `TestCheckoutAttributesRunEveryJob` (fault injection: an attribute-only commit changes a declared input's checkout bytes; an unrelated path is the negative control) in `internal/jobinputs/review_test.go` |
| AFP-V0-2343 | the `ExportedTree` branch of `Evaluate` | `TestEvaluateExportedTree` |
| AFP-V0-2344 | the `job-plan` job and `if:` conditions of `.github/workflows/ci.yml` (separate commit) | `NOT_OBSERVED` hosted; structural review |
| AFP-V0-2346 | `replay.py jobs` | build-log entry `2026-11-03-reuse-verified-results`; `ReplayFreshnessTest.test_job_plans_regenerated` in `tools/ci-reuse-replay/replay_test.py` |

## Rollback

The CI reuse path remains held as `UNAUTHENTICATED_EXECUTION`. For the CI paths, unset `CORVINT_PR_REUSE` or `CORVINT_PR_JOB_PLAN` (every shard and job then
runs as before), or revert the `.github` commit. For the product, revert the lane's commits:
delete `internal/testreuse`, `internal/jobinputs`, `cmd/corvint/test_reuse.go`,
`cmd/corvint/affected_jobs.go`, `.corvint/ci-job-inputs.json` and `tools/ci-reuse-replay`, drop
the `reuse` and `jobs` dispatch, help and catalog rows and the `HEAVY_REUSE` branch of
`script/heavy.sh`, restore AFP-V0-013's status line, and run `make spec-requirements` after
deleting this file.
