# V1-1550: capacity slots without per-use quarantine

GitHub #1008 asked for shared capacity on a scarce stateless resource without a pool member's
cleanup and quarantine. Spec choice: a new "Capacity slots" section in the existing
pool-safe-reuse spec rather than a new spec, because slots reuse the same lease verbs, pools.json
journal record, generation fence and lease reap; a separate resource kind would have needed its own
state file and journal path. `slots acquire|release` are lease verbs; entries of ended, reaped or
undeclared attempts simply stop counting and are pruned on the next slot write, so no sweep or daemon
exists. Waiters poll with a fresh request id. Grants follow (priority, seq, attemptId); an exclusive
waiter stops later shared grants; other classes' unused reservations are excluded from what a request
may take.

Requirements PSR-V0-030..038 added. Owner standing auto-accept of lane requirements (2026-10-11)
applies. Rollback: release or end every slot holder so pools.json drops `slots`, remove policy
`slots`, revert the source; older binaries refuse either key as `MALFORMED` meanwhile.
`pool_acquire.go` and `priority_admission.go` (V1-1560) were not touched. `make gate` and the full
`go test ./...` were NOT_RUN; concurrent slot writers were not separately stress-tested.
