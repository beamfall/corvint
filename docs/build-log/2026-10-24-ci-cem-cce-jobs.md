# CI jobs for the CEM release gate and the live CCE prove tests

Date: 2026-10-24. Base: `ec41e4e9c901cb3546e95a82fc665bb8bc6c58e9` (origin/main).
Branch: `claude/ci-cem-cce-jobs-1024`. Tickets: V1-2580 and the cce-live part of V1-1687.
Requirements: `AFP-V0-432` to `AFP-V0-435` (proposed). Authority: the owner's 2026-10-24
delegation of `.github/workflows/` edits, recorded in
`docs/build-log/2026-10-24-owner-queue-decisions.md` on PR #1718.

## Design decisions

- `cem-release-gate` (`AFP-V0-432`) fails on any nonzero exit, as asked, with no
  `continue-on-error`. On main today `make cem-release-gate` refuses before running suites (tool
  exit 3, `make` exit 2, about 71 s on darwin/arm64): the readiness row
  `corvint:repository-envelope` is `NOT_RUN` and the linux/amd64 and external rows are
  `NOT_OBSERVED`. The job is therefore red on every run until those rows close. That is the
  intended release signal, so the job is not a required check and is not made one here.
- `cce-live` change detection (`AFP-V0-434`): `ci.yml` has no path-detection job (`docs-plan`
  outputs only `mode`), and a workflow-level `paths` filter would leave a skipped or missing check.
  The job therefore always runs and decides in its first step, with the AFP-V0-042 first-parent
  `git diff --quiet` pattern over `internal/changee2e/` and `ci.yml` itself, so edits to the job
  exercise it. An unchanged commit concludes green with a summary line; any doubt runs the tests.
- Tuple check (`AFP-V0-435`): the expected build, executable version and Node version are copied
  from the 1.64.0 row of `qualifiedBundledBrowsers` in `internal/jstestprovider/external.go`, and
  the test fails if they drift. The job reads the revision from `playwright-core/browsers.json`
  and runs the cached executable under the default macOS `ms-playwright` cache, not
  `PLAYWRIGHT_BROWSERS_PATH`, because the prove harness may scrub the environment.
- Both jobs honor the AFP-V0-081 batched skip, so a `ci:batched` constituent (this PR included)
  does not observe them; the first observation is the batch PR's run.

## Evidence

`tools/ci-reuse-plan` tests `TestAFPV0432`..`TestAFPV0435` execute the extracted step scripts with
fake `make`, `node` and executables and with temporary Git histories. Live runner behavior is
`NOT_OBSERVED` until a non-batched run.

## Rollback

Delete the two jobs from `ci.yml`, the test file and the four requirements.
