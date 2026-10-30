# migrate-traces on histories deeper than the replay window (V1-3034)

When a repository had more than 10000 commits, `corvint migrate-traces` refused the whole store
with `Git ancestry exceeds bounded trace replay limit 10000`. The LTPM-V0-012 rebind refusal prints
that same command as its recovery, so the recovery could never succeed on a repository that deep.
Two lanes hit this on 2026-10-28.

## Contract and implementation

LTPM-V0-517 replaces the whole-store refusal with a bounded window:

- `bindAuthority` keeps the newest 10000 commits.
- `trace.MigrationAuthority` gains `AncestryLimit` and a `ReachableCommit` probe. The probe resolves
  each trace name outside the window on its own, once per name, using three Git calls:
  `cat-file --batch-check` gives the object type, `merge-base --is-ancestor` tests reachability, and
  `rev-parse <id>^{tree}` gives the tree.
- With this, the cost is bounded by the number of trace store names, not by ancestry depth.
- A stranded commit beyond the window is still quarantined under LTPM-V0-012.
- A legacy tree name cannot be proven to map to exactly one commit once the window is truncated. The
  command refuses and names the recovery: move the file out of `.context-corvint/` and rerun the dry
  run.
- Plan digests and output bytes are unchanged.

Residual (not changed here): `internal/tracerecordrepo` keeps its own replay window. It still refuses
a candidate that "predates the bounded replay window".

## Executed evidence

- `TestLTPMV0517DeepHistoryBoundsByTraceStore` passes. It injects a two-commit window over five
  commits.
- These packages pass: `internal/tracemigraterepo`, `internal/trace`, and `cmd/corvint` (filtered with
  `-run TestMigrateTraces`).
- Live run on `corvint-main`, which has 10958 commits:
  - The installed 93b9f5b0 binary refuses with the old message.
  - The lane binary's dry run returns `ok:true`.
  - In a shared clone, an amended `HEAD` trace was planned as stranded. Applying that plan
    quarantined the trace into `legacy-traces/`.

## Rollback

Revert the commit. The ancestry-limit refusal returns.

## Review fix (Codex review of the lane, 2026-10-30)

The first version charged each trace name beyond the window three Git calls (cat-file, merge-base,
rev-parse), plus one tree listing, against the fixed 1,024-operation default. A valid store of
about 250 such traces therefore refused, although the store admits 1,000. Two changes fix this:

- A single `cat-file --batch-check` now reads the object type and peels the tree.
- The commit-object check is cached.

Each trace now costs at most three operations, across both plans of an apply. The budget is the
default plus three per admitted trace file. `TestLTPMV0517StoreAtMaximumBeyondTheWindow` builds
1,000 traces beyond a two-commit window with `git fast-import`, ten of them stranded and each on
its own tree. The dry run and the apply both pass. With the default budget the dry run fails with
`git-budget-exceeded`. The test takes about 87 s because it spawns roughly 6,000 Git processes. A
true batched probe would make it faster, but it would need a batch interface on
`MigrationAuthority`.
