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
recompute the full path-NUL/content-NUL source audit. The exact digest at the first repair binding
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

## Public integration and fresh binding

The coordinator subsequently landed reviewed public main
`01f557cb47272d44c1316cc58d2b44529661756f`, including issue390's `/99` analyzer
inputs. Normal merge checkpoint `a0fac3a1cd69a609057e89accfb6b340529a3892`
preserves published ancestor `bfe2e48e51cb20789786b619e2ec087a72544dcf`.
Only the two schema-pin paths required conflict resolution; every other path agrees
with Git's automatic merge tree. The unpublished `/100` reservation now covers all
86 combined inputs, with audit SHA-256
`c444ed731cb108143898101871ff265e2b614606a69a3a810d61b879a4bcef45`.

The first repair's seven-hunk CEM, one linked `IDX-SNAP-V0-017` obligation, four
qualified checks and strict seal passed at bind `0ad18fc93244001c4aeb7f13a064f58f0cabfd00`
and seal `73ba1fa8c984de938bc60b3c61cdf18b6a36a852`. Those observations retain
that original binding; they do not qualify the later combined tree.

A new public-base `dogfood-change` correctly refused `sealed-cem-in-change` because
our two historical seals were absent at that base. Under the separately reviewed
integration precedent retained by native `V1-0527`, only those two task-owned
final-tree copies are removed. Their exact Git blobs, SHA-256 digests and raw files
are preserved by `issue408-ci-public-seal-preservation.json` outside source and by
reachable immutable `bfe2`/`73ba` ancestry. Every public-base foreign sealed map stays
byte-identical. No object or historical commit is removed, and the guard is retained.
The full feature, maintenance and integration delta is freshly bound against exact
public base `01f557`; no merge-checkpoint-only closure is claimed.

A fresh independent clone avoids the known post-seal enrollment refusal without
cancelling or deleting either previous workflow. Fresh selected checks repeat the
original feature units, CLI subsets, vet and receipt validation at the combined
binding, plus full `internal/contextindex` units, spec-index/Tasks scopes and the
non-Go CLI integration subset. Source/receipt drift found by these checks remains
blocking. Frozen adopter measurements retain their original engine/corpus bindings;
no new external outcome, broad release qualification or learned-quality claim follows
from this integration. Independent review covers the changed merge, pins, preservation
and evidence; the unchanged argv implementation keeps its prior semantic reviews.
