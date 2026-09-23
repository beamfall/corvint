# CEM 0.3 structural mechanical reasons

Owner: Russell Lewis
Date: 2026-09-22
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/CHANGE-EVIDENCE-MAP.md`, `docs/specs/cem-0.2-canonical-binding.md`,
`docs/decisions/0338-cem-0-3-structural-mechanical-reasons-2026-09-22.md`

## Agent digest
- Claim: CEM 0.3 adds Go rename, move, import-reorder and formatter-only mechanical reasons that the verifier re-proves from the base blob and patch alone.
- Status: proposed/experimental (ticket V1-0087, decision 0338)
- Exists: `internal/cem/verify/structural.go`, `internal/cem/wire/map.go`, `internal/cem/workflow/commands.go`.
- Blocked on: a portable `protocol/cem-0.3` vector set, a `cem-0.3.schema.json` under the Apache-2.0 boundary, and frontier/dashboard profile acceptance.
- Read next: `cem-0.2-canonical-binding.md`, `../CHANGE-EVIDENCE-MAP.md`.

## User and measurable job

A maintainer marks a hunk `mechanical` with a structural reason, and a CI consumer that runs no
model and trusts no author statement recomputes that reason from the base blob at `baseRevision`
and the canonical patch. A reason the consumer cannot reproduce fails verification with
`unproven-mechanical`; it is never accepted on the author's word.

## Verified current state

`cem/0.1` and `cem/0.2` register exactly two mechanical reasons, `whitespace-only` and
`line-ending-only`, proved from a hunk's removed and added bytes (`mechanicalProven` in
`internal/cem/verify/verify.go`). Renames, moves, import reorders and formatter runs had to stay
`unknown` or be cited as evidence-backed changes.

## Definitions

- **Structural reason**: one of `rename`, `move`, `import-reorder`, `formatter-only`.
- **Hunk image**: the base blob with one hunk applied in isolation (`spliceHunk`), or, for
  `move`, with the whole file group applied (`sim.ApplyHunks`).
- **Token stream**: the `go/scanner` tokens of a file with comments retained and automatic
  semicolons included, compared by kind and literal only, never by position.

## Requirements

- `CEM-SM-001`: `cem/0.3` MUST be the `cem/0.2` wire shape plus the four structural reasons.
  A `cem/0.1` or `cem/0.2` document carrying a structural reason MUST fail `invalid-field`;
  a `cem/0.3` document MUST keep every 0.2 obligation, including the fixed `excludedPath`.
- `CEM-SM-002`: The verifier MUST prove a structural reason from the base blob at `baseRevision`
  and the parsed patch alone. No target checkout, external command, or author statement is an
  input. A missing or non-regular base blob, a create or delete group, or an image identical to
  the base MUST refuse the claim.
- `CEM-SM-003`: For `rename`, `import-reorder`, and `formatter-only` the image MUST be the hunk
  applied in isolation so a sibling semantic hunk in the same file neither proves nor taints it.
- `CEM-SM-004`: For `move` the image MUST be the whole group applied, because a move is a paired
  removal and insertion that no single hunk can prove.
- `CEM-SM-005`: Any parse, scan, or format failure on the base or the image MUST refuse the
  claim with `unproven-mechanical`; the verifier never falls back to a weaker proof.
- `CEM-SM-006`: `mark --disposition mechanical` with a structural reason MUST upgrade a `cem/0.2`
  map to `cem/0.3` and MUST refuse a `cem/0.1` map with `invalid-arguments`. `prepare` keeps
  emitting `cem/0.2`, so maps without structural reasons are byte-identical to before.
- `CEM-SM-007`: `rename` MUST hold only when the token streams differ by one identifier pair
  `from -> to` and comments differ only by whole-word substitution of that pair; both names are
  unexported, `to` does not occur in the base file, `from` is declared in the file, and `from`
  never appears as a selector, composite-literal key, struct field, interface or concrete method,
  or package name. Directive comments (`//go:`, `//line `, `//export `) MUST be unchanged.
- `CEM-SM-008`: `move` MUST hold only when tokens outside top-level declarations are unchanged,
  the declarations are the same multiset in a different order, and every order-sensitive
  declaration (top-level `var` and `init`) keeps its relative order.
- `CEM-SM-009`: `import-reorder` MUST hold only when the ordered import specs differ, their
  name-and-path multiset and comments are unchanged, and every token outside the import
  declarations is unchanged.
- `CEM-SM-010`: `formatter-only` MUST hold only when `go/format` renders the base and the image
  to identical bytes.

## Non-goals and simpler baseline

Cross-file or type-aware renames, exported renames, languages other than Go, generated output,
lockfile updates, and file moves across paths stay out of scope; they remain `unknown` or
evidence-cited. The simpler baseline was to keep the two byte proofs and cite formatter runs as
evidence; it was rejected because reviewers cannot audit large reorders by hand. No schema file
ships in this change because `docs/cem-*.schema.json` sits inside the Apache-2.0 boundary that
`LICENSING.md` enumerates.

## Trust boundary limits and failure modes

- A structural predicate is a sufficient proof for its class, not an exclusive label: gofmt sorts
  imports, so an import-reorder image is also `formatter-only`. Both are mechanical.
- `rename` is file-local. A renamed unexported package-level name used by a sibling file compiles
  differently; the file-local proof still accepts it. That is the accepted 0.3 limit and the
  reason exported names are refused.
- `move` accepts reordering of `const` and `type` declarations without restriction; `iota`
  sequences live inside one declaration, so reordering declarations does not change them.
- Unparsable Go, non-Go files, or a truncated blob fail closed to `unproven-mechanical`.

## Deterministic acceptance and testing matrix

| Case | Input | Expected |
|---|---|---|
| true positive per class | `testdata/structural/<class>/{base,positive}.txt` | proven |
| near-miss per class | `testdata/structural/<class>/{base,nearmiss}.txt` | refused |
| cross-class claim | one class's positive claimed as another | refused (import-reorder as formatter-only is the admitted overlap) |
| rename via selector, exported, to-existing, directive | inline sources | refused |
| move reordering `var` or `init` | inline sources | refused |
| isolated hunk beside a semantic hunk | two-hunk group | non-move hunk proven; semantic hunk and `move` refused |
| workflow end to end | `prepare`, `mark import-reorder`, commit, `status` | spec `cem/0.3`, valid, one mechanical hunk |
| workflow wrong class | `mark rename` on an import reorder | `valid:false`, `unproven-mechanical` |
| profile gating | 0.3 accepts, 0.2 and 0.1 refuse structural reasons | `invalid-field` |

## Rollout, rollback and compatibility

`cem/0.1` and `cem/0.2` documents parse and verify exactly as before; `cem/0.3` is only produced
when an author marks a structural reason. Rollback removes `Spec03` from `wire.ParseMap`, after
which every `cem/0.3` map is refused as an unsupported spec while committed 0.2 maps stay valid.
Frontier and dashboard profile sets still pin 0.1 and 0.2 and will refuse a 0.3 map until they
are widened under their own specs.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `CEM-SM-001` | `internal/cem/wire/map.go` | `TestSpec03StructuralReasons` |
| `CEM-SM-002` | `internal/cem/verify/structural.go` `structuralProven` | `TestStructuralProvenFromBaseBlobAndPatch`, `TestStructuralProvenRefusesCreateAndDelete` |
| `CEM-SM-003` | `spliceHunk` | `TestStructuralProvenIsolatesHunksExceptMove` |
| `CEM-SM-004` | `structuralImage` with `sim.ApplyHunks` | `TestStructuralProvenIsolatesHunksExceptMove` |
| `CEM-SM-005` | `parseBoth`, `formatterOnlyProven` | `TestStructuralRefusesUnprovableInputs` |
| `CEM-SM-006` | `internal/cem/workflow/commands.go` `Mark` | `TestMarkStructuralReasonUpgradesToSpec03AndVerifies`, `TestMarkStructuralReasonWrongClassIsRefused`, `TestMarkStructuralReasonRefusedOnSpec01` |
| `CEM-SM-007` | `renameProven` | `TestStructuralTruePositives`, `TestStructuralNearMisses`, `TestStructuralRefusesUnprovableInputs` |
| `CEM-SM-008` | `moveProven` | `TestStructuralTruePositives`, `TestStructuralNearMisses`, `TestStructuralRefusesUnprovableInputs` |
| `CEM-SM-009` | `importReorderProven` | `TestStructuralTruePositives`, `TestStructuralNearMisses` |
| `CEM-SM-010` | `formatterOnlyProven` | `TestStructuralTruePositives`, `TestStructuralNearMisses` |

## Unresolved decisions

- Whether `protocol/cem-0.3` portable vectors and `docs/cem-0.3.schema.json` ship before or
  after independent interop evidence.
- Whether a `mark`-time pre-check should refuse a structural reason the verifier will not prove,
  instead of leaving the refusal to `status`.
