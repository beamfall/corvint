# Application map: injected string enums, one-level re-exports and the di-constant diagnostic (V1-1061)

Date: 2026-10-09. Ticket V1-1061; GitHub [#705](https://github.com/beamfall/corvint/issues/705).
Requirements: `AMAP-V0-024`..`AMAP-V0-026` (proposed, not accepted) in
`docs/specs/application-map-v0.md`. Follows V1-1027 (AMAP-V0-021..023).

## Problem

An injected registration `.constant('SectionNames', SectionTable)` did not resolve when the
registering file imported `SectionTable` from a directory (`'./routes'` -> `routes/index.ts`)
whose index only re-exported it (`export * from './routes.constants'` or `export { SectionTable }
from ...`). The build exited 0 and every screen read `non-literal-name`, with nothing saying which
step failed. A string enum was already read, but a mixed enum (one auto-numbered or computed
member) still resolved its literal members, although a registration hands the whole value to the
injector.

## Decision

- AMAP-V0-024: through injection only, an enum is a table when every member has a literal string
  initializer; one other member makes it no table (`non-literal-member`). A router's own imported
  enum keeps the AMAP-V0-016 rule, so AMAP-V0-016 maps are unchanged.
- AMAP-V0-025: through injection only, when the imported file does not declare the name, follow
  exactly one level of `export { T }`, `export { S as T }`, `export { default as T }` or `export *
  from` in that file. A named re-export shadows `export *` (ECMAScript). Two candidates or an
  unreadable item -> `ambiguous-barrel`; a candidate that could re-export again ->
  `barrel-depth-exceeded`; a candidate outside the repository index -> `out-of-scope`; none ->
  `identifier-not-found`. A name the imported file exports itself (an exported `let`, `var`,
  `function`, `class` or unreadable `const`, a local `export { ... }` list, or any other export
  statement) shadows every re-export and is not followed (`identifier-not-found`); a candidate
  that exports the name in any form counts toward ambiguity even when it is not a readable table.
  Fail-closed by construction: an export statement the parser does not read name by name
  (`declare`, `abstract`, `namespace`, `export =`, destructuring, several declarators) counts
  every identifier up to its top-level `;` or the next top-level `export` as an exported name. A
  file whose exports the reader cannot list may export any name (`unread`): every reader exit that
  rejects or cannot decode part of an export statement routes there, and an independent whole-file
  token pass (`auditExports`) adds brackets that do not nest and match (a typed stack), a
  backslash outside a string (an escaped identifier) and any top-level `export` token no reader
  claimed. An unread file never yields a declaration: `reexported` checks `unlisted` before
  accepting a candidate's declaration, `onlyRead` is false for every name in it, and `lookup`
  checks `unlisted` for a directly imported `export default {...}` object, which has no name for
  `onlyRead` to check. The re-export
  statement is a new anchor between the declaring line and the import. Re-exports for router-file
  imports (AMAP-V0-016 follow-up 10) stay out of scope.
- AMAP-V0-026: when the scope holds exactly one registration of the name and a read through it
  fails, the map carries `{kind: "di-constant", ref: NAME, reason, path, line}` at the
  `.constant(...)` call. This is a map unknown, not a refusal, so no error code or
  `diagnostic.Refusal` site is added (error-code ownership and diagnostic coverage unchanged).
  Maps where every injected read resolves, or with no scope, are byte-identical.
- Pure-read rule (fifth review): write detection no longer names write forms. `pureRead` accepts
  a member reference `X.Y` in the router, registering or declaring file only when the expression,
  and then each bracket group it is an element of (climbing through `,` and closing brackets, the
  way parenthesized targets and destructuring patterns enclose it), is followed by a token of a
  closed set that cannot make it an assignment target: `;` or end of file, `.`, `[`, `:`, `{`, an
  identifier other than `as`/`satisfies`/`in`/`of` (a new statement after ASI, or `instanceof`),
  `==`/`===`/`!==`, or a binary operator `assigned` does not read as an assignment or increment;
  and no level follows `delete`, `++` or `--`. Everything else is a possible write and makes the
  table not read whole: `=` and compound assignments, `++`/`--`, TypeScript assertions (`!`,
  `as`, `satisfies`), for-in/of heads, a call or tagged template through the table (`this` is
  `X`), `<` (possible type arguments), and any unlisted token. Member references are checked
  before the `export default X` case, so `export default X.Y = v` is a write. The same rule
  replaces `written` in the router-side injected-parameter check (AMAP-V0-022/023). This is the
  spec's "unassigned member read" made conservative; no spec text changes.

## Evidence

- New tests `TestAMAPV0024InjectedStringEnum` (8 cases + AMAP-V0-016 guard),
  `TestAMAPV0025InjectedTableThroughBarrel` (17 cases), `TestAMAPV0026DIConstantDiagnostics`
  (9 failure + 3 control cases), all synthetic fixtures.
- Fails on base `dd90cfa6` (test file copied to a base worktree): 32 subtests fail for the right
  reason: mixed enums resolve (5), barrels do not resolve (5), no `di-constant` unknown (22).
  All pass after the change; existing AMAP-V0-016 and AMAP-V0-021..023 tests pass unchanged.
- Review follow-up (two P2 fail-open findings): `TestAMAPV0025LocalExportShadowsStar` (3 cases,
  one a plain `let` guard that already failed closed, plus destructuring and type-only checks) and
  `TestAMAPV0025UnreadStarExportIsAmbiguous` (9 cases + a lone-`let` guard). Against
  `stateconst.go` from `720789e8`, 11 subtests and the destructuring check fail because the barrel
  resolves the star source's table (local alias, imported alias; a `let`/`var`/`function`/async/
  generator/`class`/typed-`const`/export-list/destructured second star source); all pass after
  the fix.
- Second review follow-up (P2: unrecorded export forms): both tests gained `declare const`,
  `abstract class`, second-declarator `let`/`const` (scalar and table first declarator),
  `namespace` and unbalanced-bracket cases, as star source and as the barrel's own export, plus a
  guard that a loose export not naming the table still resolves. Against `stateconst.go` from
  `2962ca8c`, 11 subtests and the unbalanced-barrel check fail because the barrel resolves
  (`declare`/`abstract`/declarators/`namespace`/unbalanced as star source; declarators and
  `namespace` as the barrel's export), and the barrel-destructuring subtest fails on its reason
  (`ambiguous-barrel`, now `identifier-not-found`); all pass after the fix.
- Third review follow-up (P2: unlistable exports): `TestAMAPV0025UnlistableExportsFailClosed`
  covers `export * as "SectionTable"`, an undecodable module name (`'./\uD800'`), an unclosed
  `export enum`, an escaped class name, a quoted local export name and a dangling `export`, each
  in a star barrel, a named-re-export barrel and a competing star source (18 subtests). Against
  `stateconst.go` from `de2a0c44`, 15 resolve silently, 2 (quoted local name in the barrel) fail
  only on their reason (`identifier-not-found`, now `ambiguous-barrel`) and 1 already failed
  closed; all pass after the fix. Audited reader exits: `readReexport` (`export *` without a
  following `from`, a non-literal or inexact module name, quoted export names, unreadable list
  items -> opaque), the declaration readers (a const, enum, let/var or function/class form not
  read exactly -> `looseExport`, an unnamed or escaped declaration -> unclaimed), skipped bodies
  (`parseValue`, `enumDecl`, `closeParen` to EOF -> bracket audit) and `export` at EOF.
- Fourth review follow-up (P2: unread declaring file accepted; mismatched brackets):
  `TestAMAPV0025UnreadDeclaringFileFailsClosed` appends an escaped write
  (`\u0053ectionTable.REPORTS = 'other';`), an unclosed `export enum`, an undecodable re-export or
  `export enum Other {]` to the declaring file, reached through a star barrel, a named barrel and
  a direct import (12 subtests), and `TestAMAPV0025UnlistableExportsFailClosed` gained the
  mismatched bracket in all three positions (3 subtests). Against `stateconst.go` from `baf81e58`
  all 15 resolve `ledger` silently; all pass after the fix.
- Fifth review follow-up (P2: TypeScript assertions on an assignment target):
  `TestAMAPV0026PossibleWritesFailClosed` appends one possible write to the declaring file, reached
  through a star barrel and a direct import (30 subtests, `not-read-whole`), plus a `reads` guard
  (comparison, ternary, value member, parenthesized and nested-array arguments, a statement
  without `;`) that still resolves; `TestAMAPV0016UnprovableConstantsStayUnknown` gained `(X.Y as
  string) = v` in the router and `X.Y! = v` in the declaring file, and
  `TestAMAPV0023UnprovableInjectionStaysUnknown` gained both on the injected parameter. Against
  `stateconst.go`/`stateinject.go` from `48bd9601`, 18 subtests fail by resolving silently:
  `X.Y! = v`, `(X.Y as string) = v`, `[X.Y, rest] = v`, `({ a: X.Y, b } = v)`, `for (X.Y of v)`,
  `X.reset()` on a table with a function member, and `export default X.Y = v` (each star and
  direct), plus the four router-side cases. Already failing closed there (kept as guards):
  `(<any>X).Y = v`, `X['Y'] = v`, `X.Y += v`, `X.Y++`, `delete X.Y`, `Object.assign(X, ...)`,
  `Object.defineProperty(X, ...)` and the alias `const Y = X; Y.M = v`.
- Fifth review follow-up (P2: unread default export): `TestAMAPV0025UnreadDeclaringFileFailsClosed`
  gained a directly imported `export default { REPORTS: 'ledger' }` followed by each of its four
  unread tails (escaped write, unclosed enum, undecodable re-export, `export enum Other {]`).
  Against `stateconst.go` from `efd52c9d` all 4 subtests resolve `ledger` silently; all pass with
  `lookup` checking `unlisted` for the default export (`not-read-whole`).
- `go test ./internal/appmap ./internal/testplan ./internal/specindex ./cmd/corvint-corpus-mcp`
  and the `cmd/corvint` flows-appmap tests pass; the lane doc gates pass.

## Limits

- No diagnostic for zero or several registrations, a poisoned scope, or a router-side binding the
  reader cannot prove (owner question 16 stays open); those keep the AMAP-V0-023 unknowns only.
- `import { X } from './a'; export { X };` (local re-export list) and deeper barrels are not
  followed; such a local export only blocks resolution. Loose export reading over-approximates:
  any identifier in such a statement (an initializer's reference, a namespace member) also counts,
  so a barrel can stay `UNKNOWN` where ECMAScript would resolve. The statement end is a top-level
  `;` or the next top-level `export`; without semicolons later non-exported code is scanned too
  (more uncertainty, never less). A lexing error that keeps brackets balanced, emits no stray
  backslash and still hides a top-level `export` token (for example a misread regular expression
  or template swallowing it) is not detected; an `unread` file also blocks an otherwise
  well-formed named re-export. The pure-read rule over-reports: provable reads such as `X.Y < z`,
  `X.Y != z`, `X.Y in o`, `X.Y as T`, `X.Y(...)` (any call through the table), a parenthesized
  group followed by `(` (a semicolon-free IIFE body) or `typeof`-free type positions such as
  `let s: X.Y = v` make the table not read whole; a block closed without `;` is climbed like a group, so a following `=`
  at that level also counts. It inherits the lexer's limits (whitespace is not kept, so `X.Y! =
  v` and `X.Y != v` are one case) and reads only the router, registering and declaring files:
  writes from a third module stay unread, as the spec states. No adopter-scale qualification
  (`NOT_RUN`); `make gate` `NOT_RUN` per lane rules.
