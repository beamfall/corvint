# Batch 1106a review repairs

Owner request: repair the three findings against sealed batch head
`c9b73eaf92c054ecb3a0777e29dca85cb9ec4e61`, bind a constituent CEM at that base,
and leave sealing and publication to the orchestrator.

The archive wrapper now compiles the producer beneath a shell with a caught HUP,
then execs the binary. Cancellation during compilation exits that shell before
producer exec, even when the compiler ignores HUP. After exec, the caught trap
resets to the default disposition until the producer installs its context handler.
This preserves GAG-V0-004 and GAG-V0-9962 without a producer flag or persistent state.
The existing fixtures and real-output injection harness follow the build/exec boundary.
The latter bakes its fallback Go path into the shim because archive subprocesses
use a closed environment; its initial live run exposed the missing fixture variable
and failed before archive production. The corrected live run passed.

The dry-run projection now bounds the complete rendered summary at complete line
boundaries, marking truncation. Oversized degradation notices preserve their prefix,
truncate on a rune boundary, and leave half the shared budget for scope, counts and
whole-package advice. STR-V0-7313 explicitly permits that marked clipping. Full and
explicit JSON projections retain their complete notices and JSON. No golden or
existing behavioral assertion was relaxed.

The invalid-history failure row again names the STR-V0-053 fallback and
`test-shards-history-invalid`. The STR-V0-7305 completion-time implementation/test
entry belongs in the three-column acceptance traceability table and is restored there.

## Acceptance evidence

- `script/go-archive-gate_test.sh` case 7 failed with H's wrapper:
  `gate-exit=143 archive-began=yes`. The same harness passed after the fix:
  `gate-exit=143 archive-began=no`; all existing wrapper cases passed too.
- `TestDryRunLongStalePathsStayWithinSummaryBudget` failed on H's production
  code: 4568 combined bytes (497 stdout, 4071 stderr). The final test definitions
  were also run against H's two production files via a Go overlay and failed.
  `TestDryRunSummaryEverySectionHonorsBudget` likewise failed on H, including
  639 output bytes with budgets of 0, 100, 300 and 600 bytes.
- `script/heavy.sh go test -count=1 -run 'TestDryRun|TestSTRV0800' ./cmd/corvint`
  passed, including both regressions, complete-line/UTF-8 checks, and full/JSON preservation.
- `script/heavy.sh go test -count=1 -run TestGroupHangupJoinsLooseDescendantsBeforeWrapperCleanup ./conformance/release-artifact-v0`
  passed both signal cases with the real producer cancellation and cleanup paths.
- `script/heavy.sh script/go-archive-gate-injection_test.sh` passed on
  `594e6de78c84e6303837408b9f8a43d119b4d19b` in 216.9 seconds: five-target build,
  unmodified replay, all five injected output refusals, and real interrupt exit 130
  with cleanup. The prior failed fixture run is retained, not counted as passing.
- `make spec-requirements` and `make -s spec-requirements-check line-citations-check`
  passed after staging their indexed inputs. The generator changed only the
  STR-V0-7313 acceptance-date metadata; no requirement IDs changed.

## Limits and route dispositions

Used: change-start collection, `affected`, focused session-runner tests, archive
lifetime tests, real-producer injection, and the constituent CEM/OCM workflow.
Original logs are retained in the worktree's private Git evidence directory under
`corvint/batch-1106a-review-fixes/`; the requested external handback locates them.
Pre-change context and query outputs were produced at H, but their binding and the
impact receipt are `NOT_PRODUCED`: the supplied path set included the extensionless
archive script and impact refused `unsupported-impact-path-suffix`. A later installed
`affected` read reported `self-binary-stale`; its advice is retained as degraded.

Independent semantic review is `NOT_PRODUCED`: the review runner could not initialize
its in-process app-server client because its runtime state location is read-only.
An ephemeral retry failed identically. Structural CEM review is a separate check.
Session-wide measurement latency, source-open counts and billed tokens remain
`NOT_OBSERVED`; no comparative savings are claimed. Retrieval/learning frozen
evaluations and external host qualification are not applicable to these repairs.
Repository-wide `make gate` was not requested or run; these results are scoped Linux
verification, not Darwin/Windows runtime or repository-wide qualification.

Rollback is a new revert of these repair commits plus a newly bound constituent CEM;
no existing seal, frozen artifact, or shared history is rewritten.
