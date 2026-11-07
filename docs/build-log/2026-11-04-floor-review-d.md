# 2026-11-04 floor: stream output writes and indirect standard-library calls

The review of `c450e3dbcdce8d8b3584013df9548afa6b34f28d` found two more P1 gaps in V1-2744's
experimental typed test narrowing. Each fix lands with a regression test that failed before it.
The existing AFP-V0-2374 text is amended, and no requirement IDs change.

## Decisions

- **AFP-V0-2374, stream output:** `fmt.Print*` and the `log` and `log/slog` output functions and
  logger methods recorded only implicit reads.
  - Failure: a `TestMain` that points `os.Stdout` or the default logger at a file let a kept test
    print a changed value while a later test that reads the file was dropped.
  - Fix: these calls, and the same functions taken as values, now write the `files` pseudo
    variable (`streamOutput`). This is coarser than the stream's real target, which the model
    cannot know, so it is fail-closed.
- **AFP-V0-2374, indirect calls:** `stdArguments` ran only for a resolved `*types.Func`.
  - Failure: `copyFn := io.Copy; copyFn(os.Stdout, ...)` skipped the opaque-argument check, and
    taking `io.Copy` as a value recorded nothing.
  - Fix, in two independent parts:
    - An unresolved callee is checked through its static signature (`opaqueArguments`).
    - A standard-library function or method value with an opaque interface parameter is itself
      unknown (`opaqueParameter`), since it may later be called with any argument.
  - The first part also applies to module function values. Their bodies already make abstract
    calls on such parameters unknown, so it adds no new loss beyond that case.

## Evidence

- `TestAFPV02374StreamOutputWritesFiles` (`fmt.Print`, a `fmt.Println` value, `log.Print`,
  `(*log.Logger).Printf` call and value, `slog.Info`).
  - Before the fix, all six subtests dropped `TestBReader`.
- `TestAFPV02374IndirectStdCallsKeepOpaqueArguments` (direct `io.Copy` and `WriteTo` as controls;
  a local and a package `io.Copy` value; a `WriteTo` method value).
  - Before the fix, the three indirect subtests dropped `TestBReader`, and the direct controls
    passed.
- Executable ordered-suite fault check: the fixtures were run as real suites under
  `script/heavy.sh` for `fmt.Print`, `slog.Info` and the `io.Copy` value.
  - With `lib.A` returning 1, the suite passed.
  - With `lib.A` returning 2, the suite failed: `TestBReader`, the only test with an assertion,
    read the changed output.
- After the fix, the whole typedreach package passes, and so do the focused `cmd/corvint` typed
  tests.
