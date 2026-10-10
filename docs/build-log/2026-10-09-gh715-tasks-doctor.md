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

Limits:

- The scan does not read the dispatcher ledger.
- The Linux `ps` walk is unqualified. Tests ran only on Darwin arm64.
- Plugins are not sandboxed.
- The cache can be stale. `--line` shows its age once it is 15 minutes old.
- Live qualification against a large real store is NOT_RUN.
