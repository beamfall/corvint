# Design: runner-contract

Status: proposed (2026-11-09). Requirement text lives in `specs/runner-contract/spec.md`.

## Decisions

1. One compiled-in interface derived from `registry.RunnerPlugin`; the external protocol is removed
   (G23).
2. Provider binaries: retire each after its built-in runner contract lands, folding it into
   `corvint`; they are not shipped as separate 1.0 artifacts (owner decision R14 (2026-11-09)).
   This answers the earlier open question, whose recommended option was to keep them as processes.
   Until a runner's contract lands its provider stays as today; after it lands the entry leaves
   `tools/release-binary/binaries.json` and RNC-V1-001 and the binary list agree (review T-M1).
3. Canonical test ID: RNC-V1-012, approved by owner decision R11 (2026-11-09). The rules are
   CTI-V1-001 to CTI-V1-012 in change `canonical-test-id` (runner, repo-relative file and
   runner-native name path, with an escape rule per runner; a file-level selection matches every
   ID in that file; duplicates refused or disambiguated). RNC-V1-012 adds no rule of its own.
4. Inventory: one committed inventory under `.corvint/` for all seven runners, written by
   `corvint inventory refresh` (owner decision R10 (2026-11-09); change `test-inventory`). Each
   runner supplies a lister marked `static` or `executing`; Ruby uses static classification (review
   T-B5). Missing inventory runs the runner's declared full command with reason `inventory-absent`;
   stale inventory runs all tests with reason `inventory-stale`.
5. Tier vocabulary: `line | file | flow | full` plus the boolean `exact` (owner decision R16
   (2026-11-09)). The earlier `superset` label maps to `exact: false` with a reason; the earlier
   `exact` tier maps to `exact: true`.
6. Kill switch: grammar owned by NMS-V1-008 (`full` or `full:<runner>[,<runner>]`, the same in the
   Action); unknown value exits 2 in the CLI (`corvint affected`, `corvint test`, `corvint watch`)
   and CI (the Action), and runs all tests in agent hooks and MCP (owner decision R13
   (2026-11-09)). A runner-name kill switch applies to every instance of that runner (review T-M8).
7. Runner instances: runner plus root directory plus config file, several per runner (owner
   decision R15 (2026-11-09); RNC-V1-014). No new cross-package analysis.
8. Sharding: deterministic partition of each runner's selection, used by the Action's shards only
   (owner decision R15 (2026-11-09); RNC-V1-015). The Action's single plan artifact and shard check
   belong to change `github-action`.
9. Argument length: argument file where the runner accepts one, otherwise widen to file level, and
   to the runner's full command if still over the limit (review T-M9; RNC-V1-016).
10. Unowned change, removed runner, unavailable runner: run all tests with a named reason;
    `UNOWNED_DIRTY_PATH` comes off the allowlist (owner decision R32 (2026-11-09)).
11. Results: per test, with per-test durations in every runner's receipt (owner decision R19
    (2026-11-09); change `per-test-durations`), feeding PROD-TEST-020 (owner decision R7
    (2026-11-09); change `run-evidence`).
12. Minitest per-test results come from Corvint's own Ruby hook, which already records an outcome
    per `Class#method` at integ `9ec3a29d0`; the hook gains `result.error?` so `error` is separate
    from `failed`. `minitest-reporters` and JUnit XML are not used (settled without the owner, D9).
13. Cypress is dropped from 1.0 and listed as post-1.0 (owner decision R6 (2026-11-09)).

## Failure modes

- Runner binary absent (the runner's own tool, for example `pytest`): the runner's units are
  `NOT_RUN`, run `INCOMPLETE`, reason printed. Under owner decision R32 (2026-11-09) the plan for
  the other runners widens to all tests with reason `runner-removed`; the run is never `PASS`.
- A runner listed in the inventory or in `.corvint/test-commands.json` has been removed from the
  repository: all tests run with reason `runner-removed` (owner decision R32 (2026-11-09)).
- A changed path that no runner instance owns: all tests run with a named reason;
  `UNOWNED_DIRTY_PATH` is no longer an allowlisted pass (owner decision R32 (2026-11-09)).
- Discovery output unparseable or stale: the runner's selection widens to its full declared command
  with a reason; never an empty selection. Inventory absent: `inventory-absent`; stale:
  `inventory-stale`.
- Selection argv can only express a superset: the selection records `exact: false` with the reason.
- Argument list over the OS limit: argument file, else widen to file level, else full command,
  each with a reason (RNC-V1-016).
- Two instances claim the same test file: the test is run by both and the receipt names both
  instances; it is never dropped.
- Machine report missing or malformed: every selected test of that runner is `NOT_RUN`.
- Kill switch set: the runner (every instance) runs its full command and says so.
- Kill switch value unknown: exit 2 with a clear message in the CLI and CI; all tests in agent hooks
  and MCP.
- `.corvint/test-commands.json` is malformed or has an unknown profile: `corvint test` exits 2 with a
  typed error and runs nothing; it does not fall back silently to the built-in default.
- Neither `.corvint/test-commands.json` nor the built-in default gives a usable full command (the
  default's tool is absent): the runner's tests are `NOT_RUN`, the run is `INCOMPLETE`, reason
  printed.

## Acceptance evidence

- One fixture repository per runner; Go tests named per requirement and runner.
- Fault-injection results per runner from change `fault-injection-harness` (G3: 100 PRs per runner).
- The Jest end-to-end fixture of RNC-V1-010, run on Linux and macOS.
- RNC-V1-013: fixture tests for the three scenarios, the malformed file, and the one-release notice
  in Corvint's own repository.
- RNC-V1-014 to 016: a pnpm workspace fixture with two Vitest configs, a two-rootdir pytest fixture,
  a shard-determinism test, and an over-limit argv fixture.
- RNC-V1-017: `binaries.json` read back with no provider entry for a runner whose contract landed.
- NOT_RUN at the time of writing: everything; this proposal is based on code reading only.

## Rollback

Each runner implementation can be switched off by the kill switch (RNC-V1-008), which falls back to
the full declared command (RNC-V1-013). To withdraw the change, revert its commits; the removed
packages and provider binaries return from the reverted diff. IDs are retired, not reused.

- RNC-V1-013: revert the commit; the full command is again read from
  `.github/cishards/test-commands.json`, and adopters without it return to `INCOMPLETE` on FULL as
  today.
- RNC-V1-014 to 016: revert; runners are again keyed by name only, and an over-limit selection
  fails as today.

## Open questions for the owner

- Keep or remove `cmd/corvint-test-runner` once `corvint test` uses the registry directly? It is
  not a provider binary, so owner decision R14 (2026-11-09) does not settle it; it is listed with
  role `product` in `tools/release-binary/binaries.json` (line 22, integ 9ec3a29d0).
- (Answered: keep the JS and Ruby providers as processes or fold them in. Owner decision R14
  (2026-11-09): retire each after its built-in contract lands.)
