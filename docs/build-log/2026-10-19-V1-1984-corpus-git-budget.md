# V1-1984: corpus Git reads cost a constant number of operations

Date: 2026-10-19
Ticket: V1-1984 (GitHub issue beamfall/corvint#1240).
Requirements: DCP-V1-155..159 (`docs/specs/documentation-corpus-v1.md`), accepted under the
owner's standing auto-accept of lane requirements (2026-10-11). The failure-mode list gains the
new code `corpus-capacity-exceeded`. No accepted requirement is amended.

## Problem

Every corpus read revalidated each pinned input that the live index does not hold clean with one
`LookupTreeEntry`. Inventory and build paid one tree lookup plus one blob read for each input the
revision index does not load as text (HTML, ERB, JSON). All of these were charged to the fixed
1,024-operation gitrun budget. A corpus with more than roughly 1,000 such inputs therefore failed
with `git-budget-exceeded`. The CLI reported that as `internal-error`; the corpus MCP server
reported `corpus-refused` with a generic message.

## Decisions

- **One verified walk per revision, not one lookup per path.** `gitauth.Repository.ResolvePaths`
  costs one operation to pin the commit and its root tree, plus one `cat-file --batch` for each
  directory level the paths pass through. Each tree body must hash to its OID, and that OID must
  be the one its verified parent names. This is the same trust model as `LookupTreeEntry`; the
  test compares the two path by path in SHA-1 and SHA-256 repositories. Total tree output is
  bounded by `MaxPathSetTreeBytes` (32 MiB, sized for a 100,000-input inventory). Exceeding it is
  `git-output-exceeded`, which is typed as below.
- **Header batch, then body batch.** `BlobHeaders` (one `--batch-check`) admits every distinct
  regular blob before any body is allocated, so the per-input bound and the new aggregate bound
  (128 MiB of pinned input) refuse early. `BlobBatch` (one `--batch`) reads every admitted body
  with stdout bounded by the admitted sizes. It then checks each record's OID, kind, exact size and
  self-hash, and charges `MaxTotalBlobBytes` once per OID.
- **Freshness.** `liveFreshness` collects the inputs that are not clean in the live index and
  resolves them in one `ResolvePaths` call. The stale and limitation rules are unchanged.
- **Typed capacity.** `doccorpus.Typed` maps `git-budget-exceeded` and `git-output-exceeded` to
  `corpus-capacity-exceeded`. The message names the bound and the phase, for example "while
  pinning N inputs". The CLI's `emitCorpusError` and the bridge's `failureCode` both call it, so
  the two surfaces cannot drift. Over-limit manifest input, scope and provider counts, and the
  inventory scope count, now use the same code and name the limit and the count. They are checked
  before any Git operation. Empty manifests keep `corpus-refused`.
- **Refusal precedence change.** Pinning now admits every input before reading any body. As a
  result, a missing, non-regular or over-bound input, or the aggregate bound, is reported before
  a content mismatch of an earlier input. Codes and messages are unchanged. Only which refusal
  wins when a manifest has several defects differs.
- **Test seam.** `newGitBudget` is a package variable, so tests can bound every phase to
  16 operations. Production still uses the default budget.

## Identity

Artifacts are byte-identical. `TestDCPV1159BatchedPinningKeepsArtifactBytes` builds a mixed
md/go/html/erb/json corpus. With the builder identity and digest zeroed, it compares the encoded
sha256 to a golden value (`ee42ecc3…f1e5`). The golden value is the same when the test runs
against the base code (99dac523), so it was not captured from the new code. The 10,000-input
before/after CLI artifacts differ only in `builder` and the `sha256` that covers it; this was
checked key by key.

## Measurements

The commands are the CLI `docs corpus build` and `search`, run on macOS with
`/usr/bin/time -l`. The base binary is built from 99dac523 and the after binary from this change.
Fixtures were generated in `/private/tmp`.

| Fixture | Operation | Before | After |
| --- | --- | --- | --- |
| 8,000 md + 2,000 html/erb/json | build | `internal-error` git-budget-exceeded, 9.87 s, 74.6 MB RSS | ok, 1.87 s, 304 MB RSS |
| same | search | `internal-error` git-budget-exceeded, 13.57 s, 160.6 MB RSS | ok, `fresh`, 2.01 s, 321.8 MB RSS |
| 10,000 md | build | ok, 1.82 s, 312.9 MB RSS | ok, 1.27 s, 305 MB RSS |
| same | search | ok, `fresh`, 1.75 s, 332.6 MB RSS | ok, `fresh`, 1.50 s, 314.6 MB RSS |

The before RSS on the mixed fixture is low only because the run stopped at the budget. The after
RSS matches the all-markdown fixture, which never hit the budget.

## Evidence

- `TestDCPV1155ReadFreshnessGitOpsIndependentOfInputCount`: 10,000 inputs, with build, read and
  freshness each held to 16 Git operations. One changed HTML input is `stale` and named, and no
  other input is reported. The test takes about 20 s.
- `TestDCPV1155ResolvePathsMatchesSingleLookupsWithinOneOpPerLevel`, in gitauth.
- `TestDCPV1156ManifestAndBuildBatchNonIndexedInputs`: 2,000 HTML, ERB and JSON inputs, with
  inventory and build each held to 16 operations. The test takes about 3.5 s.
- `TestDCPV1157CapacityRefusalsAreTypedAndEarly`: 100,001 inputs are refused under a 0-operation
  budget in well under 1 s, and an exhausted budget is typed.
- `TestDCPV1158BridgeTypesGitCapacityLikeTheCLI` and `TestDCPV1158CLITypesGitCapacityLikeTheBridge`.
- Fail without the fix: the new test files were copied into a clone at 99dac523, with a
  test-only shim declaring the seam, `CapacityCode` and an identity `Typed`.
  - 155 and 156 fail with `git-budget-exceeded`. The seam is inert at the base, so this is the
    real 1,024-operation budget being exhausted.
  - 157 fails with `corpus-refused: manifest record bound exceeded` and with an untyped success.
  - The bridge test fails with `corpus-refused`, and the CLI test fails with `internal-error`.
  - 159 passes at the base.
  - The gitauth test cannot compile at the base because `ResolvePaths` does not exist there.
- Full packages pass: `internal/cem/gitauth`, `internal/doccorpus`, `internal/mcp/corpusbridge`
  and `cmd/corvint-corpus-mcp`. In `cmd/corvint`, the tests matching `-run 'Corpus|DCPV1'` pass.

## NOT_RUN

- The rest of the `corvint affected` closure (about 150 units) was not run. They reach this
  change only through `gitauth`, which gains an additive file and changes no existing function,
  or through `doccorpus`, whose full package passed. Their exhaustive run is left to CI.
- The `cmd/corvint` package was not run in full; only the corpus subset ran.
- `make gate` was not run, per the lane policy.

## Residuals

- The symbol-anchor re-resolution (`symbol_anchor.go`) still makes one tree lookup per cited
  anchor. A failure there degrades to a `withheld` limitation, not a refusal.
- `cem.go` and behavior legacy/Playwright discovery still use the default budget per path.
- The CEM MCP bridge maps `git-budget-exceeded` to `repository-unavailable`.
- The v1 `docs corpus manifest` is still capped at 4,096 inputs. The refusal is now typed.

## Rollback

Revert the commit. No stored artifact, manifest or wire field changes, so no migration is needed.
