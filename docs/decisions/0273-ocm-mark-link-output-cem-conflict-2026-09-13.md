# Decision 0273 — OCM mark and link refuse an explicit output naming the CEM file

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`corvint ocm prepare` already refuses an OCM path that names its CEM file (`output-path-conflict`),
but `mark --output` and `link --output` had no such guard: a focused test showed both overwrite the
CEM with OCM bytes. The call is to match prepare. An explicit `link --output` that names the
`--cem` file, equal after resolving against the root or, when both exist, the same file identity
(`sameInputFile`), fails `output-path-conflict` before any write. `mark` consumes no CEM, so it
compares an explicit `--output` with the path the CEM wire contract fixes, `.corvint/change.cem.json`
(`excludedPath`); a CEM kept elsewhere is not detected by `mark`. The default destination (the map
path itself) is unchanged. Amends `OCM-V0-007`. Pinned by
`TestOCMMarkAndLinkRefuseOutputAliasOfCEMPath`. No wire change and no requirement IDs added.
