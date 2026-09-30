# Test-acceptance error ownership repair

PR #406's remote doc gate failed because 41 existing emitted codes in
`internal/testacceptance` were absent from its owning spec. The local
`make error-code-ownership-check` reproduced that failure at reviewed seal
`0cc5005cb7514855d9f1dece0f9bb1d3c3a49086`; remote Go checks passed.

Under native generation 66, document only the existing refusal and Strength-reason
branches in `new-e2e-test-acceptance-v0.md`, with each original source line as required by
`ECO-V0-006`. No code spelling, requirement ID, wire, behavior or qualification changes.
The same approved request/control and unknown-freshness boundaries remain operative.

A fresh docs-only enrollment is based on the reviewed PR head and governed by the
accepted error-ownership contract. Its selected checks are the six actual CI doc
targets and spec-index consistency. Existing source review and local browser evidence
remain at their original bindings; this repair does not repeat or upgrade them.
OCM structural linkage for the gate's unchanged requirements remains unassessed;
the actual shell gate and direct code-to-prose readback supply the scoped doc evidence,
without inventing Go test claims for a prose-only change.

Retain the original failed CI/local gate and pre-seal enrollment satisfaction separately
from the seal-head stale predicate (existing V1-0493). Positive accepted-path support
remains blocked by V1-0556. Rollback this isolated documentation/evidence continuation;
retain the prior PR head, old enrollments and failed observations. The ticket remains
open through positive-path delivery, integration and native completion.
