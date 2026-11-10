# Design: `macos-required-shards`

## Decisions

- Two stages (owner decision H11 (2026-11-10); D7, REL-V1-015). Stage 1 (release gate) needs no
  ruleset change: the release workflow runs the exhaustive macOS suite on the freeze-candidate SHA
  and the RC SHA, and a failure blocks. Stage 2 (merge gate) makes the `go-macos` aggregate a
  required check on `main`; until then the per-merge check is advisory.
- Entry condition for stage 2. Every open ticket labelled as a hosted-macOS flake is closed, and the
  aggregate has passed on 20 consecutive `main` runs (decided; D7). A `main` run is a
  `merge_group` run of the aggregate for the `main` merge queue: the workflow has no `push`
  trigger and none is added, and the merge-queue run tests the commit that lands on `main`. The
  count starts when task 2.3 lands; a docs-only skip is not counted and does not break the
  sequence. No pass-rate threshold is added. If the condition is not met by 2026-12-15, the
  per-merge check stays advisory for 1.0 and only stage 1 applies; the ruleset is not changed.
- Workflow changes (task 2.3; they do not by themselves make the check required):
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
  and keeps the test in the release gate. No standalone rule holds the RC for an open flake ticket
  (owner decision H11 (2026-11-10)): the macOS release gate and the general open-P1 rule cover it.
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
- Stage 2 not reached by 2026-12-15: the per-merge check stays advisory for 1.0. Accepted; the
  release gate still catches a darwin-only failure on the candidate and RC SHAs.

## Acceptance evidence

- The release workflow log with the macOS exhaustive run green on the RC SHA.
- If stage 2 is reached: the ruleset showing `go-macos` required on `main` (read with `gh api`),
  after the owner's change, and the 20 consecutive passing `main` runs. If not: the state of the
  flake tickets and the run count on 2026-12-15.
- A merge-queue run where the aggregate reported.
- The list of macOS flake tickets and their state at the RC.

## Rollback

- Owner removes `go-macos` from the required checks in the ruleset, if it was added.
- Restore `continue-on-error: true` and remove the `merge_group` trigger.
- The release-gate stage stays unless the owner demotes it in the acceptance record.
- R36: restore the per-push sentence in AGENTS.md.
