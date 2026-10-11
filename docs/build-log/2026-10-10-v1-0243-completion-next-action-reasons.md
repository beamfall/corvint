# V1-0243: explicit reasons when local completion has no next action

Intent: `LCP-V0-020` is accepted, decision 0510, in
`docs/specs/local-completion-policy-v0.md`. Owner acceptance is recorded in decision 0510.
Base: `29e1f653486fca13216606683d3a5c2b05cacd36`.

The additive optional Evaluation field `noActionReason` names `enrollment-inactive`,
`enrollment-cancelled` or `workflow-satisfied` when no continuation is appropriate.
Active evaluations retain keyed next actions. Partial CLI error evaluations without an
action name `evaluation-unavailable`; refusals retain existing error codes. There is no
persisted state migration. Rollback removes the optional field and its assignments.
The handoff bundle contains distinct caller-authored task actions, not Evaluation fields;
its mandatory actions and strict schema remain unchanged. Automatic event projections
retain their bounded diagnostic policy and unavailable Frontier authority.

Focused evidence: `TestCompletionNoActionReasons`,
`TestLocalCompletionFrontierUnavailableNextActionOrReason` (active/inactive/cancelled
CLI status and finish under the unavailable session-start Frontier envelope), and
`TestLocalCompletionRealEvidenceWorkflow` (real satisfied status/finish JSON and replay),
and `TestLocalCompletionUnavailableEvaluationReason` (initialized fixture, then a cancelled
context reaching the partial status-evaluation failure).
Verification and independent review results will be retained below.

Corvint self-use: query **used** with limit 1; its receipt retained four omitted results
and withheld test candidates. Affected advice **used**, retaining language frontiers and
unknown coverage, at `/private/tmp/v1-0243-affected-final.json`. Broad tests and `make gate`
are **not run**, as explicitly instructed. Dogfood/CEM enrollment is **unavailable**:
`make dogfood-change` failed creating private worktree metadata outside writable roots
(`/private/tmp/v1-0243-dogfood-start.log`). Post-commit binding, seal and outcome are
**NOT_PRODUCED** because commits are prohibited in this builder task. The host event
fallback is `corvint-event-rejected:dogfood-event-deadline`; it makes no success claim.
Retrieval/learning benchmarks and provider integrations are not applicable.

Installed Core: 1.0.0-rc.3 build 407. Tasks: 0.0.0-tcp01-unverified+build.407.
Official update checks for both components refused their version smoke with
`process-after-start-failed: descendant snapshot unavailable`; freshness and available
versions remain UNKNOWN. No installation or ticket-store write was performed.
Normal git staging is blocked by sandbox permissions on the shared Git directory.
The spec was staged with `git add` in `/private/tmp/v1-0243.index`, using isolated temporary
objects and the original database as a read-only alternate, before `make -s spec-requirements`.
The generated TSV was then staged in that same temporary index. The real index is unchanged.
GNU mktemp on PATH was needed because Darwin mktemp without a template ignored TMPDIR.
The temporary index/objects retain reproducible generation evidence, not a publication.

Model/effort: inherited session settings for a bounded change and independent review;
no model override or claimed cost savings. Account usage telemetry is NOT_OBSERVED.

Independent review: one read-only native reviewer inspected runtime paths, tests and frozen
wire compatibility, and reported no findings after the error test and golden update.
The additive inactive field is retained in the frozen status golden. Review confirmed the
handoff task-state schema is distinct. Shifted source citations in the owning and diagnostic
repair specs were repaired after the line-citation gate exposed them. The completion use-case
implementation receipt and ledger were repinned after the subject hash check failed.

Verification results (logs under `/private/tmp/v1-0243-*`):
- Baseline: `go test -count=1 -timeout 30m ./internal/localcompletion -run
  'Test(CompletionNextActions|EnrollmentAndReadOnlyPolicy)$'` PASS.
- `go test -race -count=1 -timeout 30m ./cmd/corvint -run
  'TestLocalCompletionUnavailableEvaluationReason$'` PASS.
- `sh script/repin-use-case-receipts.sh --check` PASS: all 18 receipts current.
- A filtered compatibility subtest passed its golden comparison but its parent failed the
  all-mode enumeration register. The named parent was rerun without the subtest filter;
  no product change or weakened enumeration gate was made. It failed in unrelated
  impact/affected/prove cases with Git index/status deadlines (479.868s;
  `/private/tmp/v1-0243-golden-final.log`). Its dogfood status subtest passed.
  The focused Frontier regression and `TestLocalCompletionInactiveStatusGolden`
  cover the actual unavailable Frontier and the exact frozen unenrolled fixture
  separately, without claiming all-mode enumeration coverage. An intermediate combined
  fixture comparison included another active session owner absent from the golden; that
  test failure was a fixture mismatch, corrected by using the exact golden fixture.
- `go test -race -count=1 -timeout 30m ./internal/localcompletion -run
  'Test(CompletionNoActionReasons|CompletionNextActions|EnrollmentAndReadOnlyPolicy)$'`
  PASS (720.764s); log `/private/tmp/v1-0243-units.log`.
- All requested doc gates PASS: `make -s spec-requirements-check
  requirement-definitions-check traceability-tests-check decision-numbers-check
  line-citations-check error-code-ownership-check unbounded-readers-check
  use-case-receipts-check use-case-receipts-test`, with the temporary index and GNU mktemp
  described above; log `/private/tmp/v1-0243-doc-gates-final.log`.
- `go test -race -count=1 -timeout 30m ./cmd/corvint -run
  '^TestLocalCompletion(FrontierUnavailableNextActionOrReason|InactiveStatusGolden)$'`
  PASS; log `/private/tmp/v1-0243-frontier-golden-final.log`.
- After the final test addition, `make -s traceability-tests-check unbounded-readers-check`
  PASS; the other passed doc-gate inputs are unchanged.
- The combined `go test -race -count=1 -timeout 30m ./cmd/corvint -run
  'Test(LocalCompletionFrontierUnavailableNextActionOrReason|LocalCompletionRealEvidenceWorkflow|DogfoodHandoffBundle)'`
  finished FAIL (1324.705s). The real completion workflow, including satisfied status/finish
  assertions and replay, produced no failures. Five bundle-export tests failed with
  `local-completion-failed`; the public code masks the underlying cause, which remains UNKNOWN.
  Log: `/private/tmp/v1-0243-cli.log`.
- `go test -race -parallel 1 -count=1 -timeout 30m ./cmd/corvint -run
  '^TestDogfoodHandoffBundle'` PASS; log `/private/tmp/v1-0243-bundle-serial.log`.
  The initial export failures were not reproduced serially. Their exact cause remains
  UNKNOWN; serialization passing is not a root-cause diagnosis.
The named all-mode compatibility test remains failed due to Git deadline errors; it is
not presented as passing coverage.

Final local disposition: task-specific lifecycle, Frontier, partial-error, frozen status
and serial handoff-bundle race regressions PASS; all requested doc gates and post-gate
receipt check PASS; independent review reported no findings. The satisfied status/finish
assertions ran without failure in the real completion workflow. The optional all-mode
compatibility test retains its unrelated Git-deadline failures. No full gate or broad
Go test sweep ran. Owner acceptance, real-index staging, post-commit CEM/OCM integration
and native ticket completion remain outside or blocked by this builder task's explicit
no-commit/no-ticket-write scope. There are no owned test processes left running.
