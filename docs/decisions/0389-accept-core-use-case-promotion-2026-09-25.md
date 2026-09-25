# Decision 0389 — accept the Core use-case promotion (V1-0011)

Date: 2026-09-25. Status: accepted. Authority: repository owner instruction "accept #200 and merge
it" (2026-09-25).

PR #200 promoted `UC-TASK-ORIENTATION`, `UC-CHANGE-CONSEQUENCE` and
`UC-EVIDENCE-CARRYING-COMPLETION` to `verified`/`VERIFIED` (`docs/specs/use-case-conformance-v0.md`,
V1-0011) and left two points for the owner. Both are accepted.

1. **Claim scope.** The orientation row's `beamfall-dogfood` receipt came from a build that
   includes `GPK-V0-066` (decision 0387). The published 0.8.1 archive abstains on that query
   (V1-0260). Under `UCV0-012` the `VERIFIED` claim therefore covers releases that include
   decision 0387, which means the 1.0.0-rc.1 candidate, and does not cover 0.8.1.
2. **Benchmark quality.** `UCV0-007` says that mechanical completeness does not replace an
   independent review of benchmark quality. No independent review of daily-loop run-002 is
   recorded. The owner accepts run-002 as it stands as the sealed-benchmark input for this
   promotion. The independent review stays `NOT_PRODUCED` and is not claimed.

Rollback: revert PR #200's merge, which returns the three rows to `experimental`/`UNPROVEN` and
restores the 22 `UNPROVEN` test pin, and mark this decision superseded. No receipt bytes change.
