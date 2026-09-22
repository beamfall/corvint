# Agent task manager — owner intent and build plan (2026-09-04)

Status: **accepted**, revision 6 (round 6 Claude + Codex applied; decision 0052). Owner intent is
recorded verbatim from the 2026-09-04 session. Nothing here is built or advertised. The accepted
repository is the sibling module `~/projects/corvint-taskman`; its binary is `atm`.

Gate A round 1 (2026-09-04): reviewed by `distributed-systems-architect` (control plane) and
`test-conformance-architect` (review lanes), both Claude expert reviewers — Codex was over its usage
limit for this round. Both returned REPAIR; every finding from both reports is applied below.

Gate A round 2 (2026-09-04): the same two reviewer personas re-read the revised document, again as
Claude expert reviewers. Both returned REPAIR — sixteen findings each; every finding from both
reports is applied below.

Gate A round 3 (2026-09-04): the same two reviewer personas, again as Claude expert reviewers. Both
returned REPAIR — sixteen findings each; every finding from both reports is applied below.

Gate A round 4 (2026-09-04): fresh reviewers of the same two personas, as Claude expert reviewers.
REPAIR — twelve control-plane and ten review-lane findings; every finding is applied below.

Gate A round 5 (2026-09-04): fresh reviewers of the same two personas. REPAIR — eleven
control-plane and twelve review-lane findings; every finding is applied below.

Gate A round 6 (2026-09-04): fresh reviewers of the same two personas, plus an independent Codex
review of the same revision. Claude returned REPAIR (four HIGH, nine MED, three LOW); Codex returned
REPAIR (twelve HIGH). Every finding from both was checked against the cited sources — Beamfall's
`script/roadmap.sh`/ADR-0206, the WQO/OCA/HDC specs, and `AGENTS.md` — and every finding that
verified is applied below; none was rejected this round.

## 1. Owner intent (verbatim, 2026-09-04)

> I want a task manager to also be able to manage agents. maybe it can run over multiple agents and
> be able to pick runners and reviewers. is should also be able to do expert reviews using summoned
> experts that are perfect, picking the correct model type and effort and doing adversarial reviews.
> It should also be able to review the auto generated documentation to ensure that what the tasks
> are doing matches whats in the docs

> I want a best in class tool for this. its should be the best way in the world to run and review
> tasks.

> do your research to ensure its really is world class and nothing does a better job. it should be
> fully configuble

> once the task manager is working, you should move the rest of the astra work to it and start
> using and refinding it

Reading applied: one tool that (a) runs a ticket queue across several agent runtimes, (b) chooses
the builder and the reviewer per ticket, (c) reviews with summoned expert personas at a chosen model
tier and reasoning effort, adversarially, and (d) checks that documentation, including generated
documentation, still matches what each task delivered. "Best in the world" is treated as a
measurement target (§7) backed by a dated landscape survey (§11), not a claim the tool may carry.
"Fully configurable" is read as: every policy the tool applies is data the operator can change
without a rebuild (§4.6).

## 2. What exists today (surveyed 2026-09-04)

| Piece | Where | What it already does | What it lacks for this intent |
|---|---|---|---|
| Beamfall roadmap runner | `beamfall/script/roadmap.sh` (14,532 lines) + `roadmap_lib.py` (4,569) | On-disk leases, worktrees, merge lock, builder/reviewer roles, ADR-0206 manager-mode routing (`route --json` band/builder/reviewer) with complexity-routed model selection, ADR-0206 repository boundary, `review-export` expert packets | Beamfall-bound; shell; no expert-catalog dispatch, no adversarial protocol, no doc-consistency lane, no cross-vendor reviewer independence receipt |
| Corvint work-queue observation | `docs/specs/work-queue-observation-v0.md` (WQO, decision 0046), `corvint work observe`, `propose-wave` | Read-only queue snapshots, deterministic path-clash closure, largest collision-free wave | By contract authorizes nothing and never mutates; Beamfall adapter and recorder NOT_RUN |
| All-worker cost receipt | `benchmarks/dogfood_workers.py` (AT-01) | Roles {coordinator, builder, reviewer, repair}, hosts {claude-code, codex, atm}, optional ticket attribution, at most 256 workers, tokens per worker, NOT_OBSERVED never zero (`4e8e088`) | Post-hoc only; nothing enforces caps during a run |
| Expert catalog + dispatch | `~/.agents/expert-catalog.md` (51 profiles), `expert-dispatch`, `expert-grill` skills | Selective full-profile loading; best-evidenced catalog/ad-hoc fit; persona charters carry questions, evidence and boundary; model tier and effort guidance | Manual; selection quality and effort routing are not evaluated |
| Codex root orchestration | `agent-control` skill | Leaf workers, DAG gating, state ledger, retry policy, no nested delegation | Codex-only; prose protocol, not a tool |
| Corvint evidence verbs | `corvint context`, `prove`, `cem`, `ocm`, harness receipts | Evidence pinned to Git content, receipts, CEM binding of a diff to claims | Not wired to a runner or reviewer loop |
| This session's manual loop | `docs/BUILD-LOG.md` 2026-09-04 entries; scratchpad `codex-*.md` | Gate A → build → fresh review → repair, parallel Codex and Claude lanes, Codex-outage fallback to purpose-built Claude expert reviewers, per-wave receipt (63 workers, 1,423 turns) | Hand-driven by the coordinator every round; nothing replays or audits it |

The manual loop is the behaviour to mechanize. Its rules are already written down in the repo and
in memory: fresh reviewer per round, findings are hypotheses until the cited `file:line` is opened,
REPAIR loops narrow each round, vendor outage degrades explicitly, cost is a receipt.

## 3. Decision (accepted by decision 0052)

Build the task manager as a **standalone native-Go CLI** in the sibling repository
`~/projects/corvint-taskman`, with binary name `atm`. It consumes Corvint as its
evidence engine and the expert catalog as its reviewer registry.

- Not inside `corvint`: AGENTS.md invariant 7 keeps Corvint one read-mostly binary, and WQO is
  non-operative by contract. A tool that spawns agents and advances leases is operative.
- Not a further extension of `roadmap.sh`: 19k lines of shell/Python bound to Beamfall by
  ADR-0206 §2; the lease protocol is worth keeping as **one queue adapter**, not as the core.
- Against Beamfall the tool runs only as a short-lived **ADR-0206 manager root** driving supported
  `roadmap.sh` verbs (the explicit `roadmap.sh dispatch <ticket>` allowance, ADR-0206 §1 para 3).
  Claim, review, and merge stages stay permanently `manager`: the tool never holds stage authority,
  never installs a dispatcher, scheduled steward, operator pass, or extraction adapter, and offers
  no reactivation path for the retired topology.
- Corvint WQO snapshots are the read side of every queue; `corvint prove`/`cem`/`context` supply the
  evidence for the doc-consistency lane; AT-01's receipt shape is the cost contract.

Alternatives weighed: (i) extend `roadmap.sh` (fastest, but locks the tool to one repo and one
language); (ii) new Corvint verb family (violates invariant 7 and WQO §3 authority boundary); (iii)
buy or adopt an external orchestrator. Owner instruction 2026-09-04 makes the survey mandatory: §11
records it with dated primary sources, scored against the single requirement set stated there. §11
is a first pass with named `NOT_RUN` items; that it does not gate Gate A closure is a coordinator
call recorded in §9, not a property of the survey.

## 4. Requirements (accepted by decision 0052, `ATM-V0-`)

Numbering is stable from here; Gate A may strike or amend a row but must not renumber. A struck row
keeps its id and records why it was struck. Amendments add lettered sub-clauses; genuinely new
requirements continue the number line from 028 and are placed with the section they belong to, so a
section is not always in numeric order.

### 4.1 Queue and run control

- **ATM-V0-001** Queue adapters. A queue is read through an adapter that yields the WQO snapshot
  wire shape. An adapter either invokes `corvint work observe` and consumes its validated
  observation, or is repository-tracked code selected by repository policy; a tool-resident parser
  would emit the wire name without the WQO validation, identity binding, and adapter receipts
  (`docs/specs/work-queue-observation-v0.md` §3, WQO-V0-013), so the tool ships none. Adapters in
  v0: Corvint `ROADMAP.md` tickets, Beamfall leases, and a plain JSON queue file. The Beamfall
  adapter shells out to supported `script/roadmap.sh` and `script/bf` verbs and never reads
  `.roadmap/`, roadmap shards, derived SQLite, locks, or worktrees directly. Every read-verb
  invocation runs with `GIT_OPTIONAL_LOCKS=0` (Beamfall's `audit` shells out to plain
  `git status`/`git status --porcelain`, e.g. `script/roadmap.sh:9835`, `:9934`, `:9960-9966`,
  `:10038`, which without that variable may refresh the index's stat cache), and reading's property
  is stated exactly as WQO-V0-017 states it for a path with no OS enforcement: **observed
  unchanged**, via complete pre/post manifests, with `MUTATION_ENFORCEMENT_UNQUALIFIED` recorded
  rather than an unqualified claim that reading never mutates — a claim WQO-V0-017 itself does not
  make absent OS enforcement (`docs/specs/work-queue-observation-v0.md:348-352`). A detected change
  is `CHANGED`/`MUTATION_DETECTED`, and the observation's state becomes `UNKNOWN`.
- **ATM-V0-001a** Enumerated read verbs. "Supported verb" does not mean non-mutating, so the read
  path of each adapter carries a closed list of verbs each separately verified non-mutating against
  its implementation body, not its header description, and a verb absent from the list is
  unavailable until it is verified and added. The allowlist is **embedded code**, not a
  configuration layer under ATM-V0-023/024: operator configuration can disable a listed verb but
  can never add one, because a list-replacing config layer would otherwise let an operator put a
  mutating verb on the read path and break AGENTS.md invariant 4 for ATM-V0-020's read commands.
  For Beamfall the list is exactly `count` (`script/roadmap.sh:8473-8518`), `audit` (`:10061-10088`,
  helper `repo_inventory_json` `:9875`), `lint` (`:8458-8463`, helper `approval_ledger_lint`
  `:11271`), `route --json` (`:8535-8537`), and `pause-status` (`:207`); the shared read helpers are
  `claim_json` (`:1248`), `claim_inventory_json` (`:1252`), `ready_count_readonly` (`:1264`),
  `review_jobs_count` (`:8351`, which enumerates through `_review_jobs_enum` `:12934` and is reached
  by `audit`), and `claimed_args_array` (`:1256`, reached by `ready_count_readonly`). Each is
  separately verified non-mutating: the shell paths are Git read-only, and the `python3 $LIB`
  subcommands these helpers invoke go through two different paths. `next` (invoked by
  `ready_count_readonly`, `:1270`, `:1272`), `metrics`, `lint`, and `route` all go through `parse()`
  (`roadmap_lib.py:842-846`). `claims` (`cmd_claims`, `:4013`, dispatched `:4128-4147`) is
  **path-independent**: it ignores its path argument entirely and never reaches `parse()`, so it
  carries no `is_db_path` hazard at all. `parse()` is **not** unconditionally non-mutating for the
  first set: it resolves the source and dispatches to `parse_db` (`:1217`) whenever `is_db_path`
  holds (`:825`, `:829`), and `parse_db` calls `ensure_db_fresh` (`:1100`), which rebuilds the
  derived SQLite. `is_db_path` is a pure filename-suffix test pinned against the upstream
  `roadmap_lib.py` sha this plan validated it against, recorded in the adapter's own build receipt
  rather than restated here as a number that would rot; the adapter re-checks the predicate's body
  against that pin at build time and refuses to ship if it has changed underneath. The
  non-mutation precondition is therefore an assertion the adapter makes itself: before any of
  `next`/`metrics`/`lint`/`route` the Beamfall adapter asserts `not is_db_path($ROADMAP)`, where
  `$ROADMAP` is a value the **adapter itself exports** rather than inherits from the ambient
  environment (`script/roadmap.sh:100` only documents the upstream default), and refuses the verb
  otherwise, so no read command can reach the `parse_db`/`ensure_db_fresh` write path and AGENTS.md
  invariant 4 holds for ATM-V0-020.
  *Struck at Gate A round 3:* `next|ready` and `fanout-plan` without `--claim`, previously listed
  here, are **mutating**. `cmd_next` (`:2117-2120`) calls `maybe_reap` (`:2059-2065`, which frees
  stale leases and merge locks), `mkstate` (`:140`), and `current_status_refresh` (`:446`, writing
  `.roadmap/current-status.{json,md}` under an exclusive lock `:589-600`); `cmd_fanout_plan` calls
  `current_status_refresh` at `:8633` before it parses its arguments, so `--claim` is not what makes
  it mutate. `status` is likewise mutating through the same refresh (`:589-600`), and `add`,
  `amend`, and `add-dep` commit.
- **ATM-V0-001b** Admission state. An adapter declares the queue's admission state alongside the
  snapshot. A paused queue yields no wave and refuses every claim, and the tool never sets or passes
  an adapter's pause override (`ROADMAP_PAUSE_OVERRIDE`, `script/roadmap.sh:12365`), which exists
  for attended operation; upstream a global operator pause is a hard admission barrier
  (`script/roadmap.sh:14428-14438`) and the tool honours it rather than routing around it. That
  barrier is why `pause-status` is on the ATM-V0-001a read list: it is dispatched outside the gated
  verb set (`script/roadmap.sh:14453`) and answers while everything else refuses. The gated `case`
  list (`:14433`) covers only the ATM-V0-001c **write** verbs `claim`, `begin`, `finish`,
  `apply-verdict`, and `adopt-review` — none of ATM-V0-001a's read verbs appears in it, so a read
  verb's refusal, if any, is never pause-gated, and `ADMISSION_PAUSED` classifies only a refusal
  from one of those five write verbs. A pause-gated
  refusal from one of them, together with a positive `pause-status`, is classified
  `ADMISSION_PAUSED` and never `ADAPTER_FAILED`; without a positive `pause-status` the refusal stays
  `ADAPTER_FAILED`. `ADMISSION_PAUSED` is **the tool's own admission record, carried out of band**
  and never written into a WQO wire error field: WQO closes command errors to exactly
  `ADAPTER_FAILED`, `CANCELLED`, `HOSTILE_INPUT`, `INPUT_LIMIT`, `MALFORMED_INPUT`,
  `SOURCE_UNQUALIFIED`, and `UNSUPPORTED_PLATFORM`
  (`docs/specs/work-queue-observation-v0.md:340-341`), so a tool-minted code in that field would
  break the wire shape ATM-V0-001 requires.
- **ATM-V0-001c** Enumerated write verbs (added at Gate A round 3; `release` added at round 6). The
  write path of each adapter carries the same kind of closed, embedded list, so a verb that advances
  a lease is as enumerated as one that reads. For Beamfall the list is exactly `claim`
  (`script/roadmap.sh:3814-3843`), `begin` (`:10637-10743`), `heartbeat` (`:6917-6975`), `finish`
  (`:7560-7856`), `apply-verdict` (`:13648-13888`), `adopt-review` (`:13939-13984`), and `release`
  (`:5307-5321`) — fenced to the ATM-V0-028/§8 `drain` rollback path, so `drain` can release a
  lease the tool itself claimed through one of the verbs above, and never used to relinquish a
  lease the tool does not hold. `merge`, `closeout`, and `checkoff` are
  **not** on the list: auto-merge is a v0 non-goal (§5), so the tool records a verdict and transfers
  the lease, and the merge stage stays with the adapter's own operator. Adding a write verb requires
  the same amendment path as adding a read verb.
- **ATM-V0-002** Waves. The next wave is the emitted wave of the WQO proposal (WQO-V0-020), carried
  with the proposal's own `waveOptimality` label (`MAXIMUM`, `GREEDY`, `NONE`); the tool never
  recomputes the closure and never substitutes the underlying collision-free candidate set for the
  emitted wave. An observation that is not `VALIDATED_AT` (`PARTIAL`, `CONFLICTED`,
  `STALE`, `UNKNOWN`) yields no wave: the tool reports the WQO state and its unknown codes verbatim,
  including `COLLISION_CLOSURE_INCOMPLETE`. The read-only `wave` command **prints an unpinned
  proposal and writes nothing**; `run` is what pins one, by recording the proposal it observed
  as the first receipt of the ATM-V0-019a append-only log for that invocation. Nothing is pinned
  until a mutating command pins it, which is what keeps `wave` inside ATM-V0-020. To cover the
  operator-visible gap between a printed proposal and the run that acts on it, `run` accepts
  `--from-proposal <file>`, accepted only as **canonical JSON** (ATM-V0-022's `--pretty` rendering
  is refused as input, so there is exactly one comparable rendering): the proposal named by the file
  is pinned as the first receipt in place of a fresh observation, and `run` compares exactly these
  named fields between the re-observation and the pinned file — the re-observation's
  `queueSourceId` (WQO's queue-identity field, `work-queue-observation-v0.md:172`) and, per ticket
  selected into the wave, that ticket's `CollisionGroup.source` and `CollisionGroup.memberTicketIds`
  (`:128`) — refusing when any of those differ. `VALIDATED_AT` and other observation
  timestamps are not compared: ATM-V0-002a already treats their drift as ordinary re-observation.
  Without the flag
  `run` pins its own observation and
  the propose→run gap is unchecked by construction.
- **ATM-V0-002a** Wave freshness. `VALIDATED_AT` is historical the moment it returns (WQO-V0-014).
  `run` re-observes before it claims anything and refuses the wave only when the re-observation's
  `queueSourceId`, or a wave member's `CollisionGroup.source`/`CollisionGroup.memberTicketIds`,
  differs from the proposal that `run` itself pinned as its first receipt (ATM-V0-002) — the same
  named-field comparison ATM-V0-002 defines for `--from-proposal`. A
  wave member that is simply no longer selected in the re-observation is **dropped from the wave**,
  not a whole-wave refusal: a member disappearing is the ordinary result of another root taking it,
  and the invocation proceeds with the rest of the wave. A
  changed repository checkpoint is ordinary re-observation, not a refusal: several ADR-0206 manager
  roots share one checkout, so refusing on checkpoint drift would make `run` inert. Losing the race
  for a ticket is the `TAKEN` outcome of ATM-V0-005a, after which the invocation re-observes and
  proceeds with the rest of the wave.
- **ATM-V0-003** Runtime registry. Every runner is a subprocess (`claude -p` or an Agent SDK
  subprocess, `codex exec`, later others); the tool never links a vendor SDK in process. Each entry
  declares: sandbox modes, models, effort levels, the field carrying that runtime's own conversation
  identifier, which of the four AT-01 token fields are live-observable **per field** rather than one
  runtime-wide flag, `transcript_exposes_tool_io: yes|no` (whether
  the runtime's transcript records tool invocations and their output ranges, not only a final
  answer), and cost per token where known. Registry entries are data, not code paths. Availability
  is *not* a registry field: it is run state living only in the ATM-V0-004 snapshot artefact, so
  that configuration stays free of observations that change under it.
- **ATM-V0-004** Runner selection is a pure function of recorded inputs: ticket complexity band ×
  required capabilities × an availability snapshot captured by a named mutating command
  (`availability refresh`) and pinned into the routing receipt. Read commands consume the last
  snapshot and never probe. Every selection emits a routing receipt naming the rule that fired, the
  availability snapshot it used, and the alternatives rejected.
- **ATM-V0-005** Lifecycle state machine: `ready → claimed → built | build_failed → gated |
  gate_failed → reviewed → repaired* → accepting → accepted | rejected | blocked | cancelled`, plus
  the `TAKEN → ready` edge: a claim that loses its compare-and-swap returns the ticket to `ready`
  for the winner and is not an error state for the loser.
  `build_failed` covers non-zero exit, crash, and cap cancellation; `gate_failed` covers a red gate
  before review and a red gate or merge conflict during acceptance; `blocked` and `rejected` are
  left only through a named mutating command. `accepting` is the serialized section and is
  re-entrant: a crash between fast-forward and checkoff resumes at the same idempotent
  post-fast-forward checkoff (ADR-0206 §4). Every state carries a named recovery rule. State lives
  on disk in atomic files with heartbeats and stale reclaim; every invocation is one CLI run that
  advances state and exits. No daemon.
- **ATM-V0-005a** Fencing. Every transition is a compare-and-swap on (owner, phase, head) and fails
  `TAKEN` when it loses. Reclaiming a stale lane preserves the work-bearing lane before freeing it.
  One state directory serves exactly one checkout; a second checkout against it is refused.
  Reclaim additionally requires a liveness check on the lane's recorded process group. **A lane is
  live when at least one live process carries the recorded pgid, AND either the group leader is
  absent or its start time matches the one recorded at spawn.** A bare pgid is unsound under pid
  recycling, which is what the start-time half closes; but the leader's proc entry disappears once
  it exits and is reaped while the pgid stays valid for `kill(-pgid)` as long as any member lives,
  so a **leader gone with live members is live**, not dead — that is exactly the
  runtime-outlived-SIGKILL case this check exists for. A group whose leader is present with a
  different start time is **dead**, and the tool never signals it. **Leader present** means a
  process with pid equal to the recorded pgid exists *and its own pgid equals that pid*; a process
  that merely reuses the pid without leading that group leaves the leader **absent**, not present,
  so a recycled pid can never make an orphaned-but-live group read as dead and free a lane still
  being written. The rule leans on one named per-platform assumption, recorded as such: a pid is
  not reused while it is still the pgid of live members. A lane that is live by this test
  is `RECLAIM_REFUSED` and escalates to the owner, never freed, because a runtime that outlived
  SIGKILL is still writing to the lane worktree. **Start time** is read from a named observable, not
  from `ps -o lstart=`, whose one-second granularity a pid recycled inside the same second defeats:
  on Darwin the `sysctl` `KERN_PROC_PID` entry's `p_starttime` (microseconds), on Linux field 22 of
  `/proc/<pid>/stat` combined with `/proc/stat`'s `btime`.
  **Signal safety (added at round 6).** Before any `kill(-pgid, …)` the caller re-reads the leader
  pid's start time by the same named observable and compares it against the value recorded at spawn,
  immediately before the call; a mismatch means the caller cannot prove the pgid it is about to
  signal still identifies the group it recorded, and it refuses to signal, recording
  `SIGNAL_REFUSED_IDENTITY` rather than guessing. This is narrower than the liveness rule above:
  liveness asks whether the lane is still alive; revalidation asks whether the pgid about to be
  signalled is still that lane's pgid at the moment of the signal, closing the window between the
  liveness check and the signal itself. A synthetic-process-table fixture exercises the reuse race
  directly (a leader pid recycled to an unrelated process between the liveness read and the signal).
  The **residual race** — a pid recycled within the observable's own resolution with a matching
  start time, or reused strictly between the revalidation read and the `kill()` syscall — is not
  closed by any userspace check and is a **v0 non-goal**: closing it needs a kernel-level
  generation-bound handle (a pidfd on Linux, or an equivalent), which is future work.
  The lane record also carries its **supervisor's pid and start time** (ATM-V0-005c), so
  `RECLAIM_REFUSED` and `SUPERVISOR_LOST` are decided on different observables rather than on the
  same one. Precedence is fixed: supervisor dead ⇒ `SUPERVISOR_LOST`, and the next invocation
  cancels the lane; supervisor alive ⇒ the lane is not reclaimable at all, whatever its heartbeat
  age; runtime alive with its supervisor gone and the cancel refused ⇒ `RECLAIM_REFUSED`.
  Where the queue adapter owns liveness — Beamfall's claim-meta heartbeat age and
  `roadmap.sh reap --crashed` preserve-then-free (`script/roadmap.sh:18-21`) — that adapter is the
  only reclaim authority: the tool's own heartbeat is advisory for those lanes and its reclaim path
  is disabled, so two liveness authorities never race over one lease. Because that adapter frees on
  its own clock, the tool **refreshes the adapter's own claim heartbeat** for every adapter-owned
  lane through the ATM-V0-001c `heartbeat` write verb (`script/roadmap.sh:6917-6975`), called by the
  **resident ATM-V0-005c supervisor itself on a timer**, not by `work-loop`: `cmd_work_loop`
  (`script/roadmap.sh:8327-8344`) calls `cmd_heartbeat` only once before and once after the wrapped
  command runs, so a single long-running lane command between those two calls gets no heartbeat
  refresh at all and can outlast the adapter's stale window even though it is still inside its own
  wall-clock cap. The supervisor instead issues its own `heartbeat` calls on an interval strictly
  below the adapter's stale window, independent of and concurrent with the wrapped lane command,
  with a bounded number of retries on a failed call and cancellation of the lane **before** it
  crosses the stale threshold if the retries are exhausted. A committed test runs a lane command
  past the stale window with no interior heartbeat other than the supervisor's timer and asserts the
  lane is not reaped inside its cap. Without the supervisor-timed refresh,
  `ROADMAP_STALE_MIN`/`ROADMAP_CRASH_TTL_MIN`
  (`script/roadmap.sh:109-110`) or any concurrent root's `next` → `maybe_reap` (`:2059-2065`) frees
  a lane while it is still inside its cap.
- **ATM-V0-005b** Stale threshold. The tool's own stale-reclaim threshold is at least twice the
  largest configured wall-clock cap (ATM-V0-006), so a tool-owned lane still inside its cap can
  never be reclaimed and double-built. The inequality is checked against the **tool's own stale
  threshold only**. An adapter-owned lane is not held to it: ATM-V0-005a already disables the tool's
  reclaim path there and refreshes the adapter's own claim heartbeat, so the requirement for those
  lanes is only that the refresh interval is shorter than the adapter's stale window (against
  Beamfall, `ROADMAP_STALE_MIN=90`, `script/roadmap.sh:109`). Folding the adapter's window into one
  effective minimum would instead reject at load every cap over 45 minutes against Beamfall — a
  configuration the heartbeat refresh already makes safe, two mechanisms for one hazard with the
  stricter one forbidding what the other permits. The heartbeat interval is additionally bounded
  from above: a configuration whose heartbeat interval is not strictly less than the ATM-V0-028
  drain grace period (default 60 s) is **rejected at load**, since at a longer interval a resident
  supervisor can never observe the barrier inside the grace period and `drain`'s graceful branch
  is unreachable by construction. A configuration that violates any of these conditions is
  rejected at load.
- **ATM-V0-005c** Supervision. "No daemon" does not mean nothing is alive while a lane runs: the
  invocation that spawns a lane stays resident supervising it until the lane exits or its wall-clock
  cap fires, and that supervisor is what performs the ATM-V0-006 cancellation. The invocation exits
  when its lanes are done; nothing runs between invocations. The lane record carries the
  supervisor's pid and start time, and supervisor liveness is that pair, on the ATM-V0-005a rule. A
  lane whose supervisor is dead by that test while the lane is still recorded live is
  `SUPERVISOR_LOST`: the next invocation cancels it by process group and records the outcome, and
  never reclaims it as if it were merely stale. That cancel **signals processes only**; it never
  frees, transfers, or reclaims a lease, whose disposition stays with the adapter's own reap
  (ADR-0206 §2). For an **adapter-owned** lane a lost supervisor is a **cancel-now condition** at
  the next invocation rather than something to wait out, because the supervisor is the only
  refresher of the adapter's claim heartbeat (ATM-V0-005a) and an unrefreshed lane is freed on the
  adapter's own clock. A lane whose refresh has lapsed past the adapter's stale window is treated
  as adapter-owned and lost and is **never re-claimed by the tool**; disposing of it is the
  adapter's `reap --crashed` preserve-then-free alone (`script/roadmap.sh:18-21`).
  A resident supervisor **re-reads the ATM-V0-028
  admission barrier on every heartbeat interval as well as immediately before every state write**
  and exits without writing when the barrier is present, so a supervisor with no pending write still
  observes a `drain` within one heartbeat rather than only when its wall-clock cap fires, and a
  supervisor spawned before a `drain` cannot write into the state directory the drain is removing.
- **ATM-V0-005d** Non-autonomous classes. A ticket the queue adapter reports as human,
  approval-required, design-approval, never-class, or multi-repo enters `blocked` at claim time and
  is never advanced by the tool, matching ADR-0206 §3. Against Beamfall the acceptance path the tool
  drives is `apply-verdict` (`script/roadmap.sh:13648-13888`) and `adopt-review` (`:13939-13984`),
  both on the ATM-V0-001c write list; the tool records the verdict and transfers the lease through
  those verbs and holds no stage authority of its own. It never drives `merge`, `closeout`, or
  `checkoff`, which is the same boundary §5 states as the auto-merge non-goal.
- **ATM-V0-006** Budgets and cost. Per task, wave, role, model: the four AT-01 token fields —
  `inputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `outputTokens`
  (`benchmarks/dogfood_workers.py:34-37`, `:273-276`) — observed from runtime transcripts; caps on
  turns, tokens, and wall clock enforced with
  active cancellation (SIGTERM, then SIGKILL after a grace period, process group). Observability is
  **per field, not per runtime**: `codex exec` reports `inputTokens`, `cacheReadTokens`, and
  `outputTokens` from its cumulative usage but never `cacheCreationTokens`
  (`benchmarks/dogfood_workers.py:355` vs `:361-363`), so a runtime-wide flag would either forbid
  enforceable caps it can enforce or claim one it cannot. A cap on a field the ATM-V0-003 registry
  does not mark live-observable for that runtime is recorded `NOT_ENFORCED` for that field alone;
  the wall-clock and turn caps always apply. Enforcement is evaluated at **turn boundaries**, which
  is the only point a subprocess reports usage, so a turn that overshoots its token cap is cancelled
  at the boundary and the overshoot is recorded rather than presented as a cap that held.
  Unobserved usage is `NOT_OBSERVED`, never zero (AT-01 rule). The built-in **per-lane** defaults,
  pinned by the decision-0052 coordinator call, are `inputTokens: 2,000,000`,
  `cacheCreationTokens: 1,000,000`, `cacheReadTokens: 100,000,000`,
  `outputTokens: 400,000`, `turns: 400`, and wall clock `90 minutes`. These are rounded-up
  operating caps over the observed wave-2 maxima `1,354,962 / 730,992 / 76,621,632 / 270,258 /
  281` in `benchmarks/results/dogfood-workers-at-wave2-2026-09-04.json`; they are coordinator calls,
  not empirical evidence that the defaults are optimal. A field the selected runtime cannot report
  stays `NOT_ENFORCED`; the wall-clock and turn caps remain enforced.
- **ATM-V0-007** Concurrency. Several runners of any vendor may run at once; the tool never
  serializes on vendor. Lanes are isolated by worktree; the merge or acceptance step is the only
  serialized section.

### 4.2 Reviewer independence and expert review

- **ATM-V0-008** Fresh reviewer. A reviewer never reuses a runtime session or a worktree scratch
  directory belonging to the builder it reviews or to the previous round's reviewer, and shares no
  task-specific context beyond the always-loaded instruction and memory files the runtime itself
  resolves, which the receipt enumerates. That set is **not only repository files**: it includes the
  runtime's non-repository auto-loads — for `claude -p`, `~/.claude/CLAUDE.md` and each file it
  `@`-imports, plus the repository `CLAUDE.md`/`AGENTS.md` chain — and per-project agent memory. The
  per-project agent-memory file is a live carrier of this session's own dated instructions, so it is
  **disabled for reviewer sessions**; where a runtime offers no way to disable it, its sha256 is
  pinned per round in the receipt instead and the round records that it was loaded. Its path is a
  named member of the enumerated set (for `claude -p`,
  `~/.claude/projects/<project>/memory/MEMORY.md`), not left to be discovered.
  Scratch state means everything under the lane worktree outside the Git-tracked tree.
  Always-loaded context that is **not a file** — MCP server instruction blocks, skill listings,
  settings-injected text — is either enumerated the same way or named in an explicit exclusion list
  with the reason; it is never silently outside the set.
- **ATM-V0-008a** Disjointness receipt. The receipt records the runtime's own conversation
  identifier (the field is named per registry entry — Codex `thread_id`, others as declared),
  process start time, runtime, model, effort, and the enumerated context sources with a sha256 per
  always-loaded file. It also records `enumerationMethod` — `runtime-reported` when the runtime
  itself names its loaded set, `tool-side` when the tool reconstructs vendor discovery rules — and
  the runtime version the tool-side rules were validated against, because a tool-side enumeration is
  a reimplementation that silently rots when the vendor changes discovery. A runtime that does not
  report its set is marked `ENUMERATION_UNVERIFIED` in the receipt; like other unobserved fields it
  does not by itself **block** a review, but it also does not **satisfy** ATM-V0-008's disjointness
  requirement or ATM-V0-013's verifier-disjointness requirement: `ENUMERATION_UNVERIFIED` and
  `NOT_OBSERVED` are absence of evidence, and AGENTS.md invariant 2 forbids treating that as
  certainty of independence. A review or verification carrying either flag is reported with
  `independence: unverified` in its receipt, and any HIGH finding it raises falls into the
  *unverified* partition of §7 item 2's four-way split — never *matched-label* or
  *confirmed-but-unlabelled*, and never counted against t2. Freshness beyond what the runtime itself
  exposes is recorded
  `NOT_OBSERVED`,
  never asserted, and `NOT_OBSERVED` alone never blocks a review: a review is refused only when a
  field is observed and *contradicts* disjointness (same conversation identifier, or the same lane
  scratch directory, as the builder or the previous round's reviewer).
- **ATM-V0-009** Vendor diversity. The reviewer runtime differs from the builder runtime when the
  registry offers one; on outage the tool falls back to a fresh session on the available vendor and
  records `degradation: same-vendor-review`.
- **ATM-V0-010** Expert selection. Reviewers are summoned from the expert catalog by exact id,
  subject to the catalog's Selection rules, enforced mechanically where possible: the risk-to-
  mission match is recorded, `Review focus` is quoted verbatim into the charter, `Core skills`,
  `Expected evidence`, and `Boundaries` are carried with it verbatim, and the task's binding
  material is attached. `Boundaries` is carried rather than summarised because the catalog's own
  rule is to summon an adjacent expert when a concern crosses out of a persona's discipline
  (`~/.agents/expert-catalog.md:13`), so a reviewer marks any finding it believes crosses its own
  carried boundary `boundary_crossed` in the review receipt. The flag is **reviewer-self-declared
  and recorded as an attribute only**: no party adjudicates it, it never changes a finding's grade
  or any verdict, and no acceptance depends on it, because deciding whether a finding falls
  outside a prose boundary is a judgement no assertion can check.
  A charter whose risk is time-sensitive (legal, policy,
  market, standards, or platform state) additionally carries the catalog's dated-primary-source rule
  (`:17-18`), which is what makes a landscape or vendor claim citable. When no profile owns a
  material risk the tool creates a labelled ad-hoc persona and records a use count for it. The
  recurrence bar for proposing promotion is **one use**, pinned by the decision-0052 coordinator
  call because the standing owner instruction dated 2026-09-03 says to propose promotion whenever
  an ad-hoc expert was needed at all. The tool proposes, and the owner decides. It never promotes a
  persona into the catalog on its own.
- **ATM-V0-011** Model and effort routing per expert is a table from risk class to (model tier,
  reasoning effort). The table is data. A change to the table requires the routing evaluation in
  §7 to pass on the frozen review corpus.
- **ATM-V0-012** Adversarial protocol. Each reviewer receives numbered claims and must attempt to
  falsify every one, cite `file:line` for each finding, grade HIGH/MED/LOW/OK, and return
  ACCEPT / REPAIR / REJECT with the item list. A citation counts as **opened** by *byte
  containment*: the cited file's bytes at the cited range occur, whitespace-normalised, inside the
  retained output of some recorded tool call in that session, after the cited path is resolved
  against the pinned tree root. The reader line-number prefix is stripped **from the retained
  output (the haystack), never from the file bytes (the needle)**: file bytes carry no such prefix,
  so stripping the needle would be a no-op and a multi-line citation read through `rg -n` would
  still fail containment. The strip is guarded, because `^(\S+:)?\s*\d+[:\t-]` also matches real
  content (`2026-09-04 ...`, `12:34:56 ...`, a map-key line `1: "foo",`) and over-stripping only
  shortens the compared string, which loosens containment: it applies **only when the whole
  retained block carries a consistent `path:N:` or `N:` prefix whose N are strictly increasing**,
  at most once per line, and to the haystack alone. The strip is not cosmetic: the 31
  `codex-*.jsonl` sidecars this plan was built from (the round-3 freeze, §7 item 2) carry **413
  `command_execution` items started and 413 completed** (385 `completed`, 28 `failed` — a status
  split, not an incompletion), much of that reading done through a line-prefixing reader (`rg -n`,
  `grep -n`, `nl`, `cat -n`) whose digits whitespace normalisation leaves in place. The
  line-prefixed subset count is `NOT_VERIFIED` until a committed counting script
  (`benchmarks/count_line_prefixed_reads.py`) produces it against a stated match predicate; the
  design does not depend on the number, since one legitimate read failing containment already
  drops a true finding to `hypothesis`.
  Containment rather than "the tool call that read this range"
  is the decidable test, because a compound shell invocation (`/bin/zsh -lc 'cat A B C'`) returns
  one undelimited blob in which per-file ranges cannot be attributed; truncated output contains only
  the bytes actually retained. A finding without containment is labelled `hypothesis`. "Opened" is
  undecidable on a runtime whose registry entry says `transcript_exposes_tool_io: no`, so on such a
  runtime every citation is graded `NOT_OBSERVED` and none is ever graded opened; which runtimes
  those are is registry data, not a property stated here. Verification of those findings falls
  entirely to ATM-V0-013.
- **ATM-V0-013** Coordinator verification. Before a REPAIR round is dispatched every finding is
  graded `confirmed`, `refuted`, or `unverified`. `confirmed` requires all of: (i) the cited
  location resolves at the pinned tree; (ii) a verifier session quotes the cited bytes and states
  that the text supports the claim; and (iii) a **mandatory coordinator machine check** that the
  quoted span occurs byte-identically at the cited path and range in the pinned tree, under the
  same normalisation ATM-V0-012 defines. Here the prefix strip applies to **the reviewer's quoted
  span**, which is the side that may have been copied out of a `rg -n` read, and never to the
  pinned tree's bytes; it is guarded by the same consistent-block, strictly-increasing-N rule, so
  a `rg -n` read is not misgraded and a content line is not silently shortened.
  (iii) is not optional and is not delegated to the verifier, because (ii) alone is
  satisfied by a fabricated quote; a quote that fails the machine check makes the finding
  `unverified` and the failure is recorded. The verifier is disjoint under ATM-V0-008 from **both**
  the builder and the reviewer that raised the finding — a reviewer may never certify its own
  finding — and emits its own ATM-V0-008a receipt. Because that disjointness is asserted on the
  conversation identifier, a verifier must run on a runtime whose ATM-V0-003 entry declares a
  conversation-identifier field; a finding verified on a runtime that declares none is `unverified`,
  never `confirmed`. A location that resolves but does not support the claim is
  `refuted`. Only confirmed findings drive the repair prompt; refuted and unverified findings go
  back to the reviewer with the verifier's per-finding verdict recorded.
- **ATM-V0-013a** Verifier input. The verifier receives exactly the reviewer's verbatim finding text
  plus the pinned tree, one finding per dispatch, with no coordinator commentary, restatement, or
  ordering hint; the coordinator cannot shape what is verified. The receipt stores the **reviewer
  report bytes themselves**, not only their digest, and the finding list is produced from those
  bytes by a **published deterministic parser** shipped with the tool, so any third party can
  recompute the raised count offline. That parser is specifiable only because the reviewer's
  output format is pinned: under ATM-V0-023's reviewer charters and prompt templates every
  reviewer emits its findings in a **published schema** — one JSON object per finding, with `id`,
  `severity` drawn from the closed set `HIGH|MED|LOW`, `path`, `range`, and `claim`. The parser is
  defined over that schema and over nothing else. A legacy or free-form packet is never parsed and
  never yields a raised count: it goes through §7 item 2's published normalisation map instead.
  A digest plus a coordinator-parsed list is self-certifying:
  real packets leave items unnumbered (§7 item 2), so a coordinator that parses five findings
  out of a six-finding report would satisfy every count. A report the published parser cannot
  enumerate marks the round `UNPARSED`, which **blocks ACCEPT** rather than silently standing on a
  smaller raised count. An `UNPARSED` round does **not consume** an ATM-V0-014 repair round: it is
  retried once on a fresh reviewer session, and if that retry is unparseable too the ticket
  escalates to the owner as a blocked item. Its `raised == dispatched == graded` check is recorded
  as **skipped**, because raised cannot be computed, rather than counted as satisfied.
  A report that parses cleanly to an **empty** finding list while carrying a verdict of REPAIR or
  REJECT is a **malformed round**, handled the same way as `UNPARSED`: it does not consume an
  ATM-V0-014 repair round, is retried once on a fresh reviewer session, and escalates to the
  owner if the retry is malformed too — `raised == dispatched == graded == 0` trivially satisfies
  every count check without the round having said anything, and a REPAIR/REJECT verdict with
  nothing to repair or reject is not evidence of review, it is the absence of one. An empty
  finding list carrying ACCEPT is not malformed: it is the ordinary shape of a clean review.
  The receipt also records the digest of each dispatched packet, the
  verifier session identifier, and the counts, which must satisfy **findings raised == findings
  dispatched == findings graded**. Raised is what closes the loop: dispatched == graded alone is
  satisfied by a coordinator that silently drops a finding before dispatch. The verifier's
  (runtime, model, effort) is pinned by an ATM-V0-025 rule **at review-round start** and is
  identical for every finding in a round, so verification strength does not vary within a round; an
  ATM-V0-025 `reconfigure` that changes verifier routing takes effect from the next review round.
- **ATM-V0-014** Bounded repair. A ticket may loop review → repair at most N rounds (default 2 per
  ticket, pinned by the decision-0052 coordinator call).
  Each finding carries a stable key of (normalised path, claim class from a published enumeration,
  **bytes-anchor**, **text-anchor**) — the digest of the cited source bytes and the digest of the
  normalised claim text, both, never one or the other. A single "text OR bytes" anchor fails both
  ways: text-only merges two findings a reviewer worded identically about different lines, and
  bytes-only merges two different findings about the same line. The two anchors are used at
  different scopes. **Within a round**, distinctness is on the full 4-tuple, so two findings that
  differ in either anchor stay distinct. **Across rounds**, survival and escalation are matched on
  the 3-tuple *without* the text-anchor, so a reviewer that re-words the same unrepaired finding
  cannot buy another round. Severity is an attribute, not part of either key, so a regrade does not
  mint a new finding; a finding whose path moved is matched by its bytes-anchor. A finding whose
  3-tuple survives a repair unchanged escalates to the owner as a blocked item instead of another
  round.
- **ATM-V0-014a** Re-review convergence (carried from §11's adopt list at Gate A round 3). A round
  that returns findings whose 3-tuples are all already present in the previous round's set is a
  **non-converging** round: it does not consume a repair round, it escalates immediately, so a
  reviewer that keeps restating the same unrepaired set cannot exhaust the ATM-V0-014 bound.
- **ATM-V0-015** Injection boundary. Repository content, tool output, and prior transcripts enter a
  reviewer prompt as quoted data; the tool screens packets for secrets (reuse Corvint's learning
  secret screen) and strips embedded directives before dispatch.

### 4.3 Documentation consistency

- **ATM-V0-016** Doc-consistency lane. Before acceptance, a dedicated reviewer checks that every
  document the task touched or that governs it (spec requirement rows, Agent digests, README claims,
  decision records, roadmap status lines) matches the delivered behaviour: each requirement id
  claimed implemented traces to a named test function or measurement whose failure the reviewer
  demonstrates — never to a file that merely mentions the id, which an inventory of literal mentions
  such as `script/spec-coverage-audit.py` would accept — and each behavioural claim names its
  evidence. The lane returns the same ACCEPT/REPAIR/REJECT shape as ATM-V0-012.
  **"Demonstrates" is a named mechanism, not a judgement.** A hunk is attributed to a requirement id
  by one of two named sources, checked in order: a `Requirement: ATM-V0-NNN` trailer on the commit
  that introduced the hunk, or — for a runtime or workflow that carries no commit trailers — an
  explicit `hunk_attribution` entry in the ATM-V0-023 configuration list mapping a path/range to an
  id. A hunk under neither source is **unattributed**, and the id it would have supported counts as
  untraced rather than silently matched by proximity. For a *test* trace the lane reverts the
  diff hunks attributed to that requirement id by either source (or applies an operator-declared
  mutation from the configuration list), completes the named build and test-discovery steps on
  **both** trees, and records the named assertion/oracle result. Compilation, setup, discovery,
  timeout, signal, or harness failure is `unverified`; none is an assertion failure. The pass
  condition is the ordered pair *(delivered tree: build/discovery succeeds and the named assertion
  passes; reverted or mutated tree: build/discovery succeeds and that same assertion fails with its
  expected classified failure)* — stated in that order, since the delivered tree is the green half.
  Without that pair the trace is `unverified` and the id counts as untraced. For a
  *measurement* trace the lane requires that the cited receipt's pinned tree equals the delivered
  tree, that the named field is present and not `NOT_OBSERVED`, **and** that the field's value
  satisfies a typed predicate the requirement names (comparison operator, expected value, unit, and
  measurement environment) — presence alone is not evidence of satisfaction: HDCV0-025 forbids
  turning an observed value into an acceptance decision on its own
  (`docs/specs/human-documentation-compiler-v0.md:706-708`), which is what a presence-only check
  would do. A field present, non-`NOT_OBSERVED`, and below the requirement's threshold is
  `unverified`, not passing. Both branches carry a fixture in
  §7 item 3, so a lane that demonstrates nothing cannot pass as evidence.
- **ATM-V0-016a** Governing set. The governing set is closed by a published rule in **exactly one
  step**, never transitively: for each requirement id the task touched, the declaring spec resolved
  through the `file` column of `docs/specs/REQUIREMENTS.tsv` (the named resolver), plus every
  document under an explicitly configured glob scope — for Corvint `docs/specs/**`,
  `docs/decisions/**`, `ROADMAP.md`, and `README.md` — that names an id the task changed. Ids with
  no definition line are exempt where the repository declares them so (`CEM-CB-*`, per
  `script/check-requirement-definitions.sh:9-10`). The exempt case is precisely **an id with a row
  in `REQUIREMENTS.tsv` whose resolved spec body contains no definition line**: all 51 `CEM-CB-*`
  ids do have rows with a resolvable `file` column, and the repository's own check skips them inside
  its definition-line loop (`script/check-requirement-definitions.sh:41`), not in a row lookup. An
  id with a row whose `file` does not resolve is not exempt. An unexplained unresolvable id
  is a lane finding, not a silent drop. The resolved set is printed in the lane receipt.
  ATM-V0-017's regeneration runs **before** this closure, and the closure is computed over the
  regenerated rows, not the committed ones — otherwise a stale `file` column silently narrows the
  governing set. The lane receipt records which rows the closure used and that regeneration
  preceded it.
- **ATM-V0-017** Generated documentation. The repository declares its generated documents as a
  configuration list of (path, generator command, comparison mode, companion consistency checks); no
  such declaration exists implicitly. For Corvint that list has **exactly one entry**:
  `docs/specs/REQUIREMENTS.tsv`, generated by `script/gen-spec-requirements.sh`, which prints rows
  to stdout rather than writing the file (`script/gen-spec-requirements.sh:56`) and so is compared
  by diffing its stdout against the committed file, with companion check
  `script/check-requirement-definitions.sh`. The tool regenerates each declared document and refuses
  acceptance on drift; because regeneration equality misses a deleted clause body whose generated
  row survives — the generator falls back to a table row when no definition exists, which is how
  commit dfd02ea removed eighteen GPK clause bodies with every gate green
  (`script/check-requirement-definitions.sh:4-9`) — it also runs each declared companion check.
  Regeneration runs **before** the ATM-V0-016a closure, and the receipt records that order.
  Generated documents can propose but never accept intent (AGENTS.md invariant 8).
- **ATM-V0-017a** Declared invariant checks. Repository checks that generate no document but must
  still pass before acceptance are a separate configuration list of (check command). Decision
  numbering belongs here, not in the generated-document list: `script/check-decision-numbers.sh`
  generates nothing, it globs decision filenames and fails on a numbering collision
  (`script/check-decision-numbers.sh:7`). Each entry runs before acceptance and its exit status and
  output are recorded in the lane receipt.
- **ATM-V0-018** Claim ledger. Status words ("accepted", "delivered", "shipped", "PASS") are listed
  with the evidence behind each. The **blocking set** is exactly: status words the diff introduced
  or changed, **union** status words appearing in the ATM-V0-016a one-step closure attached to a
  requirement id the diff changed. The second half is what makes §7 item 3 case (v) — drift in a
  governing document the diff never touched but that names a changed id — reachable at all; without
  it 018's "only what the diff touched" rule and that fixture contradict each other. A status word
  in the blocking set with no evidence blocks acceptance and is
  reported, never silently kept. Pre-existing unevidenced status words across the rest of the
  ATM-V0-016a governing set are a non-blocking inventory with counts in the lane receipt — Corvint
  alone carries `accepted` occurrences across 53 files in `docs/decisions/` as of 2026-09-04 (the
  file count is reproducible by directory listing; the occurrence count depends on a match
  predicate — case sensitivity, word boundary, front matter vs. prose — which the lane receipt
  must publish alongside the number rather than report bare), so a whole-set block would fail
  every first task and teach the operator to bypass the lane. The
  vocabulary is configuration and the receipt reports both the blocking set and the inventory count
  against its published predicate.

### 4.4 Evidence and receipts

- **ATM-V0-019** Every run, review, routing decision, and acceptance emits a canonical-JSON
  receipt pinned to the Git tree and commit it observed. Replay means re-deriving every
  deterministic decision byte-identically from the recorded inputs; runtime generations are
  recorded, not reproduced. The recorded inputs are, exactly: the pinned configuration document of
  ATM-V0-025; the stored ATM-V0-004 availability snapshot; the recorded ticket complexity band; the
  recorded required-capability set for that ticket, which is ATM-V0-004's other routing input and
  without which routing replay is not byte-derivable from this closed set; the
  **verbatim WQO observation and proposal bytes**; and the recorded per-finding ATM-V0-013 verdicts.
  The deterministic set is routing, verdict aggregation, and acceptance. **Wave replay is a
  re-check, not a re-derivation**: ATM-V0-002 forbids the tool from recomputing the closure, so
  replay re-checks the recorded wave against the proposal bytes `run` pinned as its first receipt
  and reports a mismatch, rather than claiming to re-derive a selection it never made. A command
  that claims a **single ticket outside any wave** has no proposal: it records the proposal input
  as absent and replay re-checks the recorded observation only, so the closed input set stays
  satisfiable in a slice that does not ship ATM-V0-002. Verdict aggregation is deterministic
  over the stored verdicts and never re-runs a reviewer, so no part of the deterministic set
  consumes a fresh runtime generation.
- **ATM-V0-019a** Chained receipts (carried from §11's adopt list at Gate A round 3). Receipts for
  one state directory form an append-only log in which each record carries `prev`, the digest of the
  record before it, so a receipt deleted or reordered after the fact is detectable by `audit`
  without trusting the record's own content. Signatures are **not** carried: v0 has no key
  management, and an unsigned chain still detects tampering by anyone who cannot rewrite the whole
  log.
- **ATM-V0-020** Read commands (`status`, `next`, `wave`, `receipt`, `audit`, `config`) never mutate
  the repository, the queue, or tool state. `wave` in particular prints an **unpinned** proposal and
  writes no receipt; pinning is `run`'s first receipt (ATM-V0-002), so no read command has to write
  in order for ATM-V0-002a and ATM-V0-019 to have something to check against. Mutating commands are
  named as such in `--help`.
- **ATM-V0-021** The tool makes no network calls of its own, other than those a registered runtime
  subprocess makes, each attributed to its runtime in the receipt. Every run reports a
  **process**-containment class using WQO-V0-013's enum verbatim rather than a vocabulary of its
  own, the enum
  `LINUX_CGROUP_V2|LINUX_PROCESS_GROUP_UNQUALIFIED|DARWIN_PROCESS_GROUP_UNQUALIFIED|UNSUPPORTED`
  (`docs/specs/work-queue-observation-v0.md:132`), ordered `UNSUPPORTED < process-group unqualified
  < LINUX_CGROUP_V2`, with `UNSUPPORTED` reserved for unsupported platforms and for mixed
  Darwin/Linux classes (`:323-324`). The development host is Darwin, whose class is therefore
  `DARWIN_PROCESS_GROUP_UNQUALIFIED`, not `UNSUPPORTED`. **No rung is a security claim**, including
  `LINUX_CGROUP_V2`: containment here is cleanup, not a sandbox (`:512`), and cgroup v2 does not
  contain egress. Network egress is `NETWORK_UNOBSERVED` at **every** rung — a WQO-V0-015 unknown
  code, never a containment class — with host state `HOST_UNOBSERVED` unless denial is
  enforced, and v0 claims no hermeticity (`:522`). v0 must not read as though containment is
  enforced anywhere.

### 4.5 Operator surface

- **ATM-V0-022** CLI first. Read commands: `status`, `next`, `wave`, `receipt`, `audit`, `config`.
  Mutating commands: `run`, `review`, `repair`, `accept`, `availability refresh`, `reconfigure`,
  `drain`. Output is JSON with a `--pretty` rendering. A dashboard, if ever built, is a rendering
  of on-disk state and is out of v0.
- **ATM-V0-028** Drain admission barrier (added at Gate A round 2; lock order and scope revised at
  round 6). **Stated acquisition order (the single order this document uses — §8 refers back
  to it and does not restate it):** `drain` first materializes an admission barrier file in the
  state directory; it then takes an exclusive advisory lock on a sibling file outside the state
  directory (`<state-dir>.lock`, `flock`); it then waits out the resident-supervisor grace period;
  only then does it cancel and delete. While the barrier is present every mutating command fails
  closed before it attempts any compare-and-swap, so no invocation can win a claim between drain's
  cancel sweep and the deletion of the state directory. `drain` itself is
  **exempt from its own barrier** — it is
  the one mutating command the barrier does not refuse — because a barrier that refused every
  mutating command including `drain` would make an interrupted drain unretryable and leave the tool
  permanently refusing work with lanes still live. `drain` is therefore idempotent and resumable:
  re-running it over a state directory that already carries the barrier resumes the cancel sweep
  from whatever is still recorded live and completes the removal. Because the barrier file lives
  inside the directory `drain` removes, the barrier alone cannot serialize `drain` against `drain`,
  nor against a `run` starting the instant the directory disappears, which is why the exclusive lock
  exists. Every mutating command of ATM-V0-022 — `run`, `review`, `repair`, `accept`,
  `availability refresh`, `reconfigure` — takes that same lock **shared, only around each
  individual compare-and-swap transaction** (one claim, or one state write), releasing it
  immediately after; it is **never** held for a lane's or an invocation's full lifetime — the
  round-5 text that held it for "the whole claim/write lifetime" is struck, since a shared holder
  resident for a lane's entire
  run would make `drain`'s exclusive acquisition wait on that lane's wall-clock cap rather than on
  one transaction. Before re-acquiring the lock for its next transaction a resident supervisor
  (ATM-V0-005c) re-reads the barrier and exits without writing if it is present, rather than
  acquiring the lock again. This is what bounds `drain`'s wait: no shared holder can hold the lock
  past its current transaction, so `drain`'s exclusive acquisition waits at most one in-flight
  transaction per resident supervisor plus the configured drain grace period — never the largest
  configured wall-clock cap, since a supervisor between transactions holds no lock at all and is
  already refusing new writes once it observes the barrier. `drain` holds the lock **exclusive** for
  the whole cancel sweep once acquired. The admission barrier stays as the fast fail-closed check,
  not as the serializer, because it lives inside the directory `drain` removes. Acquisition **fails
  closed on any error other than "held by another process"** — `ENOLCK`, `EOPNOTSUPP`, or a mount
  whose advisory locking does not work — and that failure is reported as a refusal, never a
  warning the command proceeds past. `drain` waits
  for every resident supervisor for a **configured drain grace period (default 60 s)** and then
  cancels whatever remains by its recorded process group; a committed test asserts this wait is
  bounded by the grace period under the shared-lock-per-transaction model above, never by a lane's
  wall-clock cap. Nothing writes into the directory being removed. This follows the upstream global
  operator pause, which is likewise a hard admission barrier (`script/roadmap.sh:14428-14438`).

### 4.6 Configuration

- **ATM-V0-023** Policy is data. The runtime registry, both routing tables (runner and expert),
  budget caps, the repair bound, reviewer charters and prompt templates, the doc-consistency rules,
  the generated-document list, the ATM-V0-017a declared-invariant-check list, the claim vocabulary,
  queue adapter *selection and toggles*, and every lane toggle are
  configuration. **Not configuration:** the ATM-V0-001a read-verb and ATM-V0-001c write-verb
  allowlists, which are embedded code. Configuration can disable a listed verb through exactly one
  key, `adapters.verbs.disabled`, whose value must be a subset of the embedded lists; nothing an
  operator writes can add one, because ATM-V0-024 lists replace and an operator-extended read list
  would put a mutating verb on the read path. Verb names therefore do legitimately appear in `config
  show --effective` as values of that key; neither allowlist itself ever does.
  Built-in defaults are one embedded configuration document validated by the same
  schema, not literals spread through code. The rule is syntactic so the lint is decidable: every
  policy value is reached through the configuration accessor type, and the lint fails on any literal
  of that type constructed anywhere outside the embedded defaults document. A **policy literal** is
  exactly such a construction. §7 item 4 carries a negative fixture the lint must fail; the lint's
  own-source run **excludes the fixture path named in the schema**, so the two halves of that check
  are not contradictory — the same lint would otherwise have to pass on a tree containing a file it
  is required to fail on.
- **ATM-V0-024** Layered sources. Built-in defaults < user config < repository config < invocation
  flags and environment. Each layer has exactly one fixed path — no directory search, no
  first-match-wins walk; a second file at a layer is a load error, and a file at an unexpected path
  is not read at all. Merging is: scalars replace, maps merge key-wise, lists replace unless the
  schema marks the field additive. JSON, YAML, and TOML are accepted through one schema; every file
  is decoded into the JSON data model *before* validation and any construct outside that model is
  rejected at load rather than coerced — YAML 1.1 implicit booleans (`yes`, `on`, `n`), YAML
  non-string mapping keys, TOML datetimes, and TOML's absence of null are the named cases. The
  schema is published with the tool and every file is validated before use. `config show
  --effective` prints the merged result with the layer that set each key.
- **ATM-V0-024a** Non-overridable keys (carried from §11's adopt list at Gate A round 3). A named
  key set — credential and auth material, and runtime binary paths — is settable only at the user
  layer and by invocation flags, and is **never** read from repository config. The ATM-V0-024
  precedence otherwise puts repository config above user config, which for these keys would let a
  checked-out repository redirect the tool's credentials or point a runtime at a binary of its
  choosing. A repository-config file that sets a key in this set is a **load error**, not a silently
  ignored key, so the operator sees the attempt.
- **ATM-V0-025** Configuration is pinned per wave. The **resolved effective configuration document
  itself**, not only its digest, is written into the state directory at wave start, and it is the
  only configuration a claimed lane reads for the rest of its life; the digest is carried in every
  receipt (ATM-V0-019) as the pointer to it. A digest alone would leave the next invocation unable
  to reconstruct what the lane was run under once the source files moved on. A re-read applies only
  to lanes not yet claimed, so routing stays a pure function of recorded inputs (ATM-V0-004).
  The **pin unit** is the wave, or — where a command claims a single ticket outside any wave — that
  single-ticket run, so there is no claimed lane without a pinned document. An invocation flag or
  environment value that would change the policy a lane has already claimed under is a **load
  error**, not a silent override: ATM-V0-024's top layer would otherwise beat the pin.
  Changing routing or caps for lanes already in flight requires the explicit mutating `reconfigure`
  command, which emits its own receipt, cannot lower a cap below usage already observed on the
  affected lane, does not disturb a review round already started (ATM-V0-013a pins the verifier
  tuple at round start, and a verifier-routing change applies from the next review round), and
  re-checks the ATM-V0-005b stale-threshold inequality against the new caps —
  raising a wall-clock cap through `reconfigure` would otherwise reintroduce exactly the
  reclaim-inside-cap window 005b exists to close.
- **ATM-V0-026** *Struck at Gate A round 1: speculative under the code-shape doctrine. v0 promises
  no embedding host, so a fluent options builder over the CLI's persistence layer would be a
  single-use abstraction. Out of v0 and out of §10; revisit only if a v0 embedding host appears.*
- **ATM-V0-027** Config changes are tasks. A repository-config diff runs through the same review
  and doc-consistency lanes as code, because it changes who reviews what at which tier.

## 5. Non-goals (v0)

Hosted service, permanent daemon, GUI, in-process vendor SDKs, the tool calling models directly (it
drives runtime subprocesses), replacing Corvint specs or the expert catalog, any "perfect reviewer"
guarantee, and advancing any ticket a queue adapter reports as non-autonomous under ATM-V0-005d.
**Auto-merge is a non-goal outright** (amended at Gate A round 3; previously "auto-merge without a
green gate and an ACCEPT", which left the merge stage half in scope). The tool records a verdict and
transfers the lease; `merge`, `closeout`, and `checkoff` are absent from the ATM-V0-001c write list,
and the merge stage stays with the queue adapter's own operator.

## 6. Failure modes and containment

| Failure | Containment |
|---|---|
| Runtime outage or quota | ATM-V0-004 availability snapshot + ATM-V0-009 explicit degradation |
| Observation not `VALIDATED_AT` | ATM-V0-002 yields no wave; state and unknown codes reported verbatim |
| Wave stale between propose and run | ATM-V0-002 `run --from-proposal <file>` pins the bytes `wave` emitted and refuses on queue-identity or per-ticket-closure drift against them; without the flag the check is confined to one `run` invocation (ATM-V0-002a), and a checkpoint change is ordinary re-observation |
| Another manager root claims the ticket first | ATM-V0-005 `TAKEN → ready`; the loser re-observes and continues the wave |
| Upstream queue paused by the operator | ATM-V0-001b no wave, every claim refused; the tool never sets the pause override |
| Write path refused under an upstream pause | ATM-V0-001b the gated `case` list (`script/roadmap.sh:14433`) covers the ATM-V0-001c verbs `claim`, `begin`, `finish`, `apply-verdict`, `adopt-review`; `heartbeat` is **not** gated, which is what lets ATM-V0-005a's adapter-heartbeat refresh survive a pause; a gated refusal plus a positive `pause-status` (itself ungated, `:14453`) is `ADMISSION_PAUSED`, carried out of band, never `ADAPTER_FAILED` |
| Wave member gone at re-observation | ATM-V0-002a the member is dropped from the wave; the invocation continues with the rest |
| Non-autonomous ticket (human, approval, design, never, multi-repo) | ATM-V0-005d `blocked` at claim; never advanced |
| Two coordinators on one state directory | ATM-V0-005a compare-and-swap; one state directory per checkout |
| Build failure, red gate, cancelled lane | ATM-V0-005 explicit `build_failed` / `gate_failed` / `cancelled` edges, each left only by a named recovery rule and bounded by the ATM-V0-014 repair count so a lane cannot re-enter the same failed edge without limit |
| Merge conflict or red gate at acceptance | ATM-V0-005 `accepting` aborts to `gate_failed`; checkoff is idempotent, never partial |
| Atomic-write failure or ENOSPC | ATM-V0-005a transition fails closed; the lane keeps its prior recorded state |
| Reviewer contamination | ATM-V0-008/008a disjointness receipt; unexposed fields record `NOT_OBSERVED`, review refused only on an observed contradicting field |
| Reviewer certifies its own finding | ATM-V0-013 verifier disjoint from builder and from the raising reviewer; ATM-V0-013a one verbatim finding per dispatch |
| Cheap model false ACCEPT | ATM-V0-011 table gated by §7 routing evaluation; `audit` fails when the pinned ATM-V0-025 document's routing-table digest differs from the digest the last passing §7 evaluation recorded |
| Routing table drift | table is data under version control; `audit` fails when the pinned ATM-V0-025 document's routing-table digest differs from the digest the last passing §7 evaluation recorded, so a table changed without a passing evaluation is detected at run time, not only pre-release |
| Cost blowup | ATM-V0-006 caps with active cancellation |
| Token cap unenforceable on a runtime | ATM-V0-006 records `NOT_ENFORCED` per AT-01 field, not per runtime; wall-clock and turn caps still apply; intra-turn overshoot is recorded, not hidden |
| Stale leases / crashed lanes | ATM-V0-005 heartbeats, ATM-V0-005b threshold, ATM-V0-005a preserve-then-free |
| Supervisor died while its lane still runs | ATM-V0-005c `SUPERVISOR_LOST`; the next invocation cancels by process group rather than reclaiming |
| Reclaim of a lane whose runtime survived the kill | ATM-V0-005a liveness check on the process group; `RECLAIM_REFUSED` and escalate, never free |
| Pid recycling makes a dead lane look live | ATM-V0-005a liveness is (pgid, group-leader start time); a start-time mismatch is dead and is never signalled |
| Two liveness authorities (tool heartbeat vs adapter reap) | ATM-V0-005a the adapter that owns liveness is the sole reclaim authority; the tool's reclaim path is disabled for it |
| Adapter reaps an adapter-owned lane inside its cap | ATM-V0-005a the resident supervisor issues `heartbeat` calls on its own timer, below the adapter's stale window (`ROADMAP_STALE_MIN=90`, `script/roadmap.sh:109`), independent of `work-loop`'s before/after-only calls; ATM-V0-005b's twice-the-cap inequality binds on the tool's own threshold only, so it does not reject caps the refresh already makes safe |
| Interrupted `drain` leaves the barrier in place | ATM-V0-028 `drain` is exempt from its own barrier, idempotent, and resumable; re-running it completes the sweep |
| Resident supervisor writes during a `drain` | ATM-V0-005c the supervisor re-reads the barrier on its heartbeat interval and before every state write, and exits without writing; ATM-V0-028 `drain` waits a configured grace period (default 60 s) then cancels the remainder by recorded pgid, so the wait is bounded |
| `reconfigure` raises a cap past the stale threshold | ATM-V0-025 `reconfigure` re-checks the ATM-V0-005b inequality and is refused when it would break it |
| Availability snapshot missing or stale at routing | ATM-V0-004 routing refuses rather than probing; `availability refresh` is the named mutating command that supplies one |
| Invalid or duplicated configuration layer | ATM-V0-024 one fixed path per layer; a second file at a layer, or a construct outside the JSON data model, is a load error; ATM-V0-024a repository config setting a non-overridable key is a load error |
| Config file changed between invocations on a claimed lane | ATM-V0-025 the lane reads only the pinned document written at wave start; changes need `reconfigure` |
| Dangling `accepting` lock after a crash | ATM-V0-005 `accepting` is re-entrant and idempotent post-fast-forward; the next invocation that wins the ATM-V0-005a CAS resumes it |
| Runtime survives the process-group kill | §8 `drain` reports every survivor process it could not kill; ATM-V0-005a refuses reclaim of a lane still live |
| Runtime writes outside its lane | `HOST_UNOBSERVED` in v0 and declared so: ATM-V0-021 makes no containment claim and a worktree is a path convention, not a sandbox; the one detector is a dirty check of the shared checkout at lane exit, recorded in the lane receipt |
| Rollback while lanes are in flight | §8 `drain` cancels lanes and releases leases before the state directory is removed |
| Mutating invocation races `drain`, or a second `drain` | ATM-V0-028 admission barrier materialized before the exclusive lock; every mutating command fails closed while it is present; every mutating command holds `<state-dir>.lock` shared only around each CAS transaction and `drain` holds it exclusive; the lock lives outside the directory and outlives its deletion, so no mutating command is mid-CAS during a sweep, `drain`'s wait is bounded by one transaction plus the grace period rather than any lane's wall-clock cap, and two drains can never both sweep |
| Doc drift accepted | ATM-V0-016, 016a, 017, 017a block acceptance; ATM-V0-018 blocks on diff-introduced status words and inventories the rest |
| Prompt injection via repo content | ATM-V0-015 |
| Receipts trusted as input by `audit` | receipts are data under ATM-V0-015; `audit` re-derives decisions (ATM-V0-019) rather than trusting recorded verdicts |
| Finding loops forever | ATM-V0-014 escalation on the finding key |

## 7. Acceptance evidence (what "best" is allowed to mean)

1. **Dogfood.** The tool runs one Corvint `AT-` ticket end to end (Gate A → build → review → repair
   → accept) producing every receipt, replacing the hand-driven loop of 2026-09-04. The comparison
   is per-ticket observed input, cache-read, and output tokens plus turns against a re-measured
   single-ticket slice: the wave-2 receipt
   `benchmarks/results/dogfood-workers-at-wave2-2026-09-04.json` spans many tickets with five lanes
   excluded and `NOT_OBSERVED` cost and cacheCreation fields, so it is not comparable whole, and
   every `NOT_OBSERVED` baseline field is excluded and named. The Corvint-side accounting prerequisites
   landed at `4e8e088`: `benchmarks/dogfood_workers.py` now admits host `atm`, accepts an optional
   per-worker `ticket`, and raises `MAX_WORKERS` to 256; its 20 focused tests passed. This does not
   retroactively add ticket fields to the wave-2 receipt. Of that receipt's 63 workers, only 20 are
   unambiguously attributable to one ticket from their ids; the 39 Claude `agent-<hex>` lanes, three
   non-ticket Codex/coordinator lanes, and `codex-at0607-gatea2` remain unsuitable for a single-ticket
   baseline. The acceptance comparison therefore still requires a newly measured single-ticket
   slice whose manifest supplies `ticket` where known; missing attribution stays missing rather than
   inferred.
2. **Routing evaluation.** A frozen corpus of review packets is committed under
   `benchmarks/corpora/` with a manifest recording per packet: sha256, `transcriptSha256`, runtime,
   model, effort, the tree sha reviewed, `packetKind`, and the diff bytes. `transcriptSha256` pins
   the retained transcript the ATM-V0-012 byte-containment test runs against; without it the
   manifest describes a verdict whose "opened" grades cannot be recomputed. `packetKind` is one of
   `diff-review | plan-review | hunt`, and the diff field is null for the non-diff kinds; a single
   ≥20 floor over "packets with diff bytes" is unreachable. The census is pinned to a **named
   freeze**, never to "currently in the scratchpad": at the Gate A round-3 freeze the
   `codex-*.jsonl` sidecar family held exactly 31 members, and the census over that frozen set is
   **13 diff-review, 11 plan-review, and 7 hunt** — re-cut at that freeze over the sidecar family
   rather than the 30 `.out` files, which is why it differs from the round-2 figures. The family is
   not re-counted live at admission time — a scratchpad that later accumulates more
   `codex-*.jsonl` files (it holds 37 as of round 6) does not move this census; the manifest
   records the freeze's own tree state so
   the 31-member count stays checkable against what was frozen, not against whatever the directory
   currently contains. The scorer packet is **not
   in the census**: `codex-scorer` exists only as `.md`/`.log`/`.out` with no `.jsonl`, so it can
   supply no `transcriptSha256` and is excluded for lack of a transcript. The floor is therefore
   stated per kind against what existed at that freeze: **10
   diff-review, 8 plan-review, 5 hunt**, pinned by the decision-0052 coordinator call. These are
   **whole-corpus packet floors before partitioning**, evaluated and re-pinned at every manifest
   freeze. The manifest records admitted and excluded counts by kind plus every exclusion reason; a
   kind below its floor is reported as such rather than padded. The packets reviewed working-tree state that no
   longer exists: for the already-produced packets the manifest **stores the reviewed bytes** — the
   cited files' blobs with a sha256 each — because a pinned checkout cannot reproduce a dirty tree;
   every future packet is dispatched only after `git write-tree` on the working tree, and that tree
   sha is recorded. The corpus is Codex verdicts only and the manifest says so; no Claude
   *diff-review* packet was captured this session, but the Gate A round-1 report was, and it is
   admitted as a `plan-review` packet once committed with its manifest row. Packet vocabularies
   differ ("accept / repair first",
   "proceed / revise first", never REJECT, items unnumbered), so the manifest publishes the
   normalisation mapping to ACCEPT/REPAIR/REJECT with numbered items; an unmapped packet is excluded
   and counted. Labels are **not** minted by an ATM-V0-013 `confirmed` grade alone: OCA-V0-004
   is explicit that "one owner may label and evaluate a corpus, but its result is owner-labelled
   evidence, not independent validation, promotion authority, or a truth claim about unobserved
   behavior" (`docs/specs/observation-corpus-authority-v0.md:36-37`), and `confirmed` criterion
   (ii) is a verifier session's own semantic reading — same-model agreement between a reviewer
   and a disjoint verifier is not independent evidence the finding is true. A finding mints a new
   label-set version only when either the **coordinator or owner explicitly adjudicates** it as
   true, or ATM-V0-013 criterion (iii)'s machine check confirms it against a **deterministic**
   target the claim itself names (a named test's exit status, a receipt field's value) — never
   from the verifier's prose agreement alone. A `confirmed` finding with no deterministic target
   and no owner adjudication is reported in a separate **`corroborated`** bucket — inferred
   evidence per OCA-V0-004, never a routing-truth label — and does not enter the frozen label
   set. The label set itself is **versioned, not open**: a version is the union over every
   corpus-producing configuration to that point, and every admission is stated against a named
   version. **The label-set version is frozen once per admission matrix, before any row in that
   matrix runs, and no row's own discoveries mint a new version mid-matrix**: a finding a
   candidate row mints (by adjudication or deterministic confirmation) is recorded but deferred to
   the *next* cycle's freeze,
   the same way the refresh rule below defers new packets. This closes an order-dependence the prior
   rule left open: scoring each row against "the version frozen before its own run" let an earlier
   row's mint change a later row's denominator within the same matrix, so permuting row order could
   permute results. Raised HIGH findings are
   therefore partitioned **four** ways, not two: *matched-label* (present in the version frozen for
   this matrix), *confirmed-but-unlabelled* (mints a label for the next cycle; absent from this
   matrix's frozen version by construction), *unverified* (ATM-V0-013 could neither confirm nor
   refute it — the machine check failed, or the verifier's runtime declared no
   conversation-identifier field), and *unconfirmed* (refuted by the verifier — a false HIGH).
   Recall is measured against the frozen version; *confirmed-but-unlabelled* and *unverified*
   are both excluded from both numerators and from the t2 denominator, and each is reported as
   its own count; only refuted raises count against t2. Without the *confirmed-but-unlabelled*
   partition a row that finds what the corpus missed would score as a false positive, and the
   ATM-V0-011 gate would select against reviewers that find new true positives; without
   excluding *unverified* from the
   denominator, a true finding the verifier could not process would score as a false HIGH through no
   fault of the row being scored, making admission depend on the verifier's own routing rather than
   on the row. **Denominator formulas, published in the manifest:** recall = |matched-label raises| /
   |labels in the frozen version|; false-HIGH rate = |unconfirmed raises| / |all HIGH raises
   excluding confirmed-but-unlabelled and unverified|. A closed Codex-only set would score a reviewer
   that finds something Codex missed as a false positive. The corpus is partitioned per packet with
   the assignment and its seed recorded at freeze time: **60% held out, stratified by packet kind**.
   The overall held-out count is the nearest integer to 60% of admitted packets and is allocated
   across kinds by largest remainder, with ties broken in the manifest's published kind order. Thus
   the 13/11/7 freeze yields 8/7/4 held-out and 5/4/3 development packets. A development partition
   chooses the (model, effort) table, a held-out partition is run once to admit it, and re-partitioning
   invalidates the corpus. Because that partition is burnt by the admission it serves, the corpus
   carries a **refresh rule** rather than a single admission: each admission cycle appends new
   packets, and the held-out partition for cycle k+1 is drawn — seed recorded — only from
   packets added after cycle k's freeze. A further table change with no packets added since the
   last freeze has no admissible evidence and is refused, rather than admitted on a burnt
   partition. The metric is micro-pooled per finding across packets, not averaged per
   packet, so a packet with zero labels cannot make a metric undefined; a zero-label packet
   contributes only to the false-HIGH denominator, which is what stops a row that raises nothing
   from scoring a perfect false-HIGH rate. The manifest additionally carries, at freeze time, a
   **whole-corpus, pre-partition per-kind minimum confirmed-HIGH label count** of **9 diff-review,
   6 plan-review, and 3 hunt**, re-pinned at every manifest freeze, and the label counts actually
   found are
   reported: the 10/8/5 floors above are packet counts, whereas recall and false-HIGH are
   micro-pooled per finding over labels that do not exist until the ATM-V0-013 verifier has run the
   corpus. A kind below its label minimum is omitted from that cycle's recall numerator and
   denominator; its packets still contribute eligible raises to the false-HIGH denominator, and any
   admission is explicitly scoped to the kinds that met their minimum. The whole-corpus minima do
   not imply any held-out label count because labels are not uniformly distributed across packets.
   Admission uses the decision-0052 coordinator pins **t1 = 0.80** for held-out micro-pooled recall
   and **t2 = 0.30** for the held-out false-HIGH rate. t1 is evaluated only when the eligible
   held-out frozen label set contains at least 10 findings; otherwise the cycle admits nothing and
   reports the counts. t2 is evaluated only when the row raises at least 10 denominator-eligible
   held-out HIGHs; below 10, raw counts are reported and admission turns on t1 alone. These thresholds
   are admissibility screens, not evidence that two rows are statistically distinguishable. No
   relative-advantage claim is permitted without a separate preregistered paired analysis. t1 and t2 are
   pinned once per admission cycle **after the label census and before the first candidate row runs**;
   they may be re-pinned only at a new cycle's freeze. A cycle whose pin event does not precede its
   first candidate run event is refused, not re-scored. That ordering is auditable rather than asserted: the threshold pin and
   each label-set version freeze are **events in the ATM-V0-019a append-only chain**, every
   candidate row's receipt records the chain head and the manifest sha256 at dispatch, and a row is
   admitted only when its pin event precedes its run event in that chain. The pin event's digest is
   additionally **committed alongside the manifest itself**, since the coordinator both pins the
   thresholds and writes the chain and so is the one party whose own edit the chain alone cannot
   rule out; the committed digest gives a second, independently-timestamped anchor an editing
   coordinator would also have to rewrite. Detectability is stated accordingly: the chain plus the
   committed digest detects a manifest **edited by a third party** after the fact, which a mutable
   file alone never is; it is not a claim that an unsigned v0 chain (ATM-V0-019a) rules out the
   coordinator's own edit of its own pin before committing it.
   Packets are (re-)produced
   under a mode whose registry entry declares `transcript_exposes_tool_io: yes`, or ATM-V0-012's
   "opened" test is undecidable across the whole corpus.

   **Proposed follow-up — unaccepted; no implementation or admission authority.** Before automatic
   routing is promoted beyond the accepted point-estimate screen, an owner-accepted amendment must
   pin: (a) a maximum `unverified / all HIGH raises` admission rate or an equivalent abstention rule,
   so unverifiable noise cannot be excluded without bound; (b) one-sided confidence-bound gates or
   justified larger effective samples per admitted kind, with t1/t2 retained as minimum effect-size
   floors; and (c) a matched expert-versus-generic ablation at identical runtime, model, effort,
   packet, and budget. Each ablation row records expert id, candidate expert set, risk class,
   selector inputs and selected outcome, aggregate ticket/wave fan-out, distinct confirmed HIGHs per
   100,000 input tokens, and distinct confirmed HIGHs per wall-hour. No numeric ceiling, confidence
   level, or superiority margin is accepted here; each remains `NOT_PRODUCED`, and this proposal does
   not change the decision-0052 thresholds or current screening semantics.
3. **Doc-consistency fixture.** A seeded repository must be rejected by the lane in each of the
   cases (i), (ii), (iv) through (ix), (vii-b), and (xi) below, and its correct counterpart must
   pass — every case names the requirement it discriminates, and every requirement in §4.3 is
   named by at least one case, so a fixture that discriminates nothing cannot pass as evidence.
   Cases (iii) and (x) are stated separately after them: both are the **accept-side** counterpart
   of a seeded-mutation or seeded-inventory case and assert a pass, not a rejection, so a
   harness driven off this preamble alone would wrongly assert REJECT for either:
   (i) a roadmap saying "delivered" for a requirement with no test at all (ATM-V0-016);
   (ii) a requirement id mentioned in a test file where no test asserts the behaviour — the case a
   literal-mention inventory accepts (ATM-V0-016);
   (iv) a clause body deleted while its generated row survives, the dfd02ea shape, rejected by the
   companion check and not by regeneration equality (ATM-V0-017);
   (iv-b) a drifted generated row — a line or title changed in the committed
   `docs/specs/REQUIREMENTS.tsv` that only the generator's stdout diff catches — rejected by
   regeneration and not by the companion check, the mirror image of (iv) (ATM-V0-017);
   (v) drift in a governing document the diff never touched but that names a changed id, rejected
   through the ATM-V0-016a closure and the ATM-V0-018 blocking-set union (ATM-V0-016a, ATM-V0-018);
   (vi) a seeded decision-number collision — two `docs/decisions/NNNN-*.md` files sharing a number,
   chosen so as not to reuse the pair `script/check-decision-numbers.sh:11` grandfathers — rejected
   by the ATM-V0-017a declared-invariant-check list, which no generated-document comparison catches;
   (vii) a measurement trace whose cited receipt pins a tree other than the delivered tree, or whose
   named field is `NOT_OBSERVED`, rejected by ATM-V0-016's measurement branch;
   (vii-b) a measurement trace whose cited receipt pins the delivered tree and whose named field is
   present and not `NOT_OBSERVED`, but whose value fails the requirement's typed predicate — an
   observed-but-below-threshold value — rejected by ATM-V0-016's predicate check and not merely by
   presence (ATM-V0-016);
   (viii) a stale `file` column in the committed `docs/specs/REQUIREMENTS.tsv` that regeneration
   corrects, seeded so the closure over the committed rows narrows the governing set and the
   closure over the regenerated rows does not — caught only when regeneration precedes the
   closure (ATM-V0-016a ordering);
   (ix) a requirement id with a `REQUIREMENTS.tsv` row whose `file` column does not resolve, which
   is **not** exempt and is a lane finding rather than a silent drop (ATM-V0-016a);
   (xi) a pre-existing unevidenced status word in the ATM-V0-016a governing set that the lane's
   inventory **omits entirely** — never listed at all, not merely reported as non-blocking —
   rejected as an incomplete inventory (ATM-V0-018), the failure mode (x) below does not cover.
   Accept-side counterparts, asserting a pass rather than a rejection:
   (iii) a named test shown to fail under a seeded mutation, which **must pass** the lane, proving
   the lane demonstrates failure rather than trusting the name (ATM-V0-016). That fixture is
   otherwise **lane-clean** — it passes ATM-V0-017 regeneration, every ATM-V0-017a declared check,
   and the ATM-V0-018 blocking set — so a REJECT raised by an unrelated seed cannot be mistaken
   for a failure of this case.
   (iii-b) a hunk carrying a `Requirement:` trailer for a seeded id, reverted by the lane per that
   trailer, whose named build and discovery succeed on both trees and whose named assertion then
   fails with the expected classified failure on the reverted tree and passes on the delivered tree —
   **must pass** the lane through the revert branch specifically, distinct from (iii)'s
   operator-declared-mutation branch, so the trailer/map attribution mechanism (ATM-V0-016) has a
   fixture of its own and not only the mutation path.
   (x) an unevidenced status word in a governing document the diff never touched and that names no
   changed id, correctly **listed** in the lane's non-blocking inventory with its count — this
   **must not block acceptance**, the inventory half of ATM-V0-018 that case (v) does not exercise,
   and is distinct from (xi)'s omission failure above.
4. **Configurability check.** Every knob in §4.6 is reachable from `config show --effective`
   and from at least one of the three file formats; the policy-literal lint passes on the tool's
   own source — run with the fixture path named in the ATM-V0-023 schema **excluded**, so the two
   halves of this check are not contradictory — **and** fails when run on that committed negative
   fixture, which constructs a configuration-accessor literal outside the embedded defaults
   document, so a lint that detects nothing cannot pass. The check also asserts, exactly, that no
   key whose value **is** the ATM-V0-001a read allowlist or the ATM-V0-001c write allowlist appears
   in `config show --effective`, since both are embedded code rather than knobs. It does **not**
   assert that no verb name appears: `adapters.verbs.disabled` (ATM-V0-023) legitimately carries
   verb names as its value. The check additionally asserts that every ATM-V0-013a review receipt
   schema instance carries a `boundary_crossed` field (boolean, reviewer-self-declared per
   ATM-V0-010) even when its value is `false` — presence-only, since ATM-V0-010 already forbids
   the field from changing any grade or verdict, and without this the field's very existence
   would go unasserted.
5. **Comparison arm.** The same ticket set run three ways: the manual loop, `roadmap.sh`, and the
   tool. The manual-loop arm is the same re-measured single-ticket run item 1 defines, not the
   wave-2 receipt, so the baseline is defined once in this section rather than two ways. Any
   replayed packet runs against the tree its manifest entry pins, or against its stored reviewed
   bytes where the reviewed tree was dirty. Reported: cost, wall clock, confirmed HIGH findings
   caught, doc drift caught, false
   ACCEPTs. "Best in the world" is an external claim the tool never emits; this table is the only
   thing the repository may cite. **Proposed follow-up — unaccepted; no implementation authority:**
   preregister a paired seeded-ticket outcome/cost decision rule, aggregate ticket/wave fan-out
   ceiling, critical-miss gate, and win/kill margin before using this comparison to claim the tool is
   better. Those values are `NOT_PRODUCED`; until accepted, the comparison is descriptive only.

## 8. Rollback

Rollback is the mutating `drain` command followed by deleting the tool's state directory. `drain`
follows the single acquisition order ATM-V0-028 (§4.5) states: materialize the admission barrier,
take the exclusive `<state-dir>.lock`, wait out the resident-supervisor grace period, then cancel
and delete. Because every other mutating command holds that lock only around one CAS transaction
at a time (ATM-V0-028), once `drain` holds it exclusive no shared holder is mid-transaction against
the state directory being removed, so nothing is still writing into it. `drain` then cancels live
lanes by process group (SIGTERM, then SIGKILL after a grace period), releases every adapter lease
the tool holds through the ATM-V0-001c `release` write verb, removes lane worktrees, reports any
process it could not kill, and exports the receipts. `drain` is exempt from its own barrier and is
idempotent, so an interrupted drain is retried by re-running it rather than by hand-editing the
state directory.

Runtime-authored changes are confined to lane worktrees and run branches, so
reverting what reached the repository is `git` on those paths and on the branches the receipts name.

## 9. Resolved owner inputs and coordinator calls

- **Repository and binary:** `~/projects/corvint-taskman`, binary `atm` (decision
  0052).
- **Beamfall authority: confirmed.** The tool is only a short-lived ADR-0206 manager root. ADR-0206
  §1 paragraphs 3–4 make manager roots the only orchestration actors and permanently pin claim,
  review, and merge authority to `manager`; `beamfall/script/roadmap.sh:8543-8558` rejects every
  other authority value. Its supported write surface is the seven existing verbs `claim` (`:3814`),
  `release` (`:5307`), `heartbeat` (`:6917`), `finish` (`:7560`), `begin` (`:10637`),
  `apply-verdict` (`:13648`), and `adopt-review` (`:13939`). The adapter gains no dispatcher or
  stage authority.
- **Default per-lane caps:** `inputTokens 2,000,000`, `cacheCreationTokens 1,000,000`,
  `cacheReadTokens 100,000,000`, `outputTokens 400,000`, `turns 400`, wall clock 90 minutes, and
  two repair rounds per ticket. Unreportable token fields remain `NOT_ENFORCED`; turns and wall
  clock remain enforced. §4 records the observed wave-2 maxima these rounded caps cover.
- **Routing evaluation:** whole-corpus packet floors are 10/8/5 and whole-corpus confirmed-HIGH
  label minimums are 9/6/3 for diff-review/plan-review/hunt. The held-out packet fraction is 60%,
  stratified by kind using §7's recorded-seed allocation. t1 is 0.80 with at least 10 eligible
  held-out labels; t2 is 0.30 and is evaluated only with at least 10 denominator-eligible held-out
  HIGH raises. §7 defines scope, insufficient-denominator behavior, and the two pin clocks. These
  are coordinator calls under decision 0052, not empirical proof of optimal thresholds.
- **Ad-hoc expert recurrence:** one use, per ATM-V0-010 and the standing 2026-09-03 owner rule.
- **Build rather than adopt:** §11.1/§11.2 show ATM-V0-008, 011, 016, and 019 are unmet in full by
  every surveyed tool and by the combined field. Decision 0052 authorises the standalone build.
- **Answered 2026-09-04:** when the tool starts running the queue — the owner's fourth §1 quote
  makes the open `AT-` tickets its first real queue as soon as the S1 runtime, S2 review verification,
  and S3 documentation-consistency lane have landed. That first demonstration is explicit-reviewer
  mode. S5's sealed routing digest is a later precondition for automatic reviewer selection, not for
  the explicit-reviewer demonstration.
- **Coordinator call, recorded here rather than asserted as a property of §11:** the landscape
  survey is a first pass and does not gate Gate A closure. This is the coordinator's decision about
  sequencing, reversible by the owner, not a spec label. The Qodo and Faramesh reads the first pass
  named were dispatched 2026-09-04 and returned; §11.2 records them and they did move scorecard
  rows. **Revisit recorded 2026-09-04 (Gate A round 3):** the carve-out was re-examined after §11.2
  landed and **the call stands** — the moved rows sharpen ATM-V0-016 and 019 but leave the §11.1
  verdict (008, 011, 016, 019 unmet in full by the combined field) intact, so nothing in the second
  pass changes what Gate A is deciding. The remaining §11 `NOT_RUN` set is named there.

## 10. Build slices

- **S0** This document → Gate A expert panel → accepted requirements. Decision 0052 is the explicit
  owner acceptance line required by AGENTS.md invariant 8 and
  `docs/SPEC-DRIVEN-DEVELOPMENT.md`. The standalone capability remains registered as
  **experimental** in `docs/specs/README.md` until delivery evidence supports promotion; acceptance
  authorises the governed slices but does not advertise them as delivered.
- **S0b** **Accounting portion complete at `4e8e088`**: AT-01 now allows 256 workers, admits host
  `atm`, and carries optional per-worker ticket attribution; 20 focused tests passed. The WQO
  Beamfall adapter and recorder remain `NOT_RUN` with no owner today (§2), and must still be resolved
  because ATM-V0-001 reads every queue through them.
- **S1** State machine, runtime registry with the claude-code runtime, receipts, configuration, the
  plain-JSON queue adapter, and the Corvint `ROADMAP.md` queue adapter (ATM-V0-003..007, 005a, 005b,
  005c, 005d, 019, 019a, 020..022, 028, 023, 024, 024a, 025, plus the plain-JSON and Corvint
  `ROADMAP.md` adapters under ATM-V0-001). The `ROADMAP.md` adapter **moves here from S4 at
  round 6**: ATM-V0-001a/001b/001c's enumerated verb lists are Beamfall-specific text and the
  `ROADMAP.md` adapter carries no such list to build, so nothing ties it to S4's Beamfall work,
  and S7's precondition needs a real Corvint queue adapter before it can run `AT-nn` tickets.
  ATM-V0-007's worktree lane
  isolation moves here from S2 at Gate A round 4: S1's state machine, its ATM-V0-005c supervisors,
  and ATM-V0-028 `drain` (which removes lane worktrees, §8) all act on lane worktrees, so S1 cannot
  be built or exercised without it. The ATM-V0-022 surface S1 ships **excludes `wave`**, which needs
  ATM-V0-002/002a and lands with them in S4. S1's `accept` is **gate-incomplete** until S2 and S3
  land: the ATM-V0-013 verification lane ships in S2 and the ATM-V0-016..018 doc-consistency gates
  in S3, so an S1 acceptance is recorded as gate-incomplete rather than presented as a full
  accept. The adapter moves here from S4
  because without one queue source S1 has nothing to run. Configuration moves here from S6 at Gate A
  round 3 for the same reason: ATM-V0-019 replays the pinned document of ATM-V0-025, and ATM-V0-022
  ships `config` and `reconfigure`, so S1 cannot be built or exercised while 023-025 sit in the last
  slice. The plain-JSON adapter is the **one tool-resident reader** the tool ships, and ATM-V0-001's
  "the tool ships none" is about foreign queues: WQO's validation, identity binding, and adapter
  receipts exist because a foreign queue's format is not the tool's to define, whereas the
  plain-JSON queue file is the tool's own published format with its own schema, so there is no
  foreign contract for a parser to get wrong.
- **S2** codex runtime, reviewer independence, expert dispatch, routing table (ATM-V0-008..015,
  008a, 013a, 014a); ATM-V0-007 moved to S1.
- **S3** Doc-consistency lane and generated-doc drift (ATM-V0-016, 016a, 017, 017a, 018).
- **S4** The remaining queue adapter — Beamfall — and waves (the Beamfall portion of ATM-V0-001,
  plus 001a, 001b, 001c, 002, 002a).
- **S5** Routing evaluation and the comparison arm (§7).
- **S6** The policy-literal lint and config-as-task (ATM-V0-027 and §7 item 4); the layering,
  pinning, and non-overridable-key requirements themselves (023-025, 024a) land in S1.
  ATM-V0-026 is struck and is built in no slice.
- **S7 — dogfood demonstration and cut-over.** The first real-ticket demonstration requires S1,
  S2, **and S3**. It runs in **explicit-reviewer mode**: the operator names the reviewer and its
  (model, effort) on each invocation; `route` is not called. This demonstrates the runtime, verified
  review lane, and acceptance documentation gate without claiming automatic routing. Automatic
  reviewer selection is a later cut-over requiring S5's sealed passing digest (§7 item 2): `route`
  fails closed without one (ATM-V0-011). Once S1, S2, and S3 have landed, open `AT-nn` tickets may
  run through the tool in explicit-reviewer mode; only after S5 admits a table may those tickets use
  automatic routing. **Assertion:** zero demonstration or cut-over tickets exist whose receipt lacks a recorded tool version and
  receipt id — every demonstration or cut-over ticket records the tool version and receipt id that ran it so §7
  item 1 can be re-cut from tool-run lanes, and a ticket without both is a lane defect, not a
  silent gap. The further claim that "every friction found becomes a ticket in the tool's own
  queue" names no observer, no definition of friction, and no failure condition, and is carried
  here **labelled unevidenced** rather than as a checked property. Self-use receipts are product
  evidence per `docs/DOGFOOD.md` and never replace the §7
  comparison arm. This slice answers the fourth §1 quote.

Slices may exercise S1's loop as gate-incomplete development work, but no real-ticket demonstration
starts before S1, S2, and S3 land. After that boundary the tool dogfoods itself the way Corvint does.

## 11. Landscape survey (first pass recorded 2026-09-04)

Owner instruction 2026-09-04: "do your research to ensure its really is world class and nothing
does a better job." The survey is a dated review of agent orchestrators and AI review tools scored
against one requirement set, stated here and nowhere else in this document: **ATM-V0-004, 008, 011,
012, 016, 019, and 023**.

The `EXTERNAL_DEPENDENT` label is **struck at Gate A round 2**. It rested on a *proposed* amendment
still awaiting owner acceptance (`docs/SPEC-DRIVEN-DEVELOPMENT.md:116@58f0c4b6`;
`docs/SPEC-DRIVEN-DEVELOPMENT.md:118@109bd34b`) and
scoped to promotion and kill gates (`docs/SPEC-DRIVEN-DEVELOPMENT.md:120@e8bebbfe`), which §11 is not;
and the label's own condition — report `NOT_RUN` until the
dependency exists — is self-discharged here, because the survey has been done. §11 is a **first
pass** with named `NOT_RUN` items listed below. That it does not gate Gate A closure is a
coordinator call recorded in §9, not a property the survey carries. Results are appended here with
primary sources; a vendor claim that was not independently verified is recorded `INFERRED`.

### 11.1 First pass (2026-09-04, ad-hoc `competitive-market-analyst`, Claude)

Full scorecard and sources: `docs/plans/AGENT-TASK-MANAGER-LANDSCAPE-2026-09-04.md` (fourteen
tools, every page read 2026-09-04, source range 2026-01-25 to 2026-09-04). Findings:

- **Closest tool:** `microsoft/conductor` (MIT CLI; YAML multi-agent workflows over the Copilot and
  Anthropic agent SDKs). Documented per-agent `model`, `reasoning.effort`, `context_tier`, layered
  precedence, validated `output:` schemas with `when:` routing, `human_gate`, `max_iterations`.
  Meets ATM-V0-023 and parts of 004/011. No Git-pinned receipt, no evaluation gate on the routing
  table, no reviewer-independence assertion, no doc-vs-behaviour lane.
- **Better on one row each:** Claude Code Review already flags a `CLAUDE.md` statement a diff made
  outdated (ATM-V0-016 precedent) and posts a commit-pinned machine-readable severity blob;
  CodeRabbit's pre-merge checks (`off|warning|error`) carry blocking verdict teeth.
- **No tool meets the full requirement; partial coverage per the scorecard:** ATM-V0-008
  (independence receipt), 011 (evaluation-gated routing table), 016 as a requirement-to-test lane,
  and 019 (Git-pinned replayable receipt). "Unmet" overstated it: `conductor` scores partial on 008,
  011, and 019 in the scorecard, and the shortfall is that no surveyed tool covers a whole row, not
  that none covers any of it.
- **Adopt (status per item, assigned at Gate A round 3):** the `REVIEW.md` verification bar
  ("behavior claims need a `file:line` citation, not an inference from naming") — *carried by
  ATM-V0-012*; the re-review convergence rule — *carried by ATM-V0-014a*, which round 2 recorded as
  adopted into 014 without 014 containing it; bidirectional doc drift as a finding class in 016 —
  *carried by ATM-V0-016* (a document contradicting delivered behaviour is a finding in either
  direction); declared-not-discovered topology with a validated output schema — *carried by
  ATM-V0-024* (one fixed path per layer, no discovery walk) *and ATM-V0-023* (one published schema);
  an append-only event log with deterministic replay as the 019 substrate — *carried by ATM-V0-019a*
  for the append-only chain and *ATM-V0-019* for the replay set.
- **`INFERRED` / not scored:** Cursor Cloud Agent config precedence (primary page redirected), Kiro
  (docs 404), all benchmark and acquisition claims from aggregator pages. Codex config precedence
  was `INFERRED` in this pass and is confirmed `Y` in §11.2.
- **§11's named `NOT_RUN` set (carried forward at Gate A round 3, after §11.2).** Taken verbatim
  from the landscape file's own unread/unverified list
  (`docs/plans/AGENT-TASK-MANAGER-LANDSCAPE-2026-09-04.md:122`): arXiv:2606.13175, Devin, Factory
  Droid, Jules, Greptile, Ellipsis, CrewAI, and AG2/AutoGen; plus Codex's `model_reasoning_effort`
  enum, which remains `INFERRED`. Aider, Goose, Sweep, Augment, and Sourcery were unread in the
  first pass and are judged unlikely to move a row. Qodo and Faramesh were the two first-pass
  `NOT_RUN` items; both were read in §11.2 and both moved rows, so neither is on this list.
- **Verdict of the first pass:** nothing surveyed does the whole job. This is recorded, not a gate.

### 11.2 Second pass (2026-09-04, same persona, Claude)

Recorded in the landscape file's "pass 2" section. Qodo enters at 004 N / 008 P / 011 P / 012 P /
016 P / 019 N / 023 Y: its ticket-compliance check grades a delivered diff against a linked
ticket's requirements (Fully / Partially / Not compliant) and can gate auto-approval, the closest
published analogue to ATM-V0-016, ahead of Claude Code Review and Kiro. That 016 P cell rests on
the compliance levels and citation mandate, which are `INFERRED` — the primary config pages 404 —
so it is marked inferred in the scorecard rather than read as verified. Faramesh enters at 019 P /
023 P and N elsewhere: hash-chained, signed PERMIT/DEFER/DENY decision records with deterministic
replay, policy-versioned rather than Git-pinned, replaying rulings not work; reviewer and model
concerns are out of its scope. Codex's config precedence is now confirmed `Y` on 023 from the
primary page; Cursor moves 011 U→P and 012 U→N. Still `INFERRED`: Qodo's compliance levels and
citation mandate, Cursor's model resolution order. The first-pass verdict stands:
008, 011, 016, and 019 remain unmet in full by the combined field.

**Adopt list with a status per item (assigned at Gate A round 3):**

- Per-policy-kind composition (overwrite settings, concatenate standards) — *proposed, not carried*.
  ATM-V0-024's merge rule is schema-driven (lists replace unless marked additive), which already
  expresses both kinds; a second composition vocabulary would be a parallel mechanism.
- A graded requirement-vs-diff verdict with a scope-creep check — *proposed, not carried*.
  ATM-V0-016 is binary by design because a graded verdict has no defined acceptance threshold; the
  scope-creep half is a candidate for a later slice, not a v0 requirement.
- Faramesh's `prev`-linked hash-chained record as the ATM-V0-019 shape — *carried by ATM-V0-019a*,
  **chain only, without the signature**: v0 has no key management, and round 2 recorded this as
  adopted while 019 contained neither half.
- Codex's non-overridable credential/auth key set — *carried by ATM-V0-024a*, and placed there
  rather than in 023 because it is a **precedence** rule, not a policy-is-data rule: ATM-V0-024 puts
  repository config above user config, which is the direction the Codex primary forbids for
  credential keys, so the fix has to be an exception to the layer order.
