## 2026-09-26 V1-0371 AHI-009 AHI-012: OpenCode and Gemini keep descendant cleanup alive after the leader closes

Both JavaScript adapters spawn Corvint as the leader of its own process group, and both cancelled
the group's pending SIGKILL when the leader's `close` arrived: OpenCode's `finish` cleared its
25 ms `forceTimer`, and Gemini's close handler cleared its 10 ms `hardKill`. `close` only means the
leader exited and its pipes drained, so a same-group descendant that ignored SIGTERM and closed its
inherited stdio survived a timeout or cancellation, and on a normal exit neither adapter signalled
the group at all.

Chosen: keep the existing TERM, then KILL escalation and add one group SIGKILL at completion, under
the cleanup rule `docs/specs/go-only-cutover-v0.md` already accepts for Gemini. Only a delivered
signal, `ESRCH`, or `EPERM` after the leader's observed exit confirms cleanup; any other answer
completes as `corvint-process-cleanup-unconfirmed`. OpenCode does this in `finish`, so the close,
reap-timer and error paths all kill before they resolve. Gemini does it unconditionally in its close
handler, where it previously ran only for an undecided `EPERM`. A confirmed timeout keeps its code,
`deadlineMs` and FALLBACK notice (`AHI-012`); the requirement text is in `AHI-009`.

Set aside: polling the group until `ESRCH`, as `internal/procgroup` and the Pi runtime do. SIGKILL
cannot be caught or ignored, so its delivery is the confirmation, and on this host a SIGKILLed
group took p50 4.6 ms, p95 692 ms and at most 940 ms to be reaped at load averages 218 to 293. A
disappearance poll of that length does not fit Gemini's 1,000 ms declared kill.

The native test fixture gains `orphan-hang` and `orphan-valid` modes whose descendant ignores
SIGTERM, closes its stdio, then publishes its PID. The regression tests in
`integrations/host-adapters.test.mjs` cover timeout, cancellation, normal exit, and an injected
failing group SIGKILL for both hosts. Gemini advances to 0.2.5 and OpenCode to 0.2.9 (`AHI-020`).

Evidence, at load averages 400 to 570: the fixture's own start exceeded the 4,000 ms test
deadlines there, so the four tests also ran once from a scratch copy with a 25,000 ms Gemini kill,
a 10,000 ms OpenCode query deadline and a 30 s `orphan-hang` sleep. With the fix all four passed.
With only the two adapter files stashed, all four failed with `owned same-group descendant
survived the leader`: at the timeout for both hosts, and after the normal exit for both hosts.
