## 2026-09-26 decision 0423, V1-0358, V1-0318: delegated answers A7 to A10

The owner delegated A7 to A10 ("you can answer those questions for me"); decision 0423 records the
answers.

- A7 (V1-0358): `docs/BUILD-LOG.md` is closed to new entries and none of its lines move. Each new
  entry is its own `docs/build-log/YYYY-MM-DD-<slug>.md` file, not edited after it merges. There is
  no committed index; `rg -n '^## ' docs/BUILD-LOG.md docs/build-log/` is the index. `AGENTS.md`,
  `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/DOGFOOD.md`, `docs/README.md` and `ROADMAP.md` now point
  new entries there, each with its line count unchanged.
- A8: the 2026-09-26 audit report and raw artifacts stay private. Its entry is
  `docs/build-log/2026-09-26-comprehensive-audit.md`, naming the tickets instead of linking the
  report. Moving the uncommitted output out of the primary checkout is an owner step, and it
  unblocks the v0-6 promotion script.
- A9: beamfall/corvint#175 has a status comment and stays open; beamfall/corvint#170 closes with a
  comment when `1.0.0-rc.1` is published.
- A10 (V1-0318): `beamfall/corvint-tasks` is frozen with a README pointer after `1.0.0-rc.1` is
  published.

Follow-up: the frozen `benchmarks/daily-loop-v0` harness excludes only `docs/BUILD-LOG.md` from a
task's changed paths; a later corpus revision must also exclude `docs/build-log/`. Evidence: the doc
checks pass. Exhaustive gate `NOT_RUN` (documentation only). Rollback: revert this change.
