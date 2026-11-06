# heavy.sh go-test scratch under a trailing-slash TMPDIR (V1-4738)

Finding: 9de3c3fc5 built the stable `heavy-go-test-UID` path by concatenating the caller's
`TMPDIR`. The macOS default ends in `/`, so the path held `//`; Go passes `TMPDIR` to `t.TempDir()`
uncleaned, and `internal/procgroup`'s absolute-clean-path guard refused every test working
directory (42 failures on the PR #103 go-macos shards, plus failures in other Darwin packages).

Decision: strip trailing slashes from the scratch parent before joining (`ASD-V0-734` amended).
`script/lane-scripts_test.sh` group 32 runs `go test` under `TMPDIR=<dir>//`; it failed with
`test-root///heavy-go-test-501` before the fix and passes after it. Rollback: revert the commit.
