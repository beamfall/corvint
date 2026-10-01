# LSP qualification freeze record

V1-0478. Change base: `cdd2e31fffd732419eb9ade531ccf289259f3243`.

## Decision

The owner-accepted evaluation policy (`docs/specs/lsp-qualification-evaluation-policy-v0.json`,
SHA-256 `0bd7797cbefa843902e7e2448c9bf36773e4fb892ef977e79087ada85ac607af`) already fixed the floors,
measurement protocol and custody rules. Its digests and baseline states lived only in volatile `/tmp`
evidence, and nothing machine-checkable tied tuples, floors and baselines together.
`benchmarks/lsp-quality/qualification-freeze-v0.json` is now that contract. LQP-V0-019..021 govern it,
and `benchmarks/lsp_quality_freeze_test.go` enforces it.

- The six candidate tuples are agent CLI, agent MCP, and VS Code/Neovim × definition/`corvint/context`.
  All are `UNQUALIFIED`. Each cites declared platform, provider and client identities with observed
  digests, and names its owning tickets (V1-0479..0483). Corvint builds are `PER_CANDIDATE`.
- Every hard, quality and resource floor in the policy is mirrored exactly (23 keys). Each has an
  explicit held-out state. All are `NOT_RUN` with reasons, and public calibration notes stay
  separate and cannot satisfy a floor.
- The protected held-out manifest (20 tasks, 3 repositories) is `NOT_PRODUCED` and has no digest.
  An independent custodian outside the builder workspace must produce it. No existing sealed
  holdout was opened.
- Three real-repository candidates are pinned to exact commits, but none is admitted:
  - `beamfall/corvint@25730eaa`
  - `golang/tools v0.48.0@05f9cb5d` (go.work multi-module)
  - `golang/sync v0.22.0@1eb64d4b`

  The upstream tags were checked with `git ls-remote` on 2026-10-01. Admission is blocked for three
  reasons:
  - gopls-module dependencies are absent from the local module cache, and nothing was downloaded;
  - the dependency closure and prepared-tree digests are `NOT_RUN`;
  - custodian admission has not happened.

Release authenticity is `NOT_OBSERVED` for Go, gopls and VS Code; only local digests are bound. The
Neovim archive digest matched the official release asset digest.

## Public baseline rerun

`script/measure-lsp-quality.py --repeat 3` was run with Corvint built at the change base (executable
SHA-256 `fee170c1…`) against a clean clone at sourceBase `cf93522e`. Report SHA-256:
`8af789de8b5f0da0d361d746072ee68106dfff78de3927f5b05b255d4b684fb4`, retained in builder scratch
only. Results:

| Arm | Gold | Detail |
| --- | --- | --- |
| upstream | 6/6 | exact definition spans |
| Core | 6/6 | expected paths |
| combined | **0/6** | provider `unavailable` in 4/6; in 2/6 it stopped at `soft-deadline` after 2 queries with 1 relation |

Core packet parity held in 6/6 samples, and the harness exited 1.

The 1-minute load average was 160–336 during the run, and the combined misses coincide with the
20000 ms provider wall budget. This run cannot separate host load from a product defect. It is
recorded as a FALLBACK public observation, not as a quality, latency or regression measurement.
The corrected 2026-09-30 public-main path-level run that the policy cites as
`baselineEvidence.validPathOnlyMain` (report `a1f1fb1d…`, revision `25730eaa`, 6/6 in every arm) and
the 2026-09-29 path-level run (report `748a8235…`) are listed beside it for history. Neither report
is committed, and the Corvint commit of the 2026-09-30 run is `NOT_OBSERVED`. All timings are
descriptive only.

## Acceptance state

| AC | State | Basis |
| --- | --- | --- |
| 1 | PARTIAL | The policy is owner-accepted. Owner acceptance of this record version is pending. |
| 2 | PARTIAL | Public labels are frozen. The held-out set is `NOT_PRODUCED`. Real repositories are not admitted. |
| 3 | PARTIAL | Floors are numerical and frozen. Held-out baselines are `NOT_RUN`. |
| 4 | NOT_MET | Review, integration and native completion are pending. |

## Record digest and digest checks

The record's own SHA-256 is
`3c6b8990c4be8fce06882139392ae54d9e8eaf5cf5a5d80aa3646acd94328663`. The test pins it, so any in-place
edit of version 0 fails, and a downstream claim cites this digest (LQP-V0-019). Platform, provider
and client digests name external binaries, archives and packages outside the repository. The test
checks only their format and never re-hashes those artifacts. The policy and public corpus digests
are recomputed from the tracked files.

## Mutation tests

`TestLSPQualificationFreezeRecord` re-validates mutated copies of the record and requires each to be
rejected for the named reason:
- an unknown top-level field, or `heldoutGold`/`heldoutTasks` inside a public baseline or its
  results (every nested object, public baselines included, is a closed struct);
- a malformed or incomplete public baseline;
- a deleted or blank `status`, `ticket`, `frozenAt` or `limits`, or a null `publicBaselines` (every
  top-level field must be present and non-empty);
- a QUALIFIED tuple, a drifted floor, or a missing floor metric;
- AC1, AC2 or AC3 marked MET while the protected held-out is `NOT_PRODUCED`, and AC3 marked MET on a
  frozen held-out whose baselines are still `NOT_RUN`;
- a repository marked `ADMITTED` that still lists blockers.

Each guard was confirmed load-bearing by disabling it and observing the matching case fail.

## Review repair r1

The independent review returned FAIL. The schema for `publicBaselines` was open (`[]json.RawMessage`).
Deleted top-level fields were accepted. AC1/AC3 and admitted-repository states were unguarded. The
record digest was unpinned. All of these are fixed above. No floor, tuple, corpus or custody value
changed. The only record addition is the policy-cited 2026-09-30 public baseline, which was
previously omitted.

Rollback: revert the record, the test, the LQP-V0-019..021 spec rows and the doc pointers. Nothing
else consumes the record yet.
