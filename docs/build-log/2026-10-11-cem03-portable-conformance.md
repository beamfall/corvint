# 2026-10-11 CEM 0.3 portable conformance packet (V1-0614, V1-0621)

Branch `claude/cem1-portable-03`, from `origin/main` `10efc02d1`. Scope comes from decision 0521,
which moves CEM 1.0 into rc.3. New requirements `CEM-SM-011`..`CEM-SM-018` in
`docs/specs/cem-0.3-structural-mechanical.md` are proposed and pending owner acceptance.

## Finding

The CEM 0.3 structural and witness semantics had native fixture tests only. There was no portable
vector set, no schema, and no second consumer, so nothing showed that the algorithm text alone fixes
the outcome.

Writing hand declarations before observing any result found three places where the first-draft
declarations contradicted the inherited contract. The declarations were corrected to match the
contract text; the contract was not changed:
- A CRLF-only change also proves `whitespace-only`, because the horizontal set includes 0x0d
  (cem-0.1 algorithm text).
- An unsupported mode in the canonical patch is the verification issue `unsupported-tree-mode`
  (exit 1), not an operational refusal.
- The JSON depth boundary counts the arrays nested inside one extension member. Depths 62 and 63
  reach `unknown-field`, and 64 and 65 reach `invalid-json`.

The native status verifier differs from the portable result on 17 of the 211 cases. One is
interpretive: the `CEM-SM-007` comment rule, in vector `near-miss-rename-partial-comment`, where
native accepts an unchanged comment that still names the renamed identifier. The other 16 are
code-name, placement or precedence differences, for example a non-object root, orphan or duplicate
evidence, argument-width checking, and native acceptance of `cem/0.2` and of a config-section
envelope. The first packet declared each per case in `native.note`. The follow-up below disposes
of all 17.

## Divergence disposition (follow-up, decision 0521 amendment)

- `CEM-SM-007` is strict: native `identSubstitution` now requires every non-directive comment to
  equal its whole-word substitution image and every occurrence of `from` to become `to`.
- The native single-profile reader (`CEM-SM-018`, `ReadOptions.SingleSpec`) refuses another
  spec as `unsupported-spec` and non-lowercase or mixed-width OID arguments as
  `invalid-arguments`.
- Portable `verify-03` adopts the native code name and placement for the 10 code-name and
  placement cases. `ALGORITHMS.md` already says to retain native code/issue placement.
- The mode-change case is now `malformed-patch`, which corrects the second finding above.
- `envelope-config-section` leaves the packet as a declared portable ceiling, and
  `TestCEMSM016PortableConfigCeiling` keeps the refusal on the portable side.
- New vector `json-at-map-limit`: a map of exactly 4 MiB that accepts.

## Review follow-up (Codex review of 10efc02d1..53297c9ef, three P2s)

- The packet now carries `WP1-OBSERVED.json`, a byte copy of the inherited cem-1.0 recipe pinned
  by SHA-256 `78df41fa…3630` in `ALGORITHMS.md` and `README.md`. Before this, the algorithm text
  named the recipe but the packet did not include it.
- `json-duplicate-escaped-key` was a byte-identical copy of `json-duplicate-top-level-key`. It now
  spells the duplicate as `"\u0073pec"`, so duplicate detection must run on decoded names. Both
  consumers still reject it with `invalid-json`.
- Ten text-boundary witness cases are added: survivor description at 512/513 bytes and at
  256/257 two-byte runes, description C0 and DEL, operator at 32/33 two-byte runes, and detail at
  256/257 two-byte runes.
- Failing-before: `TestCEMSM011PacketCarriesDiffRecipe`, `TestCEMSM011EscapedDuplicateKey` and
  `TestCEMSM013TextBoundaryCases` all failed against packet `cbb9b85e…818e`. They pass against
  the regenerated packet.

## Change

- `protocol/cem-0.3/` (Apache-2.0): `ALGORITHMS.md`, `cem-0.3.schema.json`, `OPERATION.json`,
  `README.md`, `LICENSE-APACHE-2.0`, and `vectors/`.
  - The vectors are 221 hand-declared cases over 3 rebuilt repositories (SHA-1, SHA-256,
    alternates) and 165 commits.
  - By category: structural 47, witness 69, wire 45, identity 31, sidecar 15, arguments 8,
    drift 6.
  - Manifest SHA-256 `9bb8393f…3931` after the review follow-up. The disposition packet was
    `cbb9b85e…818e`, and the first packet was `927abb8e…ef37`.
- `interop/cem01-go/packet03_test.go`: the generator and the independent portable consumer.
  `TestCEMSM014PacketReproducible` and `TestCEMSM016PortablePacketConformance`.
- `internal/cem/workflow/portable03_test.go`: the native consumer, which rebuilds every repository
  from the manifest. `TestCEMSM016NativePortable03Vectors`.
- V1-0621: no code change. The bypass is already closed by the non-embedded `candidateBuffer`.
  `TestCandidateBufferCopyLimits`, `TestCandidateGitDirectChildOutputLimits` and
  `Test03GitDirectChildControls` pass.
- Decision 0521, plus the spec and README rows.

## Evidence

All runs used go1.27.1 with `-p 1` and `GOMAXPROCS=3`.
- Interop `go test ./...` passes in 350s, and `go vet` passes. This includes the generator's
  byte-identical reproduction and all 221 portable cases.
- `internal/cem/...` tests pass, and `go vet ./internal/cem/...` passes.
- The native packet test passes with exact agreement on all 221 cases and no declared divergence.
  The first packet passed with 17 declared divergences.
- Failing-before for the disposition: run against the earlier native code, the exact-agreement
  native test failed on exactly the 17 cases. `TestCEMSM007RenameIsStrict` failed before the
  `identSubstitution` fix on the unchanged-comment and partial-rename images.
- On host go1.27.2, the portable conformance run gave all 44 structural-gated cases
  `unsupported-structural-runtime`/exit 2, and every other case kept its pinned result
  (`CEM-SM-017`).
- The doc gates pass.

Failing-before: the generator refused the three mis-declared cases above until each declaration
matched the contract.

## Limits

NOT_RUN:
- Go 1.24 structural parity.
- Independent external consumption (`CEM-V1-008`).
- Independent review.
- CEM dogfood bind and seal.
- The native ticket completion write (this lane was read-only).
- Device tuples other than darwin/arm64.
- Repository-wide `go test ./...` and `make gate`.

Other limits:
- The packet was generated on host go1.27.2 using the cached go1.27.1 toolchain.
- The at-limit 4 MiB map has a vector. The over-limit map is `NOT_PRODUCED`: in-process `run03`
  reports stage `wire` and the CLI reports stage `input`, so it needs a recipe or restructuring.
- The packet still pins `go1.27.1`. Retargeting it to `go1.27.2` is deferred to the Go pin change,
  which also edits the `run03` runtime gate in `interop/cem01-go/cem.go`.
- The strict rename rule also narrows the native stable verifier's rename proof, which shares
  `identSubstitution`.
- `ReadOptions.SingleSpec` has no CLI exposure; the CLI keeps the multi-version reader.
- Not every code listed in the inherited `OPERATION.json` has a vector.
