# Decision 0222 — Every ACP candidate binds UNKNOWN_FIELD to a safe canonical extension

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/specs/analyzer-candidate-profiles.md` requires an otherwise safe canonical extension to
produce the bound `UNKNOWN_FIELD` envelope for every family, and keeps `UNKNOWN_FIELD` off the
sentinel reason list. Decision 0221 (b) applied that rule to Go only. Seven other candidates
still broke it:

- Python and Kotlin/Android emitted an unbound `UNKNOWN_FIELD` sentinel.
- The shader candidate emitted the `NONCANONICAL_REQUEST` sentinel for any unknown member
  (decision 0213 (b)).
- JavaScript/TypeScript emitted an unbound `UNKNOWN_FIELD` sentinel.
- .NET, Ruby, and shell emitted the `MALFORMED_INPUT` sentinel.

The call: decision 0221 (a) placement and (b) precedence apply unchanged to the `python`,
`kotlin-android`, `shader`, `javascript-typescript`, `dotnet`, `ruby`, and `shell` candidates.

- A request is the bound `UNKNOWN_FIELD` rejection when three things hold: its only defect is
  extension members placed per 0221 (a), its remaining bytes are the canonical schema encoding,
  and that encoding passes the candidate's own sentinel (echo) checks.
- If those checks fail, the response is their listed sentinel.
- Any other request with unknown members is the `NONCANONICAL_REQUEST` sentinel.
- A request that fails to decode for a reason other than unknown members keeps its existing
  reason.

This supersedes decision 0213 (b) for unknown members only; a type mismatch in the shader candidate
is still `NONCANONICAL_REQUEST`. The Swift/Apple candidate (`SAC-002`) is not covered. Whether the
extension rule applies to its fixed three-input envelope is filed in `docs/agent-memory/ideas.md`.

Consequences:

- Each changed package has a `TestSafeCanonicalExtensionBindsUnknownField` that failed before the
  change.
- Vectors that expected the old reason for a safe extension now expect the bound `UNKNOWN_FIELD`:
  - Kotlin CLI and ratchet
  - JavaScript `TestRunPreservesStagedIdentityOnEnvelopeRejection`, whose frame now uses an unsafe
    `1.5` extension
  - Ruby `TestEnvelopeRejectsDuplicateFieldAndUnknownField`, likewise
  - the shell JSON depth and token witnesses
- The JavaScript candidate held its ACP-009 ceiling by removing dead and duplicated code with no
  behavior change.

Rollback: revert the commit. That restores each candidate's previous unknown-member reason and the
pre-amendment spec text.
