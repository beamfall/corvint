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
