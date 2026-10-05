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
- **Status gap (ESC-V0-009):** open request kinds and ages are not yet in `dispatch status`. Its
  retry state is.

## Evidence

- **New tests**, all passing:
  - `TestESCV0007_*`: 11 tests in `internal/tasks/dispatch/issue502_retry_test.go` and
    `internal/tasks/cli/dispatch_escalation_pending_internal_test.go`.
  - `TestESCV0008_*`: 6 tests across dispatch and cli.
  - `TestESCV0010_*`: 5 tests in `internal/tasks/journal` and `internal/tasks/store`.
- **Mutation check:** disabling `escalations.bind` in `journal/records.go` failed 10 subtests and
  tests (every forgery, the redo and the checkpoint case). The original was then restored.
- **Package run:** the full `./internal/tasks/... ./cmd/corvint-tasks/...` run is retained in the
  lane TMPDIR. `gofmt` and `go vet ./internal/tasks/...` are clean.
- **`corvint affected`:** it selected the exhaustive gate. Per the owner's standing preference for
  scoped issue work, focused package tests ran instead and `make gate` is NOT_RUN.

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
