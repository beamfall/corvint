# 2026-11-13: `corvint test` runs narrowed runner-contract selections (V1-5443)

Change `runner-contract`, second lane (V1-5443, with V1-5455 and V1-5456). Base `origin/main`
`39050561`. The first lane (V1-5416, `docs/build-log/2026-11-11-runner-contract.md`) built the
contract and proved its argv, report parsing and reconciliation with fixtures, but `corvint test`
did not execute them. This lane makes `corvint test` execute them and reads the kill switch.
Requirement RNC-V1-019 is new; the delivery lines of RNC-V1-001 to RNC-V1-014 are amended in
`docs/specs/runner-contract-v1.md`.

## What changed

- `internal/sessiontest` has a fourth adapter, `contract`, under the unchanged
  `corvint-test-adapter/0` manifest. On the local profile a selected `typescript:<runner>:<path>`
  (Vitest, Jest, Playwright), `ruby:rspec:<stem>` or `python:<module>` unit of a language the
  declaration names no command for becomes one manifest unit per owning instance: `unit` is the
  file-level canonical test ID and `slice` the instance key. A `python:` unit uses the test files the
  plan lists for it (`Selection.Tests`). One invocation runs per instance, from its root. Its argv
  is the contract's narrowing of the instance's full command, with the report written to a fresh
  `corvint-test-report-*` directory under the temporary directory, which is removed afterwards.
- The receipt's contract invocations add `instance`, `tier`, `exact`, `exactReason` and `tests` (one
  `{testId, outcome, durationMillis, granularity}` per test, `NOT_RUN` for a selected test the report
  does not answer). Each outcome's `testId` is the unit's ID. Playwright units carry `kind: e2e`
  with `kindSource: runner` (RNC-V1-007). Every new member is omitted elsewhere, so existing
  manifests and receipts keep their bytes.
- `internal/testrunner/killswitch` parses `CORVINT_SELECTION` in the NMS-V1-008 grammar.
  `corvint test` applies it. `corvint affected` and `corvint watch` refuse an unknown value.

## Decisions and differences from the proposal

1. **The inventory gates narrowing per instance.** RNC-V1-002 says an absent inventory runs the
   runner's full command, and a stale one runs all tests. A contract unit needs its instance's
   inventory verdict to be current. Otherwise the instance runs `declared:<runner>@<root>` with the
   verdict's reason (`inventory-absent`, `inventory-stale`, `inventory-invalid`, or `selection-full`
   under the kill switch). A stale inventory does not send Go or other runners to FULL: TIV-V1-006
   says the inventory never narrows the plan, and Go's selection does not read it. Recorded as
   not built in RNC-V1-002.
2. **What a contract unit runs.** It runs the files the plan selected, not the inventory's
   listing. The plan's selection is the evidence. The inventory only vouches that the instance's
   test set is known. Vitest and Playwright file filters therefore stay `exact: false` (no listing is
   passed), with the contract's reason printed.
3. **Where narrowing applies.** A language with a `languages` entry keeps its one command, which
   covers all of its runners (RNC-V1-013), so Corvint's own repository is unchanged. A declared
   `providers` entry still takes Vitest and Playwright first (STR-V0-8702). The ci profile runs only
   Go (STR-V0-012). In FULL every instance runs its full command, and no contract unit is added. An
   instance whose full command already runs drops its contract units.
4. **Failure handling.** A missing, non-regular or oversized report (`testid.MaxReportBytes`), an
   unreadable report, a contract refusal or an undiscoverable runner leaves every unit `NOT_RUN`,
   and the run is `INCOMPLETE`. A failing or erroring test fails its file. A run-level failure no
   test carries (Jest `success: false`, pytest exit status) fails every unit that would pass. A
   nonzero exit fails the run through `Finish`, and the receipt keeps the exit status.
5. **Kill switch.** An empty value is off, as an unset variable is, since CI templates often export
   an empty variable. `full` plans FULL with reason `kill-switch: ...`. `full:<runner>` runs every
   instance of each named runner in full, selected or not, and `full:go` runs the whole Go
   universe. The manifest and receipt carry `killSwitch`. `corvint affected` cannot widen its
   frozen `affected-plan/1` document (CCF-V1-006), so it prints the active switch on stderr and says
   its plan is not widened. `corvint watch` answers only narrowed Go selections, so a switch that
   sends Go to its full command is refused with the new code `kill-switch-full-unsupported`, rather
   than answered narrowed.
6. **JST-V1 and PYR-V1 IDs.** The charter says the jst and pyr lanes landed (`cb2178732`,
   `04cf605d1`), but neither commit is in this repository and neither `jest-runner-v1.md` nor
   `pytest-provider-v1.md` is on `origin/main`. So the delivered behaviour is traced under RNC IDs,
   and each change folder's `tasks.md` records what the run path now gives it. The charter's
   "PYR-V1-010 mapping the plan's `python:` units" is PYR-V1-002 to 004 in this repository's change
   folder, where PYR-V1-010 is the harness adapter.

## Not built (recorded blockers)

- Minitest selection and parser (task 5): the Ruby hook named in design decision 12 is the
  line-coverage collector. Its `error` status would change the dump that `internal/rubylinecov`
  parses. Minitest keeps its full command with `runner-not-implemented`.
- Task 7, the remaining surfaces: the FULL kill-switch state in the frozen `affected` plan, agent
  hooks, MCP and the Action.
- Task 8 (out of scope).
- Task 10: owned by source-removal B3 (RMV-V1-010), not landed.
- Task 11: `corvint-js-test-provider` can be retired now, but retiring it removes the STR-V0-87xx
  adapter and needs a ruling on existing `providers` entries.
- Task 12: the release manifest needs qualification results, and the shell fallback changes frozen
  `affected` output.

`.github/` remainder for a separate change, not edited here: task 10's
`.github/linecov/qualification.json` re-qualification once B3 removes `internal/runnerpolicy`
and `internal/runnerqualification`; task 11's `.github/cishards/package-costs.json` row for
`cmd/corvint-js-test-provider`; task 7's Action input; and the V1-5416 move of Corvint's own
declaration out of `.github/cishards/test-commands.json`.

## Evidence

Focused tests, each run through `script/heavy.sh`, one package per command:
`internal/sessiontest`, `internal/testrunner/killswitch`, `internal/testrunner/contract`,
`internal/capcatalog`, `internal/cliapp`, `internal/liveverify/affected/liverepo`,
`internal/mcp/bridge`, `tools/capabilities-check`, `tools/test-commands-check` and
`tools/test-commands-run` (all pass); `cmd/corvint` with `-run` over the new tests and the 101 tests
of the files that invoke `corvint test` (pass). Live Jest, Vitest, Playwright, pytest and RSpec runs
are NOT_RUN: none is installed on the lane host, so their executables are fixtures that write each
runner's documented report.
