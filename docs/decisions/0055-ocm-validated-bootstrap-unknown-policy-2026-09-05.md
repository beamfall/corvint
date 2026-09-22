# Decision 0055 — OCM-validated bootstrap unknowns precede the dogfood ceiling

Date: 2026-09-05. Status: accepted. Authority: repository owner via delegated call.

## Decision

Corvint repository dogfood subtracts OCM-validated bootstrap unknowns before applying its
`--max-unknown` policy ceiling. A bootstrap unknown is the CEM hunk for an exact declared OCM intent
path that is absent at the exact CEM base and whose final OCM map passes bootstrap validation
against the same base, target, CEM map digest, and CEM patch digest. OCM bootstrap validation keeps
the hunk explicitly `unknown`; it never converts same-change intent into historical evidence.

The current canonical patch grammar requires a created path to have exactly one hunk. Therefore,
after the complete OCM scope set verifies, the number of declared intent paths absent at the base is
exactly the number of validated bootstrap-unknown hunks. Dogfood coordination and final checking use
that count as the CEM policy allowance, leaving an effective maximum of zero after subtraction. An
unknown hunk outside those validated base-absent intent scopes still exceeds policy. If the patch
grammar ever permits more than one hunk for a created path, this counting rule must be revised before
that grammar can be used by dogfood policy.

The raw CEM dispositions and counts remain unchanged and visible. Missing, invalid, unavailable, or
drifted OCM state grants no subtraction and fails closed. Mechanical policy is unchanged.

This is a repository dogfood-policy rule. It changes neither the `cem/0.1`, `cem/0.2`, nor
`ocm/0.1-experimental` wire profile, and it does not change general CEM CLI policy semantics.

## Rationale

`OCM-V0-009` requires base-absent same-change intent to remain explicit unknown, while the former
dogfood path applied a zero-unknown CEM ceiling before OCM validation. A change introducing its
owning spec could satisfy neither branch. The selected ordering preserves the honest CEM state while
granting no general exception.

## Verification and rollback

A scratch repository proves both branches: one base-absent declared intent with its CEM hunk unknown
passes the adjusted zero-unknown dogfood policy; the same bootstrap unknown plus one unrelated
unknown fails `max-unknown-exceeded`.

Rollback restores raw CEM unknown counting in dogfood coordination and checking and reopens the
spec-introduction deadlock. No wire migration is required.
