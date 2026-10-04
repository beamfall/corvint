# Pool safe reuse successor: integration and observed qualification

Issue498 / native V1-0695, under the owner-approved successor decision recorded in
`2026-10-04-pool-safe-reuse-successor-seed.md`. This entry records only observed successor facts.
The original enrollment (base 211916, key a2284577) remains NON_SUCCESS and is not relabelled.

The documentation seed 3069405d51a7ab35b6b452f440b2b9ae42a5d74a was bound, checked, reviewed and
sealed against main 4b10a02144faed4a77b36eba4b0d1cbc315706d3: four hunks supported and one
bootstrap intent hunk unknown. Its five documentation checks passed under its own key, and keyed
finish reported satisfied before the seal. The seal 1def23a641bc00561f142e0b4b0586266609b5f2 is
the successor base. The seal made that key stale, and its successor begin was refused with
worktree-prior-completion-stale, so the seed key was cancelled under the decision. Its earlier
satisfaction is retained separately and is not claimed now.

The successor key enrolled at that seal with the single PSR intent and the original psr-focused and
psr-vet checks unchanged. Merge 8b409a2fa909a3f50f53705e304d1df23c55a8a8 ordinarily merges the
retained candidate c2471020fc88f6279230a3fbea7d3baaf75ca1fc. It keeps all 355 archives at the seal,
including main archives 63368234 and f400a78e, and omits candidate tips cd3a19d8 and ceaaddaa. The
36 PSR runtime/test paths equal c247 and the six Owner blobs equal main. The PSR catalog record
takes the candidate delivery and implementation metadata, CAL-V0-030 arrives with exactly c247's
delta, and REQUIREMENTS is regenerated. The PSR requirement clauses are byte-identical to the seed.
The merged candidate entry `2026-10-04-pool-safe-reuse-integration.md` is retained as c247 history;
its BASE-211916 archive restoration plan does not apply to this successor. The merge had re-escaped
eight non-PSR INDEX records; commit f1b1765eb19285ab1b1d6a434fbfc7915ba29fb6 restores main's
literal bytes for every non-PSR record, and the catalog and specindex checks pass after it.

At that merge (tree b3c8b3eabad11ae2081d9197fae2a66df9737eea), go1.27.1 darwin/arm64 built a fresh
corvint-tasks with SHA-256 8d2a060c4d569993c84d981a7d0e8e8d9127fa9b78114ca4877b90486087a813, used
as an absolute private PSR_TEST_BINARY. The full selection over the seven tasks packages observed
all 58 mandatory top-level passes and two inner-absence observations, but its rc was 1. The one
failure, TestCALV0044_FailedGateRemainsChargedAfterPassAndSubmit, is outside the mandatory set.
It reports a gate argv identifier of 148 bytes, over the 128-byte limit. The identifier comes from
a temporary path under the long private TMPDIR, and the same failure reproduces at the seal, which
has no PSR code. That run therefore stays FAILED, recorded as a pre-existing environment-sensitive
fixture defect. The race selection passed all 34 required tests (rc0) and focused vet passed (rc0).
Documentation, specindex and catalog checks passed at the merge.

The selected keyed checks, exact-target independent review, hosted integration and native
completion are not recorded here. Linux runtime and physical external cleanup remain unqualified.
Rollback reverts the merge and this entry; quarantine, pending owners and evidence are preserved.
