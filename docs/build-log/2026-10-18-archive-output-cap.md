# 2026-10-18: release archive subprocess output bound raised to 8 MiB

CI `artifact-integrity` (`make go-archive-gate`) failed on PRs #1094 and #1100 with
`release-artifact-v0 archive: git ls-tree -r -z --full-tree <sha>: subprocess-output-limit`.
The raw committed-tree export (`ARTIFACT-GO-V0-001`) reads the whole `ls-tree` listing through
`runContained`, whose stdout and stderr captures were each bounded by
`maximumCommandOutput = 1 << 20` (1,048,576 bytes). The listing grows with the tracked path list
and had reached the bound:

| Revision | `git ls-tree -r -z --full-tree` bytes |
| --- | --- |
| `2ab0e7442` (origin/main named in the request) | 1,044,290 |
| `575939667` (origin/main at lane start) | 1,044,510 |
| `126517da4` (sta-batch head) | 1,048,827 |

## Decision

`maximumCommandOutput` is now `8 << 20` (8,388,608 bytes), about eight times the current listing,
and still a finite capture: output past it keeps failing closed with `subprocess-output-limit`.
No spec states this value (`release-artifact-integrity-v0.md` and `go-archive-gate-v0.md` were
checked), so no spec text changed.

The other bounds were checked and do not scale with the path list, so they are unchanged:
`archiveFramingAllowance` is added to each archive's binary-plus-legal content bound;
`maximumArchiveReportBytes` and `maxWitnessBytes` bound a report and a witness that carry only
per-target digests, never tree paths; the export's `cat-file --batch` stdout is streamed per entry
rather than captured; the offline verifier request (`maximumRequestBytes`, 2 MiB) carries legal
files, not the listing. `git status --porcelain` on the clean-tree check is small whenever the
check can pass.

## Evidence

- `TestContainedRunnerOutputBoundAdmitsLargeTreeListing`
  (`conformance/release-artifact-v0/archive_gate_posix_test.go`) runs a contained helper that
  writes 1 MiB + 4 KiB (accepted), exactly the bound (accepted) and the bound + 1 byte (refused
  with `subprocess-output-limit`). With the old `1 << 20` value it failed on the first case
  (1,048,576 of 1,052,672 bytes captured).
- `go test ./conformance/release-artifact-v0/` passed on the committed fix.
- `make go-archive-gate` on a scratch clone of `126517da4`: `subprocess-output-limit` (exit 2)
  before the fix; `verdict=PASS targets=5` with the fix cherry-picked.

Rollback: revert the commit; the gate then refuses any tree whose listing exceeds 1 MiB again.
