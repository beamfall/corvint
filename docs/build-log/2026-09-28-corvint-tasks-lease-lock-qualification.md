# CAL-V0-026 qualified lease lock hold

CAL-V0-026 is MET for the measured macOS host with 12 logical CPUs, Go 1.27.1 and
`GOMAXPROCS=2`. The owner asked to finish the remaining qualification after the implementation,
independent review and ten selected checks had passed. This entry supersedes the pending
qualification in `2026-09-28-corvint-tasks-lease-lock-cost.md`; S8 pack derivation remains experimental
and explicitly enabled, and CAL-V0-018 remains partial.

## Frozen measurement

Source and final source were both `5061de48d2854415c4fe8054c67033f2ff11ffa3`, the sealed implementation.
The native candidate binary's Go build metadata names that commit, reports `vcs.modified=false`, and
uses Go 1.27.1. The baseline binary names sealed S8 `40f7d7e9b4c65da5660e42261c1c6e4c4ae31fe8`.
No source edit or commit occurred during either measurement attempt.

The synthetic fixture contained 3,000 tickets. Twenty successful claim/renew/release sequences
produced 20 lock samples per verb. Claim and renew each stayed below the required 500 ms p95:

| Command | p95 lock hold |
|---|---:|
| claim | 121.136 ms |
| renew | 104.774 ms |

The run lasted from 2026-09-28 10:25:55.644 UTC to 10:30:48.389 UTC. All 146 host-load observations,
sampled about every two seconds, are retained, including fixture construction, the full command
loop, cold CLI invocations and final cleanup. No host or command sample was excluded. One-minute
load ranged from 5.574 to 9.350; five-minute load peaked at 9.925 and fifteen-minute load at 8.881.
Every observed load average was below the 12-CPU count. The test passed and its final store audit
was consistent. The raw report and host samples are in `evidence/cal26/qualified-measurement.json`
and `evidence/cal26/qualified-host.json`; `evidence/cal26/qualification.json` records the
recalculation from raw samples and both binary digests/build identities.

`GOMAXPROCS=2` constrained the benchmark and both native CLI binaries to two Go execution threads.
This is the recorded qualification configuration. The installed/default parallelism was not changed,
and this result does not establish a default-parallelism low-load qualification. A preceding run with
no explicit parallelism setting measured claim 119.681 ms and renew 100.099 ms, but load exceeded
12 during the conservative command interval. Its complete evidence remains in
`evidence/cal26/default-parallelism-measurement.json` and `evidence/cal26/default-parallelism-host.json`;
it is unqualified. The earlier failed quiet-host wait and overloaded implementation experiment are
also retained in the previous entry.

## Complete native commands

The same fixture then ran each verb in a fresh native process, with an empty process-local cache.
Both revisions used `GOMAXPROCS=2`. These are one sample per command/revision, so they demonstrate
complete-command benefit in this run without estimating a general speedup distribution.

| Command | S8 baseline | Candidate |
|---|---:|---:|
| claim | 2,988.526 ms | 2,320.818 ms |
| renew | 2,946.733 ms | 2,168.625 ms |
| release | 3,079.183 ms | 2,212.202 ms |

## Acceptance and limits

The implementation review passed the bounded critical sections for every lease verb, guarded
recovery, immutable process-local audit cache, and mutation detection with descriptor cleanup outside
the lock. Two findings were repaired and reviewed: historical request bytes consuming the live-record
budget, and recovery errors escaping the locked recheck. The full Tasks suite, vet and all eight
selected documentation/format checks passed at bind commit `1f78bd609ed195d989456d445d1cc3c0f04bee58`.
Its seal only renamed the CEM. This change contains documentation and measured evidence; it changes
no Go source. The qualifying run supplies the remaining numerical/load evidence for Gate B.

Cache reuse is process-local. A fresh CLI process still audits the store once; unchanged snapshots
can reuse a verified audit in one process. Linux source cross-compiled, but Linux runtime and
performance qualification remain NOT_RUN. Repository-wide `make gate` remains NOT_RUN under the
owner's scoped-work instruction. The independent standalone Tasks source-archive rebuild blocker
V1-0456 remains open; this qualification does not close it.

Rollback is the prior audited lease writer, as recorded in the implementation entry. There is no
store format migration. Pre-change query and impact receipts, affected-plan unknowns, the selected
documentation checks and the new CEM binding are retained by this qualification change's dogfood
workflow. Prior OCM/frontier unknowns are not upgraded by a performance measurement.

The required pre-edit dogfood pass had no diff and retained `git-diff-failed`,
`cem-map-not-produced`, OCM `exit-2`, `intent-scope-drift` and `map-unavailable` in
`cal26-qualification-initial.log`. The final binding uses the committed documentation diff.
