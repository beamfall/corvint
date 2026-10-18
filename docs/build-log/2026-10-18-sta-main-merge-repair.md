# 2026-10-18: STA branch main-merge repair (analyzer schema 124)

Merge `8a420f0d2` took `origin/main` (`70a21da5d`, with #1108 and #1111) into `claude/sta-ci-1697`
with `-X ours`. Replaying it without the strategy option conflicts in five files; `cceccf97c`
repaired `docs/specs/README.md` and `docs/specs/REQUIREMENTS.tsv`, and this repair covers the rest:

- `docs/decisions/README.md`: main's `0543` row was dropped (`decision-numbers-check` failed);
  restored after `0541`.
- `Makefile` `.PHONY`: main's `windows-vet` and `lane-scripts-test` were dropped; restored next to
  the branch's `spec-toolchain-conformance` and `spec-toolchain-latest-test`.
- `internal/contextindex/analyzer_schema_test.go`: both sides had independently taken
  `corvint-analyzer/123` for different contextindex sources (main for V1-1698, the branch for
  STA-V0-007), and `-X ours` kept the branch's audit digest, so `TestAnalyzerSchemaInputs`
  (IDX-SNAP-V0-017) failed on the merged sources. The merged analyzer is neither side's
  `/123`, so `analyzerSchemaID` moves to `corvint-analyzer/124` with the merged-input digest
  `17dcce79...172da`. A pack written under either `/123` is rebuilt rather than reused.

`internal/contextindex` tests pass on the repaired tree. Rollback: revert this commit (the two
`/123` packs then disagree again under one ID).
