# Decision 0589: the cem/1.0 stable result schema follows accepted requirements

- Date: 2026-11-02
- Status: accepted (owner delegation of 2026-11-01, "you make the call for me", exercised by the
  orchestrator for V1-2521 and V1-2520; recorded by lane cfh-w36)
- Authority: decision 0546 (CEM-V1 contract acceptance), decision 0525 §1 (one `cem/1.0` wire),
  `docs/specs/cem-stable-v1.md` (`CEM-V1-083`, `CEM-V1-084`, `CEM-V1-620`, `CEM-V1-880`,
  `CEM-V1-091`).

## Context

The Codex review of PR #1651 (V1-2323, 2026-10-24) left two `protocol/cem-1.0/stable` inconsistencies
for the owner:

- V1-2521: `RESULT.schema.json` fixed `axes.authentication` to `const NOT_OBSERVED`, while
  `CEM-V1-084` (accepted by decision 0546, implemented in `internal/cem/wire/stable_auth.go`,
  `internal/cem/verify/stable_auth.go` and `interop/cem01-go/stable_auth.go`, public case
  `protocol/cem-1.0/stable/receipt-authentication/`) makes both consumers emit `NOT_OBSERVED`,
  `UNTRUSTED_KEY`, `VERIFIED` or `FAILED`. The `CEM-V1-084` traceability row recorded that the
  schema also lacked the four attestation issue codes (V1-1265).
- V1-2520: the sealed neutral-criterion fixture carries the provider-neutral plain criterion form
  of `CEM-V1-083` (accepted by decision 0546), while the schema's `$defs/criterionBinding` declared
  only the Tasks binding.

Both tickets asked which side is authoritative, because changing either repins sealed bytes.

## Decision

1. **Rule.** CEM 1.0 has not shipped stable. An accepted, implemented and tested requirement
   outranks a stale schema line. The schema is the side that changes, through its sealed-byte
   repin procedure (`CEM-V1-620` recorded enum widening, `CEM-V1-880` recorded `anyOf` branch,
   `CEM-V1-091` recorded member addition), never by narrowing the requirement and never by editing
   the schema by hand. `cem-stable-regen --check` must pass before and after each run, and every
   sealed case, cell, exit and verdict digest stays unchanged, because each mode only admits more.
   The spec records this as `CEM-V1-845`.
2. **V1-2521.** `CEM-V1-084` is authoritative. `axes.authentication` admits the four outcomes
   (`CEM-V1-840`); the issue code enum admits `attestation-malformed`, `attestation-key-mismatch`,
   `attestation-subject-mismatch` and `attestation-signature-invalid`, and both artifact `kind`
   enums admit `receipt-attestation` (`CEM-V1-841`); the runner receipt echo at
   `references.runnerReceipts[].authentication` admits the two `CEM-V1-084` object forms beside the
   legacy string (`CEM-V1-842`). The `CEM-V1-084` traceability row no longer names the gap.
3. **V1-2520.** `CEM-V1-083` is authoritative. The widening was already delivered on `origin/main`
   by V1-2615 (`CEM-V1-880`, `CEM-V1-881`, commit `a41d3a601`): `$defs/criterionBinding` is
   `anyOf` the sealed Tasks form unchanged and the plain form, so the neutral-criterion fixture and
   every Tasks-bound S0E result validate against the committed copies as they are. This decision
   records the owner-side choice that ticket still lacked; no further schema change is made.
4. **Proof.** `TestCEMV1843ReceiptAuthenticationResultsValidate` (`internal/cem/verify`) and
   `TestCEMV1843PortableReceiptAuthenticationResultsValidate` (`interop/cem01-go`) validate every
   complete result of the receipt-authentication case (the VERIFIED ACCEPT, five axis cases,
   thirteen refusals) on both consumers against both schema copies, and show each sealed
   predecessor node refusing it. `TestCEMV1844TasksBoundResultsValidate`,
   `TestCEMV1844TasksBoundLiveResultValidates` and `TestCEMV1844PortableTasksBoundResultsValidate`
   validate the 56 sealed full-result cases, a live Tasks-bound run of `ledger-336-evidence` on
   each consumer, and the plain neutral-criterion ACCEPT line.

## Forks taken under the delegation

The ticket text named only the axis value and the four issue codes. The evidence showed two more
refusals of the same accepted requirement, so the narrowest consistent option was to widen them by
the same recorded procedure rather than leave the proof failing:

- the receipt echo at `references.runnerReceipts[].authentication` carries the map's object form
  (`{"method":"dsse-ed25519","keyid":K,"envelopeSha256":E}`) in the sealed `EXPECTED.json`, not one
  of the four axis strings, so a `const`-to-`enum` widening there would have been wrong; two
  `CEM-V1-880` branches mirror `stable.schema.json`'s `oneOf`;
- `references.artifacts[].kind` and `artifactChecks[].kind` refused `receipt-attestation`, the
  artifact role `CEM-V1-084` requires the envelope digest to resolve to.

## Non-goals

No change to any sealed map, case, cell, exit, verdict digest, the candidate profile, the S0E
packet's expectations, either consumer, or the `cem/1.0` wire. No promotion: the packet status
stays `IMPLEMENTED_CONTRACT_PROPOSED_NOT_PROMOTED`.

## Rollback

Revert the six recorded runs (a Git revert of the regenerated packet files and the records under
`tools/cem-stable-regen/testdata/v1-2521-*.json`), the `CEM-V1-840..845` text, the frontier
allowlist entries and the tests. Both consumers then emit values the schema refuses again, which is
the state V1-2521 described.
