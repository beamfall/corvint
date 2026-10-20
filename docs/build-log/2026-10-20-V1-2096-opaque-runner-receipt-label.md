# 2026-10-20 V1-2096: opaque label for an unadmitted runner receipt

GitHub #1328. The new requirements are `CEM-V1-310..313` in `docs/specs/cem-stable-v1.md`. They are
accepted under the owner's standing auto-accept (2026-10-11).

## Problem

Without `runnerAdmission`, `corvint-cem-candidate assemble-stable --experimental` with
`cem-stable-plain-assembly/1` pins any file as runner plan and receipt by SHA-256 alone. The map
still labelled the receipt record `corvint-test-runner-receipt/0`. In a trial, a framework JSON
report and an unrelated provenance JSON verified ACCEPT as receipt and plan, so a reader could take
them for a Corvint runner receipt.

## Decision

- The producer labels an unadmitted receipt `opaque/0`. It keeps `corvint-test-runner-receipt/0`
  only after the `CEM-V1-280` admission, so admitted output is byte-identical to before.
- Both `cem/1.0` stable readers (native `internal/cem/wire` and portable `interop/cem01-go`) accept
  exactly the two labels, with the same fixed limitation fields. `stable.schema.json` and the stable
  `ALGORITHMS.md` say the same, and the stable manifest is re-pinned for those two files.
- No committed map is rewritten. Every sealed map under `.corvint/changes/` and every public case
  keeps the legacy label and stays valid. The public `neutral-criterion` case keeps its legacy
  label as the legacy-label control. The `CEM-V1-240` reproduction test compares against that case
  with the one label substituted.
- Rejected alternatives:
  - Make every unadmitted assembly decode the runner documents. That changes `CEM-V1-240`'s opaque
    contract and still would not prove execution.
  - Rewrite the public case and sealed maps to the new label. The constraint forbids rewriting
    sealed maps, and the case's verdict digests would move.

## Limits

- A third-party `cem/1.0` reader that knows only the legacy label refuses a newly produced
  unadmitted map. That is the safe direction, and it is recorded in the spec's failure modes.
- A legacy label on an old map still does not show that its bytes were admitted.
- No reader-side admission and no new result axis.
- Open (independent review finding): `RESULT.schema.json` in both copies still pins
  `references.runnerReceipts[].profile` to the legacy label. A verify result for a new unadmitted
  map therefore does not validate against the published result schema. Widening it needs a
  re-binding of the sealed `CEM-V1-091` chain, and `cem-stable-regen` has no mode for that. It is
  deferred to a follow-up.

## Verification

- `go test` passed on Darwin arm64 for `./internal/cem/wire`, `./internal/cem/verify`,
  `./internal/cemcandidate`, `./internal/cemchain`, `./cmd/corvint-cem-candidate`,
  `./tools/cem-stable-regen`, `./internal/cem/...` and the `interop/cem01-go` module.
- Independent review: one Codex round with one P2 finding, the result schema limit above.
