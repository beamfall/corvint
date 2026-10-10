# GitHub 715: `corvint-tasks doctor`

Date: 2026-10-09

GitHub issue 715 asks for one read that names stuck-queue patterns. This change adds that verb as
an experimental source implementation, specified in
[the doctor contract](../specs/corvint-tasks-doctor-v0.md). TQD-V0-001..012 are proposed and
await owner acceptance; this change records no decision.

Design choices:

- **Pure read.** The doctor reads the store through TM-V0-008 (`withStore`). Only an explicit
  `doctor --refresh` writes, and only to `<git common dir>/taskman-doctor/summary.json`. That
  cache sits outside the journal and the state directory, so plain `doctor` stays a pure read
  under Product invariant 4. The write uses a temp file, fsync and rename.
- **`--line` reads only the cache.** It takes no lock, makes one bounded read that does not
  follow links, and prints plain text. It is a documented exception to the envelope rule.
- **Journal detectors scan backward from the head.** The scan covers at most 4096 receipts and at
  most 7 days. It reads each attempt and pool afterimage only after the bytes are checked against
  the post digest. A missing or mismatched receipt stops the scan, and `scan.truncated` reports
  the stop.
- **NO_PROGRESS_HANDOFF** reuses the CAL-V0-102 no-progress test. It does not depend on the
  `loopDetection` policy.
- **Refused requests are not counted.** A refused claim or a refused `confirm-safe` leaves no
  receipt, so REPEAT_REFUSAL and SLOW_LANE_RECOVERY count only recorded generations, gate results
  and CLEANING entries.
- **FALSE_IDLE** walks only detached run supervisors (ATR-V0-010..012), because their process
  identity is recorded. The lane leader start time is not recorded, so supervised and dispatcher
  workers are out of scope.
- **Holder status logic** is extracted from `addHolderObservation` into `holderStatus` with no
  change in behavior.
- **PREFLIGHT_FAILURE** is a reserved kind that waits for #713's recorded preflight evidence.
- **Plugins** run as separate process groups. Each has a 10 s timeout and a 64 KiB output cap,
  and its output is decoded strictly. A failing plugin becomes a PLUGIN_FAILED finding instead of
  a command failure, and the envelope is marked untrusted.
- **The cache profile** `taskman-doctor-cache/0` is a live format: it refuses a later version
  with UNSUPPORTED_VERSION. The item profile `taskman-doctor/0` is output-only (CAL-V0-131).

Evidence:

- Before the verb existed, all eight `TestTQDV0*` tests in `internal/tasks/cli/doctor_test.go`
  failed with "unknown verb".
- After the change, the focused run passes:
  `go test -run 'TestTQDV0|TestCALV0131|TestCALV0047|TestHelp|TestCALV0120|Holder|PoolStatus' ./internal/tasks/cli/`.
- `TestTQDV0012_LineReadsCache` asserts that `--line` completes in under 50 ms.
- `TestTQDV0001_DoctorIsPureRead` asserts that plain `doctor` leaves the state directory, the
  intent tree and the cache directory byte-identical.
- gofmt and `go vet ./internal/tasks/cli/` are clean. The full `./internal/tasks/cli/` package passes
  (371 s), `./internal/companionrelease/` passes, and the nine doc gates pass.

Review fixes (independent Codex review of `1120e565..582aa0ab`, VERDICT FAIL, seven findings;
the orchestrator decided each fix):

1. `--refresh` followed a linked `taskman-doctor` directory or `summary.json` and chmodded the
   target. It now works through `os.Root` from the common directory, refuses a link, a
   non-directory or non-regular file, or a foreign owner with UNSUPPORTED_FILESYSTEM before any
   create, chmod or write, and creates its temp file and renames only inside the verified
   directory.
2. A plugin's background child outlived the doctor. The plugin's process group is now killed
   after every exit or timeout.
3. The cache could exceed the 1 MiB bound that `--line` reads. Refresh now drops trailing
   findings in report order until the file fits and sets `findingsTruncated`.
4. REPEAT_REFUSAL counted gate results recorded before the window. A digest now counts only at
   the receipt that adds it to its attempt; a pre-existing attempt's first scanned afterimage is
   a baseline.
5. `--refresh` overwrote a cache written by a later build. It now refuses UNSUPPORTED_VERSION and
   leaves the file byte-identical; other undecodable caches are still replaced.
6. `--line` resolved the intent worktree, which read the queue manifest and linked worktree
   entries. `intent.ResolveCommonDir` now stops at the validated common directory.
7. Plugin discovery read the whole directory. It now reads in batches up to 4096 entries and runs
   nothing beyond that.

Each fix has a test that failed before it: `TestTQDV0011_RefreshRefusesSymlinkedCache`,
`TestTQDV0010_PluginDescendantsAreKilled`, `TestTQDV0011_MaximalPluginOutputStaysReadable`,
`TestTQDV0006_OldGateResultsAreNotRecentRefusals`, `TestTQDV0011_RefreshKeepsNewerCache`,
`TestTQDV0012_ResolveCommonDirReadsNoManifest` and `TestTQDV0010_PluginDiscoveryIsBounded`.
TQD-V0-003, -006, -010, -011 and -012 are amended to match and stay proposed.

Limits:

- A descendant that calls `setsid` leaves the plugin's process group and escapes the kill.
- A gate result added in the receipt where the scan first meets a pre-existing attempt is
  treated as baseline, so REPEAT_REFUSAL can under-count by one at the window edge.
- The scan does not read the dispatcher ledger.
- The Linux `ps` walk is unqualified. Tests ran only on Darwin arm64.
- Plugins are not sandboxed.
- The cache can be stale. `--line` shows its age once it is 15 minutes old.
- Live qualification against a large real store is NOT_RUN.

## Review fixes, round 2

Codex's second review found three more defects. All three are fixed.

1. The refresh could still be redirected through a directory swap. `os.Root.OpenRoot` follows
   relative links, and a same-file check proves identity, not location. On darwin and linux the
   writer now pins the common directory, creates `taskman-doctor` beneath it if absent, and opens it
   once with `O_NOFOLLOW|O_DIRECTORY`. Type and owner are checked on that descriptor. The fchmod,
   lock, destination re-read, `O_EXCL|O_NOFOLLOW` temporary create, fsync and rename all run
   relative to the open directory, and the cache path is never resolved by name again.
   `safeopen.RootOf` bridges the pinned descriptor to an `os.Root` for the fd-relative rename and
   remove, because Darwin's `syscall` package has no `Renameat`.
2. Two concurrent refreshes could overwrite a newer cache. The refresh now takes an exclusive
   `flock` on `taskman-doctor/.lock`. It opens the lock without following links and requires a
   regular file it owns. Under the lock it re-reads `summary.json` and repeats the version check
   before it writes and renames.
3. On a store with valid tracked intent and no local journal, plain `doctor` returned
   UNINITIALIZED. It now reads through the inventory store reader, so that store refuses
   MISSING_EVIDENCE as TQD-V0-001 states, both with and without `--refresh`, and writes nothing.

Each fix has a test that failed before it:

- `TestTQDV0011_RefreshRefusesSwappedCacheDir`: a hook between the check and the open swaps the
  cache directory for a relative link into `.git/taskman`. Before the fix the refresh returned OK.
  Now it refuses UNSUPPORTED_FILESYSTEM and leaves `.git/taskman` unchanged, with nothing written
  and no mode changed.
- `TestTQDV0011_RefreshRechecksVersionUnderLock`: a hook installs a `taskman-doctor-cache/1`
  file after the first check. Before the fix the refresh replaced it. Now it refuses
  UNSUPPORTED_VERSION and the file stays byte-identical.
- `TestTQDV0001_JournalAbsentIsMissingEvidence`: before the fix the store refused UNINITIALIZED.

TQD-V0-001 and TQD-V0-011, the failure modes and traceability are amended. The requirements stay
proposed.

Limits:

- The descriptor anchoring and the lock exist on darwin and linux only. Elsewhere the old
  `os.Root` writer remains, which can follow a swapped-in link inside the common directory and
  does not serialize refreshes.
- The lock is advisory. A writer that does not take it can still replace `summary.json` after the
  re-read.
- `.lock` is a new persistent file in the cache directory. Rollback deletes it with the directory.
