# Post-merge replay: integration onto current main, deferred authoring stages

Issue #395 (native V1-0542) was held on 2026-10-01 with the PMR-V0 harness unpublished on
`codex/issue395-postmerge-replay` (base `9afd8313`). This entry records the decisions taken
when carrying that harness onto `origin/main` `ff3da727e95a1999cbc4a58a471cdd498693f902`.

Reuse, not duplication. The two harness commits applied cleanly; the connector APIs they call
(`postmergeconnector.Read`, `Build`, `ValidatePlan`, `Record`) are unchanged on main, and the
prior independent plan and code review findings stay resolved. The companion moves from
`tools/post-merge-workflow` to `cmd/corvint-postmerge-workflow`, next to the sibling
`corvint-postmerge-connect` and `corvint-postmerge-metrics` companions.

Detection separated from authoring (PMR-V0-005, amended). Before this change any documentation
target or test gap blocked the replay, so the `test_gaps` expectation could only ever be empty and
most historical merges could not be replayed. Now the harness fixes those author/scope/validation
stages and draft requests as `deferred` whenever the work exists. A driver that claims them
`observed`, or emits any connector draft, still blocks: a stage digest cannot self-certify authored
content until the scope (#393) and acceptance (#394) verifiers are integrated. Detected counts
reach the recorded follow-up request, and the report lists the deferred stages with a limit.

Report separation (PMR-V0-007, amended). The single basis-tagged mismatch list becomes
`human_verified_mismatches` and `generated_mismatches`, as the issue's report requirement asks.

Delta record (PMR-V0-009, new). `corvint delta` (#389) is not on the base. Replay does not parse
or duplicate it: the adapter result is the narrow interface, carrying flow and gap identities and
the record digest as the `delta` stage artifact. Actual delta integration is `NOT_PRODUCED`.

Remaining for #395: actual delta, author, scope and validation stage integration; historical
fixtures with real expectations; CI host containment qualification; integration and native
completion. Whole-workflow qualification stays `NOT_OBSERVED` in every report.
