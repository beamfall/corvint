# Decision 0037 — the four release owner calls from decision 0036

Date: 2026-09-03. Status: accepted. Authority: repository owner, verbatim instructions "answer the
questions in question.md for me. use expert if needed" and, on reading this record, "accepted,
commit it all" (2026-09-03). The answers were drafted by a
three-expert panel (`performance-scalability-engineer`, `wire-contract-engineer`, and
`retrieval-evaluation-methodologist`, the last added to the catalogue for this call) and every
citation below was re-opened by the drafting session before it was recorded.

## 1. Packet 5: the cold path stays binding; a snapshot-present corpus is added

The preregistration in `conformance/perf-v0/manifest.json` is not rewritten to warm. A second
preregistered corpus, `beamfall-snapshot-present`, runs `corvint index` once before the timed
samples, outside the 100 measured samples, and both corpora are reported. GPK-V0-017(b) is met on
the cold corpus until an automatic index refresh (the session-start hook decision 0032 leaves
undecided) ships and is dogfooded; only then may a dated spec edit name the snapshot-present corpus
as binding. The formal `NOT_RUN` result imported at
`conformance/perf-v0/results/packet-5-current-pin-requalification-retry-2026-09-11-formal/report.json`
stays untouched (decision 0012 R2). Dated amendment 2026-09-11 under decision 0086:
this selects the newer immutable `NOT_RUN` receipt; the 2026-08-31 receipt also stays untouched. The `indexNote` sentence that says no persisted index exists is
false since decision 0032 and is replaced when the corpus is added.

Why: no code path in `cmd/corvint` or `internal/gokernel` runs `index` for a user today, so a
warm-only number would describe a path no user experiences. The 730 ms harness figure and the
355 ms live figure diverge because the harness materialises a fresh temporary repository; neither
is final until the full protocol (≥100 fresh-process samples, five discarded warmups) reruns on both
corpora. That rerun is a follow-up to this record, not part of it.

## 2. `MinPacketBytes` is raised to a derived floor

`MinPacketBytes` (`internal/contextindex/receipt.go`) and `MIN_PACKET_BYTES`
(`src/context_corvint_learning.py`) move from 1,024 to the smallest value at which the worst-case
mandatory abstaining envelope fits, rounded up to the next 64-byte boundary, with a test that
re-derives the worst case from the mandatory field set. The spec's range prose, the budget-boundary
sweep and any parity capture that pins the floor move in the same change. A budget floor that
cannot succeed for an entire abstention class is a contract defect, and today's refusal is a typed
error rather than an abstention, which is worse for a caller than uncertainty. No shipped consumer
passes 1,024 (the hook uses ≥4,096, the VS Code extension 262,144), so the change is not breaking
for them.

## 3. Blind-v4 stays sealed under a preregistered run condition

Blind-v4 is not run now. Condition, adopted verbatim: blind-v4 may be run exactly once, under
`--engine corvint`, only when a single committed configuration shows on the same run (a) blind-v3
critical misses ≤ 4/10 and recall ≥ 0.40; (b) development corpus 0/31 critical misses, recall 1.0,
byte-weighted precision ≥ 0.80 under both engines; (c) `v4_release.blocked_by` empty in
`benchmarks/results/v4-development-go.json`; and (d) the blind-v4 manifest digest registered in
`benchmarks/run.py`'s `FIRST_RUN_EVIDENCE` before the run so the result evaluates as an attested
first run rather than `unattested-heldout`. A run outside this condition is published as a failure
and blind-v4 is retired. Blind-v3, already observed, is the development signal for mechanism work.
Blind-v5 is commissioned only after v4 is burned, with different pins and a written author-isolation
attestation; it is not pre-authored.

Why: the Go engine is below gate 5 on the development corpus itself (0.780332), wave 3 moved no
blind-v3 contract metric, and `benchmarks/run.py` has no first-run entry for blind-v4, so a run today
would burn the only sealed partition for an unattested, near-certain failure that measures
case-authoring variance rather than the engine.

## 4. Reference chains are dropped; the 0.02 allowance is replaced by a gate-tied rule

Reference chains do not merge and the allowance is not widened. Rule, adopted verbatim: no
retrieval mechanism merges into the default path if it lowers development byte-weighted precision
below 0.80 under either engine, regardless of held-out recall gain; a mechanism that costs precision
merges only bundled with a measured precision-recovering change such that the bundle's development
precision is ≥ 0.80 under both engines and its held-out recall is non-decreasing. Reference chains
may be revisited when the query result vocabulary widens (`docs/agent-memory/ideas.md`, the
precondition that also blocked the type-diversity reservation).

Why: "0.02 allowance" appears only in prose (BUILD-LOG, decision 0036, ideas.md) and no spec or
harness check enforces it. The gain is one extra critical selector out of ten on blind-v3, which is
indistinguishable from noise. The cost would put both engines below gate 5 (0.736 Python, 0.711
Go). The Git read the mechanism needs at cache-hit time contradicts DIRTY-CACHE-002, and a Go-only
adoption with a frozen oracle is a candidate-side divergence GPK-V0-033 forbids resolving toward the
candidate.

## What this record does not do

It does not tag, publish, promote, sign, or choose the next version number. It does not run the
Packet 5 protocol; item 1's rerun and corpus addition are follow-ups tracked in
`docs/agent-memory/fixes.md`. The four questions are removed from `docs/agent-memory/questions.md`
in the same change.
