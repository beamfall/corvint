# V1-1986: corpus anchors into declared additional repositories

Date: 2026-10-19
Ticket: V1-1986 (GitHub issue beamfall/corvint#1243).
Requirements: DCP-V1-175..179 (`docs/specs/documentation-corpus-v1.md`), accepted under the
owner's standing auto-accept of lane requirements (2026-10-11). No accepted requirement is amended
and no error code is added: refusals reuse `corpus-refused` and `corpus-input-unavailable`.

## Problem

A corpus could pin evidence only in its primary repository. `checkAnchor` refused any anchor whose
repository was not `manifest.repository.id`, pinning resolved every revision in the primary object
store, and freshness compared every input with the primary working tree. Documentation that
describes a library or service in a sibling repository therefore could not cite it at all.

## Decisions

- **Explicit declaration, local checkout locator.** The manifest gains
  `additional_repositories: [{id, revision, root}]`, sorted by `id`. `root` is a clean relative
  path from the primary root to a local checkout. Build, `Open` re-derivation and freshness all
  resolve the repository from the manifest alone, so no CLI or MCP flag is added, nothing is
  fetched and nothing is discovered. The locator grants no identity: the root commit of the
  declared revision in that checkout must equal `id` (invariant 1).
- **Revision owns repository.** The repository of a scope, input or anchor is the one whose pinned
  revision it names. Declared revisions must be distinct across repositories, so the mapping is
  unambiguous. An anchor naming any other repository, including an undeclared one, is refused
  (invariant 2). Provider records still name the primary repository as their source, and the
  native provider stays primary-only. Project authority is unchanged (invariant 3).
- **Per-repository freshness.** `liveFreshness` now runs the existing scope/input comparison once
  per repository against that repository's checkout. Primary checks are unchanged. Additional
  reasons carry the prefix `additional repository <id>: `. A multi-repository overlay has no
  reusable observation key, because the corpus bridge memo is keyed by the primary observation
  alone. The bridge verifies each memoized pin in its owning checkout.
- **Bounds.** At most seven additional repositories; declared revisions count toward the existing
  eight-revision bound.

## Measurements and evidence

- Byte identity (DCP-V1-179): a scratch test, not committed, built the plain `fixture` and
  `providerFixture` corpora with fixed Git dates at base 62fd18a0 and on this branch. Artifact
  and manifest SHA-256 were equal: plain artifact `8674eee0…3c73`, manifest `e683bbf1…68c9`;
  provider artifact `939725a6…3719`, manifest `e61a976c…0b36`. `TestDCPV1179SingleRepositoryManifestUnchanged`
  keeps the absent-field and reusable-observation properties.
- `TestDCPV1175AdditionalRepositoryAnchors`: a claim and subject anchored in repository B build.
  `search` and `get` cite B's id and revision. The corpus stays `fresh` after an unrelated commit
  in A and goes `stale` naming `doc.md` after B changes.
- `TestDCPV1176AdditionalRepositoryRefusals` covers these refusals: undeclared repository, the
  primary identity, bad roots, a duplicate, a shared revision, a checkout resolving to the primary
  root, a missing checkout
  (`corpus-input-unavailable`), an identity mismatch, and anchors at B's revision naming A or an
  undeclared id.
- `TestDCPV1178BridgeVerifiesPinsInOwningCheckout` covers the bridge memo pin check.
- Fail without the fix: reverting `providers.go` fails 1175 (anchor refused) and 1176 (an anchor
  naming A at B's revision builds). Reverting `query.go` fails 1175 (pinned corpus reported
  `stale`). Pointing `verifyPins` at the primary root fails 1178.
- Full `internal/doccorpus` (349 s), `internal/mcp/corpusbridge`, `internal/doccorpus/selfcorpus`,
  `cmd/corvint-corpus-mcp` and `internal/corpusindex` packages pass.

## Not done (residuals)

- An undeclared revision absent from the primary store refuses with the existing untyped Git
  error, as before this change. It is not typed `corpus-input-unavailable`.
- Still primary-only: the native provider, test-knowledge anchors, `/2` behavior repositories (a
  non-primary member stays a `repository-evidence-unavailable` gap), symbol re-resolution and the
  receipt's top-level `repository`. A `docs_get --path` lookup does not distinguish equal paths
  in two repositories.
- NOT_RUN: `cmd/corvint` package tests (about 600 s; no CLI code changed), `make gate`, and a live
  two-checkout MCP session.

## Rollback

Revert the commit. Manifests without `additional_repositories` are byte-identical, so no stored
single-repository corpus needs migration. A manifest that uses the field is refused after the
revert by the strict decoder, since the field is unknown there.
