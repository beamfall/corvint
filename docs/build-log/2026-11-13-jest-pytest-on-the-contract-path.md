# 2026-11-13: Jest and pytest on the contract path, and their durations (V1-5455, V1-5456, V1-5542)

Two lanes built the Jest and pytest providers on a base older than the runner-contract run path
(V1-5443) and the selection verdict (PR 209 and PR 210). This entry records how both were brought
onto `origin/main`, the fixes for their independent reviews, and the per-test-duration remainder.
The requirements are in `docs/specs/jest-runner-v1.md`, `docs/specs/pytest-provider-v1.md`,
`docs/specs/per-test-durations-v1.md` and `docs/specs/runner-contract-v1.md` (RNC-V1-016).

## Decisions

1. **One adapter for pytest.** The pytest lane had built its own `runner` adapter in
   `internal/sessiontest`, with its own `Input.ContractTests` and a `CORVINT_SELECTION` parser copy
   in `internal/selectionswitch`. Main's contract adapter (RNC-V1-019) and its kill-switch parser
   (`internal/testrunner/killswitch`) cover the same ground, so the lane's adapter and parser copy
   were dropped at the merge. Kept: the JUnit pytest runner and parser, the kind labels, the
   harness adapter. Ported onto the contract adapter: pytest's typed exit reasons, the deselected
   count, the runner-unavailable reason and the environment record (PYR-V1-006, PYR-V1-008).
2. **pytest's own JUnit report stays.** The pytest lane switched the contract's pytest runner from
   the pytest-json-report plugin to `--junitxml` (xunit1, call-phase time), which needs no plugin in
   the adopter's environment. Main's tests that pinned the plugin's argv and fake report now pin the
   JUnit argv and a JUnit fake; the plugin reader stays for documents that use it.
3. **Completeness fails closed (pytest review P1).** One reported test no longer stands for a
   selected file. The expected tests are the selected files plus every test the committed
   inventory's collection lists in them (RNC-V1-006), so a test a plugin deselects with no trace in
   the output (`-p no:terminal`) is NOT_RUN. A selected pytest file the inventory lists no test in
   (a changed or new test file, which TIV-V1-008 leaves unassigned) cannot prove completeness and
   makes the run INCOMPLETE with `tests-deselected`. Considered and rejected: a `--collect-only` pass
   before the run, since a plugin that deselects at collection deselects there too and would give
   false assurance. Consequence: editing a pytest test file makes its narrowed run INCOMPLETE until
   the inventory is refreshed and committed. Only an injected collection plugin could do better.
4. **The Jest name filter only when lossless (Jest review P1, JST-V1-020).** Jest drops an ancestor
   titled `ROOT_DESCRIBE_BLOCK` from its report but keeps it for `-t` matching, and the canonical ID
   comes from the report, so no listing can prove the block absent. The contract reads each selected
   file's source through `JestSetup.Source` and passes `-t` only when no file names the block and no
   selected test is in a duplicate group (CTI-V1-010, review P2); otherwise the files run whole.
5. **Jest setup is read before narrowing.** `corvint test` now reads each Jest instance's install and
   config (the `--config` the full command names, else Jest's own config files, else Jest's
   defaults); without a readable supported setup the instance runs in full (JST-V1-008). A config
   that loads or spreads another module is not proven supported (review P2); a type-only import is
   allowed, and a preset is still admitted unread (NOT_VERIFIED).
6. **Argument limit (RNC-V1-016) for Jest.** Jest has no argument file for `-t` or paths. An
   over-long pattern widens to the files; over-long paths run the full command; both name
   `argv-limit`. The bound is one argument under Linux's `MAX_ARG_STRLEN` and 256 KiB in all.
7. **Durations on the contract path (PTD-V1-002).** The contract `Result` gains two unserialized
   marks, `DurationMissing` and `DurationInvalid`, so a report time that is absent or invalid is not
   read as 0 ms. A contract file unit sums its tests' times; one missing time or a NOT_RUN test makes
   it NOT_MEASURED, and an invalid one lists it under `durationInvalid`. A pytest time that is not a
   duration no longer refuses the whole report. Minitest has no contract selection, so its receipt
   outcome stays NOT_MEASURED until change `ruby-providers`.

## Not done here

- `corvint watch` for Jest and pytest (change `watch-seven-runners`); so jest-provider task 7 stays
  open, and task 8 with it.
- Live Jest, pytest and RSpec runs: none is installed on the lane host (NOT_RUN). Every runner output
  in the tests is a stub in the runner's documented report shape.
- PYR-V1-007 edge: a selected file outside pytest's `python_files` is run by `corvint test` but not
  named by the advisory command.
- The pytest review's P2 on FULL plans (a kill-switch or fallback pytest run records no per-test
  results) is unchanged: full commands run through the declared adapter.
