# Decision 0312 — Path-to-path relations are an opt-in V2 record profile, verified per side

Date: 2026-09-18. Status: accepted (delegated call on the owner's own feature request
Beamfall/corvint#7; owner review of the intent is welcome and changes only the digest line).

## Decision

A record whose schema is `external-evidence-provider/2` may relate two repository-qualified paths
directly, as specified in `docs/specs/external-evidence-provider-v2.md` (`EEP-V2`). V2 is the V1
record shape: the schema value is the only opt-in, and a V1 record's path relation stays
`unsupported`.

- **Impact.** `context.external` gains `path_relations` only when a V2 record loads. Each item keeps
  the relation as declared, checks both endpoints against their own repositories, and takes the
  worse side's `relation_state`.
- **Selection.** For a verification type, `from` is the test and `to` is the subject. The relation
  covers any path obligation either side names, and it qualifies only when the ETS-V0 type and
  evidence checks pass and both sides are bound, fresh, verified, and clean, test side first. An
  unbound subject reports `unbound-source-repository`.
- **Widening.** A non-verification, non-context path relation on a changed path makes its other
  endpoint an obligation, one hop, as ETS-V0 does for entities. Widening only ever adds
  obligations, so it cannot cause unsafe narrowing.
- **Directory scope.** A V2 path ending in `/` is a declared scope and cannot pin a blob. As a
  subject it is evaluated once per path obligation it holds, checking the subject side as that
  held path, so every held path must be tracked, fresh, and clean on its own. A scope that holds a
  changed path also widens (`EEP-V2-012`, `EEP-V2-013`). Nothing without a trailing `/` is a scope.

## Alternatives weighed

- *Reinterpret V1 path relations*: rejected because it silently changes the meaning of existing
  records, which the request forbids.
- *A per-relation capability flag inside a V1 record*: rejected because it hides the opt-in in the
  data rather than the schema, and a V1 reader would ignore it. A schema bump keeps dispatch on one
  member.
- *A new top-level result list for selection*: rejected because the path rows reuse
  `selected`/`candidates`, marked by `subject` instead of `entity`, so existing consumers read
  them without a new member.
- *Treat any path as a prefix of its descendants*: rejected because `pkg` would then cover `pkgs/`
  and a file path would silently turn into a directory. The trailing `/` is the declaration.
- *Verify the directory once and cover every file beneath it*: rejected because a directory is not
  a Git blob, checkout trees answer only for named paths, and one stale or dirty file must still
  block. Each held path is verified on its own instead.

## Rollback

Remove `Schema2` and the path functions in `internal/extevidence` (`pathRelationsOf`,
`addPathRelations`, `pathTest`, `scopeTest`, `checkScope2`, `widenPaths`, `pathRow`), the `conformance-path` fixture and path
tests, the help lines, the V2 spec with its index rows, and this record, and revert the
`EEP-V1-001`, `EEP-V1-010`, and V1 non-goal wording. V0 and V1 records and runs without `--provider` are
unaffected in both directions.
