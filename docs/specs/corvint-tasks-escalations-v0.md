# Corvint Tasks typed escalations V0

Owner: Russell Lewis
Date: 2026-10-04
Intent status: proposed overall; Gate A decisions accepted (decision 0428, owner answer 2026-10-04)
Delivery status: experimental

Authoritative inputs: owner request [issue 502](https://github.com/beamfall/corvint/issues/502)
(native ticket V1-0699); the issue's ESC502-001..011 preparation intent with the three precision corrections adopted
during its independent preparation review (not owner acceptance); the existing
[agent lease contract](corvint-tasks-agent-leases-v0.md); and two neighbouring intents this one
composes with but does not restate: issue 501's operator notes (claim-snapshot delivery) and issue
499's dispatch model-escalation tiers. The owner accepted the Gate A decisions listed under
Unresolved decisions as written on 2026-10-04 (decision 0428), so the writer and material slice may
integrate. That acceptance covers those design choices only; every requirement below stays
proposed until its own acceptance evidence is retained. The owner-delegated decisions of 2026-10-04
adopt the 500 and 501 intents only. A separate owner decision the same day chose the writer slot
(Integration slot). The owner gave it directly, through structured questions in the orchestrating
agent session.

## Agent digest
- Claim: Workers raise typed questions from admitted claims, operators answer them by compare-and-set, and the next same-acceptance claim receives the answers.
- Status: proposed overall; Gate A decisions accepted (decision 0428, owner answer 2026-10-04); experimental delivery of the pure four-file foundation, the optional record key, the native writer, derived `ESCALATION_PENDING` admission holds, the CLI writes and reads, claim delivery, dispatcher session classification with bounded infrastructure retry, and the closed `ESCALATION` typed-event stage with the receipt binding fold; no native completion, so no installed escalation capability.
- Exists: closed request/event/reference codecs and a pure reducer with answer selection, derived holds, effective work revision, coded refusals, an operation-scoped grant check and a per-call decode memo, plus focused tests and a near-capacity benchmark. The native ticket record carries the optional tool-owned `escalations` key: ordinary mutations and re-import keep it, ADOPT_FILE, CREATE and import cannot set or change it, and Core's read-only planner validates it. The native writer (`store.Escalate`, `store.AnswerEscalation`) commits OPEN, supersession and ANSWER as LEASE transactions that audit the claim receipt, bind the actor before replay and check operation-scoped `ESCALATE`/`ANSWER` grants. A current OPEN decision, scope or blocked question is a derived `ESCALATION_PENDING` hold, computed by one shared `internal/tasks/wire` predicate in native eligibility, direct claim, claim-next, recorded claimability, Core's planner and the dispatcher's roster and status; a refusal names the sorted request IDs (at most 16). `ticket escalate` reads the origin from the claim receipt and attempt the claim returned, `ticket answer` takes `--target`, and `ticket escalation list|show|history` are pure, paged reads with honest age and provenance. Claim and claim-next pin the same-acceptance answered references on the admitted attempt and deliver `escalationAnswers` from the claim receipt's POST attempt, so replay returns the original snapshot. The dispatcher classifies each ended session before no-progress parking: an infrastructure request raised by that session's own holder charges a restart-safe, reserved retry under the optional `infrastructureRetry` policy instead of parking, and keeps the selected 499 tier; a session ending behind a typed hold is held, not counted; a typed-only control write is not progress. A committed escalation stages under the closed `ESCALATION` operation (one request, one ticket, one or two content-addressed events, at most the ticket blob; at most 7 artifacts and 1879 descriptor bytes), and receipt audit and every redo replay each question reference change against its request entry, retained grant, claim admission and the pure reducer.
- Blocked on: kinds and ages in the `dispatch status` escalation section (ESC-V0-009), and integrated witnesses on a live store.
- Read next: Requirements; Failure modes and trust; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

Workers escalate today by writing `NEEDS OPERATOR:` into a status file. The text has no type, so a
claim or queue timeout looks the same as a product-scope question. Timeouts parked tickets and led
workers to set `needs-human`, and nothing clears a request once it is answered. The workaround is a
steward script that greps first lines for timeout wording. The operator needs typed, queryable
questions with visible age; answers delivered to the next worker; infrastructure failures retried
within a bound instead of parked; and decision or scope questions holding admission until answered.
The simpler baseline, prose in a status file plus a grep, is the failure this replaces.

At public main `4b10a02144faed4a77b36eba4b0d1cbc315706d3`, the ticket record, Core reader, native
writer, stage, CLI and dispatcher have no escalation concept. Dispatch fingerprints hash the ordinary
ticket revision, so any control write that increments content would look like work progress. This
delivery adds only `internal/tasks/ticket/escalation.go`, `internal/tasks/transaction/escalation.go`
and their tests. They are closed codecs and a pure reducer over explicitly supplied audited
observations. They read and write no files, take no lock, launch no command and grant no lease. No
writer, reader, CLI verb, claim path or dispatcher calls them. No existing `ticket.Record`, attempt,
writer, stage, CLI or dispatch file changes.

## Requirements

- `ESC-V0-001`: `ticket escalate --attempt A --kind decision|infrastructure|scope|blocked --question TEXT [--options a,b]` MUST create one typed question bound to the worker's immutable successful claim admission, never to caller-supplied lifecycle text.
  The question is 1..4096 UTF-8 bytes and not whitespace-only. Options are at most 8 unique nonempty strings of at most 256 bytes each. Text is advisory data and never classifies kind. The native origin is the exact successful CLAIM or CLAIM_NEXT receipt sequence and hash plus its POST attempt digest, supplied through the worker's invocation context; ticket, acceptance revision, generation and holder are resolved from that receipt, not from the latest attempt projection, because retries reuse attempt IDs. There is no caller generation, ticket, head or timestamp override. Missing or ambiguous origin refuses `MISSING_ADMISSION_CONTEXT`. A fresh OPEN additionally requires, under the writer lock, the same current generation, holder, reservation and acceptance revision and an active unexpired lease. The invoking local actor must equal the source holder and hold an explicit policy grant. Actor equality is a consistency fence, not authentication: ActorAuthentication stays NOT_OBSERVED. Actor binding, then canonical request replay, precede fresh expiry, state and CAS checks, so a committed question replays after its source expires. Supervised runtimes return UNSUPPORTED until their grant adapter is qualified.

- `ESC-V0-002`: Questions MUST be stored as an optional tool-owned `escalations` reference on the existing native ticket plus immutable canonical `taskman-escalation-event/0` evidence blobs, with one current event per question and no separate store.
  The reference is omitted until the first typed operation, so legacy ticket and attempt bytes stay identical. It holds a transaction revision, `lastControlTicketRevision`, `workRevision` and a sorted list of at most 64 lifetime request entries, at most 16 of them OPEN at the current acceptance revision. A stale OPEN entry counts only against the 64 lifetime entries; the 16 bound needs the current acceptance revision, so the writer and every reader enforce it, not the reference codec, and a reference over it refuses as `OPEN_CAPACITY`. Each entry has request ID, origin digest (the immutable OPEN event blob digest), current head digest, event revision, source acceptance revision, kind and lifecycle OPEN, ANSWERED or SUPERSEDED. The transaction revision counts typed transactions; the sum of entry event revisions counts events, with a wire ceiling of 4096 events and 64 per request. Each event is at most 65536 encoded bytes including LF. OPEN carries the question once in its nested original request and has no self-digest; ANSWER and SUPERSEDE carry the origin and previous head digests. Native receipt identity, the question origin digest and the operation's `requestSha256` are distinct identities. Closed operation-specific shapes refuse unknown, aliased or injected fields. Capacity exhaustion refuses before publication; nothing is evicted or pruned. CREATE, REFINE, ADOPT and import cannot inject or delete the reference, and the Core reader must preserve it.

- `ESC-V0-003`: Applicability MUST derive from the source acceptance revision versus the current ticket acceptance revision, not from lease state.
  An ended or expired source attempt does not close an unanswered same-acceptance question. An acceptance-changing mutation leaves old requests visible as STALE, removes them from current holds and claim guidance, and never promotes an old answer to the new scope. Unrelated content changes keep them current. Explicit supersession, `--supersedes Q --expected-request-revision N`, replaces one OPEN question atomically: the old request gains one SUPERSEDE event, the replacement starts at revision 1, two event slots and one transaction are charged, and ticket content increments once. The worker route requires the same admitted source. A worker cannot supersede an answered question or another source's question, and the administrative cross-source route stays unsupported until separately admitted. Supersession is never inferred from similar text. Native supersession is deferred by ESC-V0-010; the pure reducer keeps its paired shape.

- `ESC-V0-004`: `ticket answer --target T --text TEXT` MUST close exactly one question, either the sole same-acceptance OPEN question at commit or an exact `--request Q --expected-request-revision N` compare-and-set.
  Shorthand with zero or several open questions refuses and names them (`NO_OPEN_QUESTION`, or `AMBIGUOUS_OPEN_QUESTIONS` carrying the sorted request IDs); it never answers all or the newest. The original request retains only the caller's selector, text and optional CAS. The writer-resolved request ID and prior revision live in event material, so replay of a committed shorthand answer returns its original question after later questions open. Exact mode cross-checks the derived fields against the caller's. Concurrent answers to one question have one winner; changed bytes under a reused request ID conflict. Answer text is 1..8192 UTF-8 bytes and not whitespace-only. OWNER may answer subject to policy narrowing; OPERATOR needs an explicit answer grant; a source-holder grant is not answer authority. A supplied policy observation names the one request operation it covers, so an OPEN grant refuses `POLICY_NOT_ALLOWED` for ANSWER. An answer closes its request and removes only that request's hold. It approves no scope, changes no acceptance, clears no dependency, hold or budget, and selects no option.

- `ESC-V0-005`: Claim and claim-next MUST return current same-acceptance answers as an `escalationAnswers` field separate from `operatorNote`, pinned by the claim's own admission.
  Under the claim writer lock, sorted answer origin and head references are copied into an optional admitted Attempt field from the same audited ticket snapshot as TicketRecordSha256; no-answer attempts omit it. Output resolves only from the successful claim receipt's POST attempt, so a claim, a later answer and a replay still yield the original snapshot. Each answer carries request ID, original question, options and kind, source provenance, answer actor, time and head, and event revision. Delivery does not consume or acknowledge an answer, and SUPERSEDED and STALE answers are history only. Aggregate guidance is at most 256 KiB encoded and full claim output at most 1 MiB, checked before admission; an answer that would exceed capacity refuses instead of truncating. A materialization failure after commit preserves the attempt and receipt and instructs exact replay. This depends on the 501 claim-snapshot path.

- `ESC-V0-006`: A current OPEN decision, scope or blocked question MUST produce an explicit `ESCALATION_PENDING` derived admission hold, and infrastructure MUST NOT.
  The hold appears in native eligibility, direct claim, claim-next and dispatch status, so bypassing the dispatcher does not bypass it. A blocked question holds through its explicitly chosen kind and question, or through an optional validated same-queue `--blocked-by T [--gate G]` relation, which creates no real dependency. No text is classified. Holds never rewrite ticket status, set needs-human or cancel the live holder. Infrastructure is a typed observation of a failed admitted session, never native HELD or ordinary dispatcher Parked. A worker without a successful claim cannot raise one.

  Cross-reference: the leases spec's CAL-V0-102..103 (V1-0791) adds a second derived admission hold of the same shape, `LOOP_DETECTED`, computed from audited attempt history under an opt-in `loopDetection` policy; its dispatcher escalation is a typed `blocked` `needs-owner` event, not a native escalation record, and it defines no ESC requirement.

- `ESC-V0-007`: An optional dispatch `infrastructureRetry` policy MUST bound automatic retries per program, canonical ticket and acceptance revision, with restart-safe reservations.
  maxRetries is 0..10 (default 3 when enabled), cooldownSeconds 1..3600 (default 30) and maxCooldownSeconds is at least cooldown and at most 86400 (default 300). maxRetries 0 shows `INFRA_RETRY_DISABLED`. The episode lives in the existing dispatch ledger, with no new sidecar, and is shared across roles and request IDs so new text cannot refill budget. Each ended session counts once. A retry reserves its exact pending launch identity, charged ordinal and cooldown deadline in the same checked ledger clone before issuing a launch; republishing reuses that identity without another charge. A crash before spawn consumes the reservation unless no-spawn is proved, and an ambiguous spawn is UNKNOWN/HOLD, never an automatic replacement. Cooldown for ordinal n is min(maxCooldown, cooldown * 2^(n-1)), saturating. Config reload may narrow but cannot refill debt or recompute a reserved deadline. All ordinary native admission invariants still apply. Exhaustion shows `INFRA_RETRY_EXHAUSTED`, `NATIVE_RETRY_EXHAUSTED` or a named HOLD/UNKNOWN and leaves the native request OPEN. Checked work progress may mark the local episode RECOVERED without answering the native question; an operator answer does not prove recovery.

- `ESC-V0-008`: Session classification MUST precede ordinary no-progress parking, and typed control writes MUST NOT count as work progress.
  A session with a validated infrastructure request and no separately proved progress suppresses its no-progress count and park, keeps unrelated parked state and, for 499 tiers, counts as unknown progress: it resets only the proved failure suffix and retains the selected tier. Decision and scope sessions are held by typed policy, not failure parking. Each typed-only write sets `lastControlTicketRevision` to the post ticket revision and keeps the prior `workRevision` only when the audited pre revision equals the prior `lastControlTicketRevision`; otherwise `workRevision` becomes the audited pre revision, which also seeds the first control. Dispatch substitutes `workRevision` only while the current ticket revision equals `lastControlTicketRevision`, so an ordinary later edit stays visible. Missing or corrupt bindings are UNKNOWN, not progress.

- `ESC-V0-009`: `ticket escalation list`, show and history MUST be bounded, non-mutating native reads with honest age and provenance.
  Filters are kind, state, target and program. Pages are 1..50 (default 20) and at most 1 MiB, with cursors anchored to immutable heads and strict chain identity, decrement and cycle checks. Each entry shows original RecordedAt, nonnegative ageSeconds, clockUncertain, current or stale applicability and source generation and holder. Program appears only from an admitted producer mapping, otherwise UNKNOWN; caller text never supplies it. The native list reports retryState NOT_OBSERVED, and dispatcher retry state is labelled a dispatcher observation for an explicitly named program. `dispatch status` shows open request kinds, ages and holds separately from parked keys, retry state and 499 tiers. Reads write no ledger and hydrate no evidence.

- `ESC-V0-010`: Escalation writes MUST use the existing writer, receipt and request-index pipeline through a distinct closed typed-event stage, declared per operation, with a closed escalation material branch.
  Owner decision 2026-10-04 first chose issue 501's single derived-event MUTATE slot; decision 0428 item 7 then put the writer on the LEASE pipeline with supersession inside the StageLease limits of 11 artifacts and 2658 descriptor bytes. This revision (agent decision 2026-10-05 under the owner's in-task delegation; the owner may redirect) adds the distinct stage that decision anticipated. A committed OPEN, supersession or ANSWER stages as the closed `ESCALATION` operation: its request entry, exactly one ticket, one or two events at their content addresses (two only for a supersession) and at most one evidence blob, which must be that ticket's record. No attempt, reservation, pool, program or gate output is admitted, and the maximal descriptor, measured at 7 artifacts and 1879 bytes, stays within the StageLease limits; stage shape alone is not authority. Unrecorded refusals never stage. Attempts, reservations and gates are untouched, and the stage gains no generic evidence-path permission. Receipt audit and redo bind the derived event: a question reference may change only in the one ticket of a completed TRANSITION receipt, and is never dropped. Material validation checks the nested request, source receipt and grant, audited holder, generation and acceptance, expected revisions, old and new heads, content and work revision arithmetic, immutable origin, queue time and all posted bytes. Replay recovers the resolved shorthand target, answer, clock and outcome after later questions. Capacity overflow refuses before commit.

- `ESC-V0-011`: The pure foundation and the integrated capability MUST keep distinct evidence and layering boundaries.
  The four files `internal/tasks/ticket/escalation.go`, `internal/tasks/ticket/escalation_test.go`, `internal/tasks/transaction/escalation.go` and `internal/tasks/transaction/escalation_test.go` compute proposed references and events only. Every proposal reports ActorAuthentication and Durability as NOT_OBSERVED. `ticket` imports only the wire codec and never transaction, mutation or snapshot. Supplied ALLOWED, AUDITED or VALIDATED observations are checked for consistency, not authenticity. Passing pure tests cannot establish native writes, replay, races, holds, claim delivery, dispatcher retry, capacity or recovery. Full delivery requires those integrated witnesses, the existing CAL claim, generation-fence, retry, recovery, failed-exemption and progress suites, independent integrated review, current-main integration and native completion.

## Non-goals

No new queue, mutable store, status-file sidecar, service or background worker. No prose, regex or
timeout-string classification of kind. No ticket per question. No cryptographic worker
authentication or role rewrite. No automatic option selection, scope approval, acceptance change,
reopen, retry-count reset or unpark. No exactly-once or read-acknowledged answer delivery. No
history erasure to free capacity. The #494 timeout cause is fixed separately.

## Failure modes and trust

Missing or corrupt evidence surfaces as `MISSING_EVIDENCE`, `JOURNAL_FORKED` or UNKNOWN, never as
no question or an answered question. Unknown replay state refuses rather than proceeding as absent.
Supplied observations are trusted only for consistency. Refusals occur before any authoritative
effect. Question and answer prose is untrusted data, never instructions or authority. Infrastructure
exhaustion is a visible automation hold that may need operator remediation; the infrastructure kind
does not promise unlimited recovery. Total cost stays within existing store, receipt, attempt,
StageLease and dispatch ledger limits (16 MiB, 8192 histories).

## Acceptance evidence and traceability

The pure deliveries supply focused evidence only. The writer slice adds native store tests that run
the real writer against real claim receipts. ESC-V0-006's derived hold is applied on the native
read and claim paths and in the dispatcher over hand-built records, and on native claims over a
writer-produced reference; every other integrated witness is NOT_RUN, and no row claims an installed capability.

TCP-00 §11 amendment (owner-accepted 2026-10-05, recorded on V1-0699 at receipt 2749): ESC-V0-006
adds `ESCALATION_PENDING` to the closed detail codes. With the 71 codes after the leases spec's A17
the extended set contains 72. The owner also accepted the compatibility cost: a Tasks reader built
before this code refuses a record or plan carrying it. Only a writer-produced reference can make it
appear.

| Requirement | Parent intent | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| ESC-V0-001 | ESC502-001 | CLI producer; receipt/grant adapter; writer fences | `TestIssue502_EscalationCodecAndBounds` (question/options bounds); `TestIssue502_AdmissionOriginAndStaleGeneration` (missing context, stale generation/holder/receipt/acceptance, expiry, actor and policy binding, unknown replay); native `TestIssue502_OpenAnswerCommitsTicketAndEvents` (real claim receipt), `TestIssue502_OpenAuditsTheClaim` (forged receipt digest abstains, expiry fences), `TestIssue502_ActorBindingBeforeReplay` (actor bound before and after commit, OPERATOR without a grant refused), `TestIssue502_SupervisedAttemptIsUnsupported` (attempt supervised after its claim); CLI `TestIssue502_EscalationCLIWritesAndReads` (origin from the claim receipt name or sequence, identical retry replays, a receipt that does not admit the attempt abstains `MISSING_ADMISSION_CONTEXT` without writing) | wrong-queue origin, compiled-binary run |
| ESC-V0-002 | ESC502-002 | ticket record and Core codecs; evidence store | `TestIssue502_EscalationCodecAndBounds` (closed shapes, noncanonical framing, question bound, 64-revision history cap; the 65536-byte event cap is untested); `TestIssue502_SupersessionAndCapacity` (transaction vs event counts); `TestIssue502_StaleOpenReleasesCapacity` (16-open bound counts the current acceptance revision only); `TestIssue502_ReadersRefuseOpenOverflow` (readers refuse a crafted 17-open reference; the capacity check refuses an invalid or older acceptance argument); `TestIssue502_RecordEscalationsKey` (legacy record bytes unchanged, carrier round trip, member equals the reference codec); `TestIssue502_RecordEscalationsRefusals` (control or acceptance revision after the record's, 17 current OPEN, near-miss key); `TestIssue502_MutationsPreserveEscalations`; `TestIssue502_AdoptRefusesEscalationEdits`; `TestIssue502_CreateCannotCarryEscalations`; `TestIssue502_ReimportKeepsEscalations`; `TestIssue502_ExportCannotCarryEscalations`; Core `TestIssue502_ReaderAdmitsSharedOptionalKeys`, `TestIssue502_ReaderRefusesMalformedOptionalKeys` and `TestIssue502_ReaderMatchesCodecBounds` (shared fixture written by the Tasks codec; Core enforces the 16-open bound, event capacity and native identifier rules) | a reference written by the native writer through every reader, evidence blobs in the store |
| ESC-V0-003 | ESC502-003 | reducer; native writer | `TestIssue502_SupersessionAndCapacity` (paired supersession, last-slot refusal, cross-source `SUPERSESSION_SOURCE`); `TestIssue502_ImmutableClaimAnswerSelection` (stale acceptance excluded); `TestIssue502_StaleOpenReleasesCapacity` (stale OPEN stays visible, holds nothing, refuses an answer); native `TestIssue502_OpenAnswerCommitsTicketAndEvents` (OPEN and ANSWER leave the acceptance revision unchanged and the attempt renewable) | acceptance edit yields STALE across show/plan/claim; administrative route |
| ESC-V0-004 | ESC502-004 | reducer; native writer; CLI | `TestIssue502_QuestionAnswerCASAndReplay` (ambiguous shorthand naming its open questions, exact CAS, Q1-answer then Q2-open shorthand replay, changed-selector conflict); `TestIssue502_AnswerGrantAndEmptyShorthand` (OPEN grant cannot answer and ANSWER grant cannot open, empty shorthand `NO_OPEN_QUESTION`, exact unknown `QUESTION_NOT_FOUND`); native `TestIssue502_AnswerRaceHasOneWinner` (two CAS answers through the store lock, one winner, the other `STALE_QUESTION_CAS`) and `TestIssue502_OpenAnswerCommitsTicketAndEvents` (shorthand answer reports its resolved target), `TestIssue502_ShorthandAnswerReplaysAfterLaterOpen` (a shorthand answer replays its original target after a later OPEN), `TestIssue502_OperatorAnswersThroughExplicitGrant` (OPERATOR refused by default, answers under an explicit row; a WORKER row naming ANSWER is refused); CLI `TestIssue502_EscalationCLIWritesAndReads` (ambiguous shorthand names both open request IDs, stale `--expected-revision` refuses `STALE_TICKET`, exact and shorthand answers report their resolved request) | compiled-binary run |
| ESC-V0-005 | ESC502-005 | lease admission; original-receipt materializer | `TestIssue502_ImmutableClaimAnswerSelection` (pinned selection, guidance capacity); `TestESCV0005_ClaimDeliversTheAnswersPinnedByItsAdmission` (no-answer claim omits the pin, an open infrastructure question admits the claim and is not delivered, a later answer moves neither the claim nor its exact replay, claim-next re-snapshots); `TestESCV0005_ClaimPinsAndResolvesAnswers` (same-acceptance ANSWERED only; missing, swapped, cross-bound and broken-chain material refused); `TestESCV0005_AttemptPinsTheAdmittedAnswers`; `TestESCV0005_ClaimResultCarriesPinnedAnswers` (UNAVAILABLE with references and replay warning; an oversized event is `JOURNAL_FORKED`) | missing-receipt replay, a 1 MiB output witness (bounded by argument: 256 KiB guidance, a 64 KiB note event and the claim base), a compiled-binary claim |
| ESC-V0-006 | ESC502-006 | shared wire predicate; native eligibility, claim and claim-next; Core planner; dispatcher roster, launch and status | `TestIssue502_TypedDispositionAndWorkRevision` (four kinds, held vs infrastructure IDs); `TestIssue502_EscalationPendingPredicate` (wire predicate: four kinds, stale, answered, superseded, unknown kind, noncanonical revision, sorted and deduplicated, 20 current questions bounded to 16); `TestIssue502_EscalationPendingView` (native eligibility blocks as `ESCALATION_PENDING` naming sorted IDs with next action `answer`; stale, infrastructure, answered and superseded admit; the record is unchanged; writes the shared case fixture); `TestIssue502_ClaimHonoursEscalationHold` (eligibility, direct claim, claim-next, recorded claimability and plan all refuse with sorted IDs and write nothing; admitted cases claim; claim-next passes over a held ticket; sixteen current IDs named); Core `TestIssue502_HoldAgreement` (Core's planner names the same holds as the Tasks reader for every shared case and blocks as `ESCALATION_PENDING`) and `TestIssue502_PlannerBlocksUnmodelledConstraints`; `TestIssue502_DispatchStatusShowsEscalationPending` (status lists held tickets with sorted request IDs from the dispatcher's own ledger, apart from parked); `TestIssue502_DispatchNeverLaunchesForEscalationHold` (the native observation carries decision, scope and blocked holds; no role, with or without a selected-plan requirement, rosters the ticket and no session launches; stale, answered, superseded and infrastructure questions launch); `TestIssue502_DispatchStatusKeepsHoldBehindOtherBlockers` (with pool, pause and ordinary-hold primary reasons, the hold survives the observation, ledger and status and is never rostered); `TestIssue502_LedgerKeepsEscalationsBesideProgress` and `TestIssue502_LedgerKeepsEscalationsBesidePoolSweeps` (a ledger carrying the hold beside progress or a pool sweep saves, loads and survives restart; the answered hold drops); `TestIssue502_LedgerRefusesMalformedEscalations` (aliased, duplicate, unsorted, repeated, empty or malformed holds are refused without a rewrite); `TestIssue502_DispatchStatusLoadsHoldBesideStrictMembers` (status loads and lists the hold beside progress and beside a pool sweep); native `TestIssue502_WriterReferenceHoldsAndReleases` (a writer-produced OPEN decision holds direct claim and claim-next after release, both refusing `ESCALATION_PENDING` naming `q-1`, refusals write nothing, the writer's ANSWER lifts the hold and the ticket claims) | a writer-produced reference through Core's planner and a live dispatcher; kinds and ages in dispatch status (ESC-V0-009) |
| ESC-V0-007 | ESC502-007 | dispatch config, ledger, loop and status | `TestIssue502_InfraRetryPolicyBoundsAndCooldown` (defaults 3/30/300, bounds refused, doubling cooldown saturating at the cap); `TestIssue502_InfraSessionChargesWithoutParking` (no launch before the deadline, the due retry RUNNING under its identity in the saved ledger, the final allowed retry then `INFRA_RETRY_EXHAUSTED` with no launch, holders still listed from earlier sessions counted once each, an operator unpark releases without refilling); `TestIssue502_InfraRetryDisabledAndNativeExhausted` (`INFRA_RETRY_DISABLED`, `NATIVE_RETRY_EXHAUSTED`); `TestIssue502_InfraReservationAcrossRestart` (restart before spawn republishes the same identity with no extra charge; a possible spawn holds `INFRA_RETRY_UNKNOWN` and launches nothing; after slot churn the identity launches in its reserved slot; a roster that now selects another role holds `INFRA_RETRY_UNKNOWN` instead of a new identity; a widened reload changes neither limit nor deadline, a narrowed one holds, an absent policy keeps the debt); `TestIssue502_InfraReservationSavedBeforeSpawn` (at the launch fence, before any spawn, the saved ledger holds the reservation with its identity, charge and deadline; interrupted there, a reopen launches that identity with no extra charge); `TestIssue502_InfraReservationSaveFailure` (an unsaved reservation launches nothing and the next tick reuses its identity and charge); `TestIssue502_InfraRecoveredAndNewAcceptance` (checked progress marks RECOVERED keeping the debt; a new acceptance revision drops the old episode); `TestIssue502_LedgerRefusesMalformedInfraRetry` (strict round trip; alias, duplicate, null, unknown or aliased member, bad state, RUNNING without launch, charged above 10 or above sessions, no sessions, bad key, noncanonical acceptance and an empty map refused; every member null and every member but launch omitted refused; a waiting, reserved or running retry without a charge or deadline, and a waiting one above its limit, refused; idle and disabled episodes load); CLI `TestIssue502_DispatchStatusShowsInfraRetry` | a live dispatcher over a native store with a real worker raising the request, a process kill between reservation and spawn |
| ESC-V0-008 | ESC502-008 | native observation; dispatch classification and fingerprint; 499 tiers | `TestIssue502_TypedDispositionAndWorkRevision` (control vs work revision); CLI `TestIssue502_ObserveNamesInfrastructureHolders` (the observation names the holder of a current infrastructure request, a decision request is a hold and not an infrastructure holder, and an escalate write leaves the observed revision unchanged); `TestIssue502_InfraClassificationNeedsTheHoldersOwnRequest` (another holder's request, an absent policy and UNKNOWN material are ordinary no-progress and park); `TestIssue502_HeldSessionIsNotFailureParked` (no count, no ladder step, session `held`); `TestIssue502_HoldOutranksInfrastructure` (a retry that raised both an infrastructure and a decision request is `held`: no charge, no exhaustion hold, and it launches once answered); `TestIssue502_InfraSessionKeepsTierAndResetsSuffix` (mixed 499 witness: two failures, an infrastructure session resets the streak to the selected tier's threshold, and two more failures climb only after the full threshold) | answer and refine in one sequence on a live dispatcher |
| ESC-V0-009 | ESC502-009 | CLI reads; dispatch status | CLI `TestIssue502_EscalationCLIWritesAndReads` (list filters by kind, state, target and program, `program` UNKNOWN with a warning for a named program, origin-digest cursor and offset, limit 51 refused; show with answer and blockedBy, missing request refused; history newest first with a chain cursor; the state directory is byte-identical across every read; a deleted origin marks list entries UNAVAILABLE and refuses show `MISSING_EVIDENCE`; history rows carry the question's age and source; a rewritten answer event refuses show and history `JOURNAL_FORKED`) | the `dispatch status` section, a clock-behind witness, the 1 MiB page cut, a forked history chain |
| ESC-V0-010 | ESC502-010 | transaction/stage/material/redo | `TestIssue502_ImmutableClaimAnswerSelection` (rehashed material mismatch); `TestIssue502_SupersessionAndCapacity` (missing and mismatched supersession pair, single atomic proposal); `BenchmarkIssue502_ApplyNearCapacity` (reducer cost at 63 answered questions); `TestESCV0010_EscalationStageNarrow` and `TestTMV0002_AS10_StageCodecActualMaxima` (the `ESCALATION` shape refuses zero or three events, an attempt or reservation post, an event off its content address, a blob that is not the ticket, a missing ticket and an oversized event; measured maximum 7 artifacts and 1879 bytes); native `TestIssue502_SupersedePostsBothEvents` (one transaction stages as `ESCALATION` and posts the ticket and both events within the `ESCALATION` and StageLease bounds; replay after a later question returns the original OPEN), `TestIssue502_InterruptedSupersessionRedoes` (a fault at every published artifact of a supersession, then a retry that commits once and redoes exactly when the receipt was published), `TestIssue502_RedoRepublishesTheTicket` (crash point C2 redo, then replay), `TestIssue502_DeletedEventIsJournalDamage` (a missing event refuses `JOURNAL_FORKED`), `TestIssue502_ForgedEventRefusesAsJournalDamage` (a self-consistent forged time, question or actor passes the generic journal audit but the binding fold refuses it settled, and refuses the pending redo `JOURNAL_FORKED` leaving head and ticket unchanged), `TestIssue502_EscalationBindingFoldRefusals` (a reference change outside a TRANSITION, no request entry, no recoverable grant, a source that is not a retained admission, an extra post, a source attempt removed, no matching reservation, an expired lease, a dropped reference, and an unreferenced event blob accepted) and `TestIssue502_SupervisedSinceClaimFoldRefuses` (supervision posted after the claim) | a live-store integrated witness, a ticket over the inline POST bound taking the blob slot, a supervised generation followed by an unsupervised one of the same attempt ID |
| ESC-V0-011 | ESC502-011 | four-file foundation; layering | `TestIssue502_EscalationCodecAndBounds`, `TestIssue502_AdmissionOriginAndStaleGeneration`, `TestIssue502_QuestionAnswerCASAndReplay`, `TestIssue502_TypedDispositionAndWorkRevision`, `TestIssue502_ImmutableClaimAnswerSelection`, `TestIssue502_SupersessionAndCapacity`, `TestIssue502_StaleOpenReleasesCapacity`, `TestIssue502_ReadersRefuseOpenOverflow`, `TestIssue502_AnswerGrantAndEmptyShorthand`; `ticket` imports only `wire` | full integration matrix, independent integrated review, native completion |

## Unresolved decisions

The owner accepted these Gate A decisions as written on 2026-10-04 (decision 0428): admitted-receipt
invocation context for the shorthand with NOT_OBSERVED authentication; one current event per
question rather than one question per ticket; sole-open-at-commit shorthand next to exact CAS;
blocked reason defaulting to the explicitly typed question; infrastructure exhaustion as a distinct
visible automation hold; the typed-control workRevision exclusion; and the optional-reference
capacities, including the 16-open bound counting only the current acceptance revision, with a
specialized StageLease shape. On 2026-10-05 the owner also accepted the TCP-00 §11 amendment adding
`ESCALATION_PENDING` (72 codes) and its reader-compatibility cost (V1-0699 receipt 2749); see
Acceptance evidence and traceability. Kill criterion: if the typed-event stage cannot fit
existing StageLease bounds, revise this contract rather than widen the bounds or bypass them.

Findings from the independent review of the first delivery and their disposition:

- Stale OPEN questions locked capacity. Fixed in the pure source by counting only same-acceptance
  OPEN questions toward the 16 bound (agent decision, 2026-10-04, accepted with the Gate A
  capacities by decision 0428); the alternative, an explicit retirement route, adds a mutation with its own
  authority and was not chosen. Stale questions stay visible and still count toward the 64
  lifetime entries, so no history is evicted.
- One supplied policy decision covers every operation. Fixed in both layers: in the pure source,
  `EscalationObservation.PolicyOperation` names the one operation a grant covers and a mismatch
  refuses `POLICY_NOT_ALLOWED`; the writer supplies the operation of its lease verb and checks an
  operation-scoped grant, `ESCALATE` for OPEN and `ANSWER` for ANSWER, from the policy's
  role row or the default matrix, where only OWNER holds them. A source holder answers only with
  its own role's `ANSWER` grant. OPERATOR gets either verb only through an explicit
  `policy.roles.OPERATOR` row that names it (the issue 559 explicit grant list, ON-V0-004's
  mechanism); no other non-owner role may name them. Under the default matrix the only role that
  can escalate is OWNER, which also holds `ANSWER`, so an OWNER-bound claimer can answer its own
  question (as ESC-V0-004 permits). Separating workers from answerers needs a policy that narrows
  OWNER or grants OPERATOR explicitly.

Findings from the writer slice and their disposition:

- ESC-V0-010 was not delivered at the stage level by the writer slice: its event POSTs used the
  lease stage's evidence allowance and only the planner checked the material. The typed-event
  stage slice closes this (below).
- The actor is part of the request digest, so the store binds the invoking actor to the request
  before its replay lookup: another actor retrying a committed request gets `ACTOR_BINDING`, not
  `REQUEST_ID_CONFLICT`.
- A program attaches supervision after the claim, so the claim receipt's POST attempt is never
  supervised. The writer also checks the current attempt and refuses `UNSUPPORTED` when either is
  supervised.
- Events are retained evidence published before the commit point. A redo republishes the ticket
  and head and reads the events back; a missing event file is journal damage (`JOURNAL_FORKED`),
  not something the redo recreates.
- Events are written in receipt POST order, which follows their digests, so a supersession's
  OPEN and SUPERSEDE events are not in operation order.
- No native witness yet: the `MaxTicketFileBytes` overflow path (`CAPACITY_EXHAUSTED`/
  `LIMIT_EXCEEDED`); `STALE_ADMISSION` for a reclaimed generation, holder or reservation; policy
  narrowing of `ESCALATE` and `ANSWER`; and a stage whose ticket exceeds the inline POST bound and
  so takes another evidence slot (the measured supersession used a small ticket).
- Refusals of ambiguous shorthand now name the open questions in the reducer
  (`EscalationRefusal.RequestIDs`); shorthand with none refuses `NO_OPEN_QUESTION` rather than
  `QUESTION_NOT_FOUND`. Rendering them is left to the CLI slice.
- Reducer refusal tests now assert exact codes through `EscalationRefusal.Code`. Ticket codec
  refusals are plain errors and still assert only that they refuse.
- Each call decoded every blob several times. Measured with `BenchmarkIssue502_ApplyNearCapacity`
  on a loaded 12-core host (load average about 7 to 8): 57 ms, 43 MB and 526k allocations per call
  before a per-call decode memo, 14 ms, 12 MB and 143k allocations after it. The memo never
  outlives one exported call, so a blob changed between calls is re-verified. A quiet-host figure
  is NOT_RUN.

Findings from the dispatcher retry slice and their disposition (agent decisions, 2026-10-05,
within ESC-V0-007 and ESC-V0-008 as written):

- Without the `infrastructureRetry` policy the dispatcher keeps its legacy accounting: an
  infrastructure session is ordinary no-progress and parks. Recorded episodes stay in the ledger,
  inert, so removing and re-adding the policy cannot refill debt.
- A session is an infrastructure session only when the native observation names its own worker ID
  as the `Source.Holder` of a current OPEN or ANSWERED infrastructure request at the ticket's
  acceptance revision. The dispatcher's `{holder}` is the worker ID, so each ended session counts
  once however many earlier requests stay listed, and another holder's request is no-progress.
- Any current typed hold on the ticket when its session ends makes the session `held`: it is not
  counted, parked or stepped on the 499 ladder, whether or not the policy is configured. The hold
  outranks the session's own infrastructure request, so such a session spends no retry and leaves
  no infrastructure hold behind the owner's answer.
- Unreadable escalation material makes the observation UNKNOWN. The session is then neither
  progress nor infrastructure: it is accounted as ordinary no-progress, the dispatcher alerts, and
  progress detection and automatic unparking treat the ticket as UNKNOWN.
- `INFRA_RETRY_EXHAUSTED`, `INFRA_RETRY_DISABLED`, `NATIVE_RETRY_EXHAUSTED` and `INFRA_RETRY_UNKNOWN`
  are dispatcher event and status labels, not wire detail codes, so the TCP-00 code set is
  unchanged. A hold emits `needs-owner` naming `dispatch unpark`; an unpark releases the hold and
  keeps sessions, charges and limit, so the next infrastructure session holds again at once.
- No-spawn is proved by the absence of the reserved worker's log directory, which launch creates
  before it starts the process. A reservation with the directory present and no recorded worker
  is UNKNOWN and never replaced automatically. A republish keeps the reserved role and slot: it
  waits while that slot is in use, and if the roster now selects another role the ticket holds
  `INFRA_RETRY_UNKNOWN` rather than launching under a new identity.
- The ledger reader requires every episode member but `launch` and refuses nulls, and a waiting,
  reserved or running retry must carry its charge and deadline, so a damaged ledger cannot decode
  as spendable debt or an elapsed cooldown.
- One retry per ticket runs at a time, shared across roles. A reload lowers the limit to the
  configured `maxRetries` and holds a waiting ordinal above it; it never raises the limit or moves
  a recorded deadline.
- Episodes of tickets the observation no longer has, and of an older acceptance revision once
  nothing of theirs runs, are dropped. The ledger holds at most 8192 episodes; past that an
  infrastructure session alerts and is accounted as ordinary no-progress.
- For the 499 tiers an infrastructure session lowers the failure streak to the largest selected
  tier threshold across roles, so every role keeps the tier it would launch at.

Findings from the typed-event stage slice and their disposition (agent decision, 2026-10-05,
under the owner's in-task delegation; the owner may redirect):

- A distinct closed `ESCALATION` stage operation, rather than more lease-stage allowances, keeps the
  LEASE shape unchanged and makes an escalation's stage admit nothing but its own artifacts. The
  writer, its LEASE request digest and native supersession are unchanged. Only a committed
  ESCALATE or ANSWER selects the operation; its refusals are unrecorded and never stage.
- The native store never persists the descriptor's operation: it lives in the frozen plan and is
  validated when the plan freezes. Rollout and rollback therefore turn on in-flight plans only.
- The material branch is the receipt binding fold (`EscalationReceiptAudit`), folded with the
  external-review fold in one pass for receipt audit and both redo paths. Its memory is the ticket
  projection plus one source per completed ADMIT receipt and the supervised attempt IDs. A claim
  admission's queue is read from the admitted attempt, because ADMIT receipts name no ticket.
- The lease-path redo (`redoLeaseProof`) skipped the binding fold that the mutation redo runs, so
  a self-consistent forged event in an interrupted escalation would have been republished. It now
  runs the same fold before republishing posts.
- An OPEN's source must still be current where the receipt sits in history, as the writer
  observes it: the latest folded attempt is the same generation, holder, ticket record and
  acceptance, live, unsupervised, unexpired at the receipt's time, and matched by the latest
  folded reservations. Supervision is read from that latest attempt, as the writer reads it, so a
  later unsupervised generation of the same attempt ID is not refused for an earlier one.
- A bound transition posts nothing but its request entry, its one ticket and the replayed events.
  An event blob that no reference change binds is accepted as opaque evidence: every referenced
  event is replayed from the reducer, so such a blob installs nothing, and a gate may legitimately
  capture identical bytes (an earlier draft refused it, which falsely refused that history).
- Events are reported in receipt POST order, which is digest order, so tests compare event sets.
- The fold does not re-observe a blocked-by relation; the writer checks it before commit.

### Integration slot

Superseded in part: decision 0428 item 7 moved the writer to the LEASE pipeline with supersession,
and the typed-event stage slice gives it the closed `ESCALATION` stage (ESC-V0-010). The original
owner decision 2026-10-04 read: the native escalation writer reuses issue 501's single derived-event
MUTATE slot, and supersession is deferred (ESC-V0-010). That slot admits exactly one derived
evidence event per MUTATE, measured at 6 of 6 artifacts and 1669 of 1670 descriptor bytes. OPEN
and ANSWER fit that one-event shape; paired supersession (ESC-V0-003) does not, and it waits for a
distinct typed-event stage within StageLease bounds. Writer integration starts after the 501 slot
reaches main, so the slot is adopted rather than copied. The writer must declare the derived
event for its own operations in the transaction model rather than rely on stage shape alone, bind
the event in receipt audit and redo (NOT_RUN until implemented), and refuse the `escalations`
reference in raw import (`importChain`) as well as in CREATE, REFINE and ADOPT.

## Rollout and rollback

This delivery adds unreferenced pure code and intent only. It installs no feature and changes no
existing byte format, so rollback is reverting the four files and this spec's catalog entries.
The record-key slice adds the optional `escalations` record key. No writer sets it, and a record
without it keeps its exact bytes, so rollback is reverting that change; a store holding a record
with the key would then refuse it as an unknown key. That cannot happen before the writer slice.
The writer slice adds the store entry points, the LEASE planner and the `ESCALATE`/`ANSWER` grants.
It is the first change that can write the key; rolling it back reverts only the writer files and
the two grants, and leaves the record-key slice in place so readers keep accepting a record that
already carries a reference. The grants also widen the policy vocabulary: the operation enum and
OPERATOR's explicit grants. Full journal audit decodes every historical policy POST, so once an
accepted policy names `ESCALATE` or `ANSWER`, a binary without that vocabulary fails audit even
after a later policy drops them. Before any policy names them, a plain revert is safe. After that,
rollback keeps the vocabulary and the readers and disables only the writer entry points.
The holds slice adds the `ESCALATION_PENDING` code and derives it on read and claim paths without
writing anything; rollback is reverting that change, after which Core's planner again blocks such a
ticket as `TICKET_STATE` and native claims stop honouring the hold. Its dispatcher ledger key
`seen.escalations` is optional; a dispatcher built before it refuses any ledger that carries it, so a
downgrade stops that program's dispatcher, backs up its `state.json`, removes only that one member
(for example `jq 'del(.seen.escalations)'` into a temporary file renamed over it), and confirms the
older binary's `dispatch status` loads it before restarting. Workers, backoff, parking, cooldown,
progress, pool sweeps, pressure and escalation tiers are kept; never delete the ledger itself.
The CLI slice adds `ticket escalate`, `ticket answer` and the three reads over the writer and adds no
byte format; rolling it back removes the verbs and leaves the writer and every stored question
readable by the store.
The claim delivery slice adds the optional `escalationAnswers` attempt key and the claim result
field. A no-answer attempt keeps its exact bytes. Before any claim pins an answer, rollback is a
plain revert. After that, journal audit decodes the pinned attempt POSTs, so rollback keeps the
attempt codec and stops only pinning and delivery.
The dispatcher retry slice adds the optional `infrastructureRetry` config member and the optional
`infraRetry` dispatch ledger member, read strictly. A dispatcher built before it refuses a config
naming the policy and a ledger carrying episodes. Downgrade by removing the policy from the config,
stopping that program's dispatcher, backing up its `state.json` and removing only `infraRetry` as for
`seen.escalations`; the backup keeps the retry debt for a later upgrade, and the older binary
parks those tickets by ordinary no-progress accounting instead.
The typed-event stage slice adds the `ESCALATION` stage operation and the receipt binding fold.
No stored byte format changes: the native store never persists the stage operation, so receipts,
posts and an interrupted write's journal are the same bytes under either binary, and an older
binary recovers them through its own generic redo. Downgrade at any time is therefore safe but
drops the fold's checks from audit and redo. Escalations committed before this slice replay through
the same fold, because it reads receipts, not stage operations. Rollback is reverting the change.
Later slices rebase onto current main and integrate in order: codecs, writer and material, holds,
CLI and reads, 501 claim delivery, then dispatcher retry with 499 composition. Once writes exist,
rollback disables new mutations and automation but keeps readers, references, questions, answers
and admission uncertainty visible. It never deletes history, resets debt or turns an unresolved
request into no request.
