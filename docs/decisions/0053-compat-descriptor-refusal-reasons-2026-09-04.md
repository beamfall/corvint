# Decision 0053 — distinguish invalid descriptors from digest mismatches

Date: 2026-09-04. Status: accepted. Authority: repository owner, answering
"Approve the correction" to the specific proposal to add a `descriptor-invalid` reason while
preserving `digest-mismatch` for actual hash mismatches.

## Correction

`CRR-V0-003` accepted a `descriptor-invalid` pre-launch refusal for any descriptor rejection,
but its closed `Reason` enum could name only `digest-mismatch` for that class. Invalid structure,
typing, or lexical containment is not evidence of a mismatched hash.

The pre-launch refusal and reason vocabularies therefore both distinguish `descriptor-invalid`
from `digest-mismatch`. The pure `procgroup.Adjudicate` input carries the distinction, so neither
the adjudicator nor the runner needs to infer it from diagnostic prose. Both remain `NOT_RUN`
with the existing compilation/setup inconclusive state and no compatibility conclusion.

This corrects diagnostics only. It adds no execution permission, host qualification, scored
trial, or acceptance of `CTR-V0-011/012`.

## Verification and rollback

Separate fixtures must assert the exact refusal and reason for malformed descriptors and actual
digest mismatches, including the pure adjudicator. Revert this decision's implementation together
with its spec amendment to restore the prior experimental wire vocabulary.
