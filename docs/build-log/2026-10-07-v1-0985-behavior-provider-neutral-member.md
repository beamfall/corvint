# V1-0985: neutral member for the behavior provider's end-to-end repository

Date: 2026-10-07. Ticket V1-0985 (residual of V1-0977).

## Decision

The `/1` behavior provider's `revisions` object names the end-to-end test repository with a member
named after a private adopter's product. The specs and this log call it the legacy /1 member.
Renaming it would change the frozen `/1` bytes (AFU-V1-006, `TestAFUV1BehaviorProviderV1BytesUnchanged`),
so `/1` keeps it and gains no alias.

The provider's next version already exists. `/2` (`corvint-corpus-behavior-provider/2`, issue 330)
replaces the three fixed members with the role-free `repositories` list. It also requires the
provider `source` to be one of those repositories, and in `/1` the end-to-end repository is the
source (`behavior_adapter.go` checks `Source == Revisions.E2E`). So `/2` already carries that
repository through neutral members, and no `/3` or new member was added. Adding an `e2e` member to
`/2` would reintroduce a fixed role member that AFU-V1-006 removed.

This change records that contract as AFU-V1-054..056, proposed (V1-0985), and pins it with tests.
`/1` encodes and decodes the legacy member and refuses a neutral alias. `/2` provider bytes and the
corpus report emit neither the legacy member nor `revisions`. `/2` producer requests carrying the
legacy member or an `e2e` alias are refused. The tests read the member's spelling from the
`BehaviorRevisions` struct tag by reflection rather than repeating it. The spec mentions in
`application-flow-understanding-v1.md` and `documentation-corpus-v1.md` now say "the legacy /1
member". No wire bytes changed.

## Remaining occurrences

- `internal/doccorpus/behavior.go` `BehaviorRevisions.E2E` struct tag: this is the single `/1` encode
  and decode declaration. The `/1` provider, its migration manifest, the live discovery, the legacy
  runtime, the adapter request and result, and the corpus manifest's `behavior_revisions` all share
  it.
- `docs/BUILD-LOG.md` (closed, decision 0423) and Git history keep earlier spellings. This change
  does not edit them.

## Limits

No new profile, no external producer and no consumer-side migration was observed. Producers still
on `/1` keep emitting the legacy member until they adopt `/2`. `/2` is declaration-only and cannot
carry runtime reconciliation, so a producer that needs `/1` runtime qualification has no neutral
path yet.
