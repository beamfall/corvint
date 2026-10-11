# V1-1128: proposed bounded hook background index refresh

Intent is proposed, pending owner acceptance (V1-1128): IDX-SNAP-V0-031 through
IDX-SNAP-V0-036 and AHI-053. Delivery is experimental. No decision file or acceptance claim.

Plan: specify first, retain the existing writer, transfer a shared-store nonblocking flock to
one detached finite child, then exercise real fixture commits, concurrency and interruption.
One stable lock inode avoids unlink/recreate races and bounds all linked-worktree refreshes.
Child CPU priority is at least +10, Go parallelism one, cancellation 55 seconds, hard stop 60.
Existing ten-minute temporary cleanup and atomic snapshot publication remain authoritative.
Rollback: CORVINT_INDEX_AUTO_REFRESH=0; revert restores manual-only remediation.

Use existing session model/effort: process ownership and concurrency require careful reasoning;
live model controls and account usage telemetry are NOT_OBSERVED. No additional agent fanout
except the required independent review. No benchmark campaign or repository-wide gate.

Baseline: origin/main 97c3a4bd03bb97b29e7378dfa116ffde2280cadb, initially clean worktree.
Installed Core 1.0.0-rc.3 build 407; Tasks 0.0.0-tcp01-unverified+build.407.
Official update checks fail version smoke with descendant snapshot unavailable; available
versions and freshness remain UNKNOWN. Verification was not bypassed.
Corvint index refresh and make dogfood-change cannot write the external Git common directory
under this sandbox. Final committed CEM/OCM/seal are NOT_PRODUCED: this builder may not commit.
The historical Python test_harness_claude.py reference has no file in this checkout.
V1-1120 HLQ load observations remain a qualification limit, not a runtime threshold.

Independent review found two issues: background-status notices omitted the accepted AHI-031
cause sentence (restored), and group-wide KILL tests did not exercise worker-PID-only SIGKILL.
The latter is an explicit portable-process limit: a killed worker cannot execute its timer or
defers. Added a leader-only KILL witness for lock recovery/no partial publication; the test owns
cleanup of the surviving group. Normal exit, INT/TERM and deadline retire the whole group;
operators must target the group for forced KILL. No supervisor was added to the chosen design.

Verification diagnostic: the initial race matrix failed because this sandbox denies
Setpriority(PRIO_PROCESS, 0, 10) with EPERM (also reproduced in an isolated syscall probe).
Production refuses the child build on this failure. The exec'd test helper substitutes only
that denied scheduler syscall, logs NOT_OBSERVED, and verifies the remaining real writer,
lock and process lifecycle. Real lowered-priority/live-host qualification remains NOT_OBSERVED.

The first interruption injection paused a cat-file used after publication, so its fresh result
was a valid completed snapshot, not partial publication. The corrected fixture pauses the
opening rev-parse and asserts misses before injection and after readiness. TERM now kills the
owned group immediately rather than waiting for cancellable build cleanup: writer eviction can
run outside that context. The independent reviewer inspected this repair with no new finding.

The broad JavaScript harness has two V1-0371 failures in unchanged sibling-reap tests that use
ps as a zombie witness; ps itself is denied by this sandbox. Preserve these failures rather than
weakening the harness. Ticket filing was not attempted because this task forbids Tasks writes.

## Final local evidence

- Final requested focused race command (with `-v` for captured diagnostics):
  `GOTOOLCHAIN=local TMPDIR=/private/tmp/claude-501/gotmp GOMAXPROCS=3 go test -v -count=1 -race -timeout 30m ./cmd/corvint -run 'Snapshot|Stale|Refresh|HostAdapter|ClaudeAdapter|CodexAdapter|FailOpen|Stability|DogfoodEvent|ClaudeNative'`.
  Exit 1, 346.511 seconds. All seven SnapshotRefresh test groups PASS, including the helper;
  TestClaudeNativeDogfoodLifecycle, TestAHI044HookAdaptersFailOpen and the existing snapshot
  equivalence test PASS. No race report. Only TestHostAdapterJavaScriptHosts fails.
- That harness has 70/73 JavaScript cases passing. Two unchanged V1-0371 sibling-reap cases
  depend on the denied ps witness (OpenCode: children did not exit; Gemini: code 0 vs 143).
  A third unchanged case, "V1-0371 Gemini exits after reporting a leader whose group kill
  failed", reports ENOENT for leader.pid. It passed in the earlier run; its cause remains
  UNKNOWN. These failures are retained, not skipped or fixed outside the requested slice.
- `GOTOOLCHAIN=local TMPDIR=/private/tmp/claude-501/gotmp GOMAXPROCS=3 go vet ./cmd/corvint`: PASS.
- Requested doc targets: spec-requirements-check, requirement-definitions-check,
  traceability-tests-check, decision-numbers-check, line-citations-check,
  error-code-ownership-check, unbounded-readers-check, use-case-receipts-check and
  use-case-receipts-test: command PASS. A scratch mktemp adapter supplied an explicit writable
  temporary directory after Darwin mktemp ignored TMPDIR for template-free calls. No Git-index
  substitution or staging was used. Index-based checks did not see unstaged changes/new files;
  their success does NOT qualify those bytes. Worktree clause/readback and diff whitespace pass.
- `make -s spec-requirements`: ran successfully against the existing Git index. REQUIREMENTS.tsv
  is consequently unchanged and does not yet contain the proposed IDs. After staging specs,
  the integrator must regenerate it and rerun the doc targets on the staged change.
- `script/repin-use-case-receipts.sh`: repinned the three main.go implementation receipts and
  their ledger. `sh script/repin-use-case-receipts.sh --check`: PASS, all 18 receipts current.
- Independent read-only review: original findings repaired; final process/lock/test deltas
  reviewed with no new findings. Ordinary test opt-out preserves the exact native-hook environment.
- Corvint context retained missing critical slots for index-snapshot/local-completion specs;
  relevant original sources were read directly. An early affected run refused concurrent tree
  drift; subsequent runs succeeded with broad Go selection plus language/frontier and unowned
  documentation/receipt-path unknowns. The owner's narrower test instruction governs; no
  make gate, go test ./..., scoring evaluation, commit, publication or Tasks write was performed.

## Remaining closeout

Owner acceptance of IDX-SNAP-V0-031 through IDX-SNAP-V0-036 and AHI-053 is pending. Real nice
priority and live-host/Linux qualification are NOT_OBSERVED; this sandbox denies Setpriority,
so production refuses to build here rather than silently running at full priority. Forced
SIGKILL of only the worker PID leaves the documented descendant residual; signal its group.
The three JavaScript failures need an environment-qualified rerun/diagnosis. Staging-dependent
requirements/doc verification, committed CEM/OCM/seal, integration and native ticket completion
remain for the owning integrator. All test-owned refresh groups were explicitly retired/reaped.

Raw local logs: /tmp/v1-1128-focused-final.log, /tmp/v1-1128-doc-gates-final.log,
/tmp/v1-1128-vet-final.log, /tmp/v1-1128-repin-check.log. Earlier failed experiments remain in
/tmp/v1-1128-refresh-tests.log, /tmp/v1-1128-diagnostic.log and /tmp/v1-1128-refresh-final.log;
the corrected targeted interruption run is /tmp/v1-1128-interrupt-fixed.log.

