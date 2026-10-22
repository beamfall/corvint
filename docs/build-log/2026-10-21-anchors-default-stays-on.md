# 2026-10-21 anchors default stays on (V1-2295 AC2)

Date: 2026-10-21. Batch: claude/batch-1021v. Decided by the orchestrator under the owner's
delegation of in-task decisions (2026-10-05); the owner can revisit it.

## Decision

V1-2295 acceptance criterion 2 asks whether to recover the trace2code recall drop by changing the
`TCP-V0-022` anchors default. The decision is to keep `TCP-V0-022` anchors on by default.

## Evidence

From the recall lane (`2026-10-21-recall-1021s-pair-budget-anchor-attribution.md`, PR #1515):

- The frozen trace2code recall@20 drop from 0.8036 to 0.7690 comes from the anchors default flip
  in `366c3756a` (#444), not from the subjectless pair rule.
- `b8fe75d` (the #444 merge) with `CORVINT_CONTEXT_ANCHORS=off` reproduces exactly the 0.8036 of
  its parent `d761a403f`, so the flip alone explains that step.
- Turning anchors off would lose the chi orientation gain that the anchors default delivers.
- The further drop from 0.7690 to 0.7640 is gin `2097c0a5` losing `path.go`, which is unrelated to
  anchors.

## Consequence

The trace2code baseline is re-based to the anchors-on value: 0.7690 at the flip, measured as
0.7640 on today's base after the gin `path.go` loss. This supersedes the recall lane's note that
the baseline is not moved pending an owner fork. Revisit if an anchor change recovers trace2code
without losing the chi gain.

## Rollback

Delete this entry and restore the earlier trace2code baseline; no code, wire or default changes
with this decision.
