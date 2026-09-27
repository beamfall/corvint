# Corvint Tasks agent leases V0

Owner: Russell Lewis
Date: 2026-09-27 (accepted the same day)
Intent status: accepted (owner decision 2026-09-27)
Delivery status: partial (S2 CAL-V0-004..006, S3 CAL-V0-007 and 009..013, S8 claim side CAL-V0-021..023 and 025 experimental)
Authoritative inputs: the Corvint Tasks contract TCP-00 (`beamfall/corvint-tasks` `docs/SPEC.md`,
§3.4, §4, §6 and §7.4), decision 0397 (corvint-tasks built in tree), decision 0423 A10,
`docs/specs/corvint-tasks-store-init-v0.md`, tickets V1-0398, V1-0184 and V1-0310, and the in-tree
sources under `internal/tasks`.

## Agent digest
- Claim: Coding agents claim, renew, gate and complete tickets through leased `corvint-tasks` attempts, replacing a repository's own task runner without a supervisor.
- Status: accepted (owner decision 2026-09-27); partial (S2 CAL-V0-004..006, S3 CAL-V0-007 and 009..013, S8 claim side CAL-V0-021..023 and 025 experimental). Drafted and accepted 2026-09-27 on the owner's request to bring corvint-tasks to a level where it can take over Beamfall's `script/roadmap.sh`.
- Exists: the TCP-00 attempt, reservation and receipt shapes (reserved, no writer), the §5.2 writer for fixture queues, and the CTS-V0-003 shadow import.
- Blocked on: the recovered task-store contract (V1-0310) for the parts of TCP-00 this spec does not restate.
- Read next: Slices; Requirements (S8 for parallel claims); Amendments to TCP-00; Failure modes.

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
(`internal/tasks/transaction/model.go:538@afae0d34`), and the writer refuses every queue that is
not a fixture (`internal/tasks/transaction/model.go:732@2635d775`).

The agents that use these queues are not processes corvint-tasks starts. They are interactive or
orchestrated sessions that call the task tool themselves. This spec keeps TCP-00's attempt,
generation, reservation and receipt records and replaces only the spawn and liveness layer: the
calling agent is the runtime, and a lease it renews stands in for process liveness. A generation
number fences every later command from a holder that lost its lease, so a stale agent can go on
editing its own worktree but can neither move its attempt nor complete the ticket.

Non-goals: a supervisor, `lane-leader`, process-group signalling or any §6.4 spawn effect; creating,
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
  evidence.
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

S7, qualification and execution cutover.

- `CAL-V0-019`: A named test suite MUST show, for `external-agent` attempts: two concurrent
  colliding claims admit exactly one; `claim`, `renew`, `reap`, `submit`, `gate run` and `complete`
  racing each other leave one consistent head; a fenced generation cannot move or complete an
  attempt; and a crash at each commit point of every lease verb leaves the whole transaction or
  none of it (the §5.3 crash matrix, lease rows).
- `CAL-V0-020`: `cutover --execution --decision <ref> --qualification <digest>` MUST, under an
  `OWNER` binding, record a `QUALIFICATION` receipt naming the CAL-V0-019 run and set the queue's
  `executionCutover` with that `decisionRef` and the run's digest in `gateEvidence`. It refuses when
  the named run is absent or not passing.

## Amendments to TCP-00

Accepting this spec accepts these amendments; each keeps the existing ID space.

- A8: runtime `external-agent`. An attempt with this runtime has `supervisor` and `lane` null in
  every generation, never has a `PROCESS_SPAWN` effect, and is exempt from the §6.4 rows.
- A9: `taskman-attempt/0` gains `lease:{holder, grantedSeq, expiresAt}|null`, non-null exactly for
  `external-agent` attempts. A lease receipt (`claim`, `renew`, `release`, `reap`, `widen`) has
  `ticketId` null and names its attempt by `attemptId` and `generation`, because TCP-00 binds a
  ticket afterimage to every completed receipt that names a ticket, and a lease writes no ticket
  file.
- A10: cause `LEASE_EXPIRED` joins the closed cause set, and quiescence `FENCED` covers a generation
  closed by `release`, `reap` or `complete`: no command of that generation can take effect after it.
- A11: for a queue whose admissions are all `external-agent`, the §7.4 execution permission needs a
  `QUALIFICATION` receipt for the CAL-V0-019 suite in place of G2's supervisor and spawn rows and G3.
- A12: an `external-agent` attempt carries `scope:{source:"DECLARED"|"REQUESTED"|"DERIVED"|
  "WHOLE_REPOSITORY", resources:[Resource], derivationSha256:Digest|null}`, non-null exactly for
  that runtime, with `derivationSha256` non-null exactly for `DERIVED`. A ticket without `QUALIFIED`
  coverage is admissible under such a scope regardless of the policy's `serialFallback`, because
  the scope is enforced at submit (CAL-V0-024).

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
| Re-import after cutover | Foreign export disagrees with the published records | `import` refuses the whole export and writes nothing |
| Non-fixture queue before execution cutover | Agents try to claim | `claim` refuses; ticket writes still work |
| Derived scope misses a file the agent needs | Agent edits outside its scope | `submit` refuses `OUT_OF_SCOPE`; the agent `widen`s, or releases and reclaims with `--scope` |
| Two disjoint scopes interfere semantically | Each passes alone, the merge breaks | Gates run at the exact rebased candidate tree before `complete` (CAL-V0-016, CAL-V0-017) |
| Context index absent or stale | No derivation | The scope is `WHOLE_REPOSITORY`, which serializes that claim as today |

## Acceptance and rollback

Acceptance evidence, per slice: named `TestCALV0NNN_*` tests for every requirement in that slice
under `internal/tasks`, the unchanged fixture tests for CAL-V0-003, and for S6 the measured first
import of the real export, and for S8 a measurement on the Beamfall fixture store of how many open
core tickets could hold concurrent claims under CAL-V0-023 against the runner's two-lane rule.
Before S7 closes, a rehearsal in a throwaway non-fixture store holding the cut-over Beamfall export
claims, gates and completes one real Beamfall ticket, and lets one lease expire and be reaped.

Rollback, per slice: S1 restores the fixture-only check at
`internal/tasks/transaction/model.go:732@2635d775`; S2 removes `cutover` (a queue it already
switched stays `NATIVE`, and its imported records stay eligible `IMPORT` records; reversing a switch
is TCP-00's §5.4 revert, which this spec does not build); S3 to S5 remove the lease verbs, and a
store that holds live `external-agent` attempts must first `release` or `reap` them, because a
rolled-back reader reports them `NOT_OBSERVED`; S6 restores the per-batch audit; S8 removes `widen`
and the scope check, and every claim reverts to `WHOLE_REPOSITORY`; S7 removes the execution cutover
verb, and an owner decision clears `executionCutover` on any queue that has it. Beamfall keeps
`roadmap.sh` untouched until TCP-09, so its runner stays available as the fallback throughout.

## Traceability

| Requirement | Evidence |
|---|---|
| CAL-V0-001..003 | NOT_RUN; accepted, not started |
| CAL-V0-004 | `TestCALV0004_CutoverSwitchesWriterInOneReceipt`, `TestCALV0004_CutoverRefusals` (`internal/tasks/store`), `TestCALV0004_CLICutoverPublishesImportedRecords` (`internal/tasks/cli`) |
| CAL-V0-005 | `TestCALV0005_ImportAfterCutoverRefusesAndWritesNothing` (`internal/tasks/store`) |
| CAL-V0-006 | `TestCALV0004_CutoverSwitchesWriterInOneReceipt` (imported record bytes unchanged) |
| CAL-V0-007 | `TestCALV0007_ClaimAdmitsOneRunningAttempt`, `TestCALV0007_ClaimRefusesBudgetUnknown` (`internal/tasks/store`) |
| CAL-V0-008 | NOT_RUN; `claim --next` answers `UNSUPPORTED` until the S4 plan is wired |
| CAL-V0-009 | `TestCALV0009_StaleGenerationIsFencedAndRecorded` (`internal/tasks/store`) |
| CAL-V0-010 | `TestCALV0010_RenewExtendsAndIsFencedAfterExpiry` (`internal/tasks/store`) |
| CAL-V0-011 | `TestCALV0011_ExpiredLeaseIsReapedByACollidingClaim`, `TestCALV0011_ReapAndRelease`, `TestCALV0011_ReleaseAndReapPassAnAllBarrier` (`internal/tasks/store`) |
| CAL-V0-012 | `TestCALV0012_LeaseBoundsAndBackwardClock`, `TestCALV0012_BackwardClockRefusesEveryWriter` (`internal/tasks/store`) |
| CAL-V0-013 | `TestCALV0013_RetryAsNextGenerationUpToThree` (`internal/tasks/store`) |
| CAL-V0-014..020 | NOT_RUN; accepted, not started |
| CAL-V0-021 | `TestCALV0021_WholeRepositoryBlocksEverything`, `TestCALV0021_DeclaredNonPathResourcesJoinTheScope` (`internal/tasks/store`) |
| CAL-V0-022 | Claim side only: `TestCALV0022_DerivedScopeWhenTheTicketDeclaresNone` (`internal/tasks/store`), with an injected deriver; the context-index deriver is NOT_RUN |
| CAL-V0-023 | `TestCALV0023_CollisionNormalization` (`internal/tasks/ticket`), `TestCALV0023_CollidingClaimsAdmitOne`, `TestCALV0023_DisjointPathScopesAreBothAdmitted` (`internal/tasks/store`) |
| CAL-V0-024 | NOT_RUN; S5 |
| CAL-V0-025 | `TestCALV0025_WidenAddsPathsAndRefusesCollision`, `TestCALV0025_WidenRefusedUnderAdmissionBarrier` (`internal/tasks/store`) |
| CAL-V0-026 | NOT_RUN; not measured |
