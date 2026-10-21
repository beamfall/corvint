# A retry racing run retirement keeps its runs (V1-0933)

Date: 2026-10-21 (lane dispatch-supervisor). Requirement: ATR-V0-171 in
`docs/specs/corvint-tasks-attempt-runner-v0.md`, accepted under the owner's standing auto-accept
(2026-10-11). ATR-V0-154's bound changes from one terminal-record read per pass to at most two.

## Decision

The suspected race is real by construction: the ATR-V0-015 terminal read takes no store lock, and the
removal followed it, so a retry opening a new generation in between lost the attempt's earlier runs.
The spec already listed this as a known limit. Holding the store lock across a filesystem sweep was
rejected (it would couple every supervisor exit to store contention). Instead a pass hides each run
it would remove with one rename to `.retiring-<attempt>-<run>`, reads the terminal records again, and
removes only runs whose attempt is still terminal at the run's generation; the rest move back. Any
pass that meets a `.retiring-` entry moves it back, so a stopped pass loses nothing, and a concurrent
pass at worst defers a removal. The existing `.retired-` prefix keeps its meaning (removal decided).

## Evidence

- `TestATRV0171_RetryDuringRetirementKeepsItsRuns` (three subtests). With the second read replaced by
  the first (scratch break script, no stash), the revived run is removed and the failed-read case
  removes a run; both subtests fail.
- Darwin `internal/tasks/cli` run of `-run 'TestATRV00|TestATRV015|TestATRV017|Attach|Detach|Retire'`
  passes (70 s). Linux: NOT_RUN.

## Limit

A hidden run is briefly missing to attach, which refuses `MISSING_EVIDENCE` as for a retired run.
