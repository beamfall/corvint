# Playwright Suite-Interaction Minimizer V0

Owner: Russell Lewis
Date: 2026-09-20
Intent status: proposed
Delivery status: experimental (closed operator executor; bounded observed-descendant cleanup)
Authoritative inputs: GitHub issue #47; AGENTS.md invariants 1-4 and 8; Playwright External
Provider V0; Documentation Corpus V1 stability identity.

## Agent digest
- Claim: A no-mutation planner and separately authorized experimental executor retain bounded Playwright trials with qualified runner, stability and application evidence.
- Status: proposed; experimental (closed operator executor; bounded observed-descendant cleanup).
- Exists: `internal/playwrightminimize`, `cmd/corvint-playwright-minimize`, integrated evidence validation and synthetic/live qualification fixtures.
- Blocked on: broader runtime qualification and production promotion; the admitted live tuple remains the exact system-Chrome profile.
- Read next: Requirements; Trust, limits, and failure modes; Acceptance and traceability.

## User and current state

The affected user is an operator investigating a Playwright test that passes alone but fails or
stalls in a wider suite. The measurable job is to retain the smallest observed reproducing ordered
predecessor sequence and unordered load set within explicit trial and wall-clock bounds, without
turning an isolated pass into a causal claim.

At base `6098291c9ed84c0de5c1a76afa3599d6a6faa352`, `internal/doccorpus/stability.go` retains exact
repeated-run identity and outcomes but does not plan suite-context minimization. The #39 and #43
work was initially on separate development refs. The integrated repair now decodes actual qualified
`corvint-playwright-external/1` receipt bytes and rederives the documentation corpus through
`doccorpus.Open`; caller booleans or digest-shaped strings do not qualify the operator-facing profile.

## Requirements

- `PSM-V0-001`: A request MUST bind immutable original-failure and isolated-pass receipts plus exact
  test and application revisions, configuration digest, runner and browser names and versions,
  project, ordered test identities, worker topology, fixture schema and digest, seed identity, and
  application-instance attestation digest. Missing, contradictory, or duplicate identity MUST refuse.
- `PSM-V0-002`: Planning MUST be no-mutation and deterministic, and MUST emit the exact ordered list
  of proposed trials, their reset policy, topology, identity, repetition, and a digest of the plan
  before any runner can be called.
- `PSM-V0-003`: Trial execution through the operator-facing boundary MUST require a separate explicit
  operator approval bound to the exact plan digest. Planning alone MUST grant no execution authority.
- `PSM-V0-004`: The planned original schedule MUST run first. A valid current pass produces
  `not_reproduced`, executes no isolation or minimization trial, and publishes no current failure
  classification. An invalid or mixed reproduction remains incomplete or nondeterministic.
  A failed reproduction MUST match the original target's exact failure-class set; otherwise it
  MUST remain incomplete with `original-failure-signature-mismatch`, not count as reproduction.
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
  The live boundary MUST rederive the original target observations from the qualified immutable
  native receipt, reject a caller-declared class set that differs, and bind the rederived
  observations into the plan. Semantic comparison uses sorted distinct classes, not per-run
  evidence digests, summaries, or multiplicity: those observations remain retained, but attempt
  timing and evidence hashes naturally vary between runs. Original receipt bytes and their exact
  digest remain immutable. Class equality is not a claim of identical underlying root cause.
- `PSM-V0-009`: A finding MUST report the smallest observed reproducing sequence or set, all
  contributing receipt digests, and each member as necessary in the observed universe or unproven.
  It MAY report proof within the fully executed bounded universe; it MUST never call that globally
  minimal.
  A failed candidate MUST match that same original class set before contributing to a finding or
  minimality proof. Mismatches invalidate the candidate, retain every observed failure, and block
  confidence. Isolation failures remain separately classified and stop minimization.
- `PSM-V0-010`: Every trial MUST retain start and publish application attestations. A changed
  application instance is a retained restart observation and invalidates that trial for comparison.
- `PSM-V0-011`: Missing or invalid runner qualification (#39), stability identity (#42), or
  application attestation (#43) MUST appear as distinct blockers and MUST prevent a confident
  interaction diagnosis even when a synthetic witness is observed.
- `PSM-V0-012`: Synthetic qualification MUST cover a true predecessor leak, a load-only failure,
  application restart, cleanup failure, nondeterminism, and isolated product regression, plus
  authorization, `not_reproduced`, and incomplete-bound controls.
- `PSM-V0-013`: The closed `corvint-playwright-suite-interaction-live/0` CLI MUST require
  `--experimental`; `plan` MUST be read-only and `execute --approve-plan DIGEST` MUST rederive the
  exact plan, command identities and composed evidence before mutation. The executor MUST construct
  Playwright selectors itself, validate reporter-observed test starts and resolved topology, run
  pinned operator reset/cleanup commands, retain process observations and invalidate every observed
  survivor or observer failure. Bounded observed-descendant cleanup MUST NOT claim full OS
  containment, and `RequireDescendantCleanup`'s existing full-containment refusal MUST remain intact.

## Non-goals and simpler baseline

The simpler baseline is manual isolation and prefix reruns, retained as the fallback. V0 does not
identify the true root cause, repair a test, choose a reset policy, manage application servers,
schedule distributed workers, or promote a stability result. Operator-owned reset and cleanup code
may mutate only its explicitly authorized environment. The pure `Runner` remains a synthetic seam;
the separate live profile calls the qualified provider itself rather than trusting returned booleans.

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

## Closed operator profile

Build `./cmd/corvint-playwright-minimize` separately. Feed a `LiveRequest` on stdin to
`plan --experimental`; feed its exact `LivePlan` output to
`execute --experimental --approve-plan sha256:...`. Planning never launches the reset, cleanup,
provider or browser. `original`, `isolated` and `corpus` are base64-encoded exact bytes, so round-trip
JSON encoding cannot change their hashes. The corpus's selected `stability_ids` must bind both
baseline receipt hashes and their exact target test identities. A failed baseline does not need a
clean stability verdict; it needs valid, rederived identity and retained denominators.

The admitted runtime is Playwright 1.63.0's qualified Darwin/arm64 system-Chrome tuple. Trials use
unsharded project-default parallelism, zero retries and repeat-each 1. Ordered trials compare actual
reporter `onTestBegin` order to the requested source/full-name/project identities; a schedule that
Playwright does not realize remains invalid/incomplete. Load trials compare exact membership and
topology without requiring start order. No private scheduler mutation or global minimality claim is
made. The fully qualified receipt, every attempt and both application attestations are retained in
each trial; source locators join baseline identities to trial-specific IDs that also bind argv.

The operator declares `playwright-use`, a fresh browser/process reset without application restart,
and `CORVINT_MINIMIZER_SEED` in the bound environment. Reset/cleanup each name one absolute,
digest-pinned executable; exact trial/seed/reset-generation data is supplied on stdin. Exit zero
with verified cleanup proves the command ran, not independently that arbitrary application state
was restored. The operator-owned reset policy remains the authority for those semantics.

Reset/cleanup commands have 5-second bounds and 64 KiB output caps. Cleanup runs on a fresh bounded
context even after reset failure or cancellation. The runner retains operating-system accounting
when available. The opt-in observer samples PID/parent/start identity every 20 ms, retains the scope
and interval, kills observed escaped descendants, then verifies absence. Observer errors and
observed survivors invalidate the trial. Fast detach/reparent between snapshots can remain
unobserved; start identity resolution is platform-dependent. This is not adversarial OS containment
or authority over the externally owned application server. The stronger `RequireDescendantCleanup`
request continues to refuse before launch under CRR-V0-003(c).

## Acceptance and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| PSM-V0-001/002/005/006 | `internal/playwrightminimize/planner.go`, `types.go` | `TestPSMV0001PlanIsExactBoundedAndReadOnly`, `TestPSMV0011WallClockAndTrialBoundsStayVisible`, `TestPSMV0012IncompleteReproductionCannotBecomeNotReproduced`, `TestPSMV0015DuplicateBaselineDigestRefuses` |
| PSM-V0-003 | `internal/playwrightminimize/executor.go` | `TestPSMV0002ExecutionRequiresSeparateAuthorization` |
| PSM-V0-004 | `internal/playwrightminimize/executor.go` | `TestPSMV0007NondeterministicReproductionStopsMinimization`, `TestPSMV0009NotReproducedHasNoMinimizationOrClassification`, `TestPSMV0018InvalidRepetitionOutranksMixedClasses` |
| PSM-V0-004/008/009 | `internal/playwrightminimize/executor.go`, `internal/playwrightminimize/live.go` | `TestPSMV0019FailureSignatureMismatchCannotMinimize`, `TestPSMV0020FailureSignatureUsesExactClassSetNotEvidenceDigest`, `TestPSMLiveOriginalFailureSignatureIsRederived` |
| PSM-V0-007/008/010 | `internal/playwrightminimize/executor.go` | `TestPSMV0005ApplicationRestartInvalidatesTrial`, `TestPSMV0006CleanupFailureInvalidatesTrial`, `TestPSMV0013RunnerGetsDeadlineAndErrorReceiptIsRetained`, `TestPSMV0016InfrastructureIsolationIsNotProductAttribution`, `TestPSMV0017CancellationDuringFinalTrialCannotPublishConfidence` |
| PSM-V0-009/012 | `internal/playwrightminimize/executor.go` | `TestPSMV0003TruePredecessorLeakQualification`, `TestPSMV0004LoadOnlyQualification`, `TestPSMV0008IsolatedProductRegressionStopsMinimization`, `TestPSMV0014NecessityRequiresMatchingTopologyAndSupportingReceipts` |
| PSM-V0-011 | `internal/playwrightminimize/planner.go`, `executor.go` | `TestPSMV0010MissingComposedQualificationsBlocksConfidence` |
| PSM-V0-013 | `internal/playwrightminimize/live.go`, `cmd/corvint-playwright-minimize`, `internal/procgroup/descendants.go` | `TestPSMLiveAuthorizationPrecedesAllMutation`, `TestMinimizerCLIRefusesImplicitExecution`, `TestPSMLiveQualifiedEvidenceAndNegativeControls`, `TestPSMLiveResetFailureStillCleansUp`, `TestPSMLiveCancellationReapsDescendants`, `TestObservedDescendantCancellationReapsEscapedChild`, `TestObservedDescendantIdentityReuseDoesNotExpandOwnership`, `TestPSMLiveDockerPredecessorQualification` |

Focused acceptance is `GOTOOLCHAIN=local go test -count=1 ./internal/playwrightminimize` plus the
repository requirement, definition, and traceability checks. The opt-in real predecessor/isolated
qualification requires `CORVINT_MINIMIZER_LIVE_QUALIFICATION=1` and `CORVINT_PLAYWRIGHT_MODULES`.
The six diagnostic classifications remain separately identified synthetic fixtures; one live
predecessor test is not proof of all six live environments or production qualification. The full
canonical gate belongs to the frozen integrated candidate.

## Rollout, rollback, compatibility, and promotion

The package and separately built CLI remain experimental with no persisted-state compatibility
promise. They are not added to the default core binary or implicitly to the qualified companion
bundle. Rollback removes the opt-in surface and observed-descendant mode without altering the
default procgroup contract. Promotion requires broader frozen live qualification without weakening
invalidation. Any causal overclaim, execution without digest-bound approval, or accepted invalid
reset/cleanup kills promotion and retains experimental status.

## Unresolved decisions

- The frozen external shape that joins #39 runner qualification and #43 application attestation is
  unavailable at this base; V0 uses explicit digests and makes no compatibility claim.
- A later slice must choose whether real adaptive delta-debugging can preserve a complete proposed
  trial schedule before execution or needs a sequence of separately authorized plan extensions.
