# Fast-PR CI cadence (V1-4750)

The owner approved "fast PRs; full targeted/nightly/release checks". A pull request or merge-queue
entry whose whole change is a build-log file (`docs/build-log/<name>.md`) or an ordinary Go test file
now DEFERs three slow advisory campaigns (CIN-V0-4750..4756). Everything else, and any classifier
doubt, runs FULL exactly as before.

## What is deferred, and where it still runs

- The native archive gate (`artifact-integrity`): skipped under a pull-request DEFER only. The merge
  queue still runs it, so `static-reuse` lets the main push reuse it, and it is part of `make gate`
  (`go-archive-gate`) in the nightly release gates.
- The selection-baseline replay inside `doc-gates`: prints NOT_RUN under a verified DEFER in a pull
  request or queue entry. It replays historical commits with binaries that never compile
  `_test.go` files, so a DOCS or TESTS change cannot alter it; the nightly release gates run
  `make selection-baseline-check` as a new `selection-baseline` entry (CIN-V0-4755).
- The macOS shards of `go-macos`: no runner starts under DEFER; the nightly macOS gate runs them
  (CIN-V0-4754).
- `doc-gates-record` and `artifact-integrity-record` record nothing under a pull-request DEFER, so the
  queue runs its own `doc-gates` (minus the deferred replay) instead of reusing a partial verdict.

## Trust model

Only the classifier copy from the event base runs, from a sparse checkout; the tested tree is read
through git only. A pull request must be the two-parent merge of its base and head. For the merge
queue the head equals the tested commit, the base is its ancestor, and the whole range is classified.
Any error is FULL; `run.sh` validates the eight-line receipt and always exits zero. A base without the
classifier (first rollout) is FULL. A test file in or under a directory whose non-test Go code has a
`go:embed` directive, or one starting with a byte-order mark (which hides a build constraint from the
lexical guard), is FULL. Every consumer acts only on a verified DEFER: a successful `cadence-plan` job
with output DEFER.

## Independent review

One independent review found no way for a pull request to get DEFER for a change that can affect the
skipped campaigns. It found seven issues, all fixed here: queue `artifact-integrity` skipping broke
the main push's static reuse; the queue ran the full replay for routine changes; an embedded-directory
test file and a byte-order-mark build constraint could DEFER; the missing-classifier fallback was
untested; DEFER checks were inconsistent; and a full-history clone sat on every pull request's
critical path (now depth 2).

## Rollback

Set the repository variable `CI_CADENCE_FORCE=FULL`, or revert the change. No state migrates and
product features are unchanged.

## Evidence

`TestCINV04750CadenceClassifier`, `TestCINV04752CadenceRefusals`, `TestCINV04752CadenceRunWrapper` and
`TestCINV04753CadenceWorkflowWiring` in `tools/ci-reuse-plan/cadence_test.go`. Hosted behaviour is
NOT_OBSERVED until the change runs in CI.
