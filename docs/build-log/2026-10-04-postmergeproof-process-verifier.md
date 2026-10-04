# Post-merge /2 process verifier slice

Date: 2026-10-04. Requirement: PMR-V2-006. Owning ticket: V1-0542 (#395), still OPEN.

Human-owned intent: on 2026-10-04 the owner scheduled the concrete `internal/postmergeproof`
process verifier now. The /1 follow-up and author integration are out of scope. PMR-V2-006 text and
the frozen `conformance/postmerge-runtime-v2/` data are unchanged.

## Change

- `VerifyProcessV2` checks the following, in order, and then mints the opaque
  `VerifiedProcessesV2` token:
  - the parent admission and host tuple;
  - the exact qualification report;
  - the closed policy;
  - the proof.
- The proof check follows a fixed step order:
  - native sources;
  - bracketed captures;
  - owned launches;
  - trusted start;
  - observed chromium-switch births;
  - native references;
  - cleanup joins;
  - the final sweep.
- Each preimage is read from the artifact store and must match its byte length and SHA-256.
- Only this package can mint a nonzero token. `ValidFor` binds it to the execution ID and the graph,
  policy, qualification and proof digests; the policy pins implementation and host tuple. A zero
  token yields BLOCKED `process-token-invalid` from `LogicalProcesses`.
- The private `verifyRawProcessV2` never mints a token. Qualification rechecks each of the ten frozen
  cases through it, so qualification does not call itself.
  - Negative cases must produce their hardcoded native code.
  - A case that is REJECTED surfaces as `process-qualification-invalid` with
    `caseID: code: detail`. A BLOCKED case keeps its own code.
- `procfs` (Linux amd64/arm64 only, by build tag) reads bracketed birth captures and the final
  `/proc` sweep. It reads in this order:
  1. boot ID
  2. stat
  3. argv
  4. executable link and bytes
  5. `/proc/self/ns/pid`
  6. native `ps -o lstart=` under `LC_ALL=C TZ=UTC`
  7. argv, link, executable digest and stat again

  Other hosts return `process-observation-unsupported` with outcome NOT_OBSERVED.

## Implementation decisions inside the frozen contract

- The host tuple identity is SHA-256 over the domain `postmerge-host-tuple/2`, then NUL, then the
  wire-canonical JSON. The frozen data names no domain for it, so a future conformance revision
  must ratify or replace it.
- Birth key: (boot ID, PID-namespace device, inode, PID, start ticks).
  - The before and after stat must agree on PID, parent PID and start; otherwise the result is
    `process-birth-changed`.
  - Zombie state, argv, executable link and executable digest must agree; otherwise the result is
    `process-bracket-changed`.
  - The PID namespace is the observer's own `/proc/self/ns/pid`. It stays readable when the target
    is a zombie.
- Native start is checked under two normalizations of the same output:
  - trimmed, as the freshness and app-instance observers record it;
  - fields joined by single spaces, as procgroup descendant rows record it.

  The capture runs `ps` under `TZ=UTC`, and joins compare strings exactly. A producer that recorded
  lstart in another time zone fails with `process-native-reference-invalid`; it is never coerced.
- Process state joins on the first stat state byte.
- Descendant cleanup keeps three cases distinct:
  - no descendants recorded;
  - descendants recorded absent;
  - descendants unknown.

  Zero, absent and unknown are never equivalent.
- The invocation environment allowlist is `LANG`, `LC_ALL`, `PATH`, `TMPDIR`, `TZ`, sorted. These
  are the testacceptance safe keys plus `TZ` for the native start tool.
- Process roles:
  - A launch's `launch_purpose` must equal its role.
  - Logical nodes are keyed by (context, role, slot).
  - Chromium-switch roles match a token exactly, or as a `switch=` prefix when the token has no `=`.
  - Births are visited in birth-key order, so witness order cannot change the result.
- Excluded from tracked workload absence: the trusted-start observer and the final-sweep observer
  witnesses.
- Native sources are sorted by (kind, id). Hook, control, attestation and producer-job locators are
  BLOCKED `process-native-source-unsupported`. `playwright-node-worker/0` is BLOCKED
  `process-role-unsupported`.

## Findings

- **Exec-window argv race (Linux 6.8 arm64).**
  - `exec.Cmd.Start` returns once exec closes the CLOEXEC pipe. That is before the kernel sets the
    new argv.
  - A capture taken in that window reads an empty `cmdline` while the exe link is already the new
    image. The verifier correctly refused it as `process-bracket-changed` (1 of 5 runs).
  - A collector must wait for exec to settle, or re-capture. The test harness waits for
    `/proc/<pid>/cmdline` to equal argv.
- **Reparented orphan.** A descendant reparented outside the captured tree is visible only if it was
  captured. A final sweep can prove a captured birth absent, but it cannot find a birth that was
  never captured. This limitation remains.

## Evidence

- darwin: `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/postmergeproof/...` passed.
  The procfs live tests skip as NOT_OBSERVED.
- Linux arm64, `golang:1.27.1` container on colima (Linux 6.8.0-117-generic aarch64, go1.27.1
  linux/arm64):
  - `go vet` was clean and both packages passed.
  - `TestLinuxProcfsOwnedExecution` ran real `/bin/sh`, `sleep` and `/bin/cat` observers. It derived
    the expected graph: supervisor, workflow root `completed/exit:0`, runner `cancelled/unreaped`.
    It recorded 7 captures, 5 swept rows and a native start from `ps`. Without a qualification
    report the result is BLOCKED `process-qualification-unavailable`.
  - `TestLinuxProcfsSweepFindsSurvivor` returned REJECTED `process-sweep-survivor`.
  - `-count=30` of the Linux and procfs tests passed.
- Linux amd64 execution is NOT_RUN: there is no amd64 image or emulation here. It was compiled and
  vetted by build tag only.
- This is source and unit evidence only. It is not host qualification.

## Remaining

The following are NOT_PRODUCED:

- the `internal/postmergehost` collector and producer integration (with exec-settle re-capture);
- PMR-V2-009/010 wiring;
- `playwright-node-worker/0`;
- the real qualification campaign;
- control-mutant server distinctness;
- hook, control, attestation and producer-job locators;
- Linux amd64 execution.

V1-0542 stays OPEN.

Rollback: revert this change. Nothing consumes the token.
