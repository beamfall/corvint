# 2026-10-14 V1-1316: plan-level regressions for the nine Java adapter review findings

V1-1316 holds nine confirmed findings from the read-only review of `40c67a604..efc6dcff3` (V1-1308).
In every finding, `corvint affected` returned a BOUNDED plan that left out a reachable test. Commit
`8a79b4015` ("fix(affected): address the Java adapter review findings (V1-1308)") landed 15 minutes
after the ticket was filed and claims to fix all nine. The ticket was never closed against that
commit. This lane checks that claim against every variant in the ticket body.

The commit's tests (`java_test.go`, `kotlin_test.go`) touch every finding, but mostly at the unit
level: they assert the imports, frontiers, reads or test classification of single files. Only the
finding 6 test calls `affected.Select`. None of them pairs the omitted test with a direct control
test and rules out `NO_SELECTABLE_TEST`, which is how the review showed each omission. Without
that check, a plan can turn UNKNOWN only through `NO_SELECTABLE_TEST` and still hide an omission.

**Result:** all 16 ticket variants pass at BASE, so they needed no production change. The lane
adds one maintained regression per variant. The Codex review then found five close variants of
findings 1, 7, 8 and 9 that BASE still omits from a BOUNDED plan. This lane fixes those five in the
Java and Kotlin adapters and pins each one with its own regression (21 subtests in all).

## Regression contract

`internal/liveverify/affected/java/review_test.go`, `TestReviewFindingsKeepReachableTests_V1_1316`,
has one subtest per variant. Each subtest builds a graph from the Java and Kotlin plugins together,
selects for the dirty file and asserts:

- the direct control test is selected, and no `NO_SELECTABLE_TEST` unknown is present, so the
  outcome cannot be masked;
- either the reachable test is selected in a BOUNDED plan, or the plan is UNKNOWN and names the
  expected frontier;
- for findings 2 and 7, the reachable test also carries the expected `UnboundedReads`
  (`java:reflection` or `java:unresolved-name`), so the selection comes from the specified
  mechanism and not from a coincidental edge.

Each Java fixture, including the three new Java fixtures for the Codex variants, compiles with
`javac` 17.0.20.1 (`-proc:none`), using a stub
`org.junit.jupiter.api.Test` annotation. Finding 7 adds a stub `Generated` in place of the
annotation-processor output. Findings 8b and 9 add Java stand-ins for the Kotlin classes. `javap`
confirms that the `\u000a`, CR-only and spaced-annotation `MissedTest` classes compile a
`@Test void t()` method. For the escaped-header variant, `javap` shows the class compiled as
`p.MissedTest`, so javac does read `package p;` after `// license \u000a`. This checks syntax only; it does not qualify the JUnit runner. Kotlin was
not compiled: no `kotlinc` is installed, so this is `NOT_RUN`.

## Finding x variant table

BASE is `92c298a6e8fa141a3cda035f2d1d8e6c6ebbf431`. Before-state: the same test file run against the
reviewed commit `efc6dcff3` (with the `UnresolvedName` constant spelled as a literal, since that
commit does not define it) fails all 16 subtests. Each failing subtest either reports the review's
BOUNDED omission or, for findings 2 and 7, the missing read. At BASE all 16 pass. The five Codex
variants fail at BASE in the same way and pass after this lane's fix.

| Finding | Variant | Subtest (`TestReviewFindingsKeepReachableTests_V1_1316/…`) | At efc6dcff3 | At BASE | Result |
|---|---|---|---|---|---|
| 1 | two-segment `new p.Core()` | `1_two-segment_qualified_name` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 1 | spaced `com . ex . core . Core` | `1_spaced_qualified_name` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 1 | comment-split `com.ex.core./*comment*/Core` | `1_comment-split_qualified_name` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 2 | `loadClass ("…").getDeclaredConstructor ().newInstance ()` | `2_spaced_reflection_markers` | no read | fixed by 8a79b4015 | `java:reflection`, selected |
| 3 | `Class.forName("com.ex.core.Outer$Inner")` | `3_forName_binary_name_of_a_nested_type` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 4 | `// comment \u000a @Test …` | `4_unicode-escaped_line_terminator` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 4 | CR-only line ends | `4_CR-only_line_ends` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 5 | `@ Test` | `5_spaced_annotation` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 5 | `@/*comment*/Test` | `5_comment-split_annotation` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 6 | `MissedTest extends` abstract `Base` | `6_inherited_test_methods` | BOUNDED, omitted | fixed by 8a79b4015 | selected, BOUNDED |
| 7 | `import com.ex.core.*` + generated `Generated` | `7_generated_type_behind_a_first-party_wildcard_import` | no read | fixed by 8a79b4015 | `java:unresolved-name`, selected |
| 8 | Kotlin `/*license*/ package` → Java `Core` | `8_Kotlin_package_header_after_a_comment` | BOUNDED, omitted | fixed by 8a79b4015 | UNKNOWN, `java:kotlin-dependent` |
| 8 | Java `/*license*/ package` → Kotlin `Core` | `8_Java_package_header_after_a_comment` | BOUNDED, omitted | fixed by 8a79b4015 | UNKNOWN, `kotlin:java-dependent` |
| 9 | `core.kt` → `CoreKt` | `9_capitalised_file_facade` | BOUNDED, omitted | fixed by 8a79b4015 | UNKNOWN, `kotlin:java-dependent` |
| 9 | `@file:kotlin.jvm.JvmName("Api")` | `9_kotlin.jvm-qualified_JvmName` | BOUNDED, omitted | fixed by 8a79b4015 | UNKNOWN, `kotlin:java-dependent` |
| 9 | `@file:JvmName ("Api")` | `9_spaced_JvmName` | BOUNDED, omitted | fixed by 8a79b4015 | UNKNOWN, `kotlin:java-dependent` |
| 1 | annotation `@p.Core` (Codex A) | `1_qualified_annotation_name` | BOUNDED, omitted | fixed here | selected, BOUNDED |
| 7 | `Generated` also declared in unrelated `com.other` (Codex B) | `7_generated_name_also_declared_in_an_unrelated_package` | no read | fixed here | `java:unresolved-name`, selected |
| 8 | Kotlin `/* a /* b */ c */ package` (Codex C) | `8_Kotlin_package_header_after_a_nested_comment` | BOUNDED, omitted | fixed here | UNKNOWN, `java:kotlin-dependent` |
| 8 | Java `// license \u000a package p;` (Codex D) | `8_Java_package_header_after_an_escaped_line_end` | BOUNDED, omitted | fixed here | UNKNOWN, `kotlin:java-dependent` |
| 9 | `@file:JvmName/*comment*/("Api")` (Codex E) | `9_comment-split_JvmName` | BOUNDED, omitted | fixed here | UNKNOWN, `kotlin:java-dependent` |

For the five Codex rows, "At efc6dcff3" means at BASE: the omission persisted until this lane.

## Codex variants and fixes

One read-only Codex round over the lane diff returned CONCERNS with five findings. Each was a
missing variant, not a defect in the regressions. A probe subtest confirmed all five at BASE, and
all five are fixed:

- A. `qualifiedReferences` skipped annotation chains, so `@p.Core` from another package gave no
  edge. Annotation chains now count as qualified references.
- B. `java:unresolved-name` used one global set of known simple names, so a `Generated` declared
  in any package hid a generated one behind `import com.ex.core.*`. Visibility is now per file: its
  own package, packages it imports on demand, Kotlin files in those packages, and the declaring
  files of types it imports or names qualified.
- C and D. Each cross-language header scan used the wrong lexer: Kotlin text went through the Java
  lexer (block comments do not nest), and Java text through the Kotlin lexer (no Unicode escapes,
  CR does not end a line comment). The Java lexer moved unchanged into
  `internal/liveverify/affected/java/javalex`, which the Kotlin plugin now uses for Java text. The
  Java plugin uses the newly exported `kotlin.SourceText` for Kotlin text. Neither plugin package
  imports the other's root in both directions: `java` imports `kotlin`, and `kotlin` imports only
  `java/javalex`.
- E. `@file:JvmName` is now matched on comment-blanked Kotlin text, and its value is read from the
  raw text at the same offset. Text that does not lex is still matched raw.
## Spec

The Codex fixes change behaviour, so the same commit amends the existing rules rather than adding
IDs: `JAT-V0-003` names annotation chains, `JAT-V0-006` defines what a file can see, and
`JAT-V0-008` requires each language's own lexer and comment-tolerant `@file:JvmName`. The
traceability table gains a `TestReviewFindingsKeepReachableTests_V1_1316` row. Accepted under the
owner's standing auto-accept of lane requirements (2026-10-11), V1-1316. `REQUIREMENTS.tsv` is
regenerated.

For the ticket's own findings: finding 1 called the two-segment case an unguarded soundness gap ("follows JAT-V0-003 literally").
`8a79b4015` already amended the rule. `JAT-V0-003` now reads "at least two segments ... with
whitespace or comments allowed around each dot". The same commit amended `JAT-V0-002`
(annotation spacing and inherited discovery), `JAT-V0-004` (Unicode escapes and CR),
`JAT-V0-006` (`java:unresolved-name`), `JAT-V0-007` (whitespace-insensitive triggers) and
`JAT-V0-008` (facade names and comment-safe headers), plus the matching Kotlin spec paragraph. No
rule was missing, so the reserved IDs `JAT-V0-011..013` and `LPCV-V0-020..021` stay unused.

## Verification

- `GOTOOLCHAIN=local go test -count=1 ./internal/liveverify/affected/...`: java, kotlin, languages
  and every other subpackage pass, as do the shared `TestSeam*` and
  `TestComposedGraphKeepsLanguagesIndependent`. The `affected` package passes once the two timing
  tests are skipped. `TestIncrementalSelectionMeetsTheLiveBudget` (p95 246 to 383 ms against a
  100 ms budget) and `TestSelectionOnTheLiveDirtyWorktree` (107 to 791 ms against 100 ms) failed on
  every run, including once alone, at host load averages of about 51 to 81. They time selection
  over this repository's Go/Python graph, which the Java and Kotlin plugins do not join, and live
  in `measure_test.go` (open PR #883). This is recorded as environment noise, not as a pass.
- `internal/specindex`: pass. The 12 CI doc gates pass on the staged tree. The `cmd/corvint`
  tests whose goldens embed an affected-graph digest pass unchanged
  (`TestAFUV1FlowsQueryGoldens`, `TestAFUV1FlowsImpactDirtyWorktreeUnknown`,
  `TestAFUV1024StrictAndCoverageBytesUnchanged`, `TestCoreVerbsEmitTheFrozenProfiles`).
- `-race` on the java and kotlin packages: pass. `go vet` on the same packages: clean.
  `gofmt -l`: empty.
- `corvint affected --base BASE` selects 94 units. java and kotlin are direct changes;
  `affected` and `affected/languages` are dependents. The rest are spec and doc readers
  (path-literal, binary-exec, declared-scope and unbounded readers) plus `LANGUAGE_FRONTIER`
  unknowns. Apart from `affected/...`, `internal/specindex` and the doc gates, those readers were
  not run locally (`NOT_RUN`, left to CI).
- Independent review: one read-only Codex round (CONCERNS, five findings, all fixed above). Under
  the one-round cap, the fixes were not re-reviewed.

## Limits and rollback

- The regressions pin the review's variants, not every Java lexical form. Remaining gaps the spec
  already names stay unchanged: a library test base class, a generated type in the same package
  behind an external on-demand import, and generated packages outside a first-party group.
- For findings 2 and 7, the reachable test is selected because of its unbounded read, not through
  a resolved edge. That satisfies the acceptance clause, since the test is selected.
- Kotlin words still count by package, not by declaration, so a Kotlin file that only mentions a
  name in a comment can still hide `java:unresolved-name` for Java files in its package.
- B's per-file visibility can add `java:unresolved-name` where the old global set did not. One
  case is a member type inherited through a supertype's own supertype, declared in a package the
  file neither imports nor names. That widens selection; it never omits a test.
- Rollback: revert this lane's commit. That restores `java/lex.go`, the global known-name set and
  the raw `@file:JvmName` match, and deletes `javalex`, `review_test.go` and this entry. No stored
  format or wire contract changes, though the affected-graph digest moves wherever it embeds Java
  or Kotlin plugin output.
