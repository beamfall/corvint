# CEM release gate: per-host records, one aggregate readiness job (AFP-V0-432)

Date: 2026-10-24. Ticket: V1-2580. Owner decision (2026-10-24): "per-host jobs pass on suites and
upload records, then an aggregate job judges readiness across both platforms. Keep the external
row; the job stays red only on it until a real external consumer reports."

## Problem

The first `cem-release-gate` job (merge-queue run 40140010715, cem-release-gate run 5) ran
readiness on each host by itself. Every suite passed on both hosts. Each host still refused,
because it observes only its own platform row:
- ubuntu refused with `release-platform-incomplete: darwin/arm64`;
- macOS refused with `release-platform-incomplete: linux/amd64`.

So the job could never pass, whatever the code did. `cem-release-suites` already provides the
cross-host route: `--record` writes a `cem-release-run/1` record on each host, and `--aggregate`
judges those records together (CEM-V1-611, CEM-V1-612). The workflow did not use it.

## Change

- **Host jobs.** Each host job runs `make cem-release-gate` with `CEM_RELEASE_RECORD` and
  `CEM_RELEASE_OUTCOME` set, and uploads the output, record and outcome.
  - A nonzero exit without both files (a suite or build failure) fails the job.
  - A nonzero exit after both files are written passes the job. That case is only the host's
    single-platform readiness view refusing.
- **New `cem-release-readiness` job.** It `needs` both host jobs, downloads their records and runs
  `cem-release-suites --aggregate` with the committed `READINESS.json`. It exits with the tool's own
  code.
- **Unchanged.** `READINESS.json` keeps every row. No refusal semantics changed.

## Expected outcome

- The two host jobs are green when the suites pass.
- `cem-release-readiness` refuses with exit 3, on
  `release-external-incomplete: independent-external-consumer` alone. The retained 27e9d98 records
  show this: `artifacts/cem-release-27e9d98/aggregate-outcome.json` clears both platform rows.
- The job stays red until an independent external consumer (V1-0014) is observed.
- This is not release support, and nothing was promoted, tagged or published.

## Evidence

- `go test ./tools/cem-release-suites` was not rerun: no tool code changed.
- `go test ./tools/ci-reuse-plan` passes (darwin). `TestAFPV0432…` drives both run scripts with
  `make` and `go` spies covering:
  - ready;
  - host refusal after the suites;
  - suite failure;
  - a record without an outcome;
  - aggregate exit 0 and exit 3.
- Live CI evidence is the first run on this PR and its merge-queue entry.

Rollback: revert the workflow to the single `cem-release-gate` job.
