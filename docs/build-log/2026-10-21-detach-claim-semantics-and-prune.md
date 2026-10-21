# V1-1557 and V1-1565: detached claims, competing request IDs, and pruning records that never finished

Spike tickets V1-1557 and V1-1565. Decisions delegated to the orchestrator (decision 0548).

## V1-1557: a detached claim by a holder whose live attempt has a different request ID

`--detach` is a transport. The detached worker runs the same `claim` against the same store, and the
store alone decides whether the holder's live attempt admits it (same-holder rebind, refusal or
idempotent replay by request ID). Nothing in `lease_detach.go` reads or interprets claim state.

**Proposed recommendation:** handle it exactly as a foreground claim and add no detach-specific
semantics. Follow-up: an ATR requirement stating that a detached command's store outcome equals the
foreground outcome for the same argv and request ID, with a test that runs both against a holder
whose live attempt has a different request ID.

## V1-1565: detached records that never finish are never pruned

The ticket names CAL-V0-325/329. The owning IDs are ATR-V0-025 (the 7-day prune of finished records)
and ATR-V0-029. `pruneLeaseDetach` (`lease_detach.go:620-651`) removes only records that have a
`result.json`. A worker that was killed, or whose host rebooted, leaves a record without a result,
and that record is kept forever.

**Proposed recommendation:** also prune a record that has no result once its recorded `ceilingAt`
(the hard stop after which the worker can never write a result) is more than 7 days in the past.
A record without a readable `ceilingAt` is kept (fail closed). This is a follow-up build with an
ATR-V0-025 amendment and a test. It is outside the spike because `internal/tasks` is under active
feat-tasks-perf work.
