# Shader runner pieces for V1-5113

The shader child implements RSP-V0-001 through RSP-V0-005 as declared T13
`script` commands. A compile check and a shader test command each remain one
indivisible project-declared identity. T3 owns inventory validation, exact run
sets, terminal-result reconciliation and the FULL retry. There is no additional
runner wire, command profile, tool invocation, discovery schema or record writer.

`internal/shaderrunner` reads immutable declarations through the existing script
factory. Its qualification piece is explicitly absent with reason `unqualified`.
The core record adapter supplements core trace reads with a pinned transitive
include graph and shader regions. A static graph cannot repair an absent or
incomplete trace. Executed lines remain absent (`shader-no-line-coverage`).
`internal/shaderdialect` supplies the registered region normalizer and free
function/kernel extractor for `.metal`, `.hlsl` and `.glsl`.

Literal quoted includes search the containing directory first, then the
core-supplied include directories in their declared order. Angled includes search
only those directories. There is no SDK, PATH, dirty-file or symlink fallback.
Unresolved includes, cycles, unsupported preprocessing/aggregates, malformed
lexical structure, and byte/token/body/file/depth caps yield named incomplete
components. The supported structural subset deliberately does not pretend to be
a compiler. Whole bodies include nested statements; no line coverage, nested-body
precision, or successful compilation is inferred from their extraction.

The include graph is supplementary core evidence, never a plugin assertion about
process activity. The shader adapter does not modify the T1 native/trace boundary,
T2 selector, T3 reconciliation, existing stderr, exit codes or frozen goldens.
The core's existing source-admission policy is unchanged; the shader dialect can
be used directly on captured shader bytes. The existing legacy shader-behavior
execution profile remains unchanged.

Context/query returned applicable RSP authority with omissions retained in the
external evidence directory. The start dogfood pass against the instructed
8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734 returned not-complete: binding inputs were
not supplied, and coordination-time impact was unsupported. The actual lane
integration base is b33282354e0e28705502554bfee2da359e4ae851. The final build-only
instruction forbids test execution and dogfood binding; neither is claimed here.
The full spec-requirements-check target invokes Go tests, so its static component
scripts are checked separately and its test portions remain NOT_RUN. The shared
spec status/digest/traceability rows remain unchanged as instructed; requirement
to fixture mapping belongs in the external lane handback for integration.

Retained qualification, mutation trials, live compilers and independent review
are NOT_RUN in this lane. The integration owner supplies the T6 qualification
harness and review. There is no promotion or narrowing claim. Rollback removes
these shader-owned packages and their dialect/ownership registrations; the
existing script runner and evidence record contracts require no migration.
