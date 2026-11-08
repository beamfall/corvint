# Go line selection: line narrowing disabled for 1.0

Tickets: V1-4364, V1-4366. Lane base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`; reviewed head
`2565be1a74a5f91f8adc4a142131ebf29918310a`. Observed 2026-11-04 on the WSL lane host (go1.27.3
linux/amd64). Delivery remains experimental; this entry records no promotion or qualification.

## Decision

The capped review round of this lane found two soundness gaps that no test in this lane proves
closed. The orchestrator decided, under the delegated 1.0 authority, to disable line-level
narrowing for 1.0 rather than run a further fix round. Every path that would narrow by line coverage
now runs the static plan and says why:

- `corvint affected --line-index` and `corvint test --line-index`: a selection that would be
  NARROWED after qualification admission is SHADOW `line-narrowing-disabled`, detail `inherited
  environment not bound (1.0)` (`linecov.Disable`, `GLS-V0-011`). The proposed units stay in the
  receipt as evidence; `corvint test` applies none of them.
- `linecov ci -selection`: a NARROWED selection, such as one an older selector on main wrote, is
  recorded SHADOW `line-narrowing-disabled` and leaves both shard lists unchanged.
- `linecov select` and `linecov ci -index`: every decided package runs whole and the plan's reason
  is `line-narrowing-disabled: inherited environment not bound (1.0)`.

`GLS-V0-020`, `GLS-V0-040` and `GLS-V0-042` are deferred with their IDs kept; the spec's new
"Disabled for 1.0" section holds the reason, the reproductions and the return condition.
Collection, the `/4` format, the proposed selection, qualification, the FALLBACK refusals, the miss
detector and the kill switch stay implemented and tested. `linecov qualify` still drives the
selector directly (`Index.Plan`), so mutation qualification keeps working.

## Findings that forced it

1. Inherited environment: the runner identity does not read `CI` or other application variables, so
   CI collection with `CI=true` and a local run with `CI` unset share one identity. A test that
   returns early under `CI` records no coverage of the code it otherwise checks, and a qualified
   index could exclude it locally.
2. FC: `FC=fortran-other go env -json -changed FC` prints `{}` on go1.27.3, while the same probe for
   CC prints the change, so the round-e FC refusal never fires.

## CI

The lane's commit `e2d70685d` bound CI collection and selection to the shards' `-timeout 50m`. With
narrowing disabled nothing consumes a matching identity, so it is reverted; both sides return to
the 10m default and stay consistent with each other. The lane's earlier workflow commits remain:
they plan and record the selection, run the miss detector and kill switch, and keep the
`AFP-V0-170` record job's FULL requirement. None of them can narrow a shard now, because
`linecov ci -selection` never sees a NARROWED selection.

## Evidence

- Fail before, pass after: `TestLineNarrowingDisabledEvenWhenQualified` (cmd/corvint, with the
  qualification check replaced by one that admits) failed with `NARROWED [calc=whole user=whole]`;
  `TestCISelectionNarrowingDisabledKeepsLists` failed with the excluded and sliced rewrite;
  `TestCINarrowingDisabledKeepsWholePackages` failed with `packages = "example.com/m/p\n"` and
  `./q` excluded. All three pass after the change.
- Focused packages pass: `internal/linecov`, `internal/sessiontest`, `tools/linecov`,
  `tools/ci-reuse-plan` and the `cmd/corvint` line-selection subset.
- Not run: the full repository gate (owner standing preference for scoped work).
