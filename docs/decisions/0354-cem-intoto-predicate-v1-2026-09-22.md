# Decision 0354 — A versioned `cem/v1` in-toto predicate that binds base and patch

Date: 2026-09-22. Status: proposed (experimental delivery, not advertised; ticket V1-0092). Adds
FPK-V0-033 to FPK-V0-036; the `cem/0` predicate and every command's emitted output are unchanged.

## Context

FPK-V0-030 attests a Change Evidence Map as an in-toto Statement v1 with predicate `cem/0`, which
binds the map's bytes, size, and spec. It does not say which base revision or patch the map covers,
so a consumer relating Corvint change evidence to a generation attestation (the OpenSSF
openfab/generation draft, ossf/tac issue 628, and agentattest) has to open the map to learn it.
The draft's field names could not be read: this work ran without network access and the
repository holds no copy of the draft or of agentattest.

## Decision

1. The predicate is versioned by its URI. `https://corvint-context.dev/attestation/cem/v1` adds
   `base` (`{"digest":{"gitCommit":OID}}`) and `patch` (`{"digest":{"sha256":HEX}}`) beside the
   `cem` ResourceDescriptor, `size`, and `spec` (FPK-V0-033). `cem/0` is not changed or retired.
2. Names are aligned only where the in-toto Statement v1, ResourceDescriptor, DigestSet, and DSSE
   names are known. Each draft concept whose name could not be confirmed is recorded as an
   UNCONFIRMED deviation in the spec's known-deviations table instead of being guessed.
3. `VerifyCEMPredicate` dispatches on a closed `predicateType` table, and
   `prove --verify-cem-attestation` uses it, so `cem/0` receipts stay byte-identical and `cem/v1`
   receipts add `baseRevision` and `patchSha256` (FPK-V0-034). Emitting `cem/v1` from the CLI is
   deferred: the emission flags live in `cmd/corvint/prove.go`, outside this change.
4. A standard-library reader in the separate `interop/cem01-go` module verifies a digest-pinned
   fixture envelope Corvint emits (FPK-V0-035). The same author wrote it, so it is not independent
   adopter evidence.
5. Sigstore gitsign, cosign, and Rekor stay an operator step outside the binary, with no Go
   dependency and no network path (FPK-V0-036, AGENTS.md invariant 7).

## Rollback

Delete `internal/attest/cem_v1.go` and its test, `interop/cem01-go/intoto.go` and its test, and
FPK-V0-033 to FPK-V0-036. Restore the `VerifyCEM` call and the two receipt members in
`cmd/corvint/prove_attest_cem.go` and the help sentence. The `cem/0` path is untouched either way.
