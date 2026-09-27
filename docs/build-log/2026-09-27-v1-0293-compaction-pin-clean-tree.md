## 2026-09-27 V1-0293 AHI-027: PreCompact pins a clean, untracked-only or over-budget tree

Found by the pre-1.0 panel. `compactionBlockFor` took the pin's revision from the compact
`SessionStart` receipt's `context.compaction` block. Only the impact block carries a revision. A
clean worktree carries no block (AHI-003 adds it only over a dirty worktree), and an untracked-only
or over-budget worktree carries a rehydration summary without one. In all three cases `pre-compact`
degraded `compaction-block-unavailable`, so the summary kept no pin, and `post-compact` then
degraded `compaction-pin-not-preserved`: two user-visible faults per compaction when nothing was
wrong, and no revision pinned.

Decision: when the block is absent or has no revision, the pin names the revision of the same
receipt's prompt packet, which the same index compiled, with the block's counts (zero when absent).
Tracked paths an over-budget block counts but does not list are counted as elided. The PostCompact
report's `current-dirty` is the receipt's tracked dirty count rather than the number of listed paths,
which differs only for an over-budget block. An invalid block or revision still degrades
`compaction-block-unavailable`. AHI-027 states the case.

Evidence: `TestAHI027ClaudeCompactionPinsCleanAndUntrackedOnlyTrees` pins the HEAD tree with no
paths on a clean and an untracked-only tree, and PostCompact reports `current=matches` without a
fault; an over-budget block with 300 tracked paths pins `tracked=300 elided=300`. On the author
host, at a load average of about 400, the Git repository probe's fixed 10-second deadline expired
before the changed code ran, in the new test and in the existing AHI-027 to AHI-029 tests alike; a
direct `session-start` read of a fresh clean fixture failed the same way. The compaction tests'
result is therefore the pull request's `go-product` run.

Rollback: revert the change; those trees again degrade at both compaction events.
