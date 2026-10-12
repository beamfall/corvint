# 2026-10-11 cem/1.0 stable additive-member regeneration (V1-1273)

Branch `claude/v1-1273-stable-regen`, from `origin/main` 2c81d1f3c.

Decision: an additive member of the sealed `cem-stable-verification/1` result lands only through
`tools/cem-stable-regen` (CEM-V1-091), never through hand edits to sealed case files. The tool
splices bytes. It inserts the member after `--after` into RESULT.schema.json (properties and
`required`), into every S0E expected result, into every envelope cell (in both the packet's
`inherited/s0e` copy and the stable copy), and into the amendment index. It then re-binds every
digest in the chain: the case files, the case and packet manifests, `TUPLE.json` inherited files,
and the stable manifest's schema, cells and prose entries. The case count, the case ids and every untouched byte stay
the same. `--check` re-verifies the chain read-only. A capture hook in
`internal/cem/verify/stable_repository_test.go` (`CEM_STABLE_REGEN_CAPTURE`) records the member's
native value per case, while still comparing all frozen keys. The Make targets are
`cem-stable-regen-capture`, `cem-stable-regen` and `cem-stable-regen-check`. They are not in
`GATE_STEPS`.

Values come from three sources:
- Captured cases take their native value.
- The two go1.24.13 cases (21, 22) cannot run on the go1.27.2 tuple, so they take `--values`.
- Each envelope cell takes the value shared by every captured case with the same exit and the same
  frozen keys, so a cell with no matching case needs an explicit `--cell-values` entry.

The tool refuses each of the following:
- matching cases that disagree
- a matched cell that also has an override
- an unmatched cell without an override
- an unknown id
- a key-order drift
- a cell list that differs between the two copies

Proof (scratch only; not part of this change): `citation` as V1-1265 would add it, run in a
throwaway worktree with the native citation patch adapted to `origin/main`.
- Regeneration exited 0, and `--check` passed: 56 cases x 22 keys.
- Values:
  - 19 of the 29 cells derived from the cases.
  - Case 21 used its native value: the block computed by `ComputeCitation` with the binding
    check bypassed on go1.27.2.
  - Case 22 was set to null (the structural refusal).
  - The 4 sha256 ACCEPT cells used the native ordinary-sha256 value. A native ordinary-sha1 run
    reproduced the captured value, which cross-checks the derivation rule.
  - The 6 repository-stage refusal cells were set to null, because `patchSha256` is null and the
    verifier returns before computing a citation.
- After 22-key bumps to six test constants, these passed:
  - `internal/cem/verify` (including TestStableRootPrecedenceBeforeReads,
    TestStableAssemblySeparateSnapshotIdentityAndFullEnvelope and 54 TestStableS0EPublicCases,
    with cases 21/22 skipped off go1.24.13)
  - `internal/cem/cli`, `internal/cemcandidate` and the `cmd/corvint` stable tests
  - interop TestStableCanonicalEnvelopeCells (29/29)

Findings for V1-1265:
- The portable consumer emits `citation: null` where the native verifier carries the block. The
  first differing key is `citation` in TestStableCanonicalFullResults, on late-cancellation and
  late-refusal cases 08, 09, 10, 11, 13, 50, 51, 53 and 54.
- `interop/cem01-go/citation_test.go` TestCEMV1082StableResultKeepsSealedShape still asserts 21
  keys and a null citation.
- 22-key constants are needed in these tests:
  - `cmd/corvint/cem_stable_test.go`
  - `internal/cem/cli/stable_test.go`, `stable_read_test.go` and `stable_report_test.go`
  - `internal/cemcandidate/stable_test.go`
  - `interop/cem01-go/stable_repository_test.go`

Independent review (Codex, read-only, 2026-10-12) returned CONCERNS; each finding was confirmed on
source and resolved in the same change:
- Decode errors in captured or frozen values were discarded, so `1e999` compared equal to `null`.
  `compareCapture` and the duplicate-id check now refuse on any decode error.
- `parseJSON` accepted duplicate object keys. It now refuses the first repeated key anywhere in
  the document, since `encoding/json` keeps the last value while the span parser keeps the first.
- The stable `manifest.json` entries for `REPOSITORY-ENVELOPE.cells.json` and
  `REPOSITORY-ENVELOPE.md` were left stale. A regeneration now re-binds them with the schema
  entry; nothing binds the stable manifest's own digest. `--check` still verifies only the schema
  entry because the committed entries have been stale since fc5588948 (V1-1287).
- `CHANGES.md` omitted the predecessor digests of re-bound files outside the packet. It now ends
  with a table of them; their new digests cannot appear because they depend on `CHANGES.md`.
- A non-object schema fragment was accepted; it is now refused.
- `--check` accepted a manifest that no longer listed a mandatory file; each manifest now requires
  its sealed files.
- `lineIndent` copied JSON syntax as indentation for compact input; it now returns none.
- Accepted limit: a `--values`-only run overrides every case, including runnable ones. The tool
  cannot know what the host can run, and `CHANGES.md` discloses the override count.

Limits:
- `--check` does not verify the stable manifest's cells and prose entries until V1-1287 repairs
  the committed digests.
- `INPUTS.json` is not touched.
