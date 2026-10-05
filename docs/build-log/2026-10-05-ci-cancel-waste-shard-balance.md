# CI cancellation waste and shard balance: V1-0809, V1-0617

Human-owned intent: the owner's 2026-10-05 CI audit request ("only run the tests that need
running") and its first two steps, which the owner chose: stop cancellation waste (V1-0809) and
balance the race shards with measured costs (V1-0617). Contracts: AFP-V0-027 (new) and AFP-V0-022
in `docs/specs/affected-plan-v0.md`. Pull-request test selection itself stays blocked on AFP-V0-014
qualification. The proposal that gates selection behind the merge queue is decision 0431.

## Evidence

- The last 100 `CI` runs (about 11 hours on 2026-10-05) used 7,063 runner-minutes (jobs API). Cancelled
  pull-request runs used 2,255 of them (31%) and cancelled push runs used 846 (11%), while
  successful pull-request runs used 3,076 (43%). Cancelled runs lived a median of 11.1 minutes on
  the batch branch and 17.4 minutes on task branches, so a start delay would not have saved them.
  The waste came from a lockstep pattern. One task pull request (#592) merged the batch branch into
  itself about 16 times in 3.5 hours while the batch pull request (#609) already carried it, so
  every round started two full suites, often on the same tree, and the next round cancelled both.
  Over 200 runs, the batch branch's pull-request runs were 25 cancelled and 4 succeeded.
- The cost table (run 37198363933) had drifted badly. Observed package times were `internal/tasks/store`
  372–871 s (table 171 s), `internal/tasks/authority` 300–413 s (table 206 s) and
  `internal/tasks/cli` 125–203 s (table 40 s), and 18 packages were missing. The last run before
  this change (37280783904) loaded its four shards with 661, 1,749, 1,547 and 2,496 s of package
  time. An LPT replay of the table reproduces that partition exactly.

## Decision

- AFP-V0-027: a `ci:batched` label skips `go-product-shard` for a batch constituent. The skip makes
  `go-product` fail, so the label can only withhold tests, never admit a merge. Constituents land
  through the batch, whose own run tests the combined tree. Batching agents also stop merging the
  batch branch back into constituents.
- The table is refreshed by `tools/ci-shard-costs refresh` from run 37280783904 (tested merge
  8beb9f70585e737ddf18a9171b0fcf79ad997a29), covering all 313 packages. `check` against the three
  later-sampled runs (37272455634, 37274602082, 37276556052) reports no drift; the two older runs
  drift only on `internal/tasks/store`, which has grown since.
- The matrix grows from 4 to 6 shards. `REUSE_SHARDS` moves with it (AFP-V0-024). The table below
  gives the replayed maximum shard package time for each of the six sampled runs, in seconds:

  | Partition | 37261129228 | 37272018512 | 37272455634 | 37274602082 | 37276556052 | 37280783904 |
  | --- | --- | --- | --- | --- | --- | --- |
  | old table, 4 shards | 1,969 | 1,792 | 2,267 | 2,027 | 2,484 | 2,496 |
  | new table, 4 shards | 1,550 | 1,448 | 1,634 | 1,545 | 1,558 | 1,614 |
  | new table, 6 shards | 1,055 | 1,006 | 1,085 | 1,032 | 1,065 | 1,076 |

  Six shards come within about 200 s of the floor that the largest package sets
  (`internal/tasks/store`, 871 s). Eight shards would gain little. Each shard adds about 2–3 minutes of
  setup and race compilation, so a full run costs about 6 more runner-minutes. In exchange, its
  wall time falls from about 44 to about 21 minutes, and each cancelled run that is superseded
  wastes less.

## Limits and rollback

- These are replay numbers. The hosted before and after wall time and skew are retained on
  V1-0617 from this pull request's own run and the next `main` push, as that ticket requires.
- The shard count and table bytes are part of the AFP-V0-014 frozen identity (AFP-V0-022). No
  selection pin is set, so nothing qualified is invalidated, but a future qualification binds the
  new digest. During the switch, a `main` push whose pull request was tested with 4 shards finds
  no 6-shard records and runs FULL.
- The label must exist in the repository before agents can apply it. Its hosted behavior is
  `NOT_OBSERVED` until a batch uses it.
- Rollback: restore the previous table bytes, the `[0, 1, 2, 3]` matrix and `REUSE_SHARDS: "4"`, and
  remove the AFP-V0-027 job condition and message.
