# Guarded one-hunk citation replacement — V1-3168

`cem cite` only added evidence, and `DCW-V0-117` refused a plan row that would change an already
cited hunk, so correcting one row of an applied staged plan meant deleting
`.corvint/change.cem.json` and re-citing every stage. This entry records the design that replaces
one hunk's citations in place.

## Decision

- Core (`CEM-V1-694`, `CEM-V1-695`): `cem cite --replace --expected-hunk-digest DIGEST` and the
  guarded `cem mark --expected-hunk-digest DIGEST`. The digest (`cem-hunk-state/1`,
  `wire.HunkDigest`) covers the map's base revision and patch digest, the hunk ID, disposition,
  reason and ordered basis. It is checked after every ordinary cite check, so a stale old digest,
  stale base or patch, invalid span or policy refusal writes nothing, and the one atomic publish
  either lands or leaves the previous map. Coverage and discrimination witnesses are bound to the
  hunk and the patch, not the basis, so they are kept.
- Flow (`DCW-V0-390..392`): `DOGFOOD_CITATION_REPLACE=HUNK_ID=DIGEST[,...]` admits replacement for
  the named, planned hunks only. The first row the plan writes to such a hunk carries the guard;
  the flow's existing staged intermediate maps still publish the tracked map once. The
  `citation-plan-recites-cited-hunk` fix line now prints the exact recovery entry with the hunk's
  current digest. An entry whose hunk already holds exactly its rows replays, so the §4 clean
  rerun with the same inputs completes.
- Rejected: a fifth plan column (would change the row grammar every precheck, draft and remap
  reads), an unguarded `--replace` (a stale correction could silently drop a newer citation), and
  ordinal entries (an ordinal moves when commits add hunks; the full ID does not).

## Evidence

- `TestCiteReplaceCEMV1694`, `TestCEMV1694ReplaceFlagsAreGuarded`: positive replacement first,
  then stale digest/base/patch, out-of-range span, missing evidence and a failed publication each
  leave the map bytes identical.
- `TestChangeDCWV0390ReplacesOnlyTheNamedCitedHunk`: guard arguments only on the first row for the
  named hunk; seven invalid or stale entry shapes cite and mark nothing; replay of an applied entry.
- `TestDogfoodCitationReplaceCorrectsOneHunkOfAnAppliedStage` (built binary, the representative
  one-hunk correction): two applied stages over three hunks; the corrected stage-one plan is
  refused with the recovery entry; the rerun replaces hunk 1 only while hunks 2 and 3 keep their
  basis bytes, with no map deletion or stage restart; a stale digest leaves the map intact; the
  bound map replays to a complete pass and `dogfood check` PASS (`hunks=3 supported=3`).

Limits: no independent consumer or outcome evidence; the replacement adds no field to the cite or
mark result envelope, so a raw Core caller computes the digest from the documented formula or
reads it from the dogfood fix line.
