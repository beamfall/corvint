# Non-regular entry case (CAX-V0-020 to CAX-V0-024, V1-2974, #1950)

A cem/1.0 map covers only mode 100644 and 100755 files, so a range that
adds a symlink is refused whole with `unsupported-tree-mode`. The additive
`cem-non-regular/1` profile (`docs/specs/cem-additive-profiles-v0.md`) maps
every regular-file hunk of such a range and records each non-regular changed
path as an explicit per-path unknown. It never reads a link target as content
and never follows a link.

- `repository.recipe.json` builds the flat-tree repository like
  `../multi-runner/`; `links` entries are mode-120000 blobs. The target edits
  `app.txt`, adds `notes.txt`, adds the symlink `escape` (pointing outside the
  tree) and retypes `config.txt` from a regular file into a symlink.
- `stable.json` is the exact `corvint cem prepare --non-regular unknown` map
  for that range. Its two hunks are `unknown` (`no-evidence`); its
  `patchSha256` pins the canonical patch with every section of the two
  non-regular paths removed.
- `EXPECTED.json` gives the rows both readers report beside the result and the
  two verdicts: the opted-in reader ACCEPTs; a plain cem/1.0 reader REJECTs
  with `patch-digest-mismatch`, because the map does not describe itself and
  the reader must opt in. The cem/1.0 schemas, vectors and wire bytes are
  unchanged.

The native reader (`internal/cem/verify`) and the portable reader
(`verify-stable-canonical --non-regular unknown`) assert the same file.
