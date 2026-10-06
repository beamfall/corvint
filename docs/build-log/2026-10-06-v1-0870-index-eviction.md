# 2026-10-06: V1-0870 named eviction and live worktree trees

## Intent

Ticket V1-0870. Running `corvint index --if-stale` in a linked worktree writes about 120 MB into the
shared `<common>/corvint/index` store. That write can silently evict a snapshot another session is
using. A fresh linked worktree's prompt hook was also reported to stay FALLBACK because of the
1.6 s `dogfoodEventDeadline`. The ticket accepts four results:

1. `index` names the shared store it wrote and every snapshot it evicted.
2. Eviction never removes a live worktree HEAD's snapshot, or else the policy is documented with a
   bounded alternative.
3. A fresh linked worktree whose tree already has a snapshot reuses it, and its prompt event
   succeeds.
4. Maintained tests cover the eviction report and the reuse.

## Already on main

- `DIRTY-CACHE-013` / V1-0212 (PR #132) already shares one store across linked worktrees, and
  `TestLinkedWorktreesShareOneCleanSnapshot` covers reuse.
- V1-0302 added the 1 GiB byte budget and engine-first ranking.

Three things were missing:

- the store path and the removed files in the receipt;
- any knowledge of live worktree HEADs in eviction;
- an end-to-end test that a linked worktree's prompt reuses a snapshot that survived another
  worktree's write.

## Change

- Spec `docs/specs/index-snapshot-v0.md` adds `IDX-SNAP-V0-025` (proposed, pending owner review),
  with amendments to 001, 007 and 011, failure modes, acceptance evidence and a traceability row.
- `WriteSnapshot` reads the live trees with `git worktree list --porcelain -z` and one
  `git cat-file --batch-check`, bounded by 10 s and 4 MiB. Bare, prunable and unborn worktrees
  contribute no tree. If either read fails, the receipt says `live_heads:"NOT_OBSERVED"` and the
  old order applies. Only this write verb reads the live set, so read verbs gain no process
  (invariant 4).
- Gob ranking is: the snapshot just written, then live trees, then the current engine, then newest.
  Both bounds are unchanged (eight per worktree up to 64 entries, and 1 GiB). Analyzer packs keep
  their age order but are now named.
- The receipt adds:
  - `store` and `store_shared`;
  - `live_heads` and `live_trees`;
  - `evicted_snapshots`, a list of `{kind, path, bytes, tree, engine, live_head}`. It is an empty
    list, never null, when nothing was removed.
  `evicted` stays as the list's length. The `--if-stale` fresh receipt adds `store`.
- `analyzerSchemaID` moves to `corvint-analyzer/106` because `TestAnalyzerSchemaInputs` pins the
  contextindex production source digest.
- Stale line citations in `falsifiable-packet-v0.md` and `revision-cache-dirty-worktree-v0.md`
  were repinned. The cited content only moved, so the hashes are unchanged.

## Decisions

- **Bounded live protection, not absolute.** The primary checkout has 143 worktrees registered.
  At about 120 MB per snapshot, the 1 GiB budget holds about eight. Absolute protection would
  either grow the store without bound or refuse the write. The chosen policy is the bounded
  alternative that acceptance 2 allows:
  - live trees rank first after the new write;
  - a live snapshot past the bound is still removed;
  - each removal is listed with `live_head:true`, so it is never silent.
- **Live outranks engine.** A live tree built by another engine (another binary build) ranks above
  a non-live current-engine snapshot, following the acceptance text literally. Owner question
  below.
- **Exact tree match only.** Reuse still requires the object format, tree and engine in the
  snapshot name to match exactly. Live trees only change ranking and never mark a snapshot fresh.

## Primary FALLBACK diagnosis

These checks were read-only on the primary checkout (`GIT_OPTIONAL_LOCKS=0`); nothing was written to
its store. Reproductions used a private clone and a private linked worktree under the lane tmp.

- **`frontier-authority-unavailable` is expected, not a defect.** `dogfoodEnvelope` and
  `dogfoodDegradations` attach `support:"FALLBACK"` and this degradation to every dogfood event
  receipt, whatever the lifecycle state (decision 0009, `change-frontier-profile-1.md`,
  `AHI-023`). So "not FALLBACK" in acceptance 3 cannot hold literally for any event. The test
  instead asserts what reuse controls: the event succeeds with a context packet on the
  snapshot's tree, with no build and no stale-snapshot refusal.
- **`dogfood-event-deadline` comes from latency, not from reuse or eviction.** The primary hits its
  own snapshot (tree `0cb16f6e`, 116 MB). It carries about 2,300 to 2,600 dirty paths, and
  `ProbeRepositoryContext` takes about 400 ms over that set. The probe runs twice per event. Under
  load averages of 17 to 45, that exceeds the adapter's effective budget of about 1.5 s from
  process start.
- **`dogfood-event-unavailable` hides the cause.** `cmd/corvint/local_completion_event.go` collapses
  every non-allowlisted error into it. A scratch reproduction in the private clone ran the event
  while a writer churned untracked files every 15 ms:
  - quiet runs succeeded;
  - churn runs returned `dogfood-event-context-drift` three times and `repository-state-unstable`
    once.

  The second message says "repository HEAD changed" even though only the dirty set changed
  (`internal/gokernel/repository.go`, about line 386). Other plausible hidden causes are:
  - `dogfood-event-repository-drift`;
  - `dogfood-event-policy-drift`;
  - `dogfood-event-native-budget`;
  - the context budget and bounds refusals.

  The primary's dirty set grows over time from concurrent writes (`.taskman/tickets` and similar),
  so drift between the two probes is the probable cause (inference: the set was stable during a
  six-sample check).
- **Eviction during a read is not the cause.** An open snapshot file survives unlink. A
  `readSnapshotIndex` failure is a miss, and with the recorded build cost it surfaces as
  `dogfood-event-index-snapshot-stale` (reproduced: 441 ms, `missed=true`), never as unavailable.
- **Proposed reason code (separate ticket):** an allowlisted passthrough. It maps
  `repository-state-unstable`, `dogfood-event-repository-drift` and `dogfood-event-context-drift`
  to `dogfood-event-worktree-churn`, passes `dogfood-event-policy-drift` through, and leaves the
  rest unavailable. These are fixed strings with no caller data, so this needs only an `LCP-V0-008`
  spec change. The adapter already passes any valid code through `adapterRejectedReason`.

## Evidence

- `TestEvictSnapshotsNamesRemovalsAndKeepsLiveHeadTreesFirst` covers three cases: live ranking,
  the old order with no live set, and live trees past the bound being flagged.
  `TestLiveWorktreeTreesNamesEveryLiveHead` covers three worktrees, one of them prunable, plus a
  missing root.
- `TestIndexKeepsALinkedWorktreesSnapshotAndItsPromptReusesIt` runs end to end:
  1. index in a linked worktree;
  2. fill the shared bound with 15 newer non-live current-engine snapshots;
  3. index in main, which names the one evicted file and keeps the linked snapshot;
  4. `--if-stale` in the linked worktree is fresh on the same path;
  5. its prompt event succeeds on the snapshot's tree with builds refused.
- Mutation check: forcing the live set to `nil` in `WriteSnapshot` makes both the unit test and the
  cmd test fail; restoring it makes them pass.
- `corvint affected --base 76f7f2ac` selected 113 units (`scope: UNKNOWN`). Run here: the
  `internal/contextindex` package and the `cmd/corvint` tests matching
  `Index|Snapshot|Evict|LinkedWorktree|DogfoodEvent`, with `GOMAXPROCS=3 -p 1`.
- gofmt, vet, the doc gates and the use-case receipts checks passed.

## NOT_RUN

- `make gate` and the full `./...` test run (owner preference for scoped issue work). The other
  111 of the 113 selected units were not run.
- Live qualification on the primary checkout: the lane forbids writing the primary's store.

## Owner questions

1. Acceptance 3 says "not FALLBACK", but every dogfood event says FALLBACK by design. Is the
   asserted "context packet with no build and no refusal" the intended meaning?
2. Is bounded live protection acceptable, or should live trees past the budget refuse the write or
   grow the budget?
3. Should a live tree from another engine outrank a non-live current-engine snapshot? This change
   says yes.
4. The `corvint-analyzer/106` bump may collide with concurrent lanes that also bump the schema at
   integration.

## Follow-up candidates

- The `dogfood-event-worktree-churn` passthrough described above.
- Correct the `repository-state-unstable` message so it names dirty-set churn instead of HEAD.
- Make one repository probe per dogfood event instead of two on large dirty worktrees, which is the
  deadline root cause.
