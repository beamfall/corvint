# 2026-10-09: pool confirm-safe on the writer checkpoint (V1-1045)

## Intent

Native ticket V1-1045 ([issue 701](https://github.com/beamfall/corvint/issues/701)) reports that
`corvint-tasks pool confirm-safe` replayed the whole receipt journal while holding
`taskman.prepare.lock`. On a queue of about 21,100 receipts this took 2 to 4 minutes or more.
Concurrent heartbeat, claim and confirm-safe calls failed with `LOCK_TIMEOUT`, and a 240 s caller
timeout killed one confirm-safe, which left its lane quarantined. Confirm-safe should build on the
writer checkpoint and take seconds. It must keep the V1-0695, V1-0674, V1-0645 and V1-0863
semantics, and a stale read must never admit an unsafe reuse.

## Root cause

`writerLeaseVerb` (`internal/tasks/store/writer_route.go:156`, before this change at :154-163)
admitted only claim, claim-next, renew, heartbeat and release to the CAL-V0-116 writer route.
`POOL_CONFIRM_SAFE` therefore declined before any observation. `leaseWrite` (`lease_write.go:362`)
then fell through to `prepareLease` (`lease_write.go:370`), which runs the complete audit
`hooks.audit(journalReader(repo, decoded))` (`lease_write.go:161`). That audit decodes every
receipt while the preparation gate is held. Each confirm-safe cost was linear in the size of the
history and was paid while holding the lock every other lease write needs.

## Change

`transaction.LeasePoolSafe` joins the writer-route lease verbs (`CAL-V0-205`, accepted by decision
0471). No model, codec or wire result changes. The confirm-safe plan (`planPoolSafe`) reads only
`pools.json` and the policy, both of which the summarized inventory selects. A sweep's
confirmation phase uses the same verb. Its witness check reads the evidence blob's digest from its
content-addressed name and listed size, as the route does for every blob. A model that reads an
elided path marks the inventory incomplete, and the route then declines.

Concurrency safety is unchanged from the other fast verbs. The route observes, models and commits
under one writer lock, which excludes cooperating writers. Before any effect,
`bindObservation` rechecks the observed head and intent tree, and `primaryBranch` rechecks the
branch. A concurrent writer therefore cannot interleave between the read and the commit, and a
stale observation is not committed. The following still hand the request to the complete route
with nothing written:

- a missing or unbound checkpoint;
- a head below the checkpoint minimum;
- a tail above 256;
- an unsettled journal;
- an elided read;
- a bind or branch mismatch.

## Tests

- `TestCALV0205_PoolConfirmSafeTakesWriterRoute` (`internal/tasks/store`, new) builds a pooled
  store, then claims, releases (quarantine) and confirms safe. It requires the confirmation to take
  the fast route, with the same bytes and report as a complete-route oracle (`writerParity`).
- The same test sends a second confirmation of the now-free member. The writer route must answer
  it (no decline, no write), and the answer must be refused exactly as the complete route refuses
  it (`sameDecision`).
- Before the fix the test failed: the confirmation `COMPLETED` through the complete route with no
  fast stages.
- Focused run `-run 'Pool|Sweep|CALV011|CALV0190|CALV0205|Writer'` over `internal/tasks/store`,
  `cli` and `transaction`: PASS. The full `internal/tasks/...` result is in the lane handback.

## Measurement (synthetic)

A scratch test, not committed, timed the `Lease` call for one confirm-safe on a `historyStore`
fixture with a pool of two members. It measured the writer route with a bound checkpoint and the
complete route with the checkpoint removed, which is the cost every confirm-safe paid before this
change. Host: Apple M2 Max, 12 CPUs, `GOMAXPROCS=3`, load average about 10 to 12 from other work.

| receipts | writer route | complete route |
|---:|---:|---:|
| 3,000 | 159 ms, 161 ms | 2.12 s, 2.33 s |
| 6,000 | 193 ms, 223 ms | 4.41 s, 4.30 s |

The complete route grows about linearly with history, at roughly 0.7 ms per receipt here. The
writer route stays near-flat. Two repetitions per size were taken because the fixture ticket's
retry budget limits further claim cycles.

## Limits

- Live qualification on the 21,100-receipt store: `NOT_RUN`.
- Without a bound writer checkpoint, confirm-safe still runs the complete audit under the
  preparation gate, as every lease verb does. The checkpoint's deferred refresh after the gate is
  released is unchanged.
- The CAL-V0-116 guarantee change now covers confirm-safe: a fast writer does not detect a prefix
  rewrite. Decision 0471 records owner acceptance, relayed by the coordinating session.
- Out of scope: other pool verbs (prepare, observe, recover, cleanup, acquire, release, sweep
  finish) still take the complete route under the gate.
- The CAL-V0-205 ID and decision number 0471 may collide with parallel lanes at integration.
