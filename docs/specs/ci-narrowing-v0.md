# CI Narrowing V0

Owner: Russell Lewis
Date: 2026-10-30
Requirement prefix: `CIN-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3309, V1-3091, V1-3158, V1-3227, V1-2902, V1-3329); CIN-V0-001 decided by the orchestrator under owner delegation (V1-3314), applying the AFP-V0-030 re-arm rule; effective only after the owner's `ci-control-plane` review of the `.github` change
Delivery status: experimental
Authoritative inputs: lane cireuse charter (2026-11-05; follow-up to V1-4727); V1-4625 lane charter (2026-11-04); native tickets V1-3314, V1-3309 (AC3), V1-3091, V1-3158, V1-3227,
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

- `CIN-V0-7500`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2864) The tier plan MUST retain every distinct FULL-forcing path reason rather than capping the list at twenty; the per-reason display limit MAY remain 300 characters.
- `CIN-V0-7501`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4119) The shared PR documentation gates MUST run the exact per-job permission/exception check before merge; each nondefault grant MUST name its exact permissions and purpose in the exception table.
- `CIN-V0-7502`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4123) PR, merge-queue and main CI MUST execute the declared TypeScript family through `make host-adapter-test`; a failure MUST fail the required `go-product` check.
- `CIN-V0-7503`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3333) `go-static` MUST restore the exact `go.mod`/`go.sum`-keyed local-authority module cache, refuse a cache miss, verify it with `go mod verify` and run the nested module tests and vet with GOPROXY/GOSUMDB off and readonly module directives. An explicit preceding preparation job MAY fetch checksum-verified dependencies and populate that cache; compiler steps MUST NOT fetch from a proxy.
- `CIN-V0-7504`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4436) CI MUST compute the shadow dry-run plan for the event range and require `web-flows` for a FULL plan, an Incomplete dry-run (exit 3), or still-required checks naming a web-flows binary consumer. That job MUST install the lockfile dependencies with lifecycle scripts disabled and the matching Playwright Chromium, then run `script/web-flows-gate`. The selector binary and confine wrapper MUST build from the trusted checkout and inspect a separate tested checkout. Consumer and read-scope declarations (`.corvint/test-binary-consumers.json` and `.corvint/test-read-scopes.json`) are tested-tree data, reviewed as code; a FULL plan MUST always require the consumer job. Any other nonzero planning exit or required consumer failure MUST fail `go-product`; an unreached consumer in a complete non-FULL plan MUST leave the job skipped.
- `CIN-V0-7505`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4574) An edit under `.github/` MUST select `tools/ci-forward-row` through its declared read scope and its workflow-executing tests in batch verification.
- `CIN-V0-7506`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4154) A new binary source or command-tree entry MUST select `conformance/cli-errors-v0` catalog-equality tests in affected and in batch verification, including when source and catalog changes meet only in the batch.

- **CIN-V0-7600:** (accepted by the owner, lane cireuse charter 2026-11-05; amends AFP-V0-024,
  AFP-V0-046, AFP-V0-170 and AFP-V0-734) Tested-tree REUSE admission SHALL depend on the
  record job and the required results it attests, regardless of the workflow run's overall
  status or conclusion. A running run, or a run failed only by advisory `go-macos`, MAY be
  reused once its one latest record job completed successfully. The full record requires
  exactly one successful completed `go-product-shard (i)` for every index `0..N-1`; the
  narrowed record additionally requires successful completed `docs-plan`, `narrowing-verdict`
  and `go-product`, matching the record job's success predicates. The full record's conditional
  docs-plan/narrowing checks do not attest their success; unrelated checks still run in the
  deciding workflow. Every record and required job SHALL have a positive unique job ID and
  belong to the admitted run and head. Missing, duplicate, renamed, extra or noncanonical shard
  identities, ID collisions, unsuccessful (including failed, cancelled, skipped or unfinished)
  required or record jobs, unreadable listings/logs and invalid records SHALL refuse REUSE
  with a precise reason in the existing decision `reason` field. Run repository, workflow,
  event and head admission, exact record tree/head/shard count, queue target and narrowed base
  binding remain mandatory. An unfinished record never vouches for a tree; rejected required
  evidence never produces a delta candidate. Advisory status supplies no authority. No JSON
  shape, record format, exit code, stderr or golden changes are authorized. The workflow's
  existing `status=success` listing filter must be removed separately before these candidate
  runs can reach the tool; this lane changes no workflow. Rollback: restore whole-run success
  admission, retaining FULL fallback. Acceptance: the CIN-V0-7600 tests and retained PR #99
  replay in build log `2026-11-05-ci-reuse-record-admission`.
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

- `CIN-V0-4625`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4625; amended under the owner's standing auto-accept (2026-10-11), V1-4732)
  Every pull request SHALL start the independent `.github/workflows/go-macos.yml` workflow,
  retaining the advisory job and check name `go-macos`. A routine-change DEFER starts no macOS runner (CIN-V0-4754). It SHALL use at most four GitHub-hosted
  `macos-15` matrix jobs with the pinned production Go toolchain and nightly macOS host provisioning.
  The `CI` workflow SHALL contain no Darwin job or dependency on this workflow; its completion and
  the `ci-control-plane` workflow-run status SHALL not wait for advisory Darwin execution.
  The independent workflow SHALL recompute the trusted documentation classifier/verifier and
  protected-base narrowing verdict, without waiting for CI or consuming its Linux selection.
  A verified `docs-plan` DOCS decision
  MAY skip its tests; a Linux REUSE decision or `ci:batched` label SHALL NOT. From native
  `go list` metadata for the root module and CI's existing nested modules `tools/local-authority`
  and `interop/cem01-go`, the job SHALL select packages containing Darwin filename/build constraints,
  `GOOS` comparisons or switch cases with `"darwin"` in code or tests, or `/private/var` or
  `/var/folders` path-alias witnesses. Build constraints SHALL conservatively include indirect Darwin terms (`unix`,
  negated OS tags such as `!linux` and `!windows`, and BSD forms); selecting every build
  constraint is permitted. Source and test files, including ignored platform files, SHALL be
  rescanned on every run; no frozen package allowlist governs the selection. The retained inventory SHALL name each
  package and the file/reason for inclusion. Tests SHALL run whole packages, without the full-suite
  gate or a race run. A deterministic longest-cost-first partition SHALL assign every inventory
  package exactly once across four shards using `.github/cishards/package-costs.json`, before each
  shard filters its assigned packages by its admitted selection. Ownership SHALL be independent
  of narrowing so mixed FULL/NARROWED matrix legs cannot omit affected packages. An unpriced
  package SHALL use the median estimate. Invalid or empty cost content SHALL fall back to complete
  round-robin placement; a cost-file I/O failure SHALL fail the shard before execution, so a
  runner-local read failure cannot silently change ownership. Each shard SHALL run one `go test`
  invocation per module over its package list, with
  three build/package workers (`-p 3`) and two test workers (`-parallel 2`), retaining all module
  failures rather than stopping after the first. A shard SHALL disable the `heavy.sh` host-load
  queue (`HEAVY_LOAD_WAIT=0`), because a hosted runner is dedicated to the job, and a failed
  module SHALL print each failed package/test with its last output lines rather than a raw
  JSON tail. Each macOS shard SHALL have a 40-minute timeout,
  with an under-30-minute FULL-selection wall-time target subject to hosted measurement. Shards
  SHALL retain separate `macos-selection-shard-N` execution artifacts even after failure; shard zero
  SHALL remain the single writer of the unchanged `macos-selection` inventory/narrowing artifact.
  Native execution time and Darwin-only outcomes remain NOT_OBSERVED until hosted
  execution; Linux cost history is an estimate, never a native measurement.
  The workflow is advisory: `go-product` SHALL NOT depend on it or inspect its result. Its
  matrix and summary job SHALL set job-level `continue-on-error: true`; the stable `go-macos`
  summary SHALL inspect all four retained shard outcomes and expose any failure or missing result.
  Only `.github/workflows/ci.yml` remains admissible to Linux merge-queue REUSE; an advisory Darwin
  run SHALL never supply that proof (V1-4727, V1-4732).
  `go-macos` MAY intersect its root-module inventory with a native narrowed selection only after
  the existing `narrowing-verdict` reports MET and the existing trusted `narrow.sh` guards admit
  it: protected main-parent tools built for Darwin, native driver and shadow, matching event/base/
  head/target, merge-queue rule, no `.github/` change, complete selection and shadow-superset proof.
  Linux package artifacts SHALL NOT drive this intersection. An absent, failed, malformed or
  mismatched native record SHALL run the entire Darwin inventory, never silently omit packages;
  nested modules SHALL always retain their full Darwin inventory because the root planner does
  not own them. Its artifact SHALL use a distinct name outside the Linux shard record/coverage
  patterns,
  so it cannot change their denominator. Activation remains subject to `ci-control-plane` admin
  review; no branch-protection change or new required status context is necessary.
- `CIN-V0-4626`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4636; deferred by the owner's fix-round charter, 2026-11-04; location clarified V1-4732) PLANNED/deferred: make the independent `go-macos`
  check required only after V1-4413, V1-4551 and V1-4553 fix the existing native
  failures and one hosted FULL run is green. Until then CIN-V0-4625 remains advisory.
  The deferred integration SHALL require PR success even for a batched label, using the independent
  workflow status rather than a dangling `ci.yml` needs edge. Non-PR CI SHALL not require a Darwin
  job that exists only in the independent pull-request workflow. Nightly `macos-15` full-gate run 41849536472
  at base `6c33df33bc5670131482f5263b410cb980580657` fails in selected packages
  (`internal/contextindex`, `internal/tasks/cli`, `internal/tasks/store`, `internal/testsupport`,
  `interop/cem01-go`); requiring this result now would block FULL-fallback pull requests.

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
  step conclusions alone; a run id listed more than once counts once. A route with at least
  `--min-eligible` eligible runs and no hit is `DEAD`
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

- **CIN-V0-2320:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3489;
  effective only after the owner's review of the `.github` change it needs) The CI workflow SHALL
  run, on every `pull_request` and `merge_group` event and on no other, a `shell-test-selected` job
  that checks out full history and runs `make -s shell-test-affected BASE=$(git merge-base HEAD
  BASE_SHA)`, `BASE_SHA` the pull request's base commit or the merge-queue entry's `base_sha`, and
  SHALL fail when neither is set. The steps it runs are the AFP-V0-2401 selection: the script
  self-tests (`make shell-test` steps) a change can affect, every step when the plan cannot narrow
  them. Before it, no PR job ran a shell-test step, so `no-python-runtime-dependency-test` was red
  on main (build-log entry `2026-11-03-selbugs-selection-correctness`). The required `go-product`
  check SHALL depend on `shell-test-selected` and require its successful result before every
  early return, including a batched pull request, DOCS or REUSE; failure, cancellation, missing
  result or a skipped job on a pull request or merge-group event SHALL fail the required check.
  A push SHALL explicitly admit the shell job's intentional `skipped` result; a failed or
  cancelled shell job SHALL never pass through that exception.

- `CIN-V0-4750`: (owner-approved 2026-11-06, "fast PRs; full targeted/nightly/release checks"; V1-4750)
  A routine-change classifier, `.github/ci-cadence/classify.sh`, SHALL decide DEFER or FULL for the
  whole change from `git diff --raw -z -M` of the base against the tested commit. DEFER SHALL need
  every changed path to be in one of two classes. DOCS is `docs/build-log/<name>.md`, top level
  only. TESTS is an ordinary `*_test.go` under `cmd/`, `internal/` or `tools/`, excluding
  `testdata`, paths with characters outside `A-Za-z0-9_./-`, the archive, selection, conformance
  and release subtrees (`tools/selection-baseline`, `tools/gate-affected-select`,
  `tools/docs-ci-plan`, `tools/ci-reuse-plan`, `tools/ci-forward-row`, `internal/releasecandidate`,
  `internal/versionline`, `internal/companionrelease`, `internal/testplatform`,
  `internal/testconfine`) and any GOOS/GOARCH file-name suffix, and whose content has no build
  constraint, `go:embed`, `go:linkname`, `go:generate`, cgo import, `GOOS`, `"darwin"`,
  `/private/var`, `/var/folders` or `TestMain`, that does not start with a UTF-8 byte-order mark,
  and that is not in or under a directory holding a non-test Go file with a `go:embed` directive
  (an embed pattern there could ship it in the binary). Only an A or M entry of a 100644 blob qualifies;
  a rename, copy, delete, type change or mode change SHALL be FULL, as SHALL an empty diff and any
  unclassified path.
- `CIN-V0-4751`: (owner-approved 2026-11-06, "fast PRs; full targeted/nightly/release checks"; V1-4750)
  The classifier SHALL print exactly eight `key=value` lines, in order: `profile=corvint-ci-cadence/0`,
  `cadence` (DEFER or FULL), `class` (NONE, DOCS, TESTS or DOCS+TESTS), `base`, `head`, `target`,
  `helper=sha256:<digest of the script>` and `reason`. A DEFER reason SHALL be `routine-change`.
  A FULL reason SHALL name its cause (`forced-full`, `invalid-oid`, `root-missing`,
  `commit-missing`, `merge-topology-mismatch`, `queue-head-mismatch`, `queue-base-not-ancestor`,
  `unsupported-event`, `tempdir-unavailable`, `diff-unreadable`, `embed-scan-failed`, `root-embed`, `empty-diff`, `diff-malformed`,
  `unexpected-mode`, `mode-change`, `rename-or-copy`, `delete`, `status-*`, `mode-*` or
  `unclassified:<path>`). A usage error exits 2; every other outcome exits zero.
- `CIN-V0-4752`: (owner-approved 2026-11-06, "fast PRs; full targeted/nightly/release checks"; V1-4750)
  Only the classifier copy of the trusted event base (the pull request's base commit or the
  queue's base) SHALL run, from a sparse checkout of `.github/ci-cadence`; the tested tree SHALL be
  read through git only and never executed. For a pull request the tested commit SHALL be the
  two-parent merge of exactly the base and head. For the merge queue the head SHALL equal the
  tested commit and the base SHALL be its ancestor, and the whole base..head range SHALL be
  classified, never a constituent pull request's decision. Any error, refusal or missing input
  SHALL yield FULL. `.github/ci-cadence/run.sh` SHALL validate the receipt (the fixed schema, the
  echoed base, head and tested commit, a DEFER class and a sha256 helper) and write `cadence`
  once, DEFER only on a valid DEFER receipt; it SHALL always exit zero. The repository variable
  `CI_CADENCE_FORCE=FULL` SHALL force FULL and is the rollback. A pull request SHALL fetch only its
  merge commit and that commit's parents (depth 2); a queue entry fetches full history.
- `CIN-V0-4753`: (owner-approved 2026-11-06, "fast PRs; full targeted/nightly/release checks"; V1-4750)
  `ci.yml` SHALL run a `cadence-plan` job on a pull request or merge-group event; a DEFER counts only
  when that job succeeded with output DEFER (a verified DEFER). A verified DEFER of a pull request or
  queue entry SHALL make `doc-gates` set `CORVINT_CI_CADENCE=DEFER` as step environment, so
  `make selection-baseline-check` prints NOT_RUN; a main push, local runs and `batch-precheck` leave
  it FULL. A verified pull-request DEFER SHALL skip `artifact-integrity`; a queue entry SHALL still
  run it, so the main push can reuse the queue's static verdicts. Under a pull-request DEFER
  `doc-gates-record` and `artifact-integrity-record` SHALL record nothing, so the queue makes its own
  `doc-gates` verdict.
- `CIN-V0-4754`: (owner-approved 2026-11-06, "fast PRs; full targeted/nightly/release checks"; V1-4750)
  `go-macos.yml` SHALL run its own `cadence-plan` job. A DEFER SHALL start no macOS runner
  (`docs-plan` and `go-macos-shard` are skipped only on a verified DEFER); the stable `go-macos` aggregator SHALL accept
  only a verified DEFER (a successful `cadence-plan` job with output DEFER) and otherwise check
  all four shards as before, so a skipped or failed plan runs every shard.
- `CIN-V0-4755`: (owner-approved 2026-11-06, "fast PRs; full targeted/nightly/release checks"; V1-4750)
  The nightly `release-gates.yml` SHALL include a `selection-baseline` entry running
  `make selection-baseline-check` in full. The native archive gate is already part of `make gate`,
  which the release gates run.
- `CIN-V0-4756`: (owner-approved 2026-11-06, "fast PRs; full targeted/nightly/release checks"; V1-4750)
  A first rollout whose event base lacks the classifier SHALL be FULL. Rollback SHALL be setting
  the repository variable `CI_CADENCE_FORCE=FULL` or reverting the change. Product features,
  evidence formats and persisted state are unchanged.

## Non-goals

merge-queue tree reuse of `go-static`, `go-interop` and `cce-live` (V1-2902 remainder: it needs
a pull-request record job and would change the AFP-V0-670 and AFP-V0-080 gated-job structure);
building the shadow planner once per run (V1-2902 remainder: it touches the pin steps PR #11
changes and needs a trusted artifact handoff); reusing a narrowed queue record for a `main` push
(rejected, CIN-V0-005); planner selection of a `tools/local-authority` unit, which belongs to the
affected planner (V1-0872), so CIN-V0-008 runs the module whenever `go-static` runs; changing the pin values (except CIN-V0-010's repin); changing the AFP-V0-816
`BOUNDED` scope check of `mq-narrow.sh`; adding `infra-reclassified.txt` keys (AFP-V0-463).

CIN-V0-4750..4756 do not defer any change beyond DOCS and TESTS, do not change the required check names, the
merge-queue `go-product` aggregate or the `make gate` contents, and do not weaken nightly or release
coverage; they add no product feature and no persisted state.

## Darwin PR job boundaries and maintenance

The smaller baseline is the nightly full macOS gate; it remains unchanged and catches
non-Darwin-specific regressions outside this inventory. This PR leg is not a full macOS
qualification, race run, release gate or language-provider live qualification. One runner avoids
adding another shard matrix to the roughly four-run Actions capacity. Native narrowing reuses
AFP-V0-098 guards rather than granting advisory affected output authority to skip tests.
The Linux forward bar is reused as the existing enablement control, not claimed as a native
cost or miss-rate measurement. Unqualified/missing narrowing inputs retain the full inventory.

The selector is lexical and conservative: comments and negated build tags may overselect; computed
OS checks, imported aliases of `GOOS`, and path-sensitive behavior without a path witness can be
missed. Add a Darwin/platform test or an explicit `/private/var` regression witness when such a
failure is found. The nightly full gate remains the backstop. New independent nested Go modules
need explicit job provisioning; fixture modules under testdata are not product packages. Missing
host tools, metadata errors, native test failures and timeout fail the advisory job
without blocking `go-product`; required integration is deferred to V1-4636 (CIN-V0-4626). The
180-minute ceiling is a hang/resource bound, not a latency promise; full fallback is expensive.
Normal narrowed changes are estimated at 5–15 runner minutes (setup/planning and selected packages),
while full fallback is conservatively budgeted up to 180; actual macOS timings are NOT_OBSERVED.

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

(12) CIN-V0-4750..4756: a classifier refusal, a malformed receipt, a skipped `cadence-plan` job or an
event base without the classifier runs every campaign (FULL), so a fault costs latency only. A
DEFER pull request skips the archive gate and the selection-baseline replay until the nightly
release gates; a regression those campaigns would have caught is found there, not on the pull
request. The TESTS content guard is lexical; a test file outside it that still alters a deferred
campaign's result would be missed until the nightly run.

## Acceptance evidence and traceability

the tests in the traceability rows; the V1-3091 request counts in build log
`2026-10-30-v1-3091-incremental-forward-listing`; the CIN-V0-001 decision in build log
`2026-10-30-v1-3314-forward-epoch-2`; the CIN-V0-005 rejection in build log
`2026-10-30-v1-3158-v1-2902-push-reuse-and-shard-skip`; the CIN-V0-008 local runs in build log
`2026-10-30-v1-3329-local-authority-ci-runner`, whose first CI run ID follows the push.

| Requirement | Implementation | Evidence |
|---|---|---|
| CIN-V0-7600 | `admits`, `recordJobLine`, `successfulJob`, `recorded`, `recordedNarrowed` in `tools/ci-reuse-plan/main.go` | `TestCINV07600RecordAdmitsBeforeAdvisoryCompletion`, `TestCINV07600RecordAdmissionFailsClosed`, `TestCINV07600NarrowedRecordRequiresItsAggregateAndGuards`, `TestCINV07600ReplayPR99`; old/new retained-input replay in `2026-11-05-ci-reuse-record-admission`; hosted REUSE pending workflow listing follow-up |
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
| CIN-V0-2320 | the `shell-test-selected` job of `ci.yml`; `make shell-test-affected`, `tools/shell-test-select` (AFP-V0-2401) | `TestCINV02320ShellTestSelectedJobRunsTheAffectedSteps`, `TestCINV02320RequiredCheckPropagatesShellFailure` in `tools/ci-reuse-plan/workflow_test.go` |
| CIN-V0-4625 | Advisory `go-macos` in `.github/workflows/go-macos.yml`; `.github/macos/{select,shard}.py`; native `.github/forward-rows/narrow.sh` invocation | `python3 -B .github/macos/select_test.py` (native files, indirect/legacy build constraints, multiline/reversed OS comparisons, switch cases, path aliases, drift, metadata errors, record validation, FULL fallback, nested-module retention and advisory wiring); `python3 -B .github/macos/shard_test.py` (complete deterministic cost partition, mixed FULL/NARROWED ownership, asymmetric cost-read failures, unknown-cost fallback, module concurrency and retained failures); `TestCINV04625AdvisoryDarwinIsOutsideRequiredNeedsGraph`, `TestCINV04625DarwinWorkflowNeverAdmitsLinuxReuse` in `tools/ci-reuse-plan`; existing narrowing guard tests in `tools/ci-forward-row`; hosted Darwin run NOT_RUN locally |
| CIN-V0-4626 | PLANNED/deferred: V1-4636; no required-aggregate dependency/result check | Required integration NOT_PRODUCED: blocked on V1-4413, V1-4551, V1-4553 and one green hosted FULL run; nightly run 41849536472 is failing at the lane base |
| CIN-V0-4750 | `.github/ci-cadence/classify.sh` DOCS and TESTS classes | `TestCINV04750CadenceClassifier` in `tools/ci-reuse-plan/cadence_test.go` |
| CIN-V0-4751 | the fixed eight-line receipt of `classify.sh` | `TestCINV04750CadenceClassifier`, `TestCINV04752CadenceRefusals` |
| CIN-V0-4752 | `classify.sh` topology and refusal checks; `.github/ci-cadence/run.sh` | `TestCINV04752CadenceRefusals`, `TestCINV04752CadenceRunWrapper` |
| CIN-V0-4753 | the `cadence-plan` job and its consumers in `ci.yml`; `selection-baseline-check` in the `Makefile` | `TestCINV04753CadenceWorkflowWiring` |
| CIN-V0-4754 | the `cadence-plan` job, `docs-plan`, `go-macos-shard` and `go-macos` aggregator in `go-macos.yml` | `TestCINV04753CadenceWorkflowWiring` |
| CIN-V0-4755 | the `selection-baseline` entry of `release-gates.yml` | `TestCINV04753CadenceWorkflowWiring` |
| CIN-V0-4756 | the `cadence-plan` fallback step and the `CI_CADENCE_FORCE` variable | `TestCINV04752CadenceRefusals`, `TestCINV04753CadenceWorkflowWiring` |

## Rollback

CIN-V0-4750..4756: set the repository variable `CI_CADENCE_FORCE=FULL`, or revert the cadence-plan jobs, `.github/ci-cadence/`,
the `CORVINT_CI_CADENCE` guard in `selection-baseline-check` and the nightly `selection-baseline` entry. No state migrates.

CIN-V0-2320: delete the `shell-test-selected` job; shell-test steps then run only in `make gate`.

CIN-V0-4625: revert the V1-4732 workflow extraction and sharder to restore the previous advisory
`ci.yml` job, accepting its workflow-completion delay; no required-status setting changes. For the
original capability rollback, remove advisory `go-macos` and `.github/macos/`. CIN-V0-4626 has no delivered
aggregate integration to roll back; V1-4636 owns its future activation.
The nightly macOS gate and existing Linux narrowing remain operative; no persistent data migrates.

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

## Lane maintenance evidence (2026-11-04)

| Requirement | Implementation and regression |
| --- | --- |
| `CIN-V0-7500` | `.github/forward-rows/tiers.jq`; `TestCINV07500AllFullReasons` (29 reasons). |
| `CIN-V0-7501` | `Makefile` DOC_GATE_TARGETS and `script/ci-least-privilege-exceptions.tsv`; `TestCINV07501PermissionsArePremerge`; `script/check-ci-least-privilege_test.sh` missing/changed per-job exception fixtures. |
| `CIN-V0-7502` | `.github/workflows/ci.yml` host-adapters and go-product; `TestCINV07502RequiredFamiliesAndOfflineModules`. |
| `CIN-V0-7503` | `.github/workflows/ci.yml` local-authority-deps and go-static; `TestCINV07502RequiredFamiliesAndOfflineModules`; hosted cache qualification remains NOT_OBSERVED. |
| `CIN-V0-7504` | `.github/workflows/ci.yml` web-flows-plan/web-flows/go-product; `TestCINV07504WebFlowsConsumerJob`, `TestCINV07504WorkflowPlanFlags` (actual CLI option parser); live PR/MQ evidence remains NOT_OBSERVED until publication is authorized. |
| `CIN-V0-7505` | `.corvint/test-read-scopes.json`, `script/batch-precheck.sh`; `TestCINV07505RepositoryCIRoutes`, `TestAFPV0591CoverageGuard`. |
| `CIN-V0-7506` | `.corvint/test-read-scopes.json`, `script/batch-precheck.sh`; `TestCINV07505RepositoryCIRoutes`, `TestCLIErrorContracts_ACXV0859`. |
