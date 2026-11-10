<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-5216: an all-FULL no-plan run is a FULL forward row, not CLEAN

Builder decision while merging `claude/layers-integ` into `main` (branch
`claude/batch-handoff-1110`, base `origin/main` 867ff4991). Supersedes the CLEAN/FLAKY outcome
recorded in `2026-11-10-forward-fallback-rows.md`; the rest of that entry stands.

## Collision

Two changes answered V1-5216 differently.

- `main`, commit d31b3470b (`.github/forward-rows/row.jq`), with the AFP-V0-047 amendment and
  `tools/ci-forward-row/fallback_test.go` from 9eefe0df5: a run whose every selection shard is
  the no-plan FULL fallback (scope `FULL`, a `no plan: ` reason, an empty plan digest) is a
  complete row whose S is every package with an outcome, so it is CLEAN or FLAKY.
- The layers branch, CIN-V0-7921 (`docs/specs/ci-narrowing-v0.md`, c7550d650, and its
  `row.jq`, `collect.sh` and `bar.jq` changes): a run whose every shadow shard ran `FULL` is the
  class `FULL`, typed `skew` or `abstain`, with S the selected packages only; a `FULL` row takes
  no place in the AFP-V0-029/030 window.

## Choice

CIN-V0-7921 wins. A run that measured no narrowing must not count as evidence for narrowing. Under
d31b3470b each fallback run would add a CLEAN row to the latest-20 window, so a planner that
fails on every pull request would look like a planner that never misses.

## Changes

- `.github/forward-rows/row.jq` keeps the layers version; it reaches `main` in the `.github`
  follow-up pull request (`claude/layers-integ-github`).
- `tools/ci-forward-row/fallback_test.go`: the two AFP-V0-047 fallback tests now assert the
  CIN-V0-7921 result (class `FULL`, type `skew` for the fallback, `abstain` for another FULL
  reason, `skew` for a missing or malformed plan digest; a mix of fallback and planned shards is
  INCOMPLETE). Their names carry the `_CINV07921` suffix, and they skip until the follow-up lands.
  The rerun dedupe test is unchanged.
- AFP-V0-047 in `docs/specs/affected-plan-v0.md` defers to CIN-V0-7921 for the all-fallback row.

Rollback: revert the commit that adds this entry; `main`'s V1-5216 spec text and tests (9eefe0df5) return.
