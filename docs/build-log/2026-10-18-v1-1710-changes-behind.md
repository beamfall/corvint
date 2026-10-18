# V1-1710: `changes behind` lists base shared-path commits a branch is missing

Owner input: [issue 1092](https://github.com/beamfall/corvint/issues/1092). A long-lived ticket
branch could miss shared helper or script commits that landed on its base, and nothing said so
until a merge or a CI run failed. SPC-V0-009..012 were added to
`docs/specs/shared-path-change-policy-v0.md`. No CAL-V0 ID was added. The requirements are accepted
under the owner's standing auto-accept of lane requirements (2026-10-11). Delivery stays
experimental, like the rest of SPC-V0.

## Design

- `corvint changes behind --base REF [--head REV] [--config FILE]` reuses the shared-path policy
  (`corvint-shared-paths/0`) and its classifier. It is a read command: no fetch, no ref, index or
  object writes. The dry merge reuses the SPC-V0-003 `merge-tree` probe with a private object
  directory, now factored into `mergeConflicts`, which `check-shared` also calls.
- The walk is bounded the way the changes range is: `rev-list --count head..base` runs first and a
  range over 512 commits is refused with `changes-shared-range-too-large`, naming `limit` and
  `commits`, before any commit is listed. The code is distinct from `changes-range-too-large`,
  which open PR #1096 adds for a different command, so the two lanes do not collide.
- Only non-merge commits that change a `shared` path count. Merges are listed in `skippedMerges`.
  `headTouches` and `conflictRisk` compare each commit's shared paths with the paths head changed
  since the merge base.
- The report records base, head, merge base, the commits and `configDigest`, the SHA-256 of the
  configuration file bytes.
- The doctor finding is delivered through the existing TQD-V0-010 doctor plugin surface:
  `--ticket ID --stale-hours N --format doctor-plugin` prints a plugin array with one
  `SHARED_BASE_BEHIND` item naming the ticket, the oldest missing shared commit by committer time
  and its age. This keeps `internal/tasks` untouched and avoids a new Tasks profile.

## Residuals

- A native corvint-tasks doctor or dispatch detector that discovers live ticket worktrees and runs
  the check itself (ticket-to-worktree mapping, built-in finding kind) is not delivered.
- The claim-result notice is not delivered.
- Age is the committer time of the base commit, not when it arrived on the base branch, and a
  stale remote-tracking base understates what is missing because the command never fetches.
