# Decision 0078 — Retire the code-first recipe experiment

Date: 2026-09-05. Status: accepted (delegated call; administrative retirement).
Authority: Russell Lewis's instruction to run wave 1 and promote or retire R3.
This disposition is owner-authored by the coordinating delegate.

## Evidence and decision

The registered unchanged `r+doctail` candidate and the pinned main baseline each
achieved natural no-gold selective success 0/50 and counterfactual success 0/32.
All 82 outcomes tie; all five ladder arms completed without errors. No-gold
nonregression passes, but provides no quality improvement. This current baseline
differs from the historical 4/50 and 10/32 report; no causal explanation is established.
(Reconciled 2026-09-12 in decision 0068's amendment: the pin counted `test` rows as a rescue.)
The registrations and original reports are retained under
`benchmarks/results/nextgen-wave1-r3-2026-09-05/`.

Prior positive development evidence for doctail reached trace2code recall@5 0.507,
but MRR 0.304 misses decision 0070's 0.372 first target. There is no promotion
evidence satisfying that decision's paired intervals, repository folds and
no-gold conditions. We retire the experiment as directed by the handoff and remove
the entire `CORVINT_CONTEXT_RECIPE` behavior, including previously falsified `r`
and `r+anchor`, following TCP-V0-018's retirement clause. This is administrative
retirement after failed promotion, not a claim that the clause's own numerical
falsifier has newly failed. Existing default behavior remains the regression oracle.

## Limits and rollback

The handoff incorrectly labels v2_abstention unobserved: base BUILD-LOG already
records its use. All v2 results are development evidence. Blind-v4 is NOT_RUN:
decision 0037's development release is blocked by precision, FIRST_RUN_EVIDENCE
has no registered v4 release, and the frozen eval/query endpoint does not measure
this context-only recipe. The seal remains closed. Removal preserves the stable
requirement ID with a retirement record and tests that former flag values produce
the default packet. Reintroduction needs a new accepted experiment contract;
historical measured reports remain unchanged.

The two runs used verified nice 15, no overlapping gate, host loads 4.76 to 7.94,
and elapsed 151.95/148.38 seconds. These are no-gold outcomes, not latency claims.

Evidence location (2026-09-05): wave-1 paths above are logical member paths in the frozen, byte-exact evidence bundle. See `benchmarks/results/NEXTGEN-WAVE1-ARCHIVES.md` for manifest verification and safe restoration; packaging did not rerun or recompute the benchmark.
