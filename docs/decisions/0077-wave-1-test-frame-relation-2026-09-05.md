# Decision 0077 — Wave-1 named-test frame relation

Date: 2026-09-05. Status: accepted disposition (delegated call; point target and promotion fail).
Authority: Russell Lewis's 2026-09-05 instruction to execute R2 under the wave-1
handoff and adjudicate its promotion or falsification. Coordinating delegate record.

## Frozen choice before repository folds

Candidate SHA-256: `9859897a46f7590de4d81a1e131b7ef2d20d35b2c9e8d23f68242832c686774a`.
R2 uses only `CORVINT_CONTEXT_FRAME_RELATION=1`; other experiments are unset.
In retrieval shape, up to three distinct sorted test paths named by the task anchor
implementation candidates through mirrored stems, import edges and whole identifiers
the test mentions. Existing relation weights apply, accumulated deterministically.
At most three `test` rows enter before mentions and definitions, replacing the later
one-row test pass for those tasks. Final corroboration, governance and spec reservations
still apply; the test itself is not implementation evidence. No constants are tuned.

Independent pre-build review found no HIGH concern. Independent code review verified
the immutable import view and deterministic frame ordering and required additional
ambiguous-basename, inactive-flag and capped-anchor witnesses before measurement.
Package tests and their registrations preserve the first failures and corrected run.

## Falsifier and promotion rule

Run A then B, frozen before either executes. Trace2code recall@5 must reach 0.55
(section-3 baseline 0.421); code2test recall@5 must stay at least 0.215
(baseline 0.225, margin 0.01). Report recall@5/10/20 and MRR on all four positive
subsets and both folds, with the strongest baseline ladder per cell and paired
intervals. No changes follow observations of either fold.

Both folds are development data, including B; decision 0070's additional promotion
gate is not reduced to these point targets. No-gold selective success cannot fall.
The already observed v2_abstention release cannot supply a clean holdout claim;
blind-v4 remains sealed under its failed decision-0037 prerequisites.

## Result and rollback

All eight registered runs completed with zero errors. Pooled recall@5/10/20 and
MRR are code2test .278302/.399371/.511635/.203718; comment2context
.252083/.345833/.516667/.275435; edit2ripple .362069/.510057/.629310/.414457;
trace2code .435644/.518152/.790429/.368402. Trace2code fails the .55 target;
code2test passes its .215 nonregression guard. The lane falsifier therefore FAILS.

Promotion also fails: trace2code fold B minus `bm25:ident` has recall@5 difference
-.202703, paired 95% interval [-.337838, -.081081], and MRR difference -.117553,
interval [-.221072, -.025012]. Both reverse positive A differences (+.013021 and
+.061566). Recall@20 also falls below the .833 historical trace baseline. Keep
this as an unaccepted opt-in reference experiment; no default promotion follows.
No-gold and blind-v4 are NOT_RUN: the positive target already fails, all v2 data
are development, and blind-v4's prerequisites remain blocked. Unsetting the flag
restores the base packet; no snapshot, query, eval, or frozen wire changes.
