# Tasks OUT_OF_SCOPE submit refusals name their paths (V1-5567)

Ticket V1-5567 in `queue:corvint:main`, filed by the Beamfall 1.0 orchestrator on 2026-11-15
against corvint-tasks 1.0.0-rc.3.dev+build.146. The work is in beamfall/corvint-tasks, so this
entry records the delivery for this queue.

## Decision and behavior

- A submit refused `OUT_OF_SCOPE` was withheld as "not valid prose" when a stale base made the
  path list huge. It now names the attempt's `baseCommit`, at most 20 offending paths within
  8 KiB (each ASCII-quoted when it is not valid prose), an omitted count and a `nextAction`, in
  prose and in a structured `outOfScope` result member (`baseCommit`, `paths`, `omitted`,
  `upstream {ref, commit, matching}`, `nextAction`).
- When the configured upstream tip already matches offending paths and `baseCommit` is a proper
  ancestor of it, the refusal names the stale base and the recovery: release, then claim again
  with `--base`. An upstream read error only drops that hint. Scope-check acceptance is unchanged.
- Governing requirement: corvint-tasks `CAL-V0-393`
  (`docs/specs/corvint-tasks-agent-leases-v0.md`).

## Evidence

- Delivered in https://github.com/beamfall/corvint-tasks/pull/8 (commits `bbe29bb`, `7a7afbf`),
  stacked on beamfall/corvint-tasks#7.
- Tests: `TestCALV0393_StaleBaseSubmitRefusalNamesPathsAndBase` (cli);
  `TestCALV0393_StaleBaseSubmitNamesBaseAndRecovery`, `TestCALV0393_OutOfScopeRefusalIsBounded`,
  `TestCALV0393_OutOfScopeRefusalBoundsBytesAndQuotes` (store);
  `TestCALV0393_DisplayPathQuotesHostileNames` (transaction). Focused store tests and the full cli
  and transaction packages pass; `go vet` and `gofmt -l` are clean. One independent review found
  no P0 or P1 issues.
- Installed locally as corvint-tasks 1.0.0-rc.3.dev+build.148, a local merge of
  beamfall/corvint-tasks#6, #7 and #8. Rollback: `~/.local/bin/corvint-tasks.pre-r148.bak`
  (build.147). corvint-tasks PR merge is NOT_PRODUCED here (that repository has no CI and merges
  by hand).
