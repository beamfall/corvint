# Release-path spec hygiene: lane relhyg (2026-11-11)

Base: origin/main `b55635262788687c3eea9a08a94378e4ab8856a4`. Tickets V1-5267, V1-5330, V1-4886,
V1-4887, V1-4888, V1-5359 and V1-5333; V1-4819 has its own entry,
`docs/build-log/2026-11-11-plugin-envelope-proposal.md`.

## V1-5267: `spec-acceptance-audit` at the base

The ticket's failure (the `ACX-V0-9301` marker, then on `REQUIREMENTS.tsv` line 105) is gone at the
base: `script/spec-acceptance-audit.sh` and `script/acceptance-log-audit.sh --strict` both exit 0,
and `ACX-V0-9301` (now line 118) parses as mode `owner`. `make -s spec-acceptance-audit` still
fails, for a different reason: its third step, `script/acceptance-plan-pin-check.sh`, exits 1 with
`STALE_PLAN` for decision 0596. Since the owner accepted the plan at blob `e4accbefde3e`, the plan
gained `PROD-INT-032` (accepted by the owner 2026-11-10, V1-5299, commit `9ebbd15d7`), and one code
citation moved to `internal/liveverify/affected/liverepo/` (commit `4cea40c19`). The check ignores
only acceptance markers, citation line numbers and the record path, so a later owner-accepted plan
change makes it fail. Rewriting the blob in decision 0596, an accepted record whose own text allows
a rewrite only "before the owner signs", is not a lane's call. Handed to the orchestrator: decide
how a post-acceptance owner amendment re-pins the plan (an amendment record the check reads, or a
new pin with the owner told).

## V1-5330: `DCW-V0-348` retired (orchestrator decision)

Orchestrator decision under the owner's 2026-11-02 delegation, not an owner acceptance:
`DCW-V0-348`, the pre-push seal hook, is retired. Decision 0594 already deleted
`script/pre-push-dogfood-seal.sh`, its test and its make target (commit `8307671c7`). The checks
before a push are `script/batch-precheck.sh` / `make batch-precheck` (`ASD-V0-029`) and `make
batch-push-check` (`ASD-V0-035`). The `DCW-V0-348` marker, its evidence row, `CIN-V0-2304` and the
ci-narrowing rollback note now say so, and section 3 of `docs/LANE-ORCHESTRATION.md` no longer tells
the batch-head checkout to install the hook; it says to remove an old one. No `.git/hooks` file
was touched.

Regression: `ASC-V0-781` makes `make flow-guide-check` read the whole guide and fail on any
`exec script/NAME` whose script is missing. `TestHookRecipeNamesAnExistingScript_ASCV0781` failed
before the check existed; with the check in place, the check fails against origin/main's guide
(`docs/LANE-ORCHESTRATION.md:575: hook recipe execs script/pre-push-dogfood-seal.sh, which does not
exist`) and passes on the edited guide.

## V1-4886, V1-4887, V1-4888

- V1-4886: `TCP-V0-961` named `internal/revieworder/order.go`, deleted by decision 0594 (commit
  `389f59ba8`). The clause now gives that declarer as history and says the test keeps it as a
  fixture; the requirement is unchanged.
- V1-4887: on origin/main `internal/docviews` still exists, so the human-documentation-compiler
  spec's present tense is true here. The deletion and the history rewording are commit
  `f3611a53f` (V1-4804) on `origin/claude/batch-1111e` (PR 191, open). Not redone; it lands with PR
  191.
- V1-4888: the nine CKN definitions used `**CKN-V0-00N:**`. The requirement-ID anchor readers
  (`definedIDs` in `internal/cem/verify/support.go`, `tools/requirement-id-collisions`) read only
  `` - `ID`: ``. `git grep -cE '^- `[A-Z][A-Z0-9]*-V[0-9]+-[0-9]+`:'` on the spec counted 0 before
  and 9 after; the wording is unchanged.

## V1-5359: dispositions of the seven undefined cited ids

The seven are the baseline `script/requirement-citations-undefined.txt` of V1-5324, which exists
only on PR 191 (`origin/claude/batch-1111e`). Dependency: when this branch and PR 191 both land,
the baseline lines for the three fixed ids are stale and that check fails until they are deleted.

| Id | Cited at | Disposition |
|---|---|---|
| `RGV-V0-016` | `internal/genesis/guidance.go` | fixed: it is `RGV-V0-050`, which the same commit (`013f8338a`, V1-2053) introduced |
| `DCP-V1-340` | `internal/doccorpus/claim_patterns.go` | fixed: `DCX-V1-340`, per the prefix crosswalk in `documentation-corpus-v1.md` |
| `EX-001` | `internal/cem/workflow/workflow_test.go` | fixed: `CEM-CB-EX-001` |
| `ADN-V0-664` | `internal/liveverify/affected/golang/symbol.go` | wrong citation: the spec and the introducing commit `9643d5715` define `ADN-V0-660..663`; the comment should read "ADN-V0-660 to ADN-V0-663". Not edited: the path belongs to another lane |
| `GPK-V0-067` | `internal/contextindex/impact_callers_test.go` | allowlist: decision 0387 rejected it and keeps it out of every spec, so it can never be defined; the citation is correct as history. Not edited: the path belongs to another lane, and the allowlist exists only on PR 191 |
| `CRB-DEC-002` | `conformance/cli-parity-v0/manifest.go` | allowlist: a decision clause of decision 0308, not a spec requirement; the manifest matches it as data |
| `CRI-V1-002` | `internal/pathbrief/brief.go` | stays in the baseline: defined in the proposed openspec change `change-report-impact`, whose capability spec `change-report-v1.md` does not exist yet; it leaves the baseline when that spec is written |

Evidence: in a temporary worktree of `origin/claude/batch-1111e`,
`script/check-requirement-definitions.sh` passed unchanged; with this lane's three fixes applied
it failed naming exactly `DCP-V1-340`, `EX-001` and `RGV-V0-016` as stale baseline lines; with
those lines deleted it passed.

## V1-5333: requirements that lost their last test citation

The range form of PR 191's `script/check-traceability-tests.sh` (TTG-V0-962) over
`538e823f4a1d..b55635262788` lists 322 requirements (the ticket's 516 was measured against an
earlier main; later merges retired or restored the rest). The committed classifier
`evidence/2026-11-11-relhyg-untested/classify.py` reads Git objects only and sorts each id by what
HEAD still holds; its output is `evidence/2026-11-11-relhyg-untested/classification.tsv`.

| Class | Count | Rule | Action in this lane |
|---|---|---|---|
| RESTORE | 1 | a cited test, or a test named for the id, is declared at HEAD | `GPK-V0-028` cites `TestGPKV0028StabilizePacketBytesReachesAFixedPoint` and `...RefusesWithoutAFixedPoint` again |
| UNTESTED | 216 | no cited test survives; a named path or Go function does, or none is named | the row states `(untested: REASON)`, written by `apply_untested.py`: 138 say a named path or function remains, 78 say whether the implementation remains is NOT_VERIFIED; 211 attribute the removal to the decision 0594 cut of PR #176 (every cited test declared at `b54ac564d898^1` and not at `b54ac564d898`) |
| RETIRE | 102 | no cited test, and none of the row's named paths or Go functions exist at HEAD | NOT_DONE: retirement candidates for a decision, listed below |
| NOT_REQUIREMENT | 3 | a ticket id (`V1-0313`, `V1-0426`, `V1-3133`) in a first cell | none; two share a row with a marked requirement and are exempt through it, `V1-0426` still reports |

Limits of the heuristics: the function check matches a name in any package, so UNTESTED may hold
ids whose own function is gone (this errs away from retirement), and RETIRE treats a moved package
as gone.

After the change, the same range form against this lane's head reports 103 ids (the 102 RETIRE
candidates and `V1-0426`) and `218 untested`.

RETIRE candidates by spec: cem-stable-v1 21, local-completion-policy-v0 10,
shared-path-change-policy-v0 8, lsp-wrapped-backends-v0 7, index-snapshot-v0 7,
lane-dogfood-tooling-v0 5, documentation-corpus-v1 5, public-release-v0 4, map-coverage-join-v0 4,
task-context-packet-v0 3, snapshot-batch-v0 3, lsp-editor-context-v0 3, learned-trace-admission-v0
3, external-frontier-obligations-v0 3, capability-catalog-v0 3, unplanned-read-events-v0 2,
non-go-impact-v0 2, language-plugins-v0 2, and one each in workspace-declaration-v0,
spec-requirement-index-generator-v0, revision-cache-dirty-worktree-v0, go-only-cutover-v0,
external-evidence-provider-v0, corvint-rebrand-v0 and affected-plan-v0. The ids are in the TSV.
