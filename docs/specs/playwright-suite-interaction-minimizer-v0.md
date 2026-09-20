# Playwright Suite-Interaction Minimizer V0

Owner: Russell Lewis
Date: 2026-09-20
Intent status: proposed
Delivery status: experimental (planner and synthetic qualification only)
Authoritative inputs: GitHub issue #47; AGENTS.md invariants 1-4 and 8; Playwright External
Provider V0; Documentation Corpus V1 stability identity.

## Agent digest
- Claim: A pure bounded planner retains exact proposed Playwright suite-interaction trials and synthetic outcomes without launching Playwright or an application.
- Status: proposed; experimental (planner and synthetic qualification only).
- Exists: `internal/playwrightminimize` and its synthetic qualification tests.
- Blocked on: accepted owner intent, exact integrated #39/#42/#43 contracts, an operator-facing contained executor, and live Playwright/application qualification.
- Read next: Requirements; Trust, limits, and failure modes; Acceptance and traceability.

## User and current state

The affected user is an operator investigating a Playwright test that passes alone but fails or
stalls in a wider suite. The measurable job is to retain the smallest observed reproducing ordered
predecessor sequence and unordered load set within explicit trial and wall-clock bounds, without
turning an isolated pass into a causal claim.

At base `6098291c9ed84c0de5c1a76afa3599d6a6faa352`, `internal/doccorpus/stability.go` retains exact
repeated-run identity and outcomes but does not plan suite-context minimization. The #39 and #43
work exists only on separate development refs in this checkout and is not an integrated frozen
contract here. This slice therefore accepts explicit dependency receipts and reports their absence;
it does not copy or claim final integration with those branches.

## Requirements

- `PSM-V0-001`: A request MUST bind immutable original-failure and isolated-pass receipts plus exact
  test and application revisions, configuration digest, runner and browser names and versions,
  project, ordered test identities, worker topology, fixture schema and digest, seed identity, and
  application-instance attestation digest. Missing, contradictory, or duplicate identity MUST refuse.
- `PSM-V0-002`: Planning MUST be no-mutation and deterministic, and MUST emit the exact ordered list
  of proposed trials, their reset policy, topology, identity, repetition, and a digest of the plan
  before any runner can be called.
- `PSM-V0-003`: Trial execution through the abstract boundary MUST require a separate explicit
  operator approval bound to the exact plan digest. Planning alone MUST grant no execution authority.
- `PSM-V0-004`: The planned original schedule MUST run first. A valid current pass produces
  `not_reproduced`, executes no isolation or minimization trial, and publishes no current failure
  classification. An invalid or mixed reproduction remains incomplete or nondeterministic.
- `PSM-V0-005`: Ordered predecessor sequences and unordered load sets MUST have separate candidate
  universes, trials, completeness flags, and findings. An isolated pass plus the original failure
  MUST NOT by itself establish order dependence.
- `PSM-V0-006`: The plan MUST enforce explicit maximum-trial, repetition, and wall-clock bounds.
  Truncation, timeout, invalid evidence, or an unexecuted candidate MUST remain visible as incomplete.
- `PSM-V0-007`: Every trial MUST declare one exact reset policy and retain digest-bound setup,
  page/assertion, retry, cleanup, server-health, resource, and failure evidence. Failed reset or
  cleanup, missing evidence, or changed run identity invalidates the trial without erasing it.
- `PSM-V0-008`: Assertion, synchronization, fixture, product, infrastructure, restart, and
  resource-exhaustion observations MUST remain separate and all observed values MUST be retained.
  Multiple or conflicting observations MUST NOT be automatically adjudicated into one cause.
- `PSM-V0-009`: A finding MUST report the smallest observed reproducing sequence or set, all
  contributing receipt digests, and each member as necessary in the observed universe or unproven.
  It MAY report proof within the fully executed bounded universe; it MUST never call that globally
  minimal.
- `PSM-V0-010`: Every trial MUST retain start and publish application attestations. A changed
  application instance is a retained restart observation and invalidates that trial for comparison.
- `PSM-V0-011`: Missing or invalid runner qualification (#39), stability identity (#42), or
  application attestation (#43) MUST appear as distinct blockers and MUST prevent a confident
  interaction diagnosis even when a synthetic witness is observed.
- `PSM-V0-012`: Synthetic qualification MUST cover a true predecessor leak, a load-only failure,
  application restart, cleanup failure, nondeterminism, and isolated product regression, plus
  authorization, `not_reproduced`, and incomplete-bound controls.
- `PSM-V0-013`: This slice MUST NOT launch Playwright, start or mutate an application, expose a CLI
  execution verb, or claim final #39/#43 integration. A later executable slice requires the exact
  frozen dependency contracts, its own operator-facing authorization, containment, and live gate.

## Non-goals and simpler baseline

The simpler baseline is manual isolation and prefix reruns, retained as the fallback. V0 does not
identify the true root cause, repair a test, mutate fixtures, choose a reset policy, manage servers,
schedule distributed workers, or promote a stability result. It has no network, browser, filesystem,
or process implementation. A caller-supplied `Runner` in tests is a synthetic qualification seam,
not a production executor.

## Trust, limits, and failure modes

Plans are derived data and carry no repository or operator authority. The planner admits at most
eight predecessor and eight load members; every plan carries a positive trial count, wall-clock
budget, and repetition count. Candidate order is deterministic. Bounded truncation produces explicit
per-universe incompleteness rather than silently dropping it.

Receipt validation fails closed on mismatched trial/run/reset identity, duplicate or malformed
receipt digests, failed reset or cleanup, missing evidence axes, impossible pass/failure combinations,
or changed application attestation. The invalid receipt and its failure observations remain in the
report. A valid pass/fail mixture for one candidate is nondeterminism, not evidence for whichever
result is more convenient. When both ordered and load witnesses exist the diagnosis stays ambiguous.

## Acceptance and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| PSM-V0-001/002/005/006 | `internal/playwrightminimize/planner.go`, `types.go` | `TestPSMV0001PlanIsExactBoundedAndReadOnly`, `TestPSMV0011WallClockAndTrialBoundsStayVisible`, `TestPSMV0012IncompleteReproductionCannotBecomeNotReproduced`, `TestPSMV0015DuplicateBaselineDigestRefuses` |
| PSM-V0-003 | `internal/playwrightminimize/executor.go` | `TestPSMV0002ExecutionRequiresSeparateAuthorization` |
| PSM-V0-004 | `internal/playwrightminimize/executor.go` | `TestPSMV0007NondeterministicReproductionStopsMinimization`, `TestPSMV0009NotReproducedHasNoMinimizationOrClassification`, `TestPSMV0018InvalidRepetitionOutranksMixedClasses` |
| PSM-V0-007/008/010 | `internal/playwrightminimize/executor.go` | `TestPSMV0005ApplicationRestartInvalidatesTrial`, `TestPSMV0006CleanupFailureInvalidatesTrial`, `TestPSMV0013RunnerGetsDeadlineAndErrorReceiptIsRetained`, `TestPSMV0016InfrastructureIsolationIsNotProductAttribution`, `TestPSMV0017CancellationDuringFinalTrialCannotPublishConfidence` |
| PSM-V0-009/012 | `internal/playwrightminimize/executor.go` | `TestPSMV0003TruePredecessorLeakQualification`, `TestPSMV0004LoadOnlyQualification`, `TestPSMV0008IsolatedProductRegressionStopsMinimization`, `TestPSMV0014NecessityRequiresMatchingTopologyAndSupportingReceipts` |
| PSM-V0-011 | `internal/playwrightminimize/planner.go`, `executor.go` | `TestPSMV0010MissingComposedQualificationsBlocksConfidence` |
| PSM-V0-013 | package import/process boundary | `go list -deps ./internal/playwrightminimize` inspection; live Playwright/application execution `NOT_RUN` by design |

Focused acceptance is `GOTOOLCHAIN=local go test -count=1 ./internal/playwrightminimize` plus the
repository requirement, definition, and traceability checks. Live runner/browser/application trials,
the full canonical gate, and #39/#43 integration are `NOT_RUN` in this slice.

## Rollout, rollback, compatibility, and promotion

The package is internal and has no caller, CLI, wire, or persisted-state compatibility promise.
Rollback removes the package, this spec/index row, and build-log entry. Promotion requires accepted
intent, exact integrated #39/#42/#43 profiles, a separately authorized contained executor, frozen
real Playwright fixtures, and a live gate demonstrating the six cases without weakening invalidation.
Any overclaim of causality, any execution without digest-bound approval, or any accepted invalid
reset/cleanup kills the executable direction and retains planning-only status.

## Unresolved decisions

- The frozen external shape that joins #39 runner qualification and #43 application attestation is
  unavailable at this base; V0 uses explicit digests and makes no compatibility claim.
- A later slice must choose whether real adaptive delta-debugging can preserve a complete proposed
  trial schedule before execution or needs a sequence of separately authorized plan extensions.
