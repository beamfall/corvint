# 2026-10-20 V1-0812: whole-epoch AFP-V0-029 shadow bar; PR narrowing held on owner questions

Ticket: V1-0812 (narrow PR race tests behind the merge queue). Spec: `AFP-V0-028` to `AFP-V0-030`
and `AFP-V0-047` in `docs/specs/affected-plan-v0.md`. The intent is decision 0431; the amendments
are decisions 0509 and 0537. The owner consented on 2026-10-14, conditional on the 20/5 bar.

## Decision

The owner amended the `AFP-V0-029` bar in owner chat on 2026-10-20, under V1-0812. It now counts
every complete row of the current epoch rather than the latest-20 window:

- at least 20 complete rows;
- at least 5 rows with failing packages (`f1`);
- no `MISS` other than `FLAKY` anywhere in the epoch.

Rows are deduplicated per (pull request, commit). `INFRA` rows stay outside the counts, and an
`INCOMPLETE` row is not complete. The `AFP-V0-030` kill switch is unchanged: 2 `MISS` or
3 `INCOMPLETE` rows in the latest 20 trip it, and `CORVINT_PR_NARROWING=off` still forces FULL.

`.github/forward-rows/bar.jq` now reports two separate counts, one for the epoch bar and one for
the kill-switch window.

The claim "no MISS in the epoch" is only evidenced when the listing reached the start of the
epoch. That holds when `bar.sh` read a short (final) page, passed as `exhausted=1`, or when the
listing names a row of an earlier epoch. Otherwise the bar reads `NOT ESTABLISHED`, never `MET`,
which is the fail-closed reading of invariant 2. `NOT MET` still wins whenever the counts fall
short or a `MISS` is listed.

## Evidence

- `TestAFPV0047BarCountsArtifactNames` covers:
  - both counts;
  - deduplication and `INFRA`;
  - a tripped switch;
  - an older `MISS` outside the window, which leaves the switch clear but the bar NOT MET;
  - failing rows outside the window;
  - `NOT ESTABLISHED` on a truncated listing;
  - coverage through an earlier-epoch row.
- Live 20-page smoke run of `bar.sh` on 2026-10-20: `NOT MET (62/20 complete, 4/5 failing)`, and
  the listing did not reach the start of epoch 1. Epoch 1 spans more than 60 of about
  176 listing pages.
- The coordinator's reference count for the whole epoch: 119 complete, 19 failing, 0 MISS,
  0 FLAKY and 0 INCOMPLETE.

## Not built: owner questions

The deciding PR path (`AFP-V0-028` queue-backstopped mode in `ci.yml`), the `AFP-V0-029/030`
runtime gate job and the driver changes are `NOT_RUN`. Three forks need the owner:

1. **Whole-epoch evidence.** Paging the whole epoch on every PR exceeds the bounded listing
   (`BAR_PAGES=20`) and the `GITHUB_TOKEN` rate budget. One possible design: the trusted
   `forward-rows` job uploads marker artifacts such as `ci-forward-bar-MET-e<N>` and
   `ci-forward-MISS-e<N>`, and the PR side queries them by exact name. That design has two limits:
   - artifacts expire after about 90 days;
   - re-running an old-epoch job can mint a row newer than rows of the current epoch.
2. **Selector identity.** `AFP-V0-028` narrows with the driver and `gate-affected-select`, but
   epoch-1 rows bind S to the `corvint test` shadow selection. After the switch, a miss would be
   computed against the wrong S, so the kill switch could fail open. Advancing the epoch
   (`STR-V0-013`) discards the 119 rows. A guard that keeps it fail-closed: narrow only when the
   driver's selection is a superset of the shadow selection for the shard.
3. **Forward rows of a narrowed PR run.** A narrowed run's outcomes cover only the selected
   packages. Its PR row would therefore be either `INCOMPLETE`, which trips the switch after
   three PRs, or an inflated `CLEAN`. `collect.sh` and `row.jq` need a rule for narrowed runs.

Whether `vars.CORVINT_PR_NARROWING` is visible to fork PRs is `NOT_OBSERVED`.

## Rollback

- R1: set `CORVINT_PR_NARROWING=off`, or clear `CORVINT_PR_TOOL_SOURCE`. Every PR run is then FULL.
- R2: after R1, remove the `merge_queue` rule.
- R3: revert `AFP-V0-028` to `AFP-V0-030`.
- This change: revert `bar.jq`, `bar.sh`, the test and the spec text. The bar is advisory, so
  nothing narrows on it today.
