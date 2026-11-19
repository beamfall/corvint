# Line evidence self-review repair

Ticket: V1-5697. Requirements: LEV-V1-002/003/007/009/012/015.
The owner's repair request identifies three freshness gaps in the experimental implementation.
This repairs those gaps without promoting line evidence or changing a wire shape or diagnostic.

## Decisions and soundness boundary

- Preserve stale FULL before Playwright flow annotation. Carry full-instance execution reasons
  in non-serialized plan state, independently of selection labels and the number of flow links.
  Flow annotation accepts only these reader forms: FILE with absent, damaged, unqualified or
  stale reason; FULL with absent or damaged reason. Unknown reasons or tiers keep FULL.
- Refresh certifies only successful, empty Git porcelain status, both before collection and
  before publication. No path or status-kind filter can omit executable modes, symbolic links,
  gitlinks, conflicts or a future status form. Check all submodule changes and override local
  file-mode suppression. Existing blob and HEAD comparisons remain additional checks.
- Shared watch permits a caller-owned working-tree planner before attempting an evidence-commit
  diff. The line-evidence caller plans HEAD through the existing reader; an unavailable commit
  is missing ancestry proof and retains FULL. Legacy diff-based watch keeps its existing path.

## Regression evidence

`TestLEVV1003StalePlaywrightExecution` covers all-flow selections with identity, unavailable-commit
and unknown-language mismatches, then builds and executes the full fixture command even when
presentation tiers are replaced by an unknown tier. `TestLEVV1003PlaywrightFlowAllowlist` verifies
unknown reason and tier refusal. `TestLEVV1012VitestWatch` adds unavailable-commit and
unknown-language fixtures, retaining index bytes.

`TestLEVV1003RefreshCleanGitState` covers nine changes before and during collection: modes,
staged modes, links, staged links, gitlinks, staged gitlinks, dirty submodules, staged content with
a HEAD-equal worktree, and an untracked link. Every refusal preserves the previous index.
`TestLEVV1003RefreshUnknownStatus` accepts only successful empty output, refusing an opaque future
status record and unsuccessful empty output. The focused refresh run passed (1.695s).

The first new execution fixture used the collector command while leaving execution's default
`npx playwright test` declaration in place. It failed its argv assertion. The fixture now declares
its private runner command explicitly; production command defaults and existing assertions stay
unchanged. Whole-package and final check results are retained in the lane handback.

## Limits and rollback

No FIH admission or default absent-store migration is claimed. Existing Ruby qualification and
frozen-output blockers remain. Independent review belongs to the orchestrator. The requested CEM
finish cannot run: decision 0594 removed its commands and targets, as `docs/DOGFOOD.md` records.
Rollback is a revert of this repair; no private evidence or wire-format migration is needed.
