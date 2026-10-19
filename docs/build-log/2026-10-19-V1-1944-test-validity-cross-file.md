# 2026-10-19 V1-1944: cross-file abstentions, fixture seeding and JSON annotation

Ticket: V1-1944, which follows V1-1919, V1-1930, V1-1931 and V1-1932 and is stacked on #1201.

## Decision

`TSA-V0-300` to `TSA-V0-312` are added to `docs/specs/test-source-analysis-v0.md` and accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). No accepted requirement is
amended.

- Cross-file abstention rather than resolution. Resolving a sibling module would mean reading
  another file inside `ValidityFindings(name, text)`. That function has no repository access, and
  the JS/TS example projection calls it too. Resolution would also turn an imported registry or a
  stub-less file into a reported finding, which accepted TSA-V0-167 and TSA-V0-171 forbid.
  Instead, a withheld reading is recorded as an abstention in a new `abstentions` list that is
  omitted when empty and changes no axis. Resolution is NOT_PRODUCED (TSA-V0-312).
- `SeverityAbstained` is not reused. It makes hygiene `ABSTAINED`, which would turn an advisory
  rule into an axis change.
- A sibling import is a stub module only when its specifier, or a name it binds, contains a stub
  word. The first draft treated every sibling import as a stub module. That draft withheld every
  finding in files that import `./fixtures`, and gave every such file without stubs an
  abstention, so it was narrowed.
- Seeding is a named call: `seed*`, `create*`, `insert*`, `populate*`, `add<Upper>`, or
  `x.post` / `x.put`. It counts in a declared fixture (followed through the fixture's own declared
  fixtures) or in a `beforeEach` / `beforeAll` hook in scope. The reading only under-reports. A
  fixture the file does not declare turns the finding into a `fixture-cross-file` abstention.
- The JSON annotation is a keyed sibling member,
  `"corvint-validity-allow": {"<key>": "EITHER-OR-OUTCOME: <reason>"}`. It is not data on the line
  above, so TSA-V0-163 stays intact.

## Evidence

Focused tests in `internal/testsource` and `internal/testvalidity` pass, as do the
`cmd/corvint` validity CLI tests. 29 mutations were run, one per new decision point, including the
five review fixes. 28 were killed. The survivor removed an untested Playwright option name
(`bypassCSP`); the option list itself is covered by the `baseURL` mutation. No live qualification
on a real suite was run (NOT_RUN).

## Independent review

One Codex read-only round covered this change's commits only.
- Fixed, each with a regression test that kills the reverted fix:
  - Several top-level JSON values kept their exemptions; the file must now be exactly one valid
    JSON value.
  - A registration nested in a call on an undeclared fixture was skipped.
  - A method declaration counted as a seeding call.
  - Import clauses longer than 64 tokens lost their names.
  - `require('./x',)` was missed.
- Clarified in the spec: names from re-exports and pre-alias names count toward the stub-word
  test. The intent is that the module exports a stub, so the wording of TSA-V0-301 now says so.
- Escalated, then amended by owner decision: TSA-V0-306 narrows the unconditional TSA-V0-172
  finding, and the last sentence of TSA-V0-172 said the reading does not model seeding. This is
  the same narrowing pattern TSA-V0-173 applies, and it is what the ticket's acceptance criterion
  (b) requires. The first submission left the accepted text unedited and escalated.

## TSA-V0-172 amendment (owner decision, 2026-10-19)

The owner decided to amend TSA-V0-172. Its MUST now excludes a test whose in-scope fixture or
hook seeds the collection as TSA-V0-306 defines, the sentence "The reading does not model fixture
seeding" is removed, and its acceptance line records "amended 2026-10-19, owner decision,
V1-1944". The V1-1919 failure mode that said a seeded fixture still gets the advisory now names
the TSA-V0-306 suppression, the TSA-V0-307 abstention and the unread in-test seeding. The
traceability row of TSA-V0-172 adds `TestVacuousEmptyEqualitySeeded`. No code changed.

## Residuals

- Following imports (TSA-V0-312), `auto` fixtures, global setup and in-test seeding are not
  modelled.
