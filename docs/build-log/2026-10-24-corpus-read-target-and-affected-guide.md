# Warm corpus read target on Linux, and the affected cache in the agent guide

Date: 2026-10-24. Lane: claude/p1-perf-1024, based on `origin/main` 0949876eb. This entry records
the measurement of V1-2477 target 1 (GitHub #1598 residual) and adds `AFP-V0-431` (proposed) for
the GitHub #1601 re-test's remaining point. No read-path code changed.

## V1-2477 target 1: warm CLI `flow` and `gaps` under 200 ms median

Fixture: the `DCP-V1-418` generator (`TestWriteLargeReadFixture`), 66,752,222-byte artifact,
rebuilt on each host by that host's binary built from a clean 0949876eb tree. Each run is a new
process: `corvint --root APP docs corpus flow --id adapter:lib0-1|gaps --artifact corpus-cli.json
--timings`, the two operations alternating.

| Host | Load | Op | n | total p50 | total p99 | parse p50 | freshness p50 | index p50 | peak RSS max |
|---|---|---|---|---|---|---|---|---|---|
| linux/amd64 WSL2, 32 threads | 4.5 | flow | 20 | 184.6 ms | 210.7 ms | 148.9 ms | 88.2 ms | 33.6 ms | 194 MB |
| linux/amd64 WSL2, 32 threads | 4.5 | gaps | 20 | 194.3 ms | 406.1 ms | 150.4 ms | 87.1 ms | 32.6 ms | 203 MB |
| linux/amd64 WSL2, 32 threads | 0.8-2.4 | flow | 20 | 180.4 ms | 291.4 ms | 147.4 ms | 80.7 ms | 30.0 ms | 203 MB |
| linux/amd64 WSL2, 32 threads | 0.8-2.4 | gaps | 20 | 188.7 ms | 274.6 ms | 146.5 ms | 86.5 ms | 29.9 ms | 223 MB |
| darwin/arm64, 12 cores | 18-20 | flow | 6 | about 290 ms | 423 ms | about 240 ms | about 240 ms | 20-44 ms | 215 MB |

With n=20, p99 is the maximum. Peak RSS stays within the `DCP-V1-430` bound of 3.5 times the
artifact (234 MB) on every run above.

Target 1 is met on the quiet Linux host, but `gaps` has only 6 to 11 ms of margin. It is not met
on the darwin build host at load 18 to 20, where parse and freshness each take 200 to 260 ms. The
darwin figure is host contention: the same binary and fixture run 1.5 times faster on the Linux
host, and the darwin numbers spread 213 to 423 ms between consecutive runs.

## Where the Linux read spends its time

A temporary probe binary (not committed) printed the elapsed time inside
`decodeArtifactStream` on the Linux host for eight `gaps` reads:

- the single-goroutine scan of the 66.75 MB input: 89 to 99 ms;
- waiting for the decode workers after the scan: 10 to 15 ms;
- concatenating the split chunks: 6 to 7 ms;
- the skeleton comparison: 4 to 5 ms.

Then `index_ms` takes about 30 ms and the query about 12 ms. Freshness (50 to 90 ms) overlaps the
parse phase and is off the critical path on Linux.

A CPU profile of 12 `flow` reads (`CORVINT_CPUPROFILE`, `PRB-V0`) shows 1.45 s of CPU per read
spread over the workers. `deriveClaimTerms` accounts for 21%, `EncodeTo` for the canonical check
19%, and GC mark 8%. The scanning goroutine is 88 ms of CPU per read: `scanValue` and `scanString`
62 ms, and dispatch, chunk allocation and copy the rest. So the critical path is that sequential
scan. More decode workers or a faster `deriveClaimTerms` do not shorten it.

On darwin the same profile spent 27% of read CPU in `madvise` from `allocSpan`→`sysUsed` (fresh
spans for 238 MB of allocation per read). Changing GOGC was tried again: 100, 400, and off with
GOMEMLIMIT=200MiB, six runs each at load 18 to 20. The differences were inside the run-to-run
noise, and RSS rose to 251 MB, above the bound. The earlier lane had already dropped this lever
(`2026-10-23-corpus-read-start-freshness.md`).

## What would close target 1 robustly, and why it is not done here

- **A parallel or vectorised boundary scan.** This is the remaining sequential phase. It must
  keep the closed-decoder refusal order (`DCP-V1-431`) and the exact byte-equality argument
  (`DCP-V1-414`, `DCP-V1-419`) across chunk boundaries: a string can span a split point, so each
  split must be resolved before its elements are dispatched. That is a decoder design change
  needing its own requirements and adversarial tests. It is NOT MET in this lane.
- **Freshness reuse across CLI processes** (the #1598 re-test's item 2 on darwin). This would need
  a persisted stamp, which is new writable state on a read (invariant 4). It is NEEDS-OWNER.
- **A sidecar binary index keyed by the artifact's sha256** (the re-test's item 3). This is a new
  on-disk encoding that the index format gate would have to admit (invariant 7). It is NEEDS-OWNER.

## GitHub #1601 re-test: the agent guide names `--cache`

The re-test on main-368b80d found the warm `--cache` path met the target (1.8 s against 8.6 s
uncached). It asked that the agent guide at least say to pass `--cache`, since reuse is opt-in.
`AFP-V0-431` adds that advice, together with the batch form, to the guide's test-selection goal.
It is checked by `TestAFPV0431ShippedGuideAdvisesBatchingAndTheAffectedCache`. A default cache
location remains a non-goal of `AFP-V0-390..397`: it would write state from a read command
(invariant 4) and is NEEDS-OWNER. A possible design is a cache under the repository's Git common
dir or the user cache dir, keyed as `--cache` is now.
