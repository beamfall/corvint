# Corpus rebuild from the embedded manifest

Date: 2026-10-24. Lane: claude/corpus-rebuild-1667, stacked on claude/docs-corpus-gh1662
(base bcbd5ded0). Issue: GitHub #1667, the residual left after `DCP-V1-471`. Requirements
`DCP-V1-495..498` (`docs/specs/documentation-corpus-v1.md`) are proposed and experimental and
await the orchestrator's acceptance.

## Decisions

- **Flag on build, not a new operation (DCP-V1-495).** `docs corpus build` takes exactly one of
  `--manifest` or `--artifact`, as `delta` and `carry-forward` already do. A rebuild is a build:
  same thresholds, admission, pin verification and output. A separate `rebuild` operation would
  have duplicated that path and its help.
- **Manifest only, no record decode (DCP-V1-495, DCP-V1-496).** `ManifestOfArtifact` unmarshals
  only `schema`, `manifest`, `manifest_sha256` and `sha256`. Other members are skipped, so a
  future derivation that changes record members can still rebuild an older artifact. It checks
  the canonical tail and content digest, a `corvint-evidence-corpus/` schema, a closed manifest
  decode and the manifest digest. It does not check the builder, derivation or pins, because
  that is the refusal the rebuild repairs. The build then rereads every pinned input from Git and
  refuses `input changed after pinning` on any blob or SHA-256 mismatch.
- **Reads never rebuild (DCP-V1-497).** The verified read still refuses. Every `builder
  differs:` refusal now ends with the explicit rebuild command. The command writes to stdout
  only, and the guide and help say to write a new file and rename it, because a shell redirect
  onto the artifact being read truncates it before the build reads it.
- **MCP case (DCP-V1-498).** Since `DCP-V1-471`, a served artifact from another attested
  revision of the same derivation already answers. Nothing was still broken, so the requirement
  is a regression test. It also covers a changed derivation, which is refused with the rebuild
  hint; a running server after a rename-over rebuild, which names its restart (`DCP-V1-143`); and
  a restarted server, which answers.
- **Which merges change the derivation version.** The `build` help topic states the rule from
  `DerivationVersion`: only a merge that can alter the records built from the same pinned inputs
  changes it. `TestDCPV1471DerivationVersionGuard` enforces it. Per-release notes naming such
  merges are not produced here (NEEDS-OWNER: release-notes policy).

## Independent review

One Codex round (`gpt-6-astra`, read-only) found three P2s, all fixed with tests that failed
first. (1) The rebuild applied only the 128 MiB ceiling, so a `/1` artifact padded past its 4 MiB
bound with skipped members still yielded a manifest; it now applies the schema's bound. (2) Flag
exclusivity checked values, so `--manifest= --artifact=X` built; it now checks option presence
and refuses an empty value. `delta` and `carry-forward` keep the old check (V1-2543). (3) The
build-counter assertion in the `DCP-V1-496` test ran in parallel with other builds; it now runs
serially. It found no digest bypass. Duplicate members are refused by the decoder. A load-dependent
failure of `TestDCPV1143LazyLoadAndMemo` in an untouched package is filed as V1-2544.

## Format and benchmark note

There is no on-disk or wire encoding change. The artifact, manifest and receipt formats are
unchanged, and old artifacts are read only for their embedded manifest. The help text changes,
so `internal/capcatalog/catalog.json` is regenerated. A rebuild costs one top-level parse of the
artifact bytes, which skips record members, plus the same build `--manifest` runs. The build
dominates. Large-artifact timing was not measured (`NOT_MEASURED`). The tests cover only
correctness and byte identity.

## Rollback

Remove `--artifact` from `build`, make `--manifest` required again, delete `rebuild.go` and drop
the hint from `sameDerivation`. No stored state needs migration.
