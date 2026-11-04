# CI Narrowing V0

Owner: Russell Lewis
Date: 2026-10-30
Requirement prefix: `CIN-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3309, V1-3091, V1-3158, V1-3227, V1-2902, V1-3329); CIN-V0-001 decided by the orchestrator under owner delegation (V1-3314), applying the AFP-V0-030 re-arm rule; effective only after the owner's `ci-control-plane` review of the `.github` change
Delivery status: experimental
Authoritative inputs: native tickets V1-3314, V1-3309 (AC3), V1-3091, V1-3158, V1-3227,
V1-2902 and V1-3329; CIN-V0-860: V1-3354; CIN-V0-861, CIN-V0-2301, CIN-V0-2302: V1-3332; CIN-V0-2300: V1-4446; CIN-V0-2303: V1-4445; CIN-V0-2304: V1-2805; CIN-V0-010: V1-3748; CIN-V0-011: V1-4354; CIN-V0-960: V1-3330. These requirements operate the CI narrowing of the [affected plan](affected-plan-v0.md)
(AFP-V0-028..030, AFP-V0-046, AFP-V0-097, AFP-V0-670, AFP-V0-730..734); that spec's
Requirements section is at the 256-obligation OCM limit (`maxObligations`), so they live here.
Requirement IDs CIN-V0-001 to CIN-V0-049 are reserved for this spec.
Lane cir-w35 (V1-4232, V1-4233, V1-4234) defines AFP-V0-100 to AFP-V0-106 here under the parent's
`AFP-V0` prefix for the same reason; see build-log entry `2026-11-01-cir-w35`.

## Agent digest
- Claim: CI narrowing isolates producing epochs and provides an operator alarm for missing forward rows independent of selection documents.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3309, V1-3091, V1-3158, V1-3227, V1-2902, V1-3329); CIN-V0-001 decided by the orchestrator under owner delegation (V1-3314), applying the AFP-V0-030 re-arm rule; effective only after the owner's `ci-control-plane` review of the `.github` change; delivery experimental.
- Exists: `.github/workflows/ci.yml`, `.github/workflows/ci-forward-bar.yml`, `.github/workflows/ci-control-plane.yml`, `.github/forward-rows/{names.sh,bar.jq,verdict.sh,accept.sh,selection.jq,collect.sh,absence.jq,narrow.sh}`, `.github/workflows/cache-prune.yml`, `Decide` in `tools/ci-reuse-plan/main.go`; `tools/ci-forward-row/absence-alarm.py` (standalone developer alarm, hosted scheduling pending); tests named for each requirement.
- Blocked on: the `CORVINT_PR_TOOL_SOURCE` and `CORVINT_TEST_SHADOW_SOURCE` repin (PR #11) for any forward row; the live V1-3158 measurement after merge.
- Read next: Requirements; Failure modes; Acceptance evidence and traceability.

## Intent and scope

Intent (orchestrator lane request, 2026-10-30): GitHub CI SHALL run only the tests a change needs
on every pull request, merge-queue entry and `main` push, without loosening any safety rule. Since
the 1.0 mint the tool and shadow pins name a commit absent from the repository (V1-3309): every
forward row was `INCOMPLETE`, five of them keep the AFP-V0-030 kill switch tripped (V1-3314), and
the failure was silent. The verdict and the pull-request acceptance listed the whole artifact
store on every run (V1-3091); the `main` push of a commit whose queue run had passed narrowed ran
all 8 shards again (V1-3158); a DOCS or REUSE decision still started 8 shard jobs that ran no
test (V1-2902); no CI check refused an AI co-author trailer (V1-3227); and no CI job ran the tests
of the nested module `tools/local-authority` (V1-3329).

Unchanged and binding: a `.github/` change forces FULL; UNKNOWN or any unreadable input forces
FULL; the AFP-V0-029 bar (8 complete rows, at least 1 failing, 0 `MISS`; lowered from 20 and 5
by the orchestrator under owner delegation, owner chat 2026-10-31) and the AFP-V0-030
kill switch keep their thresholds; no row is seeded or synthetic.

## Requirements

- **CIN-V0-001:** (decided by the orchestrator under owner delegation, V1-3314; applies the
  AFP-V0-030 re-arm rule) `CORVINT_PR_FORWARD_EPOCH` in `ci.yml`, `FORWARD_EPOCH` in
  `ci-forward-bar.yml` and the narrowing-verdict acceptance, and the verdict artifact name
  `afp-shadow-bar-epoch-2` SHALL advance together to `2`, so no epoch-1 row (the dead-pin
  `INCOMPLETE` rows among them) counts toward the AFP-V0-029 bar or the AFP-V0-030 window. The
  advance clears the window only: the bar, the kill-switch thresholds and the `MISS` rule are
  unchanged, and narrowing stays FULL until 20 genuine epoch-2 rows exist. A row's epoch SHALL be
  the producing run's own: the shadow step of that run stamps its `CORVINT_PR_FORWARD_EPOCH` as
  `forwardEpoch` into its selection document, and `collect.sh` SHALL name the row with that value,
  never with the collector's. A row is written only when every retained selection document of
  the selection run carries one identical non-negative integer epoch equal to the collector's
  `FORWARD_EPOCH`; a missing, mixed, non-integer or other epoch, or no selection document at all,
  writes no row, so such a run is never counted, not even as `INCOMPLETE`.
- **CIN-V0-002:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3309
  AC3) The pin capture and build steps of each `go-product-shard` keep `continue-on-error`, so
  the full suite still runs, but when any of them fails a final step SHALL fail the shard with an
  `::error` annotation and a step-summary line naming the pin variable, its value and whether the
  fetch or the build failed. A dead pin is then a red required check until it is repinned.
- **CIN-V0-003:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3091)
  `ci-forward-bar.yml` SHALL carry `FORWARD_EPOCH_SINCE`, a UTC time no later than the merge that
  advanced `main` to the epoch, and `verdict.sh` SHALL stop listing artifacts once a listed
  artifact is older than it. A listing that reaches it counts as covering the epoch, as an
  exhausted listing does; a malformed value fails the verdict job, and an unset value keeps the
  whole listing. Since CIN-V0-001 binds each row's epoch to its producing run, this value is only
  a listing lower bound, not an evidence boundary: an earlier value lists more, and a row produced
  before the epoch advanced carries epoch 1 whatever its time.
- **CIN-V0-004:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3091)
  The verdict SHALL record `listedAt` (when its listing began) and `windowNames` (the row names of
  its AFP-V0-030 window). `accept.sh` SHALL then list only back to one hour before `listedAt`,
  re-run the kill switch and the `MISS` count over those rows followed by the window rows, a
  re-run row replacing its older row as in a whole listing, and answer MET only when that window
  holds 20 rows of the epoch. A verdict without both members, or with either malformed, keeps the
  whole listing back to the verdict's creation; an unreadable page, a listing that neither
  reaches the stop time nor is exhausted, or an unparsable stop time SHALL answer FULL with the
  limit named. AFP-V0-097 semantics are unchanged.
- **CIN-V0-005:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3158;
  the narrowed push reuse first proposed here was rejected by the orchestrator's review decision)
  A `main` push SHALL reuse only a full `ci-tested-tree-record` of a successful `merge_group` run
  of the pushed commit that ran FULL on the identical tree; `ci-reuse-plan` SHALL refuse a
  narrowed queue record for a push, with or without `--base`, and the `ci-narrowed-tree-record`
  job stays pull-request only. A narrowed merge therefore keeps its full `main` push as the
  backstop, and that push still produces forward rows.
- **CIN-V0-006:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3227)
  On a pull request the `doc-gates` job SHALL run `make -s ai-coauthor-trailers-check` with
  `BASE_SHA` set to the pull request's base, which runs `script/check-ai-coauthor-trailers.sh
  "$BASE_SHA" HEAD` and fails with it; a make target keeps the step mirrorable by batch-precheck.
  A pull request run without `BASE_SHA` SHALL fail. A tree without the script, a merge-queue run
  and a push say so and pass; the step carries the AFP-V0-670 REUSE condition.
- **CIN-V0-007:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-2902)
  A `go-product-shard` job SHALL NOT start when `docs-plan` succeeded with mode `DOCS` or `REUSE`,
  and `go-product` SHALL accept a skipped shard result only for that decision. A skip under any
  other decision, a missing decision, or a failed or skipped `docs-plan`, and a failed or
  cancelled shard under DOCS or REUSE, SHALL fail `go-product`.
- **CIN-V0-008:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3329)
  `go-static` SHALL run `go test -count=1 -timeout 30m ./...` and `go vet ./...` in the nested
  module `tools/local-authority` on every run that starts `go-static`, so every change a plan can
  carry to the module, its own files included, is tested. Its external modules are fetched once
  with `GOFLAGS=-mod=readonly`, which fails on a hash missing from or differing with the module's
  `go.sum`; the test and vet commands then run with the job's `GOPROXY=off`. A failure fails
  `go-static` and therefore `go-product`.
- **CIN-V0-860:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3354;
  amended for V1-3480 by the orchestrator's option B decision, 2026-10-31) gate-affected-select
  (`gateselect.Select`, the CI PR tier's selector, and so every `corvint-pr-tests` plan and
  rerun) SHALL run a plan narrowed while it names unknowns only when every unknown is one of:
  (1) to (3) an entry the shared allowlist `gateselect.AllowedUnknown` admits (AFP-V0-982: a
  `LANGUAGE_FRONTIER` that is not a `go:` frontier other than `go:build-constraint-variants` or
  `go:nested-module-frontier`, an `UNOWNED_DIRTY_PATH` or a `NO_SELECTABLE_TEST`, the list the
  shadow uses; before AFP-V0-982 (1) admitted every frontier that is not module level); (4) an `UNINDEXED_SOURCE_PATH` naming a `.go`
  file that no longer exists in the tested tree and whose directory still holds an indexed Go
  package, which AFP-V0-012 rule (a) attributes to that package and its importers. Safety: CI
  narrows only the Go test shards, every other job runs in full; Select attributes every dirty
  path itself, so a Go package that encloses or names a path is selected by rule (b) or (c), and
  a package whose reads no literal bounds is selected on every dirty path by rule (d), which
  covers every Go reader of an unowned path; a unit frontier names what a language model cannot
  see inside units the dirty paths already reach, and those are selected whole; and a unit with no
  selectable test is given no test by a FULL run either. Any other unknown, an
  `UNINDEXED_SOURCE_PATH` naming an existing file or one outside an indexed package, a
  module-level frontier, a reason that is not an upper-case identifier, and an `UNKNOWN` plan that
  names no unknown SHALL answer `FALLBACK` with the reason named. The rule binds `Select` only;
  `SelectStructural` (`corvint test`, the shadow) is unchanged, so the driver's FULL is a superset
  of the shadow's selection (AFP-V0-603). This amendment supersedes V1-3354's acceptance criterion
  that a non-language-frontier `UNKNOWN` never narrows: its `UNOWNED_DIRTY_PATH` and
  `NO_SELECTABLE_TEST` negatives are inverted.
- **CIN-V0-861:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3332;
  effective only after the owner's `ci-control-plane` review of the `.github` change it needs) When
  `collect.sh` writes no row under CIN-V0-001 it SHALL name the run instead: an empty-bodied
  artifact `ci-forward-EPOCH-pr<N>-<head>-e<K>` when every selection document carries one integer
  epoch `K` below `FORWARD_EPOCH`, else `ci-forward-NOROW-pr<N>-<head>-e<FORWARD_EPOCH>-r<why>`,
  `why` `nodoc` (no selection document) or `mixed` (a missing, mixed or non-integer epoch).
  `absence.jq` SHALL count, among the latest 20 (pull request, commit) entries of the epoch formed
  by its rows that take a kill-switch place (not `INFRA`, not `NARROWED`) and its `NOROW` markers,
  a re-run's newer entry replacing the older, the `NOROW` ones, and SHALL answer `TRIPPED` at 3 (the
  `INCOMPLETE` threshold) or when the epoch or threshold is malformed. An `EPOCH` marker, a row or
  marker of another epoch, and an unknown reason SHALL take no place, so an epoch-1 run is never
  counted as absence. `verdict.sh` SHALL trip the AFP-V0-030 kill switch when either the row count
  or `absence.jq` trips. The bar, its thresholds and the `MISS` rule are unchanged.
- **CIN-V0-010:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3748;
  effective only after the owner's `ci-control-plane` review of the `.github` change it needs)
  `CORVINT_PR_TOOL_SOURCE` and `CORVINT_TEST_SHADOW_SOURCE` SHALL name a reviewed `main` commit
  that contains V1-3354. Before `narrow.sh` or `mq-narrow.sh` runs, the narrowing gate of each
  `go-product-shard` SHALL run `.github/forward-rows/pin-lag.sh` read from the `main` commit it
  compares against: the tested merge's first parent for a pull request, the commit of `main` the
  gate reads its scripts from for a merge-queue entry. `pin-lag.sh MAIN TOOL_PIN SHADOW_PIN`
  SHALL exit 0 only when each pin is a commit in MAIN's history whose selection paths are
  identical to MAIN's. The selection paths are the files, other than `*_test.go` files and files
  under a `testdata/` directory, under `internal/gateselect`, `internal/liveverify/affected`,
  `internal/sessiontest`, `tools/corvint-pr-tests`, `tools/gate-affected-select` and
  `.github/cishards` (except `package-costs.json`), plus `cmd/corvint/affected*.go`,
  `cmd/corvint/test*.go`, `.github/forward-rows/narrow.sh` and `.github/forward-rows/narrow.jq`.
  The script SHALL exit 1 when a pin lags, or is not in MAIN's history, with one `::error`
  annotation per such pin naming it and the count and the first 10 selection-path commits since
  it. It SHALL exit 2 on a malformed or unreadable pin or MAIN. On any nonzero exit, or when the
  script cannot be read, the gate SHALL fail without setting `mode`, so the shard runs FULL.
- **CIN-V0-011:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4354;
  effective only after the owner's `ci-control-plane` review of the `.github` change it needs)
  The two pins of CIN-V0-010 SHALL be floors. Before the tools are fetched, a `pin-main` step of
  each `go-product-shard` SHALL resolve the `main` commit the CIN-V0-010 gate compares against:
  the tested merge's first parent when it is in `origin/main`'s history, for a pull request; the
  queue base when it is in that history, otherwise its merge base with `origin/main`, for a
  merge-queue entry. For each pin it SHALL output that commit when the pin is in its history, and
  the pin otherwise, or when no commit resolves. The tool and shadow checkouts, their build checks,
  the shadow record's `toolSource`, the narrowing gate (its `TOOL_SOURCE` and both `pin-lag.sh`
  arguments) and the CIN-V0-002 failure step SHALL read those outputs, falling back to the pins
  when the step did not run, so a selection change merged to `main` no longer needs a repin.
- **CIN-V0-960:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3330)
  The live forward epoch has one source for the tests: `CORVINT_PR_FORWARD_EPOCH` in
  `.github/workflows/ci.yml`. A `tools/ci-forward-row` test that needs the live epoch SHALL read it
  there and derive the epochs around it (the previous and the next), and a test SHALL fail when the
  `FORWARD_EPOCH` and `CORVINT_PR_FORWARD_EPOCH` values in `ci.yml`, `FORWARD_EPOCH` in
  `ci-forward-bar.yml` and `FORWARD_EPOCH` in `ci-control-plane.yml` are missing or disagree. The
  live `FORWARD_EPOCH_SINCE` of `ci-forward-bar.yml` SHALL be checked against the live epoch read the
  same way: equal to the time recorded for an epoch already advanced (epoch 2:
  `2026-10-30T00:00:00Z`), and later than the newest recorded time for a newer epoch, so a
  synchronized advance of every site passes without editing the tests.
  Fixtures that pin a historical epoch transition (the epoch-1 dead-pin rows of CIN-V0-001, and the
  self-contained `bar.jq` fixtures that pass their own epoch) stay literal, since they do not move
  with the live epoch.

- `CIN-V0-2140`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3332)
  The standalone developer alarm `tools/ci-forward-row/absence-alarm.py OWNER/REPO EPOCH`
  SHALL alarm at three eligible completed PR CI runs without a forward row among its latest
  20 eligible runs, ordered by completion within GitHub's bounded sample of 100 latest-created
  completed PR CI runs. Eligibility SHALL NOT depend on a selection document: both FULL and
  NARROWED runs with retained `corvint-ci-outcomes/0` documents qualify. Runs completed less than
  one hour ago wait for asynchronous collection. The alarm SHALL emit JSON, an `::error`
  annotation and exit 1 at the threshold; below it, emit clear and exit 0. Repeated listings of
  the same run and attempt count once. This reporting-liveness alarm is separate from the
  AFP-V0-029 bar and AFP-V0-030 row classifier: a genuine NARROWED row proves reporting is alive
  without qualifying the selection. No repository variable or narrowing authority is written.
- `CIN-V0-2141`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3332)
  The alarm SHALL derive a candidate's epoch from `.github/workflows/ci.yml` at its immutable
  tested merge, independently of selection documents and the collector epoch. Outcome documents
  SHALL bind that merge to the completed CI run ID, attempt and PR head; GitHub SHALL prove the
  merge's two parents, the second equal to the PR head, the first descending from the declared
  PR base and belonging to main. Both PR-head and tested-merge comparisons SHALL exclude
  `.github/` changes (including renames) and change sets of 300 or more files. Only same-repository
  PRs based on main with retained outcomes and a `success` or `failure` conclusion (the only
  conclusions the `forward-rows` collector runs on) qualify; DOCS, REUSE, skipped, fork, queue and
  push runs, and cancelled, timed-out or otherwise concluded runs that retain outcomes, stay
  outside this alarm. A proven different epoch SHALL take no window place and never count
  as absence, including an epoch-1 run with no selection document. Missing or ambiguous workflow
  epoch is uncertainty, not proof of current-epoch absence.
- `CIN-V0-2142`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3332)
  Only unexpired forward artifacts uploaded by same-repository runs on main SHALL discharge
  absence. A matching name SHALL name the candidate's PR, tested commit and epoch, have creation
  time at or after completion, and its downloaded `row.json` SHALL bind profile, kind, class,
  PR, SHA, epoch, run ID, selection run ID and outcomes run ID to that candidate. Every matching
  body SHALL first be validated (profile, kind `pr`, integer PR, epoch and run IDs, and class, PR,
  SHA and epoch equal to its own artifact name); a malformed or self-contradicting body is
  UNKNOWN, not absence. A well-formed row of another run (a delayed row) SHALL NOT discharge absence.
  A row whose body names no tested merge (the classifier's fallback INCOMPLETE row) is named by the
  PR head instead: when no tested-commit row discharges a candidate, an unexpired trusted row naming
  its PR, PR head and epoch, created at or after completion, SHALL report UNKNOWN; it never
  discharges absence and never counts as absent. Because the current forward-row wire has no attempt,
  an eligible current-epoch rerun SHALL report UNKNOWN rather than claim a row covers its attempt.
  Unreadable or malformed evidence, failed GitHub calls (60-second timeout per call), more than
  20 artifact pages of 100 without reaching the oldest sampled completion, a full 100-run sample without 20 eligible runs, a full per-run artifact
  page of 100, or excessive downloads SHALL report UNKNOWN, an error annotation and exit 2.
  Before any transfer, listing metadata SHALL bound each download set to at most 32 artifacts and
  each archive to 4 MiB plus 64 KiB; the transfer SHALL stop past that size. Each archive SHALL be
  read in memory, never extracted, and hold exactly one entry, the expected document, of at most
  4 MiB checked before and during decompression, with no execution of downloaded content. Hosted activation requires a trusted caller to schedule this
  alarm and surface its nonzero exit; this lane SHALL document that wiring without editing
  `.github/`. A local replay does not claim hosted activation or ticket completion.
- `CIN-V0-2300`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4446;
  effective only after the owner's `ci-control-plane` review of the `.github` change it needs)
  When the deciding driver's selection carries a non-empty `Reason` (a fallback, so it admits no
  base), `narrow.sh` SHALL prove parent 0 itself, as AFP-V0-800 does: the tested
  merge SHALL have exactly two parents, the second the pull-request head and the first a full
  commit id descending from the event base. It SHALL then record that parent as `base` and
  `effectiveBase`, run the `.github/` change check and the shadow over it, and keep `eventBase`.
  Any failed step SHALL keep the base empty (the record names `the base, head or target is not a
  full commit id`); a base the driver admitted SHALL never be replaced, and a selection with an
  empty `Reason` whose identity does not match SHALL keep the base empty. The mode is unchanged: a
  driver `Reason` is FULL whatever the base. Run 41714153849 (PR 85) recorded base `""` with a
  readable parent because a selector FALLBACK left the jq filter empty.
- `CIN-V0-2301`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3332;
  effective only after the owner's `ci-control-plane` review) A pull-request run whose retained
  selection documents name no single tested merge of its run and head stops at
  `pr-base-target-unreadable`, before CIN-V0-861's epoch check, so a shadow-selection outage wrote
  no name. `collect.sh` SHALL name it `ci-forward-NOROW-pr<N>-<head>-e<FORWARD_EPOCH>-r<why>`,
  `why` `nodoc` when no selection document was retained and `target` otherwise, under an
  empty-bodied artifact, only when the pull request's own change (event base to head) is proven
  free of `.github/` paths: a pull request that edits its workflow could drop its shadow at will
  and trip the kill switch for every other pull request, and `absence.jq` SHALL count `target` as it counts `nodoc` and `mixed`.
  A marker carries the run's head commit, while a later row of the same head names its tested
  merge, so a pull-request marker is never replaced by that row: the count can only rise.
- `CIN-V0-2302`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3332;
  effective only after the owner's `ci-control-plane` review) `accept.sh` SHALL run `absence.jq`
  over the same names its live kill switch reads (the whole listing, or the rows since the
  verdict's listing followed by its window rows) and SHALL write `verdict=FULL`, reason starting
  `the newest forward-row names show runs that wrote no row (CIN-V0-2302)`, unless the answer is
  `clear` and its `absent` count, plus on the incremental listing the verdict's own
  `rowAbsence.absent` (its window markers are not carried), is below 3. The sum may count a
  marker twice, never too few. An incremental verdict without a numeric `rowAbsence.absent`, and an
  absence count that cannot be computed, are FULL too. Before this, an
  outage since the verdict left the row kill switch clear until the next verdict ran.
- `CIN-V0-2303`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4445;
  effective only after the owner's `ci-control-plane` review) `cache-prune.yml` SHALL run on the
  `workflow_run` completion of the `CodeQL` workflow on `main`, keep its hourly schedule as the
  backstop and its manual dispatch, and SHALL run its job for a `workflow_run` event only when the
  triggering run's head branch is `main`, its head repository is this repository and its path
  starts with `dynamic/github-code-scanning/` (CodeQL default setup). It SHALL stay the only
  workflow that grants `actions: write`. On 2026-11-03 the schedule ran twice in 8 hours while
  CodeQL ran 11 times on `main`.
- `CIN-V0-2304`: (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-2805;
  effective only after the owner's `ci-control-plane` review) For a pull request whose head branch
  starts with `claude/batch-`, the required `doc-gates` job SHALL, unless its reuse decision is
  REUSE, build the checker and its verifier from the pull request's base commit (trusted `main`,
  never the head under check) without a Git stamp, so `dogfoodStaleBinary` never judges it stale,
  check the pull-request head out in a separate worktree (its own checkout is the synthetic merge,
  which no CEM binds) and run that trusted `corvint dogfood batch-check` (DCW-V0-039) there from
  the merge base of the pull request's base and head. Any nonzero exit, and a head or merge base
  that cannot be resolved, SHALL fail the job, so a batch whose commits no sealed CEM binds cannot
  reach the merge queue.

### Merge-queue reuse on a moved or stacked base, integrity-record wait and fast-route liveness (AFP-V0-100 to AFP-V0-106; V1-4232, V1-4233, V1-4234)

Intent (orchestrator lane request, 2026-11-01; V1-4232, V1-4233, V1-4234): over the 47 merge-queue
runs of 2026-10-30 to 2026-11-01, 27 ran the full suite (median 29.8 min against 6.65 min for an
AFP-V0-046 hit): 17 were stacked on an entry ahead of them and 9 were built on a `main` that moved
after the pull-request run, so no tree matched. AFP-V0-812 to AFP-V0-816 reuse only a narrowed
record of a moved base, and nearly every pull-request run is full, so they never applied. Separately,
an entry can start before its pull-request run's `artifact-integrity-record` exists and then reruns
the 12 to 16 min archive gate (V1-4233), and a fast route can stop firing for weeks with nothing
reporting it (the DOCS route never fired 2026-09-29 to 2026-11-01, V1-4234).

Target (set before building, V1-4232): over at least 20 merge-queue runs after the change, the
median wall of moved-base and stacked entries is below 12 min, and no package whose inputs changed
between the record tree and the entry is reused. Non-goals: no reuse without the AFP-V0-813 checks
and a BOUNDED (or language-frontier-only) plan of the main delta; no change to the AFP-V0-046 and
AFP-V0-734 exact reuse; no repin of `CORVINT_PR_TOOL_SOURCE`; no change to `narrow.jq` or `narrow.sh`.

- **AFP-V0-100:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4232;
  amends AFP-V0-812) When `tools/ci-reuse-plan --event merge_group --base Q` decides FULL, names no
  AFP-V0-812 narrowed candidate, and is given `--commits DIR`, the decision SHALL carry
  `delta: {runId, recordJobId, tree, target, base, head, scope: "FULL"}` of the first admitted
  pull-request run whose one AFP-V0-170 `ci-tested-tree-record` line names the entry's head and the
  shard count but another tree. `base` is the first parent of the line's `target`, read only from
  `DIR/<target>.json`, a retained commits API response whose `sha` is `target` and whose `parents`
  are exactly two, the second equal to the head; any other response names no full-record delta. A
  narrowed candidate keeps its AFP-V0-812 shape with no `scope` member and is preferred, and a
  decision made without `--commits` is byte-identical to the previous tool's.
- **AFP-V0-101:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4232;
  amends AFP-V0-046) The docs-plan reuse step SHALL, for each passed record job log it reads, also
  retain the commits API response of the line's target, and SHALL pass `--commits` only to a tool
  whose help text lists `-commits`. A merge-queue entry whose first parent Q is not on `main` (a
  stacked entry) SHALL no longer stop before deciding: the tool is built from
  `git merge-base Q refs/remotes/origin/main`, which must be an ancestor of `origin/main` and of Q,
  never from Q or the entry, and only its `delta` is handed on; such an entry never decides REUSE.
- **AFP-V0-102:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4232;
  amends AFP-V0-813 and AFP-V0-815) `mq-narrow.sh` SHALL accept an `MQ_DELTA` whose `scope` is
  `FULL` under every AFP-V0-813 check except the record-range plan: a full record ran every package
  of tree P, so Rs is the whole queue-range selection S, the entry reuses exactly S \ D and runs
  S ∩ D, and the applied `reuse` record names `recordScope: "FULL"` with `recordSelected` equal to
  `selected`. A `scope` member that is neither absent nor `FULL` SHALL refuse reuse (AFP-V0-816).
- **AFP-V0-103:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4232)
  Before promotion an offline replay of the recent merge-queue runs SHALL predict the hit rate: for
  each missed entry it rebuilds the entry and the record commit C, applies the AFP-V0-813 checks and
  plans C..TM with the BASE-built planner and selector, and predicts the shard wall from
  `.github/cishards/package-costs.json`. The replay and its prediction are recorded in
  `docs/build-log/2026-11-01-cir-w35.md`; the live target above is `NOT_OBSERVED` until the first
  20 queue runs after merge.
- **AFP-V0-104:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4233;
  amends AFP-V0-163) When the `artifact-integrity` reuse step finds no admitted record and a
  pull-request run of the entry's head has not completed and its latest `artifact-integrity` or
  `artifact-integrity-record` job has not concluded, the step SHALL list again every 30 s, at most
  16 more times (8 min), and then run the gate in full. A run that has not completed is admitted
  only when both latest jobs concluded `success` on the head, exactly as a completed run is; a
  completed run that did not conclude `success` is never admitted. The step's timeout SHALL exceed
  the wait bound, the job's timeout (30 min) SHALL hold the wait and the full gate, and a test pins
  the bound.
- **AFP-V0-105:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4234)
  `tools/ci-reuse-plan routes` SHALL read retained run listings and each run's latest jobs listing
  (with steps) and report, for each fast route (DOCS, REUSE, NARROWED, INTEGRITY_REUSE,
  DOC_GATES_REUSE), its eligible runs and hits over the newest `--window` eligible runs, from job and
  step conclusions alone. A route with at least `--min-eligible` eligible runs and no hit is `DEAD`
  and the tool exits 1; fewer eligible runs is `NOT_OBSERVED`, never healthy; a route whose signal
  step is missing from every eligible run is `SIGNAL_MISSING` and also exits 1. It reads no log and
  no artifact.
- **AFP-V0-106:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-4234)
  A scheduled `ci-route-health` workflow SHALL run the AFP-V0-105 check daily over the newest
  `ci.yml` runs with read-only permissions, retain its report, and fail (so the owner is notified)
  on `DEAD` or `SIGNAL_MISSING`.

Failure modes: (1) a package the main-delta plan wrongly omits is reused; the main push of the
commit runs in full and catches it, as for AFP-V0-815. (2) The replay found the delta plan UNKNOWN
(an unowned path or a test-less package) in 18 of 27 misses and the delta selecting
`cmd/corvint` (17.4 min alone) in every applied case, so the shard wall stays above 20 min; the
target is not expected from this change alone. (3) Every consumer of `MQ_DELTA` sits behind the
AFP-V0-731 gate, which refuses while the CIN-V0-010 pin lags. (4) A waiting integrity step delays
the archive gate by at most 8 min when the pull-request job fails or is cancelled. (5) A renamed
signal step makes its route `SIGNAL_MISSING`, which fails loud rather than reporting healthy.

Rollback: remove `--commits` from the docs-plan step (the tool then names no full-record delta),
restore the stacked-entry early exit, remove the `FULL` branch of `reuse_` in `mq-narrow.sh`, restore
the single listing in the integrity step, and delete `.github/workflows/ci-route-health.yml` and the
`routes` subcommand.

## Non-goals

merge-queue tree reuse of `go-static`, `go-interop` and `cce-live` (V1-2902 remainder: it needs
a pull-request record job and would change the AFP-V0-670 and AFP-V0-080 gated-job structure);
building the shadow planner once per run (V1-2902 remainder: it touches the pin steps PR #11
changes and needs a trusted artifact handoff); reusing a narrowed queue record for a `main` push
(rejected, CIN-V0-005); planner selection of a `tools/local-authority` unit, which belongs to the
affected planner (V1-0872), so CIN-V0-008 runs the module whenever `go-static` runs; changing the pin values (except CIN-V0-010's repin); changing the AFP-V0-816
`BOUNDED` scope check of `mq-narrow.sh`; adding `infra-reclassified.txt` keys (AFP-V0-463).

## Failure modes

(1) A `FORWARD_EPOCH_SINCE` later than the first epoch-2 row would hide rows from the verdict;
the value precedes the batch carrying epoch 2, and it moves only with `FORWARD_EPOCH`. (2) A row
created during the verdict's own listing could be missed by the incremental acceptance; the
hour of overlap before `listedAt` covers it. (3) A run with no retained selection document
(no shadow, a dead shadow pin, or a failed artifact download) writes no row under CIN-V0-001, so
a sustained shadow outage no longer counts `INCOMPLETE` toward the kill switch; the bar still
needs 8 complete epoch-2 rows, but an already-established verdict can keep narrowing enabled
without fresh rows. CIN-V0-2140..2142 provide a separate operator alarm independent of selection
documents; hosted scheduling remains pending. CIN-V0-861 makes such a run a
`NOROW` marker that the kill switch counts; a collector that does not run at all (the
`ci-control-plane` workflow disabled or failing before `collect.sh`) still leaves no name, and only
the verdict's own staleness shows it. A queue row takes the epoch
of its pull-request selection run. (4) A dead pin turns every shard red until repinned
(CIN-V0-002); that is the intended loud failure. (5) An old `ci-reuse-plan` refusing `--base` on a
push fails the reuse step, so `docs-plan` fails and every shard runs FULL. (6) The CIN-V0-008
fetch needs the module proxy; an unreachable proxy or a `go.sum` mismatch fails `go-static`
loudly rather than skipping the module. (7) CIN-V0-860: rule (d) is only as complete as the static index's root-locator model
(AFP-V0-012 failure modes): a Go test that reads an unowned path by a computed name without being
an unbounded reader would be missed, as it already could be under BOUNDED plans. A non-Go reader of
an unowned path is not a Go shard and runs in its own full job. (8) CIN-V0-010: every selection
change merged to `main` made every Go shard FULL until both pins were repinned past it; CIN-V0-011
now builds the tools from the compared commit, so only a run whose compared commit predates a pin or
is off `main` builds the pin and can still lag. That costs latency, not soundness. The path list is declared, not derived: a selection rule moved outside it
would go unnoticed until the list follows it. A `main` change to `package-costs.json` moves shard
costs only, so it does not count as lag. (9) CIN-V0-2304 runs a checker built from the
pull request's base, so a batch cannot weaken its own check by editing `cmd/corvint` or
`script/dogfood-batch-check.sh`; a change to the check itself takes effect only after it merges, and
only an edit to the protected `.github/` paths can alter the step. A non-batch pull request is not
checked.
(10) CIN-V0-2303's trigger depends on GitHub delivering `workflow_run` for the dynamic CodeQL
workflow; that is NOT_OBSERVED until the change runs on `main`, and a run it never delivers is
caught only by the schedule. (11) CIN-V0-2301 and CIN-V0-2302 count only runs the collector saw: a
collector that never runs names nothing, as in failure mode (3).

## Acceptance evidence and traceability

the tests in the traceability rows; the V1-3091 request counts in build log
`2026-10-30-v1-3091-incremental-forward-listing`; the CIN-V0-001 decision in build log
`2026-10-30-v1-3314-forward-epoch-2`; the CIN-V0-005 rejection in build log
`2026-10-30-v1-3158-v1-2902-push-reuse-and-shard-skip`; the CIN-V0-008 local runs in build log
`2026-10-30-v1-3329-local-authority-ci-runner`, whose first CI run ID follows the push.

| Requirement | Implementation | Evidence |
|---|---|---|
| CIN-V0-001 | `CORVINT_PR_FORWARD_EPOCH`, the shadow step's `forwardEpoch` argument and the narrowing-verdict `FORWARD_EPOCH` in `ci.yml`; `FORWARD_EPOCH` and the artifact name in `ci-forward-bar.yml`; `ci-control-plane.yml`; `selection.jq`; the epoch check of `collect.sh` | `TestCINV0001EpochTwoLeavesTheDeadPinRowsBehind`; the "producer epoch names the row" case of `TestAFPV0047CollectorNamesRows`; `TestAFPV0047SelectionDocument` |
| CIN-V0-002 | the `pin-tool-*` and `pin-shadow-*` step ids and the "Fail loudly on an unreachable or unbuildable pin" step of `ci.yml` | `TestCINV0002UnreachablePinFailsTheShardLoudly` |
| CIN-V0-003 | `list_names` stop time in `names.sh`; `since`, `covered` and `reachedEpochSince` in `verdict.sh`; `FORWARD_EPOCH_SINCE` in `ci-forward-bar.yml` | `TestCINV0003VerdictListsBackToTheEpochSince` |
| CIN-V0-004 | `listedAt` and `windowNames` in `verdict.sh` and `bar.jq`; the incremental branch of `accept.sh` | the `CIN-V0-004` cases of `TestAFPV0097VerdictAcceptance` (31 reads to 2 on the replay; malformed members fall back) |
| CIN-V0-005 | `Decide` in `tools/ci-reuse-plan/main.go`; the pull-request-only `ci-narrowed-tree-record` job of `ci.yml` | `TestCINV0005PushNeverReusesANarrowedQueueRecord`; `TestAFPV0734NarrowedRecordJob` |
| CIN-V0-006 | the "Refuse AI co-author trailers" step of `doc-gates` in `ci.yml`; the `ai-coauthor-trailers-check` target in `Makefile` | `TestCINV0006DocGatesRefusesAICoauthorTrailersOnPullRequests`; `TestAFPV0670GatesWireExactTreeReuse` |
| CIN-V0-007 | the `go-product-shard` job condition and the `go-product` shard check of `ci.yml` | `TestCINV0007ShardsSkipAtJobLevelOnlyForADocsOrReuseDecision` |
| CIN-V0-008 | the "Nested module tools/local-authority tests and vet" step of `go-static` in `ci.yml` | `TestCINV0008LocalAuthorityModuleTestsRunInGoStatic`; `TestAFPV0380InteropTestsRunOnceInGoStatic` |
| CIN-V0-860 | `unknownNarrowing` and `deletedGoSource` and their calls in `selectPackages` in `internal/gateselect/select.go` | `TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860` in `internal/gateselect/unknown_cinv0860_test.go` (language frontiers, an unowned path alone or beside frontiers, no selectable test and a deleted source of an existing package run, the deleted package and its importer selected; a module-level frontier, an unindexed existing source, a deleted source of a removed package, a malformed or empty reason, an UNKNOWN plan naming none and another reason fall back; SelectStructural unchanged); the "unknown plan keeps the fallback" case of `TestSelectPackagesTaskStateOnlyDiffSelectsNothing`; the replay in build log `docs/build-log/2026-10-31-ci-select-w1.md` |
| CIN-V0-010 | `CORVINT_PR_TOOL_SOURCE`, `CORVINT_TEST_SHADOW_SOURCE` and the `pin-lag.sh` call in the narrowing gate of `ci.yml`; `.github/forward-rows/pin-lag.sh` | `TestCINV0010PinLagRefusesNarrowing` in `tools/ci-forward-row/pinlag_test.go` (a pin at MAIN passes; a selection change, a `cmd/corvint/test*.go` change and a `narrow.jq` change after a pin fail; a test, testdata, `package-costs.json` or unrelated change does not; a pin outside MAIN's history fails; malformed and unknown pins exit 2; the committed pins are one equal full commit id each; the gate calls the script from the compared `main` commit in both branches before `narrow.sh` and `mq-narrow.sh`) |
| CIN-V0-011 | the `pin-main` step of `ci.yml` and every `steps.pin-main.outputs` binding | `TestCINV0011ToolsBuildFromComparedMain` in `tools/ci-forward-row/pinlag_test.go` (the step's own script, run on a fixture repository: a pull request on a moved `main` and a merge-queue entry on `main` or behind an unmerged entry build `main`; a first parent off `main`, a `main` older than the pin, or a queue base with no `main` ancestor keeps the pin; every fetch, build, record, gate and failure binding reads the outputs) |
| CIN-V0-960 | `forwardEpoch`, `forwardEpochSites`, `forwardEpochSinceProblem`, `epochSinces` (`tools/ci-forward-row/epoch_test.go`) | `TestCINV0960WorkflowsAgreeOnTheForwardEpoch`, `TestCINV0960SynchronizedAdvancePasses`, `TestCINV0003VerdictListsBackToTheEpochSince`; `TestCINV0861RowAbsenceTripsTheKillSwitch` and `TestCINV0001EpochTwoLeavesTheDeadPinRowsBehind` read the live epoch from `ci.yml` |
| CIN-V0-861 | `.github/forward-rows/absence.jq` (moved from `tools/ci-forward-row/`); `marker` and the CIN-V0-001 exit of `.github/forward-rows/collect.sh` (the existing "Retain the forward row" step of `ci-control-plane.yml` uploads the empty body under the marker name); the `absence` join and `rowAbsence` of `.github/forward-rows/verdict.sh` | the CIN-V0-001 exclusions of `TestAFPV0047CollectorNamesRows` (each names its exact EPOCH or NOROW marker); the three-, two-marker and missing-`absence.jq` cases of `TestAFPV0097VerdictWrittenOnMain`; `TestCINV0861RowAbsenceTripsTheKillSwitch` in `tools/ci-forward-row/absence_test.go` (three NOROW markers trip, with rows beside too, and at a lower threshold; two do not; epoch-1 markers and rows, another epoch's marker, INFRA and NARROWED rows and an unknown reason take no place; a re-run's row replaces its NOROW; markers older than 20 entries do not trip; a malformed threshold trips) |
| CIN-V0-2300 | the parent-0 proof after the driver base in `.github/forward-rows/narrow.sh` | the fallback cases of `TestAFPV0098SupersetGuardSidecar` (a driver fallback keeps the proven parent, also for a stale event base, with the `.github/` check and shadow read; head mismatch, event base not an ancestor, an octopus merge and unreadable parents keep `""`; a selection without a `Reason` whose event base mismatches keeps `""` and stays FULL; both keep cases fail on the base script) |
| CIN-V0-2301 | the `pr-base-target-unreadable` exit of `.github/forward-rows/collect.sh`; the `target` reason of `absence.jq` | the mixed-targets (`rtarget`) and no-document (`rnodoc`) cases of `TestAFPV0047CollectorNamesRows`; a no-document run whose pull request touches `.github/` is not named |
| CIN-V0-2302 | `absent`, its two calls and the carried `rowAbsence.absent` in `.github/forward-rows/accept.sh` | the "runs without a row" cases of `TestAFPV0097VerdictAcceptance`, whole and incremental (CIN-V0-004) listings: three markers are FULL, two and three of another epoch are MET; two carried plus one since is FULL; an incremental verdict without a count is FULL |
| CIN-V0-2303 | the `on:` block and the job `if:` of `.github/workflows/cache-prune.yml` | `TestCINV02303CachePruneFollowsCodeQL` (trigger, backstop, guard clauses; no other workflow grants `actions: write`, with the matcher checked on a planted grant); `TestAFPV0097CachePruneKeepsNewestOverlay` |
| CIN-V0-2304 | the "Refuse a batch pull request whose commits no sealed CEM binds" step of the `doc-gates` job in `.github/workflows/ci.yml` | `TestCINV02304BatchPullRequestNeedsSealedEvidence` (the step run on a fixture repository with a stand-in `cmd/corvint`: the base-built trusted checker runs, never the head's replacement, from the head and the merge base, built without a Git stamp; a refusing check and an unresolvable head fail; skipped where no POSIX sh exists); live replay on PR 87's range (PASS, 14 s) and with one added unsealed commit (`FAIL unbound-commits count=1`), build log `docs/build-log/2026-11-03-mqfix.md` |
| CIN-V0-2140 | `tools/ci-forward-row/absence-alarm.py` | `TestCINV02140AbsenceAlarmReplay` runs the Python GitHub-boundary replay: absent selection, active narrowing, threshold, latest-20 window, duplicate run, grace |
| CIN-V0-2141 | `candidate` in `tools/ci-forward-row/absence-alarm.py` | `TestCINV02140AbsenceAlarmReplay`: epoch-1 absent selection exclusion, workflow trust, intentional exclusions, uncollected conclusions |
| CIN-V0-2142 | `trusted_rows`, `gh`, `download`, `document`, `row_binds`, `evaluate`, `main` in `tools/ci-forward-row/absence-alarm.py` | `TestCINV02140AbsenceAlarmReplay`: row provenance/body/run binding, malformed bodies and PR-head-named fallback rows UNKNOWN, stale rows, uncertain reruns, failed reads, bounded pagination, pre-transfer count/size bounds, bounded archive entries and transfer |

## Rollback

CIN-V0-2140..2142: stop invoking the standalone alarm and remove its tool and replay. It writes
no persisted state. Hosted wiring is pending; no existing workflow rollback is required. Its
100-run candidate sample cannot claim globally latest completion order (an older-created run
may complete or rerun later), and it cannot detect an outage that also removes every outcome
artifact; those remain explicit coverage limits. A trusted scheduled caller needs `actions: read`,
`contents: read`, Python 3 and authenticated `gh`, and must fail or notify on both exit 1 and exit 2.


R1: revert the lane's `.github` commits and their test commits; the epoch returns to 1 only by an
owner decision, since epoch-1 rows hold the dead-pin `INCOMPLETE` rows. No stored state depends on
these requirements; verdict artifacts without `listedAt` keep the whole listing. CIN-V0-860: remove the `unknownNarrowing` call in `selectPackages`; Select then
runs every RUNNABLE plan without a module-level frontier narrowed, as before. CIN-V0-861: drop the `absence.jq`
call in `verdict.sh`; the markers are then ignored names. CIN-V0-010: drop the two `pin-lag.sh` calls
in the narrowing gate; the pins then may lag again silently. CIN-V0-011: drop the `pin-main` step;
every `steps.pin-main.outputs` reference then falls back to the committed pins. Repinning back restores the older selection rules. CIN-V0-2300: drop the
parent-0 proof in `narrow.sh`; a driver fallback again records base `""`, still FULL.
CIN-V0-2301: drop the marker at `pr-base-target-unreadable`. CIN-V0-2302: drop the two `absent`
calls in `accept.sh`. CIN-V0-2303: drop the `workflow_run` trigger and the job guard's
`workflow_run` clause; the schedule alone remains. CIN-V0-2304: drop the batch-check step of
`doc-gates`; the DCW-V0-348 pre-push hook remains the only check.
