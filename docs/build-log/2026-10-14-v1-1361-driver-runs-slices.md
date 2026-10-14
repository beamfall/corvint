# 2026-10-14 The sharded PR driver runs AFP-V0-041 test slices (V1-1361)

Branch `claude/v1-1361-driver-runs-slices` from `origin/main`
adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f. `AFP-V0-048` is new in
`docs/specs/affected-plan-v0.md`; it and the amendments to AFP-V0-041 (6), the AFP-V0-041 pin
limit and the AFP-V0-047 parenthetical are written under the owner's standing auto-accept of
2026-10-11, so their mode is `standing` until the owner reads them back. Corvint 1.0 scope under
`PRS-V1-018`. The orchestrator owns the ticket; this lane ran no Tasks mutation.

## Problem

At BASE, `execute` in `tools/corvint-pr-tests/main.go` called `refuseSlices` on every sharded run
and exited 2 whenever `cishards.Splits` reported a split. `.github/cishards/test-slices.json`
splits `internal/tasks/store`, so setting `CORVINT_PR_TOOL_SOURCE` would fail all eight PR shards.

Acceptance clause 2 is already met on `main`: `.github/workflows/ci.yml` lines 252 to 255 at BASE
say the pin "must stay empty while .github/cishards/test-slices.json names any slice". This lane
may not edit `.github/**`. Once this change lands that comment is stale, because the driver no
longer refuses slices. V1-0812 edits that block when it sets the pin.

## Decision

1. **Option (a): teach the driver the slices now.** The orchestrator chose this over waiting for
   V1-1315. V1-0812's narrowing (decision 0431 items 3-4 and 8) runs before V1-1315's cutover,
   because V1-1315 depends on V1-0812. The narrowed PR shards execute through this driver, so it
   must place slices correctly before V1-0812 can set the pin.
2. **Rejected option (b): keep the refusal until V1-1315.** Under (b), V1-0812 could only enable
   narrowing after emptying the slice file. That would give up the AFP-V0-041 shard balance, or
   it would block V1-0812 behind a ticket that itself depends on V1-0812.
3. **Reuse the helper; never re-partition.** `shardLayout` calls `cishards.Plan`, the same call
   `ci-shards --shard --shards --slices` makes for the full path. It never calls `Intersect`,
   whose `Partition` ignores slices. `shardUnits` filters one shard of that placement:
   - FULL takes the whole shard.
   - A narrowed run keeps the selected whole packages and every slice of a selected split
     package that is placed on the shard.
   Across all shards, a selected split package therefore runs each slice exactly once and never
   whole.
4. **Execution mirrors the full branch.** Whole packages run in one `go test` invocation. Each
   slice then runs in its own invocation (`testArgs`, then the flag, the pattern and the
   package). All invocations share one 70-minute deadline, and any failure fails the shard.
   - `selection.json` gains `ShardSlices`.
   - `execution.json` gains `Slices`, with each slice's arguments and exit.
   - Both fields are `omitempty`, so a run without a split package writes the same documents and
     argv as before.
   - `str-parity` reads only unsharded `Args`, and container mode refuses sharding, so neither
     changes.
5. **Ambiguity refuses; staleness does not.** The checks run before execution and before the
   source check. Any of the following refuses with exit 2, a reason naming `AFP-V0-048` and no
   execution record:
   - a plan with the wrong shard count;
   - a package placed other than exactly once;
   - a split package that also runs whole;
   - a slice placed other than exactly once;
   - inconsistent slice counts;
   - a malformed index, flag or pattern;
   - named slices of one package that overlap, or a catch-all that does not skip exactly their
     union.

   A FULL fallback would execute the same placement, so it refuses too. Two conditions are not
   refused, because the full path tolerates them and the driver mirrors it:
   - **Stale names.** A removed name matches nothing; a new or renamed test lands in the
     catch-all `-skip` slice.
   - **An unusable or unadmitted slice file.** It keeps the affected package whole.

   The digest, shard-source and source-drift checks are unchanged.

## Evidence

- `TestAFPV0048DriverPlacesTheFullBranchUnits` covers 2 to 8 shards over a two-split fixture.
  Each FULL shard equals `PlanFrom`. Every package runs once whole, or each of its slices runs
  once and never whole. A narrowed selection with a split package covers exactly its slices.
  Shard counts below the slice count keep the package whole.
- `TestAFPV0048ZeroSlicesKeepTheIntersect`: with no split, FULL and narrowed placement equal the
  earlier `cishards.Intersect` for 2 to 8 shards. An empty slice file splits nothing.
- `TestAFPV0048MismatchedSliceFileMatchesTheFullBranch` covers seven variants: a package outside
  the universe, a package not allowed, unsorted names, overlapping slices, an invalid name, the
  wrong profile and an unknown member. In each, the driver layout equals `PlanFrom` and the
  affected packages stay whole.
- `TestAFPV0048CommittedSlicesArePlacedOnce`: the committed files split `internal/tasks/store`
  over 8 shards. Each slice is placed exactly once and the package never runs whole.
- `TestAFPV0048DriverRefusesAmbiguousPlacement` covers 15 broken placements, each refused with
  `AFP-V0-048`.
- `TestAFPV0048ShardedRunExecutesSlices` runs real `go test` on the fixture repository over 2
  shards, for FULL, narrowed and stale-name runs:
  - every top-level test terminates exactly once over the shards;
  - `b` runs only in FULL;
  - the slice argv equals the audited placement, and the exit codes follow the failing test;
  - a plan that loses the slices refuses in `execute` with exit 2 and no `execution.json`.
- Runs on this host (load average near 57):
  - `TestAFPV0048ShardedRunExecutesSlices` passed in 1379 s on commit b8bbd21d3, before the
    Codex fix. Its fixture layouts are disjoint, so the later overlap check does not change them.
  - With `-race`, the five pure AFP-V0-048 tests and both `TestSharded*_AFPV0022` tests passed
    after the fix.
  - Under `-race`, the rest of the package and the real-execution test were not rerun, because
    of load. CI runs the package in full.
- The existing `TestShardedPRExecution_AFPV0022` and `TestShardedSourceDrift_AFPV0022` still pass
  unchanged, which covers the zero-slice end-to-end path.
- Mutation checks (temporary edits to `main.go`, reverted):
  - Dropping the slice placement in `shardUnits` fails `DriverPlacesTheFullBranchUnits` and
    `CommittedSlicesArePlacedOnce`.
  - Skipping the `shardLayout` checks fails `DriverRefusesAmbiguousPlacement`.
  - Skipping only the overlap check (`partitionsTests`) fails its three cases.
  - Replacing `Plan` with the slice-blind `Partition` fails four of the five pure tests.
  - Dropping the slice invocations in `execute` was not mutation-run, because of host load.
    `ShardedRunExecutesSlices` asserts that `TestValue` terminates exactly once and that each
    slice index runs once.

## Limits and NOT_PRODUCED

- No hosted sharded driver run: `NOT_OBSERVED` until V1-0812 sets `CORVINT_PR_TOOL_SOURCE`.
- Stale wording remains in protected files this lane may not edit. Both are friction for the
  owner or V1-0812:
  - the `.github/workflows/ci.yml` comment cited above;
  - `.github/cishards/slices.go` line 208, which says "The package-level driver refuses sharded
    execution while it does". `Splits` now has no production caller; only tests use it.
- `corvint-tasks ticket show V1-1361` refused from the lane clone ("ticket does not exist in this
  queue"), because the journal lives in the primary checkout. The lane worked from the charter
  text.
- One Codex round (`codex exec -s read-only`) over `adf587fdb..b8bbd21d3` returned two
  findings:
  - **P2 (fixed).** `shardLayout` accepted overlapping named slices, and a catch-all whose skip set
    differed from their union. `partitionsTests` now refuses both, with three new refusal cases.
  - **P1 (recorded, not fixed here).** Slice invocations run without the full path's
    `-exec test-confine` and `CORVINT_CONFINE_ROOT`. This is not new: the driver's whole-package
    invocation has the same gap at BASE, which is the AFP-V0-023 residual for the AFP-V0-013
    driver. AFP-V0-028 already requires the full branch's wrapper before narrowing, so it belongs
    to V1-0812. AFP-V0-048 states it as a limit.

## Rollback

Revert the change. That restores `refuseSlices` and the AFP-V0-041 (6) refusal. Because the
pins are empty, no CI run depends on this code until V1-0812.
