## 2026-10-05 V1-0781: `queue status` reports preparation-admission pressure without a lock

Human-owned intent comes from the owner's 2026-10-05 comment on issue 494, filed as ticket V1-0781.
About 13 concurrent supervised sessions sent a pooled claim to `LOCK_TIMEOUT ... phase=queue
registered=true rank=13`. Supervisors should be able to see the writer queue before they join
it. The acceptance criteria have three parts:
- report the registered waiting writers and the caller's would-be rank without the preparation
  lock, or report NOT_OBSERVED;
- derive any wait estimate from recorded cost with its basis named, or omit it;
- provide a test with N waiters that shows the rank and shows that the read wrote nothing.

Requirement: `CAL-V0-074` in `docs/specs/corvint-tasks-agent-leases-v0.md` (V1-0781 amendment).
CAL-V0-074 is the first ID after main's CAL-V0-073; CAL-V0-069 stays reserved for PR 557. No
`origin` branch used CAL-V0-074 when this was written.

### Change

- `authority.ObservePreparationQueue` sweeps the fixed issue 494 namespace under the git common
  directory: the registry lock, then slots `00` to `63`, then the registry again.
  - It opens each existing regular file read-only through the store's root-confined `safeopen`,
    with symlink, Lstat and SameFile checks. An absent file is skipped and never created.
  - It asks the kernel whether an owner holds the file, without acquiring anything:
    - Darwin uses `fcntl(F_GETLK)`. A scratch test confirmed that it reports `flock(2)` owners,
      both in another process and on another descriptor in the same process.
    - Linux reads one `/proc/locks` snapshot, bounded at 4 MiB, through the existing
      `readProc`. It matches the slot by device major:minor and inode. Blocked waiters (`->`),
      leases and delegations are ignored. Another device holding the same inode is refused as
      ambiguous.
    - Other platforms report NOT_OBSERVED.
  - A live slot with a valid 16-byte `CPA1` record counts as a registered writer. Its rank feeds
    `max+1`. A live slot without a valid record counts as unpublished and withholds the would-be
    rank.
- `queue status` adds one `preparationAdmission` object after its store snapshot. The new fields
  are `capacity`, `snapshot` (`RACY` or `NOT_OBSERVED`), `method`, `notObservedReason`,
  `registeredWriters`, `unpublishedSlots`, `wouldBeRank`, `registryActive`, `estimatedWait` and
  `estimatedWaitBasis`. No existing key changes. The closed key lists in `internal/taskman` and
  `internal/companionrelease` already omit earlier additive status keys such as `liveAttempts`
  and `journalAudit`.
- The wait estimate is always `NOT_OBSERVED`, with the basis `no recorded per-mutation writer
  cost`. Main records no per-mutation writer time; the V1-0645 / CAL-V0-069 phase timing is
  unlanded. Receipt spacing or host load would be invented certainty.

### Why this stays a read

Invariant 4 holds as follows:
- The sweep never registers, flocks, creates, truncates or writes.
- The lock query observes and does not acquire. Neither `F_GETLK` nor reading `/proc/locks`
  perturbs admission.
- A file that is missing, unsafe or drifting stops the observation and reports NOT_OBSERVED,
  never zero.

Racy limits, labelled rather than hidden:
- Each slot is sampled at a different instant.
- While a registrant owns the registry, it probes free slots with a transient flock. Such a
  probe can make a stale record look live. `registryActive` marks that window.
- Older clients bypass registration and stay invisible.
- On Linux, locks held by owners outside the reader's PID namespace are not listed, so they are
  under-counted without detection.

### Evidence

The full test list is in the spec's evidence table.

- Darwin/APFS (go1.27.1):
  - `TestCALV0074_PreparationQueueObservation` covers:
    - a real holder plus four queued `AcquirePreparation` waiters, which report 5 registered,
      max rank 5 and would-be rank 6;
    - all seven coordination files keeping identical bytes, mode and mtime;
    - retired stale slots counting 0, with rank 1;
    - an unpublished live slot hiding the rank;
    - a directory object, and a failing lock query, both reporting NOT_OBSERVED;
    - an absent namespace staying absent.
  - `TestCALV0074_PreparationQueueObservationAcrossProcesses` counts a registration held by a
    child process.
  - `TestCALV0074_QueueStatusAdmissionPressure` (CLI) shows idle rank 1. Three held slots plus
    one stale slot report 3 writers and rank 7. The journal tree, intent tree and slot bytes are
    unchanged, and no registry file is created.
  - The authority tests passed with `-count=3`.
- Linux/arm64, kernel 6.8, in a privileged Colima container, over both overlayfs and tmpfs:
  - The same authority and CLI tests passed, together with `TestCALV0074_ProcLocksParse` and the
    existing `TestGH494PreparationAdmission`.
- Static checks:
  - `go vet` passed for darwin and for linux/arm64.
  - The `windows/amd64` `go build ./...` succeeded.
  - `gofmt` is clean.
  - The doc gates passed.
- Full-package runs on Darwin used the lane-private TMPDIR `v7t-0781`, at load average 40 to 100:
  - `internal/tasks/authority`, `internal/taskman`, `internal/specindex` and
    `internal/companionrelease` passed.
  - `internal/tasks/cli` had one timing failure, `TestPSRForegroundSignals/interrupt`; an earlier
    run had a different one, `TestATRV0004`.
  - `internal/tasks/store` failed in `TestPSRAllBarrierTerminal`.
  - Each of those failing tests then passed with `-count=2` in isolation. Neither package's
    failing code is touched by this change.
  - An earlier run in the shared TMPDIR lost its test binaries to another lane's cleanup. That
    caused `fork/exec` failures, and the run was discarded.

### NOT_RUN and limits

- Not run:
  - `make gate`, as the owner prefers for scoped work;
  - `cmd/corvint` and the broader doc-reading packages that `corvint affected` selected through
    the spec edit;
  - a live multi-session qualification at more than 13 sessions;
  - Linux filesystems other than overlayfs and tmpfs, network filesystems, and readers in
    another PID namespace;
  - the independent review and the CEM bind and seal, which the orchestrator runs.

Rollback: remove the `preparationAdmission` field and the `preparation_observe*` files. No store,
wire profile or coordination file depends on them.
