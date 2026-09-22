# Task-context lookup preparation — development measurement, 2026-09-06

Fable 5.1 implemented the consumer optimization; independent Opus 5 review found no HIGH/MED
issue. This preserves TCP-V0-010/015's lookup semantics. It changes neither format selection nor
integrity verification. Analyzer schema 10 conservatively invalidates older experimental packs
under IDX-SNAP-V0-017's source-audit policy, despite unchanged extraction and encoding.

The pinned corpus is Corvint commit `0a227a94df6ad02e35c01965d05c46be9b367905`, tree
`d02007dd61665b525f4776be393467c8a9482419`, with 2,544 tracked paths. Both binaries were built with
Go 1.27.0; their hashes, workloads, machine, load and frozen configuration are in the two
`*-summary.json` files. The candidate binary precedes only final test/documentation edits.

Each task/format/arm has 100 measured fresh CLI processes and three excluded warmups. Arms
alternate first position per ordinal. The command is `corvint --root CORPUS context --task TASK
--limit 20`, with `--subject internal/contextindex/snapshot.go` on the subject task. Both
executables' snapshots were explicitly primed before timing. The default campaign ran first;
all gob files were then held outside the private corpus for the pack campaign and restored after
it. Pack's observed `hit=true` therefore cannot hide a gob fallback. The diagnostic uses
`CORVINT_BENCH_SNAPSHOT_TRACE=1`; pack additionally sets `CORVINT_SNAPSHOT_FORMAT=pack`.

| Task | Default p95, before → after (ms) | Pack p95, before → after (ms) |
|---|---:|---:|
| `snapshot demux split` | 206.495 → 143.603 | 160.769 → 107.286 |
| ``explain `LoadContextSnapshot` `` | 195.165 → 130.621 | 167.770 → 104.999 |
| `zzCorvintNoSuchTerm8472` | 206.415 → 131.648 | 161.084 → 97.560 |
| `explain snapshot storage retrieval`, with subject | 225.149 → 225.549 | 155.662 → 131.150 |

All 1,600 measured calls succeeded with complete captures and observed snapshot hits. All 800
paired full stdout comparisons were byte-identical. All 48 warmups succeeded. `samples.csv`
retains every measured and warmup row, CPU, peak RSS (bytes), output size/hash and outcome;
summaries retain p50/p95/p99/max, failure counts, binary identity and corpus-state checks.
No changed Git state or `.corvint` name/size/mtime listing was observed during either campaign.

These are warm OS cache measurements including bounded-supervisor wall overhead. A separate
20-process no-op calibration measured midpoint-median 2.386 ms and max 4.206 ms; nothing is
subtracted. This calibration median is not the campaigns' nearest-rank p50; all 20 values are
retained in `noop-calibration-ms.json`.
The machine was shared and loaded (start load averages: default 19.52/42.57/30.23; pack
10.12/27.98/26.40). Cross-campaign default/pack comparisons are not controlled format selection.
The subject task shows no default improvement and noisy tails. Peak RSS is not a heap allocation
measurement. Cold disk, faults, physical I/O, 100K/1M scale, SQLite comparison and DNIP promotion
remain NOT_RUN; fastest/instant and absolute target compliance are not established.

The baseline CPU profile assigned 1.31 s of its 1.42 s sampled benchmark caller stack to
`testRows`, including `nameTokens` and `newTestLinker`. Pack verification also consumed CPU
outside that caller stack. The change uses exact ASCII tokenization, reuses declared-name
tokens and tracked paths per invocation, indexes mirrored candidates by stem/role, and collects
only task-eligible definition keys while preserving every symbol and definer count.

Focused source-only scratch runs over 20 tasks reported full before/after and build/gob/pack
packet parity. Existing fixture parity remains the reproducible regression gate; scratch means
are diagnostics, not the campaign statistics above. New tests compare the scanner with the old
regex on explicit and 5,000 seeded byte strings, counterpart lookup with a complete scan, and
50/51-symbol boundaries with duplicate declarations. Existing race/concurrent reads passed.

The private original evidence directory is `/private/tmp/corvint-claude-storage-20260906/`: complete
stdout/stderr, raw JSONL, CPU/allocation profiles, frozen plan, Claude transcripts and the
measurement harness. Its supervisor interruption/timeout and statistics tests passed (39 tests),
including descendants in separate process groups. The checked-in samples and summaries preserve
observations if that temporary directory expires; complete output bodies and executables do not
ship in this repository. Independent reproduction must build its own binaries and freeze its
own hashes; it must retain failures and must not reuse these numbers as fresh observations.

Rollback: revert the consumer changes and audit/schema pins, then explicitly rebuild snapshots
if desired. Read commands retain their existing fallback and never repair stored data.

Review dispositions: the final independent Opus review again found no HIGH/MED issue. Spec-side
traceability now lists the three new regressions. The suggested redundant empty-relation guard
was declined: current `pairRelation` exhaustively guarantees a nonempty relation for exact stem
and opposite role, and the differential test guards that equivalence. No future API is assumed.
The extra per-invocation token retention was measured in source-only 20-iteration diagnostics:
pack allocated 39.46 → 27.49 MB/op and gob 287.6 → 275.6 MB/op; these are not peak RSS or campaign
statistics. Original output is in the private Fable build transcript. Bare requirement-ID
subtest names follow existing schema tests and are accepted by the claim extractor.

The corpus path count, Go toolchain, pre-campaign gob removal/restoration and candidate production
source identity are parent-observed execution facts. They are not independently re-extractable
from the two summary files alone. The private priming receipts and tool outputs retain their
original witnesses; `baseline-index.json` and `candidate-index.json` retain priming receipts here.
The candidate production delta is `taskcontext.go` plus `analyzer_schema.go` at commit
`7ea1e2a`; the remaining files in that commit are tests and measurement/documentation records.
