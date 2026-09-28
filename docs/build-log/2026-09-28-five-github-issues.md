# GitHub issues 326–330: guidance fixes and multi-repository declarations

Owner request: fix the five open issues at `https://github.com/beamfall/corvint/issues`.
Base: `e9a261d26bd5915e1c4a8d10519a6355b34c5946` (`origin/main`).
Work is isolated on `codex/fix-five-issues`; pre-existing primary-worktree changes are preserved.

## Delivered scope and evidence

- #326 / RGV-V0-004: whole-value markers, Go comment-token attribution, JSON/string exclusion,
  explicit label bounds, and physical source lines despite Go `//line` directives.
- #327 / RGV-V0-005: overview reads the optional native snapshot without building it, binds matching
  data to captured commit/tree, and retains unknown state, miss reasons and omissions otherwise.
  A missing matching filename retains the combined absent/stale/unsupported-engine reason.
- #329 / RGV-V0-005: `.cc`, `.cxx`, `.hpp` identify C++; `.h` remains explicitly ambiguous.
- #330 / AFU-V1-006: a bounded `/2` producer and separate corpus ingestion preserve root-commit and
  revision membership. Local anchor bytes are checked; absent expectations/members, stale revisions,
  unavailable external evidence and unresolved joins remain gaps. Runtime qualification is absent;
  verified sets remain empty and full-suite fallback is unconditional. `/1` bytes stay unchanged.

Committed-fixture regressions reproduced the original guidance failures. The complete guidance suite
passed after repair. Producer → retained provider → manifest → Build → ReadQuery passed for one and
four repository members, including stale/missing member and anchor cases. The public producer CLI,
only-source suffix fixtures, malformed/conflicting declarations and frozen `/1` byte digest passed.
`internal/specindex`, requirement/traceability checks and line-citation checks passed.

One Astra/medium independent reviewer examined the plan, implementation and acceptance criteria.
The reviewer found a physical-line attribution error caused by Go line directives. A failing
regression reproduced it; `PositionFor(pos, false)` repaired it. The focused regression passed and
independent re-review found no remaining findings in the four delivered issues. The checkpoint
source guard also excludes the new direct snapshot probe, preserving its existing no-snapshot rule.
The terminal selected-check observations and CEM/OCM reports bind the final clean target; this entry
records development evidence and does not claim a repository-wide release gate.

## #328 remains blocked

HDCV0-030 requires an authority snapshot proving the exact effective-nav owner/span, isolated candidate
patch application, and full MkDocs configuration reload. The accepted HDCV0-CAP contract requires the
qualified native Linux capsule, image/lock and execution boundary. This checkout has no capsule
implementation/qualified runtime; direct host execution on this Mac is prohibited. The owner was
asked for the qualified runtime location. No lexical PatchPlan was promoted, no admitted-plan refusal
was weakened, and no navigation implementation or MkDocs qualification is claimed. The issue remains
open; completing it requires the accepted capsule/authority path and its live qualification.

## Corvint self-use and retained uncertainty

Used: pre-change query and tracked-path impact, enrolled dogfood plan, affected plans after meaningful
changes, and the existing CEM/OCM completion workflow. Private receipts and test logs are retained at
`/tmp/corvint-five-issues/`; original pre-change receipts also live under this worktree's private Git
Corvint directory. The original broad query returned an unrelated decision with three omitted results
and 417 exclusions; original source/spec expansion supplied the task evidence. It was not reworded to
turn that result into success. A broad path search accidentally surfaced held-out fixture text; that
text was not used for implementation, expected results, or evaluation.

Initial `dogfood-change` retained NOT_PRODUCED `git-diff-failed`, `cem-map-not-produced`, `exit-2`,
`intent-scope-drift`, `map-unavailable`, and `outcome-input-not-provided` while base equaled target and
before evidence binding. The first sandboxed enrollment attempt could not write private Git state;
it was rerun with authorization for the managed checkout. Raw reports remain retained. No failed
attempt is reclassified as a passing run.

Not applicable: external browser/providers, mutation campaigns, learning/ranking evaluations, service
startup and documentation rendering. Their mechanisms are unchanged. Repository-wide `make gate`,
release qualification and unrelated affected-plan advice remain NOT_RUN under the owner's scoped
issue-work policy. Optional HDC execution is unavailable as above. Original-context cost/tokens and
savings are NOT_OBSERVED; a private checkpoint retains task/base/time and account usage (49% → 51%).
Astra/medium was selected for contract reasoning and independent review; the running root model setting
could not be inspected or changed. Three referenced global guideline files were absent.

Rollback: revert the guidance/snapshot and `/2` producer/ingestion changes together with their specs;
existing `/1` remains available. No outward publication, GitHub completion, or native task-store
completion is claimed by these local changes.
