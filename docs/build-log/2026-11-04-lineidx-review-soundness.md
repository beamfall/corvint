# Line selection review: preserve static evidence and fail closed

The independent review of `83fcd1204a346fd25f1b4f7f63aabce5bacc3cd4` required three P1
soundness repairs and two P2 CI repairs. This change addresses those findings without enabling
coverage exclusions or claiming qualification. Requirement IDs remain stable; the owning spec
amendments accompany the core and workflow implementation commits respectively.

## Review findings and evidence

1. **Source readers** (`V1-4364`, `GLS-V0-013`, `LPCV-V0-952`). Preserve possible static
   source/data readers independently of the winning static witness. Units with path tokens/globs,
   declared or unbounded reads, embeds, import-graph reads or unresolved dependencies stay whole,
   as do their consumers. This deliberately trades precision for soundness until independently
   verified read evidence exists. `TestAffectedLineSourceReaderKeptWhole` covers both the edited
   package (whose direct-source witness masks its reads) and a downstream reader. Each source-only
   test detects the body mutation without calling the changed function. Before the fix they were
   dropped from `[calc=tests:^(TestDouble)$ user=exclude]`; after it both units are `whole`,
   categorized `static-read`, while the edit still classifies as `body`.
2. **Both ASTs** (`V1-4364`, `GLS-V0-012`, `LPCV-V0-953`). Read immutable new-side blobs and
   hunk spans as well as old-side evidence. Both sides must parse, preserve declarations and
   signatures and place paired hunks in the corresponding function. Missing new evidence abstains.
   `TestAffectedLineInsertionIntroducingInitKeepsWhole` inserts a closing brace and `init` inside
   an unexecuted function. Before the fix this yielded `calc=compile:^$`; afterward it is a
   declaration fallback, the package stays whole, and `TestState` fails on the injected state
   change. `TestAnalyzeRequiresBothSides` covers missing/invalid head evidence and signatures.
3. **Local qualification** (`V1-4365`, `V1-4366`, `GLS-V0-011/020/034`,
   `LPCV-V0-955/963`). Local selection reads qualification from the immutable planning base,
   verifies the runner and positive qualification counters and binds all source blobs to both
   the base and the clean VCS revision of the executing selector. Missing or mismatching evidence
   retains proposed decisions as `SHADOW unqualified`; the manifest keeps the static plan.
   `TestCorvintTestUnqualifiedLineIndexIsShadow` fails before the fix because the unqualified
   index removed a package; after the fix its manifest units and exclusions equal the static
   manifest. Existing static symbol advice can already slice tests; the regression compares that
   actual static plan. `TestLocalQualificationRequiresMatchingEvidence` admits a matching fixture
   and rejects missing records, misses, absent kills/controls, runner/source drift and missing or
   dirty executable provenance. CI builds its selector with VCS stamping for the same admission.
4. **Package miss confirmation** (`V1-4366`, `GLS-V0-046`, `LPCV-V0-957`). The
   `(outside every test)` sentinel matches the exact package's execution start and package-level
   terminal result, whose JSON omits `Test`. Named tests still require their exact identity.
   `TestLinecovPackageMissConfirmation` executes the workflow's actual rerun function: package
   panic and TestMain failure were rejected before the fix and are accepted afterward; build,
   setup, unrelated package/test and malformed output remain rejected.
5. **Persistent kill state** (`V1-4366`, `GLS-V0-047`, `LPCV-V0-020`). Confirmed misses create
   `refs/tags/linecov-kill-go` on the repository. This persistent lightweight ref has no artifact
   retention lifetime. CI requires an explicit 404 before treating it as absent; other lookup
   failures keep the static plan. The owner deletes the ref and any retained kill artifact only
   after repair and requalification. `TestLinecovKillSurvivesArtifactExpiry` executes the actual
   CI guard: before the fix an expired artifact enabled exclusions; after it a persistent marker
   and unreadable marker both block. `TestLinecovKillMarkerWriter` verifies creation, concurrent
   preservation and visible persistence failure. The artifact remains an expiring detail record.

## Verification and measurements

All five regressions were run before implementation through `script/heavy.sh`, with a two-minute
hang timeout, and failed for their reviewed behavior. Focused package suites then passed:
`internal/linecov` (15.145 s), `tools/linecov` (20.610 s), `tools/ci-reuse-plan` (9.344 s), and
the affected/test CLI, use-case, help and capability checks in `cmd/corvint` (13.403 s).
The three workflow YAML files and all 56 shell steps parse. All direct Go test commands name one
package and run through the required heavy wrapper.

The changed two-AST selector was remeasured using the retained 17,885,718-byte real index,
50 changed files, 386 units and 12,922 declared tests: open+plan p50 **57.965 ms**, p95
**65.076 ms**, below the 100 ms budget (`GLS-V0-003`, `LPCV-V0-958`), excluding Git reads.
This workload kept 501 tests by coverage and excluded 8,137 in the internal experiment. It is
neither local admission nor qualification. The prior 43.444 ms p95 and twenty-PR zero-reduction
measurement remain historical observations; they do not qualify this changed implementation.

Detailed commands, negative-control output and final gate receipts are retained in
`/home/russell/lanes/lineidx-evidence/review-round/` and the lane handback. Documentation gates, changed-file formatting, line citations, the per-package capability catalog
checks and Windows vet passed on the final source; their retained logs are in that directory.
The handback records the post-commit binding and check results. No repository-wide
`make gate`, live workflow execution, publication, task completion write or seal is part of this
review round. The orchestrator owns independent re-review.

## Retained limits, feature use and rollback

The real-tree qualification remains NOT_PRODUCED: the previously retained attempt failed its
unchanged baseline in this sandbox (45 tests, zero mutants), as documented in
`2026-11-03-gls-line-selection.md`. Changed selector sources require fresh qualification on a
suitable host; no record was manufactured. Hosted kill-ref permissions and live workflow execution
remain NOT_OBSERVED. The documented manual binding sequence is used because the finish wrapper
always seals and has no stop-before-seal option. A disposable probe reproduces the sandbox
report-publication rename failure (`EXDEV`) between the worktree Git directory and `.corvint/`;
final checking must retain that external failure if it recurs, without bypassing the checker. Existing tickets remain open; no completion claim follows from structural
binding. Until qualification passes, local results are shadow-only and CI remains unqualified.

Used native index, affected and requirement-specific context before editing, and `dogfood-change`
against the original lane base `6f12f4704235dc8f9bea19fc12995bfb68c746c3`. The latter retained the
bound pre-change query/impact receipts and explicitly refused missing final citation/scope/outcome
inputs at change start. Local manifest and source/fault fixtures exercise the applicable selection
route. Learning/ranking, optional providers, browser flows and hosted integrations are not applicable
to this bounded soundness repair. Independent re-review is deferred to the orchestrator.

Rollback keeps the qualification gate, restores the static plan and leaves the persistent kill ref
in place. Disabling narrowing or removing the index loses only optimization. Repair and requalification
must precede operator deletion of the kill marker; artifact expiry alone never re-enables it.
