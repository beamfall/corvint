# 2026-10-06: V1-0891 CPU pressure signal and V1-0890 dispatcher config reload

## Intent

Ticket V1-0891, GitHub [beamfall/corvint#646](https://github.com/beamfall/corvint/issues/646) (P1):
on macOS the load average counts virtualization vCPU threads, so the pressure throttle held level 2
with idle CPUs. The issue asks for CPU utilisation from tick deltas inside the per-tick sample,
optional per-OS signal selection (for example memory only on macOS), and defined first-tick
behaviour. Ticket V1-0890, GitHub [beamfall/corvint#645](https://github.com/beamfall/corvint/issues/645)
(P2): apply a changed dispatcher configuration at the next tick, refuse an invalid one while keeping
the old one, let `cap: 0` disable a role, and add lane `minAgeSeconds`.

## Change

- Spec `docs/specs/corvint-tasks-agent-leases-v0.md` adds proposed CAL-V0-125..126 (V1-0891) and
  CAL-V0-127..129 (V1-0890) in two subsections with non-goals, failure modes, acceptance evidence
  and rollback, plus slices, failure-mode and traceability rows, the issue inputs and the delivery
  status (mirrored in `docs/specs/README.md` and `INDEX.json`). CAL-V0-052's role cap bound is now
  0..64 and CAL-V0-058's vocabulary gains `config`, each citing the proposed requirement. Acceptance
  is the owner's.
- Pressure (`internal/tasks/dispatch`): the Linux sampler parses aggregate CPU ticks from the
  `/proc/stat` bytes it already reads; the dispatcher derives utilisation from the previous recorded
  sample of the run, so the first tick after a start is UNKNOWN. `calmCpu`/`cpuHigh`/`cpuCritical`
  and `signals` (per OS) are optional `pressure` members; without `signals` the level uses the
  previous default signals. `cpu` is a valid level reason; status and `throttled` report
  `cpuUtilization`.
- Reload: `Dispatcher.WatchConfig` makes each tick re-read the file before observing. Refusals are
  deduplicated by digest, recorded in the ledger `config` record and reported by `dispatch status`;
  applies append a `config` event naming removed roles. The CLI planning pools follow the applied
  configuration. Role cap validation accepts 0; lane `minAgeSeconds` uses an in-memory episode clock
  keyed on member state and `changedSeq`, so the roster stays pure.

## Decisions

- macOS CPU ticks are not delivered. `host_processor_info`/`host_statistics` are Mach calls that a
  `CGO_ENABLED=0` binary can reach only through `go:linkname` trampolines, which the release gate
  refuses; no macOS sysctl publishes cumulative ticks. macOS cannot select `cpu` (validation
  refuses it) and the issue's macOS mitigation is `signals: {"darwin": ["memory"]}`. Enabling cgo or
  linkname is an owner decision.
- The previous sample is the delta baseline, not a separate counter store: it is already in the
  ledger, already cleared on restart, and keeps the sampler a pure per-tick read.
- A reload never touches running workers; `stateDir` and `workRoot` cannot change in a running
  dispatcher because the lock, ledger and store identity are bound to them.
- The member-age clock is in memory: a restart only delays a lane launch and no store timestamp is
  trusted as a dispatcher observation.

## Evidence

- `go test ./internal/tasks/dispatch` (full package, `GOMAXPROCS=3 -p 1`): PASS.
- `go test ./internal/tasks/cli` (full package, `GOMAXPROCS=3 -p 1`): PASS (237s), including the
  CAL-V0-125/127 status tests.
- `GOOS=linux go vet` of both packages and a `GOOS=windows` build of `dispatch`: clean.
- Not run: live Linux sampling, a live dispatcher under CPU saturation, a live reload against a real
  store.

## Follow-ups

- Service mode (`corvint-tasks service`) does not reload: its configuration is manifest-pinned.
- Owner decision on a cgo or linkname route for macOS CPU ticks.
