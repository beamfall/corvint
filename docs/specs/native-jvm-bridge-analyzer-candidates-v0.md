# Native/JVM Bridge Analyzer Candidates V0

Owner: Russell Lewis
Date: 2026-08-25
Intent status: proposed
Delivery status: deferred
Disposition: 12 real target files and three CLIs leave three of the five ACP-009 binaries at zero
headroom; reallocate their byte budget to Python. Return only with a general extractor.
Authority: this profile document, `analyzer-capability-contract-v0.md`, and the direct owner task.

## Agent digest
- Claim: Three isolated native-Go extractors emit bounded structural candidates for C/JNI, Objective-C, and Java bridge source.
- Status: proposed/deferred
- Exists: isolated C, Objective-C, and Java candidate extractors plus pinned corpus evidence.
- Blocked on: zero binary-budget headroom, independent review, Windows execution, and promotion evidence.
- Read next: Job and boundary; Matrix and evidence; Rollback and promotion.

## Job and boundary

This slice supplies three independently buildable, unregistered native-Go lexical extractors for
the real Beamfall C/JNI, Objective-C, and Java bridge corpus. It reports only structural candidate
facts. Parser-local source byte/line/column spans and lexical witnesses are not emitted. It is not a
compiler, parser authority, verifier, admission result, runtime observation, or selection mechanism.

All commands use the separate versioned profile `corvint-analyzer-native-bridge/v0`. This document
owns that wire profile. It deliberately reuses the bounded JSON field shape of
`analyzer-candidate-profiles.md`, but does not amend or broaden that document's frozen Go,
JavaScript/TypeScript, .NET, and Ruby family registry. The native bridge remains an external plugin
surface with no Core registry or launch path.

| Command family | Wire profile | Language/toolchain/target coordinate |
| --- | --- | --- |
| `c` | `corvint-analyzer-native-bridge/v0` | C lexical / NDK r27d / `android,arm64-v8a,android-24` |
| `objective-c` | `corvint-analyzer-native-bridge/v0` | Objective-C lexical / Apple clang 16 / `darwin,arm64,ios-17.0` |
| `java` | `corvint-analyzer-native-bridge/v0` | Java lexical / JDK 17 / `android,arm64-v8a,android-24` |

No profile infers a nearest revision, target, toolchain, ABI, language, or feature. Inputs are only
caller-provided, immutable byte records and their caller-declared SHA-256 digests; a digest mismatch
rejects before facts are retained. Core imports none of the candidate packages and has no registry,
activation, launch, or selection path.

## Requirements

- `NJB-001`: each family is one exact profile/target tuple and a standalone CLI package; no
  omnivorous or default profile exists.
- `NJB-002`: canonical one-LF JSON input/output is bounded before decode, base64 decode, fact
  retention, and output encoding. Failures emit a whole envelope only, with the minimal sentinel
  before a complete valid request and otherwise every request binding/echo but no facts. A
  request is not valid until its family, identifiers, target, features, input handles, and digests
  pass their grammar and duplicate checks (`TestInvalidEnvelopeRejectionStaysUnbound`). Every
  shown field is required and non-null; `target.features` is a required array. Logical paths use
  slash-separated ASCII segments and reject colon, backslash, aliases, and controls. Every retained
  fact field is printable ASCII of 1 through 4,096 bytes.
- `NJB-003`: facts are typed, ordered, and digest-bound by the bridge-owned 14-field evidence
  preimage whose domain tag is `corvint-analyzer-native-bridge-evidence/v0`. Parser-local spans and
  witnesses are never emitted. Facts never state compilation, semantic correctness, `PASS`,
  compatibility, authority, or admission.
- `NJB-004`: C admits a closed bounded lexical grammar for comments/strings/escapes-safe includes,
  macros, conditionals, functions, exact JNI declarations, and CMake declarations. Objective-C
  additionally admits its closed imports, declarations, and methods. Java admits its closed
  package/import/type/native-method/static-library grammar after Java Unicode escapes are translated
  before lexical classification. Unsupported, dynamic, malformed, unterminated, unknown, forged,
  or out-of-tuple input rejects the entire request.
- `NJB-005`: production candidate code performs no repository/CWD/HOME reads or writes, PATH
  lookup, process/compiler/build-tool/dynamic-loader call, network access, or ambient output other
  than the one full response written by its CLI.
- `NJB-006`: the candidate remains experimentally unselectable until independent exact review.
  Registry, lock, containment, execution, receipt qualification, semantic compilation, and
  promotion are `NOT_RUN`.
- `NJB-007`: every test-spawned helper, CLI, and builder MUST have a bounded context; normal
  completion and forced interruption MUST reap the direct child. Each regular candidate run retains
  1,000 unique complete requests and 1,000 identical fresh built-CLI replays; race retains one
  equivalent fresh case plus forced interruption because repeated race startups are stress, not
  additional race correctness coverage.
- `NJB-008`: each CLI reads standard input only when it receives no arguments. Exactly one
  `--version` argument writes the one-LF receipt `corvint-analyzer-<command>/v0` (`c-jni`, `java`,
  or `objective-c`). Any other argv, including `--version` followed by any argument or a single
  empty argument, writes the `NJB-002` minimal `NONCANONICAL_REQUEST` sentinel. Neither path reads
  stdin, and both exit 0. A nonzero exit means only that the receipt, sentinel, or response was not
  written in full; a writer that reports a byte count below zero or above the bytes offered is
  refused as a short write, never sliced or retried. All three CLIs share one guard,
  `analyzernativebridge.Run` (decision 0243).

## Pinned Beamfall corpus

All rows identify literal source bytes committed under
`internal/analyzernativebridge/testdata/beamfall-corpus/`. `gitBlob` is the original immutable
repository object identity, `sha256` is over the committed local fixture bytes, and `bytes` is the
exact byte length. The candidate never reads these paths; callers inject matching bytes.

| Repository @ revision | Path | gitBlob | sha256 | bytes |
| --- | --- | --- | --- | ---: |
| `beamfall-android-ui@6e379d7feec88128439d1753325bcfb22194fdfc` | `kit/src/main/cpp/beamfall_mpv.c` | `c92c60b1f2cb1bf098e0899f220a7f282995a9e1` | `6a6d71a89204ec3856d2f38db5efe217c28bd2fef94b5dde61ceec12b7c0a7c0` | 10922 |
| same | `kit/src/main/cpp/CMakeLists.txt` | `20276872459adc947f16f6214a67983b44559bb8` | `4de6a87f04d0bbf32c2a984c9d0437f422f6b9c9af452fd94379e03aa5e86edc` | 1719 |
| `beamfall-apple-ui@830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6` | `Tests/BeamfallTestWatchdog/BeamfallTestWatchdog.c` | `7a870b1ad818989a8255c35f8aa9c0d3afcd6cc2` | `e3a67a5684e8acf32c9c7111d4fbf1f870ad80990037deed7055276773c9bbda` | 2956 |
| same | `Tests/BeamfallTestWatchdog/BeamfallTestCaseWatchdog.m` | `5c83304d23af5220cb0b4fbaada2f9dbba630d15` | `c71025303b0bbf5832c363f29465e46e90880b2a3ce6255d79dc2353e8a0121f` | 4817 |
| same | `Tests/BeamfallTestWatchdog/include/BeamfallTestWatchdog.h` | `232f92089832582d9c3a08419e6a68f2f2919af5` | `59e69b89a05acbf296a53ac2904a3ce3658fd82020d76db96fec5c2143d3211c` | 1167 |
| same | `Sources/BeamfallAppleLab/Shaders/VisualShared.h` | `7f474f22e5a5461a35e7d4a989afcecde75eac6c` | `625e8e091d565a652ffc61ecf86b726a733b150f0bc1888e062c2001b00ae3d6` | 34543 |
| same | `Sources/BeamfallVisualKit/Shaders/VisualShared.h` | `1fead4291acfbef1062261d736d4aaf494a2c068` | `cc172efd1de4c7a3a4e0b72af89cecc728d7e24afb9af55ab75da39131692dce` | 34041 |
| `beamfall-android-ui@6e379d7feec88128439d1753325bcfb22194fdfc` | `kit-test-entitlement-stub/src/main/java/com/beamfall/kit/test/entitlement/TestModeEntitlementMint.java` | `4eaffcfe5c1cff0b619603850b9958c85ba36c96` | `9b03d4bb0ce070c9f2d96a054371c90881a20d2ff868255ce3d6de403a655ecc` | 3355 |
| `beamfall-android@52799c501cc291ff003d05f9eda391c2008cd51e` | `decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/{FfmpegAudioDecoder,FfmpegAudioRenderer,FfmpegDecoderException,FfmpegLibrary}.java` | `efd01c98f58f3933a510db4d417111ab1b5e679f`, `cbf8a5a96a34488a3b94f995103d5a273e3a0900`, `bdc0c1b3b461e2ffae7266f7167bc07e46fa71a3`, `1e887d1c1f05f755b2806619c3893f41f48657f0` | `0813cb7b59798b705762d96560865d469243a384a03f3766cc8da26abab0d035`, `aaef8a857adfcbb57973f1798f56de025f48822f8f750c16c6443f4f6c0b8b30`, `323be6dbd6917c3d1637f3d887b01cef77010f852c2a655b6828656f3ba6fe4e`, `cf0611b1bbfb6a14570229c88b5e08e4d458c2de1f4ccf7184e02f217815d7c2` | 5683, 4995, 597, 1841 |

The complete CMake receipt is
`testdata/beamfall-corpus/android-jni.CMakeLists.txt`: original repository
`beamfall-android-ui`, revision `6e379d7feec88128439d1753325bcfb22194fdfc`, blob
`20276872459adc947f16f6214a67983b44559bb8`, SHA-256
`4de6a87f04d0bbf32c2a984c9d0437f422f6b9c9af452fd94379e03aa5e86edc`, and 1,719
bytes. It is a complete stored literal, not a reconstructed build description.

## Matrix and evidence

| Requirement | Evidence | Outcome |
| --- | --- | --- |
| `NJB-001..004` | separate bridge-v0 profile; exact JSON facts/errors/digests plus all 12 literal local corpus fixtures executed through their standalone CLIs; frozen four-family profile rejection; required-array, path, fact-field, Unicode-loader, malformed-body, and forged-JNI adversarial vectors | focused test passed |
| `NJB-002` | exact minimal-sentinel bytes for null, missing, and non-array features in all three families (`TestRequiredFeaturesAndLogicalPathGrammar/NJB-002`); bounded preflight/output vectors; `Run` short/broken-pipe/zero-write and negative/over-count write vectors (`TestRunWriteRejectsShortAndBrokenPipes`); 1,000 distinct and 1,000 identical fresh built-CLI envelopes for each profile | focused test passed |
| `NJB-005` | auto-enumerated production command/package source capability/import spies with connected positive controls | focused test passed |
| `NJB-006` | no Core edits/imports and no registry/launch change | source inspection passed |
| `NJB-007` | bounded pre-review Git/script owners and marker-PID reaping on normal completion, deadline, INT/TERM, early test return, and TERM-resistant group kill (`TestNativeBridgePreReview*/NJB-007`); bounded builder/helpers, normal-completion job release, `TestFreshHelperInterruptionReapsChild`, exact Windows Job Object ABI checks, and suspended-before-assignment launch; race retains one equivalent fresh case | focused Darwin evidence and Windows cross-build only; full Windows execution remains `NOT_RUN` |
| `NJB-008` | shared-guard argv table with a stdin reader that fails the test when read (`TestRunGuardsArgvWithoutReadingStdin`); `--version extra` and `--input-file request.json` against a valid request through each built CLI return the sentinel with exit 0 (`TestCLIInvocationFramingAndRefusal` in `cmd/corvint-analyzer-c-jni`, `cmd/corvint-analyzer-java`, `cmd/corvint-analyzer-objective-c`); sole `--version` receipt in `TestCLIInvocationFramingAndRefusal` and `TestFreshProcessPermutationMatrix` | focused test passed |

The 2026-09-06 linkage pass anchors existing assertions in
`internal/analyzernativebridge/analyzer_test.go`: `TestFeatureGrammarAndOutOfTupleIdentity/NJB-001`
checks exact Java target/features refusal; `TestExactLexicalFactsAndHostileSyntax/NJB-003` checks
literal fact type/value, absent spans/witnesses and independently derived evidence digests;
`TestJavaUnicodeEscapesPrecedeLoaderClassification/NJB-004` checks Unicode escape processing before
loader classification; `TestCandidateHasNoAmbientCapabilityImports/NJB-005` checks the enumerated
production capability boundary, with `TestAmbientCapabilitySpyPositiveControl` validating the spy.
The 2026-09-06 `verificationNJBanchors1` run that was cited as anchoring these assertions with
pre/post source identity is `NOT_PRODUCED`: it lived only under the host scratch path
`/private/tmp/corvint-obligation-repairs-20260906/verification-task-evidence/`, which was never
checked in and has since been purged (verified 2026-09-12: the directory and its 38 named
per-run subdirectories, including this one, hold zero regular files). No pre/post source-identity
evidence from that run can be produced or re-checked, and none is reconstructed here. NJB-001,
NJB-003, NJB-004, and NJB-005 rest only on the Matrix and evidence table above, whose evidence is
the checked-in tests named there, not on this withdrawn anchor claim. NJB-006 remains held and
actual Windows execution remains `NOT_RUN`.

The tested failure matrix includes malformed C, Objective-C, and Java bodies; exact JNI
call/declaration separation and forged declarations; duplicate handles; continued directives;
comments/string decoys; spaced, commented, multiline, method-reference, Runtime, and
Unicode-escaped dynamic Java linkage; required non-null feature arrays; colon-bearing paths;
4,097-byte fact fields; exact feature grammar and out-of-tuple identity; and output/preflight caps.
The CMake and
C-header vectors reject `UNSUPPORTED_SCHEMA` without widening the grammar. The scanner ratchet
checks 8/16/32/64 KiB and 1 MiB inputs, a linear work count, and a capped restored
byte-zero rescan that necessarily crosses the linear work ceiling. Latency is diagnostic `NOT_RUN`.
The five added body checks initially raised the representative C path to 210 allocations/op; the
allocation-free single-pass replacement measured 196 allocations/op in five isolated samples and
restored the <=200 cap. Wall time remains diagnostic only. External review, qualification,
and full gate are explicitly `NOT_RUN`. The exact CEM/OCM capability failure is retained in
`docs/receipts/analyzer-native-bridge-cem-ocm-unsupported-v0.json`; it is an unsupported receipt,
not a closure claim.

## Rollback and promotion

Revert the candidate package, CLI, tests, and this proposed spec together. Promotion requires owner
acceptance of this separate exact profile, full literal-corpus conformance, independent review, real
cross-platform and containment evidence, and the Analyzer Capability Contract's registry/lock and
admission gates. None is supplied here.
