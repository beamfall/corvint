# Split heavy packages out of the 30 m Darwin shard invocation (V1-4760, CIN-V0-4627)

## Decision

`go-macos` shard 1 hit `panic: test timed out after 30m0s` in `cmd/corvint` in runs 42221230970
and 42212262974, each time in a different test. The package costs 1127 s on Linux and ran inside
one `go test -p 3 -parallel 2 -timeout 30m` invocation with other packages on a 3-core
`macos-15` runner, so it was slow rather than hung. Raising the 30 m per-package hang detector is
refused by decision 0082. Base: `9d0c56665c039876b78eb71b877122f86c4ccd25`.

`.github/macos/shard.py` now scales Linux cost by two as a conservative Darwin estimate. A package
whose scaled estimate exceeds 15 minutes (half the detector) becomes `ceil(scaled / 15 min)`
placement items. The longest-cost-first rule therefore spreads one package's parts across shards.
With the committed cost history, `cmd/corvint` becomes three parts, `internal/tasks/store` four, and
`internal/liveverify/affected` and `internal/tasks/cli` two each. All four shards stay at 2,576,847
Linux ms.

Each part runs alone, before the shard's module invocations. It lists the package's top-level tests
on the runner (`go test -list .`), keeps the names whose SHA-256 modulo the part count equals its
index, and runs them with an anchored `-run '^(A|B|...)$'`. Listing on the runner means tests added
after the cost file was generated still run in exactly one part.

## Fail-closed rules

A failed, empty or malformed listing (a non-identifier or duplicate name) runs nothing for that part
and fails the shard. An assigned test without a terminal `-json` event, or an unassigned top-level
test that ran, also fails it. The shard still runs its remaining invocations. Each part retains
`module-N.list.txt` and `module-N.partition.json`, which record the listing digest and assigned names
for cross-leg audit. Missing or invalid cost history splits nothing, so the old whole-package
behaviour is the fallback.

## Alternatives not taken

The Linux AFP-V0-041 slices (`.github/cishards/test-slices.json`) already split `cmd/corvint` by
measured test cost and use a `-skip` catch-all. Reusing them would mean reimplementing the Go
allow-list and slice validation in the Python sharder, and the slices are sized for Linux shards. A
dedicated unsplit `cmd/corvint` invocation would still run the whole package serially against the
same 30 m detector.

## Evidence

- `python3 -B .github/macos/shard_test.py`: 11 tests OK, four of them new for CIN-V0-4627.
- `python3 -B .github/macos/select_test.py`: OK.
- Local Darwin end-to-end run of `run()` on two real parts of `.github/cishards` through
  `script/heavy.sh`: 25 listed tests, 11 and 14 assigned, both green, partition records retained.
- Hosted `go-macos` wall time and the ticket's two consecutive green main runs: NOT_OBSERVED. The
  shard's job timeout rose from 40 to 55 minutes (CIN-V0-4625 amended): shards plan about
  37.6 minutes of measured cost, and shard 0 of run 42266013561 ran out at 40 minutes. Running parts alone gives up `-p 3` concurrency for
  them, so hosted measurement must confirm the shard still fits.
