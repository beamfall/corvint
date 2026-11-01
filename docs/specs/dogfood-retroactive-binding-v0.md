# Dogfood retroactive binding V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `DOGFOOD-BIND`
Intent status: accepted; moved unchanged from the docs/DOGFOOD.md retroactive-binding contract (2026-11-01)
Delivery status: implemented
Authoritative inputs: `../DOGFOOD.md` "Bind a landed range after the fact" (the procedure and the
`DOGFOOD-011` and `DOGFOOD-012` product rules it serves), decision 0165, and
`daily-change-evidence-workflow-v0.md` `DCW-V0-824` to `DCW-V0-831` (which refine this contract).

## Agent digest
- Claim: A landed commit range is bound to one cited CEM after the fact by a retroactive binding commit, never by rewriting history.
- Status: accepted; moved unchanged from the docs/DOGFOOD.md retroactive-binding contract (2026-11-01) / implemented
- Exists: `script/dogfood-bind-range.sh`, `tools/dogfood-guard-hunks`, and `script/dogfood-bind-range_test.sh` (`make dogfood-bind-range-test`).
- Blocked on: nothing; `DOGFOOD-BIND-003`'s plan-map mismatch sentence stays proposed as recorded inline.
- Read next: Requirements; Failure modes; Traceability.

## User and job

An operator whose change already landed without a CEM (`BASE_SHA..TARGET_SHA`) binds it so
`dogfood-check` reports the range as retroactively bound instead of unbound. The procedure, the
console commands and the reason the normal flow cannot do this are in `../DOGFOOD.md`.

## Verified current state

The requirements below were defined in `docs/DOGFOOD.md`, which `make spec-requirements` does not
read, so no `DOGFOOD-BIND` ID appeared in `REQUIREMENTS.tsv` (V1-3386). Their text moved here
unchanged; `../DOGFOOD.md` keeps the procedure and a pointer.

## Requirements

- `DOGFOOD-BIND-001`: the script refuses with exit 2 and `dogfood-bind-range: REFUSE REASON`, before
  any CEM work, when a revision does not resolve (`base-unavailable`, `target-unavailable`), the range
  is empty (`empty-range`), BASE is not an ancestor of TARGET (`base-not-ancestor-of-target`),
  TARGET is not an ancestor of `HEAD` (`target-not-landed`), the range exceeds 256 commits
  (`range-exceeds-256-commits`), `DOGFOOD_CITATIONS` names no file
  (`citation-plan-unavailable`), or `DOGFOOD_UNKNOWN` names no file (`unknown-plan-unavailable`). `CORVINT_BIN` follows the coordinator's version rule
  (`corvint-version-mismatch expected=V`); without it the current tree is built
  (`current-tree-corvint-build-failed`). The 256-commit refusal is not a promise that one piece fits:
  size each piece for both the 256-row citation-plan limit
  (`script/dogfood-bind-range.sh:149-156@3b3a27f5`) and the verifier's 1,024-logical-Git-operation
  budget (`internal/cem/gitrun/gitrun.go:32-38@a875e210`). For the current object-identity and
  tree-walk checks, use roughly 5.7 operations per changed path plus 10 per evidence record as a
  planning estimate; path depth, shared objects, and request-memo hits change the exact count, so
  reduce the piece before either bound rather than relying on the commit count.
  When a first-parent segment's TARGET precedes the merge that introduced the reported gap BASE onto
  that branch, the gap BASE is not the segment TARGET's ancestor. Use
  `git merge-base GAP_BASE SEGMENT_TARGET` as the segment BASE. The later piece ending at the merge
  then binds the merged-in `MERGE_BASE..GAP_BASE` changes as well as its first-parent changes; size
  and cite the complete canonical patch rather than omitting those foreign commits.
- `DOGFOOD-BIND-002`: preparation and citation run in a private, never-checked-out Git worktree at
  TARGET under `<git-dir>/corvint/`. The main worktree, its tracked sidecar, and
  `.corvint/dogfood-report.json` are never written, and the private worktree is removed on every exit,
  including interruption (exit 129, 130, or 143).
- `DOGFOOD-BIND-003`: the citation plan has the coordinator's row format and limits and is written
  against this range's uncited prepared map, published as
  `<git-dir>/corvint/bind-range.B12..T12.cem.json`. Without a plan the script reports
  `cem-cite NOT_PRODUCED citation-plan-not-provided`. A range needing more than one plan's rows is
  split into contiguous sub-ranges; nothing is split or cited automatically. (proposed 2026-09-25,
  V1-0228, not accepted) As in the bind loop (`DCW-V0-019`), a nonempty plan written for another map
  fails `cem-cite citation-plan-map-mismatch` before any cite: a numeric selector that is not a
  canonical ordinal or exceeds the hunk count, or an unknown hunk named by neither the citation nor
  the `DOGFOOD_UNKNOWN` plan, by ordinal or ID.
- `DOGFOOD-BIND-004`: the binding commit has TARGET as its only parent, TARGET's tree with only
  `.corvint/change.cem.json` replaced by the cited map, and the trailer
  `Corvint-Dogfood-Binding: retroactive`. Its canonical patch is therefore exactly BASE..TARGET.
  `cem status --expected-base BASE --target BINDING --max-unknown N --max-mechanical 0` must pass,
  where N is the row count of the `DOGFOOD-BIND-007` unknown plan (0 without one);
  its stdout is published as `<git-dir>/corvint/bind-range.B12..T12.cem-status.json`.
- `DOGFOOD-BIND-005`: a retroactive binding never claims pre-change context or a local outcome. A
  passing run reports `NOT_PRODUCED retroactive-binding` for `prechange-query`, `prechange-impact`,
  `local-outcome`, and `ocm-aggregate`, then prints
  `dogfood-bind-range: PASS retroactive binding=SHA range=BASE..TARGET` and the `next:` merge
  command. The script creates no ref and performs no merge; the operator decides to land it.
- `DOGFOOD-BIND-006`: merging the binding with the `ours` strategy changes no tracked content, so
  the current sidecar and every later change's base are unaffected, and the range becomes reachable
  as a retroactive binding for `dogfood-check`.
- `DOGFOOD-BIND-007`: a hunk whose intent no stable base content records (for example an intent
  added or removed inside the range) is not cited. A fixing commit that removes its backlog entry
  MUST cite a stable owning spec requirement or accepted decision instead. The producer reads a
  requested span from `baseRevision` (`internal/cem/workflow/commands.go:489-506@c79ac767`), but
  both its stability precheck and canonical target-drift verification reject that removed span
  (`internal/cem/verify/verify.go:428-488@166fa7f2`). A backlog entry that was the only recorded
  intent therefore binds as `NOT_PRODUCED`, never as an
  invented citation, with an unknown `no-evidence` row. Its detail is the
  `removed-intent.<blob OID>.<start>-<end>` pin that the refused `cem cite` names (`CEM-CB-005`,
  decision 0165); the script accepts that detail only when the OID is a regular blob in BASE's tree
  and the byte span is nonempty and inside it, else the plan is malformed. The pin locates the
  removed intent in immutable base content; it is not evidence, and the hunk stays `unknown`.
  Intent with no base content at all keeps an `intent-unfiled-...-no-base-clause` detail.
- `DOGFOOD-BIND-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1487) after
  a passing `cem status`, the script prints the guarding-tests `NOTE` of `DCW-V0-069` for the
  bound map, `dogfood-bind-range: NOTE guarding-tests NOT_PRODUCED guarding-tests-not-run hunks=N:`,
  counting with the same rule through `tools/dogfood-guard-hunks` (`CORVINT_GUARD_HUNKS_BIN` names a
  built helper; otherwise it is built from the current tree). The command names the binding as
  `--target`, to run from a worktree checked out at it. A map with no such hunk prints no note; a
  count the helper cannot produce prints `NOTE guarding-tests NOT_PRODUCED
  guarding-tests-count-unavailable`. Neither fails the binding, and the search never runs here.
  `DOGFOOD_UNKNOWN` names a plan of distinct
  `HUNK<TAB>REASON<TAB>DETAIL` rows, with the citation plan's limits, a registered `cem mark`
  unknown reason, and a lower-case `[a-z0-9.-]` detail token. After citation each row is marked
  `unknown`, the script prints `dogfood-bind-range: NOTE cem-mark NOT_PRODUCED hunk=H reason=R
  detail=D`, and the binding commit message carries the same `NOT_PRODUCED` line per row before its
  trailer. A malformed plan fails as `cem-mark invalid-unknown-plan`.

## Non-goals

No automatic splitting or citation, no ref creation, no merge, and no rewrite of landed history.
A retroactive binding never claims pre-change context or a local outcome.

## Failure modes

Failure modes: a failing prepare, citation, binding-commit, or status step exits 1 with
`dogfood-bind-range: FAIL STEP REASON` (`cem-prepare`, `cem-cite` including
`invalid-citation-plan`, `cem-mark` including `invalid-unknown-plan`, `binding-commit-failed`, `cem-status`, or `cem-policy` after printing the
bounded policy summary of `DCW-V0-825`) and prints no `PASS` or `next:` line. A failure after the binding commit is written
leaves only an unreferenced commit object that `git gc` may prune. A citation that no longer
survives the change, or an uncited hunk absent from the unknown plan, fails status rather than
being marked.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `DOGFOOD-BIND-001` | `script/dogfood-bind-range.sh` | `script/dogfood-bind-range_test.sh` empty, reversed and not-landed refusals |
| `DOGFOOD-BIND-002` | `script/dogfood-bind-range.sh` | `script/dogfood-bind-range_test.sh` failure leaves no private worktree; pass never writes the main worktree |
| `DOGFOOD-BIND-003` | `script/dogfood-bind-range.sh` | `script/dogfood-bind-range_test.sh` plan-map mismatch and `bytes:` row cases |
| `DOGFOOD-BIND-004` | `script/dogfood-bind-range.sh` | `script/dogfood-bind-range_test.sh` binding tree, single parent and trailer |
| `DOGFOOD-BIND-005` | `script/dogfood-bind-range.sh` | `script/dogfood-bind-range_test.sh` `NOT_PRODUCED retroactive-binding` notes and PASS line |
| `DOGFOOD-BIND-006` | `git merge -s ours` (operator) | `script/dogfood-bind-range_test.sh` merged range reported as `retroactive-bound-commits` |
| `DOGFOOD-BIND-007` | `script/dogfood-bind-range.sh` | `script/dogfood-bind-range_test.sh` unknown plan and removed-intent pin cases |
| `DOGFOOD-BIND-008` | `tools/dogfood-guard-hunks`, `internal/dogfoodflow/change.go` | `script/dogfood-bind-range_test.sh` guarding-tests NOTE cases |

## Rollback

Rollback: before the merge, drop the printed binding (no ref names it) and the private
`bind-range.*` evidence. After the merge the binding stays in landed history, and a
`git revert -m 1` changes no content, so the range stays reported as retroactively bound; withdraw
the claim by recording the correction in the change review, since rewriting landed history is out
of scope. Reverting `script/dogfood-bind-range.sh` and the `dogfood-check.sh` note restores the
earlier report, in which such a range is `unbound` again.
Reverting this spec's move restores the definitions to `../DOGFOOD.md`, where no index sees them.
