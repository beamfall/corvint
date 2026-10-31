# Corvint Tasks pool safe reuse V0

Owner: Russell Lewis
Date: 2026-10-04
Intent status: accepted feature intent (issue498); PSR-V0-013..015 accepted (decision 0439); PSR-V0-016..021 accepted (decision 0441; V1-0946); PSR-V0-022 accepted (standing auto-accept 2026-10-11; V1-1406); PSR-V0-023 accepted (owner delegated to the orchestrator 2026-10-16; decision 0536; V1-1492); PSR-V0-024..026 accepted (standing auto-accept 2026-10-11; V1-1551; GitHub #1009); PSR-V0-030..043 accepted (standing auto-accept 2026-10-11; V1-1550); PSR-V0-050..057 accepted (standing auto-accept 2026-10-11; V1-1560; GitHub #1010; re-homed from the agent leases spec, owner-approved 2026-10-17); PSR-V0-080 accepted (standing auto-accept 2026-10-11; V1-1572); PSR-V0-081..086 accepted (standing auto-accept 2026-10-11; V1-1586; GitHub #1029); PSR-V0-110..116 accepted (standing auto-accept 2026-10-11; V1-1617; GitHub #1010 follow-up; GitHub #1063); PSR-V0-130..136 accepted (standing auto-accept 2026-10-11; V1-0785; GitHub #583, #1067); PSR-V0-140..143 accepted (standing auto-accept 2026-10-11; V1-1596; GitHub #1039); PSR-V0-150..153 accepted (standing auto-accept 2026-10-11; V1-1626; GitHub #1063); PSR-V0-160..167 accepted (standing auto-accept 2026-10-11; V1-1585; GitHub #1028); PSR-V0-168 accepted (standing auto-accept 2026-10-11; V1-1656); PSR-V0-169 accepted (standing auto-accept 2026-10-11; V1-1653); PSR-V0-224 accepted (standing auto-accept 2026-10-11; V1-2531); PSR-V0-225 accepted (standing auto-accept 2026-10-11; V1-2537); PSR-V0-185 accepted (standing auto-accept 2026-10-11; V1-1785; GitHub #1063); PSR-V0-190..191 accepted (standing auto-accept 2026-10-11; V1-0917, V1-0324; decision 0552); PSR-V0-196 accepted (standing auto-accept 2026-10-11; V1-1750); PSR-V0-202 accepted (standing auto-accept 2026-10-11; V1-2361); PSR-V0-200 accepted (standing auto-accept 2026-10-11; V1-1568); PSR-V0-201 accepted (standing auto-accept 2026-10-11; V1-1008); PSR-V0-203..204 accepted (standing auto-accept 2026-10-11; V1-2352, V1-2425); PSR-V0-220..221 accepted (standing auto-accept 2026-10-11; V1-1715, V1-0911, V1-1569; re-homed from CAL-V0-350 and CAL-V0-364); PSR-V0-222..223 accepted (standing auto-accept 2026-10-11; V1-2482); PSR-V0-226 accepted (standing auto-accept 2026-10-11; V1-2554); PSR-V0-227 accepted (standing auto-accept 2026-10-11; V1-1091); PSR-V0-260..266 accepted (standing auto-accept 2026-10-11; V1-2636; GitHub #1743); PSR-V0-267 accepted (standing auto-accept 2026-10-11; V1-2637; GitHub #1743); PSR-V0-268..272 accepted (standing auto-accept 2026-10-11; V1-2704; GitHub #1813); PSR-V0-273 accepted (standing auto-accept 2026-10-11; V1-2711); PSR-V0-274 experimental under the standing lane auto-accept (2026-10-11; V1-2689)
Delivery status: experimental
Authoritative inputs: https://github.com/beamfall/corvint/issues/498; CAL-V0-028..034 in corvint-tasks-agent-leases-v0.md; independent Gate A packet-r2/GATE-A.md PASS with no HIGH, MED findings incorporated before implementation.
Native ticket: V1-0695, original creation receipt2436.

## Agent digest
- Claim: Explicit foreground operator-defined reset and verification recover quarantined pool members with bound evidence.
- Status: Accepted feature intent (issue498); PSR-V0-013..015 accepted (decision 0439); PSR-V0-016..021 accepted (decision 0441; V1-0946); PSR-V0-022 accepted (standing auto-accept 2026-10-11; V1-1406); PSR-V0-023 accepted (owner delegated to the orchestrator 2026-10-16; decision 0536; V1-1492); PSR-V0-024..026 accepted (standing auto-accept 2026-10-11; V1-1551; GitHub #1009); PSR-V0-030..043 accepted (standing auto-accept 2026-10-11; V1-1550); PSR-V0-050..057 accepted (standing auto-accept 2026-10-11; V1-1560; GitHub #1010; re-homed from the agent leases spec, owner-approved 2026-10-17); PSR-V0-080 accepted (standing auto-accept 2026-10-11; V1-1572); PSR-V0-081..086 accepted (standing auto-accept 2026-10-11; V1-1586; GitHub #1029); PSR-V0-110..116 accepted (standing auto-accept 2026-10-11; V1-1617; GitHub #1010 follow-up; GitHub #1063); PSR-V0-130..136 accepted (standing auto-accept 2026-10-11; V1-0785; GitHub #583, #1067); PSR-V0-140..143 accepted (standing auto-accept 2026-10-11; V1-1596; GitHub #1039); PSR-V0-150..153 accepted (standing auto-accept 2026-10-11; V1-1626; GitHub #1063); PSR-V0-160..167 accepted (standing auto-accept 2026-10-11; V1-1585; GitHub #1028); PSR-V0-168 accepted (standing auto-accept 2026-10-11; V1-1656); PSR-V0-169 accepted (standing auto-accept 2026-10-11; V1-1653); PSR-V0-224 accepted (standing auto-accept 2026-10-11; V1-2531); PSR-V0-225 accepted (standing auto-accept 2026-10-11; V1-2537); PSR-V0-185 accepted (standing auto-accept 2026-10-11; V1-1785; GitHub #1063); PSR-V0-190..191 accepted (standing auto-accept 2026-10-11; V1-0917, V1-0324; decision 0552); PSR-V0-196 accepted (standing auto-accept 2026-10-11; V1-1750); PSR-V0-202 accepted (standing auto-accept 2026-10-11; V1-2361); PSR-V0-200 accepted (standing auto-accept 2026-10-11; V1-1568); PSR-V0-201 accepted (standing auto-accept 2026-10-11; V1-1008); PSR-V0-203..204 accepted (standing auto-accept 2026-10-11; V1-2352, V1-2425); PSR-V0-220..221 accepted (standing auto-accept 2026-10-11; V1-1715, V1-0911, V1-1569; re-homed from CAL-V0-350 and CAL-V0-364); PSR-V0-222..223 accepted (standing auto-accept 2026-10-11; V1-2482); PSR-V0-226 accepted (standing auto-accept 2026-10-11; V1-2554); PSR-V0-227 accepted (standing auto-accept 2026-10-11; V1-1091); PSR-V0-260..266 accepted (standing auto-accept 2026-10-11; V1-2636; GitHub #1743); PSR-V0-267 accepted (standing auto-accept 2026-10-11; V1-2637; GitHub #1743); PSR-V0-268..272 accepted (standing auto-accept 2026-10-11; V1-2704; GitHub #1813); PSR-V0-273 accepted (standing auto-accept 2026-10-11; V1-2711); PSR-V0-274 experimental under the standing lane auto-accept (2026-10-11; V1-2689); experimental source implementation; Gate A and independent source repair review passed. Current integrated runtime and terminal qualification remain pending.
- Exists: Foreground sweep, optional dispatcher integration and focused fixtures are implemented in source. Earlier source-bound repair evidence is retained separately from current integrated qualification.
- Blocked on: Current integrated native binary, mandatory selected tests, exact-target review, terminal evidence, hosted integration and native completion. Linux runtime and physical external cleanup remain unqualified.
- Read next: Requirements; Acceptance evidence; Non-goals and simpler baseline; Compatibility and rollback.

## Job and current state

Job: explicit foreground operator-defined reset and verification recover a quarantined member without an agent session. Scope is optional memberConfig.safeReuse, pool sweep and delegated confirmation; dispatcher opt-in is implemented under this same contract with current integrated runtime qualification pending. Successor base 1def23a641bc00561f142e0b4b0586266609b5f2 (the checked seal of documentation seed 3069405d51a7ab35b6b452f440b2b9ae42a5d74a on main 4b10a02144faed4a77b36eba4b0d1cbc315706d3) supports separate cleanup/manual confirmation; pool commands share owned process runner and ordinary observations clear ownership into quarantine. Source bindings remain in private preparation/source-bindings.json. The original seed made no executed qualification claim. Subsequent repaired-source observations remain bound to their original inputs; they do not qualify this integration or physical safe reuse.

## Requirements

- `PSR-V0-001`: Optional `memberConfig[MEMBER].safeReuse` adds operator-owned reset argv, verify argv/expected exit/literal stdout predicate, timeoutSeconds, maxAttempts and env settings to existing string-member pools. Existing policy without it is byte-identical and has no automated confirmation authority. Occupied member definitions remain immutable. No implicit shell invocation; operator may explicitly declare an interpreter argv.

- `PSR-V0-002`: `pool sweep [--member M] --request-id ID` is an OWNER/OPERATOR foreground mutation. It selects only currently QUARANTINED configured members in deterministic pool/member order; no ALLOCATED, PREPARING, CLEANING or unconfigured member executes a command. Selected allocations/definition digests are fixed before command ownership. No live store/environment runs during fixture qualification.

- `PSR-V0-003`: The writer durably records exclusive command ownership bound to original allocation, definition digest, canonical sweep request, phase/attempt, actor, source revision/tree and runner identity before executing outside the writer lock. Cleanup/reset/verify and all retries retain one durable sweep owner bound to canonical original request. Each phase receipt advances an expected predecessor-observation digest without clearing owner or exposing manual admission. Ordinary cleanup/manual confirm-safe and second sweeps refuse while owned; delegated confirmation consumes exact owner plus final successful predecessor, allocation and definition. Terminal failure/cancellation or explicit proved-orphan recovery releases owner only into quarantine. Pending replay never launches again; concurrent sweeps never both run reset. After explicit proved-orphan recovery (or another terminal write) releases the original owner, reconciling a still-Pending original completes it terminally as not freed, using the last committed phase observation or else the released owner digest as its observation; the finish accepts that owner placeholder only when no member/allocation entry still holds the owner, preserves all recorded evidence and runs no phase again. No unconditional free transition, successor allocation removal or replacement ownership.

- `PSR-V0-004`: Bounds: 1..900 seconds per member attempt, shared reset+verify deadline; maxAttempts 1..2 includes initial try. Optional existing cleanup executes first within the same deadline. A foreground invocation requires explicit `--timeout-seconds` 1..1800 total execution budget; bounded final group cleanup and receipt recording use the shared finalization allowance described below, with group/pipe cleanup separately bounded and recorded; when exhausted, no new member/attempt launches and remaining members stay quarantined. Retry starts a new reset only after failed attempt evidence is durably recorded and owned descendants are proved gone. Cancellation/unknown cleanup/fencing are terminal, not retried. A failure does not reset ticket retry debt. The same absolute historical reconciliation deadline covers initial replay, finish, history publication, lookup and retry. Fresh cancelled execution terminal writes share one bounded 30-second finalization allowance after cleanup, with no renewed per-phase allowance. Timeout or ambiguity retains the original Pending identity and quarantine; a writer still publishing cannot return clean Close. The group/pipe cleanup allowance is 5 seconds, enforced by the runner and recorded as cleanupAllowanceMillis 5000, including synthesized timing. A bounded Git source probe that times out or is interrupted records TIMEOUT or INTERRUPTED, not SOURCE_CHANGED.

- `PSR-V0-005`: Verification: reset requires exit0 and proved group cleanup; then verify must exit `expectExit` (0..255) and stdout must contain the exact declared nonempty UTF-8 literal, with no regex/prose inference; stderr cannot satisfy stdout. Empty/overflow/incomplete output or source/config change cannot pass. Output bound is 64KiB total per phase; parser bounds for argv/env remain existing policy limits. Literal stdout contains is the requested predicate; no regex/equality alternative.

- `PSR-V0-006`: Preserve existing allowlisted environment names; optional `env:"file:PATH"` names an explicit operator-owned private regular file, relative to command repository or absolute outside it. This execution input is not project authority. Read once before durable sweep preparation (max64KiB, at most64 declared NAME=VALUE lines), using the existing bounded regular-file/symlink refusal and opened-descriptor identity check (`intent.ReadFile`), plus regular nonsymlink parent-chain checks and pre/post-read identity/size/modtime checks; do not follow path aliases or traversal components. Require private access (no group/other permission bits); this is a local safety constraint, not authenticated operator identity. Parser accepts literal NAME=VALUE split at first equals, UTF-8/no NUL, no interpolation/export/quoting execution; reject duplicates, undeclared names or missing required configured file. Capture ambient allowlisted values once, then overlay file values deterministically (file wins). Filter captured merged map separately for each phase's declared keys, including existing cleanup's subset; no phase inherits undeclared names. Snapshot bytes once and record SHA256 only; use that captured map for cleanup/reset/verify and retries, never reload it. Concurrent input change during read refuses SNAPSHOT_MOVED; later file replacement does not change this run's snapshot and cannot substitute values. Structured environment values are never policy, journal, status or request preimage bytes; operator commands may echo values into private captured logs, which are not claimed secret-free; exact in-memory environment digest plus file-snapshot digest binds execution. Missing-env behavior otherwise retains existing gateEnvironment semantics. No secrets are required in Git. Admission and every phase require a clean command repository (`git status --porcelain --untracked-files=all`, excluding `.taskman`), so an env file inside the repository must be gitignored; its bytes are bound by the file-snapshot digest, never by the source tree digest.

- `PSR-V0-007`: Retain each phase's exit code or explicit unknown/signal class, stdout/stderr digests, attempt index, times, match result, deadline/cancellation, group-clean result and predecessor-observation digest. Each canonical phase observation stays <=4096 bytes and binds sweep/request/allocation/definition/source/environment/file snapshot. Capture stdout/stderr separately under one combined64KiB cap; pack both byte streams in one canonical length-delimited binary log blob (32-bit big-endian stdout length, stdout bytes, then stderr bytes; max65540 bytes). Each phase records pool-state + observation evidence + log evidence + request, receipt and head; large pool state may add one afterimage blob: exactly6 artifacts for inline pool state, at most7 with pool blob, two gate/evidence POST paths, at most1 EVIDENCE afterimage. Existing StageLease limits are11 artifacts/2658 descriptor bytes and <=3 afterimage EVIDENCE slots; gate/evidence POST count<=2, no reservation/ticket posts, so no global cap increase. Logs use existing private native evidence publication, with creation mode0600 and no inline receipt/status/source/PR upload; observations retain only digest references. Existing archive export includes evidence when explicitly requested and must document that boundary. No automatic publication, new redaction framework or invented secret-screening guarantee. If private retention/encoding/publication fails, confirmation refuses and quarantine remains.

- `PSR-V0-008`: Only after reset+verify pass, evidence is durable and descendants are proved retired may the writer apply delegated confirm-safe against the exact original allocation/definition and successful observation. Operator's optional definition is prospective delegated attestation, so it explicitly amends CAL-V0-030. If existing cleanup is configured, run and record it first, require exit0/clean descendants, then reset and verify; no implicit substitution. Its command limit is 1..3600 seconds (PSR-V0-011) and all phases share the member deadline. A failed cleanup prevents reset/verify and remains quarantined. On failure, interruption, unknown facts or stale identity, retain QUARANTINED and recorded reason/evidence; no FREE result claimed until queue readback agrees. An already owned in-flight operation may publish its terminal observation under an audited ALL barrier; fresh preparation and competing control still refuse. Under ALL, an in-flight sweep launches no further phase command: the writer probes the barrier before each launch and records INTERRUPTED without executing, and the observation planner advances only an owned passing verify to confirmation; every other pass or retryable failure ends QUARANTINED. This exception authorizes only that exact owned successful allocation/definition observation and does not grant a general OWNER bypass.

- `PSR-V0-009`: Canonical caller request identity includes member selector and explicit total budget, not mutable execution observations. Original-request replay lookup occurs before reading current source, member configuration, environment or env file, then precedes fresh eligibility. Same request returns original receipt/evidence even after successful freeing and successor allocation; changed meaningful request with same id conflicts. Child mutation IDs derive canonically from original request + original allocation + retry index + phase, never ambient time. A timed-out confirmation response is resolved by receipt lookup/readback before any retry; replay never frees a successor or reruns reset. Host death while command pending requires existing explicit orphan recover with runner identity check; sweep does not assume killed host cleaned physical resources. An optional allocation selector is part of the meaningful canonical request for the selected member; omission preserves legacy bytes and replay. Observing allocation A followed by successor B refuses before admission or command execution. First admission validates actor/source/environment/definition; historical original-request replay precedes new source/environment/eligibility and uses the shared absolute invocation deadline. Contended historical lookup retains original Pending and receipt sequence and cannot execute again or free a successor.

- `PSR-V0-010`: Foreground CLI handler installs INT/TERM cancellation using signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM), matching existing lease.go signal ownership, and defers stop so notification watcher retires. Runner execution uses absolute deadline, absolute helper executable /bin/ps and bounded subprocess probes: every PID/start/group observation gets remaining execution/cleanup budget, blocked probe becomes UNKNOWN. Cancellation joins watchers, kills/reaps owned process group and drains bounded pipes before absolute cleanup deadline and terminal evidence. Verify descendant retirement on normal return, timeout, INT and TERM. Unknown group state remains quarantined; detached/escaped processes and external services are explicit unqualified limits. New status only exposes evidence identities/reasons, and read commands remain probe-free. Dispatcher auto-sweep is optional opt-in glue under the following bounds; no daemon or service authority is added. Optional closed dispatcher poolSweep configuration has timeoutSeconds 1..1800 and intervalSeconds 1..3600. It owns at most one asynchronous selected-member sweep while unrelated workers continue. Canonical request/config budget/actor/allocation records STARTING before invocation and reuses the same request on restart. Checked candidate encoding remains bounded to 16 MiB; save failure retains Pending. The main loop owns ledger/events; the background result channel has capacity one and never writes the ledger. Run/Close/disable/error/INT/TERM cancel, collect and join the sweep before releasing the program lifetime lock. Worker adoption semantics remain unchanged; publication failure is visible and retains the old STARTING record. A Pending or unknown original is retried with the same request at most once per intervalSeconds, and replay never executes again; disabling poolSweep releases lanes held only by retained ledger records, while an in-flight job still holds its lane until collected. A TERMINAL admission refusal, including a transient one, is not retried automatically for that allocation; the remedy is an explicit foreground sweep with a new request ID or a later allocation.

- `PSR-V0-011`: `memberConfig` health and cleanup `timeoutSeconds` accepts 1..3600 (amends CAL-V0-033's 1..300); the field stays required, 0 or above 3600 refuses LIMIT_EXCEEDED at policy validation, and every other command limit is unchanged. The runner enforces the declared bound with the same process-group kill, TIMEOUT class and quarantine as before. While a long command runs: PREPARING or CLEANING ownership is already durable and the writer lock is not held; claim-time health runs before the claim commits, so no lease, heartbeat or retry debt exists for it; cleanup runs only on a quarantined allocation. No TTL reaps PREPARING or CLEANING; a killed host still needs explicit proved-orphan recovery. Inside pool sweep the bound does not lengthen anything: cleanup still shares the member's 1..900 second safeReuse deadline and the 1..1800 second total budget (foreground and dispatcher poolSweep), and whichever ends first records TIMEOUT and keeps quarantine. A dispatcher worker that claims with a long health command is subject to its role `idleSeconds`/`wallSeconds`; the operator must configure those above the health bound, and a worker killed mid-health leaves PREPARING for explicit recovery, never FREE.
- `PSR-V0-012`: A health, cleanup or safeReuse command `cwd` is `"REPOSITORY"` (default; queue checkout root, unchanged bytes) or the closed object `{"kind":"PINNED_REPOSITORY","path":ABS,"revision":OID}`. Policy validation is host-independent: `path` is an absolute, already clean POSIX path other than `/`, and `revision` is a full lowercase SHA-1 or SHA-256 object id; branch names, abbreviations, unknown keys and other strings refuse. Optional `safeReuse.cwd` applies to both reset and verify; omission preserves legacy policy bytes. Before execution the runner proves, with bounded Git probes, that `path` is its own real path (Lstat directory, no symlink alias) and a Git worktree top level (else MISSING_EVIDENCE), that `HEAD^{commit}` equals `revision` (else STALE_TREE), that `git status --porcelain --untracked-files=all` excluding `.taskman` is empty (else DIRTY_WORKTREE), mirroring PSR-V0-006's REPOSITORY clean-input rule, that HEAD still equals `revision` after that status (this narrows, but cannot exclude, a revision change around the status read), and that the directory's Lstat identity (device/inode) is unchanged across the proof; a timed-out or interrupted probe records TIMEOUT or INTERRUPTED. The proved identity is re-checked immediately before process start; a change starts nothing and records SOURCE_CHANGED. The proof runs in the writer observer at claim preparation, cleanup and sweep admission, so a refusal records no ownership and runs nothing; it repeats immediately before launch (a failure runs nothing and records a failed observation that keeps quarantine) and after exit. The post-exit proof is detection only: shown drift records SOURCE_CHANGED; a bounded probe TIMEOUT/INTERRUPTED replaces any exit class that could satisfy the phase predicate (including a verify `expectExit` other than 0) and otherwise keeps the command's own failure; a phase whose post-exit proof failed never passes, so it can never free a member. Accepted bound: the proof is a sequence of non-atomic observations of a checkout Tasks does not own, and concurrent mutation by an actor outside Tasks during or after those observations is outside the guarantee. That includes in-place edits after the status read that keep HEAD and the directory identity, a change-and-restore between any two reads, a change between the launch guard and the child's chdir/exec, edits while the command runs, and restoring the pinned state before the post-exit proof; any of these can cause different content to run undetected, and external effects of a run are never undone. The re-reads and launch guard detect only changes still visible when they observe. The proof costs about four bounded Git probes before and after each phase, all inside the member deadline. The queue repository clean-input rule still applies and a relative env file stays relative to the queue repository. The pin is part of the member definition digest, so an occupied definition cannot be re-pinned. The pinned worktree is an operator execution input, never project authority, and read commands still execute no probe.
- `PSR-V0-013`: `corvint-tasks pool status [--pool ID] [--member ID]` returns one `taskman-pool-status/0` item in the existing taskman-command-result/0 envelope, listing every configured member in policy pool/member order. Each member reports `state` (FREE, PREPARING, ALLOCATED, QUARANTINED or CLEANING from audited `pools.json`; an audited absent pool state is all FREE), `allocationId`, `holder`, `stage`, `attemptId` and `generation` (null when FREE, and attempt/generation null while PREPARING has no attempt), the member's last pool-state `changedSeq`, a pending `commandKind` (the retained command kind while PREPARING or CLEANING, null in every other state because the kind outlives its command), and `quarantine` `{reason, changedSeq, since}` when QUARANTINED. `lastHealth` and `lastCleanup` report the class, passed and groupClean of the digest-verified `taskman-pool-observation/0` blob the current allocation retains. The journal records no wall-clock time, so `since` and `observedAt` are `NOT_OBSERVED`; PSR-V0-261 amends `since` to the recorded `quarantinedAt` (still `NOT_OBSERVED` for an entry written before it), and PSR-V0-260 adds a recorded failed observation as `lastHealth` or `lastCleanup`. The lease and pool request preimages that drive allocation, quarantine, health and cleanup carry no `issuedAt` (only ticket, release and note mutation envelopes do), and the pool observation carries none either, so no recorded client time is reachable without a request or receipt format change; a time source needs a separate decision; any outcome current pool state does not retain (a FREE member's history, the other command kind, a sweep-phase observation or an unreadable blob) is `NOT_OBSERVED`, never inferred. Status: proposed (V1-0892; GitHub #647).
- `PSR-V0-014`: `--pool` and `--member` each appear at most once with a nonempty label and filter the view; `--member` alone searches every pool. An unknown pool, a member outside the selected pools, a repeated, empty or unknown flag, or a positional argument refuses ERROR `MALFORMED` naming the unknown pool or member. The verb is listed in `help`'s implemented verbs, `pool --help` and its own `pool status --help`. Status: proposed (V1-0892; GitHub #647).
- `PSR-V0-015`: `pool status` is a pure TM-V0-008 read like `queue status` and `attempt show`: it acquires neither the writer nor the preparation lock, runs no health, cleanup or Git probe, and writes no state, intent, evidence or ledger file. It succeeds while another process holds both locks and leaves the state and intent trees byte-identical. Status: proposed (V1-0892; GitHub #647).
- `PSR-V0-016`: `corvint-tasks claim <ticketId> --holder H --pool P --stage S --share-allocation ALLOCATION_ID` binds a further attempt to an existing ALLOCATED pool member instead of taking a free one. The bound attempt runs the ordinary claim checks for its own ticket (live attempt, retry, collision, capacity, author exclusion), carries the same `poolAllocation` tuple and a closed `sharedAllocation` record `{sourceAttemptId, sourceGeneration, boundSeq}` naming the entry's attempt at bind time, and is appended to the entry's ordered `shared` list in pools.json, all in its one ADMIT receipt through the existing transaction and journal path, so redo after a crash and exact replay of the request return the same binding. `--share-allocation` takes a 64-hex allocation digest, needs `--pool`, is a ticket claim only (not `--next`) and takes no `--exclude-member` or `--exclude-authors`; otherwise the request refuses ERROR `MALFORMED` at lease request validation, like the other lease flags: after the store head probe that names the queue (so a pending redo still reports `REDO_PENDING` first), before any ticket, attempt or pool state is evaluated, and with nothing written. A shared claim skips CAL-V0-101 priority yield because it takes no member. Status: (proposed, pending owner acceptance; V1-0946; GitHub #653).
- `PSR-V0-017`: A share refuses with stable codes and writes no binding when: the allocation is unknown or not ALLOCATED (REVISION_CONFLICT `FENCED`); the allocation belongs to another pool or stage (VALIDATION_FAILED `MALFORMED`); the entry's holder differs (BLOCKED `RESOURCE_COLLISION`); a bound attempt is supervised or not an external-agent attempt (UNSUPPORTED); a bound attempt's lease has expired (BLOCKED `ATTEMPT_LIVE` naming it as expired, so the store reaps it in its own receipt and retries, CAL-V0-011); or the member already binds `MaxSharedAttempts` = 4 attempts, the original included (CAPACITY_EXHAUSTED `LIMIT_EXCEEDED`). A ticket that is not claimable refuses as its ordinary claim would. Every attempt end updates the binding in the same receipt (PSR-V0-018), so a stored binding whose bound attempt is missing, terminal, at another generation, under another holder or stage, or not marked shared is an inconsistent store: loading it refuses VALIDATION_FAILED `MALFORMED`, as an inconsistent unshared allocation already does, and the claim writes nothing. Status: (proposed, pending owner acceptance; V1-0946; GitHub #653).
- `PSR-V0-018`: A shared member stays ALLOCATED until its last bound attempt ends and then takes the ordinary end-of-attempt path once (normally QUARANTINED). An ending shared attempt leaves the `shared` list. An ending entry attempt (release, handoff, cancel, failure or lease reap) hands the allocation over: the first shared attempt becomes the entry's attempt and leaves the list, nothing quarantines, and the other bound attempts are untouched. Each change advances the entry's `changedSeq` in the ending attempt's receipt. Releasing the original holder therefore hands over rather than refuses. No bound attempt's lease outlives the allocation, because the allocation lives exactly as long as some bound lease does; renewing, heartbeating or reaping one bound attempt changes no other attempt. Status: (proposed, pending owner acceptance; V1-0946; GitHub #653).
- `PSR-V0-019`: Sharing excludes the two paths whose effect spans a whole allocation: an attempt that is shared or whose allocation binds shared attempts cannot ATTACH to a supervisor (UNSUPPORTED), because a supervised stage stop quarantines the allocation; and a shared allocation is never released lane-untouched (CAL-V0-067): the attestation refuses while the entry binds shared attempts, and after any hand-over or unbind the entry's `changedSeq` no longer matches the original admission. Status: (proposed, pending owner acceptance; V1-0946; GitHub #653).
- `PSR-V0-020`: The binding is visible in pure reads (PSR-V0-015, TM-V0-008). `pool status` gives each member a `boundAttempts` array: empty unless ALLOCATED with an attempt, else `{attemptId, generation, role}` with role PRIMARY for the entry's attempt then SHARED in binding order. `attempt show` of an attempt with a `poolAllocation` adds the derived observation `poolBinding` `{allocationId, state, role, boundAttempts}` from audited pools.json: role is PRIMARY, SHARED or null when the attempt is no longer bound, and state is FREE when pools.json no longer holds the allocation. A shared claim's result carries `sharedAllocation`. Neither read takes a lock or writes any file; canonical attempt captures without observations are unchanged. Status: (proposed, pending owner acceptance; V1-0946; GitHub #653).
- `PSR-V0-021`: The encoding is additive: pools.json entries gain the optional `shared` key only when non-empty, and attempt records gain the optional `sharedAllocation` key only on shared attempts, so every record without a binding keeps identical bytes and digests, and the lease request preimage gains `shareAllocation` only when present. `shared` is non-empty, at most 3 items, only on an ALLOCATED entry, and names distinct same-queue attempts other than the entry's own; `sharedAllocation` needs a `poolAllocation`, no `directPoolAdmission`, another same-queue source attempt and a positive `boundSeq`. Both decoders are closed, so a binary that predates these keys refuses a store holding them with `MALFORMED` (unknown key) on its first audited read instead of misreading occupancy; it cannot operate that queue until the bindings end. Status: (proposed, pending owner acceptance; V1-0946; GitHub #653).
- `PSR-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1406) A bounded Git source probe (the sweep source observation, the PSR-V0-012 pinned-cwd proof and the config read) whose own owned-group cleanup the runner does not prove is an observation that saw nothing about the source: it records `UNKNOWN`, the explicit unknown class of PSR-V0-007, or, when the probe also hit its bound, the PSR-V0-004 `TIMEOUT` or `INTERRUPTED`; never `SOURCE_CHANGED`, which PSR-V0-012 keeps for shown drift. Unproved cleanup is terminal under PSR-V0-004: before launch the probe runs nothing and the member stays quarantined; after exit its class replaces the command's exit class, including a retryable failure, so no further attempt launches; an expired attempt deadline still records the PSR-V0-004 `TIMEOUT` or `INTERRUPTED`; an admission or preparation proof that meets it refuses with the probe's own `UNSUPPORTED` error rather than claiming the pinned path is not a worktree. No class, code, field or encoding is added; observations and receipts without such a probe keep identical bytes.
- `PSR-V0-023`: (owner delegated to the orchestrator (2026-10-16), decision 0536, V1-1492) Within a foreground or dispatcher sweep, the fixed observation bounds scale with the invocation's `--timeout-seconds` budget B: each bound is its fixed value times B/10 s, never below the fixed value and never above 5 times it. The scaled bounds are the sweep Git source probe (PSR-V0-004, PSR-V0-012 and PSR-V0-022 probes; fixed 2 s, carried in whole seconds and rounded up), the runner-identity and group-listing probes (fixed 1 s, pipe delay 250 ms) and the sweep command pipe delay (fixed 1 s). Every probe still shares the attempt and invocation deadlines. The group/pipe cleanup allowance stays 5 s and is still recorded as cleanupAllowanceMillis 5000; its group probes keep the scaled bounds. Pool preparation and cleanup outside a sweep have no budget and keep the fixed bounds. A scaled bound that expires records exactly the outcome its fixed bound records (TIMEOUT, INTERRUPTED, UNKNOWN or QUIESCENCE_UNPROVED, with the member kept QUARANTINED). No flag, environment variable, exported seam, class, code, field or encoding is added.
- `PSR-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1551, GitHub #1009) `memberConfig[MEMBER]` gains an optional `prepare` command with the closed health/cleanup command shape (`argv`, `cwd` per PSR-V0-012, declared `env` names within the policy allowlist, `timeoutSeconds` 1..3600 per PSR-V0-011). It is accepted only together with `safeReuse` (otherwise VALIDATION_FAILED `MALFORMED`), because only `pool sweep` runs it; it is part of the member definition digest, so occupied definitions stay immutable. A member without `prepare` keeps byte-identical policy, pool state, observations and sweep behaviour.
- `PSR-V0-025`: Within an owned sweep attempt the phase order becomes cleanup (if declared), reset, verify, prepare, confirm: a passed verify advances to `prepare` instead of `confirm`, and a passed prepare (exit 0 with proved group cleanup and post-exit source/pin proof, as for reset) advances to `confirm`. Prepare runs only while the sweep owns the member (state CLEANING from the selected QUARANTINED allocation, never FREE or allocatable), on the same owned process runner and bounded 64 KiB capture, under the member's shared attempt deadline and the sweep's `--timeout-seconds` budget (its own `timeoutSeconds` never extends them), with the same pinned-cwd checks at admission and around execution and its declared env names loaded with the member's other phase inputs. Its observation is a `taskman-pool-sweep-observation/0` with phase `prepare` chained to the passed verify; output is digest-only: stdout/stderr digests are recorded and its log evidence is the empty log, which the writer requires. Delegated confirmation then requires the passed prepare observation as the owned witness. Under an ALL barrier prepare is a further phase command: it is never launched, and a sweep that passed verify ends QUARANTINED with the ALL-barrier reason.
- `PSR-V0-026`: A prepare that does not pass (any class, including TIMEOUT at the shared deadline, OUTPUT_LIMIT, SOURCE_CHANGED or unproved cleanup) is terminal for the sweep: it is not retried even when `maxAttempts` remains, the member stays QUARANTINED with reason `CLASS; pool prepare failed`, its sweep owner is released only when group cleanup is proved, and the member is never freed. Same-request replay and reconciliation read the committed prepare receipt like any other phase receipt and never re-execute a completed prepare; a lost prepare response leaves the original pending while the sweep still owns the member. The per-member owned-step bound rises from 7 to 8 (two cleanup/reset/verify attempts, then prepare and confirm); without prepare no sweep reaches the eighth step.

### Capacity slots (V1-1550)

A scarce stateless resource (an API budget, licence seats, a GPU count) needs shared capacity but none
of a pool member's identity, cleanup or quarantine. Slots reuse the lease journal, pools.json and the
lease reap path; there is no daemon, no member and no physical state. GitHub #1008.

- `PSR-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) Optional policy `slots` declares 1..16 resources `{id, capacity[, reserved]}` with closed keys: `id` a unique label (`DUPLICATE_ID` otherwise), `capacity` 1..64, and `reserved` an object of 1..8 class labels each reserving 1..64 slots whose sum is at most `capacity`. An out-of-range count, class count or reserved sum refuses `LIMIT_EXCEEDED`; any other shape refuses `MALFORMED`. A policy without `slots` keeps identical bytes.
- `PSR-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) pools.json gains the optional `slots` array, written only when non-empty, of at most 512 entries `{slotsId, attemptId, generation, holder, count, exclusive, class, priority, seq, state}` in (slotsId, seq, attemptId) order with closed keys: a same-queue attempt, `count` 1..64, `priority` P0..P3, `seq` positive and at most the head sequence, `state` `HELD` or `WAITING`, a label or empty `class`, and one entry per (slotsId, attempt). Pool state without slots keeps identical bytes and digests.
- `PSR-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) `slots acquire --attempt ID --generation G --slots ID [--count N | --exclusive] [--class LABEL]` runs on a live attempt with a work lease, for its holder, generation-fenced like every attempt verb (`REVISION_CONFLICT` `FENCED`, recorded). It adds a `WAITING` entry with the next receipt sequence, the ticket's priority at that moment and `count` (default 1, or the full capacity when exclusive), then grants waiters and commits pools.json in one lease receipt. Only live entries count: an entry whose resource is no longer declared or whose attempt is no longer live at the entry's generation holds and waits for nothing and is pruned by the next slot write, so ending, releasing or reaping an attempt frees its slots without operator action. An acquire that finds an expired lease on another entry of the resource returns it for reaping (`BLOCKED` `ATTEMPT_LIVE` with `expired`); the store reaps it in its own receipt and retries, so a crashed holder's slots free on the next poll.
- `PSR-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) An exclusive entry is granted only when nothing of the resource is held and no exclusive waiter is ahead of it (amended by PSR-V0-041: no eligible exclusive waiter); while it is held nothing else is granted. A shared waiter is not granted while an exclusive waiter is ahead of it in wait order or was enqueued before it, so new shared grants stop behind an exclusive request and it is granted once the current holders release, whatever the shared arrival rate.
- `PSR-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) Waiters are considered in (priority, seq, attemptId) order, P0 first. A shared waiter is granted when its count fits what is available to its class; once one does not fit, a later waiter is granted only within its own class's unused reservation (no general overtaking).
- `PSR-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) For a shared request of class K (empty when unreserved), available = capacity - held - the sum over every other reserved class of max(0, reservation - held by that class). An unreserved or other-class waiter therefore never takes a class's unused reservation, whatever its priority, so a reserved class is never starved by an unreserved one. A class not reserved on the resource refuses `MALFORMED`; a count above capacity minus the other classes' reservations can never be granted and refuses `LIMIT_EXCEEDED`; an unknown resource refuses `MALFORMED`. `--exclusive` with `--count` or `--class`, or a missing `--slots`, refuses `MALFORMED` and a count outside 1..64 refuses `LIMIT_EXCEEDED`, both before any read of state.
- `PSR-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) `slots release --attempt ID --generation G --slots ID` drops the attempt's held or waiting entry and grants waiters in the same receipt; with no entry it is a no-change (amended by PSR-V0-042: recorded). An admission barrier refuses `slots acquire` `BLOCKED` `PAUSED` without a write and does not block `slots release`. A repeat acquire by an attempt that already has an entry with other count, exclusivity or class refuses `BLOCKED` `RESOURCE_COLLISION`. Both verbs accept the CAL-V0-111 `--lock-wait SECONDS` bound, extending the CAL-V0-216 verb set.
- `PSR-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) Acquire and release are idempotent by request id: a retry replays the committed receipt without a write. A waiter polls with a fresh request id and the same parameters; that poll only grants waiters and is a no-change when nothing moves (amended by PSR-V0-042: recorded). Refusals other than the recorded FENCED write nothing. The lease request preimage gains `slots`, `slotCount`, `slotExclusive` and `slotClass` only when present, so every other request keeps identical preimage bytes.
- `PSR-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) Every `slots acquire|release` result carries `slot` `{slotsId, state, count, exclusive, class, position}` from a fresh audited read after the command (state `HELD`, `WAITING` with its 1-based wait position, or `NONE`; on a replay it is the current state, not the state at the original commit; amended by PSR-V0-043: a replay carries `slot` null), or null when the command failed before the read. `queue status`, when the policy declares slots and the journal is present, adds `slots`: per resource `slotsId`, `capacity`, `reserved`, `held`, `free`, `exclusiveHeld`, `holders` and `waiters` in wait order with `position` and `priority`, each naming attempt, generation, holder, count, exclusive, class and `leaseExpired`. Both are pure reads: no lock, no reap, no write.
- `PSR-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) An attempt with a live slot entry, held or waiting, cannot attach to a supervisor: ATTACH refuses `UNSUPPORTED` without a write, because a supervised attempt is never expired, reaped or slot-released and its slots would never free. Release the slots first. An entry of an older generation is dead and does not refuse.
- `PSR-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) Grants follow the current policy. A waiter whose own acquire the current policy would refuse (its class is no longer reserved, or its count exceeds capacity minus the other classes' reservations) is neither granted nor blocks a later waiter, and stays visible until released; its poll refuses as a fresh acquire would. An exclusive grant takes the current capacity as its `count`, and an exclusive poll matches on exclusivity and class, not count. No grant raises `held` above the current capacity; holders granted under a larger capacity keep their grant until they release.
- `PSR-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) An exclusive waiter is eligible only when no reserved-class shared waiter was enqueued before it; the first eligible exclusive in wait order is granted when nothing is held. A reserved-class shared waiter is held back only by an exclusive waiter enqueued before it, whatever the priorities; an unreserved shared waiter is also held back by an exclusive waiter ahead of it in wait order. Exclusive and reserved-class requests are therefore served in enqueue order between themselves, so continuing exclusive demand cannot starve a reserved class (PSR-V0-035) and an exclusive request is still eventually granted (PSR-V0-033).
- `PSR-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) Every `slots acquire|release` that is not refused records a lease receipt, including one that changes nothing (a release with no entry, or a poll that grants nothing): it posts only the request entry with outcome `COMPLETED`, so a retry of its request id replays it and never acts on a later state.
- `PSR-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1550) A replayed `slots acquire|release` result carries `slot` null: the current entry may belong to a later request or generation, so it is not reported as the original grant. A caller reads the current state with a fresh poll or `queue status`.

### V1-1560 pool acquire under a moving journal (issue 1010)

CAL-V0 IDs in this subsection are defined in `corvint-tasks-agent-leases-v0.md`.

Human-owned input: GitHub [issue 1010](https://github.com/beamfall/corvint/issues/1010)
(2026-10-17, native ticket V1-1560). On a journal of about 45,000 receipts, with a dispatcher
committing every few seconds and 8 acquirers, `pool acquire` waited 10 to 40 minutes. Every one of
its 16 rounds (CAL-V0-263) reran the complete audit from receipt 1, and any write during that audit
made the round `SNAPSHOT_MOVED`, so the cost of a wait grew with retries and the writer rate. The
owner declined extending the writer route to every lease verb (CAL-V0-264), because it drops the
CAL-V0-116 detection of a rewritten prefix. Intent: a lease write that loses its round to a moving
journal reuses its own settled audit from receipt 1 in the next round, so every acquire completes
within a bound that does not depend on the writer rate or the number of retries. Integrity checks,
refusals on tamper or corruption, receipts and ordering stay the same.

- `PSR-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1560) A lease
  write's preparation audit from receipt 1 MUST make one complete attempt that stops early once
  `head.json` is no longer the file its capture observed (checked every 512 receipts or
  projections). When that attempt is `SNAPSHOT_MOVED`, the audit MUST walk receipts 1..H for the
  head H it captured, with the complete audit's validators, as the CAL-V0-265 refresh does. That
  prefix walk MUST never decide a refusal for a lease. A walk that refuses on an unchanged
  observation is `SNAPSHOT_MOVED` (prefix declined), and for the rest of the command the complete
  audit decides, deriving any refusal itself. This applies only on a platform that reports a file
  change time and at a head at or above the writer-checkpoint floor.
- `PSR-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1560) A resume
  base MUST come only from a settled complete (`ModeFull`) or prefix (`ModePrefix`) audit from
  receipt 1 that the same command ran, together with that audit's own physical observation. Tail,
  writer and cached results never yield one. The base records the writer checkpoint the audit
  derives and, for every receipt, request, evidence and pinned file the audit read, the file stamps
  its capture observed, its change time and the digest of the bytes it read. Receipts 1..H, every
  canonical afterimage at H and every listed request file MUST be among them, or there is no base.
  A base lives only in that command's memory: it is never written, shared or reused by another
  command or process.
- `PSR-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1560) A lease
  write whose round ended `SNAPSHOT_MOVED` after it derived a resume base MUST, in later rounds,
  first try the writer route under the preparation gate and writer lock, auditing only the tail
  after the base (at most 256 receipts) for any lease verb. Before it commits, it MUST hold that:
  - every base receipt is the same file, with the same mode, size, modification time and change
    time;
  - every base request, evidence and pinned file is likewise unchanged;
  - a base file whose stamps were not yet settled when the base capture began is re-read and
    matches its digest;
  - every request file in every shard is a base file or one the tail posted, and no base request
    is missing;
  - every evidence and pinned afterimage the tail posted matches its canonical digest;
  - the writer-checkpoint invalidation token read before the base audit is unchanged.
  Any difference MUST decline the resume, write nothing and fall through to that round's ordinary
  preparation. The receipt a resumed write commits is the one the complete route would commit.
  Resumed writes advance the retained writer checkpoint under CAL-V0-117.
- `PSR-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1560) A tampered
  prefix MUST never commit through a resume. Examples are a base receipt rewritten in place with its
  modification time restored, or a stray or removed request file. The command keeps the refusal or
  `SNAPSHOT_MOVED` that the complete route gives that journal, and CAL-V0-116 detection is unchanged
  for every lease verb.
- `PSR-V0-054`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1560) When the
  change-guard registration, which visits every store file, is overtaken (`SNAPSHOT_MOVED`) in a
  round that may resume, the preparation MUST still run the PSR-V0-050 audit, but only to derive a
  resume base, and then end the round `SNAPSHOT_MOVED`. Without a guard it MUST never decide a
  refusal, use the complete route or commit. A prefix walk declined there disables resume for the
  command.
- `PSR-V0-055`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1560) Bound. Once
  a lease write has a resume base, each later round costs at most one bounded writer section: the
  gate and lock wait, a tail of at most 256 receipts, and a listing of the receipts and request
  shards. That cost is independent of the writer rate and of the number of rounds. A command
  therefore runs at most:
  - one early-stopped complete attempt and one prefix walk before its first resume, plus one more
    preparation audit per declined resume;
  - the CAL-V0-263 bounded sections.
  CPU per waiting acquirer does not grow with retries. The documented acquire bound is under 2
  minutes for 8 acquirers and a writer every 3 seconds, measured at 10,000 receipts on a loaded
  12-CPU Darwin host (the opt-in load harness). Qualification at 45,000 receipts remains NOT_RUN,
  and by linear extrapolation the prefix walk alone would take about 3 minutes at host load 60. The Linux 45,000-receipt run is in the same build log.
  `Timing.Resumed` counts resumed rounds.
- `PSR-V0-056`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1560 review
  amendment) No change guard watches a resumed write, so the PSR-V0-052 checks MUST hold up to the
  commit, not only at the resumed audit. Under the writer lock, after modelling and after the head,
  intent and branch rebind, a resumed write MUST run the PSR-V0-052 base checks again against the
  files as they are then, with each request shard listed afresh. It MUST do this before it reports a
  result, retains a checkpoint or publishes anything. As in PSR-V0-052, any difference declines the
  resume.
- `PSR-V0-057`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1560 review
  amendment) The base checks of PSR-V0-052 and PSR-V0-056 read through a read lifetime of their own.
  A failed close of that lifetime, or of any file or directory read within it, is a cleanup failure,
  as an audit's own is: `UNSUPPORTED_FILESYSTEM` (`read lifetime`), carried in the resumed audit's
  physical observation. It MUST terminate the command, as CAL-V0-116 requires. It is never a
  decline that falls back to preparation.

Non-goals: the retryable result's retry count and audit duration (issue 1010 option, NOT_PRODUCED;
a detached acquire's PENDING item reports them from PSR-V0-112);
exposing `Resumed` in the CLI; resuming on a platform without file change times; resuming past a
tail of 256 receipts; a persisted or shared resume base; changing the CAL-V0-264 decision; a
liveness-latency bound for heartbeat or renew.

Failure modes: a resume declined because the base changed (any rewrite, a stray request, an
invalidation-token change, a tail beyond 256) costs one more preparation audit in that round. A
command whose prefix walk is declined, whose model reads an elided path, or whose request is a
handoff-policy candidate keeps the complete route each round and can still fail `SNAPSHOT_MOVED`
after 16 rounds (retryable), as under CAL-V0-263. The listings under the writer lock grow with the
receipt and request-file count. Without change times (platforms other than Darwin and Linux) lease
writes behave as before.

Acceptance evidence:
- `TestPSRV0050_PrefixFallback`, `TestPSRV0051_ResumeBaseOnlyFromSettledAudit` and
  `TestPSRV0052_ResumeDeclinesOnChangedBase` (6 cases with settled and unsettled stamps) in
  `internal/tasks/journal`.
- `TestPSRV0052_PoolAcquireResumesUnderWriter`, `TestPSRV0052_WidenResumesUnderWriter`,
  `TestPSRV0053_ResumeKeepsRewrittenPrefixDetection`, `TestPSRV0054_OvertakenGuardStillResumes` and
  `TestPSRV0054_OvertakenGuardKeepsRewrittenPrefixDetection` in `internal/tasks/store`.
- Review amendment (independent Codex review of PR #1018): `TestPSRV0056_ResumeRechecksBaseBeforeCommit`
  (`internal/tasks/store`) rewrites receipt 2 at the resumed write's `fast.lease.observe` and
  `fast.lease.model` stages. On the reviewed head c843d6aab the acquire committed over the rewritten
  prefix, and after the change it keeps the complete route's decision. In `internal/tasks/journal`,
  `TestPSRV0056_RecheckResumeSeesChangeAfterAudit` covers a rewritten receipt, a stray request in a
  new shard and a failed close. `TestPSRV0057_ResumeCheckCloseFailureIsCleanup` fails the close of
  the base check's receipts listing. On c843d6aab that was an ordinary decline, and after the change
  it is the audit's cleanup failure.
  `TestPSRV0052_PoolAcquireResumesUnderWriter` fails on base cc487c92e: 16 complete audits and 16
  rounds, then `SNAPSHOT_MOVED`. After the change it completes with one audit and one resumed round.
- The opt-in harness `TestPSRV0055_PoolAcquireLoad` (`CORVINT_TASKS_POOL_ACQUIRE_LOAD=<report>`).
  At 10,000 receipts, 8 acquirers and a writer every 3 seconds (12 CPUs, host load 53 to 68):
  - base: 2 of 8 acquires completed (252 s and 288 s), 1 ended `SNAPSHOT_MOVED` after 16 rounds
    (374 s), and 5 were unfinished at 420 s;
  - after the change: 18 of 18 completed, all resumed, with at most 3 rounds, p50 42.9 s and
    max 65.2 s.
  See `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1560-pool-acquire-starvation.md`.
- Linux, 45,000 receipts (V1-1569, 2026-10-23, 4 vCPU Firecracker VM, UNQUALIFIED at load 7 to 17): the first run failed with `no space left on device` from the inotify change guard (fixed by PSR-V0-221); after the fix all 20 acquire cycles completed with no error code, acquire p50 81.7 s and max 86.9 s, under the 2-minute bound (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-tasks-linux-qualification-b3-harness.md`). A quiet-host run (load at or below 4) is still NOT_RUN.

Rollback: revert the resume route in `prepareLease`/`leaseWrite`, `observeWriter` and `leaseWriter`,
the journal `PrefixFallback`/`ResumeBase`/`AuditForResume`/`RecheckResume`, their tests and this amendment. A
resume base is in-memory only. No store format, receipt, checkpoint format or wire result changes,
so stores written by either build are read by the other.

### V1-1568 lease paths that keep the complete route

V1-1568 (found by the V1-1560 lane, PR #1018) listed the lease paths that the PSR-V0-051/052 resume
does not cover. The PSR-V0-050..057 failure modes name them without a reason. This requirement gives
the reason for each, so that none is an unexplained gap.

- `PSR-V0-200`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1568) Each lease
  path below MUST keep the complete route, or pay the one extra preparation stated, for the reason
  given. None may be moved onto a resume without its own requirement that keeps the reason's
  guarantee.
  1. A request that is a handoff-policy candidate (a `release --reason HANDOFF` the reducer refused
     `STALE_POLICY`) keeps the complete route each round. Its conversion needs `AuditForHandoff`.
     That is one fresh complete audit selected by the attempt's admission policy digest, which reads
     the attempt's history and the original policy from receipt 1. Neither the writer checkpoint nor
     the PSR-V0-051 base retains that history. The request is a single release, not a contended
     acquire loop, so a retryable `SNAPSHOT_MOVED` after 16 rounds is the accepted cost.
  2. A request whose model reads an elided path keeps the complete route. The writer route's
     inventory carries only the paths the writer checkpoint selects (JSG-V0-004). A model question
     about any other path's metadata voids the result (`Inventory.Incomplete`), so committing it
     would rest on metadata the route never read. Carrying every path's metadata would change the
     writer-checkpoint profile and grow it with the store, which is outside the bounded route.
  3. A command whose PSR-V0-050 prefix walk was declined keeps the complete route for the rest of
     the command. A declined prefix walk means the prefix refused on an unchanged observation, and
     a prefix walk never decides a refusal (PSR-V0-050). Only the complete audit may decide it, and
     a resume base from that prefix would let a write commit over a journal the complete audit
     refuses.
  4. Every lease write on a platform without file change times (not Darwin or Linux,
     `ResumableAudits`) keeps the complete route. PSR-V0-053 detects a rewrite under a resume
     base by change time. Without it, a rewrite that restores size and modification time would go
     undetected, and that is the CAL-V0-116 rewritten-prefix detection the owner kept by declining
     CAL-V0-264.
  5. A tail longer than 256 receipts, or a changed writer-invalidation token, declines the resume
     and costs that round one more preparation audit (PSR-V0-055). The 256-receipt tail is the
     CAL-V0-116 bound that keeps a resumed write bounded. A changed token means the writer checkpoint
     under the base no longer holds, so the next preparation derives a new base.

  Acceptance evidence: paths 3 to 5 are witnessed by `TestPSRV0050_PrefixFallback`
  (`internal/tasks/journal`), `TestPSRV0150_DeclinedResumeWalksOncePerRound`,
  `TestPSRV0151_DeclinedResumeChargesNoResumedWalk` and `TestPSRV0053_ResumeKeepsRewrittenPrefixDetection`
  (`internal/tasks/store`). There are no focused tests yet for paths 1 and 2 staying on the
  complete route under a concurrent writer (NOT_RUN). Rollback: revert this paragraph. No code,
  format or wire contract changes with it.

### V1-1572 older binaries and slots

Human-owned input: native ticket V1-1572 (2026-10-17), a follow-up of V1-1550 asking that the
inferred older-binary refusal of a store with slots be observed, not only inferred from the closed
decoders.

- `PSR-V0-080`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1572) Compatibility
  statement: a corvint-tasks binary that predates slots (V1-1550) refuses, on its first audited read
  and without writing, a store whose `policy.json` declares `slots` (`MALFORMED`, `policy.json:
  closed object violated (unknown keys: slots)`) and a store whose `pools.json` carries a `slots` key
  after the policy dropped them (`MALFORMED`, `/: closed object violated (unknown keys: slots)`); the
  same store without slots is read normally. Such a binary therefore cannot operate a queue while
  slots are declared or held, and nothing in the store is misread or rewritten. The remedy is the
  slots rollback of PSR-V0-030..043 (release every entry, run one slot write, publish policy without
  `slots`) or a binary that includes V1-1550. No code, format or decoder changes.

  Acceptance evidence: the opt-in `TestPSRV0080_OlderBinaryRefusesSlots`
  (`internal/tasks/cli`, `CORVINT_TASKS_OLDER_BINARY=<binary>`) ran on 2026-10-17 against
  corvint-tasks built from source at tag `v1.0.0-rc.2` (154d663f051b7a1c8767a0934fc8e92100ae323d,
  go1.27.2, Darwin, unverified source build in a temporary directory, not installed): the control
  `queue status` was OK; `queue status` and `attempt show` were `MALFORMED` (exit 1) on both slot
  stores, with the state tree unchanged. Rollback: delete the test and this statement.

### V1-1586 member source identity and drift guard (issue 1029)

Human-owned input: GitHub issue #1029 and native ticket V1-1586 (2026-10-17): each pool member
reports the source it holds, a pool names the approved sources, and a member whose source drifted
never returns to FREE. Acceptance: a source outside the allow-list is refused at acquire with a
typed reason; a drifted member stays QUARANTINED with `SOURCE_DRIFT` and both pairs; a failing or
malformed identity command never frees the member; `pool status` shows the observed identity and
when it was observed; removing an allow-list pair does not strand a live allocation and the guard
applies at release; pools without `allowedSources` behave as before.

- `PSR-V0-081`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1586) A pool may
  declare `allowedSources`, 1..16 distinct `{"revision","tree"}` pairs of full Git object ids, and
  `defaultSource`, one of them; both are declared or neither (`MALFORMED`, a duplicate pair
  `DUPLICATE_ID`). `pool acquire --source REV:TREE` asks for a member on that pair. Before any
  allocation or health preparation, a pair outside the pool's current `allowedSources`, or any
  `--source` on a pool without them, is refused `BLOCKED` `OUT_OF_SCOPE` with a detail starting
  `SOURCE_NOT_ALLOWED`; a malformed pair is `MALFORMED`. An admitted acquire records the pair on the
  pools.json entry as an optional `source`. The wire code set is unchanged; the typed reason is the
  detail token.

- `PSR-V0-082`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1586) In a pool
  with `allowedSources`, every member declares `memberConfig[M].identity`, a pool command with the
  existing argv, cwd, env and timeoutSeconds rules, and no member of another pool declares one
  (`MALFORMED`). It is read-only by contract and prints one JSON object `{"revision","tree"}`. It
  does not combine with `safeReuse` in this slice (`MALFORMED`), so `pool sweep` never frees a
  guarded member. The identity command is part of the member definition and is immutable while the
  member is occupied; `allowedSources` and `defaultSource` are pool-level and may change while it
  is.

- `PSR-V0-083`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1586)
  `pool confirm-safe` of a guarded QUARANTINED member runs its identity command in the lease
  observer, against the policy and pool state the transaction decides on (memoized over that state,
  so an observer retry over unchanged state does not run it again, and the writer-checkpoint route
  is kept), through the owned process-group runner (timeout, group kill, bounded output). The
  observation binds pool, member, allocation and definition digest and carries the class
  (`REPORTED`, a runner class, or `MALFORMED_OUTPUT`), the reported pair when `REPORTED`, the stdout
  digest and the runner's wall-clock `observedAt`; the transaction records it in pools.json as the
  member's last `identities` entry with the receipt `observedSeq`. `pool status` for such a pool
  shows `defaultSource`, `allowedSources` and, per member, the allocation's `source` and the last
  `identity` (class, source, allocationId, outputSha256, observedAt, observedSeq) or `NOT_OBSERVED`.

- `PSR-V0-084`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1586) A guarded
  member becomes FREE only when its identity is `REPORTED` with the pool's current `defaultSource`.
  A reported other pair keeps it QUARANTINED with reason `SOURCE_DRIFT: observed R:T; default R:T`;
  any other class (failure, timeout, over-limit, malformed or trailing output) keeps it QUARANTINED
  with reason `SOURCE_IDENTITY_UNPROVED: identity command CLASS; default R:T`. Both are committed
  receipts with outcome `BLOCKED` and code `QUIESCENCE_UNPROVED`, so a request-id replay returns
  the refusal and frees nothing; a confirm-safe with no observation is refused and writes nothing.

- `PSR-V0-085`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1586) The guard
  applies at release, not at policy update: a policy update that removes a pair from
  `allowedSources` leaves every live allocation on it ALLOCATED, and only later acquires of that
  pair are refused. A lane-untouched release of a guarded member does not remove its entry; it is
  quarantined like any other release and freed only through PSR-V0-084.

- `PSR-V0-086`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1586) A pool
  without `allowedSources` is unchanged: its policy, pools.json and `pool status` bytes carry no
  `source`, `identities`, `identity` or source keys, and its members free exactly as before.

- `PSR-V0-168`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1656) The
  PSR-V0-083 identity command never runs while the writer lock or the preparation gate is held.
  On the writer-checkpoint route the observer serves only an observation memoized for the
  byte-identical policy and pool state audited under the lock; otherwise it declines the route
  (`fast.declined: ... identity ...`) without running anything and records that state. The command
  then runs for the recorded state with the lock and gate released, and the route is tried once
  more, where the lock-held observer accepts the observation only if the audited policy and pool
  bytes still equal those it was made for (revalidation under the lock). A second decline prepares
  the complete route, whose observer runs before the gate and whose locked head recheck refuses a
  moved snapshot (`SNAPSHOT_MOVED`, next round), so no observation is ever bound to state it was
  not made for. A concurrent writer therefore waits on this confirmation for at most its locked
  append, never for the identity command; a confirmation whose state is unchanged runs the
  command once. After the command ran unlocked, the write is recorded at the later of the caller's
  time and the wall clock, so a writer that committed meanwhile never leaves it earlier than the
  head receipt. Verdicts, receipts and pools.json bytes are otherwise those of PSR-V0-083/084.
  Limit: the revalidation key binds the policy and pool bytes only; the command's environment
  values and executable contents are not part of it, as for PSR-V0-083's observation.
  Claims (V1-2138): the same deferral covers a `CLAIM` or `CLAIM_NEXT`'s Git observations (the
  base commit and tree, a prepared pool observation's command source, each member's pinned config)
  and its CAL-V0-022 scope deriver. Observations that do not depend on audited state run before
  the route; on the route the lock-held observer serves only memoized results and otherwise
  declines (`fast.declined: ... claim Git observation deferred ...`) without running anything. A
  query naming an exact object (`rev-parse` or `ls-tree` of a full OID) is immutable and memoized
  for its argument list; a derivation is memoized for the identical root, tree, title and body.
  Every other Git query (a symbolic revision, worktree status) is bound to the head.json and
  intent tree digests read before it ran (the snapshot `bindObservation` checks), and the
  lock-held observer serves it only while the audited state carries that snapshot; the root's
  `HEAD` commit is instead bound to its ref files when PSR-V0-224 applies. The
  rest of the observation then runs unlocked for the recorded state and the route is tried once
  more. A journal or intent change meanwhile misses the snapshot-bound observations, and a
  commit integrated meanwhile misses the `HEAD` observation, so the retry declines again and the
  complete route
  observes Git afresh, reusing the derivation; a ticket edited meanwhile misses the derivation
  key. The reprobe is bounded by that single retry. A served observation clears the memo. A claim
  with no deriver resolves no base tree. Rollback: drop the claim's writer observer in
  `leaseOnce`; the route then observes under the lock as before.

- `PSR-V0-224`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2531) Amends
  the PSR-V0-168 claim deferral for the claim root's `HEAD` commit (a `CLAIM` or `CLAIM_NEXT`
  with no base, or base `HEAD`). Before any lock the claim asks Git once where the root's HEAD
  and common refs live (`rev-parse --path-format=absolute --git-dir --git-common-dir`). After the
  unlocked `HEAD^{commit}` query, it resolves HEAD from those files alone: the HEAD file, then up
  to five symbolic refs, each a loose ref file in the common directory (in the worktree's own
  directory for `refs/bisect/`, `refs/worktree/` and `refs/rewritten/`) or, only when that file
  is absent, the exact name's line in `packed-refs`. When that resolution names exactly the commit
  Git reported, the observation is bound to it instead of to the store snapshot; otherwise the
  PSR-V0-168 snapshot binding applies unchanged. The lock-held observer serves a `HEAD`-bound
  observation only when the same file resolution, run under the lock with no Git command, still
  names that commit, whatever the journal did meanwhile. A heartbeat or renew that committed
  between the probe and the lock therefore no longer misses a contended claim's base: a claim
  whose other observations are memoized commits on its first writer-route entry. The resolution reports nothing, so the snapshot binding applies, for a
  layout it does not read exactly as Git's files backend: a reftable store (`reftable` in the
  common directory), a symlink or non-regular HEAD or ref file, a file over 4 KiB, `packed-refs`
  over 8 MiB, an unborn branch, a ref name outside plain `refs/` path segments, or a failed layout
  query. The layout is bound to the root's route: the root's own `.git` (a directory, compared by
  file identity and required to be the reported Git directory, or a file, compared by bytes) and
  that Git directory's `gitdir` and `commondir` files, read before and after the layout query
  and required equal. Every resolution, unlocked or under the lock, re-reads the route before and
  after the chain and reports nothing on any difference, so a root whose linked worktree was moved
  away and replaced by another (a new admin directory) is never resolved through the cached old
  one, and a root without its own `.git` (a subdirectory) has no layout. After reading the target
  ref, each resolution re-reads HEAD and every symbolic ref of the chain and requires them
  byte-identical to the first read; otherwise it reports nothing. A checkout that moves HEAD to
  another branch and resets the branch HEAD left between the two reads therefore cannot pair the
  old HEAD with the reset ref's commit. Binding by the resolved commit, not by file bytes, means
  a ref that moved and moved back while Git ran still serves only the commit it now names. A commit integrated in the claim root
  after the probe is detected even when nothing writes the journal, which the snapshot binding
  alone did not detect. Non-goals: other revisions, the tree, pool sources and pinned configs keep
  their PSR-V0-168 bindings, and no Git command runs under the lock. Failure modes: an unreadable
  layout falls back to the snapshot binding and at worst the PSR-V0-168 reprobe and complete
  route; a HEAD or symref that changed during the chain read, or a changed route (a replaced or
  moved worktree, a changed `gitdir` or `commondir`), refuses the memo so the claim reprobes. Rollback: make `headQuery` report false, restoring the PSR-V0-168 binding.

- `PSR-V0-225`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2537) Amends
  where the PSR-V0-052 and PSR-V0-056 base checks, and the JSG-V0-011 segment re-hash, do their
  snapshot-independent reads; what they accept is unchanged. Before a resumed round takes the
  preparation gate, the lease write runs a precheck through a read lifetime of its own. It re-hashes
  every bound segment and manifest whose base stamps had settled, recording each one whose stamps
  are still the base's and whose size and SHA-256 match the base's digest. It also lists the names
  of every request shard whose modification and change times lie at least the PSR-V0-052
  quiescence before the precheck began, when the shard's own handle reports the stamps its parent's
  listing did. Under the gate, the resumed audit and the recheck before commit then stat every
  segment and compare its stamps, change time included, with the base's. They skip the re-hash
  only for a recorded segment. They list `requests/` afresh, and a shard whose identity, mode,
  size, modification time and change time are the recorded ones, by both that listing and the
  shard's own handle, has its recorded names reused. Every name is still stat'ed and checked as a
  base file or tail-posted request. Any other shard is listed afresh. When any names were reused,
  the `requests/` pathname must, after the last shard, still name the directory handle they were
  reached through, as every listing's binding requires; otherwise the check declines as moved. The precheck admits nothing
  and decides nothing: a fault, a mismatch or an unsettled stamp records nothing, so the gated
  check does all its PSR-V0-052 work and declines as before. A failed close of the precheck's read
  lifetime is cleanup, as PSR-V0-057 defines, and terminates the command. Failure modes: an entry
  made, removed or renamed in a trusted shard after the precheck began moves the shard's
  modification time (or, if that is restored, its change time), so the shard is listed afresh and
  a stray or missing request declines. `requests/` or an ancestor renamed and replaced during the
  gated check fails that binding and declines. A segment or request rewritten after the precheck changes
  its change time and declines by stamps, and one rewritten before it fails the precheck's digest,
  so the gated check re-hashes it and declines. A filesystem whose timestamps are coarser than the
  quiescence could hide a change, the same assumption PSR-V0-052 already makes for base files.
  Rollback: make `PrecheckResume` record nothing (or drop its call in the lease loop); the gated
  checks then run exactly as before.

- `PSR-V0-169`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1653) Amends the
  CAL-V0-117 Scheduling step. On an unsegmented store a writer-route write schedules the complete
  audit when its new head is exactly 512 receipts past `FullSeq`, or 512 plus a multiple of 64. A
  scheduled refresh that publishes nothing (intent-only divergence, pending cleanup, a moved
  journal, a lock failure, a superseded token) leaves `FullSeq` behind, and the old rule (512 or
  more) then scheduled a complete audit after every later write until the 4,096-receipt bound. With
  this rule at most one write in 64 pays one, and no two consecutive writes do. Every refresh,
  invalidation and binding rule of CAL-V0-117/CAL-V0-265 and the 4,096-receipt bound are unchanged,
  so a refresh missed this way is retried 64 receipts later, and the bound still sends every write
  to the complete route. Segmented stores are unchanged (JSG-V0-010). Rollback: restore the
  `>=` rule in `advanceWriterCheckpoint`; no stored byte depends on it.

  Failure modes: a source outside the allow-list (refused before allocation); a drifted, failing,
  timed-out, over-limit or malformed identity (member stays QUARANTINED with the reason); a missing
  observation (refused, nothing written); an observation bound to another allocation or definition
  (`MALFORMED`, nothing written); an allow-list narrowed while occupied (allocation kept, guard at
  release). Non-goals: no identity probe in `pool sweep` or the dispatcher's sweep, no identity at
  claim or acquire time, no retargeting of a member to another source (V1-1585), no trust in the
  command's report beyond its exit and output shape, and no new wire code. Acceptance evidence:
  `TestPSRV0081_SourceIdentityGuard`, `TestPSRV0082_IdentityPolicyRules`,
  `TestPSRV0085_LaneUntouchedKeepsIdentityGuard` and `TestPSRV0086_PoolsWithoutSourcesUnchanged`
  (`internal/tasks/cli`). Rollback: free every guarded member (confirm-safe with a matching
  identity), publish policy without `allowedSources`, `defaultSource` and `identity`, then revert the
  source; an older binary refuses a policy or pools.json carrying these keys as `MALFORMED` (unknown
  key), the recorded compatibility limit. No receipt or record is rewritten.

### V1-1617 bounded acquire calls (issue 1010 follow-up)

Human-owned input: native ticket V1-1617, a follow-up to GitHub
[issue 1010](https://github.com/beamfall/corvint/issues/1010) (closed). The resume base of
PSR-V0-051 lives only in one command's memory, and building it costs one early-stopped complete
attempt and one prefix walk. A client that bounds each `pool acquire` at 150 s and starts a new
process for every call never reaches a resumed round. The reporter saw 11 `ACQUIRED` against 146
`PENDING`, with one member `FREE`, at 47,000 receipts and 3 to 5 fast-route writes a minute. Intent:
bounded calls reach `ACQUIRED` while a member is free, a waiting caller can see how far its acquire
got, and a tampered journal still never commits. The ticket offered two routes: (a) accept a tail
of only heartbeat or renew receipts against the audited head, or (b) carry progress across bounded
calls without an unverified on-disk base. Route (b) is taken; (a) is declined by PSR-V0-110. See
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1617-acquire-bounded-calls.md`.

- `PSR-V0-110`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1617) No tail is
  accepted on its receipt kind, verb or request record. A tail, including one made only of heartbeat
  or renew receipts, MUST reach a commit only through the PSR-V0-052 resume:
  - each receipt validated through the shared step;
  - the request modelled again at the new head;
  - the PSR-V0-056 recheck under the writer lock.
  Any other tail declines into that round's preparation. Route (a) is declined because PSR-V0-052
  already accepts every valid tail of at most 256 receipts, whatever its kinds, and re-models at the
  new head. A kind-classified route would therefore add no resumed round. It would also have to
  trust a kind that only the step can check, and a heartbeat or renew of the acquiring attempt
  itself changes the generation the acquire was modelled on.
- `PSR-V0-111`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1617) Progress
  across bounded calls is the detached acquire. A client that bounds each call MUST issue
  `pool acquire ... --detach --request-id R --wait N`, then `pool acquire --attach --request-id R
  --wait N`, or repeat the same `--detach` (ATR-V0-020..024). The one detached child keeps its
  in-memory resume base across the client's calls, up to its 600-second ceiling (ATR-V0-023).
  Nothing about the audit or the base is persisted or shared between processes, so PSR-V0-051 is
  unchanged. A call without `--detach`, or with a new request id, starts over. A child that reaches
  its ceiling keeps a retryable `LOCK_TIMEOUT` (ATR-V0-053). Once no child runs the record, a
  repeated `--detach` under the same request id starts a new child, which builds its base again,
  and the store's request-ID replay keeps the mutation exactly once (ATR-V0-066). Any other kept
  result answers the request id and starts nothing (ATR-V0-021, ATR-V0-022). Detach is available on
  Darwin and Linux only, the platforms that resume.
- `PSR-V0-112`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1617) The `PENDING`
  item of a detached `claim` or `pool acquire` (ATR-V0-024) MUST carry a `progress` member. It is
  `null` until the child has finished its first preparation round, or when the record does not read.
  Otherwise it is an object with four members:
  - `preparationRounds`: the rounds begun, as `LeaseTiming.Rounds`;
  - `resumedRounds`: the resumed writes;
  - `auditMillis`: the summed head, inventory and audit time;
  - `updatedAt`: an RFC 3339 UTC time.
  The three counts are decimal strings. The child rewrites `progress.json` in its record directory
  after each preparation round. A failed write is logged to `child.log` and never fails the command.
  The member is advisory: no decision, replay or result reads it, `--attach` stays a pure read, and
  the kept result is unchanged byte for byte.
- `PSR-V0-113`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1617) Negative
  evidence for PSR-V0-110, in the style of PSR-V0-053:
  - Another worker's pool acquires (with a heartbeat) committed in the tail after the base audit are
    decided at the new head. The acquire never allocates a member that the audited head showed
    `FREE`, and it decides as the complete route decides that journal.
  - Each of the following, written after a heartbeat commits with its modification time restored,
    never commits, leaves the head unchanged and ends with the complete route's own decision:
    - a heartbeat whose files change pool membership (policy) or free an allocation (pools.json);
    - a forged receipt kind on the tail receipt;
    - a forged request record for the heartbeat.
- `PSR-V0-114`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1617; GitHub #1063)
  The `receipt compact` help MUST note that compaction does not shrink the work of a complete audit
  from receipt 1 (`receipt audit`, or a lease or pool acquire preparation). That audit restores
  each elided receipt before validating it, which costs more per receipt than reading a whole one.
- `PSR-V0-115`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1617; GitHub #1063)
  A journal whose receipts `receipt compact` elided MUST still yield a PSR-V0-051 resume base. The
  prefix walk restores each elided receipt, and an acquire over that journal resumes after one
  complete audit, as it does over a journal without elided receipts.
- `PSR-V0-116`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1617; review round 1)
  The PSR-V0-056 recheck MUST also cover the tail the resumed walk read. Under the writer lock,
  after modelling and before commit, it rereads every receipt after the base and every request file
  the tail posted, and compares each with the bytes that walk consumed (length and SHA-256). A
  difference, or a tail file the walk did not read, is a decline into that round's preparation. A
  tail receipt or tail request record rewritten after the resumed audit, with its modification time
  restored, therefore never commits.

Issue 1063 (pool acquire CPU-bound for 38 minutes or more on a moving journal) is folded in. Its
ask to start the acquire from a writer checkpoint, or to end the prefix walk at the checkpoint
sequence and audit only the tail, is declined here: it is the CAL-V0-264 writer route for pool
verbs, which the owner declined because it drops the CAL-V0-116 rewritten-prefix detection, and it
remains an owner decision. Its ask for walk time and restart counts is met for detached acquires by
PSR-V0-112 (`auditMillis`, `preparationRounds`) and, since V1-1626, by the PSR-V0-152 walk split
of that member; no `doctor` report is added. Its ask for a help note is PSR-V0-114. PSR-V0-115
shows that compaction alone does not keep an acquire on complete audits; the cause of the
reported run is NOT_OBSERVED. The work a command does once its resume is declined is bounded by
PSR-V0-150 (V1-1626).

Non-goals: accepting any tail on its kind (PSR-V0-110); a persisted, shared or cross-process
resume base; a faster prefix walk or audit; a retry count or audit time on a non-detached
`SNAPSHOT_MOVED` result; a detached child beyond the 600-second ceiling; changing CAL-V0-263 or
CAL-V0-264; a writer-checkpoint start or checkpoint-bounded walk for pool acquire (issue 1063).

Failure modes:
- A client that cuts each call and starts a new process without `--detach` still never resumes.
  At 47,000 receipts its first audit alone is extrapolated past 150 s.
- A detached child that reaches its ceiling ends with its own retryable `LOCK_TIMEOUT`, which is
  kept (ATR-V0-022, ATR-V0-053). A later `--detach` with the same request id, once no child runs the
  record, moves that result to `result.1.json` and starts a new child, which builds its base again
  (ATR-V0-066, PSR-V0-111). A child that exits without a result (`LOST`) is also restarted by the
  same `--detach`; any other kept result answers it and starts nothing (ATR-V0-021).
- A stale or missing `progress.json` reports `null` or an older round; it never changes a result.

Acceptance evidence:
- `TestPSRV0113_ResumeDecidesNonHeartbeatTail` and `TestPSRV0113_ForgedHeartbeatTailNeverCommits`
  (4 cases), with `TestPSRV0112_LeaseProgressAfterEachRound`, in `internal/tasks/store`.
- `TestPSRV0116_TailRewrittenAfterAuditNeverCommits` (2 cases) in `internal/tasks/store`, and the
  `rewritten-tail-receipt` and `rewritten-tail-request` cases of
  `TestPSRV0056_RecheckResumeSeesChangeAfterAudit` in `internal/tasks/journal`; each fails without
  the PSR-V0-116 recheck.
- `TestPSRV0111_CeilingRelaunchesOnSameRequestID` in `internal/tasks/cli`.
- `TestPSRV0112_PendingReportsProgress` and `TestPSRV0114_CompactHelpNotesWalkCost` in
  `internal/tasks/cli`; `TestPSRV0115_ResumeAfterCompaction` in `internal/tasks/store`.
- The opt-in harness `TestPSRV0055_PoolAcquireLoad`, extended with three variables:
  - `..._WRITER_KIND=lease`: a dispatcher that claims and then alternates heartbeat and renew;
  - `..._CALL_SECONDS=N`: each acquire cut at N seconds and retried as a new call;
  - `..._CALL_MODE=detach`: one uncut acquire, counted in N-second client windows.
  At 10,000 receipts, 2 acquirers and a heartbeat or renew every 15 s (about 4 writes a minute; 12
  CPUs, host load 69 to 87):
  - calls cut at 150 s, above the first audit at this scale: 4 of 4 acquired, none cut, p50
    56.9 s, max 65.8 s;
  - calls cut at 30 s, with the cut below the first audit as 150 s is at 47,000 receipts: 0 of 6
    acquired, all cut, none resumed, audit max 48.3 s;
  - detach, 30 s windows: 2 of 2 acquired after a resumed round, within 2 windows (58.9 s).
- 47,000-receipt, quiet-host and Linux qualification NOT_RUN. By linear extrapolation from the
  measured 33 to 60 s first audit at 10,000 receipts, the first audit at 47,000 takes 155 to 282 s.
  That exceeds a 150 s call, so at that scale only the detached route reaches `ACQUIRED`, within 2 to
  3 windows of 150 s and inside the 600-second ceiling (an inference, not a measurement).

Rollback: revert the `progress` member, `progress.json`, `WithLeaseProgress`, the compact help
sentence, the harness variables, the tests and this subsection. No store, receipt, checkpoint or kept result changes. A
record directory with a `progress.json` is read by an older build, which ignores the file.

### V1-0785 bounded pool waitlist and hand-off grace (issues 583 and 1067)

Human-owned input: native ticket V1-0785, from GitHub
[issue 583](https://github.com/beamfall/corvint/issues/583) and
[issue 1067](https://github.com/beamfall/corvint/issues/1067). A higher-priority ticket whose live
attempt waits for a member of a small pool loses each freed member to a lower-priority claim that
arrives first, and a clean HANDOFF's successor loses the member it just gave back. The owner's
design decisions (D3) are: a separate `pool wait` command; waiters block only lower-priority
claims, or claims of the same priority that arrive later; a wait of at most 600 s; and hand-off
grace opt-in with a default of 0. See `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-V1-0785-pool-waitlist.md`.

- `PSR-V0-130`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0785)
  `pool wait --pool P --ttl-seconds N --request-id ID` MUST record one `WAIT` entry on pool P's
  waitlist in `pools.json`, in one of two forms:
  - `--attempt A --generation G` registers the live attempt A at generation G, which holds a work
    lease and no allocation, for its ticket and holder;
  - `--ticket T --holder H` registers ticket T for holder H with no attempt.
  The entry records the pool, ticket, kind, holder, the ticket's priority at registration, an
  arrival sequence (the receipt sequence), `since` and `expiresAt`. The request MUST be idempotent
  per request id, and a pool holds at most one entry per ticket. A refresh of an existing live entry
  keeps its sequence, `since` and priority, and only moves its expiry; a refresh of a `GRACE` entry
  keeps the `GRACE` kind and the later expiry. The verb never allocates a member and never refuses
  for lack of a free one. It refuses, with nothing written:
  - a pool that declares no `maxWaitSeconds` (`UNSUPPORTED`);
  - a ticket that is not OPEN, or an attempt that is fenced, holds no work lease or already holds an
    allocation (`TICKET_STATE` or `RESOURCE_COLLISION`);
  - a ticket that requires a different pool (`RESOURCE_COLLISION`);
  - an admission barrier (`PAUSED`).
- `PSR-V0-131`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0785) A pool
  opts in by declaring `maxWaitSeconds` (1..600, `MaxPoolWaitSeconds`). A `--ttl-seconds` outside
  1..600 MUST be refused as `LIMIT_EXCEEDED` by the request shape, and one above the pool's
  `maxWaitSeconds` as `LIMIT_EXCEEDED` by the planner. `pools.json` (`taskman-pool-state/0`) MUST
  carry the optional `waiters` array only when it is non-empty, closed-decoded, at most 512 entries
  (`MaxPoolWaiters`), one per (pool, ticket), with a positive sequence no later than the journal
  head, `expiresAt` after `since`, and `attemptId` and `generation` together or not at all. A store
  without waiters keeps every byte.
- `PSR-V0-132`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0785) A
  pool-taking admission (claim, claim-next, or `pool acquire` with `--pool`) MUST be refused with
  `RESOURCE_COLLISION` naming the head waiter, its kind, priority, sequence and expiry when the
  pool's live entries ahead of the claimant number at least the pool's free members. Admission order
  within a pool is `GRACE` first, then ticket priority, then arrival sequence, then ticket id. A
  claimant with its own live entry yields only to entries ordered before it; one without yields to
  every `GRACE` entry and to `WAIT` entries of the same or higher priority. A lower-priority entry
  never blocks a higher-priority claimant that has no entry. With free members beyond the entries
  ahead, the claimant is admitted. On a pool that opts into priority admission (CAL-V0-101), a
  claimant MUST NOT yield to a competitor that the waitlist makes yield to the claimant, so a
  waiter that the waitlist admits is never refused by priority admission. `claim --next --pool` and
  `plan preview --pool` MUST apply this rule at the transaction's recorded time (the preview at
  its observation time): an entry the waitlist would refuse is deferred as `RESOURCE_COLLISION`
  naming the head waiter, so claim-next takes the first ticket an explicit claim would admit
  instead of refusing on a blocked one.
- `PSR-V0-133`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0785) A dead
  entry MUST NOT block. An entry is dead when its pool is no longer declared, it has expired, its
  ticket is no longer OPEN or already holds a member of the pool, or, for an attempt entry, the
  attempt is no longer live at the registered generation, its lease expired or it holds an
  allocation. Every write of `pools.json`, whichever verb posts it (including `pool release`,
  `pool confirm-safe`, pool membership and slot writes), MUST drop the entries that are dead at
  that write, judged against the attempts the same write posts. An entry never binds a member, so a crash between the wait and the claim strands
  nothing, and dropping an entry never quarantines anything.
- `PSR-V0-134`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0785) A waiting
  attempt's own `pool acquire`, or the claim of a waiting ticket, MUST be admitted ahead of the
  entries ordered after it, and the write that gives the ticket a member drops its entry.
- `PSR-V0-135`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0785)
  `plan preview` and `queue status` MUST show the live waitlist as a read-only `poolWaitlist`
  member (pool, ticket, kind, holder, priority, sequence, `since`, `expiresAt`, age in seconds and,
  for an attempt entry, the attempt and generation), in admission order, and only when it is
  non-empty. Neither view writes.
- `PSR-V0-136`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0785) A pool may
  declare `handoffGraceSeconds` (0..600; default 0, no grace). When it is positive, a clean
  `release --reason HANDOFF` of an attempt that held a member of the pool, or had a live `WAIT`
  entry on it, MUST write a `GRACE` entry for the same ticket that expires after that many seconds.
  The entry reserves one unit of free capacity for the ticket's next generation under PSR-V0-132;
  it does not reserve the specific member. It expires and is dropped like any entry (PSR-V0-133).
  When the waitlist already holds 512 entries besides the ticket's own, that HANDOFF release MUST
  be refused with `LIMIT_EXCEEDED` naming the `GRACE` entry, with nothing written, rather than drop
  the reservation silently; a release with any other reason is unaffected.

Failure modes:
- A waiter that crashes, ends or lets its ticket leave OPEN stops blocking at once (PSR-V0-133)
  and is dropped by the next pools.json write; at worst a claim is refused until the entry expires,
  which is bounded by 600 s.
- A full waitlist (512 entries) refuses the new `pool wait`, and a HANDOFF release whose `GRACE`
  entry it cannot hold, with `LIMIT_EXCEEDED`; existing entries, attempts and members are
  unchanged, and the agent may release with another reason or retry once entries expire.
- An older binary refuses a policy declaring `maxWaitSeconds` or `handoffGraceSeconds`, and a
  pools.json with a `waiters` key, as `MALFORMED` (closed decoders).

Non-goals: no push notification or wake-up (a waiter retries its acquire or claim), no binding of
a specific member, no reordering of an entry after registration (its priority is the snapshot at
registration), no waitlist for pools that do not declare `maxWaitSeconds`, and no dispatcher use of
the waitlist beyond PSR-V0-140 (a waiting attempt's session already counts as pending there).

Acceptance evidence: the focused transaction, snapshot, intent and CLI tests named in Traceability
(PSR-V0-130..143). Concurrency between waiters relies on the existing store lock and was not
separately stress-tested. Linux and live dispatcher qualification NOT_RUN.

Rollback: stop issuing `pool wait`, let the entries expire, and run one lease write so pools.json
drops the `waiters` key; then publish policy without `maxWaitSeconds` and `handoffGraceSeconds`,
and revert the source. No receipt, record or preimage is rewritten.

### V1-1596 dispatch launch room for pool-requiring tickets (issue 1039)

Human-owned input: native ticket V1-1596, from GitHub
[issue 1039](https://github.com/beamfall/corvint/issues/1039). The dispatcher can launch more
sessions for tickets that require a pool than the pool has free members; the extra sessions start,
poll and fail to acquire, each paying full start-up cost. The reporter's work-state script counts
FREE members, subtracts sessions launched but not yet holding one, and holds the excess.

- `PSR-V0-140`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1596) The
  dispatcher's native observation MUST report each declared pool's free members (members that no
  pools.json entry names). In each tick, a ticket assignment that requires an observed pool MUST be
  launched only while that pool's room is positive, and each launch spends one unit of room. Room
  is the free members less the pending workers (PSR-V0-141). The room check runs after the role,
  tier and spend fences and before the pressure budget, in the existing candidate order (pins, role
  priority, ticket priority, order), so higher-priority tickets launch first. Roles and tickets
  without a pool requirement, and a pool the observation does not report, are not gated.
- `PSR-V0-141`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1596) A pending
  worker is a RUNNING worker record for an OPEN ticket of the pool whose ticket has no live attempt
  holding a member of that pool. It MUST count against the pool's room until its attempt holds a
  member (the member then leaves the free count instead) or the worker ends.
- `PSR-V0-142`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1596) A stale
  worker record MUST be ignored: a KILLING worker, one without a pool or ticket, or one whose ticket
  is not observed OPEN.
- `PSR-V0-143`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1596) Each ticket
  held for room MUST be reported by one `pool-wait` dispatch event, added to the closed CAL-V0-058
  event vocabulary, naming the ticket, role, the reason `POOL_ROOM`, and the pool's free, pending
  and remaining room after the tick's launches. A hold is reported again only when those numbers
  change, so `dispatch status` events show each waiting ticket's reason and the computed room.

Failure modes:
- A worker that never claims keeps one unit of room until it ends; the existing idle and wall
  limits end it.
- A store whose observation carries no policy or pool state reports no pool, and the gate is off for
  that tick (the previous behaviour); an observation that fails ends the tick as before.
- A member taken outside the dispatcher reduces the free count on the next observation.

Non-goals: no subtraction of `pool wait` entries from room (a waiting attempt's session is already
pending, so subtracting would count it twice), no new dispatch ledger field (the ledger profile
`taskman-dispatch-state/6` is unchanged), and no change to roles without a pool requirement.
V1-1585 (typed member lifecycle, issue 1028) is NOT STARTED and is not part of this change.

Rollback: revert the room gate, the `PoolFree` observation and the `pool-wait` event kind. The
events log reader does not validate kinds, so an older build reads `pool-wait` lines unchanged, and
the ledger is unchanged.

### V1-1626 bounded complete audit on a moving journal (issue 1063)

Owner input: issue 1063 (P1): `pool acquire` stayed CPU-bound for 38 minutes or more on a 47,400
receipt journal whose head moved every minute or two, its profile dominated by the complete walk.
The orchestrator's decision (2026-10-17) keeps the CAL-V0-116 rewritten-prefix detection and
asks for the root cause and bounded work. The requirements below are accepted under the owner's
standing auto-accept of lane requirements (2026-10-11); see
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1626-bounded-complete-audit.md`.

Root cause (confirmed on the source, and reproduced by `TestPSRV0150_DeclinedResumeWalksOncePerRound`
on the unmodified base): once a command's resume was declined (PSR-V0-052: the prefix walk declined,
or no base could be derived), every later preparation round ran the plain complete audit, which
walks the journal up to four times when the head moves under it, and a round whose audit moved
then ran the guarded complete audit as well, up to four more walks against a head the round had
already lost. Sixteen rounds of up to eight walks at 35 to 60 s a walk is the reported shape. The
harness in the build log refutes the earlier hypothesis for the plain store: there the acquire
resumed in round 2 after one prefix walk, so the decline itself is a precondition, not the rule.

- `PSR-V0-150`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1626; GitHub #1063)
  On a journal at or above the writer-checkpoint floor, on a platform whose audits resume, a lease
  preparation round's complete audit MUST be one attempt that stops as soon as it observes
  `head.json` moved (at the next 512-receipt boundary) and returns `SNAPSHOT_MOVED`, and a round
  whose complete audit returned `SNAPSHOT_MOVED` MUST end there: it does not run the guarded
  complete audit against the head it read at its start. The next round samples the journal again.
  A settled attempt is the ordinary complete audit, with every refusal of its own (CAL-V0-116 is
  intact: PSR-V0-153); no verdict changes. A command whose resume was declined therefore walks the
  journal at most once a round, sixteen times at most, and each walk under a moving journal ends
  within 512 receipts of the move. The prefix-fallback audit (PSR-V0-050) already stopped its
  complete attempt this way and is unchanged; so are `receipt audit` and every read.
- `PSR-V0-151`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1626; GitHub #1063)
  `LeaseTiming` MUST split its audit time by walk: complete walks from receipt 1 and prefix walks
  to a captured head, each with the number that ended `SNAPSHOT_MOVED` and the summed wall time;
  resumed walks from the command's base with their wall time; and the resumes declined (a resumed
  route that declined, a prefix walk that declined, or a base that could not be derived). Each
  count is of audit attempts actually run, never of route calls: a resumed route that declines
  before its audit (an invalidated writer token, for one) charges a decline and no resumed walk,
  and a resumed audit's `SNAPSHOT_MOVED` retry is a second resumed walk with its own wall time.
  Every write audit charges its walks (`PhysicalObservation.Walks`); the split is a diagnostic that
  decides nothing and reaches no receipt, record, evidence or authority. `--timing`
  (`taskman-lease-timing/0`) is unchanged.
- `PSR-V0-152`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1626; GitHub #1063)
  The PSR-V0-112 `progress` member MUST also carry the PSR-V0-151 split as decimal strings:
  `completeWalks`, `completeWalksMoved`, `prefixWalks`, `prefixWalksMoved`, `resumedWalks`,
  `resumeDeclines`, `completeMillis`, `prefixMillis` and `resumedMillis`, in the object's
  canonical (byte-sorted) member order with the PSR-V0-112 members. `progress.json` keeps exactly
  its PSR-V0-112 members (`profile`, `preparationRounds`, `resumedRounds`, `auditMillis`,
  `updatedAt`), because an older reader decodes it with unknown members refused; the child writes
  the same report with the split to a sibling `progress-walks.json` first, which no older build
  opens. A reader takes the split from the sibling only when its PSR-V0-112 members equal
  `progress.json`'s; otherwise (an older child wrote `progress.json`, or the read fell between the
  two writes) the split reads `0`. The member stays advisory (PSR-V0-112).
- `PSR-V0-153`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1626; GitHub #1063)
  Negative evidence for PSR-V0-150: a receipt rewritten with its stamps restored while the bounded
  complete attempt walks, as the journal also moves, never commits. That attempt is
  `SNAPSHOT_MOVED`; the next round's attempt refuses, the head stays where the tamper left it, and
  the command ends with the complete route's own decision for that journal. A refusal that a
  round's preparation audit derived in one complete, unmoved walk of the head it read is that
  round's decision: the guarded complete audit reuses it instead of walking the same head again, so
  a refusing round charges one complete walk (V1-1703). A walk that moved, a declined prefix walk,
  a prefix result and `REDO_PENDING` still take the guarded complete audit.

Failure modes:
- A writer that commits faster than one complete walk, on a command whose resume is declined,
  still cannot settle a complete audit: the command ends `SNAPSHOT_MOVED` after sixteen rounds, now
  each one early-stopped walk, instead of after up to 128 walks. Converging under such a writer
  needs the resume route (PSR-V0-052) or the detached acquire (PSR-V0-111).
- A moved head is observed at 512-receipt boundaries; an attempt may walk up to 511 receipts past
  the move before it stops. On a journal below the floor, or without change times, the four-attempt
  complete audit is unchanged.
- A refusal is still derived twice in its round (the preparation audit and the guarded audit);
  that cost is unchanged and bounded by the refusal.
- A read that falls between the child's two progress writes, or an older child's record, shows the
  walk split as `0` for that report; the PSR-V0-112 counters are always the latest written.

Acceptance evidence:
- `TestPSRV0150_DeclinedResumeWalksOncePerRound` (2 cases), `TestPSRV0151_TimingSplitsResumedAcquire`,
  `TestPSRV0151_DeclinedResumeChargesNoResumedWalk` and
  `TestPSRV0153_BoundedAuditKeepsRewrittenPrefixDetection` in `internal/tasks/store`;
  `TestPSRV0151_ResumedWalksCountEachAttempt` (a `SNAPSHOT_MOVED` retry is two resumed walks) in
  `internal/tasks/journal`; `TestPSRV0112_PendingReportsProgress` in `internal/tasks/cli` checks the
  closed member list, decodes the new child's `progress.json` strictly with the base revision's
  record shape, and reads an older child's `progress.json` without a sibling.
- The opt-in harness `TestPSRV0055_PoolAcquireLoad` reports per-round rows (`rows`: each
  preparation audit's mode or code and wall time, each progress report's round counts). Its
  47,000-receipt before/after runs with hardware and host load are in the build log.

Rollback: revert `Reader.StopOnMove`, `PhysicalObservation.Walks`, the `LeaseTiming` split, the
nine `progress` members, `progress-walks.json`, the harness rows, the tests and this subsection.
No store, receipt, checkpoint, kept result or wire profile changes. `progress.json` keeps its
PSR-V0-112 shape, so an older build reads a new child's record with its counters, and a leftover
`progress-walks.json` is never opened by it and is pruned with its record directory (ATR-V0-025).

### V1-1585 typed pool member lifecycle operations (issue 1028)

Human-owned input: native ticket V1-1585, from GitHub
[issue 1028](https://github.com/beamfall/corvint/issues/1028): pool members need typed lifecycle
operations (provision, refresh, reset, retarget, teardown) with recorded outcomes, instead of
ad hoc scripts outside the store. Acceptance: a pool declares any subset of the five operations and
an undeclared one is refused with a typed reason; an operation in an illegal state is refused with a
typed reason and nothing runs; each run records operation, request id, exit class and output digest
and `pool status` shows the last receipt per operation; a same request id replays and never runs
twice; a failed retarget or reset leaves the member QUARANTINED with the operation named in the
reason; timeouts kill the process group and output is bounded; pools without the new fields behave
as before. This change delivers that subset; the residuals below stay open under issue 1028.

- `PSR-V0-160`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1585) A member
  MAY declare `memberConfig[M].operations`, a closed, non-empty object whose keys are a subset of
  `provision`, `refresh`, `reset`, `retarget` and `teardown`, each a pool command under the existing
  argv, cwd, env and timeoutSeconds rules (`MALFORMED` otherwise). The operations are part of the
  hashed member definition, so they are immutable while the member is occupied.
  `pool operate --pool P --member M --operation OP --request-id ID` names one; an unknown operation
  name is `MALFORMED`, and an operation the member does not declare is refused `BLOCKED`
  `UNSUPPORTED` with a detail starting `OPERATION_UNDECLARED`, before anything runs. The wire code
  set is unchanged; the typed reason is the detail token.
- `PSR-V0-161`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1585) An
  operation MUST start only from its legal member states: `provision` and `teardown` from FREE (no
  pools.json entry), `refresh` from FREE or QUARANTINED, `reset` and `retarget` from QUARANTINED. A
  sweep-owned member is never legal. Any other state is refused `BLOCKED` with a detail starting
  `OPERATION_STATE_ILLEGAL` that names the operation, the legal states and the member state
  (`RESOURCE_COLLISION` for an operation that needs FREE, `QUIESCENCE_UNPROVED` otherwise), and
  nothing runs. The command requires operator attestation (role OWNER or OPERATOR; `UNSUPPORTED`
  otherwise), a barrier refuses it `PAUSED`, and an `--allocation` that differs from the member's
  entry is `FENCED`.
- `PSR-V0-162`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1585) An admitted
  operation commits a PREPARING (from FREE) or CLEANING (from QUARANTINED) entry carrying the
  pending operation before its command runs. Each settled run MUST record a
  `taskman-pool-operation/0` receipt in the optional pools.json `operations` list, one per member
  and operation, sorted by member then operation: pool, member, operation, request id, allocation
  id, class (the pool observation classes plus `ORPHANED`), passed, the output digest (null exactly
  when `ORPHANED`), the retarget source (exactly for `retarget`) and the observing receipt sequence.
  The `pool operate` result carries it as `operation`. `pool status` shows, for each member that
  declares operations, `operations` with each declared operation's last receipt or `NOT_OBSERVED`;
  receipts of members no pool declares are dropped when a receipt is replaced.
- `PSR-V0-163`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1585) A passed
  operation (exit zero, group clean, within bounds) that started from FREE returns an unguarded
  member to FREE. A passed operation on an identity-guarded member (PSR-V0-082) MUST NOT free it,
  since the operation may have changed its physical source: it stays QUARANTINED with a reason
  starting `SOURCE_IDENTITY_UNPROVED`, and only `pool confirm-safe`, whose identity probe must report
  the current `defaultSource` (PSR-V0-084), frees it; a mismatched or unavailable probe keeps it
  QUARANTINED. A passed operation that started from QUARANTINED leaves it QUARANTINED with
  `cleanupPassed` false, so the existing cleanup and `pool confirm-safe` remain the only way back to
  FREE. `retarget` requires `--source REV:TREE` (only retarget takes one; `MALFORMED` otherwise),
  which a pool with `allowedSources` must allow (`OUT_OF_SCOPE`, `SOURCE_NOT_ALLOWED`), and its
  command receives the pair as `CORVINT_POOL_TARGET_SOURCE`.
- `PSR-V0-164`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1585) A same
  request id MUST replay the recorded outcome and MUST NOT run the command again. Once the pending
  entry is committed no result is retryable (CAL-V0-078); a replay of a run whose outcome is not yet
  recorded reports it pending and runs nothing. A replay returns that request's own receipt even
  after a later run of the same operation replaced the member's current receipt: it is read from
  the pools.json the request's own committed observation transaction posted, a bounded read that
  adds nothing to pools.json.
- `PSR-V0-165`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1585) A failed
  operation (any class but a passed `EXIT_ZERO`) MUST leave the member QUARANTINED with a reason that
  starts with the operation and its class. A runner refusal before execution (the configRef blob
  is absent or differs, or the prepared source revision moved or became dirty after admission) MUST
  be settled in the same request as a recorded failed receipt, `SPAWN_FAILED` or `SOURCE_CHANGED`
  respectively, quarantining the member; only when the prepared revision itself is unreadable does
  the entry stay pending for `pool recover`. An operation whose runner is gone is recovered by the
  existing `pool recover`, which records an `ORPHANED` receipt and quarantines the member with a
  reason naming the operation; a live or unknown runner is never recovered.
- `PSR-V0-166`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1585) Operation
  commands run under the existing pool command runner: a timeout or interrupt kills the process
  group and records `TIMEOUT` or `INTERRUPTED`, and output past the 64 KiB bound is
  `OUTPUT_LIMIT`; only the output digest is recorded.
- `PSR-V0-167`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1585) A policy
  without `operations` and a pools.json without `operations` or entry `operation` keys MUST encode,
  decode and behave exactly as before: no new key appears in pools.json or `pool status` for them.
- `PSR-V0-196`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1750) This amends
  the receipt list of `PSR-V0-162`: an operation receipt write MUST retain at most 512 receipts (two
  per member at the 256-member bound), evicting the oldest by observing receipt sequence; the
  receipt it records is never evicted. A full-size pool therefore keeps pools.json under its 1 MiB bound and still
  accepts acquire and release. `pool status` shows `NOT_OBSERVED` for an evicted operation, as
  before its first run; a replay still returns its own receipt through PSR-V0-164's read of the
  observation transaction's posted pools.json. Readers keep accepting the earlier 1,280-receipt
  bound, so an older pools.json stays readable and its next operation write trims it.
- `PSR-V0-202`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2361) A fresh
  `pool recover` reason, the only operator text a pools.json entry reason carries, MUST be 1..224
  bytes of prose; a longer one is refused `LIMIT_EXCEEDED` when the recover is planned, before
  anything is written. The bound is a fresh-admission check only: the lease request digest keeps
  the 4,096-byte prose bound shared with `pool confirm-safe`, so a recover recorded earlier with a
  longer reason still digests and replays its recorded outcome under its original request ID.
  The cap is derived from the worst case it covers: 256 entries, each with a maximum-length
  `configRef`, a source pair and maximum-length identifiers, one of them a CLEANING retarget;
  256 REPORTED identities with a source; and 512 operation receipts encode to 907,069 bytes with
  every reason empty. Each entry may then add the 68-byte fixed prefix plus the reason, which JSON
  escaping at most doubles (prose admits `"`, `\`, TAB, LF and CR, each escaped to two bytes),
  allowing at most 242 bytes; 224 keeps a margin for 20-digit sequences. At the cap, with every
  reason all-escaping, that state measures 1,038,649 bytes and the recovery still writes. Limit:
  the derivation assumes identifiers that need no JSON escape, one running entry, no sweep owner,
  and empty `slots` and `waiters`. Identifiers made of escaping characters, sweep owners on
  every entry, or a large slot or waiter list can still exceed 1 MiB with every reason empty, and
  such a write stays refused `LIMIT_EXCEEDED` (a residual a reason bound cannot remove). Readers
  keep accepting the earlier 4,096-byte entry reason so an older pools.json stays readable.
  Rollback: drop the planning check; recorded reasons are not rewritten.

Failure modes:
- A runner that dies after the pending entry commits leaves the member PREPARING or CLEANING until
  `pool recover` proves the runner gone; the operation is never rerun by a replay.
- A failed `provision` or `refresh` from FREE quarantines the member; recovery is the existing
  cleanup and confirm-safe, or a `reset`, `retarget` or `refresh` from QUARANTINED followed by them.
- pools.json stays bounded at 1 MiB (2 MiB since PSR-V0-266): operation receipts are retained newest-first up to 512
  (PSR-V0-196) and fresh recover reasons to 224 bytes (PSR-V0-202), so neither receipts nor
  entry reasons can fill it within PSR-V0-202's derivation; a write still past the bound is
  refused `LIMIT_EXCEEDED` rather than truncated.
- On a non-Unix host the runner reports `UNKNOWN`, which quarantines.

Non-goals: no operation by the holder of an ALLOCATED allocation, no automatic return to FREE
after a passed `reset` or `retarget`, no retry or scheduling of operations, no dispatcher or sweep
use of operations, no retained output bytes, and no proof that the physical member is in the state
the operation claims beyond its exit status.

Rollback: let every pending operation settle or recover, then publish the policy without
`operations` (each member must be unoccupied, since operations are part of the member definition)
and revert the source. The `operations` list leaves pools.json once its members are no longer
declared. An older binary refuses a policy with `operations` or a pools.json with an `operations`
or `operation` key as `MALFORMED` (unknown key), the recorded compatibility boundary while
operations are declared or recorded. No receipt, record or preimage is rewritten. Reverting
PSR-V0-196 alone restores one retained receipt per member and operation; trimmed receipts are not
restored; each one stays in the pools.json its observation transaction posted (PSR-V0-164).

### V1-1785 compaction cost and the fast route (issue 1063 ask 4)

Human-owned input: native ticket V1-1785, GitHub
[issue 1063](https://github.com/beamfall/corvint/issues/1063) ask 4. After `receipt compact` elided
8,882 receipts, a 2 s sample showed `ReceiptRestorer.Next` in about 20% of complete-walk samples.
PSR-V0-114 already says a complete audit restores each elided receipt; the help did not say which
route avoids that cost.

- `PSR-V0-185`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1785; GitHub
  #1063) The `receipt compact` help MUST also name the fast route that avoids the PSR-V0-114 restore
  cost and say when it does not apply. A writer on the CAL-V0-116 writer-checkpoint route (mutate,
  claim, claim-next, renew, heartbeat, release, and `pool confirm-safe` by CAL-V0-205) and a read
  that resumes at the read checkpoint validate only the receipts after their checkpoint, so they
  restore no elided receipt that checkpoint covers. Compaction removes both derived checkpoints, so
  the next write and the next read each run one complete audit, which pays the restore cost, until
  their checkpoint is rebuilt.

Non-goals: a faster restore, keeping a checkpoint across compaction, or a writer-checkpoint start
for pool acquire (declined above for issue 1063). Failure mode: a store below the writer-checkpoint
floor, or a writer whose route declines, takes the complete audit and pays the restore cost on every
write; the help names the route, not a guarantee that a given write takes it.

Acceptance evidence: `TestPSRV0185_CompactHelpNamesFastRoute` (`internal/tasks/cli`) reads
`receipt compact --help --verbose` and requires the writer-checkpoint route, the read checkpoint and
the post-compaction complete audit; it fails on the base help.

Rollback: revert the help sentence, the test and this subsection. No store or wire format changes.

### V1-0917 and V1-0324 read-checkpoint request and projection checks

A read that resumes from the read checkpoint walks only the tail, and the read checkpoint does not
retain `requests/` entries (CAL-V0-059..061), so the tail walk could not see a tail receipt that
posts a request ID already posted before the checkpoint; the complete audit refuses that history.
Separately, TCP-00 §3.3 lets a read serve the intent projection without a journal check, while
crash matrix C6 listed `INTENT_DIVERGED` under reads. The owner delegated the C6 reading to the
orchestrator on 2026-10-21 (decision 0552): reads stay non-blocking and warn.

- `PSR-V0-190`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0917) On a read
  resumed from the read checkpoint, a tail receipt that posts a `requests/` path with a non-null
  pre-state, which the read checkpoint does not retain, declines the checkpoint route, and the
  complete audit classifies the history (a request ID already posted refuses `JOURNAL_FORKED`). A
  request is posted once, so a non-null pre-state means the path already held a request. The check
  reads nothing beyond the tail receipt it already decoded, so the route stays bounded by the tail.
  Residual: a forged duplicate whose receipt declares a null pre-state passes this check; the
  writer route (its request index) and the complete audit (`receipt audit`, the writer refresh)
  still refuse it. PSR-V0-203 closes this residual on the read route.
- `PSR-V0-191`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0324) A read that
  does not audit the journal still serves the intent projection without blocking (TCP-00 §3.3). It
  compares each `intent/` afterimage the head receipt posts with the projected file's digest, from
  the head receipt the snapshot already read and bound to `head.json`'s `lastSeq` and
  `lastReceiptSha256`; a difference adds a `PROJECTION_STALE` warning naming the projected intent
  tree, the head sequence and receipt digest, and at most four differing paths (then a count). It
  reads no file beyond the snapshot, so it sees only paths the head receipt posted. A read that
  audits the journal (for example `ticket show`, which reads attempt state) and every writer keep
  the audit's `INTENT_DIVERGED` refusal. `PROJECTION_STALE` is a warning token, not a §11 code.
  This amends crash matrix C6 for this repository: unaudited reads report `PROJECTION_STALE`;
  audited reads and writers refuse `INTENT_DIVERGED` (the external TCP-00 wording edit is pending).

Rollback: revert the source; reads again serve a stale projection silently and the read checkpoint
again relies on the complete audit for a prefix request duplicate. No record format changes.

### Kernel runner identity (V1-1008)

V1-1008 (under the load-sensitivity umbrella V1-0849) observed that the runner identity a pool
preparation records came from `ps -p PID -o lstart=` under a 1-second bound, and that at load 46-62
process listings took 0.72-2.43 s, so a health preparation could fail only because the host was
busy. CAL-V0-188 left this as a non-goal.

- `PSR-V0-201`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1008) On Darwin
  the runner identity recorded for a pool health, cleanup, operation or sweep command MUST come
  from the kernel's `kern.proc.pid` answer (start time, as `darwin:<seconds>:<microseconds>`), with
  no process listing, whenever that read succeeds. A failed kernel read, and every other host, MUST
  keep the bounded `ps` listing with unchanged rules. Recovery MUST re-read a recorded identity with
  the reader that produced it: a `darwin:` record with the kernel read, any other record (an
  earlier binary's, or one recorded after a failed kernel read) with the listing. A kernel-read
  failure while re-reading a `darwin:` record MUST stay an error and MUST NOT make the runner count
  as gone.

Non-goals: any change to the listing bound, the group cleanup proof (CAL-V0-188) or the recorded
field's schema (`runnerStarted` stays prose of at most 128 bytes). The earlier non-goal "a Linux
`/proc` identity for the pool" is withdrawn by PSR-V0-204.

Failure modes: a runner recorded by an earlier binary keeps the listing on recovery, so it is never
compared with a kernel identity. A kernel layout the reader does not accept fails the read, which
falls back to the listing at preparation and is an error at recovery.

Rollback: revert the source. Records written as `darwin:` are then compared with a listed identity,
so recovery reports such a runner gone; drain pool allocations (no `ALLOCATED` entry with a
`darwin:` runner) before rolling back.

### V1-2352 null request pre-state on the read checkpoint route

Human-owned input: native ticket V1-2352, the PSR-V0-190 residual (journal-1021s lane, PR #1508).
Observed before this change: a tail receipt that re-posts a request already posted before the read
checkpoint, declaring a null pre-state (the request file removed first), passed a checkpoint read,
while the complete audit refused it `JOURNAL_FORKED`. The read checkpoint carried no request
paths (CAL-V0-059), so the read could not tell the duplicate from a first post. An earlier lane
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-22-tasks-oc-1022.md`) recorded that a fix needs a request set or a
filter without false negatives in the checkpoint; CAL-V0-321 adds that filter.

- `PSR-V0-203`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2352) On a read
  resumed from the read checkpoint, a tail receipt that posts a `requests/` path with a null
  pre-state, which no earlier tail receipt posted, MUST decline the checkpoint route when the
  checkpoint's request filter (CAL-V0-321) may hold that path; the complete audit then classifies
  the history, and a request ID already posted refuses `JOURNAL_FORKED`, exactly as for a non-null
  pre-state (PSR-V0-190). A path the filter does not hold was not posted at or before the
  checkpoint, so the route continues. A checkpoint without a filter MUST NOT serve a read. The
  check reads nothing beyond the tail receipt and the decoded checkpoint, so the route stays
  bounded by the tail. A false positive (about 0.05% per tail request at the filter's sizing)
  costs that read one complete audit and changes no conclusion.

Non-goals: no change to the complete audit, the writer route, the writer checkpoint or any
journal file; no exact request set in the read checkpoint.

Failure modes: a checkpoint written by an earlier binary has no filter, so reads run the complete
audit until the next writer retains a checkpoint (CAL-V0-060); a forged filter that drops a path
can only make the route accept what the complete audit would refuse, the same trust a forged
checkpoint entry already has (the checkpoint stays derived state that every read rebinds to the
retained receipt, CAL-V0-061).

Rollback: revert the source. Checkpoints that carry `requests` are then refused by the older closed
decoder, so reads run the complete audit until a writer retains a checkpoint without it.

### V1-2425 Linux pool runner identity from /proc

Human-owned input: native ticket V1-2425, the residual of PSR-V0-201 (lane perf-1022s, PR #1552).
PSR-V0-201 made the Darwin runner identity a kernel query; Linux still ran a bounded `ps -o lstart`
listing on every pool preparation. Measured on this lane's Linux host
(`BenchmarkPSRV0204_RunnerIdentity`, `internal/tasks/store`): 3.61 to 4.14 ms per call before, 8.98
to 11.90 us after.

- `PSR-V0-204`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2425) On Linux
  the runner identity recorded for a pool health, cleanup, operation or sweep command MUST come from
  `/proc/<pid>/stat` (start tick) and `/proc/stat` (`btime`), as `linux:<btime>:<starttime>` (the
  supervisor's process identity), with no process listing, whenever that read succeeds. A failed
  read MUST keep the bounded `ps` listing with unchanged rules. Recovery MUST re-read a `linux:`
  record from `/proc` and every other record with the listing, as PSR-V0-201 requires for
  `darwin:`. Because the kernel recomputes `btime` on every read, an identity another process
  recorded can differ by one second (V1-1091): a fresh read whose start tick equals the record's
  and whose `btime` is within one second of it MUST count as the recorded runner, and any other
  start tick as a different process. A fresh read whose start tick equals the record's but whose
  `btime` differs by more than one second MUST be unknown (`QUIESCENCE_UNPROVED`), never gone: a
  realtime clock step (`settimeofday`) shifts `btime` while the runner lives, so recovery MUST NOT
  clear the runner's ownership on that evidence. A `/proc` read failure while re-reading a `linux:`
  record MUST stay an error and MUST NOT make the runner count as gone.

Non-goals: no `boot_id` identity format, no tolerance for any other identity or call site (the
V1-1091 residual elsewhere stays open), and no change to the listing, the group cleanup proof or
`runnerStarted`'s schema.

Failure modes: a runner recorded by an earlier binary has a listed identity and keeps the listing on
recovery. An older binary that recovers a `linux:` record compares it with a listed identity and
reports a live runner gone, so older binaries must not run pool recovery against the store while
an entry carries a `linux:` runner (as for `darwin:`, PSR-V0-201). Where `/proc/<pid>/stat` is
hidden (`hidepid`) or the recorded PID belongs to another PID namespace, the read returns no file:
the runner reads as gone, as the `ps` listing reports it; the runner is always the recording
process itself, so this needs the store shared across namespaces. A reboot within one second of
the recording, landing on the same start tick, would read as the same runner; that needs both
coincidences at once. A real reboot that lands on the recorded start tick with a `btime` more than
one second away reads as unknown, not gone, so that entry's recovery is refused until an operator
resolves it; the record carries no stable boot identity (`boot_id`) to tell a reboot from a clock
step, and adding one would change the `runnerStarted` format older records use.

Rollback: revert the source. Records written as `linux:` are then compared with a listed identity,
so recovery reports such a runner gone; drain pool allocations (no `PREPARING`, `CLEANING` or swept
entry with a `linux:` runner) before rolling back.

### V1-1715 claim --next scope derivation through the CLI

Re-homed (owner rule 2026-10-17; batch 1023a): this requirement was first numbered CAL-V0-350 in the
agent leases spec; it moved here as PSR-V0-220 with the same text, because that spec's
`## Requirements` section is at the 256-obligation OCM bound. The old ID is a retired alias: code
comments, messages and test names keep the old numbers. CAL-V0-350 is also the retired alias of
ATR-V0-180 (re-homed 2026-10-17), so `TestCALV0350_ClaimNextDerivesScopeForThePreviewedWaiterCLI`
(`internal/tasks/cli`) names PSR-V0-220 while the `internal/tasks/dispatch` `TestCALV0350_*` tests
name ATR-V0-180.

Human-owned input: native ticket V1-1715, the gap left by the PSR-V0-132 review round. The CLI test
fixtures ran without a scope deriver, so the candidate `claim --next` hands to the deriver
(`NextClaimTicket`) was checked only by construction (both it and the claim select from
`claimNextInput`) and by a transaction-level test; a divergence in the CLI wiring of the inputs the
selection reads (the recorded time that honours the waitlist, the barrier) went undetected.

- `PSR-V0-220`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1715) For a
  ticket that declares no scope, `claim --next --pool P --stage S` MUST ask the CLI's scope
  deriver (`Env.ScopeDeriver`) about the ticket that `plan preview --pool P --stage S` marks
  `SELECTED` in the same store state, with waiters on the pool's waitlist present, and about no
  other; the claim MUST take that ticket and its result's `scope` MUST be the `DERIVED` scope the
  deriver returned for it.

Non-goals: no change to selection, to the deriver contract (CAL-V0-022) or to the default deriver.
Failure mode: a CLI that gives the claim's candidate selection other inputs than the preview, or
that does not pass `Env.ScopeDeriver`, fails `TestCALV0350_ClaimNextDerivesScopeForThePreviewedWaiterCLI`
(the second case fails with a deriver asked about no ticket; the first with a refused claim, as
`TestPSRV0132_ClaimNextPoolClaimsTheWaiterCLI` also does). Rollback: revert the test and fixture
change; no product code changed.

### V1-0911 Linux qualification of the Tasks scale work: the change guard

Re-homed (owner rule 2026-10-17; batch 1023a): this requirement was first numbered CAL-V0-364 in the
agent leases spec; it moved here as PSR-V0-221 with the same text, because that spec's
`## Requirements` section is at the 256-obligation OCM bound. The old ID is a retired alias: code
comments, messages and test names keep the old numbers.

Human-owned input: native ticket V1-0911. The scale work (V1-0893, V1-0645; CAL-V0-135..142) was
measured on Darwin only; the `/proc` process reader (CAL-V0-136), the idle CPU of the supervisor and
dispatcher (CAL-V0-137, CAL-V0-139) and the `ReadDir` listing that relies on directory-entry types
(CAL-V0-138) were not run on Linux.

- `PSR-V0-221`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0911, V1-1569) On
  Linux the change guard (CAL-V0-026, CAL-V0-219) MUST register the directories of an observation
  with inotify first, and then regular files with inotify only up to a per-guard ceiling of a
  32nd of the user's `fs.inotify.max_user_watches` allowance, at most 4,096 (256 when the allowance
  cannot be read). Every other regular file, and any regular file whose watch the kernel refuses
  with `ENOSPC`, MUST be tracked by the stat identity taken when the file was visited (device,
  inode, mode, size, modification and change time) and re-read in `Sweep`, as Darwin tracks a
  file beyond its descriptor budget, so that a difference or a failed read reports
  `SNAPSHOT_MOVED` at the next check, and registration MUST NOT fail for want of a file watch. A
  directory whose watch the kernel refuses MUST still fail the registration, as
  `UNSUPPORTED_FILESYSTEM` naming the allowance (`RESOURCE_EXHAUSTED` since CAL-V0-900), because directory events are what report an
  entry created, removed or renamed at any time. The ceiling bounds the regular-file watches of
  one guard independently of the store's receipt count; it does not bound the guard's directory
  watches, and it does not bound the sum over guards: concurrent writers, each with up to its
  ceiling of file watches plus every mandatory directory watch, can still spend the per-user
  allowance that all of them share (32 populated guards take the whole allowance in file watches
  alone), and a directory refused then fails the registration as above.

Non-goals: a change to the host's watch allowance, or a per-process total across guards (a guard
is short-lived and the ceiling bounds each); watching every file on a host whose allowance would
allow it, which is the coverage this requirement gives up for files beyond the ceiling, on the
terms CAL-V0-219 already accepts for Darwin: only an in-place write or mode change to such a file
by an actor outside the writer lock, made after the sweep read it, goes unreported, and no Tasks
writer edits a watched file in place.

Failure modes: a host whose allowance other software or concurrent guards have already spent (an
editor watching a large tree, or 32 or more live guards on populated stores) refuses a directory
with a message that names the sysctl, and the remedy is to raise it; a store whose directories
alone exceed the allowance cannot register on that host at all;
`inotify_init1` is bounded by `fs.inotify.max_user_instances` (128 on the qualification host) and
fails the registration with its own error when that many guards are live. The sweep re-reads every
file beyond the ceiling once per complete-route commit, before the lock.

Rollback: revert `change_guard_linux.go`, the `finish` hook in `change_guard.go` and the tests; a
store with more files than the allowance then fails as before.

### V1-2482 Linux change guard: watch directories, not files

Human-owned input: native ticket V1-2482, filed from the 2026-10-23 WSL2 quiet-host run
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-wsl2-quiet-host-measurements.md`), where the change guard took one
inotify watch per store path and concurrent guards over 45,000 receipts spent the user's
`fs.inotify.max_user_watches`. PSR-V0-221 bounded a guard's regular-file watches at up to 4,096;
measured on a 4 vCPU Linux VM with an allowance of 130,016, one guard still held 2,312 watches over
1,000 receipts and 4,333 over 45,000.

- `PSR-V0-222`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2482) On Linux
  the change guard MUST register no inotify watch for a regular file whose only link is in a
  directory the guard watches with the content mask, and the number of watches one guard holds
  MUST therefore be the number of observed directories plus their ancestors, independent of the
  number of receipts, requests or other regular files in the store. A directory watched with the
  content mask reports `IN_MODIFY`, `IN_ATTRIB` and `IN_CLOSE_WRITE` for every entry in it, as
  well as entries created, removed or renamed, so every mutation class CAL-V0-026 lists for such a
  file MUST still report `SNAPSHOT_MOVED` at the next check, including `CheckEvents` under the
  writer lock, without a sweep. Every regular file MUST also be tracked by its stat identity
  (PSR-V0-223). This supersedes PSR-V0-221's per-guard file-watch ceiling, which no longer exists;
  PSR-V0-221's directory refusal (`UNSUPPORTED_FILESYSTEM` naming the allowance) is unchanged.
- `PSR-V0-223`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2482) On Linux
  the change guard MUST record, for every regular file it visits, the stat identity read when it
  was visited: device, inode, mode, link count, size, modification time and change time. `Check`
  and `Sweep` MUST re-read the identity of every such file and report `SNAPSHOT_MOVED` at that and
  every later check when one differs or cannot be read, so a hard link made to the file after
  registration, and a write, truncation or mode change through such a link or through a descriptor
  opened through another link before registration, made before the re-read, is reported even
  though no watched directory sees it. A file with more than one link at registration, or whose
  parent is not watched with the content mask, is uncovered: every poll, including `CheckEvents`
  under the writer lock, MUST re-read it as well, so a write through its other link after `Sweep`
  is reported under the lock. The guard MUST NOT add an inotify watch for any of this, so its
  watch count stays the PSR-V0-222 bound, and the work under the writer lock grows only with the
  uncovered files, none of which a Tasks writer makes (CAL-V0-026).
- `PSR-V0-274`: (experimental under the standing lane auto-accept (2026-10-11), V1-2689) On Linux
  the change guard watches every ancestor of an observed root, and the kernel reports `IN_ATTRIB`
  for every entry of a watched directory, not only for the directory itself. An ancestor watch
  MUST therefore invalidate the observation only for the ancestor's own events (its deletion,
  rename or attribute change, an ignored or unmounted watch, and a queue overflow) and for events
  naming its entry on the path to an observed root; an event naming any other entry of an
  ancestor, such as a mode or time change on a sibling of the repository or on another entry of
  the system temporary directory made by an unrelated process, MUST NOT report `SNAPSHOT_MOVED`.
  A directory watched with the content mask keeps reporting every event (PSR-V0-222), so no
  mutation class CAL-V0-026 lists loses coverage, and the guard's watch count (PSR-V0-222) and the
  stat identities (PSR-V0-223) are unchanged. `poll` MUST decode whole events and MUST treat a
  buffer it cannot decode as a change, and a poll that reaches its read bound without draining
  the queue MUST report a change, because an undecoded event may name the store.

Change in guarantees: against PSR-V0-221, every file a directory watch covers gains an in-place
write or mode change at any time, which PSR-V0-221 lost for a file beyond the ceiling after the
sweep read it, and every other change keeps the coverage PSR-V0-221 gave a file beyond the
ceiling. The kernel reports an entry's events to its directory only through that entry, so a
change through another path is seen by the PSR-V0-223 stat identity instead: a link made outside
the observed tree changes the link count and change time, and a write or mode change through it,
or through a descriptor opened through a since-removed link or as an `O_TMPFILE` before
registration, changes the size, modification or change time. Residual limits, each by an actor
outside Tasks:

- a link made to a singly linked file after `Sweep` read it, and a write or mode change through
  that link or through such a descriptor in the same interval, are not reported by `CheckEvents`
  under the writer lock; that interval is the one Darwin already leaves for a file beyond its
  descriptor budget (CAL-V0-219), and a later `Check` or `Sweep` reports it unless the change is
  undone first;
- a change that restores the link count and size, and whose times fall within the filesystem's
  timestamp granularity of the file's previous change, is not seen by the stat identity; kernels
  with multigrain timestamps (Linux 6.13 and later on ext4, XFS, btrfs and tmpfs) give a change
  made after a stat read a distinct change time;
- a write through a shared `mmap` raises no inotify event; the stat identity sees only a write that
  faults its page writable, which updates the modification and change times on a filesystem that
  hooks that fault (ext4, XFS, btrfs) but not on tmpfs (PSR-V0-226), and not a later write to a
  page already mapped writable, which only CAL-V0-070's content digests show
  (`TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit` witnesses both on Linux).

No Tasks writer edits a store file in place or links one outside the store, and no store file had
more than one link in the 45,000-receipt fixture.

Non-goals: Darwin, whose kqueue watch needs a descriptor per file and keeps its budget
(CAL-V0-219); a bound on the store's directory count, which its layout sets (fixed directories and
at most 256 `requests/` shards); the sum over concurrent guards, which is now directories times
guards; `fs.inotify.max_user_instances`. For PSR-V0-274: filtering the events of a directory
watched with the content mask, which stay the store's own; Darwin, whose kqueue vnode reports no
entry's attribute change to its directory.

Failure modes: a host whose allowance is already spent refuses a directory, as under PSR-V0-221; a
kernel that did not report child events on a directory watch would miss in-place writes to
singly linked files under the writer lock, which
`TestPSRV0222_ChangeGuardSeesFileMutationsThroughItsDirectory` detects with no file watch
registered. A file that disappears or cannot be read when re-read reports `SNAPSHOT_MOVED`, never
success. The re-read of every file costs about as much as PSR-V0-221's sweep of the files beyond
its ceiling, which at 45,000 receipts was nearly all of them. Before PSR-V0-274, any process
changing the mode or times of a direct entry of `/tmp` (as the `internal/analyzerexec` and
`internal/testsupport` test fixtures do in the same CI shard) invalidated every live guard on the
host: `WatchChanges` failed its registration check, `observeMutation` fell through to a fresh
audit, and `commitLease` retried after the check under the writer lock, which
`TestCALV0070_MutateRetriesAuditWithoutWatch`, `TestCALV0070_PinnedInventoryMutateParity` and
`TestJSGV0015_LeaseCommitStagesCheckpointsOutsideWriterLock` (`internal/tasks/store`) reported as
`SNAPSHOT_MOVED`, a `fresh` stage and an O(store) checkpoint under the lock. A flood of unrelated
events is drained by at most 64 buffers per poll; what remains is read by the next poll, and a
queue that overflows meanwhile reports `IN_Q_OVERFLOW`, which still invalidates.

Acceptance evidence: `TestPSRV0222_ChangeGuardWatchCountIsBoundedByDirectories` counts the
`inotify wd:` lines of `/proc/self/fdinfo` while a guard is live and requires the same count over
10 files and over 5,010; on the PSR-V0-221 code it fails with 20 against 4,072. Measured with the
store fixture (`historyStoreAt`, 4 vCPU Linux VM): 265 watches per guard at 1,000 receipts and
270 at 45,000. `TestPSRV0055_PoolAcquireLoad` at 45,000 receipts, 8 acquirers and 180 s, with
`max_user_watches` lowered to the host's existing use plus 4,000, completed every call with no
error code, so the peak over all nine processes stayed below 4,000 watches (UNQUALIFIED: load
9.35 on 4 CPUs). The 45,000-receipt, 8-acquirer, 600 s run on the WSL2 quiet host is NEEDS-HOST.
For PSR-V0-223, `TestPSRV0223_ChangeGuardSeesAWriteThroughAnAliasLink` (a link made after
registration, alone, with a same-size write, with a resizing write then removed, with a mode
change, and a resizing write through a descriptor opened through a link removed before
registration) and `TestPSRV0223_ChangeGuardSeesASecondLinkWriteAfterTheSweep` (a resizing write,
a truncation and a mode change through a second link after `Sweep`, seen by `CheckEvents`) fail
on the PSR-V0-222-only code and pass with it (WSL2, Linux 6.18). Opt-in
`TestPSRV0223_ChangeGuardSweepCost` (`CORVINT_TASKS_GUARD_SWEEP_COST=10000,45000`, ext4, WSL2
Linux 6.18, 32 vCPU, load below 2.1) measured the median `Check` and `Sweep` at 47 ms over 20,057
files (10,000 receipts) and 250 ms over 90,057 (45,000), against 42 ms and 256 ms for
PSR-V0-221 on origin/main; `CheckEvents` stayed under 1 µs median, 15 µs maximum.
For PSR-V0-274, `TestPSRV0274_ChangeGuardIgnoresUnrelatedEntriesOfAnAncestor` fails on the
PSR-V0-223 code in its four admitted cases (a mode change on a sibling file and on a sibling
directory of the repository, a mode change and a time change on a direct entry of the system
temporary directory, each reported as `SNAPSHOT_MOVED`) and passes with it, as do its reported
cases (a mode change on the entry on the path, a mode change and a rename of the ancestor itself)
and `TestCALV0026_ChangeGuard` (WSL2, Linux 6.18, `-count=5`). With a concurrent in-process storm
of `MkdirTemp`, `Chmod 0700`, `Remove` on direct `/tmp` entries, the three `internal/tasks/store`
tests above fail on the PSR-V0-223 code with the CI shard's messages and pass with PSR-V0-274
(`-count=3`, WSL2).

Rollback: revert `change_guard_linux.go`, `change_guard.go` and their tests to PSR-V0-221; the
guard then holds up to its file ceiling of file watches again. Reverting PSR-V0-223 alone restores
the PSR-V0-222 code, whose `Check` costs nothing per singly linked file and does not see a change
through a link outside the observed tree. A directory already watched
with the content mask is not re-registered, so a later add of it as an ancestor cannot replace
its mask (`TestPSRV0222_AncestorAddKeepsTheContentMask`). Reverting PSR-V0-274 alone (the
`ancestors` map, `allow` and `invalidated` in `change_guard_linux.go`, the named ancestor
recursion in `WatchChanges`) restores a `poll` that treats any event bytes as a change, with the
unrelated-entry false positives above.

- `PSR-V0-226`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2554) Amends
  PSR-V0-223's shared-mapping limit. Linux tmpfs gives a shared mapping no write-fault hook, so on
  tmpfs the first write through a new shared `mmap` also changes no stat field and the stat
  identity MUST NOT be relied on to see it; only CAL-V0-070's content digests do, and `Mutate` MUST
  still refuse such a write made after the merged audit with the separate passes' code and publish
  nothing. The witness MUST detect tmpfs by the store file's statfs type (`TMPFS_MAGIC`) and name it
  in its log; there it MUST require the watch to report nothing for the first write, and on every
  other Linux filesystem it MUST still require `SNAPSHOT_MOVED`, so a host whose temporary
  directory is ext4, as on the GitHub-hosted runner, keeps the PSR-V0-223 assertion.

Failure modes: a Linux host whose `/tmp` is tmpfs (Ubuntu 26.04, including the WSL2 offload host
and the self-hosted runners on it) failed the PSR-V0-223 first-write assertion at origin/main
0949876eb although no product code differed; a filesystem other than tmpfs that hooks no write
fault still fails the witness, which then names the gap rather than passing silently. Acceptance
evidence: `TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit` fails on the tmpfs host at
origin/main and passes there with this change, and passes with `TMPDIR` on ext4 at both
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-wsl2-tmpfs-mapped-write.md`). Rollback: revert the test's statfs
check; the guard code is unchanged.
- `PSR-V0-247`: (proposed; V1-2537) Amends PSR-V0-052's base receipt and base file stamp check. A
  base file whose stamps had not settled when the base capture began (the files PSR-V0-052 already
  re-reads against their digests) MUST be accepted when it is the same file with the same mode,
  size and modification time and its re-read matches the digest the base audit recorded, even if
  its change time moved. A base file whose stamps had settled MUST still keep every stamp, change
  time included. What a resumed write accepts is otherwise unchanged.

Failure modes: a writer link-publishes each receipt and post from its staging slot and unlinks the
slot only after it publishes the head, so a base audit that runs after a peer's head publish
captures that peer's newest receipt with a second name; the unlink then moves the receipt's change
time and nothing else. Under PSR-V0-052 alone every such base declined with `receipts/N.json:
checkpoint unusable: base file changed since the base audit` (WSL2, 40k receipts, 15 writers:
15 of 58 widen and 8 of 31 reap writes over 5 x 120 s), and each decline fell back to a 12 to 50 s
complete audit. A file rewritten in place to the same size with its modification time restored
still fails the re-read; a replaced file fails the identity check; a mode change fails the stamp
check. Acceptance evidence: `TestPSRV0247_UnsettledBaseFileSurvivesALinkCountChange`
(`internal/tasks/journal/resume_link_count_test.go`) captures a base while the newest receipt has
a second link, removes the link, and sees the resumed audit and the recheck before commit accept
it; it fails with the production message without this change. In the same test an unsettled base
request rewritten in place, and a settled receipt whose change time moved, still decline.
Rollback: revert `resumeFile.stamped` to the full stamp comparison.

- `PSR-V0-227`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1091) Amends
  PSR-V0-204's non-goal "no tolerance for any other identity or call site". The tolerant comparison
  MUST live once, in the supervisor (`CompareIdentity`), and every Tasks site that compares a fresh
  `ProcessIdentity` read for a recorded pid with an identity another process recorded MUST use it:
  the pool runner, the program owner and leader, the workflow remover and cleanup owner, the
  dispatch worker tree, session, leader, signal, detached run and reader leader, the service pulse,
  the CLI detached run and lease child, and the supervisor's boot, owned-group, escape and member
  checks. Equal start ticks with `btime` at most one second apart MUST be the same process; another
  start tick, a non-`linux:` mismatch or an empty identity MUST be different; equal start ticks with
  `btime` further apart MUST be unknown. A site where a live process blocks an action MUST treat
  unknown as possibly live (`QUIESCENCE_UNPROVED`, uncertain, or may run); a site where liveness
  permits an action (signal, membership, heartbeat ownership, pulse `RUNNING`) MUST treat unknown as
  not the recorded process; a status reader with an `UNKNOWN` state MUST report it. The dispatch
  worker tree refresh MUST keep the recorded tree and return an uncertainty error for an unknown
  member or leader, as for an unreadable identity, so a worker is never ended or its kill reported
  done on that evidence. A comparison of
  two identities the same process recorded, or of an executable identity, MUST stay exact. The
  identity format MUST NOT change.

Failure modes: a reader in one process saw a runner, owner or worker recorded by another process
one second off and reported it gone, so a live owner could be taken over, a live worker tree treated
as stopped, or a live pulse reported `STALE`; a realtime clock step that moves `btime` by more than
one second while the process lives stays unknown at every blocking site rather than gone. Acceptance
evidence: the `TestPSRV0227_*` tests in `internal/tasks/supervisor`, `store`, `dispatch`, `service`
and `cli` cover, per site class, a one-second jitter tolerated, a start-tick mismatch different and
a gap above one second unknown; `TestPSRV0227_BtimeSampleWithinTolerance` (Linux) samples
`/proc/stat` `btime` and found 15,000 reads with one distinct value on the WSL2 host
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-v1-1091-cross-process-identity.md`). Rollback: revert the commit; sites
return to exact equality and no record or receipt changes.

### V1-2636 pool recovery stall (issue 1743)

A member whose cleanup or health keeps failing outside Tasks stayed QUARANTINED with no recorded
failure, no quarantine time and no signal: an acquire waited or timed out with a bare
`RESOURCE_COLLISION`, and dispatch kept launching sessions that could only wait. These
requirements record the failure, count it and make the stall a typed condition.

- `PSR-V0-260`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2636) `corvint-tasks
  pool observe --member ID --kind health|cleanup --result FAILED --detail-file FILE --request-id ID
  [--allocation SHA256]` MUST record one failed readiness observation of a QUARANTINED member as a
  lease write (`POOL_REPORT`). The CLI MUST read at most 1 MiB of the detail file (a larger one is
  refused `LIMIT_EXCEEDED`, never cut silently), replace invalid UTF-8 and every code point prose
  refuses with `?`, replace every line the shared secret screen touches with `[REDACTED]`, keep only
  the last 2,048 bytes cut at a rune boundary and screen that tail again, and encode the canonical
  `taskman-pool-recovery-detail/0` document (`kind`, `result`, `detail`, `bytes`, `truncated`,
  `redactedLines`). The request MUST bind the document's digest as its evidence; the write MUST
  refuse a detail that differs from that digest or is not canonical (`MALFORMED`) or still matches
  the screen (`SECRET_DETECTED`), and store the document under `evidence/<sha256>`. The entry's
  `recovery` record MUST carry kind, result, `detailSha256`, `consecutive`, `firstObservedAt`,
  `observedAt` (the receipt's recordedAt) and `observedSeq`. Only `FAILED` is accepted; the flags
  are required together and refused on every other verb, and `--evidence` is refused on `pool
  observe` (`MALFORMED`). A member that is not QUARANTINED is refused `QUIESCENCE_UNPROVED`; an
  allocation mismatch is `FENCED`; a member a sweep owns is `RESOURCE_COLLISION`. A replay returns
  the recorded outcome and never counts again. `pool status` MUST show the record as the member's
  `lastHealth` or `lastCleanup` `{result, passed:false, observedAt, observedSeq, detailSha256,
  consecutive, firstObservedAt}`. A native health or cleanup observation of the same kind,
  including a sweep cleanup phase observation whatever its outcome, supersedes and clears the
  record. Redaction cost MUST stay near-linear in the admitted input.
- `PSR-V0-261`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2636) Amends
  PSR-V0-013's `since`. Every planned pools.json write MUST stamp `quarantinedAt` = the writing
  receipt's recordedAt on an entry that becomes QUARANTINED, and keep the recorded value (or its
  absence) when the entry continues a quarantine of the same allocation: previously QUARANTINED, or
  CLEANING with a recorded `quarantinedAt`. CLEANING MUST carry a continuing quarantine's value and
  recovery record; every other state, and a CLEANING entry that continues no quarantine, MUST carry
  neither. `pool status` `quarantine.since` MUST be the recorded `quarantinedAt`; an entry written
  before this change MUST stay `NOT_OBSERVED`, and no time is ever inferred. Because the time is
  the receipt's recordedAt, redo and replay reproduce the same bytes. A pools.json no entry of
  which changes MUST stay byte-identical.
- `PSR-V0-262`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2636) A failure
  with the same kind and detail digest as the recorded one MUST increment `consecutive` (capped at
  2^31-1) and keep `firstObservedAt`; any other failure MUST restart the count at one. A
  QUARANTINED member's recovery is stalled when `consecutive` is at least 3
  (`CONSECUTIVE_FAILURES`) or, failing that, when a recorded `quarantinedAt` is at least 24 hours
  before the judging time (`QUARANTINE_AGE`). A legacy entry without `quarantinedAt` never ages, and
  no other state is ever stalled.
- `PSR-V0-263`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2636) `pool
  status` MUST add `recoveryStalled` `{code: POOL_MEMBER_RECOVERY_STALLED, trigger, since,
  consecutive, firstFailedAt, lastFailedAt, detailSha256, excerpt}` to a stalled member, where the
  excerpt is at most the last 256 bytes of the digest-verified redacted detail (null when there is
  no record or it is unreadable), judged against the reader's clock and writing nothing. A pool whose
  every member is a stalled QUARANTINED one MUST add the pool-level `recoveryStalled {code,
  members}`. Members and pools that are not stalled MUST show no new key.
- `PSR-V0-264`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2636) When an
  acquire or claim finds no free eligible member and every eligible member (after stage
  reservation and exclusions) is a stalled QUARANTINED one at the receipt's recordedAt, it MUST be
  refused with the new non-retryable code `POOL_MEMBER_RECOVERY_STALLED`, naming the pool and up to
  eight members and pointing to `pool status --pool ID`, instead of `RESOURCE_COLLISION`. Any
  other unavailable member keeps the existing `RESOURCE_COLLISION` path.
- `PSR-V0-265`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2636) Amends
  PSR-V0-140 and PSR-V0-143. Dispatch MUST hold every launch that requires a pool whose every
  member is stalled, even when room is reported, and each held ticket's `pool-wait` event MUST
  carry reason `POOL_MEMBER_RECOVERY_STALLED` and a message naming the condition and `pool status
  --pool ID`. A role without a pool requirement is unaffected.
- `PSR-V0-266`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2636) The
  pools.json bound MUST be 2 MiB (`wire.MaxPoolStateBytes`) for the store, archive layout and
  fixture sessions: the PSR-V0-202 worst case with a `quarantinedAt` and a widest `recovery` record
  on every entry and every reason at the cap measures 1,117,753 bytes. `quarantinedAt` and
  `recovery` are additive optional keys valid only on a QUARANTINED or CLEANING entry; an entry
  without them encodes byte-identically to before, and receipt, request preimage and journal
  formats do not change.
- `PSR-V0-267`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2637) Amends
  CAL-V0-097 and PSR-V0-265. When the default plan defers a ticket because its required pool has
  no free member and every member of that pool is a stalled QUARANTINED one (PSR-V0-263, at the
  reader's clock in `plan preview`, `ticket show` and dispatch), the entry MUST be DEFERRED with
  reason `POOL_MEMBER_RECOVERY_STALLED` and the pool as its blocker, instead of
  `RESOURCE_COLLISION`; `ticket show` reports it as `claimabilityReason`. Dispatch MUST treat such
  an entry, for a role whose match differs only by `planSelected`, as a PSR-V0-265 hold: no attempt
  launches and its `pool-wait` event carries reason `POOL_MEMBER_RECOVERY_STALLED` and the pool
  status route. Core's plan-history decoder MUST accept that pool blocker on the new reason as it
  does on `RESOURCE_COLLISION`. Any other pool deferral is unchanged.

- `PSR-V0-273`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2711) Amends
  PSR-V0-260. `pool observe --detail-file` MUST keep the shared bounded no-follow reader
  (`intent.ReadFile`): a detail file with a symlinked ancestor, including the macOS default
  `TMPDIR` under `/var` -> `/private/var`, is refused `ERROR` `UNSUPPORTED_FILESYSTEM` and records
  nothing; the operator names the resolved path. Decision (V1-2711): the darwin PSR test failures
  were fixed in the tests, not the product. The detail is digested into stored pool evidence, and
  every other operator-named input (config, evidence and intent files) already refuses a symlinked
  ancestor; following ancestors in one verb would make it the only reader that can be redirected
  after naming. The tests name detail files under a resolved temp directory, as V1-0753 and
  V1-0960 did for the same macOS layout. Non-goal: a path-resolving `--detail-file` mode. Failure
  mode: an operator script that passes a `/var/...` or other linked path on macOS is refused and
  must resolve it (for example with `realpath`). Acceptance evidence: Traceability
  (PSR-V0-260..267, PSR-V0-273). Rollback: revert the test commit; no product, receipt or format change.

Failure modes: an older binary refuses a pools.json carrying `quarantinedAt` or `recovery` as
`MALFORMED` (unknown key) and one past 1 MiB as `LIMIT_EXCEEDED`; that is the recorded
compatibility limit while any quarantine stamped by this change is retained. A failure recorded
for a member that is then freed is dropped with the entry. The quarantine-age trigger uses the
reader's clock in `pool status` and dispatch and the receipt's recordedAt in acquire and claim, so a
skewed clock moves when the age trigger fires but never stamps a time. The redaction relies on
the shared secret screen; a secret it does not recognise is stored in the tail, as with captured
logs (non-goals). Acceptance evidence: Traceability (PSR-V0-260..267, PSR-V0-273) and
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-pool-recovery-stall.md`. Rollback: free or confirm every quarantined
member so no entry carries the new keys, then revert the commit; the 2 MiB bound reverts with it
only once pools.json is back under 1 MiB. No receipt, request or evidence is rewritten.

### V1-2704 operator hold outside a dispatched session (issue 1813)

An operator who needed a pool member for a job outside any dispatched session (a hand-run
migration, a manual repair) had no supported write: claiming a ticket abused the attempt lifecycle,
and editing pools.json bypassed the journal. Dispatch could hand the same member to a session in
the meantime. These requirements journal an operator hold as a pools.json entry, the record type an
allocation already uses, and route its end through the existing quarantine and confirm-safe path.

- `PSR-V0-268`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2704) `corvint-tasks
  pool hold --pool ID --member ID --holder ID --lease-minutes N --reason TEXT --request-id ID` MUST
  record one lease write (`POOL_HOLD`) that adds a pools.json entry for a FREE policy member with
  state `HELD`, the holder, the reason (prose of 1..224 bytes, the pool recover reason bound), an
  allocation id derived from the queue, request id and member (so a replay names the same one), the
  member's current definition digest, and a `hold` window `{heldAt, expiresAt}` where heldAt is the
  receipt's recordedAt and expiresAt is heldAt plus the requested minutes (5..1440, the lease
  bound). The result MUST carry `allocationId` (null unless the hold was recorded). A HELD entry
  MUST carry no attempt, runner, command kind, sweep, share, source or operation, and `hold` MUST
  bind exactly a HELD entry with expiresAt after heldAt; the decoder refuses anything else
  `MALFORMED`. Only an OWNER or OPERATOR actor may hold, renew or release (`UNSUPPORTED`
  otherwise); a barrier refuses a fresh hold `PAUSED`; an unknown pool or member is `MALFORMED`; an
  occupied member, whatever its state, is `RESOURCE_COLLISION`. An exact replay returns the
  recorded outcome and writes nothing.
- `PSR-V0-269`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2704) A HELD
  member MUST be occupied for every allocation path: claim, claim-next, pool acquire and the
  dispatcher's ordering MUST pass it over exactly as they pass over any other occupied member, and
  the sweep and cleanup paths, which select only QUARANTINED members, MUST NOT touch it. A policy
  update that changes a held member's definition is refused like any other occupied member's.
- `PSR-V0-270`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2704) `pool hold
  renew --member ID --allocation SHA256 --lease-minutes N --request-id ID` (`POOL_HOLD_RENEW`)
  MUST set an unexpired hold's expiresAt to the receipt's recordedAt plus N minutes, keeping
  heldAt, and MUST refuse an expiry later than heldAt plus 1440 minutes (`MaxPoolHoldMinutes`)
  `LIMIT_EXCEEDED`, naming the latest allowed expiry. A hold whose expiresAt is at or before the
  recordedAt MUST be refused `FENCED`, naming release as its remedy: an expired hold is never
  revived. An allocation that names no entry is `FENCED`; an entry that is not HELD is
  `RESOURCE_COLLISION`.
- `PSR-V0-271`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2704) `pool hold
  release --member ID --allocation SHA256 --request-id ID` (`POOL_HOLD_RELEASE`) MUST end a hold,
  expired or not, by setting the entry QUARANTINED with reason `operator hold released; physical
  safe reuse unproved`, dropping `hold`, and stamping `quarantinedAt` (PSR-V0-261). It MUST pass an
  ALL barrier as every other release does. The member becomes FREE only through the existing
  `pool cleanup` and `pool confirm-safe`, or a configured `pool sweep`, because the operator's job
  may have changed it. A release of an entry that is not HELD is `RESOURCE_COLLISION`.
- `PSR-V0-272`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2704) `pool
  status` and `queue status` MUST show a HELD member's state and holder with
  `hold: {heldAt, expiresAt, expired, reason}`, where `expired` compares expiresAt with the reader's clock.
  An expired hold MUST stay HELD and occupied until it is released: no reader or writer converts it
  by time, and no member that is not HELD gains a `hold` key, so every other status byte is
  unchanged.

Non-goals: no automatic expiry, reaping or freeing of a hold; no hold of a member that is not
FREE, no hold without a named member (no automatic member choice), no transfer of a hold to an
attempt, no hold of a capacity slot, and no proof that the operator's job left the member clean.
If every member is held, dispatch behaves as it does when every member is otherwise occupied.
Failure modes: a lost response replays with the same allocation id; a crashed operator leaves a
HELD entry that `expired: true` makes visible and `pool hold release` ends; a renew after expiry
or past the bound is refused, never extended; a held member's worst-case pools.json entry carries
no attempt or command state and a reason within 224 bytes, so it stays within the PSR-V0-266
bound. Compatibility: an older binary refuses a pools.json with a HELD entry or a `hold` key as
`MALFORMED` (closed enum and unknown key); that is the recorded compatibility limit while any hold
is retained. Acceptance evidence: Traceability (PSR-V0-268..272) and
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-pool-operator-hold.md`. Rollback: release every hold and clean or
confirm the members, so no entry is HELD, then revert the commit. No receipt, request or evidence
is rewritten.

## Acceptance evidence

Initial fixture must initialize and canonically roundtrip policy/pool state, claim/release exact allocation and show QUARANTINED before fault injection. Positive reset+verify and optional cleanup-first produce bounded observation/log evidence, delegated confirmation and FREE readback; subsequent claim has fresh allocation. Exact replay after env deletion/current config change/successor allocation returns original evidence without execution. Barrier tests afterreset/beforeconfirm refuse competing sweep, ordinary cleanup and manual confirmation. Retry max2, combined deadline, total budget, exit/stdout/stderr predicates, overflow/source drift/private env syntax/permissions/symlinks/races, receipt publication failure and request conflict controls retain quarantine. Blocked identity/group probes are bounded. Actual foreground subprocess INT/TERM retirement observes PID/start identities and descendant absence; killed-host explicit recovery refuses live/unknown runner and retains quarantine. Large pool projection, descriptor/count maxima, duplicated evidence digest and archive evidence retention must be checked. PSR-V0-011/012 (issue628) add policy-validation tests for the 1..3600 bound and the pinned cwd shape, an injected-clock health timeout at 3600, and pinned health/sweep fixtures that run in the external worktree and refuse mismatched revision, dirty tracked or untracked input, alias, subdirectory and non-worktree paths without executing; hook-driven tests show a revision swapped between HEAD and status and a directory swapped after the proof run nothing, and post-exit bounded probes keep the command's terminal class. Current integrated tests/runtimes are NOT_RUN at this source phase; Linux and physical external cleanup remain NOT_OBSERVED. The earlier broad source run failed on historical replay timing and the clock fixture; the repaired five-cell run and focused vet passed at their original source binding. Required next evidence is a fresh integrated binary plus all 58 mandatory PASS observations (21 PSR/repair, 27 CAL dispatcher/CLI, nine issue503 and the clock witness), scoped race/vet and docs/specindex checks. Historical repair PASS is not a current broad PASS. The original enrollment (BASE211916, key a2284577) is retained as NON_SUCCESS. The successor enrollment at that seal, with the original single intent and both original selected checks unchanged, its CEM/OCM, report review, seal, hosted landing and native completion remain pending; actual receipts must replace pending claims only after execution.

Shared allocations (PSR-V0-016..021; V1-0946, GitHub #653): focused CLI, transaction, snapshot and store tests cover binding and exact replay, release in both orders with one quarantine, every refusal, lease expiry of the entry attempt and of a shared attempt, a crash at each publication of a share claim with redo or fresh retry, supervisor attach and lane-untouched refusal, the additive codec and its closed-decoder refusal, and the `pool status` and `attempt show` views as pure reads. An older binary reading a store with bindings is inferred from the closed decoders and the unknown-key refusal test; no older binary was executed. Dispatcher batching over shared allocations is not implemented.

Sweep prepare (PSR-V0-024..026; V1-1551, GitHub #1009): focused intent and store tests cover the policy shape, bound and safeReuse requirement; a prepare observed mid-run while the member is CLEANING under the owned sweep, chained after the passed verify, digest-only evidence with no output bytes, then FREE and an exact replay that does not rerun it; EXIT_NONZERO and shared-deadline TIMEOUT failures that stay QUARANTINED with the recorded reason and no retry; a lost prepare response whose replay stays pending without re-execution; and an ALL barrier that never launches prepare. Failure modes: a failed, timed-out, over-limit or source-changed prepare, a lost response and an ALL barrier each leave the member quarantined or owned, never free. Existing sweep tests run unchanged for members without prepare.
Capacity slots (PSR-V0-030..043; V1-1550, GitHub #1008): focused intent, snapshot, store and CLI tests cover the policy and pools.json codecs with every refusal; grant, wait and free after an attempt ends and after a crashed holder's lease expires (reaped by the next poll, acceptance 1); an exclusive request granted under continuing shared demand (acceptance 2); priority wait order; a reserved class granted past a higher-priority unreserved waiter (acceptance 3); request-id replay and no-change polls (acceptance 4); and the `slot` result and `queue status` `slots` view as a pure read (acceptance 5). Concurrency between two slot writers relies on the existing store lock and was not separately stress-tested. An independent review found five defects before landing; each has a regression test that fails on the reviewed head 19f66b4e4: supervisor attach with held or waiting slots is refused (PSR-V0-039); an exclusive granted after a capacity cut takes the new capacity and a stale shared waiter neither is granted nor blocks a later one (PSR-V0-040); two alternating higher-priority exclusive requesters cannot starve a reserved-class waiter (PSR-V0-041); no-change release and poll requests replay (PSR-V0-042); and a replay carries no slot (PSR-V0-043). An older binary reading a store with slots was inferred from the closed decoders at landing; PSR-V0-080 (V1-1572) later executed a `v1.0.0-rc.2` binary against such stores and observed the `MALFORMED` refusal.

Lease write resume (PSR-V0-050..057; V1-1560, GitHub #1010): the focused journal and store tests and the opt-in load harness named in that subsection and in Traceability (PSR-V0-050..057); quiet-host qualification NOT_RUN; Linux 45,000-receipt run in the 2026-10-23 build log.

Bounded acquire calls (PSR-V0-110..116; V1-1617, issue 1010 follow-up, issue 1063): the focused store and CLI tests and the opt-in load harness named in that subsection and in Traceability (PSR-V0-110..116); 47,000-receipt, quiet-host and Linux qualification NOT_RUN.

Bounded complete audit (PSR-V0-150..153; V1-1626, issue 1063): the focused store and CLI tests named in that subsection and in Traceability (PSR-V0-150..153); the 47,000-receipt harness runs before and after the change on a loaded 12-CPU host are in the build log; quiet-host and Linux qualification NOT_RUN.

Allocated changedSeq (PSR-V0-013; V1-0895): when a live attempt takes a PREPARING member to ALLOCATED, the member records that receipt's sequence as `changedSeq`, the last pool-state change that `pool status` reports. `TestPSRV0013_PreparedAllocationAdvancesChangedSeq` (`internal/tasks/transaction`) failed before the fix, with changedSeq 3 where 7 was expected. Through the CLI, `TestPSRV0013_PoolStatusAllocatedAfterPreparing` (`internal/tasks/cli`) claims a health-checked member, which takes a PREPARING receipt and then the ALLOCATED one, and `pool status` reports the second receipt's sequence; with the fix reverted it reports the PREPARING receipt (3 where 4 was expected). Receipts already written keep the sequence they recorded, and nothing is migrated. Rollback: revert the commit.

## Non-goals and simpler baseline

Baseline remains explicit pool cleanup and operator confirm-safe. No physical ownership discovery, authenticated operator, debt change, live lane reset, #494 writer repair, daemon/service, new native journal framework, broad redaction framework or dispatcher redesign. No sweep deadline or dispatcher budget increase for long cleanup (PSR-V0-011), no fetching, checkout or creation of a pinned worktree, no trust in its contents beyond the revision proof, and no atomic proof of the pinned checkout, no exclusion of concurrent mutation by actors outside Tasks during or after the proof, and no descriptor-bound (fchdir) execution (PSR-V0-012). Escaped/detached processes and external services remain outside process-group qualification; captured logs may contain secrets. Shared allocations (PSR-V0-016..021) add no dispatcher batching, no cross-holder, cross-pool or cross-stage sharing, no supervised or lane-untouched sharing, no transfer between allocations, no configurable share bound and no claim-next share selection; the simpler baseline is one allocation per attempt, which remains the default. Capacity slots (PSR-V0-030..043) add no daemon or timer, no push notification (a waiter polls with a fresh request id), no supervised-attempt or claim-time acquisition, no pool member, cleanup or quarantine, no per-holder quota, no re-ranking after enqueue (priority is the ticket priority at enqueue), no fairness among exclusive requests beyond wait order (a higher-priority exclusive can delay a lower one), no revocation of a grant made under a larger capacity, no automatic removal of a waiter the current policy can no longer grant, no replayed slot state, and no dispatcher integration; the simpler baseline remains a one-member pool per unit, which serializes and quarantines each use.

Prepare (PSR-V0-024..026) is not a health probe, does not run outside `pool sweep` or the dispatcher's sweep, has no separate deadline, retry budget or output retention, and does not prove the prepared data is correct beyond its exit status.

## Compatibility and rollback

Omitted safeReuse preserves old policy bytes/manual confirmation authority. Explicit new delegated operator profile amends CAL-V0-030 only for exact owned successful observation. Disable opt-in while preserving quarantine, pending runner ownership, receipts and private log evidence. No historical preimage rewrite, projection stripping or freeing by TTL. Older decoder rejection is recorded compatibility limit, including a pinned cwd object or a timeoutSeconds above 300. To roll back PSR-V0-011/012, free or clean every member whose occupied definition uses them (occupied definitions are immutable), then publish policy with `cwd:"REPOSITORY"` and timeoutSeconds within 1..300; source reversion of the decoder must follow that policy change. Reversion requires observed process retirement or explicit proved-orphan recovery; terminal shared spec/index/catalog updates record delivered evidence and remaining dispatcher/live/Linux limitations. To roll back PSR-V0-016..021, stop issuing `--share-allocation` and let every shared attempt end (or release them), which removes every `shared` list and leaves `sharedAllocation` only on terminal attempt records; then revert the source. A reverted or older binary still refuses a store whose retained attempt records carry `sharedAllocation`; that is a recorded compatibility limit of rollback once any share was admitted, and no historical receipt, record or preimage is rewritten to lift it. To roll back PSR-V0-030..043, release every slot entry (or let the attempts end and run one slot write so pools.json drops the `slots` key), then publish policy without `slots`; source reversion follows. An older binary refuses a policy with `slots` or a pools.json with a `slots` key as `MALFORMED` (unknown key); that is a recorded compatibility limit while slots are declared or held, observed with a `v1.0.0-rc.2` binary (PSR-V0-080). No receipt, record or preimage is rewritten.

Prepare is additive: omitting it keeps every existing byte. Older decoders refuse a policy, owner or observation carrying `prepare`, which is the recorded compatibility boundary. Rollback removes `prepare` from the member configuration only after the member holds no allocation: a policy update that changes the definition of any occupied member (including a QUARANTINED one retained after a failed prepare) is refused with `occupied pool definition cannot change`. A member quarantined by a failed prepare must therefore first be freed under the unchanged policy, either by a further `pool sweep` (which runs `prepare` again) or by the ordinary `pool cleanup` followed by `pool confirm-safe`, neither of which runs `prepare`; only then publish the policy without `prepare`. No receipt or evidence is rewritten. No other supported way to free a quarantined allocation exists.

## Traceability (PSR-V0-016..026)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-016 | `TestPSRV0016_ShareBindsAndReads`, `TestPSRV0016_ShareCrashReplay`, `TestPSRV0016_LoadPoolsValidatesSharedAttempts` |
| PSR-V0-017 | `TestPSRV0017_ShareRefusals` |
| PSR-V0-018 | `TestPSRV0018_ShareReleaseOrders`, `TestPSRV0018_ShareLeaseExpiry`, `TestPSRV0018_SharedPostHandOver` |
| PSR-V0-019 | `TestPSRV0019_SharedAttachRefused`, `TestPSRV0019_ShareExcludesLaneUntouched` |
| PSR-V0-020 | `TestPSRV0016_ShareBindsAndReads`, `TestPSRV0018_ShareReleaseOrders` |
| PSR-V0-021 | `TestPSRV0021_SharedCodec` |
| PSR-V0-022 | `TestPSRV0022_UnprovedProbeCleanupRecordsUnknown`, `TestPSRSweepSourceBoundClass` |
| PSR-V0-023 | `TestPSRV0023SweepBoundsScaleWithBudget`, `TestPSRV0023ScaledProbeBoundEnforced`, `TestPSRForegroundSignals`, `TestPSRDispatchNativeAdapter` |
| PSR-V0-024 | `TestPSRV0024PrepareConfig`, `TestPSRV0025PrepareRunsOwnedBeforeConfirm` |
| PSR-V0-025 | `TestPSRV0025PrepareRunsOwnedBeforeConfirm`, `TestPSRV0025PrepareNeverLaunchesUnderAll`, `TestPSRV0026PrepareFailureStaysQuarantined` |
| PSR-V0-026 | `TestPSRV0026PrepareFailureStaysQuarantined`, `TestPSRV0026PendingReplayNeverRerunsPrepare` |

## Traceability (PSR-V0-030..043)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-030 | `TestPSRV0030_PolicyDeclaresSlots` |
| PSR-V0-031 | `TestPSRV0031_SlotEntriesCodec` |
| PSR-V0-032 | `TestPSRV0032_SlotsGrantWaitAndFreeWithoutOperator` |
| PSR-V0-033 | `TestPSRV0033_ExclusiveEventuallyGrantedUnderSharedLoad`, `TestPSRV0041_ExclusiveDemandCannotStarveReservedClass` |
| PSR-V0-034 | `TestPSRV0034_WaitersGrantedByPriorityThenRequestOrder` |
| PSR-V0-035 | `TestPSRV0035_ReservedClassNeverStarved`, `TestPSRV0038_CLISlotResultsAndQueueStatus` |
| PSR-V0-036 | `TestPSRV0036_ReleaseAndRefusals`, `TestPSRV0038_CLISlotResultsAndQueueStatus` (`--lock-wait`) |
| PSR-V0-037 | `TestPSRV0037_AcquireIdempotentByRequestID` |
| PSR-V0-038 | `TestPSRV0038_CLISlotResultsAndQueueStatus` |
| PSR-V0-039 | `TestPSRV0039_SupervisorAttachRefusedWithSlots` |
| PSR-V0-040 | `TestPSRV0040_GrantsFollowCurrentPolicy` |
| PSR-V0-041 | `TestPSRV0041_ExclusiveDemandCannotStarveReservedClass` |
| PSR-V0-042 | `TestPSRV0042_NoChangeRequestsReplay`, `TestPSRV0036_ReleaseAndRefusals` |
| PSR-V0-043 | `TestPSRV0043_ReplayCarriesNoSlot`, `TestPSRV0037_AcquireIdempotentByRequestID`, `TestPSRV0038_CLISlotResultsAndQueueStatus` |

## Traceability (PSR-V0-050..057)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-050 | `TestPSRV0050_PrefixFallback` (`internal/tasks/journal`); `TestPSRV0052_PoolAcquireResumesUnderWriter` (`internal/tasks/store`) |
| PSR-V0-051 | `TestPSRV0051_ResumeBaseOnlyFromSettledAudit` (`internal/tasks/journal`) |
| PSR-V0-052 | `TestPSRV0052_ResumeDeclinesOnChangedBase` (`internal/tasks/journal`); `TestPSRV0052_PoolAcquireResumesUnderWriter`, `TestPSRV0052_WidenResumesUnderWriter` (`internal/tasks/store`) |
| PSR-V0-053 | `TestPSRV0053_ResumeKeepsRewrittenPrefixDetection`, `TestPSRV0054_OvertakenGuardKeepsRewrittenPrefixDetection` (`internal/tasks/store`) |
| PSR-V0-054 | `TestPSRV0054_OvertakenGuardStillResumes`, `TestPSRV0054_OvertakenGuardKeepsRewrittenPrefixDetection` (`internal/tasks/store`) |
| PSR-V0-055 | opt-in `TestPSRV0055_PoolAcquireLoad` (`internal/tasks/store`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1560-pool-acquire-starvation.md` |
| PSR-V0-056 | `TestPSRV0056_RecheckResumeSeesChangeAfterAudit` (`internal/tasks/journal`); `TestPSRV0056_ResumeRechecksBaseBeforeCommit` (`internal/tasks/store`) |
| PSR-V0-057 | `TestPSRV0057_ResumeCheckCloseFailureIsCleanup`, `TestPSRV0056_RecheckResumeSeesChangeAfterAudit` (`internal/tasks/journal`) |

## Traceability (PSR-V0-080..)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-080 | opt-in `TestPSRV0080_OlderBinaryRefusesSlots` (`internal/tasks/cli`), executed against a `v1.0.0-rc.2` source build |
| PSR-V0-081 | `TestPSRV0081_SourceIdentityGuard`, `TestPSRV0086_PoolsWithoutSourcesUnchanged` (`internal/tasks/cli`) |
| PSR-V0-082 | `TestPSRV0082_IdentityPolicyRules` (`internal/tasks/cli`) |
| PSR-V0-083 | `TestPSRV0081_SourceIdentityGuard`, `TestPSRV0085_LaneUntouchedKeepsIdentityGuard` (`internal/tasks/cli`) |
| PSR-V0-084 | `TestPSRV0081_SourceIdentityGuard` (`internal/tasks/cli`) |
| PSR-V0-085 | `TestPSRV0081_SourceIdentityGuard`, `TestPSRV0085_LaneUntouchedKeepsIdentityGuard` (`internal/tasks/cli`) |
| PSR-V0-086 | `TestPSRV0086_PoolsWithoutSourcesUnchanged` (`internal/tasks/cli`) |
| PSR-V0-168 | `TestPSRV0168_IdentityProbeOutsideWriterLock`, `TestPSRV0168_IdentityProbeStaleStateNotReused`, `TestV12138_ClaimDerivationOutsideWriterLock`, `TestV12138_UnmovedClaimServedOnWriterRoute`, `TestV12138_ClaimRebindsMovedBaseAfterJournalAdvance`, `TestV12138_LockedClaimObserverRunsNoGit` (`internal/tasks/store`) |
| PSR-V0-224 | `TestPSRV0224_HeadFilesResolveAsGit`, `TestPSRV0224_ClaimRebindsMovedHeadWithoutJournalAdvance`, `TestPSRV0224_HeadRefusesChainMovedBetweenReads`, `TestPSRV0224_CachedLayoutRefusesReplacedWorktree`, `TestV12138_ClaimDerivationOutsideWriterLock`, `TestV12138_ClaimRebindsMovedBaseAfterJournalAdvance` (`internal/tasks/store`); opt-in `TestCALV0263_LeaseGateLoad` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-claim-head-ref-binding.md`) |
| PSR-V0-225 | `TestPSRV0225_PrecheckMovesBaseReadsOutOfTheGate` (no request shard listed and no settled segment re-hashed under the gate after the precheck; every shard listed without it), `TestPSRV0225_PrecheckNeverAdmitsAStaleBase` (stray, removed or rewritten request, a stray with the shard's mtime restored, a new shard and a rewritten segment, after the precheck or after the resumed audit, decline; `requests/` replaced by a copy with a stray during the recheck declines; a segment rewritten before the precheck is not trusted; a failed precheck close is cleanup), `TestJSGV0011_ResumeDeclinesRewrittenSegment` (store, sealed: a segment rewritten at `fast.resume.precheck`, after the precheck re-hashed it, declines; nothing commits and the complete route's decision stands); B2 run in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-resume-precheck-outside-gate.md` (UNQUALIFIED macOS) |
| PSR-V0-169 | `TestPSRV0169_NonPublishingRefreshDoesNotRecurPerWrite`, `TestCALV0117_WriterAdvanceAndScheduledRefresh` (`internal/tasks/store`) |
| PSR-V0-190 | `TestPSRV0190_CheckpointTailRefusesPrefixRequestDuplicate`, `TestCALV0061_CheckpointTailEqualsFullAudit` (`internal/tasks/journal`) |
| PSR-V0-191 | `TestPSRV0191_StaleProjectionReadWarns` (`internal/tasks/cli`) |

## Traceability (PSR-V0-110..116)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-110 | `TestPSRV0113_ResumeDecidesNonHeartbeatTail`, `TestPSRV0113_ForgedHeartbeatTailNeverCommits` (`internal/tasks/store`) |
| PSR-V0-111 | opt-in `TestPSRV0055_PoolAcquireLoad` with `CORVINT_TASKS_POOL_ACQUIRE_LOAD_CALL_MODE=detach` (`internal/tasks/store`); `TestPSRV0111_CeilingRelaunchesOnSameRequestID` (`internal/tasks/cli`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1617-acquire-bounded-calls.md` |
| PSR-V0-112 | `TestPSRV0112_LeaseProgressAfterEachRound` (`internal/tasks/store`); `TestPSRV0112_PendingReportsProgress` (`internal/tasks/cli`) |
| PSR-V0-113 | `TestPSRV0113_ResumeDecidesNonHeartbeatTail`, `TestPSRV0113_ForgedHeartbeatTailNeverCommits` (`internal/tasks/store`) |
| PSR-V0-114 | `TestPSRV0114_CompactHelpNotesWalkCost` (`internal/tasks/cli`) |
| PSR-V0-115 | `TestPSRV0115_ResumeAfterCompaction` (`internal/tasks/store`) |
| PSR-V0-116 | `TestPSRV0116_TailRewrittenAfterAuditNeverCommits` (`internal/tasks/store`); `TestPSRV0056_RecheckResumeSeesChangeAfterAudit` (`internal/tasks/journal`) |

## Traceability (PSR-V0-130..143)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-130 | `TestPSRV0130_PoolWaitRequestShape`, `TestPSRV0131_WaitRegistersRefreshesAndBounds` (`internal/tasks/transaction`); `TestPSRV0135_PoolWaitCLI` (`internal/tasks/cli`) |
| PSR-V0-131 | `TestPSRV0131_PoolWaitPolicyBounds` (`internal/tasks/intent`); `TestPSRV0131_WaitersCodec` (`internal/tasks/snapshot`); `TestPSRV0131_WaitRegistersRefreshesAndBounds` (`internal/tasks/transaction`) |
| PSR-V0-132 | `TestPSRV0132_FreedMemberGoesToHeadWaiter`, `TestPSRV0132_WaitlistOutranksPriorityAdmission`, `TestPSRV0132_ClaimNextSelectsTheWaiter` (`internal/tasks/transaction`); `TestPSRV0132_ClaimNextPoolClaimsTheWaiterCLI` (`internal/tasks/cli`) |
| PSR-V0-133 | `TestPSRV0133_DeadWaitersNeverBlock`, `TestPSRV0133_EveryPoolsWriteDropsDeadWaiters` (`internal/tasks/transaction`) |
| PSR-V0-134 | `TestPSRV0134_WaitingAttemptAcquiresFreedMember` (`internal/tasks/transaction`); `TestPSRV0135_PoolWaitCLI` (`internal/tasks/cli`) |
| PSR-V0-135 | `TestPSRV0135_WaitlistView`, `TestPSRV0135_WaiterValueCarriesEveryField` (`internal/tasks/transaction`); `TestPSRV0135_PoolWaitCLI` (`internal/tasks/cli`) |
| PSR-V0-136 | `TestPSRV0136_HandoffGraceReservesForSuccessor`, `TestPSRV0136_FullWaitlistRefusesHandoffGrace` (`internal/tasks/transaction`) |
| PSR-V0-140 | `TestPSRV0140_OneFreeMemberLaunchesOne` (`internal/tasks/dispatch`); `TestPSRV0135_PoolWaitCLI` (`internal/tasks/cli`) |
| PSR-V0-141 | `TestPSRV0141_PendingWorkerCountsAgainstRoom`, `TestPSRV0143_PoolWaitEventsReportRoom` (`internal/tasks/dispatch`) |
| PSR-V0-142 | `TestPSRV0142_StaleWorkerIgnored` (`internal/tasks/dispatch`) |
| PSR-V0-143 | `TestPSRV0143_PoolWaitEventsReportRoom` (`internal/tasks/dispatch`) |

## Traceability (PSR-V0-150..153)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-150 | `TestPSRV0150_DeclinedResumeWalksOncePerRound` (`internal/tasks/store`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1626-bounded-complete-audit.md` |
| PSR-V0-151 | `TestPSRV0151_TimingSplitsResumedAcquire`, `TestPSRV0150_DeclinedResumeWalksOncePerRound` (`internal/tasks/store`) |
| PSR-V0-152 | `TestPSRV0112_PendingReportsProgress` (`internal/tasks/cli`) |
| PSR-V0-153 | `TestPSRV0153_BoundedAuditKeepsRewrittenPrefixDetection` (`internal/tasks/store`) |

## Traceability (PSR-V0-160..167)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-160 | `TestPSRV0160_UndeclaredOperationRefused` (`internal/tasks/store`); `TestPSRV0162_PoolOperateCLIAndStatus` (`internal/tasks/cli`) |
| PSR-V0-161 | `TestPSRV0161_IllegalStateRefusedNothingRuns`, `TestPSRV0163_PassedResetStaysQuarantined` (`internal/tasks/store`) |
| PSR-V0-162 | `TestPSRV0162_ReceiptRecordedMemberFree` (`internal/tasks/store`); `TestPSRV0162_OperationCodec` (`internal/tasks/snapshot`); `TestPSRV0162_PoolOperateCLIAndStatus` (`internal/tasks/cli`) |
| PSR-V0-163 | `TestPSRV0162_ReceiptRecordedMemberFree`, `TestPSRV0163_PassedResetStaysQuarantined`, `TestPSRV0163_GuardedPassedOperationNeverFree`, `TestPSRV0165_FailedResetRetargetQuarantine` (`internal/tasks/store`) |
| PSR-V0-164 | `TestPSRV0164_SameRequestReplaysNeverRunsTwice`, `TestPSRV0164_ReplayAfterSupersedingRun` (`internal/tasks/store`); `TestPSRV0162_PoolOperateCLIAndStatus` (`internal/tasks/cli`) |
| PSR-V0-165 | `TestPSRV0165_FailedResetRetargetQuarantine`, `TestPSRV0165_PreExecutionFailureSettled` (`internal/tasks/store`); `TestPSRV0165_RecoverOrphanedOperation` (`internal/tasks/transaction`) |
| PSR-V0-166 | `TestPSRV0166_TimeoutKillsGroupOutputBounded` (`internal/tasks/store`) |
| PSR-V0-167 | `TestPSRV0162_OperationCodec` (`internal/tasks/snapshot`); `TestPSRV0162_PoolOperateCLIAndStatus` (`internal/tasks/cli`) |
| PSR-V0-196 | `TestPSRV0196_FullSizePoolReceiptsStayBounded` (`internal/tasks/transaction`) |
| PSR-V0-202 | `TestPSRV0202_WorstCasePoolsStayBoundedAtReasonCap`, `TestPSRV0202_OverlongFreshRecoverRefusedTyped`, `TestPSRV0202_RecordedLongRecoverReasonReplays` (`internal/tasks/transaction`) |

## Traceability (PSR-V0-185)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-185 | `TestPSRV0185_CompactHelpNamesFastRoute` (`internal/tasks/cli`) |

## Traceability (PSR-V0-201)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-201 | `TestPSRV0201_RunnerIdentityNeedsNoListing`, `TestPSRBoundedProbes` (`internal/tasks/store`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-22-perf-1022s-store-test-parallelism.md` |

## Traceability (PSR-V0-203..204)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-203 | `TestPSRV0203_CheckpointTailRefusesNullPreStateDuplicate`, `TestPSRV0190_CheckpointTailRefusesPrefixRequestDuplicate`, `TestCALV0321_CheckpointRequestFilter` (`internal/tasks/journal`) |
| PSR-V0-204 | `TestPSRV0204_LinuxRunnerIdentityNeedsNoListing`, `TestPSRV0204_LinuxIdentityClockStepIsUnknown`, `BenchmarkPSRV0204_RunnerIdentity` (`internal/tasks/store`, Linux only); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-tasks-lane-1023.md` |

## Traceability (PSR-V0-220..223)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-220 | `TestCALV0350_ClaimNextDerivesScopeForThePreviewedWaiterCLI`, `TestPSRV0132_ClaimNextPoolClaimsTheWaiterCLI` (`internal/tasks/cli`); implementation `internal/tasks/cli/lease.go` (`Derive: env.ScopeDeriver`), `internal/tasks/transaction/claim_next.go` |
| PSR-V0-221 | `TestCALV0364_ChangeGuardRefusesADirectoryBeyondTheInotifyLimit`, opt-in `TestCALV0364_ChangeGuardUnderTheRealInotifyLimit` (`CORVINT_TASKS_INOTIFY_LIMIT=1`, root; lowers the real sysctl to this user's measured headroom plus exactly the guard's directory watches and requires registration and a reported write) (`internal/tasks/authority`); its file ceiling is superseded by PSR-V0-222 |
| PSR-V0-222 | `TestPSRV0222_ChangeGuardWatchCountIsBoundedByDirectories` (`/proc/self/fdinfo` count), `TestPSRV0222_ChangeGuardSeesFileMutationsThroughItsDirectory` (every mutation class with no file watch, via `CheckEvents`), `TestPSRV0222_ChangeGuardTracksAMultiplyLinkedFileByStat`, `TestPSRV0222_AncestorAddKeepsTheContentMask` (`internal/tasks/authority`, Linux only); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-linux-change-guard-directory-watches.md` |
| PSR-V0-223 | `TestPSRV0223_ChangeGuardSeesAWriteThroughAnAliasLink`, `TestPSRV0223_ChangeGuardSeesASecondLinkWriteAfterTheSweep`, `TestPSRV0222_ChangeGuardWatchCountIsBoundedByDirectories` (every file stat-tracked, none uncovered, watch count unchanged), `TestPSRV0222_ChangeGuardTracksAMultiplyLinkedFileByStat` (`internal/tasks/authority`, Linux only); opt-in `TestPSRV0223_ChangeGuardSweepCost` (`internal/tasks/store`, Linux only); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-linux-change-guard-directory-watches.md` |
| PSR-V0-274 | `TestPSRV0274_ChangeGuardIgnoresUnrelatedEntriesOfAnAncestor` (`internal/tasks/authority`, Linux only: a mode change on a sibling file and directory and a mode and a time change on a direct entry of the system temporary directory are admitted; a mode change on the entry on the path and a mode change and a rename of the ancestor itself are reported; fails before the change on every admitted case), `TestPSRV0274_ChangeGuardReportsAnEventBehindAnIgnoredBacklog` (an on-path mode change queued behind 9,000 ignored sibling events is reported), `TestCALV0026_ChangeGuard` (every mutation class still reported); `TestCALV0070_MutateRetriesAuditWithoutWatch`, `TestCALV0070_PinnedInventoryMutateParity`, `TestJSGV0015_LeaseCommitStagesCheckpointsOutsideWriterLock` (`internal/tasks/store`) under a concurrent `/tmp` entry mode-change storm; `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-26-linux-change-guard-ancestor-entries.md` |
| PSR-V0-226 | `TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit` (`internal/tasks/store`, Darwin and Linux; on Linux tmpfs it requires the first mapped write unseen and logs the filesystem, elsewhere `SNAPSHOT_MOVED`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-wsl2-tmpfs-mapped-write.md` |
| PSR-V0-227 | `TestPSRV0227_CompareIdentity`, `TestPSRV0227_OwnedGroupAnchorToleratesJitter`, `TestPSRV0227_EscapeAnchorToleratesJitter`, `TestPSRV0227_BtimeSampleWithinTolerance` (Linux) (`internal/tasks/supervisor`); `TestPSRV0227_RecordedOwnerAndRemoverTolerateJitter` (`internal/tasks/store`); `TestPSRV0227_WorkerTreeToleratesJitter`, `TestPSRV0227_RecordedStatesTolerateJitter`, `TestPSRV0227_ReaderLeaderToleratesJitter` (`internal/tasks/dispatch`); `TestPSRV0227_PulseIdentityToleratesJitter` (`internal/tasks/service`); `TestPSRV0227_RecordedRunAndChildTolerateJitter` (`internal/tasks/cli`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-v1-1091-cross-process-identity.md` |

## Traceability (PSR-V0-260..267, PSR-V0-273)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-260 | `TestPSRV0260_PoolRecoveryDetailBoundedAndRedacted`, `TestPSRV0260_RedactLinesSpansAndScale` (`internal/tasks/transaction`); `TestPSRV0260_PoolObserveStallsQuarantinedMember`, `TestPSRV0260_PoolObserveFlagsAndState` (`internal/tasks/cli`) |
| PSR-V0-261 | `TestPSRV0261_StampQuarantineTimestamp` (`internal/tasks/transaction`); `TestPSRV0261_QuarantineRecoveryCodec` (`internal/tasks/snapshot`); `TestPSRV0013_PoolStatusMembers` (`internal/tasks/cli`) |
| PSR-V0-262 | `TestPSRV0262_PoolReportCountsConsecutiveIdenticalFailures`, `TestPSRV0262_SweepCleanupSupersedesRecovery` (`internal/tasks/transaction`); `TestPSRV0262_RecoveryStallRule` (`internal/tasks/snapshot`) |
| PSR-V0-263 | `TestPSRV0263_PoolSummaryWhenEveryMemberStalled`, `TestPSRV0260_PoolObserveStallsQuarantinedMember` (`internal/tasks/cli`); `TestPSRV0262_RecoveryStallRule` (`internal/tasks/snapshot`) |
| PSR-V0-264 | `TestPSRV0264_StalledPoolRefusesTyped` (`internal/tasks/transaction`); `TestPSRV0263_PoolSummaryWhenEveryMemberStalled`, `TestPSRV0260_PoolObserveStallsQuarantinedMember` (`internal/tasks/cli`) |
| PSR-V0-265 | `TestPSRV0265_StalledPoolHoldsLaunches` (`internal/tasks/dispatch`); `TestPSRV0265_DispatchHoldsStalledPoolLaunchesCLI` (`internal/tasks/cli`) |
| PSR-V0-266 | `TestPSRV0202_WorstCasePoolsStayBoundedAtReasonCap` (`internal/tasks/transaction`); `TestPSRV0261_QuarantineRecoveryCodec` (`internal/tasks/snapshot`) |
| PSR-V0-267 | `TestPSRV0267_PlanStalledDeferralReachesPoolRoom` (`internal/tasks/dispatch`); `TestCALV0097_CoreDecodesResourceDeferredPlan` (`internal/taskman`); `TestPSRV0267_PlanSelectedRoleReportsStalledPoolCLI` (`internal/tasks/cli`) |
| PSR-V0-273 | `TestPSRV0273_PoolObserveDetailFileUnderSymlinkedAncestor` (`internal/tasks/cli`); the five V1-2711 PSR tests in `internal/tasks/cli` pass on darwin with the default `TMPDIR` |

## Traceability (PSR-V0-268..272)

| Requirement | Evidence |
| --- | --- |
| PSR-V0-268 | `TestPSRV0268_HoldOccupiesTheMemberAndClaimsSkipIt`, `TestPSRV0268_HoldRefusals` (`internal/tasks/transaction`); `TestPSRV0268_HoldCodec` (`internal/tasks/snapshot`); `TestPSRV0268_OperatorHoldLifecycle` (`internal/tasks/cli`) |
| PSR-V0-269 | `TestPSRV0268_HoldOccupiesTheMemberAndClaimsSkipIt`, `TestPSRV0269_ExpiredHoldStaysOccupied` (`internal/tasks/transaction`); `TestPSRV0268_OperatorHoldLifecycle` (`internal/tasks/cli`) |
| PSR-V0-270 | `TestPSRV0270_RenewIsBounded` (`internal/tasks/transaction`); `TestPSRV0268_OperatorHoldLifecycle` (`internal/tasks/cli`) |
| PSR-V0-271 | `TestPSRV0271_ReleaseQuarantines` (`internal/tasks/transaction`); `TestPSRV0268_OperatorHoldLifecycle` (`internal/tasks/cli`) |
| PSR-V0-272 | `TestPSRV0268_OperatorHoldLifecycle` (`internal/tasks/cli`); `TestPSRV0269_ExpiredHoldStaysOccupied`; `TestPSRV0270_RenewIsBounded` (`internal/tasks/transaction`, PoolHoldExpired) |
