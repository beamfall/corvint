# Java Affected-Test Adapter V0

Owner: Russell Lewis
Date: 2026-10-12
Intent status: accepted
Delivery status: experimental
Authoritative inputs: ticket V1-1308 (owner request 2026-10-12, milestone `v1-0`), standing
auto-accept of lane requirements from 2026-10-11, `affected-plan-v0.md`,
`live-proof-carrying-verification-v0.md`, `kotlin-jvm-affected-test-v0.md`,
`native-shell-affected-test-v0.md`, `../SPEC-DRIVEN-DEVELOPMENT.md`

## Agent digest
- Claim: Static Java analysis maps dirty `.java` sources to JUnit/TestNG tests; unresolved imports, reflection, build files and Kotlin reach stay UNKNOWN or widen.
- Status: accepted/experimental
- Exists: `internal/liveverify/affected/java`, registered in `languages.All()`, with focused and seam conformance tests.
- Blocked on: runner execution (Gradle/Maven test filters) and per-test-case anchors remain `NOT_RUN`.
- Read next: Requirements; Failure modes; Traceability.

## User and measurable job

Before this slice `corvint affected` had no Java plugin. The `kotlin` plugin owns only `.kt` and
`.kts`, so a dirty `.java` path was `UNOWNED_DIRTY_PATH` and any `.java` file in the tree made every
Kotlin plan UNKNOWN. Given a dirty Java path, Corvint should name every statically addressable test
file that can depend on it, with a `DEPENDENCY_PATH` witness. It should keep scope UNKNOWN, or
widen selection, wherever generated source, reflection, a build file or Kotlin code could reach
that path without being seen.

This slice plans only. It runs no compiler, build tool, annotation processor or test runner.

## Requirements

- `JAT-V0-001`: A plugin with namespace `java` MUST own exactly the names ending `.java`. Build files
  (`pom.xml`, `build.gradle`, `build.gradle.kts`, `settings.gradle*`) and resources are not owned:
  a change to one stays `UNOWNED_DIRTY_PATH`, which keeps the plan UNKNOWN, because the module
  graph, dependency versions and source-set layout they define are not modelled.
- `JAT-V0-002`: One unit MUST be one file, identity `java:<path>`. A file is a test, with `Tests`
  set to its path, only when it carries a runner signal and lies in a test location. Runner
  signals are a `@Test`, `@ParameterizedTest`, `@RepeatedTest`, `@TestFactory`, `@TestTemplate`,
  `@RunWith` or `@Nested` annotation (simple or qualified, with any whitespace or comment after the
  `@`), a `TestCase` reference (JUnit 3), or a `testng` package reference. A file also carries a
  signal when it extends or implements a type whose declaring file it reaches by an edge and that
  carries a signal, directly or by the same rule, because runners execute inherited test methods. A test location is a `test` or `tests` directory component, a
  component ending `Test` (Gradle `integrationTest`, Android `androidTest`), or a stem ending `Test`,
  `Tests`, `IT` or `TestCase` or starting `Test`. Every other file, including a helper under
  `src/test`, is a source unit. A base class outside the observed sources (a library test base
  other than `TestCase`) is not seen; that is a known gap.
- `JAT-V0-003`: Imports MUST include the declaring file of each type named by: a single-type import
  (resolved by the longest dotted prefix that names a declared type, so `p.Outer.Inner` reaches
  `p.Outer`); a static import (`import static p.C.m` and `p.C.*` reach `p.C`); a type-import-on-demand
  `import p.*` (only the types of `p` the file names by simple identifier); a same-package type
  named by simple identifier; a dotted reference in code, an annotation name included (`@p.Core`), of
  at least two segments with a lowercase first segment (a qualified name), with whitespace or
  comments allowed around each dot; and the
  string literal argument of `Class.forName`, read as a binary name, so `p.Outer$Inner` also
  reaches `p.Outer`.
  Declared types include classes, interfaces, enums, records and annotation types, with nested types
  qualified by their enclosing type.
- `JAT-V0-004`: Comments and the contents of string, character and text-block literals MUST NOT
  create edges. Unicode escapes MUST be translated before lexing, as `javac` does, so an escaped line
  terminator ends a line comment; LF, CR and CRLF all end a line. An unreadable file, a malformed
  Unicode escape, or an unterminated comment, string or text block MUST raise the global frontier
  `java:unparsed-source` and yield no unit.
- `JAT-V0-005`: A name declared by more than one file MUST keep an edge to every candidate and raise
  the global frontier `java:ambiguous-type`.
- `JAT-V0-006`: An import that resolves to no observed type, and whose package is first-party, MUST
  set `UnboundedReads` to `java:unresolved-import`. A non-test unit MUST also set `LocatesRoot`, so
  every test that reaches it is selected on any dirty path. First-party means the import's package
  equals, contains or lies under a package an observed file declares, or shares its first two
  segments with such a package when that package has more than two. This covers generated and
  annotation-processed source, which the graph cannot see. An unresolved import outside first-party
  packages is an external dependency and adds nothing. A file that names a capitalised simple
  identifier (an uppercase initial and a lowercase letter, not after a `.`) that the file cannot see
  from first-party source, that no single-type import names and that `java.lang` does not hold MUST
  set `UnboundedReads` to `java:unresolved-name`, with `LocatesRoot` on a non-test unit,
  when a first-party package could supply it: the file imports a first-party package on demand, or
  has no external on-demand import. Behind an external on-demand import alone the name is
  attributed to that package; a generated same-package type there is a known gap. A file sees a
  name when a Java type in its own package or in a package it imports on demand declares it (with
  its nested types), a Kotlin file in one of those packages mentions it or has it as facade, or the
  file declaring a type it imports or names qualified declares it. A same-named type in any other
  package does not hide the name (V1-1316).
- `JAT-V0-007`: A file that loads classes or members by computed name MUST set `UnboundedReads` to
  `java:reflection`, and a non-test unit MUST also set `LocatesRoot`. The triggers are a
  `Class.forName` call without one literal dotted argument, `ServiceLoader`, `loadClass`,
  `getMethod`, `getDeclaredMethod`, `getField`, `getDeclaredField`, `getConstructor`,
  `getDeclaredConstructor`, `newInstance` or `MethodHandles`, matched with whitespace and comments
  removed, so `l.loadClass ("x")` is a trigger.
- `JAT-V0-008`: Kotlin and Java cross-reach is reported as unit frontiers, not edges, because each
  language's dependents belong to another plugin's namespace:
  - A Java unit that a Kotlin file can reach MUST carry the unit frontier `java:kotlin-dependent`.
    The Kotlin file reaches it when it imports one of its top-level types (an alias import needs no
    later mention), or names the type by simple identifier and also shares its package, imports its
    package on demand, or qualifies into it.
  - Under the matching amendment of `kotlin-jvm-affected-test-v0.md`, a Kotlin unit that Java text
    can reach carries `kotlin:java-dependent`. Java reaches Kotlin by JVM names: a declaration's own
    name, or the file facade (the capitalised stem with `Kt`, so `core.kt` is `CoreKt`, or the
    `@file:JvmName` value, simple or `kotlin.jvm`-qualified, with any spacing).
  - Each side reads the other's package and import headers after blanking comments with the
    other language's own lexer, and qualified names with whitespace allowed around each dot. Java
    text is lexed as javac does (Unicode escapes translated, CR ends a line comment, block comments
    do not nest); Kotlin text as kotlinc does (block comments nest). The `@file:JvmName` annotation
    may also hold comments (V1-1316).
  A reached unit carrying either frontier makes the plan UNKNOWN. A plan that reaches neither kind of
  unit stays bounded in a mixed JVM tree.
- `JAT-V0-009`: The plugin MUST NOT execute repository code, a compiler, a build tool or an
  annotation processor, and MUST NOT read outside the supplied source universe. It MUST implement
  `UnitsSource` over `affected.Source`. Units, edges and frontier reasons MUST be sorted and unique,
  and two builds over the same inputs MUST produce identical graph bytes.
- `JAT-V0-010`: The plugin MUST pass the shared seam conformance suite (`core <- mid <- leaf` plus an
  unrelated `solo`) with no permanent frontier, so a plan over the fixture is BOUNDED. Focused tests
  MUST cover every frontier and unbounded-read reason, every import form above, the test-discovery
  rule, and the Kotlin cross-reach rule.

## Non-goals

Path tokens, per-test-method anchors, runner commands, Gradle/Maven module dependency graphs,
source-set configuration, JPMS `module-info.java` `requires` edges, and Groovy or Scala sources.
Edges cross module boundaries whenever a fully qualified name matches, which can only widen
selection.

## Failure modes

- A false edge (a word that matches a same-package type name, or a dotted name that is not a type)
  widens selection; that is acceptable.
- A missing edge narrows selection. Reflection, generated source and annotation processing are kept
  safe by `JAT-V0-006` and `JAT-V0-007`. Build files by `JAT-V0-001`, Kotlin reach by
  `JAT-V0-008`, and `NO_SELECTABLE_TEST` for an unreached changed unit keep such a plan UNKNOWN or
  widen it instead of reporting it bounded. A generated package that is neither under nor beside a
  first-party package's two-segment group (`example.gen` beside `example.core`) is treated as
  external; this is a known gap. So are a library test base class (`JAT-V0-002`) and a generated
  same-package type behind an external on-demand import (`JAT-V0-006`).
- `java:unresolved-name` also fires on names it cannot tell from types, such as a capitalised enum
  constant in a `case` label or a multi-letter type parameter. That widens selection.
- Registering the plugin changes `corvint affected` output for any repository holding `.java` files.
  Those paths stop being `UNOWNED_DIRTY_PATH`, and Kotlin plans in a mixed tree are no longer UNKNOWN
  merely because a `.java` file exists.

## Acceptance evidence and rollback

Acceptance: the focused `internal/liveverify/affected/java` and `kotlin` package tests, the seam
conformance suite with `java` added, and a `corvint affected` run on the shared fixture with a
dirty Java path that selects tests with `DEPENDENCY_PATH` witnesses.

Rollback: remove `java.New()` from `languages.All()`, the conformance list and
`shell.ForeignSourceSuffixes`, and revert the Kotlin amendment so a `.java` file again raises
`kotlin:jvm-sibling-source-present`. No stored format, wire contract or ledger changes.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `JAT-V0-001..009` | `internal/liveverify/affected/java/` | `internal/liveverify/affected/java` package tests |
| `JAT-V0-008` | `internal/liveverify/affected/kotlin/java.go`, `internal/liveverify/affected/java/javalex/` | `TestJavaSiblingSourceWidensCrossLanguageDependency`, `TestJavaReachesFacadesAndCommentedHeaders` |
| `JAT-V0-002..008` (V1-1316 review variants) | `internal/liveverify/affected/java/`, `internal/liveverify/affected/kotlin/java.go` | `TestReviewFindingsKeepReachableTests_V1_1316` |
| `JAT-V0-010` | `internal/liveverify/affected/languages/` | `TestSeamSelectsTheDependencyClosureInEveryLanguage` and the shared seam tests |
