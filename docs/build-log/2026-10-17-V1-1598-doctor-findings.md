# V1-1598, V1-1601, V1-1604: doctor operational, notify and host-session findings

Tickets V1-1598 (#1041), V1-1601 (#1044) and V1-1604 (#1047), with the addenda V1-1192, V1-1193,
V1-1194 and V1-1519. Spec: `docs/specs/corvint-tasks-doctor-v0.md` TQD-V0-020..033 and a TQD-V0-001
amendment (`--config`, the item's `dispatchers`). TQD-V0-025 is unused.

## Acceptance

The owner's standing auto-accept of lane requirements (2026-10-11, recorded in the owner
acceptances of that date) covers TQD-V0-020..033. V1-1519 is carried, per its ticket, as an approved TQD-V0-001
amendment for the dispatcher work-state health report only; the leases spec
(`corvint-tasks-agent-leases-v0.md`) is not edited.

## Decisions

- One operator file, `--config FILE` (`taskman-doctor-config/0`), carries every new threshold, the
  notify hook and the dispatcher list. The defaults are the issue's: lock and slow command 60 s,
  no completions 2 h, file table 80%, host thresholds 3, session window 1 h.
- Lock detection reads this queue's two lock files under the git common directory and only
  `corvint-tasks` processes that have them open, so other queues on the host are never named. Linux
  names the holder from `/proc/locks`; darwin's `F_GETLK` cannot name a flock holder, so the
  longest-running opener is named and its process age stands for the hold age (recorded residual).
- NO_COMPLETIONS needs a claimable ticket and an idle lane; its reference is the newest recorded
  completion, else the earliest scanned receipt, so an empty or busy queue reports nothing.
- The notify hook runs only inside `--refresh`, under the cache lock, keyed on a digest of the
  finding keys stored in the cache's `notify` member. A set that already ran never reruns, whether the
  run failed or not; a failure is a NOTIFY_FAILED finding re-emitted by plain `doctor` until a later
  run succeeds. The hook reuses the plugin runner bounds (process group, 10 s, 64 KiB).
- Host findings come only from `dispatch.ClassifyHostLine`, a closed classifier over the existing
  usage formats (codex, opencode, claude-code). An unknown format or line yields nothing. Each
  finding cites the `stdout.log` path and the line range of the counted events.
- V1-1192: NO_PROGRESS_HANDOFF counts only generations at the ticket's current acceptance revision,
  the same reset rule as the loop hold (CAL-V0-103). A reopen therefore starts a new streak.
- V1-1519: the dispatcher's `state.json` is read with `dispatch.LoadLedger`, keyed by the program
  directory's base name. Its `WorkState` is reported as OK, FAILING, NOT_OBSERVED or UNREADABLE.
  A failing reader is also a WORK_STATE_FAILING finding.
- V1-1193: profiling a 607-receipt synthetic store showed about 92% of the doctor's time in
  `journal.nativeRead.Read`. Of that, 0.82 s of 1.03 s went to re-resolving the state roots on each
  read. The new `journal.Native.Session()` pins the parents once per scan.
- V1-1194: `docs/TASKS-EXTERNAL-AGENTS.md` now says `doctor --line` reads only the `--refresh`
  cache and never scans (TQD-V0-012).

## Measurement (V1-1193)

`CORVINT_DOCTOR_SCAN_COST=300` builds 300 claim/release cycles (607 receipts) and runs ten
interleaved doctors. Per-read and one-session runs alternate. Host load was 40-60.

| Access | Best wall | Best CPU |
| --- | --- | --- |
| Per-read (before) | 3.04 s | 792 ms |
| One session (after) | 2.27 s | 529 ms |

Findings and `scan` were identical. Wall time is dominated by host load: an earlier quiet-ish run of
the old code measured 470 ms wall. CPU is the steadier comparison, at -33%. Scan results are not cached
between runs. Whether a cross-run cache is needed is left open; the scan stays bounded at 4096
receipts and 7 days (TQD-V0-004).

## Residuals

- Darwin lock holder approximation (above). Commands that never open a lock file are not seen.
- Linux `/proc/locks`, `/proc/<pid>/fd` and `file-nr` paths are covered only by parser fixtures.
  This lane ran on darwin.
- The notify hook holds the cache lock while it runs, so a concurrent refresh can refuse
  LOCK_TIMEOUT.
- Two detectors cannot fire for some formats, because those vocabularies lack the events:
  EARLY_REFUSAL for opencode and HOST_RETRY_STORM for claude-code.
- At host load 75-100 the darwin `lsof` opener probe often exceeds its 5 s bound. The doctor then
  reports a `scan` warning and no lock finding. `TestTQDV0021_RealProbes` retries up to three times;
  if every try is incomplete it asserts that no finding was invented and skips the positive case.
  A busy operator host can therefore miss QUEUE_LOCK_HELD.
- `TestTQDV0010_PluginFindings`, which predates this change, fails at that load. It fails the same
  way on base `55abb815c`: every plugin, including the passing one, hits the test's 500 ms timeout,
  because a fresh script's first exec took 1.4 s. The new notify test keeps the default 10 s timeout.
- `taskman-doctor-config/0` joins `LiveFormats` (CAL-V0-131). The `version` format set changes, so
  replacing an older build in place needs the usual drain (CAL-V0-130).

Rollback: revert the change. The cache's `notify` member and the config file are optional, and the
journal session is additive.
