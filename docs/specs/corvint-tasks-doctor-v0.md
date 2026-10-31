# Corvint Tasks doctor V0

Owner: Russell Lewis
Date: 2026-10-09
Intent status: accepted (decisions 0481 and 0519; GitHub #715 and #791; TQD-V0-001..013); TQD-V0-020..033 and the TQD-V0-001 `--config`/`dispatchers` amendment accepted under the owner's standing auto-accept of lane requirements (2026-10-11; V1-1598, V1-1601, V1-1604, V1-1192, V1-1193, V1-1519; GitHub #1041, #1044, #1047); TQD-V0-040..042 (doctor findings honour the adopter-declared progress signal) accepted under the same standing auto-accept (V1-1608; GitHub #1050); TQD-V0-043..044 (RUNNABLE_HELD_IDLE and ROLE_QUEUE_DEEP from the dispatcher's queue observation) accepted under the same standing auto-accept (V1-1914, V1-1916; GitHub #1181, #1183); TQD-V0-045 (elided-record walks share receipt reads) accepted under the same standing auto-accept (V1-1193; GitHub #791); TQD-V0-046 (the padded-journal fixture builder, re-homed from CAL-V0-360) accepted under the same standing auto-accept (V1-2137)
Delivery status: experimental
Authoritative inputs: owner request [issue 715](https://github.com/beamfall/corvint/issues/715);
the pure-read rule TM-V0-008 and the shared journal audit CAL-V0-061; the attempt and pool
afterimage profiles in `internal/tasks/snapshot`; the detached run records of
[the attempt runner](corvint-tasks-attempt-runner-v0.md) (ATR-V0-010..012); the no-progress
semantics of CAL-V0-102; the obligation counts of
[the obligation ledger](corvint-tasks-obligation-ledger-v0.md). This technical contract states
concrete semantics; the issue intent does not accept generated implementation detail. The owner
accepted TQD-V0-001..012 as written in decision 0481 and TQD-V0-013 in decision 0519. The
operational, notify, host-session, work-state and scan-cost requirements TQD-V0-020..033 are
recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-V1-1598-doctor-findings.md`.

## Agent digest
- Claim: `corvint-tasks doctor` names stuck-queue patterns from journal, pool and attempt data plus bounded plugins; `doctor --line` prints a cached status line.
- Status: accepted (decisions 0481 and 0519; GitHub #715 and #791; TQD-V0-001..013); TQD-V0-020..033 and the TQD-V0-001 `--config`/`dispatchers` amendment accepted under the owner's standing auto-accept of lane requirements (2026-10-11; V1-1598, V1-1601, V1-1604, V1-1192, V1-1193, V1-1519; GitHub #1041, #1044, #1047); TQD-V0-040..042 (doctor findings honour the adopter-declared progress signal) accepted under the same standing auto-accept (V1-1608; GitHub #1050); TQD-V0-043..044 (RUNNABLE_HELD_IDLE and ROLE_QUEUE_DEEP from the dispatcher's queue observation) accepted under the same standing auto-accept (V1-1914, V1-1916; GitHub #1181, #1183); TQD-V0-045 (elided-record walks share receipt reads) accepted under the same standing auto-accept (V1-1193; GitHub #791); TQD-V0-046 (the padded-journal fixture builder, re-homed from CAL-V0-360) accepted under the same standing auto-accept (V1-2137); TQD-V0-013 (COMPLETED, ARCHIVED and HELD tickets are not NO_PROGRESS_HANDOFF) from V1-1176; experimental source implementation with focused CLI tests.
- Exists: built-in PARKED_WITH_OPEN_ITEMS (configured dispatchers' parked tickets with open obligation items, TOL-V0-125), NO_PROGRESS_HANDOFF, REPEAT_REFUSAL, SLOW_LANE_RECOVERY, SETUP_ONLY_PROOF and FALSE_IDLE (detached-run supervisors only) detectors; RUNNABLE_HELD_IDLE and ROLE_QUEUE_DEEP from configured dispatchers' queue observations; QUEUE_LOCK_HELD, SLOW_QUEUE_COMMAND, NO_COMPLETIONS and FD_PRESSURE; HOST_RETRY_STORM, HOST_ERRORS and EARLY_REFUSAL from configured dispatchers' worker logs; dispatcher work-state health with WORK_STATE_FAILING; a `--config` file of thresholds; a notify hook on `--refresh` with NOTIFY_FAILED; bounded plugins with PLUGIN_FAILED; `--refresh` atomic cache; `--line` lock-free cache read.
- Blocked on: PREFLIGHT_FAILURE waits for #713's recorded preflight evidence; the Linux process walk and `/proc` lock probes are unqualified (parser tests only); darwin names a lock holder only approximately (TQD-V0-021).
- Read next: Requirements; Detectors; Failure modes; Traceability.

## Intent and boundary

An operator running several agent lanes needs one read that names the queue's stuck patterns
before they waste a day: a ticket handed off again and again with nothing to show, a candidate
the reviewer keeps returning, a pool member whose cleanup keeps failing, a completion whose
proof witnessed only setup obligations, and a session whose heartbeat went stale while its
process tree is still busy. The simpler baseline is reading `queue status`, `pool status`,
`attempt show` and the receipts by hand; it names none of these patterns.

The doctor is advisory. It reads the store through the ordinary TM-V0-008 protocol, never takes
the writer lock and never writes the journal, the intent tree, the state directory or the run
directories. Its findings carry no authority: they never change ranking, claims, holds or
evidence. Its only write is the summary cache, made by the explicit `doctor --refresh`.

## Requirements

- `TQD-V0-001`: (accepted, decision 0481; GitHub #715; `--config` and `dispatchers` amendment accepted 2026-10-17, V1-1519, standing lane auto-accept) `corvint-tasks doctor [--refresh] [--plugins DIR] [--config FILE]` is a pure read under TM-V0-008: it takes no lock and writes nothing in the journal, the state directory, the intent tree or any attempt run directory. Without `--refresh` it writes no file at all. It emits one `taskman-command-result/0` envelope whose single item has profile `taskman-doctor/0`, `queueId`, `observedAt`, `summary`, `findings`, `scan` and `mutationAuthority:false`, plus `dispatchers` when the `--config` file names any (TQD-V0-031). A store whose tracked intent is valid but whose local journal is absent refuses MISSING_EVIDENCE, not UNINITIALIZED, for plain `doctor` and `--refresh` alike, and writes nothing. Any other argument refuses VALIDATION_FAILED `MALFORMED` before the store is read.
- `TQD-V0-002`: (accepted, decision 0481; GitHub #715) `summary` is `{lanes:{free,total}, runningSessions, completions24h, alerts}`. `lanes.total` counts the policy's pool members and `lanes.free` those absent from audited pools.json; `runningSessions` counts live (non-terminal) attempts; `completions24h` is the `queue status` last-24-hours completion count at `observedAt`; `alerts` is the number of findings.
- `TQD-V0-003`: (accepted, decision 0481; GitHub #715) Each finding is `{kind, source, who, detail, remedy, firstSeen, ageSeconds, evidenceSeqs}`. `kind` is a stable upper-case code; `source` is `builtin` or `plugin:<name>`; `who` names the ticket, pool member, attempt or plugin; `remedy` is fixed operator guidance per built-in kind. Built-in journal findings take `firstSeen` from the recordedAt of their earliest evidence receipt and list those receipt sequence numbers; FALSE_IDLE and plugin findings carry `firstSeen` from the cache entry with the same kind, source and who, else `observedAt`. `ageSeconds` is `observedAt - firstSeen`, never negative. A finding not re-detected is dropped (ages out). Findings sort by kind, source and who; at most 512 are emitted, the cache written by `--refresh` keeps only the leading findings in that order whose encoded file fits 1 MiB (TQD-V0-011), and `scan.findingsTruncated` reports either cut; `summary.alerts` still counts every finding.
- `TQD-V0-004`: (accepted, decision 0481; GitHub #715) Journal-derived detectors read one bounded backward scan of receipts from the snapshot head: at most 4096 receipts and none recorded more than 7 days before `observedAt`. Each attempts/*.json and pools.json post is taken from the inline record or its `evidence/<sha256>` blob and used only when its bytes hash to the post digest. A missing, undecodable or digest-mismatched receipt stops the scan; `scan` reports `receipts`, `fromSeq`, `headSeq`, `windowSeconds` and `truncated` (and, since TQD-V0-820, `journalAudit`), so a partial scan is visible rather than read as complete.
- `TQD-V0-005`: (accepted, decision 0481; GitHub #715) NO_PROGRESS_HANDOFF names a ticket whose newest three or more ended generations in the scan are each a HANDOFF with no gate result, no review and a candidate tree that is absent or equal to the last tree recorded before it (the CAL-V0-102 no-progress test), independent of any `loopDetection` policy.
- `TQD-V0-006`: (accepted, decision 0481; GitHub #715) REPEAT_REFUSAL names a ticket and candidate tree refused three or more times in the scan. A refusal is a generation ending REVIEW_RETURNED (its own candidate tree, else the ticket's last recorded tree) or a distinct FAILED gate result recorded for that tree. A gate result counts once, at the receipt whose attempt afterimage first adds its digest; the gate results an attempt already carries when the scan first meets it, unless that receipt created the attempt, were recorded before the window and neither count nor set `firstSeen`. Refused lease requests leave no receipt and are not counted.
- `TQD-V0-007`: (accepted, decision 0481; GitHub #715) SLOW_LANE_RECOVERY names a pool member whose allocation entered CLEANING (a cleanup or sweep try) three or more times in the scan without being freed, or was freed only after three or more tries. A refused `pool confirm-safe` leaves no receipt, so the count is of recorded cleanup and sweep tries.
- `TQD-V0-008`: (accepted, decision 0481; GitHub #715) SETUP_ONLY_PROOF names a ticket COMPLETED within the window whose obligation ledger counts at least one core obligation and witnessed none of them, from the current audited record.
- `TQD-V0-009`: (accepted, decision 0481; GitHub #715) FALSE_IDLE names a live attempt whose holder status is STALE_HOLDER or LEASE_EXPIRED while one of its detached runs for the current generation is STARTING or RUNNING, its supervisor matches the recorded process identity, and a bounded local process-table walk (`/bin/ps -axo pid=,ppid=,stat=`, 5 s, 4 MiB, 4096 processes) finds a non-zombie descendant of the supervisor. A run directory that is not a directory (for example a FIFO) is skipped without blocking. A walk that cannot complete reports nothing for that attempt and adds a `scan` warning. The PREFLIGHT_FAILURE kind is reserved for #713's recorded preflight evidence and is not emitted by this delivery.
- `TQD-V0-010`: (accepted, decision 0481; GitHub #715) Plugins run only when `--plugins DIR` names a directory. Discovery opens it with `O_DIRECTORY|O_NONBLOCK` on darwin and linux, so a FIFO or other non-directory swapped in after the argv check yields one PLUGIN_FAILED naming the directory instead of blocking, and reads at most 4096 directory entries in fixed-size batches; a directory holding more runs no plugin and yields one PLUGIN_FAILED naming the directory. Its regular executable files, sorted by name and at most 32, each run with the primary worktree as cwd, no stdin, a 10 s timeout, and stdout capped at 64 KiB, in their own process group, which is killed (SIGKILL) whenever the plugin exits or times out, before the doctor continues, so no descendant outlives it. Stdout must be one JSON array of at most 64 objects with exactly `kind`, `who`, `detail` and optional `remedy` (kind matching `^[A-Z][A-Z0-9_]{0,63}$`, other strings valid prose up to 1024 bytes). Each becomes a finding with source `plugin:<name>` beside the built-ins and the envelope is marked untrusted. A nonzero exit, timeout, oversized or malformed output becomes one PLUGIN_FAILED finding naming the plugin, never a command failure.
- `TQD-V0-011`: (accepted, decision 0481; GitHub #715) `doctor --refresh` writes the item, with profile `taskman-doctor-cache/0` and `refreshedAt`, to `<git common dir>/taskman-doctor/summary.json` (directory 0700, file 0600) through a temporary file, fsync and rename, so a reader sees the old or the new cache, never a mixture. Every step is anchored on descriptors, never on a re-resolved path: the common directory is pinned, the cache directory is created beneath it if absent and opened once with `O_NOFOLLOW|O_DIRECTORY`, and its type and owner are checked on that descriptor; the chmod, lock, destination re-read, temporary-file create (`O_EXCL|O_NOFOLLOW`), fsync and rename are all relative to that open directory. Because a descriptor pins the directory but not its place, before the chmod, after taking the lock, before the temporary-file create, before the rename and before reporting success the refresh checks, relative to the pinned common directory and without following a link, that `taskman-doctor` is still the directory it opened; a directory moved away after it was opened (for example into the state directory) is refused UNSUPPORTED_FILESYSTEM and the refresh's temporary file is removed. A cache directory, `summary.json` or lock file that is a symbolic link, not a directory or regular file (including a directory where the lock file or `summary.json` belongs), or not owned by the effective user is refused UNSUPPORTED_FILESYSTEM before anything is created, chmodded or written through it, including a directory swapped for a link after the doctor's first check. The refresh holds an exclusive `flock` on `taskman-doctor/.lock` (a regular 0600 file the refresh creates) from before it re-reads the destination until after the rename, so concurrent refreshes are serialized. It acquires the lock with non-blocking `flock(LOCK_EX|LOCK_NB)` attempts every 50 ms for at most 3 s; a lock still held after that wait is refused LOCK_TIMEOUT before the destination is re-read, with nothing written. The encoded cache never exceeds the 1 MiB `--line` read bound (TQD-V0-003). The cache is outside the journal and the state directory and is never an input to any other command; plain `doctor` reads it only to carry `firstSeen`. A cache whose profile names a later `taskman-doctor-cache/N` is refused UNSUPPORTED_VERSION and treated as unavailable (CAL-V0-131); `--refresh` then refuses UNSUPPORTED_VERSION and leaves that file byte-identical, repeating that check under the lock so a newer cache installed after the first check is also kept, while any other undecodable cache is replaced.
- `TQD-V0-012`: (accepted, decision 0481; GitHub #715) `doctor --line` reads only the cache, through the repository resolution walk to the git common directory (reading no intent hint, queue manifest, HEAD or linked worktree entry) and one bounded (1 MiB) non-following regular-file read, with no lock, no store read and no write, and prints one plain-text line `lanes F/T free | sessions N | 24h N done | alerts N`, adding `| stale Nm` when the cache is at least 15 minutes old. It is a documented exception to the envelope rule. A missing or invalid cache prints `doctor cache unavailable; run corvint-tasks doctor --refresh` and exits 1. It completes in under 50 ms on a local cache.
- `TQD-V0-013`: (accepted, decision 0519; V1-1176; GitHub #791) NO_PROGRESS_HANDOFF is not reported for a ticket whose current audited record has status COMPLETED, ARCHIVED or HELD, whatever its hand-off history in the scan: a completion, archive or hold ends the hand-off loop the finding's remedy addresses. A ticket whose record is DRAFT or OPEN, or that is absent from the inventory, is still tested under TQD-V0-005. The exclusion reads the same inventory as TQD-V0-008 and adds no journal read, write or new finding kind.
- `TQD-V0-020`: (accepted 2026-10-17, V1-1598, standing lane auto-accept; GitHub #1041) `--config FILE` names an operator file (relative to the working directory, at most 64 KiB, regular, only read) holding one `taskman-doctor-config/0` object with exactly `profile` and optional `thresholds`, `notify` and `dispatchers`. `thresholds` is closed; each member is a decimal string and defaults when absent: `lockHeldSeconds` 60, `slowCommandSeconds` 60, `noCompletionsSeconds` 7200, `fdPressurePercent` 80 (at most 100), `sessionWindowSeconds` 3600 (at most 604800), `retryStorm` 3, `hostErrors` 3 and `earlyRefusal` 3; each is at least 1, durations are at most 2592000 and counts at most 1048576. `notify` is `{command, minIntervalSeconds?}`, `command` an argv array of 1..64 strings whose first is an absolute path, `minIntervalSeconds` 0..604800 (default 0). `dispatchers` is an array of 1..8 `{programDir, format?}` with `programDir` a dispatcher program state directory (relative to the config file's directory) and `format` one of the dispatch usage formats. An unknown member, a missing required member, a non-decimal or out-of-range value, or an unknown format refuses MALFORMED, and a later `taskman-doctor-config/N` refuses UNSUPPORTED_VERSION (the profile is a CAL-V0-131 live format); nothing is written. Without `--config` the defaults apply and no notify hook or dispatcher is read. The QUEUE_LOCK_HELD, SLOW_QUEUE_COMMAND, FD_PRESSURE, NOTIFY_FAILED and host-session findings carry `firstSeen` from the cache like FALSE_IDLE (TQD-V0-003); every finding of TQD-V0-021..031 is dropped when it is no longer detected.
- `TQD-V0-021`: (accepted 2026-10-17, V1-1598, standing lane auto-accept; GitHub #1041; amended 2026-10-17, Codex review of #1060, standing lane auto-accept) QUEUE_LOCK_HELD names this queue's `taskman.lock` or `taskman.prepare.lock` (in the git common directory) when it is held and its holder, a local `corvint-tasks` process, has been running at least `lockHeldSeconds`; `detail` gives the holder's pid, age in seconds, its command line (at most 256 bytes) and how many other `corvint-tasks` processes have the file open. Detection opens the lock file read-only without creating it and takes no lock: linux names the holder from `/proc/locks` and finds openers in `/proc/<pid>/fd`; darwin tests the lock with `F_GETLK`, which does not record a flock holder, so the holder named is the longest-running `corvint-tasks` process with the file open (by `lsof` restricted to those pids and files, 5 s), and its age is the process age, not the lock-hold age. Processes come from one bounded walk (`/bin/ps -axww -o pid=,etime=,command=`, the TQD-V0-009 bounds); only processes whose argv[0] base name is `corvint-tasks` and that have one of this queue's lock files open are considered, never the doctor itself. A lock held by any other process adds a `scan` warning naming the file and no finding, including on darwin when `F_GETLK` reports a holder pid that is not a considered `corvint-tasks` process: the longest-running opener stands in only for a holder whose pid the platform does not report; a probe that cannot complete adds a warning and no finding; other platforms report nothing.
- `TQD-V0-022`: (accepted 2026-10-17, V1-1598, standing lane auto-accept; GitHub #1041) SLOW_QUEUE_COMMAND names (`who` `pid N`) each other `corvint-tasks` process with one of this queue's lock files open, not already named by QUEUE_LOCK_HELD, running at least `slowCommandSeconds`, whose verb is not one that runs long by design (`run`, `run-batch`, `dispatch`, `service`, `qualify`, `doctor`, `preflight`, `pool` and `gate run`); `detail` is `corvint-tasks <verb> running <N>s with the queue lock open (<command>)`. A command that never opens a lock file is not seen.
- `TQD-V0-023`: (accepted 2026-10-17, V1-1598, standing lane auto-accept; GitHub #1041) NO_COMPLETIONS names the queue when no completion has been recorded for at least `noCompletionsSeconds` (default 2 h) while at least one ticket is claimable under the recorded claimability rules and a worker is idle: a pool member is free, or the policy has no pool members and no session is running. The reference time is the newest completion `recordedAt` in the inventory, else the earliest receipt of the TQD-V0-004 scan; with neither, or with no claimable ticket, there is no finding, so an empty queue never reports. `firstSeen` is the reference time plus the window and `detail` gives the elapsed seconds, the reference, the ready count, free and total lanes and running sessions. The finding clears at the next completion or when the queue is drained or busy.
- `TQD-V0-024`: (accepted 2026-10-17, V1-1598, standing lane auto-accept; GitHub #1041) FD_PRESSURE names `system` when the system file table's in-use handles are at least `fdPressurePercent` of its maximum (linux `/proc/sys/fs/file-nr`; darwin `kern.num_files` of `kern.maxfiles`); `detail` gives used, maximum, the percentage and the threshold. A probe failure adds a `scan` warning; other platforms report nothing.
- `TQD-V0-026`: (accepted 2026-10-17, V1-1601, standing lane auto-accept; GitHub #1044; amended 2026-10-17, Codex review of #1060, standing lane auto-accept) With `notify` configured, `doctor --refresh` runs the hook at most once per change of the finding set: while holding the cache lock it digests the sorted kind, source and who of the findings (excluding NOTIFY_FAILED) and runs the hook only when that digest differs from the last digest it ran for, recorded in the cache's `notify` member `{notifiedSet, attemptedAt, failure}`, and at least `minIntervalSeconds` have passed since `attemptedAt`. An absent or unreadable cache counts as the empty set, so a queue with no findings never notifies. The hook receives the refreshed `taskman-doctor/0` item as JSON on stdin and runs under the TQD-V0-010 plugin rules: an argv exec (no shell), the primary worktree as cwd, its own process group killed when it exits or times out, the 10 s timeout and stdout capped at 64 KiB and otherwise discarded. A set it already ran for never runs it again, whether that run succeeded or failed. A nonzero exit, timeout or oversized output is recorded as `failure` and becomes one NOTIFY_FAILED finding (`who` `notify`) in that refresh and in every plain `doctor` with the same config until a later run succeeds; the doctor itself still succeeds. Plain `doctor` and `--line` never run the hook. A `--refresh` without `notify` configured keeps the cache's recorded `notify` member unchanged, so it neither erases the last notified set nor clears a recorded failure.
- `TQD-V0-027`: (accepted 2026-10-17, V1-1604, standing lane auto-accept; GitHub #1047; amended 2026-10-17, Codex review of #1060, standing lane auto-accept) For each configured dispatcher, the doctor reads each worker directory's `stdout.log` (at most 4096 entries, 256 logs, 9 MiB each, regular files only, modified within `sessionWindowSeconds`, taken in worker-name byte order so which 256 are read never depends on directory order) under the worker's recorded usage format, else the dispatcher's `format`, and skips a worker with neither. HOST_RETRY_STORM names the worker id when its longest run of turn or step starts with no finished turn between them is at least `retryStorm`.
- `TQD-V0-028`: (accepted 2026-10-17, V1-1604, standing lane auto-accept; GitHub #1047) HOST_ERRORS names the worker id when the log holds at least `hostErrors` host error events of its format.
- `TQD-V0-029`: (accepted 2026-10-17, V1-1604, standing lane auto-accept; GitHub #1047) EARLY_REFUSAL names the worker id when at least `earlyRefusal` tool calls failed or were declined before the session's first successful tool call.
- `TQD-V0-030`: (accepted 2026-10-17, V1-1604, standing lane auto-accept; GitHub #1047; amended 2026-10-17, Codex review of #1060, standing lane auto-accept) Host-session findings derive only from lines a known usage format recognises (`dispatch.ClassifyHostLine`): codex `turn.started`, `turn.completed`, `turn.failed`, `error` and completed `command_execution`/`file_change` items by status and exit code; opencode `step_start`, `step_finish` and `error`; claude-code `result` only when the CAL-V0-075 result reader admits the line (`is_error` then distinguishes error from finish; a line it refuses counts for nothing) and each top-level `tool_result` block of a `user` line as one tool call by its `is_error`, in block order. A line with a duplicated or case-variant `type`, a line over the usage-line bound, an unrecognised line and every line of an unknown format count for nothing. Each finding's `detail` is `<count> <what> (<format>) in <stdout.log path> lines <first>-<last>`, citing the evidence file and the line range of the counted events. An unreadable workers directory or log adds a `scan` warning and no finding.
- `TQD-V0-031`: (accepted 2026-10-17, V1-1519, standing lane auto-accept; approved TQD-V0-001 amendment; amended 2026-10-17, Codex review of #1060, standing lane auto-accept) For each configured dispatcher the item's `dispatchers` array (in configuration order) holds `{programDir, workState}` with the CAL-V0-270 work-state reader health from the dispatcher's `state.json`: `health` is OK, FAILING, NOT_OBSERVED (no reader health recorded, including an absent `state.json`) or UNREADABLE (`state.json` malformed, unreadable, not a regular file or recorded for another program, with a `scan` warning; it is opened without following a final symlink and without blocking, so a FIFO is refused, not waited on), and once recorded `since`, `lastError` (`NONE` before any failure), `failedTicks` and `escalated`. A FAILING reader also yields one WORK_STATE_FAILING finding naming `programDir`, with `firstSeen` at `since`. This report reads the ledger only and amends no lease or dispatch requirement.
- `TQD-V0-032`: (accepted 2026-10-17, V1-1192, standing lane auto-accept) NO_PROGRESS_HANDOFF counts only generations claimed at the ticket's current acceptance revision, so a completion followed by a reopen, which raises the revision, starts a new streak; a ticket absent from the inventory keeps every generation. This replaces the earlier reading under which pre-completion hand-offs still counted after a reopen.
- `TQD-V0-033`: (accepted 2026-10-17, V1-1193, standing lane auto-accept; amended 2026-10-17, Codex review of #1060, standing lane auto-accept) One doctor scan reads the journal through one read session that pins the store's parent directories once and opens each receipt or evidence blob beneath them, instead of re-resolving the roots for every read; with the observation clock fixed, the complete encoded `findings` and `scan` values are byte-identical to per-read access. On a synthetic store of 607 receipts the best of five interleaved runs measured 792 ms CPU (3.04 s wall) with per-read access and 529 ms CPU (2.27 s wall) with one session on a host at load 40-60. The scan stays bounded by TQD-V0-004; results are not cached between runs.

- `TQD-V0-040`: (accepted 2026-10-18, V1-1608, standing lane auto-accept; GitHub #1050) Under a store policy that declares the ATR-V0-080 `progressSignal` `HANDOFF_EVIDENCE`, NO_PROGRESS_HANDOFF MUST NOT count an ended generation whose recorded hand-off `--evidence` reference is new (no earlier generation of the ticket recorded one) or differs from the latest earlier recorded reference; a generation with no reference, or the same one, counts as TQD-V0-005 states. The doctor reads the reference recorded on each attempt, so generations recorded before the declaration are judged by their own references. The finding's detail then names the signal: `N consecutive hand-offs with no gate result, review, new candidate tree or changed S progress fingerprint (progress signal S)`. Without the declaration the detector and its detail are unchanged.
- `TQD-V0-041`: (accepted 2026-10-18, V1-1608, standing lane auto-accept; GitHub #1050) Under the same declaration REPEAT_REFUSAL MUST restart its count for a ticket at each ended generation whose recorded hand-off reference is new or changed, so refusals of one candidate tree separated by recorded progress are not counted together, and its detail reads `candidate tree T refused N times with no changed S progress fingerprint between (progress signal S)`. Without the declaration the detector and its detail are unchanged.
- `TQD-V0-042`: (accepted 2026-10-18, V1-1608, standing lane auto-accept; GitHub #1050) SETUP_ONLY_PROOF MUST ignore the progress signal: its judgment is witnessed core obligations, which a progress fingerprint does not witness, so its findings are the same with and without the declaration.

- `TQD-V0-043`: (accepted 2026-10-19, V1-1914, standing lane auto-accept; GitHub #1181) For each configured dispatcher the doctor MUST read the ATR-V0-150 queue observation (`admission.json`, profile `taskman-dispatch-admission/0`) from its `programDir`. A missing observation yields nothing. One that does not decode (another profile, another program, out of bounds), or that is not a regular file (a FIFO is refused without blocking, and a final symbolic link is not followed), adds a `scan` warning `queue observation unreadable` and no finding; one written more than three of its own ticks before the scan's observation time adds the warning `queue observation is stale` and no finding. Each held run-only ticket the observation records with at least `runnableHeldIdleTicks` consecutive ticks (default 3, settable in the `--config` thresholds beside TQD-V0-020's, 1..1048576) and at least one free member yields one RUNNABLE_HELD_IDLE finding: `who` is `program/role/ticket`, `firstSeen` the streak's first tick, and the detail names the fence (`PLAN_WINDOW`, `GLOBAL_CAP`, `ROLE_CAP`, `TIER_CAP`, `BUDGET` or `PRESSURE`) with its recorded reason, the tick count, the pool, its free-member count and their names. Tickets of roles not declared `runOnly`, and holds while the pool had no free member, are never recorded and so never reported.
- `TQD-V0-044`: (accepted 2026-10-19, V1-1916, standing lane auto-accept; GitHub #1183) From the same observation, a role whose queue (running plus waiting sessions) stayed deeper than its effective cap for at least `roleQueueDeepTicks` consecutive ticks (default 3, settable like `runnableHeldIdleTicks`) yields one ROLE_QUEUE_DEEP finding when the observation's pressure is `CALM`: `who` is `program/role`, `firstSeen` the streak's first tick, and the detail names the depth with its running and waiting counts, the effective cap with `cap` and any `capMax`, the oldest waiting ticket and since when, and the headroom: the global cap's free sessions, or that the global cap is full. An observation whose pressure is `RAISED`, `UNKNOWN` or `NOT_CONFIGURED` yields no ROLE_QUEUE_DEEP finding: missing or unknown pressure abstains rather than suggesting a raise. These two thresholds extend TQD-V0-020's closed list by exactly these members.
- `TQD-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1193; GitHub #791) When the scan restores attempt records that elided liveness receipts omit (CAL-V0-300), walks for different attempts MUST share one read and decode of each receipt within one doctor run, so a scan reads each receipt at most twice (once in the scan, once in a walk) instead of once per attempt whose walk crosses it; a receipt's read or decode error is shared the same way, so a walk that would have failed still fails. With the observation clock fixed, the complete encoded `findings` and `scan` values are byte-identical to unshared walks. Nothing is cached between runs and the walk bound (`snapshot.MaxElidedRun`) and TQD-V0-004's scan bound are unchanged. On a synthetic compacted store of 1,009 receipts (24 live attempts x 40 heartbeats) the best of five interleaved runs measured 4.06 s CPU (3.37 s wall) unshared and 1.30 s CPU (1.03 s wall) shared at host load 29-64 on 12 CPUs. On a padded store of 30,073 receipts (24 live attempts, 29,280 elided; TQD-V0-046) the doctor, scanning its 4,096-receipt bound on a 4-vCPU Linux host, measured 38.2 s CPU (38.7 s wall) unshared and 25.5 s CPU (25.8 s wall) shared (V1-2137).

- `TQD-V0-820`: (experimental under the standing lane auto-accept; V1-1193; GitHub #791) The
  doctor's `scan` MUST report `journalAudit`, how the read's journal audit ran: `FULL` (every
  retained receipt was walked, so the doctor's cost grows with the journal) or
  `CHECKPOINT_PLUS_TAIL` (it resumed from a writer-retained read checkpoint, `CAL-V0-059`, so only
  the tail past it and the TQD-V0-004 window are read), or `NOT_OBSERVED` when the read needed no
  audit (a store with no claim yet). Nothing else in `scan`, `findings` or the
  cache changes, and the doctor never writes a checkpoint (a read writes no store state). Measured
  with `TestCALV0360_PaddedDoctorWalkCost`, which now times both cases: on a padded store of 30,073
  receipts (24 live attempts, 29,280 elided) at host load about 5 on 12 CPUs, the best of five
  doctor runs took 10.8 s wall / 17.3 s CPU with `FULL` and 1.26 s wall / 2.0 s CPU with
  `CHECKPOINT_PLUS_TAIL` (shared walks); unshared walks with the checkpoint took 7.46 s / 11.2 s.
  The scan stays bounded by TQD-V0-004 and keeps reporting `truncated`. Non-goals: a doctor result
  cache read by default, a smaller scan bound, and any change to the audit. Failure mode: a store
  whose writers never retained a checkpoint (for example one built by `fixture.PadLiveness`) still
  pays the full audit, now visible as `journalAudit: FULL`. Falsifier: a doctor run that resumed
  from a checkpoint but reports `FULL`, or the reverse. Evidence:
  `TestTQDV0820DoctorScanNamesJournalAudit`. Rollback: drop the member.

### V1-2137 padded-journal fixture builder

Re-homed (owner rule 2026-10-17; batch 1023a): this requirement was first numbered CAL-V0-360 in the
agent leases spec; it moved here as TQD-V0-046 with the same text, because that spec's
`## Requirements` section is at the 256-obligation OCM bound. The old ID is a retired alias: code
comments, messages and test names keep the old numbers.

Human-owned input: native ticket V1-2137. The doctor walk cost of TQD-V0-045 could not be measured at
30,000 receipts: a store built through the command line costs about 0.2 s per write, so 30,000
heartbeats take about 100 minutes, and the V1-1193 before and after at that size stayed `NOT_RUN`.

- `TQD-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2137) A test-only
  builder, `fixture.PadLiveness`, MUST append a requested number of completed heartbeat receipts
  to an unsegmented store that has live attempts, in O(N) file writes and without running an audit,
  so that the complete audit finds the result CONSISTENT with its projections in AGREEMENT and the
  doctor's elided-record walk (TQD-V0-045) restores records from it. Each appended receipt MUST be
  the attempt's latest whole liveness receipt re-sequenced, re-chained to the previous receipt's
  original digest, re-timed and re-requested: it posts the attempt record with a new
  `lastHeartbeatAt` and a request afterimage of its own. With elision requested, a receipt that a
  later receipt of the attempt supersedes MUST be written in the elided form of CAL-V0-300,
  except that a receipt whose journal sequence lies `snapshot.MaxElidedRun` or more receipts after
  the attempt's previous whole receipt stays whole (CAL-V0-302). The distance is measured in global
  journal sequence, across the receipts of every attempt, not in receipts of that attempt, so that
  no more than `snapshot.MaxElidedRun` receipts ever separate an elided receipt from its attempt's
  previous whole one and the backward walk stays bounded however many attempts interleave; an
  attempt's last receipt is always whole. The builder MUST refuse a store it cannot extend (no
  attempt with a lease, no whole liveness receipt among the last 4,096 receipts, a segmented head).
  A receipt changed after it was built MUST still fail the audit. It writes only attempt files,
  request files, receipts and `head.json`.

Non-goals: product code (the builder is test support in `internal/tasks/fixture`); a duration
assertion; a store that matches field use in anything but receipt count and elision shape; padding
with any receipt kind but a heartbeat of a live attempt.

Failure modes: a template receipt is found by reading back from the head, so a store whose attempts
have no whole liveness receipt near the head must record one real heartbeat per attempt first; a
retained checkpoint older than the new head is stale and the next read takes the complete audit,
as for any store.

Rollback: delete `internal/tasks/fixture/padded.go` and its tests.

## Detectors

Every built-in journal threshold is 3 within the 7-day window; the operational and host-session thresholds are TQD-V0-020's, settable by `--config`. Remedies:

| Kind | Remedy |
| --- | --- |
| NO_PROGRESS_HANDOFF | Read the hand-off evidence, then refine, split or hold the ticket instead of claiming it again. |
| REPEAT_REFUSAL | Compare the review returns and failed gates for this tree; change the candidate or the criteria before resubmitting. |
| SLOW_LANE_RECOVERY | Inspect the member's cleanup output and the external resource; repair the cleanup, then confirm safety. |
| SETUP_ONLY_PROOF | Witness the core obligations or reopen the ticket; its completion proved only setup. |
| FALSE_IDLE | The session's process tree is busy while its heartbeat is stale; inspect the run before reaping or relaunching. |
| PLUGIN_FAILED | Fix or remove the project plugin named in `who`. |
| QUEUE_LOCK_HELD | Inspect the named process; if it is wedged, stop it so the queue lock is released, then rerun the doctor. |
| SLOW_QUEUE_COMMAND | Inspect the named command; a queue command this slow is usually waiting on the lock or a large journal read. |
| NO_COMPLETIONS | Ready tickets and free capacity exist but nothing completed; check the dispatcher and the workers' recent output. |
| FD_PRESSURE | Close idle sessions or raise the system file limit before launches start failing. |
| HOLD_CONDITION_MET | The hold's typed condition is met at its ref (SCC-V0-013); release the hold (ticket release-hold) so the ticket returns to its work stage. |
| HOLD_CONDITION_UNKNOWN | The hold's condition cannot be evaluated (SCC-V0-013); fetch the named ref or commit, or correct the condition. The ticket stays held. |
| FREE_TEXT_HOLD_MET | The free-text hold names a commit or path that origin's default branch (`refs/remotes/origin/HEAD`) already has (SCC-V0-014); confirm the blocker is gone, then release the hold or replace it with a typed condition. |
| NOTIFY_FAILED | Fix the notify hook named in the doctor config; the doctor retries it at the next change of findings. |
| HOST_RETRY_STORM | The host keeps restarting turns without finishing one; check its provider limits and the worker's stdout.log lines cited. |
| HOST_ERRORS | The host reported repeated errors; read the cited stdout.log lines before relaunching the worker. |
| EARLY_REFUSAL | The session's first tool calls were all refused; check the worker's sandbox and permission profile. |
| WORK_STATE_FAILING | The dispatcher cannot read its work-state source; run dispatch status and repair the source named in lastError. |
| RUNNABLE_HELD_IDLE | Run-only work waits beside free pool members; lift or widen the named rule for it, or set the role's bypassFocus (ATR-V0-151) when the plan window holds it. |
| ROLE_QUEUE_DEEP | The role's queue stays deeper than its cap at calm pressure; raise its cap, or set capMax (ATR-V0-152), within the reported headroom. |
| PARKED_WITH_OPEN_ITEMS | A parked ticket still has open obligation items; an item leaving BLOCKED releases it at the next dispatcher observation (TOL-V0-124), otherwise decide the items or run dispatch unpark --key with the ticket. Emitted for each configured dispatcher's parked ticket key whose ticket carries an obligation ledger with fewer witnessed than total items (TOL-V0-125). |

## Failure modes

- A pruned or rewritten receipt stops the scan (`scan.truncated`); detectors then see only the newer tail.
- Refused requests (claim, release, confirm-safe) write no receipt and cannot be counted.
- TQD-V0-013 keys on the current status only; TQD-V0-032 resets the streak at a reopen through the acceptance revision. A generation whose attempt record lacks its ticket revision does not match and is not counted.
- The lane identity's leader start time is not recorded, so FALSE_IDLE uses detached run supervisor identities only; supervised and dispatcher workers are not walked.
- A plugin is operator-chosen code; the doctor bounds its time and output but does not sandbox it. A descendant that leaves the plugin's process group (for example with `setsid`) escapes the group kill.
- A gate result added in the receipt where the scan first meets a pre-existing attempt is indistinguishable from the attempt's older results and is not counted, so REPEAT_REFUSAL can under-count by one at the window edge, never over-count.
- The cache can be stale; `--line` shows its age once it is 15 minutes old.
- On darwin the QUEUE_LOCK_HELD holder is the longest-running `corvint-tasks` opener, which may be a waiter rather than the holder, and its age is the process age. A lock held by a non-`corvint-tasks` process is a warning, not a finding. Read-only queue commands that never open a lock file are invisible to SLOW_QUEUE_COMMAND.
- Process ages come from `ps` elapsed time; a pid reused between the process walk and the lock probe can be misnamed for one run.
- NO_COMPLETIONS reads claimability from the recorded inventory; a ticket made unclaimable by state the doctor does not read (for example an external hold) can keep it firing until the next completion.
- The notify hook runs while the refresh holds the cache lock, so a concurrent refresh can refuse LOCK_TIMEOUT for up to the hook's 10 s timeout. Like a plugin, the hook is operator-chosen code bounded in time and output but not sandboxed. A failed hook is not retried for the same finding set; the next change of findings retries it.
- EARLY_REFUSAL never fires for opencode and HOST_RETRY_STORM never fires for claude-code, whose usage vocabularies carry no tool result or turn start; their other detectors still apply. Logs rotated away (CAL-V0-143) are not read, and a log older than the session window is skipped.
- A dispatcher `programDir` whose base name is not the ledger's program is UNREADABLE.
- The descriptor anchoring and the refresh lock exist on darwin and linux only. Elsewhere the refresh keeps the path-rooted `os.Root` writer: a cache directory swapped for a link inside the common directory between its checks can be followed, and concurrent refreshes are not serialized, so the last rename wins.
- The location checks bracket each step but cannot be atomic with it: a cache directory moved between the last pre-rename check and the rename receives the new `summary.json`; the post-rename check then refuses UNSUPPORTED_FILESYSTEM rather than reporting success, and the write is not undone.
- A refresh that is suspended or killed while holding the lock does not keep it beyond its process (`flock` is released when the descriptor closes), but while a holder is alive every other refresh refuses LOCK_TIMEOUT after 3 s.
- The lock is advisory: a writer that does not take `taskman-doctor/.lock` (an older build of this verb, or any other program) can still replace `summary.json` after the re-read. The `.lock` file stays in the cache directory; rollback removes it with the directory.
- Under a declared progress signal an agent that records a new hand-off reference without real progress silences NO_PROGRESS_HANDOFF and restarts REPEAT_REFUSAL; that is the adopter's declared trade (TQD-V0-040, TQD-V0-041).
- RUNNABLE_HELD_IDLE and ROLE_QUEUE_DEEP see only what the dispatcher recorded at its last tick: its streaks live in its memory, so a restarted dispatcher restarts them and the findings disappear until the threshold is reached again. A dispatcher too old to write the observation, or stopped for more than three ticks, yields no finding (absent or stale), never a guessed one.

## Non-goals

- The doctor does not stop, signal or reprioritise any process, release a lock, retry a worker or change dispatcher state; every finding is advisory.
- No daemon or periodic scheduling: the notify hook runs only inside an operator- or cron-invoked `--refresh`.
- No delivery channel (mail, chat, webhook) is built in; the hook is the operator's program.
- No new journal, lease or dispatch format of the doctor's own; the leases spec is not amended. TQD-V0-043..044 read the dispatcher-owned queue observation, whose format ATR-V0-150 defines in the attempt-runner spec.

## Acceptance evidence

Focused tests in `internal/tasks/cli/doctor_test.go` build each built-in pattern through the real
CLI against a disposable fixture store, run a passing and a failing plugin, assert that plain
`doctor` leaves the state directory, intent tree and cache directory byte-identical, that
`--refresh` writes the cache atomically, refuses a linked cache directory or file, a cache directory swapped for a relative link after its first check or moved into the state directory after it was opened, a directory where the lock file belongs, and a newer cache format, including one installed after the first check, without writing, that a journal-absent store refuses MISSING_EVIDENCE, and fits the largest plugin output into a cache `--line` reads, that plugin descendants are gone when the doctor returns and an oversized plugin directory runs nothing, that a FIFO in place of a run directory does not block FALSE_IDLE discovery and a FIFO in place of the plugin directory does not block plugin discovery, that a refresh refuses LOCK_TIMEOUT within its bounded wait while another holder keeps the cache lock, and time `--line` under 50 ms. `internal/tasks/intent/intent_worktree_test.go` shows the common-directory resolution `--line` uses reads no queue manifest or linked worktree entry.

`internal/tasks/cli/doctor_ops_test.go` (darwin and linux) covers TQD-V0-020..033 through the real CLI: config refusals and defaults; QUEUE_LOCK_HELD and SLOW_QUEUE_COMMAND under injected process tables and, in `TestTQDV0021_RealProbes`, a real helper process holding the real lock through the real `ps`, `lsof` or `/proc` and file-table probes; NO_COMPLETIONS on an empty queue, inside and past the window, with a running session and after a completion; FD_PRESSURE at and below the threshold; the notify hook once per change, never on an identical set, under `minIntervalSeconds`, and NOTIFY_FAILED for a failing, timed-out and oversized hook; host-session findings with exact cited line ranges and none for unknown formats or below threshold; work-state health for NOT_OBSERVED, FAILING, OK and UNREADABLE ledgers; the reopen streak reset; and, opt-in with `CORVINT_DOCTOR_SCAN_COST=N`, the interleaved per-read versus one-session scan measurement asserting identical findings. `internal/tasks/dispatch/outcome_test.go` tables the host-line classifier, and the `/proc` and `lsof` parsers have fixture tests.

`internal/tasks/cli/progress_signal_test.go` (`TestATRV0082_CLIProgressSignal`) and
`internal/tasks/cli/doctor_progress_test.go` cover TQD-V0-040..042 through the doctor's detectors: a
changed reference is not counted and names the signal, refusals separated by a changed reference
restart, and SETUP_ONLY_PROOF is identical with and without the declaration.

`internal/tasks/cli/doctor_admission_test.go` (darwin and linux) covers TQD-V0-043..044 through the
real CLI against fixture observations: nothing for a missing observation; RUNNABLE_HELD_IDLE with the
fence, ticks, pool and free members named; none below the default or above a configured threshold;
a stale or newer-profile observation warns and reports nothing; ROLE_QUEUE_DEEP with depth, caps,
oldest wait and headroom, the full-global-cap headroom, both thresholds, and no finding at `UNKNOWN`,
`NOT_CONFIGURED` or `RAISED` pressure.

`internal/tasks/cli/doctor_walk_test.go` (darwin and linux) covers TQD-V0-045 on a store built through the
real CLI and compacted with `CompactLiveness`: shared and unshared walks give identical findings and
scan values, shared walks read no receipt more than twice, and unshared walks demonstrably re-read; opt-in
with `CORVINT_DOCTOR_WALK_COST=N` (and `CORVINT_DOCTOR_WALK_ATTEMPTS`), the interleaved cost measurement.

## Compatibility and rollback

The verb, item profile and cache profile are new; no stored format changes. Rollback removes the
verb and deletes `<git common dir>/taskman-doctor/`, which nothing else reads. TQD-V0-020..033 add
only optional members (`--config`, the item's `dispatchers`, the cache's `notify`) and new finding
kinds; rolling them back removes the config flag and those detectors, and an older doctor drops the
cache's `notify` member at its next refresh. The journal read session (TQD-V0-033) is additive; rollback
returns the scan to per-read access with identical results.
The progress-signal amendments (TQD-V0-040..042) change no format; rolling them back returns the two detectors to their undeclared
behaviour under every policy.
The TQD-V0-043..044 amendments add two finding kinds and two optional thresholds; rolling them back removes the
detectors, and a doctor config naming `runnableHeldIdleTicks` or `roleQueueDeepTicks` is then refused
as an unknown threshold, so remove those members first.
The TQD-V0-045 walk sharing is additive and in-memory; rollback returns each walk to its own reads with identical results.

## Traceability

| Requirement | Evidence |
| --- | --- |
| TQD-V0-001 | `TestTQDV0001_DoctorIsPureRead`, `TestTQDV0001_JournalAbsentIsMissingEvidence`, `TestTQDV0020_DoctorConfig`, `TestTQDV0031_WorkStateHealth` |
| TQD-V0-002 | `TestTQDV0012_LineReadsCache` |
| TQD-V0-003 | `TestTQDV0005_NoProgressHandoff`, `TestTQDV0010_PluginFindings`, `TestTQDV0011_MaximalPluginOutputStaysReadable` |
| TQD-V0-004 | `TestTQDV0005_NoProgressHandoff` |
| TQD-V0-005 | `TestTQDV0005_NoProgressHandoff` |
| TQD-V0-006 | `TestTQDV0006_RepeatRefusal`, `TestTQDV0006_OldGateResultsAreNotRecentRefusals` |
| TQD-V0-007 | `TestTQDV0007_SlowLaneRecovery` |
| TQD-V0-008 | `TestTQDV0008_SetupOnlyProof` |
| TQD-V0-009 | `TestTQDV0009_FalseIdle`, `TestTQDV0009_FalseIdleSkipsRunsFIFO` |
| TQD-V0-010 | `TestTQDV0010_PluginFindings`, `TestTQDV0010_PluginDescendantsAreKilled`, `TestTQDV0010_PluginDiscoveryIsBounded`, `TestTQDV0010_PluginDiscoveryDoesNotBlockOnFIFO` |
| TQD-V0-011 | `TestTQDV0012_LineReadsCache`, `TestCALV0131_EveryLiveFormatRefusesANewerVersion`, `TestTQDV0011_RefreshRefusesSymlinkedCache`, `TestTQDV0011_RefreshKeepsNewerCache`, `TestTQDV0011_RefreshRefusesSwappedCacheDir`, `TestTQDV0011_RefreshRefusesCacheDirMovedAfterOpen`, `TestTQDV0011_RefreshRefusesDirectoryLock`, `TestTQDV0011_RefreshLockWaitIsBounded`, `TestTQDV0011_RefreshRechecksVersionUnderLock`, `TestTQDV0011_MaximalPluginOutputStaysReadable` |
| TQD-V0-012 | `TestTQDV0012_LineReadsCache`, `TestTQDV0012_ResolveCommonDirReadsNoManifest`, `TestTQDV0011_MaximalPluginOutputStaysReadable` |
| TQD-V0-013 | `TestTQDV0013_NoProgressSkipsCompletedHeldArchived` |
| TQD-V0-020 | `TestTQDV0020_DoctorConfig`, `TestCALV0131_EveryLiveFormatRefusesANewerVersion`, `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` |
| TQD-V0-021 | `TestTQDV0021_QueueLockHeld`, `TestTQDV0021_RealProbes`, `TestTQDV0021_ProbeParsers` |
| TQD-V0-022 | `TestTQDV0022_SlowQueueCommand` |
| TQD-V0-023 | `TestTQDV0023_NoCompletions` |
| TQD-V0-024 | `TestTQDV0024_FDPressure`, `TestTQDV0021_RealProbes`, `TestTQDV0020_DoctorConfig` |
| TQD-V0-026 | `TestTQDV0026_NotifyOncePerChange`, `TestTQDV0026_NotifyFailureIsAFinding` |
| TQD-V0-027 | `TestTQDV0027_HostSessionFindings`, `TestTQDV0027_WorkerLimitInNameOrder` |
| TQD-V0-028 | `TestTQDV0027_HostSessionFindings` |
| TQD-V0-029 | `TestTQDV0027_HostSessionFindings` |
| TQD-V0-030 | `TestTQDV0030_ClassifyHostLine`, `TestTQDV0027_HostSessionFindings` |
| TQD-V0-031 | `TestTQDV0031_WorkStateHealth` |
| TQD-V0-032 | `TestTQDV0032_ReopenResetsNoProgressStreak` |
| TQD-V0-033 | `TestTQDV0033_ReadModesEquivalent`, `TestTQDV0033_ScanCostMeasurement`, `TestTQDV0005_NoProgressHandoff` |
| TQD-V0-040 | `TestATRV0082_CLIProgressSignal` |
| TQD-V0-041 | `TestTQDV0041_RepeatRefusalHonoursProgressSignal` |
| TQD-V0-042 | `TestTQDV0042_SetupOnlyProofIgnoresProgressSignal` |
| TQD-V0-043 | `TestTQDV0043_RunnableHeldIdle` |
| TQD-V0-044 | `TestTQDV0044_RoleQueueDeep` |
| TQD-V0-045 | `TestTQDV0045_ElidedWalksShareReceipts`, `TestTQDV0045_WalkCostMeasurement` |
| TQD-V0-046 | `fixture.PadLiveness` (`internal/tasks/fixture`); `TestCALV0360_PaddedLivenessStoreAuditsAndWalks` (`internal/tasks/cli`: audit CONSISTENT at the padded head, shared doctor walk reads no receipt more than twice, a rewritten padded receipt fails the audit); opt-in `TestCALV0360_PaddedDoctorWalkCost`; measurements in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-tasks-linux-qualification-b3-harness.md` |
