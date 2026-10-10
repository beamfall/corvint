# Decision 0488 — test-level CI shard splitting (AFP-V0-041) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept AFP-V0-041"), relayed by the coordinating agent.

## Context

AFP-V0-022 places whole packages across the six full-CI shards, so the slowest package sets a floor
under the slowest shard. In hosted run 38055182050 `internal/tasks/store` alone took 1,871.8s
against an ideal shard of 1,280.8s. Lane V1-1096 proposed `AFP-V0-041` in
`docs/specs/affected-plan-v0.md`:

- Only a package that the project-owned allow-list `.github/cishards/test-split-allow.json` names,
  with a reason recording that its tests neither depend on order nor share `TestMain` or package
  state, may split. The slice file `.github/cishards/test-slices.json` gives it 2 to
  min(16, shard count) slices: named slices run as `go test -run '^(A|B)$'`, and a catch-all runs
  as `-skip` over the union, so every top-level test runs in exactly one shard. Unusable input keeps
  packages whole.
- `tools/ci-test-slices generate` writes the slice file from one complete, passing hosted run,
  enumerating tests in a temporary checkout of exactly the recorded revision. `replay` predicts
  shard sums from that run's logs.
- The protected helper in `.github/cishards` places slices, never two siblings in one shard, and
  its profile digest covers the slice code and both files. `refuseSlices` in
  `tools/corvint-pr-tests` refuses sharded execution while anything splits.
- The full-CI tests step runs whole packages and then each slice, through `run_tests`, and fails
  the shard when any invocation fails.

Two independent Codex review rounds found three defects each: incomplete or failed logs accepted,
enumeration not bound to the revision, a skipped source check on empty generation, and a leaked
checkout on interruption, among others. Each was fixed in a following commit with mutation-tested
coverage. Round 2 also merged main's AFP-V0-040 work (decision 0487): every invocation now runs
inside the one shard-cost capture, the `go-static` copy of the helper gained the slice files, and
`tools/ci-shard-costs` combines a split package's slice outcomes into one cost (AFP-V0-041 (8)).
AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AFP-V0-041` as written. The requirement status, the spec header and digest, the
`docs/specs/README.md` row, `docs/specs/INDEX.json` and the lane's build log
(`docs/build-log/2026-10-10-ci-test-level-shards.md`) record the acceptance. This decision accepts
no other requirement of the spec.

## Limits

This decision settles intent only. The evidence is the `TestAFPV0041*` tests, the shared-helper
tests, `actionlint`, `make ci-least-privilege-check`, the doc gates, a replay of run 38055182050
and local runs of the slices and of the workflow's slice loop and capture.

- No hosted sliced run exists yet. Paired hosted timings and per-slice overhead (repeated build,
  link, process start and `TestMain`) are `NOT_RUN`, so no hosted speedup is claimed.
- Splitting loses cross-slice parallel interleavings, including race detection between tests in
  different slices.
- `tools/ci-test-slices generate` still refuses a sliced run, so regenerating the slice file needs a
  run in which the package ran whole.
- While a package is split, setting the AFP-V0-014 selective PR pins fails every sharded PR run
  closed until the slice file is emptied or the driver learns to run slices. The pins are empty
  today.
- The change edits `.github/workflows/ci.yml`, so its pull request needs the admin-posted
  `ci-control-plane` status (AFP-V0-016) before it can merge. Nothing in this decision posts it.

## Rollback

Revert this decision and return the AFP-V0-041 status text to proposed and experimental, pending
owner acceptance, in the requirement, the spec header and digest, the README row, `INDEX.json` and
the build log. Then regenerate `docs/specs/REQUIREMENTS.tsv`. To withdraw the behaviour, set
`packages` to `{}` in `.github/cishards/test-slices.json`, after which the plan equals AFP-V0-022,
or revert the workflow slice loop and the helper files. No stored state or required check depends on
it.
