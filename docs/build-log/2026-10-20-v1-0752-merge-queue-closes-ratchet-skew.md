# V1-0752: the merge queue closes the unbounded-readers ratchet skew

Ticket: V1-0752. Outcome: already delivered on origin/main; evidence recorded, no code change.

The skew was a pull request whose `unbounded-readers-check` passed against an older `main`
merging after an unbounded test package reached `main`, which left `main` failing the check.
Its acceptance asks that such a pull request cannot leave `main` failing, or that the failure is
detected before the merge completes.

Evidence, read 2026-10-21 at origin/main 5cf75daf09d3572cd5a134297566c73cba3f8b61:

- `.github/workflows/ci.yml` runs on `merge_group` (build log
  `2026-10-04-merge-group-trigger.md`, `AFP-V0-026`), and its `doc-gates` job runs
  `make unbounded-readers-check` with no event condition, so a queue entry checks the exact
  commit that becomes `main`.
- The `main` branch rules (`gh api repos/beamfall/corvint/rules/branches/main`) contain
  `merge_queue` and `required_status_checks` with `go-product`, `ci-control-plane` and
  `doc-gates`.
- `gh run list --workflow ci.yml --event merge_group` shows completed queue runs on
  `gh-readonly-queue/main/pr-1394-5cf75daf...`, `pr-1379-...` and `pr-1374-...` on 2026-10-21.

So a pull request merging after another one added an unbounded package is re-checked on the
combined commit, and a ceiling breach fails `doc-gates` and removes it from the queue before
`main` moves. Limit: no queue run that actually rejected such an entry was observed; the
mechanism is the one the check already runs under, not a separate reproduction.
