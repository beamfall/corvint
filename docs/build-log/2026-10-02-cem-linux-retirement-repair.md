# CEM Linux retirement availability repair

Date: 2026-10-02

## Context

PR [#476](https://github.com/beamfall/corvint/pull/476), source
`d5fdbe733b5bfbc0ff3c5a2aa2d1cc195bb8e0d2`, was blocked by hosted Linux CI.

- The canonical-create fixture called `commit-tree` without setting fixture-local
  Git author identity. Hosted CI reported “Author identity unknown” in both
  canonical-create tests. The repair sets local `user.name` and `user.email` in
  the fixture, preserving shared helpers and all assertions.
  [Source](https://github.com/beamfall/corvint/blob/d5fdbe733b5bfbc0ff3c5a2aa2d1cc195bb8e0d2/internal/cem/gitauth/canonical_create_test.go),
  [failed CI job](https://github.com/beamfall/corvint/actions/runs/36963786786/job/110703115422).
- Six stable/candidate cases refused at the repository stage. Their prior failure
  messages printed the optional code as a pointer, so hosted logs did not
  establish scalar refusal codes. Failure-only JSON diagnostics now expose the
  complete existing result without changing assertions.
  [Stable tests](https://github.com/beamfall/corvint/blob/d5fdbe733b5bfbc0ff3c5a2aa2d1cc195bb8e0d2/internal/cem/verify/stable_repository_test.go),
  [candidate tests](https://github.com/beamfall/corvint/blob/d5fdbe733b5bfbc0ff3c5a2aa2d1cc195bb8e0d2/internal/cemcandidate/stable_test.go).
- Another hosted job timed out after 50 minutes in
  `TestStableSuccessReleasesOwnedGroup`. Its stack waited in the pre-reap group
  probe loop. The test injects a fixed budget clock, and the Linux adapter treats
  a successful null signal as a live group. The repair introduces explicit
  platform retirement ordering: Darwin keeps quiet-first retirement; Linux reaps
  after the successful owned signal and observed leader exit, then performs only
  bounded signal-0 absence polling.
  [Test and clock](https://github.com/beamfall/corvint/blob/d5fdbe733b5bfbc0ff3c5a2aa2d1cc195bb8e0d2/internal/cem/gitrun/lifecycle_test.go),
  [owner](https://github.com/beamfall/corvint/blob/d5fdbe733b5bfbc0ff3c5a2aa2d1cc195bb8e0d2/internal/groupreap/owner.go),
  [adapter](https://github.com/beamfall/corvint/blob/d5fdbe733b5bfbc0ff3c5a2aa2d1cc195bb8e0d2/internal/groupreap/owner_waitid.go),
  [timeout job](https://github.com/beamfall/corvint/actions/runs/36963786786/job/110703115457).

## Decision

The proposed stable contract now records the Linux `ReapAfterSuccessfulSignal`
phase, the original allowance checks before accepting absence, and sticky `HOLD`
for pending asynchronous reap completion. Linux preserves all `KillGroup` errors,
including `EPERM`, `ESRCH` and early `Stop` failures. No real numeric group
signal may occur after reaping begins.

The original allowance and the two-second post-reap cap form an intersection.
Expiration is checked at entry, before starting primitives and before accepting
absence; cap expiry is retained even when both timers become ready together.
Only a reap admitted before expiry may remain pending under sticky `HOLD`.

The newly introduced closed-bound Owner control previously expected
`kill,probe` after an already observed exit. It now expects no primitive calls
and `HOLD`. The former expectation contradicted the bounded ownership evidence
required by CEM-V1-001 and CEM-V1-007: an observed exit retains identity but cannot
extend the retirement allowance. Historical/public fixtures are unchanged.

Linux fixture members are direct children of the test process, joined to the
held leader's process group. Their collection is deferred until every real
signal decision has completed. Cleanup registers at each successful start,
serializes direct-child signals before the sole Wait, and bounds handshakes and
collection. The positive witness collects the member inside the Owner reap
wrapper before the final actual ESRCH probe; a no-op signal control with a held
member requires HOLD. No stored-PID null-signal check grants cleanup authority.

Deterministic controls cover expiry during probes and signals, simultaneous
timer readiness, before/at/after allowance boundaries, reap completion at expiry,
transient and persistent nonabsence, invalid-mode spawn refusal, two overlapping
Finish calls with Stop, and acknowledged late reap completion after HOLD.

## Qualification Limits

At source `1a355ea551c2745d3fe9f229a36acc348cb205d8`, fresh independent
source review passed. On Darwin arm64 with Go 1.27.1, the five scoped packages
pass tests and vet; groupreap/gitrun also pass the race detector. Linux amd64 and
arm64 compilation and the Windows unavailable-owner compile control pass.

Actual Linux arm64 with Go 1.27.1, Git 2.47.3 and kernel 6.8.0-117 now passes the
five scoped packages and all 54 Go 1.27.1 public full-result cases. Each executed
public case matches all 21 frozen result fields recursively without replacement
or normalization. The public packet and original comparator remain unchanged;
a private additive diagnostic driver supplies Linux execution of the original
recipes. Existing Darwin-only package controls remain explicitly skipped, and
the two Go 1.24.13 cases are not represented as Linux results.

The original portable comparator separately passes both Go 1.24.13 compatibility
cases on actual Darwin arm64: nonstructural acceptance and structural runtime
refusal. This is historical source-floor compatibility, not Core qualification
on Go 1.24 or ongoing security support. The existing official task-local compiler
archive and all 14,179 extracted regular files were verified; the installed
Go 1.27.1 toolchain and configured paths were unchanged.

Fixture-local Git identity also passes an actual Linux control with author
environment unset, global/system configuration disabled and useConfigOnly
required. Two initial Linux qualification harness runs failed: one mounted its
temporary directory without executable-script support; the other omitted the
immutable legacy fixture directories. Correcting those harness prerequisites
required no product edits. Original failures remain retained; later source- and
binary-bound results establish the stated package passes. All owned controllers
and containers were retired.

Actual hosted Linux amd64 and portable standalone Linux execution remain pending,
as do cross-device topology, escaped-session behavior, external consumer adoption,
default promotion and release readiness. This repair qualifies the new Owner
paths only; legacy Wait/Run remains a separate tracked issue. Public packets,
historical source and prior seals remain intact. Corvint impact and affected
advice were used; build-constraint/nested-module frontiers and unowned
documentation trace gaps remain explicit. Required final checks, CEM binding
and integration remain separate completion steps.
