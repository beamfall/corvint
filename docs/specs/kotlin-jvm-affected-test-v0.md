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

Any observed `.java` source raises `kotlin:jvm-sibling-source-present`: the adapter does not claim
Java ownership, parse Kotlin/Java dependency edges, or discover Java test classes, so a bounded
scope cannot be reported for Kotlin exclusions in that mixed JVM tree.

The bounded scanner rejects unterminated comments, strings, characters, and backticked names, but it
is not a Kotlin compiler. It copies a backticked name verbatim up to its closing backtick, so a quote
or apostrophe inside ``fun `it's fine`()`` cannot open a literal that blanks a later qualified
reference and drops a `KAT-V0-006` edge. It ignores a leading UTF-8 byte order mark and accepts any whitespace after the `package`
and `import` keywords, so neither can drop the same-package or import edges `KAT-V0-006` requires. Balanced malformed syntax can remain a graph observation and will be rejected only by the
actual compiler/runtime provider; the adapter does not claim syntax-valid source evidence.

Appium Kotlin tests are discoverable only through their host JUnit/Kotest runner; Appium is not a
separate Kotlin runner. Server lifecycle and the application-under-test remain outside this graph.

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
| `KAT-V0-001..007` | `internal/liveverify/affected/kotlin/kotlin_test.go` |
| `KAT-V0-008` | `internal/liveverify/affected/conformance_test.go` |
| real-repository recall | pinned AndroidX Media ground-truth run described in `docs/BUILD-LOG.md` |

Rollback removes the Kotlin plugin package, its conformance fixture/case, and this experimental
spec. No existing caller registers the plugin implicitly, and existing Go/Python receipt bytes must
remain unchanged.
