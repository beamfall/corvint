# Native Windows Test Portability V0

Owner: Russell Lewis
Date: 2026-11-03
Intent status: accepted
Delivery status: experimental
Authoritative inputs: owner lane win-w59 request for V1-2491 and V1-2492;
`AGENTS.md`; decision 0559; `docs/SPEC-DRIVEN-DEVELOPMENT.md`.

## Agent digest
- Claim: Native Windows test fixtures distinguish POSIX assumptions from portable assertions and bound the corpus transport and dependency-observer regressions.
- Status: accepted/experimental
- Exists: explicit POSIX fixture skips, Windows temporary-directory selection, bounded MCP test pipes, an isolated dependency-observer regression, host-absolute receipt, stack-frame and preflight fixtures and exact platform-refusal skips.
- Blocked on: operator native Windows qualification and independent review; no Windows product-support claim.
- Read next: Requirements; Evidence and qualification; Boundaries and rollback.

## User job and verified baseline

The operator needs native Windows tests to skip unsupported POSIX semantics explicitly and to
finish hang regressions without waiting for the package's 30-minute watchdog. This is a testing
companion to `documentation-corpus-v1.md`, `corvint-self-development-v0.md` and
`playwright-per-test-freshness-v0.md`. Before adding requirements, the static capacity check
`script/gen-spec-requirements.sh --lines` passed on base
`5d1ed52b17d61147bd8c04b08afa6da39fa80afb`: the corpus Requirements section holds 256
obligations, self-development 244 and freshness 11. The full corpus contract therefore takes no
new obligations here. The companion uses the existing ASD-V0 test-workflow prefix, as the lane
may allocate only IDs 2230..2239 and the generator recognizes a new prefix only after a
three-digit definition. The simpler baseline is the existing explicit Windows skips and the
PTF-V0-755 platform-root termination fix already present at that base.

The original ticket bodies are retained in
`internal/tasks/similar/testdata/corpus-2026-10-30.jsonl`. They name
`TestCorpusMCPTransport/2026-07-28/DCP-V1-016_transport` and
`TestPTFV0DependencyManifestBound` as the last running tests at timeout. The original goroutine
dumps and Windows proof queue are unavailable in this lane. The attribution below is code
inference, not an observed stack trace: a loop ending only at `/` repeats at a Windows volume
root; a synchronous `io.Pipe.Write` or decoder read does not return just because a context is
cancelled. Server-exit pipe cleanup alone does not cover a server that remains running but stalls.

## Requirements

- `ASD-V0-2230`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2491) Tests
  listed by V1-2491 MUST explicitly skip POSIX-only mode, sticky-bit, shell and path expectations
  on `runtime.GOOS == "windows"`, with a specific reason, or check a Windows equivalent. The
  additional JS fake-node, fake-npx and caller-owned-runner shell fixtures follow the same rule.
  The native temp-scope test sets Windows TMP/TEMP and compares JSON-quoted native output paths.
  Retained JS receipt and model-gateway archive tests keep portable byte, path and pruning checks
  active while only their POSIX mode assertions skip. Mode skips do not establish ACL privacy.
- `ASD-V0-2231`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2492) The
  interactive corpus MCP transport test MUST release client writes and reads on server exit and
  on its own 30-second context deadline, with a separate five-second cleanup bound. Regressions
  MUST exercise startup refusal, a blocked write and a blocked response read. Native Windows
  MUST assert the existing corpus-reader platform refusal; positive fixture tests skip with the
  qualified-reader reason. The production platform boundary and MCP wire stay unchanged.
- `ASD-V0-2232`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2492) The
  synchronous dependency-manifest bound regression MUST run its actual observation in an isolated
  test child, retaining a 15-second child watchdog, 20-second parent context and one-second
  WaitDelay. Failure includes the child diagnostic output. On Windows it MUST return an
  unsupported dependency observation and never match a complete manifest. Existing volume-root
  termination, supported-host drift checks and resource limits remain in force.
- `ASD-V0-2233`: (accepted under the owner's standing auto-accept (2026-10-11); win-w59 W5)
  JS receipt fixtures whose paths production checks with `filepath.IsAbs` MUST use host-absolute
  paths: the POSIX path unchanged off Windows and the same path on the `C:` volume there. The
  bundled headless-shell path keeps its forward-slash qualified suffix. Expected generated config
  and reporter JSON MUST quote native paths as JSON does. A test that edits a qualified
  keep-reporters record MUST first fail on an unqualified base record instead of panicking.
  A Windows host that refuses symlink creation runs the other non-regular-file refusals.
  Production path rules and all POSIX fixture bytes stay unchanged.
- `ASD-V0-2234`: (accepted under the owner's standing auto-accept (2026-10-11); win-w59 W5)
  A JS provider test that needs a subprocess through `procgroup.Run`, such as Git observation, an
  attestation provider or a preflight command probe, MUST skip on native Windows only when
  `procgroup.Run` returns its listed `process-platform-unsupported` refusal (decision 0559).
  Checks that run before the subprocess stay active. Off Windows the helper runs nothing.
- `ASD-V0-2235`: (accepted under the owner's standing auto-accept (2026-10-11); win-w59 W5)
  On native Windows a complete freshness fixture MUST fail closed. The fixture MUST NOT project
  `CURRENT`, and its POSIX dependency manifest MUST NOT be complete. Supported hosts keep the exact
  `CURRENT` assertions. The codec test's contradictory-projection case skips on Windows, because
  no `CURRENT` projection exists there to contradict.
- `ASD-V0-2236`: (accepted under the owner's standing auto-accept (2026-10-11); win-w59 W5)
  The corpus MCP batch-frame test MUST still build its corpus on native Windows. It skips only when
  `doccorpus.ReadCorpusFile` returns the listed docs-corpus refusal of decision 0559. That refusal
  is probed before `corpusbridge.New`, because the bridge folds it into `corpus-unavailable`.
- `ASD-V0-2237`: (accepted under the owner's standing auto-accept (2026-10-11); win-w59 W5)
  The route-stubs producer unit MUST build its ESM stack-frame fixture from the host-absolute
  path with `pathToFileURL`: the drive-less `file:///tmp/spec.mjs` off Windows, unchanged, and a
  drive-letter file URL on Windows, where a drive-less file URL names no path. Drive-letter V8
  frames, both `C:\...` and `file:///C:/...`, MUST resolve to their file and line on native Windows.
  Elsewhere the backslash `C:\...` frame MUST name no call site, because it is not host-absolute.
  The producer's frame parsing stays unchanged.
- `ASD-V0-2238`: (accepted under the owner's standing auto-accept (2026-10-11); win-w59 W5)
  The preflight declaration test's absolute storage-state case MUST use a host-absolute path
  (`hostAbs`): `/etc/state.json` off Windows, unchanged, and `C:\etc\state.json` on Windows, where
  the rooted POSIX form is not `filepath.IsAbs`. It MUST still refuse
  `preflight-declaration-invalid` on every host. Production preflight validation stays unchanged.

## Evidence and qualification

| Requirement | Implementation / executable evidence |
|---|---|
| ASD-V0-2230 | Existing skips: `internal/stepnegation/stepnegation_test.go`, `internal/toolchainreceipt/receipt_test.go`, `internal/cem/{publish,verify}`, `internal/rubytestprovider`, `internal/testrunner/{native,platform,sql}`, `benchmarks/dogfood-workers`, `cmd/corvint-companion-release`. New witnesses: `TestNodeVersionBindsRunnerExecutable`, `TestCallerOwnedServerAdmission`, `TestCallerOwnedServerUnavailableAtPublish`, `TestPWPV0028ExternalConsoleOverBoundTruncatesAndTees`, `TestPWPV0030ExternalTimeoutInterruptsRunnerFirst`, `TestNEAV0010NativeTempScope`, `TestEmitRetainsStdoutBytesPrunesAndRefusesSymlink`, `TestLMGV0007GoTestNoiseAndWindows`, `TestLMGV0008ArchiveReplacesStaleEntry`. |
| ASD-V0-2231 | `cmd/corvint-corpus-mcp/transport_bound_test.go`: `TestASDV02231CorpusTransportBound`, `TestASDV02231CorpusWindowsRefusal`; `TestCorpusMCPTransport` uses the same bounded pipe harness. |
| ASD-V0-2232 | `internal/jstestprovider/freshness_test.go`: `TestPTFV0DependencyManifestBound`, including native Windows negative observation and the unchanged supported-platform mutation controls. |
| ASD-V0-2233 | `internal/jstestprovider/host_paths_test.go` (`hostAbs`, `hostBundledUse`) used by `qualifiedFixture`, the bundled tuple tests in `external_test.go`, `keep_reporters_qualification_test.go`, `qualified_screen_test.go`, `TestExternalCommandDefaultConfigUnchanged` and `TestKeptReporterListResolution`; the base-record guard in `TestKeepReportersQualificationRecordClosed`; the symlink case of `TestPWPV0051FreshCommandExecutableStreamDigest`. |
| ASD-V0-2234 | `skipWindowsProcessRefusal` in `TestApplicationAttestationCommandProvider` (after preparation), `TestObserveTestRepositoryIgnoresAmbientGitRedirects`, `TestObserveTestRepositoryAtStart`, `TestObserveTestRepositoryIgnoresGrafts_EEPV0771`, and the `passing`, `http-declared-status` and `command-*` subtests of `TestPWPV0091PreflightProbesRunInOrderAndStopAtFirstFailure`; refusal covered by `internal/procgroup` `TestRunPlatformRefusalIsTheSentinel`. |
| ASD-V0-2235 | `requireFreshCurrent` and `TestASDV02235FreshnessFailsClosedOnWindows`; used by `TestPTFV0ObservedCurrency`, `TestV10689BoundedWitnessFreshnessCodec`, `TestPWPV0039ReceiptsAndFreshnessAdmitFromOneTable`, `TestPWPV0050OneTupleAdmitsExternalAndFreshness`; `TestPTFV0ClosedCodec/projection` Windows skip. |
| ASD-V0-2236 | `cmd/corvint-corpus-mcp/main_test.go`: `TestDCPV1145BatchFitsOneFrame` reader probe with `testplatform.SkipWindowsRefusal`; refusal covered by `TestASDV02231CorpusWindowsRefusal`. |
| ASD-V0-2237 | `internal/jstestprovider/route-stubs_test.cjs`: `long patterns are clipped inside the bound and stack frames resolve` and `drive-letter stack frames resolve only on Windows`, run by `TestLPCVV0072ProducerNodeUnit`. |
| ASD-V0-2238 | `internal/jstestprovider/preflight_test.go`: `TestPWPV0090PreflightDeclarationValidation/absolute-state` with `hostAbs`. |

The build-log entry `2026-11-03-win-w59-windows-test-portability.md` and root
`win-w59.handback.md` retain exact commands, results, the listed-test inventory, uncertainty and
PowerShell qualification commands. Linux execution and Windows cross-vet are separate from
native Windows execution. Promotion requires the operator's exact-target Windows runs of both
packages and the listed POSIX tests with no POSIX-only failures or watchdog panic. Original-dump
attribution remains NOT_OBSERVED unless the operator supplies the retained stacks. The first
native W5 run failed. Its keep-reporters panic aborted the rest of the jstestprovider package, so
tests in files after `keep_reporters_qualification_test.go` remain NOT_OBSERVED on Windows until
a W5 rerun. ASD-V0-2233..2236 are inferred from code and that run's output, not from a passing
native run. The W5 rerun at `606e61fb8` failed only `TestPWPV0090PreflightDeclarationValidation`
and `TestLPCVV0072ProducerNodeUnit`. ASD-V0-2237..2238 are inferred from that output and the Node
win32 URL rules. Their Windows branches remain NOT_OBSERVED until a native run.

## Boundaries and rollback

Non-goals: Windows process containment, ACL qualification, enabling Windows product support,
changing dependency evidence semantics, adding a daemon or a transport-wide idle timeout.
Only tests and their contract change. The dependency child uses the current compiled test binary,
an exact test-name selection, and a child-only environment marker; it invokes no shell. A stuck
observer fails the child and parent bounds rather than certifying missing evidence.

Rollback removes these test harness changes and this companion's generated rows; the previously
accepted production refusal and platform-root termination fix stay intact. Windows qualification
is unavailable to the lane and is not inferred from compilation. A failed native run retains its
output and blocks promotion; no requirement here changes ticket completion authority.
