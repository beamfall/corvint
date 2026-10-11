# appmap: tests.root "." for an aliased test repository (V1-1145)

Date: 2026-10-10. Ticket: V1-1145 (GitHub #787). Requirements: AMAP-V0-075..077, proposed, pending
owner acceptance (V1-1145).

## Decision

A test repository whose specs and page objects sit at its top level had no valid `tests.root`,
because `safeRelative` refuses `.`. The manifest now accepts exactly `"."`. Dirty-root detection
for an aliased repository otherwise covers the whole root, so `"."` would make any stray file
refuse; for `"."` the input set is narrowed to the specs, page-object, workflow and scenario
directories, plus the manifest file (already added when the manifest comes from an alias).

## Evidence

`TestAMAPV0075WholeRepositoryRoot` (internal/appmap/roots_test.go): the join under `"."` equals the
join under the narrower root, an unrelated untracked file does not refuse, a new file in a declared
directory refuses with `appmap-root-dirty`, and `""`, `..` and `./` still refuse. Existing
roots and manifest tests are unchanged and pass, which covers AMAP-V0-077 for the paths they use.

## Limits

No adopter-scale qualification. A `"."` root with a non-aliased (same) repository is unchanged.
