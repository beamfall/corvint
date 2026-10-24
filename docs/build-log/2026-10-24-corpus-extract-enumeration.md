# Extraction enumerates large revisions in one verified level walk

Date: 2026-10-24. Lane: claude/gh1662-extract-enum (base b64eb76a1). Issue: GitHub #1662, latest
re-test on main `b64eb76`. Requirements `DCP-V1-520..522`
(`docs/specs/documentation-corpus-v1.md`) are proposed and experimental; owner acceptance is open.

## Problem

`docs corpus extract` resolved extractor globs by walking `RevisionFS` with `fs.WalkDir`. Each
`ReadDir` is one verified `cat-file` call, so enumeration cost one Git operation per directory.
A 13,806-file, 3,333-directory Rails monorepo hit the 1,024-operation bound and was refused with
`git-budget-exceeded` before any file was read (38 s on the reporter's host).

## Decisions

- **One batch per directory level.** `gitauth.Repository.RevisionPaths` uses one operation to pin
  the commit and its self-hashed root tree, then reads each directory level as one
  `cat-file --batch`. Each record must self-hash and carry the OID its verified parent names. This
  is the same verified walk `ResolvePaths` (`DCP-V1-155`) uses, and Git operations now grow with
  depth rather than directory count. A raw `ls-tree -r` was rejected: its output is not
  self-verified against the pinned objects (CEM-CB-023).
- **Prune by glob.** Until a glob's first `**` segment, each glob segment matches exactly one
  directory segment, so a directory whose path disagrees with every glob's leading segments is not
  read. `node_modules` stays pruned. A soundness test checks that every ancestor of each matched
  path is descended.
- **Bounds.** The walk refuses before reading the next level once it has listed 400,000 tree
  entries (`affected.MaxWalkEntries`) or read 32 MiB of tree output (`MaxPathSetTreeBytes`), or at
  `MaxTreeDepth` levels. The per-extractor 4,096-file record bound is unchanged.
- **No format change.** Inventory, profile, provider record and artifact formats are unchanged. A
  revision that extracted before yields the same bytes.

## Measured (linux/WSL2, go1.27.3, via heavy.sh)

These come from a scratch timing test that was not committed. It used one `def` extractor over
synthetic trees in which 3,000 `.rb` files match and the remaining directories sit outside the
globs. Times cover `BuildExtractorInventory` only:

| Tree | Before (b64eb76) | After |
| --- | --- | --- |
| 12,601 files, 3,543 dirs, `app/**/*.rb` + `lib/**/*.rb` | refused `git-budget-exceeded`, 1.14 s | 3,000 subjects, 0.38 s |
| 98,001 files, 20,143 dirs, same globs | refused `git-budget-exceeded`, 1.11 s | 3,000 subjects, 0.35 s |
| 98,001 files, 20,143 dirs, `**/*.rb` (nothing pruned) | refused `git-budget-exceeded`, 2.11 s | 3,000 subjects, 1.35 s |

The 38 s refusal was not reproduced on this host. Spawning about 1,000 Git processes takes about
1 s on WSL2, so the reporter's time reflects that host's per-process cost, which the new walk no
longer multiplies by directory count.

## Not delivered

`docs corpus discover-tests` legacy discovery (`behavior_legacy_discovery.go`) still walks
`RevisionFS` one tree per operation below its literal root (filed as a ticket). A real Rails
repository's `app/**/*.rb` match count may exceed the 4,096-file record bound. That is a separate
bound and is unchanged here.
