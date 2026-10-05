# 2026-10-05: V1-0804 service helpers, run-helper and bounded logging

## Intent

Owner-requested follow-up to [issue 500](https://github.com/beamfall/corvint/issues/500), native ticket
V1-0804 (2026-10-05). It delivers SERVICE500-007 of `docs/specs/corvint-tasks-user-service-v0.md`:

- helpers with distinct manifest, unit and runtime identity;
- `service run-helper`;
- bounded service logging;
- helper runtime restart-debt charging.

Platform qualification stays NOT_RUN and belongs to V1-0697. Requirement IDs are unchanged.

## Change

- **Wrapper:** `internal/tasks/service/helper.go` adds `RunHelper`, the foreground manager entry point
  for one helper. The order is:
  1. a nonblocking per-helper lock `helper-<H>.lock` (U) admits one wrapper;
  2. under the shared fence F the wrapper reads control and writes the durable intent
     `helper-<H>.intent.json` (SPAWNING);
  3. it starts the helper in its own process group and verifies the leader's start identity and
     parentage;
  4. only then does it acknowledge RUNNING in the intent and in the record `helper-<H>.json` (8KiB
     bound).

  F is released before any retirement wait. The lock graph is U-helper→F→S; a helper never takes O or L
  and never latches control.
- **Descendants (Linux):**
  - `descendants_linux.go` makes the wrapper a child subreaper and verifies that setting with
    PR_GET.
  - It refuses to spawn while the wrapper has any child.
  - It starts the helper through the #464 `groupreap.Owner`.
  - Retirement:
    - stops the group, where only `Released` proves it;
    - kills and reaps every reparented descendant through a concurrent sweeper;
    - accepts only `wait4(-1, WNOHANG)` reporting ECHILD within 10s as proof.
  - `descendants_other.go` answers UNSUPPORTED.
- **Install/replace:**
  - On Linux, install accepts helpers; elsewhere they are refused UNSUPPORTED.
  - A replace saves STOPPED under F, then waits outside F for the main pulse to leave RUNNING and for
    every helper intent to be gone.
  - If a tree is not proved retired, the replace rolls back (RESTORED).
  - Resume and uninstall refuse UNCERTAIN_EFFECT while any helper intent exists.
  - A reconciled resume resets helper debt and reports `helperDebtReset`.
- **Debt:** helper exits, start failures and verification failures charge `ChargeFailure` on the helper
  record under F. A replay is keyed by spawn generation. An unknown boot clock (boot id plus
  `sysinfo` uptime) HOLDs. Main-controller runtime restart debt remains NOT_OBSERVED.
- **Logging:** `logsink.go` covers:
  - **Streams:** main stderr diagnostics and helper stdout and stderr.
  - **Files:** each goes to `logs/<unit>/<stream>.log` (mode 0600) and rotates once, so a stream keeps
    at most two 8MiB files.
  - **Buffering:** each stream has a 64KiB pending buffer and one writer goroutine.
  - **Drops:** writes never block. Overflow is dropped and counted. An I/O error latches
    `LOG_IO_ERROR` and its bytes are counted as dropped.
  - **Close:** streams close concurrently. Each waits at most 2s and counts pending and in-flight bytes
    as dropped; a write that completes later changes no counter.
  - **Counters:** one publisher goroutine writes `status.json` (4096B bound), so a stalled filesystem
    never blocks supervision. Close waits at most 2s for the final counters. Only the wrapper holding
    the helper lock opens and publishes that helper's logs, and it keeps the lock until every log
    writer and the publisher have actually exited.
  - **Opens:** `logs/<unit>` is created and pinned without following any symlink. Appends, rotation
    and status go through the pinned directory. Log files are opened no-follow and nonblocking and
    must be private regular files, so a planted symlink or FIFO neither redirects nor blocks a writer
    or status.
  - **Pipes:** before the final counters the wrapper waits at most 2s for its pipe readers to
    forward what a retired tree wrote. It then closes any pipe still held open; that pipe's unread
    bytes are not counted.
  - **Status:** reports the counters and the byte count of a sanitized no-follow tail excerpt (4KiB
    bound), but withholds the excerpt text.
- **Routes:** the CLI route `service run-helper --program P --manifest ABS --helper H` and its help. The
  helper units render `run-helper` (existing renderer).
- **Boundary:** decision 0397 gains a two-file import edge so that `descendants_linux{,_test}.go` may
  import `internal/groupreap` (agent decision; see owner questions).

## Fail-closed decisions (spec did not settle)

1. **Linux only.** The helper profile is Linux-only. Darwin has no child-subreaper equivalent that
   proves escaped descendants gone. The spec allows either HOLD or "render that helper profile
   unsupported", and the unsupported option is the closed one.
2. **Existing intent at start.** If a helper intent already exists when the wrapper starts (for
   example after a wrapper SIGKILL), the wrapper HOLDs. It does not adopt the old tree or guess that it
   is gone; an operator verifies the tree and removes the file.
3. **Any exit charges.** Every helper exit charges a failure, including exit 0, because helpers are
   declared foreground and long-running. A stop that races an exit may over-charge one failure; it
   never under-charges.
4. **No health reset.** Helper debt has no 600s health-based reset. Only a reconciled resume resets
   it, because the spec's helper health signal is not defined.
5. **Excerpt withheld.** Status withholds the log excerpt text and reports only its sanitized byte
   count. Helper output is unscreened and status must expose no secrets.
6. **Logging always on.** There is no opt-out switch. Every bound is fixed.
7. **Retire on DRAINING.** Helpers retire on DRAINING as well as STOPPED, since they are not
   dispatcher work to settle.
8. **Hold blocks spawn.** A record HOLD (unproved retirement) blocks spawning until a resume. Resume
   itself refuses while the intent remains.
9. **Replace rolls back.** A replace over a live tree that does not retire within the wait rolls back
   (RESTORED) instead of committing the new units.

## Limits

- **TOCTOU.** The helper executable hash is checked before exec. A replacement between the check and
  exec is a recorded TOCTOU limit.
- **Wrapper death.** No Pdeathsig is set. If the wrapper itself dies, systemd `KillMode=control-group`
  is the cleanup, and the remaining intent forces the manual HOLD above.
- **Coverage scope.** The wrapper starts no other children. Processes that a helper hands to another
  manager (systemd-run, at, a daemonizing service) are outside its process tree and are not covered.
- **Evidence scope.**
  - The lifecycle witnesses inject a fake helper tree and fake systemd.
  - The real-process descendant witnesses ran in a `golang:1.27.1` Linux container (Docker Desktop),
    not under a systemd user manager:
    - TestSERVICE500_LinuxHelperRetiresEscapedDescendants;
    - TestSERVICE500_LinuxHelperExitLeavesNoOrphan.
  - In that container the full `internal/tasks/service` package passed. The Linux, Helper, LogSink and
    LogStatus tests passed with `-count=3`.
  - Real unit restart, cgroup cleanup and login scope are NOT_RUN (V1-0697).

## Review repair

Codex round 1 (gpt-6-astra, read-only) returned CHANGES_REQUIRED with six findings, all repaired:

1. **P1, stop ACK.** Stop could answer ACKNOWLEDGED while a helper's SPAWNING intent was unresolved.
   Suppression now counts helper intents under F: RUNNING is a committed effect, while SPAWNING or
   unreadable is unresolved, which gives PENDING. Witness:
   TestSERVICE500_StopDoesNotAcknowledgeUnresolvedHelperLaunch.
2. **P1, log status blocking.** Status publication ran `writeAtomic` synchronously in the supervision
   loops. It now goes through one publisher goroutine with latest-wins requests and a bounded close.
   Witness: TestSERVICE500_LogStatusPublicationNeverBlocks.
3. **P2, interruption during the fence wait.** The spawn now rechecks cancellation after acquiring F.
   Witness: TestSERVICE500_HelperInterruptedWhileFencedStartsNothing, which fails without the fix.
4. **P2, start-failure backoff.** Start-failure debt now uses a clock reading taken after the failure.
   The "retain the first observation through publication retries" part is declined: a retry records
   a later reading, which only lengthens the backoff, never shortens it.
5. **P2, in-flight bytes at close.** A bounded close now counts in-flight bytes as dropped, and a late
   completion changes no counter. Witness: the extended
   TestSERVICE500_LogSinkIOErrorIsReportedNotBlocking.
6. **P3, duplicate wrapper logs.** Logs are opened only after U is acquired and are closed before U is
   released. Witness: the extended TestSERVICE500_HelperSingleController. The managed main has no
   separate U lock in this slice; a manually started duplicate main can still overwrite main log
   counters, as it already can its pulse (recorded limit).

Codex round 2 confirmed the round-1 repairs and returned CHANGES_REQUIRED with three P2 findings, all
repaired:

1. **Log I/O outlives U.** After a bounded close timed out, a late writer or publisher could still
   touch the files a successor had opened. The wrapper now keeps U until every log writer and the
   publisher have exited; process exit releases it otherwise. Witness:
   TestSERVICE500_LogStallKeepsHelperLockUntilRetired.
2. **Settlement retry undoes a resume.** If the record was written but the intent removal failed
   after the unlink, a resume could reset the debt, and the retry then wrote HOLD over it. A
   settlement now remembers that its record was written. A retry only removes a remaining intent and
   never rewrites the record. A settlement whose intent vanished before its record was written holds
   UNCERTAIN_EFFECT and writes nothing (fail-closed; the operator restarts the wrapper). Witness:
   TestSERVICE500_HelperSettlementRetryKeepsResumedRecord; the REPLAY leg of
   TestSERVICE500_HelperRestartDebtChargesAndHolds now replays with its intent present.
3. **FIFO blocks status.** `logExcerpt` now opens through `safeopen.File` (no-follow in every
   component, nonblocking), and the stream writer opens with O_NONBLOCK. Witness:
   TestSERVICE500_LogFIFONeverBlocks.

All three witnesses failed with their fix removed.

Codex round 3 confirmed the round-2 repairs and returned CHANGES_REQUIRED with three P2 findings. Two
were repaired and the third was declined, then repaired after round 4:

1. **Log writers follow parent symlinks (repaired).** `MkdirAll` and the basename-only no-follow
   open accepted a symlinked `logs/` or `logs/<unit>`. `pinLogDir` now creates both components
   beneath the pinned state root and pins `logs/<unit>` with `safeopen.Root`, which refuses a
   symlink in any component. Appends open through `safeopen.InRoot` and rotation renames through
   the pinned root; status publication uses the same pin. Witness:
   TestSERVICE500_LogDirectorySymlinkIsRefused.
2. **Final counters race the pipe readers (repaired).** The drain goroutines are now joined.
   After a proved retirement, and at shutdown before the logs close, `settleDrains` waits at most
   2s for them. It then closes a pipe still held open; its unread bytes stay uncounted, a recorded
   limit. Witness: TestSERVICE500_HelperDrainsSettleBeforeFinalCounters.
3. **Partial resume and a later retry.** The finding: resume resets helper debt before its control
   write, so a failed control write leaves an unremembered request R, and a retry of R after newer
   debt would reset that debt. Round 3 declined it, arguing that an unremembered id is simply a new
   resume of the current state. Codex round 4 rejected the decline: SERVICE500-003 requires resume
   to retain its original pre/post operation, reconcile partial resets and never erase later
   failure debt.

   Repaired. Under F, before any reset, resume durably writes `resume-operation.json`. It binds the
   request and its hash, the digest of the control it observed, its timestamp, and, per declared
   helper, the record digests before and after the reset. The journal is removed after the control
   and ledger writes.
   - A retry of the same request reconciles: each record must be at its Before digest (it is then
     reset) or its After digest (already reset). Any other record is newer debt, and the retry is
     refused with RESOURCE_COLLISION, keeping the debt.
   - A retry whose control changed since the journal was written is refused with RESOURCE_COLLISION.
   - A different resume that finds an unfinished journal (one whose request the ledger does not
     bind) first records that request in the control ledger under a SUPERSEDED marker hash. A later
     retry of it is then REQUEST_ID_CONFLICT and erases nothing.

   Witness: TestSERVICE500_ResumeJournalNeverErasesLaterHelperDebt. It covers same-request
   reconciliation, newer debt, changed control, and supersede followed by new debt, a stop and a
   retry. Each of the three refusals failed its leg when removed.

   Recorded limit: the superseded marker lives in the bounded ledger (the newest 256 requests). A
   retry arriving after its eviction is treated as a new request, the existing ledger limit for
   every control verb.

Codex round 5 traced the journal across every crash boundary and confirmed the debt-preservation
repair. It returned CHANGES_REQUIRED with one P2: the unfinished journal's request id was not
reserved across verbs, so a stop, install or uninstall could reuse it. Repaired:
`pendingResumeConflict` is now consulted, before any effect, by `controlReplay` (stop, resume) and
by the operation `journal` check (install, uninstall). Only the original resume, with its own hash,
may reuse the id; anything else is REQUEST_ID_CONFLICT. The witness test gained stop, uninstall and
install legs, and removing either check fails it.

## Owner questions

1. **Recovery of a held intent.** Recovering a helper intent left by a killed wrapper is manual, by
   removing the file. Should there be an explicit confirm verb, or cgroup-based recovery?
2. **Helper health signal.** Should a helper health signal be defined so that helper debt can reset
   after 600s healthy, as main debt does in the model?
3. **Excerpt exposure.** Should status expose the sanitized excerpt text, or keep withholding it?
4. **Import edge.** Is the decision 0397 two-file `internal/groupreap` edge acceptable as an agent
   decision?
5. **Darwin helpers.** Should Darwin helpers stay UNSUPPORTED, or wait for a descendant proof?
6. **Interrupted resume retries.** The fail-closed choice refuses a same-request retry once its
   control or a helper record has changed since the journal was written. The operator must then
   issue a new resume request. Should such a retry instead re-journal against the current state?

## Rollback

Revert this change. Before reverting, an operator uninstalls, or replaces without helpers, every
program that has helpers, so that no `helper-<H>.intent.json` remains. After the revert, the retained
`helper-*` files and `logs/` are inert.
