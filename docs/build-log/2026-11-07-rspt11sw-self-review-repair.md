# Swift terminal and ELF interpreter dependency repair

Date: 2026-11-07. Tickets: V1-5113 (Swift), V1-5106 (Linux tracing).
Intent: RSP-V0-003, RSP-V0-007, RSP-V0-021, RSP-V0-022.

Self-review identified failure loss across Swift retries and a missing intermediate
ELF interpreter symlink dependency. The Swift adapter now retains validated failed
terminals independently of report completeness and process lifecycle flags. The
native parser supplies failure callbacks only after validating the terminal and its
preceding protocol state. The existing platform Parse observation, errors and wire
remain unchanged; the adapter recovers failed identities even when Swift Testing
has not emitted any observation because runEnded is absent. Successful and skipped
results still require the existing completeness checks. Corrupt or truncated later
records cannot erase an earlier validated failure. Issues without a terminal,
unbound terminals and invalid identities/duplicate JSON keys do not create failures.

Decision: retain failures through a separate in-process parser callback rather than
change the frozen legacy observation semantics or duplicate native protocol parsing.
The core's existing sticky-failure reconciliation prevents a passing FULL retry
from erasing these results. TestRSPV0003SwiftIncompleteFailureSurvivesRetry exercises
both native runners through SelectionSession and independently validates the passing
retry fixture. Its 14 cases cover missing end, partial record, malformed record,
invalid UTF-8 suffix, timeout, interruption and overflow. The 12 initial cases failed
before the implementation change; all final cases pass. The platform helper has
positive and negative terminal validation tests.

ELF PT_INTERP paths now use the same closed symlink walker and lexical/resolved
filesystem matching checks as ordinary reads. TestRSPV0007InterpreterLinks validates
a minimal ELF fixture, retains interp and bridge for interp -> bridge -> loaderA,
retargets only bridge to loaderB and checks the resolved read. Cycles and an unknown
missing/../ suffix must mark evidence incomplete. The test compiles for Linux/amd64;
this Mac cannot execute it. The entire Darwin proctrace package passes (0.566s).

Whole-package tests ran for all three touched packages through script/heavy.sh,
with the owner's shared semaphore and empty HEAVY_REMOTE_HOST. The platform run
fails only TestAndroidInstalledAPKIdentity and TestAndroidTimeoutStopsDeviceProcesses
because descendant inspection is denied (operation not permitted). Swiftselection
fails the existing TestRSPV0005SwiftLLVMLineEvidence for both runners with
executed:present-with-reason; the same failures are retained in the before-repair
run. No existing assertions or fixtures were changed. Focused TestSwift platform
checks and all TestRSPV0003 Swift adapter checks pass, as do native platform/adapter
vet and Linux/amd64 proctrace vet. Acceptance remains blocked on those whole-package
failures and Linux execution; independent review belongs to the orchestrator.

Line-citation repin found no moved citations; line-citations-check passes.
changed-checks fails on existing aggregate reader inventory/scope drift outside
these repairs; its complete diagnostics are retained. No inventory was loosened.
Rollback is a revert of this repair; there is no stored-data migration.

Self-development: context and affected used the required immutable base
8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734. Affected reports scope UNKNOWN
(self-binary-stale, language/unowned frontier); selected package boundaries were
used with explicit whole-package execution. Existing pre-change capture is reused.
CEM rebind/check follows this source commit; no new pre-change capture or seal.
Official updater checks for Core and Tasks both retain freshness UNKNOWN after
sandbox lifecycle smoke failure; installed build 127 was preserved. Account usage
and runtime effort controls are NOT_OBSERVED; no account telemetry tool is exposed.
No additional model was started. Publication, Tasks writes, whole-repository gate,
release promotion and live Swift/LLVM qualification are outside this repair.

Raw logs and the cross-compiled test binary remain outside the repository under
/Users/russelllewis/lanes/rspt11sw-evidence. Log digests below bind the reported
results, not correctness or production qualification.

| Evidence file | SHA-256 |
|---|---|
| `repair-swift-before.log` | `99b40282a52dbf5f8de9e0a3c8bb4253fa937e1d5f48ed08a9e4d534e116211d` |
| `repair-swift-final.log` | `ffbcad7177bfd1afca7917992acd56cc103e9bd41c50e07540ca00499f9698be` |
| `repair-platform-final.log` | `b69ca6989503fcc37e753986fd6ef1994ddad8210989853b4d1ade991fd95331` |
| `repair-proctrace.log` | `3f48db2428cbb21d9c9697b3ea352a1fe7bbfcd3d9c5360b2517cb47d227796f` |
| `repair-proctrace-linux-compile.log` | `c22a4f69ada3c86f833aff2d8376b92df918cf6f45803b9aa6587bfdc44d5f61` |
| `repair-platform-swift.log` | `e4d6ca5092d4df25fe04d17cf6b164b99f725eb076b1e644ef5f9a095f0fff0b` |
| `repair-swift-terminal.log` | `cf6b2a56394813199c30da460e9a3a1838ff339a4abc97a5b229b2e90dd1c2b9` |
| `repair-changed-checks.log` | `ecf1d1d76eb28d75287917f3f28a1aab3e888da97326f3549692a18359ad4081` |
