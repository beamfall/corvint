# Tasks cancellation assertion waits for descendant reaping

PR #416's Ubuntu race shard observed `TestTasksCancellationRetiresDescendant`
fail its immediate `kill(pid, 0)` probe after cancellation. The log establishes
that the PID was still observable at that instant; it does not establish a
persistent production leak or distinguish an executing process from a zombie.
The neighboring ordinary-descendant cancellation test already allows five
seconds for asynchronous exit and reaping.

This repair applies that same five-second bound and 20 ms probe interval to the
Tasks transport assertion, retaining ESRCH as the only success condition. A
persistent descendant still fails. Deferred context cancellation also covers
an early test exit. Production process handling, wire contracts, and the
experimental qualification boundary remain unchanged. The governing existing
intent is CEX-V0-004; this test-only change does not promote the companion.

The pre-change focused Tasks tests passed on Darwin with Go 1.27.1. Selected
verification is twenty race-enabled repetitions of the Tasks transport tests
and package vet. Native Linux confirmation remains the PR CI job's obligation;
local Darwin evidence does not prove Linux reaping timing. The failed CI log and
original Corvint query/impact/affected receipts remain in the coordinator's
private evidence bundle. Affected selection retains its language-frontier
unknowns. The repository-wide gate is NOT_RUN under the scoped-issue policy.

Independent review and post-commit CEM/OCM binding are required before publishing
this repair. Rollback is reverting this test-only repair and its evidence record;
no store, runtime, or dependency migration is involved.
