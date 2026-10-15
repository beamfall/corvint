# 2026-10-15 V1-1415: reusable command gate results

Lane V1-1415. New requirements `CAL-V0-240`..`CAL-V0-243` in
`docs/specs/corvint-tasks-agent-leases-v0.md`, accepted under the owner's standing auto-accept of
2026-10-11. Base `6457b86bcd4b9048cfc93645b0250c4372115d72` (origin/main). Owner input, 2026-10-15:
four tickets (V1-0252..0255) at one candidate commit each run a two-hour `make gate`, "can't they be
combined?"

## Decisions

1. **Surface: `gate run` short-circuits.** No new verb, receipt kind, record field or format.
   TCP-00 already carries a gate's `reusable` flag and a result's `reusedFrom` digest. Before this
   change every `gate run` executed the gate and wrote `reusedFrom: null`. A reuse is a
   `GATE_RESULT` receipt whose record is the source result rebound to the requesting attempt.
   `complete` is unchanged: it applies the same tree, definition, binding, `PASSED` and
   `STALE_POLICY` checks to a reused result as to an executed one.
2. **Key: the definition digest, not the policy SHA-256.** The policy SHA moves on any edit,
   including edits to capacity, holds or other gates. The definition digest moves exactly when this
   gate's entry changes. It covers argv, cwd, environment names, timeout, expected exit,
   `reusable` and `required`. A result also qualifies only if its `inputsSha256` (the tree plus any
   substituted argv) and its `environmentSha256` (which declared variables are present) match the
   values this run would record. The policy revision still governs completion: the rebound record
   carries the requesting attempt's `policySha256`, and `complete` still refuses `STALE_POLICY`.
3. **Cross-ticket reuse is allowed.** The policy has no field that forbids it, and several tickets
   at one candidate tree is the case the owner asked about. A per-ticket restriction is a non-goal
   and would need a policy field.
4. **A `RETURN` source cannot arise.** `gate run` refuses external-review gates `UNSUPPORTED`, so
   only COMMAND results exist to reuse. The state check (`PASSED` only) refuses `FAILED`,
   `TIMED_OUT` and every other state.
5. **Chaining is refused.** A source must have been executed (`reusedFrom` null), so a reuse of a
   reuse is `MALFORMED`. Each reuse points at the one execution.
6. **Tree and definition checks are shared with executed results.** The rebound record carries the
   source's tree and definition digest, so the existing CAL-V0-016 checks in `planGateRun`
   (`STALE_TREE`, `GATE_STALE`) refuse a reuse at another tree or definition. A first draft
   duplicated both checks inside `reuseRefusal`. Mutation showed the duplicates were dead (removing
   them failed no test), so they were deleted. Mutating the shared checks to skip reuses fails the
   `forge-tree` and `forge-definition` cases.

## Possible owner decision

`CAL-V0-240` narrows the "then run" of the accepted `CAL-V0-016` for gates the policy declares
reusable: a qualifying earlier `PASSED` result is recorded instead of executing the gate. Every
other part of CAL-V0-016 is unchanged. The amendment section says so explicitly. If the owner
treats this as amending an accepted row, it needs an explicit decision. The change adds new rows
and does not edit CAL-V0-016's text.

## Compatibility (no format change)

On a scratch store, the released `corvint-tasks` `0.0.0-tcp01-unverified+build.523` and this build
both answered `OK` for `receipt audit` and for `attempt show` of both attempts. This held in two
states: after a reuse (both attempts `CHECKING`) and after the reusing attempt `COMPLETED` citing the
reused result. An older binary's `gate run` executes every gate. A journal without reuses folds
unchanged, because the new `GateReuseReceiptAudit` examines only results whose `reusedFrom` is set.

## Evidence

Load averages are from `uptime` on a darwin host shared with other lanes.

| Run | Result | Load |
|---|---|---|
| `go test -run 'CALV024[0-3]' ./internal/tasks/store/` (4 tests) | ok | 45-48 |
| `go test -run TestCALV0240_GateRunReportsReusedFrom ./internal/tasks/cli/` | ok | ~50 |
| store focused: `-run 'CALV00[0-2]\|Gate\|CALV024'` (78 tests) | ok, 363 s | 46-42 |
| cli focused: `-run 'ERGV0002\|Receipt\|Redo\|Audit\|Gate\|CALV0016\|CALV0017\|CALV024'` (33 tests) | ok, 214 s | 69-44 |
| `./internal/tasks/transaction/` | ok, 44 s | 69 |
| `./internal/tasks/service/` | ok, 24 s | 46 |
| `./internal/tasks/ticket/`, `./internal/tasks/scale/`, `./internal/tasks/` | ok | 37-67 |
| `./internal/tasks/authority/` | 1 load flake (below), then the failing test alone ok | 37-67, then 43 |
| full `./internal/tasks/store/` (402 tests) | 30 m timeout, with PSR/pool failures (below) | 48-58 |
| full `./internal/tasks/cli/` | 9 m cap reached, no failure before the cap | 41-65 |
| after the Codex fixes: store `-run 'Gate\|CALV024\|Redo\|Recover\|ONV0006'` (46 tests) | ok, 363 s | 41-58 |
| after the Codex fixes: store `-run 'CALV00[0-2]'` (66 tests) | ok, 527 s | 53-57 |
| after the Codex fixes: cli focused pattern above (33 tests) | ok, 65 s | 55-42 |
| after the Codex fixes: `./internal/tasks/transaction/`, cli `-run CALV024` | ok | 46-42 |

Load flakes, with no gate code on their path:

- `TestGH494PreparationAdmission/total-budget-and-phase-cancellation` failed once with a 100 ms
  prepare-lock `LOCK_TIMEOUT` and passed alone.
- In the full store run, `TestPSRDeadlineRetryMatrix`, `TestPSRPhaseEnvironmentIsolation`,
  `TestPSRPredicateAndSourceMatrix`, `TestPSRRequestConflictAndOrphan`, `TestPoolProcessDescendants`
  and `TestPoolOutputBoundRetiresGroup` failed.
  - Run alone, the two pool tests passed.
  - Running the two PSR tests alternately on this change and on BASE (load 46-59) gave fail, fail,
    pass, pass. They fail and pass on both, so the failure is not caused by this change.

### Mutations (each restored and verified with `cmp`)

| Check | Mutation | Biting test |
|---|---|---|
| model: reusable declared | drop `!def.Reusable` in `reuseRefusal` | `forge-solo` COMPLETED instead of BLOCKED UNSUPPORTED |
| model: source PASSED | drop the state case | `forge-fails` COMPLETED instead of GATE_FAILED |
| model: source still named | drop the `GateResults` containment | `forge-unnamed` COMPLETED instead of GATE_STALE |
| model: tree, for reuses | skip the shared tree check for reuses | `forge-tree` |
| model: definition, for reuses | skip the shared definition check for reuses | `forge-definition` |
| store: reusable declared | `if false && !def.Reusable` | TestCALV0241/not_reusable |
| store: PASSED | drop `g.State != "PASSED"` | TestCALV0241/failed_source |
| store: definition | `(false && g.DefinitionSha256 != definition)` | TestCALV0241/changed_definition |
| store: inputs | `(false && g.InputsSha256 != inputs)` | TestCALV0241/different_inputs |
| store: tree | drop both tree comparisons and the inputs comparison | TestCALV0241/another_tree, different_inputs |
| audit: receipt kind | `false && rc.Kind != "GATE_RESULT"` | TestCALV0243 `kind` |
| audit: definition digest | drop the `GateDefinitionSha256` comparison | TestCALV0243 `changed definition` |
| redo: reuse fold (F2) | skip the `GATE_RESULT` branch of `redoReviewBinding` | TestCALV0243_RedoBindsAPendingReuse (forged redo published) |
| audit: retained record (F3) | `posted := false && a.accepted[d]` | TestCALV0243_ReuseRenamedAfterReturningToTheTree |
| audit: undecodable reuse (F4) | `if false && claimsReuse(raw)` | TestCALV0243 `undecodable reuse` |
| replay provenance (F5) | report nil instead of `recordedReuse` | TestCALV0240 replay check |

Some checks are redundant with others:

- **Store tree comparisons and the inputs digest.** Dropping only the store's tree comparisons
  fails no test, because `inputsSha256` digests the candidate tree. Both checks are kept as
  defence in depth.
- **The audit's `!def.Reusable` and the definition digest.** The audit's `!def.Reusable` alone is
  likewise covered by the definition digest, which hashes `reusable`. That is why the audit test
  gained a `changed definition` case, so the digest comparison is tested on its own.
- **Stale generation.** It is fenced before any reuse check (`forge-stale`, `REVISION_CONFLICT`
  `FENCED`, with no result recorded).

## Independent review (one Codex round)

`codex exec -s read-only` over `6457b86..a37eec13b` gave five findings. Each was checked against
the code before disposition.

| # | Finding | Disposition |
|---|---|---|
| F1 (P1) | The locked model does not recompute the requesting attempt's expected inputs and environment digests; it trusts the store's facts. | Accepted by design. The model cannot recompute them without the worktree, and it does not verify an executed result's inputs either: the store is the trusted computer of both. The substituted argv's base commit and claim paths are fixed at claim and generation-fenced, and a definition change moves the digest the model does check. The only forged-facts path is the test hook `GateReuseForTest`. |
| F2 (P1) | `redoReviewBinding` folded only receipts that post a ticket, so redo of a pending reuse skipped the reuse audit. | Fixed: a `GATE_RESULT` receipt also runs `FoldReceiptBindings` before redo. New `TestCALV0243_RedoBindsAPendingReuse`. |
| F3 (P2) | An attempt that reused a result, executed at another tree, then returned reuses the identical record; the receipt does not post the retained blob, and the audit refused `JOURNAL_FORKED` on a valid history. | Confirmed and fixed: the audit accepts a record it already accepted earlier in the fold, and still rechecks it in full. New `TestCALV0243_ReuseRenamedAfterReturningToTheTree`. |
| F4 (P2) | A named result that does not decode was skipped even when it claimed a `reusedFrom`. | Fixed: such bytes are refused `JOURNAL_FORKED`; an undecodable result without `reusedFrom` is still left to the generic audit. New `undecodable reuse` audit case with a passing control. |
| F5 (P2) | A replayed `gate run` reported `reusedFrom: null`. | Fixed: a replay reads the original receipt's attempt post and reports that result's source. TestCALV0240 now checks it. |

The spec rows CAL-V0-240 and CAL-V0-243, new in this change, were updated to match.

## Deviations

- Two `go test` commands exceeded the 600 s tool bound and were moved to the background by the
  harness; one broad `find` was also backgrounded and was stopped.
- The lane clone was made with `--reference ... --dissociate`.
- The full store and cli packages did not finish under the load. They are reported above as
  incomplete, not as passed.

## Residuals

- Environment values are not part of the reuse key. Only hermetic gates should be declared
  reusable.
- Declaring `full-gate` reusable is the operator's policy update, not part of this change.
- The live qualification (V1-0252..0255 closing on one `make gate` run) is NOT_RUN until that
  update is made.
