# 2026-10-21 corpuseval-1021q: `docs corpus eval` (V1-2315, GitHub #1480)

## Problem

Corpus retrieval quality had no caller-owned measure. An operator could not score their own
labelled questions against a built artifact, nor ratchet a rebuild against an earlier score.

## Decisions

- Requirements `DCP-V1-250..257` in `docs/specs/documentation-corpus-v1.md` are accepted under the
  owner's standing auto-accept (2026-10-11). The IDs sit between origin/main's highest
  (`DCP-V1-243`, with 244 held by an open PR) and the range a parallel lane reserves (280+).
- The scorer lives in `internal/mcp/corpusbridge/eval.go` and calls `Registry.CallTimed`, the
  path that serves the `corvint.docs_*` MCP tools. This means a score cannot drift from what an agent
  receives, and the package needs no new registry.
- Fail closed. A malformed case file refuses whole. An expected ID that the artifact lacks is listed as
  `unresolved`, stays in the recall denominator and fails its case. Errored cases stay in every
  summary denominator.
- The comparison identity is the case-file digest, the repository ID, the profile ID and the
  receipt schema. It is not the artifact digest, so a rebuild stays comparable while the
  artifact digest is still recorded. A different identity or case count is `comparable: false`
  and exits 1. Latency and bytes are reported but never ratcheted, because they are host-noisy.
- Origin (`labelled`, `synthetic`, `generated`) is carried per case and per group. A synthetic file
  cannot hold a labelled case.
- `--format` is shared with `import-cases`. Help is operation-aware, so eval documents
  `json|table`.

## Evidence

- Focused tests: `TestDCPV1250EvalRefusesMalformedCaseFiles`,
  `TestDCPV1251EvalScoresThroughTheMCPToolPath`, `TestDCPV1254BaselineListsEveryRegression`,
  `TestDCPV1255ChangedIdentityIsNotComparable` (`./internal/mcp/corpusbridge`), and
  `TestDCPV1257CorpusEvalCLI`, `TestDocsCorpus*`, `TestCorpusCLI*` (`./cmd/corvint`). All pass.
- Without the implementation, the corpusbridge tests fail to compile (`undefined: EvalReport`).
  At base, the flag matrix and help tests refuse the `eval` operation.
- `TestDCPV1251` re-issues each case through `Registry.Call`. It checks that `receipt_bytes` and
  `returned` equal the served receipt's.

## NOT_RUN

- Tier-2 `corvint affected` units and `make gate` (lane policy). `./internal/doccorpus` (unchanged, a
  dependency-path unit) exceeded the 600 s per-command ceiling and was stopped; its tests import
  no changed code path. The full `./cmd/corvint` package ran only its corpus, docs and help tests.
- No external labelled case set was qualified, so the scores measure only the fixture cases.
- V1-2316 (GitHub #1481, claim audit) is not started in this lane.

## Rollback

Revert the commit. This removes the `eval` operation, its two flags and `DCP-V1-250..257`. No
artifact, receipt or store format changes.
