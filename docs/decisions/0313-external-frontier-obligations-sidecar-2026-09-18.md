# Decision 0313 — External obligations join the Change Frontier as a reference-only sidecar

Date: 2026-09-18. Status: accepted (delegated call on the owner's own feature request
Beamfall/corvint#1; owner review of the intent is welcome and changes only the digest line).

## Decision

The "optional, versioned extension for external obligations and impact references" that
Beamfall/corvint#1 asks for is delivered as a separate document, not a CEM or frontier field:
`corvint obligations --cem FILE --impact FILE` writes `external-frontier-obligations/0`, specified
in `docs/specs/external-frontier-obligations-v0.md` (`EFO-V0`).

- **Why a sidecar.** `CEM-CB-002` rejects unknown CEM fields and `CF-V0-018` closes the frontier's
  top-level keys, so either wire would have to change to carry provider evidence. Decision 0309
  already refused that; this record keeps it refused.
- **Citation direction.** The frontier does not read the sidecar. The sidecar binds to the frontier:
  `binding.cem_sha256` is the same raw-copy digest `CF-V0-019` uses for `inputs.cemSha256`, and
  each row's `hunk` is the CEM hunk id the frontier's `relatedIds` cite. A reviewer joins the two by
  identity, and the frontier's state, exit code, and bytes are unchanged with or without it.
- **Key.** The sidecar is keyed by `sha256(raw impact receipt)` and `sha256(raw CEM)`, and its own
  `id` is the digest of its canonical body. The receipt is the saved stdout of
  `corvint impact --provider`; the verb reads two files and no repository.
- **Reference-only.** Every association carries the provider evidence reference, EEP path
  verification, and provider freshness, and no association carries a closure, justification, score,
  or confidence member. Explicit unknowns are coded: `no-external-evidence`, `provider-<state>`,
  `no-external-section`.

## Alternatives weighed

- *A `--obligations FILE` option on `corvint frontier`*: the wire has no member to carry the result
  and `CF-V0-026` makes the human rendering the same computation, so the option could only change
  what the frontier reads, which is exactly the authority leak the request forbids.
- *An `external.obligations` member inside the impact receipt*: the receipt has no hunk identity,
  and the ideas backlog asked for a document keyed by the receipt's digest, which a member of the
  receipt cannot be.
- *Verifying the CEM inside the verb*: a second CEM verifier beside the frontier's shared one. The
  sidecar binds the raw bytes instead and leaves verification to the frontier.

## Rollback

Remove `internal/extevidence/obligations.go`, `cmd/corvint/obligations.go`, their tests, the verb's
help text and `topLevelCommands` entry, the spec, its index rows, and this record. No other wire
changes.
