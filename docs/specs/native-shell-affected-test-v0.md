# Native and Shell Affected-Test Adapters V0

Owner: Russell Lewis
Date: 2026-10-12
Intent status: accepted
Delivery status: experimental
Authoritative inputs: ticket V1-1301 (owner request 2026-10-12, milestone `v1-0`), standing
auto-accept of lane requirements from 2026-10-11, `affected-plan-v0.md`,
`live-proof-carrying-verification-v0.md`, `ruby-affected-test-adapter-v0.md`,
`../SPEC-DRIVEN-DEVELOPMENT.md`

## Agent digest
- Claim: Static C-family and shell analysis maps dirty native and shell sources to tests without executing code; unmodelled link or cross-language reach stays UNKNOWN.
- Status: accepted/experimental; NSA-V0-016 accepted under the owner's standing lane auto-accept (2026-10-11; V1-2708)
- Exists: `internal/liveverify/affected/cfamily` and `internal/liveverify/affected/shell`, registered in `languages.All()`, with focused and seam conformance tests.
- Blocked on: runner execution (V1-0593 C/C++, V1-0594 Apple) and qualified runtime tuples remain `NOT_RUN`.
- Read next: Requirements; Exclusions; Traceability.

## User and measurable job

Corvint ships analyzers for Objective-C, C/JNI and shell, but before this slice `corvint affected`
had no plugin for them. A dirty `.c`, `.m`, `.h` or `.sh` path was `UNOWNED_DIRTY_PATH`: the plan
named no test that exercises it. Given such a path, Corvint should name every statically
addressable test file that can depend on it, with a witness, and keep the plan's scope UNKNOWN
wherever reach it cannot see could make a test fail.

This slice plans only. It does not execute tests, read a build system's link graph, or qualify a
runner tuple; runner execution belongs to V1-0593 and V1-0594.

## Requirements

### C family

- `NSA-V0-001`: One plugin with namespace `cfamily` MUST own C, C++, Objective-C, Objective-C++ and
  JNI source text: names ending `.c`, `.h`, `.cc`, `.cpp`, `.cxx`, `.c++`, `.hh`, `.hpp`, `.hxx`,
  `.h++`, `.inl`, `.ipp`, `.tpp`, `.m` or `.mm`. One plugin is required because these languages
  share headers; two plugins claiming `.h` would make ownership ambiguous. Shader sources (`.metal`,
  `.glsl`, `.hlsl`, `.wgsl`) are not owned.
- `NSA-V0-002`: One unit MUST be one repository file, identity `cfamily:<path>`. A header is never a
  test. A non-header file is a test when its path matches a test convention (a `test`, `tests`,
  `Tests`, `UnitTests` or `spec` directory component, or a stem with a `_test`, `_tests`, `_unittest`,
  `Test` or `Tests` suffix or a `test_` prefix) and its text carries a runnable-test signal: a
  GoogleTest `TEST`/`TEST_F`/`TEST_P`/`TYPED_TEST` macro, Catch2 `TEST_CASE`/`SCENARIO`, doctest
  `TEST_CASE`, Boost.Test `BOOST_AUTO_TEST_CASE`, CppUTest `TEST(`/`TEST_GROUP`, Unity `RUN_TEST`,
  Check `START_TEST`, CUnit `CU_add_test`, cmocka `cmocka_unit_test`, an XCTest `XCTestCase`
  subclass, or a `main` function in a conventional test path (a CTest executable). A conventional
  test path without a signal stays a source unit.
- `NSA-V0-003`: Imports MUST include each quoted `#include`/`#import` that resolves to one repository
  file: first relative to the including file's directory, then relative to the repository root and
  to each ancestor `include` or `src` directory. An angle-bracket include MUST resolve only when
  exactly one repository file has that path suffix; otherwise it is treated as external. A header's
  unit MUST import its implementation counterparts (same stem with a non-header C-family suffix in the
  same directory, or in a sibling `src` directory of an `include` directory) so that a change to an
  implementation reaches every test that includes its header. A backslash-newline MUST be spliced
  before directives are read, as in translation phase 2, so a continued directive keeps its edge.
- `NSA-V0-004`: A quoted include that resolves to more than one candidate MUST keep every candidate
  edge and raise `cfamily:ambiguous-include`. A quoted include resolving to no repository file MUST
  raise `cfamily:unresolved-include`. A macro-named include (`#include MACRO`) MUST raise
  `cfamily:macro-include`. Unreadable or undecodable text MUST raise `cfamily:unreadable-source`.
  A file that declares a `JNIEXPORT` function MUST carry the unit frontier `cfamily:jni-boundary`,
  because the JVM tests that load it are not linked by any include.
- `NSA-V0-005`: Whenever the source universe holds a C-family file, the plugin MUST report the
  global frontier `cfamily:link-graph-unmodelled`; a repository with none yields no units and no
  frontier.
  Which objects a test binary links is decided by CMake, Make, Bazel, Xcode, SwiftPM, cgo, Gradle or
  a Cargo build script, none of which this plugin reads. While that is true, every plan that
  `cfamily` takes part in is UNKNOWN even when it selects tests.

### Shell

- `NSA-V0-006`: One plugin with namespace `shell` MUST own names ending `.sh`, `.bash`, `.zsh`, `.ksh`
  or `.bats`. Extensionless executables are not owned (no shebang sniffing in V0).
- `NSA-V0-007`: One unit MUST be one file, identity `shell:<path>`. A test is a `.bats` file, or a
  shell file whose stem ends `_test` or `-test` or starts `test_` or `test-`, or a shell file under a
  `test` or `tests` directory that is executable test code rather than a sourced helper (it contains
  no top-level function-only body; a file that only defines functions is a helper source).
- `NSA-V0-008`: Imports MUST include each literal `source`, `.` and bats `load` target, and each
  literal invocation of a repository shell file (`bash PATH`, `sh PATH`, `./PATH`, `"$dir/PATH"`)
  that resolves to one owned shell unit. Resolution MUST accept a path relative to the repository
  root, to the referencing file's directory, and to the forms `$(dirname "$0")`, `${0%/*}`,
  `$(dirname "${BASH_SOURCE[0]}")`, `${BASH_SOURCE%/*}`, `$BATS_TEST_DIRNAME` and a variable
  assigned once from one of those forms. bats `load NAME` also tries `NAME.bash`. A backslash-newline
  MUST join the words around it. Case patterns and the operands of `[[ ]]` (including a `=~` regex)
  are not commands.
- `NSA-V0-009`: A `source`, `.` or `load` whose target is not a resolvable literal MUST set
  `UnboundedReads` to `shell:dynamic-source`, and a non-test unit MUST also set `LocatesRoot`, so
  every test that reaches it is selected on any dirty path. A unit frontier is not enough: it applies
  only when the unit is reached, and a computed target may read the dirty path without any edge.
  Unreadable or undecodable text MUST raise `shell:unreadable-source`.
- `NSA-V0-010`: Every unit MUST record as `PathTokens` the path-shaped string tokens of its own text,
  bounded and marked `PathTokensBounded` when the bound is hit, so a test that names a data file or
  another language's file is selected when that file is dirty.
- `NSA-V0-011`: A shell test that runs a build or language toolchain (`go`, `make`, `cargo`, `npm`,
  `npx`, `pnpm`, `yarn`, `node`, `python`, `python3`, `ruby`, `bundle`, `swift`, `xcodebuild`,
  `gradle`, `./gradlew`, `mvn`, `dotnet`, `cmake`, `ctest`, `bazel`) MUST set `UnboundedReads` to
  `shell:runs-toolchain`, so it is selected on any dirty path: what such a test reads is the whole
  program the toolchain builds, which no literal bounds. A function the file defines shadows a
  toolchain of the same name only for calls after its definition and outside a `bash -c` string; a
  call through `exec`, `command`, `nohup`, `sudo`, `env`, `xcrun` or `timeout` is never shadowed.
  The bodies of expanding heredocs (unquoted delimiter) MUST be scanned for substitutions.
- `NSA-V0-012`: When the source universe holds source text of another language Corvint plans (any
  name ending in a suffix owned by another plugin in `languages.All()`), every non-test shell unit
  MUST carry the unit frontier `shell:foreign-invocation-unmodelled`, because programs in those
  languages may run a shell script by a computed path. It is a unit frontier, not a global one: a
  toolchain test selected under `NSA-V0-011` is reached on every dirty path, and a global frontier
  would make every plan in a polyglot repository UNKNOWN even when no shell source changed.
  The plugin does not implement `PathReader` for the same reason; literal path-token readers are
  selected without it.
  Amended by `AFP-V0-581` (V1-2897, experimental under the owner's standing lane auto-accept
  (2026-10-11)): when this is the unit's only frontier the unit declares `frontierBears:
  DEPENDENTS`, so the frontier is named only on a plan that follows the unit's dependents (the
  script changed, a dependency it sources or runs changed, it is a test-less reader of an unowned
  dirty path, or it locates the root) or that selects the script as a reader of a dirty path,
  whichever plugin owns it: what the script does changes with a path it reads, and the frontier
  is the only thing that names its unmodelled invokers. A change that reaches no script, or
  selects one only for its tests' sake (`AFP-V0-581` (2)), does not name it.

### Both plugins

- `NSA-V0-013`: Neither plugin may execute repository code, a compiler, a shell or a package manager,
  or read outside the supplied source universe. Both MUST implement `UnitsSource` over
  `affected.Source` so `BuildFS` and the immutable-index path work.
- `NSA-V0-014`: Units, paths, imports, path tokens and frontier reasons MUST be sorted and unique,
  and two builds over the same inputs MUST produce identical graph bytes. Context cancellation MUST
  be honoured by the shared walk.
- `NSA-V0-015`: Both plugins MUST pass the shared seam conformance suite (`core <- mid <- leaf` plus an
  unrelated `solo`) with their permanent frontier declared, and focused tests MUST cover every
  frontier reason, every test-signal family listed above, header-counterpart reach, and shell
  toolchain unboundedness.
- `NSA-V0-016`: (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2708) Amending `NSA-V0-010`, a
  word a command only writes is no path token: the target of a truncating redirection (`>`,
  `>|`), every operand of `mkdir`, and the last operand of `cp` or `mv` unless `-t` or
  `--target-directory` names the destination elsewhere. Every other word stays a token: the target
  of `>>`, `<`, `<>` and `<<<`, every `cp` or `mv` source, a word the same command also reads, and
  every word of any other command. Proof that the dropped units cannot observe the change: the
  test creates or truncates that path, replacing the dirty bytes before anything reads them, and a
  path it also reads keeps its token. Failure modes, all fail closed: a write through `tee`, `dd`,
  `install`, `rsync` or inside `eval` or `bash -c` text keeps its token; a word written in one
  command and read in another keeps the token the read contributes. Rollback: `readWords` returns
  every word.
- `NSA-V0-800`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3382)
  Amending `NSA-V0-006`, the shell plugin MUST also index the shell files under the hidden
  top-level directory `.github`, which the shared source walk skips with every hidden directory,
  through the same `Source` (disk or immutable) with repository-relative paths, skipping hidden and
  skipped directories below it and refusing a non-regular or unnamable shell file as the shared walk
  does. An absent `.github` contributes nothing. Design: `.github/forward-rows/*.sh`, run by the
  `tools/ci-forward-row` tests, were owned files no unit declared, so an edit planned
  `UNINDEXED_SOURCE_PATH`, outside the `AFP-V0-028` allowlist, and `corvint test` fell back to the
  whole repository; indexed, the edit narrows through the same literal-reader selection as any
  other shell file. Non-goals: other hidden directories; files no plugin owns, such as `.jq`, which
  stay `UNOWNED_DIRTY_PATH`. Failure modes: a walk past `MaxWalkEntries` fails the build closed.
  Evidence: `TestIndexesGitHubShellScripts_NSAV0800` (disk and immutable sources; a caller's edge
  into the indexed script; skipped subdirectories; no `.github`) and
  `TestForwardRowsChangeSelectsCIForwardRow_V13382` (this repository: `verdict.sh`, `bar.sh` and
  `bar.jq` edits select `tools/ci-forward-row` with no `UNINDEXED_SOURCE_PATH`; fails on BASE for
  the scripts). Rollback: empty `HiddenRoots`.

## Exclusions

`sql-native`, `shader`, `html-css` and `structured-data` analyzer families get no affected plugin:
they are not test-bearing source languages under this seam. A dirty path of theirs stays
`UNOWNED_DIRTY_PATH`, which already keeps scope UNKNOWN. Java is planned by `java-affected-test-v0.md`.
Extensionless shell executables, shebang detection, per-test-case anchors, runner commands, and
build-system link graphs are V0 non-goals.

## Failure modes

- A false include or invocation edge widens selection; that is acceptable.
- A missing edge narrows selection. The frontiers of `NSA-V0-005` and `NSA-V0-012`, the unbounded
  reads of `NSA-V0-009` and `NSA-V0-011`, and `NO_SELECTABLE_TEST` for an unreached changed unit keep
  such a plan UNKNOWN or widen it instead of reporting it bounded.
- A word a test only writes (`NSA-V0-016`) leaves its tokens; any other way the test could read
  that path (a toolchain run, a dynamic source, a read elsewhere in the file) still selects it
  through `NSA-V0-009`, `NSA-V0-011` or the token the read contributes.
- Registering the plugins changes `corvint affected` output for any repository holding these files:
  paths that were `UNOWNED_DIRTY_PATH` now select tests or report `NO_SELECTABLE_TEST`.

## Acceptance evidence and rollback

Acceptance: the focused package tests, the seam conformance suite with both plugins added, the
`internal/liveverify/affected` and `cmd/corvint` affected tests, and a `corvint affected` run on a
fixture per plugin that selects tests with witnesses.

Rollback: remove `cfamily.New()` and `shell.New()` from `languages.All()` and the conformance list.
No stored format, wire contract or ledger changes, so a revert restores the prior plans exactly.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `NSA-V0-001..005` | `internal/liveverify/affected/cfamily/` | `internal/liveverify/affected/cfamily` package tests |
| `NSA-V0-006..012`, `NSA-V0-016` | `internal/liveverify/affected/shell/` (`readWords`, `targetDirectoryOption`, the `writeRedirect` marker for `NSA-V0-016`) | `internal/liveverify/affected/shell` package tests; `TestWriteOnlyWordsAreNoPathTokens_NSAV0016` (written words absent, read words present; `-t`, `<`, `>>`, `<>`, a source operand and `-T` keep the unit selected) |
| `NSA-V0-012` (amended by `AFP-V0-581`) | `foreignFrontier` in `internal/liveverify/affected/shell/shell.go` | `TestForeignFrontierBearsOnlyWhenDependentsAreFollowed_NSAV0012` |
| `NSA-V0-013..015` | both packages; `internal/liveverify/affected/languages/` | `TestSeamSelectsTheDependencyClosureInEveryLanguage` and the shared seam tests |
| `NSA-V0-800` | `HiddenRoots` and `hiddenFiles` in `internal/liveverify/affected/shell/shell.go` | `TestIndexesGitHubShellScripts_NSAV0800`; `TestForwardRowsChangeSelectsCIForwardRow_V13382` in `internal/liveverify/affected/replay_v13382_test.go` |
