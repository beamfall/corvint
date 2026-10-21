# 2026-10-21 affected-src-1021g: source blob-read failures, go list stderr, token search

Lane affected-src-1021g (pre-1.0 ticket sweep). Requirements below are accepted under the owner's
standing auto-accept (2026-10-11).

## Problem

- V1-2258: `corvint source` and `context --expand` reported every `git cat-file` failure as the
  byte-budget refusal (`blob-budget` / `expand-budget`), so a missing object or a git error looked
  like an oversize blob.
- V1-2257: test helpers that run `go list` used `Output()` and dropped stderr, so a broken module
  failed with only `exit status 1`.
- V1-2143: `ticket search --text` matched a multi-word phrase only as an exact substring, and a
  `--fields labels` or `effects` refusal on list or search did not say where those keys are read.

## Decisions

- ESV-V0-011: `sourceGit` returns a sentinel when procgroup reports stdout overflow. Only that
  sentinel (or a successful read over the limit) maps to `blob-budget` / `expand-budget`; every other
  read failure maps to `git-failed`. The frozen source-view digests were refrozen.
- V1-2257 has no requirement: it is test-harness diagnostics. A `goListOutput` helper appends trimmed
  stderr to the error; the live-verify ground-truth skip names stderr too. Production `go list` sites
  already carried stderr (`ExitError.Stderr` or `CombinedOutput`).
- CTS-V0-012: `--text` is token-based (every word in title or body, any order); a word-less value is
  `MALFORMED`. List and search keep omitting `labels` and `effects` (the projection stays small); the
  `--fields` refusal names `ticket show <id> --fields record.<key>` and `ticket export`. Read-only
  verb code only; no store was touched.

## Not delivered

- V1-2075 skipped: blocked by V1-2163 (witness-reason wire field); PR #1398 recorded it as unsound
  without that design, and open PR #1420 changes the same documentation-path selection.
- V1-2262 skipped: indexing struct fields adds a symbol kind that changes the index, ranking tables,
  the analyzer schema and a frozen context eval, and overlaps open PR #1439 (taskcontext and goldens).
- V1-1003 not started: three MCP wire changes (budgetBytes, truncation counting against MCPV0-031,
  tools/list dedupe) exceed the lane budget.

## Evidence

- TestSourceBlobReadFailureIsDistinctFromBudget, TestGoListOutputCarriesStderr and
  TestCTSV0012_SearchTextIsTokenBasedAndFieldsNameTheRoute each failed without their fix and pass
  with it. Tasks traps TestCALV0131_LiveFormatsCoverEveryDecodedProfile and TestImportDirection pass.

## NOT_RUN

- `make gate` (owner preference for scoped work); affected tier 2 and selected packages beyond the
  lane's 15-package cap are left to batch CI. TestCoreVerbsEmitTheFrozenProfiles fails on this host
  only because the host toolchain is go1.27.3 against a go1.27.2 golden (V1-2118).

## Rollback

Revert the lane's commits. No stored state, index format or wire profile version changed; the
refrozen source-view digests revert with the source.
