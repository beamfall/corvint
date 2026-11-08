# Rust runner selection pieces (V1-5113)

The accepted runner-selection contract governs this build-only slice:
`RSP-V0-001` through `RSP-V0-005`. V1-4375 is absorbed as the Rust native
evidence adapter. The lane starts at integration head
`89372490721bb960ff9261a930c94f8514a2056e`; the owner's requested dogfood/affected
comparison base is `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

## Decisions within the accepted contract

- Use an artifact-bound `NewRustPlugin` factory, following the existing script
  runner factory. Default manifest/registry absence declarations, their existing
  assertions, legacy commands and all frozen wires remain intact. Both factory
  instances offer inventory and native evidence; qualification is absent with
  reason `unqualified`. Even a caller-supplied qualification boolean cannot
  override that declaration. Core integration can consume these instances
  without changing the selector.
- The `cargo-test` instance uses the existing stable single-`--lib` profile;
  integration, binary and doctest runner profiles retain their existing absence
  declarations. Discovery uses `cargo test -- --list`, with a terminal count
  required to detect truncation. Nextest discovery uses its native JSON list.
  Both native formats lack source paths: the factory requires core-resolved,
  blob-bound test/source mappings instead of guessing from a test name.
- Use the existing native Cargo/JUnit result parsers and core reconciliation.
  Preserve native skip reasons and observed results on report failure. The SPI
  has no exit-code field: process success remains an executor/core obligation.
  A Cargo multi-test run and a nextest selection run use a declared FULL
  superset; an empty selected set is handled by core, never sent as an unfiltered
  command. Nextest configuration bytes come from the existing native profile;
  `RustRunFiles` returns them for core materialization in its private run directory.
- Convert complete, core-captured LLVM LCOV exports (including cargo-llvm-cov
  exports of `-C instrument-coverage` dumps) with the existing native line
  decoder. Require core observation of complete process instrumentation/dumps.
  Union exports and attribute them to every test of the captured process tree.
  Derive executable regions and a bounded boolean-flip candidate operator from
  the existing Rust lexer. Macros, attributes, closures, const-generic signatures,
  lexical failures and caps refuse the region/operator profile. No qualification
  harness or mutation execution engine is added.
- Model `loaded` conservatively with Cargo metadata, the complete tracked
  snapshot under each local package, target entry points, workspace inputs and
  captured build-script output. Missing script output, failed/incomplete builds,
  unmodeled native links and unknown build instructions remain incomplete.
  Build-phase traces stay core-owned and are preserved on every test, whether
  or not the same file has line coverage. Missing native components never become
  present empty observations.

## Verification and limits

The lane writes positive and negative fixture tests named after each of its five
requirement IDs. These fixtures exercise both runners through the existing SPI,
including missing-result FULL reruns, inventory divergence, head-new/stale tests,
mandates, unqualified plans, complete/malformed coverage and build graphs, and
core trace assembly/attribution. Rust lexer fixtures cover nested bodies, inert
raw strings/comments, unsupported syntax and boolean mutation candidates.

Execution and retained qualification records are **NOT_RUN**: the owner explicitly
reserved test execution, independent review and qualification for integration.
This is a build of unqualified pieces, not evidence of production narrowing.
The requirement-to-test map and actual static-check results are in the external
lane handback. Shared spec traceability rows and status are deliberately unchanged,
as requested; integration owns their update. No record store, tracer, selector,
default runner declaration, committed golden or core-freeze output is modified.

Rollback is to revert this lane's additive source and fixture files; existing
execution paths and evidence formats require no migration. Default static paths
are unchanged and no qualification record is created.
