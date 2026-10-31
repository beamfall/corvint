# Bug batch wave 2, Linux lane (linux-w2)

Fifteen 1.0 bug tickets that an audit on 2026-10-31 found open on `origin/main` at
`780f9e97099aa3ccd52744a0c239a744c4a30adf`, worked on a Linux host (Unraid, 12 CPUs, go1.27.3)
with no Windows, macOS, Node or Pi runtime. New requirements: `MCPV0-820`, `AHI-820`, `TQD-V0-820`
and `BBF-V0-820`; `SDD-V0-005` is amended in place. Six ticket bodies (V1-2492, V1-2507, V1-2513,
V1-2691, V1-2727, V1-3243) are not in the base's `.taskman`; they were read from the corpus
snapshot `internal/tasks/similar/testdata/corpus-2026-10-30.jsonl` and, for V1-3243, from
`docs/build-log/2026-10-30-friction-affected-selection.md`.

## Decisions

- V1-0393: freebsd, openbsd and netbsd already vet clean on the base. Solaris failed on two
  `//go:build unix` test files that call `syscall.Mkfifo`, which Solaris's `syscall` lacks. They
  now exclude Solaris. `make cross-vet` is the regression guard and already runs in `make gate`.
- V1-3243: the two scripts are tracked `100755`, and `script/check-script-modes.sh`
  (`make script-modes-check`, in `GATE_STEPS`) fails on any tracked `script/*_test.sh` that is
  not. Each script also gets one comment line, but the CEM patch grammar refuses `old mode`/`new
  mode` metadata even beside a text hunk (`propose-citations`: `patch-unavailable
  reason=malformed-patch`). The commit is therefore the branch's tip, after the seal, unbound by
  the CEM, as the 2026-10-30 friction lane left the same fix.
- V1-2416: the consume CLI already named the bound. The library refusals now name the bound, its
  value and the observed size, and `docs maintain` keeps the library message (a new
  `Refusal.Message`) instead of repeating the code. Line numbers that specs cite were kept.
- V1-2507, V1-2513: `MCPV0-021` makes protocol selection explicit, so negotiating 2025-11-25 from
  the incoming frame was ruled out. The three shipped servers now set the existing
  `NameLegacyFlag` (`DCP-V1-652`), so a 2025-11-25 `initialize` gets `-32602` with the flag named.
  The two companion binaries already accepted `--protocol-version 2025-11-25` and had legacy
  handshake tests. The Claude plugin cannot launch binaries outside the Core archive (`AHI-373`),
  so it reaches their tools through the core binary's `test-validity,docs` tool groups
  (`AHI-820`).
- V1-1193: profiling at 30,073 receipts showed that the scan window is no longer the cost. The
  padded fixture writes no read checkpoint, so every doctor run full-audited the journal. A store
  of real writes keeps one. The doctor now reports `journalAudit`, and the measurement times both
  modes. A cached or smaller scan was not added: the cost stays bounded by the window once a
  writer has retained a checkpoint, and a read must not write one.
- V1-2215: the GOROOT members a `go test` run reads are digested at the driver's check, then copied
  into a private GOROOT while being digested again. Any difference refuses, and the verified
  driver runs against that copy. The RSpec toolchain is staged as `runPinned` stages every pinned
  command. Cost: the package's provider tests went from 4.8 s to 12.8 s, about 220 MB copied per
  provider run.
- V1-2691: on the base the remote cache already lives on disk (`ASD-V0-028`) and is trimmed
  (`ASD-V0-038`). On this host the one test that leaked `/tmp/Test*` directories was
  `TestWorkRunnerCoordinatorInterruptionGuardian`: its coordinator helper is killed by design, so
  its `t.TempDir` was never removed. Its `TMPDIR` is now inside the test's own temporary directory.
  Before: one leaked directory per run. After: none over three runs.

## Follow-up fixes (second pass)

- `git-read-failed` under load (V1-1470 criterion 2): a debug pass showed
  `exec: WaitDelay expired before I/O complete` on `git cat-file` in `git03`. The 250 ms
  `WaitDelay` let the output-copy goroutines lose the race after a successful exit. It is now 5 s
  (`CEM-V1-820`). The `pipe-held-briefly` case fails before and passes after. The selection that
  failed twice under 60 busy loops (load average 23 to 41) then passed in 393 s. Other 100-250 ms
  `WaitDelay` sites remain (`conformance/release-artifact-v0`, `internal/procgroup`,
  `internal/testsupport`, `tools/beamfall-shadow`, darwin `processidentity`); none was observed to
  fail here.
- `TestTQDV0021_RealProbes` failed on the base on this host. On btrfs, `stat` reports the
  subvolume's anonymous device (`00:48`) and `/proc/locks` the superblock's (`00:2f`), so the
  doctor never named a held `taskman.lock`. It now falls back to a same-inode `FLOCK` holder that
  has the file open (`TQD-V0-821`).
- `TestDogfoodEventObservedMismatchFallsBack` failed whenever a test that starts `corvint mcp`
  in-process ran first in the same binary: a started server pins Git for its process (V1-0191),
  so the `countingGit` PATH shim saw no spawns. `TestAHI373MCPVerbOwnsItsArgumentsAndProfile`
  (on the base) and this lane's AHI-820 test both did that. The first now serves from a re-exec'd
  child; the second's served-args half moved to `internal/mcp/cli`.

## Codex review fixes

- P2, V1-2215: GOROOT staging ran outside the command budget, and the driver then got the full
  timeout again. `runProvider` now fixes one deadline: both GOROOT passes stop on cancellation or
  when it passes (`command budget exhausted`), and the driver gets `time.Until(deadline)`.
  `TestBBFV0820StagingHonorsTheCommandBudget` fails on the previous code in both cases.
- P3, TQD-V0-821: an open descriptor was taken as proof of ownership. A holder is now named only
  through a descriptor on this file (stat device and inode) whose `/proc/<pid>/fdinfo` lists the
  `FLOCK`, which the kernel prints only for the owning open file. With the fdinfo check removed,
  `TestTQDV0821OpenDescriptorWithoutLockIsNotOwnership` fails three of its four cases.

## Already delivered on the base

- V1-2727: `AFP-V0-464`. `internal/gateselect` `unresolved()` follows the planner's import-only
  reach. `TestReachMatchesThePlannerGraph_V12727` passes (fixture with a test-only import and a
  path-token holder). The STR-V0-061 boundaries row (`session-test-runner-v0.md`, rule (d) row)
  names the shared reach. A replay over the PR #1816 diff was `NOT_RUN`: that diff exists only in
  the pre-snapshot lineage.
- V1-1470 criterion 1: `CEM-V1-245` retry ladder in `TestStableLifecycleOpTimeoutRetiresDescendants`.
  It passed 5 of 5 here with 60 busy loops running.
- V1-2036 criterion 1: decision 0553 and `TestPLEV0013CodecPackageAdmitsTestdata` (passes).
- V1-0775: `ATR-V0-241` records the limit, and `TestATRV0241_MappedWriteAfterLeaseAuditRefusedByNextWrite`
  proves the lease write commits under a mapped write on ext4 and tmpfs. Closing the window needs
  a content re-read under the lock, which `CAL-V0-263` forbids.

## Measurements

V1-1193, `CORVINT_DOCTOR_WALK_COST_PADDED=30000 go test -run '^TestCALV0360_PaddedDoctorWalkCost$'
./internal/tasks/cli/`, host load 30 falling to 4, best of five doctor runs (wall / CPU):

| store | unshared walks | shared walks |
|---|---|---|
| no read checkpoint (`journalAudit: FULL`) | 16.07 s / 25.17 s | 10.57 s / 16.98 s |
| writer-retained checkpoint (`CHECKPOINT_PLUS_TAIL`) | 6.93 s / 11.79 s | 1.23 s / 2.09 s |

Both scans stop at the 4,096-receipt bound with `truncated: true`, and the findings are identical.

V1-1470 criterion 2: the whole `interop/cem01-go` package under 60 busy loops (load average 46 to
62 on 12 CPUs) did not finish inside a 570 s command. 77 top-level tests completed (495 s
summed), 22 were still running at the cut, and 3 failed, all with `git-read-failed` from the
portable reader: `TestCEMV1080RubberStampAndCarefulBothAccept`,
`TestCEMV1082StableResultCarriesTheBlock` and `TestCEMV1092PortableCitationOnlyOnAccept`. All
three pass unloaded, and pass three times under the same load when run alone. The slowest under
load were `TestStableEnvelopeAdversarialShapes` 63.7 s,
`TestCEMV1084PortableReceiptAuthenticationPublicCase` 60.2 s, `TestCEMV1111CitationVectorsPortable`
57.2 s, `TestCEMV1633PortableReaderVerdictIgnoresReceiptOutcomeAndSelection` 50.7 s,
`TestCEMV1260DriftResourceLimitPortable` 44.2 s and `TestCandidateNormativePacket` 31.7 s. These
are the names V1-0849 needs; the lane did not write V1-0849.

## Live receipts

Claude Code 2.1.311, `claude mcp list` against local-scope entries for a scratch repository `$P`,
binaries `$BIN` built from this branch (removed afterwards):

```text
corvint: $BIN/corvint mcp --root $P --protocol-version 2025-11-25 --tool-profile test-validity,docs - ✔ Connected
corvint-docs: $BIN/corvint-docs-mcp --root $P --protocol-version 2025-11-25 - ✔ Connected
corvint-test-validity: $BIN/corvint-test-validity-mcp --root $P --protocol-version 2025-11-25 - ✔ Connected
corvint-default-docs: $BIN/corvint-docs-mcp --root $P - ✔ Connected
```

The fourth line connects without the flag because Claude Code 2.1.311 opens with
`server/discover` at protocol 2026-07-28 (captured through a stdin shim), not `initialize`. The
legacy flag in the plugin is therefore needed only by older Claude Code releases.

## Blocked

- V1-2507 criterion 3: no Pi client on this host (no Node, no `pi`), so no `pi mcp list` receipt.
- V1-2492: needs the Windows host's goroutine dumps and a Windows run.
- V1-2036 criterion 2: the protected enrollment is macOS-only (Apple CLT Git, darwin publishers)
  and needs an independently controlled root operator and accounts.
- V1-2691 criterion 2: needs 20 remote `windows-vet` runs on the WSL box, which this lane cannot
  reach.
- V1-0507, V1-0509: need Pi 0.99.x with the native Tasks build and live qualification. This host
  has neither Node nor Pi.
- V1-0775: the owner's `CAL-V0-263` trade (above).
