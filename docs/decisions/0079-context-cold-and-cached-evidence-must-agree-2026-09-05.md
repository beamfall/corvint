# Decision 0079 — Restore complete and deterministic context evidence

Date: 2026-09-05. Status: accepted (delegated call; correctness repair).
Authority: Russell Lewis's wave-1 instruction and explicit request that Corvint
solve reliability problems across agent workflows. Owner-authored disposition.

## Observed defect and contract

L1's first real-tree matrix found different test evidence and withheld counts;
L3's first hit benchmark refused a cold/hit result mismatch. Independent reviews
confirmed that cold compilation still filtered imports for only the subject,
while TCP-V0-015 now consumes test imports for all candidate source anchors.
Snapshot builds kept those legitimate edges. A second defect accumulated words
in map order, producing random equal-IDF labels and floating-sum order.

New registered regressions reproduce ranking changes in Go, Python and
TypeScript: equally mentioning tests tie lexically, but only the later path has
an import witness and must win. The equal-IDF regression also fails before the
repair. Restore the complete import table for cold context builds and visit
test-anchor word keys in lexical order. Preserve query/eval narrowing, weights,
caps, authority and wire formats. The selected evidence set is governed by the
full index, never by whether an optional cache happens to exist.

Independent pre-build review found no HIGH concern. The MED tradeoff is extra
cold extraction CPU; the previous optimization is unsound for the expanded
consumer. A future selective or incremental compiler must prove equivalence
of ranking, evidence and uncertainty before promotion. The analyzer identity
advances to schema 7 with reviewed input pins.

## Verification and interpretation

The mixed-language import-winner and 64-iteration complete-packet tie witnesses
pass after the repair, along with context, frame, shard, pack and analyzer
regressions under verified nice 15. The first post-repair check stopped at a
stale source-line citation before tests; the same source passed after updating
the citation. Original failure logs and registrations remain preserved.

A separately registered corrected-tree 120-packet campaign and full code2test
cold/hit measurement validate the integration. They do not replace frozen
L1/L3 outcomes or rehabilitate R1/R2's failed promotion. Final results are recorded
in the wave report. An absent cache can now change cost, but must not change
the supported context answer. Reverting this repair requires preserving that
same invariant, not restoring the known incomplete cold graph.
