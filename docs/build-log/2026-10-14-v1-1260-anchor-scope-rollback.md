# V1-1260: anchor pointers bind the patch they describe; a reset notes ref reads rollback

Date: 2026-10-14. Ticket: V1-1260 (P1 bug). Requirement: `CEM-V1-067` in
`docs/specs/cem-stable-v1.md`, accepted under the owner's standing auto-accept (2026-10-11).
1.0 scope: `PRS-V1-018` (1.0 scope per the V1-0020 dependency). Base:
`92c298a6e8fa141a3cda035f2d1d8e6c6ebbf431`.

## Defect

`cem anchor --map M --commit REV` attached HEAD's committed map pointer to any REV, and
`cem provenance` checked only the pointer's path, blob and digest. A sibling commit with the same
parent therefore read `verified` for a map that does not describe it. Resetting
`refs/notes/corvint` to its parent made the anchor read as absent, not as rolled back.

The ticket's third clause (both paths refuse `cem/1.0` because they call `wire.ParseMap`) was
already fixed before this change: V1-1278 (d5994829b and 744e8fc44) moved anchoring to
`wire.ParseStable`, and provenance reads the map only through the pointer.

## Decision

- New pointer profile `corvint-cem-anchor/1`. It keeps the `/0` members and adds `base`,
  `patch_sha256` and `target_tree`. These are the ticket's `base`, `patchSha256` and `targetTree`,
  spelled in the pointer's existing snake_case member style. `base` and `patch_sha256` are copied
  from the map's `change.baseRevision` and `change.patchSha256`. `target_tree` is `REV^{tree}`.
- `Anchor` recomputes the canonical `base..REV` patch with the one recipe every CEM consumer uses
  (`gitauth.Repository.CanonicalDiff`, which excludes the map sidecar). When the patch does not
  hash to the map's `patchSha256`, anchor refuses with the new code `anchor-scope-mismatch` before
  it writes any note. There is no second diff recipe.
- `pointerState` keeps the path, blob and digest checks. It then reports:
  - `unscoped` for a `/0` pointer;
  - `scope-mismatch` when the map's `baseRevision` or `patchSha256` differ from the pointer, when
    `REV^{tree}` differs from `target_tree`, or when the recomputed patch does not hash to
    `patch_sha256`;
  - `scope-unavailable` when the patch cannot be derived;
  - `verified` only when every check passes.
- A `/0` pointer that carries scope members, or a `/1` pointer missing one, is `malformed`.
- `cem provenance --commit REV --notes-tip OID` adds the member `notes_tip` with `ref`, `pinned`,
  `live` and `state`. When OID is not an ancestor of the live `refs/notes/corvint`, or the ref is
  absent, `state` is `rollback` and the anchor row is a `rollback` row. The read finds the live tip
  by an exact refname match, so a prefix match cannot pick up `refs/notes/corvint/x`, and checks
  ancestry with `rev-list pinned ^live`. Without the flag the result has no `notes_tip` member,
  and the read is unchanged apart from the scope states.
- Anchoring stays an explicit operator step, with no default-on anchoring. Provenance stays
  read-only: the tests assert that every ref and the porcelain status are unchanged after each read.

## Independent review

One Codex read-only round (`codex exec -s read-only`) found no path to `verified` when the map's
base or patch digest disagrees with the canonical `base..REV` patch or the tree differs, and no
provenance write. It reported five findings, all fixed in this change:

1. Medium. `Decoder.More()` let a pointer followed by a stray `}` read as well formed; the read
   now requires end of input after the one object, so such a note is `malformed`.
2. Medium. `--notes-tip` accepted revision names, so `--notes-tip refs/notes/corvint` resolved
   to the live tip and always read `current`; it now requires a full object ID
   (`invalid-arguments` otherwise).
3. Medium. The tip comparison and the note read were separate reads of the ref. With
   `--notes-tip`, the live tip is read again at the end and a moved ref is refused
   `provenance-notes-moved`. This guard has no deterministic test (it needs a concurrent writer).
4. Low. A `/0` pointer with an empty, null or case-variant scope member read `unscoped`; member
   presence is now checked by name, so it is `malformed`.
5. Low. The no-write assertion compared only refs and porcelain status; the rollback test now
   also compares the object count and the index and config bytes.

## Scope widening after review

After the Codex round, the orchestrator widened the lane's scope to cover `cmd/corvint/cem_anchor.go`,
`cmd/corvint/cem_anchor_test.go`, `cmd/corvint/help.go`, and the
`UC-EVIDENCE-CARRYING-COMPLETION` use-case receipt repin. The CLI flag needs the first file. The CLI
interop test anchored a map whose base was absent, and the repin is required because
`cem_anchor.go` is a pinned subject. The reviewed diff covered these files, and the one-round cap
holds, so there was no second Codex round.

## What `verified` now implies

A `verified` anchor row for REV implies all of the following:
- the `refs/notes/corvint` note on REV is a `/1` pointer;
- its map blob is the one held at `cem_commit:map_path` and hashes to `map_sha256`;
- that map parses as `cem/1.0`, and its `baseRevision` and `patchSha256` equal the pointer's
  `base` and `patch_sha256`;
- `REV^{tree}` equals `target_tree`;
- the canonical `base..REV` patch hashes to `patch_sha256`.

In short, the anchored map describes exactly REV's change from its base.

It does not imply that:
- the map's citations or criteria verify (that is `cem verify`);
- the change has any particular author;
- the notes ref was not rolled back, unless `--notes-tip` was given with a tip the operator
  retained from a trusted source.

## Wire and schema

- New notes use pointer profile `/1`. `/0` notes stay readable and read `unscoped`.
- The provenance result gains the optional `notes_tip` member and the anchor states `unscoped`,
  `scope-mismatch`, `scope-unavailable` and `rollback`.
- The stable map, `cem-stable-verification/1` and the other provenance rows are unchanged.

For readers the change is additive. It is not additive for a consumer that trusted a `/0`
`verified` row, because that row now reads `unscoped`.

## Superseding notes

- Decision 0355 is not rewritten. Its `/0` pointer and its `verified` meaning (path, blob and
  digest only) are superseded by `CEM-V1-067`.
- The same applies to the `/0` pointer shape and the state list in `falsifiable-packet-v0.md`
  FPK-V0-037/038. Those clauses stay as the historical v0 record.
- Decision 0523 proposed the requirement as `CEM-V1-087`. The ticket and lane charter number it
  `CEM-V1-067`, and 087 stays unassigned. The charter's spare IDs `CEM-V1-068..069` were not
  used, because the CEM 1.0 expert panel log maps them to V1-1266.

## Evidence

- `internal/gitnotes`:
  - `TestCEMV1067SiblingCommitIsScopeMismatch` covers a sibling commit with the same parent.
    Anchor refuses it with `anchor-scope-mismatch`, and the notes ref does not move. Four forged
    `/1` notes on the sibling all read `scope-mismatch`: one copied unchanged, one with the tree
    aligned, one with the patch and tree aligned, and one with the base aligned.
  - `TestCEMV1067VerifiedImpliesTheAnchoredPatchIsBaseToREV` checks every `verified` row:
    `patch_sha256` equals the recomputed canonical `base..REV` digest, and `target_tree` equals
    `REV^{tree}`. The base commit is refused.
  - `TestCEMV1067ResetNotesRefIsRollback` covers a reset ref and a deleted ref, both reading
    `rollback`, and pinned ancestors reading `current`. With no pin, the `notes_tip` member is
    absent. An unresolvable tip is refused.
  - `TestCEMV1067LegacyPointerIsReadableAndUnscoped` checks that a `/0` note is readable, reads
    `unscoped`, and still reports `digest-mismatch`. Mixed shapes read `malformed`.
- `cmd/corvint`: `TestCEMAnchorAndProvenanceInteropThroughTheCLI` now anchors a map rebound to the
  fixture's own canonical patch, and checks `--notes-tip` after a deleted notes ref.
- `internal/cem/cli`: `TestAnchorActionsParseLikeTheirSiblings` accepts `--notes-tip`.

## Rollback

Revert the V1-1260 commit. Anchors are written as `/0` again, `/1` notes read `malformed`, and
`--notes-tip` is an unrecognized argument. No committed map, seal or golden depends on the pointer
profile.
