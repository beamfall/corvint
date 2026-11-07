# Selmin: close reader path provenance gaps

Date: 2026-11-07
Requirements: AFP-V0-9010, AFP-V0-2200 (V1-5029)
Base: cb403735903b1e54a8469373489960c0b2b89d99
Repair input: c9ba60a880dd5d046a4d8d95905199b828fc5101

The independent review retained two P1 soundness findings: a computed command
working directory could make a proven `go list .` reader observe a different
package, and an existing testdata symlink could redirect a supposedly local
fixture read into another package's source. Both require conservative selection.
The earlier build-log entry's deferred directory and symlink residuals are superseded here.

## Decision and closed proof boundaries

A `Cmd.Dir` assignment is certified only for unchanged-directory literals, a fresh
`testing.TempDir()` with its parameter confined to direct method receivers, or a
module-root derivation from this file's `runtime.Caller(0)` filename using
`filepath.Dir`, `Clean`, and `Join` with relative literals. Locals in the derivation
must have one preceding top-level definition and no uses outside the derivation.
The root case also requires identical target resolution from the root and the
reader's directory; otherwise it is unproven. All unknown forms retain whole-module
import and source observation. This closes directory redirection as a class,
including relative targets other than the reported dot, while preserving the
existing repository guard and configured-command control without changing assertions.

Own-testdata paths admit literal paths or literal joins with no climbing component
or ambiguous separator. Every ancestor and terminal entry must be an unfollowed
regular file or directory of the selected source universe. Missing entries after
checked ancestors cannot read another source; symlinks, other modes and metadata
errors fail closed. Directory listings use the existing Source API, including its
cache recording, with no direct host filesystem access for immutable sources.
`Source.Lstat` was unsuitable here: for supplied filesystems it calls `fs.Stat`,
which may follow a link. Unfollowed `ReadDir` entries establish the proof instead.

## Evidence and limits

- `TestCommandDirectoryFallsBack_AFP_V0_9010`: the reported computed byte path,
  an unknown environment origin, relative targets, root mutation/address escape,
  unknown caller depth and root-relative dot retain conservative classification.
  Unproven cases require import-graph selection for both import and body edits.
  Known root, unchanged-directory and fresh temporary-directory controls pass.
- `TestExistingTestdataSymlinkFallsBack_AFP_V0_9010`: existing intermediate,
  testdata-root and file aliases retain source-content selection; a regular fixture
  stays proven.
- `TestTestdataUnknownEntryFailsClosed_AFP_V0_9010`: symlinks, named pipes, devices,
  irregular modes and permission errors are observing, with a regular-file control.
- The entire existing and new `internal/liveverify/affected/golang` test package
  passed (9.159s). The unchanged live repository guard
  `TestRepositoryImportGraphReaders_V14409_V14362` passed (4.225s).
- Disabling both proof checks makes all reported and unknown-form regressions fail
  (mutation run exit 1); the exact production bytes were restored afterwards.
  Retained log SHA-256 values: whole package
  `d6f692a98eb5e3ad7485b0eac72d6b48540e732295ea530397e8d13e5aada431`,
  repository guard `a66c224f9336e76bf00291c2e40309729fad8a9481aecf37bdce3ababf679bdc`,
  mutation `9b685e31c029a722362c9f8b274bb98fcc38d39745ac0b8a542f8a772120bcc8`.
- The first whole-package attempt exposed a self-referential MapFS symlink stack
  overflow through the unsuitable Stat-based lookup. The directory-entry proof
  fixes that failure; no assertion was weakened.

Dogfood routes: retained pre-change capture reused as instructed; `affected --base`
used from a lane build (UNKNOWN: selected 401, excluded 476, 68 unknowns). Only the
changed package and the relevant existing repository guard ran. Global selection,
repository-wide gates, model-backed review, publication and Tasks writes are excluded
by the repair contract; the orchestrator owns independent review and host gates.
Final CEM binding/check outcome is retained in the external handback. No seal.
Raw logs stay in `/home/russell/lanes/selmin-evidence/`; no raw log is committed.

Rollback: revert this repair's code, tests and requirement clarification together;
that restores the two known P1 gaps, so do not claim selection soundness after rollback.
No wire schema, golden, error code, exit code or frozen output is changed.
