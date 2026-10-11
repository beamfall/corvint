# Dogfood lane friction: merged seals, inherited CEM, worktree lanes, check probe

Intent: `docs/specs/daily-change-evidence-workflow-v0.md`, `DCW-V0-042..045`, all proposed and
pending owner acceptance (V1-1219, V1-1198, V1-1209, V1-1207). Base: the w6 dogfood lane tip
`6c38c6c4a`. No decision file; no promotion.

## Decisions

- V1-1219 (`DCW-V0-042`): `dogfood change` admits a range that adds sealed CEMs only when each
  re-verifies with the same check batch-check applies to a constituent, and binds against the
  unmoved `BASE`. This amends the decision 0319 consequence that any sealed CEM in the range
  refuses; the owner must accept that amendment. Rejected: moving `BASE` to the merge commit
  (the lane workaround), which hides the merged hunks from the binding.
- V1-1198 (`DCW-V0-043`): an unsealed CEM that `BASE` tracks is archived by an exact one-file
  command, `.corvint/changes/<writer>.cem.json` with `BASE`'s blob, named by an early
  `dogfood change` refusal and by both seal implementations. Batch-check treats that archive as
  no constituent. Rejected: auto-sealing it, which would put a non-rename commit in the seal
  path and claim checks that never ran; the archive is noted `NOT_REVERIFIED` instead. A batch
  head that tracks an unsealed CEM already failed `unsealed-cem-at-head` before the merge.
- V1-1209 (`DCW-V0-044`): the trace store, evidence directory and operation lock were already
  per worktree; the concurrent two-worktree test passed before any code change. The friction was
  lanes switching branches in one worktree, so the stranded-trace line and `docs/DOGFOOD.md` now
  name one worktree or clone per lane. Keying one store per branch would change the local trace
  contract and is not done.
- V1-1207 (`DCW-V0-045`): `dogfood check`'s cleanliness probe is now Core's hardened status, as
  batch-check's is. Before the fix the new test's clean-filter marker was written.

## Limits

`dogfood change` itself has no worktree status read of its own; its steps' probes are not
re-examined here. The inherited archive's content is never reverified. The w6 branch's archive of
the w4 CEM is the observed case; no other repository was replayed.
