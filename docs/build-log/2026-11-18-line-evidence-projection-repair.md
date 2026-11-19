# Line evidence: self-review repair

Ticket: V1-5697. Repair input: `1295ba7877676f49c23f9f5874417b509d734192`.
Lane base: `9f4d5d89ab0648425db40e617f3d890598f9ff79`.

## Findings and decisions

1. LEV-V1-003/004: an unobserved nested Go module has no graph test files. Recording a
   whole-instance obligation must therefore not depend on adding a graph selection. The session
   planner consumes explicit Go line-evidence obligations directly: the root universe loses test filters,
   a nested instance runs its own full suite locally, and CI retains its separate module check.
   The verdict also reports the execution tier. Every nonempty obligation reason is retained,
   including future reason strings; no stale-reason enumeration decides whether it executes.
   Separate internal planner state preserves provenance: ordinary Go inventory fallbacks do not
   become line-evidence obligations or change the existing inventory selection contract.
2. LEV-V1-007/009/012: join the flow projection after the additive overlay. The join projects
   added browser files through the existing inventory, preserving projects and existing flow hits,
   and assigns declared test kinds to all selections. A whole Playwright instance makes the E2E
   projection FULL, conservatively including the entire known browser inventory. This wider
   repository-level projection avoids claiming exclusions when an instance has no graph tests.
3. LEV-V1-002/005: use the inventory's candidate set as Minitest collection arguments and bind
   that exact command on both writing and reading paths. The loader consumes argument strings,
   not generated Ruby source, and empties ARGV before Minitest parses its own options. It includes
   prefix and suffix filenames, non-test-directory candidates, and candidates from an unproven
   pattern. A second filename glob can no longer silently omit inventory candidates.

The soundness class is a narrower downstream projection dropping an upstream obligation or
candidate. The reader's closed file-bounded allowlist is tier `file` with reason absent, damaged,
unqualified or stale. The existing Playwright flow allowlist additionally admits absent/damaged
at tier `full`; stale full and all unknown tier/reason forms retain full-instance obligations.
No new line qualification is admitted. Unknown evidence languages widen, and unknown Minitest
patterns preserve the inventory's broader candidate set rather than recertifying default names.

There are no changed wire shapes, field names, error/exit codes, committed goldens or weakened
assertions. Selection contents and scope now reflect the required additions and full obligations.
Rollback is to revert this repair; it restores the three reported omissions, so the underlying
LEV delivery must remain experimental until those gaps are repaired again.

## Regression witnesses

- `TestLEVV1003StaleNestedModuleManifest`: root-only edit, nested runner mismatch and unknown
  evidence language; local whole-module invocation and CI check.
- `TestLEVV1003FileVerdictAllowlist`: understood file verdicts and future tier/reason defaults.
- `TestLEVV1003FullGoInstances`: root and nested instances, local and CI profiles, known and
  future full-obligation reasons. Existing session-test assertions remain intact.
- `TestLEVV1007EvidenceSummaryProjection`: complete summary E2E object for fresh additions,
  stale identity and unknown language, all selection test kinds, and byte-identical MCP output.
- `TestLEVV1005MinitestCandidateCommand`: both naming conventions and an unproven pattern whose
  extra Ruby candidate must be kept.
- `TestLEVV1005NativeCollectors/minitest`: mixed `calc_test.rb` and `test_other.rb`, asserting
  both canonical methods in the published index. NOT_RUN locally because Ruby is unavailable.

## Verification

The affected plan selected all four touched packages. Its 427 selections retain UNKNOWN scope,
with 54 language frontiers, two unselected binary consumers and one unowned dirty path.
It includes the branch range and mutable repair files, not immutable proof of the final tree.
Go is 1.27.4. Tests run through `script/heavy.sh`, one entire package per command, without `-run`.

- `internal/lineevidence`: PASS, 11.602s; native Vitest/Jest passed; Ruby fixtures skipped.
- `internal/sessiontest`: final PASS, 4.129s, including all original tests.
- `internal/appflows`: PASS, 15.337s.
- `cmd/corvint`: terminal FAIL, 190.592s; all 12 LEV tests passed, including both new
  integration regressions. Only the seven existing scratch-root/profile fixtures below failed.
- `make -s spec-requirements-check line-citations-check`: PASS; specindex 0.995s.
- Citation repin: no moved citations; line check passed with 184 existing basename skips.

Durations are test reports, not performance evidence. Raw logs remain outside the repository in
`/home/russell/lanes/lev-evidence/`; their final digests and terminal CLI result are retained below.
Independent review and repository-wide gates belong to the host orchestrator. No push, PR, Tasks
write, seal, model-backed reviewer or heavy-lock override is part of this repair. The CEM bind
workflow and dogfood targets were removed by decision 0594; no replacement protocol is invented.

## Failed evaluation retained

The initial whole CLI run failed (169.391s). Consuming every Go `FullInstances` entry widened
ordinary absent-inventory cases, breaking frozen selection and ownership assertions. The repair
now carries explicit Go evidence obligations separately from inventory fallback state. No golden,
stderr expectation or existing test assertion was changed. The initial nested fixture omitted
inventory instance discovery before manifest building; the summary fixture attempted to publish
synthetic Playwright evidence through the collector-only writer. Their setup now follows the
existing production planning sequence and synthetic index encoding respectively.

The original CLI failure log is retained as `self-review-cmd-corvint-initial.log`. The first
session-test result is superseded by the terminal run with provenance separation and every
pre-existing assertion intact. These failed/superseded evaluations are not promotion evidence.

The next CLI run demonstrated the nested-module unit and correct FULL E2E projection. Two new
regression expectations needed correction to the existing contracts: `Planned.Dir` is the
repository-relative module directory (see `internal/sessiontest/run.go`), and a FULL E2E summary
includes its `E2E_PLAN_UNBOUNDED` unknown. The new assertions now require those exact values;
no pre-existing assertion changed. The second CLI log (168.643s) is retained separately; its remaining seven existing
root/profile failures reproduce the scratch-parent repository-marker conflict.

The terminal CLI run's immutable source tree is
`2fb8e9bcf8d73fcc057521a54bf58dbf61745646`. The final staged code, tests and owning spec match
that tree; only this evidence record gains terminal outcomes afterward. The session, flow and
line-evidence package results bind unchanged package inputs, not an asserted repository-wide gate.

Terminal CLI failures (required TMPDIR lies below `/home/russell/lanes/.git`):

- `TestAHI373MCPVerbOwnsItsArgumentsAndProfile`
- `TestGPKV0084ImpactCapabilitiesOutsideGitRoot`
- `TestPRBV0008EveryNamedCommandWritesProfiles`
- `TestWatchHelpAndRefusals`
- `TestAffectedRejectsNonRepositoryAndExtraArguments`
- `TestCoreVerbsRefuseAWorkingDirectoryOutsideTheRootAlike`
- `TestConvertedRefusalDiagnostics`

The profile tests refuse output paths inside that enclosing repository; root-refusal fixtures
observe its marker instead of an outside-repository directory. These are the same seven failures
retained in the preceding CI-repair handback. Their existing assertions are unchanged. Host
rerun outside repository markers is required; no clean whole-CLI result is claimed.

Terminal admission waited 381.4s; test time was 190.592s. These are not benchmark results.
CEM binding remains NOT_PRODUCED because the workflow was removed; final dogfood-check is
attempted at handback and its result retained outside this commit. No seal is performed.

SHA-256 digests of retained raw evidence:

- `self-review-affected.json`:
  `001568392d2e04ba9744cdbfc0f5c280eef79a945bb68aaf20f1b969fa66ef5f`
- `self-review-affected.stderr`:
  `c07a7c761ff3da583ce84b32684f7619d59af9581e9277666bdf0aa393066a0e`
- `self-review-lineevidence.log`:
  `ff5ba7bee4de60c8c98e29c3a2aad95b2d5ad8d89f6627ffed43f6e8ba85575c`
- `self-review-sessiontest.log`:
  `a3fcf97051027c358b13d7afcd18ad64ef58b5de92dc3dfae9a4a710ea55f557`
- `self-review-appflows.log`:
  `f52a6df7b365fda0bcaf33e3f152a4f66ffe5674fc3265a4ccae2a7eb4d4e29d`
- `self-review-cmd-corvint-initial.log`:
  `e5078cdc554c6315701f607b6478d0dcaa86ba377b594dba7db4e3509a52b472`
- `self-review-cmd-corvint-second.log`:
  `ccffc45a3d53c2728058be9f39ef7e9bd9ed3fa5fed46be6cf465c10ce50775b`
- `self-review-cmd-corvint.log`:
  `f52cadce379b939d85c40d046632b25e54847ec6f7379bb6e04fdb5767662bdf`
- `self-review-spec-check.log`:
  `c475251ab41da512c39b48900b1a5dbd545fca971a6f33cef4e08fd4afc4eb23`
- `self-review-repin.log`:
  `dce4b7fe3eadc26103692787d6775a7701ed62b33f01ccc227f7182433b8ea7a`
- `self-review-citations.log`:
  `15bc6ae60273b19fee524ad4ae1318369e5b94bdb823b527120e62cb40283e63`
