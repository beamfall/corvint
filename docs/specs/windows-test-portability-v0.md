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
- Exists: explicit POSIX fixture skips, Windows temporary-directory selection, bounded MCP test pipes and an isolated dependency-observer regression.
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

## Evidence and qualification

| Requirement | Implementation / executable evidence |
|---|---|
| ASD-V0-2230 | Existing skips: `internal/stepnegation/stepnegation_test.go`, `internal/toolchainreceipt/receipt_test.go`, `internal/cem/{publish,verify}`, `internal/rubytestprovider`, `internal/testrunner/{native,platform,sql}`, `benchmarks/dogfood-workers`, `cmd/corvint-companion-release`. New witnesses: `TestNodeVersionBindsRunnerExecutable`, `TestCallerOwnedServerAdmission`, `TestCallerOwnedServerUnavailableAtPublish`, `TestPWPV0028ExternalConsoleOverBoundTruncatesAndTees`, `TestPWPV0030ExternalTimeoutInterruptsRunnerFirst`, `TestNEAV0010NativeTempScope`, `TestEmitRetainsStdoutBytesPrunesAndRefusesSymlink`, `TestLMGV0007GoTestNoiseAndWindows`, `TestLMGV0008ArchiveReplacesStaleEntry`. |
| ASD-V0-2231 | `cmd/corvint-corpus-mcp/transport_bound_test.go`: `TestASDV02231CorpusTransportBound`, `TestASDV02231CorpusWindowsRefusal`; `TestCorpusMCPTransport` uses the same bounded pipe harness. |
| ASD-V0-2232 | `internal/jstestprovider/freshness_test.go`: `TestPTFV0DependencyManifestBound`, including native Windows negative observation and the unchanged supported-platform mutation controls. |

The build-log entry `2026-11-03-win-w59-windows-test-portability.md` and root
`win-w59.handback.md` retain exact commands, results, the listed-test inventory, uncertainty and
PowerShell qualification commands. Linux execution and Windows cross-vet are separate from
native Windows execution. Promotion requires the operator's exact-target Windows runs of both
packages and the listed POSIX tests with no POSIX-only failures or watchdog panic. Original-dump
attribution remains NOT_OBSERVED unless the operator supplies the retained stacks.

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
