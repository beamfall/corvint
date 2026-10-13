# 2026-10-13 Shared stable tamper matrix run by both consumers (V1-1340)

Branch `claude/v1-1340-stable-tamper-matrix`, from `origin/main` `94a7d3dc` (after V1-1278 S3, so
`cem/1.0` is the only wire). Requirements `CEM-V1-226..229` (new, proposed) in
`docs/specs/cem-stable-v1.md`, under the owner's standing auto-accept of lane requirements
(2026-10-11).

## Intent

A test-conformance panel found that no native test on the stable path covered the classic tamper
classes (patch digest mismatch, fabricated evidence ID, uncited hunk, invalid span, span out of
range). The native tamper suites went through the `cem/0.1` path that V1-1278 removed, and the
portable consumer kept its own separate cases. This lane adds one tamper matrix to the
`cem/1.0` packet. Both consumers load the same files and assert the same pinned verdicts.

## Decisions

- **Placement.** The matrix is `protocol/cem-1.0/stable/tamper-matrix/`, beside the other public
  stable cases (`neutral-criterion/`, `receipt-authentication/`), using the same recipe-repository
  shape. Its 23 files are added to the stable `manifest.json`; no existing packet file changes.
  The status label is the packet's own `PROPOSED / EXPERIMENTAL`; no frozen V1-1333 wording is
  touched.
- **Fixture.** The two-commit greeting repository has a `SPEC.md` that defines `GRT-001` and
  `GRT-002`. Each target hunk names its requirement ID, and each cited line names the hunk's file.
  The clean map is therefore `anchored` on both hunks (anchors `id` and `path`, specificity 2), so
  a tamper that weakens the citation shows up as a tier change. The clean map came from
  `corvint cem prepare` plus `cem cite`. The 19 tampered maps were built once from it by a scratch
  script, with each evidence ID recomputed only where the case description says so. The script is
  not committed: the committed maps and `EXPECTED.json` are the contract.
- **Expectation shape (`CEM-V1-228`).** Each case pins `exit`, `outcome`, `stage`, `code`,
  `issueCodes` and `citation`. A REJECT pins the closed code and a null citation. An ACCEPT pins
  the whole `cem-citation-support/0` block, so per-hunk tiers, anchors and `specificity` are exact.
  Both tests also check the required tamper classes, so the matrix cannot quietly lose one.
- **Swap disposition (`CEM-V1-229`): ACCEPT with a degraded tier, not REJECT.** After the swap,
  each hunk cites the other hunk's requirement line. Each evidence record is still a real base-blob
  span whose ID and `spanSha256` verify, so integrity holds. `CEM-V1-080` says a support tier MUST
  NOT change the verdict, codes or drift. So the spec has no rule that refuses a well-formed but
  misdirected citation. Its only signal is the consumer-derived tier, which falls from `anchored` 2,
  specificity 2 to `cited` 2, specificity -2. A REJECT would need a new requirement that turns tiers
  into a verdict, which `CEM-V1-080` and the citation section's non-goals exclude. Verifier
  semantics are unchanged.
- **Five consumer disagreements resolved from the inherited normative text.** The first matrix run
  found five cases where the consumers differed. The native verifier already matched
  `protocol/cem-1.0/stable/inherited/cem-0.3-algorithms.md`, which the stable `ALGORITHMS.md`
  re-homes as 1.0-normative, so only the portable consumer changed:

| Case | Native (kept) | Portable before | Rule |
| --- | --- | --- | --- |
| `evidence-dropped` (basis names a missing record) | wire `invalid-field` | references `unresolved-reference` | Map-local reference checks are wire checks; an unknown basis is `invalid-field` |
| `hunk-dropped-evidence-kept` (uncited record) | wire `invalid-field` | verification `invalid-field` | An uncited evidence record (inherited `orphan-evidence`) is a wire check; stable has no `orphan-evidence` code, so `invalid-field` |
| `hunk-dropped` (patch hunk with no map entry) | verification `patch-hunk-count` | verification `invalid-field` | Inherited `uncited-hunk` is not a stable code; `patch-hunk-count` is the stable closed verification issue for a map/patch hunk count mismatch |
| `span-empty` | verification `evidence-unavailable` | wire `invalid-field` | "The wire admits ... an empty evidence span; verification refuses [it] as ... `evidence-unavailable`" |
| `span-reversed` | verification `evidence-unavailable` | wire `invalid-field` | Same rule: the span is not a non-empty in-bounds range of the base blob |

  The portable change is small. A new `stableEvidenceReferences` wire check runs in
  `decodeStable` after the hunks are decoded. Its span decode no longer refuses `end <= start`;
  both canonical evidence loops refuse that span as `evidence-unavailable` before slicing. Both
  hunk-count checks emit `patch-hunk-count`. The old portable checks that came later stay in place
  as unreachable defence.
- **Independent review (one Codex read-only round, `94a7d3dc..4e03ff0a`): CONCERNS, one finding,
  confirmed and fixed.** The new portable wire check missed a basis pair repeated within one hunk.
  The native decoder (`internal/cem/wire/map.go`, "duplicate basis pair") and the inherited text
  refuse that at the wire stage as `invalid-field`, but the portable consumer reached verification
  first. `stableEvidenceReferences` now refuses a repeated `(evidenceId, relation)` pair, and the
  matrix gained case `basis-pair-duplicated`. Both consumers return wire `invalid-field` for it.

## Matrix

| Case | Tamper class | Exit | Pinned verdict |
| --- | --- | --- | --- |
| `clean` | control | 0 | ACCEPT; anchored 2, specificity 2 |
| `patch-digest-mismatch` | PatchDigestMismatch | 1 | REJECT verification `patch-digest-mismatch` |
| `fabricated-evidence-id` | FabricatedEvidenceID | 1 | REJECT verification `fabricated-evidence-id` |
| `span-end-edited` | SpanRangeEdit | 1 | REJECT verification `fabricated-evidence-id` |
| `span-hash-mismatch` | SpanRangeEdit | 1 | REJECT verification `fabricated-evidence-id` |
| `span-hash-recomputed` | SpanRangeEdit | 0 | ACCEPT; anchored 2, specificity 2 (a different valid citation) |
| `span-empty` | InvalidSpan | 1 | REJECT verification `evidence-unavailable` |
| `span-reversed` | InvalidSpan | 1 | REJECT verification `evidence-unavailable` |
| `span-out-of-range` | SpanOutOfRange | 1 | REJECT verification `evidence-unavailable` |
| `hunk-dropped` | UncitedHunk | 1 | REJECT verification `patch-hunk-count` |
| `hunk-dropped-evidence-kept` | UncitedHunk | 2 | REJECT wire `invalid-field` |
| `evidence-dropped` | EvidenceDrop | 2 | REJECT wire `invalid-field` |
| `hunk-ranges-moved` | HunkRangeEdit | 1 | REJECT verification `invalid-field` |
| `hunk-id-forged` | HunkIdentity | 1 | REJECT verification `invalid-field` |
| `hunk-reorder` | HunkReorder | 0 | ACCEPT; anchored 2, specificity 2 (array order is not a validity rule) |
| `evidence-ids-swapped` | EvidenceIDSwap | 0 | ACCEPT; anchored 0, cited 2, specificity -2 |
| `extra-basis` | ExtraBasis | 0 | ACCEPT; anchored 2, specificity 1 |
| `producer-tier-field` | ProducerTier | 2 | REJECT wire `unknown-field` |
| `mechanical-unproven` | MechanicalClaim | 1 | REJECT verification `unproven-mechanical` |
| `basis-pair-duplicated` | BasisPairDuplicate | 2 | REJECT wire `invalid-field` |

## Failure modes

- A matrix file that drifts from its manifest entry, or a matrix file missing from the manifest,
  fails `checkStableCaseManifest` (native) and `stableCaseFiles` (portable) before any case runs.
- A recipe that no longer builds the pinned base and target commits fails both tests at
  materialization.
- A consumer whose stage, code, issue codes, exit or citation block differs from the pinned value
  fails that case's named subtest. A tier regression on an ACCEPT case, such as a swap that stops
  degrading, fails on the citation block.
- A matrix edit that drops a required tamper class fails the coverage check in both tests.
- The portable span guard prevents a slice panic on a reversed span. That input previously never
  reached verification.

## Acceptance evidence

- `TestCEMV1226TamperMatrixNative` (`internal/cem/verify/stable_tamper_matrix_test.go`): PASS, 20 case
  subtests. Run as part of
  `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/cem/... ./internal/frontier/... ./internal/cemcandidate/... ./tools/cem-stable-regen/...`:
  every package `ok`, except one flaky `internal/cem/gitrun` run (see Limits). That package is
  untouched; a rerun of it alone passed (`ok internal/cem/gitrun 7.195s`). After the review fix,
  `go test -count=1 -run 'TestCEMV1226|Manifest|Stable' ./internal/cem/verify/` gave
  `ok internal/cem/verify 450.390s`, with 20 case subtests passing.
- `TestCEMV1226TamperMatrixPortable` (`interop/cem01-go/stable_tamper_matrix_test.go`): PASS.
  Full portable suite `(cd interop/cem01-go && GOTOOLCHAIN=local go test -count=1 -timeout 30m ./...)`:
  `ok interop/cem01-go 397.190s`, and `ok interop/cem01-go 451.884s` after the review fix. `go vet ./...`
  there: rc 0 both times.
- Before this change, a scratch differential run of both consumers over the first 19 maps found five
  different results (the table above). After the portable alignment, every case agrees on the full
  stable result except `runtime`.
- `make cem-stable-regen-check`: `PASS (56 cases x 23 keys; packet manifest 9423e789...)`. The new
  directory leaves the existing regenerated cases unchanged.
- `GOTOOLCHAIN=local go build ./... && go vet ./...`: rc 0. `gofmt -l cmd internal tools .github/cishards .github/testconfine`: empty.
- `corvint affected --base 94a7d3dc` returned `PLAN_ONLY` with 87 packages. Most of them read
  named packet case paths only. The focused set above covers the readers of the whole stable
  directory and manifest, plus both consumers.

## Limits

- The matrix is a conformance fixture, not a proof of tamper evidence. Its ACCEPT rows
  (`span-hash-recomputed`, `hunk-reorder`, `evidence-ids-swapped`, `extra-basis`) record cases the
  stable contract does not refuse. A misdirected but valid citation is visible only as a tier
  change (`CEM-V1-080`).
- The native verifier was not changed. The portable consumer's earlier codes for those five cases
  were not covered by any committed vector; this lane adds the coverage.
- The repository recipe matches the `neutral-criterion` profile, so both cases materialize the
  same commits. Any change to that profile changes both.
- The `exit` codes pinned in the matrix are the stable result's documented exits (`0`, `1`, `2`).
  No CLI-level matrix run was added.
- Friction: one `go test` run of `internal/cem/gitrun` failed `TestRunStreamProcessFailureParity/exit`
  and `TestRunStreamContainsDescendants/cancellation` while the host was loaded. The package passes
  alone, and this lane does not touch it.
- Friction: the focused native command took longer than the 600s foreground tool limit, and the
  harness moved it to the background. No background job was started on purpose.
- Friction: the session hook reported `Corvint FALLBACK degraded:
  corvint-event-rejected:dogfood-event-deadline`.
- Dogfood pass0 `NOT_PRODUCED` / `NOT_OBSERVED`: prechange-query and prechange-impact
  (agent-receipt-absent), cem-prepare (git-diff-failed), cem-cite (cem-map-not-produced),
  ocm-aggregate (missing-intent-scope), cem-status (map-unavailable), local-outcome
  (outcome-input-not-provided).
- `docs/specs/INDEX.json` was left unchanged. Its claim already matches the digest claim, and its
  implementation list already names `internal/cem`, `interop/cem01-go` and `protocol/cem-1.0`.

## Rollback

Revert the V1-1340 commits. The matrix directory and its 23 manifest entries are removed. The
portable consumer goes back to its earlier codes for the five cases above and for a repeated
basis pair. No committed map, seal,
golden or task record depends on those codes or on the matrix.
