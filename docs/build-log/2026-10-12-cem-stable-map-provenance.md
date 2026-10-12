# 2026-10-12 CEM stable map provenance, producer-only members and role (V1-1265)

Branch `claude/v1-1265-map-schema` from 056b11643. Decision 0525 §3 step 3 and §4; requirements
`CEM-V1-093..095` (proposed) in `docs/specs/cem-stable-v1.md`.

## Decisions

- `provenance` is an optional closed object of five required members (`sourceSpec`,
  `sourceBlobOid`, `sourceSha256`, `bindCommit`, `converter`). A non-object is `invalid-field`;
  inside it the order is missing, unknown, invalid, in member order. It is never an input to
  authority, ranking, any axis or verdict, and is never echoed; it only changes `mapSha256`.
- `role` is an optional closed enum `change` | `bind-chore` | `seal-chore`; absence means
  `change`. Any other value is `invalid-field`. It never changes a verdict or tier. Reader grouping
  (`cem status --citation`) is left to V1-1281; whether the converter writes `role` is left to
  V1-1275 and V1-1281.
- Root order: required missing, then any unknown root key, then required members in schema order,
  then `provenance`, then `role`, then the reference graph. Both decoders implement the same order.
- Producer-only members (`CEM-V1-094`), a closed list: `criterionBindings`, `runnerReceipts`
  (with any receipt `authentication`), `criterionLinks`, `artifacts` (with any
  `receipt-attestation`). There is no intent/scope member and no structural witness member:
  structural reasons are recomputed from Git, and hunk `coverage`/`discriminates` are source
  witnesses carried unchanged, never synthesized.
- Re-homing: `protocol/cem-1.0/stable/ALGORITHMS.md` restates the 0.1 algorithms and the 0.3 hunk
  vocabulary, structural reasons (CEM-SM-001..010 as references) and witnesses as 1.0-normative
  text, and names the 13 `$defs` the stable schema restates from `inherited/cem-0.3.schema.json`
  (checked equal as JSON values). `inherited/` stays byte-identical as historical provenance.
- Provenance OIDs are 40-hex only, matching the SHA-1 repositories the converter targets. A
  SHA-256 repository form would be an additive change.

## Manifest re-binds (`protocol/cem-1.0/stable/manifest.json`)

- `stable.schema.json`: `bad81a27…d5a8` 19250 -> `7f62c445…afe3` 20751.
- `ALGORITHMS.md`: `55e65176…bd24` 23726 -> `370fa6b2…bd83` 35066.
- `README.md`: `ca656534…9d4d` 3298 -> `20e7f56b…d56c` 3517.

The only other manifest mismatches are the pre-existing `REPOSITORY-ENVELOPE.cells.json` and
`REPOSITORY-ENVELOPE.md` entries owned by the result-shape lane; they were not touched.

## Finding: unlinked-criterion code divergence (pre-existing, not changed)

A map whose criterion or receipt is declared but not linked (for example `criterionLinks: []` with
the other three arrays nonempty) is refused `reference-coherence` by the native decoder
(`internal/cem/wire/stable.go`, used-criteria/used-receipts check) and `unresolved-reference` by
the portable decoder (`interop/cem01-go/cem.go`, linked-criteria check). `ALGORITHMS.md` CEM-V1-017
requires complete linking but names no code. The shared vectors avoid this case and use
`artifacts: []` (`artifact-role-closure` on both). The divergence needs its own ticket.

## Verification

- `go build ./...`, `go vet ./...`, `make cem-stable-regen-check`: PASS.
- `go test -count=1 ./internal/cem/... ./internal/lrfrepo/... ./internal/cemcandidate/...`: PASS
  except three `internal/cem/gitrun` descendant-cleanup tests that timed out under concurrent host
  load; that package is unchanged here and passed on an isolated rerun.
- `interop/cem01-go`: `go test -count=1 ./...` and `go vet ./...` PASS.
- New tests: `TestCEMV1093StableMemberVectors`, `TestCEMV1093ProvenanceDecoded`,
  `TestCEMV1093SchemaMembers`, `TestCEMV1094ProducerOnlyEmpty`,
  `TestCEMV1094ProducerOnlyEmptyVerifyAccept`, `TestCEMV1095Role` (`internal/cem/wire`);
  `TestCEMV1093PortableStableMemberVectors`, `TestCEMV1094PortableProducerOnlyEmptyAccept`,
  `TestCEMV1095PortableRoleAndProvenanceDecoded` (`interop/cem01-go`). 50 shared vectors
  (10 accepts, 40 refusals) give the same code on both consumers.
