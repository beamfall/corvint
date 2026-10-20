# 2026-10-20 Out-of-tree map reads for `cem status` and `cem verify` (V1-2097, #1329)

Human-owned intent: `docs/specs/cem-stable-v1.md`, section "Out-of-tree map reads for `cem status`
and `cem verify`" (`CEM-V1-315..317`), accepted under the owner's standing auto-accept
(2026-10-11).

BASE: `253bc9a1dcf3284cca302aa320f57310d3a8673a`.

## Decision

`cem status` and `cem verify` read an absolute `--map` when its resolved parent directory lies
outside the worktree and both Git directories. This reuses the containment rule that an absolute
`cem report --output` already follows: file identity, compared after resolving the parent's
symlinks. The final component is read through the same no-follow, identity-checked bounded read
as a repository-relative map. An absolute path inside the repository is refused, not resolved, so
there is still exactly one spelling of an in-repository map and the report aliasing checks keep
their premise. `cem report` keeps repository-relative maps only.

The anchor and verdict-ledger half of #1329 is deferred. The `corvint-cem-anchor/1` pointer names
the map's commit, path and blob, and `cem provenance` verifies those against the product
repository. Recording a map that lives in a separate evidence repository needs a new pointer
profile and read-back contract, which is wider than this lane. Help and the spec state the limit,
and the PR says `Refs #1329`.

## Evidence

- Focused tests: `internal/cem/cli` and `internal/cem/workflow` packages, and the
  `TestCEMV1317HelpDocumentsMapPaths` and help tests in `cmd/corvint`.
- Live: the lane binary read the committed map of `830ff9027` from a scratch directory outside
  the repository. Both `cem status` and `cem verify` returned `ACCEPT`, exit 0, with a result
  identical to `cem verify-stable` over the same path, and the worktree was unchanged.
