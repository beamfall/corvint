# V1-1116: OpenCode re-probes a zombie-only group before reporting unconfirmed cleanup

Date: 2026-10-10
Ticket: V1-1116
Requirement: AHI-052 (proposed, pending owner acceptance; amends AHI-050 for OpenCode only).

## Finding

Release-gates nightly run 38052349384 (macos-15) failed four OpenCode subtests of
`TestHostAdapterJavaScriptHosts` (AHI-032 delayed startup, `opencode timeout leaves no descendant`,
V1-0371 concurrent cancellations, AHI-022 routine receipt) with `corvint-process-cleanup-unconfirmed`
where a timeout or cancellation was expected. Every one is a termination path.

Since V1-0371 (7a4078b8, fd48aee7, c05bf0a4) a group SIGKILL that answers `EPERM` is no longer
accepted after the leader's exit; completion probes `kill(-pid, 0)` once and confirms only on `ESRCH`.
On Darwin a group whose members are all unreaped zombies answers `EPERM` to every signal, signal 0
included (checked locally: `EPERM` before `waitpid`, `ESRCH` after). The leader that SIGTERM kills
during the 25 ms grace is such a zombie until Node's poll phase reaps it. libuv 1.53 (Node 22.23)
runs due timers right after the check phase, before the next poll, so when the termination turn ends
after the 100 ms completion timer is due, completion probes the unreaped leader and reports
unconfirmed cleanup. On a quiet host the turn ends about 25 ms after the request; a hosted VM
stalled for more than about 75 ms crosses the timer. An orphaned member that launchd has not yet
reaped gives the same answer after close (inference for the AHI-032 shell fixture; not observed).

Cause status: the timer-ordering mechanism is reproduced deterministically on Darwin by holding the
grace until the leader is a zombie and 150 ms have passed (base fails 5 of 5 with a single `EPERM`
probe, the hosted failure). That the hosted VM stalled past the timer in run 38052349384 is
an inference consistent with the failing set, not an observation.

## Change

`integrations/opencode/src/runtime.js` 0.7.13: a signal-0 probe answering `EPERM` is repeated up to
40 times, each a timer at least 5 ms after the last, so a poll phase runs before each repeat; the
bound is a count, not a deadline, so a stalled loop cannot exhaust it without letting a poll phase
run. Only `ESRCH` confirms; any other answer or an `EPERM` on the last repeat still completes as `corvint-process-cleanup-unconfirmed`, and no other signal is sent. Gemini is unchanged.

## Evidence

- `V1-1116 OpenCode completion after a stalled termination turn re-probes the zombie group`: fails on
  the base 5 of 5, passes after (Darwin only; Linux delivers SIGKILL to a zombie group).
- Codex review round 1 (FAIL, no P1): timing-dependent bound and an unproven regression path; both
  addressed by the count bound and the answer assertions.
- `V1-1116 OpenCode EPERM re-probe confirms only on ESRCH and is bounded`: injected answers.
- Still required: a hosted macos-15 run of `host-adapter-test` and `TestHostAdapterJavaScriptHosts`.

## Rollback

Revert the runtime change and the two tests; see AHI-052.
