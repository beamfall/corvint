# Tasks lease-gate latency: guards cleanup lock, fdinfo retention, link-count base files, claim validation

Date: 2026-10-24. Tickets: V1-2531, V1-2537, V1-2539, V1-2203. Requirements: CAL-V0-393, CAL-V0-394,
CAL-V0-396, CAL-V0-397 (`docs/specs/corvint-tasks-agent-leases-v0.md`, V1-2531 subsection) and
PSR-V0-247 (`docs/specs/corvint-tasks-pool-safe-reuse-v0.md`), all proposed. Base 0949876eb.

Platform for every number below: Windows box, WSL2 Ubuntu, linux/amd64, Ryzen 9 5950X (32
logical CPUs), go1.27.3, scratch stores under `/home/russell/bench/tmp/lat` (never the real
`.taskman`). A run is QUALIFIED when its 1-minute load is at most 12, as the harness reports.
Raw reports: `/mnt/c/cvq/proof/2026-10-24/p1-tasks-latency/` on the box (`lease-{base,head}-N.json`,
`hold-{base,head}-*.json`).

## Root causes

- A (V1-2531, CAL-V0-393). A lease round's guards ran orphan cleanup, which takes the writer lock
  whenever `staging/` is non-empty. A live peer keeps its slots in `staging/` for its whole hold,
  so a heartbeat or renew whose writer route was ready queued for that peer's hold outside the
  gate, then queued again inside the route. The ready route now takes no cleanup lock; under the
  lock it declines `staging` if any entry is present (under the lock every entry is an orphan),
  and the round then cleans up as before.
- B (V1-2203, CAL-V0-394). About 85 % of journal-write CPU in one heartbeat went to pinned-parent
  checks, each opening, reading and closing `/proc/self/fdinfo/<fd>` for about eleven mounts. The
  session now retains each descriptor's fdinfo file and re-reads it from offset 0. Decision 0411's
  per-check mount-ID observation is kept.
- C (V1-2537, PSR-V0-247). `applyArtifacts` link-publishes a receipt and unlinks its staging name
  after publishing HEAD. The unlink moves the receipt's ctime, so a widen or reap whose base audit
  caught the newest receipt declined `checkpoint unusable: base file changed` and retried the
  whole round. A base file whose stamps had not settled is now compared without ctime and its
  bytes re-hashed against the base digest; settled files keep the full stamp comparison.
- D (V1-2531, CAL-V0-397). A claim profile at 13.5k receipts and 880 tickets put 180 of 450 ms
  of samples in `loadTickets`: `ticket.Decode` and then `canonical(raw)`, a second `wire.Parse`
  plus re-encode that cannot fail once the first parse succeeds, and a rune-by-rune hostile check
  on every string. The second parse is dropped and the hostile check takes an ASCII byte scan
  first. Added after the single Codex review round, so it is not Codex-reviewed.

## Load harness: `TestCALV0263_LeaseGateLoad`, 40k receipts, 15 writers, 120 s per run

Primary: pair 6 (head first, both QUALIFIED, load 6.0 / 6.7) and pair 5 (head first, both
QUALIFIED, load 7.7 / 4.1). Milliseconds, p50/p99; n per verb per run in brackets. Pair 6 head
includes CAL-V0-397; pair 5 head does not.

| verb | metric | pair 5 base | pair 5 head | pair 6 base | pair 6 head |
|---|---|---|---|---|---|
| heartbeat (n 124-125) | wall p50/p99 | 135/500 | 141/466 | 133/740 | 126/587 |
| heartbeat | outside-gate p99 | 144.5 | 48.4 | 200.3 | 16.7 |
| renew (n 129-132) | wall p99 | 722 | 406 | 743 | 374 |
| renew | outside-gate p99 | 84.6 | 27.1 | 147.9 | 17.1 |
| claim (n 14) | wall p99 | 1871 | 1833 | 1909 | 1889 |
| claim | gate wait p99 | 1728 | 1695 | 1766 | 1730 |
| claim | hold p50/p99 | 108/114 | 108/119 | 109/116 | 100/134 |
| widen (n 9-16) | hold p99 | 192 | 149 | 195 | 140 |
| reap (n 5-8) | hold p99 | 80 | 118 | 125 | 63 |
| widen + reap | `base file changed` declines | 4 | 0 | 6 | 0 |

Supporting: pairs 3 and 4 (head QUALIFIED, base UNQUALIFIED at load 15-20) gave heartbeat
outside-gate p99 base 125 / 192 against head 19.5 / 37, widen hold p99 head 198 / 154, reap hold
p99 head 66 / 76, and declines base 3 / 1 against head 0. Pooled pairs 1-2 (both UNQUALIFIED):
base 15 widen/reap declines, head 0.

Widen and reap wall (11-48 s) is outside the gate: a fresh-process base snapshot read of the
whole receipt history, not gate hold.

## Hold profile: `TestCALV0118_WriterHoldProfile`, 880 tickets, n = 5 per verb, medians in ms

| receipts | verb | base lock hold | head lock hold | base validation | head validation |
|---|---|---|---|---|---|
| 13.5k | claim | 270.8 | 217.2 | 149.3 | 73.0 |
| 40k | claim | 285.3 | 197.1 | 145.2 | 67.0 |
| 13.5k | release | 269.3 | 247.4 | 147.0 | 79.5 |
| 40k | release | 291.7 | 216.0 | 162.7 | 66.0 |
| 13.5k | heartbeat wall | 73.0 | 67.3 (A-C) / 94.2 (A-D run) | - | - |
| 40k | heartbeat wall | 80.0 | 76.7 (A-C) / 81.5 (A-D run) | - | - |

The A-D head run measured fsync at 41-54 ms against 32-34 ms in the base run, at a similar load
(4-7). This change touches no fsync, so the heartbeat difference in that run is host fsync
variance. At 40k receipts and 31 tickets, heartbeat wall medians were base 72.9 and head 75.4,
with fsync near 32 ms and journal write 13.9 falling to 10.7.

## Outcomes

- V1-2531, heartbeat and renew p99: MET. Outside-gate p99 falls 3-12x, and renew wall p99 falls
  from 722-743 to 374-406.
- V1-2531, claim gate hold: MET on the hold profile, where validation is halved and the claim lock
  hold falls 54-88 ms at 880 tickets. The load harness's 31-ticket store moves the claim hold p50
  by about 9 ms.
- V1-2537: widen and reap hold p99 is at most 198 ms on every head run, below the 250 ms target,
  and the `base file changed` declines are gone. `TestPSRV0247_UnsettledBaseFileSurvivesALinkCountChange`
  shows the moved re-check stays bound to the base snapshot's bytes. This is MET with small n.
- V1-2539: NOT MET. The claim gate-wait p99 is 1.7 s against the 500 ms target in CAL-V0-396.
  The harness starts 14 claims together, so the wait is FIFO serialization and would need a claim
  hold near 36 ms. The fsync and commit floor alone is about 70 ms. No claim work moved out of the
  lock, so the "re-validated under the lock" test does not apply.
- V1-2203: NOT MET, NEEDS-OWNER. The heartbeat median is 67-81 ms against the 50 ms target. About
  32-54 ms of that is fsync, which decision 0549 F2 keeps uncoalesced; reaching the target needs
  an owner decision on staging-directory sync coalescing, with a crash-matrix proof.

Known unrelated failure: `TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit` fails on base and
head on WSL2 (V1-2554 / V1-2563).
