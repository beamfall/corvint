# PR443 CEM runner foundation hosted CI repairs (V1-0632)

PR443 head `f3fba61b10b5c2ad7cbbea54c56a0dccfa76b6d1` failed three hosted go-product
shards. Merge `eb7bc224a4b811742fb61a7eabdddaf61590e367` brings current public main
into the branch; the failures were reproduced or explained against that merge.

Read scopes. Shard 3 (`internal/cem/verify`: `TestCandidatePortablePacket`,
`TestStableS0EPublicCases`, `TestStableS0ELinkedControl`) and shard 2
(`internal/cem/wire`: `TestCandidateClosedProfile`, `TestCandidateAdditionalBoundaries`)
failed with permission denied, not a digest or verdict mismatch. Both packages are
declared in `.corvint/test-read-scopes.json` (AFP-V0-023) without the CEM 1.0 packet the
branch's new tests read, so the Landlock wrapper refuses the reads. This is Linux-only:
the wrapper confines only on Linux. A local Go 1.27.1 Linux arm64 container (Landlock ABI
4) running the unchanged `.github/testconfine` wrapper reproduced all five failures at
the merge. The repair adds `protocol/cem-1.0/` to `internal/cem/verify` (manifest,
listed artifacts and the stable repository-envelope packet) and the narrower
`protocol/cem-1.0/maps/` to `internal/cem/wire`, in ascending order. Reader code, the
confiner and all other allowances are unchanged.

Detached maintenance. Shard 0 `TestDirtyNonUTF8PathIsDisclosedNotRefused` failed only in
TempDir cleanup (`unlinkat .../.git: directory not empty`). Its Git helper sets
`GIT_CONFIG_GLOBAL=/dev/null`, which hides CI's global `maintenance.auto=false`, so each
`git commit` starts `git maintenance run --auto --quiet --detach` (observed with
`GIT_TRACE` on Git 2.54 and 2.47); the detached child can outlive the test and race the
cleanup on hosted Git 2.55. The test and this package are unchanged by the branch, so
the race also exists on main. Both helpers in `observation_test.go` now pass
`-c maintenance.auto=false -c gc.auto=0`, matching the existing fixture convention. No
assertion changed. The race did not reproduce locally in 300 Darwin iterations or on
Git 2.47 Linux, so hosted Git 2.55 remains the observation surface.

Rollback reverts the two allowances, restoring the demonstrated denial, or the two
config arguments. Do not broaden the protocol tree further, skip tests, or weaken
confinement. Hosted Ubuntu amd64 results remain required after publication.
