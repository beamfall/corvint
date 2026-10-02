# Explicit pool member exclusions (issue 480)

The owner-selected CAL-V0-065/S15 contract adds opt-in explicit member exclusions
to CLAIM, CLAIM_NEXT and read-only preview. Both CLI parsers accept repeatable
single-value `--exclude-member` arguments and normalize reordered duplicates.
The request digest omits the new field when absent, preserving historical bytes.
Authoritative replay precedes current-policy eligibility; fresh allocation,
health selection, prepared admission and preview share the same ordered filter.

The human-owned intent was seeded at
`fbc075adbe7901c9c8901ab15ab8024ca27d608c` before implementation enrollment.
The implementation was frozen at `255c8c99efdc61678cadbe16fa3685a4b0699d7a`.
Independent read-only source review passed that exact twelve-path diff with no
P1/P2 finding. The source blobs remain unchanged during documentation binding.

Focused transaction/store/CLI tests passed, including exact old preimage/digest,
shape/member validation, tier ordering, preview purity/capacity, valid-observation
prepared admission, excluded health markers across rounds, both next selectors,
explicit/next replay after successor/policy changes, and both parsers. The compiled
native fixture built the candidate executable, used a disposable native store,
retained ordinary release quarantine and original replay payload, and audited it.
Three-package vet passed. This is scoped local evidence, not broad runtime qualification.

The combined health-backed CLAIM_NEXT and concurrent policy change between health
preparation and final admission paths were source-reviewed, not executed as combined
fixtures. The exact legacy preimage is a source-derived fixed witness; separate
baseline executable measurement is NOT_EXECUTED. Native audit structural consistency
and projection agreement do not establish semantic coverage or runtime qualification:
those remain UNKNOWN and NOT_OBSERVED. Exclusions do not authenticate holders,
discover review history, relax quarantine or prove physical independence.

Original pre-edit query/impact receipts were retained under the leaf Git directory.
The initial no-diff change-start dogfood observation failed with its original
cem-prepare git-diff-failed/map-not-produced/OCM/intent-scope/map-unavailable reasons;
that failed observation remains retained and was not called a passed gate.
The affected-plan frontier unknowns include Go build constraints and nested modules.
Its static exhaustive-gate advice does not override the owner's scoped gate policy;
full repository gate is NOT_RUN.

Terminal enrolled checks and CEM/OCM binding are retained as actual private
receipts against the existing pre-implementation enrollment and immutable commits;
source review does not substitute for them. The final sealed map preserves the
binding commit. CI, root integration and native ticket completion remain separate
outcomes and cannot be inferred from the local source or structural proof.
Full future effects and the output manifest remain intact. Rollback and failure
modes remain in S15; history and current allocations are never rewritten to hide
exclusions or their compatibility limits.
