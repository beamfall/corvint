# CI Narrowing V0

Owner: Russell Lewis
Date: 2026-10-30
Requirement prefix: `CIN-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3309, V1-3091, V1-3158, V1-3227, V1-2902, V1-3329); CIN-V0-001 decided by the orchestrator under owner delegation (V1-3314), applying the AFP-V0-030 re-arm rule; effective only after the owner's `ci-control-plane` review of the `.github` change
Delivery status: experimental
Authoritative inputs: native tickets V1-3314, V1-3309 (AC3), V1-3091, V1-3158, V1-3227,
V1-2902 and V1-3329. These requirements operate the CI narrowing of the [affected plan](affected-plan-v0.md)
(AFP-V0-028..030, AFP-V0-046, AFP-V0-097, AFP-V0-670, AFP-V0-730..734); that spec's
Requirements section is at the 256-obligation OCM limit (`maxObligations`), so they live here.
Requirement IDs CIN-V0-001 to CIN-V0-049 are reserved for this spec.

## Agent digest
- Claim: CI narrowing restarts its epoch per producing run, fails loudly on a dead pin, lists only needed rows, skips idle shards and tests tools/local-authority.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3309, V1-3091, V1-3158, V1-3227, V1-2902, V1-3329); CIN-V0-001 decided by the orchestrator under owner delegation (V1-3314), applying the AFP-V0-030 re-arm rule; effective only after the owner's `ci-control-plane` review of the `.github` change; delivery experimental.
- Exists: `.github/workflows/ci.yml`, `.github/workflows/ci-forward-bar.yml`, `.github/workflows/ci-control-plane.yml`, `.github/forward-rows/{names.sh,bar.jq,verdict.sh,accept.sh,selection.jq,collect.sh}`, `Decide` in `tools/ci-reuse-plan/main.go`; tests named for each requirement.
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

## Non-goals

merge-queue tree reuse of `go-static`, `go-interop` and `cce-live` (V1-2902 remainder: it needs
a pull-request record job and would change the AFP-V0-670 and AFP-V0-080 gated-job structure);
building the shadow planner once per run (V1-2902 remainder: it touches the pin steps PR #11
changes and needs a trusted artifact handoff); reusing a narrowed queue record for a `main` push
(rejected, CIN-V0-005); planner selection of a `tools/local-authority` unit, which belongs to the
affected planner (V1-0872), so CIN-V0-008 runs the module whenever `go-static` runs; changing the pin values; changing the AFP-V0-816
`BOUNDED` scope check of `mq-narrow.sh`; adding `infra-reclassified.txt` keys (AFP-V0-463).

## Failure modes

(1) A `FORWARD_EPOCH_SINCE` later than the first epoch-2 row would hide rows from the verdict;
the value precedes the batch carrying epoch 2, and it moves only with `FORWARD_EPOCH`. (2) A row
created during the verdict's own listing could be missed by the incremental acceptance; the
hour of overlap before `listedAt` covers it. (3) A run with no retained selection document
(no shadow, a dead shadow pin, or a failed artifact download) writes no row under CIN-V0-001, so
a sustained shadow outage no longer counts `INCOMPLETE` toward the kill switch; the bar still
needs 8 complete epoch-2 rows, so narrowing stays FULL through it. A queue row takes the epoch
of its pull-request selection run. (4) A dead pin turns every shard red until repinned
(CIN-V0-002); that is the intended loud failure. (5) An old `ci-reuse-plan` refusing `--base` on a
push fails the reuse step, so `docs-plan` fails and every shard runs FULL. (6) The CIN-V0-008
fetch needs the module proxy; an unreachable proxy or a `go.sum` mismatch fails `go-static`
loudly rather than skipping the module.

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

## Rollback

R1: revert the lane's `.github` commits and their test commits; the epoch returns to 1 only by an
owner decision, since epoch-1 rows hold the dead-pin `INCOMPLETE` rows. No stored state depends on
these requirements; verdict artifacts without `listedAt` keep the whole listing.
