# Corvint Tasks agent leases V0

Owner: Russell Lewis
Date: 2026-09-27
Intent status: proposed
Delivery status: not-started
Authoritative inputs: the Corvint Tasks contract TCP-00 (`beamfall/corvint-tasks` `docs/SPEC.md`,
§3.4, §4, §6 and §7.4), decision 0397 (corvint-tasks built in tree), decision 0423 A10,
`docs/specs/corvint-tasks-store-init-v0.md`, tickets V1-0398, V1-0184 and V1-0310, and the in-tree
sources under `internal/tasks`.

## Agent digest
- Claim: Coding agents claim, renew, gate and complete tickets through leased `corvint-tasks` attempts, so a non-fixture queue can replace a repository's own task runner without a process supervisor.
- Status: proposed; not-started. Drafted 2026-09-27 on the owner's request to bring corvint-tasks to a level where it can take over Beamfall's `script/roadmap.sh`; nothing here is accepted.
- Exists: the TCP-00 attempt, reservation and receipt shapes (reserved, no writer), the §5.2 writer for fixture queues, and the CTS-V0-003 shadow import.
- Blocked on: owner acceptance of this spec and of the TCP-00 amendments below; the recovered task-store contract (V1-0310) for the parts of TCP-00 this spec does not restate.
- Read next: Slices; Requirements; Amendments to TCP-00; Failure modes.

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
be released (§6.2 to §6.4). None of that is built in tree: the writer requires an empty reservation
set (`internal/tasks/transaction/model.go:687`) and refuses every queue that is not a fixture
(`model.go:656`).

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
| S2 | CAL-V0-004..006 | `cutover`: imported shadow records become native records |
| S3 | CAL-V0-007..013 | `claim`, `renew`, `release`, `reap` and `attempt show` |
| S4 | CAL-V0-014 | `plan preview` (`taskman-priority-first/0`, read-only) |
| S5 | CAL-V0-015..017 | `submit`, `gate run` and `complete` |
| S6 | CAL-V0-018 | Linear first import |
| S7 | CAL-V0-019..020 | Lease race and crash qualification, and the execution cutover record |

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

- `CAL-V0-004`: `corvint-tasks cutover --source-queue <queueId> --decision <ref>` MUST, under an
  `OWNER` binding, rewrite every `IMPORT` record from that source queue as a `NATIVE` record with the
  same `ticketId`: `source` becomes `NATIVE` with null source fields, `shadowOverlay` becomes false,
  the revision advances with `previousRecordSha256` naming the imported record (the provenance), and
  `acceptanceRevision` advances because `source` is acceptance-relevant. The queue's
  `canonicalWriter` becomes `NATIVE`, and a `CUTOVER` write barrier holds from the first batch until
  the last. Receipts use the existing `AUTHORITY_SWITCH` kind and name the decision reference.
- `CAL-V0-005`: `cutover` MUST plan every record against one audit, under one lock and one
  authority session, before the first write, and refuse the whole run on any record it cannot
  rewrite. It commits in batches like `import` (CTS-V0-003), and a rerun resumes. A later `import`
  of an item whose record is already `NATIVE` MUST refuse that item with a named conflict and never
  overwrite it.
- `CAL-V0-006`: Records that `cutover` writes MUST keep their dependencies, holds, gates and
  completion; an imported completion stays completed, and an imported hold stays held.

S3, leases.

- `CAL-V0-007`: `corvint-tasks claim <ticketId> --holder <label> [--lease-minutes N]
  [--branch <label>] [--base <oid>]` MUST admit by TCP-00 §4.1 steps 1, 2, 5, 6 and 8 with runtime
  `external-agent`, in one transaction: a new attempt at generation 1 in phase `RUNNING`, with
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
  receipt, every expired lease whose reservation would otherwise block it.
- `CAL-V0-012`: A lease is `{holder:label, grantedSeq:Size, expiresAt:Timestamp}`. The default is
  60 minutes, the minimum 5 and the maximum 1,440; a request outside that range is `MALFORMED`. A
  transaction whose `recordedAt` is earlier than the head receipt's refuses `STORAGE_FAILED`
  before it writes, so a clock that steps backward cannot revive an expired lease.
- `CAL-V0-013`: A ticket whose last attempt is `FAILED` or `CANCELLED` MUST be claimable again as
  that attempt's next generation (TCP-00 §6.2 `retry`, `retryCount < 3`), and after three retries
  only an `OWNER` `ticket reopen` makes it claimable.
  `corvint-tasks attempt show <attemptId>` and `queue status` MUST report every live attempt with
  holder, phase and lease expiry, as pure reads.

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
  `external-agent` attempts.
- A10: cause `LEASE_EXPIRED` joins the closed cause set, and quiescence `FENCED` covers a generation
  closed by `release`, `reap` or `complete`: no command of that generation can take effect after it.
- A11: for a queue whose admissions are all `external-agent`, the §7.4 execution permission needs a
  `QUALIFICATION` receipt for the CAL-V0-019 suite in place of G2's supervisor and spawn rows and G3.

## Failure modes

| Failure | Effect | Handling |
|---|---|---|
| Holder crashes or abandons its session | Lease stops being renewed | `reap`, or the next colliding `claim`, fails the attempt `LEASE_EXPIRED` and frees the reservation |
| Stale holder keeps working after reap | Edits continue in its own worktree | Every command it sends is `FENCED`; the tree it built can only complete through a new claim |
| Two sessions use one holder label | Both believe they hold it | The label is a display name only; the generation returned by `claim` is the fence |
| Wall clock steps backward | An expired lease could look live | A transaction earlier than the head refuses before writing (CAL-V0-012) |
| Gate command hangs | Holder waits | The declared gate timeout records `FAILED`; the attempt stays `CHECKING` for another `gate run` or `submit` |
| Candidate rebased before merge | Tree changes | `complete` refuses until the new tree is submitted and gated |
| Cutover interrupted | Some records native, some imported | A rerun resumes from the first unwritten batch |
| Re-import after cutover | Foreign export disagrees with native records | The native record wins; the item is refused with a named conflict |
| Non-fixture queue before execution cutover | Agents try to claim | `claim` refuses; ticket writes still work |

## Acceptance and rollback

Acceptance evidence, per slice: named `TestCALV0NNN_*` tests for every requirement in that slice
under `internal/tasks`, the unchanged fixture tests for CAL-V0-003, and for S6 the measured first
import of the real export. Before S7 closes, a rehearsal in a throwaway non-fixture store holding
the cut-over Beamfall export claims, gates and completes one real Beamfall ticket, and lets one
lease expire and be reaped.

Rollback, per slice: S1 restores the fixture-only check at `model.go:656`; S2 removes `cutover`
(records it already rewrote stay valid native records); S3 to S5 remove the lease verbs, and a store
that holds live `external-agent` attempts must first `release` or `reap` them, because a rolled-back
reader reports them `NOT_OBSERVED`; S6 restores the per-batch audit; S7 removes the execution
cutover verb, and an owner decision clears `executionCutover` on any queue that has it. Beamfall
keeps `roadmap.sh` untouched until TCP-09, so its runner stays available as the fallback throughout.

## Traceability

| Requirement | Evidence |
|---|---|
| CAL-V0-001..020 | NOT_RUN; proposal only |
