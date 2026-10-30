# Decision 0019 — `reference-resolves` for JavaScript and TypeScript

Date: 2026-09-01. Status: accepted. Authority: repository owner, verbatim instruction "do 1-5. you
can make the correct choice on claim shape and summon an expert if needed" (2026-09-01), in reply
to the recommendation "`reference-resolves` for JavaScript. Still open from the plan's weeks 3 to
6; independent of the above." The plan is `docs/plans/BREAKTHROUGH-BET-2026-09-01.md` ("Weeks 3-6:
reference-resolves for Go, Python, and JavaScript").

## Scope

The instruction accepts one new clause, `FPK-V0-018` in `docs/specs/falsifiable-packet-v0.md`:
`syntax` rows of kind `reverse-import` and `reference` whose cited path ends in `.js`, `.mjs`,
`.cjs`, `.jsx`, `.ts`, or `.tsx` get the `reference-resolves` falsifier instead of `none`, judged
by `internal/liveverify/jsresolve` over the committed blob. The claim shape is the owner's
delegated choice, and the choice is:

- An import row passes on the specifier literal as the index recorded it (`imports ./core.js`),
  not on a re-resolution to the changed file: the row's reason carries the literal, and
  re-resolving would call the index's rule from the falsifier that must not trust it.
- The statement may span lines; the cited line passes when it lies between the keyword and the
  specifier literal, because `importEvidenceLine` cites the first line holding the literal or the
  keyword with the leaf word, which for a wrapped clause is not the statement's first line.
- `require("X")` passes although the index's web lexer deliberately never records it. Accepting
  it widens what a row can pass on, never what the index claims; a row citing a `require` line
  can only exist when the index found another specifier position on that line.
- A reference row is judged exactly as the Go arm: occurrence of the word on the cited line and a
  top-level declaration in the declaring blob (`function`, `class`, `const|let|var`, their
  `export` and `export default` forms, and `export { ... }` lists). The index emits no web
  `reference` rows today (`samePackageReferences` is Go-only), so the arm is proven by test over
  committed blobs and becomes live the day the index emits such rows.

## Explicit exclusions

No external parser and no toolchain execution: the check is the index's own lexer with line
numbers, so comments, strings, and templates never count, and the regular-expression ambiguity
that lexer accepts is accepted here. Destructuring declarations, TypeScript `type`, `interface`,
and `enum`, and `module.exports` assignments are not declarations under this clause. Rust, Java,
and every suffix without a reverse-import rule stay `none`, unchanged from decision 0007 D2. The
cmd file is `prove_javascript.go`, not `prove_js.go`: the Go tool reads a `_js.go` suffix as a
`GOOS=js` constraint and drops the file from every other build.

## Consequences

The `reference-resolves` week of the bet closes for all three languages named. Help topic
`prove` names JavaScript and TypeScript. The lexer in `jsresolve` duplicates
`internal/contextindex/webimports.go` on purpose and says so in its package comment; a later
change that exports a shared line-tracking lexer from the index may replace it, but the falsifier
must keep reading the committed blob and never the index's edge table.
