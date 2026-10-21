# V1-1896: repeated corvint binary builds in cmd/corvint tests are not material

Ticket: V1-1896 (suspected). Outcome: recorded finding, no code change.

## Inventory

`cmd/corvint` tests build a plain `corvint` binary three times per package run:
`record_fresh_test.go` (`go build .` from the package directory), `host_adapter_fail_open_test.go`
and `host_adapter_javascript_test.go` (`go build ./cmd/corvint` from the module root, through
`testEnvironment`). `work_adopt_test.go` builds two variants that cannot share a plain binary
(`-trimpath -ldflags -X main.build=...` and `-trimpath -buildvcs=false`); each is already built
once per run behind a `sync.Once`. `host_adapter_javascript_test.go` also builds
`./integrations/testfixture`, which is a different program, and `qualified_lifecycle_test.go` and
`cem_gate_verdict_test.go` build other programs (a wrapper and `interop/cem01-go`).

## Measurement (2026-10-21, darwin/arm64, load average 33)

With the build cache warm, as it is after the first build of the package run, a further
`go build -o <tmp> ./cmd/corvint` took 2.10, 0.96 and 0.90 s, and `go build .` from
`cmd/corvint` took 0.88 s; a `-trimpath -ldflags -X main.build=<new value>` variant took 4.99 s
and then 1.08 s. Only linking repeats, so sharing the three plain builds behind one `sync.Once`
would save about 2 to 4 s of a package whose recorded cost is about 586 s, under 1%.

## Decision

No shared build helper. The three builds run from different directories, with different
environments and VCS stamping, so merging them also needs proof that no test depends on those
differences, and the saving does not pay for that risk. Revisit if a cold-cache CI shard shows a
different picture: the first build in a run pays the compilation, and later ones still only link.
