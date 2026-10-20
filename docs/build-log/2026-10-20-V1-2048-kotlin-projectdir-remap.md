# V1-2048: Kotlin affected adapter marks a layout mapped only on a project-directory assignment

Date: 2026-10-20
Ticket: V1-2048 (P1 bug; no GitHub issue).
Requirement: `KAT-V0-009` (`docs/specs/kotlin-jvm-affected-test-v0.md`, section "Project-directory
remapping"). It is accepted under the owner's standing auto-accept of lane requirements
(2026-10-11).

## Finding

The 2026-10-20 read-only Beamfall setup review (corvint 1.0.0-rc.3 build 676) found that
`observeBuildLayout` set `projectMapped` whenever a Gradle manifest contained the substring
`projectDir`. Ordinary Android build scripts read the directory (`rootProject.projectDir.resolve(...)`,
`layout.projectDirectory.file(...)`), so in beamfall-android and beamfall-android-ui every Kotlin
test unit became `kotlin:jvm:<module>:unknown:...` with `kotlin:build-target-unresolved` and no
runnable Gradle test address.

## Decision

A layout is custom-mapped only when a manifest may assign a project directory: any whole identifier
`setProjectDir`, or a whole identifier `projectDir` (bare, backticked or quoted) followed, after
whitespace, comments and closing quotes, by `=` (not `==`) or `(`, or followed on the same line by a
Groovy command argument. An unreadable manifest still marks it mapped. Doubtful forms resolve toward
a remap, which only keeps the conservative `unknown` task. Evaluating settings scripts stays out of
scope.

The one Codex review round found that the first cut skipped only whitespace after the identifier, so
commented, backticked and quoted assignments became false negatives with an exact but wrong task, and
that the positive fixture did not require a test unit to exist. Both were fixed before binding: the
trivia skip and the command-call form above, and the fixture now requires exactly one test unit.

## Evidence

- `TestKATV0009ProjectDirectoryReadsKeepGradleTestTask`: an Android-style module whose build script
  reads `rootProject.projectDir`, `layout.projectDirectory`, `projectDir == rootDir` and
  `"$projectDir/..."` yields `kotlin:jvm:app:test:example.ExampleTest` with no build-target or
  test-address frontier. It fails on the previous substring test with
  `frontier=[kotlin:build-target-unresolved]`.
- `TestKATV0009RemapsProjectDirectoryOnlyOnAssignment`: Kotlin and Groovy assignment, setter,
  commented, backticked, quoted and command-call forms are remaps; reads, comparisons,
  interpolation and longer identifiers are not.
- `TestCustomGradleProjectDirectoryDoesNotInventATarget` (existing, now traced to `KAT-V0-009`):
  `project(":app").projectDir = file(...)` in settings keeps the unresolved frontier and task
  `unknown`.

Rollback restores the substring test in `observeBuildLayout`.
