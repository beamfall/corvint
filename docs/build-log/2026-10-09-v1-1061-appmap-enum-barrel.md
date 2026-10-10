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
  `==`/`===`/`!==`, or a binary operator other than `?` that `assigned` does not read as an
  assignment or increment; and at no level does `delete`, `++` or `--` appear before the
  expression in its statement, so a prefix assertion such as `delete <any>X.Y` cannot hide one
  (sixth review). That backward scan (`prefixWrite`, seventh review) skips each balanced `(...)`,
  `[...]` and `{...}` group with a kind-checked stack (the `{}` of `delete <{}>X.Y` no longer ends
  it), climbs out through an unmatched `(` or `[`, stops only at a `;` or unmatched `{` outside
  every skipped group or at the start of the file, and fails closed on brackets it cannot match;
  `<...>` needs no matching of its own because brackets inside type arguments nest and a `;` there
  sits inside `{}`. Everything else is a
  possible write and makes the table not read whole: `=` and compound assignments, `++`/`--`,
  TypeScript assertions (`!`, `as`, `satisfies`), for-in/of heads, a call, optional call (`?`,
  `?.`, sixth review) or tagged template through the table (`this` is `X`), `<` (possible type
  arguments), and any unlisted token. Seventh review: the write checks run on every use before
  any form that may count as a read (`readUse`): a member `X.Y`, a computed member `X[k]` checked
  through its `]`, and a bare `X`; `typeof X`, `export default X`, `export { X }` and the
  registering `.constant(...)` argument only narrow which unwritten uses are reads, never bypass
  the checks, so `typeof X.Y++`, `typeof X.reset()`, `typeof X['Y']++` and
  `export default X['Y'] = v` are writes. A spread `...X` is a use, not a property name. The
  router-side injected-parameter check (AMAP-V0-022/023) uses the same `readUse` before its
  `typeof` form; a parameter binding position stays the only non-use. Nothing in an unread file
  (`unlisted`: a stray backslash, unmatched brackets) is injected, as it already was not read
  whole; and a name used inside a template substitution, which the lexer folds into one template
  token, is neither read whole nor injected: the lexer keeps each template's raw source from its
  first `${`, skips nested quotes and templates when bounding a substitution, and marks a template
  it cannot bound (a `/` inside a substitution, or no closing backtick), which makes the file
  unread. This is the
  spec's "unassigned member read" made conservative.
- Eighth review (lexer fail-closed; chain bindings): the lexer no longer guesses where a `/`
  starts a regular expression. `regexStarts` reports whether its answer is sure; a `/` after `}`,
  a contextual `of`/`yield`/`await`, `<`, a `>` not of `=>`, or a TypeScript non-null `x!` is
  doubt, and so are an unterminated regular expression, string, block comment or template, a `/`
  or `\` inside a template substitution, a `//` comment holding a lone CR or U+2028/2029, a
  non-ASCII identifier character that is not a letter, digit, mark or connector, and an HTML-like
  comment. Doubt marks the first token unsure and `parseConstFile` makes the file unread. Sure
  answers were corrected: after a `++`/`--` run (maximal munch, counted over glued `+`/`-`) a `/`
  is division; a keyword after `.` is a property; `.if(` is not a control head; `for await (` is;
  `extends`, `default`, `break` and `continue` allow a regular expression. A `.tsx`/`.jsx` file
  with any `<` is unread (JSX text is not lexed). Chain bindings: every file on the AMAP-V0-025
  chain (registering file, barrel, declaring file) must prove every other binding of the table
  only read: an alias import of the name (or `default`) passes `onlyRead`, and a namespace
  import, dynamic `import(...)`, `require(...)` or unreadable import item may name only a package
  or a repository file that is not the declaring file, re-exports nothing and is read; a
  non-literal or unresolved specifier fails closed. This makes AMAP-V0-025/026 stricter than the
  accepted text, which left writes through another binding as a third-module limit; the spec
  amendment records the stricter rule under decision 0475's fail-closed intent.
- Ninth review (structural, no spec text change): `regexStarts` is now a standalone function
  that answers "regular expression, surely" only from a closed allowlist of predecessors: the
  start, `( , = : [ ? ~ & | ^ * % ; { /`, a `regexKeywords` word, an odd `+`/`-` run, a `)`
  closing a control head, a `=>`, and a `!` run whose own predecessor is on that list. Division is
  sure only after a name, a property, a literal, `]`, a `)` closing no control head, or an even
  `++`/`--` run; everything else is doubt, including a `!` run after an operand (`n!! / x`, the
  TypeScript postfix non-null), `debugger`, a label after `break`/`continue`, `.`, `>>` and `<`.
  Line terminators are exactly LF, CR, U+2028 and U+2029 everywhere: a hashbang or `//` comment
  ends at the first of them and is doubt when that is a lone CR or a separator (code after it
  shares the counted line); strings and regular expressions end unclosed at any of them, and
  U+2028/U+2029 in code are whitespace. `bindingsRead` now checks, in every chain file, every
  import from a module that may hold the table under any name (`mayHold`: the declaring file, a
  file that re-exports or exports a name it imports, one the reader cannot read, or an
  unresolved specifier) with `onlyRead`, whatever name it imports, so the declaring file's
  `default`, a barrel's rename and a relaying file's alias are all covered; imports of the
  table's names from any module stay checked as before. This applies the accepted AMAP-V0-025
  rule ("every other binding of the table it holds") to names the earlier code did not list, so
  the spec text is unchanged.
- Tenth review (structural, no spec text change): a parenthesized default export of an imported
  table (`export default (SectionTable)`) recorded neither a default nor an exported reference,
  so the relaying file was not a possible holder and the registering file's write through it went
  unchecked. Rather than recognize more export forms, `relays` (now a `constTable` method) asks
  how the file uses what it imports: a file relays when it exports an import binding by name, uses
  one in any way `onlyRead` does not prove a member read or `typeof` (an alias, an argument, a
  parenthesized, `as`, `satisfies`, `!` or sequence expression), starts an `export default`
  expression with one (`export default T || {}`), or holds a namespace import, dynamic import,
  `require` or unread import item of anything but a package. A file can hold another module's
  value only through those bindings, so any default-export expression the reader does not
  understand fails closed.

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
- Sixth review follow-up (P2: optional call through the table; prefix assertion hiding
  `delete`): `TestAMAPV0026PossibleWritesFailClosed` gained `SectionTable.reset?.()` (table with
  a function member) and `delete <any>SectionTable.REPORTS`, star and direct, and
  `TestAMAPV0023UnprovableInjectionStaysUnknown` gained both on the injected parameter. Against
  `stateconst.go` from `fb66afdb` all 6 subtests resolve silently; all pass with `?` removed from
  the safe followers and the statement-wide prefix scan.
- Seventh review follow-up (P2: `delete <{}>X.Y`; `typeof` before the member check; audit):
  `TestAMAPV0026PossibleWritesFailClosed` gained, star and direct, `delete <{}>X.Y`,
  `typeof X.Y++`, `typeof X.reset()`, `typeof X['Y']++`, `export default X['Y'] = v`,
  `[...X.Y] = [...]`, a write inside a template substitution, and a write after a template whose
  substitution holds `'{'`; its `reads` guard gained `typeof` reads, a template with `/` outside
  its substitution and an earlier block holding `++`. `TestAMAPV0023UnprovableInjectionStaysUnknown`
  gained the injected `delete <{}>`, `typeof` increment, method call and computed write, spread
  rest, template and escaped-identifier writes. Against `jslex.go`, `stateconst.go` and
  `stateinject.go` from `6dc1ea03`, 19 subtests resolve silently (12 star/direct, 7 injected; the
  star/direct `typeof X.Y++` and `typeof X.reset()` already failed closed there because
  `onlyRead` checked members first); all pass with the fix.
- Eighth review follow-up (escaped identifier in a substitution; postfix division read as a
  regular expression; write through a barrel's own import): `TestAMAPV0023UnprovableInjectionStaysUnknown`
  gained an escaped template write and `++`/`--` division on the injected parameter;
  `TestAMAPV0026PossibleWritesFailClosed` gained, star and direct, `++`/`--` division, a keyword
  property and a method named `if` before division, a regular expression after `break`, and one
  after `for await`; its `reads` guard gained division and regular expressions after `)`, `]`,
  `return`, `typeof`, `void`, `=>` and postfix `++`/`--`. `TestAMAPV0025UnreadDeclaringFileFailsClosed`
  gained ten lexer-doubt tails (escaped template, contextual `of`, non-null and type-argument
  division, regular expression after a block, U+2028 and CR comment ends, NBSP, unclosed comment,
  HTML comment) across its four shapes; new `TestAMAPV0025ChainBindingsFailClosed` (14 cases: alias,
  same-name, namespace, `require`, dynamic, computed and string-item imports in the barrel, an
  unresolved specifier, alias and namespace in the registering file, a self namespace and one via
  the barrel in the declaring file, a direct alias, a namespace import beside a locally declared
  table) with a reads guard (alias reads, namespace imports of a plain module and a package); and
  new `TestAMAPV0025JSXFileUnread`. Against `jslex.go`, `stateconst.go` and `stateinject.go` from
  `92882561`, 70 subtests resolve silently (3 injected, 14 chain, 2 JSX, 40 unread-declaring, 12
  possible-writes); the reads guards pass before and after; all pass with the fix.
- Ninth review follow-up (repeated non-null `!` read as a prefix; `default`/renamed/relayed
  bindings unchecked; hashbang ended by CR or a separator): `TestAMAPV0025UnreadDeclaringFileFailsClosed`
  gained `n!! / (...)`, a regular expression after `break label` and after `debugger` (four
  shapes each); `TestAMAPV0025ChainBindingsFailClosed` gained a default import of the declaring
  file, the original name imported beside a barrel rename, and an alias through a relaying file;
  new `TestAMAPV0025HashbangLineEnds` (CR, U+2028, U+2029; direct and star) with a CRLF reads
  guard. The reads guards gained an import of another table and of a plain module's function
  (still resolved) and `!!/re/`, `? /re/ :` regular expressions. Against `jslex.go` and
  `stateconst.go` from `0939aeb7`, 21 subtests resolve silently (12 unread-declaring, 3 chain, 6
  hashbang); the reads guards pass before and after; all pass with the fix. The full
  `go test -count=1 ./internal/appmap` passes (AMAP: 72 tests, 335 subtests).
- Tenth review follow-up (a parenthesized default export relays the table unchecked):
  `TestAMAPV0025ChainBindingsFailClosed` gained the review's input (registration moved to line 2)
  and a default import from a relaying file that exports the table as `(T)`, `T as ...`,
  `T satisfies ...`, `T!`, `(0, T)`, `T || {}`, `wrap(T)`, a local alias, `export const R = T`
  and `NS.SectionTable` through a namespace import. Against `stateconst.go` from `c120f247`, 10
  subtests resolve silently (all but `export const R = T`, which the loose export already caught);
  the reads guard passes before and after; all pass with the fix. The full
  `go test -count=1 -v ./internal/appmap` passes (447 passing tests and subtests).
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
  (more uncertainty, never less). The lexer fails closed where it cannot place a `/` or a
  literal end, but a wrong sure answer it still believes would hide tokens undetected; the sure
  rules are the ECMAScript ones for the previous token (identifier, keyword, punctuator, literal)
  without a full grammar; a regular expression is sure only after an allowlisted predecessor.
  The fail-closed lexing over-reports: a `/` after `}`, `of`, `debugger`, `.`, `>>`, a label after
  `break`/`continue`, TypeScript `x! / y` or `x!! / y`, `if (c) !/re/`, `f<T>() / y`, any `-->`,
  a `//` comment or hashbang ended by a lone CR or U+2028/U+2029, and any `.tsx`/`.jsx` file
  with `<` (generics included) make a file unread. An `unread` file also blocks an otherwise
  well-formed named re-export. The pure-read rule over-reports: provable reads such as `X.Y < z`,
  `X.Y != z`, `X.Y in o`, `X.Y as T`, `X.Y ? a : b`, `X.Y ?? d`, `X.Y?.length`, `X.Y(...)` (any
  call through the table), any `delete`, `++` or `--` earlier in the same statement outside a
  complete bracket group (for example `i++, X.Y`, or `i++` before a block closed without `;`), a
  bare `typeof X` or `export default X` followed by anything but a read follower (such as
  `let s: typeof X = v`), `export { X as Y }`, a spread `...X`, any use of the name inside a
  template substitution, a template whose substitution holds `/`, a parenthesized
  group followed by `(` (a semicolon-free IIFE body) or `typeof`-free type positions such as
  `let s: X.Y = v` make the table not read whole; a block closed without `;` is climbed like a group, so a following `=`
  at that level also counts. It inherits the lexer's limits (whitespace is not kept, so `X.Y! =
  v` and `X.Y != v` are one case) and reads only the router, registering and declaring files:
  writes from a module off the AMAP-V0-025 chain stay unread, as the spec states. On the chain,
  another binding of the table fails closed rather than being read: a namespace import of an
  unresolved or undeclared package, or of any file that re-exports, makes the table not read
  whole even where the file never touches it, and any import from the declaring file, a barrel,
  a file that relays an import or an unresolved module must be only read, whatever it imports
  (a function imported from the declaring file and called fails closed). A file relays, and so
  may hold the table, whenever it uses an import other than as a member read or `typeof`, or
  holds a namespace import, dynamic import or `require` of anything but a package: a helper
  module that calls what it imports makes every chain import from it subject to the write
  checks. The router-side (AMAP-V0-016, not injected) path
  does not yet apply the chain-binding check to alias and namespace imports; that needs
  AMAP-V0-016 text and is left open. `router.go` and `tests.go` do not consume the lexer's
  unsure mark. The bracket
  matching relies on `auditExports`, which marks any file with unmatched brackets unread, so
  `enclosingOpen`/`enclosingClose` run only on balanced tokens. No adopter-scale qualification
  (`NOT_RUN`); `make gate` `NOT_RUN` per lane rules.
