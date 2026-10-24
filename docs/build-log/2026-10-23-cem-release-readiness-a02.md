# Release readiness reads the exact cem/1.0 stable result (V1-2323, MIGRATION A02)

Date: 2026-10-23. Ticket: V1-2323, criteria 1 and 3, plus the local part of criterion 2.
Spec: `docs/specs/cem-stable-v1.md` CEM-V1-569..572, proposed. Nothing is promoted.

## Decisions

- **The reader rides on the native result type.** `ValidateStableReleaseEvidence` decodes the result
  into `verify.StableResult` with unknown members refused, and recomputes `verdictSha256` with
  `VerdictDigest`. That digest is canonical and does not depend on member order. A byte-for-byte
  re-encoding was rejected: the native writer emits typed reference records in their normative
  nested order, which plain `encoding/json` over decoded maps cannot reproduce. Member order is
  checked separately, at the top level, against the 23 `RESULT.schema.json` members.
- **Admission is read from the result, not supplied by the caller.** An `UNSUPPORTED` outcome, a
  structural tuple other than `NOT_REQUIRED` or `QUALIFIED_EXACT_TUPLE`, an envelope other than
  `canonical-repository-bounded/1` and `UNSUPPORTED_RUNTIME` are refused before the outcome
  checks. The verdict digest excludes `runtime` and `repositoryEnvelope`, so a result with an
  experimental tuple keeps a valid verdict and passing reference integrity. The test pins exactly
  that case.
- **`authentication` follows `ALGORITHMS.md`, not the schema const.** `RESULT.schema.json` pins
  every authority axis to `NOT_OBSERVED`, but CEM-V1-084 lets a consumer `--trusted-keys` file move
  `authentication`, and the public receipt-authentication case returns `VERIFIED`. The reader
  accepts the four values the verifier emits. The schema mismatch is left for the owner.
- **Runner, platform and external outcomes are caller-declared and closed.** The stable result
  never carries them, so each class is `{Required, Observed}`. Only `PASSED` counts, and an empty
  declaration is refused instead of being read as complete. No repository source yet says which
  rows a release requires.
- **The suite runner drives the binaries, not the sources.** `tools/cem-release-suites` reads Go
  build information (`vcs.revision`, `vcs.modified`) from both binaries and requires the checkout
  to be at the same commit, so the sealed suites it reads are the binaries' own. Each case runs
  through `corvint cem verify-stable` and `cem01-go verify-stable-canonical`, and the two result
  lines must match byte for byte apart from `runtime`. Repository-envelope cells are prose S0E
  recipes rebuilt by in-process harnesses. The target runs them at source level only and labels
  them as not run against the binaries.
- **No read scope was declared for `internal/companionrelease`.** Adding an entry to
  `.corvint/test-read-scopes.json` would have put the whole package under the CI Landlock wrapper
  (AFP-V0-023) and required declaring 14 existing reads. The package stays unscoped, and
  `make test-read-scopes-check` passes.

## Observed

- `TestCEMV1569StableReleaseReadiness`: 1 accepted case and 20 refusals, all PASS.
- `TestCEMV1572ReleaseBuildsMustMatch`: 7 cases PASS.
- Commit `2e59e569d` was built in a clean worktree with go1.27.2.
  `make cem-release-suites CORVINT_RELEASE_BIN=... CEM01_RELEASE_BIN=...` exited 0, with
  tamper-matrix 23, citation-vectors 8, neutral-criterion 8 and receipt-authentication 19 cases
  PASS on both binaries. Natively, `TestStableS0EPublicCases` (54 cases),
  `TestStableS0ELinkedControl`, `TestStableCanonicalEnvelopeCellsNative` and
  `TestStableEnvelopeAdversarialShapes` passed; in `interop/cem01-go`,
  `TestStableCanonicalEnvelopeCells` and `TestStableEnvelopeAdversarialShapes` passed.
- With corvint from `2e59e569d` and cem01-go from `368b80d43`, the run was refused with
  `builds differ: corvint 2e59e569…, cem01-go 368b80d4…` and exit 1. A build from a modified
  worktree was refused with `built from a modified worktree`.
- `MIGRATION.json` changed from sha256 `65258c13…01b4` (32369 B) to `5e630d9e…304b` (33075 B). Only
  its entry in `protocol/cem-1.0/stable/manifest.json` was re-pinned. `manifest.json` is not pinned
  elsewhere, the frozen `repository-envelope-packet/inherited/s0e/MIGRATION.json` copy is
  unchanged, and `make cem-stable-regen-check` passes.
