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
  refuses it) and the issue's macOS mitigation is `signals: {"darwin": ["memory"]}`. Coordinator
  decision (2026-10-06, review round 2): keep memory-only selection with `cpu` refused on darwin; no
  cgo and no linkname are added.
- The previous sample is the delta baseline, not a separate counter store: it is already in the
  ledger, already cleared on restart, and keeps the sampler a pure per-tick read.
- A reload never touches running workers; `stateDir` and `workRoot` cannot change in a running
  dispatcher because the lock, ledger and store identity are bound to them.
- The member-age clock is in memory: a restart only delays a lane launch and no store timestamp is
  trusted as a dispatcher observation (accepted by the coordinator in review round 2).

## Review repairs (round 2)

An independent review of the first commit found five defects; the coordinator decided each repair.

- Ledger readability: the strict progress reader did not list the `config` record, so any reload
  saved a ledger that `LoadLedger` refused once progress, sweep or retry records existed. The
  strict schema now covers `config` and its `refused` member.
- Supervision across a reload: supervision read the current configuration, so lowering
  `wallSeconds` killed running workers at once and removing a role dropped its deadlines. A reload
  now snapshots, per running worker, the configuration it launched under (in memory); wall, idle,
  host activity and kill grace follow it.
- Cap 0 starvation: a disabled role's pool stayed in the planned pools, so its higher-priority
  ticket held a `maxActiveAttempts` window an enabled role needed. Cap-0 roles are now excluded from
  the planned pools, the work-state hold and roster candidates.
- Status with an invalid file: `dispatch status` validated the file before reading the ledger and
  could not show the recorded refusal. It now recovers a single clean `stateDir` from the refused
  bytes, reads the ledger and adds `configFile` `INVALID`; unrecoverable files still refuse.
- CPU efficiency (coordinator decision: no CPU spent unless essential): samplers receive the
  selection and read nothing for unselected signals; `/proc/stat` is read only through its leading
  `cpu` lines (the long `intr` line is never copied or tokenized); CPU tick and utilisation fields
  and the first-tick problem are written only when `cpu` is selected, so default ledgers stay
  readable by older binaries and the downgrade note applies only to `cpu` opt-in.

## Evidence

- `go test ./internal/tasks/dispatch` (full package, `GOMAXPROCS=3 -p 1`): PASS.
- `go test ./internal/tasks/cli` (full package, `GOMAXPROCS=3 -p 1`): PASS (237s), including the
  CAL-V0-125/127 status tests.
- `GOOS=linux go vet` of both packages and a `GOOS=windows` build of `dispatch`: clean.
- Round 2 (review repairs): full `dispatch` package PASS (138s) and full `cli` package PASS (244s),
  `GOMAXPROCS=3 -p 1`; vet (darwin and linux) clean; doc gates clean. Each repair's new test was
  run against the pre-repair code and failed: the strict reader refused the ledger, a lowered wall
  killed the worker, a removed role was never killed, the disabled role's pool ticket took the
  window, status refused the invalid file, and the sampler read the whole `/proc/stat`, ran an
  unselected `sysctl` key and wrote CPU ticks to a default ledger.
- Not run: live Linux sampling, a live dispatcher under CPU saturation, a live reload against a real
  store.

## Follow-ups

- Service mode (`corvint-tasks service`) does not reload: its configuration is manifest-pinned.
