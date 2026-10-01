# Native runner profiles (experimental)

These adapters construct native invocations and decode bounded results. They do
not admit executables, capture source/build dependency closure, or grant CEM/OCM
semantic authority. The caller must bind the executable and independently prepared
project/binary inputs, provide a fresh report directory and enforce containment.

`ctest` executes the configured build directory, not a build step. `googletest`
and `catch2` execute independently built test binaries. Their child binaries,
libraries, configuration, fixtures and environments remain caller bindings.
`dotnet-vstest-*` uses the VSTest profile with `--no-build --no-restore`; it does
not qualify Microsoft.Testing.Platform or arbitrary browser drivers. TRX results
join native test IDs to framework-specific definitions. Parameterized rows sharing
the same method identity abstain rather than silently collapse.

`cargo-test` is a single library target; `cargo-doctest` is its separate Rustdoc
suite. `cargo-integration` and `cargo-bin` require the literal `Target` name and
execute one integration or binary test harness. Stable libtest pretty output is parsed with exact terminal counts and
serial execution, with incomplete build/collection output retained as incomplete.
Custom harness output and multi-suite invocations are unsupported. Stable Cargo
JSON describes compiler artifacts, not test outcomes: Rust 1.98.1 rejects libtest
JSON without a nightly compiler. No nightly feature is enabled silently.
`nextest` writes a fixed profile-owned TOML under the fresh report directory,
disables retries for execution and includes skipped tests in JUnit. Its parser
also preserves reported native retry extensions. It does not run doctests.

Go observes `go test -json -count=1` for one package. The existing independently
qualified Go live-test provider remains a separate authority path. Package/test
identities, incomplete events and build failures remain explicit. Subtest filter
segments require a separately specified selection profile and are rejected here.

## Retained live format evidence

The testdata reports were generated on macOS arm64 on 2026-10-01 from disposable
pass/fail/skip fixtures (GoogleTest/CTest also disabled), using:

- Go 1.27.1: `go.json`.
- CTest 4.4.3: `ctest.xml`.
- GoogleTest 1.18.0: `googletest.xml`.
- Catch2 3.16.0: `catch2.xml`.
- .NET SDK 9.0.316 / Test SDK 17.14.1: NUnit 4.3.2 with adapter 5.0.0,
  MSTest 3.6.4 (`nunit.trx`, `mstest.trx`).
- SDK template xUnit 2.9.2 with its VSTest adapter (`xunit.trx`).
- Cargo/Rust 1.98.1: `cargo.txt`, `doctest.txt`.
- nextest 0.9.146: `nextest.xml`, plus an always-failing retry fixture
  `nextest-retry.xml` with two native failed attempts.

These prove concrete native formats, not complete runner qualification. Native
runtime coverage for zero tests, build/collection failures, parameterization,
flaky recovery, process timeout/interruption/descendant retirement and every
framework/platform version remains incomplete. Unit negatives retain those gaps;
they are not substituted for live lifecycle qualification. Parent coordinator
lifecycle evidence must remain separately identified.

Official contracts: GoogleTest `docs/advanced.md`; Catch2 `docs/reporters.md` and
`docs/ci-and-misc.md`; CMake `Help/manual/ctest.1.rst`; Microsoft .NET
`docs/core/tools/dotnet-test-vstest.md`; nextest machine-readable JUnit/list docs.
The source frameworks were fetched from their official tagged repositories;
Rust/nextest executable downloads were verified against official SHA256 files.

## Requirement evidence

- TRE-V0-001/003/004/005: `TestBuildExactSelectorsAndFixedProfiles` and
  `TestQualifiedIdentitySelectors` exercise fixed argv/configuration and exact
  native identity. Caller file creation/executable admission remains common code.
- TRE-V0-006/007: `TestLiveNativeReports`, `TestNextestNativeRetriesRetained`,
  `TestRejectsIncompleteAndContradictoryReports`, `TestGoStructuredStreamAndBuildFailure`
  and `TestCargoIncompleteAndCountMismatch` retain status/count/identity failures.
- TRE-V0-002/008 are coordinator-owned; this package claims no source-closure or
  process-retirement proof. TRE-V0-009 is PARTIAL per the live matrix above.
- TRE-V0-010 is NOT_PRODUCED here: runner observations alone do not establish
  assertion mappings, causal criterion kills or Tasks completion authority.

Go selectors are exact `package::TestName` (the package must equal `Project`).
Nextest selectors are exact `binary-id::test-name`; its native `binary_id` filter
was exercised with nextest 0.9.146. Cargo selectors identify a test within the
explicit single selected target. Report identity cannot imply across-target
uniqueness without the caller's project/target/executable binding.

## Independent review repair evidence

Review cycle 1 corrected the shared args-only invocation contract and declared
native failed-test exits (CTest 8, Catch2 42, Cargo 101, nextest 100; other admitted
profiles 1). `TestNativeGoBuildExecuteParse` now exercises Build → the shared
executor → Parse → Normalize with real pinned Go/source bytes and pass/fail/skip
results. This is a Go host execution witness, not qualification of other tuples.
Go uses the local toolchain with module-proxy and sum-database network access off.

The `TestReview*` regressions cover native GoogleTest result/status/skip/disabled
contradictions, TRX summary and counter conflicts, nextest crash retry causality,
Cargo announced inventory, duplicate Go structural keys and package terminals,
and profile-specific exit codes. Retry attempt messages retain nextest's native
type and text; an ordinary nonzero process exit remains UNKNOWN cause, while
signal/abort/setup/timeout evidence is INFRASTRUCTURE, never inferred ASSERTION.
The additional `nunit-pass.trx` was emitted by the same NUnit runtime using an
exact passing method filter; its native successful summary is `Completed`.

## CMocka 2.0.2 (experimental dedicated tuple)

`cmocka-xml` runs a separately built and pinned regular native executable in one
explicit primary TEST phase with no argv. It fixes `CMOCKA_MESSAGE_OUTPUT` to
`STANDARD,XML`, `CMOCKA_ERROR_OUTPUT` to `STDERR` and a fresh `cmocka.xml` path.
The existing Target field binds one group; exact native `group::test` identities
and 1..64 expected tests are mandatory. One literal selection is supported;
wildcards, multiple selections and project/config/reporter/tool overrides are not.

Both original STANDARD streams and native XML must agree. Unrecognized output
is incomplete for this bounded profile. Native XML failure causes remain UNKNOWN;
fixture errors, including group teardown returning nonzero with native exit 0,
cannot become passing observations. Shared Normalize turns incomplete public
states UNKNOWN without rewriting the original reports. No retry is scheduled or
inferred. Per-test source paths are not invented from C function names.

Nine actual macOS arm64 cases retain mixed outcomes, exact selected pass/skip,
zero tests, per-test setup/teardown, group setup/teardown and wrong-target evidence.
The native source is CMocka 2.0.2 immutable commit
`fefa2b8a023121f7235e18ed17249e4012dd144f`; all 150 official release files matched
that official commit archive. Apache-2.0 and upstream CMake redistribution notices
were retained. Publisher PGP signatures were acquired but not authenticated;
the owner accepted this limit for the bounded trusted-local experiment only.
Static-build provenance is separate from TEST receipts; SDK/loader closure and
new timeout/interruption qualification remain unobserved. See the CMocka
provenance fixture and TRE-V0-015..017. Unity and complete C-family coverage are
not delivered by this profile.
