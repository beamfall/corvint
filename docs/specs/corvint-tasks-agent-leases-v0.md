# Corvint Tasks agent leases V0

Owner: Russell Lewis
Date: 2026-09-27 (accepted the same day)
Intent status: accepted (owner decision 2026-09-27)
Delivery status: partial (S1 CAL-V0-001..003, S2 CAL-V0-004..006, S3 CAL-V0-007 and 009..013, S4 CAL-V0-008 and 014, S5 CAL-V0-015..017 and 024, S6 CAL-V0-018 partial (audit carried; proportional cost and load condition NOT_MET), S7 CAL-V0-019..020, S8 CAL-V0-021..023 and 025 experimental with explicit pack opt-in; CAL-V0-026 MET (GOMAXPROCS=2 qualification); CAL-V0-027 implemented with scoped native release qualification; S9 CAL-V0-028..034 implemented with local native qualification; S10 CAL-V0-035..041 implemented with scoped local Codex qualification)
Authoritative inputs: owner request [issue 342](https://github.com/beamfall/corvint/issues/342) and
owner choice on 2026-09-28 to quarantine environments until confirmed safe reuse; owner request [issue 336](https://github.com/beamfall/corvint/issues/336), the Corvint Tasks contract TCP-00 (`beamfall/corvint-tasks` `docs/SPEC.md`,
§3.4, §4, §6 and §7.4), decision 0397 (corvint-tasks built in tree), decision 0423 A10,
`docs/specs/corvint-tasks-store-init-v0.md`, tickets V1-0398, V1-0184 and V1-0310, and the in-tree
sources under `internal/tasks`.

## Agent digest
- Claim: Agents claim, gate and complete scoped Tasks attempts through external leases or an explicitly enabled Codex supervisor.
- Status: accepted (owner decision 2026-09-27); partial (S1 CAL-V0-001..003, S2 CAL-V0-004..006, S3 CAL-V0-007 and 009..013, S4 CAL-V0-008 and 014, S5 CAL-V0-015..017 and 024, S6 CAL-V0-018 partial (audit carried; proportional cost and load condition NOT_MET), S7 CAL-V0-019..020, S8 CAL-V0-021..023 and 025 experimental with explicit pack opt-in; CAL-V0-026 MET (GOMAXPROCS=2 qualification); CAL-V0-027 implemented with scoped native release qualification; S9 CAL-V0-028..034 implemented with local native qualification; S10 CAL-V0-035..041 implemented with scoped local Codex qualification). Drafted and accepted 2026-09-27 on the owner's request to bring corvint-tasks to a level where it can take over Beamfall's `script/roadmap.sh`.
- Exists: the TCP-00 attempt, reservation and receipt shapes (reserved, no writer), the §5.2 writer for fixture and non-fixture queues, and the CTS-V0-003 shadow import.
- Blocked on: the recovered task-store contract (V1-0310) for the parts of TCP-00 this spec does not restate.
- Read next: Slices; Requirements (S8 for parallel claims; S9 for named pools; S10 for Codex supervision); Amendments to TCP-00; Failure modes.

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
(`internal/tasks/transaction/model.go:560@afae0d34`), and until S1 the writer refused every queue
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
  quiescence `FENCED`, and remove their entries. `claim` MUST reap, in its own transaction and
  receipt, every expired lease whose reservation would otherwise block it. An `ALL` barrier lets
  `release` and `reap` through as it lets `cancel` through (TCP-00 §3.4), and refuses `renew`,
  `claim` and `widen` `PAUSED`.
- `CAL-V0-012`: A lease is `{holder:label, grantedSeq:Size, expiresAt:Timestamp}`. The default is
  60 minutes, the minimum 5 and the maximum 1,440; a request outside that range is `MALFORMED`. A
  transaction of any operation whose `recordedAt` is earlier than the head receipt's refuses
  `STORAGE_FAILED` before it writes, so a clock that steps backward cannot record a receipt that
  later makes an expired lease look live.
- `CAL-V0-013`: A ticket whose last attempt is `FAILED` or `CANCELLED` MUST be claimable again as
  that attempt's next generation (TCP-00 §6.2 `retry`, `retryCount < 3`), and after three retries
  only an `OWNER` `ticket reopen` makes it claimable.
  `corvint-tasks attempt show <attemptId>` and `queue status` MUST report every live attempt with
  holder, phase and lease expiry, as pure reads. `queue status` reports `attempts` as the count of
  live attempts and lists them in `liveAttempts`.

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
  worker.

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
  Allocated state MUST agree with the complete attempt allocation tuple, holder and stage.
  Replayed claims MUST return their original receipt-bound allocation, never a successor's.
- `CAL-V0-030`: Release, expiry/reap and completion MUST quarantine the exact allocation while
  freeing the ordinary scope reservation. A retry MUST acquire a new allocation. Only an
  OWNER/OPERATOR `pool confirm-safe` naming the current allocation, an evidence reference and
  reason MAY clear quarantine. Configured cleanup success is necessary but insufficient: the
  confirmation is a local operator attestation of external revocation/reset, not observed physical
  exclusivity. Stale confirmation MUST refuse. There is no TTL or implicit safe reuse.
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
  preview batch MUST consume eligible free member capacity, excluding other-stage reservations.
  Archive, journal recovery and authority-confined projection publication MUST retain pool state.

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
  claim, exact candidate tree, distinct holder and distinct host session. Returned work retains
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

## Amendments to TCP-00

Accepting this spec accepts these amendments; each keeps the existing ID space.

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
| CAL-V0-012 | `TestCALV0012_LeaseBoundsAndBackwardClock`, `TestCALV0012_BackwardClockRefusesEveryWriter` (`internal/tasks/store`) |
| CAL-V0-013 | `TestCALV0013_RetryAsNextGenerationUpToThree` (`internal/tasks/store`) |
| CAL-V0-014 | `TestCALV0014_PlanPreviewIsAPurePriorityFirstPlan` (`internal/tasks/cli`); `plan preview` in `TestTMV0008_AS07_ReadsLeaveStoreByteIdentical` (`internal/tasks/cli`) |
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
