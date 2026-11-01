# Corvint Tasks delivery lifecycle V0

Owner: Russell Lewis
Date: 2026-11-01
Intent status: proposed
Delivery status: experimental

Authoritative inputs: the agent-filed native tickets V1-3945 (no read says where a ticket's
delivery stands: OPEN covers unclaimed, delivered-on-a-lane and merged-awaiting-closeout alike) and
V1-3856 (delivered lane heads are invisible until a batch lands). The progress phase of
[lane flow](corvint-tasks-lane-flow-v0.md) (`TLF-V0-880` to `TLF-V0-883`), the attempt receipts of
the [agent lease contract](corvint-tasks-agent-leases-v0.md) and the reads of
[closeout debt](corvint-tasks-closeout-debt-v0.md) are reused unchanged.

## Agent digest
- Claim: Show, search, roadmap and release readiness report one derived delivery lifecycle per ticket, with evidence from status, holds, attempts, progress and Git.
- Status: intent proposed; delivery experimental: the derivation, the batched Git observation and the four reads exist with focused tests.
- Exists: package `internal/tasks/lifecycle`, `store.ObserveDeliveries`, member `lifecycle` in show, search and roadmap rows, `ticket search --lifecycle`, facet `byLifecycle`, release readiness `memberCounts.byLifecycle` and `release readiness --lifecycle`.
- Blocked on: owner acceptance; durable qualification.
- Read next: Requirements; Failure modes and trust; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

The stored status has five values. While the queue barrier is OPERATOR/ADMISSION, every in-flight
ticket reads OPEN: one that nobody has started, one delivered on an unmerged lane head and one whose
commit already sits on `main` look alike. The progress phase of `TLF-V0-880` records a lane's claim
of delivery, but nothing checks it against Git, and no read counts tickets by where delivery stands.
Agents and the operator then re-dispatch delivered work or miss closeout.

## Requirements

- `TLC-V0-001`: Every read that reports a lifecycle MUST compute it at read time from the record,
  the live attempt receipts, the effective progress (`TLF-V0-881`) and observed Git refs. It MUST NOT
  be stored, MUST NOT be a ticket status, and MUST NOT be read by admission, eligibility, planning,
  gates, completion or authority. The `state` is one of `DRAFT`, `OPEN`, `IN_PROGRESS`, `SUBMITTED`,
  `IN_REVIEW`, `AWAITING_INTEGRATION`, `AWAITING_CLOSEOUT`, `HELD`, `COMPLETED`, `ARCHIVED` or
  `UNKNOWN`. No title, body or label text is an input.
- `TLC-V0-002`: Precedence MUST be, first match wins: status COMPLETED or ARCHIVED; status DRAFT;
  status HELD (evidence: the hold IDs); a live attempt, where phase `READY_FOR_INTEGRATION` is
  `AWAITING_INTEGRATION`, phase `REVIEWING` or stage `review` is `IN_REVIEW`, a recorded candidate
  tree is `SUBMITTED` and any other live phase is `IN_PROGRESS`; the effective progress phase under
  `TLC-V0-003`; and otherwise `OPEN`. When attempt receipts are not observable (the journal is
  absent or unreadable) and no progress phase verified by Git applies, the state MUST be `UNKNOWN`
  with reason `ATTEMPTS_NOT_OBSERVED`, never `OPEN`.
- `TLC-V0-003`: A `DELIVERED_ON_BRANCH` or `MERGED_AWAITING_CLOSEOUT` progress phase MUST be checked
  against Git. The integration ref is `refs/remotes/origin/<intentBranch>` when it exists and
  `refs/heads/<intentBranch>` otherwise. A head reachable from that ref's commit is
  `AWAITING_CLOSEOUT`. A `DELIVERED_ON_BRANCH` head that is not integrated but is reachable from its
  branch ref is `AWAITING_INTEGRATION`. The branch is resolved as `refs/heads/<branch>`, then
  `refs/remotes/origin/<branch>`, then the only other remote-tracking ref with that name; a branch
  that starts with `refs/` is used as given.
- `TLC-V0-004`: A delivered phase that Git does not confirm MUST fall back with a named reason: to
  `OPEN` with `COMMIT_MISSING` (no such commit object), `REF_DELETED` (no branch ref),
  `HEAD_NOT_ON_REF` (the branch was rebased or reset past the head) or `NOT_ON_INTEGRATION_REF` (a
  `MERGED_AWAITING_CLOSEOUT` head that is not integrated); and to `UNKNOWN` with `GIT_NOT_OBSERVED`
  (no repository or a failed Git read), `INTEGRATION_REF_MISSING` or `REACHABILITY_NOT_OBSERVED`
  (an observation budget was spent). Phase `IN_REVIEW` or `IN_MERGE_QUEUE` reads `IN_REVIEW`,
  `IN_PROGRESS` reads `IN_PROGRESS` and `BLOCKED` reads `OPEN` with reason `REPORTED_BLOCKED`; none
  of these is checked against Git.
- `TLC-V0-005`: The `lifecycle` value MUST be the closed object `{state, reason, derivedFrom,
  status, holdIds, attempt, progress, git}`: `reason` is a code or null; `derivedFrom` is one of
  `status`, `hold`, `attempt`, `progress` or `none`; `attempt` is null, `NOT_OBSERVED` or
  `{attemptId, phase, stage, candidateTreeOid}`; `progress` is the effective progress entry or null;
  `git` is null or `{integrationRef, integrationCommit, ref, refCommit, headExists, integrated,
  headOnRef}` with each observation true, false or null for not observed.
- `TLC-V0-006`: One read MUST observe Git for all of its tickets at once with a fixed number of Git
  processes that does not grow with the number of tickets: one ref listing, one object check, one
  integration walk and one walk over the distinct branch refs, plus at most 16 bounded ancestry
  checks for heads the walks leave ambiguous. Each walk is capped; a spent cap yields
  `REACHABILITY_NOT_OBSERVED` for the heads it leaves undecided.
- `TLC-V0-007`: A lifecycle read MUST NOT change the repository, the store or the journal.
- `TLC-V0-008`: `ticket show` MUST report member `lifecycle`.
- `TLC-V0-009`: `ticket search` MUST report `lifecycle` on each row, MUST accept
  `--lifecycle STATE` as a filter that alone satisfies the at-least-one-filter rule, and with
  `--facets` or `--count` MUST report `facets.byLifecycle` with a count for every state. This amends
  the closed `CAL-V0-206` summary by one optional member: a result decoder MUST admit
  `byLifecycle` only as an object of Counts with exactly the `TLC-V0-001` states, summing to
  `total`, and every other read keeps its earlier facet bytes.
- `TLC-V0-010`: `roadmap` rows MUST report `lifecycle`.
- `TLC-V0-011`: `release readiness` MUST report `memberCounts.byLifecycle` with a count for every
  state over the release's member tickets, and `release readiness RELEASE --lifecycle STATE` MUST
  also report `lifecycleMembers` as a count plus at most 100 sorted ticket IDs in that state.
- `TLC-V0-012`: Recording delivery is the existing `ticket progress` phase `DELIVERED_ON_BRANCH`
  with branch and head (`TLF-V0-880`), which needs no claim. Once that head is integrated,
  closeout-debt lists the ticket as a `progress-head` row (`TLF-V0-883`); this spec adds no verb and
  no write.

## Failure modes and trust

- A progress entry is a principal's claim. The lifecycle checks it against local refs only: a lane
  head on another host that was never fetched reads `REF_DELETED` or `COMMIT_MISSING`. The reason
  names the observation, not the cause.
- A force-push that keeps the head reachable reads as delivered; a rebase that drops it reads
  `HEAD_NOT_ON_REF`. Neither is an error.
- An over-budget walk or a failed Git process degrades to `UNKNOWN`, never to a guess. A stale
  remote-tracking ref gives a stale answer: no read fetches.
- Attempt receipts and Git are read at different instants; a concurrent writer can make one row
  older than another. Each value carries the evidence it used.

## Non-goals and simpler baseline

- No sixth status and no stored lifecycle: the simpler baseline, a stored state, would drift from
  Git and from the receipts.
- No GitHub query and no PR state: `IN_REVIEW` from progress stays advisory.
- No change to closeout-debt, delivery-screen or dispatch.
- V1-3856's orphan-lane report, dispatch refusal on orphan-cited tickets and batch-branch reachability
  on lane hosts are outside this slice.

## Acceptance evidence and traceability

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| TLC-V0-001, TLC-V0-002, TLC-V0-005 | V1-3945 criteria 1 and 4 | `internal/tasks/lifecycle` | `TestTLCV0001_Vocabulary`; `TestTLCV0002_Precedence`; `TestTLCV0005_EvidenceValue` | none |
| TLC-V0-003, TLC-V0-004 | V1-3945 criteria 2 and 4 | `internal/tasks/lifecycle`; `internal/tasks/store/delivery_refs.go` | `TestTLCV0004_FallbackReasons`; `TestTLCV0003_ObserveDeliveries`; `TestTLCV0008_ShowLifecycleTransitions` | none |
| TLC-V0-006 | V1-3945 (batched observation) | `internal/tasks/store/delivery_refs.go` | `TestTLCV0006_ObservationIsBatched`; build-log measurement on the 1,200-ticket store | durable qualification |
| TLC-V0-007 | product invariant 4 | `internal/tasks/cli/lifecycle.go` | `TestTLCV0007_LifecycleReadsAreReadOnly` | none |
| TLC-V0-008 | V1-3945 criterion 1 | `internal/tasks/cli` (show) | `TestTLCV0008_ShowLifecycleTransitions` | none |
| TLC-V0-009 | V1-3945 criteria 1 and 3 | `internal/tasks/cli` (search, facets, help); `internal/tasks/wire/facets.go` | `TestTLCV0009_SearchLifecycleFilterAndFacet`; `TestCALV0206_FacetsMemberIsTheClosedSummary` | none |
| TLC-V0-010 | V1-3945 criterion 1 | `internal/tasks/cli` (roadmap) | `TestTLCV0010_RoadmapLifecycle` | none |
| TLC-V0-011 | V1-3945 criteria 1 and 3 | `internal/tasks/cli/release.go` | `TestTLCV0011_ReleaseReadinessLifecycleCounts` | none |
| TLC-V0-012 | V1-3945 criterion 2; V1-3856 (delivered lane heads visible) | reuse of `TLF-V0-880`, `TLF-V0-883` | `TestTLCV0012_DeliveredThenIntegratedIsCloseoutDebt` | none |

## Resolved decisions

1. Derived, not stored: the value is recomputed on each read from its evidence, so it cannot drift
   and needs no migration.
2. `AWAITING_CLOSEOUT` joins the ticket's vocabulary: an integrated head that is not yet completed is
   the closeout-debt case, and folding it into `AWAITING_INTEGRATION` would hide it.
3. Recording delivery reuses `ticket progress DELIVERED_ON_BRANCH` instead of a new verb, since it
   already needs no claim and passes the operator barrier.
4. A live attempt outranks progress: receipts are evidence of the runtime, progress is a claim.
5. Git observation is batched per read and capped, not per ticket.

## Unresolved decisions

- Whether dispatch should refuse tickets whose lifecycle is `AWAITING_INTEGRATION` (V1-3856).

## Rollout and rollback

Rollout is additive: new read members, one optional facet member, one new search flag and one new
release readiness flag; no record, receipt or journal bytes change. Rollback is reverting the change; no store state depends on
it.
