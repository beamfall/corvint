# Kotlin/Android Native Candidate V0

Owner: Russell Lewis
Intent status: proposed
Delivery status: deferred
Disposition: return only with a general extractor; retain this candidate's fact matrix as the model.

## Agent digest
- Claim: A deferred isolated Kotlin/Android extractor retains a structural fact matrix but is not registered, selected, or qualified.
- Status: proposed/deferred
- Exists: an isolated candidate contract and retained fact matrix.
- Blocked on: a general extractor; this candidate remains unregistered and unqualified.
- Read next: Intent, boundary, and non-goals; Requirements; Closed fact matrix.

## Intent, boundary, and non-goals

This separately buildable native-Go executable recognizes one byte-pinned Beamfall Android dogfood
projection. It is neither selected, installed, launched, admitted, nor an exact verifier. It
consumes one canonical LF-framed request on standard input. It has no repository, Gradle, Java,
process, network, environment, HOME, CWD, or pathname-opening capability.

This document is self-contained for the Kotlin/Android candidate. The general initial-candidate
profile deliberately governs other named language families and does not silently govern this one.
All requirements below remain experimental; they are not an assertion of support or qualification.

## Requirements

- `KA-V0-001`: The command and its package remain isolated, unregistered, and unreachable from
  Core selection. The wire family is exactly `kotlin-android`.
- `KA-V0-002`: A request is one UTF-8, canonical-JSON object followed by one LF. It has the closed
  root, target, and input key sets; no duplicate or unknown key, whitespace, alternate ordering,
  malformed base64, or noncanonical value is accepted. `target.features` is required and is exactly
  the non-null array `[]` for this projection; `null` is noncanonical.
- `KA-V0-003`: The target is exactly Android/arm64/none. The twelve input rows below occur once,
  in canonical input order, with their listed path, SHA-256 pin, and byte content. Missing, extra,
  duplicate, shifted, or changed content rejects the whole request before usable facts.
- `KA-V0-004`: Extractors emit only the closed fact matrix below. A Gradle alias retains its full
  static dotted version-catalog coordinate (for example `libs.androidx.activity.compose`), never
  the collapsed value `libs`. Kotlin source symbols, Java toolchain values, Gradle plugin/version
  coordinates, Android SDK/application values, and catalog version coordinates are distinct
  textual identities; the candidate does not resolve, translate, or infer one from another. A
  Gradle or Android string literal that interpolates (`$name` or `${...}`) and would otherwise
  supply a fact value rejects the whole request with `DYNAMIC_INPUT` rather than emitting a fact
  about bytes the interpolation would supply at runtime; a literal `$` not followed by an
  identifier start or `{` is not interpolation, and an interpolated literal that feeds no fact
  (for example inside a log or exception message) does not by itself reject the request.
- `KA-V0-005`: Every fact has the input handle, compilation unit, exact digest, and one-based,
  half-open source span that produced it. Lexer tokens retain both start and exclusive end
  line/column; all four coordinates participate in the evidence digest.
- `KA-V0-006`: Request bytes are limited to 1,500,000; aggregate decoded input bytes and
  LF-inclusive output bytes are each limited to 1,048,576; inputs are limited to 128, features to
  64, facts to 4,096, nesting to 8, JSON tokens to 4,096, and input base64 strings to 1,398,104.
  The bounded field-streaming encoder charges every success and rejection byte before append,
  returns no partial frame, and falls back to a small sentinel if an echoed invalid request would
  overflow. A rejection binds only after family, identifiers, target, input handles, and input
  digests pass their grammar and duplicate checks; before that it is the fixed sentinel, and a
  malformed digest rejects as `DIGEST_MISMATCH` (`TestInvalidEnvelopeRejectionStaysUnbound`).
- `KA-V0-007`: A regular-file descriptor is read only through that descriptor. Its identity,
  mode, size, modification time, and two content snapshots are checked without a pathname reopen;
  same-inode mutation rejects. A non-regular standard-input stream retains descriptor identity and
  mode checks but has no seek/reopen capability.
- `KA-V0-008`: The candidate does not execute Gradle, Java, Git, a shell, or another process; open
  a repository or ambient file; read environment or CWD; load a library; or contact a network.
- `KA-V0-009`: Facts are typed-sorted and unique. A successful result is one canonical object plus
  LF with reason `NONE`; a rejected result has no facts, path, content, or diagnostic text.
- `KA-V0-010`: Focused correctness, race, vet, cross-build, CEM, and isolation evidence may
  demonstrate this candidate slice only. Registry, selection, admission, exact verification,
  installation, launch, host containment, and support remain `NOT_RUN`.

## Exact projection and input matrix

The Android revision is `52799c501cc291ff003d05f9eda391c2008cd51e`; the Android-UI revision is
`6e379d7feec88128439d1753325bcfb22194fdfc`. The SHA-256 values are source constants in
`internal/analyzerkotlinandroid/analyzer.go`; checked-in testdata is the literal byte corpus.

| Family | Exact logical path | Closed extractor |
|---|---|---|
| `android.project.revision` | `project.revision` | LF revision literal |
| `android.gradle.build` | `build.gradle.kts` | static Gradle Kotlin DSL |
| `android.gradle.settings` | `settings.gradle.kts` | static Gradle Kotlin DSL |
| `android.gradle.wrapper` | `gradle/wrapper/gradle-wrapper.properties` | closed properties rows |
| `android.project.version` | `gradle/version.properties` | closed properties rows |
| `android.version.catalog` | `gradle/libs.versions.toml` | closed catalog rows |
| `android.gradle.module` | `app-mobile/build.gradle.kts` | static Gradle Kotlin DSL |
| `kotlin.source` | `app-mobile/src/main/kotlin/com/beamfall/mobile/BuildIdentity.kt` | static Kotlin lexer |
| `android-ui.gradle.build` | `ui/build.gradle.kts` | static Gradle Kotlin DSL |
| `android-ui.version.catalog` | `ui/gradle/libs.versions.toml` | closed catalog rows |
| `android-ui.kotlin.source` | `ui/kit/src/main/kotlin/com/beamfall/kit/BeamfallKit.kt` | static Kotlin lexer |
| `android.ui.revision` | `android-ui.revision` | LF revision literal |

## Closed fact matrix

| Fact kind | Closed source form | Value identity |
|---|---|---|
| `android.project.revision`, `android.ui.revision` | exact revision input | respective revision only |
| `android.gradle.plugin` | `id("PLUGIN") version "VERSION"` | literal Gradle plugin version |
| `android.gradle.plugin.alias` | `alias(libs.DOTTED)` | full `libs.DOTTED` coordinate |
| `android.gradle.dependency` | dependency call with string or `libs.DOTTED` | literal coordinate or full alias |
| `android.gradle.dependency.alias` | dependency call with `libs.DOTTED` | full `libs.DOTTED` coordinate |
| `android.gradle.project` | static `include` or `includeBuild` | literal project value |
| `android.gradle.android` | static Android scalar assignment/call | Android scalar only |
| `android.gradle.source-set` | static `srcDir` or `srcDirs` | literal source directory |
| `gradle.distribution` | pinned wrapper distribution row | Gradle version only |
| `android.project.version`, `android.project.version-code` | pinned properties row | Android application value only |
| `android.version.catalog`, `android.version.catalog.plugin` | closed TOML versions/plugins row | catalog key/value only |
| `kotlin.package`, `kotlin.import.static`, `kotlin.annotation` | static Kotlin lexical form | source spelling only |
| `kotlin.declaration`, `kotlin.context.receiver` | static declaration/context form | source spelling/presence only |
| `kotlin.type.generic`, `kotlin.type.nullable`, `kotlin.call.static` | static lexical use | source spelling only |

No Java fact kind, installed-tool fact, compatibility fact, resolved dependency, runtime fact,
support assertion, or PASS fact is in this candidate matrix.

## Closed rejection matrix

| Reason | Boundary |
|---|---|
| `ANALYZER_FAILURE` | an impossible rejection-encoding failure only |
| `LIMIT_EXCEEDED` | request, input, count, parser, or aggregate byte bound |
| `OUTPUT_LIMIT` | success or rejection envelope would exceed the output cap |
| `NONCANONICAL_REQUEST` | framing, canonical JSON, UTF-8, or `features:null` failure; descriptor or reader failure, including detected descriptor drift (`ACP-012`, decision 0228) |
| `UNKNOWN_FIELD` | closed object key-set failure whose only defect is a safe canonical extension, bound (decision 0222, `TestSafeCanonicalExtensionBindsUnknownField`); any other unknown key is `NONCANONICAL_REQUEST` |
| `UNKNOWN_FAMILY`, `UNSUPPORTED_SCHEMA` | unsupported request family or input schema |
| `INVALID_IDENTIFIER`, `INVALID_PATH`, `MALFORMED_INPUT` | closed scalar grammar failure |
| `DUPLICATE_VALUE`, `AMBIGUOUS_BINDING` | ordering, duplicate value, or multiple binding failure |
| `DIGEST_MISMATCH`, `EXACT_BINDING_UNAVAILABLE` | supplied digest/content or pinned projection mismatch |
| `DYNAMIC_INPUT` | a fact-bound Gradle/Android string literal interpolates (`$name` or `${...}`); the candidate never invents a fact about bytes the interpolation would supply at runtime |

## Required evidence before any promotion

The package retains literal-fixture, hostile lexer, exact success/error byte, 1,001 fresh built-CLI
success, ordered-permutation rejection, full global-bound under/at/over, every-reason,
descriptor/process/network/ambient spy with positive-control, race, vet, Darwin/Linux/Windows,
Core-zero-delta, CEM, and causal allocation ratchet evidence. The paired latency observation is a
non-promotional regression guard unless repeated controlled measurements establish a stable causal
result. This specification makes no claim that any promotion condition is satisfied.
