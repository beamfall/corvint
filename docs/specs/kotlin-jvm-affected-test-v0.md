# Kotlin/JVM Affected-Test Adapter V0

Owner: Russell Lewis
Date: 2026-08-29
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `AGENTS.md`, `docs/specs/live-proof-carrying-verification-v0.md`, and the
operator-approved Kotlin/JVM adapter task of 2026-08-29

## Agent digest
- Claim: Static Kotlin/JVM analysis conservatively maps changes to JUnit and Kotest selections while exposing unsupported ownership.
- Status: proposed/experimental
- Exists: the adapter contract and `internal/liveverify/affected.Language` integration seam.
- Blocked on: unresolved static-discovery frontiers and the missing LPCV exclusion-evidence fields.
- Read next: User job and boundary; Requirements; Unit identity and invocation.

## User job and boundary

Corvint needs to map a Kotlin source edit to statically reachable JVM or Android test units without
executing Gradle, Maven, a JVM, an emulator, or repository code. This adapter is an experimental
source observation behind `internal/liveverify/affected.Language`; it does not qualify a runtime,
claim safe test omission, or amend the LPCV promotion tranche.

## Requirements

- `KAT-V0-001`: The adapter MUST own exactly `.kt` and `.kts` source paths and return deterministic,
  sorted, `kotlin:`-namespaced units. It MUST NOT claim YAML, Gherkin, Java, or another host
  language's Appium source.
- `KAT-V0-002`: One source file is one path-owned unit. A runnable test unit is the statically known
  set of top-level runner classes in that file. A unit with one class is addressable as specified
  below. Multiple, generated, inherited, or otherwise incomplete class sets MUST raise a frontier.
- `KAT-V0-003`: JUnit Jupiter discovery MUST cover `Test`, `ParameterizedTest`, `RepeatedTest`,
  `TestFactory`, and `TestTemplate`, including imported aliases and fully qualified annotations.
  JUnit 4 and `kotlin.test.Test` are retained for Android/Robolectric compatibility.
- `KAT-V0-004`: Kotest discovery MUST cover direct `StringSpec`, `FunSpec`, `BehaviorSpec`,
  `DescribeSpec`, `ShouldSpec`, `FeatureSpec`, `FreeSpec`, `WordSpec`, `ExpectSpec`, and
  `AnnotationSpec` subclasses. Their different leaf DSLs run through the containing Spec class;
  MockK is auxiliary and MUST NOT be treated as a runner.
- `KAT-V0-005`: `src/test` and `src/jvmTest` are local JVM partitions. `src/androidTest` is an
  instrumentation partition used by Espresso and UI Automator. Robolectric in `src/test` remains a
  local JVM test. A conflicting framework/source-set combination MUST raise a frontier.
- `KAT-V0-006`: Static package/import edges, aliases, wildcard imports, same-package visibility,
  and applicable Kotlin Gradle configuration edges MUST participate in selection. Ambiguous
  declarations MUST retain every candidate edge and raise a frontier.
- `KAT-V0-007`: Unparsed source, dynamic/reflection-driven discovery, custom source sets, unknown
  build targets, ambiguous test addresses, non-build Kotlin script contexts, sibling Java source,
  and custom discovery MUST become `Result.Frontier`, producing `UNKNOWN_SCOPE` under LPCV-V0-016.
- `KAT-V0-008`: Fixed inputs MUST reproduce byte-identical graph and selection bodies
  under LPCV-V0-019. The unchanged seam supplies an LPCV-V0-013 dependency witness for each selection and
  its current bounded exclusion record for each unselected eligible test. That exclusion record
  does not carry separate provider and evidence-identity fields required by literal LPCV-V0-014;
  this adapter cannot close that interface-level requirement and MUST NOT claim it does.
- `KAT-V0-009`: Only a Gradle manifest that may assign a project directory (`projectDir = ...`,
  `project(":x").projectDir = ...`, a backticked, quoted or commented form of that assignment, a
  `setProjectDir` call, or a Groovy `projectDir` command call) or cannot be read MUST mark the
  layout as custom-mapped and raise the `kotlin:build-target-unresolved` frontier with task
  `unknown`. Reading a project directory (`rootProject.projectDir.resolve(...)`,
  `layout.projectDirectory.file(...)`, `projectDir == rootDir`, or `"$projectDir/..."`) MUST NOT
  mark it, so a conventional module keeps its exact Gradle test task.
- `KAT-V0-761`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2054)
  Amending `KAT-V0-001`: the adapter MUST also own each Gradle default version catalog, a path whose
  base name is `libs.versions.toml` in a directory named `gradle`. A catalog is the build
  configuration unit `kotlin:config:<path>`, never scanned as Kotlin, and every Kotlin unit under
  its build root (the directory holding that `gradle/` directory; the repository root for
  `gradle/libs.versions.toml`) imports it, build scripts included. The stated reason: a catalog
  alias can reach any module through a build script or a convention plugin, and alias-level
  resolution is not modelled, so a catalog change selects every test of that build rather than
  guessing which aliases a module reads. Catalogs named by `versionCatalogs { from(...) }` elsewhere
  stay unowned. Rollback: drop the catalog from `Owns` and the config edge.
- `KAT-V0-762`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2054) A
  module whose Gradle build script applies the `com.android.test` plugin (the literal plugin id, or
  `alias(libs.plugins.NAME)` where a version catalog of that build maps `NAME` to that id) is an
  Android test module: its tests live in `src/main`. A test there MUST be the typed unit
  `kotlin:android-test:<module>:unknown:<classes>` and raise the typed frontier
  `kotlin:android-test-module`, because the connected task is variant-defined by the build (for
  example `connectedBenchmarkAndroidTest`), rather than the `kotlin:unknown` kind with
  `kotlin:custom-source-set`. Non-goals: no task-name inference, no device execution. Rollback:
  remove the module detection and the frontier.

## Unit identity and invocation

Source units use `kotlin:source:<repository-path>`. Kotlin Gradle build scripts use
`kotlin:config:<repository-path>` and are conservative dependencies of affected descendant units.
Runnable unit identities have this shape:

```text
kotlin:<jvm|instrumentation|maven>:<module-directory>:<task>:<fully-qualified-class>
```

For a standard Gradle module, replace a slash-separated module directory with its colon-separated
Gradle project path. The exact class-level commands are:

```sh
./gradlew :module:test --tests 'com.example.MyTest'
./gradlew :app:testDebugUnitTest --tests 'com.example.MyRobolectricTest'
./gradlew :app:connectedDebugAndroidTest -Pandroid.testInstrumentationRunnerArguments.class=com.example.MyDeviceTest
./mvnw -pl module -Dtest=com.example.MyTest test
```

The unqualified `src/test` and `src/androidTest` aggregate task identities are `test` and
`connectedAndroidTest`. Variant/custom source sets, custom tasks, Gradle JVM test suites, KMP target
fan-out, custom project-directory mappings, or a file with more than one possible runner class are
not asserted to have an exact invocation; the adapter raises a frontier. The
unchanged `Unit` wire carries paths and IDs but no command field, so a runtime provider must
deterministically derive and validate the command from the identity and repository configuration
before execution.

## Discovery and dependency limits

Static class selection still executes parameterized, repeated, factory-created, and Kotest DSL
children inside the selected class. Their child identities and counts are runtime-only. The adapter
raises `kotlin:dynamic-test-discovery` for reflection/class loading, JUnit templates and suite
selectors, Kotest included/data factories, and other source-visible dynamic registration. Custom
composed annotations, external inherited tests/specs, generated sources, custom engines/extensions,
DI/service loading, resources, fixtures, and build-logic-defined source sets cannot be closed by
this source-only adapter and remain unknown. Kotlin under conventional `buildSrc`/`build-logic`
paths and Kotlin Gradle scripts conservatively reaches repository tests instead of being treated as
ordinary application source. A wildcard import whose name is no repository package imports the
members of a class, object, or enum, so it resolves through that declaration or its enclosing
package exactly as a single-name import does. When a top-level declaration and a package share the
qualified name an import walks back to, both remain candidate edges and the ambiguous-declaration
frontier is raised, so the declaration cannot hide the package.

Java is owned by the `java` plugin (`java-affected-test-v0.md`). A Kotlin unit that observed Java
text can reach carries the unit frontier `kotlin:java-dependent` (`JAT-V0-008`): Java reaches it by a
declaration's simple name, an alias import name, or its file facade (the capitalised stem with `Kt`,
so `core.kt` is `CoreKt`, or the `@file:JvmName` value), within the same package, a single or on-demand import, or a qualified
reference. The Java dependent belongs to another namespace, so a plan that reaches such a unit is
UNKNOWN; a plan in a mixed JVM tree that reaches none stays bounded.

The bounded scanner rejects unterminated comments, strings, characters, and backticked names, but it
is not a Kotlin compiler. It copies a backticked name verbatim up to its closing backtick, so a quote
or apostrophe inside ``fun `it's fine`()`` cannot open a literal that blanks a later qualified
reference and drops a `KAT-V0-006` edge. It ignores a leading UTF-8 byte order mark and accepts any whitespace after the `package`
and `import` keywords, so neither can drop the same-package or import edges `KAT-V0-006` requires. Balanced malformed syntax can remain a graph observation and will be rejected only by the
actual compiler/runtime provider; the adapter does not claim syntax-valid source evidence.

Appium Kotlin tests are discoverable only through their host JUnit/Kotest runner; Appium is not a
separate Kotlin runner. Server lifecycle and the application-under-test remain outside this graph.

## Project-directory remapping (`KAT-V0-009`)

A Gradle settings script can remap where a project lives, after which the module directory no
longer names the Gradle project path and no exact task can be derived. Ordinary Android build
scripts read the project directory instead, for signing, ProGuard, or generated-file paths. Treating
every mention of `projectDir` as a remap turned every test in such a repository into
`kotlin:jvm:<module>:unknown:...` with build-target and test-address frontiers (V1-2048).

- Detection, anywhere in a `build.gradle`, `build.gradle.kts`, `settings.gradle`,
  `settings.gradle.kts` or `pom.xml` manifest: any whole identifier `setProjectDir`; or a whole
  identifier `projectDir` (bare, backticked or quoted) that is followed, after whitespace, comments
  and closing quotes or backticks, by `=` (not `==`) or `(`, or that is followed on the same line by
  whitespace and an argument (a Groovy command call such as `projectDir file('m')`).
- Doubtful forms resolve toward a remap. Comments and string literals are not excluded, an assigned
  local variable or named argument called `projectDir` counts, and `"$projectDir is ..."` reads as a
  command call; each over-reports a remap, which only keeps the conservative `unknown` task.
- Failure modes: a remap through another API (for example a `ProjectDescriptor` obtained under a
  different name and assigned through reflection, or a remap in an included build-logic plugin) is
  not seen, and a derived task can name the wrong project path. The runtime provider still validates
  the command against the repository configuration before execution, as stated above.
- Non-goals: evaluating settings scripts, resolving the remapped directory to a Gradle path, or
  parsing Gradle DSL beyond this bounded token test.
- Evidence: `TestKATV0009ProjectDirectoryReadsKeepGradleTestTask`,
  `TestKATV0009RemapsProjectDirectoryOnlyOnAssignment`, and
  `TestCustomGradleProjectDirectoryDoesNotInventATarget` in
  `internal/liveverify/affected/kotlin/kotlin_test.go`.
- Rollback: restore the substring test in `observeBuildLayout`; every Gradle test whose manifests
  mention `projectDir` then returns to task `unknown`, which is conservative but not runnable.

## Cross-language ownership decision required

Maestro flows (`.yaml`/`.yml`) and Gherkin scenarios (`.feature`) are language-neutral artifacts.
Appium bodies belong to whichever host language contains them. This Kotlin plugin deliberately does
not claim those paths.

The current seam has no multi-owner semantics. If two plugins admit one path, `Build` fails with
`ErrDuplicateOwner`. If several plugins merely return `Owns(path) == true` without admitting it,
dirty-path classification collapses the conflict into `UNINDEXED_SOURCE_PATH` and does not identify
which runner or witness should receive it. Silently assigning YAML or Gherkin to Kotlin would also
prevent another language plugin from representing the same scenario.

A resolution must define either one authoritative artifact owner, explicit multi-owner fan-out, or
a non-language artifact namespace. It must also specify collision ordering, unit identity, runner
association, dependency edges to each host language, witness and exclusion semantics, and dirty
path behavior. No such choice is made by this adapter; until the operator accepts one, these files
remain unowned and a dirty one correctly widens with `UNOWNED_DIRTY_PATH`.

## Acceptance evidence and rollback

| Requirement | Evidence |
|---|---|
| `KAT-V0-001..007`, `KAT-V0-009` | `internal/liveverify/affected/kotlin/kotlin_test.go` |
| `KAT-V0-761` | `TestVersionCatalogIsABuildConfigurationOfItsBuild_KATV0761` in `internal/liveverify/affected/kotlin/catalog_test.go` |
| `KAT-V0-762` | `TestAndroidTestModuleYieldsATypedUnit_KATV0762` in `internal/liveverify/affected/kotlin/catalog_test.go` |
| `KAT-V0-008` | `internal/liveverify/affected/conformance_test.go` |
| real-repository recall | pinned AndroidX Media ground-truth run described in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md` |

Rollback removes the Kotlin plugin package, its conformance fixture/case, and this experimental
spec. No existing caller registers the plugin implicitly, and existing Go/Python receipt bytes must
remain unchanged.
