# V1-1055: procgroup proves quiescence before the reap (AHI-049, proposed)

Date: 2026-10-09. Ticket: V1-1055. Lane base: `92e91583` (batch Q integration, which contains
V1-1040). Requirement: `AHI-049` (proposed) in `docs/specs/agent-harness-integration-v0.md`,
extended by this change and not accepted.

## Decision

`internal/procgroup` `run` called `proveProcessGroupQuiescent` after `command.Wait`. It polled
`kill(-pgid, 0)` and guarded the result with the leader's start time. After the reap the numeric
group ID is not pinned, so the probe could observe a reused group and report presence or
quiescence wrongly. A reused PID with a different start time was also accepted as proof that the
group was gone.

The proof now runs before `releaseProcessCommand` and `close(reapAllowed)`, while the exited
leader is held unreaped and its PID pins the group ID. It polls the new
`groupreap.HeldGroupQuiet` with 5 ms to 50 ms backoff until the shutdown deadline. That function
is the existing `WaitResidue` proof: the `PGO-V0-006` /proc scan on Linux, and on Darwin signal 0
whose EPERM counts only when the process table shows a zombie-only group (V1-1036). After the
release, `run` sends no numeric group operation, including signal 0.

The cleanup is reported unproved in these cases:

- the exit was not observed by the shutdown deadline (`errUnheldGroupUnproved`);
- the proof fails;
- a live member remains at the deadline;
- the exit observation fails (unchanged from V1-1040).

A `JoinAncestorProcessGroup` run leads no group. Its observed exit is its quiescence, because the
earlier post-reap single-PID probe had the same reuse problem.

Two consequences follow:

- **Identity read removed.** The launch-time start-time read (`processidentity.Start`) and its
  `PARTIAL` fallback only guarded the post-reap probe, so both are removed.
- **Reap bound added.** A proof that uses up the shutdown deadline would forfeit the reap of an
  already-exited leader. The reap therefore keeps a 100 ms bound (`exitedLeaderReapBound`) so the
  exit status is still read.

Review of the first revision (`07fca80f`) found that ownership could be released concurrently.
Timeout cleanup chose `leaderExited` once. A foreign reap then made the waitid goroutine fail with
ECHILD and release the group. The grace loop kept probing and could send SIGKILL to the numeric
group after that release. Every numeric group signal, probe and proof of `run` now goes through a
`groupOwnership` lock that is also taken by the release. A call after the release is refused with
`errGroupOwnershipReleased` and the cleanup stays unproved. This holds on the success, error,
timeout and ECHILD paths. Foreign in-process reaping remains an unsupported condition (`AHI-050`):
between the foreign reap and the goroutine's release the group ID is not pinned. It no longer
causes signals after the release.

The test hook `groupProbe` now covers every group signal `run` sends itself: termination, the
pre-reap grace probe and SIGKILL. A new `quietProof` hook replaces `HeldGroupQuiet`. Production
passes no hooks.

`CRR-V0-003`(d) is accepted and requires the post-reap identity-bound probe. This change adds a
dated note there proposing its replacement under `AHI-049`. The note needs owner acceptance and is
not marked accepted.

## Evidence

- `TestV11055NoNumericGroupProbeAfterReap` runs `sh -c '/bin/sleep 30 & exit 0'` and fails if
  any group operation reaches the `groupProbe` hook after the release. It also requires the
  cleanup to be proved. On `92e91583` it fails on Darwin arm64 with "3 numeric group operations
  after the group was released for the reap", and after the change it passes.
- `TestV11055QuiescenceProvedWhileLeaderHeld` requires every proof call to run before the release
  while `waitid(WNOWAIT)` still sees the leader. It also requires an injected proof error to report
  `process-cleanup-failed` without proved cleanup.
- `TestV11055NoGroupOperationAfterConcurrentRelease` kills and reaps the leader with `Wait4`
  from the first termination signal, after a 20 ms timeout. It requires no group operation after
  the release and unproved cleanup. It retries when waitid saw the exit before the foreign reap.
  On `07fca80f` it fails with "3 numeric group operations after the group was released".
- `TestRunProcessQuiescenceTreatsPermissionDeniedAsPresent` (cited by `CRR-V0-003`) now injects an
  unproved held proof. It requires unproved cleanup with the leader's exit status retained.
- Two identity tests are deleted with the identity path:
  `TestRunProcessQuiescenceIgnoresReusedPIDIdentity` and
  `TestRunProcessQuiescenceDisclosesIdentityFallback`.

Results on Darwin arm64, Go 1.27.1:

- `go test -race -p 1 -count=1` passed for `internal/procgroup`, `internal/groupreap` and
  `internal/gitstatus`.
- `go test -p 1 -count=1` passed for those three and for the direct `internal/procgroup` dependents
  that `corvint affected --base 92e91583` selected, including `cmd/corvint` (1310 s) and
  `conformance/cli-parity-v0`.
- `GOOS=linux go vet` passed for `internal/procgroup` and `internal/groupreap`. The lane doc gates
  and `go test ./internal/specindex` passed.
- `NOT_RUN`: the direct dependents of `internal/groupreap` other than `internal/gitstatus`. The
  groupreap change only adds an exported function.
- The review fix (ownership lock) was retested with `go test -race -count=1` for
  `internal/procgroup` and `internal/groupreap`, which passed, and `GOOS=linux` and `GOOS=windows`
  vet. The `internal/procgroup` dependents and `internal/gitstatus` were not rerun after it; the
  fix changes no exported API.

## Limits

- Linux runtime: `NOT_RUN` on this host. Only `GOOS=linux go vet` ran for `internal/procgroup` and
  `internal/groupreap`. The Linux path is the existing `PGO-V0-006` proof, which requires the
  leader to be this process's exited child.
- Darwin keeps the `PGO-V0-007` window: the process table and signal 0 are read separately, so a
  process joining the group between them is not excluded. This is the same as `WaitResidue`.

Rollback: revert this commit. `run` returns to the post-reap identity-bound probe.
