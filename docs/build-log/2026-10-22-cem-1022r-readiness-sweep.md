# CEM 1.0 readiness lane 1022r: anchorable warnings, seal identity, frontier site counts, catalog drift gate

Lane `cem-1022r`, branch `claude/cem-1022r`, base `6e90476ec`. Seven tickets were screened. New requirements
DCW-V0-165, DCW-V0-166, AFP-V0-381 and CAP-V0-007 to CAP-V0-009, plus the CAP-V0-006 read-scope amendment,
are accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

- **V1-2367, V1-2379.** `dogfood check` warned "below anchored" on every cited or unknown hunk, including hunks
  where no stable BASE line exists to anchor against. A large lane printed hundreds of warnings that no edit
  could clear.
- **V1-2404.** Seal commits appeared to ignore the lane identity. The code already honoured `GIT_AUTHOR_*` and
  `GIT_COMMITTER_*`. The real cause was that agent shells do not keep exported variables between tool calls,
  and no test asserted the identity.
- **V1-2363 / GH #1519 remainder.** RUBY-AFFECTED-V0-046..048 locate Ruby frontier sites. But a plan only
  carried the frontier entries the dirty set crossed. So a consumer could not tell how many repository
  frontier sites exist by reason.
- **V1-2020.** `tools/capabilities-check` did not check three things: subcommands, Experimental-group maturity,
  and refusal codes. The new checks found drift on the first run:
  - five `cem` subcommands were missing from their row;
  - four `lease` subcommands and six `flows` subcommands were missing from theirs;
  - the `prove` row listed `public-key-unavailable`, but the code emits `attest-public-key-unavailable`.

## Decisions

- **DCW-V0-165.** Only hunks with an available anchor warn. The unanchorable hunks are counted in one NOTE line.
- **DCW-V0-166.** Tests now assert the seal and finish-lane commit identity, and `docs/DOGFOOD.md` says to set
  the identity in the same shell call.
- **AFP-V0-381.** The plan gains an optional `plan.frontierSites` member: a `reason → count` map. It sums
  plugin-wide and per-unit frontier entries, and an entry `reason:+N` counts as N. It does not depend on the
  dirty set. It is omitted when there is no frontier, so it is compatible under CCF-V1-006. The compact view
  keeps it.
- **CAP-V0-007.** The check reads the declared tables with `go/parser`: `helpSubcommands` (following
  `cemHelpActions`), `docsSubcommands`, `flowSubcommands` plus its `init` registrations, and `leaseActions`. A
  missing table is a check error, not a pass.
- **CAP-V0-008.** An Experimental-group verb must have maturity `experimental`.
- **CAP-V0-009.** A code-shaped token in a refusal cell must be a whole word of some non-test string literal
  under `cmd/` or `internal/`. The literals are tokenised with `go/scanner`. No process or network imports
  were added. The test read scope widens from `internal/mcp/` to `internal/`.

## Skipped

- **V1-2322.** It needs the native Tasks lifecycle, and `corvint-tasks` is outside this lane.
- **V1-1383.** Its acceptance needs an owner-recorded accepted exclusion for the cem and ocm skips. A frontier
  replay also needs a cem/1.0 map that 0.8.1 cannot read.
- **V1-2323.** It needs an owner decision on how the recorded 1.0 limits feed readiness. A reader that refused
  incomplete runner, platform or external outcomes would refuse every release today. The open items behind
  this are V1-2327, V1-0014 and V1-2107, plus the protocol/cem-1.0 stable re-seal.

## Evidence

Toolchain: go1.27.2 (`GOTOOLCHAIN=local`, with the pinned toolchain first on PATH).

- The new tests fail without the fix. For AFP-V0-381 the mutations were: drop the Select copy, count `+N` as 1,
  and drop the compact copy. For CAP-V0-007 to CAP-V0-009 they were: drop each check call, drop the
  Experimental case, and drop the registration merge.
- These packages pass:
  - `./internal/liveverify/affected/`;
  - `./internal/liveverify/affected/ruby/`;
  - `./tools/capabilities-check/`;
  - `./internal/dogfoodflow/`;
  - `./cmd/corvint/`, focused runs only.
- `go run ./tools/capabilities-check` reports ok (50 commands, 55 binaries, 36 MCP tools).

## NOT_RUN

`make gate`. This was scoped issue work, so the owner's standing preference applies: focused tests, the doc
gates, `make windows-vet` and dogfood.

## Rollback

Each requirement's spec names its rollback:

- DCW-V0-165: revert the anchorability decision.
- AFP-V0-381: remove `Plan.FrontierSites` and `Graph.frontierSites`.
- CAP-V0-007 to CAP-V0-009: remove the three checks and narrow the read scope back to `internal/mcp/`.

No stored state or wire format depends on these changes.
