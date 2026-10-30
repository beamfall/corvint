# Decision 0217 — observe mode reads blocking stdin on a goroutine against its lifetime

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`corvint-native-hook-observer observe` reads hook frames from stdin. `runObserver` bounded that read
only with `SetReadDeadline`, whose error it discarded. A blocking pipe fd (for example `os.NewFile`
over `syscall.Pipe`, as an inherited shell-pipeline stdin can be) is not pollable, so the deadline
never fired and a partial frame kept the process alive past its 30-second lifetime.

Two fixes were weighed. Refusing input whose read deadline cannot be set was rejected: stdin from a
shell pipeline is the normal input for this mode, so refusal would reject the ordinary case.
Chosen: each read runs on a goroutine and is selected against a context whose deadline is the
lifetime. On expiry the mode emits the existing `observer-input-unavailable` failure and returns 1
on time. The read deadline is still set where it works, so a pollable read also ends cleanly.

Consequence stated plainly: after expiry the abandoned read stays blocked on its goroutine until
input arrives or the process exits. `runObserver` issues no further read after a failed one, and
`main` exits immediately after it returns, so the blocked read ends with process exit. An embedding
caller that keeps the process alive retains that one goroutine and fd reference.

Spec: `NPO-V0-009` in `docs/specs/native-observer-prearm-v0.md`. Evidence:
`TestObserverBlockingPipeInputHonorsLifetime`.
