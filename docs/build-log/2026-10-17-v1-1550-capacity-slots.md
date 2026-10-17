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

## Independent review repair (PSR-V0-039..043)

A Codex review of PR #1019 at 19f66b4e4 reported five defects; all five were confirmed against the
source and repaired at the root, each with a regression test that fails on 19f66b4e4:

1. Supervisor attach stranded slots (supervised attempts are never expired, reaped or
   slot-released). ATTACH now refuses `UNSUPPORTED` while the attempt has a live slot entry
   (PSR-V0-039). The guard is at attach rather than in slot pruning because a supervised attempt is
   legitimately live; letting it keep slots would need a supervisor-owned release path.
2. Policy changes could grant above the current capacity. The policy-update route does not load
   attempts, so the fix is in grant settlement, not a policy guard: a waiter the current policy
   would refuse is neither granted nor blocks others, and an exclusive takes the current capacity
   (PSR-V0-040). Holders granted under a larger capacity keep their grant (no revocation).
3. Alternating higher-priority exclusive requests could starve a reserved class. Exclusive and
   reserved-class requests are now served in enqueue order between themselves; the earliest such
   request is never blocked, so neither side starves (PSR-V0-041, amends PSR-V0-033).
4. No-change release and poll requests wrote no receipt, so a retry could act on a later state.
   They now record a request-only `COMPLETED` receipt (PSR-V0-042, amends PSR-V0-036/037).
5. A replay reported the current entry, which may be a later request's grant. A replay now carries
   `slot` null (PSR-V0-043, amends the PSR-V0-038 replay clause). Reconstructing the original grant
   would need the receipt to carry the slot outcome, which is a wire change not needed for safety.
