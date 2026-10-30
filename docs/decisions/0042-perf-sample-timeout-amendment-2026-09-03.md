# Decision 0042 — the perf-v0 sample timeout rises from 30 s to 120 s

Date: 2026-09-03. Status: accepted. Authority: repository owner, explicit answer to a posed choice
(2026-09-03): raise the timeout to 120 s as a dated amendment, in preference to dropping the task or
releasing with packet-5 `NOT_RUN`.

## What was measured, and why the current bound is wrong

`beamfall-impact-path-snapshot-present` has been the single remaining packet-5 blocker. The
2026-09-03 run recorded it `insufficient_evidence` with five reasons: `sample-failure`, `timeout`,
`unequal-exit-status`, `unequal-stdout` and `unstable-oracle-stdout`. Read together those look like
a badly-behaved case. They are one cause and four consequences.

The oracle's p95 on that task was **16,979 ms** against a preregistered 30,000 ms sample timeout, so
the distribution's tail crosses the bound. A killed sample writes partial stdout and a non-zero
exit, which is exactly `sample-failure`, `unequal-exit-status`, and — because a truncated sample
differs from the first retained one — `unstable-oracle-stdout`.

Reproducing the task in isolation against the same materialized corpus (pin `fa3b1e7`, `index` run
as the declared setup, oracle and candidate built from `d995231`, runner-sanitized environment)
gives an oracle of 9.6 s per sample and stdout that is **stable across eight of nine samples**. The
one differing sample is the cold first run, whose test-marker evidence rows are permuted; the
protocol's five discarded warmups absorb it before measurement begins. So the task is not
nondeterministic. It is slow, and under the load of a full 22-task run it is roughly 1.8x slower
still, which is what pushes its tail past 30 s.

## The change

`preregistration.sampleTimeoutMilliseconds` rises from `30000` to `120000`.

## Why this is not relaxing a threshold to make a gate pass

The sample timeout is not a performance threshold. The thresholds are `GPK-V0-017(a)`, `(b)` and
`(c)`, and none of them moves. The timeout exists to stop a hung process from wedging a run, and an
oracle that reliably answers in 9.6 s standalone and ~17 s under load is slow, not hung. At 30 s it
had stopped doing its stated job and started deciding measurements.

The direction of any bias is also checkable, and it does not favour the candidate. The candidate
answers this invocation in about one second, so the bound only ever binds the oracle. A timeout does
not drop the slow samples and keep the fast ones — it invalidates the whole task — so the previous
setting did not flatter the candidate's numbers either; it deleted the comparison. Raising the bound
lets the oracle's genuine slowness be measured and reported, which is the direction that makes the
candidate's advantage on this task *harder* to overstate, not easier.

This is a dated preregistration change on the same footing as decision 0037 item 1 and decision
0038: the next full run is the first under this manifest, and no earlier result is re-scored. It was
decided before the run that measures under it, not after seeing that run's numbers.

## Rollback

Restore `30000` in `conformance/perf-v0/manifest.json`. `beamfall-impact-path-snapshot-present`
returns to `insufficient_evidence` with its five reasons and packet-5 returns to `NOT_RUN`, which is
its state before this decision.
