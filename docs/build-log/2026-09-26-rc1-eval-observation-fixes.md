## 2026-09-26 V1-0366, V1-0367, V1-0368, V1-0370: rc1 evaluation, observation and traceability fixes

V1-0366 (`LTA-V0-011`, `LTA-V0-012`). The slot-weight loader and `corvint eval --reset-slot-weights`
resolved `.context-corvint/slot-weights.json` by path, so a symlinked `.context-corvint` let the
loader admit, and reset delete, a file outside the repository. `contextindex.OpenSlotWeightsStore`
now pins the store directory inside an opened repository root (Lstat, open, same-file check); load
and reset both go through it, refuse a symlinked or non-directory store, and reset removes a leaf
symlink instead of following it. The IDX-SNAP-V0-017 audit digest is re-pinned without a schema
bump, because analyzer extraction and encoding are unchanged.

V1-0367 (`SOL-V0-001`). The observation writer's ignore-coverage check read `.gitignore` and
`.corvint/.gitignore` with an unbounded, link-following read that blocks on a FIFO. The writer now
reads each file through an `os.Root`, only as a regular file of at most 256 KiB, with a
non-blocking open re-checked by fstat. Any other file skips the observation. The unplanned-read
ledger (`URE-V0-003`) inherits the rule through the shared check.

V1-0368 (retrieval-bench `--summarize`, no numbered requirement). The merge kept the first report's
limit and Corvint identity while later reports replaced arm results. Reports at different limits,
or measuring different Corvint binaries, are now refused. The merged identity comes from whichever
report measured a binary, so a lexical-only (`NOT_RUN`) first report no longer names it.

V1-0370 (`TTG-V0-003`, `TTG-V0-009`, `TTG-V0-010`). A deeper heading that also matched
"traceability" replaced the open section's level, so a sibling heading closed the outer section and
its rows went unchecked. Only a closed section now takes a new opening level. Nested, repeated and
closing-heading fixtures were added to `script/check-traceability-tests_test.sh`, which is now wired
as `make traceability-tests-test` in `GATE_STEPS`. On the current specs `traceability-tests-check`
reports no unresolved names, so no trace rows were added.

Checks. Each ticket's regression test failed on the pre-fix tree and passes after the fix. Passed:
`go test` for slotlearn, observations, unplannedread, retrieval-bench and specindex, `-run` subsets
of contextindex and `cmd/corvint`, and `go vet` on all of them; the documentation checks and
`traceability-tests-test`; `use-case-receipts-check`, `use-case-receipts-test` and
`conformance/use-cases-v0`. A clean clone passed both traceability targets, and a checker mutated to
admit untracked declarations failed `traceability-tests-test`.

NOT_RUN: `go test ./...`, `make gate`, `make dogfood-*`, and the Windows runtime. The full
contextindex package did not complete at host load ~490: unrelated tests hit git subprocess
deadline errors, then the 30m timeout. CI does not yet invoke `traceability-tests-test`, and
`.github/` was out of scope.
