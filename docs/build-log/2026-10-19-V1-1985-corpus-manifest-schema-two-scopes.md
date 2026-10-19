# V1-1985: `docs corpus manifest` emits multi-scope /2 manifests

Date: 2026-10-19
Ticket: V1-1985 (GitHub issue beamfall/corvint#1241).
Requirements: DCP-V1-160..163 (`docs/specs/documentation-corpus-v1.md`), accepted under the
owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

`docs corpus manifest` inventoried exactly one scope, always wrote `corvint-corpus-input/1`, and
refused any scope over 4,096 files with `empty or oversized scope`, although a /2 manifest admits
100,000 records. A 29,600-file repository needed 78 runs and a hand merge.

## Decisions

- **`--schema 1|2`.** The issue's spelling; no existing corpus option selects a schema. Default and
  `--schema 1` keep /1; `--schema 2` writes `corvint-corpus-input/2` and bounds the inventory by
  `Manifest.recordLimit()` (100,000) instead of the /1 constant. Other values are `invalid-arguments`.
- **Repeated `--scope`.** `--scope` is the one corpus option exempt from the duplicate-option
  refusal. Scopes are sorted in byte order, so argument order never changes the manifest. A
  duplicate scope, or a scope nested in another requested scope, is refused (`corpus-refused`,
  naming both paths) rather than silently merged: every input then belongs to exactly one scope,
  and the caller sees an overlap instead of a manifest that differs from what they wrote.
  Multiple scopes are admitted for /1 too, under the /1 bound; `validateManifest` already accepted
  multi-scope /1 manifests.
- **Bound refusals name their numbers.** An empty scope names the scope; an oversized inventory
  names the scopes, the tracked-file count, the schema and its bound, and a /1 refusal points at
  `--schema 2`. The check runs before any input blob is read. The code stays `corpus-refused`.
- **Surgical `Inventory` edit.** `Inventory(…, scope, …)` is now a wrapper over
  `InventoryScopes(…, scopes, schema, …)`; scope membership walks a path's ancestors against the
  scope set (O(depth) per tracked path). The per-input `loadInput` loop and its Git budget are
  untouched, leaving that to V1-1984 (#1240).

## Identity

Default single-scope output is byte-identical: the base binary (99dac5234) and this branch printed
identical manifests for `internal/doccompiler` (8,819 B), `docs/specs` (59,367 B) and
`internal/doccompiler/draft.go` (813 B) at the base revision. Only the refusal text of an empty or
oversized scope changed (it used to be `empty or oversized scope`).

## Evidence

- `TestDCPV1160SchemaTwoManifestOfTwentyThousandFilesBuilds`: a 20,000-file fixture (two scopes of
  10,000 Markdown files, one commit via `git fast-import`) yields one /2 manifest with scopes `a`,
  `b` and 20,000 inputs, which `docs corpus build` accepts as a /2 artifact; the /1 inventory of
  one 10,000-file scope is refused naming scope, count and bound. Local timing: fixture 2.3 s,
  manifest 0.7 s, build 2.5 s, test 7.1 s.
- `TestDCPV1160DefaultSingleScopeUnchanged`, `TestDCPV1160SchemaFlagValues`,
  `TestDCPV1161ScopesSortedAndOverlapRefused`, `TestDCPV1162BoundRefusalNamesBoundScopeAndCount`
  (the /2 refusal above 100,000 is tested on the bound function: a 100,001-file fixture is not
  cheap).
- Fail without the fix: the `cmd/corvint` tests copied onto the base worktree fail three of four
  (`unsupported corpus option`, `duplicate corpus option`, `--schema 1` refused);
  `TestDCPV1160SchemaFlagValues` is a refusal guard that also passes at base; the `doccorpus` test
  does not compile at base (`checkInventoryBound` is new).
- `docs corpus manifest --help` and the flag matrix list `--schema`.

## NOT_RUN

`make gate` (owner preference for scoped work); the full `cmd/corvint` package (ran the corpus,
help and flag-matrix subsets).

## Rollback

Revert the commit; manifests produced earlier remain valid inputs to `docs corpus build`.
