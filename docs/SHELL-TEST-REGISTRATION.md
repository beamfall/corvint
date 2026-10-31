# Registering a shell-script test

A `script/*_test.sh` test is registered in three places that different checks keep in step. This
note is the one place that lists them; the checks point here when one is missing.

1. **A `Makefile` target and a `GATE_STEPS` entry.** Add `<name>-test:` with a recipe that runs
   `@script/<name>_test.sh`, and append `<name>-test` to the single plain `GATE_STEPS = ...` line.
   `make gate` runs only `GATE_STEPS` (AFP-V0-025 names the same line for `unbounded-readers-check`).
2. **The `shell-test` prerequisites.** `shell-test` must list exactly the `GATE_STEPS` steps whose
   recipe runs a `script/*_test.sh`, with no recipe of its own. This is the shell full command
   that `.github/cishards/test-commands.json` declares (`STR-V0-004`, `docs/specs/session-test-runner-v0.md`).
   Add the same `<name>-test` to the `shell-test:` line. `TestSTRV0004ShellTestIsTheGateShellSteps`
   (`tools/test-commands-check`, run alone by `make shell-test-registration-check`) fails when the
   two lists differ.
3. **The unbounded-readers record.** A shell test is a test unit identified by its own file, so the
   planner selects it on every change unless its reads are bounded. Either declare its reads in
   `.corvint/test-read-scopes.json` (`AFP-V0-023`) or add the file path (for example
   `script/<name>_test.sh`) to `units`, in strictly ascending order, with an entry in `reasons`
   that names the unbounded read and why it cannot be declared (`AFP-V0-025`, `AFP-V0-033`).
   `make unbounded-readers-check` fails naming the file when neither is done.

Run `make -s --keep-going unbounded-readers-check shell-test-registration-check` after editing, or
`make doc-gates`, which keeps going past a failing gate and so names every missing registration in
one run (`DCW-V0-813`).
