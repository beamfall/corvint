# Decision 0320 — A free-plan CI control plane and a hosted qualification workflow

Date: 2026-09-19. Status: accepted (owner call: "as recommended", on selective PR tests). Amends
decision 0289's protected-workflow gate and AFP-V0-014's serial campaign wording; it sets no pin.

## Context

Decision 0289 keeps the PR test pins empty until the owner configures "the applicable GitHub
required-workflow/ruleset policy" so a PR cannot replace the trusted workflow or its pins.
Organization rulesets, which carry required workflows, need GitHub Team or Enterprise; Beamfall is
on the free plan. Repository rulesets are available on the public repository, but a `pull_request`
workflow always runs the PR's own copy of `ci.yml`, so a required `go-product` check alone would
accept a PR that edits its own pins and then passes its own narrowed run.

The 200-row campaign also needs a Linux host identical to CI. At 35–45 minutes per full race row,
a serial campaign is about six days of one runner.

## Decision

1. `AFP-V0-016`: `.github/workflows/ci-control-plane.yml` runs on `workflow_run` after each `CI`
   run. GitHub runs that trigger from the default branch's copy, so a PR cannot alter it. It never
   checks out or runs PR code; it reads PR metadata through the API and posts the commit status
   `ci-control-plane` on the PR head: `failure` when any changed path, or any renamed-from path,
   is under `.github/`, or when the PR exceeds the 3000 files the API lists; `success` otherwise.
   An API failure stops the job with no status, so the merge stays blocked. It is not
   `pull_request_target`, and it holds only `contents: read`, `pull-requests: read` and
   `statuses: write`.
2. A repository ruleset on `main` requires a pull request and the `go-product` and
   `ci-control-plane` checks. The only bypass actor is the repository admin role, in
   pull-request mode, so a change to `.github/` merges only through an explicit, logged admin
   bypass. The ruleset is created only after (1) is on `main`; before that, no PR could pass it.
   Decision 0390 amends this step: `doc-gates` is also required, there is no bypass actor, and a
   `.github/` change merges after an admin posts `ci-control-plane` `success` on its head SHA.
3. Threat model: a fork PR's token is read-only and cannot post a status to the base repository;
   a same-repository branch is pushed by someone who already holds write access, which the
   ruleset does not claim to contain. Any status other than the workflow's own requires write
   access. With (1) and (2) in place, decision 0289's protected-workflow gate is met for this
   repository; the pins still stay empty until a qualification artifact exists.
4. `AFP-V0-017`: `.github/workflows/pr-tests-qualification.yml` is owner-dispatched only
   (`workflow_dispatch`, `contents: read`, no persisted checkout credentials). One job builds
   the planner, selector and driver from a named reviewed `tool_source` commit and freezes the
   corpus ending at `corpus_end`; each matrix job runs one frozen row (`--row N`, AFP-V0-015) on
   its own fresh hosted runner provisioned exactly as `ci.yml`; a final job runs `--mode
   qualify` over the retained rows with the frozen driver. `rows=1` measures one row first;
   `rows=all` runs the 200. Rows may run concurrently on separate runners: each row still has
   one host, one cold cache and one full invocation, and every row's recorded identity (Go
   binary, OS release, compiler) must equal the frozen one, so a runner image change mid-campaign
   fails qualification instead of mixing hosts. This replaces "serial" in AFP-V0-014 for hosted
   runs only. Outputs are artifacts; the workflow commits, pins and publishes nothing.
5. The campaign cannot start yet: `freeze` requires 201 first-parent commits on `main`, and on
   2026-09-19 `main` has 15. The requirement is not lowered.

## Rollback

Delete the ruleset; remove `ci-control-plane.yml` (the ruleset must go first, or every PR
blocks); remove `pr-tests-qualification.yml`. No pin, source, or other gate changes.
