# Nightly release-gates failure signal (V1-1114, SOP-V0-013)

Date: 2026-10-15. Ticket: V1-1114. Requirement: `SOP-V0-013` in
`docs/specs/stable-operations-v0.md`, new, accepted under the owner's standing auto-accept of lane
requirements (2026-10-11). No accepted requirement is amended.

## Problem

Scheduled `release-gates` runs 37628218263, 37784861599, 37935833612 and 38052349384
(2026-10-07..10) all failed on `main`, and nobody saw them until a V1-0848 review on 2026-10-10.
A failed scheduled run raised no alert and had no triage owner (decision 0435 added the schedule
but no signal).

## Contract

- A new `nightly-failure-signal` job in `.github/workflows/release-gates.yml` needs `gate` and
  `linux-core`. It runs only when `always() && github.event_name == 'schedule' && github.ref ==
  'refs/heads/main'` and a needed job's result is `failure` or `cancelled`. A dispatched run or a
  passing nightly never reaches it.
- Least privilege: the workflow-level block stays `contents: read`. Only this job holds
  `issues: write`, beside `actions: read` (reads its own attempt's jobs) and `contents: read` (sparse
  checkout). The checkout is `.github/nightly-signal` only, at `github.sha` (the scheduled `main`
  commit, so the code is trusted), with `persist-credentials: false`. No `${{ }}` reaches the script
  text: every value is passed through `env`.
- Dedup: `.github/nightly-signal/signal.sh` (sh, gh, jq) ensures the `nightly-triage` label
  (`gh label create --force`), then comments on the oldest open issue with that label (REST
  `issues?labels=nightly-triage&state=open&sort=created&direction=asc`, pull requests dropped), or opens
  `Nightly release gates failing on main` when none is open. One open issue therefore collects every
  failed nightly until the triage owner (the release owner) closes it after a passing nightly.
- Body (`body.jq`): the attempt URL, the gated commit, and each job with a failing conclusion
  (`failure`, `cancelled`, `timed_out`, `startup_failure`, `action_required`), its failing step
  names and job URL. The signal job's own row has no conclusion yet and is never listed.
- Failure: any failed `gh` call exits 1 naming the call, so the job, and the run, show red instead
  of passing silently. A malformed `RUN_ID`, `RUN_ATTEMPT` or `SHA` stops before any call.

## Decisions

- No auto-close on a green nightly. The ticket allowed it ("may"), but it would run the
  issue-writing job on every passing nightly, against the "only on schedule plus failure" bound.
  Closing stays a human triage act.
- A hand-cancelled scheduled run counts as a failure (one comment). A job that exceeds its
  `timeout-minutes` can end `cancelled` rather than `failure` (inferred from the hosted UI, not
  measured here), so the condition accepts both.
- The step names of a `full-gate` failure are `Run full-gate`. Naming the failed `make` steps
  inside it would need log downloads and parsing; the job URL leads there in one click.
- `script/check-ci-least-privilege.sh` (ARTIFACT-V0-008) covers `ci.yml` only and is unchanged,
  since widening it to this workflow would amend an accepted requirement. The release-gates
  permissions are held by `TestSOPV0013WorkflowBinding` instead.
- The script lives under `.github/`, like `.github/forward-rows`, so changing it needs
  AFP-V0-016 admin consent.

- The job has `concurrency: {group: nightly-failure-signal, cancel-in-progress: false}`, so a rerun
  that overlaps the next nightly cannot run list-then-create twice in parallel. GitHub keeps one
  pending job per group, so a third overlapping signal would be cancelled (visibly) rather than
  duplicated.

## Independent review

One Codex round (`codex exec -s read-only`) over the diff. Findings, both fixed in this change:

- P2: list-then-create was not serialized; overlapping runs could open two issues. Fixed with the
  job-level concurrency group above, held by `TestSOPV0013WorkflowBinding`.
- P3: `gh issue list --limit 100` plus `min` did not guarantee the oldest open issue. Fixed by the
  oldest-first REST listing; the test's fake answers only that exact URL and includes a labelled
  pull request that must be skipped.

## Evidence

- Fixtures: `gh run view <run> --attempt 1 --json jobs` of runs 37628218263 and 38052349384,
  recorded read-only into `tools/ci-nightly-signal/testdata/`. Nothing was posted; no workflow was
  dispatched.
- `go test -count=1 ./tools/ci-nightly-signal/`: 4 tests pass (3 subtests in the failure test).
  The shell cases take 14-21 s each under a host load average near 65-70 on 12 cores.
- Mutations, each one failing its named test and then reverted: always opening instead of
  commenting fails `TestSOPV0013CommentsOnTheOldestOpenIssue`; a body without failing steps fails
  `TestSOPV0013OpensOneIssueWhenNoneIsOpen`; `issues: write` at workflow level fails
  `TestSOPV0013WorkflowBinding`; dropping the pull-request filter from the issue listing fails
  `TestSOPV0013CommentsOnTheOldestOpenIssue` (it comments on the labelled pull request #5).
- `actionlint` (all workflows) and `make ci-least-privilege-check` pass.
- NOT_RUN: a real scheduled failure after merge. The first failed nightly after merge is the live
  check, and its issue or comment should be read back then.
