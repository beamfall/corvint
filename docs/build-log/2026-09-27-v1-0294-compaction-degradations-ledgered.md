## 2026-09-27 V1-0294 SOL-V0-010 AHI-029: compaction degradations reach the self-observation ledger

Found by the pre-1.0 panel. The Claude Code adapter attempts a SOL-V0-010 row for every
degradation it returns after resolving the project root, including on `pre-compact` and
`post-compact`. The ledger's closed registry in `internal/observations/observations.go` admitted
neither event nor any compaction code, so `Append` refused every such row as prohibited content and
the fail-open writer dropped it. AHI-029 names that row as the only write on the compaction path,
and `corvint doctor` evidence showed zero compaction failures however many occurred.

Decision: the registry admits the events `pre-compact` and `post-compact` and the codes the
compaction path returns after root resolution: `compaction-block-unavailable`,
`compaction-pin-not-preserved`, `compaction-pin-revision-unavailable`,
`compaction-pin-verification-unavailable`, `git-unavailable` and `invalid-compaction-trigger`. It
also admits `corvint-degradations-unrecognised`, which every Claude Code and Codex event can return
after root resolution and which was dropped the same way. Each is a closed code with no content, so
the row schema and its content refusals are unchanged.

Evidence: `TestAdapterDegradationAdmitsCompactionEventsAndCodes` appends one row per code on each
compaction event and expects 14 rows; without the change it fails with no ledger written. The
package's other adapter-degradation tests pass. `TestAppendSerializesConcurrentProcesses` (24
processes within 10 seconds) timed out on the author host at a load average of about 400 with and
without the change.

Rollback: revert the registry change; those degradations are again dropped.
