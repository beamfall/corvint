# Reproduction Bundle V0

Owner: Russell Lewis
Date: 2026-10-27
Intent status: experimental under the standing lane auto-accept (2026-10-27; V1-2840)
Delivery status: experimental
Authoritative inputs: V1-2840 / GitHub #1914 owner request (record a corvint command with its
inputs pinned by digest or revision, its output, exit code and optional expectations; replay it
against the current build printing pass/fail per expectation plus build identity; bundles are
plain files attachable to issues); `PUB-V0-050..052` (V1-2839, #1902) for the build identity;
AGENTS.md invariants 1, 2, 4 and 8.

## Agent digest
- Claim: `corvint repro` records one invocation with pinned inputs as a bundle and replays it, reporting PASS/FAIL per expectation and both builds.
- Status: experimental under the standing lane auto-accept (2026-10-27; V1-2840)/experimental
- Exists: `internal/repro` (schema, validation, evaluation) and `cmd/corvint/repro.go`; `run` refuses, running nothing, when HEAD, the tracked tree or an input file differs from the pin.
- Blocked on: owner acceptance; stdin, environment and non-repository inputs are not captured.
- Read next: Requirements; Boundaries and failure modes.

## User and verified baseline

A user who meets a wrong answer or refusal from corvint files an issue with a pasted command and
output. The maintainer cannot tell which repository state, input file or binary produced it, and a
later fix has no executable check that the reported behaviour changed. Before this spec corvint had
no record or replay verb; `--version --json` gained a source commit in `PUB-V0-050`.

## Requirements

- `RPB-V0-001`: A bundle MUST be one JSON object with exactly the members `profile`
  (`corvint-repro/0`), `recordedBy` (`version`, `displayVersion`, `build`, `commit`, `commitState`,
  `release` from the recording binary's `PUB-V0-050..052` identity, with `build` `unstamped` for an
  unstamped binary), `command.arguments` (the corvint arguments after the global `--root`
  preamble), `inputs.repository` (`commit`, `tree`), `inputs.files` (`path`, `sha256`), `observed`
  (`exitCode`, `stdoutSha256`, `stdoutBytes`, `stdout`, `stdoutTruncated`, `stderr`,
  `stderrTruncated`) and `expectations`. Decoding MUST refuse an unknown member, trailing data or
  another profile.
- `RPB-V0-002`: Bounds: a bundle of at most 4 MiB; 1 to 64 arguments of at most 4096 bytes and no
  NUL; at most 32 input files, each a clean repository-relative slash path; 1 to 32 expectations
  with unique IDs; stdout kept up to 64 KiB and stderr up to 16 KiB, with `stdoutSha256` and
  `stdoutBytes` always over the full stdout and untruncated stdout equal to its digest. `record`
  MUST create `--output` exclusively and refuse an existing file (`repro-output-exists`); a write
  failure removes the partial file (`repro-output-failed`). A malformed bundle refuses with
  `repro-bundle-invalid`, an unreadable one with `repro-bundle-unavailable`, and a malformed
  invocation with `repro-invalid-arguments`.
- `RPB-V0-003`: Inputs are pinned, never assumed. `record` MUST refuse with
  `repro-inputs-unpinned` when the tracked tree under `--root` has uncommitted changes or a declared
  `--input` is missing, not a regular file, or larger than 16 MiB. `run` MUST refuse with
  `repro-inputs-unavailable`, running nothing and printing nothing on stdout, when HEAD is not the
  pinned commit (naming whether that commit is absent from the repository or merely not checked
  out), the tracked tree is dirty, or an input file is missing or its SHA-256 differs; the message
  names every unavailable input. A `--root` that is not a readable Git checkout refuses with
  `repro-repository-unavailable`. Untracked files other than declared inputs are not pinned.
- `RPB-V0-004`: `record` and `run` MUST execute the command in-process as
  `corvint --root ROOT ARGUMENTS...` with empty stdin, so the replay runs on exactly the binary that
  prints its identity. Arguments MUST NOT begin with `--root` (the replay supplies it) or name
  `repro` (no recursion).
- `RPB-V0-005`: `record` exits 0 whatever the recorded command returned and prints
  `corvint-repro-record/0` (`bundle`, `exitCode`, `stdoutSha256`, `repository`, `inputFiles`,
  `expectations`).
- `RPB-V0-006`: Expectations are `exit-code` (`exit` 0..255), `stdout-contains`,
  `stdout-not-contains`, `stderr-contains` (`text` of 1 to 4096 bytes) and `stdout-same` (the full
  stdout digest equals the recorded one). `record` builds them from `--expect-exit`,
  `--expect-stdout-contains`, `--expect-stdout-not-contains`, `--expect-stderr-contains` and
  `--expect-same-stdout` in flag order with IDs `e1`, `e2`, ...; with no flag the expectations are
  the recorded exit code and `stdout-same`.
- `RPB-V0-007`: `run` MUST print `corvint-repro-run/0` with `outcome` (`PASS` when every
  expectation passes, else `FAIL`), `results` (per expectation `id`, `kind`, `status`
  `PASS`|`FAIL`, `expected`, `observed`), `build` (this binary's identity), `recordedBy`,
  `sameBuild` (true only when both identities are equal and the commit is known), `repository`,
  `arguments` and `observed` (`exitCode`, `stdoutSha256`, `stdoutBytes`), and exit 0 on `PASS`
  and 1 on `FAIL`. Refusals exit 2.

- `RPB-V0-047`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2917, GitHub
  #1923) `record` and `run` MUST execute the command with the process working directory set to
  `--root` for the duration of the run (restored afterwards, under a lock), so a relative path in the
  recorded arguments resolves against the repository root and a record and a replay agree whatever
  directory the caller is in. The working directory is therefore not an unrecorded input and no
  field records it. Non-goal: resolving paths in arguments other than by running from the root.
  Falsifier: the same bundle yields different observations from two caller directories. Rollback:
  remove the working-directory change in `reproExecute`.
- `RPB-V0-048`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2917, GitHub
  #1923) `record` MUST add `warnings` (one `the observed run does not satisfy expectation ID` per
  failing expectation) to its output when the observed run fails an expectation of its own bundle;
  the bundle is still written and the exit stays 0 (RPB-V0-005). The member is absent otherwise.
  Falsifier: a record whose own run fails an expectation without a warning.

## Boundaries and failure modes

Non-goals: stdin, environment variables, the user cache and files outside the repository are not
captured; replay never checks out, fetches or rewrites anything to reach the pinned commit; bundles
are not signed; output normalisation (timestamps, absolute paths) is left to `*-contains`
expectations; recording other binaries (`corvint-tasks`, the MCP servers).

Failure modes: a command whose stdout embeds the absolute `--root` or a time makes `stdout-same`
fail on another machine or later run, which is visible as FAIL with both digests, never a silent
pass; a recorded command that writes local state (for example a CEM verb) writes it again on
replay, so a replay is exactly as mutating as the recorded command; a dependency on an undeclared
untracked file is not detected (declare it with `--input`).

## Implementation, rollout and rollback

`internal/repro/repro.go` holds the schema, bounds, validation and evaluation;
`cmd/corvint/repro.go` holds the CLI, Git pinning and in-process execution. Rollback removes the
`repro` verb, its help topic, catalog row and the package; no other command reads a bundle.

## Acceptance evidence

| Requirement | Implementation | Test |
|---|---|---|
| RPB-V0-001 | `internal/repro/repro.go` (`Bundle`, `Decode`) | `TestRPBV0001BundleRoundTripsThroughClosedSchema` |
| RPB-V0-002 | `internal/repro/repro.go` (`Validate`), `cmd/corvint/repro.go` (`reproRecord`, `reproRun`) | `TestRPBV0002BundleBoundsAndPins`, `TestRPBV0002RunRefusesMalformedBundles` |
| RPB-V0-003 | `cmd/corvint/repro.go` (`reproRepository`, `reproInputDigest`) | `TestRPBV0003UnpinnableInputsRefuse` |
| RPB-V0-004 | `cmd/corvint/repro.go` (`reproExecute`), `internal/repro/repro.go` (`ValidateArguments`) | `TestRPBV0004RecordAndRunRoundTrip`, `TestRPBV0002BundleBoundsAndPins` |
| RPB-V0-005 | `cmd/corvint/repro.go` (`reproRecord`), `internal/repro/repro.go` (`Observe`) | `TestRPBV0004RecordAndRunRoundTrip`, `TestRPBV0005ObservationKeepsBoundedText` |
| RPB-V0-006 | `cmd/corvint/repro.go` (`parseReproRecord`) | `TestRPBV0006RunFailsAnUnmetExpectation`, `TestRPBV0004RecordAndRunRoundTrip` |
| RPB-V0-007 | `internal/repro/repro.go` (`Evaluate`), `cmd/corvint/repro.go` (`reproRun`) | `TestRPBV0007EvaluateJudgesEachExpectation`, `TestRPBV0006RunFailsAnUnmetExpectation` |
| RPB-V0-047 | `cmd/corvint/repro.go` (`reproExecute`) | `TestRPBV0047RelativeArgumentsResolveAgainstRoot` |
| RPB-V0-048 | `cmd/corvint/repro.go` (`reproRecord`) | `TestRPBV0047RelativeArgumentsResolveAgainstRoot` |

## Delivery

Experimental. Live use on the Corvint checkout and an issue-attached bundle are `NOT_RUN`.
