# 2026-10-05: issue 502 bounded infrastructure retry, park classification and escalation material

## Intent

Owner request 2026-10-05 for [issue 502](https://github.com/beamfall/corvint/issues/502) (ticket
V1-0699): finish typed worker escalations so the issue can close. This slice covers the rest after
PR #592 (base `d524530fc9f53e62a14a3918a760b6df9d80f66e`). The owner decision binding V1-0791 is
unchanged: that escalation stays a dispatcher `needs-owner` event acknowledged only by
`ticket reopen`, and `loopEscalations` is not touched.

## Requirements

`ESC-V0-005`, `ESC-V0-007`, `ESC-V0-008` and `ESC-V0-010` in
`docs/specs/corvint-tasks-escalations-v0.md`. That spec's Acceptance evidence, findings, Integration
slot and Rollout sections are updated in this change.

## Change

- **ESC-V0-005 (claim delivery):** already on main through PR #592. Verified on this base by
  `TestESCV0005_*` in the full tasks package run. No code change.
- **ESC-V0-007 (bounded infrastructure retry):** the optional dispatch config is
  `infrastructureRetry {maxRetries, cooldownSeconds, maxCooldownSeconds}`. The optional ledger
  member is `infraRetry`, keyed by ticket and holding the acceptance revision, state, charged
  count, sessions, requests, next eligible time, reason, fingerprint and the pending reservation.
  - A retry is reserved and saved before launch. A failed save launches nothing that tick.
  - On restart, a reservation with no worker directory is reused without a new charge. Any other
    reservation is UNKNOWN (`RESERVATION_UNRESOLVED`).
  - An ambiguous spawn is UNKNOWN (`SPAWN_AMBIGUOUS`).
  - Exhaustion is named `INFRA_RETRY_EXHAUSTED`, `INFRA_RETRY_DISABLED` or
    `NATIVE_RETRY_EXHAUSTED`. The native request stays OPEN.
  - `dispatch status` lists the episodes as `infrastructureRetry`.
  - Existing debt, native exhaustion and ownership rules stay authoritative: a native
    `RETRY_EXHAUSTED` plan reason always wins.
- **ESC-V0-008 (park classification):** the native observation now carries each ticket's
  acceptance revision and its typed requests: kind, state, audited source holder and open time.
  Unreadable material is `EscalationUnknown`. Session classification runs before no-progress
  parking:
  - A session that raised a decision, scope or blocked request is held. It leaves the ladder,
    count and backoff untouched.
  - A session that raised an infrastructure request charges its retry episode. It resets only the
    proved failure streak and keeps the reached 499 tier as a floor (`retainTiers`).
  - Neither kind of session parks.
- **ESC-V0-010 (typed-event stage, material branch):** `internal/tasks/journal/escalation_audit.go`
  binds every receipt that carries escalation events, in receipt audit and redo. The receipt must
  be the exact shape the native writer commits:
  - The target ticket is replayed from its audited pre-record. Only the reference, revision,
    previous-record digest and update stamp may change.
  - Revision, control and work arithmetic must match the reducer.
  - Each new or terminal entry must be explained by exactly one event. That event's identity,
    actor, time, request, chain, origin and source must agree with the receipt.
  - Any other receipt must leave every walked ticket's reference byte-identical.
  - The events now earn per-post semantic coverage.

## Decisions (agent, fail-closed; raised as owner questions)

- **Absent policy:** an absent `infrastructureRetry` policy is treated as disabled. The first
  infrastructure session holds as `INFRA_RETRY_DISABLED` instead of retrying.
- **No reset command:** EXHAUSTED and UNKNOWN episodes clear only through:
  - checked progress;
  - an outside change of the work fingerprint;
  - an acceptance change;
  - the ticket leaving the observation.

  An operator answer is not progress.
- **Classification scope:** a session matches requests only by the holder recorded in the request's
  audited source, on the current acceptance revision, and only while the request is OPEN or
  ANSWERED. A ticket with no observed acceptance revision gets ordinary accounting.
- **LEASE route kept:** ESC-V0-010's text names issue 501's MUTATE slot, with supersession
  deferred. The writer already ships on the LEASE stage with supersession. This slice binds that
  route's material in the journal. It does not widen any StageLease bound and adds no distinct
  stage operation. The owner must decide whether to accept the LEASE route and amend the text, or
  move the writer to the MUTATE slot.

## Limits

- **Claim receipt not re-read:** the journal does not re-read the claim receipt to verify its
  digest; the writer verified it before commit. The journal checks the source's queue, ticket,
  acceptance and holder, and that the source sequence precedes the receipt.
- **Coverage stays UNKNOWN in lease stores:** ADMIT receipts' `attempts/` and `reservations.json`
  posts keep store-level semantic coverage UNKNOWN. That predates this slice. Event coverage is
  asserted at the journal level.
- **Checkpoint tail:** a checkpoint-resumed read does not compare the first tail post of a ticket
  whose reference predates the checkpoint; the complete `receipt audit` does.
- **Status gap (ESC-V0-009):** open request kinds and ages are not yet in `dispatch status`. Its
  retry state is.

## Evidence

- **New tests**, all passing:
  - `TestESCV0007_*`: 14 tests in `internal/tasks/dispatch/issue502_retry_test.go` and
    `internal/tasks/cli/dispatch_escalation_pending_internal_test.go`.
  - `TestESCV0008_*`: 6 tests across dispatch and cli.
  - `TestESCV0010_*`: 6 tests in `internal/tasks/journal` and `internal/tasks/store`.
- **Mutation check:** disabling `escalations.bind` in `journal/records.go` failed 10 subtests and
  tests (every forgery, the redo and the checkpoint case). The original was then restored.
- **Package run:** the full `./internal/tasks/... ./cmd/corvint-tasks/...` run is retained in the
  lane TMPDIR. `gofmt` and `go vet ./internal/tasks/...` are clean.
- **`corvint affected`:** it selected the exhaustive gate. Per the owner's standing preference for
  scoped issue work, focused package tests ran instead and `make gate` is NOT_RUN.

## Codex review

Round 1 (`codex exec -m gpt-6-astra -s read-only` over `d524530f..44d7facf`) reported three P2
findings. All three were verified against the code and fixed with regressions that fail without the
fix:

- **Reserved retry over a narrowed policy:** a reservation saved before a reload was relaunched even
  when the new policy no longer allowed its ordinal. `reserveInfra` now checks the reserved
  ordinal against the current bound and holds it (`INFRA_RETRY_EXHAUSTED`, or `INFRA_RETRY_DISABLED`
  at 0), keeping the charged count. Regression: `TestESCV0007_NarrowedPolicyHoldsAReservedRetry`.
- **Event request not bound to the receipt request:** an event's `OriginalRequest` could be
  rewritten (an answer's text, say) with the event, evidence path and ticket head rehashed, and the
  audit accepted it. The journal now hashes the retained request, restates the LEASE request digest
  preimage, and requires it to equal the receipt entry's mutation digest. It also binds
  `ResolvedRequestID` and `ResolvedPreviousRevision`, and the request's operation, selector,
  expected revision and replacement to the event.
- **Terminal source not bound to the question's source:** a terminal event could carry a source
  other than the one its question was opened with. The audit now keeps each opened question's
  source and refuses a terminal step that changes it; an origin before a checkpoint falls back to
  the full audit.

Regression for the last two: `TestESCV0010_ConsistentlyRehashedEventIsJournalForked`. With the
pre-fix `escalation_audit.go` both subtests' forgeries audit clean.

Round 2 (over `d524530f..3d4b9023`) reported three P2 findings. Two were verified and fixed with
regressions that fail without the fix; one is declined:

- **Stale backoff recovered an episode:** `unpark` recovered the infrastructure episode whenever a
  non-parked backoff's fingerprint differed from the current one. A backoff left at an older
  fingerprint by an ordinary failure therefore cleared every later infrastructure episode on the
  next tick, so the retry bound never held. `unpark` no longer recovers episodes; `reconcileInfra`
  compares the episode's own baseline. Regression: `TestESCV0007_StaleBackoffDoesNotRecoverAnEpisode`.
- **First progress token recovered an episode:** first-token admission rewrapped the worker and
  backoff baselines but not the episode's, so the token alone looked like progress. The episode
  baseline is now rewrapped in the same staged ledger. Regression:
  `TestESCV0007_FirstProgressTokenDoesNotRecover`.
- **Declined: checkpoint tail does not compare a pre-checkpoint reference.** In a
  checkpoint-resumed read, the first ordinary post of a ticket last posted before the checkpoint is
  not compared against its earlier reference, because that reference lives in the prefix the
  checkpoint read must not open. Returning `errCheckpoint` there would send every ordinary edit
  tail to the complete audit and break the accepted operator-note contract, which mirrors this
  rule (`internal/tasks/journal/operator_note.go`; `TestONV0006_CheckpointTailNeverReadsThePrefix` keeps a REFINE tail in
  checkpoint mode). `receipt audit` never resumes from a checkpoint and refuses the forgery.
  Raised as an owner question: carry walked reference digests in the checkpoint, or accept the
  read-path limit for notes and escalations alike.

## NOT_RUN

- a live dispatcher and compiled-binary witness
- interrupted paired publication at each artifact
- a distinct typed-event stage operation
- open kinds and ages in `dispatch status`
- `make gate`

## Rollback

See the spec's Rollout and rollback section:

- The ledger members `infraRetry` and `retainTiers` and the config key `infrastructureRetry` are
  removed with `jq` after a backup.
- The journal binding is a plain revert of `escalation_audit.go` and its call sites.
