# 2026-10-24 p1-cce-2: CCE block vocabulary and API contract reach

Lane p1-cce-2, stacked on p1-cce (#1702, head c4ec0f3fd7fa7e1e22d9e91a3ec82b8c2d9a76a4). Tickets
V1-1684 (rest of the block vocabulary) and V1-1685 (API contract joins) under umbrella V1-1666, which
this lane does not claim. New requirements `CCE-V0-105..115` are proposed and experimental under
the owner's standing auto-accept (2026-10-11); `116..129` stay free in this lane's reservation.

## Problems

- **V1-1684.** The compiler emitted bare `test()` bodies: a fixture could not hand a value to a
  later block, a precondition only a signed-in user meets had no block, cleanup ran inline and was
  skipped after a failing step, outcomes and inputs could only be test IDs, an entry needing URL
  parameters stayed `requires-parent-params`, and nothing pinned the rendered output.
- **V1-1685.** Reach had impact, runtime coverage and the module graph; a backend change that no
  front-end file imports still read only `unplaced-path`, though the repository can name the
  endpoint the screen calls.

## Decisions

- **One optional member per addition, closed decoding.** Fixture `outputs`, `sessions`,
  `locators`, `routes` and `api_contracts` are optional members of `e2e-block-declarations/0`. An
  old file reads unchanged; an older binary refuses a file that uses them instead of dropping them.
- **Data flows only through declared outputs.** A route parameter names a fixture and an output;
  the compiled fixture checks the output exists before the body runs, so a seeding failure reads
  `fixture-output-missing`, not a behaviour failure. A scenario that does not set the fixture misses
  the path with `route-param-unbound`.
- **Fixtures are `base.extend` fixtures.** Teardown after `use` runs whatever the body does. A
  declared method is callable only with no parameters (appmap `Method.Callable`), so cleanup reads
  its state from the page object rather than from the setup's return value.
- **No vacuous omitted-event control.** Removing a setup whose outputs a later block reads fails the
  output check by construction; it would show nothing about the setup's effect, so that control is
  not generated and the prove limitations say so.
- **Sessions never enter the scenario text.** The storage-state path comes from a declared
  environment variable; the scenario skips when it is unset, and the run declares the variable to
  the provider only when it is set, so the receipt records it.
- **A route's verdict replaces the entry's reason.** Before, a rejected route appended a second
  unadmitted row and the compile miss still read `requires-parent-params`. A guarded entry keeps
  its more specific `guarded(...)` reason.
- **Goldens, normalised only where revisions appear.** Four goldens pin the rendered text at
  `corvint-test-e2e-compile/1`; the CCE-V0-100 recordings were regenerated for the new compiler.
- **API contracts are read at base and only widen.** As with `reach_exempt`, a diff cannot declare
  its own join. Hits are not counted as placed and add no span on a screen without an outcome, so
  the changed path's own `unplaced-path` span stands. The endpoint literal matches only when not
  followed by an identifier byte. Over 512 placed files the join adds nothing.
- **One reason per span is not a demotion.** CCE-V0-115's first draft required every baseline
  unknown to keep its reason; with coverage on every screen, the ranked `oneReasonPerSpan` restated
  `unplaced-path` as `screen-without-outcome`. The span stayed unknown and the state `UNKNOWN`, which
  is the property; the test checks spans, and CCE-V0-114 separately checks the joined path keeps
  `unplaced-path`.

## Evidence

- linux/WSL2 (`heavy.sh`, go1.27.3): `./internal/changee2e`, `./cmd/corvint-test`,
  `./internal/specindex` PASS; darwin/arm64 focused runs PASS.
- Live Playwright for sessions, teardown and typed locators: NOT_RUN (no qualified Playwright
  1.64.0, Node v24.11.1, headless shell 1248 tuple on this host). The inferences that a file-level
  `test.skip(cond, desc)` and a fixture teardown failure located at the cleanup line behave as
  modelled are unverified live.

## Rollback

Remove `api_contract.go`, the new declarations members and their tests and goldens, and restore
`CompilerVersion` `corvint-test-e2e-compile/0` with the `/0` recordings. No stored state migrates.
