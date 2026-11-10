# Change proposal: `corvint watch` for the seven blocking runners, gated on save-to-answer time

Change name: `watch-seven-runners`
Ticket: none filed yet (to be filed in `.taskman/` before dispatch).
Product requirement served: PROD-TEST-007 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: a new spec `docs/specs/watch-v1.md` (prefix `WCH-V1`). It does not exist
yet; the lane creates it in its first commit. It amends the watch requirements of the line-precise
coverage spec (LPCV-V0-1086, LPCV-V0-1087) where stated in the spec delta.
Requirement IDs this change adds: WCH-V1-001 to WCH-V1-014. `WCH-` has no entry in
`docs/specs/REQUIREMENTS.tsv` (checked 2026-11-09 on `claude/layers-integ` at `9ec3a29d0`).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started (a Go-only watch exists; see Why)
Written before dispatch: yes.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`, `specs/watch/spec.md`).

## Why

G7: `corvint watch` must answer on save for all seven blocking runners (Go, Vitest, Jest,
Playwright, pytest, RSpec, Minitest; owner decision R7 (2026-11-09)) within p95 <= 1.0 s and
p50 <= 300 ms, measured from save to answer with a warm cache on named macOS and Linux reference
machines and a named reference repo, before release. The default output is the list of tests with
a ready-to-run command; `--run` runs them. Each answer carries its tier. macOS is proven by a real
run. The owner chose this against the panel's recommendation to gate only Go, Vitest and Jest; the
recorded risk is that the bar for Playwright, pytest and Ruby is unproven and may delay 1.0.

### Today's state, from the code at `9ec3a29d0`

- `corvint watch` takes a Go `linecov-index/4` file (`cmd/corvint/watch.go:26-27`) and refuses an
  index of another language (`watch.go:129`, refusal `watch-index-language`). Other suites are only
  reachable through `linecov watch -command` (`watch.go:48`).
- Events come from inotify or kqueue, or from `--events-stdin`; there are `--once` and `--dry-run`
  options, a replan time, and exit 2 on an event-source error (`watch.go:35-41`, LPCV-V0-1086 and
  LPCV-V0-1087).
- Unless `--dry-run` is given, watch runs the tests (`watch.go:173-190`). G7 makes listing the
  default.
- Neither `cmd/corvint/watch.go` nor `internal/linecov/watch` (`loop.go`, `relevant.go`,
  `source.go`, `source_darwin.go`, `source_linux.go`, `source_other.go`) references the
  `contextindex` snapshot. Today's watch does not use it.
- Performance scenario `watch-replan` (`internal/perfbudget/budgets.json:223-248`): 5 samples;
  target p95 1000 ms, advisory, profile experimental (PRB-V0-1014); enforced ceilings p95 2500 ms
  (three times a measured 752 ms p95 on an uncontended 32-core Linux host) and peak RSS 64 MiB.
  Scenario `watch-replan-self` (`budgets.json:250-276`): p95 ceiling 6500 ms (five times 1253 ms on
  this repository at `c0724a7fa`, about 9,900 files), RSS 128 MiB.
- The recorded measurement (`budgets.json:804-830`) runs
  `corvint --root <fixture>/code-app watch --index <fixture>/code-linecov.idx --once --dry-run`, with
  samples 696.74, 738.34, 697.81 and 752.46 ms. That is a cold process including start-up, not a
  save-to-answer time in a running watch, so it does not measure the G7 bar. Read as it stands, its
  median (about 700 ms) is above the 300 ms p50 bar.
- A planning note records that the kqueue tests have never run on macOS
  (`panels/milestone-fixes.md:4`, scratch planning material; NOT_VERIFIED against CI history).

## What Changes

- `corvint watch` answers for all seven runners from one running process, using each runner's
  contract implementation (change `runner-contract`) and the affected engine. Each answer names the
  runner instance keyed by root (owner decision R15 (2026-11-09)) and uses canonical test IDs.
- Default output: the selected tests, their tier (`line | file | flow | full`), the `exact` flag
  and a ready-to-run command (owner decision R16 (2026-11-09)); pytest answers carry `file` (owner
  decision R8 (2026-11-09)). `--run` runs them. `--dry-run` is removed with no alias:
  `cmd/corvint/watch.go` is absent from `origin/main` (`9684e632e`) and from `v1.0.0-rc.2`, so no
  released script depends on it (settled, testing S2).
- Watch reads the committed inventory and never refreshes it; absent or stale inventory answers
  `inventory-absent` or `inventory-stale` (owner decision R10 (2026-11-09)).
- Kill switch with the `full:<runner>` grammar of NMS-V1-008; an unknown value exits 2 (owner
  decision R13 (2026-11-09)).
- A new performance gate measures save-to-answer time in a warm, running watch per runner on the
  reference machines and repo. The `watch-replan` target moves from advisory to enforced.
- The `contextindex` snapshot is not part of watch. It is kept as a warm cache only if a
  measurement shows it is needed to meet the bar (G25 item 5).

## Capabilities

### New Capabilities

- `watch`: multi-runner save-to-answer watch.

### Modified Capabilities

- `line-precise-coverage` (LPCV-V0-1086, LPCV-V0-1087): watch is no longer Go-only and no longer
  runs tests by default.
- `performance-budgets` (PRB-V0): new save-to-answer scenarios with enforced targets.

## Non-goals

- The time to run tests (G7 measures time to the list).
- A permanent daemon (invariant 7): watch is started by the user and ends with the terminal.
- Refreshing the inventory (change `test-inventory`).
- Cypress: dropped from 1.0 and listed as post-1.0 (owner decision R6 (2026-11-09)).

## Impact

- Code: `cmd/corvint/watch.go`, `internal/linecov/watch`, `internal/perfbudget/budgets.json`, the
  runner contract, a save-to-answer timing harness.
- Reference machines (panel proposal, owner to confirm): an M2-class Mac with 16 GB and an 8-core
  Linux host with NVMe storage. Reference repo (panel proposal): about 500k lines, 5k test files and
  50k tests. On WSL only the ext4 filesystem is supported; `/mnt/c` falls back to
  `--events-stdin`.
