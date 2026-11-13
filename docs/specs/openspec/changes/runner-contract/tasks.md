# Tasks: runner-contract

Build in this order. Each task names its requirement IDs and its proof. The canonical test ID
(change `canonical-test-id`) and the inventory (change `test-inventory`) land first (owner decision
R38 (2026-11-09) build order).
Tasks 6 and 9 were delivered before task 5, so they are listed first (OSC-V1-005); the task
numbers are kept because the spec and the build log cite them.

- [x] 1 Reserve `RNC-V1` 001..018; file the Jest ticket; set `requirementRefs`. Proof: `ticket show`
   read back. V1-5416 filed; RNC-V1 reserved. Jest delivered in task 5 under V1-5416.
- [x] 2 First commit: create `runner-contract-v1.md`; add the README entry; regenerate the index.
   Requirements: all. Proof: spec diff and index check.
- [x] 3 Define the interface from `RunnerPlugin`, keyed by canonical test ID and runner instance;
   adapt Go, Vitest and Playwright. Requirements: RNC-V1-001, 003, 005, 006, 012, 014. Proof:
   per-runner fixtures.
- [x] 4 Supply each runner's lister to `corvint inventory refresh`, marked static or executing; Ruby
   static. Requirements: RNC-V1-002. Proof: no child process from `corvint affected`; lister kind in
   the inventory.
- [x] 6 Read the full command from `.corvint/test-commands.json` with built-in defaults and instance
   entries; move Corvint's own declaration there and keep the old path for one release.
   Requirements: RNC-V1-013. Proof: the RNC-V1-013 scenarios. Corvint's own declaration is now
   also at `.corvint/test-commands.json`, kept byte-identical to the old path by
   `tools/test-commands-check`; deleting the old file and moving its readers edits `.github/`, so
   the build log records that step.
- [x] 9 Unowned path and removed runner run all tests; `UNOWNED_DIRTY_PATH` passes the allowlist
   only for a path a Go selection rule attributes (scope ruling, 2026-11-11). Requirements:
   RNC-V1-004 (fallback), owner decision R32. Proof: removed-runner fixture; the attributed and
   unattributed unowned-path fixtures.
- [ ] 5 Implement Jest, pytest, RSpec and Minitest. Requirements: RNC-V1-004, 005, 007, 010. Proof:
   the Jest end-to-end fixture and one fixture per runner. V1-5443: `corvint test` runs narrowed
   Vitest, Jest, Playwright, pytest and RSpec selections through the contract with per-test receipt
   results (RNC-V1-019; `TestRNCV1010JestSelectionRunsEndToEnd`, `TestRNCV1019*`), and Playwright
   units carry the runner kind signal. Blocker, kept open: the Minitest selection and parser. The
   Ruby hook design 12 names is the line-coverage collector (`internal/rubylinecov/linecov.rb`):
   per-test results through it mean running coverage collection in every Minitest run, and its new
   `error` status changes the dump its Go readers parse, which needs its own design note.
- [ ] 7 Kill switch on every surface using the NMS-V1-008 parser. Requirements: RNC-V1-008. Proof:
   one scenario per surface. V1-5443: `internal/testrunner/killswitch` reads the NMS-V1-008 grammar;
   `corvint test` applies it, and `corvint test`, `affected` and `watch` refuse an unknown value
   with exit 2 (`TestRNCV1008*`). Blocker, kept open: the FULL kill-switch state in the frozen
   `corvint affected` plan needs a CCF-V1-006 decision; agent hooks, the MCP tools and the GitHub
   Action (`.github/`) are not built.
- [ ] 8 Sharding for the Action and argv bounds. Requirements: RNC-V1-015, 016. Proof: shard
   determinism and over-limit fixtures.
- [ ] 10 Remove the external protocol packages and tools. Requirements: RNC-V1-009. Proof: build and
    `go vet ./...`. Blocker (V1-5443): the deletion is source-removal batch B3 (RMV-V1-010, its
    task 6), which carries its own batch proof (RMV-V1-003, RMV-V1-004) and the RSP-V0 retired
    status lines (RMV-V1-026); B3 has not landed (all six paths exist at `39050561`), and
    `.github/linecov/qualification.json` names two of the packages.
- [ ] 11 Retire each provider binary after its runner lands; edit `binaries.json`. Requirements:
    RNC-V1-017. Proof: binary list read back. Blocker (V1-5443): `corvint test` now runs Vitest and
    Playwright through the contract, so `corvint-js-test-provider` is retirable, but retiring it
    removes the STR-V0-8701 to STR-V0-8707 provider adapter and the `providers` declaration member,
    which needs a decision on refusing or ignoring an existing `providers` entry; the Ruby provider
    waits on Minitest (task 5), and the Go provider on the `corvint-test-runner` open question.
- [ ] 12 Tier output, release manifest fields; the support table row for the shell provider (`kept,
    not qualified`) and its full-selection fallback; docs; build-log; Tasks gate. Requirements:
    RNC-V1-011, RNC-V1-018. Proof: manifest fixture, a shell fixture whose unmapped change
    selects every shell test, `receipt audit`. V1-5443: `corvint test` prints and records each
    contract run's tier and exact flag. Blocker, kept open: the release manifest's per-runner
    tier and limits need the runner qualification results (change `fault-injection-harness`),
    and the shell full-selection fallback changes frozen `corvint affected` output (CCF-V1-006).
