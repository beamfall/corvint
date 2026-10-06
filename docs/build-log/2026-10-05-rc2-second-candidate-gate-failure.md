# 2026-10-05: 1.0.0-rc.2 second candidate fails the darwin gates

## Result

Candidate `da78c1572211a85e6c9944adb98350d69964eb9b` (build 358, PR #617) failed two gates and was
not tagged:

- the release runbook's step-4 `make gate` on darwin/arm64;
- the hosted macOS `full-gate` job (Release gates run 37381427566).

Every other hosted job passed, including the ubuntu `full-gate` that failed the first candidate (see
`2026-10-05-rc2-first-candidate-gate-failure.md`). Every other runbook step passed except one HLQ tuple
(item 4 below):

- archive, checksums, reproducibility;
- lifecycle, N-1 lifecycle, hostile regressions and `core-n1-replay` on darwin/arm64, darwin/amd64
  and linux/arm64;
- interop gate, focused docs, legal files, candidate checks.

Its evidence is retained under the private release-evidence directory `1.0.0-rc.2-da78c157`. A third
candidate will be cut from main once the fixes below merge.

## Defects

1. **Darwin change guard exhausts descriptors (V1-0841, product).**
   - `authority.WatchChanges` holds one `O_EVTONLY` kqueue descriptor per watched path, and it is new
     since rc.1.
   - On a 5000-receipt store, `TestCALV0026_AuditInventoryParityAndFallback` and
     `TestCALV0026_GuardedInventoryPopulated5000` failed with `too many open files`.
   - Go raises the soft `RLIMIT_NOFILE` only to `kern.maxfilesperproc`, which is 10240 on the hosted
     runner. Hosts with a higher limit, like the local release host, hide the defect.
   - Lease reads refuse when the watch fails, so a large store on a default macOS host would refuse.
   - Fix: regular-file descriptors now come from a process-wide budget of half the soft limit. Files
     past the budget are tracked by their stat tuple (device, inode, mode, size, mtime and ctime in
     nanoseconds), checked at registration and on every poll; directories are always watched with a
     descriptor.
   - The `corvint-tasks-agent-leases-v0` "Descriptor budget (macOS)" paragraph states the fallback's
     limit: a write past the budget that preserves size and both timestamps is caught only by the
     content digest.
2. **Symlinked TMPDIR in `internal/localcompletion` (V1-0842, test-only).** This is the same class as
   the first candidate's V1-0840 and V1-0753. Three aggregate tests failed under the macOS default
   TMPDIR because local state refuses a symlinked ancestor. The package's `TestMain` now resolves
   `TMPDIR`. The first candidate's gates stopped before reaching this package.
3. **S0E lingering-descendant case misattributes host fork refusal (V1-0843, test-only).**
   - On the hosted runner, `normal-exit-lingering-descendant-contained` failed with `git-diff-failed`
     after 8 operations.
   - The case's shim forks a background sleeper before `exec`ing Git. If the host refuses that fork,
     macOS `/bin/sh` exits 128 before the exec, and the product correctly reports a failed Git.
   - Capping `RLIMIT_NPROC` reproduces the exact signature. Why the runner refused the fork was not
     observed.
   - The shim now records the sleeper's PID and its own stderr. A missing PID is reported as a
     harness precondition failure, and the sleeper's retirement is asserted after the run. A
     fork-starved host still fails the case.
4. **Intermittent `hlq-claude-code` frontier (V1-0844, open).**
   - The runbook's Claude Code HLQ tuple failed once: an enrolled incomplete Stop returned no decision.
   - The product source matches the first candidate, which passed. Three reruns on this candidate
     passed under heavy host load.
   - The harness discards the hook's output, so the suspected cause (the 1600 ms event deadline
     failing open) is not yet evidenced. The tuple is rerun on every candidate.

## Hollow linux/arm64 evidence

The runbook's linux/arm64 container step mounted the candidate clone and N-1 archives from
`/private/tmp`. Docker Desktop does not share that path, so the mounts were empty and every step in
the container exited 127.

- The first candidate's step reported success in one second without running anything. Its
  linux/arm64 lifecycle, N-1 and hostile evidence is void.
- The second candidate's first container run failed the same way.
- The script now stages both inputs under the shared evidence directory. Rerun on this candidate,
  the lifecycle, N-1 lifecycle and hostile checks each exited 0. The mountless output is retained
  beside the passing one.
