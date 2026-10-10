# Corvint Tasks doctor V0

Owner: Russell Lewis
Date: 2026-10-09
Intent status: proposed (GitHub #715; TQD-V0-001..012 pending owner acceptance)
Delivery status: experimental
Authoritative inputs: owner request [issue 715](https://github.com/beamfall/corvint/issues/715);
the pure-read rule TM-V0-008 and the shared journal audit CAL-V0-061; the attempt and pool
afterimage profiles in `internal/tasks/snapshot`; the detached run records of
[the attempt runner](corvint-tasks-attempt-runner-v0.md) (ATR-V0-010..012); the no-progress
semantics of CAL-V0-102; the obligation counts of
[the obligation ledger](corvint-tasks-obligation-ledger-v0.md). This technical contract proposes
concrete semantics; the issue intent does not accept generated implementation detail.

## Agent digest
- Claim: `corvint-tasks doctor` names stuck-queue patterns from journal, pool and attempt data plus bounded plugins; `doctor --line` prints a cached status line.
- Status: proposed (GitHub #715; TQD-V0-001..012 pending owner acceptance); experimental source implementation with focused CLI tests.
- Exists: built-in NO_PROGRESS_HANDOFF, REPEAT_REFUSAL, SLOW_LANE_RECOVERY, SETUP_ONLY_PROOF and FALSE_IDLE (detached-run supervisors only) detectors; bounded plugins with PLUGIN_FAILED; `--refresh` atomic cache; `--line` lock-free cache read.
- Blocked on: owner acceptance; PREFLIGHT_FAILURE waits for #713's recorded preflight evidence; the dispatcher ledger is not read; the Linux process walk is unqualified.
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

- `TQD-V0-001`: `corvint-tasks doctor [--refresh] [--plugins DIR]` is a pure read under TM-V0-008: it takes no lock and writes nothing in the journal, the state directory, the intent tree or any attempt run directory. Without `--refresh` it writes no file at all. It emits one `taskman-command-result/0` envelope whose single item has profile `taskman-doctor/0`, `queueId`, `observedAt`, `summary`, `findings`, `scan` and `mutationAuthority:false`. A store whose tracked intent is valid but whose local journal is absent refuses MISSING_EVIDENCE, not UNINITIALIZED, for plain `doctor` and `--refresh` alike, and writes nothing. Any other argument refuses VALIDATION_FAILED `MALFORMED` before the store is read. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-002`: `summary` is `{lanes:{free,total}, runningSessions, completions24h, alerts}`. `lanes.total` counts the policy's pool members and `lanes.free` those absent from audited pools.json; `runningSessions` counts live (non-terminal) attempts; `completions24h` is the `queue status` last-24-hours completion count at `observedAt`; `alerts` is the number of findings. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-003`: Each finding is `{kind, source, who, detail, remedy, firstSeen, ageSeconds, evidenceSeqs}`. `kind` is a stable upper-case code; `source` is `builtin` or `plugin:<name>`; `who` names the ticket, pool member, attempt or plugin; `remedy` is fixed operator guidance per built-in kind. Built-in journal findings take `firstSeen` from the recordedAt of their earliest evidence receipt and list those receipt sequence numbers; FALSE_IDLE and plugin findings carry `firstSeen` from the cache entry with the same kind, source and who, else `observedAt`. `ageSeconds` is `observedAt - firstSeen`, never negative. A finding not re-detected is dropped (ages out). Findings sort by kind, source and who; at most 512 are emitted, the cache written by `--refresh` keeps only the leading findings in that order whose encoded file fits 1 MiB (TQD-V0-011), and `scan.findingsTruncated` reports either cut; `summary.alerts` still counts every finding. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-004`: Journal-derived detectors read one bounded backward scan of receipts from the snapshot head: at most 4096 receipts and none recorded more than 7 days before `observedAt`. Each attempts/*.json and pools.json post is taken from the inline record or its `evidence/<sha256>` blob and used only when its bytes hash to the post digest. A missing, undecodable or digest-mismatched receipt stops the scan; `scan` reports `receipts`, `fromSeq`, `headSeq`, `windowSeconds` and `truncated`, so a partial scan is visible rather than read as complete. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-005`: NO_PROGRESS_HANDOFF names a ticket whose newest three or more ended generations in the scan are each a HANDOFF with no gate result, no review and a candidate tree that is absent or equal to the last tree recorded before it (the CAL-V0-102 no-progress test), independent of any `loopDetection` policy. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-006`: REPEAT_REFUSAL names a ticket and candidate tree refused three or more times in the scan. A refusal is a generation ending REVIEW_RETURNED (its own candidate tree, else the ticket's last recorded tree) or a distinct FAILED gate result recorded for that tree. A gate result counts once, at the receipt whose attempt afterimage first adds its digest; the gate results an attempt already carries when the scan first meets it, unless that receipt created the attempt, were recorded before the window and neither count nor set `firstSeen`. Refused lease requests leave no receipt and are not counted. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-007`: SLOW_LANE_RECOVERY names a pool member whose allocation entered CLEANING (a cleanup or sweep try) three or more times in the scan without being freed, or was freed only after three or more tries. A refused `pool confirm-safe` leaves no receipt, so the count is of recorded cleanup and sweep tries. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-008`: SETUP_ONLY_PROOF names a ticket COMPLETED within the window whose obligation ledger counts at least one core obligation and witnessed none of them, from the current audited record. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-009`: FALSE_IDLE names a live attempt whose holder status is STALE_HOLDER or LEASE_EXPIRED while one of its detached runs for the current generation is STARTING or RUNNING, its supervisor matches the recorded process identity, and a bounded local process-table walk (`/bin/ps -axo pid=,ppid=,stat=`, 5 s, 4 MiB, 4096 processes) finds a non-zombie descendant of the supervisor. A run directory that is not a directory (for example a FIFO) is skipped without blocking. A walk that cannot complete reports nothing for that attempt and adds a `scan` warning. The PREFLIGHT_FAILURE kind is reserved for #713's recorded preflight evidence and is not emitted by this delivery. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-010`: Plugins run only when `--plugins DIR` names a directory. Discovery reads at most 4096 directory entries in fixed-size batches; a directory holding more runs no plugin and yields one PLUGIN_FAILED naming the directory. Its regular executable files, sorted by name and at most 32, each run with the primary worktree as cwd, no stdin, a 10 s timeout, and stdout capped at 64 KiB, in their own process group, which is killed (SIGKILL) whenever the plugin exits or times out, before the doctor continues, so no descendant outlives it. Stdout must be one JSON array of at most 64 objects with exactly `kind`, `who`, `detail` and optional `remedy` (kind matching `^[A-Z][A-Z0-9_]{0,63}$`, other strings valid prose up to 1024 bytes). Each becomes a finding with source `plugin:<name>` beside the built-ins and the envelope is marked untrusted. A nonzero exit, timeout, oversized or malformed output becomes one PLUGIN_FAILED finding naming the plugin, never a command failure. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-011`: `doctor --refresh` writes the item, with profile `taskman-doctor-cache/0` and `refreshedAt`, to `<git common dir>/taskman-doctor/summary.json` (directory 0700, file 0600) through a temporary file, fsync and rename, so a reader sees the old or the new cache, never a mixture. Every step is anchored on descriptors, never on a re-resolved path: the common directory is pinned, the cache directory is created beneath it if absent and opened once with `O_NOFOLLOW|O_DIRECTORY`, and its type and owner are checked on that descriptor; the chmod, lock, destination re-read, temporary-file create (`O_EXCL|O_NOFOLLOW`), fsync and rename are all relative to that open directory. Because a descriptor pins the directory but not its place, before the chmod, after taking the lock, before the temporary-file create, before the rename and before reporting success the refresh checks, relative to the pinned common directory and without following a link, that `taskman-doctor` is still the directory it opened; a directory moved away after it was opened (for example into the state directory) is refused UNSUPPORTED_FILESYSTEM and the refresh's temporary file is removed. A cache directory, `summary.json` or lock file that is a symbolic link, not a directory or regular file (including a directory where the lock file or `summary.json` belongs), or not owned by the effective user is refused UNSUPPORTED_FILESYSTEM before anything is created, chmodded or written through it, including a directory swapped for a link after the doctor's first check. The refresh holds an exclusive `flock` on `taskman-doctor/.lock` (a regular 0600 file the refresh creates) from before it re-reads the destination until after the rename, so concurrent refreshes are serialized. The encoded cache never exceeds the 1 MiB `--line` read bound (TQD-V0-003). The cache is outside the journal and the state directory and is never an input to any other command; plain `doctor` reads it only to carry `firstSeen`. A cache whose profile names a later `taskman-doctor-cache/N` is refused UNSUPPORTED_VERSION and treated as unavailable (CAL-V0-131); `--refresh` then refuses UNSUPPORTED_VERSION and leaves that file byte-identical, repeating that check under the lock so a newer cache installed after the first check is also kept, while any other undecodable cache is replaced. Status: (proposed, pending owner acceptance; GitHub #715).
- `TQD-V0-012`: `doctor --line` reads only the cache, through the repository resolution walk to the git common directory (reading no intent hint, queue manifest, HEAD or linked worktree entry) and one bounded (1 MiB) non-following regular-file read, with no lock, no store read and no write, and prints one plain-text line `lanes F/T free | sessions N | 24h N done | alerts N`, adding `| stale Nm` when the cache is at least 15 minutes old. It is a documented exception to the envelope rule. A missing or invalid cache prints `doctor cache unavailable; run corvint-tasks doctor --refresh` and exits 1. It completes in under 50 ms on a local cache. Status: (proposed, pending owner acceptance; GitHub #715).

## Detectors

Every built-in threshold is 3 within the 7-day window. Remedies:

| Kind | Remedy |
| --- | --- |
| NO_PROGRESS_HANDOFF | Read the hand-off evidence, then refine, split or hold the ticket instead of claiming it again. |
| REPEAT_REFUSAL | Compare the review returns and failed gates for this tree; change the candidate or the criteria before resubmitting. |
| SLOW_LANE_RECOVERY | Inspect the member's cleanup output and the external resource; repair the cleanup, then confirm safety. |
| SETUP_ONLY_PROOF | Witness the core obligations or reopen the ticket; its completion proved only setup. |
| FALSE_IDLE | The session's process tree is busy while its heartbeat is stale; inspect the run before reaping or relaunching. |
| PLUGIN_FAILED | Fix or remove the project plugin named in `who`. |

## Failure modes

- A pruned or rewritten receipt stops the scan (`scan.truncated`); detectors then see only the newer tail.
- Refused requests (claim, release, confirm-safe) write no receipt and cannot be counted.
- The lane identity's leader start time is not recorded, so FALSE_IDLE uses detached run supervisor identities only; supervised and dispatcher workers are not walked.
- A plugin is operator-chosen code; the doctor bounds its time and output but does not sandbox it. A descendant that leaves the plugin's process group (for example with `setsid`) escapes the group kill.
- A gate result added in the receipt where the scan first meets a pre-existing attempt is indistinguishable from the attempt's older results and is not counted, so REPEAT_REFUSAL can under-count by one at the window edge, never over-count.
- The cache can be stale; `--line` shows its age once it is 15 minutes old.
- The descriptor anchoring and the refresh lock exist on darwin and linux only. Elsewhere the refresh keeps the path-rooted `os.Root` writer: a cache directory swapped for a link inside the common directory between its checks can be followed, and concurrent refreshes are not serialized, so the last rename wins.
- The location checks bracket each step but cannot be atomic with it: a cache directory moved between the last pre-rename check and the rename receives the new `summary.json`; the post-rename check then refuses UNSUPPORTED_FILESYSTEM rather than reporting success, and the write is not undone.
- The lock is advisory: a writer that does not take `taskman-doctor/.lock` (an older build of this verb, or any other program) can still replace `summary.json` after the re-read. The `.lock` file stays in the cache directory; rollback removes it with the directory.

## Acceptance evidence

Focused tests in `internal/tasks/cli/doctor_test.go` build each built-in pattern through the real
CLI against a disposable fixture store, run a passing and a failing plugin, assert that plain
`doctor` leaves the state directory, intent tree and cache directory byte-identical, that
`--refresh` writes the cache atomically, refuses a linked cache directory or file, a cache directory swapped for a relative link after its first check or moved into the state directory after it was opened, a directory where the lock file belongs, and a newer cache format, including one installed after the first check, without writing, that a journal-absent store refuses MISSING_EVIDENCE, and fits the largest plugin output into a cache `--line` reads, that plugin descendants are gone when the doctor returns and an oversized plugin directory runs nothing, that a FIFO in place of a run directory does not block FALSE_IDLE discovery, and time `--line` under 50 ms. `internal/tasks/intent/intent_worktree_test.go` shows the common-directory resolution `--line` uses reads no queue manifest or linked worktree entry.

## Compatibility and rollback

The verb, item profile and cache profile are new; no stored format changes. Rollback removes the
verb and deletes `<git common dir>/taskman-doctor/`, which nothing else reads.

## Traceability

| Requirement | Evidence |
| --- | --- |
| TQD-V0-001 | `TestTQDV0001_DoctorIsPureRead`, `TestTQDV0001_JournalAbsentIsMissingEvidence` |
| TQD-V0-002 | `TestTQDV0012_LineReadsCache` |
| TQD-V0-003 | `TestTQDV0005_NoProgressHandoff`, `TestTQDV0010_PluginFindings`, `TestTQDV0011_MaximalPluginOutputStaysReadable` |
| TQD-V0-004 | `TestTQDV0005_NoProgressHandoff` |
| TQD-V0-005 | `TestTQDV0005_NoProgressHandoff` |
| TQD-V0-006 | `TestTQDV0006_RepeatRefusal`, `TestTQDV0006_OldGateResultsAreNotRecentRefusals` |
| TQD-V0-007 | `TestTQDV0007_SlowLaneRecovery` |
| TQD-V0-008 | `TestTQDV0008_SetupOnlyProof` |
| TQD-V0-009 | `TestTQDV0009_FalseIdle`, `TestTQDV0009_FalseIdleSkipsRunsFIFO` |
| TQD-V0-010 | `TestTQDV0010_PluginFindings`, `TestTQDV0010_PluginDescendantsAreKilled`, `TestTQDV0010_PluginDiscoveryIsBounded` |
| TQD-V0-011 | `TestTQDV0012_LineReadsCache`, `TestCALV0131_EveryLiveFormatRefusesANewerVersion`, `TestTQDV0011_RefreshRefusesSymlinkedCache`, `TestTQDV0011_RefreshKeepsNewerCache`, `TestTQDV0011_RefreshRefusesSwappedCacheDir`, `TestTQDV0011_RefreshRefusesCacheDirMovedAfterOpen`, `TestTQDV0011_RefreshRefusesDirectoryLock`, `TestTQDV0011_RefreshRechecksVersionUnderLock`, `TestTQDV0011_MaximalPluginOutputStaysReadable` |
| TQD-V0-012 | `TestTQDV0012_LineReadsCache`, `TestTQDV0012_ResolveCommonDirReadsNoManifest`, `TestTQDV0011_MaximalPluginOutputStaysReadable` |
