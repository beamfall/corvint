# Retrieval evaluation comparability V0

Owner: Russell Lewis
Date: 2026-09-12
Requirement prefix: `REC-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: AGENTS.md invariants 2 and 8; `learned-trace-admission-v0.md` `LTA-V0-001`;
`../../benchmarks/README.md` (first-observation rule); `benchmarks/manifest.json`.

## Agent digest
- Claim: The offline retrieval benchmark reports dev and held-out splits beside the full set, recall at fixed byte budgets, and an explicit downstream-outcome slot.
- Status: proposed/experimental
- Exists: `internal/evalrepo` split, budget-yield and outcome helpers; `benchmarks/runner` `comparability` blocks, `-purpose` and the committed `benchmarks/eval-split-v0.json`.
- Blocked on: owner acceptance; every v0 held-out row was observed during development, so no generalization claim exists.
- Read next: Requirements; Non-goals and failure modes.

## Human intent and scope

The frozen retrieval numbers from `go run ./benchmarks/runner` are scored on the same rows that
were used to develop ranking, cannot be read as a yield-for-bytes curve, and say nothing about
whether a downstream task succeeded. Those gaps make the numbers impossible to compare with
published retrieval baselines. This contract adds a deterministic dev/held-out assignment, a fixed
byte-budget recall curve, and an outcome slot that records absence explicitly. The owning harness
is `benchmarks/runner` over `internal/evalrepo`. Existing frozen metrics, the V4 release checks
and the `corvint eval` report bytes are unchanged.

## Requirements

- `REC-V0-001`: A golden row's split MUST be derived from its `id` alone: take SHA-256 of the
  bytes `atlas-eval-split-v0` (the historical key; renaming it would reassign rows), a NUL byte and the id; read the first 8 digest bytes as a
  big-endian unsigned integer; the row is `heldout` when that integer modulo 5 is 0 and `dev`
  otherwise. Row position, file, partition label and content MUST NOT affect the assignment. The
  existing `partition` field keeps its first-observation meaning; `split` is a separate axis.
- `REC-V0-002`: `benchmarks/eval-split-v0.json` MUST list, per release corpus path and in corpus
  order, each row's `id`, `split` and the SHA-256 of its canonical JSON. The runner MUST refuse to
  start when the manifest's SHA-256 differs from the registered digest, and MUST refuse a corpus
  whose rows differ from the manifest in count, order, id, split or row digest. A release-manifest
  corpus absent from the split manifest MUST refuse; any other corpus is reported `unregistered`.
  `-write-split-manifest FILE` regenerates the manifest and prints its digest; changing it is a
  reviewed change to the registered constant.
- `REC-V0-003`: Under the default `score` purpose every repository record and the top-level report
  MUST carry a `comparability` block whose `splits` holds `full`, `dev` and `heldout` metrics side
  by side: case, must-read, critical, exclusion, top-5, abstention and epistemic counts; the
  matching ratios; serialized-byte precision; and `budget_yield`. The `full` split MUST equal the
  existing aggregate for every shared metric. A ratio over a zero denominator MUST be `null`, not
  a conventional 0 or 1. `corvint eval` output MUST stay byte-identical (`LTA-V0-001`).
- `REC-V0-004`: A `tune` purpose MUST score, audit, baseline and report dev rows only and record
  `heldout_rows_withheld`; its `splits` holds `full` over dev rows only. The scorer MUST return
  `ErrHeldoutRowInTuning` before running any query when a tuning run is handed a held-out row. A
  tune run adds the failing check `score_purpose` and is never release-ready. Whole-corpus
  integrity checks (source-proof validation, digests, fixture contamination) are not scoring and
  still read every row. Any future calibration or tuning entry point MUST use this purpose.
- `REC-V0-005`: `budget_yield` MUST use the ladder 512, 1024, 2048, 4096, 8192 and 16384 bytes. For
  each case, walk the packet's results in packet order accumulating each result's weighted JSON
  bytes (the precision byte measure). A `must_include` selector is hit at budget B when the
  cumulative bytes through its first occurrence are at most B. `recall_at_budget[B]` is summed hits
  at B over summed `must_read_total`; `area` is the arithmetic mean of the unrounded recall values
  over the ladder, rounded to six places. The curve is a prefix of the one packet produced at the
  case's own `budget_bytes`; it does not rerun the product at each budget.
- `REC-V0-006`: Every scored case record MUST carry `downstream_outcome`, either exactly
  `{"state":"not-recorded"}` or exactly `{"state":"recorded","outcome":"passed"|"failed",
  "evidence":"<non-empty>"}`. The runner MUST validate every slot before writing a report and count
  states in `downstream_outcomes`. This harness observes no task outcome, so it emits only
  `not-recorded`; it MUST NOT infer an outcome from retrieval metrics.
- `REC-V0-007`: The v0 split manifest MUST label held-out rows `previously-observed`. Every v0 row
  had been read during development before assignment, so held-out numbers are a stable partition
  for later comparison, not evidence of generalization. A row first observed after assignment still
  needs the `benchmarks/README.md` first-run registration to support a blind claim.

## Non-goals and failure modes

No product ranking, packet, contextindex schema or golden change; no token-denominated budgets; no
re-run at each budget; no producer of recorded outcomes; and no change to the external Agent
Retrieval Bench in `tools/retrieval-bench`, which keeps its own split labels. Adding, removing or
editing a release corpus row without regenerating the manifest refuses the run (`REC-V0-002`). A
modulus-5 hash gives an uneven per-corpus split (cobra has no held-out rows in v0); corpus-level
held-out ratios are therefore `null`, not certainty. Budget recall below full recall at 16384 bytes
means a must-read selector sat beyond 16 KiB of results, not that it was missed.

## Evidence and rollback

Acceptance evidence: focused `internal/evalrepo` and `benchmarks/runner` tests, plus a runner report
over the pinned release checkouts whose `full` split and aggregate match a report from the pre-change
tree (latency excluded), recorded in `docs/BUILD-LOG.md` on 2026-09-12. Rollback removes the
`comparability` blocks, the `-purpose` and `-write-split-manifest` flags, the outcome slot and the
split manifest; the aggregate, V4 checks and `corvint eval` report need no migration.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `REC-V0-001` | `internal/evalrepo` `CaseSplit` | `TestCaseSplitIsAHashOfTheRowIDNotItsPosition` |
| `REC-V0-002` | `internal/evalrepo` `SplitManifest`; `benchmarks/runner` `loadSplitManifest` | `TestSplitManifestRefusesDriftedCorpus`, `TestCommittedSplitManifestRegistersReleaseCorpora` |
| `REC-V0-003` | `internal/evalrepo` `EvaluateComparable`, `MergeSplitMetrics`; `benchmarks/runner` `comparabilityReport` | `TestBudgetYieldCurveAreaAndMerge`; 2026-09-12 BUILD-LOG run |
| `REC-V0-004` | `internal/evalrepo` `requireReadable`; `benchmarks/runner` `readableCases` | `TestTuningPurposeRefusesHeldOutRows`, `TestCommittedSplitManifestRegistersReleaseCorpora` |
| `REC-V0-005` | `internal/evalrepo` `BudgetYield` | `TestBudgetYieldCurveAreaAndMerge` |
| `REC-V0-006` | `internal/evalrepo` `ValidateDownstreamOutcome` | `TestDownstreamOutcomeSlotValidation` |
| `REC-V0-007` | `benchmarks/eval-split-v0.json` `heldout_provenance` | `TestCommittedSplitManifestRegistersReleaseCorpora` |
