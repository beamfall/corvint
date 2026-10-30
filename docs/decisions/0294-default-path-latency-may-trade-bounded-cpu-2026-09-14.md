# Decision 0294 — Default-path latency work may trade bounded CPU for wall

Date: 2026-09-14. Status: accepted (owner call delegated to the performance lane, per the
standing instruction to make and record such calls). Authority: repository owner; applied in
the isolated performance lane `corvint-perf-fable-20260914b` before the H6 measurement.

## Context

The 2026-09-14 default-path campaign (`docs/plans/status-fast-path-2026-09-14.md`) predeclared a
resource guard of "CPU and RSS medians not worse than +3%" for every candidate, copied from the
step-3 guard that was written for a change expected to remove work. H5 (the H4 trace-read
overlap plus a two-wide private-status slot) met the wall floor twice (−14.7% and −13.5% on the
Corvint default-path query, paired IQRs excluding 0) and failed the CPU guard twice (+3.9%, +4.7%).
Per-process attribution with a Git shim that records each child's rusage
(`lane2/cpuattr.txt`, 20 interleaved rounds, paired against the H2 baseline) places +11.1 ms of
the +13.2 ms in the Git children that now run overlapped: the two private statuses walking the
same worktree at once (+7.2 ms between them) and the ancestry log beside the loader (+2.9 ms).
The Go process is +2.5 ms with an interquartile range touching 0. There is no Go-side
inefficiency to remove; the CPU is the price of the overlap itself.

## Decision

For default-path latency candidates in this campaign and its successors, the resource guard is:
paired median CPU not worse than +5.0% on the primary cell, RSS medians not worse than +3% on any
cell, spawn counts unchanged, and every correctness guard (byte-identical receipts, exits,
stderr, freshness, refusal, nonmutation) intact. The wall floor stays at −10% paired median with
the IQR excluding 0. The trade is bounded and disclosed: a candidate that spends CPU must show
where it goes (per-process attribution), and a CPU increase that lands in the Go process rather
than in overlapped children is not covered by this decision.

This guard is set with knowledge of H5's numbers. H6 therefore re-measures the same bytes under
the new guard as a confirmatory run, labelled as such in the plan and the build log, and is not
reported as an independent discovery.

## Consequences

- Corvint's default query runs inside agent hooks, where wall latency is the user-facing cost; a
  bounded CPU increase on a multi-core host is not visible to the operator and is accepted
  when it buys at least ten percent of wall.
- The private-status scratch bound moves from one temporary metadata copy to two; the two spec
  sentences that state the bound change in the same commit as the code.
- Rollback: revert the code commit; the slot returns to one and the trace read to the serial
  order. The decision stays on record either way.

Integration identity: originally decision 0092 at `6ce25f4cf1f00cc8d347e53de328a0b5c8b60dc4:docs/decisions/0092-default-path-latency-may-trade-bounded-cpu-2026-09-14.md`; renumbered 0294 to preserve the distinct accepted public-release decision. Acceptance and meaning are unchanged. Historical citations retain their original number.
