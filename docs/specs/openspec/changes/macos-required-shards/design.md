# Design: `macos-required-shards`

## Decisions

- Two stages. Stage 1 (release gate) needs no ruleset change: the release workflow runs the
  exhaustive macOS suite on the freeze-candidate SHA and the RC SHA, and a failure blocks. Stage 2
  (merge gate) makes the `go-macos` aggregate a required check on `main` after the flake tickets are
  closed.
- Entry condition for stage 2. Every open ticket labelled as a hosted-macOS flake is closed, and the
  aggregate has passed on a run of consecutive `main` merges. The run length and any pass-rate
  threshold are not decided (Open; owner question).
- Workflow changes for stage 2:
  - add `merge_group` to `on:` so the required check reports in the merge queue;
  - remove `continue-on-error` from `go-macos-shard` and `go-macos`;
  - the cadence DEFER decision no longer skips the shards; a verified docs-only plan still skips, and
    the aggregate reports success with the reason `docs-only` so the required check is satisfied;
  - change the header comment so it no longer says the workflow never delays CI.
- Timing. `LINECOV_PLAN_P95_BUDGET` and other timing budgets stay advisory on hosted macOS
  (MCJ-V0-019); a budget miss is printed and does not fail the aggregate.
- Flakes. A macOS test that fails and then passes on rerun of the same SHA is a flake. Each flake is
  filed at milestone `v1-0`, priority P1, labels `agent-memory` and `bugs`, with the run URL, the
  test ID and both outcomes. The test is not quarantined silently; a quarantine needs its own ticket
  and keeps the test in the release gate.
- Windows. Native Windows vet and archive work leaves the 1.0 bar. The CI Windows vet step stays as
  it is until a separate change removes it. Removal lanes run `make windows-vet` once per removal
  batch, before that batch's push (R36); other lanes keep the AGENTS.md rule.

## Failure modes

- A required check that never reports (a workflow skipped at the trigger level): the merge queue
  stalls. Prevented by the `merge_group` trigger and by reporting success with a reason for docs-only
  plans instead of skipping the job.
- Hosted macOS capacity: queue time rises. Accepted; `max-parallel: 2` may be raised in the same
  change if the queue time exceeds the shard timeout (owner question on budget).
- A darwin-only failure introduced by a change that selected no darwin tests: caught by the
  exhaustive macOS run on the candidate and RC SHAs.
- A flake masked by rerun: the flake ticket is required before a rerun result is accepted.

## Acceptance evidence

- The release workflow log with the macOS exhaustive run green on the RC SHA.
- The ruleset showing `go-macos` required on `main` (read with `gh api`), after the owner's change.
- A merge-queue run where the aggregate reported.
- The list of macOS flake tickets, all closed at the RC.

## Rollback

- Owner removes `go-macos` from the required checks in the ruleset.
- Restore `continue-on-error: true` and remove the `merge_group` trigger.
- The release-gate stage stays unless the owner demotes it in the acceptance record.
- R36: restore the per-push sentence in AGENTS.md.
