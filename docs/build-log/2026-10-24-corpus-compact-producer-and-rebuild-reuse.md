# Compact provider producer and same-derivation rebuild reuse

Date: 2026-10-24. Lane: claude/gh1664-1667-corpus-build, base b64eb76a1. Issues: GitHub #1664
(a producer for the compact `/2` provider encoding) and #1667 (`docs corpus build --artifact` paid
a full rebuild). Requirements `DCP-V1-530..534` (`docs/specs/documentation-corpus-v1.md`) are
proposed and experimental and await the orchestrator's acceptance.

## Compact producer (#1664)

- **No format change.** `/2` is already readable (`DCP-V1-487..489`); only its encoder lived in a
  test. `EncodeCompactProvider` (`internal/doccorpus/compact_anchor.go`) is that encoder moved into
  the package, and `TestDCPV1530CompactEncoderMatchesReference` keeps the test copy as the
  reference and requires byte equality for three records and three default sets.
- **Opt-in producer.** `docs corpus compact-provider --input FILE` reads one `/1` record or shard
  under the same bound as a build, takes the most frequent anchor authority, kind and reason as
  defaults (a tie goes to the smallest value; a record without anchors gets none) and prints the
  `/2` bytes. It decodes its own output again and refuses unless the expansion is the input record,
  so a written `/2` file admits exactly the input's records (`DCP-V1-531`). Nothing rewrites a
  repository's provider files implicitly.

## Rebuild reuse (#1667)

- **What is reused.** `--artifact OLD.json` always builds OLD's embedded manifest, so every shard
  digest of the new build is OLD's. Reuse is therefore of the whole artifact, not per shard: when
  the running builder is attested and OLD records the running derivation, the same engine, the
  digest of this manifest and of the running profile, and OLD's tree is the one a pin of the
  manifest yields, `RebuildArtifact` (`internal/doccorpus/rebuild.go`) keeps OLD's records,
  rewrites only the builder identity and artifact digest, and admits the result as a full build
  does (`DCP-V1-532`). Every other case is today's full build (`DCP-V1-533`).
- **Trust boundary.** The pin still reads and checks every pinned input, so a refusal is the full
  build's own refusal. OLD's records are trusted to the extent the verified read already trusts an
  artifact of the running derivation; a hand-edited artifact with a recomputed digest is reused as
  it stands, exactly as `docs corpus read` would serve it.
- **Refusal kept.** An unrecorded-derivation artifact is never reused; it is rebuilt in full, and
  the verified read still refuses it (`TestDCPV1533RebuildRederivesOtherwise`).
- **Equality proof.** `TestDCPV1532RebuildReusesSameDerivation` compares the reused bytes with a
  full build of the same manifest for a record, two shards and a 48-file, six-shard inventory, and
  counts zero compiler builds.

## Measurement (DCP-V1-534)

`CORVINT_REBUILD_MEASURE_FILES=9500` on the WSL2 box (linux/amd64, go1.27.3): a synthetic
inventory of 9,500 Go files over eight `/1` shards, 38,010 subjects, 28,511 claims, 28,500
provider anchors, a 61,707,283-byte artifact. Each mode runs in its own process; a third process
prepares the corpus, because a Linux child's peak RSS starts at its parent's at fork.

| Mode | Wall | Peak RSS |
| --- | --- | --- |
| full rebuild, another derivation (before) | 5.012 s | 601 MiB |
| reuse, another attested revision (after) | 2.521 s | 421 MiB |

darwin/arm64 for reference: 5.39 s / 687 MiB before, 2.47 s / 475 MiB after.

## Limits

- A Go toolchain or engine change is a derivation change and still rebuilds in full; the issue's
  11 s / 1.6 GB rebuild from an earlier builder is that case and is not sped up.
- The remaining reuse cost is the pin, which builds a context index per pinned revision only to
  verify the inputs.
