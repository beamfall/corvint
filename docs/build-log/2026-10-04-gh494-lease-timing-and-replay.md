# Issue 494: lease phase timing, timed-out claim replay and the retry-charging boundary

Issue 494 reported claim, heartbeat and release taking 10 to 41 seconds under about ten concurrent workers. Callers timed out, and an EXPIRED retry was later charged although the caller never used an attempt. #508 bounded and ordered preparation admission (30-second LOCK_TIMEOUT). #546 added the opt-in starvation reproduction. This slice adds the four remaining asks that do not need V1-0645 (history-independent writer cost). The specification is `CAL-V0-069` and the issue 494 amendment in `docs/specs/corvint-tasks-agent-leases-v0.md`.

Decision 1, timing. `--timing` is an opt-in boolean on `claim`, `renew`, `attempt heartbeat` and `release`, and the other lease verbs refuse it MALFORMED. It is kept outside the request, so request IDs, preimages, digests and replay are unchanged. A collector carried in the command context sums every lease transaction the command runs, including a claim's reaps. It reports the writer's real phases in milliseconds:
- admission wait
- guards and orphan recovery
- snapshot read (head, inventory, audit or verified reuse)
- validation (replay lookup, model, handoff history)
- monitor teardown
- lock wait and lock hold
- journal write without sync
- fsync

Fsync is measured inside the authority session at its three sync call sites. The tests' `syncFile`/`syncDirectory` indirection is unchanged. The object is profile `taskman-lease-timing/0` in the first result item, and on ERROR it is the only member of one item. It is diagnostic only and never persisted. A separate `--json` timing mode was rejected because the envelope is already JSON. A per-call environment variable was rejected because it is invisible in help.

Decision 2, retry charging: conflict reported, no semantic change. Charging happens only in `admitted` (`internal/tasks/transaction/lease_claim.go`), and only when a prior attempt exists and was not a clean handoff:
- `LEASE_EXPIRED` failure charges EXPIRED
- other failure charges FAILED
- cancellation charges RELEASED

A claim refused or cancelled before its receipt writes nothing, so it charges nothing; the new test pins this. The reported charge comes from a different case. The claim committed after its caller gave up (or its pending receipt was redone), the unused lease expired and was reaped, and the next claim was charged EXPIRED.

Exempting that case contradicts accepted text:
- `CAL-V0-044`: "Ordinary cancellation, failure and expiry remain charged regardless of stage"
- `CAL-V0-045`: failures, cancellations and expiry remain charged
- `CAL-V0-067`: not a retry exemption

The spec is accepted by owner decision 2026-09-27, so this slice does not change it. Any exemption also needs evidence that the attempt was never used, such as no heartbeat, submit or evidence since admission; the store records none of that today. That is an owner decision.

Decision 3, recovery. `TestGH494_TimedOutClaimReplaysExactly` pins exact-request-ID recovery:
- A claim cancelled in admission leaves the tree and head byte-identical. Its retry admits generation 1 with zero retry charges.
- A committed claim replays the same attempt and generation at +1, +30 and +90 minutes. +90 is past its 60-minute lease, because replay lookup precedes reaping. The head digest and journal tree stay unchanged.
- A claim faulted after its receipt is redone by the retry and then replays with an unchanged head digest.

Decision 4, latency. The specification records the issue 545 numbers as an observation with their conditions: 7,140 receipts, ten writers, median 28 s, worst 33 s, and about 2.9 s per serialized preparation. It is not a promise or a p95 budget.

Observed while testing: a colliding claim over one expired lease runs three lease transactions (refused claim, reap, claim). `--timing` reports all of them.

Limits: focused tests only, on local Darwin. `--timing` was not run against a large store or a concurrent fleet, so no new latency evidence is claimed. Linux NOT_RUN. Repository-wide `make gate` NOT_RUN by standing owner preference. `claim --async` / `claim status` and V1-0645 remain out of scope.

Rollback removes the flag, the result member, the store collector, the session sync sum and the tests. No store, request or receipt bytes depend on them.
