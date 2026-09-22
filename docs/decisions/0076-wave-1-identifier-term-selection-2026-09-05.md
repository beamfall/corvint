# Decision 0076 — Wave-1 identifier term selection

Date: 2026-09-05. Status: accepted (delegated call; point target passes, promotion fails).
Authority: Russell Lewis's 2026-09-05 instruction to run wave 1 and decide promotion or
falsification from the handoff's measurements. This record is written by the coordinating
delegate, not the implementation lane.

## Frozen choice before repository folds

R1 tests `CORVINT_CONTEXT_TERMS=ident`, with every other ranking experiment unset.
Valid JSON contributes string values, never object keys; ordinary tasks use their text.
Code-shaped identifiers and code filename stems supply whole names at weight 3 and
camel-split parts at weight 1, duplicates taking their maximum weight. If none are
present, the cleaned ordinary lexical terms remain. Only lexical scoring changes.
Authority and structural slots keep their existing rules. The default flag is off.

Candidate SHA-256: `70c0fff121fb8bad6914605c43105d4c32a5cc2a15e284839bda52438270b77f`.
Registration is preserved under the wave's results directory at landing, including
source/patch identity, binary and samples digests, fold map, arms, and commands.
The mechanism and weights are frozen before A and B; no code/parameter repair follows
inspection of either fold. All v2 positives are development data; B is an untouched
confirmation run for this configuration, not an unobserved corpus.

## Falsifier and promotion rule

Trace2code recall@5 must reach 0.502 and MRR 0.372 without code2test recall@5 below
0.225. Section-3 baselines: trace2code 0.421/0.543/0.833 and code2test
0.225/0.356/0.485 (recall@5/10/20). Report all four positive subsets, both folds,
paired intervals against each arm of the ladder and its strongest per-cell arm.
Passing a point target alone never passes decision 0070. Promotion additionally needs
its paired interval and no-gold conditions; a sign flip is not promotable.

The handoff's unobserved-v2_abstention assertion is contradicted by the existing
BUILD-LOG's answerability runs and withdrawn rules. It is development data too.
Blind-v4 remains governed by decision 0037: the existing development release report
is blocked by precision, and opening the seal does not test this context-only change
through the frozen eval/query interface.

## Result and rollback

The registered eight runs completed with zero arm errors. Pooled recall@5/10/20
and MRR were code2test .268868/.407547/.559119/.217134; comment2context
.233333/.362500/.485417/.266931; edit2ripple .362069/.517241/.622126/.417776;
trace2code .531353/.676568/.811881/.409993. The first point target passes.

Promotion fails: trace2code fold B versus `bm25:ident` has recall@5 difference
-.094595, paired 95% interval [-.189189, -.013514], and MRR difference -.095146,
interval [-.174558, -.018054]. Both reverse fold A. No fold-B subset resolves both
endpoints positively against the strongest ladder arm. The opt-in remains a
proposed reference experiment; it is not accepted default ranking. Unsetting
the flag restores the base packet; no snapshot or frozen wire changes. All 345
grep rankings equal their historical counterparts. Timing under loads 5.13–9.68
does not establish a latency result. Raw splits, registrations and derived
summaries are retained with the lane evidence.
