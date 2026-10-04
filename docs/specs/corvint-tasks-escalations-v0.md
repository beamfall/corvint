# Corvint Tasks typed escalations V0

Owner: Russell Lewis
Date: 2026-10-04
Intent status: proposed
Delivery status: experimental

Authoritative inputs: owner request [issue 502](https://github.com/beamfall/corvint/issues/502)
(native ticket V1-0699); the issue's ESC502-001..011 preparation intent with the three precision corrections adopted
during its independent preparation review (not owner acceptance); the existing
[agent lease contract](corvint-tasks-agent-leases-v0.md); and two neighbouring intents this one
composes with but does not restate: issue 501's operator notes (claim-snapshot delivery) and issue
499's dispatch model-escalation tiers. No owner acceptance of this intent has been recorded. The
owner-delegated decisions of 2026-10-04 adopt the 500 and 501 intents only. Every requirement below
is therefore proposed, and the Gate A decisions listed under Unresolved decisions still need
explicit acceptance.

## Agent digest
- Claim: Workers raise typed questions from admitted claims, operators answer them by compare-and-set, and the next same-acceptance claim receives the answers.
- Status: proposed intent; experimental delivery of the pure four-file foundation only; no installed escalation capability.
- Exists: closed request/event/reference codecs and a pure reducer with answer selection, derived holds, effective work revision, coded refusals and a per-call decode memo, plus focused tests and a near-capacity benchmark. Nothing calls them yet.
- Blocked on: owner acceptance of the Gate A decisions; native writer/stage/material/replay integration, CLI, native holds, the 501 claim-snapshot path, dispatcher retry and 499 tier composition.
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
  An ended or expired source attempt does not close an unanswered same-acceptance question. An acceptance-changing mutation leaves old requests visible as STALE, removes them from current holds and claim guidance, and never promotes an old answer to the new scope. Unrelated content changes keep them current. Explicit supersession, `--supersedes Q --expected-request-revision N`, replaces one OPEN question atomically: the old request gains one SUPERSEDE event, the replacement starts at revision 1, two event slots and one transaction are charged, and ticket content increments once. The worker route requires the same admitted source. A worker cannot supersede an answered question or another source's question, and the administrative cross-source route stays unsupported until separately admitted. Supersession is never inferred from similar text.

- `ESC-V0-004`: `ticket answer --target T --text TEXT` MUST close exactly one question, either the sole same-acceptance OPEN question at commit or an exact `--request Q --expected-request-revision N` compare-and-set.
  Shorthand with zero or several open questions refuses and names them (`AMBIGUOUS_OPEN_QUESTIONS` carries the sorted request IDs); it never answers all or the newest. The original request retains only the caller's selector, text and optional CAS. The writer-resolved request ID and prior revision live in event material, so replay of a committed shorthand answer returns its original question after later questions open. Exact mode cross-checks the derived fields against the caller's. Concurrent answers to one question have one winner; changed bytes under a reused request ID conflict. Answer text is 1..8192 UTF-8 bytes and not whitespace-only. OWNER may answer subject to policy narrowing; OPERATOR needs an explicit answer grant; a source-holder grant is not answer authority. An answer closes its request and removes only that request's hold. It approves no scope, changes no acceptance, clears no dependency, hold or budget, and selects no option.

- `ESC-V0-005`: Claim and claim-next MUST return current same-acceptance answers as an `escalationAnswers` field separate from `operatorNote`, pinned by the claim's own admission.
  Under the claim writer lock, sorted answer origin and head references are copied into an optional admitted Attempt field from the same audited ticket snapshot as TicketRecordSha256; no-answer attempts omit it. Output resolves only from the successful claim receipt's POST attempt, so a claim, a later answer and a replay still yield the original snapshot. Each answer carries request ID, original question, options and kind, source provenance, answer actor, time and head, and event revision. Delivery does not consume or acknowledge an answer, and SUPERSEDED and STALE answers are history only. Aggregate guidance is at most 256 KiB encoded and full claim output at most 1 MiB, checked before admission; an answer that would exceed capacity refuses instead of truncating. A materialization failure after commit preserves the attempt and receipt and instructs exact replay. This depends on the 501 claim-snapshot path.

- `ESC-V0-006`: A current OPEN decision, scope or blocked question MUST produce an explicit `ESCALATION_PENDING` derived admission hold, and infrastructure MUST NOT.
  The hold appears in native eligibility, direct claim, claim-next and dispatch status, so bypassing the dispatcher does not bypass it. A blocked question holds through its explicitly chosen kind and question, or through an optional validated same-queue `--blocked-by T [--gate G]` relation, which creates no real dependency. No text is classified. Holds never rewrite ticket status, set needs-human or cancel the live holder. Infrastructure is a typed observation of a failed admitted session, never native HELD or ordinary dispatcher Parked. A worker without a successful claim cannot raise one.

- `ESC-V0-007`: An optional dispatch `infrastructureRetry` policy MUST bound automatic retries per program, canonical ticket and acceptance revision, with restart-safe reservations.
  maxRetries is 0..10 (default 3 when enabled), cooldownSeconds 1..3600 (default 30) and maxCooldownSeconds is at least cooldown and at most 86400 (default 300). maxRetries 0 shows `INFRA_RETRY_DISABLED`. The episode lives in the existing dispatch ledger, with no new sidecar, and is shared across roles and request IDs so new text cannot refill budget. Each ended session counts once. A retry reserves its exact pending launch identity, charged ordinal and cooldown deadline in the same checked ledger clone before issuing a launch; republishing reuses that identity without another charge. A crash before spawn consumes the reservation unless no-spawn is proved, and an ambiguous spawn is UNKNOWN/HOLD, never an automatic replacement. Cooldown for ordinal n is min(maxCooldown, cooldown * 2^(n-1)), saturating. Config reload may narrow but cannot refill debt or recompute a reserved deadline. All ordinary native admission invariants still apply. Exhaustion shows `INFRA_RETRY_EXHAUSTED`, `NATIVE_RETRY_EXHAUSTED` or a named HOLD/UNKNOWN and leaves the native request OPEN. Checked work progress may mark the local episode RECOVERED without answering the native question; an operator answer does not prove recovery.

- `ESC-V0-008`: Session classification MUST precede ordinary no-progress parking, and typed control writes MUST NOT count as work progress.
  A session with a validated infrastructure request and no separately proved progress suppresses its no-progress count and park, keeps unrelated parked state and, for 499 tiers, counts as unknown progress: it resets only the proved failure suffix and retains the selected tier. Decision and scope sessions are held by typed policy, not failure parking. Each typed-only write sets `lastControlTicketRevision` to the post ticket revision and keeps the prior `workRevision` only when the audited pre revision equals the prior `lastControlTicketRevision`; otherwise `workRevision` becomes the audited pre revision, which also seeds the first control. Dispatch substitutes `workRevision` only while the current ticket revision equals `lastControlTicketRevision`, so an ordinary later edit stays visible. Missing or corrupt bindings are UNKNOWN, not progress.

- `ESC-V0-009`: `ticket escalation list`, show and history MUST be bounded, non-mutating native reads with honest age and provenance.
  Filters are kind, state, target and program. Pages are 1..50 (default 20) and at most 1 MiB, with cursors anchored to immutable heads and strict chain identity, decrement and cycle checks. Each entry shows original RecordedAt, nonnegative ageSeconds, clockUncertain, current or stale applicability and source generation and holder. Program appears only from an admitted producer mapping, otherwise UNKNOWN; caller text never supplies it. The native list reports retryState NOT_OBSERVED, and dispatcher retry state is labelled a dispatcher observation for an explicitly named program. `dispatch status` shows open request kinds, ages and holds separately from parked keys, retry state and 499 tiers. Reads write no ledger and hydrate no evidence.

- `ESC-V0-010`: Escalation writes MUST use the existing writer, receipt and request-index pipeline through a distinct closed typed-event stage and material branch.
  OPEN and ANSWER post one ticket and one event; supersession posts two events atomically, and an interrupted publication cannot expose one side as current. Attempts, reservations and gates are untouched. The stage shape is request, ticket POST, event POST(s), receipt and head; it gains no generic evidence-path permission, and the existing StageLease limits of 11 artifacts and 2658 descriptor bytes stay binding, with the maximal descriptor measured before promotion. Material validation checks the nested request, source receipt and grant, audited holder, generation and acceptance, expected revisions, old and new heads, content and work revision arithmetic, immutable origin, queue time and all posted bytes. Replay recovers the resolved shorthand target, answer, clock and outcome after later questions. Capacity overflow refuses before commit.

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

The first delivery supplies pure, focused evidence only. Every integrated witness is NOT_RUN, and
no row claims delivered native behavior.

| Requirement | Parent intent | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| ESC-V0-001 | ESC502-001 | CLI producer; receipt/grant adapter; writer fences | `TestIssue502_EscalationCodecAndBounds` (question/options bounds); `TestIssue502_AdmissionOriginAndStaleGeneration` (missing context, stale generation/holder/receipt/acceptance, expiry, actor and policy binding, unknown replay) | real claim receipt, forged/wrong-queue origins, compiled CLI, supervised UNSUPPORTED |
| ESC-V0-002 | ESC502-002 | ticket record and Core codecs; evidence store | `TestIssue502_EscalationCodecAndBounds` (closed shapes, noncanonical framing, question bound, 64-revision history cap; the 65536-byte event cap is untested); `TestIssue502_SupersessionAndCapacity` (transaction vs event counts); `TestIssue502_StaleOpenReleasesCapacity` (16-open bound counts the current acceptance revision only); `TestIssue502_ReadersRefuseOpenOverflow` (readers refuse a crafted 17-open reference; the capacity check refuses an invalid or older acceptance argument) | legacy bytes through the record codec, import/CREATE injection refusal, Core reader |
| ESC-V0-003 | ESC502-003 | reducer; native writer | `TestIssue502_SupersessionAndCapacity` (paired supersession, last-slot refusal, cross-source `SUPERSESSION_SOURCE`); `TestIssue502_ImmutableClaimAnswerSelection` (stale acceptance excluded); `TestIssue502_StaleOpenReleasesCapacity` (stale OPEN stays visible, holds nothing, refuses an answer) | acceptance edit yields STALE across show/plan/claim; administrative route |
| ESC-V0-004 | ESC502-004 | reducer; native writer; CLI | `TestIssue502_QuestionAnswerCASAndReplay` (ambiguous shorthand naming its open questions, exact CAS, Q1-answer then Q2-open shorthand replay, changed-selector conflict) | real writer race with one winner, compiled CLI |
| ESC-V0-005 | ESC502-005 | lease admission; original-receipt materializer | `TestIssue502_ImmutableClaimAnswerSelection` (pinned selection, guidance capacity) | original claim snapshot after concurrent answers, missing-receipt replay, 1 MiB output |
| ESC-V0-006 | ESC502-006 | native eligibility; dispatch status | `TestIssue502_TypedDispositionAndWorkRevision` (four kinds, held vs infrastructure IDs) | hold parity across direct claim, claim-next and plan |
| ESC-V0-007 | ESC502-007 | dispatch ledger, loop and status | none | restart before/after spawn, failed save, duplicate session, final allowed and next exhausted launch, cooldown caps |
| ESC-V0-008 | ESC502-008 | dispatch roster/fingerprint; 499 tiers | `TestIssue502_TypedDispositionAndWorkRevision` (control vs work revision) | refine/escalate/answer sequence, mixed 499 tier witness |
| ESC-V0-009 | ESC502-009 | CLI reads; dispatch status | none | pagination, cursors, age and clock, program UNKNOWN, reads do not mutate |
| ESC-V0-010 | ESC502-010 | transaction/stage/material/redo | `TestIssue502_ImmutableClaimAnswerSelection` (rehashed material mismatch); `TestIssue502_SupersessionAndCapacity` (missing and mismatched supersession pair, single atomic proposal); `BenchmarkIssue502_ApplyNearCapacity` (reducer cost at 63 answered questions) | measured descriptor, interrupted paired publication, redo |
| ESC-V0-011 | ESC502-011 | four-file foundation; layering | `TestIssue502_EscalationCodecAndBounds`, `TestIssue502_AdmissionOriginAndStaleGeneration`, `TestIssue502_QuestionAnswerCASAndReplay`, `TestIssue502_TypedDispositionAndWorkRevision`, `TestIssue502_ImmutableClaimAnswerSelection`, `TestIssue502_SupersessionAndCapacity`, `TestIssue502_StaleOpenReleasesCapacity`, `TestIssue502_ReadersRefuseOpenOverflow`; `ticket` imports only `wire` | full integration matrix, independent integrated review, native completion |

## Unresolved decisions

These Gate A decisions need explicit owner acceptance before integration: admitted-receipt
invocation context for the shorthand with NOT_OBSERVED authentication; one current event per
question rather than one question per ticket; sole-open-at-commit shorthand next to exact CAS;
blocked reason defaulting to the explicitly typed question; infrastructure exhaustion as a distinct
visible automation hold; the typed-control workRevision exclusion; and the optional-reference
capacities, including the 16-open bound counting only the current acceptance revision, with a
specialized StageLease shape. Kill criterion: if the typed-event stage cannot fit
existing StageLease bounds, revise this contract rather than widen the bounds or bypass them.

Findings from the independent review of the first delivery and their disposition:

- Stale OPEN questions locked capacity. Fixed in the pure source by counting only same-acceptance
  OPEN questions toward the 16 bound (agent decision, 2026-10-04, pending the Gate A capacity
  acceptance above); the alternative, an explicit retirement route, adds a mutation with its own
  authority and was not chosen. Stale questions stay visible and still count toward the 64
  lifetime entries, so no history is evicted.
- One supplied policy decision covers every operation. Still open: the reducer does not refuse an
  answer by the question's own source holder, so the answer-grant separation of ESC-V0-004 must be
  enforced by an operation-scoped grant in the writer slice.
- Refusals of ambiguous shorthand now name the open questions in the reducer
  (`EscalationRefusal.RequestIDs`); rendering them is left to the CLI slice.
- Reducer refusal tests now assert exact codes through `EscalationRefusal.Code`. Ticket codec
  refusals are plain errors and still assert only that they refuse.
- Each call decoded every blob several times. Measured with `BenchmarkIssue502_ApplyNearCapacity`
  on a loaded 12-core host (load average about 7 to 8): 57 ms, 43 MB and 526k allocations per call
  before a per-call decode memo, 14 ms, 12 MB and 143k allocations after it. The memo never
  outlives one exported call, so a blob changed between calls is re-verified. A quiet-host figure
  is NOT_RUN.

## Rollout and rollback

This delivery adds unreferenced pure code and intent only. It installs no feature and changes no
existing byte format, so rollback is reverting the four files and this spec's catalog entries.
Later slices rebase onto current main and integrate in order: codecs, writer and material, holds,
CLI and reads, 501 claim delivery, then dispatcher retry with 499 composition. Once writes exist,
rollback disables new mutations and automation but keeps readers, references, questions, answers
and admission uncertainty visible. It never deletes history, resets debt or turns an unresolved
request into no request.
