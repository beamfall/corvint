# Additive Narrowing V0

Owner: Russell Lewis
Date: 2026-10-29
Requirement prefix: `ADN-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3123, V1-3129, V1-3138); ADN-V0-009 amends AFP-V0-094
Delivery status: experimental
Authoritative inputs: native tickets V1-3123 (related V1-0417), V1-3129 (related V1-2450) and
V1-3138. These requirements narrow the selection of the [affected plan](affected-plan-v0.md)
and of `corvint test`; that spec's Requirements section holds 250 requirements, near the
256-obligation OCM limit (`maxObligations`), so they live here.

## Agent digest
- Claim: Additive Go changes select only importers that reference a new declaration; documentation and sealed-CEM paths take their readers from the plan.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3123, V1-3129, V1-3138); ADN-V0-009 amends AFP-V0-094.
- Exists: `GoAdditive` records and `plan.goAdditive` (`internal/liveverify/affected/additive.go`, `internal/liveverify/affected/golang/additive.go`); `additiveNarrowing` and `planReaders` in `internal/gateselect/select.go`; tests named for each requirement.
- Blocked on: nothing; CI's `gateselect.Select` deliberately keeps the whole attribution.
- Read next: Requirements; Failure modes; Acceptance evidence and traceability.

## Intent and scope

Intent (orchestrator lane request, 2026-10-29; V1-3123, related V1-0417): a Go diff that only adds
top-level declarations to a package changes nothing an importer already uses, yet the planner
carried it to every importer. On the V1-3090 replay (0a04f0813..d9eaf9ae1, one added function in
`internal/contextindex`) that selected 138 units and 169 `corvint test` units. The package's own
tests SHALL always run; its importers SHALL be selected only when they reference a new identifier;
every narrowing SHALL carry a witness, and anything uncertain SHALL fall back to package-level
selection with a named reason. Addenda (V1-3129, related V1-2450; V1-3138): a spec-only change with
its regenerated `INDEX.json`, `README.md` and `REQUIREMENTS.tsv`, and a newly sealed CEM under
`.corvint/changes/`, made `corvint test` select every package `internal/gateselect` rule (c) could
not bound, although the planner had already resolved their readers with witnesses.

## Requirements

- **ADN-V0-001:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3123)
  For each Go unit with a changed non-test source of its own, the golang plugin SHALL compare the
  package at the measured-from commit with the observed tree and record one `GoAdditive` entry:
  `NARROWED` with `declarations` (the new top-level identifiers, a method as `Type.Method`),
  `referencing` and `unreferenced` importers, or `FALLBACK` with a `reason`. A package is additive
  only when, for every changed file, the base file's comment-free top-level declarations are an
  in-order subsequence of the head file's, the package clause, the `//go:`, `//line`, `+build`
  and cgo `//export` directives and the imports are unchanged except for an added unnamed import
  the package's other files already hold, and at least one declaration is new.
- **ADN-V0-002:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3123)
  The records SHALL be a selection input set by `Graph.WithGoAdditive`, like the read-scope base
  (AFP-V0-050): they SHALL NOT change the observed graph or its digest.
- **ADN-V0-003:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3123)
  Selection SHALL keep a `NARROWED` package's own unit and tests and follow its import and test
  import edges only to `referencing` importers; advisory and routed edges, rule (c) readers, rule
  (d) and every other seed SHALL be followed as before.
- **ADN-V0-004:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3123)
  A record SHALL fall back, selecting the package the ordinary way, when: the base is unreadable or
  lacks the package; a dirty path in the package directory is not a Go source or test of the unit
  or is deleted; the package embeds files or is `main`; a file does not parse; the package clause,
  a directive or an import changes (an added import that is named, dot, blank or absent from the
  package's other files included); a
  constrained file is new; any Go file of the package directory, at base or head and under any
  build constraint, imports `C`; an existing declaration is edited, removed or reordered; a new
  declaration is `init`, has no body, is a method on a type the change does not declare, shadows a
  predeclared identifier, is declared by another base file of the package under any build
  constraint or twice in the change, or is a `var` whose initializer is not proved safe at init
  (only literals, names, function literals, qualified names, composite literals with constant or
  field keys, `&` of a name or composite literal, unary `+ - ! ^` and binary
  `+ - * & | ^ &^ && || < <= > >=` over safe operands are; a call, conversion, receive, division,
  modulo, shift, `==`, `!=`, index, slice, dereference or type assertion is not); no declaration is
  new; or an importer dot-imports the package. The graph SHALL also demote a
  `NARROWED` record whose unit is not a direct source seed, is reached by a data path, owns dirty
  paths other than the record's, or is reached from another changed unit's walk, naming
  `NOT_A_DIRECT_SOURCE_CHANGE`, `DATA_PATH_CHANGE`, `DIRTY_SET_DIFFERS_FROM_RECORD` or
  `REACHED_THROUGH_ANOTHER_CHANGED_UNIT`.
- **ADN-V0-005:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3123)
  A unit the unnarrowed plan selects but the narrowed one does not SHALL be listed in the optional
  member `goAdditiveExcluded` (full and compact receipts; omitted when empty), each entry naming
  `unitId`, `through` (the narrowed package whose `goAdditive` record is its witness), `universe`
  and invalidation `REFERENCE_TO_NEW_DECLARATION_OR_NON_ADDITIVE_CHANGE`, and SHALL NOT be listed in
  `excluded`, whose closed reason and invalidation values stay unchanged (CCF-V1-006: a new closed
  value needs a new profile version and an N-1 reader, an added optional member does not). A unit
  SHALL appear in at most one of the two lists.
- **ADN-V0-006:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3123)
  `gateselect.SelectStructural` (`corvint test`) SHALL limit a `NARROWED` non-test source's
  dependents to the importers the record does not list as `unreferenced`, keeping the package,
  its namers and rule (c). `gateselect.Select`, the CI PR tier's frozen selector, SHALL keep the
  whole attribution, as it keeps rule (d) (STR-V0-061).
- **ADN-V0-007:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3123)
  An importer SHALL count as referencing when any of its source or test files that imports the
  package selects a new exported identifier on the import's local name. Every Go file of the
  importer's directory, under any build constraint and test files included, SHALL be parsed and
  each import path unquoted, so a raw or escaped spelling matches. An importer that is not a Go
  unit, has an unreadable or unparsable file or import path, has no file importing the package,
  blank-imports it, holds a `//go:linkname` directive, or whose directory holds assembly (`.s`,
  `.S`, `.sx`) SHALL count as referencing. Unexported and `Type.Method` names need no importer check: an importer can reach a
  new method only through a new exported identifier.
- **ADN-V0-008:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3123)
  The plan SHALL report the records as `plan.goAdditive` (full and compact receipts; omitted when
  there are none), a `FALLBACK` record without `referencing` or `unreferenced`.
- **ADN-V0-009:** (experimental, amending AFP-V0-094, under the owner's standing lane auto-accept,
  2026-10-11; V1-3123) `corvint affected` SHALL measure from the `--base` commit, else `HEAD`; an
  unresolvable base SHALL record every candidate `FALLBACK` `BASE_UNREADABLE`. A review's
  materialized snapshot SHALL measure its tree from the review base in the repository, so
  `review.affected` stays the `corvint affected --base` projection. `--path` mode, which has no
  measured-from tree, SHALL record nothing and select as before; AFP-V0-094's equality with the
  `--base` plan of the same committed change therefore holds except for `plan.goAdditive` and the
  narrowing it applies.
- **ADN-V0-010:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3129)
  For a dirty path the plan classifies `DECLARED_DOCUMENTATION` that is not package source,
  `SelectStructural` SHALL replace its rule (c) readers with the plan's reader selections, printing
  `documentation <path>`; the plan's reader rules (literal, glob and declared read scope) already
  select each such reader with a witness, and `corvint test` unions the plan's selections.
- **ADN-V0-011:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3138)
  For a sealed CEM directly under `.corvint/changes/` (not `.corvint/change.cem.json`),
  `SelectStructural` SHALL likewise take its readers from the plan, which resolves each unit's path
  token from the unit's directory (AFP-V0-152), printing `evidence <path>`, instead of
  `internal/gateselect`'s rule (c), whose bare `/` root anchor matched about 39 packages. Rule (d)
  units stay selected from the plan with their `UNBOUNDED_READER` witnesses. ADN-V0-010 and
  ADN-V0-011 SHALL keep the whole rule (c) for every path when the plan states a
  `:path-token-bound:` language frontier (AFP-V0-021), and SHALL NOT change `Select`. Neither SHALL
  trust the plan's classification: `SelectStructural` SHALL take a path only when it is dirty in
  the plan, a documentation path is declared by the root's own `.corvint/documentation-paths.json`,
  a CEM name is a 40- or 64-digit lowercase hexadecimal commit, and every package holding a literal
  that resolves, from the holder's own directory, to the path or an ancestor directory other than
  the root is among the plan's selected packages, unless the root's own read-scope declaration,
  not dirty in the plan, confines that package outside the path (AFP-V0-161, as the planner); a
  stale or forged plan failing any check SHALL keep the whole rule (c) for that path.

## Non-goals

type-checked reference analysis, narrowing an edited, removed or reordered declaration,
narrowing a changed `main`, embedding or cgo package, any change to `gateselect.Select` or the CI
PR tier, and bounding rule (d) readers of a sealed CEM (a package with no bounding literal can read
any path; AFP-V0-023 read scopes remain the project-owned way to bound it).

## Failure modes

(1) a reference the syntactic check misses (reflection over a package value cannot
enumerate package-level names; a new method needs a new exported type or function, which is
checked) would drop an importer; the package's own tests still run and CI's `Select` keeps it.
(2) A dependent that compiles only because a new identifier now exists is a referencing importer.
(3) A plan from an older planner carries no `goAdditive` and no narrowing applies. (4) A unit the
plan's reader rules miss for a documentation or CEM path is missed by `corvint test` only when
no literal of it resolves to the path: such a literal reader, a declaration change and a CEM name
that is no commit are checked by the consumer, and declared read scopes are read by the consumer
itself, so the residual is a glob-only reader the plan misses; the path-token-bound frontier keeps
the whole rule (c) where the plan says its reads are incomplete.

## Acceptance evidence and traceability

the tests in the traceability rows, failing on base `0a04f0813`; replays in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-29-v1-3123-additive-narrowing.md` (V1-3090: `corvint test` 169 to 87
units; V1-3129 docs-only: 128 to 49; V1-3138 CEM-only: 77 to 38, gh2004-2005 diff 175 to 113,
each after the consumer checks); the
negative replay editing an existing exported function selects every importer again.

| Requirement | Implementation | Evidence |
|---|---|---|
| ADN-V0-001 | `AdditiveChanges`, `AdditiveRecords`, `additiveRecord`, `additiveCompareFile`, `additiveNewDeclarations` and `additiveDeclarationNames` in `internal/liveverify/affected/golang/additive.go`; `GoAdditive` in `internal/liveverify/affected/additive.go` | `TestADNV0006AddedFunctionSelectsOnlyReferencingImporters`; the NARROWED cases of `TestADNV0004AdditiveFallbacks` (a method on a new type, a var literal with a const, an import the package already holds) |
| ADN-V0-002 | `Graph.WithGoAdditive` | `TestADNV0003AdditiveRecordPrunesUnreferencedImporters` (digest unchanged); `TestADNV0006AddedFunctionSelectsOnlyReferencingImporters` |
| ADN-V0-003 | `additiveTraverse`, `follows`, the `allowed` parameter of `traverse` and `testUsersOf` | `TestADNV0003AdditiveRecordPrunesUnreferencedImporters`; `TestADNV0003OnlyTestChangeIsNoCandidate` |
| ADN-V0-004 | `additiveRefusal` and `reachedThrough` in `internal/liveverify/affected/additive.go`; the fallback constants and checks of `golang/additive.go` | `TestADNV0004GraphRefusesARecordItCannotCheck`; `TestADNV0004ModifiedExportedFunctionSelectsEveryImporter` (the negative case); the 36 fallback cases of `TestADNV0004AdditiveFallbacks` (among them 12 unproved `var` initializers, a type, method and function a platform or tagged variant declares, a name declared twice, and cgo in an unchanged file) |
| ADN-V0-005 | `additiveExclusions` and the `GoAdditiveExcluded` branch of `selectPlan`; `GoAdditiveExclusion`; `affectedCompactPlan.GoAdditiveExcluded` | `TestADNV0003AdditiveRecordPrunesUnreferencedImporters` (closed `excluded` values unchanged; no unit in both lists); `TestADNV0004ModifiedExportedFunctionSelectsEveryImporter` (no entry) |
| ADN-V0-006 | `additiveNarrowing` and `attributeNarrowed` in `internal/gateselect/select.go`; the `withUnresolved` guard in `selectPackages` | `TestSelectStructuralHonorsAdditiveNarrowing_ADNV0006` (Select keeps the whole attribution) |
| ADN-V0-007 | `additiveReferences` in `golang/additive.go` | `TestADNV0006AddedFunctionSelectsOnlyReferencingImporters` (an aliased import references; an existing-name user does not); the dot-importer case of `TestADNV0004AdditiveFallbacks`; `TestADNV0006ImporterSpellingsAndRetainedImporters` (raw and escaped spellings, a test-only reference, a blank import, `//go:linkname` and assembly retain the importer) |
| ADN-V0-008 | `Plan.GoAdditive`, `additiveSummary`; `affectedCompactPlan.GoAdditive` in `cmd/corvint/affected_compact.go` | `TestADNV0003AdditiveRecordPrunesUnreferencedImporters`; `TestADNV0004GraphRefusesARecordItCannotCheck` (a FALLBACK record carries no importer lists); the compact member is a pass-through, observed in the replays of build log 2026-10-29-v1-3123-additive-narrowing |
| ADN-V0-009 | `affectedGoAdditive` and the `PathList` guard of `compileAffectedGraph` in `cmd/corvint/affected.go`; `AdditiveChangesAt` in `internal/liveverify/affected/golang/additive.go` and `snapshotReviewAffected` in `cmd/corvint/repository_guidance.go` | `TestADNV0009UnreadableBaseFallsBack`, `TestAFPV0094PathListSelectsTwoHopImportersAtHead`, `TestRepositoryGuidanceReviewBranches`; the `--base` replays of build log 2026-10-29-v1-3123-additive-narrowing |
| ADN-V0-010 | the `documentation` branch of `planReaders`, `documentationDeclaration` in `internal/gateselect/select.go`; `holderReaders` in `internal/gateselect/readers.go` | `TestSelectStructuralTakesDocumentationReadersFromThePlan_ADNV0010` (a plan without the resolving reader and an undeclared path keep rule (c)); `TestADNV0010And0011ProducerToConsumer` in `internal/liveverify/affected/golang` (a real planner run through the JSON plan; a dropped reader and a removed declaration keep rule (c)) |
| ADN-V0-011 | the `evidence` branch of `planReaders` and `sealedEvidence` in `internal/gateselect/select.go` | `TestSelectStructuralTakesSealedEvidenceReadersFromThePlan_ADNV0011` (an enumerating fixture test stays selected; a `/`-anchored package is dropped; a path-token-bound frontier, a plan without the enumerating reader, three forged CEM names and `Select` keep the whole set; a holder whose declared read scope excludes the CEM does not block it, unless the declaration is dirty; the sidecar is unchanged); `TestADNV0010And0011ProducerToConsumer` (a real planner run; a dropped reader and a forged CEM name keep rule (c)) |

## Rollback

revert `additive.go` in both packages, the `reach`/`traverse` allowed-set parameter, the
`goAdditive` and `through` wire members, and `planReaders`/`additiveNarrowing` in
`internal/gateselect`; plans then select as before. No stored state depends on the records.
