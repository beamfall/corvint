# Operator update V0

Owner: Russell Lewis
Date: 2026-09-29
Requirement prefix: `UPD-V0`
Intent status: proposed technical contract under owner-requested V1-0505 outcome
Delivery status: experimental
Authoritative inputs: owner instructions of 2026-09-29 to use current Corvint/Core/Tasks machine-wide
and implement V1-0505; `../../AGENTS.md` invariants 4 and 7; `../INSTALL.md` upgrade/rollback contract.

## Agent digest
- Claim: An optional operator updater checks Core and Tasks release channels and preserves verified rollback bytes without adding network access to Core.
- Status: proposed technical contract under owner-requested V1-0505 outcome / experimental
- Exists: separate `../../cmd/corvint-update` and `../../internal/update`; focused regression and actual macOS evidence are required before delivery.
- Blocked on: final focused checks, independent review, actual published-archive lifecycle and native completion. Publisher identity and full Tasks qualification remain NOT_VERIFIED/NOT_OBSERVED.
- Read next: Requirements; Trust boundary and limits; Acceptance evidence. A Core switch refreshes index snapshots of named and indexed enclosing checkouts (`UPD-V0-008`). `check` states the installed-to-available relation (`UPD-V0-009`). An unnumbered local install (build 0 or none) still checks, as `installedBuildStatus` `UNNUMBERED`, and `apply` refuses to replace it (`UPD-V0-010`). A stop is observed before switch or rollback, and the old build restarts under a cleanup context (`UPD-V0-015`); check and apply share the Tasks report binding (`UPD-V0-016`), read per platform with a legacy fallback (`UPD-V0-017`).

## Human intent and current state

The owner asks all machine work to use Corvint and Tasks wherever possible, retain observed gaps as
native tickets, and use latest compatible versions. On 2026-09-29 the machine had Core rc.1 build163
and Tasks build163; official Tasks developer release `tasks-dev-20260929.2` provides build202.
The installation guide requires manual checksums and versioned rollback. V1-0505 requests a reusable
optional updater. The technical contract is an experimental proposal; observed implementation does
not accept new product authority or promote a release.

User/job: a local operator checks available releases, activates one checksum-verified compatible
component, and can restore exact previous executable bytes without itself touching any repository or
store; after a Core switch it asks Core's own `index` verb to refresh derived snapshots (`UPD-V0-008`).
The baseline is manual archive verification and copying binaries, with external agent scheduling.

## Requirements

- `UPD-V0-001`: `check` MUST require explicit network opt-in, distinguish Core `v*` and standalone
  Tasks `tasks-dev-*` release channels, select host-compatible assets independently, and report
  installed/available identity and freshness UNKNOWN when network, installed-build identity,
  assets or complete bounded discovery are unavailable. Check MUST write no files.
- `UPD-V0-002`: `apply` MUST verify the release archive checksum before extraction, verify internal
  checksum inventories before execution, retain the supplied archive notices, source/manifests when provided, and qualification evidence,
  and refuse missing/inconsistent host or build identity. Checksums MUST NOT imply publisher identity
  or full Tasks runtime qualification. Core's internal checksum list covers the binary alone;
  its current archive provides no separate source archive or release manifest, which MUST remain
  explicitly NOT_PROVIDED rather than synthesized.
- `UPD-V0-003`: Downloads, release discovery, extracted bytes/members and subprocess output/time MUST
  be bounded. Extraction MUST refuse unsafe paths (since V1-3179, any member that `filepath.IsLocal`
  rejects on the running platform, joined through the shared `internal/archivepath` check), aliases, duplicate files and all link/special
  entries. Managed executable/state paths MUST refuse symlink aliases and overlap. Mutation MUST
  serialize by canonical destination independently of state-directory choice.
- `UPD-V0-004`: Apply MUST mutate only one selected component, refuse downgrades/unknown builds,
  smoke-check the verified candidate, preserve prior executable bytes and a bound prepared receipt,
  revalidate the current destination just before atomic activation, and leave it unchanged on any
  earlier error or cancellation. Same-byte current installation is a no-op after final destination digest revalidation;
  redundant staging MUST be removed so scheduled unchanged checks do not accumulate downloads.
- `UPD-V0-005`: Offline `rollback` MUST use a locally retained transaction bound to component,
  canonical destination and both digests; verify saved bytes and refuse destination drift. A prepared
  receipt MUST permit recovery after interruption between activation and any subsequent bookkeeping.
  Rollback MUST preserve both transaction evidence and repository/store data.
- `UPD-V0-006`: Owned executable probes MUST isolate HOME/cwd, bound output/time, and retire ordinary
  descendants on timeout or interruption. Unobserved cleanup MUST fail the probe. Operators MUST see
  qualification limits, partial failures and known durability exclusions in command output/docs.
- `UPD-V0-007`: (accepted by decision 0440; V1-0929) The state directory MUST
  stay bounded to what rollback needs. `apply` and `rollback` MUST hold an exclusive lock on the
  state directory for the whole run; under it they first remove every updater transaction
  directory (named `transaction-<digits>`, as `os.MkdirTemp` creates it) that has no `receipt.json`
  and was last modified more than 30 minutes ago: an apply killed before its receipt, which rollback
  can never use for any component, and which no live run can still own because a run is bounded at
  five minutes (this also covers an older updater without the state lock sharing the directory). A
  younger one is kept and named in `left`, and is swept by a later run. The age is the directory's
  wall-clock modification time: a forward clock jump of more than 30 minutes during a live pre-lock
  updater's staging can sweep its directory, and that apply then fails before its receipt and
  activation with its destination unchanged. After
  a successful activation, apply MUST remove the committed transaction's `archive.tar.gz`,
  `smoke-home` and extracted candidate executable (whose bytes are now the installed ones), keeping
  `receipt.json`, `previous`, the release checksums and metadata, qualification evidence, and the
  retained notices, manifests and source; it then removes every other transaction whose receipt
  names the same component and canonical destination. Each removed path is named in the result's
  `removed`; a transaction entry that is a link or not a directory, has an unreadable or malformed
  receipt, or fails to remove is kept and named in `left` as `path: reason`. Transactions with a
  receipt for another component or destination, and entries not named like an updater transaction,
  are untouched. `check` writes nothing and
  takes no state lock. Rollback depth is therefore one step: the latest committed transaction
  restores the previous executable; an earlier one is gone once superseded. A transaction left by a
  failed activation after its receipt was written is kept until the next successful apply of that
  component and destination. Falsifier: after an apply, a state directory holding a superseded
  transaction of the same component and destination, an archive, smoke home or candidate copy of
  the committed one, or a receipt-less updater transaction last modified more than 30 minutes before
  the run; a removal not named in `removed`; or a rollback
  after pruning that does not restore the exact previous digest. Rollback of this requirement:
  revert `sweepIncomplete`, `retainCommitted`, the state lock and the two result fields; transactions
  then accumulate as before and the operator prunes them by hand (older ones are not needed for the
  latest rollback).
- `UPD-V0-008`: (proposed (V1-1078)) An index snapshot is keyed by the Core executable's digest
  (`IDX-SNAP-V0-003`), so every Core switch leaves each checkout without a matching snapshot, and the
  host hooks then degrade with `dogfood-event-index-snapshot-stale` because they may not rebuild it
  (`IDX-SNAP-V0-012`). After a successful Core `apply` that activates a new binary, and after a
  successful Core `rollback`, the updater MUST run the switched binary's
  `corvint --root ROOT index --if-stale` once per checkout, in the foreground, as an owned process
  group bounded at two minutes, with the operator's environment, and cancelled by the updater's
  interruption, while it still holds the binary-directory lock, so a concurrent updater cannot
  replace that binary between refreshes. The checkouts are each `--refresh-index ROOT` the operator named, and the checkout
  enclosing the working directory (its nearest ancestor with a `.git` entry) only when it already
  has a snapshot store, shared or worktree fallback (`IDX-SNAP-V0-025`); one resolved path is
  refreshed once. The result's `indexRefresh` names each checkout with `state` `built`, `fresh`,
  `skipped` (an enclosing checkout without a store) or `failed` with a `reason`. A failed or
  interrupted refresh MUST NOT fail or reverse the switch. The updater writes nothing in any
  checkout itself: every write is Core's own `index` writer, under that verb's derived-state,
  symlink-refusal and eviction rules (`IDX-SNAP-V0-005`, `IDX-SNAP-V0-007`, `IDX-SNAP-V0-011`).
  An unchanged `apply`, `check`, and any Tasks switch run no refresh. Failure modes: a checkout the
  operator did not name and did not run from keeps its stale snapshot until its own explicit
  `index --if-stale`; a binary replaced outside the updater (a local build copied into place)
  refreshes nothing; a refresh longer than its bound is killed and reported `failed`, and the
  hook outcome stays the existing stale-snapshot notice. Falsifier: a Core switch whose result
  lacks an outcome for a named or indexed enclosing checkout; a refresh of an unindexed enclosing
  checkout; a refresh after `check`, an unchanged apply or a Tasks switch; or a failed refresh that
  changes the exit status or the installed binary. Rollback of this requirement: remove the
  refresh call, the `--refresh-index` flag and the `indexRefresh` field; the operator runs
  `corvint --root ROOT index --if-stale` by hand after each switch, as before.
- `UPD-V0-009`: (proposed (V1-0639)) Every `check` result that observes a release MUST state
  `relation` as `CURRENT`, `UPDATE_AVAILABLE`, `LOCAL_NEWER` or `UNKNOWN`, with `relationBasis`
  naming the compared field, so a caller never compares version strings itself. Tasks compares
  `installedBuild` with the integer `build` declared by the release's Tasks report for the host
  platform (`UPD-V0-017`; reported as `availableBuild`), accepted only when its `profile` is `corvint-tasks-archive/0`, its
  `target` is the host platform and its `archiveSha256` equals the `SHA256SUMS` digest of the
  platform archive; basis `build`. Core release metadata declares no build number, so Core compares
  the whole-field semantic version (SemVer 2.0.0, no leading zeros) in the installed version label
  with the release tag by precedence, ignoring build metadata; basis `version`. `LOCAL_NEWER`
  carries a `relationReason` naming the would-be downgrade: on basis `build` one that `apply`
  refuses (`UPD-V0-004`); on basis `version` one that `apply` refuses only when the verified
  candidate build is lower. A missing, unreadable, oversized (over the metadata bound) or unbound
  declaration, or a label or tag that is not a semantic version, yields `UNKNOWN` with a
  `relationReason` and no basis. `check` still writes no files and the relation never gates
  `apply`, which keeps deciding from the verified candidate. Failure modes: a Tasks release whose
  declaration disagrees with the archive's internal build is caught only by `apply`'s verification;
  two Core builds of one version label report `CURRENT`. Falsifier: a check result with an observed
  release and no relation, an `UNKNOWN` without a reason, a relation computed from an unbound
  declaration or a malformed version, or a basis-`build` `LOCAL_NEWER` for a release `apply` would
  install. Rollback of this requirement:
  remove `relate` and the four result fields; callers compare `installedVersion` and `availableTag`
  by hand, as before.
- `UPD-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1364) An installed
  version label that passes the component identity check and carries build 0 or no build marker
  at all (a marker `build` followed by `.`, space, `=` or `:` that is not read as digits stays
  `invalid numeric build`), as a local
  build from source reports (`Corvint 1.0.0-rc.3 (build 0)`, since PUB-V0-053 `Corvint 1.0.0-rc.3.dev
  (commit <sha>)`, `0.0.0-tcp01-unverified+build.0`), is an
  unnumbered build, not an unknown identity. `check` MUST still report the available release
  (`availableTag`, and `availableBuild` when a Tasks declaration is bound as in `UPD-V0-009`) with
  freshness `RELEASE_OBSERVED`, set `installedBuildStatus` to `UNNUMBERED`, and state `relation`
  `UNKNOWN` with no basis and a `relationReason` saying the installed build is unnumbered or local so
  its freshness cannot be ordered against releases; it MUST NOT exit with only an opaque build error.
  This narrows `UPD-V0-001`'s UNKNOWN freshness for an unavailable installed-build identity to labels
  that fail the component identity check. `apply` MUST refuse to replace an unnumbered installed build
  after release discovery and before any download, naming the reason, leave the destination
  unchanged and create no transaction (the lock-held sweep of stale receipt-less transactions,
  `UPD-V0-007`, still runs as for every apply); no flag overrides it, because force-downgrade is a non-goal, so the
  operator who chooses to replace a local build does so by hand. A candidate whose own version label
  is unnumbered stays an unknown build that `apply` refuses (`UPD-V0-004`). `rollback` is unchanged.
  Failure modes: a numbered build that a local source build reuses is still compared as that number;
  an unnumbered build newer than every release keeps reporting `UNKNOWN` until it is replaced.
  Falsifier: a check of an unnumbered install that exits without `availableTag`, reports a relation
  other than `UNKNOWN` or lacks `installedBuildStatus` `UNNUMBERED`; or an apply that replaces an
  unnumbered install. Rollback of this requirement: restore the build-0 refusal in `version` and
  remove `installedBuildStatus`; check then exits with `invalid numeric build` again.
- `UPD-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1599)
  `apply --restart-service PROGRAM` MUST drive the operator's service around the switch through
  `PROGRAM status|stop|start|live-attempts`, each a bounded owned process (30 s per step) run with
  `CORVINT_UPDATE_COMPONENT` and `CORVINT_UPDATE_BINARY` (the managed destination) in its environment.
  `PROGRAM` MUST be a clean absolute path to an executable regular file; the service and hook options
  are refused by `check` and `rollback`. `status` exits 0 when the service is running and 3 (the LSB
  code) when it is stopped; any other exit, or an unobserved exit, is unknown and refuses before any
  stop or switch (`FAILED`, destination unchanged). The run reads `status` before the switch: a
  service that was not running is switched under and left stopped (`NOT_RUNNING_BEFORE`); a running one is stopped
  before activation and started on the new build afterwards, and `start` succeeds only when `status`
  reports running within `--start-timeout` (default 30 s, at most 2 min). The result carries
  `service` with `program`, `mode`, `wasRunning`, `liveAttempts` and a final `state` (`RESUMED`,
  `NOT_RUNNING_BEFORE`, `RESUMED_OLD`, `ROLLED_BACK` or `FAILED`) and `switched`. A failure before
  activation (stop, drain or a later preparation refusal) starts the old build again (`RESUMED_OLD`)
  with the destination unchanged. Failure modes: a control program that reports running before its
  service accepts work is trusted; a service that cannot be restarted on the old build ends `FAILED`
  and names it. Falsifier: an apply that switches while the service still runs, or leaves a
  previously running service stopped without reporting `FAILED`. Rollback of this requirement:
  remove the option; apply then switches without touching any service, as before.
- `UPD-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1599) Each
  `--post-switch CMD` (at most 8, each at most 4096 bytes) MUST run through `/bin/sh -c` only after a
  switch that stands, in order, in the working directory (or the install directory), with
  `CORVINT_UPDATE_COMPONENT`, `CORVINT_UPDATE_BINARY` and `CORVINT_UPDATE_BUILD`, as an owned process
  bounded by `--hook-timeout` (default 2 min, at most 10 min) and retried up to `--hook-attempts`
  runs (default 3, at most 5). The result lists one `hooks` entry per command with `command`,
  `attempts`, `exitStatus` (-1 when no exit was observed), `state` (`SUCCEEDED` or `FAILED`), the last
  2048 bytes of its combined output and a reason. A failed hook MUST be reported (exit 1, with an
  error saying the switch to the new build stands) and MUST NOT reverse the switch: `switched` stays
  true and the installed build is the new one. Every hook runs even after an earlier one fails.
  Failure modes: a hook that is not idempotent is retried anyway; output beyond the tail is dropped.
  Falsifier: a hook failure that rolls the binary back, hides its retry count, or reports success.
  Rollback of this requirement: remove the option.
- `UPD-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1599) When a
  service that was running does not report running on the new build within the start bound, apply
  MUST stop it, restore the previous binary through the same verified rollback as `UPD-V0-005`, start
  the service on the previous build, run no hooks, and report action `rolled-back`, `switched` false,
  the previous build as installed, service state `ROLLED_BACK` (or `FAILED` when the old build does
  not start or the rollback itself fails, the latter leaving `switched` true) and an error. No
  rollback is attempted when the new build's service cannot be stopped, or in `DRAIN` mode (Tasks
  formats differ or are unknown): the new build stays installed, `switched` is true, the state is
  `FAILED` and the error says so. The stop, the restore and the old-build start run under a fresh
  cleanup context that survives the run's cancellation, bounded by the start bound plus two steps
  (V1-3033), so a run cancelled during the rollback still restores the previous build and keeps the
  start failure in the reason. Failure modes: a service that starts and then crashes after the bound is not observed. Falsifier: a failed
  start that leaves the new binary installed, leaves the service stopped while reporting
  `ROLLED_BACK`, or runs hooks. Rollback of this requirement: remove the automatic rollback; the
  operator runs `rollback` by hand.
- `UPD-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1599) Live
  attempts MUST follow the Tasks compatibility rules (`CAL-V0-130`, `CAL-V0-131`): for Tasks the
  installed and candidate builds' sorted `formats` from `version` are compared; equal, non-empty sets
  run in `ADOPT` mode (stop, switch, start; the restarted service adopts live attempts and
  `liveAttempts` records how many there were); different or unreadable sets run in `DRAIN` mode,
  where the update waits after `stop` until `live-attempts` prints 0, within `--drain-timeout`
  (default 10 min, at most 15 min). `DRAIN` mode waits even when the service was already stopped,
  since stopped dispatchers can leave live attempts. A drain that does not reach zero, or a count
  that cannot be read, refuses before activation, starts the old build again when it was running
  and leaves the destination unchanged. Core
  runs in `RESTART` mode. The run budget grows by the drain bound, four start bounds and six steps
  (`UPD-V0-015`).
  Failure modes: a control program that reports zero while attempts still run is trusted.
  Falsifier: a `DRAIN` mode switch while `live-attempts` is non-zero, or an `ADOPT` choice for
  different format sets. Rollback of this requirement: drain always.

- `UPD-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3011, V1-3012)
  After `stop`, apply MUST poll `status` until it reports `STOPPED`, within the start bound, before
  draining, activation or the automatic rollback (`UPD-V0-013`). A service still `RUNNING` at the
  bound, or a status that is neither running nor stopped, refuses: before activation the
  destination is unchanged and the old build is started again; before rollback the new build stays
  installed with `FAILED` and no rollback is attempted. Every restart of the old build after a
  refusal (a stop that is not observed, a drain failure, a cancelled or failed run before the
  switch, and the start after a rollback) MUST run under a fresh cleanup context that survives the
  run's cancellation, bounded by the start bound plus two steps, so a cancelled operation still
  resumes the old service. A restart that fails is retained: the service state is `FAILED` and the
  reason keeps the original refusal followed by `old build restart: <cause>`. The run budget is the
  drain bound, four start bounds and six steps. Failure modes: a service that reports `STOPPED`
  while its processes still exit is trusted. Falsifier: activation or rollback while `status` still
  reports running or unknown; a cancelled run that leaves a previously running service stopped
  without attempting the restart; or a failed restart reported as `RESUMED_OLD` or with the
  original reason dropped. Rollback of this requirement: remove the status poll and the cleanup
  context; stop is trusted on exit 0 and a cancelled run leaves the service stopped, as before.
- `UPD-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3013) `check` and
  `apply` MUST validate the Tasks report through one shared check: profile
  `corvint-tasks-archive/0`, `target` the host platform, a positive integer `build`, and
  `archiveSha256` equal to the digest of the platform archive (the `SHA256SUMS` digest for `check`,
  the downloaded and verified archive for `apply`). `apply` refuses a mismatch before activation
  with the destination unchanged. Failure modes: a publisher that signs a consistent but wrong
  report and archive pair is not detected (publisher authentication is out of scope). Falsifier:
  an `apply` that installs from an archive whose digest differs from the report's
  `archiveSha256`, or a report `check` accepts and `apply` refuses for the same reason (or the
  reverse). Rollback of this requirement: drop the digest comparison from `apply`; `check` keeps
  its own binding, as before.
- `UPD-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3014) A Tasks
  release MAY carry one report per platform archive, as the asset
  `build-verification_<goos>_<goarch>.json` with the unchanged `corvint-tasks-archive/0` content.
  `check` and `apply` MUST read the host platform's per-platform report when the release lists it,
  and otherwise the legacy single `build-verification.json`, which qualifies only the platform its
  `target` names. `RunTasksArchive` MUST retain the per-platform report beside the archive and
  `SHA256SUMS`, bound to the archive digest and without the host-local `archivePath`; the
  `SHA256SUMS` bytes are unchanged. Failure modes: a release whose legacy report names the other
  platform and that lacks a per-platform report for this one stays `UNKNOWN` and refuses to apply;
  a publisher who uploads only the legacy asset qualifies one platform. Falsifier: a release with
  per-platform reports for both archives that fails to qualify either platform; a legacy report
  that qualifies a platform it does not name; or a producer bundle without the per-platform report.
  Rollback of this requirement: stop publishing and reading the per-platform asset; releases then
  qualify only the platform named by the legacy report, as before.

## Non-goals

No Core network dependency, daemon, account, silent background service, mutable repository index,
package manager, publisher authentication, dirty-source installation, configuration rewrite,
force-downgrade, store migration or release publication. The command is a separately built optional
operator tool. Scheduling belongs to the caller; it does not change project authority.
The retention bound (`UPD-V0-007`) is not multi-step rollback history, a generic state-directory
cleaner, or a garbage collector for other components' transactions; it never removes the installed
destination or anything outside `transaction-*` directories of the state directory.
The snapshot refresh (`UPD-V0-008`) is not a repository scanner, a background or scheduled refresh,
or a hook-time rebuild: it touches only checkouts named or enclosing the run, and only through
Core's explicit `index` verb.
The service restart (`UPD-V0-011` to `UPD-V0-014`) is not a service manager: the operator supplies
the control program, `check` and `rollback` never touch a service, and no launchd, systemd or Tasks
`service` verb is bound in; a live platform service remains NOT_RUN until observed.

## Trust boundary and limits

Only official public GitHub release endpoints and their bounded HTTPS asset redirects are used.
Explicit network opt-in grants downloading, not execution of unchecked bytes. Verified executable
probes run only after complete archive/internal checksum and platform validation. The threat boundary
is an operator-owned local install/state tree with cooperating concurrent updater processes; another
same-user malicious writer, mutable parent replacement and publisher compromise are not contained.
Symlink/overlap checks, destination-scoped locking and digest revalidation refuse detectable drift.

Release inventory: at most five pages of 100 records; exhausting that window retains UNKNOWN.
Metadata/checksums: 2 MiB per response. Archives: 120 MiB compressed; extraction admits at most 4096 members and 300 MiB total
unpacked bytes. Network and probes have timeouts and cancellation. Exact code
limits are retained in the implementation and focused tests. Requests do not forward agent secrets.

Atomic activation and prepared receipts permit ordinary process-interruption recovery. This is not
power-loss durability: filesystem/hardware failures can require operator repair using retained
copies. Without `--restart-service` the updater does not stop active agents/providers; already running
executables continue, and callers restart them when the applicable host integration requires it.
With it, only the operator's control program is run (`UPD-V0-011`).

## Acceptance evidence

| Requirement | Implementation | Focused evidence |
|---|---|---|
| UPD-V0-001 | `internal/update`, `cmd/corvint-update` | `TestUPDV0001OfflineReadOnlyAndChannels`, `TestUPDV0001PaginationCapUnknown` and `TestUPDV0001TasksInstalledVersion`: channels, offline/unknown, read-only, Tasks version string from its command result |
| UPD-V0-002 | archive/identity verification | `TestUPDV0002ChecksumsAndArchive`, `TestUPDV0002PlatformAndQualificationIdentity`, `TestUPDV0002InternalTamperWithValidOuterChecksum`: outer/internal checksums, host, notices, evidence |
| UPD-V0-003 | bounded transport, paths and destination lock | `TestUPDV0003ArchivePathsAndLocks`: caps, aliases, traversal, concurrent state roots; `TestV13179ExtractConfinesMembersToDest` (V1-3179 shared join) |
| UPD-V0-004 | candidate preparation and activation | `TestUPDV0004ActivationDowngradeCancelRace`, `TestUPDV0004UnchangedCleanupAndDestinationDrift`, `TestUPDV0004PartialArchiveCancellation`: no-op, downgrade, race and partial-download cancellation |
| UPD-V0-005 | bound prepared receipts and rollback | `TestUPDV0005ApplyRollbackAndPreparedReceipt` and `TestUPDV0005PreparedReceiptInterruptionRecovery`: exact restored digest, interrupted prepared state and drift |
| UPD-V0-007 | `sweepIncomplete`, `retainCommitted`, state lock | `TestUPDV0007RetentionBoundInterruptedSweepAndRollback`: three padded applies keep one transaction's previous bytes (state constant instead of growing per apply), exact `removed`/`left`, other component kept, malformed receipt left and named, a fresh receipt-less transaction and a `transaction-notes` operator directory kept, a stale interrupted (receipt-less) transaction swept by the next rollback, rollback after pruning restores the exact previous digest, a held state lock refuses the run; `TestUPDV0005ApplyRollbackAndPreparedReceipt` tampers the retained transaction |
| UPD-V0-008 | `refreshIndexes`, `--refresh-index` | `TestUPDV0008CoreSwitchRefreshesIndexedCheckouts`: an apply and a rollback refresh a named root once when it also encloses the working directory; outcomes `failed` (exit status and stderr line), `built`, `fresh` in order, an unindexed enclosing checkout `skipped` without a run, no outcome outside any checkout, exact argv, no refresh after an unchanged apply or a successful Tasks rollback; live: the installed Core build 404 reported `built` then `fresh` on a scratch indexed repository (2026-10-09) |
| UPD-V0-009 | `relate`, `declaredTasksBuild`, `compareSemver` | `TestUPDV0009CheckStatesRelation`: Core `CURRENT`, `LOCAL_NEWER`, `UPDATE_AVAILABLE` (including a prerelease below its release, numeric prerelease order beyond machine integers, and build metadata ignored) and `UNKNOWN` for a non-semantic tag, leading zeros or a malformed installed label; Tasks `LOCAL_NEWER` with the downgrade reason (declared 202 against installed 407), `CURRENT`, `UPDATE_AVAILABLE`, and `UNKNOWN` for an unbound or missing declaration; no state written |
| UPD-V0-010 | `version`, `errUnnumbered`, `relate`, apply refusal | `TestUPDV0010UnnumberedLocalBuild`: Core and Tasks installs with build 0 and with no build number check as `UNNUMBERED` / `UNKNOWN` with the available tag (and Tasks declared build 500), write no state, and apply refuses with the destination unchanged and no transaction created; a `build -1` label stays `invalid numeric build`; a build-0 candidate is refused as an unknown candidate build; live (2026-10-14): installed Core `1.0.0-rc.3 (build 0)` and Tasks `+build.0` reported `v1.0.0-rc.2` and `tasks-dev-20260929.2` (build 202) |
| UPD-V0-011 | `validateService`, `service`, `quiesce`, apply restart | `TestUPDV0011ServiceRestartsAroundSwitch`: stop on build 162 before start on build 163 with component and binary in the environment, `RESUMED`; a stopped service switched under and not started; check and relative-program refusals; hook bound refusal; `TestUPDV0014StoppedDispatcherStillDrainsAndUnknownStatusRefuses`: status exit 2 refuses before stop or switch |
| UPD-V0-012 | `runHooks` | `TestUPDV0012PostSwitchHooksReportedAndSwitchStands`: four hooks (success, success on the third attempt, exit 7 three times, timeout with exit -1) reported with attempts and output tails; error says the switch stands; new build stays installed |
| UPD-V0-013 | apply rollback on failed start | `TestUPDV0013RollbackWhenServiceDoesNotStart`: build 163 never reports running; previous digest restored, service running on build 162, action `rolled-back`, no hooks run; `TestUPDV0013NoRollbackWithoutStopOrAcrossFormats`: a new build that cannot be stopped stays installed with `FAILED`, and `DRAIN` refuses rollback; `TestUPDV0013CancelledRollbackStillRestores`: a run cancelled as the rollback begins still restores build 162, starts the service on it and reports `ROLLED_BACK` with the start failure |
| UPD-V0-014 | `serviceMode`, `tasksFormats`, `quiesce` drain | `TestUPDV0014LiveAttemptsAdoptedOrDrained`: mode table, sorted formats and unreadable output; adopt stops with two live attempts; drain waits to zero; a drain past the bound resumes the old build and refuses; `TestUPDV0014StoppedDispatcherStillDrainsAndUnknownStatusRefuses`: a stopped service still drains |
| UPD-V0-015 | `service.stopped`, `service.stop`, `service.restartOld`, `quiesce`, `rollbackPermitted` | `TestUPDV0015CancelledRunRestartsOldService`: a run cancelled just before activation resumes the old build (`RESUMED_OLD`), and a failing restart is `FAILED` with `old build restart` after the original refusal; `TestUPDV0015CancelledDrainRestartsOldService`: a cancelled drain resumes the old build; `TestUPDV0015StopObservedBeforeSwitch`: a slow stop is polled until `STOPPED` before the switch, and a service that never stops or whose status breaks after stop refuses with the destination unchanged and the old build resumed; `TestUPDV0015StopObservedBeforeRollback`: a new build that never stops stays installed with `FAILED`, a slow one is rolled back |
| UPD-V0-016 | `tasksDeclaredBuild`, `declaredTasksBuild`, `verifyQualification` | `TestUPDV0016ApplyBindsReportToArchiveDigest`: a report naming another archive's digest is `UNKNOWN` for check and refused by apply with the destination unchanged; the bound report applies |
| UPD-V0-017 | `tasksDeclaration`, `tasksReportAsset`, `RunTasksArchive` | `TestUPDV0017PerPlatformTasksReports`: a release with both archives and per-platform reports, the same beside a legacy report naming the other platform, and the legacy single-platform layout each check and apply; a legacy report for the other platform alone is `UNKNOWN` and refused; `TestUPDV0017TasksReportAssetPerPlatform`: the producer names one report per target, bound to the archive digest, without the host-local path |
| UPD-V0-006 | owned bounded process probes | `TestUPDV0006SubprocessCancellationCleanup`: timeout/interruption descendant cleanup |

Actual macOS evidence MUST start disposable installs with retained Core/Tasks bytes, exercise the
published release check/apply path, upgrade Tasks build163 to build202, and restore the exact old
SHA-256. Negative checksum/availability/download/compatibility cases are deterministic fixtures;
fixtures do not qualify remote publisher identity or every platform. Linux and other host outcomes
remain NOT_RUN until observed. Retain original command output and independent findings in the task
build-log entry; do not replace full Tasks qualification with this install smoke.

## Rollout, rollback and maintenance

Build separately with `go build ./cmd/corvint-update`; install only this optional operator executable.
Keep existing Core/Tasks paths and stores. Adopt the command in this machine's existing update
heartbeat after focused checks, actual macOS lifecycle and independent review. Disabling the caller's
schedule/removing the separate updater returns to the manual baseline. Rollback remains explicit
and local, and reaches one step back once `UPD-V0-007` has pruned superseded transactions. Recheck official archive/manifest layouts when releases drift; refuse unknown layouts
instead of guessing. Any claim of stable delivery or additional platform support needs its own
retained acceptance evidence and owner-approved promotion.

## Remaining decisions

Technical contract acceptance, publisher-authenticated distribution and full Tasks runtime
qualification remain open. The requested local outcome can be observed without claiming those
promotions. Native ticket completion still needs the current queue's integration and gate rules.
