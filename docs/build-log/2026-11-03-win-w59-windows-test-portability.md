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

## W5 native failure repair (ASD-V0-2233..2236)

The operator's native run (go1.27.3, Windows 11, `C:\cv`) passed W1-W4. W5, the package-wide run of
the touched packages, failed in `cmd/corvint-corpus-mcp` and `internal/jstestprovider`. In the
second package, a panic in `TestKeepReportersQualificationRecordClosed` aborted every later test
file. Native Windows execution is unavailable to this lane: WSL interop is refused. Each cause
below is therefore inferred from code and the retained W5 output.

- Host path rules (ASD-V0-2233). Production checks receipt-reported paths with `filepath.IsAbs`:
  the config file, keep-reporters config inputs and the bundled headless-shell path. The fixtures
  used POSIX `/repo/...` and `/portable/...` literals. Those are not absolute on Windows, so
  qualified projections abstained and keep-reporters runs read as incomplete. That explains
  `TestQualifiedReceiptProjection`, the Playwright 1.63/1.64 tuple tests,
  `TestProfileAttemptEmptyProject`, `TestQualifiedReceiptBinding...` and the
  `TestQualifyKeepReporters*` failures. A not-run record has no entries, so
  `q.Entries[1]` panicked. `browserless_test.go` already used a real temp-dir config and passed
  natively, which supports this attribution. The fixtures now use `hostAbs`/`hostBundledUse`,
  which leave the POSIX bytes unchanged off Windows. The record test fails on an unqualified base
  record before mutating it. Production path rules are unchanged: a real Windows receipt carries
  native paths, and decision 0559 refuses Windows receipt reads.
- procgroup refusal (ASD-V0-2234). The attestation provider's `observe` and the Git test-repository
  observer run through `procgroup.Run`, which returns the listed `process-platform-unsupported`
  refusal on Windows. Those tests now skip only on that exact refusal, after the checks that do
  not need a subprocess. The grafts test and the preflight command-probe test have the same latent
  cause behind the panic and got the same treatment.
- Freshness on Windows (ASD-V0-2235). `freshManifestComplete` requires files below
  `root+"/"`, and a Windows path that `filepath.Clean` leaves unchanged has no `/` separator.
  No manifest can be complete there, so the complete fixture cannot be `CURRENT`. Dependency
  observation also refuses (ASD-V0-2232). On Windows the `CURRENT` assertions now require the
  projection to fail closed, and a Windows-only test pins that. The codec's
  contradictory-projection case skips there because there is no `CURRENT` to contradict.
- Corpus bridge (ASD-V0-2236). `TestDCPV1145BatchFitsOneFrame` builds its own corpus.
  `corpusbridge.New` folds the decision 0559 reader refusal into `corpus-unavailable`. The test
  now probes `doccorpus.ReadCorpusFile` and skips only on the listed refusal.

An independent subagent review found no blocking defect. It found that the preflight skip was
broader than the procgroup boundary; it now covers only the subtests that reach a command. It also
found two reporter tests behind the panic that compared raw native paths in JSON or the generated
config; they now JSON-quote host-absolute paths. A Windows host that refuses symlink creation now
logs only that case as NOT_RUN.

Findings for the orchestrator are not fixed here, because production is out of scope. The corpus
bridge, and so the MCP server, reports a native Windows read refusal as
`corpus-unavailable`/"repository unavailable" rather than `corpus-platform-unsupported`. This
may fall under rule 2 of decision 0559. Freshness and qualified-receipt projections abstain on
Windows without a typed platform reason. That is fail-closed, and receipt reads are refused
upstream. Linux full-package runs of all four touched packages passed, as did targeted
`GOOS=windows go vet` and `make windows-vet`. Native W5 remains to be rerun. Tests behind the former
panic stay NOT_OBSERVED, including possible `os.Symlink` privilege failures. Full commands and
results: `win-w59.fix-handback.md`.

## W5 rerun repair (ASD-V0-2237..2238)

The native W5 rerun at `606e61fb8` failed two tests. As before, each cause below is inferred from
code and the retained output; it was not run on native Windows.

- Route-stubs frame fixture (ASD-V0-2237). The unit built its ESM frame as
  `file:///tmp/spec.mjs`. On win32, `fileURLToPath` throws `ERR_INVALID_FILE_URL_PATH` for a
  drive-less file URL, so `frameLocation` returned `null`. This was confirmed with Node's
  `{ windows: true }` URL mode. That URL is not a frame V8 emits on Windows. Real Windows frames
  (`C:\...:L:C` and `file:///C:/...:L:C`) parse, because the greedy `(.*):(\d+):\d+$` keeps the
  drive colon and win32 `path.isAbsolute` admits the result. This is a fixture defect, not a
  producer bug. The fixture now uses `pathToFileURL(path.resolve(...))`, which gives the same URL
  bytes off Windows. A new case pins the drive-letter frames: both shapes resolve on Windows, and
  the backslash frame names no call site elsewhere. (Off Windows, `file:///C:/...` decodes to the
  POSIX path `/C:/...`, which V8 never emits there.)
- Preflight absolute state file (ASD-V0-2238). On Windows, `/etc/state.json` is rooted but not
  `filepath.IsAbs`, so the declaration passed validation. The case now uses
  `hostAbs("/etc/state.json")`, unchanged off Windows.

Finding for the orchestrator (production; not fixed here): on Windows, the storage-state check
(`preflight.go:225`) admits a rooted `\x` path and then joins it under `--dir`. It also admits a
drive-relative `C:x` path and reserved device names. `filepath.IsLocal` would refuse all three
and behaves the same on POSIX. Whether `C:x` or `NUL` can reach outside `--dir` was not tested.
Commands and results: `win-w59.fix2-handback.md`.

## Rollback

Revert the task's test changes and the companion/index rows. Keep the pre-existing production
platform refusals, Windows skips and PTF-V0-755 root-termination fix. No production wire, process
containment, release qualification or native ticket state changed.
