# 2026-10-12 CEM cem/0.x -> cem/1.0 map converter (V1-1275)

Branch `claude/v1-1275-convert` from b8ef66364. Decision 0525 §4 "Converter contract" and §5
"Sealed-map rule"; requirements `CEM-V1-096..105` (proposed) in `docs/specs/cem-stable-v1.md`,
building on the V1-1265 shape (`CEM-V1-093..095`).

## Decisions

- The converter is `internal/cem/convert.Convert(src, Inputs{BindCommit, Git})`. Its output is
  the `wire.EncodeStable` bytes of a map built only from the source bytes and the bind commit:
  `spec` `cem/1.0`; `baseRevision`, `patchSha256`, `evidence` and `hunks` carried member for member
  (0.3 `coverage`/`discriminates` carried when present, never synthesized); `excludedPath` the
  protocol literal; the four producer-only arrays empty; `role` omitted; `provenance`
  {`sourceSpec`, `sourceBlobOid` (Git SHA-1 blob OID of the exact bytes), `sourceSha256`,
  `bindCommit`, `converter` `corvint-cem-convert/1`}. No member needed a value the source and the
  bind commit do not determine, so there was no STOP gap.
- The only Git read is the report-side source binding check (the blob at
  `.corvint/change.cem.json` in the bind commit): EXACT, ABSENT, UNAVAILABLE, MISMATCH or
  NOT_CHECKED. Only MISMATCH refuses; the output bytes never depend on the state.
- `bindCommit` comes from the file name (`<40 lowercase hex>.cem.json`) or `--bind-commit`; a
  missing or malformed one refuses `convert-bind-commit-unavailable` instead of being guessed.
- Reconverting a cem/1.0 map refuses `convert-source-is-stable` rather than returning it
  unchanged, so a converted map never gains a second provenance.
- A zero-hunk 0.x map (the 0.x reader admits one) refuses `convert-empty-hunks`.
- Parity is measured at a substituted target T' (bind tree with only the sidecar replaced by the
  converted bytes, written into a scratch clone), because verifying the converted map at the bind
  commit itself is `REJECT excluded-artifact-mismatch` on both consumers for every map: the bind
  commit's sidecar holds the 0.2 bytes. That gap is the §5 sealing step's to close (V1-1276/1277).
- `corvint cem convert` is listed in `corvint help cem` after `verify-stable`; `cmd/corvint/main.go`
  is unchanged, so no use-case receipt repin was needed.

## Evidence

Tests: `TestCEMV1096ConvertAcceptsEachZeroDialect`, `TestCEMV1097ConvertMemberMapping`,
`TestCEMV1098ConvertProvenanceValues`, `TestCEMV1099ConvertSourceBinding`,
`TestCEMV1099ConvertGitReaderStates`, `TestCEMV1100ConvertCorpusIdempotent` (all 603 committed
maps converted twice, byte-equal, reconversion refused), `TestCEMV1100ConvertF0fda542`,
`TestCEMV1101ConvertRefusalCodes`, `TestCEMV1103ConvertUnavailableObjects`,
`TestCEMV1104ConvertParityCEM03`, `TestCEMV1104ConvertParityCEM01` (`internal/cem/convert`);
`TestCEMV1102CEMConvertCommand` (`cmd/corvint`); `TestCEMV1104PortableConvertedFixtures`
(`interop/cem01-go`); `TestCEMV1104ConvertParitySample`, `TestCEMV1104ConvertParityClassify`
(`tools/cem-convert-parity`). Fixtures in `internal/cem/convert/testdata/`: a reachable cem/0.2
bind (a79ab231), cem/0.1 (`cem01-valid`, byte copy of `conformance/cem-0.1/valid.cem.json`), cem/0.3
(`cem03-coverage-covered`, byte copy of `protocol/cem-0.3/vectors/maps/coverage-covered.json`) and f0fda542 (877 hunks); the
unavailable-object case uses the reachable fixture against an empty repository.

Full-corpus parity, `tools/cem-convert-parity` over all 603 `.corvint/changes/*.cem.json` against a
`git clone --local --no-checkout` scratch clone, `--reachable-revs` from `git rev-list origin/main`,
`--jobs 6`, go1.27.2, rc=0:

| File | SHA-256 |
| --- | --- |
| `docs/build-log/evidence/v1-1275/parity.jsonl` (603 rows; equals summary `paritySha256`) | `bf9b354bdab31ed5a330a38c17850c7f95f1ffc79aabc3ce428321a6d9fa7ba9` |
| `docs/build-log/evidence/v1-1275/summary.json` | `51681224fb196c56244f90fee39bdf19e077d8115457ceae7c75e8830cb0d5e6` |

- Classes: 602 `identical`, 1 `pre-existing-refusal` (f0fda542), 0 `empty-member-explained`,
  0 `unavailable-objects`, 0 `NOT_OBSERVED`, 0 `different`.
- 603 idempotent; 603 reconversions `convert-source-is-stable`; 603 source bindings EXACT; 565
  bind commits reachable from `origin/main` (the other 38 are present in the clone).
- Source 0.2 outcomes: 602 ACCEPT, 1 UNSUPPORTED `unsupported-resource-limit`; the substituted
  native stable outcomes are identical, and native equals portable on 602 full results.
- Direct verification at the bind commit: 603 `REJECT excluded-artifact-mismatch` on both
  consumers, native equal to portable on all 603.
- An earlier full run before the f0fda542 classification change gave the same 602 identical rows
  and the same f0fda542 results (then classed `different`), so the f0fda542 divergence reproduced
  identically across two runs.

## Gaps and limits

- f0fda542: native and portable both report UNSUPPORTED `repository` `unsupported-resource-limit`
  (as the 0.2 source) with equal hunks, but their partial drift lists differ deterministically
  (native 68 entries, portable 102, source 34) and so do their `verdictSha256` values. This is a
  stable-consumer divergence on a resource-limit abort, independent of the converter; it is listed
  under `nativePortableDiffer` in the summary.
- cem/0.1: its `patchSha256` digests an out-of-band patch that the canonical diff does not
  reproduce, so a converted 0.1 map verifies `REJECT` `patch-digest-mismatch` on both consumers, the
  same result `verify.Exact` gives the source with canonical-diff bytes. cem/1.0 has no out-of-band
  patch member; the committed corpus has no 0.1 map.
- Unavailable pinned objects surface as `git-read-failed` at the `repository` stage (both stable
  consumers and the 0.2 verifier), not `repository-object-unavailable`.
- `protocol/cem-1.0/migration-plan.json`, named by `parentPlan` in
  `protocol/cem-1.0/stable/MIGRATION.json`, does not exist at b8ef66364.

## Rollback

Revert `internal/cem/convert`, `internal/cem/cli/convert.go`, the `convert` dispatch in
`internal/cem/cli/stable.go`, the help lines in `cmd/corvint/help.go`, `tools/cem-convert-parity`,
`interop/cem01-go/convert_fixture_test.go`, the CEM-V1-096..105 spec section with its regenerated
`REQUIREMENTS.tsv` rows, the `internal/cem/convert` read scope in `.corvint/test-read-scopes.json`,
the `tools/cem-convert-parity` unit in `.corvint/unbounded-readers.json`, the repinned
`cmd/corvint/help.go` citation in `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md`, this entry and
`docs/build-log/evidence/v1-1275/`. No committed map, schema, consumer or sealed packet changed.
