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
- Read next: Requirements; Trust boundary and limits; Acceptance evidence. A Core switch refreshes index snapshots of named and indexed enclosing checkouts (`UPD-V0-008`).

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
  be bounded. Extraction MUST refuse unsafe paths, aliases, duplicate files and all link/special
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
copies. The updater does not stop active agents/providers; already running executables continue,
and callers restart them when the applicable host integration requires it.

## Acceptance evidence

| Requirement | Implementation | Focused evidence |
|---|---|---|
| UPD-V0-001 | `internal/update`, `cmd/corvint-update` | `TestUPDV0001OfflineReadOnlyAndChannels` and `TestUPDV0001PaginationCapUnknown`: channels, offline/unknown, read-only |
| UPD-V0-002 | archive/identity verification | `TestUPDV0002ChecksumsAndArchive`, `TestUPDV0002PlatformAndQualificationIdentity`, `TestUPDV0002InternalTamperWithValidOuterChecksum`: outer/internal checksums, host, notices, evidence |
| UPD-V0-003 | bounded transport, paths and destination lock | `TestUPDV0003ArchivePathsAndLocks`: caps, aliases, traversal, concurrent state roots |
| UPD-V0-004 | candidate preparation and activation | `TestUPDV0004ActivationDowngradeCancelRace`, `TestUPDV0004UnchangedCleanupAndDestinationDrift`, `TestUPDV0004PartialArchiveCancellation`: no-op, downgrade, race and partial-download cancellation |
| UPD-V0-005 | bound prepared receipts and rollback | `TestUPDV0005ApplyRollbackAndPreparedReceipt` and `TestUPDV0005PreparedReceiptInterruptionRecovery`: exact restored digest, interrupted prepared state and drift |
| UPD-V0-007 | `sweepIncomplete`, `retainCommitted`, state lock | `TestUPDV0007RetentionBoundInterruptedSweepAndRollback`: three padded applies keep one transaction's previous bytes (state constant instead of growing per apply), exact `removed`/`left`, other component kept, malformed receipt left and named, a fresh receipt-less transaction and a `transaction-notes` operator directory kept, a stale interrupted (receipt-less) transaction swept by the next rollback, rollback after pruning restores the exact previous digest, a held state lock refuses the run; `TestUPDV0005ApplyRollbackAndPreparedReceipt` tampers the retained transaction |
| UPD-V0-008 | `refreshIndexes`, `--refresh-index` | `TestUPDV0008CoreSwitchRefreshesIndexedCheckouts`: an apply and a rollback refresh a named root once when it also encloses the working directory; outcomes `failed` (exit status and stderr line), `built`, `fresh` in order, an unindexed enclosing checkout `skipped` without a run, no outcome outside any checkout, exact argv, no refresh after an unchanged apply or a successful Tasks rollback; live: the installed Core build 404 reported `built` then `fresh` on a scratch indexed repository (2026-10-09) |
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
