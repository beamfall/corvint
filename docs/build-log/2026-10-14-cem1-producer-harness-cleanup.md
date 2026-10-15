# 2026-10-14 cem1-producer-harness-cleanup-20261001: stable proof harness cleanup is the owned Go runners

1.0 scope: `PRS-V1-018` (1.0 scope per the V1-0020 dependency). Base: origin/main
b675a948191bf5b48036b1910bcde8454b82bdf0. Governing contract: `docs/specs/process-group-owner-v0.md`
(PGO-V0-001, -003, -006, -009). New requirement: `PGO-V0-012`, accepted under the owner's standing
auto-accept (2026-10-11); cem1-producer-harness-cleanup-20261001. The ticket stays with the
orchestrator; this lane ran no Tasks mutations.

## Original failed harness evidence (preserved by citation)

The defect (SP-IMPL-M1) was in a private scratch script, `prove_historical.py`, used for the
2026-10-01 stable producer prototype's historical join proof. Its lines 23-32 called an unbounded
`communicate()`, sent a numeric process-group signal after the child was reaped, and then wrote
`ownedGroupRetired=true` unconditionally. The retained evidence is:

- the ticket body, `.taskman/tickets/cem1-producer-harness-cleanup-20261001.json` (blob
  1fcbde16e8402e78624760fa1596efc144d35830 at BASE), with the frozen packet digest
  9dc271862aafe7d3a72a599f20601290c01a9a38af1d0ce2d9558505e33f3513 and the review digest
  e1f022326664cd8f42172bb6572996570faeefd56d023610c2f35fb85fe0d148;
- `docs/build-log/2026-10-01-stable-producer-prototype.md` (blob
  5ee3ce91bdfbd1470b9ac60fa7e27c2924614a80), which names the evidence root
  `/private/tmp/cem10-build/stable-next/stable-producer-prototype`.

The frozen packet bytes are no longer on this host. The evidence root holds only
`ordinary-sha256-envelope/`; `prove_historical.py`, `closed-manifest.json` and
`historical-proof/summary.json` are gone. Nothing was reconstructed, and the unsafe harness was
neither recreated nor run. At BASE, nothing outside the ticket references `prove_historical` or
`ownedGroupRetired`. The replacement evidence below is bound separately by this change's CEM.

## Inventory of paths in the harness role

A path is in the role if it runs a producer, verifier or historical-proof subprocess and records a
retirement or cleanup outcome.

| Path | Unbounded wait | Post-reap group signal | Retirement recorded without observation |
|---|---|---|---|
| `internal/cem/gitrun` stable runner (`runOwned`, `Session` owned path) | no: `FinishBounded` under the retire bound | no: Owner, one pre-reap `KillGroup`, signal-0 only after the reap | no: anything unproved is `ProcessContainment` HOLD |
| `internal/cem/gitrun` legacy `Run` | no | no: `groupreap.Wait` signals before the reap (PGO-V0-008) | records none |
| `interop/cem01-go/stable_process_unix.go` | no: `stableRetire` under `s.bound()`, post-reap poll capped at 2s | no: `kill()` only before `stableRetire` | no: failure retains the keeper and returns `unsupported-process-containment` |
| `internal/procgroup` (`tools/cem-trial`, `benchmarks/runner`) | no | no: quiescence proved before the reap (AHI-049) | no |
| `cmd/corvint-cem-candidate assemble-stable` | no subprocess | - | - |
| `tools/cem-citation-baseline` | no | leader-only `CommandContext` kill | records none |

The prototype harness was superseded by the Core stable verifier and the portable verifier, which
were the two subprocess-running verifiers in the historical proof. Both already meet acceptance 1,
so this change adds no product code. It adds maintained tests that pin acceptance 2 on those paths,
plus `PGO-V0-012`.

## Acceptance mapping

1. Bounded, ownership-safe cleanup with truthful outcomes is already in place. These tests pin it:
   - Owner: `TestOwnerNormalExitRetiresProbesReapsAndObservesAbsence`,
     `TestOwnerStopSignalsOnceAndFinishDoesNotSignalAgain`,
     `TestOwnerHoldsWithoutSignallingWhenExitObservationFails`,
     `TestOwnerHoldsWhenExitIsNotObservedWithinTheBound`, `TestOwnerHoldPaths`,
     `TestOwnerConcurrentFinishAndStopSignalAndReapOnce`,
     `TestOwnerHoldsOnInjectedSignalFailureWithoutReaping`,
     `TestLinuxOwnerHoldsWhenQuietProofUnavailable`.
   - Stable runner: `TestStableCausePrecedence` (unobserved cleanup gives `ProcessContainment`).
   - Portable verifier: `TestStableLifecycleRetireBoundHold`, `TestStableLifecycleCloseFailHolds`.
2. Normal and interrupted descendants retire with no signal after identity ownership ends. Existing
   coverage: Owner `TestOwnerRetiresLingeringMemberAfterNormalExit`,
   `TestOwnerStopRetiresRunningGroup`, `TestOwnerRetiresHostileGroups`; portable
   `TestStableLifecycleLingeringDescendantRetired`, `TestStableLifecycleCancelRetiresDescendants`,
   `TestStableLifecycleHostileDescendantsRetired`. The stable runner had no real-descendant test and
   no signal-versus-reap ordering check, and the portable timeout test did not check descendants.
   Added:
   - `internal/cem/gitrun/ownership_test.go`:
     - `TestPGOV0012StableDescendantRetiresWithoutPostReapSignal` covers `normal-exit`, `cancel` and
       `timeout`, each with a real lingering `/bin/sleep` descendant. A ledger wraps the real
       `KillGroup` and `Reap` primitives (platform-default quiet proof and probes) and must read
       exactly `kill-group, reap`. The budget must be released, not held. The descendant must
       reach ESRCH, observed with signal 0 only.
     - `TestPGOV0012StableFailedQuietProofHoldsWithoutReapOrSecondSignal`: an injected quiet or
       identity proof failure gives `ProcessContainment` and a sticky hold, with the ledger at
       `kill-group` only. There is no reap, and the next operation is refused without a signal.
     - `TestPGOV0012StableUnobservedExitHoldsWithoutAnySignal`: an injected exit-observation
       failure gives `ProcessContainment` and a hold, with no group signal and no reap.
   - `interop/cem01-go`: `TestStableLifecycleOpTimeoutRetiresDescendants` (a per-operation timeout
     retires a lingering descendant, with no hold).

Test cleanup for a HOLD is test-owned. It first removes the leader from the exit-kill registry
(`groupreap.ReleaseLive`). Only then does it signal and `Wait4` the leader, which is still this
process's unreaped child, so its PID cannot be reused.

## Verification

Darwin arm64, go1.27.2, `GOTOOLCHAIN=local`:

- `go test -race -count=1 -run TestPGOV0012 -v ./internal/cem/gitrun/`: all five cases PASS.
- `go test -race -count=30 -run TestPGOV0012 ./internal/cem/gitrun/`: PASS. The new tests ran 30
  times under host load averages of about 60 to 100.
- `go test -race -count=1 ./internal/cem/gitrun/`: PASS at moderate load, four times. At load
  averages above 100 it fails, with or without the new tests (`-skip TestPGOV0012` gives the same
  failures). The failing tests are pre-existing ones: `TestDescendantCleanupOnTimeout`,
  `TestRunStreamContainsDescendants` and siblings, which wait a fixed time for a grandchild PID
  file. This is environment load, not this change.
- `go test -race -count=1 ./internal/groupreap/`: PASS, run alone. Its first run, under load,
  failed `TestRunContainedRetiresEscapedSessionWithoutCollateral` on the bound.
- `(cd interop/cem01-go && go test -race -count=1 -run TestStableLifecycle -v .)`: 19 PASS.
- `go vet` on the touched packages (also with `GOOS=linux`): PASS. `gofmt -l`: empty.
- `corvint affected --base BASE` selected 100 units over 6 dirty paths (1 direct test change; the
  rest are path-literal, unbounded or declared-scope readers):
  - 82 Go packages, excluding `cmd/corvint`, ran in two batches under load. 71 passed. The other 11
    failed on timing or tree state; each passed when rerun alone once: `internal/mcp/bridge`,
    `cmd/corvint-go-test-provider`, `conformance/cli-parity-v0`, `conformance/go-live-test-v0`,
    `conformance/release-artifact-v0` (worktree was dirty mid-run), `internal/appmap` (30m
    timeout), `internal/companionrelease`, `internal/lspstdio`, `internal/tasks/cli`,
    `internal/testrunner` and `tools/cem-trial`.
  - `cmd/corvint` is NOT_RUN locally: alone it needs about 600s. It is left to PR CI.
  - 17 shell tests: 16 PASS. `script/gate-affected_test.sh` also fails (rc=1) in a clean BASE
    worktree. On this branch it stops at the missing `frontier` lines for a deleted file. It is
    pre-existing and outside this diff.
- 12 CI doc gates, run after staging: PASS.

Linux runtime for the new tests comes from this change's PR CI only.

## Limits

- `stableClose` in `interop/cem01-go` keeps a `stableHeld` entry whose `proc` was already reaped
  when a post-reap poll fails. A second `stableClose` would then signal that group after the reap.
  This cannot happen today: `stableClose` runs at most once, guarded by the `closed` flag in
  `stable_repository.go`. It is recorded here and left unchanged.
- Paths outside the harness role have the same defect class. They are out of scope here and are
  left for the orchestrator to file:
  - `script/qualify-checkpoint-handoff.py` (`killpg` after `communicate(timeout=90)`, then an
    unbounded `communicate()`);
  - `script/measure-lsp-quality.py` and `script/qualify-lsp.py` (`killpg`, then an unbounded
    `communicate()`);
  - `tools/beamfall-shadow/process_unix.go` `cleanupProcessGroup` and
    `tools/beamfall-shadow/wrapper/main.go` `reap` (group signals after `Wait`);
  - `tools/ci-test-slices/procgroup_unix.go` (`cmd.Cancel` group signal can race the reap).

  The `internal/liveverify/gorunner` post-reap cleanup remains the known PGO-V0-009 gap.
- Linux runtime: NOT_RUN on this host.
- Darwin signal-0 EPERM counts as quiet before the reap, in both the Owner and the portable
  verifier. A live member that cannot be signalled ends as HOLD after the reap; it is not proved
  absent beforehand.

## Review

One Codex read-only round (`codex exec -s read-only`, diff BASE..change commit). Four findings:

- HIGH, refuted as stated. The claim was that a failed Darwin quiet probe turns `stableRetire` into
  an unbounded `proc.Wait()`. `stableRetire` runs only after the group SIGKILL. The keeper is this
  process's own child, so while it lives, signal 0 to its group succeeds and the loop keeps
  checking the bound. `proc.Wait()` is therefore reached only for an exited keeper and returns
  promptly. The underlying observation is real and is recorded as a limit: Darwin signal-0 EPERM
  counts as quiet. The Owner's default `ProbeGroup` does the same. A live member this process may
  not signal is therefore reaped past, and the post-reap probe never sees ESRCH, so the outcome is
  HOLD and stays truthful. PGO-V0-012 now states this limit.
- MED, confirmed and recorded. The portable timeout test cannot see kill-versus-reap order. The
  portable verifier has no recording seam, and adding one would be a product change outside this
  test-only slice. Its order is pinned by the code structure (`kill()` precedes `stableRetire`)
  and by the Core ledger test. PGO-V0-012 now states this.
- MED, fixed. The new portable test used `pidGone`, which sends SIGKILL to a bare, already
  unpinned PID on failure. It now observes with signal 0 only and fails without signalling.
- LOW, fixed. PGO-V0-012 said the SIGKILL is sent while the exited leader is held. On cancellation
  and timeout, `Stop` sends it while the leader may still run, before the reap. The text now says
  so.

## Rollback

Revert this change. It adds tests, one requirement and this entry. Runner behaviour, wire formats
and stores are unchanged.
