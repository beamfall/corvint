# mbfix Greptile fix round 2: aggregate admission count without a clean worktree

Date: 2026-11-07. Branch `claude/mbfix` (PR #130). The round started at the sealed head
`a763cc718bdf5c2c0400a45a7f6a25ea0f5d5ba4`. Greptile raised one P1 on
`internal/tracerecordrepo/adapter.go` (`DogfoodNeedsAggregate`). No new requirement ID; the round
amends `DCW-V0-7403`.

## Finding and verdict

**P1, a stopped run cannot restart: CONFIRMED and fixed.** `finishLane.run` calls
`requireVerifyInput` before `requireCleanWorktree`, which admits exactly one dirty path, the
unfinished `.corvint/change.cem.json`, on a restart before the bind commit. With
`AggregateOverLimit` and typed VERIFY argv (a quoted `-run` alternation), `requireVerifyInput`
calls `DogfoodNeedsAggregate`. When more than 200 raw paths changed, that built the index through
`recordIndex`, which ends in `recordSourceStatus` and refuses any non-private dirty path. The
restart was therefore refused `aggregate-admission-unavailable`, even when at most 200 admitted
source paths changed and no aggregate was needed.

Both new tests reproduced this at the round's base with no production change: the
`dirty-over-limit` and `dirty-at-limit-raw-over` cases failed with `trace recording requires a clean
Git tree: repository identity changed`, and the finish-lane cases refused
`aggregate-admission-unavailable`.

Decision: count the paths without requiring recorder-ready state, the smaller of the two options.
Moving the check after the sidecar is handled would place it after prepare, which `DCW-V0-7403`
forbids. The count does not need a clean tree. `contextindex.Build` takes its sources and tracked
set from `ls-tree` of the HEAD tree, and the changed paths come from the committed base-to-target
diff, so worktree dirt cannot change the admitted set. `recordIndex` is now split.
`admissionIndex` builds the index and keeps the tracked-private-artifact refusal, which reads only
committed state. `recordIndex` adds the clean-status check on top. `DogfoodNeedsAggregate` uses
`admissionIndex`, and `RecordDogfood` and `Record` still use `recordIndex`, so recording still
requires a clean tree.

Tests:

- `TestDCWV07403DogfoodAdmissionReadOnly` gained two cases with an untracked, unfinished
  `.corvint/change.cem.json`: 201 source paths (aggregate needed) and 200 source paths plus one
  non-source path (201 raw, 200 admitted, no aggregate). Neither mutates the repository.
- `TestDCWV07403UnfinishedSidecarRestartCounts` runs the finish-lane pre-flight with that sidecar,
  `AggregateOverLimit` and `go test -run 'TestA|TestB' .`. With 203 admitted paths it refuses
  `aggregate-verify-argv-unsupported` before prepare. With 200 admitted of 202 raw it passes the
  pre-check and reaches prepare. Neither case is refused `aggregate-admission-unavailable`.

## Evidence and limits

- `corvint affected --base a763cc718…` (lane build at HEAD) put three packages in tier 0:
  `internal/tracerecordrepo`, `internal/dogfoodflow` and `internal/liveverify/affected`. Each ran
  whole through `script/heavy.sh` under the `corvint test --dry-run` manifest, and all passed.
- The other selected units (tier 1 and 2, 334 selected in total, 68 `LANGUAGE_FRONTIER` unknowns)
  are NOT_RUN, per the round's scope. `make gate` is NOT_RUN under the owner's standing preference.
- Rollback: revert the fix and spec commits. Recording behavior is unchanged.
