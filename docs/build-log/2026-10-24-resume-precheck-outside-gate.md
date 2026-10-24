# 2026-10-24: resume base checks precheck outside the gate (V1-2537, PSR-V0-225)

## Decision

A resumed lease round (widen and reap here) now runs its snapshot-independent base reads before it
takes the preparation gate (PSR-V0-225, amending where PSR-V0-052, PSR-V0-056 and JSG-V0-011 read).
`Reader.PrecheckResume` re-hashes every settled bound segment and lists every settled request
shard through a read lifetime of its own, and records only what it verified. Under the gate the
resumed audit and the recheck before commit still stat every segment and every request and compare
stamps, change time included. They skip only the re-hash of a recorded segment and the listing of
a shard whose identity and stamps, by both the fresh `requests/` listing and the shard's own handle,
are the recorded ones, and after the last such shard the `requests/` pathname must still name the
handle they were reached through. The precheck admits nothing: a fault, mismatch or unsettled stamp records
nothing, and the gated check does its full work and declines as before. Implementation:
`internal/tasks/journal/resume.go` (`PrecheckResume`, `precheck`, `shardEntries`, `rebind`,
`sameStamps`), and
the call in the lease loop in `internal/tasks/store/lease_write.go` (stage `fast.resume.precheck`).

## Evidence (macOS arm64, 12 CPUs, UNQUALIFIED)

The harness prints `QUALIFIED: load <= 12`, but macOS numbers are not the acceptance host. Profile
first: on base `1bcc921` (PR #1672 head), a widen or reap hold was lease.observe (216 to 240 ms)
plus lease.bind (68 to 80 ms). The resumed audit's JSG-V0-011 re-hash of 78 segments (about 134 ms)
and the request-shard listing (about 256 shards and 600 files, about 60 ms in the audit and again
in the recheck) did that work under the gate. None of it depends on the snapshot the lock observes.

`TestCALV0263_LeaseGateLoad` (40,000 receipts, 15 writers, 90 s, 10 s think), wall and hold in ms:

| verb | before (base, load 3.4 to 5.7) | after run 1 (load 5.5 to 7.0) | after run 2 (load 7.4 to 6.0) | final (load 5.4 to 6.1) |
| --- | --- | --- | --- | --- |
| widen hold p50/p99 | 334/371 | 121/135 | 120/132 | 124/147 |
| reap hold p50/p99 | 295/306 | 84/98 | 77/85 | 82/87 |
| heartbeat wall p99, gate wait p99 | 360, 299 | 1,013, 153 | 235, 155 | 306, 223 |
| renew wall p99, gate wait p99 | 822, 761 | 187, 119 | 205, 120 | 332, 267 |

Runs 1 and 2 predate the review fix below; the final run is the delivered code. The precheck
stage, outside the gate, costs 201 to 238 ms per resumed round. Under the gate, widen and reap
lease.observe fell to 42 to 56 ms and lease.bind to 26 to 41 ms. In the final run the five
heartbeat and renew gate waits over 150 ms overlapped holds by widen (459 ms in total), renew
(397 ms) and heartbeat (61 ms); widen and reap holds are no longer the tail by themselves. Under
this macOS load heartbeat and renew p99 stays noisy, from 187 to 332 ms across runs. Widen and reap wall time
(about 10.5 s) is unchanged. In after run 1, two heartbeats spent about 0.9 s outside the gate and
its stages, and one had a 474 ms lease.commit. Neither is in the code this change touches; both
are inferred to be host stalls, and run 2 shows no such samples. These numbers are not comparable
to the WSL2 numbers on V1-2205 or V1-2203. The WSL2 quiet-host rerun decides acceptance:

```console
$ CORVINT_TASKS_LEASE_GATE_LOAD=/tmp/v1-2537-<head|base>.json CORVINT_TASKS_LEASE_GATE_LOAD_SECONDS=90 \
    GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/tasks/store -run '^TestCALV0263_LeaseGateLoad$'
```

Run it once on the PR head and once on base `1bcc921`.

Failure-first: with the reuse disabled (segments always re-hashed, every shard always listed),
`TestPSRV0225_PrecheckMovesBaseReadsOutOfTheGate` fails with `after the precheck: 14 shard
listings, 1 segment reads under the gate; want none`, and passes with the change.
`TestPSRV0225_PrecheckNeverAdmitsAStaleBase` proves the moved work stays bound to what it
observed. These all decline: a stray request in a trusted shard (also with the shard's mtime
restored), a removed or rewritten request, a stray in a new shard, and a rewritten segment, whether
after the precheck or after the resumed audit. A segment rewritten before the precheck is not
trusted, and a failed precheck close is cleanup. `TestJSGV0011_ResumeDeclinesRewrittenSegment`
now rewrites the segment at `fast.resume.precheck`, after the precheck re-hashed it, and the
sealed route still declines.

## Review findings (Codex, one round)

- Cached shard names skipped the pathname binding (P1, confirmed). A reused shard was reached
  through the pinned `requests/` handle, so renaming `requests/` during the recheck and putting a
  copy with a stray in its place validated the detached old tree. `List` had bound each shard's
  pathname. A first repair bound each reused shard (`DirInfo`), but it cost about 40 ms per gated
  pass: widen hold rose to 209/245 ms and reap to 164/181 ms (load 6 to 7). The delivered repair
  binds `requests/` once, after its last shard (`rebind`), which covers every ancestor and the
  whole shard loop. A shard replaced inside `requests/` has another identity and is listed afresh.
  `TestPSRV0225_PrecheckNeverAdmitsAStaleBase/during-recheck/requests-replaced` fails without the
  rebind (`decline <nil> cleanup <nil>`) and passes with it.

## Found, not fixed

Claim gate wait p99 stays about 1.3 s under this load: claims contend with each other, not with
widen or reap. The after-run-1 heartbeat stalls outside the gate are unexplained.

## Rollback

Make `PrecheckResume` record nothing, or drop its call in the lease loop. The gated checks then run
exactly as before.
