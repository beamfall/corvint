# 2026-10-08: MTP native socket path admission (V1-0601)

## Intent

Ticket V1-0601 recorded that the experimental MTP profiles failed on macOS whenever
the report directory was long. Microsoft.Testing.Platform 2.4.1 creates
Unix-domain-socket pipes under `TMPDIR`, which the executor sets to
`<reportDir>/.tmp`. Past the 103-byte native limit, the host throws and exits 134
before writing a report. Acceptance asks for:

- the exact native reproduction and the expected admission behaviour;
- qualification of long and short paths;
- bounded profile admission or a task-owned lifecycle remedy, with no false pass
  and no leaked process.

This change adds proposed `TRE-V0-027` to `docs/specs/test-runner-execution-v0.md`.

## Decisions

- **Bounded admission, not a pipe-directory override.** `buildMTP` refuses before
  launch when `len(<reportDir>/.tmp) + 1 + 46 > 103`, and the error names both
  lengths. Setting `TESTINGPLATFORM_PIPE_DIRECTORY` to a short task-owned directory
  would avoid the limit, but it would change every MTP invocation identity and add
  a shared directory outside the report root. Owning that directory's lifecycle
  would then become a new responsibility.
- **Longest pipe name.** The MTP 2.4.1 binaries contain the prefixes
  `CONTROLTOHOST_` and `MONITORTOHOST_`, plus the unprefixed 32-hex pipe. The
  longest is 14 + 32 = 46 bytes, which matches the measured boundary below.
- **One bound everywhere.** 103 is the macOS `sun_path` limit that MTP enforces.
  Linux keeps the same bound, so admission is never looser than on the qualified
  host.
- **Shared temp path.** `testrunner.ExecutionTempDir` now defines the executor
  `TMPDIR`, so the profile and the executor cannot drift apart. The value is
  unchanged.
- **Live test.** `TestMTPLiveExecution` takes `CORVINT_MTP_LIVE_TFM` and
  `CORVINT_MTP_LIVE_RUN_DIR`. The platform default temporary directory on macOS
  is itself past the bound. A `long-report-dir` case checks both behaviours:
  admission refuses the long directory, and a forced run of the same invocation
  past admission yields no complete observation.

## Evidence

- Original reproduction, retained outside the repository:
  `/private/tmp/cem10-build/mtp/executor-evidence-1428139223/*/.phase-00-stderr`.
  For example, the xunit-skip pipe path
  `.../xunit-skip/.tmp/08a8f5f4b91e4fd7898bdfd84ec3389c` is 106 bytes, over the
  maximum of 103. The .NET 10 host and binaries from that run have been pruned.
- Native boundary probe: the net9.0 MSTest probe was built from the repository
  sources (MSTest.Sdk 4.4.1, MTP 2.4.1, SDK 9.0.316, runtime 9.0.18) and run
  directly with a closed environment.
  - `TMPDIR` of 47 to 56 bytes runs and writes TRX.
  - 57 bytes exits 134 with `MONITORTOHOST_<32hex>`, 104 bytes, and writes no
    report.
  - Earlier probes at 70, 71 and 73 bytes also exit 134 without a report.
- Failing before / passing after: `TestMTPSocketPathAdmission` fails when the bound
  is disabled. It passes with the bound: 51-byte directory admitted, 52 refused,
  and the reproduced 68-byte directory refused for all three frameworks.
- Live qualification: net9.0 NUnit 5.0.0/NUnit3TestAdapter 6.3.0, MSTest.Sdk 4.4.1
  and xunit.v3.mtp-v2 4.0.1 probes were built with TrxReport 2.4.1, restored from
  nuget.org into a task-local package folder. `TestMTPLiveExecution` passed all
  18 subtests.
  - pass/fail/skip/infra are complete and zero is incomplete, at 49 to 51 bytes.
    `mstest-infra` sits exactly on the 51-byte boundary.
  - Each `long-report-dir` was refused by `Build`. The forced execution aborted
    natively with a 152 to 153 byte pipe path, and the executor returned an error
    for the missing report.
  - No probe or compiler server processes remained afterwards. Evidence is retained
    privately at
    `/private/tmp/claude-501/dotnet-td/evidence/mtp-live-qualification.json`.

## Non-goals

No change to the executor environment, the VSTest profile, report parsing or
invocation identity. No new MTP version, target framework or extension is
qualified.

## Failure modes

A report directory longer than 51 bytes refuses the MTP profile, so no result is
reported instead of a false failure or pass. A future MTP version with a longer
pipe name could still abort natively. That abort is a missing report: an executor
error, never a complete observation.

## NOT_RUN

- Linux execution, where the native limit is larger and the bound is conservative.
- .NET 10 / net10.0 fixture rebuild: no .NET 10 SDK locally.
- MTP versions other than 2.4.1.

## Rollback

Remove the admission check, `TestMTPSocketPathAdmission`, the live-test additions
and the `TRE-V0-027` section. `ExecutionTempDir` can stay, because it preserves
the executor value byte for byte.
