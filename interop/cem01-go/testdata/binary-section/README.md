# Canonical binary section case (CEM-V1-934 to CEM-V1-936, V1-3443)

Git reports a changed file that holds a NUL byte with its canonical
full-index binary section (`index <full>..<full>` and `Binary files ...
differ`) instead of hunks. The native producer binds such a section by blob
identity beside the textual hunks (CEM-V1-980 to CEM-V1-985). The portable
consumer admits the same section on its stable routes under the revised
packet (`protocol/cem-1.0/stable/ALGORITHMS.md`, revision r2), so both
readers reach one verdict on one map.

- `repository.recipe.json` builds the flat-tree repository like
  `../non-regular/`. Its `.bin` files hold a NUL byte. The target edits
  `text.txt`, modifies `image.bin`, adds `added.bin` and deletes
  `removed.bin`: one section of each binary shape beside one textual hunk.
- `stable.json` is the exact `corvint cem prepare` map for that range. Its
  one hunk is `unknown` (`no-evidence`); its `patchSha256` pins the
  canonical patch with all four sections.
- `EXPECTED.json` gives the shared verdict: ACCEPT at `complete`, with only
  `text.txt` mapped and the three binary paths bound by object ID.

The native reader (`internal/cem/verify`) and the portable reader (both
`verify-stable` and `verify-stable-canonical`) assert the same file, and both
refuse the map once its patch digest is altered.
