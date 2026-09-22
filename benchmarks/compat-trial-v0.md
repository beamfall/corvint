# Compatibility trial v0 preregistration

Status: `PREREGISTERED_NOT_RUN`

Authority: CTR-V0-001..010, accepted by decision 0052. The case labels, criticality rubric,
budgets, baseline contract, and scorer details below are coordinator calls delegated by that
decision. They are frozen policy, not empirical findings.

## Frozen artefacts

- Label and exclusion registry: `benchmarks/compat-trial-v0.json`.
- Comparator contract and pin state: `benchmarks/compat-baseline-v0.md`.
- Comparison policy: `docs/specs/compat-trial-v0.md`, CTR-V0-002.
- Resource policy: `docs/specs/compat-trial-v0.md`, CTR-V0-003/009.
- Runnable CTR-V0-001 descriptor: `NOT_PRODUCED`.
- Scored observations: `NOT_RUN`.

The JSON file is a label/preregistration registry, not an executable CTR-V0-001 descriptor. It does
not invent snapshots, argv, stdin, executable builds, or their digests. A scored run is unavailable
until a complete descriptor and the five baseline pins exist.

## Pilot selection and partition

Eligibility is decided before any result. A candidate must change an implementation or dependency
on a read-only CLI verb's reachable path, be exercisable by one invocation with frozen argv/stdin,
and represent one coherent intent. A positive case additionally predicts different old/new bytes or
exit status; accepted intent decides whether that difference is a defect or intentional. A compatible
control predicts exact old/new equality despite the implementation change. Ten candidates qualify;
ten are excluded with reasons in the JSON registry.

All ten eligible cases are development pilot cases. None is held out. The pilot is 6 defect, 1
intentional, and 3 compatible, so it does not satisfy the planned 20-case 10/5/5 development mix.
It exists only to debug the protocol. CTR-V0-006's 40 new cases from at least four unrelated
projects are the separate screen and holdout population.

The coordinator's pilot labels remain frozen developmental calls, but case-specific independent
accepted-intent references are `NOT_PRODUCED`. Candidate-side fixes and tests are discovery evidence,
not independent intent authority. No pilot case may enter a scored run until its independent intent
predicate is pinned from a pre-run owner/spec/test-oracle artefact.

## Per-case budgets

| Resource | Frozen value |
|---|---:|
| repetitions per version | 3 |
| input | 65,536 bytes |
| output per stream | 1,048,576 bytes |
| fixture entries | 100 |
| aggregate fixture bytes | 10,485,760 bytes |
| process timeout | 10 seconds |
| request timeout | 120 seconds per case |
| build timeout | 300 seconds per version |
| attempts | 1 per frozen case descriptor |
| human active time | 45 minutes per positive attempt per arm |
| allowed effects for the pilot | none |

Build time and unsuccessful attempts remain in the active-cost numerator. Re-executing the same
descriptor is a protocol amendment, not another attempt.

## Scorer

`C_arm = sum(active_minutes for all positive attempts) / count(independently solved positives)`.
Time is accumulated in whole seconds. Ratios are evaluated exactly and rounded to three decimals
only for display.

A positive is solved only when the arm independently matches the accepted defect label and
reproduces the disagreement within the frozen budget. Unsupported cases and all six CTR-V0-008
inconclusive states are unsolved. Zero solved positives has infinite cost. Missing time, zero
baseline cost, or another undefined ratio is inconclusive and cannot pass.

A critical miss is an unsolved positive with `critical: true`. Corvint has an extra critical miss when
its integer count is greater than the baseline's. `C_Corvint / C_baseline <= 0.70` includes equality;
completion at least as high as the baseline also includes equality.

## Interval rule and claim scope

The ten-case pilot reports per-case states and counts only. It has no confidence interval and cannot
support superiority language.

The 40-case screen uses 10,000 paired cluster-bootstrap resamples clustered by change, seed
`20260904`, and an upper one-sided 95% bound on the cost ratio. Undefined draws remain `+inf`; if
more than 5% of draws are undefined, the bound is undefined. The screen remains screening-only:
there is no accepted sample-size justification for the five-percentage-point completion
non-inferiority claim in E:65.

## First-run conditions

The first scored run counts only when all of the following precede launch:

1. A complete CTR-V0-001 descriptor pins every case input, snapshot, executable, and digest.
2. Every included case has `accepted_by: coordinator:decision-0052`, defined developmental
   criticality, and a case-specific independent accepted-intent reference that is not
   `NOT_PRODUCED` and does not derive from the candidate-side fix.
3. All five comparator artefacts in `benchmarks/compat-baseline-v0.md` have real paths and sha256s,
   after an independent reviewer improves the harness on the pilot.
4. CRR-V0-003(d)'s shared process-group extraction has landed and the applicable containment gate
   permits the requested execution profile.
5. Arm order is randomized from a recorded seed and separate operators run the arms.
6. The frozen manifest is executed exactly once.

Until then, every case remains `NOT_RUN`.
