# Selwide: workflow-placement subtest moves to the ci.yml follow-up PR

PR #140 (V1-5018 / AFP-V0-9012) carried the `TestAFPV0098SupersetGuardSidecar`
subtest "AFP-V0-9012 shard 0 runs hidden packages outside placement", which
extracts the shard placement and execution filter from
`.github/workflows/ci.yml` and asserts that shard 0 runs
`corvint-pr-tests --mode packages` output outside placement. The matching
`ci.yml` change ships in a separate follow-up PR, because the
github-split-check gate refuses a range that mixes `.github` and code. Required
check `go-product-shard (2)` therefore failed in `tools/ci-forward-row` with
"workflow placement/filter or hidden discovery not found".

Orchestrator decision: remove that subtest and its `buildPackageDiscovery`
helper from this PR unchanged, so they land with the `ci.yml` follow-up PR.
Every subtest of the scenario set (narrowed, disagreement, FULL, other shard,
discovery failure) reads the workflow text, so none is a non-workflow assertion
that could stay. All other AFP-V0-9012 evidence (selection, FULL argv, universe,
receipt check, sharded race-only placement, source drift) stays in this PR and
is unchanged. `tools/ci-forward-row/narrowing_test.go` is now byte-identical to
its merge base. The AFP-V0-9012 traceability row records the workflow-placement
evidence as pending that follow-up PR.

No assertion of any other test changes. `.github` is untouched. Rollback:
restore the subtest from the follow-up PR, or revert this commit.

The `go-macos-shard (1)` failure at `additional_freshness_test.go:251` (warm
total p95 1043 ms against a 1000 ms bound) is an advisory macOS shard and is
treated as a likely performance flake; its bound is unchanged.
