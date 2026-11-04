# pk-affected: selection diagnostics, projections and test qualification

Owner intent: Russell Lewis's pk-affected lane charter, 2026-11-04. Base:
`6c33df33bc5670131482f5263b410cb980580657`; branch `claude/pk-affected`.
Ticket exports under `/home/russell/lanes/tickets/` supplied the bodies and acceptance criteria;
the native store is deliberately not written by this lane.

## Decisions and boundaries

- Preserve the existing read-only shared snapshot design. Query and impact already reuse the
  exact committed-tree snapshot across linked worktrees. A miss cannot silently write repository
  state; its existing notice explains the explicit `index --if-stale` recovery. The focused test
  forces the notice threshold to zero so a silent hit proves reuse independently of latency.
- Preserve the project's caller/mention declaration. A Go comment remains a mention requiring
  declaration; an invalid declaration now also names the offending file and line. No syntax
  inference grants caller authority.
- Label an unbounded witness's representative dirtyPath with `anyDirtyPath`. Give full, compact
  and by-path projections the same witness object shape, with viaLength and an optional via array.
  This changes projection, not selection. The compact viaLinks encoding remains available.
- Prefer bounded receipt-header completion time to filesystem mtime, retaining legacy fallback.
  Disclose metadata failures even outside the newest 64 receipts. Completion time is ordering
  advice only; canonical receipt admission still decides whether a failure contributes.
- Make the local dry-run default the existing 4096-byte summary and expose JSON through --full.
  Keep the CI profile's JSON default because pinned shadow consumers parse it. All projections
  produce the same manifest. A retained wide plan exposed a second output path: 4075 bytes
  on stdout plus 16942 bytes of whole-package advice on stderr. The bounded form now carries
  top causes and one ready heavy reason within its single budget, with no duplicate verdict
  or detailed stderr list. The 400-unit regression reproduces 24241 bytes before the fix.
  No CI control-plane files change.
- Honor the admitted shared Go cache in both executable-build helpers. Add parallel admission to
  90 command tests, 20 Tasks CLI tests and 60 companion-release tests after an AST candidate audit
  and shuffled execution. Keep environment, chdir, hook and timing users serial. The retained
  [serial inventory](2026-11-04-pk-affected-serial-tests.tsv) states each remaining command test's
  reason. A shuffled failure exposed a `clear` of the shared parsed-source cache that the candidate
  audit missed; that test remains serial. The audit is a candidate finder, not proof of isolation.
- Check the complete AND/OR list for a backgrounded fresh-directory assignment. Its trailing
  ampersand runs the assignment in a child, so the parent may retain an inherited repository path.
  Both AND and OR regressions reproduce the unsafe directory case; the exact plain-mktemp forms
  remain conservatively unbounded, and foreground directory lists remain eligible.
- Bracket every incremental timing sample with calibration probes before and after it. Use the
  slower adjacent probe and take the p95 of normalized samples, rather than dividing unrelated
  percentiles. Keep the 100 ms bound and three-round retry limit. An absolute miss remains an
  explicit SKIP/NOT_ENFORCED on a contended host; this is not an absolute latency pass.

The full Affected Plan and Self-Development specs each already have 256 obligations. Their new
clauses therefore belong to the existing Test Selection Precision companion, with TSP-V0-7301,
7302, 7303, 7305, 7311, 7315, 7317 and 7318. STR-V0-7305 and STR-V0-7313 govern the receipt field
and local dry-run default. Every allocated ID was checked across all Git refs before use.
A proposed new companion prefix was rejected by the generator: four-digit-only prefixes require
an existing three-digit definition, which conflicts with this lane's assigned numeric block.
Reusing the established companion avoids changing the generator or allocating an out-of-block ID.

## Ticket disposition and regression witnesses

DONE below means the lane's implementation/acceptance evidence, not a native ticket completion write.

| Ticket | Disposition and requirement evidence | Regression / retained witness |
|---|---|---|
| V1-3880 | DONE; existing IDX-SNAP-V0-019/115 reuse verified, timing added | `TestSnapshotMissNoticeNamesTheBuild`: cold and snapshot-hit impact timings; silent hit at zero threshold |
| V1-3912 | DONE; TSP-V0-7301 | `TestAffectedTempFailureNamesPathAndRemedy`, `TestAffectedTempFailureOnUnwritableParent`; actual failed private scratch creation maps to actionable `unsupported-affected-temp` |
| V1-3913 | DONE; TSP-V0-7302 | `TestForeignCallerCommentDiagnostic`, offending `notes.go:3`; bounded worker scan regression retained |
| V1-4279 | DONE; TSP-V0-7303 | `TestUnboundedWitnessSaysAnyDirtyPath`; base-selector overlay omits the label and fails |
| V1-4314 | DONE; existing DCW-V0-922 already applies before the first finish/closeout prepare | `TestDCWV0922VerifyRunMatchingNoTestIsRefused`; `dogfood-closeout` calls inputs-check before finish |
| V1-4472 | DONE; TSP-V0-7305 and STR-V0-7305 | `TestFailFastContentTimeSurvivesTimestampTies`, `TestFailFastDroppedInfoFailureIsDisclosed`; both fail on the old retention behavior |
| V1-3983 | DONE; existing TSP-V0-900 and AFP-V0-023 read scopes verified for the exact source-documentation spec | `TestDeclaredReadScopesNarrowSpecAndRequirementsPlans_TSP_V0_900` now includes that spec and requires the whole lrfrepo test unit |
| V1-4043 | DONE; existing AFP-V0-012 rule (d) selects the repository import guard | `TestNewCoreImportSelectsTasksBoundary`, including a newly named Core package path |
| V1-4073 | DONE; existing TSA-V0-960 already implements precise none/no-setups | `TestSelectSetupEmptyUniverseIsNone` |
| V1-4087 | DONE, not an acronym defect; TCP-V0-011 test-link relation semantics | `TestDiffGoTestKindIsIndependentOfAcronymMode`, `TestDiffGoLinkedFromTestUsesTestRelationInBothAcronymModes`; a production diff.go counterpart can carry relation kind=test in both modes without becoming a test path |
| V1-4392 | DONE; existing AFP-V0-160/511/560/581 bounds verified | `TestNewLeafPackageWithoutShellCallerIsBounded`: unimported leaf plus declared documentation and an unrelated script selects only the leaf's tests, BOUNDED |
| V1-4439 | DONE; TSP-V0-7311 | `TestWitnessFormsShareOneObjectDecoder` decodes all three projections |
| V1-4012 | DONE; existing IDX-SNAP-V0-115 shared store verified | `TestSnapshotMissNoticeNamesTheBuild` creates a fresh linked worktree and exercises both query profiles and impact without rebuilding |
| V1-3424 | DONE; STR-V0-7313 refines STR-V0-800 | `TestDryRunDefaultsToBoundedSummary`, `TestDryRunCIKeepsPinnedJSONDefault`, existing 400-unit summary bound and CLI summary tests; manifest equality between default and --full |
| V1-3728 | DONE; existing AHI-803 attribution verified | `TestAHI803SiblingRepositoryEditIsAttributedToItsOwner` retains two spellings in one session, with zero outside-project ledger rows |
| V1-4381 | PARTIAL: implementation delivered under TSP-V0-7315; quiet-host/CI package wall-time A/B remains externally blocked by the shared host's sustained load | `TestExecutableBuildHelpersHonorSharedGoCache` fails before the cache fix; shuffled runs cover all 170 newly admitted parallel tests; per-test serial reasons retained |
| V1-3354 | DONE under existing CIN-V0-860; V1-3480's accepted option B expressly supersedes the old blanket non-language-frontier criterion | `TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860`: PLAN_TRUNCATED and other unattributable/malformed unknowns FALLBACK; documented attributable exceptions remain |
| V1-4484 | DONE, confirmed directory-case bug fixed; TSP-V0-7317 | `TestFreshDirectoryRejectsBackgroundAndOrList`, both operators and both mktemp forms; foreground directory positives retained |
| V1-4576 | PARTIAL: paired calibration delivered under TSP-V0-7318; twenty parallel full-suite runs are outside the owner's focused-only execution authorization | `TestSelectionCalibrationBracketsLoadChanges` fails with the old one-sided calculation and preserves a sustained-regression negative; focused repeated live measurement retains absolute SKIPs |

## Verification and qualification limits

The starting `make dogfood-change` recorded pre-change context. The explicit prechange helper then
retained query, context and impact receipts against the base. Directory impact was unsupported;
concrete source-file paths produced the impact receipt. No learning or ranking changes occur, so
learning-weight frozen evaluations are inapplicable, NOT_RUN.

The affected selection retained scope UNKNOWN with Go build-variant/nested-module and shell foreign
frontiers, plus two unselected web-flow binary consumers. Focused tests cover the changed Go units
and conservative refusal cases; they do not establish those external browser outcomes or Darwin
qualification. Exhaustive `make gate`, parallel full-suite execution and Windows vet are NOT_RUN
under the owner's focused-only/no-push instructions. `script/batch-precheck_test.sh` is deliberately
untouched because V1-4525 is already failing at the base.

Focused runs and raw-log digests are retained in the [verification evidence](../../.agent-evidence/pk-affected-verification.json) and the lane
handback. Twelve focused verification commands passed; the combined-output regression and the
recency-comment check and the actual unwritable-parent scratch regression passed separately after their final inputs changed. The final real plan emits 4083 stdout bytes and zero stderr bytes for 221 units (219 Go). Before/after performance observations use explicit serial overlays and the same selected
candidate sets; overloaded-host numbers are not a quiet-host or CI qualification. Initial broad
candidate baselines also exposed sandbox fixture failures, which are kept as failures, not passes.
The prototype commits a673dd7c7 and 8ef4154fc are absent from this checkout's object database;
equivalent cache precedence and safely qualified parallel admission were implemented directly.

The orchestrator runs independent review. This lane does not run a model-backed reviewer, publish,
write the native Tasks store, or seal. Final CEM binding and dogfood-check are local structural
checks; their success does not promote the two incomplete performance qualifications.

The serial-overlay/candidate timings below isolate parallel admission over the same focused sets.
Candidate times are the total for two shuffled runs; these are loaded-host observations, not the
required quiet-host or CI full-package A/B.

| Package subset | Tests | Serial overlay, one run | Candidate, two runs |
|---|---:|---:|---:|
| cmd/corvint | 90 | 17.195 s | 12.651 s |
| internal/tasks/cli | 20 | 8.880 s | 4.561 s |
| internal/companionrelease | 60 | 1.734 s | 3.025 s |

The 20-count live-budget invocation hit the unchanged 30-minute package hang detector after
19 SKIPs. Those skips mean the calibration-relative bound held while the absolute bound was
NOT_ENFORCED. The last observation did not complete. Worktree inputs also evolved during the live
observations, so this is not a frozen-corpus or parallel full-suite qualification. The timeout and
all SKIPs remain retained failures/limits; the bound and wrapper policy were not relaxed.

## Rollback

Revert the corresponding implementation and requirement together. Reverting output projection
restores the previous wires without changing selection. Reverting parallel admissions restores
serial scheduling. Keep the recorded failures and qualification limits; never turn a SKIP or a
missing external outcome into a pass.
