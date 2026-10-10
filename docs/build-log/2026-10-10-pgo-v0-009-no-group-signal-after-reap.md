# PGO-V0-009: no process-group signal after the reap (RC3 lane B)

Date: 2026-10-10. Base 64b5d174. Host Darwin arm64. Decision 0492 accepts `PGO-V0-009`.
Tickets: V1-1036, V1-1040, V1-1047, V1-1050, V1-1055 (earlier commits in this lane, each with its
own 2026-10-09 build log), V1-1056, and the RC3 inventory of the remaining sites.

## Decision

After a leader is reaped, its numeric group ID can be reused. Any group signal sent then can reach
an unrelated group. This lane closes the remaining product sites that sent such signals and adds
one shared fix in the `groupreap` primitive. Signal 0 delivers nothing, so a post-reap signal-0
check is kept only where it fails closed: a reused ID can only make the group look present.

| Site | Before | After |
| --- | --- | --- |
| `internal/tasks/store` `RunBatchCommand`, preflight (`killGateGroup`) | Real group SIGKILL after `execute` reaped the leader | Removed. `reapGate` kills the group before the reap; `execution` no longer carries a group ID |
| `internal/dashboard/repository` Linux wait (`quiescent`, `stopAndProve`, `groupExists`) | Post-reap signal-0 quiescence check, plus dead post-reap stop code | Residue, cleanup and quiescence come from the `PGO-V0-006` proof itself (new `groupreap.ProcGroupLive`: leader identity, status `Threads` counts, failures reported as live) before `reapAllowed` closes. The lane's first version walked `task/` and ignored read errors; review found it. The dead code is removed. The Darwin stop loop now checks that the leader is still pinned before every signal-0 probe and signal |
| `internal/tasks/cli` doctor plugin | `Cancel` was a group SIGKILL that could race the reap, then an unconditional post-reap group SIGKILL | `groupreap.Contain` plus `groupreap.Run`. The timeout stops the leader only, and the group is killed before the reap |
| `cmd/corvint` work version probe | `Cancel` was a group SIGKILL that could race the reap, and there was no sweep: a detached descendant survived | `groupreap.Contain` plus `groupreap.Run` |
| `cmd/corvint` work adapter runner | Setpgid group started with plain `command.Start`, so `KillLive` could not retire it | `groupreap.StartLive`; the existing pre-reap `WaitResidue` releases it |
| `groupreap.Run`, `RunRetiring`, `RunContainedRetiring` (V1-1056) | Plain `command.Start`, not recorded for the `AHI-048` exit kill | Start through the live registry. `RunContainedRetiring` releases after its last pre-reap group signal, and before `Wait` on the failed-observation path |

The retained fail-closed post-reap signal-0 observations are:
- the pool's `CAL-V0-188` absence check (`poolGroupLiveContext`);
- the release gate's `processGroupGone`;
- the Owner's absence check (`PGO-V0-001`, `PGO-V0-003`).

The supervisor's `groupExists` checks a recorded identity this process did not start, so it is out
of scope.

## Evidence

Each new regression test failed with the old code and passes with the change (Darwin; the store,
dashboard, groupreap and cli tests ran with `-race`):

- `TestPGOV0009BatchCommandSendsNoGroupSignalAfterTheReap`. A `signalGateGroup(-1<<20, 9)` call
  re-inserted after `execute` fails it with "store signalled groups [-1048576]".
- `TestPGOV0009RunFamilyRecordsGroupUntilPreReapRelease`. With the old `Run`, `RunRetiring` and
  `RunContained`, each subtest fails with "did not record the group". It waits for the
  registration and checks that the group is still recorded at the pre-reap group SIGKILL.
- `TestPGOV0009DarwinStopSendsNothingOnceUnpinned`. With the old stop loop, an exit observation
  that fails during the grace period is followed by 175 more signal-0 probes.
- `TestPGOV0009ProcGroupLiveUsesTheQuietProof` covers the Linux proof on a fake /proc: a zombie
  member with a surviving thread is live, and an identity mismatch or a missing `Threads` count
  is an error.
- `TestPGOV0009WorkRunnerRecordsItsGroupForTheExitKill`. With the old runner, the adapter never
  starts through the live registry.
- `TestPGOV0009DoctorPluginCancelSendsNoSignalAfterTheReap`. With the old group-kill `Cancel`, a
  cancel after the reap sends a group signal and gets ESRCH instead of `os.ErrProcessDone`.
- `TestPGOV0009WorkVersionProbeKillsItsGroupBeforeTheReap`. With the old probe, the detached
  member survives.

Packages and checks:
- `internal/groupreap`, `internal/dashboard/repository`, `internal/doccompiler`,
  `internal/taskman`, `internal/liveverify/mutate` and `internal/criterionexperiment` pass in
  full with `-race`.
- The `internal/tasks/store`, `internal/tasks/cli` and `cmd/corvint` tests that cover the changed
  paths pass.
- `GOOS=linux go vet` and `GOOS=windows` vet/build pass for the changed packages.

## Limits

- Linux amd64 runtime is NOT_RUN. On Linux arm64, a local `golang:1.27.1` container with a
  tmpfs `/tmp` passes `internal/groupreap`, `internal/dashboard/repository`, and the focused
  `internal/tasks/store`, `internal/tasks/cli` and `cmd/corvint` tests. This is not hosted
  qualification.
- `internal/liveverify/gorunner` `cleanupCommand` still sends a real group SIGKILL and a signal-0
  poll after `command.Wait` has reaped the leader. Its `Cancel` (`terminateCommand`) also sends a
  group SIGKILL that can race the reap. The go-live-test-provider contract and its tests pin this
  order, so changing it needs a contract amendment and provider requalification. It is proposed
  as follow-up work, not changed here.
- The pool and release-gate checks could move to a pre-reap quiet proof, but `CAL-V0-188`
  currently requires the pool's post-reap ESRCH fast path. They are kept as documented,
  fail-closed exceptions.
- V1-0668 still needs hosted Linux amd64 and arm64 qualification. The cem1 producer harness lives
  outside this repository, so it is not addressed here.
- Platforms without waitid keep the legacy post-reap signal (`PGO-V0-008`).
- Pre-existing flake: `TestRetirerRetiresDetachedDescendants` (Darwin) sometimes fails under host
  load with "owner token unreadable for pid N: invalid argument". This happens when a same-user
  process exits during the ownership scan. It passes on rerun, and its path does not call the
  changed functions.

## Rollback

Revert the lane's PGO-V0-009 commits and decision 0492, then regenerate
`docs/specs/REQUIREMENTS.tsv`. No store or wire format changes.
