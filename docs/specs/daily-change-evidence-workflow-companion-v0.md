# Daily Change-Evidence Workflow Companion V0

Owner: Russell Lewis
Date: 2026-11-02
Requirement prefix: `DWC-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: repository-owner request (V1-1467, V1-4318); `AGENTS.md` invariants 1, 2 and 8; `docs/specs/daily-change-evidence-workflow-v0.md` `DCW-V0-039`, `DCW-V0-401`.

## Agent digest
- Claim: Daily-workflow requirements beyond the 256-obligation limit of `daily-change-evidence-workflow-v0.md`, under their own prefix.
- Status: proposed/experimental
- Exists: `internal/dogfoodflow/regenerated_specs.go`, `reused_worktree.go`, `script/merge-regenerate-specs.sh` and their focused tests.
- Blocked on: Nothing; companion of the daily change-evidence workflow, whose Requirements section is at the 256-obligation limit.
- Read next: Requirements; Acceptance evidence and rollback.

## User and measurable job

This spec extends `docs/specs/daily-change-evidence-workflow-v0.md`, whose Requirements section is at the `ocm prepare` limit; new daily-workflow requirements go here. Two lanes that each add requirement rows conflict in the generated `REQUIREMENTS.tsv` even when their specs merge cleanly (PR #952, #927). The batch owner needs one merge command and a batch-check that binds the regenerated merge instead of reporting `merge-resolution-unbound`.

## Requirements

- `DWC-V0-929`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4628
  closeout blocker; refines `DCW-V0-005`) `dogfood check` MUST replace its private report
  atomically using a mode-0600 temporary file in the report's own directory, closing it before
  rename and removing it on return. It MUST NOT depend on renaming a file from Git metadata:
  a linked worktree and its Git directory may be distinct filesystem or sandbox mounts.
  A failed update MUST preserve the prior report. The adopter ignore list MUST cover
  `.corvint/.dogfood-report.*.tmp`, so interrupted staging artifacts are private scratch.
  Evidence: `TestCheckDWCV0929ReportStaysOnWorktreeMount`; live lane dogfood-check on separate
  sandbox mounts. Rollback: restore Git-directory staging; cross-mount checks fail again.

- `DWC-V0-926`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4628;
  amends `DCW-V0-161`, `DCW-V0-962`, `DCW-V0-805`, `DCW-V0-841`, `DCW-V0-803`, `DCW-V0-385`, `DCW-V0-548`, `DCW-V0-966` and `DCW-V0-967`)
  Every newly drafted `propose-citations` span MUST name the hunk's full path, its basename
  of at least 6 bytes, or define a requirement ID the hunk adds or names, using `CEM-V1-081`
  anchoring. Similar words, sibling contracts and change-wide governing specs alone MUST NOT
  justify a row. If no admissible stable BASE span exists, draft `unknown` with the reason.
  Author-reviewed rows carried by refresh retain their existing semantics. Evidence:
  `TestProposeCitationsDWCV0926OnlyAnchoredDrafts`. Rollback: remove the drafted-row anchor filter.
- `DWC-V0-927`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4628;
  amends `DCW-V0-439` and `DCW-V0-962`) A newly drafted span MUST NOT consist only of a bare
  Markdown section heading, even when it names the hunk path. Try an admissible substantive
  line instead, or keep the row `unknown`. Evidence: `TestProposeCitationsDWCV0927NoHeadings`.
  Rollback: restore heading candidates.
- `DWC-V0-928`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4628;
  refines `DCW-V0-967` and amends `DCW-V0-805`) For a spec amendment naming a governing ID,
  the draft MUST prefer its BASE ``- `ID`:`` requirement definition over changed text,
  a section heading or a guessed cross-file span. An unchanged definition within hunk
  context is eligible when the citation stability verifier admits it. A definition removed or
  repeated by the change MUST NOT be cited; retain uncertainty or another admissible anchor.
  Evidence: `TestProposeCitationsDWCV0928BaseGoverningDefinition`. Rollback: remove the
  governing-definition preference.

- `DWC-V0-001`: (experimental under the standing lane auto-accept, 2026-11-02; V1-1467; refines
  `DCW-V0-039` and `DCW-V0-401`) `dogfood batch-check` MUST accept, as derivation
  `regenerated-specs`, a merge whose ort merge of its two parents conflicts only in
  `docs/specs/REQUIREMENTS.tsv`, whose tree differs from ort's written tree only in
  `docs/specs/REQUIREMENTS.tsv`, `docs/specs/INDEX.json` and `docs/specs/README.md`, and whose
  tree (including file modes) equals the tree a private clone of ort's written tree stages after
  running the merged inputs' `script/gen-spec-requirements.sh --all` and writing its three outputs
  over the checkout. Replay MUST start from those merged inputs alone, including README prose and
  INDEX fields such as `implementation` and `dependsOn`, so candidate hand edits to preserved fields
  cannot validate themselves. Each generated path in both trees MUST be a regular file; before
  reading an output or writing its destination, replay MUST refuse symlinks and other non-regular
  files rather than follow them. The
  replay MUST run as `DCW-V0-401`'s does (a clone outside the repository reading its objects as an
  alternate, the same sanitized environment, its own process group, a 5-minute bound) and MUST NOT
  write the repository, its index or refs. A conflict in any other path, including INDEX.json and
  README.md (projections onto a kept file whose hand edits a regeneration from one side could
  drop), a hand edit beside the regenerated bytes, a merge without the generator, or a replay that
  fails or times out stays `merge-resolution-unbound`. Design: two lanes that each add requirement
  rows conflict in the generated file even when their specs merge cleanly (PR #952, #927), and the
  only repair then rewrote a pushed branch. Evidence:
  `TestBatchCheckDWCV0001RegeneratedRequirementsMerge` (derived and passing; a hand edit beside
  the regeneration is unbound; it fails before the change),
  `TestBatchCheckDWCV0001OtherConflictStaysUnbound`,
  `TestBatchCheckDWCV0001RejectsGeneratedSymlinksAndModes` (each generated path refuses a symlink
  without modifying its external target, and rejects a changed executable mode),
  `TestBatchCheckDWCV0001RejectsPreservedHandEdits` (README prose and INDEX fields stay unbound).
  Rollback: remove `regeneratedSpecs` and its
  call in `deriveMerge`; such merges are unbound again.

- `DWC-V0-002`: (experimental under the standing lane auto-accept, 2026-11-02; V1-1467)
  `script/merge-regenerate-specs.sh REV [MESSAGE]` MUST merge REV into HEAD with `git merge
  --no-ff`; when the only conflict is `docs/specs/REQUIREMENTS.tsv` it MUST keep HEAD's side, run
  `script/gen-spec-requirements.sh --write` over the merged, staged specs and commit the merge with
  `--cleanup=strip`, so `DWC-V0-001` derives it. Any other conflict MUST stay in progress with
  each path listed and a `fix:` line, exit 1; a checkout with tracked changes or a missing REV exits
  2. It is the merge step of `docs/DOGFOOD.md` batch section 2 for a branch whose only conflict is
  that file. Evidence: `make lane-scripts-test` merge-regenerate-specs group with the real
  generator. Rollback: delete the script; such a conflict again needs the rebase recovery.
- `DWC-V0-003`: (experimental under the standing lane auto-accept (2026-11-02); V1-4318; amends
  `DCW-V0-400`, `DCW-V0-406`, `DCW-V0-810` and `DCW-V0-844`) The check-only `dogfood
  inputs-check` pre-flight MUST NOT refuse `reused-worktree-state` for a stranded trace whose
  `migrate-traces --dry-run` plan holds only stranded traces: it MUST print one `NOTE stranded
  trace` line naming the revision and the plan digest that `dogfood finish-lane` applies itself
  (`DCW-V0-406`), and continue, as it already does for stale sidecars alone (`DCW-V0-810`). So
  resetting a reused rebind worktree and rerunning the same step needs no manual sidecar move and
  no manual `migrate-traces --apply`. A plan that also rekeys a reachable tree-keyed legacy trace
  still refuses in both the pre-flight and a real run, and that refusal, together with any other
  `reused-worktree-state` refusal, MUST print exactly one `fix:` line that pastes as one POSIX
  command: the sidecar `mkdir -p`/`mv` and the `migrate-traces --apply --plan-digest` joined by
  `&&`, still naming the reset case and its recovery (`DCW-V0-844`) and the legacy rekey count.
  The `DCW-V0-400`, `DCW-V0-406` and `DCW-V0-844` texts that say a check-only pre-flight still
  refuses a stranded trace are amended accordingly. Design: accepting a reset to an ancestor of
  the recorded state was rejected, since the rerun merges a new base and HEAD is then a new
  merge commit, not an ancestor of the stranded seal commits. Non-goals: applying the migration
  in the pre-flight (it stays check-only), deleting any trace or sidecar. Failure modes: the
  note path lets the pre-flight run its steps and write `.corvint/dogfood-report.json`, as any
  passing pre-flight does; a trace store that cannot be evaluated is left to the steps that
  already report it. Evidence: `TestDCWV0844BindResetRerunNamesAndRetiresTheResetCase` (bind,
  `git reset --hard`, rerun: the pre-flight notes the stranded trace and runs its steps, the real
  run retires it), `TestDWCV0003PreflightRefusalPrintsOneCommand` (stale sidecar plus a stranded
  and a legacy trace: one refusal, one `fix:` line, and its move part runs as printed) and
  `TestFinishLaneDCWV0400InputsCheckRefusesAStrandedTrace` (a plan with a legacy rekey still
  refuses); all three fail on the BASE `reused_worktree.go`. Rollback: restore the
  `len(stranded) == 0` pre-flight condition and the separate fix lines in `reused_worktree.go`.

## Failure modes

| Condition | Required observation |
|---|---|
| Conflict in any other path, including INDEX.json and README.md | `merge-resolution-unbound`; the script leaves the merge in progress with a `fix:` line, exit 1 |
| Hand edit beside the regenerated bytes, or preserved-field edits | `merge-resolution-unbound` |
| Generated path is a symlink or non-regular file, or a mode changed | Replay refuses without following it |
| Replay fails or exceeds 5 minutes | `merge-resolution-unbound` |
| Tracked changes or missing REV | Script exits 2 |

## Acceptance evidence

| Requirement | Evidence | State |
|---|---|---|
| `DWC-V0-929` | `internal/dogfoodflow/check.go`; `TestCheckDWCV0929ReportStaysOnWorktreeMount` | implemented; focused regression |
| `DWC-V0-926` | `internal/dogfoodflow/propose_anchors.go`; `TestProposeCitationsDWCV0926OnlyAnchoredDrafts` | implemented; focused regression |
| `DWC-V0-927` | `internal/dogfoodflow/propose_anchors.go`; `TestProposeCitationsDWCV0927NoHeadings` | implemented; focused regression |
| `DWC-V0-928` | `internal/dogfoodflow/propose_anchors.go`; `TestProposeCitationsDWCV0928BaseGoverningDefinition` | implemented; focused regression |
| `DWC-V0-001` (experimental under the standing lane auto-accept; V1-1467) | `internal/dogfoodflow/regenerated_specs.go` `regeneratedSpecs`, `replaySpecs`; `batch.go` `deriveMerge`; `TestBatchCheckDWCV0001RegeneratedRequirementsMerge`, `TestBatchCheckDWCV0001OtherConflictStaysUnbound`, `TestBatchCheckDWCV0001RejectsGeneratedSymlinksAndModes`, `TestBatchCheckDWCV0001RejectsPreservedHandEdits` | implemented; focused tests PASS |
| `DWC-V0-002` (experimental under the standing lane auto-accept; V1-1467) | `script/merge-regenerate-specs.sh`; `make lane-scripts-test` merge-regenerate-specs group | implemented; shell test PASS |
| `DWC-V0-003` (experimental under the standing lane auto-accept (2026-11-02); V1-4318) | `internal/dogfoodflow/reused_worktree.go` `requireFreshWorktree`; `TestDCWV0844BindResetRerunNamesAndRetiresTheResetCase`, `TestDWCV0003PreflightRefusalPrintsOneCommand`, `TestFinishLaneDCWV0400InputsCheckRefusesAStrandedTrace` | implemented; focused tests PASS |

## Non-goals and limits

No resolution of conflicts in any other file, no rewrite of pushed branches, and no change to `DCW-V0-401` regenerated commits.

## Rollback

For `DWC-V0-001` and `DWC-V0-002`, remove `regeneratedSpecs` and its call in `deriveMerge`, and delete `script/merge-regenerate-specs.sh`; such merges are unbound again and need the rebase recovery. For `DWC-V0-003`, see its own rollback.
