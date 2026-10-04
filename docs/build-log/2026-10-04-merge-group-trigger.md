## 2026-10-04 V1-0752: CI runs on merge-queue entries

Human-owned intent: the owner asked for the `merge_group` trigger and will enable the merge queue
on `main` themselves. The queue is what closes the residual skews recorded for AFP-V0-025: two
changes that each pass against an older `main` and fail together.

### Change

- **`.github/workflows/ci.yml`** also runs on `merge_group` `checks_requested`. Every narrowing in
  the workflow is conditioned on `pull_request` or `push`, so a queue entry runs the complete
  suite on the commit that becomes `main`.
- **`.github/workflows/ci-control-plane.yml`** gains a `merge-group` job. The main ruleset requires
  `ci-control-plane`, and the existing job posts it only on pull-request heads, so without this
  every queue entry would wait for a status that never arrives. The job posts `success` on the
  head of a completed `merge_group` run of `CI` whose branch begins `gh-readonly-queue/main/pr-`
  and nothing otherwise. It runs no repository code.
- **Spec.** New `AFP-V0-026` in `docs/specs/affected-plan-v0.md`.

### Why the unconditional success is not a new consent path

A pull request can be queued only when its own head already satisfies the ruleset, which includes
the AFP-V0-016 decision or the admin's status on that head. A queue commit is `main` plus such pull
requests. Only GitHub raises `merge_group`; a branch pushed under the queue prefix raises `push`,
which `ci.yml` runs only for `main`.

### Limits

- Hosted queue behavior is `NOT_OBSERVED`: the queue is not enabled, so neither the trigger nor
  the new job has run.
- Each merge runs the suite on the queue commit and again on the `main` push, unless `main` had
  not moved and the pull-request run tested the same tree. AFP-V0-024 reuse admits only
  pull-request results, so it does not match the queue run of the same commit.
- `ci-control-plane` is posted only after the whole `CI` run completes, and `go-product` takes
  about an hour. The queue's status-check timeout (60 minutes by default, as recalled; not checked
  against current GitHub documentation) needs to be set well above that, about 120 minutes.
- Assumed GitHub behavior, not observed here: a pull request can be queued only when its own head
  meets the ruleset; `workflow_run` fires for `merge_group` runs; their `head_branch` is
  `gh-readonly-queue/main/pr-N-SHA`. If either of the last two is wrong the job posts nothing
  and the queue stalls rather than admitting anything.
- The success stays on a queue commit after its entry leaves the queue. A pull request opened with
  that commit as its head shows the status until its own `CI` run completes and the AFP-V0-016
  job replaces it. In that window it could be queued carrying `.github/` content that was already
  consented to on a queued head, and nothing else. Found in independent review; accepted as a
  limit, not closed.
- `intake-containment.yml` is not required and was left on its current triggers.
- This change is under `.github/`, so it merges only with the owner's `ci-control-plane` status
  on its reviewed head.

### Verification

`actionlint` on all workflows, `make ci-least-privilege-check`, the documentation gate steps and
`go test ./internal/specindex ./tools/docs-ci-plan ./tools/ci-reuse-plan ./internal/genesis` pass.
`corvint affected` selected 165 packages for a workflow change; the rest are left to hosted CI.
One independent review found no blocking defect; its limits are recorded above. `make gate` `NOT_RUN`
(owner's standing preference for scoped work).

### Rollback

Disable the queue in the ruleset first, then revert this change.
