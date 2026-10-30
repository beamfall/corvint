# Decision 0075 — Measure observed snapshot hits separately

Date: 2026-09-05. Status: accepted bounded measurement contract (delegated call; corrected falsifier passes).
Authority: Russell Lewis's wave-1 instruction. Owner-authored coordinating record.

## Contract and preregistration

TCP-V0-021 gives the retrieval bench one materialized snapshot per repository and
base commit, registers inputs before retrieval, observes the context loader's
actual hit, and reports cold and hit latency separately. The optional diagnostic
is on stderr; packet bytes remain unchanged. Every cold/warm pair must have
identical ranking and abstention; unknown cache state refuses the measurement.
Cancellation restores hidden scratch snapshots and terminates owned descendants.
Read-only review found a registration/output alias overwrite risk; the repair
refuses normalized, hardlink and symlink aliases before retrieval or writes and
rechecks before publication. Seven alias preservation witnesses pass.

The full 106-sample v2_code2test run uses all five ladder arms, default gob and
default ranking. The falsifier is observed-hit context p50 below 100 ms, with
every paired result identical. Section-3 ranking baseline is recall@5/10/20
.225/.356/.485. Inputs, folds (49 A/57 B), candidate and benchmark hashes are
registered under `benchmarks/results/next-gen-wave1-2026-09-05/` before execution.
The repaired benchmark is frozen at SHA-256
`654ad7f6998e669ad45be40a8a54df33d44f36c0fc9795c00da8c17708b163fc`.
The trace-capable context binary is frozen at SHA-256
`248934235839abbd6578b34f8861189cb6ed1200b29580fd4d8f330266844199`.
This is development-corpus performance evidence, not a held-out retrieval claim.

## Result and rollback

The first registered full106 run refuses at sample 38 because cold and cached
results differ. No partial retrieval/latency report is published: cold/hit
distributions, recall and per-arm CPU are NOT_PRODUCED. Thirty-seven preceding
pairs passed, but their rows were not retained by the fail-closed benchmark.
Whole-run wall/CPU/RSS cannot substitute for per-arm observations.
Decision 0079 repairs cold import completeness and deterministic test evidence;
a separate corrected-tree campaign is registered under
`benchmarks/results/nextgen-wave1-repair-2026-09-05/L3/`.
The corrected candidate (`3f1e6998f96c2b7086ad9836a904d62878769b57dea6250b196e1f4a6e3ece1f`)
passes all 106 cold/hit pairs with 106 observed hits and zero context errors.
Hit min/p50/p95/max: 44.148/81.485/214.431/658.170 ms; cold:
69.386/214.884/944.343/1950.070 ms. The <100 ms hit-p50 falsifier PASSES;
p95 remains 214.431 ms. Current-default code2test recall@5/10/20 is
.287736/.399371/.511635, MRR .205416. Do not attribute this corrected-default
development ranking to the measurement tool or infer held-out performance.
Accept TCP-V0-021 as the bounded benchmark contract; no production snapshot
format or ranking experiment is promoted by this decision.
The original failure is preserved. Benchmark package tests, exact
TCP-V0-021 subtest claim, cancellation witnesses and five checks pass under
verified nice 15. Per-arm CPU is unavailable in the frozen benchmark and will
remain NOT_PRODUCED; whole-run CPU/RSS may be recorded without inferring per-arm
values. Omit `--snapshot-latency` to restore the existing single-pass runner.

Evidence location (2026-09-05): wave-1 paths above are logical member paths in the frozen, byte-exact evidence bundle. See `benchmarks/results/NEXTGEN-WAVE1-ARCHIVES.md` for manifest verification and safe restoration; packaging did not rerun or recompute the benchmark.
