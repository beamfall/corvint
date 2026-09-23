# Decision 0365 — Emit `cem/v1` from `prove` and record the checked draft alignment

Date: 2026-09-23. Status: proposed (experimental delivery, not advertised; ticket V1-0092). Amends
decision 0354. Adds FPK-V0-050 and FPK-V0-051 and amends the FPK-V0-033, FPK-V0-035, and FPK-V0-036
text.

## Context

Decision 0354 left two gaps. No command emitted `cem/v1`, because the emission flags were outside
that change. Every OpenSSF `openfab/generation` (ossf/tac issue 628) and agentattest field name was
also UNCONFIRMED, because that work had no network access. Both sources have now been fetched.
ossf/tac issue 628 links the OpenFab draft `https://open-fab.ai/attestation/generation/v0.1`
(Open-fab-ai/openfab `f558da05`). agentattest predicate v1 is at AuroraAeon/agentattest
`a19e7f96`.

## Decision

1. A new flag on the existing `prove` CEM mode, `--attest-cem-v1`, emits the `cem/v1` statement where
   `--attest-cem` emits `cem/0`. The flag takes no value, cannot be given with `--attest-cem`, and
   implies `--attest`. No root verb is added, and every invocation without the flag keeps its bytes
   (FPK-V0-050). A separate flag was chosen over a value on `--attest-cem` because FPK-V0-031 refuses
   `--attest-cem=VALUE`, and a value would change that refusal.
2. The fetched sources confirm the Statement layer as aligned. Both drafts use in-toto Statement v1
   with `_type`, `subject`, `predicateType`, and `predicate`, and both use
   `{"name","digest":{"sha256"}}` subjects. agentattest signs standard DSSE with
   `application/vnd.in-toto+json`, like Corvint. The OpenFab v0.1 envelope is DSSE-style and not
   DSSE: it has no PAE, and the statement travels in the clear. That is a recorded deviation, and
   the draft says v0.2 intends to adopt standard DSSE.
3. `cem/v1` adopts no predicate field from either draft, and its bytes are unchanged. Both drafts
   describe a generation run (agent, model, prompt, times, approvals), which Corvint does not
   observe. Neither draft is an accepted OpenSSF or in-toto predicate. A rename would also break the
   FPK-V0-035 pinned fixture. Every deviation is listed by concept in the spec's comparison table
   (FPK-V0-051). A later adoption takes a new `predicateType` URI.
4. Sigstore gitsign, cosign, and Rekor remain an operator step outside the binary and are NOT_RUN.
   `go list -deps ./cmd/corvint` names no Sigstore-family package and nothing outside the standard
   library and the Corvint module (FPK-V0-036).

## Rollback

Delete the `--attest-cem-v1` branch, `attestCEMV1`, `cemAttestation.v1`, `attestFlagRefusal`,
`cemAttestation.statement`, the help lines, the two FPK-V0-050 tests, FPK-V0-050, and FPK-V0-051. Restore the
revision 22 UNCONFIRMED table. The `cem/0` emission and every `cem/v1` byte are unchanged either way.
