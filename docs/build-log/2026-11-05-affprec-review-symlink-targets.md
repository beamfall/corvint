# affprec review fix: declared scanners retain source-alias targets

Date: 2026-11-05
Owner direction: repair only the independent review's P1 symlink-target finding.
Requirements: AFP-V0-9000, AFP-V0-9001; existing cache bindings AFP-V0-391/392.

## Finding and decision

A scanner that opens `probe/check.go` follows its symlink to valid Go source
in `data/imports.txt`. Editing only that target can introduce a forbidden Tasks
import, while the declaration's `*.go` and `go.mod` globs match neither the
changed path nor a dependency supplied by the existing Go-package alias logic.
The latter observes a different relationship and intentionally requires a Go
source target in an observed package.

Resolve scanner aliases separately from compile dependencies. Share one lazy,
bounded directory traversal among the declared source scanners, including
source-free directories and targets outside the alias's module or under ignored
source directories. Add literal, root-anchored reader globs for resolved regular
in-repository targets. Escape glob characters in filenames. When source aliases
exist, retain the repository's alias paths as dependencies too: retargeting a
non-Go intermediate hop can change the scanner's input without editing the final
target. Git metadata is outside this traversal. Unresolved, external or
non-regular targets, failed listings and exhausted entry/dependency bounds keep
the scanner's original lexical unbounded reasons. Fixture-only declarations
remain outside this traversal.

Use a cache-stamped source directory read rather than an unrecorded disk walk
or an always-uncacheable subtree walk. Stamp each directory before listing and
use the existing followed-link target stamp; adding an alias, editing its target
or retargeting it invalidates the derived graph. Keep all existing wire members,
error/exit codes, diagnostics, frozen outputs and assertions. This repair amends
the experimental reader contract and its test traceability without allocating a
new requirement. Rollback removes only the alias dependencies/traversal and the
source directory helper; it restores the independently observed underselection.

## Verification and limits

`TestBoundarySymlinkTarget_AFPV09001` copies the actual Tasks boundary scanner,
creates a source-free `probe` directory containing only a `.go` symlink, and
changes only the non-Go target from valid source to a forbidden Tasks import.
It asserts Tasks selection before and after the change, checks filenames with
glob characters, an ignored-source-directory target and a non-Go intermediate
hop, and retains exclusion of unrelated JSON.

The same regression with a Go overlay restoring the pre-fix observer fails all
four cases because Tasks is omitted. The overlay and output are retained outside
the repository. `TestReadKindsUnresolvedAlias_AFPV09000` retains the actual
unbounded-reader witness for dangling and cyclic aliases.
`TestBoundaryAliasCache_AFPV09000` requires real cache hits and misses across
alias creation, target-content change and retargeting on supported platforms.

Raw logs, the pre-fix overlay and command inputs are under
`/home/russell/lanes/affprec-evidence/review-fix/`. Whole-package results, log
digests, CEM/check outcome and numbered host friction are retained in the lane
handback. Existing pre-change receipts are reused; no change-start capture ran.
The affected route was used before testing and selected both touched packages;
its installed binary's stale warning and UNKNOWN frontier remain explicit.
The independent review of this repair and host qualification are reserved to
orchestration. Repository-wide gates, doc-gates, selection-baseline, publication,
native ticket writes and sealing are outside this repair round. No new performance,
Darwin, Windows or external interoperability claim is made.
