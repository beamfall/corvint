# Range impact: refuse an over-bound range before copy detection (V1-1881)

Date: 2026-10-21. Base: `5cf75daf0` (`origin/main`). Requirement: `ERI-V0-060`
(`docs/specs/expanded-range-impact-v0.md`), accepted under the owner's standing lane auto-accept
(2026-10-11).

## Finding

The ticket attributes the over-bound refusal's cost to decoding the snapshot. On this host the
snapshot is not the main cost. The range diff (`diff-tree --find-renames --find-copies-harder`) is.
That diff compares every added file with every base-tree file before the path count is checked.

The measurement used a scratch clone of `origin/main` with a fresh snapshot for each binary. The
range was `--base HEAD~60`: 2,193 paths, of which 1,575 were added and 618 modified.

- Copy-detecting diff alone: 5.42 s wall.
- The same diff without rename or copy detection: 0.03 s.
- `git status`: 0.07 s.

## Decision

The conservative option keeps every refusal that precedes the count in the same order: snapshot,
worktree, module and ancestry. A pre-load refusal would have reordered those, and would have needed
the snapshot's module and source tables to match them byte for byte. So the snapshot is still
loaded, and the lower bound runs at the point where the exact diff used to start.

The lower bound is the rename-free count minus the smaller of its additions and deletions. Detection
never adds an entry, so this is a floor on the exact count.

The exact parse validates every rename and copy source before it counts. A copy source can be any
unchanged base-tree path, so validating the changed paths alone would be unsound. Corvint's own tree
holds one such path (`interop/cem01-go/.../caf\303\251.txt`).

For those paths, the lower bound offers each unsafe unchanged path as the only copy source for the
additions. A path that fails the similarity threshold there cannot pass it among more candidates. If
any copy is found, the exact diff decides. Partial clones also defer to the exact diff, because it
reports missing blobs. The test proves both deferrals, including the unsafe-copy case whose exact
refusal names the path.

## Measurement

Before: installed `corvint` 1.0.0-rc.3 build 696, whose range code matches this base. After: this
branch's binary. Both ran `corvint impact --base <HEAD~60>`, three runs each, on a quiet host.

| | wall (s) | user (s) | max RSS |
|---|---|---|---|
| before | 6.04, 6.01, 7.37 | 5.24, 5.31, 5.39 | 434-595 MB |
| after | 0.55, 0.55, 0.55 | 0.43, 0.42, 0.42 | 434-435 MB |

The refusal bytes are identical. An in-bound range (`HEAD~1`, 88 paths) printed byte-identical
receipts from both binaries.

## Residual

AC1 asks for the refusal without decoding the snapshot. That part is not met. The snapshot load
remains about 0.4 s and 434 MB RSS, and it stays until the preceding refusals can be evaluated
without the snapshot.

Rollback: remove `rangeCertainlyExceeds` and its call.
