# Decision 0056 — CEM cite accepts stable same-path base spans

Date: 2026-09-05. Status: accepted. Authority: repository owner via direct repair instruction.

## Decision

`cem cite` no longer refuses base evidence merely because its path is modified by the candidate
patch. The producer obtains the exact patch from a `baseRevision`-to-`HEAD` canonical derivation or
producer cache, validates it against the map's `patchSha256`, applies the evidence path's complete
hunk group to the base blob with `internal/cem/sim`, and compares the simulated result with the exact
selected base bytes. A byte-identical result is accepted as stable. Otherwise, one occurrence is
accepted as relocated; zero occurrences (stale) and multiple occurrences (ambiguous) fail with the
producer code `cite-span-not-stable`.

The verifier already accepts a uniquely relocated span and rejects stale or ambiguous drift. This
decision aligns the producer with that existing rule; it changes neither verifier behavior nor the
wire-frozen `cem/0.1` reference, `interop/cem-0.1`, or `interop/cem01-go` implementation.

## Rationale

Base-side bytes are immutable, blob-pinned, and predate the change even when their path is modified.
The former path-identity refusal therefore excluded legitimate implementation evidence without
preventing a hunk from citing bytes it adds, because added bytes are absent from the base blob and
cannot be selected. In the last dogfood binding, the refusal forced 153 of 313 hunks to cite a
generic change-loop clause instead of the modified file's own base bytes.

The selected rule preserves the useful protection: the exact cited base bytes must survive the
map-bound candidate patch, uniquely when the blob changes. It does not admit stale evidence or
lexical similarity.

## Verification and rollback

Focused producer regressions cover a relocated span accepted, a stale span refused, an ambiguous
span refused, and removal of the old blanket same-path refusal. Rollback restores the producer-only
path check and reopens the producer/verifier disagreement; no wire migration is required.

## Implementation note (2026-09-05, not part of the accepted text)

The landed producer routes its same-path precheck through the verifier's `ClassifySpan`, so the
producer and verifier share one drift classifier; a same-path result absent after deletion or
rename is classified `deleted` and refused with `cite-span-not-stable` alongside `stale` and
`ambiguous`. Low-level 0.1 `begin` still publishes the map only (oracle parity for
`cem-begin-map`); the rename regression runs through 0.2 `prepare`, which retains the patch.
Regressions:
`TestCiteRefusesDeletedEvidenceSpanWithVerifierClassification` and
`TestCiteRefusesRenamedSourceEvidenceWithVerifierClassification`. The decision text above is
unchanged; the spec `docs/specs/cem-0.2-canonical-binding.md` carries the normative wording.
