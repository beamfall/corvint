# CEM criterion experiments V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/CHANGE-EVIDENCE-MAP.md`, `docs/specs/cem-0.2-canonical-binding.md`,
`docs/specs/applied-intelligence-breakthroughs-v0.md`,
`docs/specs/corvint-tasks-agent-leases-v0.md`, native tickets V1-0573 and V1-0575.

## Agent digest
- Claim: An optional companion binds task criteria to Go experiments, preserves historical verification, and refuses stale Tasks gate evidence.
- Status: proposed/experimental; precursor research for a possible CEM 0.4, not a promoted CEM wire.
- Exists: `internal/criterionexperiment` and `cmd/corvint-cem-experiments`; see the traceability and evidence record below for actual qualification.
- Blocked on: qualified broader providers, owner acceptance of the technical profile, and the external three-arm outcome evaluation, which remains NOT_RUN.
- Read next: Requirements; Trust and execution boundary; Outcome experiment; `../CHANGE-EVIDENCE-MAP.md`.

## User and measurable job

A reviewer should see which experiments distinguish the submitted implementation from registered
alternatives that the owner considers incorrect for one task criterion. An agent should retain the
exact unresolved case through a handoff and know which verification action is useful next. A Tasks
command gate should refuse an old packet after its acceptance, execution inputs, generation or
candidate changes, while that packet remains verifiable as historical evidence.

The hypothesis is reusable experimental evidence through the task lifecycle. Mutation testing,
fail-before/pass-after tests, traceability, affected tests and assurance cases are established
methods; their presence is not novelty or proof of product value.

## Verified starting state

CEM verifies canonical patch and immutable evidence identity, not semantic support or correctness.
Its experimental 0.3 profile already carries coverage and mutation witnesses. OCM maps scoped
requirements to structural change/test links. The missing-test hypothesis catalog already proposes
fail-before/pass-after or registered-mutation qualification. Native external-agent Tasks can retain
observed COMMAND gates at exact submitted candidates. This slice composes those boundaries; it
does not redefine their claims, defaults, frozen packets or completion authority.

The source baseline is `9afd8313edad658d6c40b1abe454fe7417eb6cad` on public `origin/main`.
The primary working tree and its unrelated changes are outside this build. Newer uncommitted Tasks
review capabilities in the primary tree are not dependencies of this public-base adapter.

## Definitions

- **Criterion**: one indexed member of the canonical native ticket's ordered acceptance array,
  bound to that array and its acceptance revision; neither a generated summary nor inferred intent.
- **Oracle**: pinned Go test bytes and a reviewed expectation anchored to the accepted criterion.
  Pinning bytes does not establish that expectation's semantic authority.
- **Control**: a registered immutable alternative whose relationship to the criterion is explicitly
  reviewer-attributed. Registered controls are a finite declared denominator, not all possible bugs.
- **Execution snapshot**: the bounded module files from an immutable commit, with the same pinned
  oracle overlaid at the same path, identified by the reconstructed path/mode/content inventory.
- **Historical verification**: recomputation of packet bindings and recorded classifications with
  retained inputs through an independently pinned native Tasks read verifier; it launches no experiment or test command. Core consumes only Tasks wire contracts, never Tasks private CLI, intent, snapshot or ticket packages.
- **Live applicability**: whether that historically valid packet still matches the current audited
  native task/attempt/policy and the submitted clean candidate. It grants no integration authority.

## Requirements

- `CEX-V0-001`: The companion MUST remain explicitly experimental and opt-in, with separate
  plan/run/verify/gate operations and bounded closed profiles. Unknown, repeated, case-variant,
  malformed or trailing fields MUST refuse. Execution requires trusted-local admission and approval
  of the exact plan digest; ordinary verification never executes a plan-supplied command.
- `CEX-V0-002`: Planning MUST bind independently supplied full base/candidate identities, exact
  canonical `cem/0.2` verification and raw map/patch digests, actual hunk references, canonical native
  task identity, acceptance revision, ordered complete criterion inventory, policy/configuration
  digest and attempt generation. Live authority MUST come from coherent audited native reads,
  not an unaudited ticket file or a caller's invented scope. Missing, surplus or duplicate criteria
  and unresolved selectors refuse. Hunk-to-criterion meaning remains reviewer-attributed.
- `CEX-V0-003`: Each supported scenario MUST reconstruct a committed Go module in one literal
  subdirectory through immutable Git authority, reject symlinks/gitlinks/special entries and unsupported
  module dependencies, and overlay the same exact oracle bytes. Commit/tree, oracle selector and
  reconstructed execution-inventory digest MUST remain distinct. The initial profile admits only
  standard-library dependencies; it refuses nested modules, replacements, workspaces and package patterns.
- `CEX-V0-004`: The runner MUST use only fixed, fully anchored top-level Go test selectors, a pinned
  absolute Go executable, a closed local/offline environment, bounded time/output, and newly created
  disposable workspaces. Owned ordinary descendants MUST retire on normal exit, timeout, overflow
  and interruption. It MUST retain every requested scenario and partial/error result without silently
  replacing attempts, modifying authoritative repository/Tasks records, or writing outside the
  caller-selected fresh artifact directory. Hostile detached execution is outside qualification.
- `CEX-V0-005`: A passing observation MUST include the exact package/test run and successful terminal
  events with no skips or other failures. An expected failure MUST be a normal assertion failure of
  that named test, with its expected marker in that test's output and complete package failure.
  The initial Go 1.27 profile requires exactly one `OutputType:error` source diagnostic whose
  parsed message equals the assertion marker or starts with that marker followed by `: `.
  Marker-bearing filenames, larger words and contradictory passing error output refuse.
  Build failures, panic, timeout, malformed/truncated/duplicate events, wrong assertions and unrelated
  failures MUST NOT count as a killed control. A passing eligible control survives and remains unresolved.
- `CEX-V0-006`: Relations MUST distinguish repair, preservation and new behavior. Repair requires
  expected base failure and candidate pass under the same oracle; preservation requires both passes;
  new behavior does not require an executable base comparison. The selected experiment gate requires
  candidate success and at least one registered valid killed control per criterion, with every other
  registered control resolved under that policy. Unsupported/incompatible/circular/ambiguous evidence
  remains unresolved. Requested, applicable and executed denominators MUST remain inspectable.
- `CEX-V0-007`: Historical verification MUST rederive canonical input/inventory/artifact identities,
  complete scenario enumeration and classifications from retained original observations without
  running tests. Historical task/attempt captures MUST remain distinct from current authority.
  A digest-valid packet establishes integrity and recorded observations only; execution remains
  CALLER_REPORTED and semantic mappings REVIEWER_ATTESTED, never authenticated or universally adequate.
- `CEX-V0-008`: A native COMMAND gate MUST independently recheck coherent current ticket/queue/attempt
  snapshots before and after verification, reject pending redo/barriers/movement, and require unchanged
  acceptance/policy/configuration/generation plus a live unexpired BUILT/CHECKING lease and exact
  submitted clean candidate. Historical validity MUST survive a later acceptance change while live
  applicability refuses it. The first slice invalidates whole applicability; selective reuse is excluded.
- `CEX-V0-009`: Successful gate output MUST retain bounded structured identifiers/digests for the
  exact plan, receipt, CEM, acceptance inventory, raw-artifact manifest, task/attempt generation and
  candidate. Actual compiled CLI and disposable nonfixture native Tasks init, qualification, cutover,
  claim, submit, command gate, integration, completion, readback and audit MUST be retained before
  describing this integration as locally exercised. This does not activate production policy.
- `CEX-V0-010`: CEM 0.1/0.2/0.3, frozen compatibility packets, Core release content and default Tasks
  schemas/policy MUST remain unchanged. The companion MUST retain bounded failures and inert next
  actions, never automatically change/reopen a production ticket, narrow required verification or
  grant merge authority. Product superiority and broader-provider qualification remain separate
  outcome obligations; a passing feasibility demo cannot promote this profile.

## Trust and execution boundary

Owner-accepted criteria remain authoritative. Oracle expectations and control relevance need a
reviewed anchor independent of the candidate's implementation outputs. A different author/model,
hash or mutation score does not by itself establish that independence. A circular or disputed
oracle cannot close the selected criterion experiment policy.

The runner executes trusted local Go source; it is not an operating-system security sandbox.
The companion itself writes only the explicitly selected new artifact directory. Selecting output
inside a repository can dirty its worktree and make the live clean-candidate gate refuse; prefer
private output outside the repository. Test code is not constrained to that output directory.
It disables network module resolution and inherited Go configuration, but cannot prove that arbitrary
test code never contacts a service. No repository text implicitly authorizes execution. The command
requires explicit admission and exact-plan approval. Go executable identity is narrower than full
toolchain/environment attestation. Same-user hostile mutation and detached subprocesses are excluded.

Raw test/native-read observations are local artifacts beside the plan/receipt. CEM itself remains
reference-only and source-content-free under its existing narrow definition. These additional
artifacts can contain test output and acceptance text and are not automatically safe to publish.
CLI summaries retain digests and outcomes rather than printing raw logs.

The plan is bounded to at most 16 criteria, four controls per criterion, 256 regular source files
and 16 MiB per reconstructed source inventory. Each process admits a timeout of 1–120 seconds;
stdout and stderr each cap at 1 MiB, with immediate owned-group cancellation on overflow.
Individual JSON documents cap at 4 MiB. Source files retain their Git regular-file permissions
(`100644` or `100755`) in the constructed workspace, independent of the host umask. Failures preserve
historical data and block current experiment satisfaction. The consumer does not infer completeness
of requirements beyond the declared canonical acceptance array or correctness beyond registered controls.

## Failure modes and acceptance

Required adversarial cases include valid repair/preservation/new-behavior observations, a real
surviving control, a base-incompatible oracle, circular or unreviewed oracle, invalid build control,
wrong assertion, missing/surplus criterion, malformed/duplicate/truncated observation, tampered log,
stale CEM/source/oracle, moved native snapshot, changed acceptance/policy/configuration/generation,
different integrated tree, expired lease, output overflow and interrupted process descendants.
Unknown or unavailable evidence never receives a success substitute.

Focused package tests and vet, compiled CLI usage, native disposable lifecycle and independent
criterion review are required. Frozen CEM verification and dogfood bind/check/seal apply to this
Corvint source change. A fixture source application's behavior proves only that synthetic scenario;
using a real nonfixture Tasks store does not make the application an external adopter.

## Outcome experiment

The outcome protocol is proposed and must be frozen with actual cohort identities before execution.
External held-out cohort: 30 owner-qualified task/change pairs, ten each repair, preservation and
new behavior, from at least three repositories; record every screened case and rejection. Missing
cohort/corpus access is EXTERNAL_DEPENDENT and NOT_RUN, not a promotion pass.

Each pair has three arms: current CEM workflow; ordinary stronger testing with the same challenge
capability; and that same testing plus persistent criterion packets and Tasks consumption. Match
model/effort, available tools, review time, experiment compute and full-task budget. Randomize arm
order with preregistered seed 573; use separate sessions/maintainers to prevent cross-arm learning.
Independent adjudicators receive owner criteria and held-out wrong/correct patches, including
follow-up edits, without arm identity. Cases with unavailable execution remain in reported denominators.

Primary outcomes are incorrect acceptance and false rejection; secondary outcomes are missed
preservation failures, correct next-action selection, review/full-lifecycle effort and stale-evidence
rejection after a later edit. Report raw counts, paired uncertainty and all losing arms. Proposed
advancement requires at least 20% fewer incorrect acceptances than equally strong ordinary testing,
with a positive paired 95% interval, no more than two percentage points worse false rejection, and
no more than 15% greater median full-lifecycle effort. Alternatively, at least 20% lower median
full-lifecycle effort with no observed accuracy regression can justify a narrower workflow hypothesis,
not a correctness breakthrough. These thresholds need owner acceptance and a power analysis before
claiming a qualifying trial; thirty cases may establish feasibility without sufficient power.

Stop or simplify the new-profile hypothesis when persistent packets fail to improve decisions or
lifecycle cost over equally strong ordinary testing. Do not use mutation score, citation count,
packet size, a toy example or an empty frontier as a substitute outcome measure. Parent V1-0573
retains the original outcome obligations; implementation ticket V1-0575 is the bounded feasibility slice.

## Rollout, compatibility and rollback

Build the separate companion explicitly. Use it only in a caller-owned disposable trial or a
separately adopted command-gate policy. Nothing installs it into Core, qualifies `cem/0.4`, changes
native defaults or launches it from read hooks. Publish source/examples and precise limitations first.
Remove the optional companion/profile/docs to roll back; retain every emitted historical packet,
Tasks receipt and failed experiment. Older Core/Tasks readers keep their existing behavior.

## Traceability and evidence

| Requirement | Implementation | Verification anchor |
|---|---|---|
| `CEX-V0-001` | `schema.go`, companion CLI | `TestSchemaAdmissionClosedAndBounded`; `TestCLIAdmissionAndUnknownVerb` |
| `CEX-V0-002` | `authority.go`, `PlanExperiment` | `TestCriterionCaptureRetainsClaimAcrossNativeBodyEdit`; `TestClaimedAcceptanceAndSequenceBindings`; actual compiled `demo.py` |
| `CEX-V0-003` | `source.go` inventory and pinned overlay | `TestImmutableInventoryOverlayAndRefusals`; `TestFixedRunnerActualPassFailureAndBinding` |
| `CEX-V0-004` | `run_unix.go`, `Run` | `TestFixedRunnerActualPassFailureAndBinding`; `TestRunnerCancellationRetiresOrdinaryDescendants`; `TestRunnerOutputOverflowCancelsGroup` |
| `CEX-V0-005` | `classify.go` | `TestClassifierNamedAssertionAndCompleteEvents`; `TestFixedRunnerWrongAssertionDiagnostic` |
| `CEX-V0-006` | scenario enumeration and `Verify` | `TestRelationsPreserveScenarioDenominators`; `TestHistoricalVerifyArtifactsAndSurvivors`; three-relation live demo |
| `CEX-V0-007` | captured inputs and historical `Verify` | `TestHistoricalVerifyArtifactsAndSurvivors`; live demo historical verification after acceptance change |
| `CEX-V0-008` | coherent audited native reads and live checks | `TestLiveProjectionStaleness`; live demo dirty submitted candidate and changed/terminal task refusal |
| `CEX-V0-009` | `Summary` and standalone gate CLI | `TestHistoricalVerifyArtifactsAndSurvivors`; actual nonfixture native command-gate/lifecycle observations |
| `CEX-V0-010` | separate companion and unchanged provider contracts | `TestSourceProfileRejectsBroadenedExecution`; frozen CEM corpus; exact scoped diff/review |

Implementation filenames in this table are under `internal/criterionexperiment`; CLI tests are
under `cmd/corvint-cem-experiments`. Concrete run records, independent findings and final CEM
binding are retained in `docs/build-log/2026-09-30-cem-criterion-experiments.md` and private
local evidence. Unrun external outcomes stay NOT_RUN. Native binding field negatives have
focused component coverage; the live acceptance-change case also follows terminal completion,
so it does not isolate that field as a causal comparison. The fixture source application is
not evidence from an external adopter.

## Open decisions

Technical-profile acceptance, portable independent-consumer qualification, broader languages,
selective evidence reuse, full toolchain attestation and external product superiority are not
decided by this experimental implementation. Gate satisfaction is satisfaction of the explicitly
selected local experiment policy, not a theorem that the task is correctly or completely implemented.
