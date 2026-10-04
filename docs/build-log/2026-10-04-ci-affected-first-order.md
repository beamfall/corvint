# CI affected-first package order inside full shards (V1-0716)

Human-owned intent: ticket V1-0716, under the owner-directed AFP-V0-022 profile. Selection
promotion stays blocked (V1-0616); this change orders the complete universe and selects nothing.

## Finding

Over the 60 most recent failed pull-request CI runs (read 2026-10-04), 56 `go-product-shard` jobs
failed; 15 of them, in 12 runs, contain a failing package outcome. The other 41 failed in a
non-test step, timed out or were cancelled. In those 12 runs the first failing package finished a
median 11.0 minutes after the run was created, at median position 35 of 75 in its shard. Packages
ran in the helper's lexical order, so a changed package waited behind unrelated ones.

## Change

- `.github/cishards/order.go`: `Order` permutes one shard so the Go units an `affected-plan/0`
  selected run first: `DIRECT_SOURCE_CHANGE`, `DIRECT_TEST_CHANGE` and `DEPENDENCY_PATH`, then other
  bounded witnesses, then `UNBOUNDED_READER`, then unselected packages. The sort is stable. A plan
  that is oversized, unparseable, another profile or not `ok` returns the input order.
- `ci-shards --order FILE` applies it after partitioning; an unreadable file keeps the order. The
  file is bound into the protected profile digest and copied into the isolated helper module.
- `ci.yml`: on pull requests outside DOCS mode, `docs-plan` builds the planner from the event base
  checkout it already holds, plans the tested merge against that base, and retains the plan as the
  `affected-order` artifact. Both steps are `continue-on-error`. Each shard fetches the artifact
  (also `continue-on-error`) and passes it to the helper in the full branch only. Main pushes and
  the pinned-driver branch are unchanged.

`go test -p 1` runs packages in argument order (observed locally with three packages given in
reverse lexical order).

## Evidence and limits

- Helper tests pass in the repository module and in the isolated module the workflow builds
  (`-race`, `go vet`, Windows cross-build): the output is the same set, the input is not mutated,
  and six unusable plans keep the order. The shard-1 package set hashed identically with a real
  plan, a missing plan and no `--order`.
- Replay on the 12 retained failed runs, using each run's recorded package times and a plan
  computed for the failing head against its merge's first parent with the installed planner
  (1.0.0-rc.1 build 163): summed package time up to and including the first failing package fell
  from a median 8.7 to 4.9 minutes (mean 10.1 to 7.9). Eight runs improved by more than a minute,
  two got 0.3 minutes later, two improved by less than a minute. Every first-failing package was in
  its plan's selection. This is a replay, not a hosted measurement: hosted time to first failing
  package after the change is `NOT_OBSERVED`.
- The planner step costs every non-DOCS pull-request run a cold planner build and one plan before
  the shards start. Locally that was 16 s to build and 2.4 s to plan; the hosted cost is
  `NOT_OBSERVED` until this change's own CI run.
- A shard still runs every package, so the check result arrives when the shard ends. Only the
  failing package's position in the live log moves. Reporting the first failure before the shard
  ends is not part of this change.
- Most selected units are `UNBOUNDED_READER`, so failures in those packages move little.

## Rollback

Remove `--order` from the full branch in `ci.yml`; the planner and fetch steps then have no effect
and can be deleted with it.
