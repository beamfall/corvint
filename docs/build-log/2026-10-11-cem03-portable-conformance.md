# 2026-10-11 CEM 0.3 portable conformance packet (V1-0614, V1-0621)

Branch `claude/cem1-portable-03`, from `origin/main` `10efc02d1`. Scope comes from decision 0521,
which moves CEM 1.0 into rc.3. New requirements `CEM-SM-011`..`CEM-SM-017` in
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
envelope. Each is declared per case in `native.note` and stays open. The algorithm text says a
native/portable mismatch blocks qualification.

## Change

- `protocol/cem-0.3/` (Apache-2.0): `ALGORITHMS.md`, `cem-0.3.schema.json`, `OPERATION.json`,
  `README.md`, `LICENSE-APACHE-2.0`, and `vectors/`.
  - The vectors are 211 hand-declared cases over 4 rebuilt repositories (SHA-1, SHA-256,
    alternates, config-section) and 168 commits.
  - By category: structural 47, witness 59, wire 44, identity 31, sidecar 15, arguments 9,
    drift 6.
  - Manifest SHA-256 `927abb8e…ef37`.
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
  byte-identical reproduction and all 211 portable cases.
- `internal/cem/...` tests pass, and `go vet ./internal/cem/...` passes.
- The native packet test passes, with 17 declared divergences.
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
- Repository-wide `go test ./...`.

Other limits:
- The packet was generated on host go1.27.2 using the cached go1.27.1 toolchain.
- The 4 MiB map-size limit has no vector, because of packet size.
- Not every code listed in the inherited `OPERATION.json` has a vector.
