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
`mm:ss.cc` form, whose minutes keep counting past 59, and the Linux `[dd-]hh:mm:ss` form). The
dispatcher keeps an in-memory baseline per worker, retaken at the first observation and at every
active tick. A non-ignored child is busy only when it is new since the baseline, its CPU time is
unreadable, or it used at least 1% of the wall time since the baseline. The ledger format is
unchanged (no `taskman-dispatch-state` version move), so a restart retakes the baseline.

The 1% share is fixed rather than configured. Measuring from the last activity rather than per
tick keeps Linux's one-second CPU resolution from counting a low-CPU helper busy on every bump:
after the first window the share converges to the helper's real rate.

## Evidence

`TestCALV0218_SleepingChildDoesNotBlockIdleStop` launches a host `sleep 300 & wait` and fails on
the previous `busyChild` (the leader survives) and passes with the change. Unit tests cover the
parser, the busy-child conditions and the baseline following activity. Focused dispatch tests ran
with `-race`. Live qualification against a real agent host with MCP servers is NOT_RUN.
