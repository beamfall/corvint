# Preregistration: V1-2716 `corvint query` recall at precision (ContextBench, 2026-10-26)

Status: frozen before any tuning. This file and the split file beside it are committed first;
the build-log entry cites this commit. Nothing below is edited after tuning starts; a later
correction is a new dated section, never a rewrite.

## Target (unchanged from V1-2716 acceptance bullet 1)

On full ContextBench (Contextbench/ContextBench @ c2855792b006af41c67202d33883fb9d46362853,
`data/full.parquet`, 1132 scored rows, 66 repositories; exported row digest
`c9ea7da74497d2bc162e8534e59cb2d8303df9d0c4f7da640d09e63e34b4b427`, scored row ids
`f8bffb21d2ae0bf0d427e838bd7225cbb3c4db0490b8485b575571f27ced3c15`), the `corvint` arm of
`tools/retrieval-bench` (`corvint query --task <json> --limit 20`, ranked = confident results then
`context.widening` rows, distinct, packet order) MUST reach

- `cb_file_coverage` (file recall) >= the `bm25:ident` arm's `cb_file_coverage` on the same rows, and
- `cb_file_precision` >= 0.25,

on the held-out split AND on the full set. The dev split is reported but is not the criterion.

## Metrics (as implemented in `tools/retrieval-bench/contextbench.go`, `coveragePrecision`)

Per row: coverage = |answer ∩ gold| / |gold| (empty gold → 1.0); precision = |answer ∩ gold| /
|answer| (EMPTY answer → 1.0, the abstention edge rule). Arm metrics are the row means over all
scored rows. Both edge rules stay exactly as the bench computes them. The build-log entry also
reports, for disclosure only, precision over answered rows and the abstention share, so a reader
can see how much of any precision figure the edge rule supplies. `bm25:ident`'s 0.299 precision is
0.0537 from answered rows plus 0.2456 abstention share.

## Frozen split (repository level)

`benchmarks/results/v2716-contextbench-split-2026-10-26.json`: a repository is `dev` when
`int(sha256(utf8(repo)).hexdigest(), 16) % 2 == 0`, else `heldout`. dev = 30 repositories, 419
rows; heldout = 36 repositories, 713 rows. The held-out rows are not inspected, simulated on, or
used to choose any parameter until the final run. No row is filtered, re-weighted or excluded after
the fact; a Corvint error row counts as an empty answer exactly as the bench scores it.

## Baseline on each split (prior run, same binary as `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-26-v2716-v2717-query-recall-opencode-adoption.md`)

| split | rows | bm25:ident cov / prec / abstain | corvint (confident+widening, 20) cov / prec / abstain |
|---|---|---|---|
| dev | 419 | 0.3160 / 0.3783 / 0.3294 | 0.4160 / 0.0814 / 0.0024 |
| heldout | 713 | 0.3857 / 0.2528 / 0.1964 | 0.4943 / 0.0936 / 0.0126 |
| full | 1132 | 0.3599 / 0.2993 / 0.2456 | 0.4653 / 0.0890 / 0.0088 |

## Diagnosis that motivates the design (no tuning involved)

Over the 4540 gold files: 795 (17.5%) carry a suffix the immutable index does not admit
(`textSuffixes`, `internal/contextindex/index.go`): `.java` 239, `.h` 151, `.c` 142, `.hpp` 70,
`.pony` 28, `.cpp` 27, `.asciidoc` 21, `.pcss` 16, `.tpl` 15, `.cc` 11. `bm25:ident` finds 204 of
those; Corvint finds 7 (all by path). On admitted suffixes Corvint already finds more gold than
`bm25:ident` (1540 vs 1010). Eight repositories (jackson, zstd, nlohmann/json, fmt, logstash,
ponyc, simdjson, jq) have Corvint recall near 0 for this reason alone. Query truncation to 2000
characters (250 rows) is not the main cause: on rows at or under 2000 characters Corvint's
widened list beats `bm25:ident` 0.457 to 0.322.

The second gap is ordering and cutoff: the whole 20-row list has recall 0.465 at precision 0.089;
the confident tier alone has recall 0.239 at precision 0.249. The target needs roughly 1.4 gold
hits in about 6 returned rows per answered row, so the gold must be ordered to the top and the
list cut where evidence stops.

## Design space (what may be built and tuned; nothing else)

1. Candidate generation: admit further source and prose suffixes to the index as searchable text
   with no symbol extractor, exactly as `IDX-SNAP-V0-108`/`-109` did for `.mts`, `.cts` and the
   template suffixes (analyzer schema bump; `UnsupportedSuffixCount` falls for trees that carry
   them). Candidate list: C family (`.c`, `.h`, `.cc`, `.cpp`, `.cxx`, `.hh`, `.hpp`, `.hxx`),
   `.java`, `.scala`, `.pony`, AsciiDoc (`.adoc`, `.asciidoc`), CSS family (`.css`, `.scss`,
   `.less`, `.pcss`) and `.tpl`. Choosing which of these to admit is not tuned on the benchmark:
   the list is fixed here, before measurement, as the suffixes with gold in the diagnosis plus
   their obvious siblings.
2. Ordering: the standalone query's `widening` rows are ordered by reciprocal-rank fusion of the
   structural TaskContext order and the lexical identifier channel (BM25 over the term table,
   whole-identifier field weighted), with project authority rows kept first (invariant 3). Every
   row explains its inclusion (invariant 1): slot, rank in each fused list, and the identifiers
   that matched.
3. Cutoff: the widening quota is not a fixed `limit - confident`; rows past an evidence cutoff are
   withheld and the receipt states the withheld count and the reason (`coverage.uncertainty`,
   invariant 2). Cutoff forms allowed: a relative score threshold against the top fused score, a
   score-gap rule, and a hard cap on widened rows. A row with no lexical and no structural
   evidence is never returned.

Parameters that may be tuned, on dev rows only: the RRF constant, the relative-score threshold,
the gap ratio, the widened-row cap, and whether the whole-identifier field's weight is 1 or 2.
Each dev simulation and run is logged in the build-log entry with its numbers. The parameter
set that is run on held-out and full is chosen once, from dev, before those runs start.

## Reporting rules

The build-log entry reports dev, held-out and full numbers for `bm25:ident` and for the new
`corvint` arm, with the edge-rule disclosure above, and names PASS or FAIL against the criterion
as written. If the criterion is not met, the entry reports the best honest frontier reached and
the measured reason; the criterion, the split, and the metric are not changed. Frozen retrieval
evaluations (`benchmarks/runner --enforce-v4`) and the GPK conformance goldens are run or their
non-run reason is stated exactly.

## Amendment 2026-10-26 (after dev tuning, before any held-out or full run)

This section is appended, not a rewrite. It is recorded before the first held-out row is run.

### Dev outcome of the design space as preregistered

The design space above was simulated offline on the 419 dev rows from one permissive Phase-1
run (`k=60`, floor 0, gap 0, cap 50; per-row structural and lexical ranks exported) over
`k ∈ {10,20,30,60,100}`, cap `∈ {0..20}`, floor and gap `∈ {0,50,60,70,80,90}` (1,980 configs).
No configuration other than cap 0 (confident rows only: coverage 0.1943, precision 0.3315)
reached precision ≥ 0.25. Every widening configuration has abstention 0, because the cutoff
forms above never withhold the first fused row, and the best coverage at or above the
`bm25:ident` dev coverage (0.3160) was `k=10 floor=90 cap=5`: coverage 0.3174, precision 0.1619.
Against the criterion as written, the preregistered design FAILS on dev; the frontier is in the
build-log entry.

### Change to the design space (one addition)

Identifier gate: a fused row is carried only when its document names at least one task identifier
by exact spelling (the identifiers the lexical channel searched for). Rows that rest on prose
terms or on the structural ranking alone are withheld and counted in `widening.withheld`, and a
task that names no identifier widens to nothing and says so in `widening.uncertainty`. This is a
tightening of rule 3 ("a row with no lexical and no structural evidence is never returned") to
"no identifier evidence"; it adds no evidence source, no learned weight and no new tunable.

Dev simulation with the gate (same export, same metric): `k=10 floor=50 gap=70 cap=10` gives
coverage 0.3461, precision 0.2602, abstention 0.0979 (bm25:ident dev 0.3160 / 0.3783 / 0.3294).
Neighbouring configurations were within 0.005 on both axes (`k=10 floor=0 gap=70 cap=10`:
0.3488 / 0.2596; `k=60 floor=80 cap=15`: 0.3508 / 0.2535). The offline simulation orders rows by
fused score alone for `k≠60` (authority-first ordering is not recoverable from the export), so the
dev numbers below come from a rerun of the runtime, not from the simulation.

### Constants frozen for the held-out and full runs

`k=10`, relative floor 50 %, gap ratio 70 %, cap 10, lexical channel weight 1 (the
`lexical_weight_percent` member is fixed at 100 and was never varied), whole-identifier field
weight left at the implemented value of 2 (the weight-1 alternative was not run). The result on
held-out and full is reported once, with the gate disclosed as a post-dev amendment; the
criterion, split and metric are unchanged. The outcome against the preregistered design is FAIL
on dev regardless of what the amended design reaches.

## Outcome 2026-10-26

Runtime rerun with the frozen constants above, one pass per split, same metric and same
`bm25:ident` rows (receipt `benchmarks/results/v2716-query-fusion-contextbench-2026-10-26.json`):

| split | rows | corvint coverage / precision / abstention | bm25:ident coverage / precision / abstention | criterion |
| --- | --- | --- | --- | --- |
| dev | 419 | 0.3476 / 0.2568 / 0.0955 | 0.3160 / 0.3783 / 0.3294 | PASS |
| held-out | 713 | 0.4083 / 0.1909 / 0.0449 | 0.3857 / 0.2528 / 0.1964 | FAIL (precision < 0.25) |
| full | 1132 | 0.3858 / 0.2153 / 0.0636 | 0.3599 / 0.2993 / 0.2456 | FAIL (precision < 0.25) |

Verdict against the preregistered criterion (held-out AND full): **FAIL**. Coverage exceeds
`bm25:ident` on every split; precision does not reach 0.25 outside dev. The measured reason is
that the confident tier alone is already below 0.25 on held-out (0.2086) and full (0.2309), so
no widening cutoff can recover the floor there; `bm25:ident` reaches its precision through
abstention scored as 1 (precision over answered rows 0.0702 / 0.0711, against corvint's 0.1529 /
0.1620); every evaluated widening cutoff lowers precision further, so none of the evaluated
configurations reaches 0.25 outside dev. A post-hoc grid of 648 tightenings of the frozen cutoff (reported only, not adopted) has
0 configurations meeting the criterion on held-out or full; its maximum precision is 0.2164 on
held-out and 0.2369 on full. The criterion, split, metric and constants are unchanged.

Corrections to the Amendment, recorded here rather than by editing it: (1) the "whole-identifier
field weight ... implemented value of 2" misread the field index of the third BM25 field for a
weight; the field carries the standard term weight `idf*(k1+1)/(1+k1)` and no multiplier, and the
preregistered weight-1/weight-2 alternative was never implemented or run; (2) the gate as
implemented and measured carries a row only when the lexical channel ranked it among its
candidates (the channel attaches the identifiers it searched for to the candidates it ranks), so a
row ranked by the task-context list alone is withheld even when its document names the identifier
beyond that window; GPK-V0-092 states the gate in this form.
