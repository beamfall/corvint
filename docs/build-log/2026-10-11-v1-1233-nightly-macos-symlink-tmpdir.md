# V1-1233: dogfoodflow trace-path-limit test under a symlinked TMPDIR

## Finding

The macOS nightly failed `TestLocalOutcomeTracePathLimitKeepsAdmissionReason`. The test used
`t.TempDir()` as the evidence root. The recorder publishes the aggregate through
`aggregateReadFile`, which refuses a symlinked ancestor (`dogfoodoperation.CheckParents`). Hosted
macOS TMPDIR lives under `/var`, a symlink to `/private/var`, so `localOutcome` returned
`aggregate-publication-failed` instead of the admission reason. Linux runners have no such ancestor.

## Change

The test resolves its temp root with `filepath.EvalSymlinks`, as `aggregate_worker_test.go` already
does. The product refusal is intended and unchanged.

## Evidence

`TMPDIR=/tmp/claude-501-symtmp go test -count=1 ./internal/dogfoodflow` failed only this test before
the change and passes after it on macOS. No other test in the package depended on an unresolved
temp root.
