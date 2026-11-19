# Source removal B2/B3: verifier repair 2

Ticket: V1-5694. Parent: `76ebd4a5f4051bbb1c57cd16e1c5164e3b1536d4`.
Requirements: RMV-V1-008/010; AFP-V0-3420/3421/3424.

## Finding and repair

The previous repair repinned the use-case ledger and two implementation receipts, all inputs
of the use-case package's record-entry read scope. Its source digest still described their
previous bytes. The unbounded-readers checker correctly refused the stale declaration.

Audited `testUCV0ProfileMigration`, `validate`, `validator.receipt` and `validator.read` in
conformance/use-cases-v0 against the ledger and receipt subject paths. The record read remains
exactly `.corvint/changes/8668f77b73eaf7b203abbb922aa1fe9cff8fd6c8.cem.json`.
No source, test, input inventory, declared read, or reason changed. Only `sourceSha256` changes
in `.corvint/record-entry-read-scopes.json`, using the AFP-V0-3421 encoding over 21 files:
two Go files and nineteen declared data inputs. Recomputing against the repair parent's parent
reproduces the old digest exactly; the three changed data inputs account for the new binding.

- Old: `947dd4badd52ae2de830944e8def56316eeae30ce63dd6cb2bc0fee960fb032e`.
- New: `a33212969bd7b6229c3f19d5db972ea5127c89789f657d0d1298bf4d5529f8ad`.

Class: stale source-bound read declarations after data repinning, not an analysis soundness gap.
The existing closed declaration profile and exact source binding remain mandatory; unknown or
stale bindings remain refused. An analyzer-form allowlist and unknown-form regression are
NOT_APPLICABLE. No frozen contract, golden, test assertion or selection policy changed.
Rollback: revert this digest update; the ratchet will again refuse the stale declaration.

## Verification

`corvint affected --base 9f4d5d89ab0648425db40e617f3d890598f9ff79 --format json` ran before
tests and selected use-cases-v0. Its scope remains UNKNOWN: 447 selected, 248 excluded,
105 unknowns, eight unresolved packages. This plan describes the parent, before the edit.

- `make -s unbounded-readers-check`: reproduced exit 2 before repair; PASS after repair.
- Whole `conformance/use-cases-v0` existing tests: PASS, 0.207s, through `script/heavy.sh`,
  `go test -count=1 -timeout 30m`, no test filter, Go 1.27.4 and the configured semaphore.
- No specs, scripts, citations or Go files changed. Broader gates and independent review remain
  the host's responsibility. No task-store write, publication or seal is authorized in this round.
- CEM bind/check cannot be completed: docs/DOGFOOD.md records decision 0594's removal of the
  workflow. The overall ticket retains the prior external qualification/integration blockers.

Raw evidence is outside Git under `~/lanes/sr-b23-evidence/`:

| File | SHA-256 |
|---|---|
| repair2-affected.json | `2123a75ff69624118d2b1b81895a416176898614f1d74731a504a06b4e23603f` |
| repair2-unbounded-before.log | `c7aa5986aedce0feca7fdf095e590bf9f7d2c764508b7e4fad88fc65850dfa0d` |
| repair2-unbounded-after.log (empty on success) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| repair2-use-cases-test.log | `50155ade943a6e3b5680452772a1873cda761b22c1048d26120314d5c7360737` |
