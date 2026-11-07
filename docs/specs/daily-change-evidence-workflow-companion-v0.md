# Daily Change-Evidence Workflow Companion V0

Owner: Russell Lewis
Date: 2026-11-02
Requirement prefix: `DWC-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: repository-owner request (V1-1467, V1-4318, V1-4545, V1-4546, V1-4548, V1-4541, V1-4573); `AGENTS.md` invariants 1, 2 and 8; `docs/specs/daily-change-evidence-workflow-v0.md` `DCW-V0-039`, `DCW-V0-401`.

## Agent digest
- Claim: Daily-workflow requirements beyond the 256-obligation limit of `daily-change-evidence-workflow-v0.md`, under their own prefix.
- Status: proposed/experimental
- Exists: `internal/dogfoodflow/regenerated_specs.go`, `reused_worktree.go`, `rebind.go`, `seal_review.go`, `script/merge-regenerate-specs.sh`, `script/lane-handback.sh` and their focused tests.
- Blocked on: Nothing; companion of the daily change-evidence workflow, whose Requirements section is at the 256-obligation limit.
- Read next: Requirements; Acceptance evidence and rollback.

## User and measurable job

This spec extends `docs/specs/daily-change-evidence-workflow-v0.md`, whose Requirements section is at the `ocm prepare` limit; new daily-workflow requirements go here. Two lanes that each add requirement rows conflict in the generated `REQUIREMENTS.tsv` even when their specs merge cleanly (PR #952, #927). The batch owner needs one merge command and a batch-check that binds the regenerated merge instead of reporting `merge-resolution-unbound`.

## Requirements

- `DWC-V0-7500`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3291) The shell inputs-check/finish path MUST check line citations before a build or bind, refuse moved citations with `line-citations-invalid`, and name `script/check-line-citations.sh --repin` plus review/commit before rebinding. The sealed-head closeout precheck MUST disable mechanical regeneration, so a repin cannot create an unbound post-seal change.
- `DWC-V0-926`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4628;
  amended 2026-11-04 fix round 1; amends `DCW-V0-440`; refines `DCW-V0-161`, `DCW-V0-962`, `DCW-V0-805`, `DCW-V0-841`, `DCW-V0-803`, `DCW-V0-385`, `DCW-V0-548`, `DCW-V0-966` and `DCW-V0-967`)
  Cross-file drafts MUST have exact path, basename of at least 6 bytes, or requirement-definition
  evidence admitted by `CEM-V1-081`, except the deterministic seal-contract (`DCW-V0-385`)
  and repin-rule (`DCW-V0-548`) rows, which explain the operations that produce their hunks.
  Preserve marked same-file nearest stable substantive line proposals (`DCW-V0-962`), without
  claiming an anchored tier, including references to same-change IDs in an existing file;
  the new definition itself is not BASE evidence. Decision rows remain eligible when the span names the exact file
  or defines an ID the hunk names. Cross-file sibling, intent and governing-spec guesses alone
  MUST NOT justify a row. Otherwise draft reasoned `unknown`. Reviewed refresh rows retain
  their semantics. Evidence: `TestProposeCitationsDWCV0926OnlyAnchoredDrafts`, `TestProposeCitationsDWCV0926SameChangeReferenceKeepsOwnContext`,
  `TestProposeCitationsDCWV0385CitesPriorSealOnStackedRepair`, `TestProposeCitationsDCWV0548RepinOnlyHunkCitesTheRepinRule`.
  Rollback: remove the cross-file drafted-row filter.
- `DWC-V0-927`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4628;
  amended 2026-11-04 fix round 1; amends `DCW-V0-439` and `DCW-V0-962`) A newly drafted
  span MUST NOT consist only of a bare Markdown heading. Track fenced code so its content
  is not a heading. A setext underline requires a preceding paragraph, not a list item or YAML
  key. Every line of a multiline setext heading's paragraph is a heading line. Try substantive
  evidence instead or keep `unknown`. The rejection applies to drafting only: `dogfood check`'s
  anchorability search keeps the verifier's `CEM-V1-081` admission, headings included. Evidence:
  `TestProposeCitationsDWCV0927NoHeadings`, `TestProposeCitationsDWCV0927MarkdownContexts`,
  `TestDWCV0927CheckKeepsHeadingAnchorability`.
  Rollback: restore heading candidates.
- `DWC-V0-928`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4628;
  amended 2026-11-04 fix round 1; refines `DCW-V0-967` and amends `DCW-V0-805`) For a
  spec amendment, prefer the BASE definition of the entry containing the edited line, including
  a continuation edit that does not retype its ID. Cite only a definition the stability verifier
  admits and no other row already used (`DCW-V0-382`). When added bytes omit the ID, label it
  a same-file proposal with an anchored-by-id review note and explicitly state that `CEM-V1-081`
  cannot anchor it. A rewritten, removed or repeated definition MUST remain a governing-ID
  review note, never a false cite of its removed bytes; another stable proposal may serve.
  Evidence: `TestProposeCitationsDWCV0928BaseGoverningDefinition`,
  `TestProposeCitationsDWCV0928DistinctGoverningSpans`. Rollback: remove the preference.

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

- `DWC-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4545)
  After successfully preparing and citing a replacement for a committed CEM, a standalone
  `dogfood change` whose only tracked worktree change is that CEM MUST stop before OCM/status
  and outcome recording, retain an incomplete report, and print one actionable line to commit
  the rebound CEM and rerun with the same base. A coordination-time impact
  unsupported-impact-worktree row caused by the dirty CEM keeps that same recovery and MUST
  remain visible in the incomplete report; it MUST NOT expand the downstream cascade.
  Other failures retain their own diagnostics.
- `DWC-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4546;
  amends DCW-V0-842) Refresh MUST preserve matched rows and repin their evidence as before,
  but draft each unmatched hunk from its own content through the ordinary proposal rules.
  It MUST NOT copy another hunk's citation merely because both edit the same file.
- `DWC-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4548)
  `DOGFOOD_REVIEW=FILE` MAY attach an external UTF-8 report with exactly one header
  `dogfood-review: reviewed-head=FULL_SHA verdict=PASS`. Only a line starting `dogfood-review:`
  declares a header or selects this format; a prose mention elsewhere is ordinary text. The regular report is at most 1 MiB
  and MUST name the exact pre-seal HEAD; a missing, malformed, nonpassing or differently headed
  supplied report MUST refuse before check or mutation. A successful seal MUST record OBSERVED,
  the report path, SHA-256, verdict and reviewed head in its output and commit message, retaining
  the rename-only tree change. With no report, review-before-seal remains NOT_OBSERVED. The
  existing pinned reviewer-output format keeps its metadata-only descendant admission. The
  report is caller-supplied evidence, not authentication of reviewer independence or correctness.
- `DWC-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4541)
  `script/lane-handback.sh template [PREVIOUS]` MUST produce required base, fix head, bind head,
  sealed head, delivered tickets, superseded tickets and GitHub issues closed fields. Template
  review rounds MUST carry cumulative delivered/superseded ticket lists from PREVIOUS.
  `lint FILE [PREVIOUS]` MUST refuse missing fields, a missing sealed head or empty delivered
  ticket list, and tickets lost from PREVIOUS (a delivered ticket may become superseded).
  Heads are full Git SHAs; an unproduced bind or seal MUST explicitly say NOT_PRODUCED and why,
  so a lane delegated to stop after check never invents a seal. No ticket or issue writes occur.

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
| `DWC-V0-926` | `internal/dogfoodflow/propose_anchors.go`; `TestProposeCitationsDWCV0926OnlyAnchoredDrafts` | implemented; focused regression |
| `DWC-V0-927` | `internal/dogfoodflow/propose_anchors.go`; `TestProposeCitationsDWCV0927NoHeadings` | implemented; focused regression |
| `DWC-V0-928` | `internal/dogfoodflow/propose_anchors.go`; `TestProposeCitationsDWCV0928BaseGoverningDefinition` | implemented; focused regression |
| `DWC-V0-001` (experimental under the standing lane auto-accept; V1-1467) | `internal/dogfoodflow/regenerated_specs.go` `regeneratedSpecs`, `replaySpecs`; `batch.go` `deriveMerge`; `TestBatchCheckDWCV0001RegeneratedRequirementsMerge`, `TestBatchCheckDWCV0001OtherConflictStaysUnbound`, `TestBatchCheckDWCV0001RejectsGeneratedSymlinksAndModes`, `TestBatchCheckDWCV0001RejectsPreservedHandEdits` | implemented; focused tests PASS |
| `DWC-V0-002` (experimental under the standing lane auto-accept; V1-1467) | `script/merge-regenerate-specs.sh`; `make lane-scripts-test` merge-regenerate-specs group | implemented; shell test PASS |
| `DWC-V0-003` (experimental under the standing lane auto-accept (2026-11-02); V1-4318) | `internal/dogfoodflow/reused_worktree.go` `requireFreshWorktree`; `TestDCWV0844BindResetRerunNamesAndRetiresTheResetCase`, `TestDWCV0003PreflightRefusalPrintsOneCommand`, `TestFinishLaneDCWV0400InputsCheckRefusesAStrandedTrace` | implemented; focused tests PASS |
| `DWC-V0-044` | `TestDWCV0044RebindStopsBeforeCascadingFailures` | implemented; focused regression PASS |
| `DWC-V0-045` | `TestDWCV0045RefreshDraftsNewHunkFromItsOwnContent`, `TestProposeCitationsDCWV0842RefreshAcrossAMainlineMerge` | implemented; focused regressions PASS |
| `DWC-V0-046` | `TestDWCV0046ExternalReviewSeal`, `make review-head-check-test` | implemented; native and wrapper regressions PASS |
| `DWC-V0-047` | `TestDWCV0047HandbackCumulativeTickets` | implemented; template/lint regression PASS |

## Non-goals and limits

No resolution of conflicts in any other file, no rewrite of pushed branches, and no change to `DCW-V0-401` regenerated commits.

## Rollback

For `DWC-V0-001` and `DWC-V0-002`, remove `regeneratedSpecs` and its call in `deriveMerge`, and delete `script/merge-regenerate-specs.sh`; such merges are unbound again and need the rebase recovery. For `DWC-V0-003`, see its own rollback.

## Lane maintenance evidence (2026-11-04)

| Requirement | Implementation and regression |
| --- | --- |
| `DWC-V0-7500` | `script/dogfood-finish.sh`, `Makefile` dogfood-closeout-precheck; `script/dogfood-cache_test.sh` pre-build refusal and post-seal no-regeneration recipe. |
