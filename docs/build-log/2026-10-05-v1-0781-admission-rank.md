## 2026-10-05 V1-0781: `queue status` reports preparation-admission pressure without a lock

Human-owned intent comes from the owner's 2026-10-05 comment on issue 494, filed as ticket V1-0781.
About 13 concurrent supervised sessions sent a pooled claim to `LOCK_TIMEOUT ... phase=queue
registered=true rank=13`. Supervisors should be able to see the writer queue before they join
it. The acceptance criteria have three parts:
- report the registered waiting writers and the caller's would-be rank without the preparation
  lock, or report NOT_OBSERVED;
- derive any wait estimate from recorded cost with its basis named, or omit it;
- provide a test with N waiters that shows the rank and shows that the read wrote nothing.

Requirement: `CAL-V0-095` in `docs/specs/corvint-tasks-agent-leases-v0.md` (V1-0781 amendment).
CAL-V0-095 is the first ID after main's CAL-V0-073; CAL-V0-069 stays reserved for PR 557. No
`origin` branch used CAL-V0-095 when this was written.

### Change

- `authority.ObservePreparationQueue` sweeps the fixed issue 494 namespace under the git common
  directory: the registry lock, then slots `00` to `63`, then the registry again.
  - It opens each existing regular file read-only through the store's root-confined `safeopen`,
    with symlink, Lstat and SameFile checks. A file absent at its first stat is skipped and
    never created. A file that disappears or is replaced after that stat refuses as identity
    drift.
  - It asks the kernel whether an owner holds the file, without acquiring anything:
    - Darwin uses `fcntl(F_GETLK)`. A scratch test confirmed that it reports `flock(2)` owners
      with `l_pid` -1, both in another process and on another descriptor in the same process.
      A POSIX record lock reports its owner's pid, and the read then abstains: the query names
      only the first conflicting lock, so a coexisting flock cannot be ruled out.
    - Linux first requires `/proc/self/ns/pid` to be the initial PID namespace. This also
      proves that the reader is visible in the procfs mount, so `/proc/locks` lists every live
      owner. Otherwise the read abstains. It then reads one `/proc/locks` snapshot, bounded at
      4 MiB, through the existing `readProc`, and keeps granted `FLOCK` entries only. POSIX
      and OFD record locks do not interact with `flock` on local Linux filesystems. It matches
      the slot by device major:minor and inode. Another device holding the same inode is
      refused as ambiguous.
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
- A Linux reader outside the initial PID namespace, as in an ordinary container, reports
  NOT_OBSERVED instead of counting.
- Even in the initial namespace, Linux hides a `flock` whose locking process exited while another
  process still holds the file description. Corvint registrations cannot reach that state, but a
  foreign process could.
- An inode freed and reused by a replacement between stat and open has the same identity and is
  read as the replacement.

### Evidence

The full test list is in the spec's evidence table.

- Darwin/APFS (go1.27.1):
  - `TestCALV0095_PreparationQueueObservation` covers:
    - a real holder plus four queued `AcquirePreparation` waiters, which report 5 registered,
      max rank 5 and would-be rank 6;
    - all seven coordination files keeping identical bytes, mode and mtime;
    - retired stale slots counting 0, with rank 1;
    - an unpublished live slot hiding the rank;
    - a directory object, and a failing lock query, both reporting NOT_OBSERVED;
    - an absent namespace staying absent.
  - `TestCALV0095_PreparationQueueObservationAcrossProcesses` counts a registration held by a
    child process.
  - `TestCALV0095_QueueStatusAdmissionPressure` (CLI) shows idle rank 1. Three held slots plus
    one stale slot report 3 writers and rank 7. The journal tree, intent tree and slot bytes are
    unchanged, and no registry file is created.
  - The authority tests passed with `-count=3`.
- Linux/arm64, kernel 6.8, in privileged Colima containers, over both overlayfs and tmpfs, with
  both the container PID namespace and `--pid=host`:
  - The same authority tests passed, together with `TestCALV0095_ProcLocksParse`,
    `TestCALV0095_ProcLocksCompleteness` and the existing `TestGH494PreparationAdmission`.
  - In the container namespace, the authority counting tests assume a complete table, because
    every writer is a same-namespace test process. The CLI test verifies the real abstention and
    then skips its count assertions.
  - With `--pid=host`, the real completeness check passes, and the CLI test asserts counts.
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

### Review repair (Codex CHANGES_REQUIRED, four P2 findings)

1. Namespace-hidden writers no longer read as zero. A Linux reader outside the initial PID
   namespace abstains.
   - `TestCALV0095_ProcLocksCompleteness` covers the initial, child-namespace and not-visible
     links, plus a held slot that reads as NOT_OBSERVED under an incomplete table.
   - The container-namespace runs show that the CLI abstains for real. The host-PID runs show
     real counting.
   - A true cross-namespace writer was not run; the in-container runs test the gate rather than
     a hidden writer.
2. Only `flock` evidence counts.
   - Linux ignores POSIX and OFD entries; there are new parse cases.
   - Darwin abstains on a record lock.
   - `TestCALV0095_RecordLockIsNotARegistration` holds a real POSIX lock (Darwin), and a POSIX
     and an OFD lock (Linux), in a separate process on a retired rank-9 slot.
3. Disappearance after the first stat is identity drift, not absence. The
   `drift-after-stat-not-observed` subtest removes, or renames a replacement over, a slot between
   stat and open. The rename keeps both inodes distinct, because on overlayfs an
   unlink-then-create reused the inode.
4. The normative bullet moved into `## Requirements` as a V1-0781 subsection, and the ID moved
   from CAL-V0-074 to CAL-V0-095, because V1-0755 uses CAL-V0-074. A scratch call to
   `lrfrepo.requirementsFromBlob` enumerated CAL-V0-095 among 70 requirements; the scratch file
   was removed.

### Review repair (Codex round 2, one P2 finding)

Post-open disappearance escaped drift detection. If a slot was unlinked or replaced after the
open and before the descriptor stat, both compared identities described the original inode, so
the read reported RACY with zero writers for a vanished slot. The finding was reproduced before
the fix: with revalidation disabled, the new subtest read one registered writer and no reason.
After the lock query, and after the record read for a live slot, the observer now runs `Lstat` on
the pathname again and requires it to name the opened descriptor. This applies to both the
unlocked and live branches and to the registry. A mismatch or absence is identity drift.

- The `drift-after-open-not-observed` subtest unlinks the file, or renames a replacement over it,
  through a nil-in-production seam between open and stat. It does this for an unheld slot and for
  a slot held by the test.
- `registry-drift-after-open-not-observed` does the same for the registry.

The checks below ran under a private TMPDIR:

- Darwin, with `-count=3`:
  - authority `TestCALV0095|TestGH494PreparationAdmission$`;
  - cli `TestCALV0095`.
- The four Linux container configurations: overlayfs and tmpfs, each in a container and a
  host-PID namespace.

### Review repair (Codex round 3, one P2 finding)

Codex accepted the round-2 slot and registry repair. The remaining gap was the common directory.
If it was renamed away and replaced after `openRoot`, every per-file check still resolved inside
the original pinned directory, so the read could report RACY counts from a displaced namespace.

This was reproduced before the fix: with the new check disabled, the regression subtest read two
registered writers and gave no reason. Before reporting, the observer now checks the absolute
common-directory pathname again, as `preparationScope.check` does: `Lstat` must name the
originally stat'ed directory, it must not be a symlink, and `intent.CheckNoSymlink` must pass.
Otherwise the result is `common directory identity drift`, reported as NOT_OBSERVED.

`common-dir-replaced-after-open-not-observed` runs inside the `observeAfterOpen` seam. It renames
the common directory away, then puts either a fresh directory or a symlink to the displaced
original at its pathname. The test expects NOT_OBSERVED with no rank, and it restores the original
afterwards so the fixture's store audit holds.

These checks pass under a private TMPDIR:

- On Darwin, with `-count=3`, the authority tests `TestCALV0095|TestGH494PreparationAdmission$`
  and the cli tests `TestCALV0095`.
- The four Linux container configurations.

### NOT_RUN and limits

- Not run:
  - `make gate`, as the owner prefers for scoped work;
  - `cmd/corvint` and the broader doc-reading packages that `corvint affected` selected through
    the spec edit;
  - a live multi-session qualification at more than 13 sessions;
  - Linux filesystems other than overlayfs and tmpfs, and network filesystems;
  - an initial-namespace reader observing a real writer in another PID namespace;
  - Darwin OFD-style locks, which are unqualified;
  - the Codex re-review, and the CEM bind and seal, which the orchestrator runs.

Rollback: remove the `preparationAdmission` field and the `preparation_observe*` files. No store,
wire profile or coordination file depends on them.
