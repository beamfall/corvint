# Native line coverage (experimental)

`Collect` drives the existing native runner invocations and parsers, runs the
ordinary suite, then isolates each passing native identity. The result uses
`linecov.Outcome` and `linecov.Record`; `Build` binds source paths to Git blobs
with the existing index encoding. All packages stay unordered, so these
unqualified families keep static selection. `Analyze` is a fixture/candidate
helper requiring a source provider's executable bodies bound to the base blob.

The caller supplies one admitted `testrunner.Request`, exact runner/build
version identity, a private scratch parent and explicit native source mappings.
`Options.Run` is the native execution/export boundary: execute the supplied
request and invocation, retain its native `testrunner.Input`, retire the entire
process tree, and export native coverage for each instrumented process. Existing
`testrunner.Execute` can execute admitted invocations; its receipt alone does not
prove coverage completeness. Do not set `TreeComplete` from directory contents.
A missing launch observer/exporter is an incomplete run, never a complete record.

Native exports supported:

- JVM: JaCoCo XML after each isolated VM's dump, with a fresh process resetting
  counters and `JAVA_TOOL_OPTIONS` propagating the agent. The exporter must join
  and verify all VM dumps, including subprocesses, before returning.
- .NET: coverlet VSTest collection or Microsoft MTP coverage exported as
  Cobertura. Supply the project's already installed coverage extension; no
  automatic restore or tool installation occurs. An unobserved subprocess
  cannot be considered covered by the parent report.
- C/C++: build binaries independently with clang profile instrumentation or
  gcov; export LCOV for every loaded repository binary.
- Rust: `-C instrument-coverage`, LLVM profile exports as LCOV.
- Swift: `--enable-code-coverage`, LLVM profile exports as LCOV.

LLVM capture uses `%p-%m` to keep process/module profiles separate. Merge with
the matching `llvm-profdata`, export each binary's mapping with `llvm-cov export
-format=lcov`, and attach it to its observed process. Report conversion never
reads or executes paths named inside native exports. Missing source mappings,
zero processes, missing dumps, cycles, unfinished processes and malformed native
reports refuse records. Use `Selectors` when reported native IDs differ from
native filter names; MTP method/GUID and Swift source suffixes are resolved by
default. TestNG gets an isolated suite XML preserving its suite/group identity.

Fixtures do not require native toolchains. They exercise native report parsing,
exact isolation, subprocess-only coverage, instrumented zero-hit lines, static
declaration fallback, and refusal paths. They are not the pinned-repository
mutation qualification required by LPCV-V0-963.
