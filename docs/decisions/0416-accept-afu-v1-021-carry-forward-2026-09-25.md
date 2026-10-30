# Decision 0416 — widen `AFU-V1-021` to the whole Verified carry-forward rule

Date: 2026-09-25. Status: accepted. Authority: repository owner answers on PR #248, "widen
AFU-V1-021, add the case to the corpus" and "accept the head-revision import graph" (2026-09-25).
Ticket: V1-0251.

`AFU-V1-021` cited the Verified carry-forward rule but glossed it as "so no covered path changed",
and the `e2e-safe` coverage basis implemented only that gloss plus global paths. A test file edited
after its coverage run, outside its covered paths, was then omitted on the `coverage` basis although
the fault broke it (BUILD-LOG, 2026-09-25 V1-0251 AFU-V1-021).

Accepted as written in `docs/specs/application-flow-understanding-v1.md`:

1. `AFU-V1-021` states the whole rule: between the evidence commit and the base, no covered path, no
   global path, no path in the test's static reach, no link target of a flow that links the test, and
   no path the impact graph reaches from such a target may have changed. The ID is unchanged.
2. The frozen AFU-V1-040 corpus gains the reproduction as case `spec-after-coverage`. Its report
   now has 21 cases, `e2e-map-stale` 3, and reductions 15/105 (`coverage`) and 3/105
   (`reviewed-links`), both with 0 unsafe omissions.
3. The staleness check uses the head revision's impact graph for the diff between the evidence
   commit and the base, the same graph as the obligation closure.

Evidence: `TestAFUV1021CoverageStaleWhenStaticReachChanged` and `TestAFUV1040SelectionCorpusReport`
pass. With the pre-fix `internal/appflows/selection.go`, the corpus reports an unsafe omission of
`search.spec.ts > finds` and withdraws the `coverage` basis.

Rollback: restore the previous `AFU-V1-021` text, drop the `spec-after-coverage` case and its report
rows, and revert the `coverageStale` change; the `coverage` basis is then withdrawn, because the
corpus would no longer protect it against this fault.
