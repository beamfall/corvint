# 2026-10-21 tasksperf-1021o: cem-verdict gate names a verifier build mismatch

Ticket: V1-1761 (criterion 1 delivered; criterion 2 not delivered). Requirement: TCB-V0-045 (accepted
under the owner's standing auto-accept of 2026-10-11).

## Problem

On V1-1677 the `cem-verdict` gate refused `CEM_VERDICT_VERIFIER_DISAGREEMENT` /
`verifier-disagreement` because `~/.local/bin/cem01-go` was built before an `interop/cem01-go`
change that the installed `corvint` already carried. The refusal read like a verifier defect.

## Decisions

- Only after a disagreement, the gate reads the Go build information both executables record
  (`runtime/debug.ReadBuildInfo` for itself, `debug/buildinfo.ReadFile` for the consumer, which is
  never executed for this) and refuses with the new reason `verifier-build-mismatch` when both
  record a `vcs.revision` and they differ or either tree is modified. The code stays
  `CEM_VERDICT_VERIFIER_DISAGREEMENT`, so Tasks policy and consumers keying on the code are
  unchanged and the closed code set does not grow.
- An unrecorded build proves nothing and keeps `verifier-disagreement`; a build difference never
  refuses agreeing legs, because the consumer's source may be unchanged across revisions.
- No revision or path enters the gate document (TCB-V0-040 forbids version strings there).
- Criterion 2 (`corvint-update` installs `cem01-go` with `corvint`) is not delivered:
  `corvint-update` installs official release components, and shipping `cem01-go` in a release is a
  release-packaging decision for the owner. TCB-V0-045 documents the rebuild command instead.

## Evidence

- `TestTCBV0045DisagreementNamesAVerifierBuildMismatch` and
  `TestTCBV0045RecordedBuildReadsVCSSettings` pass, with `TestTCBV0016*`, `TestTCBV0017*` and
  `TestTCBV0040*`; the first fails with the build check disabled.

## NOT_RUN

- A rehearsal against a stale installed `cem01-go` (the V1-1761 host condition).

## Rollback

Revert the commit: the gate returns to `verifier-disagreement` for every disagreement.
