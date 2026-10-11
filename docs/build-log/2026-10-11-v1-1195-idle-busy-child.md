# V1-1195 busy child for the dispatcher idle timeout (issue 793)

Date: 2026-10-11. Base: `55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821`.
Requirement: `CAL-V0-218`, proposed, pending owner acceptance (V1-1195). No decision file.

## Finding

`busyChild` counted every non-leader member whose command was not in `idleIgnore` as busy because
it existed. Agent hosts keep an agent server child and MCP servers for the whole session, so every
tick refreshed `LastActive` and the CAL-V0-056 IDLE branch was unreachable. The issue reports no
IDLE stop in about 670 stops and a hung session (log silent 25+ minutes, all members sleeping)
running to `wallSeconds`.

## Change

The process table read adds the cumulative CPU time column (`time=`, parsed in both the Darwin
`mm:ss.cc` form, whose minutes keep counting past 59, and the Linux `[dd-]hh:mm:ss` form). Linux
prints whole seconds, so for the children that count, the dispatcher reads utime+stime from
`/proc/<pid>/stat` instead (USER_HZ 100, 10 ms; review round 2: with whole seconds a child at 2% CPU
under a 30 second idle timeout could show no advance and be stopped). The
dispatcher keeps an in-memory baseline per worker, retaken at the first observation and at every
active tick, keyed by PID and start identity (Linux identities omit the PID). A non-ignored child
is busy only when it is new since the baseline, its CPU time is unreadable or below the baseline,
or it used at least 1% of the wall time since the baseline. The ledger format is unchanged (no
`taskman-dispatch-state` version move); after a restart every child is new, so an adopted worker
gets a full idle timeout before it can be stopped IDLE (review round 1).

`supervise` now observes every worker's tree and activity in one pass and stops workers in a
second pass (review round 4): before, a stop blocking for its kill grace delayed a later worker's
`/proc` sample while its baseline kept the tick's start time, understating its CPU share.

The 1% share is fixed rather than configured. Measuring from the last activity rather than per
tick keeps counter rounding from counting a low-CPU helper busy on every tick: after the first
window the share converges to the helper's real rate.

## Evidence

`TestCALV0218_SleepingChildDoesNotBlockIdleStop` launches a host `sleep 300 & wait` and fails on
the previous `busyChild` (the leader survives) and passes with the change. Unit tests cover the
parser, the busy-child conditions and the baseline following activity. Focused dispatch tests ran
with `-race` on darwin/arm64; the Linux build and tests were compiled (`GOOS=linux go vet`) but
not executed here, and `TestCALV0218_ParseProcStatCPU` covers the Linux stat parser on the host. Live qualification against a real agent host with MCP servers is NOT_RUN.
