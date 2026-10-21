# hooks-1021: dogfood-event snapshot prefetch (V1-1524, V1-2144) and Gemini EPERM re-probe (V1-1122)

Date: 2026-10-21. Lane: `claude/hooks-1021` (base `f284484eb`). Requirements `LCP-V0-033` and
`AHI-058` are accepted under the owner's standing lane auto-acceptance of 2026-10-11.

## Problem

The primary checkout's self-observation ledger keeps reporting `dogfood-event-deadline` fallbacks for
the Claude Code adapters (rows deduplicated hourly: `pre-compact` 30, `user-prompt` 34, `session-start`
35) while the worktree carries about 1,270 dirty `.taskman/` projection paths (708 modified tracked,
248-759 untracked). V1-1524 asked which phase dominates; V1-2144 asked that `pre-compact` emit its pin
inside the 2 s host kill in at least 99% of runs on a worktree with more than 700 dirty tracked paths.

## Reproduction and per-phase timings

A plain clone at the base with 708 modified tracked and 759 untracked `.taskman/` paths (1,467 dirty;
the snapshot indexed for each binary's engine) on the 12-core lane host. A temporary,
never-committed phase-instrumented build (`CORVINT_PHASE_LOG`) of the `user-prompt` adapter at load
about 80, in milliseconds from process start:

| Phase | Typical | Range |
|---|---|---|
| process start to event read | 11 | 6-12 |
| opening observation (rev-parse identity + isolated `git status`) | 200 | 200-300 |
| policy evaluation | 1 | 1 |
| snapshot load (170 MB gob decode + SHA-256 trailer) | 370 | 270-390 |
| context compile | 6 | 5-6 |
| closing observation | 235 | 230-370 |
| total to emit | 830 | 800-900 |

Cause, measured: the two status observations and the snapshot decode together are about 95% of the
event, and all three are CPU-bound work that host load stretches. The dirty count is not the cause:
raw `git status --porcelain=v1 -z --untracked-files=all` costs 0.13-0.23 s on the clean and the
1,467-path dirty clone alike (untracked cache and fsmonitor are off, so the whole worktree is scanned
either way). The base already met the 1.6 s event deadline in every run here: 40 sequential
`pre-compact` runs at load up to 150 gave p50 0.931 s, p90 1.356 s, max 1.723 s (the max is the adapter
process wall, still inside the 2 s kill). The production fallbacks are therefore load spikes on a host
that ran 5-7 lanes at load 30-150, not a deterministic overrun.

## Decision

Keep failing open (the AHI-055 notice text already names heavy machine load as a cause and stays
unchanged), and remove the largest serial step: `LCP-V0-033` starts the snapshot decode as soon as the
opening observation's identity read returns, overlapping that observation's status read. The bracket
hands the identity it already read to the prefetch (`gokernel.ProbeRepositoryAroundPrefetching`), so
`LCP-V0-016` still holds: four Git spawns, and the loader takes no identity or status read of its own.
The decoded index takes the complete opening observation afterwards (`Index.AdoptObservation`, the
same assignment `LoadSnapshotObserved` makes), so the answer bytes are unchanged. A prefetch for
another object format or tree is not used. A separate rev-parse for the prefetch was rejected because
it would add a fifth spawn and break `LCP-V0-016`.

## Evidence (A/B, interleaved, same clone, adapter process wall time)

`corvint adapter claude-code <event>` with the hook's JSON on stdin, base and fix binaries alternating
each iteration:

| Event | Runs each | Load | Base p50 / p90 / max / mean (s) | Fix p50 / p90 / max / mean (s) | Over 1.6 s |
|---|---|---|---|---|---|
| `pre-compact` | 100 | 31-41 | 0.879 / 0.996 / 1.444 / 0.889 | 0.676 / 0.787 / 1.469 / 0.704 | 0 / 0 |
| `user-prompt` | 50 | 31-41 | 0.728 / 0.892 / 1.206 / 0.752 | 0.607 / 0.774 / 1.038 / 0.627 | 0 / 0 |

Every run emitted a non-degraded answer; the base and fix outputs are byte-identical (shasum) for
both events. The median `pre-compact` cost fell 23% (about 200 ms of headroom under the deadline).
V1-2144's acceptance bar holds in this reproduction: 100 of 100 fixed `pre-compact` runs on a worktree
with 708 dirty tracked paths emitted the pin inside the 2 s kill (and 40 of 40 base runs at load up
to 150). It does not prove the 23% production miss rate is gone; that depends on host load, which
V1-1627/V1-1628 govern.

Tests: `TestProbeAroundPrefetchingOverlapsTheOpeningStatusRead` (gokernel; failed with the identity
callback moved after the status wait: "the opening status read finished before prefetch started"),
`TestDogfoodEventPrefetchesTheSnapshotFromTheOpeningIdentity` (failed with the prefetch key removed:
one load against the complete observation instead of the identity), `TestSnapshotPrefetchServesOnlyItsOwnTree`,
and the unchanged `TestDogfoodEventSpawnsOneBracket` (four spawns), the drift refusal and the
checkpoint snapshot-seam guards; the cmd tests also pass under `-race`.

## V1-1122: Gemini cleanup EPERM re-probe

The Gemini hook confirmed cleanup with one `kill(-pid, 0)` and accepted only `ESRCH`, so a zombie-only
group (Darwin answers `EPERM`) read as `corvint-process-cleanup-unconfirmed`, the defect `AHI-052` fixed
for OpenCode. `AHI-058` gives Gemini the same bounded re-probe: up to 40 repeats, each a 5 ms timer so
a poll phase can reap the leader, signal 0 only. The deterministic test injects the answers through a
preload; on the base hook it reported the degradation for the three-`EPERM` case, after the fix it
reports the timeout cause after four probes and the degradation after exactly 41.

## NOT_RUN

- A hosted macos-15 `host-adapter-test` run for `AHI-058` (CI on the PR covers it).
- The `pre-compact` miss rate in live Claude Code compactions after the fix (needs installed-host use).
- `make gate` (owner preference for scoped work).

## Rollback

`LCP-V0-033`: call `gokernel.ProbeRepositoryAround` from `localEventRead` again and drop
`dogfoodSnapshotPrefetchKey`/`snapshotPrefetch`; `ProbeRepositoryAroundPrefetching` and
`AdoptObservation` can stay as unused helpers or be removed. `AHI-058`: restore the single probe in
`cleanupConfirmed` and remove the V1-1122 test.
