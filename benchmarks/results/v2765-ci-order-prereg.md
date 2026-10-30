# V1-2765 CI package order replay: preregistration

Status: preregistered 2026-10-27. Ticket V1-2765. This file is committed before any order is
computed for any run. The sample below was drawn using only run keys, PR numbers and
eligibility; no timing or order was looked at. Departures are reported as deviations in the
result. Results are reported unrepaired.

## Question

Corvint CI orders each shard's packages affected-first (`.github/cishards/order.go`, AFP-V0-022):
it stable-sorts by witness rank, then runs the packages with `go test -json -p 1`. Does this
order reach the first confirmed failure sooner than two cheaper orders? And does any of the three
beat plain name order?

## Data (retained locally, no new CI runs)

- **Runs.** The 2026-10-26 shadow-traffic study of Corvint CI covers PR and merge-queue runs from
  2026-10-14 to 2026-10-26. For each run it retains, per shard:
  - the package outcome document, which is the shard's run set;
  - the selection document;
  - a hop graph with these fields:
    - changed packages;
    - each package's reverse-import distance from the change;
    - read-scope packages;
    - unbounded-reader packages.
- **Costs.** Package durations come from `.github/cishards/package-costs.json`. That file is the
  profile CI uses to partition shards, taken from one CI run at revision `16fb20b2ba6f`. No per-run
  package elapsed times, step durations or affected-order plans were retained. The replay is
  therefore a static-cost simulation, not a timing measurement.

## Sample (frozen)

- **Eligible runs.** A failing row with at least one non-flaky failing package and a hop graph.
  That gives 30 rows over 22 PRs.
- **One run per PR.** Pick it with `rng.choice(sorted(keys))`, where `rng = random.Random(20261027)`
  and PRs are taken in numeric order.
- **Draw.** `rng.sample(sorted(chosen), 20)`. Runs are keyed `<pr>-<sha>`:

```
1059-3f65c634de9497074dc90cf0be87849a05c56af9   939-0feab94c81f93c360ec2e5e6475a897a02858ccb
1076-b32bb2b613df70cd39ba853a440d3259cd3d0b52   910-10b0a18f9569170c3406ee51e7a555fb98519ca7
1054-ecfcf902aef405c16281db397f3c99ee9fbf6873   1062-b2ef021e0ddeabb6baa1cf24d7cd868077eb337c
878-5c7b49aeceacf470a6c0027746af80e0fe644402    929-9a2ed80d5efc2a12f9bdc25afa7a60657d899689
1058-6b5b68922663c45f138a2837b4346353c02137ca   1016-5ca8df1309d5be1a51def34d717fc1b5b823fd95
1015-71248d512e47e5ce60fb7053d3e394a21654260a   902-b65bb671529ab06ca03b35679029d7534364ce35
1049-35a9d16697e9c6c10a1dbdb78515cd1d57355007   960-8d517204f4645fbf30d41f6efbf73c45b354e8ad
955-24e9425176ed6509aa2e6fe784e2d1c354abd8c7    1061-c45f2ecc57c728cda109c4e3270ce08c19821360
961-574239894b61ae84ad52656b2eaae09f6021e613    998-e3b1923cbe72d030f6c23284c9717d8eb86aee79
968-fb5eb1c7c2638937b7c139111f62c8fe26a1a332    1074-6143661bc7d75afc78ee921dad191bfa15efcba9
```

## Arms

Every arm permutes the same package set: each shard's whole packages, taken from its outcome
document. Split-package slices run after the whole packages, in their recorded order, in every
arm, matching CI's `run_tests`.

- **A0, name order (reference).** Sorted package names, the partitioner's order without
  `--order`.
- **A1, affected-first.** A stable sort of A0 by rank, as `order.go` defines it:
  - rank 0: a selected package within the dependency closure (finite distance);
  - rank 2: a selected package that is only an unbounded reader;
  - rank 1: any other selected package;
  - rank 3: an unselected package.

  The selected set is the run's retained shadow selection. Deviation: witness kinds are rebuilt
  from the hop graph, because the run's own plan was not retained.
- **A2, changed-packages-first.** The run's changed packages in name order, then the rest in name
  order.
- **A3, changed packages by duration.** The changed packages, shortest historical duration first,
  then the rest, also shortest first. Ties go by name. The ticket's failure-rate variant is
  NOT_RUN: the 13-day window has no failure history that predates the sampled runs.

## Measures

- **Shard TTFF.** Time to first failure is the cumulative cost, in the shard's arm order, up to
  and including the first package with a confirmed non-flaky failure in that shard. A sliced
  package contributes its cost divided by its slice count.
- **Run TTFF.** The minimum over the shards that hold such a failure; shards run in parallel.
- **Assumptions.** These apply to every arm alike:
  - `-p 1` runs packages in argument order;
  - build time is part of the profiled cost, or does not depend on order.
- **Green latency and CPU.** A permutation leaves both unchanged in the simulation, so they are
  equal by construction. Live values are NOT_OBSERVED.
- **Ordering overhead.** Its live cost, the plan artifact and the order call, is NOT_OBSERVED.
  The simulation counts it as zero.
- **Contrasts.**
  - Per-run ratios TTFF(X) / TTFF(A1) for X in {A0, A2, A3}, and TTFF(A1) / TTFF(A0).
  - Pooled: the median per-run ratio, the ratio of summed TTFF, and the share of runs where X is
    worse than A1.
  - Uncertainty: a 95% percentile bootstrap over runs (10,000 resamples, seed 20261027). There is
    one run per PR, so the run is the cluster.

## Decision rule (frozen)

**Practical threshold (from the ticket).** An order wins when all three hold:

- its median per-run TTFF ratio against the other order is ≤ 0.80;
- it is worse on at most 25% of runs;
- the ratio of summed TTFF points the same way.

**Verdicts:**

- **SIM_AFFECTED_FIRST_BEATS_NAIVE.** A1 wins against both A2 and A3.
- **SIM_NAIVE_BEATS_AFFECTED_FIRST.** A2 or A3 wins against A1. File a follow-up ticket proposing
  a switch, conditional on live confirmation.
- **SIM_NO_PRACTICAL_DIFFERENCE.** Neither direction wins. The cheaper order is preferable only
  after live overhead is measured.
- **Secondary.** Whether A1 wins against A0, which shows whether ordering pays at all.

**Limits on any verdict.**

- A simulation verdict cannot change CI by itself. Any switch needs a live paired confirmation:
  the same runs, recorded per-package elapsed times, and green latency and CPU observed.
- Twenty runs bound only large effects.

## Limits

- **Costs.** They come from one profiling run, not the sampled runs' own timings. Queue time,
  runner variance, `-race` effects that differ from the profile, and setup are not modelled.
- **Failures.** Confirmed failures come from the CI rerun rule (a package fails, and its rerun
  fails or is not run). Flaky-only packages are excluded.
- **Shard layout.** Each run keeps its recorded shard layout. Arms do not move packages between
  shards.
