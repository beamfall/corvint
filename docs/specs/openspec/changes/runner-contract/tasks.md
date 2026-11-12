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
   the Jest end-to-end fixture and one fixture per runner.
- [ ] 7 Kill switch on every surface using the NMS-V1-008 parser. Requirements: RNC-V1-008. Proof:
   one scenario per surface.
- [ ] 8 Sharding for the Action and argv bounds. Requirements: RNC-V1-015, 016. Proof: shard
   determinism and over-limit fixtures.
- [ ] 10 Remove the external protocol packages and tools. Requirements: RNC-V1-009. Proof: build and
    `go vet ./...`.
- [ ] 11 Retire each provider binary after its runner lands; edit `binaries.json`. Requirements:
    RNC-V1-017. Proof: binary list read back.
- [ ] 12 Tier output, release manifest fields; the support table row for the shell provider (`kept,
    not qualified`) and its full-selection fallback; docs; build-log; Tasks gate. Requirements:
    RNC-V1-011, RNC-V1-018. Proof: manifest fixture, a shell fixture whose unmapped change
    selects every shell test, `receipt audit`.
