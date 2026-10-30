# Decision 0073 — Blob-fact reconstruction misses the latency target

Date: 2026-09-05. Status: accepted disposition (delegated call; falsified).
Authority: Russell Lewis's wave-1 instruction. Owner-authored coordinating record.

## Registered result

L1's actual Corvint tree changed twenty existing source files after priming prior
blob facts. Thirty fresh-process paired observations at verified nice 15 produced:

| Path | Min | p50 | p95 | Max |
|---|---:|---:|---:|---:|
| Full build | 502.94 ms | 530.25 ms | 566.01 ms | 579.31 ms |
| JSON blob facts | 657.31 ms | 680.97 ms | 735.51 ms | 776.59 ms |

The <=120 ms target FAILS. Shards are 28.4% slower at p50; the full build matches
the handoff's 480–560 ms baseline. No other owned benchmark/gate ran; observed
one-minute load stayed at most 8.10. Snapshot read-only inventory passes.
Reusing parsed facts while reconstructing every table does not earn its cost.

The 20-task × subject/no-subject × full/shard/gob matrix also FAILS: 15 of 120
packets differ (3 shard/full, 12 gob/full). Paths and ranks match on these tasks,
but tied test explanations differ and eight gob cells have additional withheld
test candidates. Independent investigation identifies map-order tie labels and
cold import narrowing that omits edges now required by TCP-V0-015. These existing
defects do not waive L1's first failure; a separately registered repair validation
will have its own result. The private historical task script was unavailable;
this run preserves an explicit 20-task actual-tree matrix of the same dimensions.

Original source/binary registration, raw timing, all packets and the difference
classification are under `benchmarks/results/nextgen-wave1-2026-09-05/L1/`.
The frozen binary begins SHA-256 `590c5155a6cf`; the full digest is in registration.

## Disposition and boundaries

Keep IDX-SNAP-V0-016 proposed and disabled. No latency or shard-format promotion
follows. The prototype verifies blob identities, bounds decoded input and reader
concurrency, refuses symlinks/FIFOs, and confines publication through descriptors;
unit, race and synthetic CLI witnesses pass. Full-path keying conservatively
does not reuse renames; bounded on-disk shard retention is not delivered.

Staging shares L2's reviewed analyzer identity for both readers and writers.
The first staging run caught their mismatched keys; correcting the writer and
re-running registered shard/analyzer/pack witnesses passes. This integration
repair leaves original measured binaries/results unchanged. The next performance
hypothesis must reuse compiled sections or prove another lower-cost mechanism,
rather than assuming per-blob JSON caching is faster. Unset `CORVINT_INDEX_SHARDS`
to bypass the prototype; explicit removal of derived `blobs/` is its rollback.

Evidence location (2026-09-05): wave-1 paths above are logical member paths in the frozen, byte-exact evidence bundle. See `benchmarks/results/NEXTGEN-WAVE1-ARCHIVES.md` for manifest verification and safe restoration; packaging did not rerun or recompute the benchmark.
