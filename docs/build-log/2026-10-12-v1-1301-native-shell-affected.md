# 2026-10-12 C-family and shell affected-test adapters (V1-1301)

Branch `claude/v1-1301-affected-objc-c-shell` from `origin/main` 9deae4838. Spec
`docs/specs/native-shell-affected-test-v0.md`, requirements `NSA-V0-001..015`, accepted under the
standing lane auto-accept of 2026-10-11; amends `AFP-V0-025` in place.

## Intent

Corvint ships Objective-C, C/JNI and shell analyzers, but `corvint affected` had no plugin for
them, so a dirty `.c`, `.m`, `.h` or `.sh` path was `UNOWNED_DIRTY_PATH`. This slice adds static
planning only; runner execution stays with V1-0593 and V1-0594.

## Decisions

1. One `cfamily` plugin owns C, C++, Objective-C(++) and JNI text. Two plugins claiming `.h` would
   make ownership ambiguous; headers are shared across these languages.
2. `cfamily:link-graph-unmodelled` is a global frontier reported only when a C-family file exists.
   No build system is read, so every plan `cfamily` joins is UNKNOWN even when it selects tests; a
   repository with no C-family file is unaffected.
3. `shell:foreign-invocation-unmodelled` is a unit frontier on non-test shell units, reported only
   when another planned language's source exists. A global frontier would make every polyglot plan
   UNKNOWN, because a toolchain test is reached on every dirty path. For the same reason the shell
   plugin does not implement `PathReader`.
4. A shell test that runs a toolchain is `UnboundedReads` (`shell:runs-toolchain`). A function the
   file defines shadows a toolchain of the same name (`script/verify-receipt-bundle_test.sh`
   defines `bundle`).
5. `AFP-V0-025` keys a single-test-file unit by its file path, because ten `script/*_test.sh`
   toolchain tests share one directory. The nine real ones are recorded in
   `.corvint/unbounded-readers.json` with reasons.
6. `ForeignSourceSuffixes` lists exactly the suffixes other registered plugins own; a languages
   test keeps it in sync. Java has no affected plugin (`kotlin` owns only `.kt`/`.kts`), so a dirty
   `.java` path stays `UNOWNED_DIRTY_PATH`; an earlier answer in this lane wrongly said kotlin
   covered Java.
7. Source in the new packages avoids the literal `--show-toplevel` and test literals `./...` /
   `//...`, which the Go plugin's root-locator heuristic would otherwise mark unbounded.

## Evidence

- Focused: `go test ./internal/liveverify/affected/...` (cfamily, shell, languages, seam
  conformance with both plugins), `tools/unbounded-readers`, and `-run 'Affected|Impact|Unbounded'`
  on `cmd/corvint`, `internal/delta`, `internal/appflows`; `make unbounded-readers-check` passes.
- Fixture `corvint affected` (worktree build): dirty `native/core/core.c` selects core, mid and
  leaf tests with `DEPENDENCY_PATH` witnesses through the headers, excludes solo, scope UNKNOWN on
  `cfamily:link-graph-unmodelled`. Dirty `shell/lib/core.sh` selects the three tests through
  `source` edges, scope BOUNDED; adding one `.go` file makes it UNKNOWN on
  `shell:foreign-invocation-unmodelled`.
- Repository scan: 106 shell units, 41 tests, 10 classified toolchain tests (one false positive
  fixed by decision 4), 3 with `shell:dynamic-source`.
- `TestSelectionOnTheLiveDirtyWorktree` exceeded its 100 ms budget at load average 21–43;
  interleaved baseline and change runs both measured about 265 ms, so it is host load, not this
  change (tracked by V1-0564).

## Rollback

Remove `cfamily.New()` and `shell.New()` from `languages.All()` and the conformance list, and drop
the nine ratchet entries. No stored format or wire contract changes.
