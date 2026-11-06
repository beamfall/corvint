# V1-4701: darwin go-macos failures

Date: 2026-11-06. Bug repairs for the go-macos job of PR #104 (run 42183569212).
Governing contracts: the Tasks CLI symlinked-ancestry refusals (unchanged), the gate argv
entry limit of 128 bytes (unchanged), and the evidence publication paragraph of
`docs/specs/protected-local-execution-v0.md` (amended prose; no requirement ID added or
renumbered). Related tickets: V1-4619 (resolved temporary directories), V1-4734
(restrictive-umask evidence modes).

Baseline: origin/main `9208cc7e25bb82503ef9d61b0ce6a5f02039e649`. Every reproduction and
test ran on the Mac (macOS 26.6.2, Go 1.27.3) through `script/heavy.sh` with
`HEAVY_REMOTE=0`.

## Causes and decisions

On macOS `/var` is a symlink to `/private/var`, so `t.TempDir()` and `os.MkdirTemp("")`
return paths whose ancestry includes a symlink. The product refuses symlinked ancestry
by design; no refusal was weakened.

- `internal/tasks/cli` TestPURV02211, TestTCLV0802 and TestTCLV0869: the tests passed
  unresolved temporary paths to readers that refuse symlinked ancestry. They now use the
  package's existing `resolvedTempDir` helper. No new shared helper was added, so V1-4619
  is not closed by this change.
- `internal/tasks/cli` TestTCDV0770: a **product** defect. `batch close` derived the CI
  run record into `os.MkdirTemp("")` and then handed that path to its own `--ci-run`
  reader, which refused the symlinked ancestry. Every derived-CI batch close failed on
  macOS. The private directory is now resolved with `filepath.EvalSymlinks` before use,
  and removed if resolution fails.
- `internal/cemchain` TestCEMV1286TasksRunnerProducerVerifyChain: the cem-verdict gate
  argv names built binaries, and the long macOS `t.TempDir()` path pushed one entry past
  128 bytes. The test now builds its binaries under a short resolved `os.MkdirTemp`
  directory. The limit is unchanged.
- `interop/cem01-go` TestCEMV1753LegacyEnvelopeNamesLinkedWorktree: the linked worktree
  lived under the unresolved temporary directory, so the envelope's worktree path did not
  match Git's resolved path. The test now uses the module's existing `s0eTemp` helper.
- `tools/local-authority` TestEvidencePublicationUsesExactModesUnderRestrictiveUmask
  (V1-4734): a **product** defect. The stage root was set to its final 0550 before the
  `renameatx_np(RENAME_EXCL)` publication. In XNU, `vn_authorize_renamex_with_paths`
  requires `KAUTH_VNODE_DELETE | KAUTH_VNODE_ADD_SUBDIRECTORY` on a renamed directory even
  within one parent (xnu-11215 `bsd/vfs/vfs_subr.c`, the macOS 15 source line, and current
  main). A non-root owner therefore cannot rename a 0550 directory. Production publishes
  as the dedicated non-root authority account, so publication failed on macOS 15.
  Decision: keep the audit unchanged (it requires 0550 directories), then flip only the
  audited stage root to owner-writable 0700 across the exclusive rename, and restore 0550
  and sync it before success is reported. Cleanup after a failure follows the stage's
  current name. A crash inside that window leaves a 0700 root, which every evidence
  reader refuses (directories must be exactly 0550), so no weaker view is ever accepted.

## Verification and limits

- `internal/tasks/cli` focused run of
  `TestPURV02211|TestTCLV0802|TestTCLV0869|TestTCDV0770|TestFormatVersion`: PASS.
- `internal/cemchain` whole package: PASS (38.5 s).
- `tools/local-authority` whole module package: PASS with a slash-free `TMPDIR`.
- `interop/cem01-go` `TestCEMV175x`: PASS.
- gofmt clean; `go vet` passed for all four touched packages.

`corvint affected --base 9208cc7e25bb82503ef9d61b0ce6a5f02039e649` selected 87 units. By
standing lane rules only the focused tests above ran; batch CI owns the rest.

The local-authority repair is **NOT_OBSERVED on macOS 15**. macOS 26.6.2 does not
enforce the source-directory write check, so the original failure did not reproduce
locally. The cause rests on XNU source evidence; the go-macos CI job on macos-15 is the
first direct observation.

Local friction, not caused by this change: `TestPiBuildManifestDetectsCopiedImageDrift`
fails when `heavy.sh` builds a test `TMPDIR` containing `//` from a parent `TMPDIR` that
ends in `/` ("absolute Pi build directory required"). It passes with a slash-free
`TMPDIR`.

Rollback: revert the commit. No persisted format, migration or store state changes.
