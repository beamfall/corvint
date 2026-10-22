# 2026-10-21 lane affected-1021s: affected `--format` refusal, coded dubious ownership, non-UTF-8 query history

## Problem

- V1-2348: `corvint affected --format json` fell through to the generic `unrecognized arguments`
  refusal, which neither says the command has a single JSON output nor lists what it accepts.
- V1-0288: Corvint runs Git with global and system configuration nulled (`GPK-V0-009`), so on a
  checkout owned by another user every Core verb ended in Git's uncoded `detected dubious
  ownership` exit, whose `git config --global --add safe.directory` advice cannot work under
  Corvint.
- V1-0313: `query --task` and `prove --task` refused a whole repository with
  `unsupported-query-history` when any commit in the bounded history named a non-UTF-8 path, which
  every other surface already excludes and discloses (decision 0394).

## Decisions

- `AFP-V0-201` (new, accepted under the owner's standing auto-accept of 2026-10-11): `--format` and
  `--format=VALUE` refuse `invalid-arguments` with a message naming the one output and every
  accepted option, derived from the parser's own option set so the list cannot drift.
- `GPK-V0-071` accepted and narrowed, and `GPK-V0-072` accepted as proposed, under the owner's
  2026-10-21 delegation of owner-blocked ticket decisions ("make decisions for me") as routed by the
  orchestrator, plus the standing auto-accept. For `GPK-V0-071` the remedy is only "run as the
  owning user or change ownership": the `safe.directory` opt-in is declined because it bypasses
  Git's protection against planted configuration and not every env builder neutralizes filters,
  hooks, fsmonitor and textconv. The classification sits in the central CLI error writer
  (`emitError`), so every verb whose error reaches it gets the same code,
  `repository-ownership-refused`, with no change to the thirty per-capability env builders.
- `GPK-V0-072` uses the existing `skipNonUTF8` seam of `parseHistory` on the query-learning path.
  No existing `history_digest` moves, because every repository affected refused before. The Python
  oracle is retired (decision 0088), so no divergence-register entry is opened.

- The `eval_query.go` and `history.go` edits are analyzer inputs (IDX-SNAP-V0-017), so the schema
  moves to `corvint-analyzer/141` through `script/analyzer-schema.sh bump` (140 is reserved for
  V1-2285; the orchestrator renumbers at batch time). Index extraction and encoding are unchanged.
  Batch 1021v renumbered it to `corvint-analyzer/143`, because batch #1505 took 142.

## Evidence

- `TestAFPV0201AffectedFormatIsRefusedWithAcceptedOptions` (`cmd/corvint`), FAILS without the change.
- `TestGPKV0071DubiousOwnershipIsOneCodedRefusal` drives index, query, impact, context and feature
  through a PATH Git shim that reports dubious ownership (the sanitized Git environment strips
  `GIT_TEST_ASSUME_DIFFERENT_OWNER`, so real Git cannot be made to refuse); with
  `TestGPKV0071DubiousOwnershipClassifierIsExact` it FAILS without the change.
- `TestGPKV0072QueryLearningSkipsNonUTF8HistoryPaths` (`internal/contextindex`) commits a Latin-1
  path and FAILS without the change with the old refusal; after it, learning considers both commits
  and no result row names the path, while the index exclusion still discloses it.

## Residuals and NOT_RUN

- `GPK-V0-071` limit: `affected` (reports no resolvable HEAD commit), `init` (succeeds with an empty
  inventory) and `overview` (`invalid-repository`) swallow Git's stderr before the error writer and
  are not classified; filed as a residual.
- Tier 2 of `corvint affected --base` and the exhaustive repository gate are `NOT_RUN` (left to
  batch CI, `AFP-V0-162`).

## Rollback

Revert the lane commit: remove the `--format` branch and `affectedFormatRefusal`
(`cmd/corvint/affected.go`), `dubiousOwnershipRefusal` and its call in `emitError`
(`cmd/corvint/main.go`), and pass `false` again in `evalLearnedCandidates`
(`internal/contextindex/eval_query.go`); restore the proposed text of `GPK-V0-071`/`072` and drop
`AFP-V0-201`.
