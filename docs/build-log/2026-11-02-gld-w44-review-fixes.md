# gld-w44 independent-review corrections

Base: `1bf1bfd03adbe9189b2991a23e698701317a8284`. This records the six supplied
independent-review findings and the local repair evidence; no new capability or requirement ID.
The orchestrator owns committing, CEM binding, post-commit checks and integration. Those steps
are `NOT_PRODUCED` here because this repair task explicitly forbids commits and changes under
`.corvint/changes/`, and the real Git directory is read-only.

- `DRS-V0-001`: replay now uses ort's merged inputs alone, with its scratch object store as
  an alternate. It rejects non-regular generated entries before loading the tree and checks
  output/destination types before writing. Complete tree comparison includes modes. The new
  `TestBatchCheckDRSV0001RejectsGeneratedSymlinksAndModes` reproduced writes to external targets
  for all three generated paths before the fix. The new
  `TestBatchCheckDRSV0001RejectsPreservedHandEdits` reproduced accepted README prose and INDEX
  `implementation`/`dependsOn` edits before the fix. Both pass after it; the existing valid merge
  and other-conflict regressions also pass.
- `ASD-V0-1922`: reconciliation fetches origin/main and verifies the reported merge commit is
  contained before scanning. A fetch/containment failure leaves the marker unset; refused scans
  also remain retryable. The real local-origin fixture in `script/lane-scripts_test.sh`
  reproduced the premature marker and missed refresh, then passed with failed-fetch,
  absent-merge, successful-retry and later-merge coverage. The full suite passed all 55 groups.
- `ASD-V0-1923`: selection failures propagate through `static_steps` and `static_phase`;
  test-only packages skip build but remain in vet. Group 17 of `script/batch-precheck_test.sh`
  reproduced invalid-base empty selection, injected discovery failures in both modes, and
  build rejection before vet for a test-only package. The extracted group passes after the fix.
- `PUR-V0-1920`: a test-looking executable name also needs a `-test.*` argument to enable the
  source-checkout guard. `TestPURV01920_IsTestBinary` reproduced false classification of ordinary
  `.test` and `.test.exe` production names and now passes, together with the existing own-checkout
  guard test. `TestPURV01921_ForbiddenStoreRefusesPause` also passes.

Focused Go checks used `script/heavy.sh`, writable private `/tmp` lock/log/cache paths, and
explicit `-run` selectors; no whole-repository tests or `make gate` ran. `make spec-requirements`
passed using a temporary index and object directory: generated files were unchanged. `gofmt -l`
for both touched Go directories, `bash -n` for the four touched shell files, and `git diff --check`
were clean. Real `go vet ./internal/tasks` also passed through the wrapper. After resolving
sandbox cache paths and updating the older invalid-base assertion to the new early failure, the
stable full batch-precheck shell run failed only its pre-existing Pi tuple assertion (case 1b),
also observed before these edits; case 6 and repair-specific group 17 passed. The final handback
retains the exact runs, failed attempts and limitations.

Corvint orientation used `context --task` and `affected --base` at the base and after edits.
Context treated the lane's four-digit requirement names as ticket-shaped unresolved anchors;
original requirement definitions and trace rows were read directly. The installed binary warned
`self-binary-stale`; the later affected plan retained two `LANGUAGE_FRONTIER` unknowns. Those are
not claims of complete coverage, external qualification or independent verifier evidence.
