# CEM 0.3 structural mechanical reasons

Owner: Russell Lewis
Date: 2026-09-22
Intent status: retired by decision 0525, superseded by `docs/specs/cem-stable-v1.md`
Delivery status: superseded
Retirement note: V1-1278 S3 (`CEM-V1-220`, `CEM-V1-221`, `CEM-V1-225`) deleted `protocol/cem-0.3`
and the portable 0.3 tests; the body below is historical and its requirement IDs stay for traceability.
Authoritative successor: `docs/specs/cem-stable-v1.md`
Authoritative inputs: `docs/CHANGE-EVIDENCE-MAP.md`, `docs/specs/cem-0.2-canonical-binding.md`,
`docs/decisions/0338-cem-0-3-structural-mechanical-reasons-2026-09-22.md`

## Agent digest
- Successor: `docs/specs/cem-stable-v1.md` is the authoritative replacement.
- Why: decision 0525 removes `cem/0.3`; S3 deleted `protocol/cem-0.3` and kept its algorithms text as `protocol/cem-1.0/stable/inherited/cem-0.3-algorithms.md`, which `cem/1.0` structural reasons inherit.

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
  (proposed, decision 0398) The upgraded map MUST NOT replace its `cem/0.2` input in place and MUST NOT be
  written to `.corvint/change.cem.json`, the Core sidecar that frontier and OCM read as `cem/0.2`
  only (`CF-V0-001`); `mark`, `cover` and `discriminate` refuse either with `invalid-arguments`,
  leave the input unchanged, and need `--output` naming another path. A `cem/0.3` map is therefore
  an experimental local copy (`CCF-V1-003`): it verifies against a target that commits no sidecar,
  and a target that commits the `cem/0.2` sidecar fails it with `excluded-artifact-mismatch`.
- `CEM-SM-007`: `rename` MUST hold only when the token streams differ by one identifier pair
  `from -> to`, every occurrence of `from` becomes `to`, and every non-directive comment equals
  its whole-word substitution image, so an unchanged comment that still names `from` is not a
  proven rename (decision 0521); both names are
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
- `CEM-SM-011`: The portable packet `protocol/cem-0.3` (Apache-2.0) MUST contain the algorithm
  text, the shape schema, the `verify-03` operation contract and `vectors/manifest.json`. The
  manifest MUST pin every raw map by SHA-256 and describe SHA-1 and SHA-256 repositories whose
  commits a consumer rebuilds from literal bytes with Git plumbing alone, requiring each exact
  tree and revision. Both consumers MUST pin the manifest digest and check every artifact before
  executing any case.
- `CEM-SM-012`: The packet MUST include a literal positive for each structural reason and each
  byte mechanical reason, and the admitted overlaps (`formatter-only` on an import reorder,
  `whitespace-only` on a CRLF-only change). It MUST include semantic near-misses for every
  refusal rule in `CEM-SM-002`, `CEM-SM-005` and `CEM-SM-007`..`CEM-SM-010`, and cross-class
  claims. Each case's outcome, stage and code MUST be declared by hand, not captured.
- `CEM-SM-013`: The packet MUST include coverage and discriminates witness cases at every
  declared byte, character, range, arithmetic and state boundary of the algorithm text. Accepted
  witnesses MUST be retained by hunk ID in the result and MUST NOT change any hunk identity.
- `CEM-SM-014`: A checked-in generator MUST regenerate the packet byte for byte from its declared
  case table on the qualified runtime. It MUST refuse to write a packet when any observed outcome,
  stage, code or drift status differs from the declaration.
- `CEM-SM-015`: The packet MUST distinguish the sidecar at each side:
  - Base: absent or `100644` passes. Any other kind is `excluded-path-not-file`.
  - Target: absent, or a `100644` blob equal to the raw input map bytes, passes. A trailing
    newline, a historical 0.2 sidecar, an executable bit, a symlink, a tree or a gitlink is
    `excluded-artifact-mismatch`.
- `CEM-SM-016`: For every case, the standalone `verify-03` process MUST reproduce the pinned
  exit code and full result, apart from `runtime.goVersion`. The native status verifier MUST
  reach the projection of that result on every case: ACCEPT is valid, a binding, verification or
  drift REJECT is the first issue code, and a wire REJECT or any UNSUPPORTED is the error code. No
  per-case divergence is admitted. The portable repository envelope
  `primary-clean-config-bounded/1` is a declared implementation ceiling, tested only on the
  portable side and not a packet case.
- `CEM-SM-017`: On any runtime outside the admitted structural tuples (`go1.27.1`, the packet's
  qualified runtime, and `go1.27.2` under decision 0522 and `CEM-V1-050`), a case that claims a
  structural reason MUST yield UNSUPPORTED at stage `runtime` with
  `unsupported-structural-runtime` and exit 2, never a byte-proof fallback. Every other case MUST
  keep its pinned result on every runtime.
- `CEM-SM-018`: The native reader MUST offer a single-profile mode (`ReadOptions.SingleSpec`).
  In it, a map whose `spec` is another supported version is `unsupported-spec` at the wire
  stage, before the key-set check. The base and target arguments MUST be full lowercase commit
  OIDs of one width, else `invalid-arguments`. Without it the multi-version reader is unchanged and still admits
  `--target HEAD`. The mode has no CLI exposure.

## Non-goals and simpler baseline

Cross-file or type-aware renames, exported renames, languages other than Go, generated output,
lockfile updates, and file moves across paths stay out of scope; they remain `unknown` or
evidence-cited. The simpler baseline was to keep the two byte proofs and cite formatter runs as
evidence; it was rejected because reviewers cannot audit large reorders by hand. The shape schema
ships in `protocol/cem-0.3/cem-0.3.schema.json`, inside the Apache-2.0 `protocol/**` boundary,
and not as a `docs/cem-*.schema.json` file.

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
| portable packet | `protocol/cem-0.3/vectors` (221 cases) through `verify-03` and native single-profile `status` | pinned result per case; native equals its projection on all 221 |
| config ceiling | sha1 repository with a `[gc]` config section | portable UNSUPPORTED `unsupported-repository-envelope` |
| runtime gate | structural cases on a runtime that is not an admitted structural tuple | UNSUPPORTED `unsupported-structural-runtime`, exit 2 |

## Rollout, rollback and compatibility

`cem/0.1` and `cem/0.2` documents parse and verify exactly as before; `cem/0.3` is only produced
when an author marks a structural reason. Rollback removes `Spec03` from `wire.ParseMap`, after
which every `cem/0.3` map is refused as an unsupported spec while committed 0.2 maps stay valid.
Frontier and dashboard profile sets still pin 0.1 and 0.2 and will refuse a 0.3 map until they
are widened under their own specs.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `CEM-SM-001` | `internal/cem/wire/map.go` | `TestInheritedStructuralReasons` (since V1-1278 S2b the structural reasons are the vocabulary `cem/1.0` inherits; the `cem/0.3` spec gate is retired, `CEM-V1-215`) |
| `CEM-SM-002` | `internal/cem/verify/structural.go` `structuralProven` | `TestStructuralProvenFromBaseBlobAndPatch`, `TestStructuralProvenRefusesCreateAndDelete` |
| `CEM-SM-003` | `spliceHunk` | `TestStructuralProvenIsolatesHunksExceptMove` |
| `CEM-SM-004` | `structuralImage` with `sim.ApplyHunks` | `TestStructuralProvenIsolatesHunksExceptMove` |
| `CEM-SM-005` | `parseBoth`, `formatterOnlyProven` | `TestStructuralRefusesUnprovableInputs` |
| `CEM-SM-006` | `internal/cem/workflow/commands.go` `Mark`, `checkSpec03Output`, `writeMap` (every writer, V1-0384) | retired by V1-1278 S2b (`CEM-V1-215`): `checkSpec03Output` and the `cem/0.3` upgrade are deleted, so their tests are deleted; `TestMarkRefusesARetiredMapBeforeMutation` holds the retired-map refusal and `TestCEMV1143StableProducerMutationsStayStable` the stable mark |
| `CEM-SM-007` | `renameProven`, `identSubstitution` | `TestStructuralTruePositives`, `TestStructuralNearMisses`, `TestStructuralRefusesUnprovableInputs`, `TestCEMSM007RenameIsStrict` (the native 0.3 vector test was retired by V1-1278 S2b (`CEM-V1-215`)) |
| `CEM-SM-008` | `moveProven` | `TestStructuralTruePositives`, `TestStructuralNearMisses`, `TestStructuralRefusesUnprovableInputs` |
| `CEM-SM-009` | `importReorderProven` | `TestStructuralTruePositives`, `TestStructuralNearMisses` |
| `CEM-SM-010` | `formatterOnlyProven` | `TestStructuralTruePositives`, `TestStructuralNearMisses` |
| `CEM-SM-011` | `protocol/cem-0.3`, `interop/cem01-go/packet03_test.go` `readPacket03`, `internal/cem/workflow/portable03_test.go` | retired by V1-1278 S3 (`CEM-V1-220`): `protocol/cem-0.3`, `verify-03` and the portable 0.3 packet tests are deleted; the tests are kept at commit `5a056d0a` |
| `CEM-SM-012` | `cases03` structural rows | retired by V1-1278 S3 (`CEM-V1-220`): `protocol/cem-0.3`, `verify-03` and the portable 0.3 packet tests are deleted; the tests are kept at commit `5a056d0a` |
| `CEM-SM-013` | `cases03` witness rows | retired by V1-1278 S3 (`CEM-V1-220`): `protocol/cem-0.3`, `verify-03` and the portable 0.3 packet tests are deleted; the tests are kept at commit `5a056d0a` |
| `CEM-SM-014` | `generatePacket03` | retired by V1-1278 S3 (`CEM-V1-220`): `protocol/cem-0.3`, `verify-03` and the portable 0.3 packet tests are deleted; the tests are kept at commit `5a056d0a` |
| `CEM-SM-015` | `cases03` sidecar rows | retired by V1-1278 S3 (`CEM-V1-220`): `protocol/cem-0.3`, `verify-03` and the portable 0.3 packet tests are deleted; the tests are kept at commit `5a056d0a` |
| `CEM-SM-016` | `verify-03` (`decode03`, `run03`), `portable03Projection` | retired by V1-1278 S3 (`CEM-V1-220`): `protocol/cem-0.3`, `verify-03` and the portable 0.3 packet tests are deleted; the tests are kept at commit `5a056d0a` |
| `CEM-SM-017` | `run03` runtime stage | retired by V1-1278 S3 (`CEM-V1-220`): `protocol/cem-0.3`, `verify-03` and the portable 0.3 packet tests are deleted; the tests are kept at commit `5a056d0a` |
| `CEM-SM-018` | `internal/cem/wire/map.go` `ParseMapSpec`, `internal/cem/workflow/read.go` `ReadOptions.SingleSpec` | retired by V1-1278 S2b (`CEM-V1-215`): `ParseMapSpec` and `ReadOptions.SingleSpec` are deleted with the `cem/0.x` wire, and so is their native vector test |

## Unresolved decisions

- The packet and schema now ship as `proposed` in `protocol/cem-0.3`, before independent interop
  evidence (decision 0521). Freezing them still needs owner acceptance.
- Whether a `mark`-time pre-check should refuse a structural reason the verifier will not prove,
  instead of leaving the refusal to `status`.
