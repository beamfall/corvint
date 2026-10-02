# groupreap: owner that signals before the leader is reaped

Status: experimental; no caller in this change. Part of the CEM stable S0E native
repair (V1-0632). `groupreap.Wait` and `groupreap.Run` are unchanged.

## What landed

`internal/groupreap/owner*.go` adds an `Owner` for one process group. It keeps the
leader unreaped while the group is signalled, so the numeric group ID stays
reserved and cannot be reused before the signal. After the leader is reaped the
owner sends no further signal: it probes with signal 0 only, until `ESRCH` or a
fixed 2-second deadline.

A deadline, a probe error, or any group-signal error (including one from an
early `Stop`) is a HOLD. The Stable caller maps HOLD to
`repository/unsupported-process-containment`, exit 2, with no retry. A HOLD in a
short-lived CLI leaves the group to the operating system at process exit; that is
not observed cleanup. Platforms other than Darwin and Linux return
`ErrUnsupported`.

## Evidence

- Two independent reviews of the native overlay containing this code. The first
  required the bounded poll in place of a single probe; the second found that an
  early `Stop` dropped a signal error, which is repaired with
  `TestOwnerStopKillErrorThenFinishHolds`. The confirming review approved.
- Darwin arm64, Go 1.27.1: `go test -count=20 ./internal/groupreap/` passes;
  `go vet` passes natively and for `GOOS=linux` and `GOOS=windows`.

## Not run / open

- Linux lifecycle is `NOT_RUN`; cross-vet is compile evidence only.
- The callers (`internal/cem/gitrun`, `internal/cem/gitauth`, Stable verify) are
  not in this change: those paths are outside the declared effects of V1-0632
  until the owner amends them. The reviewed overlay is retained outside the
  repository.
- Legacy callers of `Wait`/`Run` keep their existing behaviour (V1-0652).

## Rollback

Delete the five `owner*` files. Nothing imports them.
