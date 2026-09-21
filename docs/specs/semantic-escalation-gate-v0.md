# Semantic Escalation Gate V0

Owner: Russell Lewis
Date: 2026-08-23
Intent status: accepted (decision 0047, 2026-09-04)
Delivery status: experimental (deterministic gate slice; no provider, calibration, or kill gate)
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`,
`docs/ARCHITECTURE.md`, `docs/specs/genesis-backfill.md`
Owner amendment anchor: direct 2026-09-21 instruction to adopt the useful typed-decision,
probability-distribution, and explicit-abstention ideas without adding Jev or another hosted
dependency to Corvint Core.

## Agent digest
- Claim: Models may propose evidence for named semantic gaps only through bounded calls and independent registered verifiers.
- Status: accepted (decision 0047, 2026-09-04)/experimental (deterministic gate slice; no provider, calibration, or kill gate)
- Exists: the contract plus an unwired in-process gate (`internal/semescalate`) tested only against provider spies, including a typed-choice path over mechanically supplied anchored options.
- Blocked on: a registered extractor profile, frozen calibration, held-out replay, and kill-gate evidence.
- Read next: User and measurable job; Deterministic acceptance and adversarial matrix; Traceability.

## User and measurable job

When Corvint backfills or refreshes knowledge, it should establish as much as possible from Git,
syntax, manifests, tests, specifications, history, tickets, documents, and registered mechanical
verifiers. If a named semantic gap remains, Corvint may use the least-cost adequate model only to
propose evidence for an existing verifier. It must preserve uncertainty, authority, privacy,
reproducibility, and hard cost bounds.

The V0 job is successful when model calls are rare, bounded, independently attributable, and add
measured critical recall over a mechanical-only baseline without changing what Corvint is allowed to
claim.

## Verified current state

- `docs/TECHNICAL-BRAIN.md` requires unchanged content to avoid repeat model calls and model output
  to remain content-addressed, mechanically anchored, and non-authoritative.
- `docs/ARCHITECTURE.md` defines optional anchored extraction and requires an edge-specific verifier,
  but does not define an executable routing, selection, receipt, or replay contract.
- `src/context_corvint_claims.py` has one generic lexical checker shared by `test-proves-claim`,
  `adr-governs-symbol`, and `handler-implements-feature`. Its own contract says it verifies anchoring
  and lexical support, not semantic truth. It is therefore an **admission-only** checker: an accepted
  result is an anchored `INFERRED` candidate and cannot satisfy a semantic obligation or close a
  frontier.
- No model runner, real provider adapter, or calibrated model registry is implemented. No such
  integration is in scope until an anchored extractor has a registered task, response, and
  admission-verifier profile.
- `internal/semescalate` is an experimental deterministic gate slice (decision 0152). It registers
  no provider or verifier, imports no process, network, or filesystem package, keeps its derivation
  ledger in memory only, and is imported by no `cmd/` or `internal/` production package. Its tests
  drive it only through an in-process provider spy that counts invocations and records every request.

## Requirements

- `SEG-001`: Corvint MUST complete deterministic inventory, syntax extraction, exact joins,
  Git/history links, test-name/docstring extraction, registered static verification, bounded
  widening, and cached-derivation lookup before considering a model.
- `SEG-002`: Every residual semantic gap MUST have a deterministic identity over repository
  authority, revision/tree, task profile, ordered evidence handles, access context, verifier
  profile, and policy digest.
- `SEG-003`: Corvint MUST emit a route result for every eligible gap, including `NO_CALL`. Stable
  no-call reasons are `MECHANICALLY_RESOLVED`, `CACHED_DERIVATION`, `NO_ADMISSION_VERIFIER`,
  `POLICY_DENIED`, `UNAUTHORIZED_INPUT`, `SECRET_RISK`, `INCOMPLETE_SCOPE`, `UNSUPPORTED_INPUT`,
  `BUDGET_EXHAUSTED`, and `NO_CALIBRATED_MODEL`.
- `SEG-004`: No registered deterministic admission verifier means no model call and an explicit
  `UNKNOWN(verifier-unavailable)` frontier. An anchor checker is not a semantic closure verifier.
- `SEG-005`: A call is permitted only for a named unresolved gap where a model can propose inputs
  to an existing admission verifier. V0 call triggers are limited to
  `DETERMINISTIC_CANDIDATE_EMPTY`, `RANKER_DISAGREEMENT`, and `ANCHOR_AMBIGUITY`.
- `SEG-006`: Core MUST use a provider-neutral adapter boundary equivalent to
  `describe() -> capabilities` and
  `invoke(canonical request, limits) -> opaque response bytes plus operational observation`.
  Adapters receive no Corvint authority API and no implicit filesystem, Git, credential, or tool
  access.
- `SEG-007`: An eligible model MUST support the exact response schema, input bound, task/language
  profile, data boundary, and hard budgets, and cite a frozen calibration artifact for its exact
  model revision. Corvint MUST select the least-cost adequate model deterministically; provider names,
  mutable aliases, parameter-count claims, and model self-reports are not adequacy evidence.
- `SEG-008`: Normal-profile adequacy requires independently labelled accepted-candidate precision
  of at least `0.85`, zero critical fabrication, and at least `0.99` schema-valid output on the
  frozen profile corpus. Sensitive and critical profiles additionally require local-only execution
  and human review. No adequate model produces `NO_CALL/NO_CALIBRATED_MODEL`, never a guessed route.
- `SEG-009`: Escalation MUST stop after two total model calls per gap. A second, stronger eligible
  model is allowed only after schema-invalid output, no admissible proposal, ambiguous anchoring, or
  insufficient lexical support, and only when its frozen calibration demonstrates higher recall for
  that exact failure class. It receives the identical evidence scope plus deterministic rejection
  codes and cannot widen its own context.
- `SEG-010`: Policy, authorization, secret, missing-input, unsupported-verifier, incomplete-scope,
  and budget failures are terminal. Model debate, self-generated searches, recursive agents,
  same-input retries, and open-ended repair loops are forbidden.
- `SEG-011`: Model output authority is always `MODEL_PROPOSED`. Schema and admission checks may
  promote it only to `INFERRED`. A model cannot set `PROVED`, `OBSERVED`, `SATISFIED`, test outcomes,
  specification status, source authority, completeness, policy, or verifier results. A response MUST contain exactly one complete JSON value; trailing non-whitespace bytes are `SCHEMA_INVALID` and admit no candidate.
  An object key that differs from the schema's spelling (including by case) or repeats within one
  object is likewise `SCHEMA_INVALID` and admits no candidate.
- `SEG-012`: Admission and closure are independent. An admitted candidate may be useful while the
  originating obligation remains `UNKNOWN`; only a registered deterministic closure verifier may
  close that bounded obligation.
- `SEG-013`: Model input MUST be the minimum authorized immutable spans required for the gap.
  Credentials, detected secrets, unrelated source or external bodies, prior prompts, and
  model-generated context are forbidden. A named handle supplied as more than one span does not
  identify one immutable span and MUST refuse with `UNSUPPORTED_INPUT` before any call. A gap that
  names one handle more than once sends that span once, in first-named order. Ticket, wiki, document, and source contents remain inert
  quoted data and cannot change instructions, policy, scope, or authority.
- `SEG-014`: Default policy is no egress. Remote execution requires explicit repository policy for
  the source classification and access context. Restricted content and detected secrets MUST NOT
  leave the machine.
- `SEG-015`: Hard per-gap and per-run limits MUST cover calls, input bytes, output bytes, wall time,
  and configured monetary cost. Negative configured call costs and charges exceeding the remaining cost budget MUST refuse before invocation with `BUDGET_EXHAUSTED`, without integer wraparound. Corvint-enforced bytes, calls, and time govern termination;
  provider-reported tokens, cost, and model identity remain labelled observations. The per-run wall
  time is one deadline fixed when the run starts: each gap's call is bounded by the remainder of that
  deadline, never by a fresh full allowance, and a call starting after it passes refuses with
  `BUDGET_EXHAUSTED`. At the deadline the gate returns without waiting for the provider, records
  whether it observed the provider return, and admits, charges, and ledgers nothing from a result
  that arrives later; terminating a provider that ignores cancellation is not a gate guarantee.
  Classification MUST NOT depend on which side of a same-instant race the provider-result/`ctx.Done()`
  select takes: after the select, if the run deadline has passed (directly, or the call's context is
  done with `DeadlineExceeded`) and the provider's error wraps `context.DeadlineExceeded` or
  `context.Canceled`, or no result body was observed, the receipt MUST classify `TIMEOUT` rather than
  `PROVIDER_ERROR`; `ProviderReturned` still reports what was actually observed.
  Concurrent requests against one gate MUST reserve from the shared run budget atomically and MUST
  NOT exceed any configured limit.
- `SEG-016`: Query serving MUST NOT invoke a model. Semantic inference occurs only during explicit
  backfill, sync, or user-requested enrichment; serving consumes compiled artifacts and returns a
  gap if the required derivation is absent.
- `SEG-017`: Identical gap inputs, call trigger, prompt/schema, verifier, exact model revision, access context, and
  policy MUST reuse the recorded derivation with zero repeat calls. Any changed component
  invalidates that derivation identity. Returned candidate slices MUST NOT alias the recorded derivation: caller edits to either a fresh result or a cache hit cannot change later replayed candidates or their authority.
  Concurrent requests for an identical derivation MUST still make at most one provider call.
- `SEG-018`: The experimental gate MAY offer a typed-choice profile only over `2..254` ordered,
  mechanically supplied options plus one reserved abstain option. Every option MUST carry a unique
  bounded lowercase ID and one unique `(handle, excerpt)` anchor already present in the authorized
  selected spans. The question ID, instructions, ordered option IDs, handles, excerpts, probability
  scale, and caller-owned confidence threshold MUST be canonical and content-addressed. Invalid,
  secret-shaped, unanchored, duplicated, or oversized question input MUST refuse before invocation.
  Every caller-authored string transmitted to the provider MUST be secret-screened, and selected
  evidence bytes MUST be snapshotted before request construction so later caller mutation cannot
  change either the request or verification input.
- `SEG-019`: A typed-choice response MUST contain exactly one complete object with exact-case,
  non-duplicate `questionId`, `choice`, and `probabilities` keys. `probabilities` MUST contain exactly
  one non-null integer mass in `[0, 1_000_000]` for every supplied option plus abstain, with an
  overflow-safe total of exactly `1_000_000`. The selected choice MUST be the unique maximum. Core
  computes the uncalibrated confidence margin as winner minus runner-up; a tie, explicit abstain, or
  margin below the canonical threshold admits no candidate and leaves the frontier `UNKNOWN`.
- `SEG-020`: A typed-choice provider MUST NOT author an evidence handle, excerpt, authority, verdict,
  threshold, or confidence value. Core reconstructs the selected proposal from the canonical option
  and submits it to the same registered independent verifier. Only verifier admission may emit an
  `INFERRED` candidate, and admission still cannot close the originating obligation.
- `SEG-021`: Typed-choice eligibility MUST require its exact response schema and MUST NOT change or
  accept the legacy proposal schema. The derivation identity MUST bind the choice prompt, response
  schema, complete canonical question digest, verifier, exact model revision/calibration, and input
  span digests. Any question mutation invalidates reuse; an unchanged complete identity makes zero
  repeat calls.

## Canonical receipt requirements

Every eligible gap emits a `corvint-semantic-route/0` receipt containing the gap identity, task and
risk profiles, source-manifest digest, opaque access-context ID, policy digest, verifier profile,
verifier version/code digest, verifier power (`admission` or `closure`), `CALL` or `NO_CALL`, stable
reason, eligible exact model revisions and calibration digests, selected model when any, and hard
limits. The receipt records decision inputs and results without source bodies.

Every attempted invocation emits a `corvint-semantic-call/0` receipt containing:

- exact ordered immutable input handles and span digests;
- prompt-template, response-schema, adapter, route-policy, and calibration digests;
- exact model reference/revision, trigger, attempt number, and limits;
- canonical request-byte, raw response-byte, and parsed-output digests;
- for a typed decision, the canonical question digest, ordered integer distribution, selected option,
  caller threshold, explicit abstention state, and gate-derived uncalibrated confidence basis;
- locally observed input/output bytes, call count, wall time, termination state, and whether the
  provider's return was observed;
- separately labelled provider-reported token, cost, and model observations; and
- independently produced admission/closure results and stable rejection reasons.

Model-authored verdict, policy, authority, verifier, or completeness fields are invalid input.
Timestamps and latency observations do not participate in content identity. Receipts are private by
default: paths and hashes are not assumed anonymous or safe to publish.

## Deterministic replay boundary

Corvint guarantees deterministic reverification of stored response bytes, byte-identical clean and
incremental compilation from the same authorized source manifest plus semantic-derivation ledger,
and zero calls for an unchanged gap identity. It does not claim that invoking a model again will
reproduce its prior bytes.

A discarded model response is a missing derivation input, not a cache miss. A source-only rebuild
omits that inference and returns `UNKNOWN(MISSING_DERIVATION)`, or consumes an explicitly supplied
derivation ledger. Re-invocation creates a new observation and derivation identity; it is never
described as deterministic replay.

## Trust boundary, limits, and failure behavior

The deterministic router owns scope, authorization, model eligibility, input construction, limits,
receipt construction, schema validation, and verifier invocation. The provider adapter owns only
transport to one configured model endpoint. Provider output is hostile untrusted data.

V0 permits at most two calls for one deterministic gap and no model call on the serving path. A
limit, timeout, malformed response, ambiguous or stale anchor, missing derivation, missing exact
model revision, access-context mismatch, or verifier failure preserves a complete failure receipt
and an `UNKNOWN` frontier. No partial candidate enters durable evidence.

Critical areas may receive local model proposals for human review, but model evidence never closes
security, authorization, privacy, payment, destructive-operation, migration, or incident-response
obligations by itself.

## Non-goals and simpler baseline

The baseline is mechanical-only Genesis plus explicit `UNKNOWN` gaps. V0 does not require a hosted
service, provider SDK in Core, embeddings, vector or graph database, autonomous agent, tool-using
model, general prompt framework, model debate, automatic model training, generated ontology,
provider marketplace, prose quality ranker, or runtime model calls.

V0 registers no provider and makes no call until one narrow anchored-extraction profile and its
admission verifier, corpus, adapter, limits, and rollback path exist. Cited document/spec drafting is
a later task profile, not an implicit expansion of this contract.

## Deterministic acceptance and adversarial matrix

| Case | Required result |
|---|---|
| Mechanically resolvable or cached gap | Zero provider calls; deterministic `NO_CALL` receipt |
| Unsupported relation or missing admission verifier | Zero calls; exact `UNKNOWN(verifier-unavailable)` |
| Two adequate models | Deterministically select the least-cost adequate exact revision |
| Smaller model lacks exact-profile calibration | Select the next adequate revision or `NO_CALIBRATED_MODEL`; never guess |
| First output schema-invalid or anchor-inadmissible | At most one allowed escalation; second failure stops |
| Policy, secret, ACL, scope, unsupported-verifier, or budget failure | Zero escalation and no partial evidence |
| Model requests tools, files, searches, commands, wider scope, or policy changes | Reject or ignore request; adapter has no such capability |
| Prompt-like external content | Remains inert; cannot influence policy, authority, limits, or routing |
| Unprovided path/span, forged hash, moved/duplicate/comment/string-decoy anchor | Stable rejection before durable graph mutation |
| Lexically supported but semantically false relationship | At most anchored `INFERRED`; obligation remains `UNKNOWN` |
| Model emits authoritative verdicts or test/spec state | Reject fields; independent result is unchanged |
| Provider misreports usage or identity | Locally enforced byte/time/call bounds still hold; observations stay labelled |
| Output exceeds byte/time bound | Terminate, retain failure receipt, admit no candidate |
| Provider ignores cancellation at the run deadline | Return at the deadline with `TIMEOUT` and provider return unobserved; the late result reaches no candidate, usage, or ledger |
| Several gaps each consume most of the wall-time budget | Calls share one run deadline; a gap after it passes refuses before invocation with `BUDGET_EXHAUSTED` |
| Two attempts disagree | Preserve explicit conflict/unknown; never choose by size or authority claim |
| Unchanged complete identity | Zero calls and byte-identical compilation with the same derivation ledger |
| Missing derivation ledger | No silent serving/rebuild call; exact `UNKNOWN(MISSING_DERIVATION)` |
| High-access derivation under a low-access caller | Invalidate without exposing restricted selectors, paths, hashes, counts, or claims |
| Choice option names an unprovided handle, fabricated excerpt, duplicate anchor, or reserved ID | Refuse before invocation with no partial candidate |
| Choice response omits/adds/duplicates a label, uses non-integer or out-of-range mass, sums incorrectly, or has no unique maximum | `SCHEMA_INVALID`; no candidate |
| Choice response selects abstain or falls below the caller-owned confidence margin | No candidate; explicit decision rejection; frontier remains `UNKNOWN` |
| Choice response selects a valid high-margin option | Reconstruct the immutable proposal, run the registered verifier, and cap any admitted result at `INFERRED` |
| Choice-capable provider enters the proposal path, or the reverse | `NO_CALIBRATED_MODEL`; zero calls |

Every test fixture set includes genuine-pass, fabricated-fail, and no-input cases. Provider spies
must independently count invocations so a self-reported zero cannot pass.

## Calibration, promotion, and kill criteria

A promoted task profile freezes at least 50 independently labelled cases, including at least 20
no-gold or adversarial cases. Each pins the repository revision, authorized evidence handles,
mechanical output, residual gap, verifier availability/power, language/format, risk/privacy class,
gold candidates or no-gold result, expected route, and mechanical-only and model-everything
baselines.

Promotion requires all of:

- mechanical yield of at least `0.70` over independently labelled useful evidence in supported
  sources;
- model-call rate at most `0.20` over deterministic eligible gap IDs;
- accepted anchored-candidate precision at least `0.85` and zero critical fabrication;
- critical-recall-at-five improvement at least `0.10` over mechanical-only;
- at least `0.50` lower model input tokens than model-everything;
- `1.0` route-selection, privacy, authority-ceiling, budget, unchanged-input reuse, and
  clean/incremental equality compliance; and
- no regression in first-use time, end-task correctness, or critical misses.

Kill or disable the semantic path immediately after unauthorized egress, a call without a registered
admission verifier, authority promotion, a critical false candidate, or a falsely closed frontier.
Kill or narrow a profile above `0.35` model-call rate, below `0.85` accepted-candidate precision,
below `0.10` critical-recall-at-five gain, or below `0.50` token reduction. Remove second-tier
escalation if it adds less than `0.05` absolute recall while adding more than `0.20` model cost.

## Rollout, rollback, compatibility, and drift

Rollout is mechanical-only instrumentation, then frozen calibration, then one local opt-in
admission-only extractor profile, then a held-out advisory backfill trial. Closure or enforcement is
separate and requires a registered closure verifier plus its own outcome gate.

Rollback disables the profile and removes disposable model-derived views while preserving source,
accepted intent, route/failure evidence permitted by retention policy, and explicit unknowns. The
provider-neutral receipt and adapter boundary must permit changing or removing every model without
changing Core evidence authority.

Changes to prompt, schema, adapter, verifier, routing policy, model revision, calibration, source
manifest, or access context create a new derivation identity. Mutable model aliases are ineligible
for promoted profiles. Calibration drift or precision below the promotion floor disables new calls;
existing candidates remain visibly tied to their original derivation and may be reverified but not
silently regenerated.

## Unresolved decisions

- The first narrow anchored-extraction task profile and independently owned calibration corpus.
- The local adapter process protocol and private derivation-ledger location/retention policy.
- The deterministic least-cost ordering when local monetary cost, latency, energy, and remote cost
  disagree.

None blocks mechanical Genesis work. They block model runner/provider integration.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `SEG-002` | `internal/semescalate.GapID` | `TestGapIDIsDeterministicOverIdentityFields` |
| `SEG-003` | `Gate.Escalate` route receipt on every path | `TestMechanicallyResolvedGapNeverCallsProvider`, `TestUnnamedGapIsRefused`, `TestBudgetsRefuseBeforeInvocation` |
| `SEG-004` | no registered verifier profile | `TestMissingAdmissionVerifierAbstains` |
| `SEG-005` | named-trigger admission; an unnamed trigger is `UNSUPPORTED_INPUT` | `TestUnnamedGapIsRefused` |
| `SEG-006` | `Provider` (`Describe`, `Invoke` over request bytes and limits) | `TestSecretAndOutOfScopeContentNeverReachProvider`, `TestPackageHasNoProcessNetworkOrFilesystemImports` |
| `SEG-010` | terminal refusals; at most one call per gap; failed calls are ledgered, never retried | `TestOutputAndWallTimeLimitsAdmitNoCandidateAndNeverRetry` |
| `SEG-011` | strict proposal schema; gate-set `INFERRED` ceiling | `TestAdmittedProposalsCapAtInferredAndLeaveObligationUnknown`, `TestProposalSchemaRequiresCompleteJSON`, `TestProposalSchemaRefusesFoldedAndDuplicateKeys` |
| `SEG-012` | admission half only: admitted candidates leave the frontier `UNKNOWN` | `TestAdmittedProposalsCapAtInferredAndLeaveObligationUnknown` |
| `SEG-013` | named spans only; unauthorized, missing, secret-shaped, or ambiguously duplicated spans refuse; a handle named twice is sent once | `TestSecretAndOutOfScopeContentNeverReachProvider`, `TestAdmittedProposalsCapAtInferredAndLeaveObligationUnknown`, `TestGapNamingOneHandleTwiceSendsTheSpanOnce` |
| `SEG-014` | default no egress: a remote-capable provider is `POLICY_DENIED` without explicit allow | `TestRemoteProviderIsDeniedByDefault` |
| `SEG-015` | per-run calls, input/output bytes, configured cost, and one run wall-time deadline; atomic concurrent reservation; unobserved provider return disclosed; provider usage labelled; deadline-race classification is deterministic | `TestBudgetsRefuseBeforeInvocation`, `TestOutputAndWallTimeLimitsAdmitNoCandidateAndNeverRetry`, `TestCostBudgetCannotOverflowOrCreditNegativeCosts`, `TestRunWallTimeIsOneDeadlineChargedAcrossGaps`, `TestTimedOutProviderIgnoringCancellationIsDisclosedAndCannotLeak`, `TestCooperativeDeadlineErrorIsAlwaysClassifiedTimeout`, `TestConcurrentChoiceCallsPreserveBudgetAndCacheInvariants` |
| `SEG-017` | in-memory derivation ledger keyed on the call trigger as well as the gap, prompt, verifier, model, and span digests; atomic concurrent reuse | `TestUnchangedIdentityReusesDerivationAndChangedComponentInvalidates`, `TestReturnedCandidatesCannotRewriteCachedDerivation`, `TestConcurrentChoiceCallsPreserveBudgetAndCacheInvariants` |
| `SEG-018` | canonical bounded `ChoiceQuestion` over mechanically anchored, snapshotted, secret-screened options | `TestChoiceQuestionRefusesUnanchoredOrUnboundedInputsBeforeCall`, `TestChoiceRefusesSecretHandleBeforeCall`, `TestChoiceSnapshotsEvidenceBeforeProviderLatency`, `TestTypedChoiceAdmitsOnlyMechanicallySuppliedOption` |
| `SEG-019` | exact integer distribution validation, unique maximum, derived margin, and explicit abstention | `TestChoiceResponseRequiresExactCompleteDistribution`, `TestChoiceConfidenceAndExplicitAbstentionAdmitNothing` |
| `SEG-020` | gate reconstruction of the selected option followed by the registered verifier and `INFERRED` ceiling | `TestTypedChoiceAdmitsOnlyMechanicallySuppliedOption` |
| `SEG-021` | schema-aware eligibility and complete question-bound derivation identity | `TestChoiceAndProposalSchemasCannotCross`, `TestChoiceQuestionIdentityInvalidatesCachedDerivation` |
| `SEG-001`, `SEG-007`, `SEG-008`, `SEG-009`, `SEG-016` | not delivered | mechanical-pipeline integration, least-cost model selection, calibration thresholds, second-tier escalation, and serving integration are absent; the slice consumes the caller's mechanical result, refuses a provider without an exact revision and calibration digest (`TestUncalibratedOrMissingProviderIsNotCalled`), caps each gap at one call, and `TestNoProductionPackageImportsTheGate` keeps it off every serving path |
| Whole profile | not delivered | real provider adapters, persisted derivation ledger, restricted-classification egress policy, closure verifiers, frozen calibration corpus, held-out backfill replay, ACL invalidation, and kill-gate evidence pending |
