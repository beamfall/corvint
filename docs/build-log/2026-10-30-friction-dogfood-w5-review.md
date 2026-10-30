# Independent review of the friction-dogfood-w5 lane

Lane friction-dogfood-w5; immutable base `c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`. One Codex
review round (read-only) over the delivered lane diff returned eight P2 findings. All eight were
judged valid and fixed in one follow-up commit, each with a regression test. There was no second
review round.

| # | Finding | Resolution |
|---|---------|------------|
| 1 | A `--refresh` plan or a staged finish-lane plan that is a FIFO blocked forever in `readPrefix` | `readRefreshPlan` and `stagePlan` check `isRegular` before opening. A FIFO is refused `refresh-plan-unavailable` or left to the ordinary input refusal. Test: `TestProposeCitationsDCWV0442RefreshRefusesFIFO`. |
| 2 | `mapStale` compared only path and BASE range, so a content-only amend kept the map's obsolete hunk IDs | When the map records an ID, it must equal the current patch's ID at that range (`DCW-V0-441`). Test: `TestProposeCitationsDCWV0441ContentAmendMakesMapStale`, which fails without the ID comparison. |
| 3 | The same-change requirement rule ran after the sealed and anchored proposals, so a BASE line naming the path displaced the mandatory unknown row | The check now runs first (`DCW-V0-440`). The competing-anchor fixture in `TestProposeCitationsDCWV0440SameChangeRequirementStaysUnknown` fails on the old order. |
| 4 | For an enrolled owner's `local-completion/<key>/state.json`, the aggregate-marker refusal suggested moving the state aside, which leaves the owner naming missing state | That marker now names the keyed `dogfood status` and `dogfood finish --aggregate-outcome-profile` recovery (`DCW-V0-448`). Test: `TestDCWV0448EnrolledStateMarkerNamesKeyedFinish`. |
| 5 | The `receipt` and `report` schema fixes said to rerun `dogfood change BASE`, which the aggregate-marker guard refuses while the malformed file is present | Both fixes first move the file aside (`ASD-V0-089`). |
| 6 | `dogfood-batch-check.sh` exited before running the check when the report directory could not be created | The check always runs, and an unwritable report prints `NOTE report NOT_PRODUCED` (`ASD-V0-090`). Covered by `script/dogfood-cache_test.sh`. |
| 7 | `dogfood-check.sh --bound-base` read `baseRevision` with a regex, so a nested-only member was accepted | A string-aware awk scan reads only top-level members. A missing, repeated or non-hex member fails closed (`ASD-V0-085`). Covered by `script/dogfood-cache_test.sh`. |
| 8 | `--refresh` shared one history budget between first-parent commits and the reflog | Each source has its own `refreshHistoryLimit` budget (`DCW-V0-442`). |

## Orchestrator Codex round

The orchestrator then ran its own read-only Codex review of `9ba44871f` against the same base. It
returned four P2 findings. All four were judged valid and fixed in one commit, which this lane's
single reseal covers along with the earlier post-seal line-citation repin.

| # | Finding | Resolution |
|---|---------|------------|
| 1 | `--refresh` resolved ordinal rows through the prepared map on disk, which a later prepare may have renumbered, so an old plan's ordinals could name other hunks | Ordinals resolve only through the map recorded by the citation-plan binding for this plan's digest (with or without its deferral rows). With no matching binding they are dropped with a note naming the full hunk ID (`DCW-V0-442`). Test: `TestProposeCitationsDCWV0442RefreshOrdinalsNeedBinding`. |
| 2 | Greedy forward stage packing refused a bindable plan: 216 two-row hunks then 144 one-row hunks packed 256/64/256, leaving 216 hunks before the middle stage | The stage before the final one is the longest trailing run that fits, and the rest pack forward. That plan now stages 64/256/256. The refusal detail says no plan-order boundary avoids it (`DCW-V0-446`, `DCW-V0-447`). Test: the mixed case in `TestFinishLaneDCWV0446PlanStagesSplitsOversizedPlan`. |
| 3 | `--refresh` dropped valid `deferred<TAB>PATH<TAB>REASON` rows as "not four-field" | Deferral rows that validate under `DCW-V0-370` are carried as written and counted as `deferred=`. Invalid ones are dropped with a note (`DCW-V0-442`). Test: `TestProposeCitationsDCWV0442RefreshOrdinalsNeedBinding`. |
| 4 | The `--bound-base` scan admitted `{"baseRevision":{},"baseRevision":"<sha>"}` and `{"baseRevision":"<sha>",}` | The CEM must pass `jq -e 'type == "object"'`, and a missing `jq` fails closed. The scan counts top-level `baseRevision` keys under any value type and requires exactly one (`ASD-V0-085`). Covered by `script/dogfood-cache_test.sh`, which fails with both guards removed. |
