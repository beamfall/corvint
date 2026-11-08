# .NET runner selection pieces (V1-5113, absorbs V1-4370)

Implements fixture-driven compiled-in attachments for the existing six MTP/VSTest
runner declarations, under decision 0592 and RSP-V0-001..005. No existing wire,
legacy execution path, error, golden or core-freeze output changes. The RSP spec's
status and shared traceability rows remain unchanged as required by the lane charter;
the lane handback supplies the integration requirement-to-test mapping.

`registry.NewDotnetPlugin` takes immutable core-captured inputs and implements the
existing runner SPI. The unconfigured registry route continues to declare absence,
as does the existing script factory pattern. Inventory binds one project and TFM
into both the configuration digest and test identities, retaining target OS/arch.
VSTest's English list uses captured PDB/adapter source bindings. MTP consumes the
native schema-1 JSON list with UID and location. Unknown or mismatched platform,
failed/capped capture, malformed list, duplicate identity and unbound source refuse
discovery. No project evaluation, restore, test execution or filesystem/store write
is performed by a plugin method.

Commands distinguish VSTest, native MTP, and legacy MTP MSBuild integration. They
request discovery per project and framework. Run and collection commands intentionally
omit filters and report a FULL superset when given a subset: parameterized display
names and framework-specific filter semantics have no qualification here. Existing
strict native TRX parsers retain results; core inventory reconciliation owns missing
results, retry and sticky divergence. An explicit core-captured TRX-to-list identity
bridge is required when the native identities differ; the plugin cannot guess it.

Cobertura exports, including coverlet's Cobertura output, retain executed and
instrumented-but-unexecuted lines. Unknown XML structure/attributes, malformed
exports, unmapped sources and incomplete instrumentation/export cannot yield a
present complete component. Shared-tree native coverage is union-attributed to every
test of that tree. Core trace components join only through `testevidence.Assemble`.
Sources are checked against their Git blob digests. Loaded closure uses evaluated
compile items/project references and checks each framework's restore assets against
those references; absent assets, cycles and unresolved closure are incomplete.

C# regions reuse the existing dialect's declaration and comment/literal scanner.
Only exact whole-line interiors of flat block methods are admitted; signatures and
brace lines stay outside. Unsupported syntax, preprocessing and capped/unfinished
walks are incomplete. No approximate symbol extent is called an executable body.
The scalar-return mutation operator offers deterministic body-edit candidates to
the future harness; it refuses literals/comments and unsupported regions. It never
counts trials, generates qualification, or runs a mutation engine.

All classes remain FULL with qualification reason `unqualified`; unknown platforms
instead name `dotnet-platform-undetermined` or `dotnet-platform-runner-mismatch`.
Retained qualification, native runtime compatibility, mutation trials, replay, and
tracing parity are NOT_RUN pending the parallel T6 harness and integration tests.
This is a bounded native artifact profile, not broad runtime qualification.

Official command references checked during implementation:

- https://learn.microsoft.com/en-us/dotnet/core/tools/dotnet-test-vstest
- https://learn.microsoft.com/en-us/dotnet/core/tools/dotnet-test-mtp
- https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-with-dotnet-test
- https://github.com/microsoft/testfx/pull/8280 (native MTP schema-1 discovery)

Corvint context was used at initial HEAD b33282354e0e28705502554bfee2da359e4ae851;
it reported omitted spec-cited sources and no Tasks ticket resolution. The context
receipt and final affected plan are retained outside tracked source. The
requested initial dogfood-change against 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734
ran but failed for missing citation/intent/outcome inputs; impact was NOT_PRODUCED
(`unsupported-impact-range`). The temporary tracked CEM change was restored.
The later, explicit build-only charter defers final binding/checking/sealing, all
test execution and independent review to integration. No Tasks writes or publication.

Rollback: remove this factory and its .NET-specific helpers/tests. The original
registry declarations and legacy execution behavior remain intact, with no state
migration or promoted qualification to revoke.

Static validation: Go 1.27.3; `go vet` on `internal/contextindex` and
`internal/testrunner/registry` each exited 0, including test type-checking.
`gofmt -l` emitted nothing. Citation repinning found no moved citation;
`make -s line-citations-check`, the spec README check, requirement generator
`--check`, and spec worktree consistency check each exited 0. The full
`spec-requirements-check` target is NOT_RUN because its recipe also executes
Go tests, which the build-only charter prohibits. No assertion was executed.

The final affected receipt exited 0 but retained UNKNOWN scope, 535 selected
units and 98 unknowns (77 LANGUAGE_FRONTIER, 1 NO_SELECTABLE_TEST,
20 UNOWNED_DIRTY_PATH). It includes inherited integration changes because the
requested affected base differs from this lane's starting HEAD. Both touched
packages are direct selections. The installed binary also reported
`self-binary-stale`; no complete impact or test-coverage claim follows.
Affected receipt SHA-256: `0cddaf13ea0f0c1aa15399ceb39e5b62534ce43641f0817a15d51826578f58f0`.
