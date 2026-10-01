# Typed trace CI maintenance (2026-10-01)

## Observed failure and accepted scope

PR #413 at `bfe2e48e51cb20789786b619e2ec087a72544dcf` failed shard 2 in
`TestAnalyzerSchemaInputs/IDX-SNAP-V0-017` and `TestIndexCoversSpecsAndHeaders`.
The earlier closeout missed the analyzer audit and the spec-index consistency test.
The coordinator authorized only the schema/audit pins and four index-claim corrections;
the two semantic production repair cycles remain exhausted.

The accepted analyzer contract is `IDX-SNAP-V0-017` in
`docs/specs/index-snapshot-v0.md`. The conservative source audit includes the newly
added `internal/secretscreen/argv.go`, even though context extraction still calls
`MatchString` and the new consumer API does not change extraction. Its imports are
standard-library `encoding/json`, `path`, `regexp` and `strings`; no new extraction
dependency is introduced. The complete audit covers 85 production/module inputs.

## Maintenance decision and rollback

Reserve `corvint-analyzer/100` for this branch because the separately published
issue390 branch already uses `/99`. After changing the production schema ID,
recompute the full path-NUL/content-NUL source audit. The exact digest for this tree
is `d041b96d750ea4fd9879b945dd0baa7db17265f22daafda101101de32e480b10`.
The runtime does not use that source digest as its cache key. A schema bump invalidates
experimental analyzer packs and pack-derived Tasks scope identities; the default
gob engine continues to use the executable digest. Later merged analyzer inputs
require a fresh integration audit and schema decision, not reuse of this digest.

Remove only the typed-argv amendment suffix from four `docs/specs/INDEX.json` claims.
This restores their existing bounded Agent digest and README claims. Accepted typed
argv requirements, schema-v1 compatibility and their original evidence stay intact.
Rollback reverts these maintenance commits; a rollback must retain the resulting
known audit/index failures rather than advertise the original pins as qualified.

## Evidence route and verification scope

Used: installed Corvint pre-change query and tracked-path impact, fresh enrollment,
`affected`, CEM/OCM, bound selected checks and strict seal. Original receipts are
retained under the repair clone's private Git directory; all omissions and
`NOT_PRODUCED` reasons remain in the private reports. The initial no-diff
`dogfood-change` could not prepare a CEM and is retained as failed observation.

The already sealed workflow refuses a subsequent `dogfood begin` with
`prior-completion-stale`. Its original state is preserved. An independent local clone
of the same branch/base supplies a fresh repair window without alternates or hardlinks.
This confirmed friction was deduplicated to native ticket `V1-0493` with reproduction
and acceptance evidence. No tool lifecycle implementation is changed here.

The frozen selected checks are analyzer schema/pack regressions (`-run TestAnalyzer`),
`internal/specindex` and `internal/tasks/scopes`, vet for those packages, and the five
focused documentation checks. The final observations and independent mechanical-diff
review are retained in `/tmp/corvint-parallel-dispatch/issue408-ci-repair-*` artifacts.
Native generation 86 submits the sealed tree and runs its required `focused-docs` gate.
Repository-wide `make gate` is not requested for this scoped maintenance repair;
unchanged feature qualification and semantic review are retained at their original bindings.
Optional mutation, provider, console, learning and external adopter evaluations are not
applicable to these maintenance edits. Structural completion does not establish broad
release qualification or an independent external outcome.
