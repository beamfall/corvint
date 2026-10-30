# Finish-lane staging and stale aggregate markers

Lane `friction-dogfood-w5`, native tickets `V1-3167` and `V1-3169`. The immutable base is
`c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`. The new requirements are `DCW-V0-446` to
`DCW-V0-448`, experimental under the standing lane auto-accept.

## Staged plans in finish-lane (V1-3167)

`dogfood finish-lane` passed one `--citations` plan through every pass. A plan of more than
256 rows was therefore refused as `citation-plan-row-limit-exceeded` at prepare, even though
`DCW-V0-037` already defines a staged route on one retained map. Finish-lane now runs that route
itself.

The split is computed from the plan bytes alone. Each selector's rows stay together. The final
stage takes the selectors with the fewest rows (the later ones among equals) and every deferral
row, and the earlier stages take the rest in plan order. Because the split is deterministic, a
run resumed from the bind commit rebuilds the same final stage for its replaying rebind. The
stage files live in a private temporary directory under `$GIT_DIR/corvint/`, removed on exit.

A plan of at most 256 rows keeps the ordinary path and its exact output.

Decision: staging must not weaken the whole-plan checks. No stage names every hunk, so the
`DCW-V0-048` uniformity check (`uniformCitation`) would never fire on any single stage. The
uniformity check therefore runs on the whole plan against the first prepared map. A hunk named
in two stages by different selectors is also refused, because the stages could split its rows.
Plans that `DCW-V0-037` cannot admit are refused before any step runs (`DCW-V0-447`). These
include a plan whose last stage would need more than 256 rows once at most 256 hunks remain.

The manual staged route remains for explicit control over stage boundaries. Its own test
(`TestDogfoodSingleCommitStagedCitationPlans`) still passes a uniform `5:5` plan, because each
manual stage evades the uniformity check. That gap is recorded as friction, not changed here.

## Stale aggregate receipt (V1-3169)

`AggregateMarkerPresent` treated any `$GIT_DIR/corvint/local-outcome.json` aggregate receipt
that no plain report bound as enrollment. A bind pass writes a null local-outcome digest, so
after a sealed aggregate stage the next commit's rebind refused
`aggregate-enrollment-required` and named no file.

`dogfood-change` now resolves HEAD first and skips an aggregate receipt whose `target` is
another commit: that receipt is evidence for a different change. A marker that still applies
is refused with `marker: <path>` and a reversible `mv <path> <path>.stale` fix.
`dogfood check` keeps the target-free guard.

## Evidence

All of the following passed in an isolated tree holding BASE plus only this lane's files, run
through `script/heavy.sh`:

- `TestDogfoodFinishLaneDCWV0446StagesOversizedPlan` binds, checks (`hunks=300 supported=300`)
  and seals a 300-hunk commit. At BASE it fails `citation-plan-row-limit-exceeded`, which was
  observed.
- `TestFinishLaneDCWV0446PlanStagesSplitsOversizedPlan` and
  `TestFinishLaneDCWV0447PlanStagesRefusesUnstageablePlan`.
- `TestDCWV0448StaleAggregateReceiptForAnotherTargetIsIgnored`, which fails with the target
  check removed.
- The existing `TestDogfoodFinishLane*`, `TestDogfoodSingleCommitStagedCitationPlans`,
  `TestDCWV0374*` and `TestDCWV0375*`.

Rollback: remove `finishlane_stages.go` and the stage loops in `finishlane.go`, then pass an
empty target in `requireNoAggregateMarker`.
