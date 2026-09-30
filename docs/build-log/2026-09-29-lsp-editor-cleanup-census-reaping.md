# Cleanup census lineage lost by premature reaping

Independent review of frozen `5500cbf7` reproduced HIGH false retirement: census saw the direct
owner and an unscoped child, then `poll()` reaped the owner before lineage seeding. Later the
live orphan `(Code)` was omitted, and cleanup falsely reported retirement. The cited original
report/reproduction and local before-fix output are preserved privately; this candidate was
CHANGES REQUIRED, not reviewed PASS. No actual client runs used it.

The bounded review repair records cached `Popen.returncode is None` before census, under the
single-thread/no-other-reaper invariant. This unreaped direct child pins its PID even if already
exited. Validate its census row, seed it and expand observed descendants before any poll/wait/
communicate. Missing/malformed expected identity stays UNKNOWN. Already-reaped numeric PIDs
never grant lineage. Immediate unreaped-only signal guards remain separate; sampled rows
authorize no indirect signal. Setup/direct/observation budgets stay1+3+3+5 seconds plus overhead.

Focused fixtures cover exit before/during census, surviving orphan hold, actual disappearance
pass, missing/malformed expected row UNKNOWN and already-reaped unrelated PID reuse exclusion.
Existing zero-indirect-signal, deadline, late-child, held-pipe and actual outer INT/TERM checks
remain required. Original VSCode failed raw and UNKNOWN cause/state/latency remain unchanged.
Fresh editor outcomes remain NOT_RUN pending independent re-review and root binding. All tuples
remain UNQUALIFIED. Rollback restores the previous retained candidate without rewriting evidence.
No server edits, installs, full gate, native Tasks writes, publication or nested agents occurred.
