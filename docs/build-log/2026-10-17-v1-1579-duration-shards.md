# V1-1579: duration-balanced shard planner and measured placement

Date: 2026-10-17. Ticket V1-1579, GitHub beamfall/corvint#1022. Spec
`docs/specs/session-test-runner-v0.md`, `STR-V0-050..054`, all experimental. The requirements are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11). They are not an
owner review of the design.

## Contract

- `corvint test shards --count N --files PATH [--durations PATH]... [--runs K]` prints one
  canonical `corvint-test-shards/0` plan.
  - Each history file is one retained full run in the `corvint-ci-package-costs/0` shape that
    `tools/ci-shard-costs refresh` writes, given oldest first.
  - Each listed file weighs the median of the last K admitted runs that record it.
  - Unknown files weigh the median known weight and are listed in `unknown`.
  - Files go heaviest first onto the lightest shard.
  - The partition digest binds only the profile, the count and the assignment.
- Any unusable history yields the complete round-robin. The plan records the reason in `fallback`
  and stderr says `FALLBACK`. One bad table discards the whole history, so a partial read never
  silently weighs the rest.
- `corvint test --placement measured --shard-plan PATH` places each run-set Go package whole on its
  planned shard.
  - The manifest's `partitionDigest` is the plan's, so receipts and `verify-shards` bind it.
    `verify-shards` is unchanged and stays `INCOMPLETE` for a missing shard receipt.
  - A count mismatch, or a plan that omits a run-set package, places no Go unit and is `INCOMPLETE`.
  - As with balanced placement, it executes only locally. CI may only `--dry-run`, and `.github/`
    is unchanged.
- The planner never reads `corvint-test-run/0` receipts (STR-V0-011). It reads only its named
  inputs and writes only stdout and stderr (invariant 4).

## Choices

- The history is the existing cost table. It has no new retention store: the operator supplies
  retained tables, and the git history of `.github/cishards/package-costs.json` already holds
  several.
- The plan is a separate input to `corvint test`. Planning the shards does not happen inside a
  run, so every shard of one sharded run uses one plan, pinned by its digest in the one dry-run
  manifest.
- Measured placement is whole-package and does not apply AFP-V0-041 test slices. A package heavier
  than the ideal shard sets the floor. This is a known limit, not a defect.

## Live qualification

Inputs:

- the 339 packages of `go list ./...` at `55abb815c`
- the six git-history versions of `.github/cishards/package-costs.json`, oldest first
- K=5 and 8 shards

Command: `corvint test shards --count 8 --files packages.txt --durations hist/1..6 --runs 5`.

Results:

- It exited 0 and printed `shards 339 files over 8 shards from 5 runs, 5 unknown`.
- A rerun was byte-identical.
- Digest: `sha256:fac0bfb69d5599592b2e3fc0fe161e168bf72ebc77d8a892fb9975ce46db626d`.
- The five unknown packages, which are newer than the tables:
  - internal/cem/corpusbinding
  - internal/liveverify/affected/java/javalex
  - tools/ci-forward-row
  - tools/ci-nightly-signal
  - tools/test-read-scopes-check
- Every package was placed exactly once (339 placed, 339 distinct).

| Shard | Files | Weight (ms) |
|---|---|---|
| 0 | 1 | 895576 (`internal/tasks/store` alone) |
| 1 | 38 | 824957 |
| 2 | 55 | 823961 |
| 3 | 55 | 823961 |
| 4 | 55 | 823960 |
| 5 | 45 | 824957 |
| 6 | 45 | 824957 |
| 7 | 45 | 824957 |

Follow-on checks:

- Adding a table without a revision gave the reported round-robin:
  `FALLBACK invalid run durations: bad.json names no full source revision`.
- `corvint test --placement measured --shard-plan plan8.json --shard 1/8 --dry-run` against the
  same base gave `NARROWED`, 185 execution units, no `incomplete`, and the plan's digest as its
  `partitionDigest`.

## Not run

- No measured shard execution on CI. CI placement is unchanged.
- No wall-clock comparison against universe or balanced placement. Promotion needs balanced
  placement's local 20% bar.
- No `make gate`. The focused package tests and doc-gates are recorded in the PR.
