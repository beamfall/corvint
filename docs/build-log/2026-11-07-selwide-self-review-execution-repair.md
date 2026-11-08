# Selwide self-review repair: hidden tests in execution and FULL fallback

Repair intent: V1-5018 / AFP-V0-9012. The review found two remaining ways
to omit a hidden package after successfully selecting it: CI rebuilt its
execution universe with only `go list ./...`, and the PR driver derived
explicit hidden discovery arguments from a selection that FULL replaces
with `./...`.

## Decisions and evidence

CI execution now discovers tracked hidden Go source directories independently
of the narrowing record. It passes them explicitly alongside the wildcard to
the Go tool before computing either partition or balanced placement. Nested
modules, testdata, excluded metadata directories, ignored Go filenames and
declared retained outputs stay outside this root-module supplement. A discovery
failure stops execution rather than authorizing a truncated universe. Discovery
lives inside the existing workflow, so it needs no new file in the separately
pinned trusted-tools checkout.

The driver likewise discovers tracked hidden directories independently of
`selection.Packages`. Sharded discovery feeds the existing AFP-V0-048 complete
placement; unsharded FULL appends the hidden directories to the wildcard test
invocation. The persisted FULL selection remains `./...`. No JSON member,
exit-code contract, golden, core-freeze output or existing assertion changes.

`TestAFPV0098SupersetGuardSidecar` continues from the narrowing record through
the actual workflow discovery and execution filter. Placement must consume a
universe containing `nested/.helpers`; successful narrowing, balanced-placement
disagreement and FULL must each retain that package for execution (AFP-V0-098,
AFP-V0-590). The original record assertions remain intact.

`TestHiddenFullExecution_AFPV09012` uses a real repository, planner, selector
and Go test execution. Its visible consumer passes while the hidden package's
test fails. Missing qualification, planner fallback and driver identity drift
are each exercised with one and four shards: every combination must emit the
hidden test's failure and fail its owning invocation. The prior narrowed
hidden-package regression also passes unchanged.

AFP-V0-9012's contract and implementation/test mapping are updated in this
change. V1-4941 / AFP-V0-9013 and V1-4710 / AFP-V0-9014 are unchanged; earlier
evidence and host integration limits remain applicable. Rollback reverts this
repair's source/spec change and reopens both execution omissions; no migration
is needed.

## Verification and limits

Pre-change capture is reused. The required affected read against
`c3b29cbb54d31c8526d3cce4ad74356b6fb9b943` selects both touched Go packages,
with 421 selected units and 71 unknown entries, scope UNKNOWN. The installed
planner reports itself stale; this is selection guidance, not current-tree
qualification. The driver regression builds and uses this tree's planner and
selector. All Go tests use `script/heavy.sh`, one package per command, with
the required private TMPDIR and no exported Git identity.

Raw logs remain outside the repository in
`/home/russell/lanes/selwide-evidence/`. Final results and digests follow below.
The first workflow regression attempt accidentally extracted an earlier
advisory share step; its missing fixture directory failure is retained in
`self-review-2-forward-focused.log`. The extractor was corrected to begin at
execution discovery, and the focused regression passed. This was a test
harness error; no production assertion or frozen output was changed.

Independent review and hosted qualification are orchestrator-owned and
NOT_RUN here. Repository-wide selection, gates, Tasks writes, publication
and sealing are excluded by the repair charter. CEM checking remains structural
evidence; native tickets and prior host-validation blockers remain open.

| Artifact | SHA-256 | Result |
|---|---|---|
| `self-review-2-forward-whole-final.log` | `ccaf021aebbba170bcb809d813b1590c305899ac175f76a173eb72389397d98e` | ok  	github.com/Beamfall/corvint/tools/ci-forward-row	94.251s |
| `self-review-2-driver-whole.log` | `5040ccd9812a2ece843256c14e844afbec77d3cac47ddbf54ffdeb960b3d7a47` | ok  	github.com/Beamfall/corvint/tools/corvint-pr-tests	143.594s |
| `self-review-2-forward-focused-fixed.log` | `e84867253caf8b5fd91db6a11e55ecc72cc9daf2089fd0de75f3b30412f005aa` | ok  	github.com/Beamfall/corvint/tools/ci-forward-row	2.209s |
| `self-review-2-driver-focused.log` | `4db0f1472425396f118b9a2f2862792137b255e9a60e60a5fb0f51870a59efc7` | ok  	github.com/Beamfall/corvint/tools/corvint-pr-tests	48.126s |
| `self-review-2-spec-citations-check.log` | `21e02443e14a1f9adc6b286135b4dc1d72df47fd4c63177ddfdc323a552744f8` | ok  	github.com/Beamfall/corvint/internal/specindex	1.329s |
| `self-review-2-affected.json` | `5dc08418a4a644c93609e742e25402df1cebadf68032cda5143d8c5306a49093` | UNKNOWN retained |
