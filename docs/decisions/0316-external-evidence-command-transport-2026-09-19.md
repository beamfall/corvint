# Decision 0316 — External evidence providers may run as one contained local command

Date: 2026-09-19. Status: accepted (owner call on Beamfall/corvint#11).

## Decision

The command transport ships as `docs/specs/external-evidence-provider-transports-v0.md`
(`EEP-TR-001` to `EEP-TR-008`). `corvint impact --provider-command ARGV_JSON` is an explicit opt-in:
the operator names a local executable as a JSON argv with an absolute path; Corvint invokes no shell
and makes no `PATH` lookup. The child runs in its own process group with stdin closed, an
environment of only `PATH`, `TMPDIR`, `LANG=C`, and `LC_ALL=C`, and the repository root as its
working directory. Corvint grants it no network access and no credential. Hard bounds are 10 s wall
time, 1 MiB stdout (the record bound), 64 KiB stderr, one record, and one launch; at the wall-time
bound the whole group is killed. stderr is bounded, discarded, and never echoed.

A command's complete stdout goes through the file transport's decode unchanged: the same digest,
schema dispatch, strict decode, freshness, reference verification, `learned` exclusion, and
Core-assigned `external-provider` authority. Every transport failure is one closed provider row with
no digest and no record content; partial output is never decoded.

This narrows decision 0309's deferral of executed providers to "an ACC-V0 profile family". ACC-V0
governs digest-pinned analyzers whose output becomes Core evidence; a provider command's output
never becomes Core evidence and is decoded exactly as a file the operator could have produced by
running the same command. Corvint contains the process it launches but does not sandbox what an
operator-chosen executable does; the operator's opt-in accepts that.

`corvint affected` does not take `--provider-command` in this slice.

## Rollback

Remove `internal/extevidence/transport.go`, the command branch in `load`, the `--provider-command`
option and help text, the transports spec, and its index rows. The receipt wire is unchanged.
