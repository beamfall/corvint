# Corvint Tasks agent leases V0

Owner: Russell Lewis
Date: 2026-09-27 (accepted the same day)
Intent status: accepted (owner decision 2026-09-27)
Delivery status: partial (S1 CAL-V0-001..003, S2 CAL-V0-004..006, S3 CAL-V0-007 and 009..013, S4 CAL-V0-008 and 014, S5 CAL-V0-015..017 and 024, S6 CAL-V0-018 partial (audit carried; proportional cost and load condition NOT_MET), S7 CAL-V0-019..020, S8 CAL-V0-021..023 and 025 experimental with explicit pack opt-in; CAL-V0-026 MET (GOMAXPROCS=2 qualification); CAL-V0-027 implemented with scoped native release qualification; S9 CAL-V0-028..034 implemented with local native qualification; S10 CAL-V0-035..041 implemented with scoped local Codex qualification; CAL-V0-044 implemented with disposable fixture-profile qualification; CAL-V0-045..047 implemented with scoped disposable qualification; CAL-V0-048..051 implemented with focused local qualification and independent source review; S11 CAL-V0-052..058 implemented with local OpenCode qualification, plus Claude Code and Codex host qualification of launch, claim, handoff and summary; S12 CAL-V0-059..061 implemented with focused tests and a live-store measurement; S13 CAL-V0-062..063 implemented with focused tests, live Codex qualification NOT_RUN; issue 482 CAL-V0-044/046 experimental compatibility implemented, independently reviewed and integrated with scoped fixture qualification; native completion recorded); S15 CAL-V0-065 implemented with focused tests, a compiled native fixture and independent source review; S17 CAL-V0-067 experimental implementation with focused tests, independent review and scoped native/archive/crash fixture qualification; physical facts NOT_OBSERVED; S14 CAL-V0-064 experimental implementation with original scoped macOS checks, independent review and sealed binding; current-main integration, Linux qualification and native completion pending
Authoritative inputs: owner request [issue 482](https://github.com/beamfall/corvint/issues/482) (proposed CAL-V0-044/046 compatibility amendment);
owner requests [issue 426](https://github.com/beamfall/corvint/issues/426),
[issue 427](https://github.com/beamfall/corvint/issues/427), [issue 428](https://github.com/beamfall/corvint/issues/428),
and [issue 430](https://github.com/beamfall/corvint/issues/430), explicitly commissioned 2026-10-01 (CAL-V0-048..051); owner request [issue 342](https://github.com/beamfall/corvint/issues/342),
owner approval on 2026-09-30 of prospective handoff accounting for [issue 412](https://github.com/beamfall/corvint/issues/412) (CAL-V0-044),
owner requests [issue 420](https://github.com/beamfall/corvint/issues/420),
[issue 421](https://github.com/beamfall/corvint/issues/421) and
[issue 422](https://github.com/beamfall/corvint/issues/422) (CAL-V0-045..047),
owner request [issue 431](https://github.com/beamfall/corvint/issues/431) (CAL-V0-052..058),
owner request [issue 446](https://github.com/beamfall/corvint/issues/446) (CAL-V0-059..061),
owner request [issue 354](https://github.com/beamfall/corvint/issues/354) (CAL-V0-062..063),
owner request [issue 378](https://github.com/beamfall/corvint/issues/378),
owner request [issue 370](https://github.com/beamfall/corvint/issues/370), and
owner choice on 2026-09-28 to quarantine environments until confirmed safe reuse; owner request [issue 336](https://github.com/beamfall/corvint/issues/336), the Corvint Tasks contract TCP-00 (`beamfall/corvint-tasks` `docs/SPEC.md`,
§3.4, §4, §6 and §7.4), decision 0397 (corvint-tasks built in tree), decision 0423 A10,
`docs/specs/corvint-tasks-store-init-v0.md`, tickets V1-0398, V1-0184 and V1-0310, and the in-tree
sources under `internal/tasks`.

## Agent digest
- Claim: Agents claim, gate and complete scoped Tasks attempts through external leases or an explicitly enabled Codex supervisor.
- Status: accepted (owner decision 2026-09-27); partial (S1 CAL-V0-001..003, S2 CAL-V0-004..006, S3 CAL-V0-007 and 009..013, S4 CAL-V0-008 and 014, S5 CAL-V0-015..017 and 024, S6 CAL-V0-018 partial (audit carried; proportional cost and load condition NOT_MET), S7 CAL-V0-019..020, S8 CAL-V0-021..023 and 025 experimental with explicit pack opt-in; CAL-V0-026 MET (GOMAXPROCS=2 qualification); CAL-V0-027 implemented with scoped native release qualification; S9 CAL-V0-028..034 implemented with local native qualification; S10 CAL-V0-035..041 implemented with scoped local Codex qualification; CAL-V0-044 implemented with disposable fixture-profile qualification; CAL-V0-045..047 implemented with scoped disposable qualification; CAL-V0-048..051 implemented with focused local qualification and independent source review; S11 CAL-V0-052..058 implemented with local OpenCode qualification, plus Claude Code and Codex host qualification of launch, claim, handoff and summary; S12 CAL-V0-059..061 implemented with focused tests and a live-store measurement; S13 CAL-V0-062..063 implemented with focused tests, live Codex qualification NOT_RUN; issue 482 CAL-V0-044/046 experimental compatibility implemented, independently reviewed and integrated with scoped fixture qualification; native completion recorded); S15 CAL-V0-065 implemented with focused tests, a compiled native fixture and independent source review; S17 CAL-V0-067 experimental implementation with focused tests, independent review and scoped native/archive/crash fixture qualification; physical facts NOT_OBSERVED; S14 CAL-V0-064 experimental implementation with original scoped macOS checks, independent review and sealed binding; current-main integration, Linux qualification and native completion pending. Drafted and accepted 2026-09-27 on the owner's request to bring corvint-tasks to a level where it can take over Beamfall's `script/roadmap.sh`.
- Exists: the TCP-00 attempt, reservation and receipt shapes (reserved, no writer), the §5.2 writer for fixture and non-fixture queues, and the CTS-V0-003 shadow import.
- Blocked on: the recovered task-store contract (V1-0310) for the parts of TCP-00 this spec does not restate.
- Read next: Slices; Requirements (S8 for parallel claims; S9 for named pools; S10 for Codex supervision; S11 for the continuous dispatcher; S12 for read cost; S13 for supervised effort and stage wall; S14 explicit command progress; S15 explicit exclusions; S17 proposed operator-attested untouched release); Amendments to TCP-00; Failure modes.

## User and boundary

Beamfall's `script/roadmap.sh` is a 14,532-line Bash runner over Markdown roadmap shards. Claude and
Codex sessions each run the same loop through it: pick the next ready ticket, claim it with a
`mkdir` lease, work in their own worktree, run the repository gate engine, merge when safe and check
the ticket off; `reap` frees leases whose holder crashed. It is the only sanctioned writer of those
shards (Beamfall `AGENTS.md`).

corvint-tasks already holds the ticket inventory, dependencies, holds, releases, roadmap views and a
shadow import of the Beamfall export. TCP-00 defines the rest of the execution model around a
supervisor: `admit` reserves the ticket, a supervisor forks a `lane-leader`, a `.boot`/`.ack`
handshake proves whether the runtime ran, and process-group liveness decides when a reservation may
be released (§6.2 to §6.4). None of that is built in tree: the only reservations are S3's
`external-agent` leases, `cutover` requires an empty reservation set
(`internal/tasks/transaction/model.go:580@afae0d34`), and until S1 the writer refused every queue
that was not a fixture.

The agents that use these queues are not processes corvint-tasks starts. They are interactive or
orchestrated sessions that call the task tool themselves. This spec keeps TCP-00's attempt,
generation, reservation and receipt records and replaces only the spawn and liveness layer: the
calling agent is the runtime, and a lease it renews stands in for process liveness. A generation
number fences every later command from a holder that lost its lease, so a stale agent can go on
editing its own worktree but can neither move its attempt nor complete the ticket.

External-agent non-goals (S10 explicitly qualifies only its own Codex children): a supervisor, `lane-leader`, process-group signalling of external agents or any §6.4 spawn effect; creating,
removing or inspecting worktrees; budgets beyond reporting them `NOT_OBSERVED`; review lanes (§7.2)
and the completion-manifest reducer beyond the tree and gate check in CAL-V0-016; fanout (TCP-07) and
routing (TCP-08); the import-map writer; automatic reaping by anything other than an invoked command
(no daemon, no timer); any network use; and any change to another repository. Switching Beamfall's
agents and `roadmap.sh` onto these verbs is a separate, owner-run step in the Beamfall repository
(TCP-09), taken only after S7 below.

## Slices

Each slice is independently reviewable and lands in order; a later slice never weakens an earlier
one.

| Slice | Requirements | Delivers |
|---|---|---|
| S1 | CAL-V0-001..003 | Durable writes to a non-fixture queue (resolves V1-0398) |
| S2 | CAL-V0-004..006 | `cutover`: one authority switch publishes the imported shadow records |
| S3 | CAL-V0-007..013 | `claim`, `renew`, `release`, `reap` and `attempt show` |
| S4 | CAL-V0-014 | `plan preview` (`taskman-priority-first/0`, read-only) |
| S5 | CAL-V0-015..017 | `submit`, `gate run` and `complete` |
| S6 | CAL-V0-018 | Linear first import |
| S7 | CAL-V0-019..020 | Lease race and crash qualification, and the execution cutover record |
| S8 | CAL-V0-021..026 | Parallel claims: scoped claims, path-overlap collisions, scope enforcement, bounded lock hold |
| S11 | CAL-V0-052..058 | `dispatch`: continuous roster, supervised host workers, handoff, reap, backoff and events |
| S12 | CAL-V0-059..061 | Read cost independent of receipt history: one audit per read, resumed from a writer-retained checkpoint |
| S14 | CAL-V0-064 | Proposed explicit command progress; original reviewed source and sealed evidence retained, current-main composition pending |
| S15 | CAL-V0-065 | Opt-in explicit per-claim member exclusions; focused tests and a compiled native fixture |
| S17 | CAL-V0-067 | Experimental operator-attested untouched release; scoped native/archive/crash fixtures passed, physical facts NOT_OBSERVED |

CAL-V0-062/063 (S13, issue 456) and CAL-V0-064 (S14, issue 468) are reserved
by coordinated unlanded work; CAL-V0-066/S16 remains reserved for issue 464 if used.
The coordinator assigned CAL-V0-067/S17 to issue 479. This seed does not claim
implementation or qualification of the new profile or any reserved slice.

S8 was added by owner decision on 2026-09-27 and lands directly after S3, before S4.

Corvint's own queue is a fixture queue written daily through the same §5.2 writer, and a fixture
queue admits in mode `DEVELOPMENT` without an execution cutover (TCP-00 §4.1 step 2). S2 to S6
therefore work on a fixture queue, and they land first, before S1. A repository can switch to them
in `DEVELOPMENT` mode, as Corvint runs today. S1 and S7 then turn that queue into a qualified,
non-fixture one. Whether a repository switches before S1 and S7 is an owner decision for that
repository.

## Requirements

S1, non-fixture writer.

- `CAL-V0-001`: The §5.2 writer MUST apply ticket mutations, `import`, `pause`, `unpause` and
  `policy update` to a queue whose `fixture` is false and whose `importMapSha256` is null, under
  exactly the checks it applies to a fixture queue. A queue with a non-null `importMapSha256` stays
  refused.
- `CAL-V0-002`: A non-fixture queue MUST refuse `claim` until its `executionCutover` names an owner
  decision and a `QUALIFICATION` receipt for the CAL-V0-019 suite exists at or before the head
  (CAL-V0-020). A fixture queue keeps `mode` `DEVELOPMENT` and needs neither.
- `CAL-V0-003`: Fixture queues MUST behave byte-for-byte as before S1: the same receipts, outcomes
  and codes for the same inputs.

S2, cutover of imported records.

- `CAL-V0-004`: `corvint-tasks cutover --decision <ref>` MUST, under an `OWNER` binding, commit one
  `AUTHORITY_SWITCH` receipt whose `requestId` is the decision reference and whose only post is
  `queue.json` with `canonicalWriter` `NATIVE`, a null `foreignAdapterId`, and a `CUTOVER` write
  barrier on the old source from the receipt's time (TCP-00 §5.4 A5). That receipt is the single
  publication boundary: every `IMPORT` shadow record stops reading `CUTOVER_MISSING` and is judged
  by the ordinary eligibility rules. It MUST refuse `UNAUTHORIZED` under any other role, `PAUSED`
  while a barrier is present, and `BLOCKED` once the queue is already `NATIVE`; the same decision
  reference replays.
- `CAL-V0-005`: After the switch, `import` MUST refuse and write nothing, so a later foreign export
  can never overwrite a record.
- `CAL-V0-006`: The switch MUST leave every record file byte-identical: imported records keep their
  `IMPORT` source as provenance, and their dependencies, holds, gates and completion.

S3, leases.

- `CAL-V0-007`: `corvint-tasks claim <ticketId> --holder <label> [--lease-minutes N]
  [--branch <label>] [--base <oid>]` MUST admit by TCP-00 §4.1 steps 1, 2, 5, 6 and 8 with runtime
  `external-agent`, in one transaction: a new attempt at the next queue-wide generation (TCP-00
  TM-V0-011) in phase `RUNNING`, with
  `supervisor` and `lane` null, a `lease` (CAL-V0-012), and one `ACTIVE` reservation entry. Budget
  (step 4) is `NOT_OBSERVED` and admission is refused `BUDGET_UNKNOWN` when the policy requires any
  enforced budget field. It returns `attemptId` and `generation`.
- `CAL-V0-008`: `claim --next --holder <label>` MUST claim the first `SELECTED` entry of the
  CAL-V0-014 plan computed inside the same transaction, or answer `BLOCKED` with the plan's reasons
  when none is selected.
  Because the plan reads every reservation, `claim --next` first reaps every expired lease, not only
  the colliding ones, then plans and claims the ticket under its `DECLARED` or `WHOLE_REPOSITORY`
  scope; it takes no `--scope` and derives none. With nothing selected it answers the first
  entry's reason, or `TICKET_STATE` when no ticket is `OPEN` or `HELD`.
- `CAL-V0-009`: Every command that names an attempt (`renew`, `release`, `submit`, `gate run`,
  `complete`) MUST carry `--attempt <attemptId> --generation <G>`, and a generation other than the
  attempt's current one, or an attempt in a terminal phase, MUST refuse `REVISION_CONFLICT` with
  `FENCED` and record the refusal (TM-V0-011).
- `CAL-V0-010`: `renew` MUST extend a live lease to `recordedAt + lease` and refuse `FENCED` once
  the lease has expired, even before a `reap` has recorded it.
- `CAL-V0-011`: `release [--reason <code>]` MUST move the attempt to `CANCELLED` with quiescence
  `FENCED` and remove its reservation entry. `reap` MUST move every non-terminal `external-agent`
  attempt whose lease expired at its own `recordedAt` to `FAILED` with cause `LEASE_EXPIRED` and
  quiescence `FENCED`, and remove their entries. A no-argument `reap` MUST report its receiptless
  survey separately from the completed per-attempt transactions and name each fresh child receipt;
  an empty survey retains the ordinary no-change warning. `claim` MUST reap, in its own transaction and
  receipt, every expired lease whose reservation would otherwise block it. An `ALL` barrier lets
  `release` and `reap` through as it lets `cancel` through (TCP-00 §3.4), and refuses `renew`,
  `claim` and `widen` `PAUSED`.
- `CAL-V0-012`: A lease is `{holder:label, grantedSeq:Size, expiresAt:Timestamp}`. The default is
  60 minutes, the minimum 5 and the maximum 1,440; a request outside that range is `MALFORMED`. A
  transaction of any operation whose `recordedAt` is earlier than the head receipt's refuses
  `STORAGE_FAILED` before it writes, so a clock that steps backward cannot record a receipt that
  later makes an expired lease look live.
  A writer with a live clock samples `recordedAt` again once it holds the head it plans against
  (after it takes the store lock, or after a lease preparation reads the head), and uses the later
  of that sample and its caller's. A writer that waited behind another writer's commit is therefore
  not refused as a backward step; a live clock that itself reads earlier than the head still refuses.
- `CAL-V0-013`: A ticket whose last attempt is `FAILED` or `CANCELLED` MUST be claimable again as
  that attempt's next generation while its charged retry count is below the current policy limit
  (CAL-V0-045), and after exhaustion only an `OWNER` `ticket reopen` makes it claimable,
  unless the existing prospective clean handoff exemption applies.
  CAL-V0-043 specifies readmission of an exhausted `OPEN` ticket. Cancellations consume retries
  except the prospective writer-verified clean handoffs in CAL-V0-044. Reopen creates fresh
  acceptance, not an automatic retry refund.
  `corvint-tasks attempt show <attemptId>` and `queue status` MUST report every live attempt with
  holder, phase and lease expiry, as pure reads. `queue status` reports `attempts` as the count of
  live attempts and lists them in `liveAttempts`.

### Prospective handoff accounting (issue 412)

- `CAL-V0-044`: Only an upgraded writer's verified terminal external-agent handoff MAY preserve
  the cumulative retry count on the next generation. `release --reason HANDOFF` requests this
  check for an implement/review/integrate lease; `REVIEW_RETURNED` requires a review lease. The live,
  unexpired, current generation MUST have a scope-checked submitted candidate or the explicit
  no-tree evidence branch in CAL-V0-046, no pending effects,
  unchanged acceptance, and unchanged policy/config or the complete proposed issue 482
  compatibility proof below, and prospective generation accounting without a recorded
  non-PASSED gate result. These reason strings and stage/holder changes alone are not evidence.
  A failed eligibility check MUST refuse without converting the attempt to a clean cancellation.
  Missing legacy accounting remains charged. Ordinary cancellation, failure and expiry remain
  charged regardless of stage. A clean handoff at the policy retry limit MAY continue at that count;
  a subsequent non-exempt termination MUST block the next claim. Claim, pure plan preview and
  exhausted-OPEN owner recovery MUST use the same exhaustion rule, preserving all CAL-V0-043
  safety and authorization preconditions.
  The optional closed `retryAccounting` object has profile `taskman-retry-accounting/0`, boolean
  `failedOrUnknown`, and disposition `NONE|HANDOFF|REVIEW_RETURNED`. An upgraded claim initializes
  NONE/false. Every recorded non-PASSED gate sets the boolean atomically and permanently for that
  generation; a later PASS, replacement gate result or resubmission MUST NOT clear it. Only the
  verified release writer may set a clean disposition. The next generation gets fresh local
  accounting while preserving or incrementing accumulated debt; gates, candidates, approvals and
  review evidence do not gain successor authority. Supervised attempts may retain inert metadata
  after attachment but MUST NOT receive this exemption. All records remain journal-bound, with
  original request replay, stale-generation fencing, reservation release and default pool quarantine intact.
  The proposed CAL-V0-067 exception affects only its explicitly eligible attested occupancy;
  it does not grant a retry exemption or change clean-handoff accounting.
  Legacy absent-member bytes MUST round-trip unchanged; unknown/malformed metadata MUST refuse.
  Old readers may refuse from the first accounting-bearing claim. Rollback MUST retain the journal
  and use a compatible reader/writer after stopping admissions; stripping metadata or downgrading
  an affected store is not supported. No historical refund, live migration or automatic owner reopen is authorized.
  Issues 420/421 amend only the explicit retry bound and handoff branch in CAL-V0-045/046. Verification describes recorded accounting eligibility,
  not actor authentication, unreported external failures, physical quiescence or independent review.

### Proposed issue 482 amendment: unrelated policy handoff compatibility

Status: proposed intent; experimental source implemented, independently reviewed and qualified in disposable fixtures. Final keyed qualification, integration and native completion remain pending. Authoritative human input:
[issue 482](https://github.com/beamfall/corvint/issues/482). This amendment narrows the meaning of
unchanged policy/config for clean CAL-V0-044/046 release only; it does not promote a prototype or
change completion authority. Existing installed writers retain their qualified behavior. The
experimental source and scoped candidate qualification are recorded in
`docs/build-log/2026-10-02-tasks-unrelated-policy-handoff.md`; this is not whole-delivery promotion.

1. A clean external-agent HANDOFF or REVIEW_RETURNED MAY remain eligible after policyVersion and
   other-member reservedFor changes in its exact allocated pool only. Every other raw field and
   ordered array, the own-member reservation/config/definition, acceptance, stage, generation,
   lease and existing candidate/no-tree/accounting conditions remain bound. No-pool attempts allow
   only policyVersion differences. Original attempt policy/config/capability hashes never change.
2. Compatibility MUST cover every committed policy afterimage after the exact original policy
   through the same settled fully audited head. Any relevant intermediate change stays
   incompatible after restoration. Retain one original canonical file blob and bounded metadata;
   no whole-history cache, extra wire or migration. Missing, unknown, malformed or mismatched
   history MUST refuse. Check every receipt/post/codec and projection even after incompatibility.
3. Original policy provenance MUST bind nondeleted intent/policy.json path, committed nonzero
   sequence, exact file digest including LF and containing receipt digest. The first exact
   attempt/generation post must follow it and carry matching original policy/config identities;
   renewed GrantedSeq does not substitute for that structural provenance. Historical acceptance,
   authentication, holder liveness and physical quiescence remain NOT_OBSERVED.
4. One additional full audit per otherwise eligible stale-release preparation MAY supply this
   observation under the same ChangeGuard outside the lock, after request replay handling. Bind
   both observations' head, receipt, inventory, intent and final policy identity; reject staging,
   pending redo, IntentError and drift. Bounded preparation retries are measured separately.
   Shared pure comparison uses cloned raw wire trees, preserving every unremoved optional field
   and all array/member order; only the allocated pool's empty/absent reservation map normalizes.
5. This exception MUST NOT grant stale-holder reap, physical reuse, old-generation completion,
   successor gate/candidate/approval authority, retry refunds or actor authentication. Preserve
   release fencing, ordinary cancellation, pool quarantine and request replay/conflict exactly.

Acceptance must include current-source CLI red/green and multiple allowed updates; budget,
retry, gate, role and runtime/environment change-then-restore refusals; renewal and history/guard
controls; legacy/invalid/fenced refusals; stale-policy completion and fresh-successor controls;
read purity, aggregate bounds and per-preparation scan cost. The CAL-V0-044/046 clauses and owning metadata are seeded before the first plan freeze.
Named tests and retained independent review/dogfood/native evidence are required before any
delivery claim; the external-agent guide and help gain the verified behavior at terminal binding.
Rollback preserves all journal and attempt bytes; a compatible previous writer resumes whole-policy
refusal after admissions stop. No destructive downgrade or canonical live-policy rewrite.


Wire and refusal boundary: the RELEASE reason/evidence request shape, attempt retryAccounting
and handoffEvidence members, canonical request preimages, policy profile and journal receipt
codec stay unchanged. Compatibility history is internal preparation evidence, never a new
persistent authority field. Replay/conflict runs before any new history observation. Both full
audits must share head/receipt/LastSeq, inventory and intent identities under one ChangeGuard;
staging, pending redo, IntentError, selected historical drift or final-policy mismatch refuses.
Only an otherwise eligible stale clean release gets this observation; ordinary cancellation,
reap, equal-policy handoff and invalid/fenced requests retain their existing reducer ordering.
The eligible original attempt/generation must carry matching original policy/config/capability
identities. Exact LF-bearing file digests are required; namespaced policy-body digests cannot
substitute. All history remains codec/digest checked after incompatibility. Original admitted
identities, required gate results and candidate/scope authority are never rewritten or inherited.
No new public verb, flag, schema profile or journal/attempt member is added.

### Configurable retries, external work handoff and help (issues 420–422)

- `CAL-V0-045`: Claim (explicit and next), pure plan preview (including pool/stage selection), and
  exhausted-OPEN owner recovery MUST apply the same current `retries.admissionsPerRevision` value.
  The historical field name denotes charged retries after the initial admission, not total
  generations. Its required canonical Count MUST be in 0..16; the fixture/default policy retains
  3, and 0 permits the initial admission but no charged retry. Missing or malformed fields MUST
  refuse. Clean CAL-V0-044 handoffs preserve debt at any limit; failures, cancellations and expiry
  remain charged. Changing a policy MUST NOT erase debt, acceptance history or owner recovery
  safety checks. A raised limit makes a previously exhausted attempt eligible only through the
  ordinary current-policy admission predicate. This code change does not authorize updating any
  real queue policy or migrating live attempts. Readers limited to 3 may refuse a policy above 3;
  retain compatible tooling and the complete store rather than stripping fields or downgrading.
- `CAL-V0-046`: An external-agent `release --reason HANDOFF --evidence REF` MUST support work
  outside the queue repository without submitting an unchanged or unrelated tree. `HANDOFF`
  permits implement, review and integrate; `REVIEW_RETURNED` permits review only. With REF, the
  live, unexpired current generation MUST be RUNNING, have no candidate, scopeCheck UNKNOWN,
  no gate results or pending effects, and prospective NONE/false retry accounting. Unchanged
  acceptance and unchanged policy/config or the complete proposed issue 482 compatibility
  proof remain mandatory. Without REF, the existing BUILT/CHECKING,
  candidate and WITHIN requirements remain mandatory. Evidence on ordinary cancellation MUST refuse unless the explicit CAL-V0-067 profile is
  independently eligible; evidence on a candidate-bearing release MUST refuse. A non-PASSED gate remains sticky, and missing legacy
  accounting and supervised attempts never qualify. Failed checks MUST NOT record a clean
  disposition. The optional attempt member `handoffEvidence` MUST be absent or a nonempty
  `Identifier` (1..128 UTF-8 bytes, no hostile code points or TAB/LF/CR); null, empty, unknown and
  overlong forms MUST refuse. It is required exactly for the no-tree clean terminal branch and
  forbidden on live, NONE, legacy, supervised, ordinary-cancel and candidate-tree records.
  The decoder MUST jointly bind reason/cause/disposition, stage, CANCELLED phase, FENCED
  quiescence, candidate absence, UNKNOWN scope and empty gate/pending-effect sets. The reference
  MUST join the canonical RELEASE request preimage only when present: absent evidence MUST
  preserve legacy preimage bytes and replay. Same-reference replay MUST be idempotent;
  changed-reference replay MUST conflict. New metadata remains journal-bound; old readers may
  refuse and MUST NOT be used to strip or rewrite it. A reference is inert caller evidence, never
  fetched or executed and not proof of its contents, work quality or physical cleanup. Release
  MUST remove the reservation and retain existing pool quarantine; reuse still requires the
  existing cleanup/safe-confirm flow unless the separately defined explicit CAL-V0-067 profile
  is requested and independently eligible. An inert handoff reference alone never frees a member. Journal consistency, logical fencing and quarantine MUST
  remain distinct from separately observed physical cleanup. No completion, review, integration,
  publication, automatic reopen or historical refund authority is added.
- `CAL-V0-047`: Every implemented and omitted public command path and command family MUST return
  read-only OK for an exact trailing `--help` or `-h` help request, with command-specific usage,
  flags and applicable reason codes. The lease release help MUST state CAL-V0-044/046 eligibility
  and refusal codes, including that relevant or unproved policy changes make live handoffs
  STALE_POLICY and the proposed issue 482 exception needs a fully audited compatible interval. Help MUST
  require no initialized store and perform no store read/write/lock, stdin read, archive stream,
  command execution or launcher action. Omitted execution remains NOT_RUN and its help MUST say
  so without inventing execution flags. Unknown paths and malformed non-help invocations retain
  ordinary behavior. Existing mutation help alongside flags, operation and payloadKeys remain
  supported; a scalar flag value spelled --help or -h MUST NOT become a help request. The
  --version alias and release lease/artifact-family dispatch MUST remain compatible.

- `CAL-V0-048`: `attempt heartbeat --attempt ID --generation G --request-id ID` MUST
  record a generation-local optional `lastHeartbeatAt` through the journal writer, using ordinary
  generation, terminal and lease-expiry fencing. Fresh claims initialize the recorded signal;
  readmission resets it. Heartbeat MUST NOT extend the work lease, charge retries, release a
  reservation or change physical quiescence. Existing request preimages and legacy attempt bytes
  remain unchanged. Replay MUST return the original result without refreshing a timestamp,
  including after expiry or readmission. `attempt show` and live `queue status` entries MUST expose
  `lastHeartbeatAt` (null for legacy absence), `holderStatus`, `observedAt` and
  `heartbeatTTLSeconds:"600"`. The observation uses one time per command. Legacy absence is
  NOT_OBSERVED; otherwise terminal is TERMINAL, work-lease expiry is LEASE_EXPIRED, an observation
  preceding the signal is CLOCK_BEFORE_HEARTBEAT, age >=600 seconds with a live lease is
  STALE_HOLDER, and a younger signal is FRESH_HOLDER. These are recorded freshness observations,
  never authenticated holder liveness, proof of death, cleanup or release authority. Derived
  display fields MUST stay outside canonical attempt evidence in criterion captures. Automatic
  reaping or stale-holder handoff is outside this amendment.

- `CAL-V0-049`: `ticket show`, full `plan preview` entries and `queue status.retries` MUST
  expose current acceptance-revision `retries` with canonical Counts `charged`, `limit`,
  `remaining` (floored at zero), boolean `exhausted`, closed `byReason` and `reasonHistory`.
  Queue retry entries cover OPEN/HELD tickets and name ticketId/ticketRevision. Charged debt uses
  the latest attempt's stored retryCount at that acceptance revision; the bound and exhaustion
  predicate MUST be the admission predicate, including clean handoff at the bound and initial
  admission when the bound is zero. Journal-absent inventory reads retain NOT_OBSERVED debt,
  remaining and reason history, null exhaustion and byReason. Changing policy MUST NOT erase debt.
  Optional prospective attempt `retryReasons` has exactly EXPIRED, RELEASED, FAILED, UNKNOWN
  Counts whose sum MUST equal retryCount. Only a charged readmission increments one bucket:
  terminal FAILED with cause LEASE_EXPIRED is EXPIRED, other FAILED is FAILED, CANCELLED is
  RELEASED, and unclassified state is UNKNOWN. Clean handoffs copy counts without increment;
  new acceptance resets them. Legacy debt becomes UNKNOWN, never inferred specific reasons.
  reasonHistory is INCOMPLETE while UNKNOWN is nonzero, otherwise COMPLETE; this records cause
  classification, not authenticated physical failures or historical acceptance qualification.
  Issue 503 adds advisory `remainingMeaning: RETRY_CAPACITY` and `retryAdmissionReason`
  (`INITIAL_ADMISSION`, `NEW_ACCEPTANCE`, `COMPLETED_ATTEMPT`, `VERIFIED_HANDOFF`,
  `RETRY_AVAILABLE`, `RETRY_EXHAUSTED`; `NOT_OBSERVED` without a journal). Zero remaining
  does not itself establish exhaustion: initial admission, a completed latest attempt and a
  verified clean handoff retain the existing admission exceptions without changing charged debt.
  `ticket show` and `ticket blockers` MUST expose boolean-or-null `claimable`,
  `claimabilityReason` and `claimabilityScope: RECORDED_DEFAULT_EXTERNAL_AGENT_PLAN`.
  The observation is this ticket's recorded default external-agent plan before reap, without
  earlier proposed selections; it never reserves capacity, validates caller branch/base/holder
  or scope arguments, proves physical quiescence or promises a future or pool/supervised claim.
  Unknown-only admission evidence yields null; a known blocker, reservation collision or capacity
  limit yields false even with unknown evidence. Existing whole-repository coverage fallback
  remains unchanged. Reasons describe this read profile, not the writer's first refusal ordering.
  Journal absence yields null claimability and `NOT_OBSERVED`; invalid audited inputs retain read
  refusal. Recorded expired reservations remain until an explicit reap. Read projections MUST
  NOT mutate attempts, reservations, retry accounting or queue state.

- `CAL-V0-050`: Read-only `policy show` MUST return effective canonical policy, policyVersion
  and the existing policySha256 content identity from one consistent snapshot, without writing,
  locking, reading stdin or launching commands. `policy update --help` and external-agent docs
  MUST say that files use canonical UTF-8 JSON, sorted keys, no insignificant whitespace and
  exactly one trailing LF, and file policyVersion MUST equal expectedPolicyVersion+1. The flag
  names the current version. Dry-run and real-queue policy changes are outside this amendment.

- `CAL-V0-051`: CREATE help MUST omit --target and --expected-revision, which CREATE refuses.
  It MUST explain optional payload `localToken` with a meaningful-ID example and its existing
  queue-local grammar/collision validation. Existing CREATE localToken behavior and canonical
  ticket identity through show, claim and plan preview MUST remain unchanged; no new alias,
  serial allocator, wire migration or target-ID semantics are introduced.

Rollback stops admissions before switching to a compatible writer. Preserve every journal,
request and optional metadata member. No destructive downgrade, migration or live policy rewrite
is part of these amendments. Failure witnesses include mismatched terminal metadata, changed
reference replay, changed policy/acceptance, charged expiry, zero-budget exhaustion, and help that
reads stdin or leaves any filesystem artifact.

S4, planning.

- `CAL-V0-014`: `corvint-tasks plan preview` MUST implement `taskman-priority-first/0` (TCP-00
  §4.3) over the current inventory and live reservations, as a pure read with
  `mutationAuthority:false`; `deferredSinceSeq` is null because this spec pins no plans.
  The plan covers `OPEN` and `HELD` tickets, and its eligibility predicate is the claim's, so a
  `SELECTED` entry is exactly a ticket `claim` would admit. Each entry is `BLOCKED` with the first
  of: `PAUSED` under an admission barrier, `BUDGET_UNKNOWN` when the policy requires an enforced
  budget field, the ticket's own blockers and unknowns except `COVERAGE_UNKNOWN` (an undeclared
  ticket claims `WHOLE_REPOSITORY`), and `RETRY_EXHAUSTED`. An eligible entry is `DEFERRED
  RESOURCE_COLLISION`, naming the colliding ticket, when its resources collide with a live
  reservation or an earlier selection, and `DEFERRED LIMIT_EXCEEDED` once reservations plus
  selections reach `maxActiveAttempts`; otherwise it is `SELECTED` with reason `DEVELOPMENT_MODE`.
  `availableWorkers` is reported and never decides, because an `external-agent` attempt holds no
  worker. With explicit `--selected-only`, the command MUST project that same complete plan as the
  closed `taskman-plan-selected/0` profile: `planningProfile`, `queueId`, every selected ticket ID
  in plan order, `selectedTotal`, `complete:true`, and `mutationAuthority:false`. The projection is
  never paginated or truncated. The detailed `taskman-plan/0` remains the default and planning
  decisions do not depend on the projection.

S5, gates and completion.

- `CAL-V0-015`: `submit --tree <oid>` MUST record the candidate tree and move `RUNNING` to `BUILT`;
  a new `submit` from `BUILT` or `CHECKING` replaces the tree and marks every earlier gate result
  `STALE`.
- `CAL-V0-016`: `gate run --gate <gateId> --worktree <path>` MUST refuse unless the worktree's
  `HEAD^{tree}` equals the candidate tree and the worktree is clean, then run the policy-declared
  command gate there with its declared argv and timeout, and record a `GATE_RESULT` receipt with
  the tree, exit status, duration and the SHA-256 of the captured output kept under `evidence/`.
  corvint-tasks observes the exit status itself; a holder cannot report a gate as passed.
- `CAL-V0-017`: `complete --commit <oid>` MUST refuse unless the commit is reachable from the
  queue's `intentBranch`, its tree equals the candidate tree, and every required gate of the
  ticket has a `PASSED` result at that tree. It then completes the ticket with those digests as its
  completion evidence, moves the attempt to `COMPLETED` with quiescence `FENCED`, and removes the
  reservation, in one transaction. A held ticket refuses `TICKET_HELD`. Integration itself (rebase,
  merge, push) stays with the repository's own tooling.

S6, import cost.

- `CAL-V0-018`: A first import MUST cost one full audit plus work proportional to the records it
  writes: the audit is taken once under the import's lock and session, and each batch re-checks
  only the head receipt it expects and the files it posts. Measured on the 2,894-item Beamfall
  export, the first import MUST take under 5 minutes on a host with a load below the CPU count.

S8, parallel claims.

The owner's goal is many concurrent agents against one repository. Beamfall's runner admits at most
two claims per repository, and a ticket that declares no paths collides with every other claim;
most of Beamfall's core tickets declare none. S8 gives every claim an explicit scope, derives one
from Corvint's own context index when the ticket declares none, and makes the scope binding at
submit, so that disjoint work runs concurrently and a wrong prediction is refused rather than
silently shared.

- `CAL-V0-021`: Every `external-agent` attempt MUST carry a scope: a set of `PATH` resources with a
  `scopeSource`. In order of precedence, the scope is the ticket's `effects.touchPaths` and `PATH`
  resources when its coverage is `QUALIFIED` (`DECLARED`); else the paths given by `claim --scope
  PATH...` (`REQUESTED`); else the CAL-V0-022 derivation (`DERIVED`); else one `WHOLE_REPOSITORY`
  resource (`WHOLE_REPOSITORY`). Every scope other than `WHOLE_REPOSITORY` also holds each
  non-`PATH` resource the ticket declares, so claims on one database, port or shared gate still
  collide. A `PATH` key ending in `/` covers every path under it; any other key names one path
  (TCP-00 §4.2). The reservation entry holds the same resource set.
- `CAL-V0-022`: A `DERIVED` scope MUST come from Corvint's local context index for the ticket's
  title and body at the attempt's base tree, computed in process with no network and no write
  outside the task store, bounded to at most `MaxTouchPaths` paths. The attempt records the
  derivation's input digest. When the index is absent, stale for that tree, or abstains, the scope
  is `WHOLE_REPOSITORY`; a derivation never widens authority and is never an input to ranking or
  evidence. The accepted S8 integration requires explicit `CORVINT_SNAPSHOT_FORMAT=pack`
  opt-in for reuse between the Corvint and Tasks binaries. Other formats abstain; this does not
  promote the experimental pack profile. Incomplete context packets also abstain. For
  `claim --next`, derive only the ticket selected by the existing conservative priority plan,
  bind the facts to that ticket, and recheck collisions; a blocked plan stays blocked.
  Decision 0397's CAL-V0-022 addendum permits only the scope adapter's Core imports and its
  pack-fixture test import. Standalone source-archive rebuild remains blocked by V1-0456.
- `CAL-V0-023`: Two live attempts MUST collide exactly when their resource sets collide under
  TCP-00 §4.2 path normalization; `WHOLE_REPOSITORY` collides with every live entry and every live
  entry collides with it. A colliding `claim` refuses `RESOURCE_COLLISION` naming the other
  attempt. There is no per-repository lane limit beyond the policy's `maxActiveAttempts`, and scopes
  are not expanded by dependency or import closure; interference between disjoint scopes is caught
  by the CAL-V0-016 gates at the exact candidate tree.
- `CAL-V0-024`: `submit --tree <oid>` MUST refuse `OUT_OF_SCOPE`, naming every offending path,
  when the diff from the attempt's base tree to the candidate tree adds, deletes, renames or
  modifies a path not covered by the attempt's scope under §4.2 normalization.
- `CAL-V0-025`: `widen --attempt <id> --generation <G> --scope PATH...` MUST add the paths to a live
  attempt's scope and reservation in one transaction, and refuse `RESOURCE_COLLISION` without
  writing when any added path collides with another live attempt. Widening to `WHOLE_REPOSITORY`
  is allowed only when no other attempt is live. An `ADMISSION` barrier refuses `widen` `PAUSED`,
  as it refuses scope-expand (TCP-00 §3.4).
- `CAL-V0-026`: Each lease command (`claim`, `renew`, `release`, `reap`, `widen`, `submit`, `gate
  run` excluding the gate's own run time, and `complete`) MUST hold the store lock for work that
  does not grow with the number of tickets in the store, beyond the records it writes: the full
  audit is reused from a verified cache keyed by the head receipt digest and the intent tree
  digest, and is otherwise taken once. Measured on a synthetic 3,000-ticket fixture store, the p95
  lock hold of `claim` and `renew` MUST be under 500 ms on a host with a load below the CPU count.

  The cache is process-local and retains one successful audit; fresh CLI processes audit once.
  Reuse also verifies every physical content digest and path membership under a transient native
  change monitor. Inventory, audit, model construction and monitor teardown run outside the lock.
  Locked guards bind the prepared result to the unchanged head and monitored bytes before applying
  the bounded writes. A changed observation retries; unavailable monitoring refuses. Pending-receipt
  recovery prepares its full proof outside the lock before bounded redo. This changes no store format.

#### Issue 494: bounded preparation admission

Cooperating lease preparations use a bounded registered-order admission step before the existing `taskman.prepare.lock`. A successfully published registration cannot enter preparation while a smaller continuously live registration exists. Publication order is not CLI arrival order; pre-registration scheduling, mixed-version fairness and universal starvation freedom are not qualified. The default and maximum acquisition wait remain 30 seconds total from acquisition entry across identity resolution, registration, scans, queue wait and the final gate. It must never restart at phase boundaries. Full audits, journal/intent formats, writer authority, request identity, replay and the original shared 180-second qualification context remain unchanged.

The private coordination namespace consists of the inert `taskman.prepare.registry.lock` and 64 fixed `taskman.prepare.slot.00` through `.63` regular files under the pinned Git common directory, outside journal and intent. A slot has 16 bytes: `CPA1`, four zero reserved bytes, unsigned 64-bit big-endian nonzero rank. A live slot is owned by an exclusive nonblocking-flock open description. Registry-held publication chooses one greater than the maximum live rank, or 1 when none are live. Rank overflow and a scan observing 64 live slots refuse LIMIT_EXCEEDED without a receipt. This fixed technical bound includes the serving holder and is independent of lease/reservation capacity. A scan is not an instantaneous-capacity promise. No arbitrary namespace enumeration, durable counter, PID/time-based eviction, daemon, database service or new authority is introduced.

Only a successful lock probe proves an abandoned slot reusable. Held malformed/duplicate-rank state, unknown format, unsafe objects and observed name/root/inode drift refuse. Partial unowned bytes can be overwritten only through the acquired slot descriptor under the registry. The registry must not surround a sleep, final preparation gate, writer work, inventory/audit, monitor teardown or external execution. Slot/registry files are not normally removed or replaced. Scratch bytes are scheduling data, never proof/intent/ranking/attempt authority; read verbs do not create, clean or mutate them. Existing final-gate exclusion remains compatible with older callers, while their bypass of registration leaves mixed-version fairness unqualified.

Preparation owns all registry/slot/probe/gate/root descriptors it obtains. Every locally owned cleanup failure is retained with the primary error. Temporary-root cleanup completes before returning a successful composite handle; if it fails after gate acquisition, gate and slot are retired and the call fails. This does not broaden the public writer helper's cleanup behavior or claim coverage of hidden safeopen traversal ownership. One synchronized composite Close owns retirement: gate first, slot regardless of gate error, aggregate all failures, then deliver one observation. Hold measurement ends at the actual gate-release boundary; observation delivery waits for mandatory slot cleanup. An acquired-but-not-cleanly-released handle cannot be reported as fully released success. Concurrent/double Close returns the recorded result without duplicate cleanup or callback. Close/cancel never requires registry ownership; observers run outside internal locks and never under registry ownership.

Cancellation closes owned references; process-death recovery requires those references actually gone, demonstrated after joined exit. CLOEXEC is mandatory; no descriptor handoff to helpers is permitted. A still-held inherited or leaked reference remains live and is never evicted by age. Process exit may release descriptors at different instants, so transient conservative refusal before joined exit is allowed. Normal machine restart leaves no live owner locks, so stale scheduling bytes convey no authority and need no durable recovery claim. Boundary identity checking is not continuous hostile-filesystem monitoring.

Deterministic proof must acknowledge reached publication/entry/injection boundaries and cover registered non-overtaking, head/middle cancellation, death before/during partial publication and after publication/while registry/while serving, actual independent-open exclusion, exec closed-FD witness, capacity/overflow/malformed/identity refusals, total deadline, locally owned cleanup failures and synchronized composite Close. Canonical fixture initialization/audit precedes injection; journal/intent snapshots and authorized product effects are asserted separately from scheduling scratch. Actual Darwin and Linux execution evidence is required before the corresponding platform claim; cross-compilation is insufficient and an absent runner stays NOT_RUN.

After focused checks and independent source PASS, one unchanged original 5000-receipt/10-worker/30-operation mixed qualification must show 5000→5030 full consistent/agreeing audits, unique completed operations, ten stable identities/generations, all final CANCELLED/FENCED, zero active reservations, exact-ID replays with unchanged digest and after-replay audit 5030, plus bound source/binary and retired owned processes. Retain phase/rank/publication diagnostics on timeout without retries or changed deadlines. A failed wave is preserved and triggers diagnosis, not another automatic wave. The existing original CAL-V0-026 acceptance is not replaced by this test.


S7, qualification and execution cutover.

- `CAL-V0-019`: A named test suite MUST show, for `external-agent` attempts: two concurrent
  colliding claims admit exactly one; `claim`, `renew`, `reap`, `submit`, `gate run` and `complete`
  racing each other leave one consistent head; a fenced generation cannot move or complete an
  attempt; and a crash at each commit point of every lease verb leaves the whole transaction or
  none of it (the §5.3 crash matrix, lease rows).
- `CAL-V0-020`: `cutover --execution --decision <ref> --qualification <file>` MUST, under an
  `OWNER` binding, record a `QUALIFICATION` receipt naming the CAL-V0-019 run and set the queue's
  `executionCutover` with that `decisionRef` and the run's digest in `gateEvidence`. The file is the
  `go test -json` output of the suite, with each named test run and pass and the final package
  pass; it is posted as an evidence blob under its digest. It
  refuses when the run is absent (`MISSING_EVIDENCE`), has a failing test (`GATE_FAILED`), or is
  not `go test -json` output (`MALFORMED`); on a fixture queue, before the authority switch
  (`CUTOVER_MISSING`), and when `executionCutover` is already recorded.

Non-fixture release lifecycle (owner request 2026-09-28 to complete the Tasks takeover).

- `CAL-V0-027`: A non-fixture queue with null `importMapSha256` MUST admit release creation,
  update, candidate capture, attestation and promotion under the existing actor, policy, CAS,
  source, ticket acceptance, gate and predecessor checks, both before and after qualified
  execution cutover. Release writes MUST NOT change queue authority, policy or execution cutover;
  CAL-V0-002 still blocks unqualified claims. Shared staging observation MUST admit the same
  non-fixture queue identity for supported operations while retaining layout, size, digest,
  queue/head/base/request/receipt binding and malformed/fork refusals. Completed observations
  MUST use the closed receipt kinds emitted by each supported stage class, including recorded
  FENCED transitions with their original refusal outcome and codes; cross-class or unknown kinds
  refuse. Import-mapped queues,
  fixture execution cutover and INIT with execution cutover remain refused. Observation MUST
  NOT remove stage bytes or authorize execution. The existing locked writer retry MUST recover
  orphan slots and redo a durable receipt exactly once; an unchanged request replays and a
  changed request with the same ID refuses. Active descriptors retain the existing unsupported
  recovery boundary. Release reconciliation MUST remain settled-state `KEEP_JOURNAL` only,
  bind the exact offered bytes and canonical digest, preserve conflicting bytes as evidence,
  and refuse pending receipts and active staging without cleanup. `ADOPT_FILE` stays `NOT_RUN`.

- `CAL-V0-042`: A separate `corvint-tasks-archive/0` build path MUST package the Tasks binary,
  corresponding immutable source, license/notices, manifest, and checksums without changing the
  Core archive or claiming workflow-bundle qualification. The initial target is native macOS arm64;
  others remain NOT_RUN. Two isolated builds and two archive assemblies MUST agree, and the
  extracted binary MUST expose plan, claim, submit, gate and completion in a native help smoke.
  The manifest MUST retain the unverified version label, source commit/tree and pinned build count.
  This does not publish a release, authenticate an operator or qualify a task queue.

### S9 — Named environment pools (issue 342)

- `CAL-V0-028`: Policy MAY add optional `pools`; tickets MAY add acceptance-relevant
  `requiresPool`; attempts MAY add `stage` and `poolAllocation`. Omission MUST preserve old
  canonical bytes. Pool/member identities MUST be unique within the queue. The bound is 64 pools,
  256 total members, the existing 256 KiB policy, and a 1 MiB `taskman-pool-state/0` projection. Lease staging permits 11 artifacts,
  three blob afterimages and a 2658-byte descriptor; other operation limits remain unchanged.
  The shared temporary descriptor admission bound is therefore 2658 bytes.
  A member definition includes its pool, reservation stage, configuration reference and commands.
  Removing or changing an occupied definition MUST refuse; unrelated policy changes MAY proceed.
- `CAL-V0-029`: Claim and claim-next MUST atomically reserve one eligible free member of an
  explicitly requested pool with the attempt and ordinary scope reservation. `requiresPool` MUST
  match the explicit request. No request consumes no pool. Reserved members require matching
  `implement|review|integrate` stage, an operator claim rather than authenticated identity.
  An eligible free member reserved for the requested stage MUST be selected before an unreserved
  free member, preserving policy member order within each tier. If matching reserved members are
  occupied or unavailable, an unreserved member remains eligible fallback capacity. A claim with
  no stage admits only unreserved members, and members reserved for another stage remain ineligible.
  Allocated state MUST agree with the complete attempt allocation tuple, holder and stage.
  Replayed claims MUST return their original receipt-bound allocation, never a successor's.
  CAL-V0-065 adds explicit per-claim exclusions to this eligibility rule.
- `CAL-V0-030`: By default, release, expiry/reap and completion MUST quarantine the exact allocation while
  freeing the ordinary scope reservation. A retry MUST acquire a new allocation. Only an
  OWNER/OPERATOR `pool confirm-safe` naming the current allocation, an evidence reference and
  reason MAY clear quarantine. Configured cleanup success is necessary but insufficient: the
  confirmation is a local operator attestation of external revocation/reset, not observed physical
  exclusivity. Stale confirmation MUST refuse. There is no TTL or implicit safe reuse.
  The separately proposed explicit CAL-V0-067 operator-attested release profile is the only
  proposed exception; it does not apply to ordinary release, expiry/reap or completion.
- `CAL-V0-031`: A configured health command MUST acquire durable PREPARING ownership before
  execution outside the writer lock. Failed members MUST remain quarantined, be reported with
  reason and observation digest, and be skipped for the current claim. A passing health result
  MUST bind allocation, definition, immutable source revision/tree and command environment digest,
  then be retained atomically with admission after rechecking current eligibility. Standalone
  `health --member` MUST also leave quarantine, including on success, until operator confirmation.
- `CAL-V0-032`: Cleanup MUST acquire durable CLEANING ownership before execution. Pending command
  replay MUST NOT execute again. Explicit `pool recover` MUST refuse an observed live runner and
  quarantine an orphan without implying cleanup. Interrupted or uncertain execution MUST never
  make a member free. Journal redo publishes committed artifacts only. Runner PID/start observations
  are local observations, not authentication or an exactly-once execution guarantee.
  The proposed CAL-V0-067 profile MUST refuse every tracked started, pending, interrupted or
  uncertain use; missing command metadata alone MUST NOT qualify an allocation for that profile.
- `CAL-V0-033`: Pool commands MUST use bounded trusted operator argv, declared environment keys,
  a clean repository outside `.taskman`, a 1..300 second timeout and at most 64 KiB captured output.
  Observations retain the output digest, not raw output. The implementation MUST join cancellation
  handling and stop/check the owned process group after normal exit, timeout and interruption;
  unproved cleanup MUST refuse admission. Detached processes, external services and a killed host
  are outside this process-group qualification. Immutable configuration references MUST name exact
  regular Git blobs, including for claims without a health command; symlinks and missing bytes refuse.
- `CAL-V0-034`: Queue occupancy and plan preview MUST remain read-only and execute no probes.
  Occupancy MUST distinguish free, preparing, allocated, cleaning and quarantined members, with
  original allocation identity and retained command reason/observation where present. A selected
  preview batch MUST consume eligible free member capacity under the same ordered eligibility rule
  as claim, excluding other-stage reservations without executing health probes.
  Archive, journal recovery and authority-confined projection publication MUST retain pool state.
  CAL-V0-065 applies the same explicit exclusion set to preview capacity.

The optional policy shape is `pools:[{id,members:[MEMBER],reservedFor:{MEMBER:STAGE},
memberConfig:{MEMBER:{configRef:{revision,path,blob},health:COMMAND,cleanup:COMMAND}}}]`.
Each map is closed over declared member names; each nested addition is optional. A command is
`{argv:[ARG],cwd:"REPOSITORY",env:[NAME],timeoutSeconds:"N"}`. Git references return only identity,
never configuration bodies. Duplicate identical configuration references refuse; differently named
references cannot prove distinct physical environments. The command interpreter and external services
are operator-provided dependencies, not attested deployed lineage. Commands run in the caller's
repository checkout, falling back to the primary worktree.

`poolAllocation` contains `poolId`, `memberId`, `allocationId`, `definitionSha256`, `allocatedSeq`,
and optional `configRef`. The allocation digest binds queue, request and member; it is not a secret
capability. `pools.json` is a separate authoritative receipt projection, never an extra ordinary
reservation. Its closed entries retain allocation, state, holder/stage, attempt/generation,
changed sequence, policy/request digests, command kind/revision, runner observation, cleanup result,
observation digest and reason. `taskman-pool-observation/0` is bounded to 4096 bytes and retains
allocation/definition, command kind, revision/tree, result class, passed/group-clean flags and
output/environment digests. Missing inventory-bound state is corruption, never free capacity.

### S10 — Foreground Codex programs (issue 341)

Authoritative inputs: [issue 341](https://github.com/Beamfall/corvint/issues/341), the owner's
2026-09-29 Codex-only direction, and the reviewed local exact-tree/expected-base integration
boundary. Claude support and remote publication are outside this slice. The existing external-agent
branch and absent optional-field bytes remain unchanged. Qualification is scoped to the pinned
Codex executable and observed event vocabulary; it does not attest authentication or hostile-child
containment. Frozen native qualification is recorded in docs/build-log/2026-09-29-tasks-codex-supervision.md.

- `CAL-V0-035`: The optional `taskman-codex-supervisor/0` policy profile MUST dispatch a pinned
  Codex executable through a journaled SPAWNING effect, exclusive durable boot record, validated
  PID/start/group identity, RUNNING commit and exact acknowledgment before execution. Unsupported
  platforms MUST compile and refuse. Truncated output MUST remain an invalid/unknown result.
- `CAL-V0-036`: Implement, independent review, repair and integrate MUST be native attempt stages.
  Optional acceptance-relevant `requiredRoles` maps implement/review/integrate to existing runtime
  roles; enabled runtime roles and worker limits govern dispatch. Review MUST bind every acceptance
  claim, exact candidate tree, distinct holder and distinct host session.
  Initial Core context queries preserve the exact ticket title followed by its canonical ticket ID
  as one task argument; they add no inferred paths. Combined input exceeding Core's 8000-rune
  UTF-8 task bound refuses without truncation. READY, freshness and exact-tree checks remain required. Returned work retains
  feedback and candidate; missing or failed required gates MUST block before any target mutation.
- `CAL-V0-037`: A live owner MUST NOT be stolen. Explicit quiescent owner release or native identity
  proof permits a fenced epoch transfer. Drain, cancel and recovery MUST retain uncertain scope,
  worker and pool resources; proved stage shutdown releases workers and quarantines its physical
  pool allocation. A subsequent role obtains a fresh allocation. Reused PGIDs and escaped anchors
  MUST NOT authorize adoption or signaling of unknown processes.
- `CAL-V0-038`: WAIT MUST preserve the exact session, worktree, partial candidate and handoff.
  Questions and answers MUST bind attempt generation and acceptance revision. Read-only pending
  state MUST expose questions and integration waits. Explicit resume/retry MUST retain feedback;
  neither an answer nor a host result grants integration approval.
- `CAL-V0-039`: Every dispatch MUST reserve a turn under the native writer lock. Concurrent lanes
  share one program's cumulative counters and start time across ticket reassignment. Active
  deadlines MUST respect lane and remaining program wall caps. Qualified JSONL token usage is
  OBSERVED, missing dimensions NOT_OBSERVED; required hard token enforcement is unsupported.
  Observed token cutoffs block subsequent dispatch, with at most one already-admitted turn per
  active lane of overshoot. Refused pre-fork work leaves a resumable no-exec outcome.
- `CAL-V0-040`: Every assignment/stage MUST use a distinct registered worktree/private Git directory.
  Add/remove and integration effects MUST be durable before mutation. Exact directory/common-dir,
  commit/tree and clean-state bindings govern recovery. Only an explicitly designated integration
  checkout may advance, and its tip MUST still equal the candidate's original base and grant binding.
  Advanced targets require a new candidate, review, gates and grant. Crash recovery recognizes only
  the exact clean applied candidate, including the interval before native completion.
- `CAL-V0-041`: Foreground role workers MUST pull eligible work without a daemon, select existing
  review/integration attempts, and treat absence of eligible work as idle completion. Terminal proved
  slots may be reassigned with exact attempt/generation and assignment fencing, preserving shared
  budget history and journal handoffs. The 64-slot bound is concurrent retained state, not a lifetime
  ticket limit. New program records and evidence MUST participate in native journal projection,
  archive and audit, with no independent authority database.

Wire amendment: optional policy `supervision` contains profile, contextRequired=true,
maxRepairCycles (0..2), and program turns/wallClockMinutes/inputTokens/outputTokens caps.
Optional ticket `requiredRoles` is a closed nonempty role array per stage. `programs.json` is a
bounded 1 MiB, 64-slot journal-authoritative projection; program changes and handoffs are bounded
64 KiB, with host stdout/stderr individually capped at 16 KiB. Stage context is a pinned native Core
query against the isolated checkout: READY/fresh tree revision must equal the stage commit's tree;
explicit uncertainty is carried unchanged. No inferred context becomes accepted intent.

### Owner retry readmission (issue 378)

- `CAL-V0-043`: `ticket reopen` MUST accept an `OPEN` ticket only for an explicit `OWNER`
  invocation permitted by policy, carrying a nonempty reason, request ID and exact expected
  ticket revision, when the latest attempt is `FAILED` or `CANCELLED`, is bound to the current
  acceptance revision and has exhausted the current policy retry limit (CAL-V0-045). The writer MUST derive recovery
  facts from complete, schema-valid, canonical journal-backed attempt bytes and reservations.
  Every attempt for the target ticket MUST be terminal, without pending effects or reservations;
  external-agent attempts MUST be `FENCED`, and supervised attempts MUST have `PROVED`
  quiescence with no worker. Unknown, mismatched, inconsistent or ambiguous generation facts
  MUST refuse. The pure mutation observation MUST bind the ticket and acceptance revision;
  a missing observation MUST NOT authorize recovery. Existing completed-ticket reopen semantics
  and policy role narrowing remain unchanged.
  Issue 503 requires OPEN recovery refusals to explain the failed recorded condition:
  missing/mismatched recovery observation, reservation, ambiguous generation, live attempt,
  pending effects, unproved quiescence/unknown runtime, absent prior attempt, acceptance mismatch,
  unexhausted retry budget or a latest phase other than FAILED/CANCELLED. The result retains
  `TICKET_STATE`; diagnostic detail grants no recovery authority. An unexhausted budget refusal
  directs the operator to `ticket show` claimability instead of implying that an OPEN ticket
  needs reopening. Ticket/revision binding, OWNER policy and all recovery predicates stay intact.
  Recovery MUST increment ticket revision and acceptance revision exactly once, preserving the
  acceptance criteria, dependencies, gates, effects, prior records, attempts and gate history.
  A later claim MUST start a fresh attempt with zero retries and remain subject to ordinary
  admission, dependency, approval, scope, pool and gate checks. Old acceptance-bound approval
  and gate evidence MUST NOT authorize the new acceptance. Recovery is not completion.
  The successful transaction MUST retain the exact canonical reason-bearing mutation envelope
  as `evidence/<request-sha256>` and bind it in that same receipt's POST set. The MUTATE stage
  contract permits at most one such optional POST, with SHA and path matching RequestSha256
  and size at most MaxMutationEnvelopeBytes (256 KiB). It MUST NOT coexist with CREATE's queue
  POST; the six-artifact maximum remains, with measured descriptor ceiling 1,670 bytes.
  Existing descriptors without this evidence remain valid. Identical request replay MUST create
  no second receipt; a changed reason under the same request ID MUST conflict. Refused recovery
  MUST not plan this evidence POST. Publication failures before the receipt may leave an
  unreferenced evidence blob under the existing writer contract; only a committed receipt makes
  it recovery evidence, and post-receipt interruption MUST remain redo-safe.

The explicit local operator command is:

```sh
corvint-tasks ticket reopen --target FL-001.matrix --expected-revision 7 \
  --request-id recover-FL-001-matrix --role OWNER \
  --payload '{"reason":"Owner readmits the ticket after cancelled attempts"}'
```

Use the actual current revision from `ticket show`. For exact replay retain the original
`--issued-at` timestamp as well as all request bytes. The owner role and reason are local
operator-supplied claims under the existing authority boundary, not authenticated identity or
new execution authority. Reason text is durable local journal evidence and follows the store's
existing retention, backup and export behavior; no host data is added.

For reason lookup, follow the successful command's receipt filename to the receipt POST at
`evidence/<request digest>`, then read the canonical envelope's `payload.reason`, actor, target
and expected revision. `receipt audit` validates journal/projection consistency but does not
print reason text; `receipt show` is not delivered. An unreferenced evidence file alone is not
proof of recovery. `plan preview` and `claim` use the new acceptance revision only after the
recovery transaction commits.

Failure modes include non-owner or narrowed policy, stale expected revision, nonexhausted or
wrong-acceptance attempts, live or unsafe older attempts, mismatched physical/journal records,
and malformed/incomplete/ambiguous attempt inventory. These preserve the exhaustion boundary;
recovery does not bypass a failing gate, held dependency, missing approval or admission barrier.
Regression witnesses are `TestCALV0043_RecoveryFactsAndOwnerBinding`,
`TestCALV0043_RecoveryExaminesEveryAttempt`, `TestCALV0043_OwnerReopensExhaustedCancelledTicket`,
`TestCALV0043_RecoveryRefusesStaleAndTamperedAttempts`,
`TestCALV0043_RecoveryPreservesRequiredGateFailures`,
`TestCALV0043_RecoveryInvalidatesOldApprovals`, `TestCALV0043_RecoveryPublicationFaults`,
`TestCALV0043_CLIRecoveryAndPreview`, and `TestCALV0043_MutationRequestEvidenceBounds`.
Rollback disables new OPEN readmission while retaining history and already-issued receipts;
readers of recovery receipts must retain support for the bounded MUTATE evidence artifact.

### S11 — Continuous dispatcher (issue 431)

Authoritative input: owner request [issue 431](https://github.com/beamfall/corvint/issues/431).
The dispatcher is an operator-started foreground program, not a daemon: it runs only while
`corvint-tasks dispatch` runs, and stopping it leaves its workers running for the next dispatcher.
It adds no queue authority. Every store change goes through the existing lease transactions
(`release`, `reap`), and workers act through the ordinary CLI under their own holder name.
Its private state (ledger, events, worker logs and unpark requests) lives under the configured
`stateDir`, never in the native store. That state is not an input to the queue, ranking, evidence or
learning. Live qualification is recorded in `docs/build-log/2026-10-01-tasks-continuous-dispatch.md`
(OpenCode), `docs/build-log/2026-10-01-tasks-dispatch-claude-code.md` (Claude Code) and
`docs/build-log/2026-10-01-tasks-dispatch-codex.md` (Codex). Gemini CLI summary support is
derived from the installed CLI source and is not live-qualified; see
`docs/build-log/2026-10-01-tasks-dispatch-gemini.md`.

- `CAL-V0-052`: `dispatch --program ID --config FILE [--once | --ticks N]` MUST decode a closed
  `taskman-dispatch/0` configuration of at most 256 KiB, read without following symlinks, and refuse
  unknown members, trailing data, unknown placeholders and out-of-range bounds. The bounds are:
  tickSeconds 1..3600, globalCap 1..64, killGraceSeconds 1..120, 1..8 hosts with absolute
  executables, 1..32 roles, cap 1..64, priority 0..1000, idleSeconds 30..86400,
  wallSeconds 60..604800, cooldownSeconds 0..86400, parkAfter 1..100 and at most 256 pins.
  Host env MUST NOT set `CORVINT_DISPATCH_*`. One exclusive non-blocking lock per program state
  directory MUST refuse a second dispatcher. `dispatch status` and `dispatch unpark` MUST NOT read,
  lock or write the native store. Status reports workers, parked and cooling keys, the dispatcher's
  liveness (RUNNING, NOT_RUNNING or UNKNOWN, read from the lock file's process identity without
  taking the lock) and the event tail. Unpark writes one atomic request file that the running
  dispatcher consumes. SIGINT, SIGTERM and SIGHUP stop the loop after the current tick without
  killing workers. `workRoot` MUST resolve to the same task store as the dispatcher's working
  directory, so workers claim in the store that heal and reap act on.
- `CAL-V0-053`: The optional per-ticket work state MUST come from either a `status-line` reader (one
  `key: value` line in an absolute per-ticket file of at most 64 KiB) or a `command` reader (one JSON
  object of ticket ID or local name to state, at most 1 MiB of output, 60 s timeout). Values are at most
  64 printable bytes. A missing file is `NONE`. Every read failure MUST yield `UNKNOWN` and an alert,
  never a guessed state. Roles that match states MUST refuse without a reader.
- `CAL-V0-054`: The roster MUST be a pure function of the configuration, one observation, the
  running workers and the backoff skip set. Roles match tickets by labels, kinds, an ID glob, work
  states, excluded states, statuses and plan selection, or lane roles match quarantined members of
  one pool. A ticket with any live attempt is never a candidate; an expired lease becomes free only
  after a reap. Candidates order by pin, role priority, P-rank, plan order, key and role index. The
  global cap, then the per-role cap, bound the result, and each assignment takes the lowest free slot.
  One key holds at most one worker.
- `CAL-V0-055`: Each assignment MUST launch one independent process in its own session, with
  stdout and stderr appended to per-worker logs. The prompt and argv are rendered in a single pass,
  so a substituted value is never re-expanded, and `{prompt}` may appear at most once in argv. The
  worker receives `CORVINT_DISPATCH_PROGRAM`, `_ROLE`, `_SLOT`, `_TICKET` and `_WORKER`. Its holder
  name is the worker ID `<program>.<role>.<slot>.<nonce>-<seq>`. Names cannot contain `.`, and the
  nonce is random per dispatcher start, so IDs never collide across programs or roles, nor repeat
  after a crash or a deleted state directory. A host's `activityPaths` are rendered per worker. The
  worker MUST be saved to the ledger immediately after launch. A launch whose start identity cannot
  be read MUST kill the new session and fail. A launch failure MUST emit `launch-failed` and cool the
  key down for ten ticks while keeping its accumulated no-progress count.
- `CAL-V0-056`: Supervision MUST track every process in the worker's session, process group or
  descendant tree by verified start identity, so a reused PID is never signalled. A worker is
  stopped for WALL (wall cap), IDLE (no log growth, activity-path change or non-ignored busy child
  within the idle timeout) or ORPHANED (the leader exited while members remain). Stopping MUST send
  SIGTERM once to each member of the whole tree, then SIGKILL after the grace deadline, which is
  kept in the ledger so later ticks and restarts do not extend it. Survivors are reported as an alert
  while supervision continues. An unreadable process identity MUST keep the recorded tree, so an
  unobservable worker is never treated as ended. Supervision MUST run even when the store is
  unreadable; ended workers are then accounted on the next readable tick. A restarted dispatcher
  MUST adopt recorded workers whose identities still match. With `heal.handoff`, a live attempt held by an ended worker MUST be released as
  `HANDOFF`, with `--evidence dispatch:<worker>` when it has no candidate (CAL-V0-046). A refused
  handoff MUST emit `needs-owner` and leave the attempt untouched. With `heal.reap`, an expired lease
  whose holder is not a running worker of this program MUST be reaped, whoever held it, since an
  expired lease is reapable by any operator. Heal request IDs are deterministic, so a
  repeated heal replays.
- `CAL-V0-057`: An ended worker made progress exactly when the key's durable fingerprint changed.
  The fingerprint covers ticket status, revision and work state, plus attempts with a candidate,
  gates, reviews or a durable phase; for lanes, the member state, holder and attempt. Progress
  clears backoff. An `UNKNOWN` work state is never progress and never unparks a key. No progress MUST
  start a cooldown, and after `parkAfter` consecutive runs MUST
  park the key and emit `needs-owner`. A parked key resumes when its fingerprint changes or on an
  operator unpark request. A `RETRY_EXHAUSTED` plan entry MUST NOT be readmitted by the dispatcher.
  It emits `needs-owner` naming `ticket reopen` (CAL-V0-043), because readmission is owner
  authority.
- `CAL-V0-058`: Every decision MUST append one `taskman-dispatch-event/0` line to `events.jsonl`
  and print it to stderr as plain language. The event kinds form a closed vocabulary: started,
  stopped, adopted, launched, launch-failed, finished, killing, killed, handoff, handoff-refused,
  reaped, state, claim, release, lane, cooldown, parked, unparked, alert and needs-owner. A
  `finished` event carries the exit code (`NOT_OBSERVED` for an adopted worker), whether progress was
  made, and a bounded summary of the worker's last agent message: the final text of a recognized
  host event stream (OpenCode `run --format json`, Codex `exec --json`, Claude Code `-p
  --output-format stream-json --verbose`, or `json` without `--verbose`, Gemini CLI `-p -o
  stream-json`, whose consecutive assistant delta chunks form one reply, or `json`), ignoring
  subagent messages, otherwise the sanitized output tail. State changes
  compare against the previous observation; the first observation records only a baseline.

Non-goals: readmitting exhausted tickets; creating or cleaning worktrees; any network, account or
hosted service; hostile-process containment; enforcing a host's own permission deny-list; Windows
support (it compiles and refuses); and treating a worker's own report as progress. Failure modes:
a host that ignores SIGTERM is killed after the grace period; a process outside the worker's
session, group and tree escapes supervision; a broken work-state reader makes every state `UNKNOWN`;
and a store read failure skips heal, accounting and launches with an alert, while supervision
continues. Rollback stops the dispatcher. Workers
already launched keep running, and their attempts are released or reaped by the ordinary lease verbs.
Deleting the state directory loses only dispatcher history, never queue state.
Regression witnesses are the CAL-V0-052..058 tests in the traceability table: in `internal/tasks/dispatch`, `TestCALV0052_DecodeConfigIsClosedAndBounded`,
`TestCALV0052_RenderIsSinglePass`, `TestCALV0053_WorkStateReaders`, `TestCALV0054_RosterIsDeterministicAndCapped`,
`TestCALV0054_RosterStatePredicatesAndLanes`, `TestCALV0055_LaunchFinishBackoffAndPark`, `TestCALV0056_HandoffAndReap`,
`TestCALV0056_KillsWholeTreeAndAdoptsAcrossRestart`, `TestCALV0056_KillsOrphanedProcessesBySession`,
`TestCALV0056_IdentityOutageAndUnknownState` and
`TestCALV0057_FingerprintIgnoresNonDurableAttempts` and `TestCALV0058_SummaryReadsHostFinalText`; and
`TestCALV0052_DispatchCLIClaimHandoffAndStatus` (`internal/tasks/cli`).

### S12 — Read cost independent of receipt history (issue 446)

Authoritative input: owner request [issue 446](https://github.com/beamfall/corvint/issues/446).
Before this slice every read command replayed the whole receipt chain, and `queue status` and
`plan preview` did so more than once, so a read cost seconds at about 1,800 receipts and held
the files a concurrent writer wanted. The journal stays the only authority. This slice changes
how much of it a read must replay, never what a read may conclude.

- `CAL-V0-059`: The audit checkpoint is derived state with profile `taskman-audit-checkpoint/0`,
  stored as `<state directory>.checkpoint.json` beside, never inside, the journal state
  directory, so state scans, archive export and older runtimes do not see it. It records the
  queue ID, primary worktree, init digest, generation, semantic coverage, the sequence and
  digest of one receipt, and for every non-request path the sequence and digest (or retained
  deletion) of its latest canonical afterimage at that sequence, strictly path-ordered. The
  codec is closed and bounded (16 MiB; entry bound derived from the existing scan, intent,
  ticket and release limits). A checkpoint MUST be derived only from a complete, settled,
  consistent `FULL` audit whose last receipt is the head; never from a checkpoint-resumed audit
  and never over a pending receipt. It carries no request, evidence or receipt bytes, is never
  posted by a receipt, and is never an input to authority, ranking or archive content.
  Deleting it costs the next read one complete audit and nothing else.
- `CAL-V0-060`: Only a writer retains a checkpoint: after its complete settled audit, while it
  holds the writer lock and the head is still the audited one, by one fixed temporary file and
  rename, best-effort. Audits that read verbs share with writers MUST NOT retain one. A failure to retain it MUST
  NOT fail or change the transaction. Read commands MUST NOT create, replace or remove the
  checkpoint (product invariant 4). The retained checkpoint therefore names the head the writer
  observed before its own receipt, and a later read replays at least that receipt.
- `CAL-V0-061`: A read command MUST perform at most one journal audit and derive every
  projection it prints from that one observation. Where a checkpoint is present, a read MAY
  resume from it: it MUST confirm the queue ID, primary worktree, init digest and generation
  and the head's version digest; confirm that the checkpoint sequence does not exceed the head
  and that the named receipt still hashes to the recorded digest and carries the recorded
  sequence and generation; replay every receipt after it through the
  head with the same per-receipt validators as the complete audit; and verify every non-request
  projection, staging emptiness and the intent tree against the resulting afterimages exactly as
  the complete audit does. The result reports `journalAudit` `CHECKPOINT_PLUS_TAIL` and
  structural consistency `CHECKPOINT_PLUS_TAIL`, never `CONSISTENT`. Any decode error,
  mismatch or refusal on that path MUST fall back to the complete audit, whose verdict is the
  one reported; a head that moves between captures is retried at most four times first. An
  unusable checkpoint therefore never produces a refusal, a different projection or a weaker
  verdict than no checkpoint. Every mutation, barrier removal, reconciliation, request lookup
  and `receipt audit` MUST keep the complete audit (`journalAudit` `FULL`).

The checkpoint has the same local trust as the journal directory it sits beside and no more:
its entries and semantic coverage are not re-derived from the receipts before its sequence. A
party that rewrites a projection and the matching checkpoint entry together, or adds a stray
intent file with a matching entry, is therefore not detected by a resumed read; the complete
audit refuses it. Detection limits of a checkpoint-resumed read, each of which the complete
audit still covers on `receipt audit` and on every mutation: a checkpoint and projection altered
together as above; an altered receipt before the checkpoint sequence; a
stray receipt beyond head+1; altered or stray files under `requests/` and `evidence/` that the
replayed tail does not post; duplicate request IDs against the prefix; and stray files in
directories the resumed read does not list. The inventory digest of a resumed read differs from
the complete audit's; only the head and intent-tree digests are comparable between them.

Non-goals: accelerating writers (they keep the complete audit; ticket V1-0645); a
`queue status --summary` flag (the default read is now fast; ticket V1-0647 asks whether it is
still wanted); removing the remaining per-read intent-tree passes (ticket V1-0646); any daemon,
database or cache that a read mutates; and treating the checkpoint as evidence of anything.
Failure modes are in the table below. Rollback: delete `<state directory>.checkpoint.json` to
force complete audits until the next write, or revert the reader option; no journal, intent or
archive format changes, and older runtimes ignore the file.

Measured on the live store at 1,829 receipts (macOS, Go 1.27.1, warm cache): `queue status`
4.5–4.9 s before; 1.08–1.43 s with one complete audit; 0.24–0.25 s resumed from a checkpoint
(journal audit about 83 ms). `plan preview` 1.18–1.20 s complete, 0.25 s resumed. The remaining
cost is proportional to the intent tree and the number of retained paths (including retained
deletions), not to the number of receipts. See
`docs/build-log/2026-10-01-tasks-read-checkpoint.md`.

### S13 — Supervised effort and stage wall (issue 354, partial)

Authoritative input: owner request [issue 354](https://github.com/beamfall/corvint/issues/354),
the S10 supervisor's fixed `effort: "low"` and `wallSeconds` 1..3600 config bounds, and native
ticket V1-0475 criteria 4 and 5. This slice delivers only owner-bounded effort and a longer
policy-bounded stage wall for the existing `taskman-codex-supervisor/0` Codex supervisor.
Multi-repository programs, non-Codex supervisor adapters and checkpointed continuation beyond the
existing WAIT/resume path remain open under V1-0475. The continuous dispatcher (S11) is unchanged:
its host argv already carries any effort flag and its role `wallSeconds` already reach seven days.

- `CAL-V0-062`: The optional policy `supervision` object MAY carry `efforts`, a closed object whose
  keys are a nonempty subset of `implement`, `review` and `integrate`, each a nonempty,
  canonical-byte-sorted, duplicate-free array of `low`, `medium` and `high`. A stage without an
  entry, or a policy without `efforts` or `supervision`, admits only `low`, so existing policy bytes
  keep their meaning. The supervisor config MAY carry `stageEfforts`, a map from those stages to an
  effort overriding `effort` for that stage. A new program MUST be refused, before its runtime read,
  program record, worktree, effect or host process, when its config names an unknown stage or any
  stage effort the policy does not admit. An existing program MUST be re-checked against the current
  policy before every stage launch, so a later narrowing refuses further stages without blocking
  `drain` or `cancel`. The admitted stage effort MUST be
  the `model_reasoning_effort` of both new and resumed Codex invocations for that stage.
- `CAL-V0-063`: The optional policy `supervision.stageWallMinutes` (Count 1..240, the lane
  `wallClockMinutes` ceiling) MUST bound the config `wallSeconds` to 1..`stageWallMinutes`×60;
  absent, the bound stays 1..3600. A policy value outside 1..240 MUST be refused with
  `LIMIT_EXCEEDED`, because no stage can outlast the lane cap. A config outside the bound MUST be
  refused at the same points as CAL-V0-062. The active stage deadline remains the minimum of
  `wallSeconds`, the policy lane `wallClockMinutes` and the program's remaining
  `supervision.program.wallClockMinutes`, so the longest reachable stage is four hours and a stage
  above one hour also needs those caps raised. Heartbeat
  renewal, WAIT handoff on expiry and session resume are unchanged (CAL-V0-038, CAL-V0-039).

Non-goals: efforts beyond `low|medium|high` (for example Codex `minimal` or `xhigh`); per-role
models; proof that the provider applied the requested effort, which stays `NOT_OBSERVED` beyond the
argv the supervisor passed; and any change to token accounting. Failure modes: a policy that admits
`medium` only for `implement` refuses a config whose default `effort` is `low` (the default applies
to every stage); a lane cap shorter than `wallSeconds` silently shortens the stage, as before.
Rollback removes `efforts` and `stageWallMinutes` from the policy, which restores the low-only,
one-hour behaviour for every later dispatch; recorded program configs keep their digests because
`stageEfforts` is omitted when absent. Regression witnesses: `TestCALV0062_PolicyEffortAllowlist`,
`TestCALV0063_PolicyStageWallBound` (`internal/tasks/intent`); `TestCALV0062_StageEffortSelection`,
`TestCALV0062_CheckProgramConfigEffort`, `TestCALV0062_StageRechecksCurrentPolicy`,
`TestCALV0063_CheckProgramConfigStageWall` and
`TestCALV0062_OpenWorkflowRefusesBeforeMutation` (`internal/tasks/store`). Live Codex qualification at
a non-low effort is `NOT_RUN`; see `docs/build-log/2026-10-01-tasks-supervisor-effort-wall.md`.

### S14 — Explicit command progress (issue 468)

Human-owned input: [issue 468](https://github.com/beamfall/corvint/issues/468) requests
a bounded explicit command token independent of role matching. This is a proposed
technical contract. The original scoped source has separate reviewed and sealed
evidence; this intent-only seed makes no current-main integration, Linux, installed
runtime, native completion or new delivery claim. CAL-V0-062/063 are defined separately in S13.

- `CAL-V0-064`: A command work-state reader MAY return a legacy state string or a closed object
  with required string `state` and optional string `progress` per ticket. State retains CAL-V0-053's
  bounds and is the only role-matching value. The optional progress token is byte-opaque printable
  UTF-8, at most 128 bytes. Missing or empty progress makes no additional claim. Full ticket ID takes
  precedence over local name, including an empty full-ID state normalized to `NONE`; state and token
  MUST come from the same selected value. Ordinary JSON whitespace, key order and valid escapes
  remain compatible. Duplicate ticket/member keys, unknown object members, null or non-string
  values, invalid UTF-8, lone surrogate escapes and trailing JSON MUST fail as ordinary reader
  errors; valid surrogate pairs are retained. Unknown-ticket tokens never consume history.
  The dispatcher retains only SHA-256 digests in private per-ticket history. First valid token seeds
  a baseline without credit. A never-observed digest advances once; a current duplicate, observed
  A-to-B-to-A replay or missing token keeps the last accepted digest. UNKNOWN, read failure and
  cancellation before admission change no token history. This is a producer assertion, not artifact
  authentication: an unseen old assertion cannot be recognized as stale.
  History is bounded to 256 lifetime distinct digests per key and 8,192 per program, including first
  seeds and deleted/completed keys. New slots are allocated in canonical full-ticket-ID byte order.
  At either cap, retain history, admit no new token, emit a bounded needs-owner diagnostic, and keep
  ordinary cooldown/parking. No eviction, reset or operator-unpark capacity restoration is allowed.
  The one checked admission barrier uses the final successful observation after supervision/heal
  and re-observation, before accounting/unpark/state publication/launch. It stages cloned history,
  legacy first-seed baselines and token-dependent accounting, checks the outer context, and MUST
  save atomically before publishing or granting effects. Save failure discards staging and returns
  an explicit tick error; Close/deferred saves MUST NOT persist failed staging or overwrite successful
  admission with a captured old ledger. A token grant for an ended worker commits its removal and
  backoff deletion together; a parked-key grant commits its backoff deletion with consumption.
  Active-worker credit remains pending relative to its launch digest. A token-enabled ended worker
  with pending credit and UNKNOWN latest state retains worker/backoff accounting, with a bounded
  alert, until a healthy observation grants once. Tokenless behavior remains CAL-V0-057. Cancellation
  after commit retains completed facts and stops downstream work at the next checkpoint; no
  whole-tick rollback is promised. Strict ledger loading validates full ticket keys, digest grammar,
  sorted uniqueness, current membership, both caps and worker/backoff baseline-history consistency.
  Any case-folded root progress member enables strict validation before struct decoding. Token-enabled
  ledgers reject duplicate members and aliases of canonical static schema fields; dynamic ticket and
  observation-map keys retain their case-sensitive identities. Programs admitting no tokens omit
  optional fields, preserve legacy field matching, and retain legacy fingerprints/member shape.


Failure modes: producer tokens do not verify work, lifetime exhaustion can eventually permit parking,
and atomic rename gives process-restart visibility, not power-loss durability or exact event delivery.
Non-goals: native handoff/evidence wire changes, evidence fetching, progressPaths, changed role rules,
automatic migration, indefinite retention capacity or fixing all legacy ledger I/O failures.
Rollback preserves the current ledger and uses backups only as evidence. An older reader refusing new
members is a valid fail-closed downgrade; never restore an older snapshot, strip history or reset it.
Final integration acceptance requires fresh parser/role/token/replay/restart/capacity/checked-save
and cancellation witnesses on the actual composed target, the original four check argv,
scoped registry checks, CEM/OCM, independent composition inspection, public integration
and supported native completion. No current-main integration evidence is produced by this seed.


Original reviewed source fbc80a5e1de6261ea1ce5290a4aa6451fd0c0b2f is unchanged in this
composition. Historical binds 5565697748a56c285a2e30d747a05d70c04d0df3 and
e359cc16c7b9c4e1bd5b025b0ea815b3173cdd49 and pure seals 95b7d5a/021cf0f4 remain
in ordinary public ancestry. PR491 passed Linux CI on main094, with tested tree7df05582;
that success does not qualify current def2a85a composition. Fresh CEM/frozen checks,
independent composition inspection, combined CI and native completion remain pending.
The owner-authored issue permits any one signal and explicitly names the chosen object
option; detailed technical CAL064 remains proposed, with no new ratification claim.

### S15 — Explicit pool member exclusions (issue 480)

Human-owned input: [issue 480](https://github.com/beamfall/corvint/issues/480) permits
the per-claim exclusion alternative. CAL-V0-065 was seeded before implementation
enrollment. Scoped focused tests, a compiled disposable native fixture and
independent source review passed; broad runtime qualification is NOT_OBSERVED.

- `CAL-V0-065`: CLAIM, CLAIM_NEXT and read-only plan preview MAY accept an opt-in bounded set of explicit pool member exclusions. A supplied set MUST require an explicit pool, be nonempty and contain at most 256 sorted unique valid member labels. CLI repeated single-value `--exclude-member` flags MUST normalize order and duplicates while rejecting missing/empty values; other repeated single-value flags retain their existing refusal. Canonical request preimages MUST omit the new field entirely when absent, preserving historical bytes. Shape, syntax, canonical order and absolute bound checks MAY precede authoritative request replay; current-policy member/count eligibility MUST apply only to fresh admission after that replay lookup. An identical receipt-bound claim MUST return its original allocation after release, successor allocation or a permitted policy change, and a changed valid exclusion set under the same request ID MUST conflict before current eligibility checks.
  Fresh explicit/next claim, every health-selection round, final prepared-allocation admission and preview capacity MUST apply the same stage/order/occupancy/exclusion predicate. Current requested-pool membership MUST be checked before any health preparation. Excluded members MUST never be allocated or probed, including matching-stage reservations and unreserved fallback; otherwise eligible members retain existing deterministic tier and member order. A matching health observation MUST NOT bypass final exclusion validation. No eligible member MUST produce RESOURCE_COLLISION rather than ignored exclusions or fallback to an excluded member. Preview MUST write no receipt, projection, trace or probe state. Ordinary claim resource scope and requiresPool remain binding; CAL-V0-029/030/032/034/046 safety and historical replay rules are unchanged. Exclusions are caller-selected member facts, not automatic ticket-history discovery, authenticated reviewer identity or proof of distinct physical environments.

Failure modes: excluded reserved member/busy remainder; malformed or foreign member; preparation/admission policy drift; replay under changed policy; excluded health-start bypass; caller assumes labels authenticate independence. All remain explicit refusal/uncertainty, never ignored constraints or safe reuse inference.

Acceptance evidence: focused transaction/store/CLI tests passed for the fixed
historical preimage/digest witness, shape and membership validation, allocation
order, preview capacity and purity, prepared admission, health filtering,
both claim-next selectors, explicit/next replay after successor and policy changes,
and both CLI parsers. `TestCALV0065_NativeFixture` builds and runs the candidate
executable against a disposable native store, preserving ordinary quarantine.
Independent source review of the frozen twelve-path implementation passed with no
P1/P2 finding. Existing pool/quarantine/stage-order tests and three-package vet passed.

Limits: health-backed CLAIM_NEXT with exclusions and concurrent policy change
between health preparation and final admission were inspected in source rather
than executed as combined fixtures. The historical preimage control is an
independently retained literal from the old source; a separate baseline executable
measurement is NOT_EXECUTED. Native journal audit establishes structural consistency
and projection agreement, with semantic coverage UNKNOWN and runtime qualification
NOT_OBSERVED. Exclusions never authenticate a holder or establish physical independence.

Non-goals: automatic history inference; per-pool independentStages policy; holder authentication; new physical access broker; issue479 terminal fast release; shrinking complete effect/resource intent. Rollback: opt-in command support can be reverted only with current request/profile compatibility limits retained; no projection stripping, historical-request rewriting or unsafe pool state migration. Absent requests remain exact old bytes.

### S17 — Operator-attested untouched pool release (issue 479, proposed)

The owner-authored issue 479 accepts an explicit attestation alternative. This candidate
intent follows the bounded Gate A R1 PASS at proposal SHA-256
`23269d211a3c2b1b4acef38cb08aa0392634d35ba3b141abb840f29242207198`.
The coordinator assigned CAL-V0-067/S17 before this intent seed.
Implementation, tests and native qualification are NOT_RUN.

- `CAL-V0-067`: RELEASE MAY accept an explicit `--lane-untouched --evidence REF` opt-in under
  a separately labelled local OWNER/OPERATOR attestation profile. It MUST retain four fixed true
  acknowledgements: no physical lane access occurred, no lane command was issued, no physical
  lane capability/resource was issued or remains retained, and the operator accepts responsibility
  for the statement and safe reuse. Logical source/PATH reservations are distinct from those physical
  resources. REF MUST be a required valid inert Identifier, never fetched or executed. Recorded actor
  identity is not authentication; physical non-use/revocation remains NOT_OBSERVED. No broker,
  implicit exemption, arbitrary checker command or automatic history discovery is introduced.
  Fresh opt-in MUST require exact current policy/config identity, even when ordinary
  CAL-V0-044/046 handoff could accept the issue 482 compatibility proof. The flagged profile
  MUST be excluded from HandoffPolicyCandidate fallback; a compatible history observation
  MUST NOT authorize its physical reuse exception. Ordinary compatible handoff and its
  quarantine remain unchanged. Original flagged request replay keeps its existing precedence.
  Fresh eligibility MUST require the exact live/unexpired current external-agent RUNNING generation,
  unchanged acceptance/policy/member definition and complete holder/stage/allocation tuple, a matching
  ALLOCATED entry, and a prospective writer-produced `taskman-direct-pool-admission/0` witness.
  Only upgraded fresh direct no-health CLAIM/CLAIM_NEXT admission MAY mint that witness; it binds
  original admission sequence, attempt/generation and the full allocation tuple. No prepared/health
  origin, legacy/backfilled witness or retry inheritance qualifies. Current allocation and pool changed
  sequences and Lease.GrantedSeq MUST match original admission; renewed leases are ineligible.
  Started/pending/unknown/interrupted command history, runner identity, worker/spawn/supervision/lane
  identity, candidate/gate/reviews/manifest, failed-or-unknown retry accounting and pending effects MUST
  refuse without freeing. Null command metadata is not authority. Programs bytes MUST be decoded
  against the same inventory/head/queue; absence qualifies only when the inventory proves absence.
  Matching CurrentAttempt/CurrentGeneration, including ADMITTED before ATTACH with zero leader PID,
  and ambiguous same-attempt generation associations MUST refuse regardless of phase or OwnerReleased.
  Missing, unbound, digest-mismatched, malformed, unknown, duplicate or foreign Programs input MUST
  refuse. Private Dispatcher records are external/non-native and MUST NOT be reported as scanned;
  known or uncertain external use prevents the operator from honestly making the acknowledgements.
  An eligible opt-in MUST atomically retain a closed `taskman-lane-untouched-attestation/0` terminal
  record, end the generation with logical FENCED quiescence, remove its ordinary reservation and omit
  only its exact current occupancy, without executing configured cleanup. The attestation MUST bind
  original allocation/member/definition/allocated sequence, attempt/generation/holder/stage, actor role
  and ID, recorded sequence/time, REF, profile and the four acknowledgements. It MUST NOT claim physical
  cleanup or PROVED quiescence. Default release/handoff, expiry/reap and completion retain quarantine.
  Request shape and static field validation MAY precede authoritative replay; fresh current eligibility
  MUST follow it. New flag/evidence/profile acknowledgements join the request preimage conditionally;
  absence preserves historical request, attempt and ordinary result bytes. Actually changed named
  fields under one request ID MUST conflict. Fresh/replay opt-in reports MUST reconstruct the original
  RELEASE receipt's terminal attempt, inline or bounded blob, verify canonical payload digest and full
  receipt/actor/profile/evidence/tuple bindings, and return that original allocation and attestation.
  Successor/current policy state MUST NOT substitute for original payload; missing/damaged payload
  MUST refuse. Crash/redo and archive round-trip MUST preserve complete afterimages and evidence;
  a member MUST NOT become free with a live logical attempt or absent attestation. Legacy/new-reader
  and old-reader refusal limits MUST remain explicit. HANDOFF/REVIEW_RETURNED accounting still applies
  independently; this profile grants no retry refund, completion, review or integration authority.

Qualification MUST include the configured-cleanup/no-health true-native positive fixture, default
quarantine/missing-cleanup confirmation controls, legacy/renewal/expiry/wrong-role/history/state/tuple
negatives, ADMITTED-before-ATTACH controls, original-payload replay after successor/policy change,
inline/blob damage refusal, archive byte preservation and crash/redo all-or-nothing afterimages.
Focused snapshot/transaction/store/CLI tests and vet, independent implementation and acceptance review,
CEM/OCM frozen checks with truthful unknowns, CI/integration and native completion are required.
Rollback stops future opt-in use while preserving witness/attestation history and exact replay;
retain a compatible reader, never strip metadata, silently downgrade or rewrite successors.

## Amendments to TCP-00

Accepting this spec accepts these amendments; each keeps the existing ID space.

- A18: CAL-V0-045 raises the admitted retry bound to 16 without changing legacy value-3
  semantics. CAL-V0-046 adds absent-only optional `handoffEvidence` to the closed attempt codec
  and conditional evidence to RELEASE preimages. Existing absent-member bytes are unchanged.

- A17: CAL-V0-044 adds `HANDOFF` and `REVIEW_RETURNED` to TCP-00 §11's closed detail
  codes for the verified release requests and recorded dispositions it defines. Together with
  the original 69 codes, the extended set contains 71; neither code alone grants an exemption.

- A16: S10 adds the named supervised branch, optional supervision/role fields and `programs.json`.
  Program-only LEASE posts admit one bounded projection plus retained request/output evidence;
  existing operation limits and external-agent semantics otherwise remain in force. Rollback requires
  drained proved sessions and retained/migrated supervised records; an old reader must not silently
  discard these fields. Read commands remain nonmutating.

- A15: issue 342 adds S9's optional policy/ticket/attempt fields and the bounded `pools.json`
  projection. S9 opt-in health/cleanup signals its own trusted command process group; it does not
  control external agents. LEASE staging expands to 11 artifacts, three blob afterimages and
  a 2658-byte descriptor (shared temporary descriptor cap); other operation limits stay unchanged.
  Pool-only TRANSITION receipts have no attempt/generation when none exists yet.
  Observation, cleanup, recovery and safe confirmation are cancellation-class writes permitted
  under an ALL barrier; preparation and admission remain blocked. Prior omitted-field /0 bytes
  remain valid; old readers cannot consume new records. Pool-aware rollback requires stopping
  claims, resolving quarantine and a recorded safe migration, not merely installing an old binary.

- A8: runtime `external-agent`. An attempt with this runtime has `supervisor` and `lane` null in
  every generation, never has a `PROCESS_SPAWN` effect, and is exempt from the §6.4 rows.
- A9: `taskman-attempt/0` gains `lease:{holder, grantedSeq, expiresAt}|null`, non-null exactly for
  `external-agent` attempts. A lease receipt (`claim`, `renew`, `release`, `reap`, `widen`,
  `submit`, `gate run`) has `ticketId` null and names its attempt by `attemptId` and `generation`,
  because TCP-00 binds a ticket afterimage to every completed receipt that names a ticket, and a
  lease writes no ticket file. A completed `complete` receipt is the exception: it writes the
  ticket file, so it names the ticket and carries its resulting revision like any ticket
  mutation.
- A10: cause `LEASE_EXPIRED` joins the closed cause set, and quiescence `FENCED` covers a generation
  closed by `release`, `reap` or `complete`: no command of that generation can take effect after it.
- A11: for a queue whose admissions are all `external-agent`, the §7.4 execution permission needs a
  `QUALIFICATION` receipt for the CAL-V0-019 suite in place of G2's supervisor and spawn rows and G3.
- A12: an `external-agent` attempt carries `scope:{source:"DECLARED"|"REQUESTED"|"DERIVED"|
  "WHOLE_REPOSITORY", resources:[Resource], derivationSha256:Digest|null}`, non-null exactly for
  that runtime, with `derivationSha256` non-null exactly for `DERIVED`. A ticket without `QUALIFIED`
  coverage is admissible under such a scope regardless of the policy's `serialFallback`, because
  the scope is enforced at submit (CAL-V0-024).
- A13: S5 for `external-agent` attempts. A `gate run` supports `COMMAND` gates with an expected
  exit code, `cwd` `WORKTREE`, no reducer, no `sharedResource`, no declared `inputs` and no
  evidence label beyond the captured output; any other gate refuses `UNSUPPORTED` before it runs.
  The gate runs in the caller's worktree (`executedCwd` `WORKTREE`) with only the environment
  names the gate's `env` declares, taken from the caller, so a gate whose commands need `PATH`
  must declare it. The first argv element is resolved on `corvint-tasks`' own `PATH`, not the
  declared one. An interrupt kills the gate's process group and records nothing. The CAL-V0-024
  scope check at `submit` counts only paths that differ from both the base commit's tree and the
  intent branch tip's tree, so a candidate rebased onto a later `main` is not charged with paths
  other tickets merged. The check therefore guards what a lease completion certifies, not the
  branch itself: a path an agent commits straight to the intent branch before `submit` is
  identical at the tip and is not counted. A result's staleness is its tree binding: after a new
  `submit`, earlier results stay in `gateResults` unchanged and count as `GATE_STALE` because
  their `candidateTreeOid` differs, which is how CAL-V0-015's "marks every earlier gate result
  `STALE`" is met without rewriting evidence. A `gate run` repeated under the same request id runs
  the gate again before the replay is found, and the replay returns the original receipt.
  `complete` refuses unless the commit is reachable and carries the candidate tree, the ticket is
  `OPEN` and not held at the acceptance revision the attempt read, the policy is the one the claim
  read, the scope check is `WITHIN`, an `APPROVAL_REQUIRED` ticket has a `COMPLETE` grant at that
  revision, and every required gate (the policy's `required` gates plus the ticket's
  `requiredGates`) has a `PASSED` result at the candidate tree. It completes the ticket `VERIFIED`
  with a `MANIFEST` evidence record; the supervisor, lane, spawn and review checks of §7.3 do not
  apply to this runtime. The lease verbs read Git in the caller's checkout, else the primary
  worktree.
- A14: S1. `init` accepts a queue whose `fixture` is false, with `importMapSha256` and
  `executionCutover` null, so that a non-fixture store exists to write to; a queue that names an
  import map or an execution cutover still refuses `MALFORMED`. Such a store also takes ticket
  `reconcile` and the S2 writer `cutover`. Only S7's `QUALIFICATION` receipt (CAL-V0-020) sets
  `executionCutover`, and `init` still refuses one; until it is set, on a non-fixture queue `claim` and
  `claim --next` refuse `BLOCKED` `CUTOVER_MISSING` for the missing execution cutover, and
  `plan preview` plans each ticket `BLOCKED` with `CUTOVER_MISSING` after `PAUSED` and before
  `BUDGET_UNKNOWN`. CAL-V0-027 extends release mutations, settled release reconciliation and bounded shared staging
  observation to native queues with a null import map, before or after valid execution cutover.
  INIT, claim/plan qualification and cleanup authority remain unchanged.

## Failure modes

| Failure | Effect | Handling |
|---|---|---|
| Holder crashes or abandons its session | Lease stops being renewed | `reap`, or the next colliding `claim`, fails the attempt `LEASE_EXPIRED` and frees the reservation |
| Stale holder keeps working after reap | Edits continue in its own worktree | Every command it sends is `FENCED`; the tree it built can only complete through a new claim |
| Two sessions use one holder label | Both believe they hold it | The label is a display name only; the generation returned by `claim` is the fence |
| Wall clock steps backward | An expired lease could look live | A transaction earlier than the head refuses before writing (CAL-V0-012) |
| A writer waits behind another writer's commit | Its earlier timestamp would look like a backward clock | The writer samples its live clock again against the head it holds (CAL-V0-012) |
| Gate command hangs | Holder waits | The declared gate timeout records `FAILED`; the attempt stays `CHECKING` for another `gate run` or `submit` |
| Candidate rebased before merge | Tree changes | `complete` refuses until the new tree is submitted and gated |
| Cutover interrupted | One receipt either committed or not | A rerun with the same decision replays or commits it |
| Store edited outside corvint-tasks during an import (`git pull`, an editor) | Batches after the first check only the head and the files they post | A ticket the batch posts refuses `INTENT_DIVERGED` and a moved head refuses `SNAPSHOT_MOVED`; other drift is not seen until the next command audits the store (CAL-V0-018) |
| Re-import after cutover | Foreign export disagrees with the published records | `import` refuses the whole export and writes nothing |
| Non-fixture queue before execution cutover | Agents try to claim | `claim` and `claim --next` refuse `BLOCKED` `CUTOVER_MISSING` and `plan preview` plans every ticket `BLOCKED` until `cutover --execution` records a passing qualification run; ticket writes still work |
| Writer killed between its first staged artifact and its head | Staging slots stay behind with no `staging/active.json` | Reads refuse `MALFORMED` `unassigned stage slot` until the next writer. Every ticket, lease and administrative write, and `gate run` before it runs a gate, first removes the orphan slots under the writer lock and redoes a receipt already linked in (CAL-V0-019). Barrier and reconcile writes do not recover: another writer must run first. Slots beside a `staging/active.json` descriptor are active staging, which stays refused `UNSUPPORTED` |
| Derived scope misses a file the agent needs | Agent edits outside its scope | `submit` refuses `OUT_OF_SCOPE`; the agent `widen`s, or releases and reclaims with `--scope` |
| Two disjoint scopes interfere semantically | Each passes alone, the merge breaks | Gates run at the exact rebased candidate tree before `complete` (CAL-V0-016, CAL-V0-017) |
| Context index absent or stale | No derivation | The scope is `WHOLE_REPOSITORY`, which serializes that claim as today |
| Dispatcher crashes or is stopped | Workers keep running unsupervised | The next `dispatch` adopts workers whose recorded identities still match, then supervises and heals them (CAL-V0-056) |
| Dispatched worker loops without progress | Repeated launches spend host budget | Cooldown, then park and `needs-owner` after `parkAfter` runs (CAL-V0-057) |
| Checkpoint absent, corrupt, oversized, foreign or ahead of the head | A read cannot resume | The read runs the complete audit and reports `FULL`; output is otherwise identical (CAL-V0-061) |
| Checkpoint disagrees with a receipt, projection, staging or the intent tree | A resumed read would mis-state the store | The resumed path refuses internally and the complete audit decides the reported verdict (CAL-V0-061) |
| Journal prefix, or a checkpoint entry together with its projection, altered behind a still-matching checkpoint | A resumed read does not see it | `receipt audit` and every mutation run the complete audit and refuse; the read's verdict says `CHECKPOINT_PLUS_TAIL`, not `CONSISTENT` (CAL-V0-061) |
| Writer cannot retain the checkpoint (full disk, permissions, crash before rename) | Reads stay at complete-audit cost | The transaction is unaffected; the next successful writer retains one (CAL-V0-060) |

## Acceptance and rollback

S9 evidence: `TestPoolAllocationQuarantine`, `TestPoolAllocationTupleCorrespondence`,
`TestPoolNoHealthConfigReference`, `TestPoolReplayReturnsOriginalAllocation`,
`TestPoolHealthSkipsFailedMember`, `TestPoolProcessDescendants`,
`TestPoolPlanConsumesEligibleSlots`, and `TestPoolPreviewConsumesMembersWithoutProbes` under
`internal/tasks`. Native disposable queue qualification covers independent concurrent processes,
reserved review capacity, required-pool refusal, read-only occupancy, health skip/pass,
cleanup-before-confirmation, success/timeout/SIGTERM descendants and SIGKILL/replay/recovery.
This qualifies trusted same-process-group commands on the observed native host, not hostile
containment or real deployment isolation. Final frozen enrollment and closeout remain required.

Acceptance evidence, per slice: named `TestCALV0NNN_*` tests for every requirement in that slice
under `internal/tasks`, the unchanged fixture tests for CAL-V0-003, and for S6 the measured first
import of the real export, and for S8 a measurement on the Beamfall fixture store of how many open
core tickets could hold concurrent claims under CAL-V0-023 against the runner's two-lane rule.
Before S7 closes, a rehearsal in a throwaway non-fixture store holding the cut-over Beamfall export
claims, gates and completes one real Beamfall ticket, and lets one lease expire and be reaped.

CAL-V0-027 acceptance uses the compiled disposable non-fixture lifecycle, the existing lifecycle
assertions under both queue profiles, before/after-receipt fault injection, exact replay/conflict,
qualified post-cutover writes, and pending/active reconciliation refusal. Returned publication
faults and reconstructed orphan slots prove the bounded retry path; arbitrary process-crash
recovery is not claimed by this slice. Completed ordinary mutation and lease observations are
proved against real published artifacts, including gate, manifest and FENCED refusal receipts.
CAL-V0-027 rollback restores the three fixture-only admission boundaries; retain every existing receipt,
release projection and evidence blob. Production migration and concurrent-agent rehearsal remain
separate release obligations; this slice does not switch Beamfall or establish complete takeover.

Rollback, per slice: S1 restores the fixture-only checks in the INIT digest and `validateInput`
(`internal/tasks/transaction/model.go`) and removes the `CUTOVER_MISSING` claim check (a
non-fixture store it initialized then refuses every write until it is removed); S2 removes
`cutover` (a queue it already switched stays `NATIVE`, and its imported records stay eligible
`IMPORT` records; reversing a switch is TCP-00's §5.4 revert, which this spec does not build); S3 to S5 remove the lease verbs, and a
store that holds live `external-agent` attempts must first `release` or `reap` them, because a
rolled-back reader reports them `NOT_OBSERVED`; S6 restores the per-batch audit; S8 removes `widen`
and the scope check, and every claim reverts to `WHOLE_REPOSITORY`; S7 removes the execution cutover
verb, and an owner decision clears `executionCutover` on any queue that has it. Beamfall keeps
`roadmap.sh` untouched until TCP-09, so its runner stays available as the fallback throughout.

## Traceability

| Requirement | Evidence |
|---|---|
| CAL-V0-001 | `TestCALV0001_NonFixtureQueueTakesEveryWrite`, `TestCALV0001_ImportMappedQueueStaysRefused` (`internal/tasks/store`) |
| CAL-V0-002 | `TestCALV0002_NonFixtureQueueRefusesClaims` (`internal/tasks/store`), `TestCALV0002_PlanPreviewBlocksANonFixtureQueue` (`internal/tasks/cli`); the cutover side is `TestCALV0020_ExecutionCutoverAdmitsClaims` (`internal/tasks/store`) |
| CAL-V0-003 | The fixture tests under `internal/tasks` pass unchanged |
| CAL-V0-004 | `TestCALV0004_CutoverSwitchesWriterInOneReceipt`, `TestCALV0004_CutoverRefusals` (`internal/tasks/store`), `TestCALV0004_CLICutoverPublishesImportedRecords` (`internal/tasks/cli`) |
| CAL-V0-005 | `TestCALV0005_ImportAfterCutoverRefusesAndWritesNothing` (`internal/tasks/store`) |
| CAL-V0-006 | `TestCALV0004_CutoverSwitchesWriterInOneReceipt` (imported record bytes unchanged) |
| CAL-V0-007 | `TestCALV0007_ClaimAdmitsOneRunningAttempt`, `TestCALV0007_ClaimRefusesBudgetUnknown` (`internal/tasks/store`) |
| CAL-V0-008 | `TestCALV0008_ClaimNextTakesThePlanInPriorityOrder`, `TestCALV0008_ClaimNextRefusesWithoutACandidate`, `TestCALV0008_ClaimNextReapsEveryExpiredLeaseFirst` (`internal/tasks/store`) |
| CAL-V0-009 | `TestCALV0009_StaleGenerationIsFencedAndRecorded` (`internal/tasks/store`) |
| CAL-V0-010 | `TestCALV0010_RenewExtendsAndIsFencedAfterExpiry` (`internal/tasks/store`) |
| CAL-V0-011 | `TestCALV0011_ExpiredLeaseIsReapedByACollidingClaim`, `TestCALV0011_ReapAndRelease`, `TestCALV0011_ReleaseAndReapPassAnAllBarrier` (`internal/tasks/store`) |
| CAL-V0-012 | `TestCALV0012_LeaseBoundsAndBackwardClock`, `TestCALV0012_BackwardClockRefusesEveryWriter`, `TestCALV0012_WriterBehindNewerHeadSamplesAgain` (`internal/tasks/store`) |
| CAL-V0-044 | `TestCALV0044_CleanHandoffsPreserveRetryDebt`, `TestCALV0044_HandoffNeedsRecordedEligibility`, `TestCALV0044_FailedGateRemainsChargedAfterPassAndSubmit`, `TestCALV0044_ReviewExpiryStillExhausts`, `TestCALV0044_TimeoutRemainsSticky` (`internal/tasks/store`); `TestCALV0044_LegacyReasonCannotExempt` (`internal/tasks/transaction`); `TestCALV0044_AccountingSchema` (`internal/tasks/snapshot`); `TestCALV0044_CLIHandoffAccounting` (`internal/tasks/cli`) |
| CAL-V0-045 | `TestCALV0045_RetryPolicyBounds` (`internal/tasks/intent`); `TestCALV0045_PolicyControlsAdmissionAndRecovery`, `TestCALV0045_RecoveryUsesCurrentPolicy` (`internal/tasks/store`); `TestCALV0045_CLIConfiguredRetriesAndNoTreeHandoff` (`internal/tasks/cli`) |
| CAL-V0-046 | `TestCALV0046_ReleasePreimageCompatibility`, `TestCALV0046_NoTreeEligibilityBindings` (`internal/tasks/transaction`); `TestCALV0046_NoTreeHandoffSchema` (`internal/tasks/snapshot`); `TestCALV0046_NoTreeHandoffAndIntegrate`, `TestCALV0046_NoTreeRefusals` (`internal/tasks/store`); `TestCALV0045_CLIConfiguredRetriesAndNoTreeHandoff`, `TestCALV0046_CLICompatibility`, `TestCALV0046_CLIPoolHandoffQuarantines` (`internal/tasks/cli`) |
| CAL-V0-047 | `TestCALV0047_AllCommandHelpIsReadOnly`, `TestCALV0047_MalformedInputsStillRefuse` (`internal/tasks/cli`) |
| CAL-V0-052 | `TestCALV0052_DecodeConfigIsClosedAndBounded`, `TestCALV0052_RenderIsSinglePass` (`internal/tasks/dispatch`); `TestCALV0052_DispatchCLIClaimHandoffAndStatus` (`internal/tasks/cli`) |
| CAL-V0-053 | `TestCALV0053_WorkStateReaders` (`internal/tasks/dispatch`) |
| CAL-V0-054 | `TestCALV0054_RosterIsDeterministicAndCapped`, `TestCALV0054_RosterStatePredicatesAndLanes` (`internal/tasks/dispatch`) |
| CAL-V0-055 | `TestCALV0055_LaunchFinishBackoffAndPark` (`internal/tasks/dispatch`); live OpenCode run in `docs/build-log/2026-10-01-tasks-continuous-dispatch.md`; live Claude Code and Codex runs in `docs/build-log/2026-10-01-tasks-dispatch-claude-code.md` and `docs/build-log/2026-10-01-tasks-dispatch-codex.md` |
| CAL-V0-056 | `TestCALV0056_HandoffAndReap`, `TestCALV0056_KillsWholeTreeAndAdoptsAcrossRestart`, `TestCALV0056_KillsOrphanedProcessesBySession`, `TestCALV0056_IdentityOutageAndUnknownState` (`internal/tasks/dispatch`); `TestCALV0052_DispatchCLIClaimHandoffAndStatus` (`internal/tasks/cli`) |
| CAL-V0-057 | `TestCALV0057_FingerprintIgnoresNonDurableAttempts`, `TestCALV0055_LaunchFinishBackoffAndPark` (`internal/tasks/dispatch`) |
| CAL-V0-058 | Event assertions in `TestCALV0055_LaunchFinishBackoffAndPark`, `TestCALV0056_HandoffAndReap`, `TestCALV0056_KillsWholeTreeAndAdoptsAcrossRestart`, `TestCALV0058_SummaryReadsHostFinalText` (`internal/tasks/dispatch`) and `TestCALV0052_DispatchCLIClaimHandoffAndStatus` (`internal/tasks/cli`) |
| CAL-V0-059 | `TestCALV0059_CheckpointCodecAndDerivation` (`internal/tasks/journal`) |
| CAL-V0-060 | `TestCALV0060_WritersRetainACheckpointReadsResumeFromIt` (`internal/tasks/cli`), including `pending`, which shares the lease audit with writers |
| CAL-V0-061 | `TestCALV0061_CheckpointTailEqualsFullAudit`, `TestCALV0061_CheckpointFallsBackToFullAudit`, `TestCALV0061_CheckpointScopeAndMovement`, `TestCALV0061_CheckpointLimitsStayWithFullAudit` (`internal/tasks/journal`); `TestCALV0060_WritersRetainACheckpointReadsResumeFromIt` (`internal/tasks/cli`); live-store measurement in `docs/build-log/2026-10-01-tasks-read-checkpoint.md` |
| CAL-V0-062 | `TestCALV0062_PolicyEffortAllowlist` (`internal/tasks/intent`); `TestCALV0062_StageEffortSelection`, `TestCALV0062_CheckProgramConfigEffort`, `TestCALV0062_StageRechecksCurrentPolicy`, `TestCALV0062_OpenWorkflowRefusesBeforeMutation` (`internal/tasks/store`); live Codex at non-low effort NOT_RUN |
| CAL-V0-063 | `TestCALV0063_PolicyStageWallBound` (`internal/tasks/intent`); `TestCALV0063_CheckProgramConfigStageWall`, `TestCALV0062_OpenWorkflowRefusesBeforeMutation` (`internal/tasks/store`); live stage beyond one hour NOT_RUN |
| CAL-V0-064 | `TestCALV0064_CommandGrammarAndRoleSeparation`; `TestCALV0064_ChangedFileReplayAndRestart`; `TestCALV0064_CheckedSaveFailureDoesNotGrantOrEscapeThroughClose`; `TestCALV0064_LaterSaveFailureCannotReviveGrantedParking`; `TestCALV0064_ActivePendingUnknownAndSeedAccounting`; `TestCALV0064_FirstSeedIsNotProgressAndCancellationIsNotAdmission`; `TestCALV0064_FirstSeedEndedWorkerAndLaterFailure`; `TestCALV0064_CanceledReobservationCannotAdmitEarlierToken`; `TestCALV0064_PostCommitCancellationPreservesFactsAndStopsEffects`; `TestCALV0064_CapacitySortedAllocationAndStrictLoad`; `TestCALV0064_NoTokenPreservesLegacyLedgerAndFingerprint`; `TestCALV0064_LedgerCanonicalFieldsAndCaseSensitiveKeys`; `TestCALV0064_KeyBoundaryAndOperatorUnparkRetainLifetimeBudget` (`internal/tasks/dispatch`); `TestCALV0064_DispatchCLIFileProgressAndReplay` (`internal/tasks/cli`); manual source/test evidence in `docs/build-log/2026-10-02-dispatch-explicit-progress.md`, optional OCM linkage unassessed |
| CAL-V0-065 | `TestCALV0065_AbsentPreimage`, `TestCALV0065_RequestShapeAndCurrentMembership`, `TestCALV0065_AllocationPreviewAndPreparedAdmission` (`internal/tasks/transaction`); `TestCALV0065_HealthFiltersEveryRound`, `TestCALV0065_ReplayAfterSuccessorAndPolicyChange`, `TestCALV0065_ClaimNextSelectors` (`internal/tasks/store`); `TestCALV0065_CLIExclusionsAndPreviewPurity`, `TestCALV0065_NativeFixture` (`internal/tasks/cli`); scoped evidence and limits in `docs/build-log/2026-10-02-tasks-member-exclusions.md` |
| CAL-V0-013 | `TestCALV0013_RetryAsNextGenerationUpToThree` (`internal/tasks/store`) |
| CAL-V0-014 | `TestCALV0014_PlanPreviewIsAPurePriorityFirstPlan`, `TestCALV0014_SelectedOnlyPlanPreviewIsComplete` (`internal/tasks/cli`); `plan preview` in `TestTMV0008_AS07_ReadsLeaveStoreByteIdentical` (`internal/tasks/cli`) |
| CAL-V0-015 | `TestCALV0015_SubmitRecordsTheCandidateTree` (`internal/tasks/store`) |
| CAL-V0-016 | `TestCALV0016_GateRunRecordsEachResult`, `TestCALV0016_GateRunRefusesWithoutRunning` (`internal/tasks/store`), `TestCALV0016_GateResultRoundTrips`, `TestCALV0016_PassedNeedsACleanExitAtTheCandidate` (`internal/tasks/snapshot`) |
| CAL-V0-017 | `TestCALV0017_CompleteVerifiesTheTicket`, `TestCALV0017_CompletionRefusals`, `TestCALV0017_CompletionNeedsEveryRequiredGateAndApproval` (`internal/tasks/store`), `TestCALV0017_ManifestRoundTrips` (`internal/tasks/snapshot`); live CLI run in `docs/build-log/2026-09-27-corvint-tasks-lease-gates.md` |
| CAL-V0-018 | `TestCALV0018_LaterBatchesCheckTheirHeadAndPosts` (`internal/tasks/store`); first import of the 2,894-item Beamfall export in 271 s in `docs/build-log/2026-09-27-corvint-tasks-import-cost.md`, taken at load 37 to 60 on 12 CPUs, so the load condition is NOT_MET; each batch still re-validates every stored ticket in `transaction.Model` |
| CAL-V0-019 | `TestCALV0019_ConcurrentCollidingClaimsAdmitOne`, `TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead`, `TestCALV0019_FencedGenerationCannotMoveOrComplete`, `TestCALV0019_FaultAtEveryArtifactIsAllOrNothing`, `TestCALV0019_KilledWriterRecovers` (`internal/tasks/store`) |
| CAL-V0-020 | `TestCALV0020_ExecutionCutoverAdmitsClaims`, `TestCALV0020_ExecutionCutoverRefusals` (`internal/tasks/store`), `TestCALV0020_CLIExecutionCutover` (`internal/tasks/cli`); rehearsal on the Beamfall export in `docs/build-log/2026-09-27-corvint-tasks-qualification.md` |
| CAL-V0-021 | `TestCALV0021_WholeRepositoryBlocksEverything`, `TestCALV0021_DeclaredNonPathResourcesJoinTheScope` (`internal/tasks/store`) |
| CAL-V0-022 | `TestCALV0022_PackScopeAndAbstention` (`internal/tasks/scopes`), `TestCALV0022_CLIUsesOptInPack` (`internal/tasks/cli`), and `TestCALV0022_NextDerivesOnlySelectedTicket` (`internal/tasks/store`); explicit pack opt-in, conservative defaults |
| CAL-V0-023 | `TestCALV0023_CollisionNormalization` (`internal/tasks/ticket`), `TestCALV0023_CollidingClaimsAdmitOne`, `TestCALV0023_DisjointPathScopesAreBothAdmitted` (`internal/tasks/store`) |
| CAL-V0-024 | `TestCALV0024_SubmitOutsideTheScopeIsRefused` (`internal/tasks/store`) |
| CAL-V0-025 | `TestCALV0025_WidenAddsPathsAndRefusesCollision`, `TestCALV0025_WidenRefusedUnderAdmissionBarrier` (`internal/tasks/store`) |
| CAL-V0-026 | MET on macOS with Go 1.27.1 and `GOMAXPROCS=2`: 3,000 tickets, 20 samples per verb, claim p95 121.136 ms and renew 104.774 ms; every sampled load average below 12 CPUs. `TestCALV0026_VerifiedAuditReuse`, `TestCALV0026_PreparationFailureWaitsForWriter` (`internal/tasks/store`) and `TestCALV0026_ChangeGuardDescriptorExhaustion` (`internal/tasks/authority`) cover cache trust, concurrency and cleanup. Opt-in `TestCALV0026_LockHoldMeasurement` retains 3,000-ticket timings; see `docs/build-log/2026-09-28-corvint-tasks-lease-lock-qualification.md`. |
| CAL-V0-042 | `internal/companionrelease/tasks_archive.go`, companion release `-tasks-only`; `TestTasksArchiveAssembly`, `TestTasksArchiveHelpRefusesOldRuntime`; native archive build retained in change evidence |

| CAL-V0-027 | `TestCALV0027_CompiledNonfixtureReleaseLifecycle`, `TestCALV0027_NonfixtureReleaseBindings`, `TestCALV0027_NonfixtureReleaseReadinessRefusals` (`internal/tasks/cli`); `TestCALV0027_ReleaseAfterQualifiedCutover`, `TestCALV0027_ReleaseInterruptionRecovery`, `TestCALV0027_ReleaseActiveStageAndReconciliation`, `TestCALV0027_ReleaseWrongActor`, `TestCALV0027_ActualCompletedStages` (`internal/tasks/store`); `TestCALV0027_NonfixtureStageBinding`, `TestCALV0027_CompletedStageReceiptKinds`, `TestCALV0027_CompletedStageInnerBindings` (`internal/tasks/snapshot`). |

## Holder, retry and policy observation acceptance


Issue 494 preparation admission has focused and independent source review plus ten reached protocol cases on local Darwin/APFS and Linux/arm64 Colima/tmpfs. Source snapshot `82d8d532f7f537e93af2889d2fc1468a7953e9df0ccc39ff8e690e3c499c9e92` binds those protocol runs. The only later source change repairs the mixed harness replay receipt assumption; the tiny `TestGH494CLIResultContract` passed its parent and eight cases with exact captured identities on source snapshot `e716f41b793d1ad6517eeb47d3601fd227c4ee6357500e50b5fe94a07886a77b`. The original mixed Go test remains failed; independent raw readback establishes 30/30 operations, full 5000→5030→5030 audits, ten fenced attempts, empty reservations and exact-ID replay without changed bytes. No repeated mixed wave is implied. Current-source CAL-V0-026 measurement met the original threshold on local macOS with Go 1.27.1 and GOMAXPROCS=2: 3000 tickets, 20 samples per verb, claim p95 108.467584 ms and renew p95 97.003583 ms. All 105 host-load observations were below 12 CPUs. Raw measurement SHA256 `25b0076542c81b7bcce9c62badbc2f0bb9f18732eb821058218e64036830eecd`, host observation SHA256 `3a29706d5f4abee9d20313fd0f499a326a6495bcf26a4d6aef6b09114c54fd33` and source manifest `e716f41b793d1ad6517eeb47d3601fd227c4ee6357500e50b5fe94a07886a77b` bind this result. Linux and default-parallelism performance remain unqualified. Committed pending-receipt redo and the other four selected checks are recorded through the keyed post-commit plan in the build log; their results must be read from receipts bound to the actual commit. Terminal independent review, CEM/check/seal, integration and native completion remain pending. No mixed-version fairness, CLI arrival order or universal starvation freedom is claimed. See `docs/build-log/2026-10-04-gh494-fair-preparation-admission.md`. Rollback retires owned work before reverting code, while preserving store/journal/intent and inert coordination files.


Tests for `CAL-V0-048`: `TestCALV0048_HeartbeatLegacyRoundTrip`,
`TestCALV0048_HeartbeatFenceReplayAndLeaseInvariant`,
`TestCALV0048_HeartbeatCLIReplayAndFence`, `TestCALV0048_HolderObservationBoundaries`.
Tests for `CAL-V0-049`: `TestCALV0049_ReasonTotalsAndClosedSchema`,
`TestCALV0049_RetryObservationMatchesAdmission`, `TestCALV0049_ChargeReasonPartition`,
`TestCALV0049_RetryReadProjections`, `TestIssue503_ZeroRemainingHandoffAdmission`,
`TestIssue503_JournalAbsentAdmissionUnknown`, `TestIssue503_RecordedAdmissionProvenance`,
`TestIssue503_RetryExplanation`. `CAL-V0-043` refusal diagnostics are covered by
`TestIssue503_ReopenReasonDoesNotAuthorize` and `TestCALV0043_RecoveryExaminesEveryAttempt`.
Rollback of issue 503 removes these additive read fields and diagnostic detail; no stored schema,
retry charging, migration or acceptance reset changes are required.
Tests for `CAL-V0-050`: `TestCALV0050_PolicyShowPureAndUpdateHelp` plus existing policy-update refusal/replay tests.
Tests for `CAL-V0-051`: `TestCALV0051_CreateHelpAndMeaningfulIDs` plus existing allocator/collision tests.
Failure modes retain stale-generation refusals, expired heartbeat replay, backwards clocks,
legacy reason uncertainty, zero-limit initial admission and canonical/version update refusal.
Rollback requires a compatible reader/writer for added optional attempt members; preserve journal
and request bytes, stop admissions before replacing a writer, and never silently downgrade over
records an older closed codec cannot read. Focused qualification establishes these disposable
seams only; repository-wide gate, production process liveness and hosted outcomes remain unclaimed.
