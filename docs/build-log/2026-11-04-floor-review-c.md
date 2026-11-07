# 2026-11-04 floor: root-package declarations, stored method reads and signal.Stop

The review of `340edac997f92842d0456c03ad87acd7652f48b2` found three P1 gaps in V1-2744's
experimental typed test narrowing. Each fix lands with a regression test that failed before
it, and the existing AFP-V0-2374 and AFP-V0-2376 text is amended. No requirement IDs change.

## Decisions

- **AFP-V0-2376, root package:** the changed declarations of a file beside `go.mod` were
  attributed to `module/.`, a package the loader never indexes, so `resolve` and the base
  check ignored them and an allow-listed dependent could be planned `-run ^$`. Both
  `corvint test` and `tools/typed-reach-measure` now name the package with
  `typedreach.ImportPath` (`path.Join` of the module path and the file's directory).
- **AFP-V0-2374, stored method values:** `valueEffects` returned for receiver methods before
  recording implicit reads, so a package variable holding `log.Default().Prefix` or
  `(*log.Logger).Prefix` read no default logger. Implicit reads are now recorded first;
  `os.Process` method values also read signal dispositions and `os.Root` method values read
  and write the file system, matching the call path.
- **AFP-V0-2374, signal.Stop:** Stop was only a signal reader. It now writes signal
  dispositions as a call or a value, so a later test that signals itself is retained.

## Evidence

Fail-before (fixes stashed, tests present), then pass after:

- `TestAFPV02376DryRunReadsRootPackageChanges` (cmd/corvint): before, both subtests planned
  p as `^$` with `selected: no test reaches lib.go`; after, an edit of B narrows to
  `^(TestB)$` and an edit of the base shared writer A keeps p whole with
  `BASE_MAY_WRITE_SHARED_STATE`. `TestAFPV02376ImportPathOfRootPackageFiles` pins the helper.
- `TestAFPV02374StoredMethodValuesCarryImplicitReads`: before, the bound method value and
  method expression readers were dropped; after, both are kept with
  `READS_STATE_A_KEPT_TEST_WRITES` and the unrelated control stays dropped.
- `TestAFPV02374SignalStopWritesSignalState`: before, the self-signalling test was dropped for
  a direct and a function-value Stop; after, it is kept and the control stays dropped.
- Executable fault check of the signal scenario: with `lib.A` returning 1 (Stop runs) the
  full suite ends `signal: terminated` / FAIL; with it returning 0 the suite passes, so the
  dropped test did observe the change.

Package runs: `./internal/liveverify/affected/golang/typedreach/` ok;
`./cmd/corvint/ -run 'TestAFPV0237|TestAFPV0236'` ok, including the refactored
`TestAFPV02370DryRunNarrowsAnAllowListedDependent`.
