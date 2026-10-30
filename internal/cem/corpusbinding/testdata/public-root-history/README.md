# Original binding objects for the retained public-root CEM fixtures

This test-only fixture preserves CEM-V1-201's original Git-object comparisons after
decision 0587 started the new repository at public root
`9477995d776ddf88d7f6a82790dd9c22d51da765`. It covers only the eleven historical CEM
files that root retains for use-case receipts and affected-plan replay. It grants
no evidence coverage or correctness claim for the new root or current changes.

Source repository: `https://github.com/beamfall/corvint-history`, pinned at
`1c4d8793cf71cbb01b8509b68a6cb1502a40a13e`. Construction used the locally retained
original objects on 2026-10-30, with Git 2.54.0 (Apple Git-157). Every retained file
was compared byte-for-byte against that archive commit. The eleven original binds
were observed to be ancestors of that pin in the complete source store; this is
construction provenance, not an ancestry claim independently provable from the
sparse fixture.

`manifest.json` pins the exact object inventory, pack SHA-256/size, original
commit/path bindings, retained file identities, source notices and observed
ancestry identities. The test also pins the manifest SHA-256 independently.
`objects.pack.base64` is a textual encoding of a non-thin Git pack. It contains:

- 14 commits: eleven original binds, two frozen migration commits and the archive pin;
- 31 trees on the required original sidecar, migrated-map and archive-file paths;
- 12 map/source blobs and six original license/boundary notice blobs.

Total: 63 objects, 542,370 uncompressed content bytes; 249,286 pack bytes. The
base64 file is data, not an executable archive or a replacement repository. Parent
commits and off-path tree targets are intentionally absent. Exact commit-to-path
and source-blob bindings can be checked; full connectivity, complete history and
independent ancestry traversal cannot. A full `git fsck` connectivity claim would
be inappropriate. No unrelated source-file blobs, refs, remotes or alternates are
included.

The test initializes a fresh bare repository with an empty template, disables
inherited Git repository/object/config injection, imports the pack there and
checks the exact imported OID/type/size inventory. Original binding comparisons
then run unchanged. Historical admission also requires the exact public-root names
and bytes, including detection of removed files and matching archive path bytes.
Unknown/current maps resolve only in current repository Git. Missing current
binding evidence still fails under `CI=true` (V1-1334).

## Source notices

The files in `notices/` retain the exact original `LICENSE` and `LICENSING.md` bytes
named by each included commit. The archive pin carries Apache-2.0; the thirteen
older commits carry the earlier Corvint license and its recorded path-boundary
versions. These are historical fixture metadata with their original notices and
identities preserved, not a claim that the current root's notice silently relabels
all predecessor bytes. The loader checks the notice SHA-256, blob identity and
original commit path. The manifest records the commit-to-notice mapping.

## Reproduction

From an authorized local checkout of the complete pinned archive, use the sorted
`objects[].oid` list in the manifest as the exact stdin of:

```
git pack-objects --stdout --window=0 --no-reuse-delta --no-reuse-object --compression=9
```

Do not pass `--revs`, traverse history, create a thin pack or add unspecified objects.
Base64-encode the resulting pack with 76-column lines. Its SHA-256 and size must
match the manifest. Compression bytes can depend on Git/zlib versions; a differing
pack is not an authorized fixture update. The read-only original object inventory,
source ancestry observations and notices must be rechecked before any approved
fixture regeneration. Neither tests nor builds need network access or the complete
archive checkout.
