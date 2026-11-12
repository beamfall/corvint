# Affsym fix round 2: root edges of link targets (V1-5368)

The link-target part of the second review-fix round on the affstatus and V1-5368 stack, carried
from `docs/build-log/2026-11-11-affstack-fix-round-2.md` on branch lane/affstack-fix (38e5e0d1d)
when the symlink work was split from the held name-listing gate (see
`docs/build-log/2026-11-11-affstack-split.md`). Range base `origin/main`
`4c3c203336a6cc4a9772af39b1a771f34236138e`. AFP-V0-9965 stays proposed, with experimental
delivery; no requirement ID was added and no owner acceptance was recorded. The failing tests
went in first, and each fix has one recorded mutation. Source commits: 7cd31b96b (failing
rows), 73ea0ebce (P2-a), d4eec83e1 (P2-b), 69f0c4265 (P3).

## What was wrong

- **P2-a: `<tree>/` as a link text.** Cutting the tree or repository prefix from an absolute
  text that is the root plus `/` left the empty string. That reads as "no path", where it
  should be the root `.`.
- **P2-b: case-only re-entry.** A target that reaches the tree only when case is folded
  (`../REPO/c1` beside `repo`) is opened in the tree on APFS. `EvalSymlinks` keeps the case it
  was given, so the exact prefix check put the target outside.
- **P3: snapshot plans.** With a snapshot tree, a link that names the repository by its
  literal path was cut to the repository path. The same link reached through an alias, or
  through the physical path when the repository was given by alias, resolved against the
  tree's physical path only, and so read as outside.

## Invariants now enforced

- An absolute link text that is the tree or the repository followed by `/` resolves to `.`.
- `physicalTarget` returns the exact repository path when the target reaches a root. It returns
  `.` when the target reaches a root only under an ASCII case fold, or when the reached path or
  any root holds a non-ASCII byte. It returns "outside" otherwise. (The split moved the
  non-ASCII check ahead of the exact match; see the split entry.)
- The roots are the tree's physical path and, when the tree is not the repository, the
  repository's physical path. If the repository's physical path cannot be read, the links
  count as not listed.

## Precision cost

- When a root path holds a non-ASCII byte, every evaluated outside target resolves to `.`.
- A snapshot plan costs one more `EvalSymlinks` call.

## Hunt table

"Pre" is the result before the fixes. Every row passes at the head.

| Class | Test | Forms tried | Pre |
|---|---|---|---|
| Trailing slash (P2-a) | `TestSymlinksRootEdges_AFPV09965` | slash_physical and slash_alias, each in the physical, alias and both snapshot runs | `""` |
| Case fold (P2-b) | same test | case_rel, case_abs (physical and alias runs) | `""` |
| Snapshot (P3) | same test | snapshot run: slash_alias, repo_alias; snapshot_alias run: slash_physical, repo_physical | `""` |

The four snapshot rows above still failed after the P2-a fix and passed only with P3.

## Mutations

Each mutation was applied to the committed fix on the source branch, run, and restored with
`git checkout`.

| Fix | Mutation | Result |
|---|---|---|
| P2-a | `if false && inside == ""` | FAIL: 6 slash rows |
| P2-b | `false && ok` on the fold check | FAIL: 4 case rows |
| P3 | the repository root is never added | FAIL: the 4 snapshot rows |

## Open misses and limits

- **In-tree case mismatch.** A link whose target stays inside the tree lexically
  (`l -> Script` beside `script/`) is still matched case-sensitively. This is recorded in
  AFP-V0-9965's "Not modelled".
