# Native Microsoft.Testing.Platform evidence

These are captured reports from three **experimental, separately named** profiles:
`dotnet-mtp-nunit`, `dotnet-mtp-mstest`, and `dotnet-mtp-xunit`. They do not reuse
VSTest adapter identifiers or its skipped-counter exception.

The disposable fixtures were built with the task-local Microsoft .NET SDK
10.0.401 (runtime 10.0.12), NUnit 5.0.0 / NUnit3TestAdapter 6.3.0,
MSTest.Sdk 4.4.1, xunit.v3.mtp-v2 4.0.1, and
Microsoft.Testing.Extensions.TrxReport 2.4.1 on macOS arm64. `provenance.json`
retains hashes of the native reports and streams. Sources and exact project
package references accompany each framework. `mixed2` was captured before the
separate setup-failure fixture was added; its three-test inventory is intentional.

The SDK archive's published SHA512 was verified before extraction. The archive,
Microsoft release metadata, SDK licenses, build logs, and live shared-executor
receipts remain in `/private/tmp/cem10-build/mtp` for this build's qualification.
No machine SDK installation was changed. Native MTP uses local named pipes;
the initial sandbox-denied attempts are retained there and were rerun with IPC
permission.

The profile takes a root-relative prebuilt DLL as `project`; that DLL and its
same-stem `.runtimeconfig.json` and `.deps.json` must be declared hashed inputs.
It executes the independently pinned `dotnet` host directly with native MTP TRX
flags. It does not run `dotnet test`, restore, build, or evaluate a project.
Selectors are literal `FullyQualifiedName` method names; selecting a parameterized
method can select multiple cases. Observed identities append `::` and the native
TRX test GUID to the fully qualified method name, preserving distinct data rows.
Expected identities refer to those observed IDs, not filter expressions.

Native pass and skip exit 0; failed tests and setup exceptions exit 2; empty
selection exits 8 and remains incomplete. TRX setup exceptions and assertions
share the same outcome, so failed rows retain their native message and `UNKNOWN`
failure cause. Retry information remains `NOT_REPORTED`. The parser requires the
qualified adapter identity, complete counters, and matching result/definition/
entry execution identities; unknown adapters are refused. The shared TRX XML
guard rejects unknown children/attributes and namespace aliases. Passed rows with
ErrorInfo and summary-level ErrorInfo make the observation incomplete; native
NotExecuted ErrorInfo remains a retained skip reason. This is a bounded
version profile, not a compatibility claim for every MTP release.

`TestMTPLiveExecution` is opt-in via `CORVINT_MTP_LIVE_ROOT`; it downloads nothing
and consumes prebuilt applications plus the SDK in that directory. It runs pass,
fail, skip, zero-selection, and setup-failure cases through Build/Execute/Parse/
Normalize and retains JSON receipts. Fresh report paths are created beneath the process temporary directory. On macOS,
use a short `TMPDIR` such as `/private/tmp`: native MTP named-pipe paths must fit
the operating system's 103-byte socket limit. Longer paths fail before report
creation; those failed qualification attempts remain retained.

Full runtime/dependency closure, discovery-specific exceptions, retries,
parameterized-case live qualification, other operating systems, and causal
criterion adequacy are **NOT_OBSERVED**. Fixture success does not promote those
claims or authorize executing an untrusted test application.

Official references:

- [MTP with dotnet test](https://learn.microsoft.com/en-us/dotnet/core/tools/dotnet-test-mtp)
- [MTP exit codes](https://learn.microsoft.com/en-us/dotnet/core/testing/microsoft-testing-platform-troubleshooting)
- [NUnit MTP setup](https://docs.nunit.org/articles/vs-test-adapter/NUnit-And-Microsoft-Test-Platform.html)
- [xUnit MTP setup](https://xunit.net/docs/getting-started/v3/microsoft-testing-platform)
