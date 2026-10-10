# 2026-10-10: Advisory CI drift report for the shard cost table (AFP-V0-040)

## Finding

The cost table refreshed on 2026-10-04 (build log 2026-10-04-ci-shard-cost-refresh) went stale
within six days and nobody saw it until the shards visibly unbalanced. In hosted run
<https://github.com/beamfall/corvint/actions/runs/38055182050> the six shard jobs took
2348/1265/1295/977/1292/1490s. `internal/tasks/store` was recorded at 871s and observed at 1,872s,
and seven executed packages had no entry. `tools/ci-shard-costs check` already detects this, but
only when an operator scrapes the logs and runs it by hand.

## Change

- Each full `go-product-shard` tees its `go test -json` stream to
  `$RUNNER_TEMP/shard-costs/shard-<n>.json` and, only when that invocation passed, uploads it as
  `ci-shard-costs-<n>` (3-day retention, `overwrite: true`, continue-on-error). `go test`'s own
  status is read from `PIPESTATUS[0]`, so `tee` can never decide the shard. No extra test run.
  Capture is best-effort: if the directory or file cannot be created, the shard emits a notice, runs
  `go test` without `tee` and retains nothing.
- New job `ci-shard-cost-drift` (`needs: go-product-shard`, `always() && !cancelled()`, not a
  required check): downloads the artifacts, abstains in the job summary unless the shards
  succeeded and exactly `SHARDS` (6, must equal the matrix) artifacts are present, and only then
  checks out, builds `tools/ci-shard-costs` and runs `check --advisory`. The report step is
  continue-on-error and its shell never propagates the tool's exit status. The job itself and
  every step (download, completeness check, checkout, setup-go, report) are continue-on-error with
  their own timeouts, so the job never ends red. The job posts no status and never writes the
  table.
- `tools/ci-shard-costs check --advisory --shards N [--share P] [--summary FILE]` requires exactly
  N logs, each a well-formed, finished `go test -json` stream (every non-blank line an event, at
  least one terminal package outcome, no package start left unfinished), appends a Markdown table to the summary, prints at most ten `::warning::` workflow
  commands (one per material finding plus one count of the rest) and exits 0 on findings. Unusable
  input exits 2 and writes `Abstained: <reason>` to the summary. Plain `check` and `refresh` are
  unchanged.
- Threshold: a finding is material when the time it misplaces reaches 10% of the ideal shard
  (observed package time / N). In run 38055182050 the ideal shard was 1,281s, so 128s. The largest
  noise-level differences were about 60s (`tools/corvint-pr-tests`, `internal/companionrelease`),
  `internal/tasks/cli` was 111s (8.7%, below), and the two that misplace real time were flagged:
  `internal/tasks/store` 1,001s (78.2%) and missing `internal/appmap` 132s (10.3%). A material
  drift is reported even inside the factor-2 ratio, because a 1,000s package drifting to 1,700s
  unbalances the shards while staying under factor 2. Stale entries place nothing and are never
  material; they are still listed and counted.
- Retaining raw streams avoids the retained limit of the 2026-10-04 entry (a hosted log prefix
  containing `{` hid a package), and the same artifacts are valid `refresh` input for the operator.

## Evidence

- `GOTOOLCHAIN=local go test -count=1 ./tools/ci-shard-costs`: pass, including
  `TestAFPV0040AdvisoryReportNeverFailsOnFindings`,
  `TestAFPV0040AdvisoryWarningsStayWithinTheStepLimit` and
  `TestAFPV0040AdvisoryAbstainsOnPartialOrUnusableInput`, and the unchanged AFP-V0-022 tests.
- `actionlint .github/workflows/ci.yml` (with shellcheck) and `make ci-least-privilege-check`: pass.
- Local dry run: the `run` blocks of the new job were extracted from `ci.yml` and executed with
  the six ANSI-stripped shard logs of run 38055182050 as `shard-0..5.json`. All six present:
  exit 0, two material warnings (`internal/tasks/store`, `internal/appmap`) plus one count warning
  (4 drift, 6 missing, 0 stale), and the 12-row summary table. Five of six present: abstained with
  "5 of 6 shard outcome artifacts present". Shards failed: abstained with
  "go-product-shard result is failure". One stream with a failed package: the tool exited 2, the
  summary says why, and the step still exited 0. The shard-step pipe kept a simulated `go test`
  exit status of 3 and set `costs=1` only on 0.

## Limits

- Independent review of 22159dea found three P2 gaps, fixed in the following commit: the drift
  job could end red on a checkout or setup-go failure; an empty, malformed or unfinished stream
  was accepted; and the capture `mkdir` ran under errexit in the required shard. New abstention
  cases (empty log, malformed record, prefixed record, unfinished stream) each use a valid table.
  Rerun dry run: the six hosted logs, reduced to raw streams (timestamp prefix stripped, non-event
  lines dropped), gave the same two material warnings and count warning; the logs as downloaded,
  with prefixes, now abstain.
- Hosted behaviour is `NOT_OBSERVED` until this change's own CI runs: artifact upload and
  `pattern`/`merge-multiple` download, annotation rendering, the summary, and the extra minute of
  job time are untested on GitHub. Re-run attempts and the download step's behaviour when no
  artifact matches are also `NOT_OBSERVED`; both are continue-on-error and end in abstention.
- The change edits `.github/workflows/ci.yml`, so its pull request needs the admin-posted
  `ci-control-plane` status (AFP-V0-016) before it can merge.
- Every pull-request run with tests reports against the table on its own tested tree. A PR that
  makes a package slower will warn on that PR; that is intended, and is advisory.
- AFP-V0-040 is accepted by decision 0487 (owner approval in chat, 2026-10-10); acceptance settles
  intent only, and hosted behaviour stays `NOT_OBSERVED`.

## Rollback

Revert this change: the shard step returns to the plain `go test` invocation, the retention step
and the `ci-shard-cost-drift` job disappear, and `tools/ci-shard-costs` loses `--advisory`. No
table, status or required check depends on any of it.
