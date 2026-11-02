# win-w59: bounded Windows portability regressions

Base: `5d1ed52b17d61147bd8c04b08afa6da39fa80afb` (`origin/main`).
Tickets: V1-2491 and V1-2492. Requirements: ASD-V0-2230, ASD-V0-2231, ASD-V0-2232 in
`docs/specs/windows-test-portability-v0.md`, a testing companion under the existing ASD prefix.
The owner prohibited commits, pushes, Tasks commands and Codex; this is an uncommitted builder
delivery, not ticket completion or Windows platform qualification.

## Decision and evidence basis

The base already includes commit `0f2f6bbdb`'s explicit Windows skips for the reported POSIX mode,
TMPDIR, shell and native-runner path tests, plus earlier benchmark and companion-release guards.
It also includes `949a3a7fb`'s PTF-V0-755 dependency-walk termination at a self-parent directory and
MCP pipe cleanup when the server exits. Reuse those fixes rather than broadening Windows support.

The ticket bodies in `internal/tasks/similar/testdata/corpus-2026-10-30.jsonl` identify the last
tests before timeout, but contain no goroutine dump. Searches of build-log, decisions and local
proof/queue filenames found no original Windows stack. The retained Windows runbook locates the
operator's logs under `C:\cvq\proof\2026-10-24\V1-2441\main\`; this lane cannot reach them.
Identification is therefore **code inference**: the former dependency ancestor loop repeats at
`C:\`, while synchronous MCP pipe writes and response reads remain blocked after context
cancellation unless something closes the pipes. The original timeout stacks remain NOT_OBSERVED.

The MCP test now shares a pipe harness whose deadline closes both server pipe ends. The normal
transport allowance is 30 seconds, with a separate five-second cleanup bound. Actual startup
refusal, a live server that does not read, and a live server that never responds are exercised.
Positive corpus fixtures skip on Windows because production corpus reads are intentionally
unsupported under decision 0559; a Windows-only negative test asserts the typed reader refusal.
Production transport and platform support are unchanged.

The dependency-manifest regression runs its synchronous observer in the compiled test binary as
a child: 15-second test watchdog, 20-second parent context, one-second WaitDelay and retained
failure output. This can terminate a reintroduced volume-root loop; a context passed only around
the synchronous call could not. The existing supported-platform observation/drift controls and
unsupported-platform negative observation are preserved.

A broad test-source grep found 304 candidate files, including platform-tagged and explicitly
unsupported subsystems. Additional task-related defects were unguarded shell fakes in JS node
identity, caller-owned runner and external-runner fixtures; TMPDIR-only temp-scope selection and
unquoted native-path comparison; and unconditional POSIX private-mode assertions in the JS CLI
and model-gateway archive tests. Shell fixtures now skip specifically on Windows. The temp test
uses TMP/TEMP on Windows and JSON-quotes its native path. Byte, location and pruning assertions
remain active while mode-only subtests skip; no ACL privacy is inferred.

## Verification and limits

The exact commands, listed-test inventory and per-criterion results are retained in
`win-w59.handback.md`. Focused MCP transport tests passed with the race detector; non-network JS
dependency/identity/projection tests and JS CLI/model-gateway retention tests passed. The listed
POSIX cases passed in nine packages; the companion-release fixture needed the disposable-clone
route because its gate correctly rejects output inside its source checkout; that rerun passed
(0.036s). Additional Ruby
shell-fixture and basic publication cases passed.

Both targeted `GOOS=windows go vet` and `script/heavy.sh make windows-vet` passed, including the
interop module. Gofmt, normal-staged spec generation/consistency, staged line-citation and flow-guide checks
passed. Spec generation reported no changes in the verification clone. The static spec
capacity check ran before adding requirements: corpus 256 obligations, self-development 244,
freshness 11; the companion has three. All added IDs are within 2230..2239. The committed-ref and
worktree scan found no other definition across 880 refs and 168 worktrees.

Broader JS tests hit this sandbox's denied TCP/Unix socket creation; those executions are retained
as failed environment-limited evidence, not renamed passing runs. Native Windows execution is
NOT_RUN and belongs to the operator's PowerShell qualification. No repository-wide gate ran.

The shared Git directory is read-only, blocking normal staging and the start-of-change dogfood
recorder. A private snapshot index initially allowed staging, but inherited Git environment
overrides interfere with fixture repositories. That route is not a final gate witness. Final
staged checks use a disposable, writable shared-object clone with normal staging and byte-identical
task files. The original worktree remains uncommitted and unstaged in its real index.
Post-commit CEM/OCM binding, reviewer-report inspection and seal are NOT_PRODUCED because commits
are explicitly prohibited. Independent review and native ticket completion remain operator steps.
Corvint affected/context/query receipts retain their actual uncertainty; later successful context
reads do not replace the missing pre-change binding.

## Rollback

Revert the task's test changes and the companion/index rows. Keep the pre-existing production
platform refusals, Windows skips and PTF-V0-755 root-termination fix. No production wire, process
containment, release qualification or native ticket state changed.
