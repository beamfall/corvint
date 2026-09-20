# Decision 0326 — Work-queue adoption binds one explicit Corvint executable

Date: 2026-09-20. Status: accepted (delegated call on the owner's feature request
Beamfall/corvint#45).

## Decision

`corvint work init` requires `--corvint-executable ABSOLUTE_FILE` and records that file's canonical
path, SHA-256, version/build output, and Go module/VCS source identity in the reviewed adapter.
Relative, missing, linked, unsafe-parent, writable, or repository-controlled executables refuse
before any adoption file is written. `work rebind` is the only supported identity update and changes
only the adapter for review and commit.

Companion bundles intentionally use `-buildvcs=false`. Their binding records package,
module/version and Go toolchain identity plus the explicit absence of VCS settings; the exact SHA-256
and version/build remain mandatory. Corvint rejects inconsistent partial VCS settings rather than
rejecting the release profile for omitting metadata by design.

The observer never searches ambient `PATH`. It opens the bound path without following links, verifies
all recorded identities, and materializes only those opened bytes as a private `0500` executable under
a private `0700` directory. The canonical adapter receives that absolute private path as trusted argv
before the fixed policy operation. Original pathname and private-byte identities are
checked before and after every operation; drift is `ERROR/SOURCE_UNQUALIFIED`. This exact-byte
materialization closes pathname substitution on Darwin, where descriptor execution is unavailable,
and keeps Darwin/Linux behavior identical. The receipt remains `UNQUALIFIED` under VPO-V0-024 because
Darwin cannot prove kernel execution of the already-opened object; the binding does not overclaim
attestation. The other local containment, mutation-enforcement, and network unknowns remain.

## Alternatives weighed

- *Add `~/.local/bin` to the fixed `PATH`*: rejected because it restores ambient executable
  substitution and does not bind reviewed bytes.
- *Execute the recorded pathname after hashing it*: rejected because a rename between hash and exec
  could run unreviewed bytes.
- *Execute an open descriptor directly*: preferred in principle, but Darwin exposes no supported
  `fexecve`/`execveat` path and `/dev/fd/N` is not executable there. Private exact-byte
  materialization gives one portable fail-closed boundary.
- *Put executable fields in `work-queue-policy/0`*: rejected because this adoption correction does
  not need a wire-profile change; the already reviewed generated adapter is the binding artifact.

## Rollback

Revert WQO-V0-049..050 and the binding implementation, then regenerate the legacy bare-command
adapter. Existing bound adapters become inert under the reverted observer; no policy or worklist
migration is required.
