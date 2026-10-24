# Compact per-file anchor table for adoption provider records

Date: 2026-10-24. Lane: claude/corpus-anchors-1664 (stacked on claude/docs-corpus-gh1662,
base bcbd5ded0). Issue: GitHub #1664, the residual that #1669 left open. Requirements
`DCP-V1-487..489` (`docs/specs/documentation-corpus-v1.md`) are proposed and experimental and
await the orchestrator's acceptance.

## Format

`corvint-corpus-adoption-provider/2` is the `/1` adoption record plus two top-level members:

- `files`: `[{repository?, revision?, path, blob, sha256}]`, at most 100,000 entries. An absent
  repository or revision is the record's `source` one; a present one, even empty, is kept.
- `anchor_defaults`: optional `{authority?, evidence_kind?, reason?}`.

Every anchor of a `/2` record, in evidence `anchors` and observation `step_evidence`, is compact:
`{file, start_line, end_line, span_sha256, symbol?, region?, authority?, evidence_kind?, reason?}`.
`file` is a required zero-based index. An absent authority, kind or reason is the default's; a
present one, even empty, is kept, so it fails the `/1` anchor check as an empty `/1` member does.
A `/1` file member inside a compact anchor is an unknown
member. The span hash stays per anchor, because the anchor check verifies it per span.

## Decisions

- **Expand at decode, admit as `/1`.** `decodeProviderRecord` (`internal/doccorpus/compact_anchor.go`)
  is the only provider decode site. It first runs today's closed decode, so every other schema
  costs what it did. Only when that decode fails or reads the `/2` schema does it scan the
  top-level `schema` with `jsontext`, skipping every other member. Only for `/2` does it scan
  again for `source`, `files` and `anchor_defaults`, so a refusal inside them is located whatever
  the member order. It then decodes again
  with a json/v2 `UnmarshalFromFunc` that turns each compact anchor into its full `Anchor`. The
  record's schema is then set to `/1`. The manifest `/2` requirement, `checkAnchor`, shard
  identity, incremental reuse and capacity charges are therefore unchanged code paths. Capacity
  is charged on the compact pinned bytes.
- **No artifact format change.** The artifact, its anchors and receipts are unchanged, and `/1`
  inputs derive the same bytes, so the derivation version (`DCP-V1-471`) is not raised.
- **Refusals are located.** An index error is returned from the unmarshal function. json/v2 wraps
  it in a `SemanticError` carrying the pointer, and `locateDecodeError` reports the reason
  verbatim (for example `evidence/provider.json: /subjects/0/evidence/anchors/0: compact anchor
  file index 99 is outside the file table of 1 entries (offset ...)`).
- **No producer API.** The builder writes no `/2` record. The test file holds a two-pass
  `MarshalToFunc` encoder as the reference producer.
- **Bounds unchanged.** `maxPinnedInputBytes` and `maxProviderRecordBytes` stay 128 MiB, and the
  per-shard bound stays 64 MiB. `TestDCPV1489CompactAnchorSizeMeasured` asserts this.

## Measurement

`TestDCPV1489CompactAnchorSizeMeasured` builds a synthetic adoption record shaped like #1664:

- 28,500 inventory subjects, each with one exact one-line anchor.
- The first 17,400 units are file-level; the other 11,100 are symbol units in the same files.
- Hashes are 40- and 64-hex digests, and paths are about 35 bytes.
- The `/2` form uses anchor defaults for authority, kind and reason.

The anchor bytes of each encoding are its record size minus the same record without anchors.
Measured on darwin/arm64 with Go 1.27, run of 2026-10-24:

| Shape | Record without anchors | `/1` record | `/1` per anchor | `/2` record | `/2` per anchor | `/2` anchor bytes / `/1` |
| --- | --- | --- | --- | --- | --- | --- |
| 28,500 anchors over 17,400 files | 9,640,445 | 23,992,145 | 504 B | 16,476,674 | 240 B | 47.6% |
| 28,500 anchors over 2,000 files | 10,458,245 | 25,206,245 | 517 B | 15,219,744 | 167 B | 32.3% |

The `/1` cost of 504 to 517 bytes per anchor matches the issue's figure of about 530.

On the issue's shape, the 14.4 MB of anchors falls to 6.8 MB. Most of what remains is
irreducible: the span hash, plus one file entry for each of the 17,400 files that holds only one
anchor. The issue's provider shards would fall from about 100.5 MB to about 93 MB. Since #1669
(`DCP-V1-472`) charges those shards apart from the about 39 MB of source inputs, that corpus
already fits either encoding. The compact form leaves about 41 MB of provider headroom under
the 128 MiB (134.2 MB) bound, where `/1` leaves about 34 MB.

Decode time on the same records, in one run (not a benchmark; the host was shared):

| Shape | `/1` decode | `/2` decode |
| --- | --- | --- |
| 17,400 files | 79 ms | 181 ms |
| 2,000 files | 127 ms | 164 ms |

The compact path costs two top-level scans and a second decode, so `/2` takes up to about twice
as long. It is
paid only by `/2` inputs.

## Evidence

- `TestDCPV1487CompactAnchorTableEquivalent`: the adoption fixture's record, re-encoded as `/2`
  both without and with anchor defaults, builds byte-identical subjects, claims, relations,
  journeys and details. The artifact opens, and `/1` shards still build.
- `TestDCPV1488CompactAnchorRefusals`: thirteen refusal cases, with the located reason and
  pointer checked where they apply:
  - an index out of range, a negative index or a missing index
  - a `/1` member inside a compact anchor
  - an unknown member in the file table or in the defaults, which sort before `schema`
  - an explicit empty reason under a reason default, and an explicit empty file revision
  - a file table of 100,001 entries
  - a file table or defaults in a `/1` record
  - a wrong file revision
  - a `/2` record under a `/1` manifest

## Independent review

One Codex review round (gpt-6-astra, read-only) found two P2 issues and one P3 issue. All three
are fixed with tests in `TestDCPV1488CompactAnchorRefusals`:

- **P2: explicit empty members took defaults.** An explicit empty `reason` took the default, and
  so did an explicit empty authority, kind, file repository or revision. Those members are now
  pointers, so only an absent member takes a default.
- **P2: a refusal inside `files` or `anchor_defaults` was located only when `schema` came
  first.** The schema is now found by its own scan.
- **P3: the file-table bound refusal was unlocated.** It now names the input and `/files`.

## Rollback

Remove `compact_anchor.go` and restore the `decodeNamed` call in `shards.go`. `/1` corpora are
unaffected. Corpora that pin `/2` records are then refused until they are rewritten as `/1`.
