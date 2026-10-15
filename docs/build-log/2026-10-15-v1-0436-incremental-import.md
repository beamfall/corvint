# 2026-10-15 V1-0436: incremental import sequence (CAL-V0-018)

`corvint-tasks import` no longer re-runs the whole-store `transaction.Model` for every
`IMPORT_APPLY` batch. Before this change each batch re-validated every stored ticket
(`validateInput`) and re-measured the whole inventory (`CheckCapacity`), so a first import was
quadratic in the export size (`docs/build-log/2026-09-27-corvint-tasks-import-cost.md`: Model was
112 s of 208 s CPU on the 2,894-item Beamfall export). This entry ports the qualified but unmerged
implementation from `claude/v1-0436-import-source-2026-09-28` (commits 6f8244753, 441a3a843,
75a938e24 and 30e306ec0 over merge-base 2e281305c) onto origin/main 3337f5f6d as one change.
No journal, receipt or wire format changes. No requirement text changes: CAL-V0-018 already
requires one full audit plus per-batch work on the head and the posted files.

## Contract

- The public `transaction.Model` is unchanged and keeps full untrusted-input validation.
- `transaction.ImportSequence` is import-only and process-local. `NewImportSequence` validates the
  store once (the one full audit) and owns the canonical ticket state, ID and case-fold indexes,
  receipt identity and a private copy of the physical inventory. `Prepare` runs the ordinary model
  over that cached state and returns a detached `ImportBatch` (no generic `Plan`, no mutable final
  inventory). `Advance` accepts exactly the pending batch once, after a successful publication.
- Any refusal, failed apply, cancellation or session exit closes the sequence. The next command
  performs pending redo and a fresh audit; nothing is carried across processes.
- Capacity is carried, not recomputed: per-entry archive arithmetic (`archive.EntryCost`, the
  CAL-V0-117 `FileSetCost` prefix and `MeasureManifestEncodingWith`) updates only the touched
  files with checked add and subtract, at every existing publication prefix. Capacity forks share
  the owned metadata with bounded undo maps and roll back. Request-shard reserve probes use
  carried per-shard hints instead of scanning the inventory.
- Before a fresh import receipt links, every posted ticket's projection must match its preimage
  digest, including exact absence; otherwise the batch refuses `INTENT_DIVERGED` and commits no
  receipt. Committed-but-unapplied receipts still recover through unchanged redo before the audit.

## Port decisions

- origin/main gained the CAL-V0-117 summarized inventory and `archive.FileSetCost` (V1-0645) after
  the source branch was cut. The source's own `archive.FileEncoding` / `MeasureManifestTotals`
  API was dropped; `internal/tasks/archive` is unchanged and the carried totals use main's API.
- `NewImportSequence` refuses a summarized inventory (`MALFORMED`, "complete inventory
  required"): the carried totals are seeded from per-file metadata a summarized inventory elides.
  Import already reads a complete inventory, so no current caller reaches the refusal.
- An extra archive-file-count guard tried during the port was removed so the sequence and the
  full path refuse with identical errors at the exact limit.
- Main's later import checks (external reviews, operator note, attached evidence, obligations,
  know-how, recorded-at clock, locked journal reader, author exclusion in `Result`) are inherited
  because the sequence calls the same `importChain` and model code.

## Evidence

Criterion 1 (per-batch model time proportional to the batch, not the store).
`BenchmarkCALV0018_ImportBatchModel` posts 7 new tickets per batch against synthetic stores of
500, 1,500 and 3,000 tickets. The `full` arm is the public Model, the per-batch path before this
change; the `sequence` arm is `Prepare` plus `Advance`. Host: 12 CPUs, load 37 to 61 throughout.

| Stored tickets | full B/op | full allocs/op | sequence B/op | sequence allocs/op |
|---|---|---|---|---|
| 500 | 77.08 MB | 672,418 | 2.06 MB | 18,465 to 18,497 |
| 1,500 | 225.9 MB | 1,970,213 to 1,970,229 | 2.06 MB | 18,471 to 18,498 |
| 3,000 | 449.3 MB | 3,917,323 to 3,917,342 | 2.06 MB | 18,483 to 18,500 |

ns/op are reported but are load-dominated: full 0.12 to 2.55 s/op growing with size, sequence
4.8 to 11.5 ms/op with no trend. CPU profiles restricted to the measured closure
(`-focus 'ImportBatchModel\.func'`):

| Stored tickets | full: Model cum per 5 batches | sequence: model cum per 200 batches |
|---|---|---|
| 500 | 0.30 s (validateInput 0.11, checkCapacity 0.18) | 0.30 s (checkCapacity 0.07) |
| 1,500 | 1.06 s (0.48, 0.57) | 0.32 s (0.04) |
| 3,000 | 1.89 s (0.78, 1.08) | 0.31 s (0.09) |

Per batch that is about 60, 212 and 378 ms for the full path and a flat 1.5 ms for the sequence.
The sequence profiles also show `validateInput` at 0.02, 0.18 and 0.35 s; `pprof -focus
validateInput` attributes all of it to `NewImportSequence`, the one-time seed audit inside the
sub-benchmark closure, and none to `Prepare`.

Tests. Transaction: `TestCALV0018_SequenceParityAndOwnership`, `SequenceRefusalParity`,
`SequenceInvalidation`, `SequenceBatchGrowth`, `SequenceCapacityPrefixes`,
`ReserveProbeOwnership`. Store: `TestCALV0018_ImportPublicationFailureRestarts`,
`ImportCancellationAndProjectionDrift`, plus the existing `LaterBatchesCheckTheirHeadAndPosts`
and the CTS-V0-003 import tests. Focused run `-run 'CALV0018|CTSV0003'` over store and
transaction: 16 PASS, 0 FAIL (load 37 to 42). The transaction package passes in full; the
`internal/tasks/cli` import, cutover and CAL-V0-020 tests pass.

Mutations, each reverted:
- `importCost.replace` ignores the prior entry (totals only grow): `SequenceCapacityPrefixes`,
  `ReserveProbeOwnership` and `SequenceParityAndOwnership` FAIL (3 of 6 transaction CAL-V0-018
  tests).
- The import publication skips the fresh-receipt preimage guard:
  `ImportCancellationAndProjectionDrift` FAILS ("fresh receipt linked after failed guard").

Criterion 2 (the 2,894-item Beamfall export on a host with load below the CPU count) is NOT_RUN.
The shared 12-CPU host stayed at load 35 to 61 for the whole lane, and the export is not present
locally: the pinned input (SHA-256 `be368b0524acacf375dcd28bf104646b691f9a88382f892ac58c6556c07c3d01`,
14,102,044 bytes, 2,894 records, 1,450 edges) was not found under `/private/tmp`, and its earlier
location `/tmp/corvint-1-0-orchestrator-20260928/import-beamfall` no longer exists. Beamfall owns the
exporter. The source branch recorded an early matched pair on that input at quieter load (baseline
2e281305c 177.97 s, prototype 441a3a843 60.60 s, every sampled load below 12); it predates the
cancellation and preimage guards and is not evidence for this source.

The prepared run is `script/measure-import-cal018.sh EXPORT OUTDIR`. It verifies the export digest,
builds `corvint-tasks` from HEAD, initializes a fresh ROADMAP fixture store, reads the load before
and after, times the import with `/usr/bin/time -p`, runs `receipt audit`, and checks that an
immediate reimport leaves every store byte unchanged. It prints QUALIFYING PASS only for the
pinned digest, every load reading below the CPU count and a wall time under 300 s; expected
outcome is 414 receipts, head 415, CONSISTENT/AGREES. `--allow-loaded` runs the procedure as a
NONQUALIFYING smoke check: on a 30-record synthetic export at load 50 it wrote 30 tickets in
5 receipts, audit CONSISTENT/AGREES, reimport 30 unchanged with store bytes unchanged.

## Rollback

Revert the change. Import then calls the public Model per batch again (correct, quadratic in the
export size). No durable state is migrated, so no store needs repair.
