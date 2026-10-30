# Batch-precheck names host-resource failures — ASD-V0-054

Friction from the V1-3168 lane closeout: with the shared `/tmp` tmpfs full, `batch-drift` failed a
double-build test with `no space left on device`. The `--base` comparison's `sort` spilled into the
same directory (`comm: file 1 is not in sorted order`), so the step printed `introduced: not
present at BASE`, and `capability-catalog-check` printed `FLAKY` for the same cause. Rerun with
`TMPDIR` on a disk with room, all nine steps passed.

## Decision

A failure whose output names an exhausted host resource still fails, but is reported
`host-resource: LINE` with the `TMPDIR` to free, and is not rerun at the base, which would exhaust
the same resource. A `FLAKY` pass names the resource its first attempt hit. A `TMPDIR` under 4 GiB
free warns before any step runs. No step is passed, warned or retried more than before.

## Friction reviewed and left unchanged

- A plan for a map of at most 256 hunks must name every unknown hunk (`DCW-V0-019`), so a
  "staged" plan there names the hunks still unknown. This is the evidence contract, not friction.
- `corvint test --base` selects 286 units when `internal/cem/cemcode` or `internal/cem/wire`
  changes: those packages are imported that widely, so the selection is correct. The lane ran
  focused `-run` sets under the owner's standing preference.
- Parallel lanes can allocate the same next requirement ID. That needs a lane-level allocation
  in the orchestrator, outside this repository's gates.

## Evidence

`make batch-precheck-test` section 9 (fails on the parent commit at the low-tmp-space assertion;
passes with the change, run with `TMPDIR` on disk because `/tmp` was 98% full).
