# V1-1587, V1-1589, V1-1592 test source analysis: review fixes

Date: 2026-10-17. Base: the `origin/main` merge `8a91f16b313da1861626cbf6d15cf73fd3d32928` on PR #1054.
Scope: fixes for the eight findings of the one Codex review round on PR #1054, with amended wording of
`TSA-V0-001`, `-003`, `-010`, `-012`, `-020` and `-021`, accepted under the owner's standing auto-accept
of lane requirements (2026-10-11). No new requirement IDs, decisions, Core profile changes or
ticket-store writes.

## Findings and dispositions

All eight were fixed; each fix has a focused test that failed on the unfixed tree (run against a
detached worktree of the merge with only the new tests copied in).

- Gate passed with unresolved pairs beyond the 1000-pair bound: decisions now apply to every pair
  before the lists are bounded, and the gate reads `unresolvedCount`
  (`TestDuplicatesGateCountsBeyondOutputBound`).
- ALL omitted unindexed setup files: the universe is built from `Index.Tracked` (minus non-blob
  entries), not `Index.Sources` (`TestSelectSetupUniverseIncludesUnindexedFiles`). A reached or
  registry-scanned setup file the index did not read is `source-unreadable`, so a narrower answer is
  never claimed for it.
- Deletion-only hunks were ignored: a `+N,0` hunk records a deletion after line N, which brings a
  test or test-level finding whose span strictly contains it into scope
  (`TestDuplicatesGateSeesDeletionOnlyChange`).
- Uncommitted attributes could mark sources binary: every diff read passes `--text`, fixed prefixes
  and no drivers (`TestChangedPathsIgnoreAttributesAndDecodeNames`, attributes phase).
- Paths with spaces escaped analysis: changed paths come from a NUL-delimited inventory and every
  `+++` header is decoded (trailing tab, C-quoting) and must match it, else the read refuses. The
  parser also now counts hunk lines, so an added line beginning `++ ` is never read as a header
  (same test, names phase).
- Lossy assertion normalisation: `JSToken.Source` carries a regex's source and a substituted
  template's raw text, which `render` uses (`TestRenderKeepsRegexAndSubstitutions`).
- Registry-selected seeders were not traversed: selection alternates traversal and seeder discovery
  until the closure stops growing (`TestSelectSetupTraversesRegistrySeeders`).
- `--all-lines` excluded unchanged files: it now reads every test and page-object file at the
  revision (`TestLintAllLinesReadsUnchangedFiles`).

## Limits

Real-suite qualification remains NOT_RUN. A repository with a generated or oversized setup file and
registry lookups now returns ALL for registry-keyed specs; that is the safe superset, not a
regression of a proved answer.
