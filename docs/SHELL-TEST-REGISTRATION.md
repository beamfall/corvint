# Registering a shell-script test

A `script/*_test.sh` test is registered in three places that different checks keep in step. This
note is the one place that lists them; the checks point here when one is missing.

1. **A `Makefile` target and a `GATE_STEPS` entry.** Add `<name>-test:` with a recipe that runs
   `@script/<name>_test.sh`, and add `<name>-test` as its own line, ending in ` \`, at its sorted place
   in the `GATE_STEPS = \` list (after the fixed first five steps; `STR-V0-920`).
   `make gate` runs only `GATE_STEPS` (AFP-V0-025 names the same list for `unbounded-readers-check`).
   The target is required even for a suite kept out of the gate: `TestASDV0942EveryScriptTestHasAMakeTarget`
   (`make test-commands-check`) fails naming any `script/*_test.sh` no recipe starts (ASD-V0-942).
   A suite left out of `GATE_STEPS` says why in the comment above its target.
2. **The `shell-test` prerequisites.** `shell-test` must list exactly the `GATE_STEPS` steps whose
   recipe runs a `script/*_test.sh`, with no recipe of its own. This is the shell full command
   that `.github/cishards/test-commands.json` declares (`STR-V0-004`, `docs/specs/session-test-runner-v0.md`).
   Add the same `<name>-test` as its own line at its sorted place in the `shell-test: \` list. `TestSTRV0004ShellTestIsTheGateShellSteps`
   (`tools/test-commands-check`, run alone by `make shell-test-registration-check`) fails when the
   two lists differ.
3. **The unbounded-readers record.** A shell test is a test unit identified by its own file, so the
   planner selects it on every change unless its reads are bounded. Either declare its reads in
   `.corvint/test-read-scopes.json` (`AFP-V0-023`) or add the file path (for example
   `script/<name>_test.sh`) to `units`, in strictly ascending order, with an entry in `reasons`
   that names the unbounded read and why it cannot be declared (`AFP-V0-025`, `AFP-V0-033`).
   `make unbounded-readers-check` fails naming the file when neither is done. A shell test that runs
   a build or language toolchain (`go`, `make`, `node` and the other `shell.Toolchains` words) is
   marked `shell:runs-toolchain`. `.corvint/test-read-scopes.json` is keyed by Go package and
   cannot bound it; instead declare the paths it reads outside its own directory in
   `.corvint/shell-read-scopes.json` (`TSP-V0-950`: sorted `reads` entries and a one-line `reason`,
   for example the packages a `go build` it runs compiles), or, when its reads cannot be listed (a
   whole-program build, a `make` recipe), add the file to `units` with a `reasons` entry naming the
   toolchain read. A test that also sources a dynamic path cannot be declared. The refusal names
   both routes for such a unit (V1-3320, V1-2167).
   What an entry allows, and how to check a declared scope locally before confined CI does, is in
   [TEST-READ-SCOPES.md](TEST-READ-SCOPES.md).

## Pinning a Makefile recipe expansion

Do not pin a recipe with `make -n` from a `script/*_test.sh`: `make` is a toolchain word, so the
shell test becomes an unbounded reader selected on every change (V1-2159). Pin it from a Go test in a
package whose `.corvint/test-read-scopes.json` entry already declares `Makefile`, such as
`tools/test-commands-check`, which runs `make -s -n -C ../.. TARGET VAR=VALUE` and compares the
output: `TestInstructionConflictsRecipeExpansion` pins the `instruction-conflicts` recipe this way.

Run `make -s --keep-going unbounded-readers-check shell-test-registration-check` after editing, or
`make doc-gates`, which keeps going past a failing gate and so names every missing registration in
one run (`DCW-V0-813`).
