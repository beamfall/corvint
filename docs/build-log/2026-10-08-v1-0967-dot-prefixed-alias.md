# 2026-10-08: Dot-prefixed bare web specifiers reach the alias arm (V1-0967)

## Intent

Ticket V1-0967 (V1-0958 review finding P2) records that `Resolve` and `buildWebImportGraph` in
`internal/contextindex/webresolve.go` read every specifier starting with `.` as relative.
TypeScript reads a specifier as relative only when it matches `^\.\.?($|/)`, so a tsconfig
`paths` key such as `.api/*` claims `.api/client` as a bare name. Corvint skipped it in the alias
arm and resolved it as a missing relative path, so `impact` on the target missed the importer and
added no `GPK-V0-080` disclosure. The change adds proposed `GPK-V0-082` to
`docs/specs/go-production-kernel-migration-v0.md`.

## Decisions

- **One whole-segment predicate in both places.** `webRelativeSpecifier` accepts `.`, `..`,
  `./...` and `../...` only. `Resolve` and `buildWebImportGraph` both call it, so the exported
  resolver and the `impact` alias arm agree.
- **The rule (c) oracle arm is unchanged.** `webSpecifierAdmitted` in `reverseimports.go` still
  admits any leading dot, because its rows are the frozen `impact-web` parity corpus's bytes
  (`GPK-V0-027`, `GPK-V0-033`). A dot-prefixed bare specifier now reaches both arms. The oracle
  arm joins it onto the importer's directory, which normally finds nothing. When both arms find
  the same importer, `impact` keeps one row per importer path.
- **An unmatched dot-prefixed name is disclosed.** It is now a bare specifier, so
  when it resolves to no repository file or declared package it is counted in the `GPK-V0-080`
  incompleteness line and in the `bare-import-unresolved` unknown. Before this change it was
  silently a missing relative target.
- The ticket's literal reproduction used `src/generated/client.ts`, which `impact` excludes as a
  generated path (`OUT_OF_SCOPE`), so the regression uses `.api/*` and `src/api/client.ts`. The
  resolver defect is the same.

## Evidence

`TestImpactResolvesDotPrefixedAliasImporters` (`internal/contextindex/webresolve_test.go`),
run against the unfixed source on this branch:

```
Resolve(src/app.ts, .api/client) = {Target: State:unresolved}, want {Target:src/api/client.ts State:repository}
reverse importers = map[src/relative.ts:map[imports ./api/client:true]], want src/app.ts through the `.api/*` alias
uncertainty = [no language-specific verification command is known for this repository], want "... 1 bare import specifiers in 1 sources ..."
```

It passes with the fix. The existing `TestWebImportResolverFixture`,
`TestImpactResolvesTSConfigPathAliasImporters`, `TestImpactSyntaxReportsBareImportUnresolved`
and `TestImpactKeepsFrozenBareReadingWithoutManifest` are unchanged and pass.

## Rollback

Restore the leading-dot test in `Resolve` and `buildWebImportGraph`. No index, snapshot,
receipt or trace changes.
