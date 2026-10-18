# V1-1587, V1-1589, V1-1592 test source analysis

Date: 2026-10-17. Base: `55abb815c4c20a771363d38e315ddfddf14c6972`.
Scope: new experimental `TSA-V0` requirements in `docs/specs/test-source-analysis-v0.md`, accepted
under the owner's standing auto-accept of lane requirements (2026-10-11); no decisions, Core profile
changes or ticket-store writes.

## Design

One lexer: the application map's JavaScript/TypeScript lexer gains a comment-collecting variant and
an exported token view (`appmap.LexJS`), so annotations and comment-only mentions are handled the
same way for all three commands and no dependency is added. Lint and duplicates read committed
blobs through one `cat-file --batch` per command and never the worktree. Uncommitted edits are a
non-goal: the commands answer for a revision, which keeps them deterministic and lets a gate cite
the commit.

Setup selection runs on the revision index (`BuildRevisionContext`) because it needs the existing
string-key registry dialect and the web import resolver. A registry key selects the setup files that
import the key's bound module directly. This is narrower than "every setup file that mentions the
key" and wider than nothing; a looser link (runtime project dependencies) is listed as a failure
mode, not inferred. Every unprovable step falls back to ALL with a typed reason, which is the safe
superset for a test provider.

Duplicates use assertion-line Jaccard with a frequency cutoff, the issue's chosen measure. The
decision file is project-owned, read at the analysed revision so a gate result names its inputs, and
strictly decoded; a reasonless decision is listed, not silently applied.

## Evidence

Focused tests in `internal/testsource`, `internal/contextindex` (setup selection), `internal/appmap`
and `cmd/corvint` (`TestTestsCLI`, `TestRequireCleanHonouredOrRefusedByEveryCommand`) pass on
darwin/arm64. Real-suite qualification is `NOT_RUN`.

## Residuals

- V1-1587: the optional MCP tool for setup-select is not built.
- V1-1589: issue #1032 AC5, preflight requiring a clean lint for the same tree, is not wired.
