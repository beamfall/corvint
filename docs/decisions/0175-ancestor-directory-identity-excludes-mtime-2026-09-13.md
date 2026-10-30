# Decision 0175 — ancestor directory re-identification excludes modification time

Date: 2026-09-13. Status: accepted, delegated coordinator call. Authority: repository owner
delegation to make owner calls and record them (orchestration thread, 2026-09-12).

The question. `verifyExecutableBinding` (`internal/dashboard/repository/filesystem.go`) reopens the
resolved Git executable immediately before each child `Start` and after its leader `Wait` by walking
every ancestor directory through `openChildDirectory`. That walk compared the parent-relative
`Lstat`, descriptor `fstat`, and second `Lstat` with the full file identity, which includes
modification time, size, and link count. Any concurrent entry creation in an ancestor such as
`TMPDIR` or `/opt/homebrew/bin` therefore marked an unchanged executable as drifted, and LOD-V0-005
discarded the whole authority as `REPOSITORY_UNAVAILABLE`. Commit `ec5ca28a` reproduced it in 41 of
100 acquisitions and added a test-side retry; `local-observability-dashboard-v0.md` said only that
the re-identification MUST match.

The call. A directory in that sequence is re-identified by directory identity only: platform file
identity (device and inode on POSIX, volume serial and file index on Windows), file type and
permission bits, and on POSIX owner and group. Its modification time, size, and link count are not
drift evidence, because creating or removing a sibling entry changes them without changing what the
path resolves to. The executable file itself keeps its complete existing comparison (full identity
including modification time, byte count, and path-to-descriptor binding), so replacement by rename
or rewrite is still drift.

Scope. The comparison lives in the one shared `openChildDirectory` sequence, so the same projection
applies to the open-time probes of `corvint-dashboard-git-layout/0` directory bindings. Each binding
still records the descriptor `fstat`'s full identity, and the retained-binding re-identification at
Finish (`reopenDirectory`, `retainedDirectoryStable`) still compares that full identity; layout
file reads and executable file comparisons are unchanged.

Consequences: `directoryIdentity` and per-platform `directoryIdentityFromFile`; the spec clauses on
executable re-identification and the directory-binding sequence name this projection;
`TestExecutableVerificationIgnoresAncestorEntryChurn` (0 of 500 drifts under a concurrent sibling
mkdir/rmdir loop, 485 of 500 before) and `TestExecutableVerificationDetectsReplacedExecutable` pin
both halves; the `LOD-V0-005 phase/unknown-mode` retry added by `ec5ca28a` is removed.
