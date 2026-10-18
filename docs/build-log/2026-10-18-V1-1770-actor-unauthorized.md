# V1-1770: ACTOR-UNAUTHORIZED fixture actors (issue 1115)

Ticket V1-1770, GitHub issue 1115. Requirements TSA-V0-030 to TSA-V0-036 in
[test-source-analysis-v0](../specs/test-source-analysis-v0.md), accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). This is not a new owner decision; it applies that
standing acceptance to this lane's requirements. No CAL-V0 change.

## Decisions

- **Home: TSA, not LPCV or the application map.** The issue asked for a test-validity or
  application-map requirement. The check is a static read of committed test source before a run,
  like `tests lint` and `tests duplicates`, and it projects no runtime receipt, so it lives in
  `internal/testsource` and the TSA spec. This also keeps it clear of the open test-validity edits
  in PRs 1101 and 1112.
- **Parse literal Ruby, no declared table.** The literal `routes.rb`, controller `authorize!` and
  `Ability` role-method forms were small enough to read directly, so no project-owned role/ability
  table was added. Every other form is recorded as unparsed. A conclusion that depends on it is
  UNKNOWN with a closed reason (TSA-V0-035), never authorized.
- **Lowest sufficient role.** A role that holds the ability and inherits no other role that holds it.
  In the repro, admin inherits manager, so only manager is named. Its granting rule is line 14 of
  the fixture Ability class.
- **Library first.** The slice is `testsource.ActorAuthorization` with focused tests. A
  `corvint tests actor-auth` command is a residual, kept off `cmd/corvint/tests.go` while PR 1096
  edits it.

## Supported frameworks

- Client side: Playwright-style JS/TS tests and fixture builders with a literal `role:` or a literal
  factory default, a `*login*`/`*signin*` call naming `fixture.key`, and
  `X.get|post|put|patch|delete('/path')` requests.
- Server side:
  - Rails `config/routes.rb` literal forms (namespace, resources, verb routes, root);
  - a literal `authorize! :action, Constant` in the controller action;
  - a CanCanCan-style `class Ability` with `send(user.role)` dispatch and inherited role methods.

## Non-goals

- Other authorization libraries (Pundit, Action Policy, hand-written guards).
- Runtime policy: roles held in a database, conditions evaluated against records.
- `fetch()` and navigation requests, `load_and_authorize_resource`, and `if`/`case` role dispatch.
  Each of these is UNKNOWN or not read, never authorized.

## Evidence

`internal/testsource/actor_auth_test.go` runs on a synthetic Git repository built from the issue's
repro.

- The assistant's `PUT /api/widgets/:id` is ACTOR-UNAUTHORIZED. The finding names the granting rule,
  the lowest role `manager` and the alternative `beta.manager`.
- The manager pair is AUTHORIZED.
- The 403-asserting test is EXEMPT.
- Each of the 26 closed UNKNOWN reasons has a case.

No live qualification was run on a real suite or application (NOT_RUN).
