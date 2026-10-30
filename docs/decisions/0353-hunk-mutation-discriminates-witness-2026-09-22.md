# Decision 0353 — A bounded hunk-mutation `discriminates` witness on the CEM

Date: 2026-09-22. Status: accepted; experimental delivery (ticket V1-0086). Extends decision 0347's
`cem/0.3` hunk witnesses and reuses the FPK-V0-028 mutation runner; it changes no `protocol/**`
schema and no `prove` output.

## Context

Ticket V1-0086 asks for incremental hunk mutation wired into a `discriminates` witness. `prove
--mutate` (FPK-V0-028, decision 0063 item 5) already exports a revision into a sandbox, plans
four deterministic operators over Go function bodies, and runs the cited test per mutant, but it
is a prove-only prototype: it is not connected to the change frontier, it stops at the first kill,
and it says nothing about which mutants a test let live. A coverage witness (decision 0347) says a
test reached a hunk; it cannot say whether the test notices when the hunk changes. Prior art:
SWE-ABS (arXiv 2603.00520) and Trail of Bits `mewt` both restrict mutation to the diff to keep
the cost bounded.

## Decision

- One explicit action, `corvint cem discriminate --map MAP --target REV [--max-hunks N]
  [--max-mutants N] [--wall-time DURATION] [--output PATH]`, mutates only the map's changed Go
  hunks against the `_test.go` files their `test-claim` basis cites (`TCQ-V0-055`, `TCQ-V0-057`).
  Bounds are the hunk count, the per-hunk mutant count, and one wall-time budget over the export
  and every run; defaults 8, 8, `10m`.
- The run is pinned to the resolved `--target` object ID, which must reproduce the map's
  `patchSha256` from the map base, and to `selectionSha256`, the digest of the sorted selected
  test paths. Both are derived by Corvint, never operator-supplied.
- The witness is one optional closed-key `cem/0.3` hunk member `discriminates` with `state`
  `discriminates`, `survived`, or `not-run`, killed/survived counts, every surviving mutant
  described by operator, line, and text, and the bounds the run enforced (`TCQ-V0-056`).
  `cem/0.1` and `cem/0.2` reject it as `unknown-field`. Every hunk gets a member; a hunk the
  bounds or the host leave unjudged is `not-run` with a reason, never absent by accident.
- `cem report` shows a `survived` witness as the downgrade reason `mutants-survived` with each
  survivor listed, a `discriminates` witness as a kill count, and a `not-run` witness with its
  reason (`TCQ-V0-058`). The downgrade is rendering only: no disposition, count, worklist, envelope,
  or exit status changes, so a surviving mutant never fails the build and is never silent.
- The runner is reused, not duplicated: `mutate.Open` and `Export.Judge` with `Complete` set, plus
  one additive export, `Report.Survivors` (`[]Survivor{Operator, Line, Start, End}`), filled by
  both the single-claim and the grouped judge. `prove --mutate` reads none of it, so its output is
  unchanged; the FPK-V0-028 paragraph records the shared runner and keeps its own experimental
  label and 19-of-20 replay gate `NOT_RUN`.
- Measured cost is recorded in the TCQ spec from this host: 5.7 s for one bounded run of one
  hunk and four mutants on a quiet host, 30–33 s under seven concurrent agent workloads.

## Alternatives weighed

- Run mutation inside `cem status`/`verify`: rejected; read commands must not run tests or
  export trees, and the cost is bounded only when the operator asks for it.
- Grouped first-kill judging (`Export.JudgeGroup`): rejected for the witness because it stops at
  the first kill and cannot count survivors; kept for `prove`.
- A second, hunk-aware mutator: rejected; one deterministic operator table and one sandbox keep
  a witness replayable from the recorded revision, operators, and lines.
- Failing `verify` on survivors: rejected by the ticket; a survivor is a visible downgrade, not a
  policy decision this slice owns.

## Consequences

- Rollback: remove `cem discriminate`, the `discriminates` member from the `cem/0.3` validator,
  and the mutation note in the report; every map written without the action is unchanged, and a
  map carrying the member fails closed as `unknown-field`. `Report.Survivors` can stay or go
  without touching `prove` output.
- Not delivered: non-Go hunks, mutation of test files or unchanged lines, mutation-score
  thresholds, new operators, and any promotion claim for the FPK-V0-028 replay cohort.
