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
  - **Close:** waits at most 2s and counts unwritten bytes as dropped.
  - **Counters:** published to `status.json` (4096B bound).
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

## Owner questions

1. **Recovery of a held intent.** Recovering a helper intent left by a killed wrapper is manual, by
   removing the file. Should there be an explicit confirm verb, or cgroup-based recovery?
2. **Helper health signal.** Should a helper health signal be defined so that helper debt can reset
   after 600s healthy, as main debt does in the model?
3. **Excerpt exposure.** Should status expose the sanitized excerpt text, or keep withholding it?
4. **Import edge.** Is the decision 0397 two-file `internal/groupreap` edge acceptable as an agent
   decision?
5. **Darwin helpers.** Should Darwin helpers stay UNSUPPORTED, or wait for a descendant proof?

## Rollback

Revert this change. Before reverting, an operator uninstalls, or replaces without helpers, every
program that has helpers, so that no `helper-<H>.intent.json` remains. After the revert, the retained
`helper-*` files and `logs/` are inert.
