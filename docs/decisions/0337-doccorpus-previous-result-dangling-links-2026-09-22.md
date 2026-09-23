# Decision 0337 — doccorpus previous-result tolerance for dangling reverse links

Date: 2026-09-22 (UTC). Status: accepted, owner call recorded as open on the disqualification
question (V1-0060).
Authority: batch-D ticket work on V1-0044 and V1-0050 against `docs/specs/documentation-corpus-v1.md`
DCP-V1-031/DCP-V1-032. DCP-V1-031 already commits forward emission (`reconcileTest`) to retaining a
criterion or claim it cannot resolve against the normative variation set and reporting it as
`undocumented-tested-behavior`, never pruning it. `validatePreviousBehaviorAdapterResult` (the
`--previous` strict-consistency check DCP-V1-032 requires before a delta) disagreed: it fatally
refused any criterion or claim referencing a variation absent from that set, so a legitimately
emitted `run1.json` carrying exactly that retained finding could not be reused as `--previous`,
making the DCP-V1-032 delta unusable in the one case DCP-V1-031 exists to surface.

## Decision

`validatePreviousBehaviorAdapterResult` (`internal/doccorpus/behavior_adapter.go`) now tolerates a
dangling criterion or claim on the same terms forward emission retains it: a test criterion absent
from the normative variation set, or a claim naming such a criterion, is accepted when the test's
own declared `Criteria` still lists it — the shape `reconcileTest` reports as
`undocumented-tested-behavior` rather than pruning. A claim naming a variation the test does not
declare at all (never emitted by forward emission, only constructible by tampering) still refuses as
before. DCP-V1-032's "semantic-link validation" clause is amended to state this tolerance and to cite
this decision.

Separately, `lost_reverse_links` entries (`BehaviorAdapterDelta.LostReverseLinks`) were joined with a
literal NUL (`\x00`) between fields, which `textOK` (`encoding.go`) forbids in every other published
corpus text field, and the read side never validated the field, so a NUL-bearing value round-tripped
silently. The six key-building sites in `behaviorReverseLinks` now join fields with a printable `|`
separator through a new `reverseLinkJoin` helper that backslash-escapes a literal `|` or `\` inside a
field first, preserving the same collision-freedom the NUL separator gave against fields containing
`:` or other punctuation. `validatePreviousBehaviorAdapterResult` now runs `textOK` over every
`Delta.LostReverseLinks` entry on a `--previous` result, refusing one that still carries a NUL.

DCP-V1-032's owner question — whether a prior bundle carrying dangling criteria should disqualify a
delta outright instead of being tolerated — is recorded as open in `.taskman/` ticket V1-0060 per the
batch-D brief; this decision implements the tolerant reading pending that answer and does not close
V1-0060.

## Rollback

Revert `internal/doccorpus/behavior_adapter.go`'s `validatePreviousBehaviorAdapterResult` dangling-
reference branches and `reverseLinkJoin`/its six call sites, and the DCP-V1-032 wording amendment.
`lost_reverse_links` values produced before this change used `\x00`; a validator reverted to the prior
fatal-refusal behavior treats any dangling-criterion prior (whether `\x00`- or `|`-joined) as invalid
again, matching the pre-0337 contract. No stored data or wire schema field was renamed, so no
migration is required either direction.
