# N-1 first release: v1.0.0-rc.3 has no predecessor (V1-3177)

The Release gates workflow failed its two `core-n1-replay-v1.0.0-rc.1` and
`core-n1-replay-v0.8.1` rows with `not a valid object name`: this repository was recreated on
2026-10-30 as a single-root history (root `9477995d`), and neither tag nor its published release is
in it. Owner decision 2026-10-30: v0.8.1 and v1.0.0-rc.1 are gone for good, reachable only in an
archived repository; `v1.0.0-rc.3` is the first release of this repository, and every later
release's N-1 is the previous release published here. rc.3 must pass Release gates before it is
tagged, so the first release needs an explicit path rather than a skipped or deleted check.

## Decision

- `script/select-previous-release.sh` exits 3, stdout empty, when no prior `v*` tag is reachable,
  separate from every refusal and Git error (exit 2).
- New `script/core-n1-source.sh` is the one N-1 resolver for both checks. A selected tag must be a
  published, non-draft release of `$GITHUB_REPOSITORY` (`gh release view`). With no reachable tag it
  lists the published releases: FIRST_RELEASE (exit 3) only when each one is the candidate itself or
  a descendant of it; any other one, or one whose tag is not local, fails closed and asks for the
  explicit `previous_release_tag`/`previous_release_oid` inputs.
- The two pinned replay rows become one `core-n1-replay` row, `script/core-n1-source.sh --replay`,
  which runs `make core-n1-replay CORE_N1_TAG=<tag>` or logs `NOT_APPLICABLE FIRST_RELEASE` with a
  notice and a step-summary line. Only that row receives `GH_TOKEN` (read-only `contents`).
- `linux-core`'s `n1` step sets `first_release`. The N-1 install lifecycle then writes a
  NOT_APPLICABLE log, notice and summary line. The host lifecycle qualification runs without
  `--base-corvint` and accepts exit 1 only when its report shows the upgrade case as the single
  `NOT_RUN` and the other eight cases passed. A fetched predecessor keeps the previous behaviour.

CCF-V1-007 and release-runbook step 8 carry the amendment; requirement IDs are unchanged.

## Evidence and limits

`script/select-previous-release_test.sh` (exit-3 contract and the extracted workflow `n1` step,
including the FIRST_RELEASE path) and `script/core-n1-source_test.sh` (published check,
FIRST_RELEASE, descendant releases, fail-closed refusals, `--replay`) pass locally; both now run
under `make install-lifecycle-test`. NOT_OBSERVED: a hosted Release gates run of this workflow; the
FIRST_RELEASE notices, step summary and HLQ acceptance on a real runner are first observed on the
rc.3 dispatch. The `CORE_N1_TAG` Makefile default still excludes pre-release tags, so a manual
`make core-n1-replay` after rc.3 must name the tag.

Rollback: revert this change; the pinned rows would fail again until a new baseline is named.
