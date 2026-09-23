# Decision 0343 — Freeze the OCM V0 wire with producer-derived conformance vectors

Date: 2026-09-22. Status: accepted (wave brief for ticket V1-0013; assigned number 0327 was
already taken by the v0.5.0a1 publication record, so this record takes the next free number).
Adds `OCM-V0-014` to `docs/specs/ocm-v0-dogfood.md`; changes no wire byte or refusal code.

## Context

Ticket V1-0013 asks for the minimum portable proof wire to be frozen before the 0.9 release
candidate. `ocm/0.1-experimental` had a wire profile and per-package tests in `internal/lrfrepo`,
but no directory that pins the exact producer bytes, the canonical encoding, every disposition and
`unknown` reason, and the refusal code for each hostile input in one place, the way
`conformance/frontier-v0/` does for `frontier/0`. A wire change could therefore land as a passing
unit-test edit without a spec entry.

The frontier suite is hand-authored from clause text because `CF-V0-028` demands an independent
consumer. The OCM suite has a different job: notice drift in what Corvint's own producers emit and
what its parser and verifier refuse. Hand-authoring the valid bytes would only re-derive what the
producers already print, while a producer-derived vector that is re-derived on every run proves
determinism as well as freezing the bytes.

## Decision

- `conformance/ocm-v0/` freezes 25 structural vectors (5 valid, 20 hostile) and 4 fixtures with
  19 verifier cases, listed in `manifest.json` against the wire-profile clauses they cover.
- Valid vectors are the byte output of the real `prepare`, `link`, and `mark` producers over a
  deterministic seed repository (`universe.go`, pinned Git identity and dates). The producer test
  rebuilds each universe and requires byte equality, so the same data proves determinism.
- Every vector and fixture case runs against the real structural parser and the real `status`
  verifier through two small seams (`adapter.go`); no double stands in for the implementation.
- Two observed outcomes are frozen as the contract rather than changed: an extra top-level member
  refuses with `unknown-field` (closed object, no preservation path), and a bound OID absent from
  the repository refuses with `repository-object-unavailable` ahead of `target-mismatch`.
- A drift in any frozen byte or code is a wire change: the spec records it first, then the data is
  refrozen. The internal OCM package was not modified; no vector exposed a defect.
