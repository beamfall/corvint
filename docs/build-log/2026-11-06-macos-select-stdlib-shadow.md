# go-macos shards: helper `select.py` shadowed the stdlib `select` module

Date: 2026-11-06. Ticket: V1-4732. PR: beamfall/corvint#103. Requirement: `CIN-V0-4625`.

## Finding

The first hosted run on PR #103 (run 42172216166, head 01d1ed948) failed all four
`go-macos-shard` legs within 22 s, and the `go-macos` summary failed with them. Python puts a
script's own directory first on `sys.path`, so `.github/macos/select.py` replaced the standard
library `select` module. `select_test.py` now imports `subprocess`, which imports `selectors`,
which reads `select.select`. That raised AttributeError under hosted Python 3.15. `shard.py`
also imports `subprocess` from the same directory and would have failed the same way.

## Decision

Rename the helper to `.github/macos/darwin_select.py` and update the workflow and both test
loaders. Running every call with `python3 -I` would also have avoided it, but only at call sites
that remember the flag, so the cause is removed instead. `select_test.py` gains
`test_CINV04625_helpers_never_shadow_stdlib_modules`, which fails if any helper in the directory
has the name of a builtin or importable stdlib module. It flags `select.py` and passes
`darwin_select.py`.

Per the Greptile review, the regression step now runs `shard_test.py` as well as
`select_test.py`, so the mixed FULL/NARROWED ownership and cost-read failure tests run in CI.

## Limits

Darwin test outcomes and shard wall time remain NOT_OBSERVED until the next hosted run.
