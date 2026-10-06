# V1-0864 deterministic journal-audit refusal order

Date: 2026-10-06. Ticket: V1-0864 (P2 bug). Spec: `docs/specs/corvint-tasks-agent-leases-v0.md`,
CAL-V0-114.

## Intent

With two or more divergent projection files, `Reader.projections` in
`internal/tasks/journal/records.go` ranged over the `canonical` map and returned the first mismatch,
so identical stores refused with a random path and, when intent and state both diverged, a random
code (`INTENT_DIVERGED` or `JOURNAL_FORKED`).

## Decision

Rule: among faults of the same kind the audit names the byte-smallest store-relative path. A small
generic `sortedPaths` helper returns map keys in byte order; it now drives the four walks in the
package that refuse while ranging over a map:

- `Reader.projections` (canonical projection comparison; the reported defect);
- `Reader.strays` (projections with no retained afterimage; same pattern, same file);
- the checkpoint-resumed projection walk in `records.go` (its refusals fall back to the complete
  audit, so this only makes the internal path and the selected-bytes accumulation deterministic);
- the unassigned stage slot check in `stage_read.go`.

The order between kinds of checks is unchanged (projections still precede strays). Remaining map
walks in the package are order-independent (boolean or set results: `sameObservation`, the
uninitialized remnant scan, `observedSelection`). Independent review round 1 (Codex) noted that
`nativeRead.close` joined close failures in map order, so the diagnostic text (not its code or
path) varied; it is now walked in path order too and pinned by
`TestCALV0114_CloseFailuresJoinInPathOrder`, which failed before (`"c\ne\nb\nd\na"`).

## Evidence

- Before the fix, `go test -count=20 -run CALV0114 ./internal/tasks/journal/` failed all 80
  repeated subtests (4 cases x 20 runs), each naming a different path than the byte-smallest one
  within its 32 repeated audits (for example `reservations.json` instead of `intent/tickets/A.json`,
  `intent/tickets/D.json` instead of `C.json`, `staging/a03` or `a05` instead of `a01`).
- After the fix the same command passes; `go test -count=1 ./internal/tasks/journal/...` passes;
  `go vet` clean.
- `TestCALV0114_SingleDivergenceAndSuccessUnchanged` pins the single-fault refusal and the success
  path.

## Review

Codex round 1: no P0-P2; two P3. (1) The spec overstated that a divergent projection always
precedes a stray; the writer audits check state before intent in separate passes. The failure-mode
text now says the precedence holds within one pass. (2) The close-order diagnostic above, fixed.

## Limits

No live-store qualification is needed: the change only orders in-memory walks. `make gate` NOT_RUN
(owner preference for scoped issue work).
