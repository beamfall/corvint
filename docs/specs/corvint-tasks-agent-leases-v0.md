# Corvint Tasks agent leases V0

Owner: Russell Lewis
Date: 2026-09-27 (accepted the same day)
Intent status: accepted (owner decision 2026-09-27)
Delivery status: partial (S1 CAL-V0-001..003, S2 CAL-V0-004..006, S3 CAL-V0-007 and 009..013, S4 CAL-V0-008 and 014, S5 CAL-V0-015..017 and 024, S6 CAL-V0-018 partial (audit carried; proportional cost and load condition NOT_MET), S7 CAL-V0-019..020, S8 CAL-V0-021..023 and 025 experimental with explicit pack opt-in; CAL-V0-026 MET (GOMAXPROCS=2 qualification); CAL-V0-027 implemented with scoped native release qualification; S9 CAL-V0-028..034 implemented with local native qualification; S10 CAL-V0-035..041 implemented with scoped local Codex qualification; CAL-V0-044 implemented with disposable fixture-profile qualification; CAL-V0-045..047 implemented with scoped disposable qualification; CAL-V0-048..051 implemented with focused local qualification and independent source review; S11 CAL-V0-052..058 implemented with local OpenCode qualification, plus Claude Code and Codex host qualification of launch, claim, handoff and summary; S12 CAL-V0-059..061 implemented with focused tests and a live-store measurement; S13 CAL-V0-062..063 implemented with focused tests, live Codex qualification NOT_RUN; issue 482 CAL-V0-044/046 experimental compatibility implemented, independently reviewed and integrated with scoped fixture qualification; native completion recorded); S15 CAL-V0-065 implemented with focused tests, a compiled native fixture and independent source review; S17 CAL-V0-067 experimental implementation with focused tests, independent review and scoped native/archive/crash fixture qualification; physical facts NOT_OBSERVED; S14 CAL-V0-064 experimental implementation with original scoped macOS checks, independent review and sealed binding; current-main integration, Linux qualification and native completion pending; S18 CAL-V0-068 experimental host-pressure launch throttle integrated on main by PR #513 (eeef6276) with focused tests and independent review; live Linux sampling and live multi-agent saturation NOT_RUN, native completion of V1-0694 pending; issue 499 escalation ladder (CAL-V0-052, 054, 055, 057, 058) implemented with focused tests, live model-host qualification and native completion pending; S19 CAL-V0-069 opt-in lease `--timing` phase breakdown and timed-out claim replay implemented with focused tests and independent review; live fleet timing and Linux NOT_RUN, native completion pending; S20 CAL-V0-070 one-pass Mutate and pinned journal reads implemented with focused equivalence tests and a before/after benchmark, writer checkpoint deferred (owner decision pending); S21 CAL-V0-071..072 and 087..088 multi-repository programs (policy declarations, sibling worktrees, composite candidate, review and gate binding, designated per-repository integration with exactly-once recovery, per-repository Core context) implemented with focused and fake-host tests, undesignated changed repositories fail closed, live Codex qualification NOT_RUN; S21 CAL-V0-089 policy-bounded checkpointed stage continuation for Codex and OpenCode implemented with fake-host tests, Claude Code and token-capped policies refused `UNSUPPORTED`, live host qualification NOT_RUN; V1-0751 CAL-V0-073 read-only CREATE payload template implemented with focused fixture tests; S22 CAL-V0-074..075 Claude Code supervised host with focused and fake-host tests, live Claude Code qualification NOT_RUN; S23 CAL-V0-076..077 OpenCode supervised host with focused and fake-host tests, live OpenCode qualification NOT_RUN; V1-0793 CAL-V0-079..081 read-only critical-path report implemented with focused fixture tests and independent review; V1-0788 CAL-V0-096 prior-generation stage and member, claim-result allocation and claim-event member implemented with focused tests; V1-0790 CAL-V0-082..085 recorded hand-off target and derived `nextStage` implemented with focused tests; S24 CAL-V0-097 resource-aware default selection implemented with focused tests, live dispatcher qualification NOT_RUN; V1-0784 CAL-V0-101 priority-yield admission implemented with focused and property tests, live qualification NOT_RUN; V1-0781 CAL-V0-095 experimental lock-free preparation-admission pressure in queue status with focused Darwin and Linux container tests; V1-0787 CAL-V0-099 stage-scoped execution prerequisites implemented with focused tests, live store qualification NOT_RUN; V1-0789 CAL-V0-098 review and integrate implement-author exclusion implemented with focused tests, live qualification NOT_RUN; V1-0780 CAL-V0-078 explicit result retryability implemented with focused wire and CLI tests; V1-0791 CAL-V0-102..103 opt-in no-progress loop detection hold implemented with focused tests and an N-1 digest, live dispatcher qualification NOT_RUN; V1-0772 CAL-V0-086 supervised stage worktree PathText bound, unproved-stop `SURVIVORS` and drain `EPERM` re-probe implemented with focused tests; V1-0851 CAL-V0-104 dispatcher worker-exit release retry and reap implemented with focused fake-queue tests, live dispatcher qualification NOT_RUN; V1-0853 CAL-V0-105 work-state-held tickets outside the dispatcher's selection window implemented with focused tests, live dispatcher qualification NOT_RUN; V1-0852 CAL-V0-107 caller-asserted author-exclusion cover implemented with focused tests, live qualification NOT_RUN; V1-0855 CAL-V0-108 cross-stage priority-admission order implemented with focused and property tests, live qualification NOT_RUN; V1-0854 CAL-V0-106 chunked batch ticket refine implemented with focused tests, live 110-ticket qualification NOT_RUN; V1-0862 CAL-V0-109..110 Darwin kernel memory-pressure signal and recorded level reason implemented with focused tests and a live Darwin sample, live dispatcher saturation NOT_RUN; V1-0863 CAL-V0-111..113 caller-bounded lease lock wait, same-request HANDOFF replay after LOCK_TIMEOUT and no plain-release conversion implemented with focused tests, live fleet qualification NOT_RUN; V1-0891 CAL-V0-125..126 proposed Linux CPU-utilisation signal and per-OS pressure signal selection implemented with focused tests, macOS CPU ticks NOT_DELIVERED (no cgo-free source), live Linux sampling NOT_RUN; V1-0890 CAL-V0-127..129 proposed dispatcher configuration reload, cap 0 and lane minAgeSeconds implemented with focused tests, live dispatcher qualification NOT_RUN
Authoritative inputs: owner request [issue 637](https://github.com/beamfall/corvint/issues/637) (V1-0863, CAL-V0-111..113); owner request [issue 622](https://github.com/beamfall/corvint/issues/622) (V1-0851, CAL-V0-104); owner request [issue 624](https://github.com/beamfall/corvint/issues/624) (V1-0853, CAL-V0-105); owner request [issue 623](https://github.com/beamfall/corvint/issues/623) (V1-0852, CAL-V0-107); owner request [issue 626](https://github.com/beamfall/corvint/issues/626) (V1-0855, CAL-V0-108); owner request [issue 583](https://github.com/beamfall/corvint/issues/583) (V1-0784, CAL-V0-101); owner request [issue 584](https://github.com/beamfall/corvint/issues/584) with owner answer 2026-10-05 (D1) (CAL-V0-097); owner request [issue 482](https://github.com/beamfall/corvint/issues/482) (proposed CAL-V0-044/046 compatibility amendment);
owner requests [issue 426](https://github.com/beamfall/corvint/issues/426),
[issue 427](https://github.com/beamfall/corvint/issues/427), [issue 428](https://github.com/beamfall/corvint/issues/428),
and [issue 430](https://github.com/beamfall/corvint/issues/430), explicitly commissioned 2026-10-01 (CAL-V0-048..051); owner request [issue 342](https://github.com/beamfall/corvint/issues/342),
owner approval on 2026-09-30 of prospective handoff accounting for [issue 412](https://github.com/beamfall/corvint/issues/412) (CAL-V0-044),
owner requests [issue 420](https://github.com/beamfall/corvint/issues/420),
[issue 421](https://github.com/beamfall/corvint/issues/421) and
[issue 422](https://github.com/beamfall/corvint/issues/422) (CAL-V0-045..047),
owner request [issue 431](https://github.com/beamfall/corvint/issues/431) (CAL-V0-052..058),
owner request [issue 499](https://github.com/beamfall/corvint/issues/499) (CAL-V0-052, 054, 055, 057 and 058 escalation ladder),
owner request [issue 497](https://github.com/beamfall/corvint/issues/497) (CAL-V0-068),
owner request [issue 494](https://github.com/beamfall/corvint/issues/494) (CAL-V0-069),
owner request [issue 588](https://github.com/beamfall/corvint/issues/588) with owner answer 2026-10-05 (D8) (CAL-V0-079..081),
owner comment of 2026-10-05 on [issue 494](https://github.com/beamfall/corvint/issues/494) (CAL-V0-095, ticket V1-0781),
ticket V1-0780, the owner's follow-up to [issue 494](https://github.com/beamfall/corvint/issues/494) (CAL-V0-078),
owner request [issue 494](https://github.com/beamfall/corvint/issues/494) (CAL-V0-069),
owner request [issue 446](https://github.com/beamfall/corvint/issues/446) (CAL-V0-059..061),
owner request [issue 354](https://github.com/beamfall/corvint/issues/354) (CAL-V0-062..063, 071..072; owner scope split 2026-10-04) and its split native tickets V1-0755 (CAL-V0-074..075) and V1-0756 (CAL-V0-076..077),
owner request [issue 585](https://github.com/beamfall/corvint/issues/585) and native ticket V1-0787 with owner decision D4 (CAL-V0-099),
owner request [issue 378](https://github.com/beamfall/corvint/issues/378),
owner request [issue 586](https://github.com/beamfall/corvint/issues/586) part 1 (CAL-V0-096),
owner request [issue 587](https://github.com/beamfall/corvint/issues/587) part 1 (CAL-V0-082..085),
owner request [issue 587](https://github.com/beamfall/corvint/issues/587) part 2 with owner decision D8 (CAL-V0-102..103, ticket V1-0791),
owner request [issue 586](https://github.com/beamfall/corvint/issues/586) part 2 (CAL-V0-098, ticket V1-0789),
owner request [issue 623](https://github.com/beamfall/corvint/issues/623) (CAL-V0-107, ticket V1-0852),
owner request [issue 626](https://github.com/beamfall/corvint/issues/626) (CAL-V0-108, ticket V1-0855),
owner request [issue 625](https://github.com/beamfall/corvint/issues/625) (CAL-V0-106, ticket V1-0854), owner request [issue 636](https://github.com/beamfall/corvint/issues/636) (CAL-V0-109..110, ticket V1-0862), owner request [issue 646](https://github.com/beamfall/corvint/issues/646) (proposed CAL-V0-125..126, ticket V1-0891), owner request [issue 645](https://github.com/beamfall/corvint/issues/645) (proposed CAL-V0-127..129, ticket V1-0890),
owner request [issue 370](https://github.com/beamfall/corvint/issues/370), and
owner choice on 2026-09-28 to quarantine environments until confirmed safe reuse; owner request [issue 336](https://github.com/beamfall/corvint/issues/336), the Corvint Tasks contract TCP-00 (`beamfall/corvint-tasks` `docs/SPEC.md`,
§3.4, §4, §6 and §7.4), decision 0397 (corvint-tasks built in tree), decision 0423 A10,
`docs/specs/corvint-tasks-store-init-v0.md`, tickets V1-0398, V1-0184 and V1-0310, and the in-tree
sources under `internal/tasks`.

## Agent digest
- Claim: Agents claim, gate and complete scoped Tasks attempts through external leases or an explicitly enabled Codex, Claude Code or OpenCode supervisor.
- Status: accepted (owner decision 2026-09-27); partial (S1 CAL-V0-001..003, S2 CAL-V0-004..006, S3 CAL-V0-007 and 009..013, S4 CAL-V0-008 and 014, S5 CAL-V0-015..017 and 024, S6 CAL-V0-018 partial (audit carried; proportional cost and load condition NOT_MET), S7 CAL-V0-019..020, S8 CAL-V0-021..023 and 025 experimental with explicit pack opt-in; CAL-V0-026 MET (GOMAXPROCS=2 qualification); CAL-V0-027 implemented with scoped native release qualification; S9 CAL-V0-028..034 implemented with local native qualification; S10 CAL-V0-035..041 implemented with scoped local Codex qualification; CAL-V0-044 implemented with disposable fixture-profile qualification; CAL-V0-045..047 implemented with scoped disposable qualification; CAL-V0-048..051 implemented with focused local qualification and independent source review; S11 CAL-V0-052..058 implemented with local OpenCode qualification, plus Claude Code and Codex host qualification of launch, claim, handoff and summary; S12 CAL-V0-059..061 implemented with focused tests and a live-store measurement; S13 CAL-V0-062..063 implemented with focused tests, live Codex qualification NOT_RUN; issue 482 CAL-V0-044/046 experimental compatibility implemented, independently reviewed and integrated with scoped fixture qualification; native completion recorded); S15 CAL-V0-065 implemented with focused tests, a compiled native fixture and independent source review; S17 CAL-V0-067 experimental implementation with focused tests, independent review and scoped native/archive/crash fixture qualification; physical facts NOT_OBSERVED; S14 CAL-V0-064 experimental implementation with original scoped macOS checks, independent review and sealed binding; current-main integration, Linux qualification and native completion pending; S18 CAL-V0-068 experimental host-pressure launch throttle integrated on main by PR #513 (eeef6276) with focused tests and independent review; live Linux sampling and live multi-agent saturation NOT_RUN, native completion of V1-0694 pending; issue 499 escalation ladder (CAL-V0-052, 054, 055, 057, 058) implemented with focused tests, live model-host qualification and native completion pending; S19 CAL-V0-069 opt-in lease `--timing` phase breakdown and timed-out claim replay implemented with focused tests and independent review; live fleet timing and Linux NOT_RUN, native completion pending; S20 CAL-V0-070 one-pass Mutate and pinned journal reads implemented with focused equivalence tests and a before/after benchmark, writer checkpoint deferred (owner decision pending); S21 CAL-V0-071..072 and 087..088 multi-repository programs (policy declarations, sibling worktrees, composite candidate, review and gate binding, designated per-repository integration with exactly-once recovery, per-repository Core context) implemented with focused and fake-host tests, undesignated changed repositories fail closed, live Codex qualification NOT_RUN; S21 CAL-V0-089 policy-bounded checkpointed stage continuation for Codex and OpenCode implemented with fake-host tests, Claude Code and token-capped policies refused `UNSUPPORTED`, live host qualification NOT_RUN; V1-0751 CAL-V0-073 read-only CREATE payload template implemented with focused fixture tests; S22 CAL-V0-074..075 Claude Code supervised host with focused and fake-host tests, live Claude Code qualification NOT_RUN; S23 CAL-V0-076..077 OpenCode supervised host with focused and fake-host tests, live OpenCode qualification NOT_RUN; V1-0793 CAL-V0-079..081 read-only critical-path report implemented with focused fixture tests and independent review; V1-0788 CAL-V0-096 prior-generation stage and member, claim-result allocation and claim-event member implemented with focused tests; V1-0790 CAL-V0-082..085 recorded hand-off target and derived `nextStage` implemented with focused tests; S24 CAL-V0-097 resource-aware default selection implemented with focused tests, live dispatcher qualification NOT_RUN; V1-0784 CAL-V0-101 priority-yield admission implemented with focused and property tests, live qualification NOT_RUN; V1-0781 CAL-V0-095 experimental lock-free preparation-admission pressure in queue status with focused Darwin and Linux container tests; V1-0787 CAL-V0-099 stage-scoped execution prerequisites implemented with focused tests, live store qualification NOT_RUN; V1-0789 CAL-V0-098 review and integrate implement-author exclusion implemented with focused tests, live qualification NOT_RUN; V1-0780 CAL-V0-078 explicit result retryability implemented with focused wire and CLI tests; V1-0791 CAL-V0-102..103 opt-in no-progress loop detection hold implemented with focused tests and an N-1 digest, live dispatcher qualification NOT_RUN; V1-0772 CAL-V0-086 supervised stage worktree PathText bound, unproved-stop `SURVIVORS` and drain `EPERM` re-probe implemented with focused tests; V1-0851 CAL-V0-104 dispatcher worker-exit release retry and reap implemented with focused fake-queue tests, live dispatcher qualification NOT_RUN; V1-0853 CAL-V0-105 work-state-held tickets outside the dispatcher's selection window implemented with focused tests, live dispatcher qualification NOT_RUN; V1-0852 CAL-V0-107 caller-asserted author-exclusion cover implemented with focused tests, live qualification NOT_RUN; V1-0855 CAL-V0-108 cross-stage priority-admission order implemented with focused and property tests, live qualification NOT_RUN; V1-0854 CAL-V0-106 chunked batch ticket refine implemented with focused tests, live 110-ticket qualification NOT_RUN; V1-0862 CAL-V0-109..110 Darwin kernel memory-pressure signal and recorded level reason implemented with focused tests and a live Darwin sample, live dispatcher saturation NOT_RUN; V1-0863 CAL-V0-111..113 caller-bounded lease lock wait, same-request HANDOFF replay after LOCK_TIMEOUT and no plain-release conversion implemented with focused tests, live fleet qualification NOT_RUN; V1-0891 CAL-V0-125..126 proposed Linux CPU-utilisation signal and per-OS pressure signal selection implemented with focused tests, macOS CPU ticks NOT_DELIVERED (no cgo-free source), live Linux sampling NOT_RUN; V1-0890 CAL-V0-127..129 proposed dispatcher configuration reload, cap 0 and lane minAgeSeconds implemented with focused tests, live dispatcher qualification NOT_RUN. Drafted and accepted 2026-09-27 on the owner's request to bring corvint-tasks to a level where it can take over Beamfall's `script/roadmap.sh`.
- Exists: the TCP-00 attempt, reservation and receipt shapes (reserved, no writer), the §5.2 writer for fixture and non-fixture queues, and the CTS-V0-003 shadow import.
- Blocked on: the recovered task-store contract (V1-0310) for the parts of TCP-00 this spec does not restate.
- Read next: V1-0780 retryable result amendment (which codes a caller may retry); #464 command-reader lifecycle amendment; Slices; Requirements (S8 for parallel claims; S9 for named pools; S10 for Codex supervision; S11 for the continuous dispatcher; S12 for read cost; S13 for supervised effort and stage wall; S14 explicit command progress; S15 explicit exclusions; S17 proposed operator-attested untouched release; S18 host-pressure launch throttle; S19 lease timing and timed-out claim recovery; S19 lease timing and timed-out claim recovery; S21 multi-repository programs; S22 Claude Code supervised host; S23 OpenCode supervised host; V1-0793 critical-path read; S24 resource-aware default selection; V1-0781 admission pressure amendment; V1-0787 stage-scoped execution prerequisites; V1-0789 implement-author exclusion; V1-0791 no-progress loop detection; V1-0772 supervised stage worktree bound and unproved stops; V1-0854 batch refine amendment); Amendments to TCP-00; Failure modes.

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
(`internal/tasks/transaction/model.go:615@afae0d34`), and until S1 the writer refused every queue
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
| S13 | CAL-V0-062..063 | Policy-bounded supervised Codex effort and stage wall; focused tests, live Codex qualification NOT_RUN |
| S14 | CAL-V0-064 | Proposed explicit command progress; original reviewed source and sealed evidence retained, current-main composition pending |
| S15 | CAL-V0-065 | Opt-in explicit per-claim member exclusions; focused tests and a compiled native fixture |
| S17 | CAL-V0-067 | Experimental operator-attested untouched release; scoped native/archive/crash fixtures passed, physical facts NOT_OBSERVED |
| S18 | CAL-V0-068 | Experimental host-pressure launch throttle: hysteresis level caps new non-exempt launches; running workers untouched |
| S19 | CAL-V0-069 | Issue 494 opt-in `--timing` phase breakdown for claim, renew, heartbeat and release; timed-out claim replay pinned by test |
| S20 | CAL-V0-070 | Writer cost against receipt history (V1-0645): one-pass `Mutate` and pinned journal reads implemented with equivalence tests and a before/after benchmark; writer checkpoint proposed, owner decision pending, deferred 2026-10-04 |
| S21 | CAL-V0-071..072, 087..089 | Multi-repository supervised programs: policy-pinned extra checkouts, sibling worktrees, composite candidate, review and gate binding, designated per-repository integration with exactly-once recovery, per-repository Core context; policy-bounded checkpointed stage continuation on hosts that report an interrupted session |
| S22 | CAL-V0-074..075 | Claude Code supervised host: policy-selected host, named pin refusals, Claude Code argv and result vocabulary; live Claude Code qualification NOT_RUN |
| S23 | CAL-V0-076..077 | OpenCode supervised host: detached-host escape drain, inline stage permissions, OpenCode argv, event-stream vocabulary with complete-accounting usage, and forked resume check; live OpenCode qualification NOT_RUN |
| S24 | CAL-V0-097 | Resource-aware default plan: pool-needing selections capped at free eligible members, the excess deferred outside the window; dispatcher pool routing (`match.pool`) and Core plan decoding; focused tests |
| V1-0789 | CAL-V0-098 | Opt-in review/integrate exclusion of recorded implement-generation authors; unverifiable history refuses; focused tests |
| V1-0852 | CAL-V0-107 | Explicit `--exclude-member` values cover generations with no recorded member, reported as caller-asserted; a ticket with no implement generation is not refused; focused tests |
| V1-0862 | CAL-V0-109..110 | Darwin pressure memory signal is the kernel memory-pressure level instead of sticky swap; the level records the signals that set it, reported in `dispatch status` and `throttled`; focused tests and a live Darwin sample |
| V1-0855 | CAL-V0-108 | Priority admission orders competitors across stages: at equal priority a ticket handed off to review or integrate ranks first, earlier handoff first; derived, no stored state; focused and property tests |
| V1-0863 | CAL-V0-111..113 | Caller-bounded `--lock-wait` (1..300 s) for release and attempt heartbeat; same-request HANDOFF replay after LOCK_TIMEOUT proven, no code change; no plain-release-to-HANDOFF conversion; focused tests |
| V1-0891 | CAL-V0-125..126 | Proposed: Linux CPU utilisation from `/proc/stat` tick deltas as an opt-in pressure signal, UNKNOWN on the first tick of a run; per-OS signal selection (for example memory only on macOS); macOS CPU ticks not delivered; focused tests |
| V1-0890 | CAL-V0-127..129 | Proposed: changed dispatcher configuration re-validated and applied at the next tick, refusals reported and the applied configuration kept; role `cap: 0` disables a role; lane `minAgeSeconds`; focused tests |
| V1-0791 | CAL-V0-102..103 | Opt-in derived `LOOP_DETECTED` hold over audited no-progress and alternating-return generations; owner reopen clears it; dispatcher raises one blocked event per episode; focused tests |
| V1-0851 | CAL-V0-104 | Dispatcher worker-exit recovery: bounded hand-off release retries with backoff, reap once the lease expires, `needs-owner` only when both fail; `heal.exitRecovery` default on; focused tests |
| V1-0853 | CAL-V0-105 | Dispatcher replans with the tickets its work state holds deferred `WORK_STATE_HELD` outside the selection window; UNKNOWN, NONE and no reader keep today's window; focused tests |
| V1-0772 | CAL-V0-086 | Supervised stage worktree path recorded as PathText (4096 bytes), refused before mutation when longer; an unproved stage drain ends the role `SURVIVORS` instead of `FINISHED`; drain re-probes `EPERM`; bounded watcher read tolerance; focused tests |

CAL-V0-062/063 are defined in S13 (issue 354). CAL-V0-064 (S14, issue 468) is reserved
by coordinated unlanded work; CAL-V0-066/S16 remains reserved for issue 464 if used.
The coordinator assigned CAL-V0-067/S17 to issue 479 and CAL-V0-068/S18 to issue 497. CAL-V0-069/S19 is issue 494 and CAL-V0-071..072/S21 extend issue 354 (CAL-V0-070 and S20 are left to coordinated unlanded work). CAL-V0-073 is the V1-0751 CREATE payload template amendment. CAL-V0-074..075/S22 deliver native ticket V1-0755, split from issue 354. CAL-V0-076..077/S23 deliver native ticket V1-0756, split from issue 354. CAL-V0-079..081 are the V1-0793 critical-path read. CAL-V0-095 is the V1-0781 preparation-admission pressure amendment. CAL-V0-096 is the V1-0788 prior-generation stage and member amendment. CAL-V0-097/S24 is issue 584. CAL-V0-098 is the V1-0789 implement-author exclusion. CAL-V0-107 is the V1-0852 caller-asserted author cover (issue 623). CAL-V0-108 is the V1-0855 cross-stage admission order (issue 626). CAL-V0-109..110 are the V1-0862 Darwin pressure signal and level reason (issue 636). CAL-V0-125..126 are the proposed V1-0891 CPU signal and per-OS signal selection (issue 646); CAL-V0-127..129 are the proposed V1-0890 configuration reload, cap 0 and lane minimum age (issue 645). CAL-V0-099 is the V1-0787 stage-scoped execution prerequisites amendment. CAL-V0-101 is the V1-0784 priority-yield amendment (issue 583). CAL-V0-078 is the V1-0780 retryable result amendment (issue 494 follow-up). CAL-V0-102..103 are the V1-0791 loop-detection amendment. CAL-V0-086 is the V1-0772 supervised stage worktree bound and unproved-stop amendment (issue 354). CAL-V0-087..089 complete S21 for issue 354 (V1-0475). This seed does not claim
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
  CAL-V0-097 amends this for pools: without `--pool` it claims the first `SELECTED` entry that
  requires no pool, and answers `RESOURCE_COLLISION` when every `SELECTED` entry requires one.
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
  CAL-V0-097 defines how the default plan (no `--pool`) treats a ticket's `requiresPool`.

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
  pack-fixture test import; its #464 addendum admits `internal/tasks/dispatch` importing
  `internal/groupreap`, and its #481 addendum admits only `internal/tasks/cli/attempt_run.go` and
  `attempt_run_test.go` importing it, both owner-approved on 2026-10-04. Standalone source-archive rebuild remains blocked by V1-0456.
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

Cooperating lease preparations use a bounded registered-order admission step before the existing `taskman.prepare.lock`. A successfully published registration cannot enter preparation while a smaller continuously live registration exists. Publication order is not CLI arrival order; pre-registration scheduling, mixed-version fairness and universal starvation freedom are not qualified. The default and maximum acquisition wait remain 30 seconds total from acquisition entry across identity resolution, registration, scans, queue wait and the final gate, except that an explicit CAL-V0-111 caller wait replaces that bound for its own command. It must never restart at phase boundaries. Full audits, journal/intent formats, writer authority, request identity, replay and the original shared 180-second qualification context remain unchanged.

The private coordination namespace consists of the inert `taskman.prepare.registry.lock` and 64 fixed `taskman.prepare.slot.00` through `.63` regular files under the pinned Git common directory, outside journal and intent. A slot has 16 bytes: `CPA1`, four zero reserved bytes, unsigned 64-bit big-endian nonzero rank. A live slot is owned by an exclusive nonblocking-flock open description. Registry-held publication chooses one greater than the maximum live rank, or 1 when none are live. Rank overflow and a scan observing 64 live slots refuse LIMIT_EXCEEDED without a receipt. This fixed technical bound includes the serving holder and is independent of lease/reservation capacity. A scan is not an instantaneous-capacity promise. No arbitrary namespace enumeration, durable counter, PID/time-based eviction, daemon, database service or new authority is introduced.

Only a successful lock probe proves an abandoned slot reusable. Held malformed/duplicate-rank state, unknown format, unsafe objects and observed name/root/inode drift refuse. Partial unowned bytes can be overwritten only through the acquired slot descriptor under the registry. The registry must not surround a sleep, final preparation gate, writer work, inventory/audit, monitor teardown or external execution. Slot/registry files are not normally removed or replaced. Scratch bytes are scheduling data, never proof/intent/ranking/attempt authority; read verbs do not create, clean or mutate them. Existing final-gate exclusion remains compatible with older callers, while their bypass of registration leaves mixed-version fairness unqualified.

Preparation owns all registry/slot/probe/gate/root descriptors it obtains. Every locally owned cleanup failure is retained with the primary error. Temporary-root cleanup completes before returning a successful composite handle; if it fails after gate acquisition, gate and slot are retired and the call fails. This does not broaden the public writer helper's cleanup behavior or claim coverage of hidden safeopen traversal ownership. One synchronized composite Close owns retirement: gate first, slot regardless of gate error, aggregate all failures, then deliver one observation. Hold measurement ends at the actual gate-release boundary; observation delivery waits for mandatory slot cleanup. An acquired-but-not-cleanly-released handle cannot be reported as fully released success. Concurrent/double Close returns the recorded result without duplicate cleanup or callback. Close/cancel never requires registry ownership; observers run outside internal locks and never under registry ownership.

Cancellation closes owned references; process-death recovery requires those references actually gone, demonstrated after joined exit. CLOEXEC is mandatory; no descriptor handoff to helpers is permitted. A still-held inherited or leaked reference remains live and is never evicted by age. Process exit may release descriptors at different instants, so transient conservative refusal before joined exit is allowed. Normal machine restart leaves no live owner locks, so stale scheduling bytes convey no authority and need no durable recovery claim. Boundary identity checking is not continuous hostile-filesystem monitoring.

Deterministic proof must acknowledge reached publication/entry/injection boundaries and cover registered non-overtaking, head/middle cancellation, death before/during partial publication and after publication/while registry/while serving, actual independent-open exclusion, exec closed-FD witness, capacity/overflow/malformed/identity refusals, total deadline, locally owned cleanup failures and synchronized composite Close. Canonical fixture initialization/audit precedes injection; journal/intent snapshots and authorized product effects are asserted separately from scheduling scratch. Actual Darwin and Linux execution evidence is required before the corresponding platform claim; cross-compilation is insufficient and an absent runner stays NOT_RUN.

After focused checks and independent source PASS, one unchanged original 5000-receipt/10-worker/30-operation mixed qualification must show 5000→5030 full consistent/agreeing audits, unique completed operations, ten stable identities/generations, all final CANCELLED/FENCED, zero active reservations, exact-ID replays with unchanged digest and after-replay audit 5030, plus bound source/binary and retired owned processes. Retain phase/rank/publication diagnostics on timeout without retries or changed deadlines. A failed wave is preserved and triggers diagnosis, not another automatic wave. The existing original CAL-V0-026 acceptance is not replaced by this test.

#### Issue 494: phase timing, timed-out claim recovery and latency observation (S19)

Issue 494 callers saw claim, heartbeat and release take 10 to 41 seconds under about ten concurrent writers, gave up, and later found an EXPIRED retry charged. This amendment makes the time visible and pins the recovery path. It changes no stored format, request preimage, digest, retry rule or deadline.

- `CAL-V0-069`: `claim`, `renew`, `attempt heartbeat` and `release` MUST accept an opt-in boolean `--timing` flag; every other lease verb MUST refuse it MALFORMED without writing. The flag MUST NOT enter the request, its preimage or digest, so the same request ID with or without it replays the same receipt-bound result. With the flag, the result MUST carry one closed `timing` object, profile `taskman-lease-timing/0`, in the first item: on OK and REFUSED results beside the existing members, and on ERROR results as the only member of a single item. Members are decimal Size strings: `totalMillis` (command entry to result), `transactions` (lease transactions entered, including a claim's reaps and refused attempts), `preparationRounds`, and the summed milliseconds of `admissionWaitMillis` (registered-order preparation admission), `guardMillis` (writer guards and orphan-stage recovery), `snapshotReadMillis` (head, inventory and full audit or verified cache reuse), `validationMillis` (request replay lookup, model and handoff-history audit), `monitorCloseMillis` (change-monitor teardown), `lockWaitMillis`, `lockHoldMillis`, `journalWriteMillis` (session, staged writes, links and pending-receipt redo, excluding sync) and `fsyncMillis` (file and directory sync calls of the writer session). Lock hold contains journal write and sync. A phase not reached reports `0`. Without the flag the output MUST be byte-for-byte the existing result. Timing is diagnostic only: never written to a receipt, request record, evidence, ranking, learning or authority, and never an input to a decision. Command help MUST list the flag for the four verbs.

Timed-out claim recovery, specified from existing behavior (no new semantics). A claim refused or ended by cancellation or deadline before its receipt is published writes nothing: no attempt exists and no retry is charged (CAL-V0-044/045 charge only a prior attempt). A caller whose claim outcome is unknown MUST recover by retrying the exact request ID: a committed claim replays its original attempt ID, generation and lease, with the head digest and journal bytes unchanged, including after that lease has expired; a claim whose receipt was published but whose head was not advanced is redone by the next writer and then replays. `TestGH494_TimedOutClaimReplaysExactly` covers the three cases, and that a claim cancelled after a charged attempt adds no charge. A fresh request ID after an unknown outcome may admit a second attempt and is the caller's error.

Unresolved owner decision (issue 494): retry charging for a committed but unused claim is unchanged. A claim that committed after its caller gave up holds a real attempt; when its lease expires, reaping records `LEASE_EXPIRED` and the next claim of that ticket is charged EXPIRED under the accepted CAL-V0-044, CAL-V0-045 and CAL-V0-067 rules ("Ordinary cancellation, failure and expiry remain charged regardless of stage"). Exempting such an attempt would change accepted semantics; the owner must decide whether to amend those requirements and what store-recorded evidence (for example no heartbeat, submit or evidence since admission) proves an attempt unused (see the 2026-10-04 build-log entry). This amendment does not implement it.

Latency observation, not a promise. On local Darwin (macOS 26.6.2, twelve CPUs, Go 1.27.1) with main `a5bb0d8f`, a disposable 7,140-receipt store and ten fresh-process writers each running claim, heartbeat, renew, heartbeat and release (one wave), 50 of 50 operations completed, median 28,165 ms and worst 32,959 ms; a fourteen-writer wave completed 55 of 70 with three LOCK_TIMEOUT refusals at about 30 s. Each preparation then cost about 2.9 s of complete-history inventory and audit, so a queued writer waits roughly 2.9 s times its queue position, bounded by the 30-second admission budget (issue 545 measurements). Latency grows with receipt history and writer count until V1-0645 makes writer cost history-independent. Linux, mixed-binary fleets, non-lease writers in the workload and more than one wave per binary are NOT_RUN. `--timing` is the instrument for collecting further observations; no p95 or latency budget is claimed.

Non-goals: `claim --async` and `claim status --request-id`; history-independent writer cost (V1-0645); timing for `reap`, `widen`, `submit`, `gate run`, `complete` or pool commands; persisting or aggregating timing; any change to deadlines, admission order or retry charging. Failure modes: a phase that fails or is cancelled is still counted up to its end; sums across rounds and transactions exceed any single round; an ERROR before the first transaction reports zeros except `totalMillis`; measurement uses the monotonic clock, so a wall-clock step does not skew it, but host load does. Acceptance: `TestGH494_LeaseTimingIsOptInDiagnostic` (`internal/tasks/cli`), `TestGH494_LeaseTimingSumsEveryTransaction` and `TestGH494_TimedOutClaimReplaysExactly` (`internal/tasks/store`). Rollback removes the flag, the result member and the collector; no store, request or receipt bytes depend on them.


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
  The separate explicit CAL-V0-067 operator-attested release profile (S17, experimental implementation) does not apply to ordinary release, expiry/reap or completion.
  The accepted PSR-V0-008 optional operator-owned safeReuse profile also permits delegated
  confirm-safe only for the exact owned successful original allocation/definition observation,
  durable reset/verify evidence, retired owned process tree and agreeing queue readback.
  An owned terminal observation may finalize under an audited ALL barrier; fresh preparation
  and competing control remain refused. Ordinary manual confirmation and quarantine rules
  otherwise remain unchanged; no physical exclusivity or authenticated operator is inferred.

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
  a clean repository outside `.taskman`, a 1..3600 second timeout (PSR-V0-011) and at most 64 KiB captured output.
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
  CAL-V0-065 applies the same explicit exclusion set to preview capacity. CAL-V0-097 adds the
  default plan's per-pool cap under the same rule.

The optional policy shape is `pools:[{id,members:[MEMBER],reservedFor:{MEMBER:STAGE},
memberConfig:{MEMBER:{configRef:{revision,path,blob},health:COMMAND,cleanup:COMMAND}},
priorityAdmission:BOOL}]` (`priorityAdmission` is CAL-V0-101's opt-in priority yield).
Each map is closed over declared member names; each nested addition is optional. A command is
`{argv:[ARG],cwd:"REPOSITORY"|PINNED,env:[NAME],timeoutSeconds:"N"}`, where PINNED is PSR-V0-012's
`{kind:"PINNED_REPOSITORY",path,revision}`. Git references return only identity,
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
`docs/build-log/2026-10-01-tasks-dispatch-gemini.md`. The escalation ladder amendment to CAL-V0-052,
054, 055, 057 and 058 comes from owner request
[issue 499](https://github.com/beamfall/corvint/issues/499); its delivery record is
`docs/build-log/2026-10-04-dispatch-escalation-ladder.md`.

- `CAL-V0-052`: `dispatch --program ID --config FILE [--once | --ticks N]` MUST decode a closed
  `taskman-dispatch/0` configuration of at most 256 KiB, read without following symlinks, and refuse
  unknown members, trailing data, unknown placeholders and out-of-range bounds. The bounds are:
  tickSeconds 1..3600, globalCap 1..64, killGraceSeconds 1..120, 1..8 hosts with absolute
  executables, 1..32 roles, cap 0..64 (0 disables the role, proposed CAL-V0-128; previously 1..64), priority 0..1000, idleSeconds 30..86400,
  wallSeconds 60..604800, cooldownSeconds 0..86400, parkAfter 1..100 and at most 256 pins. The
  optional `pressure` member has the bounds in CAL-V0-068.
  A role MAY name a base `model` (`[A-Za-z0-9][A-Za-z0-9._:/@+-]{0,127}`) only when its host's argv
  or env renders `{model}`, and a host whose argv, env or `activityPaths` renders `{model}` MAY serve
  only roles that name one, so an unsupported model configuration is refused rather than silently ignored. `escalate` (1..8 tiers
  of `after`, `model`, optional `cap` and `notes`) and `deescalateOnProgress` (default true) require a
  base model and a match role; `after` is 1..100 and strictly increasing, each tier's model differs
  from the tier below, a tier `cap` is 0 (role cap only) or 1..role cap, and notes are at most
  1024 bytes without control characters.
  Host env MUST NOT set `CORVINT_DISPATCH_*`. One exclusive non-blocking lock per program state
  directory MUST refuse a second dispatcher. `dispatch status` and `dispatch unpark` MUST NOT read,
  lock or write the native store. Status reports workers (with tier and model when the role names a
  model), parked and cooling keys, with a ladder the per-ticket no-progress streak and the tier and
  model each escalating role launches next, the dispatcher's
  liveness (RUNNING, NOT_RUNNING or UNKNOWN, read from the lock file's process identity without
  taking the lock) and the event tail. Unpark writes one atomic request file that the running
  dispatcher consumes. SIGINT, SIGTERM and SIGHUP stop the loop after the current tick without
  killing workers. `workRoot` MUST resolve to the same task store as the dispatcher's working
  directory, so workers claim in the store that heal and reap act on.
- `CAL-V0-053`: The optional per-ticket work state MUST come from either a `status-line` reader (one
  `key: value` line in an absolute per-ticket file of at most 64 KiB) or a `command` reader (one JSON
  object of ticket ID or local name to state, at most 1 MiB of output, 60 s timeout). Values are at most
  64 printable bytes. A missing file is `NONE`. Every read failure MUST yield `UNKNOWN` and an alert,
  never a guessed state. Roles that match states MUST refuse without a reader. See the #464 command-reader lifecycle amendment below.
- `CAL-V0-054`: The roster MUST be a pure function of the configuration, one observation, the
  running workers and the backoff skip set. Roles match tickets by labels, kinds, an ID glob, work
  states, excluded states, statuses and plan selection, or lane roles match quarantined members of
  one pool. A ticket with any live attempt is never a candidate; an expired lease becomes free only
  after a reap. Candidates order by pin, role priority, P-rank, plan order, key and role index. The
  global cap, then the per-role cap, bound the result, and each assignment takes the lowest free slot.
  One key holds at most one worker. An assignment of an escalating role carries the CAL-V0-057 tier
  computed from the ledger's escalation record; a tier with a `cap` admits a candidate only while
  fewer running and newly assigned workers of that role hold that tier, and a candidate at a full
  tier waits rather than launching at a lower tier.
  An optional pressure budget (CAL-V0-068) is applied after these fences, including the tier
  cap, and only holds candidates.
- `CAL-V0-055`: Each assignment MUST launch one independent process in its own session, with
  stdout and stderr appended to per-worker logs. The prompt and argv are rendered in a single pass,
  so a substituted value is never re-expanded, and `{prompt}` may appear at most once in argv. The
  worker receives `CORVINT_DISPATCH_PROGRAM`, `_ROLE`, `_SLOT`, `_TICKET` and `_WORKER`. Its holder
  name is the worker ID `<program>.<role>.<slot>.<nonce>-<seq>`. Names cannot contain `.`, and the
  nonce is random per dispatcher start, so IDs never collide across programs or roles, nor repeat
  after a crash or a deleted state directory. A host's `activityPaths` are rendered per worker.
  `{model}` renders the assignment's tier model (the base model at tier 0), and the worker records its
  tier and model. The
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
  handoff MUST emit `handoff-refused`. With `heal.exitRecovery` off it MUST also emit `needs-owner` and
  leave the attempt untouched; otherwise the bounded exit recovery of CAL-V0-104 follows. With `heal.reap`, an expired lease
  whose holder is not a running worker of this program MUST be reaped, whoever held it, since an
  expired lease is reapable by any operator. Heal request IDs are deterministic, so a
  repeated heal replays.
- `CAL-V0-057`: An ended worker made progress exactly when the key's durable fingerprint changed.
  The fingerprint covers ticket status, revision and work state, plus attempts with a candidate,
  gates, reviews or a durable phase; for lanes, the member state, holder and attempt. Progress
  clears backoff. An `UNKNOWN` work state is never progress and never unparks a key. No progress MUST
  start a cooldown, and after `parkAfter` consecutive runs MUST
  park the key and emit `needs-owner`. A parked key resumes when its fingerprint changes or on an
  operator unpark request. While any role has an `escalate` ladder, the ledger MUST keep, per
  observed ticket, the streak of consecutive finished sessions without progress (counted from when a
  ladder is configured) and the tier each escalating role last launched at. Progress resets the
  streak to 0 and returns every role with `deescalateOnProgress` to its base model; a role with
  `deescalateOnProgress: false` keeps its reached tier. Progress is a finished session with progress
  or, for a backed-off key with no worker, a known fingerprint change between sessions, including a
  newly admitted declared progress token (CAL-V0-064) that unparks it. An operator unpark is not
  progress and does not reset the streak. A launch uses the highest tier whose `after` is at most
  the streak, or the kept tier when higher. A tier whose `after` is at least `parkAfter` is reached
  only by a launch after an operator unpark, because a state change that unparks the key resets the
  streak. Records are
  dropped for tickets absent from the observation and, with no ladder configured, entirely.
  A `RETRY_EXHAUSTED` plan entry MUST NOT be readmitted by the dispatcher.
  It emits `needs-owner` naming `ticket reopen` (CAL-V0-043), because readmission is owner
  authority.
- `CAL-V0-058`: Every decision MUST append one `taskman-dispatch-event/0` line to `events.jsonl`
  and print it to stderr as plain language. The event kinds form a closed vocabulary: started,
  stopped, adopted, launched, launch-failed, finished, killing, killed, handoff, handoff-refused,
  reaped, state, claim, release, lane, cooldown, parked, unparked, alert, needs-owner, throttled (CAL-V0-068), escalated and config (proposed CAL-V0-127).
  `escalated` is emitted after the launch that raises a role's tier on a ticket and carries the from
  and to tier, both models, the streak and the tier notes; `launched` carries the tier and model of a
  model-naming role. A
  `finished` event carries the exit code (`NOT_OBSERVED` for an adopted worker), whether progress was
  made, and a bounded summary of the worker's last agent message: the final text of a recognized
  host event stream (OpenCode `run --format json`, Codex `exec --json`, Claude Code `-p
  --output-format stream-json --verbose`, or `json` without `--verbose`, Gemini CLI `-p -o
  stream-json`, whose consecutive assistant delta chunks form one reply, or `json`), ignoring
  subagent messages, otherwise the sanitized output tail. State changes
  compare against the previous observation; the first observation records only a baseline.

Non-goals: readmitting exhausted tickets; creating or cleaning worktrees; any network, account or
hosted service; hostile-process containment; enforcing a host's own permission deny-list; Windows
support (it compiles and refuses); treating a worker's own report as progress; judging relative
model strength (the ladder renders operator-chosen model strings); and the typed top-tier
NEEDS-OPERATOR worker request, which belongs to the escalation request of
[issue 502](https://github.com/beamfall/corvint/issues/502). Failure modes:
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
`TestCALV0057_FingerprintIgnoresNonDurableAttempts` and `TestCALV0058_SummaryReadsHostFinalText`;
for the issue 499 ladder, `TestCALV0052_EscalationConfigRefusesUnsupportedModels`,
`TestCALV0054_RosterTierCapsWaitWithoutDowngrade`, `TestCALV0057_EscalationLadderClimbsAndResetsOnProgress`,
`TestCALV0057_StickyTierAndOperatorUnparkKeepStreak`, `TestCALV0057_ProgressOutsideSessionResetsLadder`,
`TestCALV0057_NoLadderKeepsLegacyLedger` and `TestCALV0057_LedgerEscalationIsBounded`, with `TestCALV0057_DispatchStatusShowsTierAndStreak`
in `internal/tasks/cli`; and
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
Owner scope decision 2026-10-04: issue 354 closes on multi-repository programs and Codex
continuation across longer runs, using the effort delivered here; Claude Code and OpenCode
supervisor hosts move to separate native tickets V1-0755 and V1-0756 and are non-goals of this spec's issue 354 slices.
Multi-repository programs, gates and integration are delivered in S21 (CAL-V0-071..072, 087..088),
and checkpointed continuation of a stage past its wall in S21 (CAL-V0-089). The continuous dispatcher (S11) is unchanged:
its host argv already carries any effort flag and its role `wallSeconds` already reach seven days.

- `CAL-V0-062`: The optional policy `supervision` object MAY carry `efforts`, a closed object whose
  keys are a nonempty subset of `implement`, `review` and `integrate`, each a nonempty,
  canonical-byte-sorted, duplicate-free array of `low`, `medium` and `high`. A stage without an
  entry, or a policy without `efforts` or `supervision`, admits only `low`, so existing policy bytes
  keep their meaning. The supervisor config MAY carry `stageEfforts`, a map from those stages to an
  effort overriding `effort` for that stage. A new program MUST be refused, before its runtime read,
  program record, worktree, effect or host process, when its config names an unknown stage or any
  stage effort the policy does not admit, or when its base `effort` is not itself one of `low`,
  `medium` or `high`, even if every stage is overridden. An existing program MUST be re-checked
  against the current policy before every stage launch, without the base-effort rule, so a program
  recorded before that rule is not stranded while every stage stays overridden, so a later narrowing refuses further stages without blocking
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
to every stage); a lane cap shorter than `wallSeconds` silently shortens the stage, as before; a
program recorded with an empty base `effort` keeps running only while `stageEfforts` overrides every
stage, and a stage it does not override is refused as an effort the policy does not admit.
Rollback removes `efforts` and `stageWallMinutes` from the policy, which restores the low-only,
one-hour behaviour for every later dispatch; recorded program configs keep their digests because
`stageEfforts` is omitted when absent. Regression witnesses: `TestCALV0062_PolicyEffortAllowlist`,
`TestCALV0063_PolicyStageWallBound` (`internal/tasks/intent`); `TestCALV0062_StageEffortSelection`,
`TestCALV0062_CheckProgramConfigEffort`, `TestCALV0062_BaseEffortRequired`,
`TestCALV0062_StageRechecksCurrentPolicy`,
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

### S18 — Host-pressure launch throttle (issue 497)

Authoritative input: owner request [issue 497](https://github.com/beamfall/corvint/issues/497),
which asks the dispatcher to sample host load and swap each tick, cap new launches by pressure level
with hysteresis, never stop running workers, keep pinned and explicitly exempt work running, and
report the level, inputs and held work. The issue's free-form `exempt` list is refined here into two
explicit identity lists, shipped as `exemptRoles` and `exemptTickets`. No role or ticket is inferred to be critical path, review or integration
from its name. The coordinator assigned CAL-V0-068/S18 to this issue.

- `CAL-V0-068`: The `taskman-dispatch/0` configuration MAY carry one closed `pressure` object:
  `calmLoadPerCpu < loadPerCpuHigh < loadPerCpuCritical` (finite, non-negative),
  `calmSwap < swapHigh < swapCritical <= 1` (non-negative), `ticksToChange` 1..3600, `levelCaps`
  with exactly the levels `"1"` and `"2"` where `0 <= cap2 <= cap1 <= 64`, at most 32 unique
  `exemptRoles` that name configured roles, and at most 512 unique `exemptTickets` of 1..256 bytes
  without spaces, control characters or glob characters. When it is present, each tick that reaches
  launch admission MUST take one bounded host sample: on Linux `/proc/loadavg`, `/proc/stat` and
  `/proc/meminfo` read with byte limits; on macOS one `/usr/sbin/sysctl vm.loadavg kern.memorystatus_vm_pressure_level
  hw.logicalcpu` (CAL-V0-109) with a 2 s timeout and bounded output; on any other OS an UNKNOWN
  sample. The inputs are the one-minute load average per host-visible CPU and one memory signal:
  used/total swap on Linux, where zero total swap is an observed fraction of zero, and the kernel
  memory-pressure level on macOS (CAL-V0-109). The level (0, 1 or 2) is a pure step. Either metric at or
  above its critical threshold targets level 2, and at or above its high threshold targets at least
  level 1. Only both metrics at or below their calm thresholds target level 0; otherwise the target is
  the current level, so a throttle never steps down from 2 to 1 and releases only to 0. The level changes only
  after `ticksToChange` consecutive ticks target the same new level. A sample with either input
  UNKNOWN MUST keep the current level, cancel pending dwell, and never count as calm. Above level 0,
  the roster (CAL-V0-054) MUST admit at most `levelCaps[level]` non-exempt workers, counting the
  running non-exempt workers. The pressure budget is consulted only after the global cap, the
  key, skip, per-role and CAL-V0-057 tier-cap fences admit a candidate, so a held candidate
  consumes no slot and reserves no key. A refused candidate is reported as held only while the
  role, tier and global caps, charged with earlier launches and holds, would still have admitted it. Pins, `exemptTickets` (by ticket ID or local name) and `exemptRoles` are exempt
  and are never held. Pressure MUST NOT stop, signal or otherwise change a running worker, and MUST
  NOT bypass any static cap or native lease check. The level, pending dwell, newest bounded sample
  (valid UTF-8; source at most 256 bytes, at most 8 problems of at most 200 bytes) and at most 8192 held
  launches are kept in the private ledger. A restart keeps the recorded level, cancels pending
  dwell, clears the previous sample and is UNKNOWN until its first sample. If the budget cannot be
  built, the tick launches nothing and appends an `alert`. Removing `pressure` from the configuration drops the
  record. `dispatch status` MUST report the level, pending dwell, sample (`OBSERVED` or `UNKNOWN`),
  each input or `UNKNOWN`, problems, the active cap (`NONE` at level 0, otherwise from the
  configuration given to `dispatch status`, or `UNKNOWN` without one) and the held launches. One
  `throttled` event (CAL-V0-058) MUST be appended when the level, the sample's knowledge or the set
  of held keys changes, naming the level, the inputs and at most ten held tickets.

Non-goals: stopping, pausing or deprioritizing running workers; inferring critical path, review or
integration roles; memory compression, PSI or OS memory-pressure signals other than the CAL-V0-109
Darwin kernel level; container or cgroup
limits; per-role pressure caps; and any network or hosted metric. Failure modes: an unreadable,
oversized, malformed or timed-out sample is UNKNOWN and keeps the level, so a throttle is never
released on missing evidence and never raised without it; a saturated host with an all-exempt
roster keeps launching exempt work; load from processes outside the dispatcher still counts.
Rollback removes `pressure` from the configuration (the record is dropped at the next start) or
stops the dispatcher; running workers are unaffected either way. A binary without CAL-V0-068 refuses
a ledger that carries the record, so a downgrade first needs one start without `pressure`. Linux sampling is covered by
fixture parsing only; live Linux sampling and a live multi-agent dispatch under real host saturation are NOT_RUN, and other operating systems are always
UNKNOWN. Regression witnesses are the CAL-V0-068 and issue-497 tests in the traceability table.

V1-0862 amendment (owner request [issue 636](https://github.com/beamfall/corvint/issues/636)). On
macOS the throttle sat at level 2 all day while the host had spare capacity. `vm.swapusage` used
is sticky: macOS keeps pages in swap after pressure passes until their owner touches them, so the
swap fraction ratchets up and a static high value alone held the level. The issue also asked to see
which signal pins the level. The coordinator assigned CAL-V0-109..110 to this ticket. The optional
CPU-utilisation signal from the issue is not delivered, and the load average stays the load signal
on both operating systems.

- `CAL-V0-109`: On macOS the memory signal MUST be the kernel memory-pressure level
  `kern.memorystatus_vm_pressure_level`, read in the same sysctl call as the load and CPU count:
  `1` (normal) is calm, `2` (warn) is at least high and targets at least level 1, and `4`
  (critical) targets level 2. The swap thresholds do not apply to it. The macOS sample MUST NOT
  read or carry swap, so a high but static swap level no longer holds a level. A sample that
  carries a kernel level uses it as its memory signal even when swap is also present. A missing,
  duplicated or unexpected sysctl field, or any other level value, makes the memory signal
  UNKNOWN; it MUST never fall back to swap or count as calm or pressure, and the CAL-V0-068
  UNKNOWN rule applies. The sample records the level as `memoryPressureLevel` with
  `memoryPressureKnown`; the ledger refuses any other level or a level without the flag. Linux
  sampling and its swap signal are unchanged.
- `CAL-V0-110`: When the level changes, the pressure state MUST record as `reason` the sorted
  signals (`load`, `memory`, `swap`) that set it: for a raise, every signal at or above the new
  level's threshold (high for level 1, critical for level 2). A release to 0 clears it. Holding a
  level, an UNKNOWN sample and a restart keep it. The ledger refuses a reason at level 0, an
  unknown signal name, or an unsorted or duplicated list. `dispatch status` MUST report `reason`
  (`NONE` at level 0, the comma-joined signals, or `UNKNOWN` for a level recorded without one) and
  `memoryPressureLevel` (the observed kernel level or `UNKNOWN`). The `throttled` event MUST carry
  the same `reason` and `memoryPressureLevel` details and name the reason and the memory signal in
  its message. A reason never triggers an event of its own.

Non-goals: a CPU-utilisation signal (`host_processor_info`; the proposed V1-0891 amendment below adds a Linux-only CPU signal and per-OS selection, and macOS CPU ticks stay undelivered), discounting virtualization vCPU
threads from the load average, a swap-out rate signal, configurable kernel-level thresholds, and
reporting which signal currently prevents a release. Failure modes: a macOS release without the
sysctl makes every sample UNKNOWN, so the level never moves (reported in `problems`); an inflated
load average still raises and holds the level, and `reason: load` shows it. Rollback reverts the
change; a ledger whose pressure record carries `reason` or a kernel level is refused by an older
binary, so a downgrade first needs one start without `pressure`. Live dispatcher operation under
real macOS memory pressure is NOT_RUN.

### S20 — Writer cost independent of receipt history (V1-0645; one-pass `Mutate` delivered, writer checkpoint deferred)

Authoritative input: native ticket V1-0645, the writer follow-up named in S12's non-goals,
owner-prioritised on 2026-10-04 as the root cause behind issues 494 and 545. An uncontended mutation
costs about 2.9 s at 7,140 live receipts. Ten concurrent writers therefore sit at the 30-second
admission budget, and a 14-writer wave completed 55 of 70 operations with three LOCK_TIMEOUT
refusals.

Owner decision 2026-10-04: implement (A), one audit per mutation with its digests reused, and (B),
each parent directory opened once per scan, now and strictly inside the existing contract, so that
audit results and refusals are byte-identical to before. Defer (C), the writer checkpoint, until A
and B have measured before/after numbers. The coordinator confirmed CAL-V0-070/S20: CAL-V0-069/S19
belongs to issue 494 and CAL-V0-071..072/S21 to issue 354. CAL-V0-070 records A and B. The writer
checkpoint further below is a proposed design, owner decision pending, deferred 2026-10-04; it
governs nothing. The last sentence of CAL-V0-061 still applies unchanged: every mutation and
request lookup keeps the complete audit, which `store.Mutate` now runs once instead of twice.

Delivered: A for `store.Mutate`, B for journal audit reads, their equivalence tests and the
before/after measurement below. Not delivered: the writer checkpoint; A for release, policy,
barrier, reconciliation, import and pool sweep, which keep their separate passes; B for the
inventory's fresh reads, which still resolve every path from `/`. Writer cost stays proportional to
receipt history.

Measured cost before A and B. The opt-in `TestCALV0070_WriterHistoryProfile`
(`internal/tasks/store`) measures each writer primitive separately on a settled synthetic store. The store has 50 tickets, and every
receipt posts one request plus one 4 KiB-padded ticket afterimage (about 6 KB per receipt). It has
no evidence, attempts or reservations. Conditions: Apple M2 Max, 12 CPUs, Go 1.27.1, darwin/arm64,
medians of 3, host load average 25–33 from concurrent agents.

| Receipts | Request lookup audit | Inventory | Complete audit | `Mutate` total | Lease observed audit | Guarded inventory | S12 checkpoint + tail |
|---|---|---|---|---|---|---|---|
| 500 | 412 ms | 320 ms | 513 ms | 1,471 ms | 525 ms | 8.5 ms | 14 ms |
| 2,000 | 1,534 ms | 1,415 ms | 1,434 ms | 5,033 ms | 1,364 ms | 22 ms | 12 ms |
| 7,000 | 4,558 ms | 4,129 ms | 3,939 ms | 16,561 ms | 4,716 ms | 56 ms | 13 ms |

No single step dominates. `store.Mutate` takes three passes over history under the writer lock:
- the request-lookup audit;
- an inventory that opens, reads and hashes every state file;
- a second complete audit for the projected records.

Each pass costs about a third, roughly 0.5–0.6 ms per receipt. Lease commands (claim, renew,
heartbeat, release), which issues 494 and 545 exercise, already make one observed pass and reuse
its digests. They still pay that pass, plus a change guard that watches every retained file (570 ms
at 7,000 receipts). The live figure of 2.9 s per operation is consistent with that single pass on a
quieter host, which is an inference.

CPU profiles of three mutations put most of the time in these passes:

| Receipts | Journal audits | Inventory | Capacity model |
|---|---|---|---|
| 2,000 | 64% | 29% | — |
| 7,000 | 59% | 37% | under 3% |

Most of that time is per-file `safeopen` work (41% cumulative in `InRoot` at 2,000 receipts, with
flat time mostly in syscalls): a journal read made three opens and three closes per file, and the
inventory resolves every path from `/`. SHA-256 is not visible
in the profile. Each mutation allocates about 1.65 GB, in 9.0 million allocations (receipt and
record decode/encode). The wall-time remainder of about 0.66 ms per receipt appears as no separate
CPU step; the likely cause is contention and GC under host load, which is an inference.

Two bounded paths already exist:
- the S12 checkpoint-plus-tail read is flat at 12–14 ms;
- the lease path's guarded inventory, which reuses the digests observed by its one audit, costs
  22 ms against 1,415 ms for a fresh inventory.

The maintained `BenchmarkCALV0070_MutateAt2000Receipts` records the before number. See
`docs/build-log/2026-10-04-tasks-writer-history-cost.md`.

- `CAL-V0-070`: `store.Mutate` MUST take its request lookup and its canonical intent records from one
  complete audit, the records only once a fresh inventory shows every file that audit read
  unchanged, and a journal audit attempt MUST open each parent directory it reads from at most
  once. Every outcome, audit result and replay MUST be byte-identical, and every refusal MUST carry
  the same code, as the separate passes these replace give under some interleaving of the same
  outside edits.
  1. One pass. `Reader.AuditForMutation` runs `RequestIndex.Lookup`'s audit once. That audit also
     retains the queue, the policy and every observed ticket and release, which is exactly the
     selection `Mutate` derives from its inventory, and the physical digests it read. Its error is
     exactly Lookup's. The two refusals that only the later `Audit` of that selection raised, the
     selection-budget overflow and intent divergence, are deferred. A found request therefore
     replays as before, and an inventory refusal still comes first. `Canonical` then returns the
     first deferred refusal, or the Result that `Audit` returned. It answers only for exactly the
     retained selection, under `Audit`'s own path and count checks; any other selection runs a
     separate `Audit` as before.
  2. Content check. The inventory is read fresh, as before. The physical digests are published
     only when the audit completed with no deferred refusal. Its records stand in for the second
     `Audit` only if every file it read is listed in that inventory with the same digest and size.
     (Until the second review fix the inventory took its digests from the audit, as lease
     preparation does; that left a write through a shared mapping unseen, see the race limit. The
     lease path's exposure is V1-0775.)
  3. Pinned parents. A journal audit attempt opens each parent directory it reads from once
     (`safeopen.PinDir`), by the same no-follow, identity-checked step that began every per-file
     open. It then opens each file beneath it with one no-follow `openat` (`safeopen.InDir`)
     instead of three opens and three closes. Errors keep their text, and the descriptors close
     with the attempt.
  4. Change watch. Before the merged audit reads anything, `Mutate` registers
     `authority.WatchChanges` over the state directory and the intent tree, the watch lease
     preparation already uses. Item 1's records stand in for the second `Audit` only if the
     inventory succeeds, the content check passes, `Canonical` answers and, once all are taken,
     the watch's `Check` reports no change. Otherwise, and whenever the watch cannot be registered,
     `Mutate` closes the watch, takes the inventory again and runs the second `Audit` as before.
     Every refusal therefore comes from the passes that raised it before. The digests vouch for
     content and the watch for names, types and modes: a write through a shared writable mapping
     changes what a read returns without any inotify event, and without any kqueue event before
     `msync`. The watch's descriptors share the process limit with the audit's reads, so a refusal
     of the merged audit itself is taken again with the watch closed. On success the watch closes
     after the writer lock is released.

Race limit. Under the writer lock only an outside actor can change the store. An outside edit made
after the merged audit and before the watch's check, which the watch or the content check reports,
makes the inventory and the second `Audit` run again, so it is refused as the separate passes
refused an edit made after their lookup. The watch reports an entry created, removed, renamed or
re-moded, and an ordinary write. The content check reports changed content that the merged audit
read before the inventory read it. An overwritten request projection, an edited
`reservations.json` or a stray `barrier.json` is refused `JOURNAL_FORKED`, a projection replaced by
a symlink `UNSUPPORTED_FILESYSTEM`, and an edited ticket `INTENT_DIVERGED`.

Descriptor budget (macOS). kqueue needs one open descriptor per watched path, and the soft
descriptor limit is often 10240, below a populated store's file count. Every directory and
ancestor is always registered with kqueue. Regular files take a kqueue descriptor only while the
regular-file descriptors held by all live watches in the process stay within half the soft limit
read when the watch starts; the other half is left for directories, the audit's reads and
concurrent work, and Close returns the watch's share. A file beyond the budget is recorded by its
device, inode, mode, size, modification time and change time, read before and after registration
and required to agree, and every `Check` re-reads them: any difference, or a failed read, reports a
change, and the watch keeps reporting it. The directory watch still reports any entry created,
removed or renamed. Regular files
therefore no longer exhaust descriptors as a store grows. Directories are not budgeted, so a
directory-heavy tree or many concurrent watches can still be refused with too many open files, as
can any other registration failure. Two limits apply beyond the budget:
- A write that leaves size, modification time and change time unchanged, possible only through a
  timestamp collision, is missed by the watch. Only a path that rereads the file's bytes can catch
  it, and an inventory built from audit observations reuses cached digests (the exposure recorded
  above as V1-0775).
- Lease commits keep the re-read out of the writer lock (V1-0845). Just before taking the lock they
  `Sweep` the over-budget stat tuples, and under the lock they call `CheckEvents`, which only polls
  kqueue and the result kept from earlier reads, so guard polling under the lock does not grow with
  the store. This does not by itself meet CAL-V0-026's locked-work bound: `retainCheckpoint`, still
  called under the lock, traverses, sorts and encodes the whole canonical map, a remaining
  store-size cost tracked separately. A store writer changes files only by creating, linking,
  renaming or removing entries (`authority.Session`), which a watched directory reports whenever it
  happens, so no store write between the sweep and the locked check is lost; no Tasks writer edits a
  watched file in place. The accepted bound is an in-place write or mode change to an over-budget
  file by an actor that does not take the writer lock (an external editor or tool), made after the
  sweep read that file: the locked check does not see it. The lease then commits on the canonical
  journal content, because `commitLease` rebinds only `head.json` and not the intent tree. The next
  audit refuses a content edit (`INTENT_DIVERGED` for an intent projection such as `policy.json` or
  a ticket); a mode change that leaves the file readable and its content unchanged can pass that
  audit, because projection validation compares content digests and compares mode only between
  observations within one audit. Such an actor is not ordered against the lock, so the stat re-read under the lock only
  moved where that window began, by the lock wait. Other `Check` callers still re-read every tuple.

Linux inotify holds no descriptor per watched path and needs no budget.

A write through a shared writable mapping is the case only the content check sees. Probes of
`authority.WatchChanges` on this host's APFS (macOS, kqueue) and in a Linux arm64 container on
tmpfs (inotify) found that reads returned the new bytes at once on both. On Linux the watch
reported nothing, with or without `msync` (`MS_ASYNC` or `MS_SYNC`) or `munmap`. On macOS it
reported the write only after `msync`, not without it and not on `munmap`. Such a write made after
the merged audit read a file and before the inventory read it is refused as above. One made after
the inventory read the file is seen by neither check. Every read `Mutate` made of that file then
returned the content before the write, so the outcome is what the separate passes give when the
same write follows their second pass, and it is handled as such an edit is (below). The content
check does not depend on which writes a filesystem reports. `Mutate` qualifies the Git common dir
against the §5.1 allowlist first; ext4, xfs, btrfs and hfs were not probed, which bears only on the
watch's own reports.

An edit after the check meets what an edit after the old second pass met. The pre-apply binding
refuses a changed `head.json` or intent tree `SNAPSHOT_MOVED`. `barrier.json` and
`reservations.json` are re-read for the model, which validates them. An edit to any other journal
file is seen by the next audit. Only the refusal code is claimed: with two or more diverged files,
the path a refusal names follows Go map order, before this change as after it.

The watch is not free: on macOS it holds one descriptor per watched path within the descriptor
budget above, and registering and closing it costs 5–11% of an after-change `Mutate`'s CPU time,
growing with history (see below; measured before the budget).
Where it cannot be registered (a platform other than macOS or Linux, the entry bound, a path that
is neither a regular file nor a directory, or an exhausted watch limit), `Mutate` runs the separate
passes at their old cost.

Measured cost after A and B. The profile and the benchmark now also record the process's CPU time
(user plus system, all threads): over the same evening, wall time for identical runs varied up to
fourfold with concurrent agents' load. Each pair runs this change against its base `d4a896bd`,
with identical measurement code, on the fixture and host above.

Profile, medians of 3, after then base back to back, one-minute host load average 11–12:

| Receipts | `Mutate` total, base → after | `Mutate` CPU, base → after | Lookup + complete audit (base) | Merged audit (after) | Inventory, fresh → observed | Lease observed audit, base → after | S12 checkpoint + tail |
|---|---|---|---|---|---|---|---|
| 500 | 1,132 → 565 ms | 1,287 → 648 ms | 699 ms | 321 ms | 204 → 8 ms | 362 → 324 ms | 10 → 7 ms |
| 2,000 | 4,111 → 1,298 ms | 4,298 → 1,678 ms | 2,475 ms | 939 ms | 892 → 17 ms | 1,334 → 987 ms | 12 → 7 ms |
| 7,000 | 11,395 → 3,891 ms | 13,883 → 5,209 ms | 6,908 ms | 3,077 ms | 2,990 → 49 ms | 3,385 → 3,239 ms | 10 → 8 ms |

`BenchmarkCALV0070_MutateAt2000Receipts` with `-benchtime 5x -benchmem`, three alternating rounds,
one-minute host load average falling from 34 to 13:

| Tree | Wall per `Mutate` | CPU per `Mutate` | Allocated | Allocations |
|---|---|---|---|---|
| Base | 3.33–3.75 s, median 3.41 s | 4,065–4,421 ms, median 4,178 ms | 1.655 GB | 9.006 million |
| After A and B | 1.34–1.61 s, median 1.46 s | 1,688–1,945 ms, median 1,793 ms | 0.861 GB | 5.218 million |

A and B cut `Mutate`'s wall and CPU time by about 57–68% at 2,000 and 7,000 receipts, and its
allocation by 48%. A `Mutate` now costs about one complete audit plus a fixed remainder. That is
still proportional to history: about 0.5 ms per receipt in wall time at this load, against about
1.6 ms before. Lease commands, which issues 494 and 545 exercise, already made one pass. B alone
lowers their observed audit's CPU time by 6–9%. Whether (C) is still needed, chiefly for the lease
path, remains the pending owner decision. Runs made earlier the same evening at load 41–124 had
identical allocation counts but wall times that overlapped between trees. The build-log keeps them
as evidence of load sensitivity, not as the comparison.

Measured cost with the change watch (item 4). The pair was run again after the review fix, against
the same base, at one-minute host load average 27–59 from concurrent agents. Wall times are not
comparable at that load, and CPU times are inflated on both sides. Profile medians of 3, CPU time,
base → after:

| Receipts | `Mutate` | Watch setup and close (after) |
|---|---|---|
| 500 | 1,888 → 960 ms | 73 ms |
| 2,000 | 5,836 → 2,503 ms | 231 ms |
| 7,000 | 16,610 → 6,769 ms | 714 ms |

The benchmark, three alternating rounds, gave a median of 5,381 → 2,299 ms CPU per `Mutate`
(ranges 5,323–5,688 and 2,059–2,325 ms). Allocation fell from 1.655 to 0.868 GB, and from 9.006 to
5.277 million allocations. At 2,000 receipts the cut in CPU time stays at 57%. The watch adds
0.007 GB and 0.059 million allocations to each `Mutate`. In the low-load profile above, the
watch's setup and close took 34, 112 and 509 ms of CPU time: 5%, 7% and 10% of the after-change
`Mutate`.

Measured cost with the content check (second review fix). The two blocks above measured the reuse
of inventory digests that this fix withdrew. The pair was run again against the same base. On
macOS the one-minute host load average was 34–52, falling to 15 during the last base profile.
Profile medians of 3, CPU time, base → after:

| Receipts | `Mutate` | Merged audit (after) | Fresh inventory and content check (after) |
|---|---|---|---|
| 500 | 1,796 → 1,337 ms | 537 ms | 466 ms |
| 2,000 | 4,895 → 3,751 ms | 1,562 ms | 1,488 ms |
| 7,000 | 14,032 → 10,733 ms | 4,528 ms | 4,624 ms |

The benchmark, three alternating rounds, gave a median of 5,494 → 3,151 ms CPU per `Mutate`, a 42.6%
cut (ranges 4,081–5,644 and 2,857–3,725 ms; paired cuts 30%, 44% and 32%). Allocation fell from
1.655 to 1.035 GB, and from 9.006 to 5.618 million allocations. The fresh inventory adds 0.167 GB
and 0.341 million allocations to the digest-reusing version.

A `Mutate` now saves one complete audit, not an audit and an inventory: a 42.6% median cut at this
load, against 57% with the digests reused. The profile's cut, 23–26%, is likely understated, because
the base profile ran as load fell.

On Linux, the benchmark ran in an arm64 container (`golang:1.27.1`, 6 virtual CPUs, tmpfs) on the
same host, at host load 12–25, three alternating rounds. It gave a median of 4,019 → 2,547 ms CPU
per `Mutate`, a 36.6% cut (ranges 3,855–4,938 and 2,159–3,105 ms; paired cuts 44%, 37% and 37%), and
2.80 → 1.78 s wall. Allocation fell from 1.621 to 1.000 GB, and from 9.070 to 5.679 million
allocations. The Linux profile and the earlier designs on Linux were not measured.

Non-goals:
- any change to the receipt, journal, intent, request, evidence or archive format;
- writer-retained state that a read mutates;
- relaxing CAL-V0-061 for reads;
- lock admission and fairness (issue 494);
- evidence deduplication;
- removing the per-read intent-tree passes (V1-0646).

Failure modes of A and B:

| Failure | Handling |
|---|---|
| Request found while a deferred refusal is held | Replays as before; the deferred refusal is never raised, because the old flow never audited the selection |
| Inventory refusal while a deferred refusal is held | No digests are published, so nothing is reused; the inventory refuses first, as before |
| The inventory names a selection the audit did not retain | `Canonical` declines and a separate `Audit` runs, as before |
| A file the merged audit read is unlisted, or listed with another digest or size, in the inventory | No reuse: the watch is closed and the inventory and the second `Audit` run again, as before |
| A write through a shared mapping after the merged audit read the file, before the inventory read it | The watch may not report it; the content check does, and the passes run again (`TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit`) |
| A write through a shared mapping after the inventory read the file | Neither check sees it; every read already returned the earlier content, so it is handled as an edit after the old second pass |
| A parent directory is renamed or replaced during an attempt | Reads continue beneath the parent the attempt already retained, as every per-file open did; the attempt's identity checks refuse as before |
| A platform without the Unix open path | `PinDir` and `InDir` return the same unsupported error as `InRoot` |
| An edit detected by the watch or the content check, made after the merged audit and before the watch's check | The inventory and the second `Audit` run fresh and refuse as before. A write through a shared mapping after the inventory read the file is detected by neither; the shared-mapping rows above give that residual window |
| The merged audit refuses while the watch is held | The watch is closed and the audit runs again, as `Lookup` ran, so descriptors the watch holds cannot cause a refusal the separate passes did not raise (`TestCALV0070_MutateRetriesAuditWithoutWatch`) |
| The inventory or `Canonical` refuses | The watch is closed and the fresh inventory and second `Audit` raise the refusal, as before |
| An outside edit after the watch's check | Handled as an edit after the old second pass (see the race limit) |
| The watch cannot be registered, or reports a change made by the merged audit's own reads | No reuse: the separate passes run as before, at their old cost. `TestCALV0070_MutateRefusesChangesAfterMergedAudit` fails if an unchanged store is observed twice |
| A defect in the merged audit | The equivalence tests below compare every combination; rollback reverts the change |

Acceptance evidence for A and B (delivered):
1. `TestCALV0070_MergedMutationAuditEquivalence` (`internal/tasks/journal`), on 24 store states:
   clean, with the request absent and found; a rebound, rewritten or missing receipt; a missing,
   corrupt, stray, symlinked or FIFO request file; a request file removed or symlinked between
   capture and read; an edited, missing, stray or symlinked ticket; an edited queue; the selection
   budget exceeded, alone, with a found request, before a corrupt request and before an edited
   ticket; and a pending receipt. For each it compares four flows: separate `Lookup` and `Audit`
   with per-file opens, the same with pinned opens, and the merged audit with each. The error,
   found entry, ticket, identity and Result must all be equal. Published digests must equal a fresh
   read, and a subset, superset or unchecked selection, or a found request, is never answered.
2. `TestCALV0070_PinnedDirOpensMatchInRoot` (`internal/tasks/safeopen`): for a regular file, an
   absent name, file and dangling symlinks, a FIFO, a directory, dot, dot-dot, empty, absolute and
   unclean names, `InDir` opens the same file or returns the same error text as `InRoot`, after the
   directory's path has been renamed and replaced. It never descends.
3. `TestCALV0070_MutateAuditSequenceEquivalence` (`internal/tasks/store`): on a store built by
   `Mutate` with 70 receipts, `Mutate`'s old and new sequences give the same refusal in the same
   phase, the same inventory and the same canonical Result. The states are: clean with the request
   absent and found, an edited ticket with the request absent and found, a corrupt request file,
   an edited `reservations.json`, a stray state directory, and a stray state directory with an
   edited ticket. The new sequence runs `Mutate`'s own `observeMutation` under a change watch. It
   fails if an unchanged store that it accepts is observed twice, or if a refusal does not come
   from the fresh passes. Where the store is accepted, every file the merged audit read passes the
   content check against a fresh inventory.
4. `TestCALV0070_MutateRefusesChangesAfterMergedAudit` (`internal/tasks/store`) changes the store
   through the real `Mutate`, at a fixed point after the merged audit or after the inventory and
   `Canonical`, before the watch's check. The changes are an overwritten request projection (at
   both points), a request projection replaced by a symlink, an edited ticket (at both points), an
   edited `reservations.json` and a stray `barrier.json`. Each must be refused with the code the
   separate passes raise when the same change follows their lookup, after a fresh inventory and
   audit. Head, receipts and staging must be unchanged, and the request must not be projected. An
   overwritten projection and an edited ticket made before `Mutate` starts must be refused with the
   separate passes' code, the ticket by the fresh passes. An unchanged store must commit without a
   fresh pass. With the check's result ignored, the overwritten and symlinked projections
   committed, the edited tickets were refused `SNAPSHOT_MOVED`, and the `reservations.json` and
   `barrier.json` edits failed validation. Returning a refusal from the reuse instead of the fresh
   passes failed the stage checks.
5. `internal/tasks/store/mutation_audit_unix_test.go`, run on macOS and in a Linux arm64 container
   (`golang:1.27.1`, tmpfs). `TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit` maps a request
   projection and a ticket shared and writable before `Mutate` starts, first showing that the
   watch does not report a write through the mapping that reads do see. It then changes one byte
   through the mapping, without `msync`, after the merged audit. `Mutate` must refuse with the
   separate passes' code (`JOURNAL_FORKED`, `INTENT_DIVERGED`) through the fresh passes, publish
   nothing and project nothing. With the content check forced to pass, the projection case
   committed on both platforms. `TestCALV0070_MutateRetriesAuditWithoutWatch` runs `Mutate` in a
   child process whose descriptor limit leaves the merged audit, once the watch is registered, its
   clean need (137 at 70 receipts, found by search) less the watch's descriptors, or none if the
   watch holds more: none on macOS, where it holds 276, and 136 on Linux, where it holds 1. Closing
   the watch alone gives the audit enough. The audit must fail for want of descriptors, the retry
   must find exactly the watch's descriptors released, and `Mutate` must complete through the fresh
   passes. On a corrupt request projection, whose need varies with read order, the audit gets no
   free descriptor while the watch is held and the retry must refuse with the separate passes'
   `JOURNAL_FORKED`. No descriptor may stay open. With the retry removed, both cases failed on both
   platforms.
6. The existing `internal/tasks/journal`, `internal/tasks/safeopen` and `internal/tasks/store`
   tests, unchanged.
7. The after measurements above.

Rollback of A and B: revert the change. Nothing is retained, so no store needs repair.

#### Proposed writer checkpoint (owner decision pending, deferred 2026-10-04)

This design is (C). It carries no requirement ID and governs nothing until the owner accepts it,
which waits on the A and B numbers above. It would let a journal writer (`store.Mutate` and lease
preparation) make at most one pass over receipt history, and only when its writer checkpoint cannot
be used, and otherwise not read, hash, open or individually watch the files that history alone
retains.

1. Writer checkpoint. The file is `<state directory>.writer-checkpoint.json`, with profile
   `taskman-writer-checkpoint/0`. It sits beside the journal state directory, never inside it,
   and is separate from the S12 read checkpoint so that older runtimes neither read nor replace
   it. Its codec is closed and bounded by the CAL-V0-059 limits. It records:
   - everything CAL-V0-059 records;
   - the request index: every `requests/` path with the sequence and digest of its afterimage,
     path-ordered;
   - the name sets of `receipts/`, `requests/` and `evidence/`, together with the exact count,
     byte and archive-manifest-encoding contribution of those write-once files, so that capacity
     is measured without opening them.

   It MUST be derived only from a complete, settled, consistent `FULL` audit, retained by the
   writer under CAL-V0-060's conditions, and never derived from a resumed observation. A request
   index that would exceed the codec bound is not retained; writers then keep the complete audit.
2. Rebinding. Under the writer lock and after pending-receipt redo, a writer that resumes MUST
   confirm:
   - the queue ID, primary worktree, init digest, generation and version digest;
   - that the head is at or beyond the checkpoint sequence by at most `K` receipts;
   - that the named receipt still hashes to its recorded digest;
   - that a names-only listing of each write-once directory equals the checkpoint's names plus
     the tail's posts.

   It MUST then:
   - replay every tail receipt with the unchanged per-receipt validators, including the duplicate
     request check against the request index;
   - verify every live projection, staging emptiness, the barrier, reservations and the intent
     tree exactly as the complete audit does;
   - verify the target request: for a replay it is present with its indexed digest, and for a
     new request it is absent from both the index and the listing.

   Lease preparation resumes the same way. Its change guard watches the live files and the
   write-once directories rather than every retained file.
3. Equivalence. Wherever the resumed path completes, the outcome, receipt bytes, replay result and
   capacity cost MUST equal those of the complete-audit path. Any decode error, mismatch, refusal
   or oversized tail MUST fall back to the complete audit, whose verdict governs. An unusable
   checkpoint never yields a refusal, a receipt or a capacity verdict that the complete audit
   would not. Every barrier removal, reconciliation and `receipt audit` keeps the complete audit,
   and so does every mutation once `K` receipts have landed since the last complete audit.

Detection limit (the owner trade-off). A resumed writer does not re-read history before the
checkpoint sequence. An altered prefix receipt, an altered `requests/` or `evidence/` file that the
tail does not post, or a checkpoint altered together with its projection is therefore detected only
by the next complete audit, at most `K` receipts later, or by `receipt audit`. Up to `K` receipts may
land on such a prefix before the store refuses, as it refuses today. `K` is owner-set; this
proposal suggests 256, which at the measured 0.5–0.7 ms per replayed receipt bounds a resumed tail
at roughly 0.2 s (an inference). Name-level strays, gaps and
deletions in the write-once directories, and duplicate request IDs, are still refused or fall back
on every mutation.

Interaction with product invariant 7. The writer checkpoint is derived state: a write-once encoding
that is replaced by rename and that nothing ever updates in place. It is not a database, has no query
engine, needs no daemon, and is never an input to authority, ranking or archive content. Deleting it
costs one complete audit. Because it is an immutable derived index encoding, it still needs the
benchmark and format gate before acceptance:
- this benchmark, before and after on the same host;
- decode-limit and malformed-input tests for the closed codec;
- an exact capacity-parity test against the full inventory;
- a build-log format entry.

Checkpoint failure modes:

| Failure | Handling |
|---|---|
| Crash during retention (torn temporary file) | The rename leaves the old or the new file; a decode error falls back to the complete audit |
| Stale checkpoint | The tail is replayed; a tail longer than `K` forces the complete audit |
| Checkpoint from another queue, generation or forked history | Rebinding fails, and the complete audit governs |
| Older runtime writes receipts without maintaining the checkpoint | The tail grows until `K` forces a complete audit |
| Write-once name added, removed or missing | The listing disagrees, so the complete audit governs |
| Prefix bytes tampered | Detected at most `K` receipts later, as under the detection limit above |
| Capacity aggregate wrong (defect) | The parity tests and every complete audit recompute it; the complete audit governs |

The S12 read checkpoint is then refreshed only on complete audits, so a read replays at most `K`
receipts.

Checkpoint acceptance evidence, if accepted:
1. `BenchmarkCALV0070_MutateAt2000Receipts` before and after, on the same host and fixture with
   `-benchtime 5x`, recording load.
2. `TestCALV0070_WriterHistoryProfile` at 500, 2,000 and 7,000 receipts, where the resumed
   `Mutate` cost at 7,000 is within an owner-set factor of its cost at 500.
3. A differential test, for every mutation kind, showing that the resumed and complete paths
   produce identical outcomes, receipt bytes and capacity cost.
4. Refusal or fallback for each of these counterexamples:
   - a duplicate request ID against the prefix, with its projection deleted;
   - an altered receipt at the checkpoint sequence;
   - a stray, missing or gapped receipt;
   - a foreign or torn checkpoint;
   - a tail longer than `K`;
   - capacity at each hard limit.
5. Crash injection before and after the retention rename.
6. `receipt audit` returning `FULL` with CONSISTENT/AGREES after a concurrent wave.
7. The opt-in issue 545 wave at 7,140 receipts, with its outcome counts and worst latency retained.

Rollback: delete `<state directory>.writer-checkpoint.json` to force the complete audit until the
next complete audit retains a new one, or revert the writer option. No retained format changes.

Implementation plan, in order:
- (A) Merge the lookup and complete audits and reuse the observed digests in the inventory.
  Delivered for `store.Mutate` (CAL-V0-070 items 1 and 2). The digest reuse was withdrawn in the
  second review fix: the inventory is read fresh and checked against the audit's digests, so A
  saves the second audit only. Lease commands already made one pass, so A does not help them.
- (B) Open each parent directory once per audit attempt instead of three opens per file, keeping
  the no-follow and identity checks. Delivered for journal audit reads (CAL-V0-070 item 3); the
  inventory's fresh reads are unchanged.
- (C) The writer checkpoint and rebinding above, as its own reviewed slice: owner decision
  pending, deferred 2026-10-04.

### S21 — Multi-repository supervised programs (issue 354)

Authoritative input: owner request [issue 354](https://github.com/beamfall/corvint/issues/354) and
the owner scope decision of 2026-10-04 recorded in S13; native ticket V1-0475. This slice lets one
`taskman-codex-supervisor/0` program carry candidate changes in up to eight additional,
operator-declared Git checkouts beside the queue repository, and binds them into one reviewable
candidate (CAL-V0-071..072). CAL-V0-087 evaluates required gates over that composite candidate and
integrates each changed repository only into its own operator-designated checkout, exactly once
across interruption and restart; a changed repository without a designation still fails closed.
CAL-V0-088 gives every stage a Core context packet for each repository. CAL-V0-089 lets any
supervised program, with one repository or several, continue a stage that its own stage wall stopped,
resuming the same host session in the same preserved worktree a policy-bounded number of times.

- `CAL-V0-071`: The optional policy `supervision` object MAY carry `repositories`, a closed object of
  1..8 entries keyed by a name (a lowercase ASCII letter followed by lowercase letters, digits or
  `-`, at most 32 bytes), each a closed object `{pathSha256}` pinning the SHA-256 of one absolute
  checkout path. The supervisor config MAY carry `repositories`, a name-sorted, duplicate-free array
  of `{name, checkout}`. A new program MUST be refused, before its runtime read, program record,
  worktree, effect or host process, when any entry is not declared by the policy, its checkout is not
  an absolute, lexically clean path, or its path digest differs from the pin; an existing program is re-checked
  at every stage launch as in CAL-V0-062. At admission the supervisor MUST record, for each entry,
  the checkout, its Git common-directory identity and its `HEAD` commit as the repository base, with
  no candidate. Each repository's worktree is the detached sibling `<primary worktree>@<name>` at
  that base for `implement` and at its recorded candidate for later stages. Only the `implement`
  Codex invocation receives those sibling worktrees as `sandbox_workspace_write.writable_roots`;
  review and integrate invocations receive none. A later program transaction MUST refuse a changed
  repository name, checkout or identity (`immutable repository binding differs`); a base changes
  only on reassignment, which also clears the candidate. Changed paths in repository `name` are
  scope-checked as `@name/<path>` against the ticket's touch paths, so an undeclared repository path
  blocks the program `OUT_OF_SCOPE` exactly like a primary path. Absent `repositories`, policy,
  config and `programs.json` bytes are unchanged.
- `CAL-V0-072`: After `implement`, each repository with changes MUST receive a candidate commit
  under its own `refs/corvint/tasks/` namespace, parented on its base, without moving that
  checkout's `HEAD` or index. The program candidate MUST then be the composite tree whose `.queue`
  entry is the primary candidate tree and whose `<name>` entries are `160000` gitlinks to each
  repository candidate (or base when unchanged); a primary changed path beginning with `@` returns
  the question `ambiguous multi-repository path` instead of a tree. Review MUST recompute the
  composite from the recorded candidates and refuse any mismatch, so `READY_FOR_INTEGRATION` binds
  every repository's exact commit. CAL-V0-087 governs gates over, and integration of, that
  composite candidate.
- `CAL-V0-087`: A config `repositories` entry MAY carry `integrationBranch`, a label naming the
  branch of that checkout that receives the repository's candidate. When the config also sets
  `ownIntegrationCheckout`, admission MUST record each such entry's `integrationBranch` and the
  checkout's directory identity as `integrationIdentity` in its `programs.json` repository record
  (both or neither), as part of the immutable repository binding; without `ownIntegrationCheckout`
  no designation is recorded. Then:
  1. Each required gate of a multi-repository attempt MUST run in the primary stage worktree with
     every sibling `<worktree>@<name>` present, and MUST observe the composite of the worktree's
     `HEAD` tree and each sibling's `HEAD` commit, clean only when every worktree is clean. A
     sibling that is absent or not of its recorded repository refuses `STALE_TREE`; an
     uncommitted or untracked change in any of them refuses `DIRTY_WORKTREE`; the gate result's
     candidate and executed tree are composites. A direct `gate run` on such an attempt observes
     the same composite.
  2. With repositories, the INTEGRATE grant scope MUST be `taskman-integration:` followed by the
     SHA-256 of the base commit, candidate tree and queue intent branch joined by NUL, followed,
     for each repository in name order, by NUL, its name, NUL and its `integrationBranch` (empty
     when undesignated). A grant whose scope is the single-repository formula, or names other
     branches, is refused `APPROVAL_MISSING`.
  3. Before a grant is recorded, the integrator MUST refuse `UNSUPPORTED` (`repository <name>
     changed but has no integration designation`) when any repository's candidate differs from
     its base and the record carries no designation; and for each designated changed repository
     MUST refuse unless its checkout is the recorded directory of the recorded repository, on
     `refs/heads/<integrationBranch>`, clean and at its base (`designated checkout branch differs`,
     `TARGET_ADVANCED: repository <name> advanced`). A repository whose candidate equals its base
     needs no designation and is never moved.
  4. After `INTEGRATE_INTENT` and under the store lock, the supervisor MUST re-check every target,
     then fast-forward each changed repository's designated checkout to its candidate in name
     order, with hooks disabled, before fast-forwarding the queue checkout. A checkout already at
     its candidate is not landed again.
  5. A restarted integrator with a pending integration effect MUST resume: with the queue checkout
     at its base it repeats item 4, skipping landed repositories; with the queue checkout at the
     candidate it MUST find every changed repository's candidate on its integration branch, or
     stop `BLOCKED_RECOVERY`, before recording `INTEGRATED` and native completion once.
  6. Native completion MUST compare the composite of the integrated commit and each repository
     candidate with the candidate tree, and MUST find each changed repository's candidate on its
     designated integration branch.
  7. Worktree cleanup after proved completion MUST remove each repository's sibling stage
     worktrees from that repository's own worktree registry.
- `CAL-V0-088`: Every stage of a multi-repository program MUST obtain, besides the primary packet,
  one Core context packet per repository, queried in that repository's sibling worktree at its
  `HEAD` and held to the same READY, fresh and revision checks; any failure refuses the stage
  `repository <name>: CONTEXT_UNAVAILABLE` before the host is launched. The packets reach the host
  prompt as `repositoryContext`, keyed by repository name, beside `repositories`.
- `CAL-V0-089`: The optional policy `supervision` object MAY carry `continuations`, a canonical
  decimal count from 1 to 16; absent means none, and 0, a count above 16 or a noncanonical value is
  refused. With `continuations` present:
  1. A new program, before its runtime read, program record, worktree, effect or host process, and
     every later stage launch, MUST refuse `UNSUPPORTED` when the configured host does not report
     the session of a turn stopped before its final result (Claude Code, whose session appears
     only in that result), or when the lane or the program declares a nonzero input or output
     token cap, because no supported host reports the token usage of an interrupted turn and
     DISPATCH would refuse every continuation as `lane token usage unknown`.
  2. A stage is continuable only when its supervised run was stopped by that stage's own wall while
     the caller's context was live: not by the program wall, a program control, a lost heartbeat,
     a failed watch read or the caller. It must then have stopped cleanly into `WAITING` on the same
     stage, unanswered, with a recorded host session, its preserved worktree, no live worker and
     proved quiescence. An `implement` stage stopped at its wall still commits its partial work
     as a per-turn candidate ref before it waits.
  3. In the same role invocation, the supervisor MUST then answer the recorded question with
     `checkpointed continuation <n> of <N>` and launch the stage again, resuming the recorded
     session in the same worktree (Codex `exec resume`; OpenCode `--session` with `--fork`,
     CAL-V0-077), at most N times. Each continuation is an ordinary ANSWER and DISPATCH, so ticket
     and policy revision fencing, the lane turn cap and the shared program turn and wall caps still
     apply. The supervisor MUST NOT start one when the lane turn cap, the program group's turn cap
     or its wall leaves no room, or when a program control is pending; the attempt then stays
     `WAITING` with the question unanswered, the program `FINISHED` with its owner released, and the
     role returns the stage's deadline error. A continuation's admission, its first program write,
     MUST refuse `FENCED` when the program revision it replaces carries a control, so a drain or
     cancel recorded after that check starts no host turn: the program stays `FINISHED` with its
     owner released and the control recorded, and the attempt stays `WAITING` with the continuation
     answer recorded until an operator resumes the program. A control recorded after admission stops
     the running turn through the stage watcher, as for any stage.
  4. An integrate-stage continuation MUST reuse the grant recorded before the wait, not record
     another, and the queue checkout MUST NOT move until `INTEGRATE_INTENT`, so integration
     happens once after the last continuation.
  5. No unanswered checkpoint is continued automatically by `run`. An explicit operator `retry`
     answers it and resumes the same session and worktree, refused by the same caps. An answered
     integrate-stage wait resumes under its recorded grant; a different nonempty grant is refused
     `APPROVAL_MISSING`.

  Absent `continuations`, policy and program bytes and every stage's behavior are unchanged.

Non-goals: pushing any checkout; atomicity against Git writers outside the store lock (see the
partial-landing failure mode below); integration of a changed repository that has no designation;
a repository whose Core index is unavailable (the stage refuses rather than degrading to
primary-only context); submodule semantics in the operator's checkouts (the gitlinks exist only in the
composite object); Claude Code and OpenCode supervisor hosts (V1-0755, V1-0756); continuation on a
host that does not report an interrupted session (Claude Code would need its session fixed at
launch or a streamed session event), continuation under a token cap, a single turn longer than
the stage wall, automatic continuation after the owning process exits, and the writer checkpoint
(V1-0645). Failure modes: a checkout moved
to another path no longer matches its pin and is refused; a sibling worktree whose common
identity, commit or cleanliness differs from the record refuses the stage rather than overwriting
work; uncommitted edits in the operator's checkout are neither read nor carried, because the
sibling starts from the recorded commit; an extra repository edit outside the
declared `@name/` touch paths blocks the program; a checkout re-cloned or retargeted after
admission keeps its path pin but is refused by its recorded common identity before Git registers a
worktree in it; two queues sharing one extra checkout and one program ID collide on its
`refs/corvint/tasks/` candidate ref, and the second ref write fails closed; the composite tree object
is referenced by no ref and may be pruned by `git gc`, which is harmless because review, gates and
completion recompute it from the recorded per-repository candidates; a designated checkout that
another writer advances after an earlier repository landed stops integration `TARGET_ADVANCED` with
that earlier landing kept and never repeated, and the attempt stays `READY_FOR_INTEGRATION` with its
pending effect until the operator returns the advanced checkout to its base or candidate (CAL-V0-087);
a continuation resumes the host's own record of the session, so a host that discarded or corrupted
that session fails the resumed turn and leaves the preserved worktree and its per-turn refs for the
operator; an owner process killed during a continuation leaves the attempt to the existing stale
owner recovery, never a second concurrent continuation; the stage watcher polls once a second, so a
control recorded just after a continuation's admission lets the resumed host run until the next
poll before it is drained, as for any stage (CAL-V0-089).
Downgrade is one-way: a binary without this slice
refuses a `programs.json` that records repositories (`unknown field`), and because a full journal
walk revalidates every retained post, it also refuses the retained history once any
multi-repository program record is journaled, even after those programs are drained or cancelled,
unless its reader resumes from a checkpoint after that record; the same holds for a record that
carries `integrationBranch` against a binary without CAL-V0-087. Rollback removes `repositories` from the policy, which refuses every later stage of a
multi-repository program while leaving single-repository programs and their bytes unchanged; the
candidate refs and sibling worktrees are ordinary Git state the operator may remove; dropping
`integrationBranch` from a config makes a later program's changed repositories refuse integration
again; removing `continuations` returns every wall stop to the operator `retry` path, and a binary
without CAL-V0-089 refuses a policy that carries it (`unknown field`). Regression witnesses are the
CAL-V0-071, CAL-V0-072, CAL-V0-087, CAL-V0-088 and CAL-V0-089 rows in the traceability table,
including the fake-host end-to-end `TestCALV0071_MultiRepositoryProgramFakeHost`,
`TestCALV0087_DesignatedMultiRepositoryIntegration` and
`TestCALV0089_CodexContinuationResumesPreservedSession`. Live Codex and OpenCode qualification is
`NOT_RUN`; see
`docs/build-log/2026-10-04-tasks-multirepo-programs.md` and
`docs/build-log/2026-10-05-tasks-multirepo-continuation.md`.

Decided 2026-10-05 (ticket V1-0475, issue 354): the owner accepted S21 (CAL-V0-071..072 and
087..089) and CAL-V0-086 with live host qualification `NOT_RUN`. Under the owner's delegation the
orchestrator kept every fail-closed implementation choice: the 4096-byte `worktreePath` PathText
with its one-way downgrade, the 30-second watcher read tolerance and drain `EPERM` re-probe, Core
context required for every repository, `integrationBranch` validated only as a label, an earlier
repository staying landed when a later one refuses `TARGET_ADVANCED`, a continuation ceiling of 16,
continuation refused under token caps and for Claude Code, and `continuable` failing closed on a
transient read error. The CAL-V0-086..089 allocation stands. Live Codex, Claude Code and OpenCode
qualification, live multi-repository integration, Linux and `make gate` are `NOT_RUN` and tracked
with the remaining questions by follow-up V1-0824; writer checkpoint part C stays deferred (V1-0645).

### S22 — Claude Code supervised host (V1-0755, split from issue 354)

Authoritative input: owner request [issue 354](https://github.com/beamfall/corvint/issues/354), which asks for host adapters
beyond Codex and names a Claude Code adapter as the next one; the owner scope decision of
2026-10-04 recorded in S13, which moved the Claude Code and OpenCode supervisor hosts to native
tickets V1-0755 and V1-0756; and V1-0755's acceptance criteria. This slice lets an owner-enabled
`taskman-codex-supervisor/0` program drive a pinned Claude Code executable in place of Codex. It
uses the same S10 lifecycle, S13 effort and stage wall, and S21 repository bounds. The profile and
runtime ID keep their names: they identify the supervisor protocol and the single policy runtime
slot, not the vendor of the pinned binary. The host seam added here is a vocabulary, meaning an
argv builder, a result decoder, a session reader and a usage reader, selected by one host name. A
later host such as OpenCode (V1-0756) adds one entry; it does not change the lifecycle.

- `CAL-V0-074`: The optional policy `supervision` object MAY carry `host`, whose admitted
  values are `claude-code` and, per CAL-V0-076, `opencode`; absent means Codex. The supervisor
  config MAY carry `host` with the same values and meaning. A program MUST be refused with `UNSUPPORTED` before its runtime read,
  program record, claim, lease, worktree, effect or host process when either of these holds:
  - the config `host` is another value;
  - the config `host` differs from the policy host.

  This check runs for a new program and again at every later stage launch, as in CAL-V0-062. An
  existing program also repeats it on every reopen whose current attempt is absent, terminal or
  unsupervised, before it reassigns, claims or attaches work. A live supervised attempt keeps only
  drain and cancel after a host change. The config digest binds `host`, so a recorded program
  cannot change hosts. `run`, `admit`, `resume`,
  `retry`, `answer`, `drain` and `cancel` MAY pass `--host codex` or `--host claude-code`. When
  given, the value MUST equal the config host, or the command is refused `UNSUPPORTED` before any
  store mutation.

  The pinned-executable check is host-neutral, and admission applies the launch-time check itself.
  A program MUST be refused with `CAPABILITY_UNAVAILABLE` before any program record, claim or
  lease in any of these cases:
  - its executable path is not absolute, is missing or unreadable, or is not a regular file;
  - the path is a symlink;
  - the file has no execute bit;
  - it does not match an enabled policy `runtimes` entry for the profile on path digest, content
    digest and mode;
  - its digest differs from the config `executableSha256`.

  A symlink is refused rather than resolved, and the refusal names its target to pin instead. The
  evidence: on the owner's host, `/opt/homebrew/bin/claude` and `/opt/homebrew/bin/codex` are
  package-manager symlinks to regular executables that a package update retargets, so a pinned path
  could change meaning without its pin changing. The lane leader's capsule validation already
  refused symlinks and non-regular files. The check opens the path once without following a final
  symlink, and takes the file type, mode and bytes from that one descriptor.

  The lane leader MUST execute the object it verified, not a second lookup of the path. A runtime
  whose canonical path and every ancestor directory are owned by root, writable by neither group
  nor other, and carry no access control list runs by its path, because substituting it needs root.
  An ACL entry can grant write access that the mode does not show, so any ACL, or a failure to read
  one, sends the runtime to the copy path. Any other runtime runs from a
  private copy of the verified bytes, which the leader writes before boot into the private effect
  directory and removes when the host exits. Replacing or rewriting the pinned path while the
  supervisor acknowledges the boot therefore cannot change what runs. Two limits follow:
  - A runtime that loads files relative to its own path runs without them from the copy, so pin a
    self-contained binary. On the owner's host this means the Claude Code `claude.exe` binary, and
    not a script wrapper such as Codex's `codex.js`.
  - macOS launch constraints kill a copied platform binary such as `/bin/sh`. Those binaries are
    root-protected, so they run by path.

  Each stage launch of an unprotected runtime writes one copy of up to 256 MiB beside the stage
  worktree. If the leader is killed while the host runs, the copy stays in the retained effect
  directory.

  If launch is refused after admission, for example because the file changed in between, only a
  refusal made before the lane leader is forked counts. The dispatched stage then settles as
  `NO_EXEC` with quiescence proved and the program is recorded `FINISHED` while its owner still
  holds it. The attempt is then cancelled, which releases its claim and reservation, and only then
  is the owner released. While the owner is live and unreleased no other owner can take the
  program, so none can fence the cancel. The stage returns `CAPABILITY_UNAVAILABLE`.
  The stop, the `FINISHED` record, the cancel and the owner release are separate journal writes,
  because the attempt and program records have no combined transition. If the owner dies after
  the `FINISHED` record, a replacement process may take the program over once the owner is gone.
  Before the cancel the attempt stays `WAITING`, stopped and quiescent, with its claim and
  reservation, and a replacement that reopens the program with its original config and pin and
  cancels it releases them. After the cancel the claim is already released and a reopen reassigns
  the program.
  Known limit: an owner that dies before the `FINISHED` record leaves the program `SPAWNING`, if it
  dies before the settlement starts, or `STOPPING` after that. Neither is a safe takeover phase, and
  recovery evidence covers only an attempt that still has a worker and a retained leader boot
  record, which a refused launch never writes. A replacement is therefore refused, and the attempt
  keeps its claim and reservation. Closing this
  window needs a combined attempt and program transition, or a takeover rule that admits a bound,
  stopped and quiescent attempt; both change the transaction contract. A failure after the
  fork never settles as `NO_EXEC`, even when it carries no outcome class: it keeps the drain result,
  and an unproved drain leaves the attempt in `BLOCKED_RECOVERY`. This also names the existing Codex
  refusal, which was previously the unnamed `MALFORMED`. The Core
  CLI requirement, `requireEnforcedFields`, roles, worker limits and budgets are unchanged. When
  `host` is absent, the policy, config, capsule, argv and `programs.json` bytes are unchanged.
- `CAL-V0-075`: A `claude-code` stage MUST invoke the pinned executable with the prompt on
  standard input. Its argv is:
  1. `-p --output-format json --model M --effort E --setting-sources project --strict-mcp-config --permission-prompts none`, where E is the stage's configured
     effort (CAL-V0-062).
  2. The permission flags: `--permission-mode acceptEdits` for `implement`; for `review` and
     `integrate`, `--permission-mode dontAsk --disallowedTools Edit,Write,NotebookEdit`.
  3. `--resume S` only when the stage continues the answered WAIT session S. For either host, a
     later stage of the same attempt starts a fresh session, so review never resumes the
     author's session.
  4. When any exist, `--add-dir` followed by the sorted S21 sibling worktrees. This applies in
     every stage, because Claude Code confines its file tools to its working directories.

  The capsule MUST record `host`, so that the lane leader and the supervisor apply one result
  vocabulary. On a zero exit, standard output MUST be exactly one JSON object, optionally followed
  by whitespace, with all of the following:
  - `type` `result`, `subtype` `success` and `is_error` false;
  - a nonempty `session_id` of at most 128 bytes;
  - a nonempty string `result` that strictly decodes to the S10 minimum handoff object: kind
    `HANDOFF`, `BUILT`, `REVIEW` or `WAIT`, with a nonempty summary and nextAction.

  No object in the result, its `usage` or the decoded handoff may repeat a member name. Neither the
  result object nor the handoff may carry a member that differs from one the profile reads only by
  case folding, such as `IS_ERROR`, `Usage` or `Kind`, because Go's decoder would match it to the
  same field. Escaped spellings count as the names they decode to. Without these rules a later
  `is_error` false could override an earlier true, and two partial `usage` objects could merge into
  one that looks complete. Any other output is `INVALID_RESULT`. The result, session and usage
  readers share these rules, so a repeated or aliased member in the object or in a JSON handoff
  leaves all three unobserved. A prose `result`, such as an error report, is not a handoff and
  keeps its session and usage observable. Token usage is `OBSERVED` only when the object's `usage`
  carries integer `input_tokens` and `output_tokens`. Input then adds any present integer
  `cache_creation_input_tokens` and `cache_read_input_tokens`, so it counts all input as Codex's
  counter does. A missing or non-integer counter, or an overflow, makes the turn `NOT_OBSERVED`.
  Program and attempt usage are re-derived from the retained output in the vocabulary of the
  policy's host, so no counter is invented. Lifecycle, process-group ownership, the 16 KiB output
  caps, WAIT and resume, review independence, gates and integration binding are those of S10, S13
  and S21, unchanged.

Non-goals:
- Mixed hosts within one policy or program. The transaction layer has one supervised runtime slot,
  and per-host runtime IDs, roles and worker caps need a multi-runtime inventory.
- The OpenCode host (V1-0756), delivered in S23.
- Claude Code `stream-json` output.
- Efforts beyond the S13 set.
- Blocking Claude Code subagents beyond the prompt instruction.
- Containing Bash in any stage. Project permission rules govern Bash, and read-only stages stay
  subject to the existing check, made after the host exits, that the tree is unchanged.
- Host authentication.
- The unused `RunProgram` qualification helper, which refuses a non-Codex host.

Failure modes:
- Config and policy hosts differ, or the config names an unknown host: refused `UNSUPPORTED` before
  any mutation. This includes reopening an existing idle or completed program after the policy host
  changed.
- The pinned path is a symlink or lacks an execute bit: admission refuses
  `CAPABILITY_UNAVAILABLE` before any record. A launch refusal after admission and before the lane
  leader is forked settles the stage `NO_EXEC` and cancels the attempt, instead of leaving it
  `SPAWNING` or holding its claim.
- The pinned path is replaced or rewritten after the leader's check and before the acknowledgment:
  the leader runs the verified bytes, from a private copy unless the runtime is root-protected.
- The leader fails after it is forked, for example a boot identity mismatch or a refused `RUNNING`
  journal write, and its drain is not proved: the attempt stays in `BLOCKED_RECOVERY`, never `NO_EXEC`.
- The result object repeats a member, such as `is_error` or `usage`, or aliases one by case, such as
  `IS_ERROR`: `INVALID_RESULT`, with session and usage `NOT_OBSERVED`. The same holds for the
  handoff.
- The Claude Code binary is upgraded or replaced: its digest differs, so admission refuses
  `CAPABILITY_UNAVAILABLE` and an existing program refuses its next stage. Re-pinning is an owner
  policy change.
- Result text is wrapped in prose or a Markdown fence, or the object reports `is_error` or a
  non-success subtype such as a turn or budget limit: the stage is `INVALID_RESULT` on a zero exit,
  or `EXIT_NONZERO` otherwise. The retained output and S10's invalid-result path apply.
- The result object exceeds 16 KiB, for example because of a long `permission_denials` list: the
  stage is `OUTPUT_LIMIT`.
- The owner changes the policy host while a stage runs: the stage's finish transition is refused,
  because usage no longer derives from the retained output in the policy host's vocabulary. The
  program is left to S10 recovery until the policy host is restored, and the config host check
  refuses every later stage.
- Claude Code's `usage` totals differ from billed usage: this is unverified until live
  qualification.
- Downgrade: a binary without this slice refuses a policy that carries `supervision.host` (closed
  keys), a config that carries `host` (unknown field) and a capsule that carries `host`. Its full
  journal walk also refuses retained history that includes such a policy, unless its reader resumes
  from a checkpoint after that record.

Rollback: the config digest binds `host`, so an existing program can be reopened only with its
original config, and a config with `host` removed is refused with `program config differs`.
Rollback therefore takes three steps:
1. While the `claude-code` policy and its runtime pin are still in force, cancel every
   `claude-code` program using its original config, for example
   `corvint-tasks cancel --program P --config ORIGINAL`. A program drained first MUST still be
   cancelled: a drain leaves a live `WAITING` attempt that holds its claim and reservation.
2. Only then remove `host` from the policy, and re-pin the Codex runtime if it differs.
3. Start new programs from configs without `host`.

Codex policy and config bytes are unchanged by this slice. Once the Codex runtime pin replaces the
Claude Code one, every original `claude-code` config is refused `CAPABILITY_UNAVAILABLE` on reopen.
A program missed in step 1 therefore keeps its claim and cannot be cancelled through its own
workflow. To recover it, re-pin its Claude Code runtime with `host` still absent, reopen it with its
original config, cancel it, and restore the Codex pin. Its live supervised attempt then keeps drain
and cancel access but never launches another stage. An idle or completed program is refused on
reopen and holds no claim. Regression witnesses are the CAL-V0-074 and CAL-V0-075 rows in the traceability
table, including the fake-host end-to-end `TestCALV0075_ClaudeCodeProgramFakeHost`. Live Claude
Code qualification on a disposable program is `NOT_RUN`; see
`docs/build-log/2026-10-04-tasks-claude-code-supervisor-host.md`.

### S23 — OpenCode supervised host (V1-0756, split from issue 354)

Authoritative input: owner request [issue 354](https://github.com/beamfall/corvint/issues/354), which asks for host adapters
beyond Codex; the owner scope decision of 2026-10-04 recorded in S13, which moved the OpenCode
supervisor host to native ticket V1-0756; and V1-0756's acceptance criteria. This slice adds
OpenCode as one more entry on the S22 host seam. Lifecycle, process-group ownership, the 16 KiB
output caps, WAIT and resume, review independence, gates and integration binding stay those of S10,
S13 and S22.

OpenCode differs from Codex and Claude Code in two ways that matter here. Its extensions are
in-process plugins loaded by its session server, not hooks run as separate commands (AHI-044), and
`opencode run` attaches by default to a persistent shared background service. A supervised stage
therefore defines its own process and lifecycle boundary. The foreground process that the lane
leader owns and reaps is `opencode run --standalone`. That process starts its private session
server as its own child, from the same executable, over standard I/O (`serve --stdio`), with
`SIGTERM` and a forced kill after 3 seconds when the run ends. The plugins load in that server.
OpenCode's process spawner defaults to `detached`, and neither the server spawn nor its tool
(`bash`) spawns override it, so the server and every tool process start in a process group and
session of their own, outside the supervisor-owned group. No flag or environment entry keeps them
in. OpenCode is therefore a detached host: the supervisor discovers, drains and proves gone those
escaped groups itself (CAL-V0-077). Nothing a plugin does is a supervisor input. The contract
below was read from OpenCode 2.0.21 (`opencode run --help` in an isolated home, and its bundled
`run`, standalone-endpoint, process-spawner and session modules, binary SHA-256
`0b2b68c1efaf20a29aaf636c2ffccc1abb56243a82f48cce45e257d232e03442`); no live model run was made.

The S22 verified-runtime launch applies unchanged. The 2.0.21 executable is a self-contained 180 MiB
Mach-O binary, and the pin must name it, not a symlink to it. On the owner's host it is not
root-protected (a Homebrew Cellar file), so each stage launch writes a private copy of it beside the
stage worktree. Whether a copied OpenCode binary starts is unverified until live qualification.

- `CAL-V0-076`: The policy `supervision.host` and the config `host` of CAL-V0-074 MAY also be
  `opencode`, and `--host opencode` is admitted where `--host` is. Every CAL-V0-074 rule applies
  unchanged: refusal with `UNSUPPORTED` on a host mismatch, the host-neutral
  `CAPABILITY_UNAVAILABLE` executable pin, the config digest binding the host, and unchanged bytes
  when `host` is absent. An `opencode` program MUST additionally be refused with `UNSUPPORTED`,
  at the same point and again at every later stage launch, when either of these holds:
  - the config `model` is not one `provider/model` reference with nonempty parts and no `#`,
    because the stage effort becomes the model variant;
  - the config names `repositories`, because every OpenCode stage denies directories outside its
    worktree (CAL-V0-077), so S21 programs are not supported on this host.
- `CAL-V0-077`: An `opencode` stage MUST invoke the pinned executable with the prompt on standard
  input and this argv:
  1. `run --standalone --format json --model M#E`, where E is the stage's configured effort
     (CAL-V0-062). `--standalone` runs a private session server for this invocation instead of
     attaching to the shared background service. `--server`, `--continue`, `--auto` and the
     permission-skipping flags are never passed.
  2. `--session S --fork` when the stage continues the recorded WAIT session S. With `--fork`,
     OpenCode refuses a session that does not exist (`Session not found`, exit 1) instead of
     creating it under the same ID, and continues an existing one in a new session forked from
     its history.

  The stage environment is the S10 inherited set plus exactly these entries:
  - `OPENCODE_DISABLE_AUTOUPDATE=1`, so the pinned binary is never replaced mid-program;
  - `OPENCODE_DISABLE_PROJECT_CONFIG=1`, so configuration and plugins in the untrusted stage
    worktree are not loaded;
  - `OPENCODE_PRINT_LOGS=1` and `OPENCODE_LOG_LEVEL=ERROR`, so the detached server inherits the
    stage's standard error (at error level only) instead of discarding it;
  - `OPENCODE_CONFIG_CONTENT` carrying an inline `permission` object that denies `task` (sub-agents)
    and `external_directory` in every stage, and also `edit` in `review` and `integrate`. Rules
    that would ask are rejected, because `--auto` is never passed.

  The process boundary: the capsule of a detached host MUST carry, as the last assignment of its
  key, every entry its vocabulary requires (`OPENCODE_PRINT_LOGS=1`), or the supervisor and the
  lane leader refuse it before any spawn. While the host runs, the supervisor MUST observe, at
  least every 200 ms and once more before cleanup, every live process whose parent belongs to the
  owned group or to an escape already found but whose own group differs. Such an escape MUST lead
  its own group and is retained by PID and start identity; an escape that joined another group, or
  an observation that fails, is uncertainty. A numeric group is never trusted on its own: a group
  is expanded only while its leader still holds the start identity recorded before the process
  snapshot, checked after that snapshot, and a candidate is retained only when a later snapshot
  still shows it, under the identity read before that snapshot, as a child of such a group. A
  leader PID now held by another process retires the recorded group, since a PID is not reused
  while its group exists, and nothing in a reused group is adopted or signalled. At cleanup the owned group is drained as in S10, then
  each escaped group is drained children first and its leader with `SIGTERM` and, after a bound,
  `SIGKILL`. The stop is clean only when no observation was uncertain, every group is gone, and
  both host output pipes reach end of file within a bound. Because the server holds the stage's
  standard error, that end of file is the proof that no server outlived the run, including one
  orphaned before any observation found it. Recovery after a lost supervisor (CAL-V0-033) of a
  detached host drains the retained leader group and the escapes it can observe, but MUST NOT
  prove quiescence: an escape started after its discovery snapshot, or orphaned before it, has no
  link to the retained group, and no witness of absence survives the supervisor (the output pipes
  end with it). Detached recovery therefore always refuses as quiescence uncertain, and the
  operator must clear it.

  The plugin boundary: plugins from the operator's own configuration under `HOME` still load,
  in-process, in the detached server. This is a known limit of this slice (owner decision
  2026-10-04): they are not contained, audited or allowlisted. They are bounded only by the
  escaped-group drain above, the stage wall and the output caps. The supervisor reads only the
  stage's standard output and exit status. No plugin, server endpoint or shared service is a
  supervisor channel, and no plugin output is evidence.

  The capsule MUST record `host`. On a zero exit, every standard-output line MUST be a JSON object
  whose `type` is `step_start`, `text`, `reasoning`, `tool_use`, `step_finish` or `error`. The
  stream as a whole MUST meet all of the following:
  - every line carries one nonempty `sessionID` of at most 128 bytes, constant across the stream;
  - there is no `error` event;
  - there is at least one `step_finish`, and the last one has `part.type` `step-finish` and
    `part.reason` `stop`;
  - the last `text` event comes before that final `step_finish`, has `part.type` `text`, and its
    `part.text` strictly decodes to the S10 minimum handoff object;
  - no object in any line repeats a member name, and no member of an object the profile reads
    (the event, a text or step-finish part, its `tokens` and `tokens.cache`) or of a JSON last
    `part.text` differs from a read name only by case folding, as for Claude Code (CAL-V0-075).
    Such a stream leaves the result, the session and usage unobserved.

  Any other output is `INVALID_RESULT`. When the stage continues the recorded session S, it MUST
  advance only on positive evidence that S and its history existed: a run that decodes a session
  other than S, which under `--fork` exists only as a fork of an existing S. A run that names no
  session (the fork failed, for example because S is missing) or names S itself (a same-ID
  recreation without the history) MUST NOT advance, whatever its exit. The stage stops with the
  S10 resumable-handoff question instead of its expected kind, the attempt stays `WAITING` with S
  retained as its resume target, and the candidate is preserved as for any stopped implement
  stage. Its result class stays the one the host-neutral decoder recorded, because the S22
  vocabulary seam does not receive S. That the fork carries S's history is read from the bundle,
  not observed live.

  Token usage is `OBSERVED` only when accounting is complete: every `step_finish` carries
  nonnegative integer `tokens.input`, `tokens.output`, `tokens.reasoning`, `tokens.cache.read`
  and `tokens.cache.write`; the stream has no `error` event; every `step_start` has a later
  `step_finish`; the last `step_finish` has reason `stop`; and the retained output is shorter than
  the 16 KiB cap, since output that fills it may have been cut. These counters are disjoint, so
  input is input plus cache read plus cache write, and output is output plus reasoning, each
  summed over all steps with overflow refusal. Otherwise the turn is `NOT_OBSERVED`, never the
  partial sum. Program and attempt usage are re-derived from the retained output in this
  vocabulary (CAL-V0-075).

Non-goals:
- Running or qualifying a live OpenCode model; this slice uses a pinned fake host only.
- Supervising through the shared background service, `--server`, or an attached TUI.
- A plugin-based supervisor channel, and loading, auditing, allowlisting or otherwise containing
  operator (user-configuration) plugins: a known limit of this slice (owner decision 2026-10-04).
- Keeping the standalone server or tool processes inside the owned process group; OpenCode 2.0.21
  offers no way to, so they are drained as escapes instead.
- An OS sandbox. OpenCode has none, so `bash` stays governed by OpenCode's defaults in every stage,
  and read-only stages rely on the `edit` denial plus the existing check, after the host exits, that
  the tree is unchanged.
- Multi-repository programs on this host; mixed hosts within one policy or program (S22).
- Raising the 16 KiB output cap (owner decision 2026-10-04: overflow fails closed as
  `OUTPUT_LIMIT`), `--format default` output, and `--thinking`.
- Provider-specific variant names beyond the S13 effort set; the `RunProgram` qualification
  helper, which refuses a non-Codex host.

Failure modes:
- OpenCode's JSON stream includes full tool output in `tool_use` events, so a tool-heavy stage can
  exceed 16 KiB: the stage is `OUTPUT_LIMIT`, retained and recoverable as in S10. This is the
  likeliest live failure and is unmeasured until live qualification.
- The model's provider does not offer the effort as a variant, or ignores it: unverified until live
  qualification. A rejected variant makes the run fail, which is `EXIT_NONZERO` or
  `INVALID_RESULT`.
- The server, a tool or a plugin starts a process in its own group: it is an escape, observed
  through its live parent and drained (CAL-V0-077). An escape that is orphaned before any
  observation finds it is not reachable. If it holds the stage's standard error, as the server
  does, the stop is not clean (`BLOCKED_RECOVERY`). If it does not, it is not detected; this
  residual risk is the same as for any host child.
- The supervisor is lost: recovery drains what it can observe, but a late or orphaned escape could
  survive with no witness, so detached recovery always refuses as quiescence uncertain and the
  operator must clear it. This is a known limit of the slice; a crash-surviving witness is not
  defined.
- An escape exits and its PGID is reused by an unrelated process: the recorded leader identity no
  longer matches, so the group is retired and neither the reused group nor its children are
  adopted or signalled.
- OpenCode's own stderr logs at error level exceed the 16 KiB cap: `OUTPUT_LIMIT`, retained and
  recoverable as in S10.
- `OPENCODE_DISABLE_PROJECT_CONFIG` does not cover a project-level configuration source in some
  OpenCode version: that configuration could widen permissions. The permission object still
  arrives inline, but its precedence over project sources is unverified.
- The handoff is wrapped in prose or a Markdown fence, the final step ends for `length` or
  `tool-calls`, a step is left open, or the host reports an `error`: `INVALID_RESULT` on a zero
  exit, or `EXIT_NONZERO` (OpenCode exits 1) otherwise; usage is `NOT_OBSERVED`.
- The recorded session no longer exists when a WAIT stage resumes: the fork fails, and the session
  check keeps the attempt `WAITING` with S retained. A host that ignored `--fork` and recreated S
  under the same ID is refused the same way.
- OpenCode's `tokens` totals differ from billed usage: unverified until live qualification.
- The OpenCode binary is upgraded or replaced: its digest differs, and CAL-V0-074 refuses
  `CAPABILITY_UNAVAILABLE`.
- Downgrade: a binary with S22 but without this slice refuses a policy or config whose host is
  `opencode` with `UNSUPPORTED`, and a capsule whose host it does not know; an older binary
  refuses as S22 describes.

Rollback: the config digest binds `host`, so the S22 ordered rollback applies with `opencode` in
place of `claude-code`:
1. While the `opencode` policy and its runtime pin are still in force, cancel every `opencode`
   program using its original config. A program drained first MUST still be cancelled: a drain
   leaves a live `WAITING` attempt that holds its claim and reservation.
2. Only then remove or change `host` in the policy, and re-pin the other host's runtime.
3. Start new programs from configs for that host.

Once the pin changes, an original `opencode` config is refused `CAPABILITY_UNAVAILABLE` on reopen,
and an edited one `program config differs`. A program missed in step 1 therefore keeps its claim.
To recover it, re-pin its OpenCode runtime with the policy host unchanged, reopen it with its
original config, cancel it, and restore the other pin; its stages stay refused `UNSUPPORTED`.
No Codex or Claude Code bytes change. Regression witnesses are the CAL-V0-076 and CAL-V0-077 rows in the
traceability table, including the fake-host end-to-end `TestCALV0077_OpenCodeProgramFakeHost`.
Live OpenCode qualification on a disposable program is `NOT_RUN`; see
`docs/build-log/2026-10-04-tasks-opencode-supervisor-host.md`.

### V1-0793 critical-path read (issue 588)

Authority: owner answer 2026-10-05 (D8), profile `taskman-critical-path/0`.

- `CAL-V0-079`: `corvint-tasks critical-path <ticketId|local>` MUST return one
  `taskman-critical-path/0` item for any ticket (a gate ticket is an ordinary ticket). The closure
  starts at that ticket and follows, transitively, every dependency obligation that is not
  satisfied: `COMPLETED` unless the dependency is `COMPLETED` or archived from `COMPLETED`, and
  `GATE_PASSED` unless the gate oracle observes it satisfied (an unobservable gate result is
  followed with observation `NOT_OBSERVED`, never treated as satisfied). `chains` holds, for each
  frontier node (a closure node the walk follows no edge from), the longest root-to-frontier path,
  root first; chains are ordered longest first, then by the frontier's planning order. Each
  `nodes` entry carries `ticketId`, `status`, `priority`, `eligibility`, `depth` (nodes on its
  longest path from the root), `firstBlocker`, `blockers`, `holds`, `waitingOn` and `attempt`.
  `attempt` carries `observation` (`LIVE`, `NONE`, or `NOT_OBSERVED` when the journal is absent)
  and `attemptId`, `phase`, `holder` (lease holder), `stage`, `member` (pool member),
  `lastProgressSeq` (the attempt's `phaseSinceSeq`) and `lastProgressAt` (its recorded
  `lastHeartbeatAt`); each is the string `NOT_OBSERVED` when the reader did not observe it. The
  item also carries `estimate`, which is always `NOT_OBSERVED` in v0, and `human`, a short human
  form: one header line, one line per returned chain and one line per cycle.
- `CAL-V0-080`: The item MUST report `bounds` (`maxNodes` 256, `maxChains` 32), `nodesTotal`,
  `nodesReturned`, `chainsTotal` and `chainsReturned`, and `truncated: true` when either bound is
  exceeded. At most 256 nodes are returned (nodes on returned chains first, in chain order, then
  the rest of the closure root-first) and at most 32 chains, each with `length` and at most 256
  `ticketIds` (`truncated: true` on a cut chain). Edges between members of one dependency cycle are
  not followed, so the walk terminates; every reached cycle is listed once in `cycles` with the
  existing code `CYCLE` and its sorted members, the members appear as nodes with their `CYCLE`
  blocker, and a member first reached through its cycle is placed one step after the member that
  reached it. Cycle membership is computed once per component, the walk keeps only compact node
  records, and node details and blockers are derived only for returned nodes and human chain
  nodes, so the read stays linear in the closure even for a large cycle.
- `CAL-V0-081`: The verb MUST be a pure read over the TM-V0-008 snapshot used by `ticket show`
  (journal-absent stores use the same inventory projection): it takes no lock, writes no file,
  ledger or journal record, and carries `mutationAuthority: false` (product invariant 4).
  `blockers` is the planner's claim-blocker derivation for the recorded default external-agent
  plan (`blockerScope: RECORDED_DEFAULT_EXTERNAL_AGENT_PLAN`), in planner order: pool collision,
  pause barrier, cutover, enforced-budget, ticket-view blockers and unknowns except
  `COVERAGE_UNKNOWN`, then `RETRY_EXHAUSTED`; certain entries carry `observation: CERTAIN` and
  unknowns `NOT_OBSERVED`. `firstBlocker` is the first certain entry, else the first unknown, else
  null, which matches the reason `plan preview` reports; `eligibility` is `BLOCKED` with a certain
  blocker and otherwise `UNKNOWN` when blockers remain. A journal-absent store observes no barrier,
  attempt or pool, so attempt liveness stays `NOT_OBSERVED`. Blocker codes are
  an open set and `waitingOn` entries carry `kind` (`DEPENDENCY` in v0): execution prerequisites
  (V1-0787), `LOOP_DETECTED` (V1-0791) and derived `ESCALATION_PENDING` holds MUST appear later as
  further `blockers` entries or `waitingOn` kinds of the same shape, without a profile change.
  Readers MUST treat an unknown code or kind as an opaque blocker. `critical-path` is listed in
  the command verb inventory and answers `--help` without I/O (CAL-V0-047).

### V1-0788 prior-generation stage and member (issue 586)

Authoritative input: owner request [issue 586](https://github.com/beamfall/corvint/issues/586)
part 1, ticket V1-0788: a retried attempt keeps only `generation`, `quiescence` and `provedSeq`
for each ended generation, so which pool member and stage ran a failed generation could be
recovered only from receipts, and neither a claim result nor the dispatcher `claim` event named
the member. This amendment adds optional keys to the TCP-00 `taskman-attempt/0`
`priorGenerations[]` entry; the profile name and every other key are unchanged.

- `CAL-V0-096`: When a CLAIM or CLAIM_NEXT admits the next generation of an
  `external-agent` attempt, the appended `priorGenerations[]` entry MUST carry `stage`, `poolId`
  and `memberId`, copied from the ended generation's `stage` and `poolAllocation`; each is
  `null` when that generation had none, and `poolId` and `memberId` are null together. The three
  keys are present together or absent together; the closed decoder MUST refuse a partial set, an
  unpaired null, a `stage` outside `implement`, `review` and `integrate`, and any other key. An
  entry without them (legacy, or a supervised generation) MUST decode with no history and
  re-encode byte-identical. `attempt show` MUST add `history` to each entry: `RECORDED` when the
  keys are stored, otherwise `NOT_OBSERVED` with `stage`, `poolId` and `memberId` each rendered
  `NOT_OBSERVED`, never inferred. Every `claim` result item, a refusal included, MUST include
  `poolAllocation` (`null` without an allocation; an `ERROR` result carries no item), and the dispatcher `claim` event detail MUST include `pool` and
  `member` (empty without an allocation). `attempt show` and the other reads write nothing.

Non-goals: excluding a ticket's earlier authors from selection (issue 586 part 2); history for
supervised generations, which span stages and release their allocation per stage, so a single
stage and member would be a guess; recovering a legacy entry's member from receipt POST state;
backfilling existing records; and treating a recorded member label as identity or authority (a
label proves nothing about who did the work).

Failure modes: a generation ended before this change shows `NOT_OBSERVED` for good. A member
label may itself be the text `NOT_OBSERVED`; the `history` discriminator, not the value, says
whether a value was recorded. The claim event names the pool member current when the dispatcher
observes the claim, which is the claimed allocation unless it changed before that observation.

Rollback and downgrade: the decoders of earlier binaries are closed and refuse a
`priorGenerations[]` entry carrying the new keys as `MALFORMED`, so after a retry is admitted
by this version an older binary cannot read that attempt (`attempt show`, claims and other
commands that decode it). An older `receipt audit` can still succeed, because journal validation
checks attempt identity and does not run the attempt decoder, so audit success does not
demonstrate downgrade compatibility.
Reverting the code is safe only while no attempt record holds the new keys (check each
`<stateDir>/attempts/*.json` with `jq -e '[.priorGenerations[] | has("stage")] | any'`). Once one does, keep this version; there is no
supported rewrite of attempt records, and editing them by hand breaks receipt-bound audit. The
claim result and dispatcher event additions are read-side only and revert with the code.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-096 | `TestCALV0096_LegacyPriorGenerationsRoundTrip` (N-1 bytes pinned from origin/main ac818777), `TestCALV0096_RecordedPriorGenerationsRoundTrip`, `TestCALV0096_MalformedPriorGenerationHistory` (`internal/tasks/snapshot`); `TestCALV0096_EndedHistoryCopiesOnlyExact` (`internal/tasks/transaction`); `TestCALV0096_PriorGenerationRecordsStageAndMember` (`internal/tasks/store`); `TestCALV0096_HistoryObservation`, `TestCALV0096_ClaimAllocationAndPriorHistory` (`internal/tasks/cli`); `TestCALV0096_ClaimEventNamesMember` (`internal/tasks/dispatch`) |

### V1-0790 recorded hand-off target (issue 587 part 1)

Authoritative input: owner request [issue 587](https://github.com/beamfall/corvint/issues/587)
part 1, ticket V1-0790, and owner decision D6 (the target is stored on the attempt generation, not
as a ticket-record key, and surfaced as a derived `nextStage`). Free-form release prose was the only
place a worker could say which stage should run next, and routing loops built on that prose
consumed about 26 sessions on one ticket. This amendment adds optional keys to the TCP-00
`taskman-attempt/0` record and its `priorGenerations[]` entry. The profile name and every other key
are unchanged.

- `CAL-V0-082`: `release` MUST accept `--handoff-to implement|review|integrate` (the closed
  `intent.StageRoles`) and an optional `--handoff-reason CODE` only with `--reason HANDOFF` or
  `--reason REVIEW_RETURNED`, and only after the CAL-V0-044/046 clean-handoff verification passes.
  The release writer MUST record them as the optional top-level attempt keys `handoffTo` and
  `handoffReason` on the terminal generation it writes, beside `retryAccounting.disposition`. A
  `REVIEW_RETURNED` release MAY target only `implement`. A reason without a target, a target on any
  other release, an empty value, an unknown stage or reason, the flags on any other verb, and a
  reason inconsistent with the generation's stage under CAL-V0-083 MUST refuse `MALFORMED` without
  ending the generation. When a later CLAIM or CLAIM_NEXT admits the next generation, the ended
  generation's recorded keys MUST move into its `priorGenerations[]` entry, which then MUST also
  carry the V1-0788 prior-generation history keys with a non-null `stage`. The closed decoders MUST
  refuse the keys on a live or non-clean generation, a prior entry without a recorded stage, and
  any combination this requirement or CAL-V0-083 forbids.
- `CAL-V0-083`: The `--handoff-reason` set is closed: `CHANGES_REQUESTED` returns review or
  integrate work to `implement` and is refused from an implement generation; `STAGE_INCOMPLETE`
  hands off to the generation's own stage; `STAGE_COMPLETE` hands off to a different stage. The
  reason is optional, advisory metadata; `release --help` MUST list the targets
  (`handoffTargets`) and the reasons (`handoffReasonCodes`).
- `CAL-V0-084`: `ticket show`, each `plan preview` entry and the dispatcher's native ticket
  observation MUST expose `nextStage`, derived only from the ticket's highest-generation attempt:
  the recorded `handoffTo` of a clean terminal generation; `implement` for a `REVIEW_RETURNED`
  generation without one; and `null` (`NONE` in the dispatcher) when there is no attempt, the
  generation is live, its disposition is not clean, a `HANDOFF` recorded no target, or the journal
  is absent. When that generation's `ticketRevision` differs from the ticket's current
  `acceptanceRevision`, `nextStage` MUST be `STALE` instead of a stage. The dispatcher MUST render
  it through the `{nextStage}` launch placeholder and record it as `nextStage` in the `launched`
  event detail. It is advisory: no claim, plan selection, role match or launch is refused,
  filtered or reordered by it, and the reads write nothing.
- `CAL-V0-085`: A release without the new flags MUST keep its exact request preimage, replay and
  attempt bytes; the `REVIEW_RETURNED` default is derived at read time and never written. An
  attempt or prior-generation entry without the keys MUST decode and re-encode byte-identical, and
  retry accounting MUST be unchanged: a recorded target neither charges nor refunds a retry, and
  the CAL-V0-044 tests remain the regression witnesses.

Non-goals: enforcing the target on a later claim or refusing a claim for another stage; a
ticket-record key; routing loop detection over the recorded history (later parts of issue 587);
recording a target on supervised or non-clean releases; and treating the target or reason as proof
of review, approval or authority. They are inert caller statements.

Failure modes: a target recorded before an acceptance change reads `STALE`, never a stage; a target
recorded by a generation that is then superseded by a live claim reads `null` until that claim ends;
a `HANDOFF` that a fenced, stale-policy or ineligible check refuses records nothing, because the
target is checked only after the clean-handoff verification passes; and the dispatcher's
`nextStage` is the one observed at launch, which may change before the worker reads it.

Rollback and downgrade: earlier binaries' decoders are closed and refuse `handoffTo` and
`handoffReason`, at the top level or in a `priorGenerations[]` entry, as `MALFORMED`. Once a
release records a target, an older binary cannot read that attempt or audit a store containing it.
Reverting the code is safe only while no attempt record holds the keys (check each
`<stateDir>/attempts/*.json` with
`jq -e 'has("handoffTo") or ([.priorGenerations[] | has("handoffTo")] | any)'`). Once one does,
keep this version; there is no supported rewrite of attempt records, and editing them by hand
breaks receipt-bound audit. Releases that do not pass the flags write nothing new, so a store that
never used them remains readable by the previous binary. The `nextStage` read fields, help keys and
dispatcher placeholder are read-side only and revert with the code.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-082 | `TestCALV0082_RecordedHandoffRoundTrip`, `TestCALV0082_MalformedHandoffTargets`, `TestCALV0082_PriorGenerationHandoff` (`internal/tasks/snapshot`); `TestCALV0082_HandoffTargetPreimage` (`internal/tasks/transaction`); `TestCALV0084_CLIHandoffTargetNextStage` (`internal/tasks/cli`) |
| CAL-V0-083 | `TestCALV0083_ClosedHandoffReasons`, `TestCALV0082_MalformedHandoffTargets` (`internal/tasks/snapshot`); `TestCALV0083_ReleaseHelpListsHandoffCodes` (`internal/tasks/cli`) |
| CAL-V0-084 | `TestCALV0084_NextStageDerivation` (`internal/tasks/transaction`); `TestCALV0084_CLIHandoffTargetNextStage` (`internal/tasks/cli`); `TestCALV0084_DispatcherNextStage` (`internal/tasks/dispatch`) |
| CAL-V0-085 | `TestCALV0082_UntargetedHandoffRoundTrip` (`internal/tasks/snapshot`); `TestCALV0082_HandoffTargetPreimage`, `TestCALV0046_ReleasePreimageCompatibility` (`internal/tasks/transaction`); `TestCALV0044_AccountingSchema` (`internal/tasks/snapshot`), `TestCALV0044_CleanHandoffsPreserveRetryDebt` (`internal/tasks/store`), `TestCALV0044_CLIHandoffAccounting` (`internal/tasks/cli`) |

### S24 — Resource-aware default selection (issue 584)

Human-owned input: [issue 584](https://github.com/beamfall/corvint/issues/584) reports that
`SELECTED` tickets waiting for a pool member fill the `maxActiveAttempts` window while lane-free
work defers `LIMIT_EXCEEDED`. The owner chose option (a) in the owner answer 2026-10-05 (D1): cap
pool-needing selections at the pool's currently free eligible members, defer the excess with the
existing `RESOURCE_COLLISION` code outside the window, add no policy setting, and report an
additive per-pool resource-deferred count in the plan.

- `CAL-V0-097`: In the default plan (no `--pool`), a ticket whose `requiresPool` names a pool the
  policy declares MUST NOT be `BLOCKED` merely because the plan ran without `--pool`. Taken in plan
  order, it is `SELECTED` only while the earlier selections requiring the same pool are fewer than
  that pool's free eligible members. Otherwise it is `DEFERRED RESOURCE_COLLISION` with blockers
  `[<pool>]` and does not count against `maxActiveAttempts`. Free eligible members use CAL-V0-029's
  ordered eligibility for the plan's `--stage` (none: unreserved members only) and the occupancy in
  `pools.json`. When that occupancy is unobservable, availability is `NOT_OBSERVED` and the ticket
  defers; it is never selected on an assumption. A store audit that proves `pools.json` absent is
  the empty occupancy a claim reads. A `requiresPool` the policy does not declare stays `BLOCKED
  RESOURCE_COLLISION`. A default plan whose policy declares pools MUST add a top-level
  `resourceDeferred` array, one row per policy pool in policy order:
  `{poolId, availability: OBSERVED|NOT_OBSERVED, freeEligibleMembers: count|null, selected, deferred}`.
  A `--pool` plan, a policy without pools and the closed `--selected-only` projection omit it.
  Parity with CAL-V0-008 and CAL-V0-029: `claim --next` without `--pool` claims the first
  `SELECTED` entry that requires no pool; a `SELECTED` pool entry is claimed by an explicit claim or
  `claim --next` naming its pool. The `--pool`/`--stage` plan, its cap and CAL-V0-065 exclusions
  are unchanged. Plan preview stays a pure read with no lock, write or probe (CAL-V0-034).
  `ticket show` claimability follows the same rule: `null` with reason `NOT_OBSERVED` when the
  member state is unobservable. The continuous dispatcher (CAL-V0-052..058) MUST carry each
  ticket's `requiresPool` into its ticket view and route a ticket that requires pool P only to a
  ticket role whose `match.pool` is P, binding `{pool}` to P for that worker; a role naming a pool
  matches only that pool's tickets, and a role without one only tickets that require no pool. The
  dispatcher's own plan MUST treat a pool that no ticket role names as unclaimable: its tickets
  defer `RESOURCE_COLLISION` naming the pool before they count against `maxActiveAttempts`, so a
  pool ticket no worker can claim never takes the window from lane-free work and never accrues a
  refused claim, cooldown or parking on a role that cannot claim it. Core's `taskman-plan/0`
  decoder MUST accept the optional `resourceDeferred` rows and a pool blocker on a `DEFERRED
  RESOURCE_COLLISION` entry only when that pool has a row.

Non-goals: a reserved per-class window or policy setting (option b); inferring a pool for tickets
that do not declare `requiresPool` (V1-0754, V1-0758, V1-0759); stage-aware member counting when the
default plan has no `--stage`; probing member health; changing dispatcher launch order. Failure
modes: a pool-waiting ticket without `requiresPool` still fills the window, so operators must record
`requiresPool` (see `docs/TASKS-EXTERNAL-AGENTS.md`); a pool whose members are all reserved for
stages selects nothing in a stage-less default plan, which defers rather than over-selects; a
`SELECTED` pool ticket that no caller claims with its pool can still hold a window slot in the
preview and in `claim --next`'s plan, bounded by the pool's free members (an explicit claim checks
only live reservations, and the dispatcher plans such a pool as unclaimable); a dispatcher role
whose host claims without `--pool {pool}` but names `match.pool` is refused and counts toward
cooldown and parking (CAL-V0-057), which is a configuration error the dispatcher does not detect;
occupancy that changes after the preview is rechecked by the claim transaction. Rollback reverts
the planner, the `claim --next` filter, the `resourceDeferred` member, the dispatcher's pool
routing and `match.pool`, and the Core decoder rows together; no store, journal or wire state
depends on them, and pool tickets return to `BLOCKED` in the default plan.

### V1-0781 — Preparation admission pressure (issue 494 follow-up)

Intent, non-goals, failure modes, acceptance evidence and rollback are in the V1-0781 amendment
section below.

- `CAL-V0-095`: `queue status` MUST add one `preparationAdmission` object to its item, observed
  after and outside the store snapshot. The object names the fixed `capacity` of `64`.
  `registeredWriters` is the number of live registrations with a valid published record. This
  count includes any serving holder, whose slot stays live until it closes. `unpublishedSlots`
  counts live slots without a readable valid record. `wouldBeRank` is the rank a registration
  published now would receive: the largest observed live rank plus one, or `1` when none is live.
  The same rank appears in a `LOCK_TIMEOUT` diagnostic. `registryActive` reports a registry lock
  held at the start or end of the sweep. `snapshot` is `RACY`, and `method` names the lock query.
  A slot is live only when a lock query that acquires nothing observes a `flock(2)` owner on it,
  the preparation protocol's only lock. Darwin uses `fcntl(F_GETLK)`, which reports a `flock`
  owner with `l_pid` -1. Because it names only the first conflicting lock, a POSIX record lock on
  a coordination file MUST make the read abstain. Linux reads one bounded `/proc/locks` snapshot,
  keeps only granted `FLOCK` entries and matches the slot's device and inode. POSIX and OFD record
  locks are ignored there, because on local Linux filesystems they neither conflict with `flock`
  nor mark a registration. The Linux table omits owners outside the procfs PID namespace, so it
  MUST be used only when the reader's PID namespace is the initial one and the reader is visible
  in that procfs mount; otherwise the read abstains. A slot without an observed owner is stale
  scheduling bytes and is not counted. The read MUST NOT register, `flock`, create, truncate or
  write any coordination, journal or intent file. If the lock query, an incomplete lock table, an
  unsafe object, a file that disappears or is replaced after its first stat (the read
  revalidates each pathname against its opened descriptor after the lock query, and the
  common-directory pathname against its pinned root, without a symlink, before it reports), or the common
  directory prevents observation, then `snapshot`, `method` and every count are `NOT_OBSERVED`,
  `notObservedReason` names the cause and `registryActive` is null. A file absent at its first
  stat is absent. A live slot without a valid record also makes `wouldBeRank` `NOT_OBSERVED`. The
  store records no per-mutation writer cost, so `estimatedWait` is `NOT_OBSERVED` with
  `estimatedWaitBasis` `no recorded per-mutation writer cost`. Any future estimate MUST derive
  from recorded cost, name its basis and never be presented as a guarantee.

### V1-0787 stage-scoped execution prerequisites amendment

Authoritative input: owner request [issue 585](https://github.com/beamfall/corvint/issues/585) and
native ticket V1-0787, with owner decision D4: the new refusal is the closed code
`PREREQUISITE_UNSATISFIED`, and the key uses the same byte-sorted canonical ordering and validation
as `dependencies`. A dependency gates the whole ticket and takes part in cycle detection and
completion. Some orderings matter only to one stage. For example, a ticket may be implemented at once
but must not be integrated before another lands. This amendment adds one optional record key for
such orderings. A prerequisite blocks claims and plans for the stages it lists and nothing else.

- `CAL-V0-099`: A ticket record MAY carry the optional key `executionPrerequisites`. Its value is an
  array of closed objects `{ticketId, obligation: "COMPLETED"|"GATE_PASSED", gateId:
  Label|null, stages: [Stage]}`, where `Stage` is `implement`, `review` or `integrate`.
  1. Shape. The key is omitted when the set is empty. An empty array MUST be refused `MALFORMED`.
     The array MUST hold at most 64 entries (`LIMIT_EXCEEDED`), the `dependencies` bound. It MUST be
     a canonical-byte-sorted, duplicate-free set. Each `stages` MUST be a non-empty, sorted,
     duplicate-free set of known stages. `gateId` MUST be non-null exactly when the obligation is
     `GATE_PASSED`. A strict decoder refuses any violation `MALFORMED`. The CLI input boundary
     sorts both sets, as it does for `dependencies`.
  2. Edges. A prerequisite MUST name a ticket in the same queue (`DEPENDENCY_MISSING`) and never the
     ticket itself (`MALFORMED`). At most one entry may exist per `(ticketId, obligation, gateId)`
     (`DUPLICATE_ID`). Writers additionally refuse two cases: a ticket absent from the store or
     export (`DEPENDENCY_MISSING`), and a gate the policy does not declare (`GATE_UNKNOWN`). Those
     writers are `REFINE`, `ADOPT_FILE` and the importer.
  3. Isolation. The key is excluded from cycle detection, completion checks and `requiredGates`. It
     is not acceptance-relevant: changing it bumps `revision` but not `acceptanceRevision`, and is
     allowed under a live attempt. Completing or gating the prerequisite ticket writes only that
     ticket's own record. Legacy records without the key round-trip byte-identically.
  4. Stage scope. The ticket view evaluates a prerequisite when the read's stage is listed in its
     `stages`. A stageless read evaluates every prerequisite (fail closed). Stageless reads are
     `claim`, `claim-next` or `plan` without `--stage`, plus `ticket show` and `ticket blockers`.
     - An unmet `COMPLETED` obligation is a blocker with code `PREREQUISITE_UNSATISFIED`. It is met
       by `COMPLETED`, or by `ARCHIVED` from `COMPLETED`. A missing prerequisite ticket is also a
       blocker.
     - A `GATE_PASSED` obligation follows the dependency gate rule in `view.go`. An observed failure
       is a blocker. An unobservable gate is an unknown with the same code and the result
       `NOT_OBSERVED`, which leaves eligibility `UNKNOWN`. The native reader has no gate oracle, so
       today it always reports this unknown.
     - Each blocker or unknown names the prerequisite ticket. Its detail names the stages it gates.
       The next action is `wait-dependency` for a blocker and also for an unknown, because
       admission refuses both; the view never recommends `admit` while a prerequisite is unknown.
       The view echoes `executionPrerequisites` so that `ticket blockers` explains the block.
     - `claim` and `claim-next` refuse `BLOCKED/PREREQUISITE_UNSATISFIED` on such a blocker or
       unknown. When `claim-next` selects nothing and its first planned entry is blocked by
       prerequisites, the refusal detail names each prerequisite ticket and its stages. `plan`
       reports the entry `BLOCKED` with that reason and the prerequisite ticket. Recorded
       claimability is `false` for a blocker and `null` for an unknown.
     - A claim or plan for an unlisted stage is unaffected.
  5. Writers. `ticket refine` sets the key with an array and clears it with `null`. `ADOPT_FILE`
     treats it as a routine field composed through `REFINE`. An import export item MAY carry the
     key; when a re-import omits it, the record's existing set is kept. `CREATE` does not accept
     the key.
  6. Core. The Core decoder (`internal/taskman`) admits the key under the same shape and edge
     rules: each `ticketId` is a well-formed ticket ID, in the record's queue and not the record
     itself, and each `(ticketId, obligation, gateId)` appears once. Both readers MUST refuse every
     case in the shared fixture `cal-v0-099-prerequisite-refusals.json`. The Core planner is
     stageless. It reports `GATE_UNKNOWN` for a `GATE_PASSED` prerequisite and
     `PREREQUISITE_UNSATISFIED` for a missing or uncompleted `COMPLETED` prerequisite.
  7. Code. `PREREQUISITE_UNSATISFIED` joins TCP-00 §11's closed detail codes (A21).

Non-goals:
- prerequisites on other queues or repositories;
- a gate oracle for native reads;
- per-stage dependencies in `CREATE`;
- automatic re-planning or notification when a prerequisite completes;
- any change to `dependencies`, cycle detection or completion.

Failure modes:
- A prerequisite on the wrong stage silently gates nothing for the intended stage. The view and
  `ticket blockers` echo the set and name the stages, so the error is visible on a stageless read.
- A prerequisite ticket is later deleted or moved. The read reports a blocker, not a pass.
- Without a gate oracle, a `GATE_PASSED` prerequisite keeps listed-stage claims at `UNKNOWN`, and
  they are refused until the record changes. This is deliberate: an unobservable gate is never
  assumed to have passed.
- A caller sends an unsorted set directly to the strict envelope decoder. It is refused
  `MALFORMED`, and the CLI boundary sorts it first.

Acceptance evidence is listed in the table below. Focused package tests ran on the change; live
store qualification is `NOT_RUN`.

Rollback: once any record has carried `executionPrerequisites`, clearing current records is not a
rollback. Older binaries decode ticket records strictly with a closed key set and do not know the
`PREREQUISITE_UNSATISFIED` code. `ticket refine` with `"executionPrerequisites": null` changes only
the current record. The journal keeps every earlier ticket afterimage, and journal audit and replay
decode each historical ticket record through the strict ticket decoder
(`internal/tasks/journal/records.go`). An older binary therefore still refuses the store after the
key is cleared. Exports that carry the key are refused the same way. Rollback takes one of two
routes:

- Keep a compatible reader: this binary or a later one that accepts the key. Clearing the key then
  only stops the gating.
- Restore the whole store, intent and state directory together, from a backup taken before the key
  was first written. Verify it with `receipt audit` under the older binary before resuming. Every
  later transaction is lost.

To tell whether the key was ever written, search the store for the byte string
`"executionPrerequisites"`. Search the tracked ticket records and the state directory
(`<git common dir>/taskman`), including journal receipts, which hold inline post records, and
`evidence/` blobs. Ticket records are canonical JSON, so every record that ever carried the key
contains the string. No match means older binaries still read the store. Records never written
with the key are byte-identical to legacy records, and no other store, journal or wire state
depends on this amendment.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-099 | `TestCALV0099_RecordKeyRoundTrip`, `TestCALV0099_RecordKeyRefusals`, `TestCALV0099_SharedRefusals`, `TestCALV0099_StageScopedView`, `TestIssue502_RecordEscalationsKey` (`internal/tasks/ticket`); `TestCALV0099_RefineSetsAndClearsPrerequisites`, `TestCALV0099_CompletingPrerequisiteChangesNoOtherRecord`, `TestCALV0099_AdoptComposesPrerequisites` (`internal/tasks/mutation`); `TestCALV0099_ImportAcceptsAndPreservesPrerequisites` (`internal/tasks/importer`); `TestCALV0099_ClaimAndPlanAreStageScoped`, `TestCALV0099_ClaimNextRefusalNamesPrerequisite` (`internal/tasks/transaction`); `TestCALV0099_CoreReaderValidatesExecutionPrerequisites`, `TestCALV0099_CoreSharedRefusals`, `TestCALV0099_CorePlannerAppliesEveryPrerequisite` (`internal/taskman`); `TestTMV0002_AS01_CommandResultEnvelope` (`internal/tasks/wire`, 73 closed codes) |

### V1-0789 — Exclude implement authors (issue 586 part 2)

Human-owned input: [issue 586](https://github.com/beamfall/corvint/issues/586) part 2 and ticket
V1-0789, with owner decisions: by default exclude only the most recent implement generation's member
(D5); offer `--exclude-authors=all` for every recorded implement generation, under which any
`NOT_OBSERVED` generation in scope also refuses; no new codes; no backfill of legacy member facts.
It builds on the V1-0788 prior-generation history and on CAL-V0-065 explicit exclusions.

- `CAL-V0-098`: CLAIM, CLAIM_NEXT and read-only plan preview MAY accept an opt-in author-exclusion mode, `LATEST` (CLI `--exclude-authors`) or `ALL` (CLI `--exclude-authors=all`). A supplied mode MUST require an explicit pool and stage `review` or `integrate`; any other stage, an absent pool, another mode value, a repeated CLI flag or another verb MUST refuse as malformed, except that the internal POOL_PREPARE for such a claim MUST carry the mode together with the derived ticket and refuses either alone, and carries the claim's explicit members only together with the mode. Canonical request preimages MUST bind the mode as `excludeAuthors` and MUST omit the field when absent, preserving historical bytes; the derived member set is not part of the request, so replay precedes derivation, an identical request replays its original allocation, and a changed mode under the same request ID MUST conflict with `REQUEST_ID_CONFLICT`. On fresh admission the derivation MUST read every generation of the ticket's attempts, prior generations from the V1-0788 prior-generation history and each attempt's current generation as that history would record it once ended, newest generation first. It MUST skip a generation recorded at stage `review` or `integrate`; every other generation it reaches MUST be an `implement` generation with a recorded pool member, which is an author. `LATEST` MUST stop at the first author; `ALL` MUST read every generation. A reached generation whose history is `NOT_OBSERVED`, one with no recorded stage, or an implement generation without a pool member MUST refuse with `INDEPENDENCE_UNVERIFIED` and a detail naming the generation, before any member selection or health preparation, unless CAL-V0-107 covers it; a ticket with no implement generation has nothing to exclude (CAL-V0-107); the claim is never silently unfiltered and no member is inferred from receipts or any other source. The effective exclusion set MUST be the union of explicit `--exclude-member` labels and the authors' members recorded in the requested pool, and the CAL-V0-065 predicate MUST apply it to allocation, every health-selection round, final prepared admission and preview capacity. POOL_PREPARE MUST rederive the authors from the snapshot it commits against and MUST refuse, before recording PREPARING and so before any health command runs, a member that derivation excludes or a derivation that is unverified under the claim's own CAL-V0-107 cover; the claim is then reevaluated with a fresh derivation. When no member remains, the claim MUST refuse `RESOURCE_COLLISION` with a detail naming each excluded author's member, pool and generation. Plan preview, and so CLAIM_NEXT, MUST evaluate the same derivation per ticket before pool capacity, as a named claim does, reporting the same refusal code and detail as a blocker even when the pool is exhausted, and, only when the mode is supplied, MUST add `detail` (string or null) and `excludedAuthors` (array of `{attemptId, generation, poolId, memberId}`, or null when unverified) to each entry.

Non-goals: this is not actor authentication, not an independence proof, and not a statement that two
member labels name physically distinct environments (the issue 480 ERG non-goal stands); no backfill
or inference of legacy member facts; no per-pool `independentStages` policy; no new result codes; no
exclusion by holder label; no change to implement-stage claims.

Failure modes: a legacy or supervised generation (`NOT_OBSERVED`), a stage-less claim that may have
implemented the ticket, or an implement claim outside any pool refuses `INDEPENDENCE_UNVERIFIED`
unless explicit exclusions cover it (CAL-V0-107, which also admits a ticket never implemented); an author in another pool contributes no exclusion to the requested pool;
an exhausted pool refuses `RESOURCE_COLLISION` naming the authors; a later policy change cannot
change a replayed allocation; a caller who reads the exclusion as proof of independence is told
otherwise by the documented non-goal.

Limits: multi-selection preview counts global pool slots with explicit exclusions only, and catches
a ticket whose own derived set leaves zero slots as a per-ticket blocker; CLAIM_NEXT uses only its
first selected entry, so its admission is exact. Scope covers every attempt of the ticket across
acceptance revisions.

Acceptance evidence: focused tests `TestCALV0098_PreimageBindsMode`, `TestCALV0098_RequestShape`,
`TestCALV0098_DeriveAuthors` (`internal/tasks/transaction`);
`TestCALV0098_ClaimExcludesImplementAuthor`, `TestCALV0098_ClaimNextExcludesAuthors`,
`TestCALV0098_HealthSkipsAuthor`, `TestCALV0098_HealthPrepareRederivesAuthors` (`internal/tasks/store`);
`TestCALV0098_CLIExcludeAuthors`, `TestCALV0098_ExhaustedPoolParity` (`internal/tasks/cli`); `TestAgentLeasesSpecEnumeratesCALV0098` (`internal/lrfrepo`). Live multi-agent qualification is `NOT_RUN`; see
`docs/build-log/2026-10-05-v1-0789-exclude-authors.md`.

Rollback: revert the code. The mode is opt-in and absent from every historical preimage; attempt
and pool records gain no field, so no stored state migrates. A request that used the mode retains
`excludeAuthors` in its bound preimage; a binary without this slice refuses the mode, and how it
treats such retained requests on replay or audit is `NOT_OBSERVED`.

### V1-0784 priority-yield admission amendment (issue 583)

Human-owned input: [issue 583](https://github.com/beamfall/corvint/issues/583) (native ticket
V1-0784) reports that an explicit `claim <ticket> --pool P` takes P's last free member even when a
higher-priority ticket that needs P is waiting, so the lower-priority ticket starves the
higher-priority one. The owner directed an opt-in, derived priority yield for explicit pooled
claims with no new state, no new result code and no waitlist (V1-0785 stays separate).

- `CAL-V0-101`: A policy pool MAY set the optional boolean `priorityAdmission`; omission and
  `false` mean today's admission. When it is `true`, an explicit `claim <ticket> --pool P` MUST be
  refused `BLOCKED RESOURCE_COLLISION` when P has at least one free eligible member and the
  competing tickets are at least as many as those free members. Free eligible members are the
  claim's own CAL-V0-029 ordered eligibility for its `--stage`, its CAL-V0-065 exclusions and the
  occupancy in `pools.json`; a prepared allocation of the claim counts as free. A competing ticket
  is `OPEN`, precedes the claimed ticket in admission order (CAL-V0-108; plan order, that is
  priority, order and ticket ID, when no ticket waits for a downstream stage), records
  `requiresPool` P, and has no claim blocker, which includes a live attempt (`ATTEMPT_LIVE`). The
  detail names the pool, the competitor and free counts, and the first competitor in admission order
  (`yields to <ticketId>`). The check runs after the claimed ticket's own state and live-attempt
  checks and before scope collision, capacity, retry, health preparation and member allocation, so
  a yielded claim posts nothing, prepares no member and runs no command. A claim without `--pool`,
  on a pool without the flag, or with no free eligible member is unaffected. Plan preview MUST
  apply the same rule: with `--pool P`, and in the default plan for an entry that
  requires P, an entry the rule would refuse is `DEFERRED RESOURCE_COLLISION` with blockers
  `[<ticketId yielded to>]` before the pool cap, outside `maxActiveAttempts`, and the default
  plan's `resourceDeferred` row counts it as deferred. `claim --next` claims a `SELECTED` entry of
  that plan, so it never claims a ticket an explicit claim would refuse. A competitor whose only
  blockers are unobservable (`NOT_OBSERVED`, for example a `GATE_PASSED` dependency) MUST NOT cause
  a refusal or a deferral; it stays its own `BLOCKED` plan entry, and `ticket show` reports the
  claimed ticket's claimability as `null` with reason `NOT_OBSERVED` when counting it would yield.
  The rule is derived from the plan inputs on every read and claim; it adds no store, journal or
  wire state, and plan preview stays a pure read with no lock, write or probe (CAL-V0-034).

Non-goals: a durable waitlist, reservation hand-off or re-claim grace for a competitor that is
briefly blocked or unobservable (V1-0785); new result or detail codes; changing `claim --next`,
pool health or cleanup; the supervisor's stage-transition member allocation, which is not a claim;
aging or fairness between equal-priority tickets beyond plan order and the CAL-V0-108 downstream
rank; any Core change (Core reads only
`policySha256`, and its `taskman-plan/0` decoder already admits a ticket ID blocker on a `DEFERRED
RESOURCE_COLLISION` entry). Failure modes: a competitor that collides on scope with a live
reservation is still claim-eligible and still counts, so a lower-priority ticket can yield to a
ticket that cannot run yet; a competitor that becomes blocked or unobservable stops counting at the
next read, so its member can be taken (V1-0785); an operator who wants a lower-priority claim to
proceed must claim the competitor, change priorities or drop the flag; a claim is checked before
health preparation, but if a competitor becomes eligible between preparation and the re-claim, the
re-claim yields and the existing observation path leaves the prepared member `QUARANTINED` until
operator confirmation, as for any other failed re-claim; a non-boolean value refuses the policy `MALFORMED`. Acceptance evidence:
`TestCALV0101_ExplicitPooledClaimYields`, `TestCALV0101_FlagOffMatchesNMinusOne` (absent flag pinned
to the pre-change transcript digest; `false` equal modulo policy identity),
`TestCALV0101_PlanClaimAndClaimNextAgree` (400-case property test), and
`TestCALV0101_UnobservedCompetitorIsNotObserved` (`internal/tasks/transaction`);
`TestCALV0101_PriorityYieldThroughTheCLI` (`internal/tasks/cli`).

Rollback: disabling admission and downgrading are different operations. To disable priority yield,
remove the key or set it to `false` with `policy update` on this binary or a later one; admission
returns to the pre-amendment rule at the next read or claim. Once any policy has carried
`priorityAdmission`, removing it is not a downgrade. Older binaries decode the policy strictly with
a closed pool key set and refuse the key, even `false`, as `MALFORMED`. `policy update` changes
only the current record. The journal keeps every earlier policy afterimage, and journal audit and
replay validate each historical post, decoding every `intent/policy.json` record through the strict
policy decoder (`internal/tasks/journal/records.go`). An older binary therefore still refuses the
store, including `receipt audit`, after the key is removed. Exports that carry the key are refused
the same way. A downgrade takes one of two routes. Keep a compatible reader: this binary or a later
one that accepts the key; removing the key then only stops the yield. Or restore the whole store,
intent and state directory together, from a backup taken before the key was first written, verify
it with `receipt audit` under the older binary before resuming, and accept that every later
transaction is lost. To tell whether the key was ever written, search the store for the byte string
`"priorityAdmission"`: the tracked policy record and the state directory
(`<git common dir>/taskman`), including journal receipts, which hold inline post records, and
`evidence/` blobs. Policy records are canonical JSON, so every record that ever carried the key
contains the string. No match means older binaries still read the store. A policy never written with
the key is byte-identical to a legacy policy, and no other store, journal, receipt or pool state
depends on this amendment; refusals already recorded keep their existing `RESOURCE_COLLISION` code.

### V1-0780 retryable command results

Source, classification table, limits and evidence: the V1-0780 retryable result amendment below.

- `CAL-V0-078`: Tasks MUST classify every §11 detail code in `internal/tasks/wire/codes.go` as
  retryable or not, with a stated condition, in one table (`wire.RetryOf`). A code is retryable
  only when every in-tree producer reports it before anything was decided or written, and its
  usual cause is a concurrent writer, so that reissuing the same command with the same
  `--request-id` after a bounded backoff can succeed once that writer finishes; the producers
  whose cause is the caller's own stale input are named below and repeat until it changes. The retryable set is exactly `LOCK_TIMEOUT` (the store lock or a
  preparation admission was held past the wait budget), `SNAPSHOT_MOVED` (the store, head, intent
  tree or worktree moved during a read or before commit) and `REDO_PENDING` (a writer sits between
  receipt link-in and head rename; one that outlives the budget is redone by the next mutating
  command). Fencing codes (`FENCED`, `BOOT_FENCED`, `SUPERVISOR_LOST`) MUST never be retryable,
  and a code whose producers are mixed, uncertain or absent MUST be classified not retryable, with
  the reason recorded. Every `taskman-command-result/0` envelope whose outcome is not `OK` and
  whose `codes` is non-empty MUST carry `retryable`, true only when every code is retryable. The
  commands that run a program a retry would run again MUST report false whatever their codes:
  `attempt run` once its child has run, `gate run` once its gate program has started, and a
  supervised `run --role` once its stage dispatch has committed (including a failed refresh or
  program record afterwards), because a repeated run no longer selects the attempt. This lasts
  until a committed `STOPPED` is read back in a phase the role selects again, as integration's
  `READY_FOR_INTEGRATION` is. A `GRANT` and integration recovery leave the attempt selectable, so
  their failures keep their codes' classification. A batch with `--count` reports false when any
  failed lane is not retryable, by its mark or its code. A
  command that commits a first step it may then fail to finish MUST also report false, because a
  same-request retry replays that step without finishing it: `health` and `pool cleanup` once
  their preparation or cleanup receipt commits, and `pool sweep` once a fresh sweep commits its
  owner. `OK` and uncoded results MUST NOT carry
  the member. A decoder MUST accept a coded envelope without the member, and MUST refuse
  `MALFORMED` a member on an `OK` or uncoded result, a non-boolean value, or `true` beside a code
  that is not retryable.

### V1-0791 no-progress loop detection amendment (issue 587 part 2)

Human-owned input: owner request [issue 587](https://github.com/beamfall/corvint/issues/587)
part 2, native ticket V1-0791 and owner decision D8. Routing loops that hand a ticket back and
forth without changing anything consumed about 26 sessions on one ticket, and no read could see the
loop. The owner asked for an opt-in derived hold computed from audited attempt history, shaped like
ESC-V0-006, with byte-identical behaviour while the policy does not opt in. This amendment adds an
optional policy key, an optional prior-generation key and one closed detail code (A23).

- `CAL-V0-102`: A policy MAY carry the optional top-level key `loopDetection`, a closed object
  with the required counts `maxNoProgressGenerations` and `maxAlternatingReturns`, each 1..256; an
  unknown or missing member, a zero, an out-of-range or non-count value, `null` and `{}` refuse
  the policy `MALFORMED`. Omission keeps today's canonical policy bytes and behaviour. While the
  policy carries the key, a CLAIM or CLAIM_NEXT that admits the next generation of an ended
  attempt MUST record the ended generation's work evidence as the optional `priorGenerations[]`
  key `loopEvidence {disposition, candidateTreeOid, gateResults, reviews}`: the retry-accounting
  disposition (`HANDOFF`, `REVIEW_RETURNED`, or `NONE` for any other end), the submitted candidate
  tree or `null`, and the number of gate results and external reviews it recorded. The key needs
  the V1-0788 recorded history, and the closed decoder refuses it without that history, with a
  `NONE` disposition beside a recorded `handoffTo`, or as `REVIEW_RETURNED` on a non-review stage.
  A claim under a policy without the key writes no `loopEvidence`. The `LOOP_DETECTED` hold is
  derived on every read and claim, only from the ticket's newest attempt when it is ended, not
  `COMPLETED`, and bound to the ticket's current `acceptanceRevision`; its generations are the
  prior entries, oldest first, then the ended current generation, whose evidence is the attempt
  itself. Two signals count. *No progress*: the newest consecutive generations that each ended in
  a clean `HANDOFF` with zero gate results, zero reviews and no new candidate tree (none
  submitted, or the same tree as the latest earlier one); a hand-off that carries `--evidence` but
  no tree still counts. *Alternating returns*: the newest consecutive pairs of an `implement`
  `HANDOFF` (untargeted or targeting `review`) followed by a `review` `REVIEW_RETURNED`, with a
  trailing implement hand-off counted in the run. When a run's length (no progress) or its number
  of returns (alternating) is above the policy bound, the ticket is held. A prior entry without
  recorded history or evidence (legacy, supervised, or written while the policy was absent) is
  UNKNOWN: it never counts, it ends the run it interrupts, and a later tree cannot be compared
  across it. The hold MUST appear as a `LOOP_DETECTED` blocker in native eligibility, `ticket
  show`, `ticket blockers`, direct claim, claim-next, recorded claimability, `plan preview`
  (`BLOCKED LOOP_DETECTED`), the dispatcher's native ticket observation (the roster skips the
  ticket) and `dispatch status` (`loopDetected`), with next action `reopen`. Its detail names the
  signal, the acceptance revision, the counted generations, which are its evidence, and the policy
  bound. Every plan entry so blocked, with or without author exclusion, MUST carry the optional
  member `loop {signal, acceptanceRevision, generations, limit}`, absent from every other entry,
  and a claim-next refusal whose first entry is `LOOP_DETECTED` MUST append the same detail. Like
  ESC-V0-006 it never rewrites ticket status, sets a ticket hold or cancels anything,
  and plan, eligibility and status reads stay pure: no lock, probe or write (CAL-V0-034).
- `CAL-V0-103`: The hold MUST clear when the ticket's `acceptanceRevision` changes, which makes
  the held attempt stale, or when the owner acknowledges it with the existing `ticket reopen`
  writer (CAL-V0-043), which bumps the acceptance revision and readmits the ticket with a fresh
  attempt. Reopen MUST admit a loop-held ticket as it admits a retry-exhausted one; a ticket that
  is neither still refuses `RETRY_BUDGET_NOT_EXHAUSTED`. The dispatcher MUST raise one typed #502
  `needs-owner` event with detail
  `{kind: blocked, code: LOOP_DETECTED, signal, acceptanceRevision, generations}` per episode, an
  episode being the ticket, signal, acceptance revision and newest counted generation, and persist
  the episode in the optional dispatch-ledger member `seen.loops` so that a restart does not raise
  it again; a cleared hold drops the member and a recurrence raises again. An episode whose event
  append fails MUST stay recorded with `pending: true` and be raised again on the next tick or after
  a restart until the append succeeds. Before raising an episode, the dispatcher MUST skip the
  append and record the episode when the readable tail of the current or rotated event log already
  holds that episode's event, so an append whose line landed before its error, or a crash between
  the append and the ledger save, raises no duplicate. Every event append MUST first end a trailing
  unterminated fragment, which a part-way failed append can leave, with a newline, so the appended
  line parses; a well-formed log receives exactly the line. The ledger reader is closed whenever
  `seen.loops` is present, with or without progress members, and refuses an alias, a duplicate or
  unknown member, a `null` map or hold, an unknown signal, a malformed acceptance revision, a
  malformed or empty generation list, a `pending` other than `true`, or a bad ticket key. A ledger
  without holds keeps its bytes.

Non-goals: a new acknowledgement verb or a lighter operator acknowledgement (reopen is reused);
a native escalation record (the ESC-V0-010 writer opens a question only under a live claim, so the
dispatcher's escalation is an event, not a record); counting supervised generations, text,
session cost or time; holding on any other signal; changing ticket status, retry accounting or the
retry budget; evaluating the hold in `ticket list`, `roadmap` or `queue status`; and any Core
change. Core's closed policy reader already refuses a policy carrying `loopDetection`, as it does
for `pools`, `supervision` and `externalReviews`, and Core does not evaluate the hold.

Owner decision (2026-10-05, ticket V1-0791): both open questions keep the delivered behavior.
The owner's `ticket reopen` stays the only acknowledgement, with no lighter operator verb, and the
loop escalation stays a dispatcher `needs-owner` event, with no dispatcher- or operator-origin OPEN
that would relax the ESC-V0-010 live-claim requirement.

Failure modes: history recorded before the policy opted in is UNKNOWN, so a loop that began
earlier is detected only after enough newly recorded generations; a hand-off that changed the
work outside the candidate tree, gates and reviews counts as no progress, and the owner's reopen is
the release valve; reopen also restarts the retry budget through the fresh attempt; the dispatcher
raises its event when it first observes a hold, including at its baseline observation after a
start; a ledger save failure or crash after an event leaves the episode unrecorded, and the
duplicate is suppressed while the event stays in the readable log tail (the newest 1 MiB of the
current and rotated logs), so it can repeat once only when that log is unreadable or the event has
left the scanned tail; an unwritable event log keeps the episode pending, so the event is delayed,
not lost; a part-way failed append leaves an unparseable fragment line, which the next append
terminates and readers skip; a malformed `loopEvidence` or `seen.loops` refuses as `MALFORMED` rather than being ignored.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-102 | `TestCALV0102_PolicyLoopDetectionOptIn` (`internal/tasks/intent`); `TestCALV0102_PriorLoopEvidenceRoundTrip`, `TestCALV0102_LoopEvidenceOf` (`internal/tasks/snapshot`); `TestCALV0102_LoopHoldSignals`, `TestCALV0102_ClaimHonoursLoopHold`, `TestCALV0102_PolicyAbsentMatchesNMinusOne` (D8 digest pinned to the pre-change source), `TestCALV0102_NMinusOneScenarioOptsIn` (`internal/tasks/transaction`); `TestCALV0102_StoreLoopHoldAndOwnerReopen` (`internal/tasks/store`); `TestCALV0102_CLILoopHoldSurfaces`, `TestCALV0102_DispatchStatusShowsLoopDetected` (`internal/tasks/cli`); `TestCALV0102_DispatcherSkipsAndEscalatesLoopOnce` (`internal/tasks/dispatch`); `TestTMV0002_AS01_CommandResultEnvelope` code count 74 (`internal/tasks/wire`); `TestAgentLeasesSpecEnumeratesCALV0102` (`internal/lrfrepo`) |
| CAL-V0-103 | `TestCALV0102_StoreLoopHoldAndOwnerReopen`, `TestCALV0103_ReopenRefusesUnheldTicket` (`internal/tasks/store`); `TestCALV0102_CLILoopHoldSurfaces` (`internal/tasks/cli`); `TestCALV0102_DispatcherSkipsAndEscalatesLoopOnce`, `TestCALV0102_LedgerLoopsAreClosedAndOptional`, `TestCALV0103_LedgerLoopsStrictWithoutProgress`, `TestCALV0103_LoopEscalationSurvivesEventAppendFailure`, `TestCALV0103_LoopEscalationSurvivesPartialEventWrite`, `TestCALV0103_LoopEscalationNotDuplicatedAfterCloseFailure`, `TestCALV0103_LoopEscalationNotDuplicatedAfterUnsavedLedger` (`internal/tasks/dispatch`); `TestAgentLeasesSpecEnumeratesCALV0102` (`internal/lrfrepo`) |

Rollback: to disable detection, remove `loopDetection` with `policy update`; the hold disappears
at the next read and claims stop recording `loopEvidence`. Downgrading is different. An older
binary decodes the policy with a closed top-level key set and refuses a policy carrying
`loopDetection` as `MALFORMED`; because journal audit and replay decode every historical policy
afterimage, it keeps refusing the store, including `receipt audit`, after the key is removed. Its
closed attempt decoder likewise refuses any attempt whose prior entry carries `loopEvidence`
(inferred from the closed prior-generation key set; not separately exercised on an older binary).
A downgrade therefore keeps this binary or a later reader, or restores the whole store, intent and
state directory together, from a backup taken before the key was first written, verified with
`receipt audit` under the older binary, accepting that later transactions are lost. Search the
store for the byte strings `"loopDetection"` and `"loopEvidence"` to tell whether either was ever
written; no match means older binaries still read it. An older dispatcher refuses a ledger that
carries `seen.loops`; delete only that member while no hold is current, or start from a fresh
ledger. Older readers also refuse a record or plan carrying `LOOP_DETECTED`; the code appears only
in derived output and in dispatcher events, never in a stored ticket record.

### V1-0772 supervised stage worktree bound and unproved stops (issue 354)

Human-owned input: owner request [issue 354](https://github.com/beamfall/corvint/issues/354)
(2026-10-05: finish the remaining code), native suspected bugs V1-0772 and V1-0771. Root cause, on
base `d524530f`: a supervised stage worktree is `<work root>/<program>/<assignment>/<generation>/<stage>-<turn>`,
and the attempt decoded `worktreePath` as a 128-byte Identifier. With a fixture work root under a
resolved `TMPDIR` longer than about 81 bytes (an agent scratchpad, for example), the path exceeded
128 bytes and DISPATCH refused `LIMIT_EXCEEDED` (`/worktreePath: Identifier longer than 128 bytes`)
after the worktree and its program records existed. That part is deterministic, not load-dependent.
Two further failures appeared only under concurrent host load (V1-0771). First, on Darwin,
`kill(-group, 0)` answers `EPERM` for a process group whose only member is an unreaped zombie. The
lane leader is one until `Run`'s `Wait` reaps it, so the drain reported survivors. The role then
journaled the program `FINISHED` over `BLOCKED_RECOVERY` and was refused
`MALFORMED: program transition`. Second, the stage watcher cancelled a running stage on one
failed unlocked program read. That read had raced a concurrent writer's journal staging
(`MALFORMED: staging/aNN: unassigned stage slot`).

- `CAL-V0-086`: An attempt's `worktreePath` MUST decode as an absolute PathText of at most 4096
  bytes, not an Identifier. A path that is empty, relative, contains a control character or exceeds
  4096 bytes MUST still be refused with its code. Before any directory, program record or Git worktree
  exists, the supervisor MUST refuse a stage worktree path, or a `<path>@<name>` sibling path, that
  is not such a PathText. DISPATCH MUST refuse it as well. A stage whose drain does not prove
  quiescence MUST end its role with a non-retryable `SURVIVORS` error, which takes precedence over
  the stage's own failure (a wall timeout, nonzero exit or invalid result) and keeps its text. The
  classification MUST precede any candidate preservation or read-only candidate check, so a stage
  over an existing candidate that also changed its worktree is still `SURVIVORS`, never left
  `STOPPING`. The program and attempt then stay `BLOCKED_RECOVERY`, the program quiescence is
  recorded `UNKNOWN`, and the owner stays unreleased. Such a stage MUST NOT journal `FINISHED`, and a
  role MUST NOT go on to gates, `READY` or integration after it. The drain MUST treat `EPERM` from the process-group
  probe as neither gone nor observable. It re-probes until the group is proved gone (`ESRCH`) or the drain deadline passes,
  and it never signals on, or counts as gone, an `EPERM` answer. The stage watcher MUST tolerate
  failing unlocked program reads for at most 30 seconds of continuous failure before it stops the
  stage, and a successful read resets that window. A heartbeat refusal still stops the stage at once.

Non-goals: fixing the journal reader's transient `unassigned stage slot` refusal itself (filed
separately), and a longer drain deadline. Failure modes: a group that keeps answering `EPERM` (for
example, a member this user may not signal) is still unclean once the deadline passes. Program
reads that fail for 30 seconds still stop the stage, so a drain or cancel control is observed up to
30 seconds late while reads fail. Downgrade is one-way: a binary without this amendment refuses an
attempt whose recorded `worktreePath` is longer than 128 bytes (`LIMIT_EXCEEDED`). Because a full
journal walk revalidates every retained post, it then refuses the retained history, unless its reader
resumes from a checkpoint after that record. Attempts with shorter paths keep identical bytes.
Rollback reverts the decoder and the workflow together; the drain and watcher changes write no new
bytes. Regression witnesses are in the CAL-V0-086 traceability row; see
`docs/build-log/2026-10-05-tasks-multirepo-continuation.md`.

### V1-0851 worker-exit release or reap (issue 622)

Human-owned input: owner request [issue 622](https://github.com/beamfall/corvint/issues/622) and
native ticket V1-0851. Observed on program `flowproof` (2026-10-05/06): six dispatcher-launched
workers ended without releasing their attempts. Each time the dispatcher's one hand-off was refused
and it reported `needs-owner` and left the attempt RUNNING and its pool member ALLOCATED, even after
the lease expired. In one case the attempt had meanwhile become FAILED/FENCED, so a manual reap
changed nothing. The owner asked for a bounded release retry, a reap once the lease expires, and
`needs-owner` only when both fail, behind a dispatcher switch that defaults on.

- `CAL-V0-104`: With `heal.handoff` and `heal.exitRecovery` (optional boolean, default true), a
  refused hand-off of a live attempt held by an ended worker this dispatcher launched or adopted
  MUST NOT emit `needs-owner` at once. The dispatcher MUST track that attempt at the generation and
  holder it observed, in memory for its run, and on each later readable tick:
  (1) if the attempt is absent or no longer live (released, reaped, or FAILED/FENCED by another
  path), it MUST drop the recovery with a `handoff` event whose detail `recovery` is
  `ALREADY_ENDED`, and write nothing;
  (2) if the attempt's generation or holder changed, it MUST drop the recovery with a `handoff`
  event whose `recovery` is `SUPERSEDED`, and never release or reap it;
  (3) once the lease has expired, it MUST reap the attempt through the fenced reap transaction
  (which fails the attempt and quarantines its pool member as an ordinary reap does), at most three
  times;
  (4) before the lease expires, it MUST retry the `HANDOFF` release, with the same evidence rule as
  CAL-V0-056, so that at most three releases are made including the first; after the last refusal
  it waits, without writing, for the lease to expire.
  Writes are spaced by `tickSeconds` doubled per write and capped at five minutes, on the
  dispatcher's clock. Each retry uses a distinct deterministic request ID, derived from the worker,
  attempt, generation and try number, so a recorded refusal is never replayed as the retry's
  answer. A successful retry emits `handoff` or `reaped` with `recovery` `RELEASED` or `REAPED`; a
  reap that the store reports as changing nothing is resolved too. `needs-owner` MUST be emitted
  exactly once, naming both last errors, only when the reaps are exhausted, or when the releases
  are exhausted and the attempt has no lease to reap. An attempt held by any other holder is never
  recovered; only the generic `heal.reap` of CAL-V0-056 applies to it. With `heal.exitRecovery`
  false, CAL-V0-056's single hand-off and immediate `needs-owner` are unchanged. While a recovery
  is pending or exhausted, a worker reported as ended again (because a tick failed before the worker
  was accounted) MUST NOT restart it: the tries, backoff and request IDs continue, and an exhausted
  recovery writes nothing more until the attempt ends or is superseded.

Non-goals: persisting the recovery across a dispatcher restart (after a restart `heal.reap` still
reaps the expired lease of a holder that is not a running worker, but releases are not retried);
recovering attempts this dispatcher did not launch or adopt; a configurable retry count or backoff;
new event kinds. Failure modes: an unreadable store skips heal, so the recovery waits for the next
readable tick; a store write cancelled by shutdown leaves the recovery pending for the next tick of
the same run; a restart loses pending recoveries; a lease renewed by a still-running orphan keeps
the attempt live and, once releases are exhausted, the recovery waits for that new expiry; and
with `heal.reap` also on, an expired attempt is written by the recovery only, never twice in one
tick.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-104 | `TestCALV0104_ExitRecoveryRetriesHandoffOnLiveLease`, `TestCALV0104_ExitRecoveryReapsExpiredLease`, `TestCALV0104_ExitRecoveryResolvesFencedAttempt`, `TestCALV0104_ExitRecoveryNeedsOwnerOnlyWhenBothFail`, `TestCALV0104_ExitRecoverySurvivesFailedPostHealObservation`, `TestCALV0104_ExitRecoverySwitchOff`, `TestCALV0104_ExitRecoveryConfig` (`internal/tasks/dispatch`); `TestCALV0056_HandoffAndReap`, `TestCALV0056_CancelledHealingStopsNextWrite` unchanged |

Rollback: set `heal.exitRecovery` to false for CAL-V0-056's original behaviour, or revert the
change. The switch is an optional config member and the recovery writes no new ledger, store or
wire bytes, but an older binary refuses a config that names `exitRecovery` (closed decoding), so
remove the member before downgrading. See `docs/build-log/2026-10-06-v1-0851-worker-exit-recovery.md`.

### V1-0853 held work outside the dispatcher's selection window (issue 624)

Human-owned input: owner request [issue 624](https://github.com/beamfall/corvint/issues/624) and
native ticket V1-0853. Observed on 2026-10-05: the program's work state held 21 tickets
(`hold-lane`, `hold-dep`), yet they stayed SELECTED in the `maxActiveAttempts` window of 24.
That left 142 actionable tickets DEFERRED `LIMIT_EXCEEDED`, so no role could ever launch them. The
owner asked that the window count only tickets the dispatcher can act on. The preferred form is
derived and needs no new store state.

- `CAL-V0-105`: When the dispatcher has a work-state reader, a ticket MUST count as held exactly when
  its observed work state is a known value (neither `NONE` nor `UNKNOWN`) that the state
  predicate (`states`/`excludeStates`, CAL-V0-054) of no ticket role admits. Lane roles do not
  count. The dispatcher MUST then replan the same observed snapshot, with no further store read and
  no store write, passing the held ticket IDs as a derived input. In that plan, a held ticket that
  would otherwise reach the selection window MUST be `DEFERRED` with reason and blocker
  `WORK_STATE_HELD`, MUST NOT count toward `maxActiveAttempts`, and MUST NOT block a later ticket by
  resource collision. A ticket that is BLOCKED (held record, dependency, prerequisite, loop) keeps
  that decision. A held OPEN ticket that requires a pool still counts as waiting for CAL-V0-101
  priority yield, so the dispatcher's plan agrees with the native claim's admission. Without a
  reader, without a ticket role, with no held ticket, or for a ticket whose state is `UNKNOWN` or
  `NONE`, the plan MUST be exactly today's (fail closed). The decision MUST be a pure function of
  the snapshot and the work-state observation. `WORK_STATE_HELD` is derived dispatcher output only:
  it appears in the dispatcher's ticket plan and `state` events. It is never a wire code, a stored
  value, a plan-preview reason or a claim refusal.

Non-goals: a native, revisionless program hold (the issue's alternative), counting held tickets for
plan preview or `claim-next`, which have no work-state observation and keep today's window, a
"not routed for N ticks" heuristic, and any change to the native claim's live-reservation limit.
Failure modes: a work-state reader that fails reports `UNKNOWN`, so the affected tickets stay in the
window as before. A role whose predicate admits a hold value means that value is not a hold. A
worker launched by a non-dispatcher client can still claim a held ticket through `claim-next`,
because the native plan does not see work state. The dispatcher's plan can therefore differ from
`plan preview` while tickets are held.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-105 | `TestCALV0105_WorkStateHeldTicketsLeaveWindow`, `TestCALV0105_HeldSetDoesNotOverrideBlockedOrUnheld`, `TestCALV0105_HeldPooledTicketStillTakesPriorityYield` (`internal/tasks/transaction`); `TestCALV0105_HeldWorkStateLeavesDispatcherWindow`, `TestCALV0105_UnknownOrUnreadStateKeepsWindow`, `TestCALV0105_StateMatchesIsTheRolePredicate` (`internal/tasks/dispatch`); `TestCALV0105_DispatchObservationReplansHeldTickets` (`internal/tasks/cli`) |

Rollback: revert the change. No config, ledger, store or wire bytes change, so every binary reads the
same state. Removing the work-state reader, or giving a role a predicate that admits the hold values,
also restores today's window. See `docs/build-log/2026-10-06-v1-0853-workstate-held-window.md`.
### V1-0852 caller-asserted author cover (issue 623)

Human-owned input: owner request [issue 623](https://github.com/beamfall/corvint/issues/623) (native
ticket V1-0852). A review claim with `--exclude-authors` on a ticket whose implement generation
predates V1-0788 (no recorded member) refused `INDEPENDENCE_UNVERIFIED` even when the caller named
the author with `--exclude-member`, and a ticket with no implement generation at all refused although
it has no author to exclude. The V1-0789 build log records both refusals as an implementer extension
raised as owner questions, not owner decisions; this amendment answers them as the issue asks.

- `CAL-V0-107`: When a CAL-V0-098 claim, CLAIM_NEXT or plan preview carries at least one explicit
  `--exclude-member`, a reached generation whose history is `NOT_OBSERVED`, one with no recorded
  stage and no recorded pool member, or an implement generation without a pool member MUST be
  treated as covered by the explicit set: it contributes no member, does not refuse, and the walk
  continues to older generations (`LATEST` still stops at the first recorded author). A generation
  with no recorded stage but a recorded pool member MUST still refuse; a recorded member is never
  replaced by the caller's assertion. Without explicit exclusions every CAL-V0-098 refusal is
  unchanged. A ticket with no recorded implement author (none implemented, only review or integrate
  generations, or only covered generations) MUST NOT be refused for that reason; its effective
  exclusion set is the explicit members, and an exhausted pool's `RESOURCE_COLLISION` detail says
  `excluded implement authors: none recorded`. An admitted claim's result MUST carry a warning
  naming each covered generation as covered by the caller's explicit `--exclude-member` set, not by
  recorded members, and a warning when no implement generation records an author. The request
  preimage, replay and `REQUEST_ID_CONFLICT` behaviour of CAL-V0-065 and CAL-V0-098 are unchanged:
  the warnings come from fresh derivation and an exact replay does not repeat them. POOL_PREPARE
  MUST carry the claim's explicit members and apply the cover only when they are present, so the
  cover is never implied: without them a generation that became unrecorded after the claim's
  derivation refuses `INDEPENDENCE_UNVERIFIED` before PREPARING is recorded and so before any
  health command runs or any member is quarantined, and the claim is reevaluated.

Non-goals: no backfill or inference of legacy member facts; no proof that the explicit set names the
author; no new result code or stored field; the plan preview keeps its CAL-V0-098 shape.

Failure modes: a caller who names the wrong member is admitted on the real author's member; the
warning records the claim as caller-asserted, which is the only safeguard. A stage-less pooled
generation still refuses. A binary without this amendment refuses the same claims as before.

Acceptance evidence: `TestCALV0107_ExplicitMembersCoverUnrecordedGenerations`,
`TestCALV0098_DeriveAuthors` (`internal/tasks/transaction`); `TestCALV0098_ClaimExcludesImplementAuthor`,
`TestCALV0098_ClaimNextExcludesAuthors`, `TestCALV0098_HealthSkipsAuthor` (`internal/tasks/store`);
`TestCALV0107_CLIClaimReportsCoveredGenerations`, `TestCALV0098_CLIExcludeAuthors`,
`TestCALV0098_ExhaustedPoolParity` (`internal/tasks/cli`); see
`docs/build-log/2026-10-06-v1-0852-author-exclusion-cover.md`.

Rollback: revert the code and this amendment. No stored state or preimage changes; claims admitted
under the cover keep their recorded allocations.

### V1-0855 cross-stage admission order (issue 626)

Human-owned input: owner request [issue 626](https://github.com/beamfall/corvint/issues/626) (native
ticket V1-0855). With `priorityAdmission` on, an implement claim for an equal-priority ticket earlier
in plan order took the only free member while a review claim for a ticket that gated others had been
waiting longer. CAL-V0-101 ranked competitors by plan order only, so stage and waiting time did not
count. The issue asks that a waiting review claim for a blocking ticket beat new implement claims,
and that the decision be visible.

- `CAL-V0-108`: CAL-V0-101 MUST rank competitors and the claimed ticket in admission order: priority
  first; at equal priority a downstream ticket before any other; two downstream tickets by the
  earlier handoff sequence; then plan order (order, ticket ID). A ticket is downstream when its
  latest attempt generation is terminal, is bound to the ticket's current acceptance revision, and
  recorded a `HANDOFF` to `review` or `integrate` (its CAL-V0-084 `nextStage`); its handoff sequence
  is that generation's `phaseSinceSeq`. The rank MUST be derived from the attempts on every read and
  claim and MUST NOT add store, journal, request or wire state. A downstream competitor's claim
  blockers MUST be read at its own downstream stage, the stage its claim names, so CAL-V0-099
  stage-scoped prerequisites apply as they would to that claim; any other competitor's blockers are
  read as before. Every competitor MUST be read against the pool itself, needing a free member
  eligible for the stage it is read at, and without the plan's or claim's own `--exclude-member` or
  `--exclude-authors`, so the default plan, a `--pool` plan and a claim count the same competitors.
  The yield refusal detail keeps its CAL-V0-101 text and, when the ticket yielded
  to is downstream, MUST append `; <ticketId> awaits <stage> since seq <seq>`. Plan preview (default
  and `--pool`), `claim --next` and `ticket show` claimability MUST use the same order, so they agree
  with an explicit claim. Without a downstream competitor the order is plan order and every
  CAL-V0-101 result is unchanged; a pool without the flag is unaffected.

Non-goals: a review or integrate ticket does not outrank a higher-priority implement ticket; there
is no priority inheritance from dependent tickets and no `critical` or `blocks-gate` ticket field
(the issue's alternative; an owner question). The time an implement-stage ticket has waited is not
recorded, so among non-downstream tickets plan order stands; downstream-first is the derived stand-in
for "waiting longer" and covers a review that open tickets depend on. A competitor's own
`--exclude-member` or `--exclude-authors` is not known, so free members are counted for the claimed
ticket only, as before. `dispatch status` is not changed: it reads the dispatcher configuration and
ledger, not admission, and the decision is visible in the refusal detail, the plan entry's blocker
and the competitor's `nextStage`.

Failure modes: an implement claim on a pool with a downstream ticket waiting at equal priority now
yields where it was admitted before; the operator claims the downstream ticket, changes priorities
or drops the flag. A downstream ticket that never claims keeps its rank until its handoff is
superseded by a new generation or its acceptance revision changes. A claim on a ticket whose
recorded next stage is downstream ranks downstream whatever `--stage` it names, because the rank is
derived from records only.

Acceptance evidence: `TestCALV0108_WaitingReviewOutranksArrivingImplement` (issue order: review
waiting, implement claim arrives, a member frees, the implement claim yields and the review is
admitted; flag off admits the implement claim), `TestCALV0108_EarlierHandoffWinsAtEqualPriority`,
`TestCALV0108_AdmissionRank`, `TestCALV0101_PlanClaimAndClaimNextAgree` (now with generated
handoffs), `TestCALV0101_FlagOffMatchesNMinusOne` (pinned digest unchanged)
(`internal/tasks/transaction`); see `docs/build-log/2026-10-06-v1-0855-cross-stage-admission.md`.

Rollback: revert the code and this amendment. No stored state or preimage changes; admission
returns to plan order at the next read or claim.

### V1-0863 caller-bounded lease lock wait (issue 637)

Human-owned input: owner request [issue 637](https://github.com/beamfall/corvint/issues/637) (native
ticket V1-0863). With 6 to 10 concurrent writers, `release --reason HANDOFF --evidence ...` returned
the retryable LOCK_TIMEOUT on `taskman.prepare.lock` after the fixed 30-second wait. The issue asks
for a caller-chosen wait, safe same-request resubmission, and offers a third option that would turn
a plain release into a HANDOFF when evidence exists.

- `CAL-V0-111`: `release` and `attempt heartbeat` MUST accept `--lock-wait SECONDS`, whole seconds
  in canonical decimal from 1 to 300 (`MaxCallerLockWait`). Any other value, a repeated flag, or the
  flag on another lease verb MUST refuse MALFORMED before any store read or write. With the flag,
  the lease transaction's preparation admission and each writer-lock acquisition, including the
  orphan-stage cleanup lock, MUST wait up to
  that bound, spent across all phases without restarting, instead of the 30-second default; an
  expired bound still refuses the retryable LOCK_TIMEOUT before any write. The value MUST NOT be part
  of the request, its digest or any receipt. Without the flag every wait is unchanged: the §1
  default and maximum of 30 seconds still apply to every other caller.
- `CAL-V0-112`: An evidence HANDOFF release refused LOCK_TIMEOUT MUST leave the store unchanged, so
  resubmitting the same request ID with the same arguments MUST commit exactly once, and every later
  resubmission MUST replay that committed receipt sequence without a write, with or without
  `--lock-wait`. The replayed HANDOFF MUST carry its recorded evidence and MUST NOT charge a retry
  to the next claim. This already held before V1-0863 (the lock refusals precede every write and the
  wait is outside the request digest); the requirement pins it with maintained tests.
- `CAL-V0-113`: No release path MAY convert a plain release into a HANDOFF. A plain release after
  a timed-out HANDOFF is a new request that records no handoff evidence and charges a retry as
  before.

Non-goals: the issue's option 3 (treating a plain release as a HANDOFF when evidence exists) is
rejected: it would silently change the retry accounting the caller asked for and depend on a
heuristic over evidence the caller did not name. The 30-second §1 default, the preparation
admission order and the registered-slot bound are unchanged; `--lock-wait` is not added to claim,
renew, reap or batch verbs. Contention itself is not reduced.

Failure modes: a caller with a long `--lock-wait` holds a preparation slot for up to that bound,
which can delay later registrations that then time out at their own bound. A wait beyond 300
seconds refuses MALFORMED rather than clamping silently. A resubmission with changed arguments
under the same request ID refuses REQUEST_ID_CONFLICT, as before.

Acceptance evidence: `TestCALV0111_CallerWaitBound`, `TestCALV0111_CallerWaitOutlastsDefault`
(`internal/tasks/authority`); `TestCALV0111_OrphanCleanupSpendsCallerWait`,
`TestCALV0112_HandoffReleaseReplaysAfterLockTimeout` (`internal/tasks/store`); `TestCALV0111_LockWaitFlag`, `TestCALV0111_LockWaitBoundsContendedRelease`,
`TestCALV0112_SameRequestHandoffReplayAfterLockTimeout`,
`TestCALV0113_PlainReleaseAfterTimedOutHandoffIsCharged` (`internal/tasks/cli`); see
`docs/build-log/2026-10-06-v1-0863-handoff-lock-wait.md`. Live fleet qualification under 6 to 10
writers is NOT_RUN.

Rollback: revert the code and this amendment. No stored state, request digest or receipt shape
changes; callers that pass `--lock-wait` then refuse MALFORMED as an unknown flag.

### V1-0891 CPU signal and per-OS pressure signal selection (issue 646)

Human-owned input: owner request [issue 646](https://github.com/beamfall/corvint/issues/646) (native
ticket V1-0891, P1). On macOS the one-minute load average counts virtualization vCPU threads and
uninterruptible waits, so the throttle held level 2 while the CPUs had spare capacity. The issue asks
for CPU utilisation from tick deltas inside the per-tick sample without an extra command, for the
level to use it, for optional per-OS signal selection (for example memory only on macOS), and for
defined first-tick behaviour. The coordinator assigned CAL-V0-125..126. Both requirements are
proposed; acceptance is the owner's.

The macOS tick source is not delivered. `host_processor_info` and `host_statistics(HOST_CPU_LOAD_INFO)`
are Mach calls that a `CGO_ENABLED=0` binary reaches only through `go:linkname` trampolines, which the
release gate forbids; Go's `syscall` and the standard library expose no wrapper, and no macOS sysctl
publishes cumulative CPU ticks. macOS samples therefore carry no tick counters and cannot select
`cpu`. The macOS mitigation the issue names is delivered: selecting `memory` alone removes the load
average from the level.

- `CAL-V0-125`: proposed (V1-0891; GitHub #646). The Linux pressure sample MUST read cumulative CPU
  ticks from the aggregate `cpu` line of the same bounded `/proc/stat` read that counts CPUs: total is
  user, nice, system, idle, iowait, irq, softirq and steal; busy is total minus idle and iowait. A
  missing, duplicated, short, non-numeric or overflowing line adds a problem and leaves the counters
  unknown. The dispatcher MUST derive `cpuUtilization` as the busy delta over the total delta between
  the previous recorded sample of this run and the current one, and only when both carry counters
  from the same source, the total advanced, neither counter regressed and the busy delta does not
  exceed the total delta; otherwise utilisation is UNKNOWN with a named problem. The first sample of
  a run, including the first after a restart (which clears the previous sample, CAL-V0-068), is
  therefore UNKNOWN. The optional `pressure` thresholds `calmCpu < cpuHigh < cpuCritical <= 1`
  (finite, non-negative) are required when any OS selects `cpu` and validated whenever one is
  configured. When selected, utilisation is classified like load and swap and `cpu` is a valid level
  `reason` (CAL-V0-110), sorted before `load`. An UNKNOWN utilisation of a selected `cpu` signal makes
  the sample UNKNOWN and the CAL-V0-068 UNKNOWN rule applies. The ledger refuses a utilisation outside
  0..1 or one without its known flag. `dispatch status` and the `throttled` event MUST report
  `cpuUtilization` (the fraction or `UNKNOWN`). Sampling MUST follow the selection: a sampler reads
  and parses nothing for an unselected signal (Linux: `/proc/loadavg` only for `load`, `/proc/stat`
  only for `load` or `cpu`, `/proc/meminfo` only for `swap`; macOS: one `sysctl` of the selected
  keys only, none when nothing observable is selected), and `/proc/stat` is read only up to the end
  of its leading `cpu` lines. Without `cpu` selected, the sample carries no tick counters or
  utilisation and records no tick problem, so those ledger fields are written only when `cpu` is
  selected.
- `CAL-V0-126`: proposed (V1-0891; GitHub #646). The `pressure` object MAY carry `signals`, a map
  from host OS (`darwin` or `linux`) to 1..N unique signal names that host can observe (`darwin`:
  `load`, `memory`; `linux`: `cpu`, `load`, `swap`). Any other OS key, an empty or duplicated list, or
  an unobservable name MUST be refused. On a host whose OS has an entry, exactly the named signals set
  the level: any of them can raise it, all must be calm to release it, and an UNKNOWN unselected
  signal does not make the sample UNKNOWN. A host without an entry keeps the CAL-V0-068/109 default
  (load plus the kernel memory-pressure level on macOS, load plus swap on Linux), so a configuration
  without `signals` behaves as before.

Non-goals: macOS CPU utilisation (needs cgo or forbidden linkname; owner decision); per-CPU or
per-process utilisation; PSI or cgroup CPU limits; signal weighting; selecting signals per role.
Failure modes: the first tick of every run is UNKNOWN when `cpu` is selected, so a recorded level is
held (never released or raised) for one admission tick after a start; a tick interval with no clock
advance in `/proc/stat` is UNKNOWN; a host whose counters reset (a different source) starts a new
baseline. A selection that omits `load` lets an inflated load average no longer throttle, which is
the intent. Rollback removes `signals` and the CPU thresholds from the configuration; a ledger
whose sample carries CPU counters or utilisation is refused by an older binary, so a downgrade first
needs one start without `pressure`. Because those fields are written only while a host selects `cpu`,
this downgrade risk applies only to operators who opt into `cpu`; default ledgers stay readable by
older binaries.

Acceptance evidence: `TestCALV0125_CPUUtilizationDelta`, `TestCALV0125_LinuxCPUTicksParsing`,
`TestCALV0125_DispatcherFirstTickUnknown`, `TestCALV0125_LinuxSamplingFollowsSelection`,
`TestCALV0125_DarwinSamplingFollowsSelection`, `TestCALV0125_UnselectedCPULeavesLedgerUnchanged`,
`TestCALV0126_SignalSelection`,
`TestCALV0126_SignalSelectionValidation` (`internal/tasks/dispatch`);
`TestCALV0125_DispatchStatusCPUUtilization` (`internal/tasks/cli`); see
`docs/build-log/2026-10-06-dispatch-cpu-signal-config-reload.md`. Live Linux sampling and a live
dispatcher under CPU saturation are NOT_RUN.

### V1-0890 dispatcher configuration reload, cap 0 and lane minimum age (issue 645)

Human-owned input: owner request [issue 645](https://github.com/beamfall/corvint/issues/645) (native
ticket V1-0890, P2): apply a changed configuration file without restarting the dispatcher, let
`cap: 0` disable a role, and let a lane wait until a member has stayed in its state for a minimum
time. The coordinator assigned CAL-V0-127..129. All three requirements are proposed; acceptance is
the owner's.

- `CAL-V0-127`: proposed (V1-0890; GitHub #645). A running `dispatch` MUST re-read its `--config`
  file at the start of each tick, before it observes the store, with the CAL-V0-052 read and decode
  rules. Unchanged bytes (by SHA-256) do nothing. A changed file that decodes and validates and keeps
  `stateDir` and `workRoot` MUST become the applied configuration for that tick and every later one,
  appending one `config` event (`outcome` `APPLIED`, the new digest, and any removed roles). An
  unreadable or invalid file, or one that changes `stateDir` or `workRoot`, MUST be refused: the
  applied configuration stays active, one `alert` event (`config` `REFUSED`, the digest or `UNKNOWN`
  when unreadable) is appended once per distinct refused content, and the refusal is recorded. When
  the file again matches the applied configuration the refusal is cleared with a `config` event
  (`outcome` `RESTORED`). A reload MUST NOT stop, signal or relaunch a running worker: a removed
  role launches nothing further and its running workers stay supervised to their normal end. A
  worker running when a reload applies MUST stay supervised under the configuration it launched
  under (its role's wall and idle timeouts, its host's activity rules and the kill grace), even when
  the reload lowers those timeouts or removes the role; that launch configuration is kept in memory
  for the run, and a restart supervises adopted workers under the file it starts with. Across
  a reload the recorded pressure level is kept and pending dwell restarts; adding `pressure` starts an
  UNKNOWN record and removing it drops the record. The ledger keeps this run's record (applied
  digest and time, newest refusal of at most 1024 bytes); a restart applies the file afresh and drops
  it. `dispatch status` MUST report it as `config` (`appliedSha256`, `appliedAt`, and `refused` with
  `sha256`, `at` and `reason`, or `NONE`) once a change has been seen. Pool routing for planning
  follows the applied configuration. `dispatch status --config FILE` MUST still read the ledger when
  FILE no longer decodes or validates, provided a single, exactly spelled, clean absolute top-level
  `stateDir` string precedes any syntax error: it then reports the recorded `config` refusal and
  adds `configFile` (`state` `INVALID`, the file's `sha256`, `reason`); the escalation view is
  omitted, pressure caps are not shown and the infrastructure-retry `policy` is `UNKNOWN`. Without a
  recoverable `stateDir` status refuses as usage, and `dispatch unpark` always requires a valid
  file.
- `CAL-V0-128`: proposed (V1-0890; GitHub #645). A role `cap` MAY be 0. A role with cap 0 MUST
  launch nothing; its running workers are unaffected, and every other CAL-V0-052 bound is unchanged.
  A disabled role MUST take no part in planning: its pool is not among the pools the plan may select
  (CAL-V0-097), its work-state predicate holds nothing in the window (CAL-V0-105) and it yields no
  roster candidates, so a ticket only it could take never uses the `maxActiveAttempts` window of an
  enabled role.
- `CAL-V0-129`: proposed (V1-0890; GitHub #645). A lane MAY carry `minAgeSeconds` 0..604800. When
  positive, the roster MUST admit a pool member only after the dispatcher has observed it in the
  same state and change sequence for at least that many seconds on the dispatcher clock. A changed
  state or change sequence, a member that disappears, a clock step backwards and a dispatcher
  restart each start a new episode; the episode clock is in memory only, so a restart can only delay
  a lane launch, never admit a member early. The roster stays a pure function of its inputs (CAL-V0-054).

Non-goals: a filesystem watcher or signal-triggered reload; reloading `stateDir` or `workRoot`;
stopping or draining workers of a removed or disabled role; reloading in `service` mode (the
manifest-pinned service keeps its pinned configuration); a persistent member-age clock; ages read
from the store's receipt timestamps. Failure modes: a file edited in several writes may be read
half-written and refused, then applied at the next tick once complete; a refused file keeps the
previous configuration indefinitely and is visible only in status and the event log; a reload that
narrows caps leaves already running workers above the new cap until they end. Rollback reverts the
file (applied at the next tick) or restarts the dispatcher; a ledger carrying the reload record is
refused by an older binary, so a downgrade first needs one restart (which drops the record).

Acceptance evidence: `TestCALV0127_ConfigReloadAppliesAndRefuses`,
`TestCALV0127_ReloadRemovedRoleKeepsWorkers`, `TestCALV0127_ConfigRecordValidation`,
`TestCALV0127_ConfigRecordBesideStrictRecords`, `TestCALV0127_ReloadKeepsLaunchDeadlines`,
`TestCALV0127_ReloadRemovedRoleKeepsDeadlines`, `TestCALV0128_CapZeroDisablesRole`,
`TestCALV0129_LaneMinAge` (`internal/tasks/dispatch`); `TestCALV0127_DispatchStatusConfigRecord`,
`TestCALV0127_DispatchStatusWithInvalidConfigFile`, `TestCALV0128_DisabledRoleDoesNotStarveEnabledRole`
(`internal/tasks/cli`); see
`docs/build-log/2026-10-06-dispatch-cpu-signal-config-reload.md`. Live dispatcher qualification is
NOT_RUN.

## Amendments to TCP-00

Accepting this spec accepts these amendments; each keeps the existing ID space.
The experimental `RUN_OUTCOME` observation verb is amended in by `corvint-tasks-attempt-runner-v0.md` (ATR-V0-005), not here.
The `ESCALATION_PENDING` detail code (72 codes after A17) is amended in by `corvint-tasks-escalations-v0.md` (ESC-V0-006; owner-accepted 2026-10-05), not here.

- A23: CAL-V0-102 adds the optional top-level policy key `loopDetection`, the optional
  absent-only `priorGenerations[]` key `loopEvidence` of `taskman-attempt/0`, written only while
  the policy carries `loopDetection`, and the closed detail code `LOOP_DETECTED`; after A21's 73
  codes the closed set contains 74. A policy, attempt or ledger without the keys keeps its bytes; older strict
  readers refuse a policy or attempt that carries them.

- A22: CAL-V0-078 adds an absent-only optional boolean `retryable` to TCP-00 §3.3's closed
  `taskman-command-result/0` envelope, present exactly on a non-`OK` result that carries at least
  one §11 code. `OK` and uncoded envelopes keep their bytes, and decoders accept the earlier coded
  bytes without the member, so the profile stays `taskman-command-result/0`.

- A21: CAL-V0-099 adds the optional absent-only ticket record key `executionPrerequisites` and the
  closed detail code `PREREQUISITE_UNSATISFIED`; with A17 and ESC-V0-006 the closed set contains 73
  codes. Records without the key keep their bytes; older strict readers refuse a record that
  carries it.

- A20: CAL-V0-101 adds the optional boolean `priorityAdmission` to a policy pool under the A15
  pattern. Omission keeps the existing canonical policy bytes and admission; a reader that predates
  it refuses a policy that carries it, and journal audit keeps refusing after the key is removed
  because earlier policy afterimages remain. No attempt, reservation, pool-state or plan member
  changes.

- A19: CAL-V0-097 adds the optional top-level `resourceDeferred` member to `taskman-plan/0` and
  admits a declared pool ID as a `DEFERRED RESOURCE_COLLISION` blocker. Both appear only in a
  default plan whose policy declares pools; a plan without them keeps its exact bytes. This is an
  additive /0 change under the A9/A15 pattern with no profile bump: old readers cannot consume the
  new records. Core's `taskman-plan/0` decoder is updated in the same change to validate the rows
  and admit a pool blocker only for a pool that has a row; it still refuses any other member.


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
| Escalated tier floods a scarce model | Many tickets launch on the stronger model at once | A tier `cap` holds the excess candidates waiting, never downgraded (CAL-V0-054) |
| Ladder names a model its host cannot render | The model setting would be silently ignored | The configuration is refused before any launch (CAL-V0-052) |
| Host saturated by load or swap while the dispatcher launches | New workers deepen the overload | With `pressure` configured, the level caps new non-exempt launches after its dwell and a `throttled` event names the held work; running workers are untouched (CAL-V0-068) |
| Host pressure sample unreadable, oversized, malformed or timed out | The level cannot be recomputed | The sample is `UNKNOWN`, the level and its cap are kept and pending dwell restarts; it never counts as calm (CAL-V0-068) |
| macOS swap use stays high after memory pressure passes | A static swap level would pin the throttle | The macOS memory signal is the kernel memory-pressure level; swap is not sampled there, and a missing level is `UNKNOWN`, never swap (CAL-V0-109) |
| Operator cannot tell which threshold set the level | Thresholds are tuned blindly | The level records its `reason`, shown in `dispatch status` and the `throttled` event (CAL-V0-110) |
| macOS load average counts vCPU threads and stays inflated | The throttle holds a level with idle CPUs | Per-OS `signals` can select `memory` alone on macOS; Linux can select `cpu` utilisation from tick deltas, UNKNOWN on the first tick of a run (proposed CAL-V0-125..126) |
| An edited dispatcher configuration is invalid | A restart or reload would stop dispatching | The applied configuration stays active and the refusal is reported in status and one alert (proposed CAL-V0-127) |
| Checkpoint absent, corrupt, oversized, foreign or ahead of the head | A read cannot resume | The read runs the complete audit and reports `FULL`; output is otherwise identical (CAL-V0-061) |
| Checkpoint disagrees with a receipt, projection, staging or the intent tree | A resumed read would mis-state the store | The resumed path refuses internally and the complete audit decides the reported verdict (CAL-V0-061) |
| Journal prefix, or a checkpoint entry together with its projection, altered behind a still-matching checkpoint | A resumed read does not see it | `receipt audit` and every mutation run the complete audit and refuse; the read's verdict says `CHECKPOINT_PLUS_TAIL`, not `CONSISTENT` (CAL-V0-061) |
| Writer cannot retain the checkpoint (full disk, permissions, crash before rename) | Reads stay at complete-audit cost | The transaction is unaffected; the next successful writer retains one (CAL-V0-060) |
| Extra repository moved, re-cloned, retargeted or undeclared | A program would edit an unintended checkout | Admission and every stage refuse unless the policy `supervision.repositories` pin matches the configured path, and every stage refuses a checkout whose common Git identity differs from the program record before any Git write (CAL-V0-071) |
| Extra repository edited outside the ticket's `@name/` touch paths | Candidate widens scope silently | The implement stage blocks `OUT_OF_SCOPE` with no candidate (CAL-V0-071) |
| Extra repository sibling worktree dirty or retargeted when a gate runs | A gate result would certify a tree other than the composite candidate | The gate refuses `DIRTY_WORKTREE` or `STALE_TREE` and records no result; the attempt does not reach `READY_FOR_INTEGRATION` (CAL-V0-087) |
| Changed extra repository has no integration designation | Its candidate would land in an unintended checkout or nowhere | The integrator refuses `UNSUPPORTED` before any grant or Git write; the composite candidate stays recorded (CAL-V0-087) |
| Integration grant omits a repository or its branch | A grant for one landing would authorize another | Its scope differs from the multi-repository formula and the grant is refused `APPROVAL_MISSING` (CAL-V0-087) |
| Designated checkout advanced, switched branch or dirty before integration | A fast-forward would fail mid-landing or land on the wrong branch | Refused before the grant (`TARGET_ADVANCED`, `designated checkout branch differs`) and re-checked under the lock before the first landing (CAL-V0-087) |
| Supervisor interrupted between repository landings, or before `INTEGRATED` | A restart would land a candidate twice or complete over a missing landing | A landed checkout is skipped; recovery requires every candidate on its branch or stops `BLOCKED_RECOVERY` (CAL-V0-087) |
| Outside writer advances a later designated checkout after an earlier one landed | Integration is not atomic against writers outside the store lock | Integration stops `TARGET_ADVANCED` with the earlier landing kept and never repeated; the operator restores the advanced checkout (known limit, CAL-V0-087) |
| Extra repository has no READY, fresh Core index | The host would edit a repository without its context | The stage refuses `repository <name>: CONTEXT_UNAVAILABLE` before launch (CAL-V0-088) |
| Stage stopped by its wall with `continuations` configured | Work past one wall would be lost or need an operator | The stage resumes its recorded session in the preserved worktree up to the policy bound, each as an ordinary ANSWER and DISPATCH under every cap (CAL-V0-089) |
| Continuation would exceed the lane or program turn cap, the program wall, or meets a pending drain or cancel | A continuation would overrun a bound or race the control | No continuation starts; the attempt waits unanswered for an operator `retry`, which the same caps refuse (CAL-V0-089) |
| Drain or cancel recorded after the continuation check, before its admission | The control returns as settled and another host turn launches anyway | The continuation's first program write refuses `FENCED` against the revision it replaces; the program stays `FINISHED` and released with the control recorded, and no host turn starts (CAL-V0-089) |
| `continuations` with Claude Code, or with a lane or program token cap | The stage could not resume, or every continuation would be refused as usage unknown | Program admission and every stage launch refuse `UNSUPPORTED` before mutation (CAL-V0-089) |
| Supervisor config host differs from the policy host, or names an unknown host | A program would speak the wrong vocabulary to the pinned binary | Admission and every stage refuse `UNSUPPORTED` before any record, lease or host process (CAL-V0-074) |
| Pinned Claude Code or Codex executable missing, replaced or re-moded | An unqualified binary would run | Refused `CAPABILITY_UNAVAILABLE` before any program record or lease (CAL-V0-074) |
| Pinned executable is a symlink or lacks an execute bit | Admission takes a lease for a stage that launch refuses, leaving the worker unresolved | Admission runs the launch-time check and refuses `CAPABILITY_UNAVAILABLE` before any record; a later refusal before the leader fork settles the stage `NO_EXEC` and cancels the attempt (CAL-V0-074) |
| Pinned executable replaced between the leader's check and its acknowledgment | Unchecked bytes would run | The leader runs the verified object: a root-protected path with no ACL on it or any ancestor, or a private copy of the verified bytes (CAL-V0-074) |
| Owner dies after a launch refusal's `FINISHED` record and before its owner release | The program stays `FINISHED` and unreleased; before the cancel the stopped attempt keeps its claim and reservation | A replacement process takes over the `FINISHED` program once the owner is gone, then cancels the attempt or, after the cancel, reassigns the program (CAL-V0-074) |
| Owner dies after a launch refusal's dispatch and before its `FINISHED` record | The program stays `SPAWNING` (before the settlement starts) or `STOPPING`, and the attempt keeps its claim and reservation | Known limit: a replacement is refused, because neither phase is safe for takeover and no leader boot record exists to recover; closing it needs a transaction contract change (CAL-V0-074) |
| Leader fails after the fork with no outcome class, and its drain is unproved | A spawned host would be recorded as never run and its claim released | Only a pre-fork refusal settles `NO_EXEC`; the attempt stays in `BLOCKED_RECOVERY` (CAL-V0-074) |
| Policy host changes under an existing idle or completed program | A reopen would reassign and claim work under the old host | Reopen refuses `UNSUPPORTED` before reassignment, claim or attach; a live attempt keeps drain and cancel only (CAL-V0-074) |
| Runtime pin changes while a drained `claude-code` attempt is live | Its claim and reservation would be stranded behind a config the new pin refuses | Rollback cancels every program, drained ones included, before the pin changes; a missed one is recovered by re-pinning its runtime to cancel it (CAL-V0-074) |
| Claude Code result is not one exact success object with a strict handoff | A host claim would be invented from prose | The stage is `INVALID_RESULT`; missing usage counters stay `NOT_OBSERVED` (CAL-V0-075) |
| Claude Code result or handoff repeats a JSON member, or aliases one by case | Last-wins or case-insensitive decoding would turn an error into success or merge partial usage | `INVALID_RESULT`, with session and usage `NOT_OBSERVED` (CAL-V0-075) |
| Runtime pin changes while a drained `opencode` attempt is live | Its claim and reservation would be stranded behind a config the new pin refuses | Rollback cancels every program, drained ones included, before the pin changes; a missed one is recovered by re-pinning its runtime to cancel it (CAL-V0-076) |
| OpenCode stream line or JSON handoff repeats a member, or aliases a read one by case | Last-wins or case-insensitive decoding would mask a host error, accept a refused review or merge partial counters | `INVALID_RESULT`, with session and usage `NOT_OBSERVED` (CAL-V0-077) |
| OpenCode program names a variant model or extra repositories | The stage effort or the worktree boundary would be silently overridden | Admission and every stage refuse `UNSUPPORTED` before any record (CAL-V0-076) |
| OpenCode stream reports an error, changes session, leaves a step open, is cut at the output cap, or ends without a stop-finished step carrying a strict handoff | A host claim or partial usage would be invented from a partial or failed turn | The stage is `INVALID_RESULT` (or `OUTPUT_LIMIT`); usage stays `NOT_OBSERVED` unless accounting is complete (CAL-V0-077) |
| Resumed OpenCode stage fails the fork or answers from the same session ID | An answer would be accepted without the history of the WAIT question it continues | The attempt stays `WAITING` with the original session retained (CAL-V0-077) |
| OpenCode standalone server or tool process leaves the owned process group | A host process would outlive a stage reported clean | Escapes are observed through their live parent and drained; the server holds the stage's standard error, so a survivor blocks end of file and the stop is not clean (CAL-V0-077) |
| OpenCode plugin or project configuration in the worktree | Untrusted code or permissions would load into the stage | Project configuration is disabled and permissions arrive inline; user-configured plugins are a known, uncontained limit (owner decision 2026-10-04) (CAL-V0-077) |
| Supervised stage worktree path longer than 128 bytes (deep work root or agent `TMPDIR`) | DISPATCH would refuse `LIMIT_EXCEEDED` after the worktree and program records exist | The attempt records the path as PathText up to 4096 bytes; a longer or invalid path is refused before any directory, record or Git worktree (CAL-V0-086) |
| Stage drain cannot prove quiescence (a host process escaped, or the probe never answers gone) | The role would journal `FINISHED` over `BLOCKED_RECOVERY` and be refused `MALFORMED`, report a wall timeout, nonzero exit or invalid result as `MALFORMED`, or (over an existing candidate it changed) refuse `read-only stage changed candidate` with the attempt left `STOPPING` | The role ends with non-retryable `SURVIVORS`, keeping any stage failure's text; program and attempt stay `BLOCKED_RECOVERY`, program quiescence `UNKNOWN`, owner unreleased (CAL-V0-086) |
| Darwin answers `EPERM` for a zombie-only process group before its leader is reaped | A finished stage would be reported unclean under load | The drain re-probes until `ESRCH` or its deadline and never counts `EPERM` as gone (CAL-V0-086) |
| Unlocked program read fails while a concurrent writer stages its journal | The watcher would cancel a healthy stage | Reads may fail for up to 30 seconds of continuous failure before the stage stops; heartbeat refusals still stop it at once (CAL-V0-086) |

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
| CAL-V0-052 | `TestCALV0052_RunNormalizesShutdown`, `TestCALV0052_DecodeConfigIsClosedAndBounded`, `TestCALV0052_RenderIsSinglePass`, `TestCALV0052_EscalationConfigRefusesUnsupportedModels` (`internal/tasks/dispatch`); `TestCALV0052_DispatchCLIClaimHandoffAndStatus`, `TestCALV0057_DispatchStatusShowsTierAndStreak` (`internal/tasks/cli`) |
| CAL-V0-053 | `TestCALV0053_WorkStateReaders`, `TestCALV0053_ReaderReleased`, `TestCALV0053_ReaderFixedRetirementBound`, `TestCALV0053_ReaderExpiredBeforeRetirement`, `TestCALV0053_ReaderHoldIsSticky`, `TestCALV0053_ReaderMarkerRefusesUnsafeEvidence`, `TestCALV0053_ReaderMarkerIdentityAndSetup`, `TestCALV0053_ReaderLocalTimeout`, `TestCALV0053_ReaderClearFailureQuarantines`, `TestCALV0053_ReaderFilesystemFailures`, `TestCALV0053_ReaderFailedDiagnostic`, `TestCALV0053_ReaderDescendants`, `TestCALV0053_CancelledTickKeepsState`, `TestCALV0053_CancelledReobservationKeepsDurableState`, `TestCALV0053_EndedContextDoesNotObserve` (`internal/tasks/dispatch`); `TestCALV0053_DispatchReaderCLIQuarantineAndStatus`, `TestCALV0053_DispatchReaderCLIClearFailure`, `TestCALV0053_DispatchReaderCLIReleased`, `TestCALV0053_DispatchReaderCLICrashRetainsQuarantine` (`internal/tasks/cli`); #464 evidence and limits below |
| CAL-V0-054 | `TestCALV0054_RosterIsDeterministicAndCapped`, `TestCALV0054_RosterStatePredicatesAndLanes`, `TestCALV0054_RosterTierCapsWaitWithoutDowngrade`, `TestCALV0054_TierCapPrecedesPressureBudget` (`internal/tasks/dispatch`) |
| CAL-V0-055 | `TestCALV0055_LaunchFinishBackoffAndPark` (`internal/tasks/dispatch`); live OpenCode run in `docs/build-log/2026-10-01-tasks-continuous-dispatch.md`; live Claude Code and Codex runs in `docs/build-log/2026-10-01-tasks-dispatch-claude-code.md` and `docs/build-log/2026-10-01-tasks-dispatch-codex.md` |
| CAL-V0-056 | `TestCALV0056_CancelledHealingStopsNextWrite`, `TestCALV0056_HandoffAndReap`, `TestCALV0056_KillsWholeTreeAndAdoptsAcrossRestart`, `TestCALV0056_KillsOrphanedProcessesBySession`, `TestCALV0056_IdentityOutageAndUnknownState` (`internal/tasks/dispatch`); `TestCALV0052_DispatchCLIClaimHandoffAndStatus` (`internal/tasks/cli`) |
| CAL-V0-057 | `TestCALV0057_FingerprintIgnoresNonDurableAttempts`, `TestCALV0055_LaunchFinishBackoffAndPark`, `TestCALV0057_EscalationLadderClimbsAndResetsOnProgress`, `TestCALV0057_StickyTierAndOperatorUnparkKeepStreak`, `TestCALV0057_ProgressOutsideSessionResetsLadder`, `TestCALV0057_NoLadderKeepsLegacyLedger`, `TestCALV0057_LedgerEscalationIsBounded` (`internal/tasks/dispatch`); `TestCALV0057_DispatchStatusShowsTierAndStreak` (`internal/tasks/cli`); live model-host qualification NOT_RUN |
| CAL-V0-058 | Event assertions in `TestCALV0055_LaunchFinishBackoffAndPark`, `TestCALV0056_HandoffAndReap`, `TestCALV0056_KillsWholeTreeAndAdoptsAcrossRestart`, `TestCALV0058_SummaryReadsHostFinalText`, `TestCALV0057_EscalationLadderClimbsAndResetsOnProgress` (`internal/tasks/dispatch`) and `TestCALV0052_DispatchCLIClaimHandoffAndStatus` (`internal/tasks/cli`) |
| CAL-V0-068 | `TestCALV0068_RosterPressurePrecedence`, `TestCALV0068_HeldRespectsStaticCaps`, `TestCALV0068_ConfigValidation`, `TestCALV0068_DispatcherThrottlesNewLaunchesOnly`, `TestCALV0068_RestartKeepsLevelAndDisableClears`, `TestCALV0068_LedgerPressureRecordValidated`, `TestIssue497_PressureHysteresis`, `TestIssue497_UnknownRetainsLevel`, `TestIssue497_NormalizesHostCPU`, `TestIssue497_MixedMetricTransitions`, `TestIssue497_NonExemptBudget`, `TestIssue497_InvalidConfig`, `TestIssue497_SampleParsing`, `TestIssue497_BoundedFileReads`, `TestIssue497_CommandFixture`, `TestIssue497_CommandLifecycle`, `TestIssue497_LiveSampler` (`internal/tasks/dispatch`); `TestCALV0068_DispatchStatusPressure` (`internal/tasks/cli`); live Linux sampling NOT_RUN |
| CAL-V0-109 | `TestCALV0109_StickySwapWithNormalPressureReleases`, `TestCALV0109_KernelMemoryLevelMapping`, `TestCALV0109_DarwinSysctlParsing`, `TestCALV0110_LedgerReasonAndMemoryLevelValidated`, `TestIssue497_LiveSampler` (`internal/tasks/dispatch`); live macOS dispatcher under memory pressure NOT_RUN |
| CAL-V0-110 | `TestCALV0110_ReasonRecordedAtLevelChange`, `TestCALV0110_ThrottledEventReportsReason`, `TestCALV0110_LedgerReasonAndMemoryLevelValidated` (`internal/tasks/dispatch`); `TestCALV0110_DispatchStatusReason` (`internal/tasks/cli`) |
| CAL-V0-069 | `TestGH494_LeaseTimingIsOptInDiagnostic` (`internal/tasks/cli`); `TestGH494_LeaseTimingSumsEveryTransaction`, `TestGH494_TimedOutClaimReplaysExactly` (`internal/tasks/store`) |
| CAL-V0-059 | `TestCALV0059_CheckpointCodecAndDerivation` (`internal/tasks/journal`) |
| CAL-V0-060 | `TestCALV0060_WritersRetainACheckpointReadsResumeFromIt` (`internal/tasks/cli`), including `pending`, which shares the lease audit with writers |
| CAL-V0-061 | `TestCALV0061_CheckpointTailEqualsFullAudit`, `TestCALV0061_CheckpointFallsBackToFullAudit`, `TestCALV0061_CheckpointScopeAndMovement`, `TestCALV0061_CheckpointLimitsStayWithFullAudit` (`internal/tasks/journal`); `TestCALV0060_WritersRetainACheckpointReadsResumeFromIt` (`internal/tasks/cli`); live-store measurement in `docs/build-log/2026-10-01-tasks-read-checkpoint.md` |
| CAL-V0-062 | `TestCALV0062_PolicyEffortAllowlist` (`internal/tasks/intent`); `TestCALV0062_StageEffortSelection`, `TestCALV0062_CheckProgramConfigEffort`, `TestCALV0062_BaseEffortRequired`, `TestCALV0062_StageRechecksCurrentPolicy`, `TestCALV0062_OpenWorkflowRefusesBeforeMutation` (`internal/tasks/store`); live Codex at non-low effort NOT_RUN |
| CAL-V0-063 | `TestCALV0063_PolicyStageWallBound` (`internal/tasks/intent`); `TestCALV0063_CheckProgramConfigStageWall`, `TestCALV0062_OpenWorkflowRefusesBeforeMutation` (`internal/tasks/store`); live stage beyond one hour NOT_RUN |
| CAL-V0-064 | `TestCALV0064_CommandGrammarAndRoleSeparation`; `TestCALV0064_ChangedFileReplayAndRestart`; `TestCALV0064_CheckedSaveFailureDoesNotGrantOrEscapeThroughClose`; `TestCALV0064_LaterSaveFailureCannotReviveGrantedParking`; `TestCALV0064_ActivePendingUnknownAndSeedAccounting`; `TestCALV0064_FirstSeedIsNotProgressAndCancellationIsNotAdmission`; `TestCALV0064_FirstSeedEndedWorkerAndLaterFailure`; `TestCALV0064_CanceledReobservationCannotAdmitEarlierToken`; `TestCALV0064_PostCommitCancellationPreservesFactsAndStopsEffects`; `TestCALV0064_CapacitySortedAllocationAndStrictLoad`; `TestCALV0064_NoTokenPreservesLegacyLedgerAndFingerprint`; `TestCALV0064_LedgerCanonicalFieldsAndCaseSensitiveKeys`; `TestCALV0064_KeyBoundaryAndOperatorUnparkRetainLifetimeBudget` (`internal/tasks/dispatch`); `TestCALV0064_DispatchCLIFileProgressAndReplay` (`internal/tasks/cli`); manual source/test evidence in `docs/build-log/2026-10-02-dispatch-explicit-progress.md`, optional OCM linkage unassessed |
| CAL-V0-065 | `TestCALV0065_AbsentPreimage`, `TestCALV0065_RequestShapeAndCurrentMembership`, `TestCALV0065_AllocationPreviewAndPreparedAdmission` (`internal/tasks/transaction`); `TestCALV0065_HealthFiltersEveryRound`, `TestCALV0065_ReplayAfterSuccessorAndPolicyChange`, `TestCALV0065_ClaimNextSelectors` (`internal/tasks/store`); `TestCALV0065_CLIExclusionsAndPreviewPurity`, `TestCALV0065_NativeFixture` (`internal/tasks/cli`); scoped evidence and limits in `docs/build-log/2026-10-02-tasks-member-exclusions.md` |
| CAL-V0-071 | `TestCALV0071_PolicyRepositories` (`internal/tasks/intent`); `TestCALV0071_ProgramRepositoryRecords` (`internal/tasks/snapshot`); `TestCALV0071_RepositoryBindingImmutable` (`internal/tasks/transaction`); `TestCALV0071_CheckProgramConfigRepositories`, `TestCALV0071_WritableRoots`, `TestCALV0071_UndeclaredRepositoryRefusedBeforeMutation`, `TestCALV0071_ExtraRepositoryPathsAreScoped`, `TestCALV0071_RetargetedCheckoutRefusedBeforeWrite`, `TestCALV0071_MultiRepositoryProgramFakeHost` (`internal/tasks/store`); live Codex NOT_RUN |
| CAL-V0-072 | `TestCALV0071_MultiRepositoryProgramFakeHost` (composite tree, unmoved checkout `HEAD`, review binding), `TestCALV0072_MultiRepositoryGatesFailClosed` (dirty sibling refuses its gate) (`internal/tasks/store`); live Codex NOT_RUN |
| CAL-V0-087 | `TestCALV0071_ProgramRepositoryRecords` (designation record) (`internal/tasks/snapshot`); `TestCALV0071_CheckProgramConfigRepositories` (designation label), `TestCALV0071_MultiRepositoryProgramFakeHost` (undesignated `UNSUPPORTED`), `TestCALV0072_MultiRepositoryGatesFailClosed`, `TestCALV0087_DesignatedMultiRepositoryIntegration`, `TestCALV0087_ExtraWorktreeCleanup`, `TestCALV0087_GrantMustNameEveryTarget`, `TestCALV0087_DesignatedTargetChecked`, `TestCALV0087_InterruptedIntegrationLandsOnce`, `TestCALV0087_UnchangedRepositoryNeedsNoDesignation` (`internal/tasks/store`); live Codex NOT_RUN |
| CAL-V0-088 | `TestCALV0088_ExtraRepositoryContextRequired`, `TestCALV0087_DesignatedMultiRepositoryIntegration` (`internal/tasks/store`); live Codex NOT_RUN |
| CAL-V0-089 | `TestCALV0089_PolicyContinuationsBound` (`internal/tasks/intent`); `TestCALV0089_InterruptedSessionCapability` (`internal/tasks/supervisor`); `TestCALV0089_StageRechecksContinuations`, `TestCALV0089_CodexContinuationResumesPreservedSession`, `TestCALV0089_OpenCodeContinuationForksPreservedSession`, `TestCALV0089_ContinuationBoundThenOperatorRestart`, `TestCALV0089_TurnCapsBoundContinuation`, `TestCALV0089_DrainStopsContinuation`, `TestCALV0089_ProgramWallExpiryEndsContinuation`, `TestCALV0089_IntegrateCheckpointRestartKeepsGrant`, `TestCALV0089_UnsupportedContinuationRefusedBeforeMutation` (`internal/tasks/store`); live Codex and OpenCode NOT_RUN |
| CAL-V0-074 | `TestCALV0074_PolicyHost` (`internal/tasks/intent`); `TestCALV0074_CapsuleHost`, `TestCALV0074_RuntimeReplacedAtAck`, `TestCALV0074_ACLProbe`, `TestCALV0074_PrelaunchErrorOnlyBeforeSpawn` (`internal/tasks/supervisor`); `TestCALV0074_CheckProgramConfigHost`, `TestCALV0074_OpenWorkflowRefusesHostBeforeMutation`, `TestCALV0074_AdmissionRunsLaunchCheck`, `TestCALV0074_SpawnedFailureIsNotNoExec`, `TestCALV0074_NoExecCancelBeforeRelease`, `TestCALV0074_NoExecCrashTakeover`, `TestCALV0074_HostSwitchAndRollback` (`internal/tasks/store`); `TestCALV0074_RunHostFlag` (`internal/tasks/cli`) |
| CAL-V0-075 | `TestCALV0075_ClaudeResultVocabulary`, `TestCALV0075_ClaudeUsageObservedOrUnknown`, `TestCALV0075_ClaudeDuplicateMembers` (`internal/tasks/supervisor`); `TestCALV0075_ClaudeStageArgv`, `TestCALV0075_ClaudeCodeProgramFakeHost` (`internal/tasks/store`); live Claude Code NOT_RUN |
| CAL-V0-076 | `TestCALV0076_PolicyHostOpenCode` (`internal/tasks/intent`); `TestCALV0076_OpenCodeVocabularySelected` (`internal/tasks/supervisor`); `TestCALV0076_CheckOpenCodeConfig`, `TestCALV0076_CheckProgramConfigOpenCodeHost`, `TestCALV0076_OpenCodeHostRollback` (`internal/tasks/store`); `TestCALV0076_ConfigHostFlag` (`internal/tasks/cli`) |
| CAL-V0-077 | `TestCALV0077_OpenCodeResultVocabulary`, `TestCALV0077_OpenCodeUsageObservedOrUnknown`, `TestCALV0077_OpenCodeDuplicateMembers` (`internal/tasks/supervisor`); `TestCALV0077_OpenCodeUsageIncompleteAccounting`, `TestCALV0077_DetachedHostEnvRequired`, `TestCALV0077_DetachedServerTimeout`, `TestCALV0077_DetachedServerForcedKill`, `TestCALV0077_DetachedServerHostCrash`, `TestCALV0077_DetachedOrphanFailsClosed`, `TestCALV0077_EscapeObservationUncertain`, `TestCALV0077_EscapeGroupReuse`, `TestCALV0077_RecoverDetachedHost`, `TestCALV0077_RecoverDetachedLateEscape` (`internal/tasks/supervisor`); `TestCALV0077_OpenCodeStageArgv`, `TestCALV0077_OpenCodeProgramFakeHost`, `TestCALV0077_OpenCodeResumeRequiresFork`, `TestCALV0077_OpenCodeOutputLimitUsageUnknown` (`internal/tasks/store`); live OpenCode NOT_RUN |
| CAL-V0-098 | `TestCALV0098_PreimageBindsMode`, `TestCALV0098_RequestShape`, `TestCALV0098_DeriveAuthors` (`internal/tasks/transaction`); `TestCALV0098_ClaimExcludesImplementAuthor`, `TestCALV0098_ClaimNextExcludesAuthors`, `TestCALV0098_HealthSkipsAuthor`, `TestCALV0098_HealthPrepareRederivesAuthors` (`internal/tasks/store`); `TestCALV0098_CLIExcludeAuthors`, `TestCALV0098_ExhaustedPoolParity` (`internal/tasks/cli`); `TestAgentLeasesSpecEnumeratesCALV0098` (`internal/lrfrepo`) |
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
| CAL-V0-070 | Implemented for `store.Mutate` and journal audit reads: `TestCALV0070_MergedMutationAuditEquivalence` (`internal/tasks/journal`), `TestCALV0070_PinnedDirOpensMatchInRoot` (`internal/tasks/safeopen`), `TestCALV0070_MutateAuditSequenceEquivalence`, `TestCALV0070_MutateRefusesChangesAfterMergedAudit`, `TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit` and `TestCALV0070_MutateRetriesAuditWithoutWatch` (`internal/tasks/store`, the last two also in a Linux arm64 container); before/after `BenchmarkCALV0070_MutateAt2000Receipts` (median `Mutate` CPU 4,178 → 1,793 ms at 2,000 receipts; with the change watch, 5,381 → 2,299 ms at load 27–59; with the content check, 5,494 → 3,151 ms (42.6%) on macOS at load 34–52 and 4,019 → 2,547 ms (36.6%) in a Linux arm64 container) and opt-in `TestCALV0070_WriterHistoryProfile` (`internal/tasks/store`); see `docs/build-log/2026-10-04-tasks-writer-history-cost.md` and `docs/build-log/2026-10-04-tasks-writer-one-pass.md`. The proposed writer checkpoint is NOT_RUN (owner decision pending, deferred 2026-10-04); the Linux profile, the live store and the issue 545 waves are NOT_RUN |
| CAL-V0-097 | `TestCALV0097_PoolWaitingTicketsDoNotConsumeWindow`, `TestCALV0097_CapAtFreeEligibleMembers`, `TestCALV0097_UnobservedPoolStateDefers`, `TestCALV0097_UndeclaredPoolBlocks`, `TestCALV0097_PoolPlanUnchanged`, `TestCALV0097_UnclaimablePoolsDoNotConsumeWindow` (`internal/tasks/transaction`); `TestCALV0097_DefaultPreviewIsResourceAware`, `TestCALV0097_DispatchRoutesPoolTicketsAndKeepsLaneFreeProgress` (`internal/tasks/cli`); `TestCALV0097_RosterRoutesPoolTicketsToMatchingRole` (`internal/tasks/dispatch`); `TestCALV0097_CoreDecodesResourceDeferredPlan` (`internal/taskman`); `TestAgentLeasesSpecEnumeratesCALV0097` (`internal/lrfrepo`) |
| CAL-V0-101 | `TestCALV0101_ExplicitPooledClaimYields`, `TestCALV0101_FlagOffMatchesNMinusOne`, `TestCALV0101_PlanClaimAndClaimNextAgree`, `TestCALV0101_UnobservedCompetitorIsNotObserved` (`internal/tasks/transaction`); `TestCALV0101_PriorityYieldThroughTheCLI` (`internal/tasks/cli`) |
| CAL-V0-102 | See the V1-0791 amendment table: intent, snapshot, transaction (including the pinned N-1 digest), store, cli, dispatch, wire and lrfrepo tests |
| CAL-V0-103 | See the V1-0791 amendment table: owner reopen in store and cli tests; dispatcher once-per-episode event and closed ledger member in dispatch tests |
| CAL-V0-104 | See the V1-0851 amendment table: worker-exit release retry, reap, fenced resolution and switch-off tests in `internal/tasks/dispatch` |
| CAL-V0-105 | See the V1-0853 amendment table: planner, dispatcher and native-observation replan tests |
| CAL-V0-107 | `TestCALV0107_ExplicitMembersCoverUnrecordedGenerations`, `TestCALV0098_DeriveAuthors` (`internal/tasks/transaction`); `TestCALV0098_ClaimExcludesImplementAuthor`, `TestCALV0098_ClaimNextExcludesAuthors`, `TestCALV0098_HealthSkipsAuthor`, `TestCALV0107_HealthPrepareNeverImpliesCover` (`internal/tasks/store`); `TestCALV0107_CLIClaimReportsCoveredGenerations`, `TestCALV0098_CLIExcludeAuthors`, `TestCALV0098_ExhaustedPoolParity` (`internal/tasks/cli`) |
| CAL-V0-108 | `TestCALV0108_WaitingReviewOutranksArrivingImplement`, `TestCALV0108_EarlierHandoffWinsAtEqualPriority`, `TestCALV0108_AdmissionRank`, `TestCALV0108_DownstreamCompetitorNeedsItsStageMember`, `TestCALV0101_PlanClaimAndClaimNextAgree`, `TestCALV0101_FlagOffMatchesNMinusOne` (`internal/tasks/transaction`) |
| CAL-V0-111 | `TestCALV0111_CallerWaitBound`, `TestCALV0111_CallerWaitOutlastsDefault` (`internal/tasks/authority`); `TestCALV0111_OrphanCleanupSpendsCallerWait` (`internal/tasks/store`); `TestCALV0111_LockWaitFlag`, `TestCALV0111_LockWaitBoundsContendedRelease` (`internal/tasks/cli`) |
| CAL-V0-112 | `TestCALV0112_HandoffReleaseReplaysAfterLockTimeout` (`internal/tasks/store`); `TestCALV0112_SameRequestHandoffReplayAfterLockTimeout` (`internal/tasks/cli`) |
| CAL-V0-113 | `TestCALV0113_PlainReleaseAfterTimedOutHandoffIsCharged` (`internal/tasks/cli`) |
| CAL-V0-125 | `TestCALV0125_CPUUtilizationDelta`, `TestCALV0125_LinuxCPUTicksParsing`, `TestCALV0125_DispatcherFirstTickUnknown`, `TestCALV0125_LinuxSamplingFollowsSelection`, `TestCALV0125_DarwinSamplingFollowsSelection`, `TestCALV0125_UnselectedCPULeavesLedgerUnchanged` (`internal/tasks/dispatch`); `TestCALV0125_DispatchStatusCPUUtilization` (`internal/tasks/cli`); macOS ticks NOT_DELIVERED, live Linux sampling NOT_RUN |
| CAL-V0-126 | `TestCALV0126_SignalSelection`, `TestCALV0126_SignalSelectionValidation` (`internal/tasks/dispatch`) |
| CAL-V0-127 | `TestCALV0127_ConfigReloadAppliesAndRefuses`, `TestCALV0127_ReloadRemovedRoleKeepsWorkers`, `TestCALV0127_ConfigRecordValidation`, `TestCALV0127_ConfigRecordBesideStrictRecords`, `TestCALV0127_ReloadKeepsLaunchDeadlines`, `TestCALV0127_ReloadRemovedRoleKeepsDeadlines` (`internal/tasks/dispatch`); `TestCALV0127_DispatchStatusConfigRecord`, `TestCALV0127_DispatchStatusWithInvalidConfigFile` (`internal/tasks/cli`) |
| CAL-V0-128 | `TestCALV0128_CapZeroDisablesRole` (`internal/tasks/dispatch`); `TestCALV0128_DisabledRoleDoesNotStarveEnabledRole` (`internal/tasks/cli`) |
| CAL-V0-129 | `TestCALV0129_LaneMinAge` (`internal/tasks/dispatch`) |
| CAL-V0-086 | `TestCALV0086_AttemptWorktreePathIsPathText` (`internal/tasks/snapshot`); `TestCALV0086_LongWorkRootStageDispatches`, `TestCALV0086_OverlongWorktreeRefusedBeforeMutation`, `TestCALV0086_UnprovedStopIsNotFinished`, `TestCALV0086_WatcherToleratesTransientReadFailure` (`internal/tasks/store`); `TestCALV0086_DrainWaitsOutUnprovableGroupProbe`, `TestCALV0086_DrainProvesReapedZombieGroupGone` (Darwin) (`internal/tasks/supervisor`); acceptance `go test -count=10 -run TestCALV0072_MultiRepositoryGatesFailClosed` under a 113-byte resolved `TMPDIR` and concurrent load, see `docs/build-log/2026-10-05-tasks-multirepo-continuation.md` |

## Holder, retry and policy observation acceptance


Issue 494 preparation admission has focused and independent source review plus ten reached protocol cases on local Darwin/APFS and Linux/arm64 Colima/tmpfs. Source snapshot `82d8d532f7f537e93af2889d2fc1468a7953e9df0ccc39ff8e690e3c499c9e92` binds those protocol runs. The only later source change repairs the mixed harness replay receipt assumption; the tiny `TestGH494CLIResultContract` passed its parent and eight cases with exact captured identities on source snapshot `e716f41b793d1ad6517eeb47d3601fd227c4ee6357500e50b5fe94a07886a77b`. The original mixed Go test remains failed; independent raw readback establishes 30/30 operations, full 5000→5030→5030 audits, ten fenced attempts, empty reservations and exact-ID replay without changed bytes. No repeated mixed wave is implied. Current-source CAL-V0-026 measurement met the original threshold on local macOS with Go 1.27.1 and GOMAXPROCS=2: 3000 tickets, 20 samples per verb, claim p95 108.467584 ms and renew p95 97.003583 ms. All 105 host-load observations were below 12 CPUs. Raw measurement SHA256 `25b0076542c81b7bcce9c62badbc2f0bb9f18732eb821058218e64036830eecd`, host observation SHA256 `3a29706d5f4abee9d20313fd0f499a326a6495bcf26a4d6aef6b09114c54fd33` and source manifest `e716f41b793d1ad6517eeb47d3601fd227c4ee6357500e50b5fe94a07886a77b` bind this result. Linux and default-parallelism performance remain unqualified. Committed pending-receipt redo and the other four selected checks are recorded through the keyed post-commit plan in the build log; their results must be read from receipts bound to the actual commit. Terminal independent review, CEM/check/seal, integration and native completion remain pending. No mixed-version fairness, CLI arrival order or universal starvation freedom is claimed. See `docs/build-log/2026-10-04-gh494-fair-preparation-admission.md`. Rollback retires owned work before reverting code, while preserving store/journal/intent and inert coordination files.

Issue 545 (claims exiting SNAPSHOT_MOVED after minutes of preparation at about 7,140 receipts) was reported against `32aa5a3f2b7f3fe658d208049fb009e2c9e24c13`, which predates issue 494 admission. The opt-in `TestGH545ClaimStarvation` (`GH545_CLI`, `GH545_EVIDENCE_DIR`; defaults 7,140 receipts, ten fresh-process writers, claim/heartbeat/renew/heartbeat/release, 60-second per-operation bound) retains whole-operation time and outcome. One local Darwin wave per binary: the reported commit completed 25 of 50 operations, five refused SNAPSHOT_MOVED and the worst ran 360,063 ms; the admission source at `a5bb0d8fe4f5748f45181f49cb3c9e8e877b3d7d` completed 50 of 50 with the worst at 32,959 ms. This changes no behaviour or requirement. It is not a latency budget: an uncontended operation still costs about 3 seconds at this history, so the ten-writer queue sits at the 30-second admission budget and a 14-writer wave refused three writers LOCK_TIMEOUT (55 of 70). History-independent mutation cost, Linux, p95 and fairness remain unqualified. See `docs/build-log/2026-10-04-gh545-claim-starvation-qualification.md`.


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

## #464 command-reader lifecycle amendment

This records the experimental implementation of the existing CAL-V0-053 repair
under GitHub #464 / V1-0654. It adds no requirement ID or new acceptance of the
proposed `process-group-owner-v0.md` contract. The accepted work-state limits and
UNKNOWN/alert behavior above remain in force. Final integration qualification is
pending; see `docs/build-log/2026-10-04-dispatch-reader-lifecycle-integration.md`.

A command reader holds one creation-owned process slot through result handling.
After command exit or cancellation, retirement uses one fixed deadline: the earlier
of command deadline plus one second and retirement trigger plus one second. Only
Owner RELEASED permits inspecting output and returning ordinary read results.
Unproved retirement retains UNKNOWN and poisons the dispatcher invocation;
subsequent Tick/Run calls stop, including bounded runs and canceled contexts.
Close releases the lock through the terminal UNKNOWN path without normal ledger
or event writes. Cancellation also stops subsequent healing writes; it does not
roll back an already completed mutation.

Before spawning, the locked program directory receives a bounded canonical
`reader-lifecycle.json` marker with profile `taskman-dispatch-reader-lifecycle/0`,
program, random run identifier and lifecycle UNKNOWN. It contains no PID authority.
Restart checks refuse unresolved, malformed, symlink or nonregular evidence before
owner/ledger/event writes or worker adoption. Only matching file identity and bytes
are cleared after RELEASED/result handling or a start failure that owns no child.
Publication or clear failure stops this invocation; failed diagnostics do not clear
quarantine. A publication failure before creation need not leave a marker because
no reader was spawned. There is no automatic recovery or marker-clear endpoint.

Read-only status separates `readerContainment`, `readerQuarantined` and
`readerDiagnostic` from dispatcher PID liveness. Marker absence is NOT_OBSERVED,
not proof of RELEASED. Exported ReadStates command calls retain the same boundary
under the private `work-state-reader` program; status-line reads retain their
existing behavior. Unsupported process-owner platforms refuse before creation.

The marker covers process restart/crash visibility, not power loss. Old binaries,
manual deletion and alternate state directories can bypass it. No escaped-session
or host-wide containment, immortal handle retention, Linux amd64 runtime, or
promotion of the Owner dependency is claimed. Rollback requires quiescing owned
readers and preserving unresolved markers/evidence; reverting code alone does not
prove cleanup or make an older reader safe.

## V1-0751 CREATE payload template amendment

Authoritative input: ticket V1-0751 (owner-filed friction, 2026-10-04): `ticket create --help`
names the 20 CREATE keys and canonical rules, but value types, the nested `effects` and `source`
shapes, null-able keys and the `order` string were discoverable only by refusal, and
`source.sourceQueueId` had to be guessed from `queue.json`. This amendment adds one read-only
command. It changes no wire profile, payload key, validation rule or writer.

- `CAL-V0-073`: `corvint-tasks ticket create --template` MUST return, in one
  `taskman-command-result/0` item, a byte-canonical CREATE `payload` for the current queue (also
  as the exact string `payloadCanonical`) whose `source` is `NATIVE` with `sourceQueueId` read
  from `queue.json`, and which `ticket create` accepts once `title`, `body` and
  `acceptanceCriteria` (named in `fill`) are filled in. The template title is empty, so an
  unedited template refuses. A separate `fields` object MUST document every CREATE key, the
  optional `localToken`, `requiredRoles` and `requiresPool`, and the nested `effects`, `source`,
  `effects.resources[]` and `dependencies[]` members by dotted path, each with its type and
  `nullable` flag. `values` MUST mean only the closed enum of that key's own scalar value, taken
  from the vocabularies the decoder enforces (gate and pool values from the current policy);
  `elementValues` is the closed set each array element is drawn from (`requiredGates`, and the
  roles inside `requiredRoles`); `current` is this queue's value of an open field
  (`source.sourceQueueId`, an Identifier). `nullableKeys` lists the paths marked nullable;
  `dependencies[].gateId` is nullable only with obligation `COMPLETED`, as its note says.
  The command reads only the intent store: it MUST NOT read stdin or the journal, take the lock,
  or write any file, works before `init`, refuses `--template` on other verbs and when any other
  flag is passed (by presence, whatever its value), and `ticket create --help` names it.

Non-goals: templates for other verbs, interactive prompting, defaults inferred from history, and
any change to CREATE validation or canonicalization. Failure modes, as the tests enforce them: the
`fields` keys MUST equal the key paths of the emitted payload (with one `dependencies[]` and one
`effects.resources[]` element populated) plus `optionalKeys`, and each of the CREATE keys in
`mutation.PayloadKeys`; each payload path marked nullable MUST be accepted as null by `ticket
create` and each path marked non-nullable MUST be refused as null. The nullability of the optional
`localToken`, `requiredRoles` and `requiresPool` is documented but not probed, and `type`, `note`
and enum `values` text is checked only for the enum vocabularies listed in the test. A decoder key
the template does not render is not detected unless it is a `mutation.PayloadKeys` entry. Rollback
removes the flag; no store, wire or journal state depends on it.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-073 | `TestCALV0073_CreateTemplateIsReadOnlyAndAccepted`, `TestCALV0073_CreateTemplateNamesEnumsAndNullableKeys`, `TestCALV0073_TemplateRefusesOtherVerbsAndFlags`, `TestCALV0073_TemplateFieldsMatchPayloadNullability` (`internal/tasks/cli`) |

## V1-0793 critical-path read amendment

Authoritative inputs: owner request [issue 588](https://github.com/beamfall/corvint/issues/588)
(ticket V1-0793) and owner answer 2026-10-05 (D8): a new profile `taskman-critical-path/0`,
bounded to 256 nodes and 32 chains, with no duration estimates in v0. On 2026-10-04 two handoffs
circulated a wrong path to a gate because finding the real open dependencies took manual
`ticket show` walks. This amendment adds one read verb. It changes no record, journal, writer,
plan or existing wire profile.

The normative requirements CAL-V0-079 to CAL-V0-081 are defined in the Requirements section under
"V1-0793 critical-path read (issue 588)"; this amendment records their scope and evidence.

Non-goals: duration or completion-time estimates (D8 defers them; `estimate` stays
`NOT_OBSERVED`), dispatcher-private progress tokens (CAL-V0-064 history is not read), execution
prerequisites and loop or escalation holds before their own specs land, owner-set bounds, paging,
and any change to eligibility, planning or claim behavior.

Failure modes: an unknown or foreign ticket refuses as `ticket show` does; a malformed argument is
`MALFORMED`; a dependency cycle never loops the walk; a missing dependency remains a
`DEPENDENCY_MISSING` blocker on its dependent and is not a node; an unobservable fact is
`NOT_OBSERVED`, never inferred. The 256-node bound keeps the item well under the 16 MiB list-result
bound even when every node has the maximum 64 dependencies. Rollback removes the verb; no store,
journal or wire state depends on it.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-079 | `TestCALV0079_CriticalPathChainsAndNodeFacts`, `TestCALV0079_CriticalPathLiveAttemptFacts` (`internal/tasks/cli`) |
| CAL-V0-080 | `TestCALV0080_CriticalPathBoundsAndCycles`, `TestCALV0080_CriticalPathLargeCycleIsBounded` (10,000-ticket ring) (`internal/tasks/cli`) |
| CAL-V0-081 | `TestCALV0081_CriticalPathBlockersMatchPlanner` (pool, coverage, gate observation, archived completion, budget; `plan preview` parity), `TestCALV0081_CriticalPathRetryAndPauseMatchPlanner`, `TestCALV0079_CriticalPathChainsAndNodeFacts` (purity and journal-absent `NOT_OBSERVED`), `TestCALV0079_CriticalPathLiveAttemptFacts` (purity with a live journal), `TestCALV0047_AllCommandHelpIsReadOnly` (`internal/tasks/cli`) |

## V1-0781 preparation admission pressure amendment

Authoritative input: the owner's 2026-10-05 comment on issue 494, filed as ticket V1-0781. At about
8,900 receipts with up to 13 concurrent supervised sessions, a pooled claim returned
`LOCK_TIMEOUT` after 30 seconds with `phase=queue registered=true rank=13`. The owner asked
`queue status` to expose the current admission rank and estimated wait so that supervisors can
back off before they queue. This amendment adds one read-only observation to `queue status`. It
changes no admission protocol, slot format, writer behavior, journal, intent or existing field.

The normative requirement is `CAL-V0-095` in the Requirements section (V1-0781 subsection).

Non-goals: no change to admission order, fairness, capacity, deadlines or the slot record; no
wait estimate from inter-receipt timing, host load or guesswork; no counting of older clients
that bypass registration; no backoff policy or dispatcher throttle (a supervisor decides); no
pressure history or ledger; no Windows observation.

Failure modes: each slot is observed at a different instant. A registrant can publish, or a
holder can retire, during the sweep. A registry-held scan briefly probes free slots by `flock`,
so a free slot with a stale record can be counted; `registryActive` flags this window. Older
clients that take the final gate without registering are invisible. A Linux reader outside the
initial PID namespace, as in an ordinary container, abstains rather than under-count writers
in other namespaces. Even in the initial namespace, Linux hides a `flock` whose locking process
exited while another process still holds the file description. Corvint descriptors are
close-on-exec and Go does not fork without exec, so a registration cannot outlive its locker that
way, but a foreign process doing so is not detected. An entry for the same inode on another
device refuses as ambiguous rather than reporting absence. A Darwin POSIX record lock on a
coordination file makes the read abstain. Darwin's private OFD-style locks are unqualified and
may read as `flock`. A lock table larger than 4 MiB, a missing `/proc` or an unsupported
platform reports `NOT_OBSERVED`. Observation over network filesystems is unqualified.

Acceptance evidence: on Darwin/APFS, and on Linux/arm64 in Colima containers over overlayfs and
tmpfs, both in a container PID namespace and with the host PID namespace, the tests below show the
following. Four real waiters queued behind a holder report five
registered writers and would-be rank six. Every coordination file keeps identical bytes, mode and
modification time, and the fixture journal and intent audits are unchanged. Retired slots with
stale records count zero. A registration held by another process is counted. A partial live
record hides the rank. An unsafe slot object, an unavailable lock query, an incomplete Linux lock
table, a Darwin record lock, and a slot or registry that disappears or is replaced after its
first stat or after its open, and a common directory renamed away and replaced after the root is
pinned, each report `NOT_OBSERVED`. Linux POSIX and OFD record locks on stale slots count zero. The status read creates no registry file. Rollback removes the observation and
the field; no store, wire profile or coordination file depends on it.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-095 | `TestCALV0095_PreparationQueueObservation`, `TestCALV0095_PreparationQueueObservationAcrossProcesses`, `TestCALV0095_RecordLockIsNotARegistration`, `TestCALV0095_ProcLocksParse` and `TestCALV0095_ProcLocksCompleteness` (Linux) (`internal/tasks/authority`); `TestCALV0095_QueueStatusAdmissionPressure` (`internal/tasks/cli`) |

## V1-0780 retryable result amendment

Authoritative input: ticket V1-0780, the follow-up the owner filed to
[issue 494](https://github.com/beamfall/corvint/issues/494): at about 8,900 receipts with 13
concurrent sessions, lease renew and heartbeat returned `ERROR`/`LOCK_TIMEOUT` while the lease was
FRESH, and the client runner killed healthy runs. Its client now matches a private list of codes.
The owner asked that Tasks document which result codes are retryable and mark each result with an
explicit `retryable` fact so callers stop pattern-matching codes.

The requirement `CAL-V0-078` is defined in the Requirements section (V1-0780 retryable command
results); this section records its source, classification, limits and evidence.

The classification, as `wire.RetryOf` records it:

| Codes | Retryable | Condition or reason |
| --- | --- | --- |
| `LOCK_TIMEOUT` | yes | Lock or preparation admission held by another writer past the wait; nothing locked or written |
| `SNAPSHOT_MOVED` | yes | Store, head, intent tree or worktree moved during a read or before commit; nothing decided. A release or attestation candidate whose head no longer matches, and a criterion capture wrapping a refused read, repeat until their input changes |
| `REDO_PENDING` | yes | A writer is between receipt link-in and head rename; the next mutating command redoes a crashed writer's receipt |
| `FENCED`, `BOOT_FENCED`, `SUPERVISOR_LOST` | no | Fencing: the attempt or generation really lost; start a new attempt |
| `LIMIT_EXCEEDED` | no | Mostly static size bounds; the transient preparation-slot and active-attempt caps share the code |
| `JOURNAL_SATURATED`, `UNSUPPORTED_FILESYSTEM` | no | Capacity, I/O or filesystem-identity conditions that backoff is not known to clear |
| `STALE_TICKET`, `STALE_POLICY`, `STALE_TREE` | no | A recorded fact moved; re-read and rebuild the request |
| `REQUEST_ID_CONFLICT`, `MALFORMED`, `DUPLICATE_ID`, `CYCLE`, `DEPENDENCY_MISSING`, `INVALID_PRIORITY`, `GATE_UNKNOWN`, `ADOPT_UNSUPPORTED_FIELD`, `OUT_OF_SCOPE`, `UNSUPPORTED`, `UNSUPPORTED_VERSION`, `INTENT_BRANCH_MISMATCH`, `DIRTY_WORKTREE` | no | The request or local input must change first |
| `ATTEMPT_LIVE`, `RESOURCE_COLLISION`, `PAUSED`, `DEPENDENCY_UNSATISFIED`, `TICKET_HELD`, `TICKET_STATE`, `APPROVAL_MISSING`, `APPROVAL_REVOKED`, `CUTOVER_MISSING`, `QUIESCENCE_UNPROVED`, `RETRY_EXHAUSTED`, `GATE_FAILED`, `GATE_STALE`, `MISSING_GATE`, `MISSING_EVIDENCE`, `BUDGET_UNKNOWN`, `COVERAGE_UNKNOWN`, `EXTERNAL_UNBOUNDED`, `UNINITIALIZED`, `RESTORE_INCOMPLETE`, `INTENT_DIVERGED`, `JOURNAL_FORKED`, `ESCALATION_PENDING`, `PREREQUISITE_UNSATISFIED`, `LOOP_DETECTED` | no | Another actor, the owner or a recorded state must change first |
| `NOEXEC`, `SURVIVORS` | no | Execution results that the same inputs reproduce |
| `HANDOFF`, `REVIEW_RETURNED`, `DEVELOPMENT_MODE` | no | Dispositions, not failures |
| `ADJUDICATION`, `BOOT_TIMEOUT`, `BUDGET_EXCEEDED`, `CAPABILITY_UNAVAILABLE`, `CEM_MISSING`, `CONTAMINATED`, `CUTOVER_IN_PROGRESS`, `DOCS_MISSING`, `EFFECT_OWNED`, `INDEPENDENCE_UNVERIFIED`, `OCM_MISSING`, `PLAN_STALE`, `RESTORED`, `REVIEW_INCOMPLETE`, `REVIEW_REJECTED`, `SIGNAL_REFUSED_IDENTITY`, `UNCERTAIN_EFFECT`, `UNPUBLISHED`, `UNRESOLVED_FINDING` | no | Reserved in §11 with no in-tree producer |

The issue 494 client list also names `STALE`, `STORAGE_FAILED` and `HEAD_MOVED`. None is a §11
code: `STALE` is a gate or review status, `STORAGE_FAILED` is a `taskman-outcome/0` mutation
outcome (its envelope carries the code beside it, or none), and `HEAD_MOVED` has no producer. The
member therefore cannot classify them; a `STORAGE_FAILED` mutation reported without a code carries
no `retryable` member.

Non-goals: no new code, no recoding of an existing producer, no change to `outcomeFor`, exit
status, `taskman-outcome/0` or the receipt profiles, no retry or backoff inside Tasks, no member on
`OK` or uncoded results, and no retry budget or deadline advice. `retryable` states that a retry can
succeed, not that it will, and a command that ran a program reports false rather than rerun it on retry. A claim that ran pool
health probes before a later refusal is not covered: a retry of that claim may probe again.
A renew retried after the lease's `expiresAt` can still be refused or fenced.

Failure modes: a new §11 code without a classification fails
`TestCALV0078_ClassificationCoversEveryCode`; a fencing code marked retryable fails
`TestCALV0078_FencingNeverRetryable`. A consumer built before this amendment that decodes the
closed envelope exactly (the earlier `wire.DecodeResult`, or the companion release's 10-key
`taskEnvelope` check) refuses a coded non-`OK` envelope that carries the member; no in-tree
consumer reads such envelopes, because every in-tree closed reader accepts only `OK` results or
the uncoded `attempt` `NOT_RUN`. `LIMIT_EXCEEDED` stays not retryable even though two of its
producers (the 64 live preparation slots and the active-attempt cap) can clear without the caller,
because its other producers are fixed limits; separating them needs a new code. `SNAPSHOT_MOVED` is
retryable, because its usual cause is a concurrent writer. Its stale-input producers (a release or
attestation candidate whose head no longer matches, and a criterion capture that wraps a refused
read) are the named exception: they repeat until the caller changes its input.

Acceptance evidence is the traceability row below plus unchanged bytes for `OK` and uncoded
results under the existing `internal/tasks` tests. Rollback removes the member from
`Result.Value`, the decoder's optional key, `Report.Unretryable`, `PoolSweepReport.Unretryable`, the `wire.Error.NotRetryable` mark and the `attempt run` override; earlier decoders then
read every envelope again, and no store, journal or receipt state depends on it.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-078 | `TestCALV0078_ClassificationCoversEveryCode`, `TestCALV0078_FencingNeverRetryable`, `TestCALV0078_ResultRetryablePresence`, `TestCALV0078_EnvelopeRetryableMember`, `TestCALV0078_WithoutRetryKeepsCode` (`internal/tasks/wire`); `TestCALV0078_RedoPendingReadIsRetryable`, `TestCALV0078_UnrecordedRunIsNotRetryable`, `TestCALV0078_ExecutedGateRunIsNotRetryable`, `TestCALV0078_CommittedSweepIsNotRetryable`, `TestCALV0078_MarkedErrorIsNotRetryable`, `TestCALV0078_AnyMarkedLaneMakesBatchNotRetryable` (`internal/tasks/cli`); `TestCALV0078_GateRunContentionAfterExecutionIsReported`, `TestCALV0078_PoolCommandReportsExecution`, `TestCALV0078_PreparedPoolCommandIsNotRetryable`, `TestCALV0078_SweepResponseLossAfterExecutionIsNotRetryable`, `TestCALV0078_SweepContentionBeforeOwnerCommitIsRetryable`, `TestCALV0078_SweepContentionBeforeObservationCommitIsNotRetryable`, `TestCALV0078_SupervisedGateRecordFailureIsNotRetryable`, `TestCALV0078_SupervisedCheckingFailureIsNotRetryable`, `TestCALV0078_SupervisedRunAfterCommitIsNotRetryable`, `TestCALV0078_IntegratorFailureBeforeLeavingSelectionIsRetryable` (`internal/tasks/store`) |

## V1-0854 batch refine amendment

Authoritative input: owner request [issue 625](https://github.com/beamfall/corvint/issues/625)
(ticket V1-0854). Refining about 110 tickets took 110 separate `ticket refine` processes. Each one
acquired the writer lock, ran the writer guards and §5.2 redo, and audited the store, while
concurrent claims and heartbeats waited behind them. A single all-or-nothing batch would hold the
lock for the whole run instead. This amendment adds a chunked batch form of `ticket refine`. It
changes no wire profile, envelope, payload rule, receipt or replay rule, and it leaves the
single-ticket `ticket refine` unchanged.

- `CAL-V0-106`: `corvint-tasks ticket refine --batch --request-id ID (--payload-stdin | --payload
  JSON) [--issued-at TS] [--role ROLE]` MUST read a non-empty JSON array of at most 1000 entries,
  and at most 8 MiB, each a closed object `{target, expectedRevision, payload}`. `target` is a
  ticket ID or local ID, `expectedRevision` a Count string and `payload` the REFINE payload
  canonicalized as the single form canonicalizes it. Before taking the writer lock, the command
  MUST validate every entry: the entry's shape, its payload, that its target exists in the
  current queue, that no target is named twice, and that the entry's derived envelope decodes. It
  MUST also check that the actor is admitted. Any failure refuses the whole batch with the failing
  entry's path and writes nothing. Entry `i` is the ordinary §3.3 REFINE envelope with request ID
  `ID/i` (the derived IDs must fit `wire.MaxRequestIDBytes`) and the batch's one `issuedAt`. The
  command MUST apply the entries in index order, in chunks. Each chunk acquires the writer lock
  once, runs the writer guards and §5.2 redo once, and starts no entry after 8 entries
  (`store.BatchChunkEntries`) or once it has held the lock for 2 s (`store.BatchChunkHold`). Its
  first entry always runs. Between chunks the command MUST release the lock and wait
  `store.BatchYield` (5 lock polls) before acquiring it again, so a polling claim, renew or
  heartbeat can commit between chunks. Within a chunk, each entry MUST be committed exactly as the
  single form commits it: its own request lookup and replay (TM-V0-006), CAS on
  `expectedRevision`, recorded time sampled again from the command's clock, and receipt.
  A refused entry, such as a stale `expectedRevision` (`REVISION_CONFLICT`) or a request ID
  conflict, MUST write nothing for that entry, and the batch continues. An error, including a
  writer-guard refusal while a chunk opens, MUST stop the batch at that entry and report every
  later entry as `NOT_ATTEMPTED`. The command MUST return one `taskman-command-result/0` item with
  `batchRequestId`, `issuedAt`, `chunkEntries`, `chunks`, the counts `completed`, `replayed`,
  `failed` and `notAttempted`, and an `entries` array index-aligned with the input. Each entry
  gives `index`, `requestId`, `ticketId`, `chunk`, `outcome`, `codes`, `replayed`, `receipt`,
  `resultingRevision` and `detail`. The envelope is `OK` only when every entry completed. It is
  `ERROR` when an entry erred, and otherwise `REFUSED`, with the union of the entry codes. Repeating
  the command with the same `--request-id`, `--issued-at` and input MUST replay every completed
  entry without a second receipt. The same request ID with a different `issuedAt` or entry is
  refused per entry as `REQUEST_ID_CONFLICT`. `--batch` refuses `--target`, `--expected-revision`
  and `--template`, is recognized only in a flag position (never as another flag's value), works only
  on `ticket refine`, and `ticket refine --help` documents it. The 8 MiB bound applies to
  `--payload` and `--payload-stdin` alike.

Non-goals: batch forms of other verbs; one atomic all-or-nothing batch; fairness for the polling
writer lock beyond the yield; any change to per-entry audit cost (each entry still audits the store,
and only process start, lock acquisition, guards and redo are shared per chunk); and a default
`issuedAt` that a retry can reproduce. A retry MUST pass the `issuedAt` that the first result
reports.

Failure modes: a waiter that does not poll during the yield can still lose the lock to the next
chunk. It waits at most one chunk (8 entries or about 2 s plus one entry) before the next release.
An interrupted batch leaves a completed prefix. Each entry is either committed or not, a pending
receipt is redone by the next writer (§5.2), and the retry replays the prefix. A ticket changed by
another writer between validation and its chunk refuses that entry as `REVISION_CONFLICT`, as the
single form would. A batch request ID reused with a different input replays identical entries,
refuses changed ones as `REQUEST_ID_CONFLICT`, and applies entries at indexes it did not use before. Rollback removes the `--batch` flag, `store.MutateBatch`
and the `mutateLocked` split. Batch entries are ordinary REFINE receipts, so no store, journal or
receipt state depends on the batch form.

| Requirement | Evidence |
| --- | --- |
| CAL-V0-106 | `TestCALV0106_BatchRefineAllSuccess`, `TestCALV0106_BatchRefineStaleEntryFailsAlone`, `TestCALV0106_BatchRefineMalformedEntryWritesNothing`, `TestCALV0106_BatchRefineReplayIsIdempotent`, `TestCALV0106_BatchRefineHelpAndFlags`, `TestCALV0106_BatchFlagOnlyInFlagPosition`, `TestCALV0106_BatchRefineBoundsInlinePayload` (`internal/tasks/cli`); `TestCALV0106_ClaimCommitsBetweenChunks`, `TestCALV0106_UnadmittedOrMalformedBatchWritesNothing`, `TestCALV0106_RetryAfterInterruptedBatchRedoesReplaysAndCompletes` (`internal/tasks/store`) |
