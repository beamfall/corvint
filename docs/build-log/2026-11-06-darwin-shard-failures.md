# V1-4701, V1-4734, V1-4760, V1-4761: macOS shard failures on main

Date: 2026-11-06. Root-cause pass over the go-macos shard failures in runs 42221230970,
42219240909 and 42212262974. Baseline: origin/main `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`.
Reproductions ran on the Mac (macOS 26.6, Go 1.27.3) through `script/heavy.sh` with
`HEAVY_REMOTE=0`. Governing contracts: LPCV-V0-950/954 (linecov collection, unchanged),
ATR-V0-7203 (locked lease content binding, unchanged) and its prose in
`docs/specs/corvint-tasks-agent-leases-v0.md` (amended, no requirement ID added or renumbered),
and the Kotlin candidate's non-promotional paired latency guard
(`docs/specs/kotlin-android-candidate-v0.md`, unchanged).

## Already fixed on main (V1-4701, V1-4734)

`internal/cemchain` TestCEMV1286, `internal/tasks/cli` PURV02211, TCLV0802, TCLV0869 and
TCDV0770, `interop/cem01-go` TestCEMV1753, and the `tools/local-authority` umask test were
repaired by `79ef4205e` (build-log `2026-11-06-darwin-gomacos-fixes.md`). Runs 42221230970
and 42219240909 tested PR heads that do not contain that commit. Run 42212262974, at main's
`2e09023b`, passed all of them on macOS 15. That is also the first macOS 15 pass of the
V1-4734 publication fix. This change does not touch them.

## Causes and decisions

- `internal/linecov` TestInOrderWindows (V1-4701): a **product** defect. `Collect` built the
  `-overlay` keys from the root it was given, `/var/...` under a macOS temporary directory.
  `exec.Cmd` with a non-nil `Env` passes a stale `PWD`, so the go command uses `getcwd`, which
  returns `/private/var/...`. The keys never matched, so the window overlay was silently
  ignored and every in-order record fell back to `window-order-unavailable`. Any linecov
  collection on a symlinked root (a real macOS checkout under `/var` or any linked path) lost
  its test windows. Decision: `Collect` resolves the root with `filepath.EvalSymlinks` before
  it derives any path, and returns the error if resolution fails.
- `internal/tasks/store` TestCALV0026_InPlaceIntentEditAfterCommitSweep/lock (V1-4701): the
  **test** was stale. It recorded the old CAL-V0-026 window, in which an in-place edit after
  the commit sweep let the lease commit and only the next audit refused. ATR-V0-7203 added
  the locked content binding in `commitLease`, which now refuses the same lease with
  `INTENT_DIVERGED` and publishes no receipt. That is the stronger behaviour the mapped-write
  test already requires on Linux. The darwin-only test never ran on Linux CI, so it went
  stale while ATR-V0-7203 recorded macOS as NOT_RUN. The test now requires the same-lease
  refusal, an empty receipt, and the next-audit refusal. The spec paragraph that described
  the superseded window now describes the binding.
- `internal/analyzerkotlinandroid` TestRepresentativeAllocationRatchetAndRestoredDecoder
  (V1-4761, suspected): wall-clock noise in the **test**. In the failing run the benchmark put
  the candidate at 0.52x the restored decoder's ns/op, and its allocations were lower (52 vs
  73 allocs, 146,656 vs 280,352 B). The paired guard summed 512 timed calls per side and
  reported 3.2x (568 ms vs 179 ms). Two wall-clock views of the same code in one process
  disagreed by 6x. A sum lets one sub-millisecond call that the host descheduled decide the
  comparison. Decision: keep the 2x threshold, the pairing and the allocation caps, and
  compare each side's median per-call latency instead of the sum. A real regression slows
  every call, so it still moves the median.
- `cmd/corvint` package timeout (V1-4760): **not a hang**, and not reproduced as one. In both
  timeout dumps the running leaf tests had been running 4 to 16 s. No goroutine was blocked
  in exec, pipe or syscall waits for a minute or more, apart from the `os/signal` loop. The
  long waits were all `testing` parents waiting for parallel subtests. 1,508 and 1,490 of
  1,536 top-level tests had finished, and test results kept arriving at up to 205 a minute
  until the alarm fired. The test named in the panic only marks where the suite was at 30 minutes, so
  it differs per run. Cause: `.github/macos/shard.py` runs whole packages with
  `-p 3 -parallel 2 -timeout 30m` on a 3-core runner. cmd/corvint's Linux cost history is
  1,127 s, and here it shares the cores with packages such as `internal/liveverify/mutate`,
  which spawns its own `go test` runs. Decision: no product or test change. Decision 0082
  makes 30 minutes a hang detector, so raising it would be the wrong fix. The workflow fix
  is to give cmd/corvint its own invocation, or split it. That is a `.github` change and
  needs its own PR. It is left to the coordinator.

## Verification and limits

- linecov: before, TestInOrderWindows FAIL locally with `window-order-unavailable`. After,
  PASS with `-count=3`, and the whole `internal/linecov` package passes (24 s).
- tasks/store: before, the `lock` subtest FAIL 5 of 5 with `INTENT_DIVERGED`. After, every
  `TestCALV0026` and `TestATRV07203_MappedWriteAfterLeaseAuditRefusesCommit` PASS with
  `-count=3`.
- Kotlin ratchet: the CI failure did not reproduce locally. Before the change, 10 runs
  passed: the paired sums were 1.08 to 1.11x on quiet runs and 0.77 to 1.20x overall, while
  benchmark ns/op for the same decoder varied from 125 to 323 µs. After, 3 runs passed at median ratios of 1.07
  to 1.09x, while benchmark ns/op swung from 138 to 402 µs. In a temporary mutant the
  candidate did 3x the work, and the guard still failed it (420 µs vs 122 µs).
- cmd/corvint: evidence comes only from the retained timeout dumps. The workflow change is
  not made or measured here.
- `corvint test --base 8fc0cf37642e7012f4c26671c9ed97a8706d6e09 --dry-run` selected 59
  whole packages, mostly because of the spec edit. Under the standing lane rules only the
  focused runs above were made, and batch CI covers the rest.

Rollback: revert this commit. That restores the root as given in `Collect`, the summed
latency guard, and the stale CAL-V0-026 expectation, which fails on darwin again.
