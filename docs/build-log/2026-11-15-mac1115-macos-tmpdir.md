# mac1115: macOS TMPDIR and hosted-macOS ticket premises

Date: 2026-11-15. Base: `origin/main` `1a02c5f39ce333278987900d1afacf6324ebab25`. Tickets:
V1-4551, V1-4841, V1-4840, V1-4838, V1-4945. Host: Linux (WSL2); no macOS host or hosted runner
was available to this lane, so every macOS result below is NOT_OBSERVED. The `/var ->
/private/var` ancestry was emulated by a TMPDIR under a private `var -> private/var` symlink.

## Premise checks on the base

- **V1-4551 criteria 1-3** were delivered before this lane. The cem2proto fixture of the original
  failure was retired with the package (`38b36384a`, V1-4747). `interop/cem01-go`
  `TestCEMV1753LegacyEnvelopeNamesLinkedWorktree` keeps its checkouts under resolved directories
  (`s0eTemp`) and pins the symlinked-ancestor refusal on every host (`430dc9d2e`): the same symlink
  family, not a separate cause. It passes here with the default TMPDIR and the emulated one.
- **V1-4551 criterion 8, V1-4840 and V1-4945** name `internal/tasks` tests. That tree left this
  repository with the Tasks extraction (`a1a9bc4fa`, decision 0594), so this repository holds no
  code or test for them. V1-4840's darwin repairs (`79ef4205e`) were made before the extraction.
- **V1-4841** was delivered by `c945f6cbb` and `9923ac104` (`ASD-V0-9950`):
  `script/lane-scripts_test.sh --heavy-host-env-only` passes 20 cases, including a non-existent
  Mac-style TMPDIR that is named as dropped and the `HEAVY_REMOTE_TMPDIR` opt-in; the `core` group
  covers a kept caller `env TMPDIR=PATH`.
- **V1-4838** code and regression tests were delivered by `639617d89` and `7e57975e2`
  (`AMAP-V0-8780`). The hosted macos-15 cause from Git's stderr is still NOT_OBSERVED, as the
  requirement says.

## Change

V1-4551 criteria 4-7 named the worktree-executable build of `script/lane-corvint.sh`
(`DTL-V0-003`). `TestDTLV0003WorktreeBuild` resolved its expected path, but only macOS reached
the symlinked case. The test now runs two cases, each with the executable's output and the
injected build-failure refusal: an ordinary temporary root and a checkout reached through a
symlinked temporary parent. The second case must build at the canonical git-dir path on every
host, and it checks that the alias differs from the canonical root. With `lane-corvint.sh`
changed to build under the unresolved checkout path, the symlinked case fails on Linux and the
ordinary case passes. No product, script or spec behaviour changed.

Rollback: revert the commit.
