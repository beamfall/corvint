# External Evidence Provider V2 — opt-in path-to-path relations

Owner: Russell Lewis
Date: 2026-09-18
Intent status: accepted (decision 0312)
Delivery status: experimental (file transport only)
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/external-evidence-provider-v1.md`, `docs/specs/external-test-selection-v0.md`,
decisions 0310 and 0311, and the feature request Beamfall/corvint#7.

## Agent digest
- Claim: An `external-evidence-provider/2` record may relate two repository-qualified paths; each side is verified on its own and the relation takes the worse state.
- Status: accepted (decision 0312, a delegated call on Beamfall/corvint#7)/experimental (file transport only); checked by `TestSelectionConformance`, `TestSelectionEvaluation`, `TestPathRelationsImpact`, and `TestAffectedSelectionPathRelation`.
- Exists: `Schema2` in `internal/extevidence/record1.go`, `pathRelationsOf` in `compose.go`, `pathTest`, `scopeTest`, `widenPaths`, and `pathRow` in `selection.go`, `checkScope2` in `record1.go`, and the independent two-repository fixture under `internal/extevidence/testdata/conformance-path/`.
- Blocked on: an ACC-V0 provider profile before any executed transport; an independent adopter record before promotion.
- Read next: Requirements; Trust boundary, limits, and failure modes.

## User and measurable job

A provider that knows "this end-to-end spec exercises this handler" should be able to say so
without inventing an entity to carry the relation, and without losing the relation's type or
evidence kind. The job: a direct path relation reaches `corvint impact` and `corvint affected`
with both endpoints checked against their own repositories. In the labelled path corpus, no case
that must require the full suite narrows (unsafe-narrowing rate 0), every narrow selection names
only labelled tests (precision 1), and V0 and V1 records keep their bytes.

## Verified current state

Before this slice, `EEP-V1-010` reported any relation whose two endpoints were paths as an
`unsupported` unknown. `ETS-V0` therefore qualified tests only through an entity. The ideas
backlog carried this as a V1 follow-up.

## Definitions

- **Path relation**: a relation whose `from` and `to` are both `{repository, path[, blob]}`
  endpoints in a V2 record.
- **Test side** and **subject side**: for a verification type (`verifies`, `asserts`, `covers`),
  `from` is the test and `to` is the path it verifies.
- **Path obligation**: a changed root path (ETS-V0), or the other endpoint of a non-verification,
  non-context path relation on a changed root path (`EEP-V2-008`).
- **Directory scope**: a V2 path endpoint whose path ends in `/`. It **holds** every path
  obligation in the same repository whose path starts with it and is longer than it.
- **Worst side**: the relation state ordered `unresolved` > `stale` > `not-verified` > `fresh`.

## Requirements

- `EEP-V2-001`: A record whose `schema` is `external-evidence-provider/2` MUST decode under every
  `EEP-V1-001` to `EEP-V1-003` rule, with the same members and bounds. It is the only way to opt
  in: a V1 record's path relation MUST stay an `unsupported` unknown (`EEP-V1-010`), and any
  other schema value follows `EEP-V0-001`.
- `EEP-V2-002`: Each endpoint of a path relation MUST resolve independently under the `EEP-V1`
  endpoint rules (declared repository, path bounds, optional pinned blob). A failure on either side
  MUST make the relation an `unresolved` unknown with a reason. Core never repairs, renames,
  or infers a relation, its type, or an endpoint.
- `EEP-V2-003`: When at least one V2 record loads, `context.external` MUST gain `path_relations`
  and `omitted.path_relations`. Every path relation with a changed root path on either side
  appears once, anchored on the changed side (`from` preferred), with `relation` (type, evidence,
  rule, reference exactly as declared), `endpoints` (both sides' identity, binding, revision,
  captured revision, freshness, and verification), `relation_state`, `crosses_repositories`,
  `provider`, `provider_revision`, `authority`, a deterministic `reason`, and `limitations`
  (`not-coverage-proof`, `not-executed`, plus `checkout-worktree-not-inspected` when either side is
  checkout-bound). Items never enter core results or ranking, and `rule` and `reference` are listed
  in `untrusted_text_fields`.
- `EEP-V2-004`: `relation_state` MUST be the worst side of the two endpoints. Examples: a missing
  path gives `stale`, an unbound repository gives `not-verified`, an ambiguous identity or a
  mismatched checkout gives `unresolved`, and two verified sides at fresh revisions give `fresh`.
- `EEP-V2-005`: A run without `--provider`, and a run whose records are all V0 or V1, MUST produce
  the same bytes as before this slice. With a V2 record, every core receipt member MUST stay
  byte-identical. The CEM wire format does not change.
- `EEP-V2-006`: In `advice.test_selection`, a verification-type path relation MUST fold into every
  path obligation that either endpoint names, including a changed test path. It qualifies only when
  the ETS-V0 type and evidence checks pass (`ETS-V0-004`, `ETS-V0-005`) and both sides pass
  identity, binding, freshness, verification, and the root-worktree check, test side first.
- `EEP-V2-007`: A failing side MUST report its `ETS-V0` code, except that an unbound subject side
  reports `unbound-source-repository`. Weak codes stay non-blocking candidates, and every other code
  blocks the obligation.
- `EEP-V2-008`: A path relation whose type is neither a verification nor a context type (for example
  `implements`, `generates`, `consumes`, `depends-on`, or a namespaced type) and that touches a
  changed root path MUST make its other endpoint a path obligation, one hop and never further.
  An uncovered widened obligation is listed in `uncovered_paths` with `provider`, `repository`, and
  `path`. Such relations never qualify a test themselves.
- `EEP-V2-009`: Every selected, candidate, or blocking path row MUST carry `schema`, `provider`,
  `provider_revision`, `authority`, `confidence` (`unscored`), `test` and `subject` endpoint
  objects, `relation`, `relation_type`, `evidence`, `verification`, `identity`, `binding`,
  `freshness`, `test_revision`, `source_revision`, the worst-side `relation_state`,
  `crosses_repositories`, `limitations`, and a deterministic `reason`. A row that does not
  qualify also carries `code` and `blocking`. It never carries `entity`.
- `EEP-V2-010`: Path rows and items MUST obey `ETS-V0-010` and `ETS-V0-013`: record order never
  changes the output, every list is bounded with its omissions counted, a checkout is echoed as
  given and never resolved, and no file body appears.
- `EEP-V2-011`: The conformance corpus MUST include an independent two-repository path fixture with
  positive, negative, stale, missing, ambiguous, unsupported, and abstention cases. The
  `TestSelectionEvaluation` measures (precision, unsafe narrowing, abstention accuracy, latency,
  receipt size) MUST run over it with unsafe narrowing 0.
- `EEP-V2-012`: A V2 directory-scope endpoint MUST NOT pin a blob; such a record is invalid. As the
  subject of a verification or context path relation, a scope MUST be evaluated once per path
  obligation it holds, with the subject side checked as that obligation's own path under
  `EEP-V2-006` and `EEP-V2-007` (identity, binding, freshness, tracked at the side's revision, and
  the root worktree). Each evaluation folds into the held obligation only and yields its own row,
  whose `subject` is the held path and whose `subject_scope` names the declared scope. A test side
  that the scope holds is evaluated only as a held obligation; a test side outside the scope is
  evaluated against the scope itself, which is never a tracked path, and blocks. A path without a
  trailing `/` never holds anything.
- `EEP-V2-013`: A non-verification, non-context path relation whose scope endpoint holds a changed
  root path MUST widen to its other endpoint under `EEP-V2-008`. A widened scope is never tracked, so
  it stays uncovered. An unresolved V2 relation whose raw endpoint is a scope MUST block every
  obligation that scope holds.

## Non-goals and simpler baseline

No inference of path relations from names, imports, routes, co-change, or similarity. No anchor
repair, no product-specific relation types in Core, no executed tests, no change to mandatory
checks, the CEM, or the Change Frontier, and no remote transport. The simpler baseline, an
intermediate entity, still works unchanged and remains the only option under V1. A directory is
never implied: only a declared trailing `/` makes a scope, and V1 gives it no meaning. Impact
`path_relations` stays exact-match: a scope relation appears there only when a side is itself a
changed path.

## Trust boundary, limits, and failure modes

A V2 record is provider-authored data with the same authority as V1: `external-provider`, never
project authority. A path relation can justify narrowing only when both sides are Git-checked.
Limits are the V1 record bounds plus the section and selection list limits.

| Condition | Result |
|---|---|
| V1 record with a path relation | `unsupported` unknown; blocking `unsupported-relation` on a touched obligation |
| Undeclared repository or malformed endpoint | `unresolved` unknown, never composed |
| Stale revision on either side | `stale-provider-revision`, blocking |
| Tree mismatch or unrelated history | `stale-provider-revision`, blocking |
| Missing path or pinned-blob mismatch on either side | `missing-path-reference` or `stale-path-reference`, blocking |
| Unbound test side / unbound subject side | `unbound-test-repository` / `unbound-source-repository`, blocking |
| Ambiguous identity / checkout of other history | `ambiguous-repository-identity` / `repository-binding-mismatch`, blocking |
| `candidate`, `navigates`, `inferred`, or `covers` under strict | coded candidate; obligation stays uncovered |
| `implements`, `generates`, `consumes`, `depends-on`, namespaced | widens one hop; never qualifies |
| Directory scope pinning a blob (V2) | invalid record |
| Held path missing, stale, or dirty | that path's code, blocking; other held paths are evaluated on their own |
| Test side outside the scope it verifies | `missing-path-reference`, blocking |

## Deterministic acceptance and testing matrix

| Case | Expected | Test |
|---|---|---|
| 40 labelled cases in `conformance-path/cases.json` | labelled state, codes, and selected tests | `TestSelectionConformance` |
| Evaluation over both corpora | unsafe 0, precision 1, abstention exact | `TestSelectionEvaluation` |
| Path row fields and worst-side state | full provenance, both sides | `TestPathRowProvenance` |
| Reversed relations; limit 1 | identical bytes; counted omission | `TestPathDeterministicAndBounded` |
| Relative checkout | no resolved directory or body | `TestPathPrivate` |
| Scope row, scope blob, V1 scope, unresolved scope | held path and `subject_scope`; invalid; V1 decodes; held path blocked | `TestPathScope` |
| Invalid V2 records; undeclared repository | invalid; `unresolved` unknown | `TestPathRecordStrict` |
| Impact `path_relations`; V1 unchanged | two items, worst side; V1 unsupported, no member | `TestPathRelationsImpact`, `TestPathRelationsBounded` |
| Mandatory checks | echoed unchanged | `TestSelectionMandatoryEchoedUnchanged` |
| CLI impact with a V2 record | core receipt byte-identical | `TestImpactProviderV2PathRelations` |
| CLI affected, V2 versus V1 | narrow versus full | `TestAffectedSelectionPathRelation` |
| No-provider and V1 runs | unchanged bytes | `TestImpactProviderSectionSeparation`, `TestImpactProviderV1CrossRepository`, `TestAffectedSelectionAddsOneMemberAndKeepsEverythingElse` |

## Rollout, rollback, and compatibility

Additive and opt-in by schema. Rollback removes `Schema2`, `pathRelationsOf`, `addPathRelations`,
`pathTest`, `scopeTest`, `checkScope2`, `widenPaths`, `pathRow`, the path fixture and tests, the
help lines, this document, and decision 0312. V0 and V1 records and runs without `--provider` are unchanged in both directions.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `EEP-V2-001` | `Schema2`, `validate1`, `load`, `view1`, `resolve1` | `TestPathRecordStrict`, `TestPathRelationsImpact` |
| `EEP-V2-002` | `resolve1`, `endpoint1` in `internal/extevidence/repository.go` | `TestPathRecordStrict` |
| `EEP-V2-003`, `EEP-V2-004` | `pathRelationsOf`, `pathLimitations` in `compose.go`; `addPathRelations` in `section.go`; `annotate` | `TestPathRelationsImpact`, `TestPathRelationsBounded`, `TestPathRowProvenance` |
| `EEP-V2-005` | `hasPathProfile` in `section.go`; `item.toMap` | `TestImpactProviderV2PathRelations`, `TestImpactProviderV1CrossRepository`, `TestV0SectionCarriesNoV1Members` |
| `EEP-V2-006`, `EEP-V2-007` | `pathTest`, `sideCode`, `sourceCodes` in `selection.go` | `TestSelectionConformance`, `TestAffectedSelectionPathRelation` |
| `EEP-V2-008` | `widenPaths`, `pathObligation`, `uncovered` in `selection.go` | `TestSelectionConformance` |
| `EEP-V2-009` | `pathRow` in `selection.go` | `TestPathRowProvenance` |
| `EEP-V2-010` | `result`, `pathRow`, `addPathRelations` | `TestPathDeterministicAndBounded`, `TestPathPrivate` |
| `EEP-V2-011` | `internal/extevidence/testdata/conformance-path/` | `TestSelectionConformance`, `TestSelectionEvaluation` |
| `EEP-V2-012` | `checkScope2`; `pathTest`, `scopeTest`, `descendants` in `selection.go`; `held` in `repository.go` | `TestSelectionConformance`, `TestPathScope` |
| `EEP-V2-013` | `touchesChanged`, `touched` in `selection.go` | `TestSelectionConformance`, `TestPathScope` |

## Unresolved decisions and promotion or kill criteria

- Promotion needs one independent adopter's V2 record and a corpus drawn from a real change
  history, evaluated with the same measures.
- Kill path-relation narrowing if any adopter-labelled case narrows unsafely; `path_relations`
  would stay as impact context only.
