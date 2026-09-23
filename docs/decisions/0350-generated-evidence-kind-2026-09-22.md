# Decision 0350 — A `generated` evidence kind for external provider records

Date: 2026-09-22. Status: accepted (ticket V1-0107, a delegated call on Beamfall/corvint#64).
Amends `EEP-V0-007` and adds `EEP-V0-019` and `ETS-V0-014`; it changes no existing record's decode
or receipt.

## Context

An external provider record says how each relation was established through its `evidence` kind:
`declared`, `observed`, or `inferred`. Issue 64 points out that a consumer cannot tell a relation a
test run or a file read at a revision supports from one a model or heuristic produced, and so
cannot down-weight or exclude the latter. `inferred` names a deterministic provider rule, and
`learned` is refused outright because it names a feedback-trained source whose derivation the
record cannot cite. Neither is the honest label for a model-suggested link the provider has not
executed.

`evidence` is validated as an identifier and unknown values are excluded at composition time, so a
new kind value is additive: no existing record decodes differently, and a record that already
carries `generated` moves from `excluded-evidence-kind` to admitted-and-marked.

## Decision

- `generated` is the fourth evidence kind: a relation produced by a model or heuristic with no
  observation behind it. It must still carry the generator as `rule` and its material as
  `reference`, like every other admitted relation.
- `impact` composes a `generated` relation exactly as the other admitted kinds; every item it admits
  carries `generated` in `relation.evidence` and names the kind in its `reason`. Core neither ranks
  nor down-weights external items (`EEP-V0-015`); exclusion is the consumer's call on that member.
- `affected` test selection treats `generated` as weak evidence: a candidate coded
  `generated-only-evidence` that never qualifies an obligation and never blocks one, exactly as
  `inferred`.
- `learned` stays excluded. The fixture `internal/extevidence/testdata/conformance-selection/generated.json`
  carries one `observed` and one `generated` `verifies` relation to the same obligation and pins
  both treatments.

## Rollback

Remove `EvidenceGenerated` from the kind map in `internal/extevidence/compose.go` and from the
weak-evidence table in `internal/extevidence/selection.go`, delete the fixture and its case, and
revert `EEP-V0-007`, `EEP-V0-019`, and `ETS-V0-014`. A record carrying `generated` then returns to
`excluded-evidence-kind`; no other record or receipt changes.
