# Mac dogfood repairs and privacy investigation

Base: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`. Lane: `mac-dogfix`.
Owner-directed charter: V1-4648, V1-4650, V1-3675, V1-1020, in that order.

## Decisions and scope

V1-4648 amends `DCW-V0-380`. The read-style prepare pre-flight snapshots a
tracked CEM's bytes and mode and restores them through a deferred cleanup on
success, refusal and cancellation. An untracked probe remains temporary.
The regression uses a real prepared and committed CEM and checks clean tracked
Git status, exact bytes and mode after both success and refusal; a repeated
successful check must actually reuse its matching receipt.

The second requested criterion, a replacement tracked CEM passing a single
`dogfood-change` without a bind commit, remains PARTIAL. `CEM-CB-009` requires
present target-side CEM bytes to equal the verified input (`checkTargetSidecar`
in `internal/cem/verify/verify.go`). A regenerated CEM that differs from the
tracked blob cannot satisfy that frozen contract until the ordinary bind
commit. Removing that check, suppressing its error or auto-committing from the
read command is outside the charter. This repair removes the destructive
pre-flight, without weakening artifact verification or adding a hidden commit.

V1-4650 adds `CEM-V1-8600`. The recovery search continues through ambiguous
unedited runs across insertion boundaries in both directions. It classifies
at most 16 runs per direction, preserves nearest-first order, clips each run
to the existing byte bound, and names only spans the existing classifier
accepts. This fixed bound preserves linear classification cost in file size;
more distant candidates remain unobserved. Both directional fixtures fail on
the original implementation, then pass and cite the offered spans.

V1-3675 adds `CEP-V0-8600`. The audit found exactly one per-sample metric
reading a different arm: `cb_file_baseline_precision_length_matched`, produced
by `contextBenchLengthMatched`. `resummarize` already invokes
`rederiveContextBench` after all arm replacements. The bug was invalidation
after the error check, which retained stale values on errored replacement
arms. Invalidation now precedes that check, including for the baseline itself.

The other metric producers (`score`, `contextBenchMetrics`, `spanMetrics`,
`budgetMetrics`) use only their own answer plus sample/gold/snapshot inputs.
`pairedDifferences` reads two arms but produces an aggregate, not a stored
per-sample metric; `paired`, `summarize`, `latency` and `outcomes` regenerate
aggregate values from merged details. They are not copied from source report
summaries. Regressions merge two reports with stale sentinel values and cover
baseline replacement, answer-length replacement, empty/error baselines and
abstaining/error answers. No ranking, metric name, report shape or frozen
benchmark output changes.

V1-1020 amends `ASD-V0-2421`. Source inspection found an automatic read route:
a journal-absent `ticket show` miss calls `stagedSnapshotHint`, then
`stagedSnapshots` and `stagedSnapshotQueue`. `os.Open` on `~/lane-inputs` and
`os.Stat`/`os.Open` on candidates followed symlinks without a media boundary.
A staging link could therefore enumerate or open a path in `~/Music` without
an explicit snapshot input. The replacement checks each symlink destination
before descent and uses the existing no-follow opening boundary after bounded
resolution. It rejects protected home folder destinations while keeping the
existing benign linked staging directory behavior and unchanged diagnostic
text. Explicit operator snapshot reads retain their existing contract.

The original 2026-10-08 prompt remains a suspected defect, not a proven
historical instance of this route: the staged-hint code was added later
(the relevant test last changed in `e30adafe212212d60d7a6ce939c20ff00b8e2e14`
on 2026-11-03). The supplied ticket records the historical binary as
`/Users/russelllewis/.local/bin/corvint-tasks`, build 202, Go 1.27.1, with
read-only ticket show/search/audit and approval writes from the main checkout,
plus concurrent test lanes. It does not retain exact argv or parent PID.
No new Tasks command was run in this lane. The live macOS TCC log query
returned rc 64, `Cannot run while sandboxed`; parent inspection was also
sandbox-denied. The historical responsible process and exact invocation are
NOT_OBSERVED. Only that live-attribution criterion is PARTIAL; host execution
of the regression remains blocked separately by the test harness below.

## Verification and retained limits

Detailed commands, return codes and raw logs are outside the repository at
`/Users/russelllewis/lanes/mac-dogfix-evidence/commands.txt` and sibling logs.
The installed Core is `1.0.0-rc.3.dev`, build 88, commit
`d436dff76de1b8e75c427069ae7f80003ae99bc2`. Official freshness is UNKNOWN:
`corvint-update check --component core --allow-network` failed its sandboxed
process-cleanup/version probe; no executable was switched. Tasks execution
and store writes are prohibited by this charter, so its current version and
release check are NOT_RUN. Go is `go1.27.3` on Darwin arm64.

Pre-change `make dogfood-change` and the bound prechange context/query/impact
receipts were produced before edits. `corvint affected --base` was used before
each regression group. It retains `LANGUAGE_FRONTIER` unknowns and broad
unbounded-reader selections; the lane uses the touched whole packages plus
focused relevant checks, without claiming exhaustive coverage. All Go tests
use `script/heavy.sh`, one package per command, with the owner's shared lock
configuration, empty remote host and lane-local TMPDIR. No lock override,
background job, model-backed review, push, PR or native task mutation was used.

The Tasks CLI package cannot start its required dead-man reaper in this
sandbox (`operation not permitted`) and exits before running tests. Neither
the reaper nor an assertion was bypassed. Its test binary compiles. The host
verifier must run the whole package and observe the symlink regression plus
the existing linked-staging test before runtime qualification can be claimed.
Whole-package results and the final dogfood binding are recorded in the lane
handback; a CEM check is structural evidence, not independent review or live
TCC attribution. The orchestrator owns independent review. Frozen retrieval
evaluations and `make gate` are NOT_RUN: no ranking change and the scoped
charter does not authorize the exhaustive gate. No core-freeze output or
golden was changed.

The original requirement sections of the daily workflow and self-development
specs already contain 256 obligations. The generator refused a 257th entry,
and also refused moving it outside `## Requirements`. The final change amends
the existing owning requirements instead; the two genuinely new rows use the
owning prefixes and the charter's reserved 8600 block.

Rollback reverts these source/spec changes and regenerates the requirements
projections. No store, migration, release or remote-state rollback is needed.

## Evidence digests

Raw files are retained outside the repository in the evidence directory named above.

| Log | rc | SHA-256 | Observation |
| --- | --- | --- | --- |
| `baseline-dogfood.log` | 1 | `5b5a9da94c2c369a032516c34dbabeb9acc3beb08964ceacecc4840b2cf68a28` | Final preservation regression deletes the tracked file under the original finishlane; the three existing regeneration failures reproduce there too. |
| `red-4650.log` | 1 | `dd72d32773015b6a43dcd86677bb40b4c1d9bed2a7e11772e7687b83a4c20f2e` | Both directional insertion fixtures fail. |
| `green-4650.log` | 0 | `6752c7d52af059070b8cd03e531118476550bca65bf1226b61d22cccc2b0b9b3` | Both directional fixtures pass and offered spans cite. |
| `red-3675-real.log` | 1 | `2d946c86b19377ef51b135d7fae13fe2f24b651df482cb9bbc502a55dddcdb98` | Errored baseline and answer retain stale cross-arm values. |
| `green-3675-real.log` | 0 | `8f3d09f36ff78cef8ab1d03c9dd71659c44f0aef3b6ba958132903eada0985a7` | All six merged-report cases pass. |
| `whole-workflow.log` | 0 | `758490577008e352a9b0650573020a6cbb1b35d2a30491e57c3af03fcadea257` | Whole package: 57.102 seconds. |
| `whole-retrieval.log` | 0 | `92d4fdec7f6a799837faa2e02930c09459ab86679cfe3106610208ba3e5a302e` | Whole package: 9.717 seconds. |
| `whole-tasks-final.log` | 1 | `dffe0732262bd0ef4f93800596ab4097a2f5c4d5f7eddae5a611a02464515ace` | No tests run: reaper start denied. |
| `compile-tasks-final.log` | 0 | `80b112e479f33ce1e2d7c5f7eb407825816144e3fc737add1dab802ca323e6f7` | Final Tasks test binary compiles. |
| `windows-vet-final.log` | 0 | `e2129c7a7b94a7be1d7c9271f623733d3177903f420a85bbabb73e779a9d9720` | Both repository and interop Windows vet pass. |
| `whole-dogfood-final.log` | 1 | `417be4d10bc87e250bd359fba9ba3626b38d379ea0766e5ff19f46ba9f273b36` | Whole package: 311.956 seconds; exactly the three baseline regeneration failures, no preservation regression failure. |
| `whole-tasks-encoded.log` | 1 | `b1d82b942732f1437ea74ba3a6a5ea6566edb10721b73d04e2081dbe1c049475` | Final encoded positive-control fixture cannot run: TestMain reaper start denied. |
| `compile-tasks-encoded.log` | 0 | `81b5a4ee439f12fabe378cdedc57440855f336037e95667cc9bd5fec9f6c56b7` | Final encoded privacy fixture compiles. |
| `preserve-confirm.log` | 0 | `cfb20fa2d18b70c2a496e5e81a878d0e97a455efed603ac4e48f8d6eaf3796eb` | Final preservation regression passes with real committed CEM, success/refusal and actual receipt reuse. |

The dogfood whole-package failures are `TestBatchCheckDWCV0001RegeneratedRequirementsMerge`, `TestBatchCheckDCWV0401AcceptsRegeneratedCommits` and `TestChangeDCWV0398CitesRegeneratedHunksOnly`. All three reproduce with the original `finishlane.go` restored; their cause remains unqualified in this sandbox. The final package run retains those same failures. No existing assertion was changed. Final local outcome is **blocked**, including the failed whole-package commands in the verification declaration; this does not claim a green test gate.
