# Decision 0283 — A live-test pass requires observed run evidence

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-13).

The session projected the last terminal action named for each `(Package, Test)` pair. A malformed or
truncated stream could therefore name a test only in an `output`, `pass`, `fail`, or `skip` event;
the terminal alone became an execution outcome even though the session had never observed that test
start. In particular, a lone `pass` produced a ghost per-test `PASSED` projection.

The call: a per-test outcome requires an observed `run` action for the same exact package and test
name. Any named events without that start evidence preserve the row's first-seen position but leave
its action `none` and execution `UNSUPPORTED/no-execution-input`; a terminal `pass` alone never
projects `PASSED`. After `run`, the existing terminal and conflicting-terminal rules are unchanged.

Rejected: trusting terminal actions because a well-formed Go 1.27 stream normally emits `run`
first. These projections consume retained subprocess output directly, and missing evidence must
abstain even when the outer process exits zero.

Consequence: no wire shape or requirement ID changes. `GLTP-V0-052` states the run-evidence rule.
`TestPassWithoutRunAbstains` drives `session.Run` with an exit-zero stream containing a test-level
`pass` but no `run` and requires action `none` with an unsupported execution projection.

Rollback: revert the commit. That restores terminal-only pass projection and the ghost-pass defect.
