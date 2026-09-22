# Decision 0130 — the work-queue adapter build trims paths; the build stays inside the adapter

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

## Context

`script/corvint-work-queue` builds `cmd/corvint-work-queue` in every capture. 8a6062e3 pointed that
build at a per-user Go cache, and 03430c56 turned the former 30 s per-operation bound into one
10-minute hang detector (§4.4, decision 0082). Decision 0111 still recorded the production path as
cold-compiling inside a 30 s bound. Neither is true on this base. One cost remained. Without
`-trimpath`, Go puts each package's source directory in its compile key. Every capture builds from
a fresh private target, so all 11 repository packages miss the cache: 1.2 s wall and 3 s CPU per
build at load 12-17, and 2.1 s at load 84 (0111). With `-trimpath`, builds from two other
directories took 0.33 s and 0.24 s after the first and produced byte-identical binaries.

## Call

1. The script builds with `-trimpath`. Everything else stays as it was: the per-user cache, its
   ownership checks, the run-private fallback, and one build from this run's source bytes per
   capture (WQO-V0-005).
2. Corvint does not build the adapter before or outside the observation. Corvint runs only the policy's
   fixed operations on an adapter it does not interpret (WQO-V0-004). Pre-building a Go producer
   would make Corvint depend on how this repository implements its adapter. It would also bypass the
   build branch that the actual-script tests exercise. The hang detector already covers the build
   without acting as a budget, so moving the build would change no outcome.
3. Tests keep running the real script build. A prebuilt test binary was set aside for the reason in
   0111 point 4.

## Evidence

- Parity (scratch, not kept): the same repository-queue fixture (`corvint work observe`, base
  script vs `-trimpath` script, three runs each) produced deterministic 5537-byte results with empty
  stderr. Base vs trimmed differ in 18 of 95 fields. A control that only appends a comment to the
  base script differs in the same 18 fields: the adapter blob/file digest, receipt/observation/
  snapshot/source IDs, checkpoint versions, and receipt stdout digests. The build flag therefore
  changes no observation field beyond what any script byte change does.
- End-to-end `work observe`, interleaved, load 20→41: base 23.6/20.1/20.2/17.1 s, trimmed
  19.6/16.2/43.6/14.8 s. Trimmed was faster in 3 of 4 pairs.
- `go test -json -run '^TestWork|^TestObserveWork' ./cmd/corvint`, 42 tests, exit 0 both times:
  summed 299.1 s before (load 6.8→12.5) and 346.2 s after (load 11.5→15.9). The test wall time is
  not dominated by compilation, and this change does not claim a test-suite speed-up.

## Rollback

Revert the commit: the script builds without `-trimpath`, and WQO-V0-005's text returns to the prior
wording.
