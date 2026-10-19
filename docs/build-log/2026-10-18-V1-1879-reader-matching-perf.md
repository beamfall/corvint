# 2026-10-18 V1-1879: path-literal reader matching without per-step rescans

Ticket: V1-1879 (affected and gateselect reader matching cost O(dirty × units × tokens) with a map
lookup per step).

## Decision

This is a performance-only change. The relations in `docs/specs/affected-plan-v0.md` (AFP-V0-012
rule (c), AFP-V0-021, AFP-V0-072, and the V1-0290 file-name-only rule for lone components) are
unchanged, so no requirement text changes.

- `internal/liveverify/affected` `readers()` now loops over units in id order. Each unit tries the
  sorted dirty paths in order and stops at the first one it resolves to. That path is still the
  smallest naming dirty path, so it is still the unit's witness. Narrowing still collects the
  frontier reasons of every naming path that did not resolve before the witness.
  - Each unit's tokens are split into component runs once, before the loop, through
    `pathTokens()`. The lone-component `nameOnly` test is decided once per token.
  - Each dirty path is split once.
  - This removes the old cost: one `runCache` map lookup and two string scans for every
    (dirty path, unit, token), plus a rescan of units that were already reached.
- `internal/gateselect` Select computes rule (c) for all dirty paths in one pass with
  `pathReaders`. That function uses the existing `pathMatcher`, which `-bounds` already proves equal
  to `namesPath`. The per-path `readers()` remains the documented relation and the test oracle.
  The CEM sidecar keeps `resolvingReaders`.

## Evidence

- Equivalence tests: `TestReadersMatchesDirtyMajorScan` and `TestPathReadersAgreesWithReaders`
  compare the new code with the previous algorithms on fixtures. The fixtures cover reached,
  narrowed, prior-reached, anchored, climbing, partial and sidecar cases.
- Output identity: the output is byte-identical between the base build (`6ceee260b`) and this
  change. This holds for `corvint affected --base` and `corvint test --dry-run --base`, on both the
  592-path HEAD~20 range and the 97-path HEAD~5 range.
- Timings (darwin, load average 11 to 22, so treat wall times as noisy):

| Measurement | Base | New |
|---|---|---|
| `affected` on HEAD~20, wall | 8.3–8.8 s | 3.6–5.0 s |
| `affected` on HEAD~20, user CPU | 7.5 s | 3.0 s |
| `test --dry-run` on HEAD~20, wall | 13.3–13.5 s | 4.7–5.0 s |
| `test --dry-run` on HEAD~20, user CPU | 15.8–16.0 s | 4.1 s |

- Profile: `affected.(*Graph).readers` takes about 0.10 s CPU, down from about 4.4 s before.
  `gateselect.pathReaders` takes 0.02 s. What remains is mostly the Go plugin's directory and
  source reads.
- Benchmark: `BenchmarkPathReaders` with 1,200 paths and 200 packages runs in 7.6 ms batched and
  76.5 ms per path.
- The ticket targeted ≤ 2 s wall at the original load (about 6). It is not reproducible at this
  host's current load. The CPU drop is the load-independent evidence.

## Rollback

Revert the change commit. Nothing persisted or on the wire changes.
