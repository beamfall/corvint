# Freshness validator typed-schema and READY lineage findings

Independent source review of d82b455c2140167607ee47b3a89446d6502af832 required changes for two
P2 findings. Python equality admitted integer-valued float request limits and error codes, including
frontend-only error aliases. A rapid READY scheduling-miss packet could switch session/capture while
being classified as a legitimate NOT_WITNESSED outcome. The original candidate and failed review
remain preserved at /tmp/lsp-editor-freshness-source-review.md; no actual editor witness occurred.

This first bounded repair retains frozen fbbef086 and active 21e7 enrollment. Explicit integer limits
and independently typed closed wire/frontend error schemas now apply before comparison in every
branch. Every successful Core packet remains independently validated against fixture Git facts.
Baseline/retry full Core equality is required for this unchanged fixture/task. A rapid READY A packet
must match A baseline session/capture/full Core; READY B must match B retry. The test's impossible
READY-A capture2 was corrected to its independently captured A identity1. No client, ownership,
server or experimental mode behavior changed; no broader contract or qualification is promoted.

Exact before reproduction /tmp/lsp-freshness-repair-before.log retained seven accepted-invalid cases;
/tmp/lsp-freshness-repair-regression-before.log fails on the new float-code regression before fix.
After repair /tmp/lsp-freshness-repair-after.log rejects all seven with FAILED. Focused regression
checks include wire/frontend and frontend-only float codes, boolean aliases, float limits, stale and
scheduling-miss branches, READY-A/B session/capture mismatch and individually valid Core reason drift.
/tmp/lsp-freshness-repair-regression-after.log passes these plus all previous 40 negatives.
Existing context checks and Python syntax pass in /tmp/lsp-freshness-repair-context.log and
/tmp/lsp-freshness-repair-syntax.log. Required affected observation is retained separately; unchanged
client lifecycle/server suites were not rerun. Independent successor review is pending.

All tuples remain UNQUALIFIED; actual freshness clients, broader conformance, full gate and promotion
remain NOT_RUN. No CEM binding, actual runtime, publication or Tasks write occurred. Rollback retains
the original failed d82 candidate and restores the preceding validator while preserving raw evidence;
it cannot be represented as an admitted or qualified implementation.
