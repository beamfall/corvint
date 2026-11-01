# Non-Go impact V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner-requested GitHub issue #390 (native V1-0537), and the
2026-09-30 instruction to implement it. This technical contract proposes concrete
semantics; issue intent does not accept generated implementation detail.

## Agent digest
- Claim: Ruby, JavaScript and TypeScript path/range impact binds syntax evidence and explicitly names unresolved dynamic dispatch.
- Status: proposed/experimental; local source and focused conformance implemented; human acceptance and integration pending. NGI-V0-007..011 (declared `string-key-registry/0` dialect: literal registry keys to the tests passing them) are accepted under the owner's standing auto-accept (2026-10-11; V1-1365) and implemented over synthetic repositories. NGI-V0-012..015 (an unresolved entry widens every JS/TS path; lazy `require()`/`import()` entries; a non-exported top-level registry; a same-file top-level `const` key) are accepted under the same standing auto-accept (V1-1455, V1-1456, V1-1457). NGI-V0-016..021 (Ruby named-constant reverse references by autoload path convention: path-to-constant mapping, default roots or a declared `ruby-autoload/0` file, literal inflections, a reference class ranked below imports, metaprogrammed names kept unknown, and a report of the roots and inflections applied) are accepted under the same standing auto-accept (V1-1674). NGI-V0-027 (`bundle exec rspec` named for a Ruby path when the tree tracks a root Gemfile and `.rspec` or `spec/`) is accepted under the same standing auto-accept (V1-1796; GitHub #1130). NGI-V0-051..053 (the Ruby command in committed-range receipts; RSpec gated on an rspec gem in the committed Gemfile or Gemfile.lock; `bin/rails test` or `bundle exec rake test` for a proved Minitest suite) are accepted under the same standing auto-accept (V1-1804, V1-1805; GitHub #1128, #1130). NGI-V0-059 (the unknown-language line in a Ruby range receipt whose plan names no language command) is accepted under the same standing auto-accept (V1-1827; GitHub #1130). NGI-V0-060..062 (the root AGENTS.md declared verify commands close path and range plans, a `verification_authority` label separating them from inferred commands, and one closing-gate rule for path and range impact) are accepted under the same standing auto-accept (V1-2071, V1-1848). NGI-V0-033..040 (Ruby autoload reverse references on by default for any request naming a Ruby path, a bounded second hop, advisory marking, no `reverse-import-rule-unavailable` for a mapped path, range impact, RSpec test paths in the Ruby reading's test rule, the reusable `internal/rubyautoload` mapper, and a bounded reference index) are accepted under the same standing auto-accept (V1-1794, V1-1700; GitHub #1129). NGI-V0-063..064 (the RSpec test rule in every row of a Ruby request, and the bounded unknown counted only on paths a cut could reach) are accepted under the same standing auto-accept (V1-1823). NGI-V0-065..067 (an accessor call inside a template substitution, a registry written after its declaration, and an eager entry with open constructor arguments; NGI-V0-067 amends NGI-V0-008) are accepted under the same standing auto-accept and decision 0552 (V1-1478). NGI-V0-068..069 (one shared syntax-profile path impact entry for CLI and MCP with no implicit budget, and `--language-profile` in any position) are accepted under the same standing auto-accept (V1-2686; GitHub #1786).
- Exists: Experimental pinned whole-tree import JSONL and paginated import-graph MCP export (`NGI-V0-301..308`), including literal Angular component resource edges and single-candidate Zeitwerk constant edges; Ruby/JS/TS versioned positional, committed-range and MCP admission; bounded structured unknowns; existing JS/TS import rules; a declared string-keyed registry dialect and advisory Ruby named-constant reverse references by autoload convention, two hops deep, for default, syntax-profile and range impact.
- Blocked on: shared registry/CEM closeout, draft publication, integration and owner acceptance; no runtime-dispatch qualification.
- Read next: Requirements; Trust and failure boundaries; Acceptance and traceability.

## User and measurable job

A maintainer of Ruby/JS/TS product code needs immutable impact evidence for paths
or a complete committed range. The baseline CLI already admits these indexed
paths, but the range compiler requires a Go module and only qualifies Go hunks;
MCP admission requires .go. Syntax observations cannot establish runtime call or
test closure. The useful result is bounded evidence with an open semantic frontier.

## Requirements

- `NGI-V0-001`: Positional impact and MCP impact MUST admit tracked `.rb`, `.js`,
  `.jsx`, `.mjs`, `.cjs`, `.ts` and `.tsx` paths. MCP schema and runtime MUST agree
  on the same closed suffix set, retain path confinement, duplicates and bounds,
  and keep unchanged Go admission. CLI's broader indexed-path admission remains.
- `NGI-V0-002`: Default and expanded committed ranges MUST bind Ruby/JS/TS path
  membership and exact old/new textual hunk spans using the existing range
  identity, status, ancestry, path, source-size, deadline and drift checks.
  Go module/package/parser preconditions MUST apply only to changed Go members;
  root-level Ruby/JS/TS and repositories without Go modules MUST be admitted.
- `NGI-V0-003`: Every Ruby/JS/TS changed path MUST name a syntax-only dynamic
  dispatch limitation in structured unknowns independent of ranked-result limits.
  Specific detected computed dispatch, dynamic load, reflection or Rails autoload
  MUST add closed frontier codes. Detector absence MUST NOT establish closure.
  Positional CLI requests select `--language-profile non-go-syntax-v0`; the
  historical default path receipt remains byte-identical. MCP non-Go requests
  and non-Go range receipts select/name this experimental profile automatically.
  The selector is unavailable with `--base` or `--working-tree-untracked`.
  Ruby reverse-import (`require`) resolution remains explicitly unavailable; the
  syntax profile adds named-constant reverse references by autoload convention
  (`NGI-V0-016` to `NGI-V0-021`), which do not close it; JS/TS reuses
  existing relative/profile import rules, plus the tsconfig/jsconfig alias arm
  `GPK-V0-077` to `GPK-V0-081` propose (proposed, pending owner acceptance;
  V1-0958), without inferring runtime call semantics. A non-test JS/TS changed
  path in a repository with an unresolved bare import specifier carries
  `bare-import-unresolved` with the repository-wide count (`GPK-V0-080`).
- `NGI-V0-004`: Added paths MUST carry the same receipt/evidence envelope and
  target Git blob binding as Go evidence. Range markers/ADR relations MUST remain
  exact-hunk qualified and caller-authored authority MUST remain withheld.
  Result omissions, critical misses and unsupported language members MUST remain
  explicit. Legacy Go-only receipts and canonical digest preimages MUST retain
  their exact bytes when no new language members occur. Existing refusal
  conditions and codes remain. Module validation keeps historical precedence in
  repositories without admitted Ruby/JS/TS source, including the no-module
  daily-workflow abstention. In a repository admitting those languages it depends
  on changed Go membership and follows snapshot/base validation; exact precedence
  for multiply-invalid mixed repositories is not a compatibility claim. Frozen
  oracle expectations remain immutable and are never regenerated from a candidate.
- `NGI-V0-005`: `impact --help` and MCP description MUST publish a per-language
  capability table identifying path/range support, syntax-only import rules and
  unresolved dispatch. Working-tree-untracked remains the separately bounded Go
  profile. Help MUST distinguish receipt production from semantic completeness.
- `NGI-V0-006`: Conformance MUST include each new suffix/language, negative
  dynamic cases and comment/string twins, mixed Go/web ranges, root-level inputs,
  no-Go-module repositories, limits, dirty/drift/excluded/binary refusals, and
  byte-identical repeat runs. Tests MUST establish schema/runtime agreement and
  unchanged Go-only and historical default path bytes. The CLI selector MUST be
  tested for invalid/repeated values and mutually exclusive profiles. No test execution or qualification is implied
  by an impact receipt.

### Declared string-keyed registries (V1-1365; GitHub #871)

A module registered in an exported object literal and reached by tests only
through an accessor that takes the literal key has no import edge to those tests.
This narrow dialect, declared like the application map's router dialects, binds
the key instead. It is read only by syntax-profile path impact (positional
`--language-profile non-go-syntax-v0` and MCP); the historical default receipt
and range impact are unchanged.

- `NGI-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365) A
  registry MUST be declared as an entry of the optional `registries` member of a committed
  `application-map-manifest/0` document: `path` (a clean repository-relative JS/TS path with
  no backslash and no `.git` first segment),
  `export` (an identifier), `dialect` exactly `string-key-registry/0`, and 1..16 unique
  identifier `accessors`; at most 64 entries, unique by path and export. The application map
  validates the member with the same closed check and refuses an invalid one with
  `appmap-invalid-manifest` but does not read it. Impact reads every indexed manifest at the
  evaluated revision; a member that does not decode or validate, or a manifest that mentions
  `registries` but exceeds 256 KiB or is not strict JSON (a duplicate member included), is
  counted as `registry-declaration-invalid` on every JS/TS changed path and contributes no edge.
  No accessor or registry is ever inferred.
- `NGI-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365) The
  dialect MUST read the declared export as `export const|let|var NAME [: Type] = { ... }` with
  the existing web lexer. An entry gives a key -> module edge only when its key is a quoted
  string, a template literal without substitution, an identifier, or a computed key holding
  exactly one such literal, and its value is `X`, `ns.X`, `new X(...)` or `new ns.X(...)`
  (optionally followed by `as` or `satisfies`) where `X` or `ns` is a value binding of a static
  import; the edge's module is that import's specifier as resolved by the existing reverse-import
  rules. Every other entry (a spread, a computed key needing evaluation, a template with a
  substitution, a method, an escaped or overlong key, a value that is not an imported binding)
  is counted, never dropped. A declared file whose export is not such an object literal (for
  example a re-export) or is unreadable has no known key.
- `NGI-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365) A call
  site is a call of a declared accessor, by name or by a local alias from an import clause,
  in any indexed JS/TS source; a declaration of the method or function is not one, and an
  uncalled member reference (`dp.accessor` passed on) is a call site with a non-literal key. Its
  key is literal only when the first argument is a closed quoted string or substitution-free
  template followed by `,` or `)`. For a changed path the registry imports, impact MUST return each
  calling file whose literal key the registry maps to that path, as a `test` result (a test path)
  or `reference` result, with two `high`/`syntax` evidence rows pinned to Git blobs: the registry
  entry line and the call-site line. The same key in another registry selects only through that
  registry's accessors.
- `NGI-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365) Selection
  MUST fail closed. A non-literal key at any call site of a registry's accessor adds
  `accessor-key-nonliteral` (count of such calls, with `registry` and `accessor`); an
  unresolved entry adds `registry-key-unresolved` (count, with `registry`); an unresolved export
  adds `registry-export-unresolved` and applies to every JS/TS changed path, and to no changed
  path of another language (Ruby included; V1-1479). Each of these, and
  a change to the registry file itself, MUST widen selection to every caller of that registry's
  accessors with `medium` call-site evidence. A source that names an accessor but cannot be
  read or lexed completely adds `registry-caller-unavailable`. These rows sit in structured
  unknowns, independent of the result limit.
- `NGI-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1365)
  Conformance MUST cover the issue-shaped fixture (several entries, literal-key specs, a spec
  with a computed key, a change to the registry file), a registered module under a test tree,
  and the hostile cases: a spread entry, a re-exported registry, an aliased accessor import,
  template keys with and without substitution, a key collision across two registries, an
  invalid declaration, and an unchanged default-profile receipt. The real CLI output on the
  fixture is retained in the build log.

The following rows extend NGI-V0-008 and NGI-V0-009 with further recognised shapes; they do not
change what any shape those rows already recognised selects, and every shape they do not name
stays counted and widened under NGI-V0-010 (GitHub #945, #946, #947).

- `NGI-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1455) A
  declared registry with any unresolved entry may map a key to a module no import edge reaches
  (for example one loaded only by `require()`), so its `registry-key-unresolved` row and the
  widening to every caller of its accessors MUST apply to every JavaScript or TypeScript changed
  path, as an unresolved export already does, and not only to paths the registry imports. A
  changed path in another language gets no registry row.
- `NGI-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1456) An entry
  value that is an arrow with no parameters whose whole body is `new (require(S).X)()` with no
  constructor arguments, `require(S).X` (including `.default`), or `import(S).then(CB)` where CB
  is `P => P.X` or `P => new P.X()` (P optionally parenthesised), optionally followed by `as` or
  `satisfies` and a dotted type name (repeatable), with S a closed quoted string or
  substitution-free template, MUST give a key -> module edge when S is a relative or
  profile-alias specifier that the existing reverse-import rule (c) resolves, from the registry
  file, to an indexed JS/TS source (directly or as a directory `index`). The `require` forms
  apply only when `require` is unbound in the registry file: the file has no identifier escape
  (`\`), and every occurrence of `require` outside a member access is a call that is not a
  function declaration or the name of a one-line TypeScript ambient `declare const|let|var
  require:` declaration, which binds no value (V1-1479). The edge binds a changed path exactly as rule (c) binds an importer to
  it. A bare or unresolvable specifier, an arrow with parameters or a block body, a dynamic or
  concatenated S, constructor or callback arguments, any other callback, an expression after
  the assertion, and any other body shape stay unresolved entries.
- `NGI-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1456) The
  declared `export` identifier MAY also name a top-level `const NAME [: Type] = { ... }` the file
  does not export (read through an accessor the file defines); no manifest field changes. A
  `const` nested in a block, function or call, or a member access ending in NAME, is not the
  registry, which then stays `registry-export-unresolved`. So does a file whose module export
  NAME may be another binding: one with an `export { ... }` clause naming NAME, `export *`,
  `export =`, or any `exports` word.
- `NGI-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1457) A call
  site's key is also literal when its first argument, followed by `,` or `)`, is an identifier
  bound in the same file by a top-level `const NAME = L` (optionally `as const`) that ends the
  statement, L being a closed quoted string or substitution-free template, and every other
  occurrence of NAME in that file outside a member access is itself such a lone accessor
  argument. Any other occurrence (an assignment, a parameter, catch or block binding, a nested
  or repeated declaration, an import, another read), a type annotation, a `let` or `var`
  binding, a template with a substitution, a concatenation, a binding continued past the literal,
  and any file containing a `with (...)` statement or an identifier escape (`\`) keep the key
  non-literal. A rest or spread `...NAME` is an occurrence, not a member access.

The following rows close three shapes decision 0552 found open in V1-1478. NGI-V0-067 amends
NGI-V0-008: an eager `new X(...)` entry keeps its key -> X edge only when its arguments are
closed.

- `NGI-V0-065`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1478) A call
  of a declared accessor inside a template substitution (`` `${dp.accessor('k')}` ``) MUST be a
  call site like any other, with its own line, and its key is read by NGI-V0-009 and NGI-V0-015.
  The template token precedes the tokens of its own substitutions, so a template passed as an
  accessor argument whose substitution supplies part or all of the key (`` `ctx:${n}` ``,
  `` `${'k'}` ``) stays non-literal, nested or not. The registry object reader does not see
  substitution tokens. Failure mode: before this row the substitution body was one opaque token,
  so a spec reaching a registry key only inside one was never selected for the key's module.
  Rollback: revert the substitution tokens in `registryCallTokens`; the call sites drop out.
- `NGI-V0-066`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1478) A write
  to a readable registry after its declaration MUST widen like an unresolved entry: the
  registry's `registry-written-after-declaration` unknown, with the count of writes, applies to
  every JS/TS changed path, and every caller of its accessors is selected with the reason that
  the registry is written after its declaration. A write is an assignment or compound assignment
  (`=`, `op=`, `??=`, `||=`, `&&=`), a prefix or postfix `++`/`--`, or `delete` through the
  object or a member chain on it, or `Object.assign|defineProperty|defineProperties|
  setPrototypeOf` or `Reflect.set|defineProperty|deleteProperty|setPrototypeOf` with it as the
  first argument. Writes are counted in the declaring file through the declared name and in any
  indexed JS/TS source through a value import of it (named, default, or a namespace member),
  including a named import of the export whose module is unresolved. A declaration of the name,
  a `typeof`/`keyof` query, a comparison, an arrow and a read are not writes. Retained limits:
  an alias (`const r = registry`), a destructuring target and a write through a function the
  registry is passed to are not followed, and a source is scanned only when it spells the
  export name or the module stem (the directory name for an index file), so a bare package
  specifier that spells neither is not scanned. Failure mode: before this row a spec that relied
  on a key a setup file added at run time was treated as closed. Rollback: drop `registryWrites`.
- `NGI-V0-067`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1478) An eager
  `new X(args)` or `new ns.X(args)` entry whose arguments are not all closed literals (quoted
  strings, substitution-free templates, numbers, `true`, `false`, `null`, `undefined`) MAY hand
  any module to the instance, so the registry's `registry-constructor-arguments-open` unknown,
  with the count of such entries, MUST apply to every JS/TS changed path, and each caller
  passing such an entry's literal key is selected with the reason that the entry passes
  constructor arguments that may reach another module. The key -> X edge stays precise. Callers
  of other keys are not widened by this row. Failure mode: before this row a change to a module
  reached only through a constructor argument selected no spec. Rollback: drop the `arguments`
  flag in `registryValueBinding`.

### CLI and MCP path impact parity (V1-2686; GitHub #1786)

The following rows were found open on main-95be041: on a Ruby path the CLI compiled the syntax
receipt under the historical 5952-byte harness default budget, while MCP compiled it unbudgeted,
so the CLI dropped advisory reference and test rows the MCP receipt kept.

- `NGI-V0-068`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2686) A
  syntax-profile path impact receipt, selected by `--language-profile non-go-syntax-v0` or by a
  request naming a Ruby path (`NGI-V0-033`), MUST be compiled by the one shared entry
  `contextindex.PathImpactSyntax`, which both CLI `corvint impact` (and its batch verb) and MCP
  `corvint.impact` call. For the same paths, revision, limit and caller budget the two surfaces
  MUST return the same receipt: results, advisory rows, coverage, uncertainty and unknowns. Only a
  caller budget (`--budget-bytes`, `budgetBytes`) compiles it; no surface adds an implicit budget.
  The historical default profile keeps its harness default budget for a non-Ruby path no
  reverse-import rule names, which MCP never serves; `impact --help` names that difference.
  Failure mode: the CLI silently omitted advisory rows and reported `advisory_results: 0` where
  MCP reported 3. Rollback: restore `impactBudget` for syntax-profile requests.
- `NGI-V0-069`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2686)
  `--language-profile` MUST be accepted before, between or after the paths and other impact
  options, in both the space and `=` forms, and every placement MUST yield the same receipt bytes.
  The reported refusal of the flag in first position did not reproduce at main-95be041 or at
  this change's base; this row pins the behavior end to end. Failure mode: a position-dependent
  refusal. Rollback: none needed; the parser already accepted every position.

### Ruby named-constant reverse references (V1-1674; GitHub #1077)

Rails and Zeitwerk load a constant from the file its name implies, so a caller often names
`Thing` with no `require`. NGI-V0-016 to NGI-V0-021 introduced the reading for the
`non-go-syntax-v0` path impact of a changed `.rb` path only; NGI-V0-033 to NGI-V0-040
(V1-1794, V1-1700; GitHub #1129) amend that opt-in so any path or range request naming a Ruby
path receives it by default, add a bounded second hop, and mark every such row advisory.

- `NGI-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1674) A
  Ruby path below an autoload root (`NGI-V0-017`), and not below an ignored directory, MUST
  map to the constant its relative path names: each directory and the basename without `.rb`
  camelized as ActiveSupport does (split on `_`, capitalize each word, acronyms applied per
  word, a whole-basename override first), joined with `::`. The longest matching root wins.
  A segment that is not lower snake case (`[a-z][a-z0-9_]*`, no empty word) or does not
  camelize to a constant name leaves the path unmapped. An unmapped changed path carries
  `ruby-constant-unmapped` and no constant reference rows.
- `NGI-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1674) Roots
  are the documented defaults unless `.corvint/ruby-autoload.json` is present: each `app/<dir>`
  holding Ruby source except `assets`, `javascript` and `views`, each `app/<dir>/concerns`,
  and `lib`, with `lib/assets` and `lib/tasks` ignored. The declaration is one strict JSON
  object `{"schema":"ruby-autoload/0","roots":[...],"inflections":{word:Constant}}` of at most
  64 KiB, 1 to 64 distinct clean relative roots inside the repository (never `.`, `..` or a
  `../` path) and outside `.git`, and at most 256 inflections
  with `word` matching `[a-z0-9_]{1,64}` and `Constant` matching `[A-Z][A-Za-z0-9_]{0,63}`;
  declared roots replace the defaults and ignore nothing. An unreadable, excluded, unknown-member,
  duplicate-key, trailing-document or otherwise invalid declaration maps no path and carries
  `ruby-autoload-declaration-invalid`; it never falls back to the defaults. Inflections are also
  read from non-test Ruby source: a literal `.acronym "API"` (or `('API')`) and the literal
  `"word" => "Constant"` pairs of one `.inflect(` call (Zeitwerk overrides, up to 64 lines to
  its `)`). Declared entries come first; for each word the first entry read wins. Any other
  `.acronym` or `.inflect(` form, any `.collapse`, and, without a declaration, any
  `autoload_paths`, `eager_load_paths`, `autoload_once_paths`, `autoload_lib`,
  `autoload_lib_once` or `push_dir` site counts in `ruby-autoload-config-unresolved`, except one
  generated Rails 7.1+ line `config.autoload_lib(ignore: %w[...])`, alone on its line but for a
  trailing comment, with a literal word list: it adds `lib` as a root and ignores exactly
  `lib/<word>` for each listed word, replacing the default ignored set (V1-1850,
  `RUBY-AFFECTED-V0-042`). A second such line still counts.
- `NGI-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1674) For a
  mapped changed path naming constant C, every other indexed Ruby source with a constant
  reference resolving to C or a constant nested in C MUST be one result: kind `reference`
  score 525, or kind `test` score 500 for a test path (the shared test rule, a `spec/`
  directory or a `_spec.rb` name), each strictly below every direct and type-only import
  score (the weakest, a type-only test import, is 550). Its summary names C and the autoload
  convention; at most 10 evidence rows each bind
  the referencing line and blob with confidence `medium` and authority `syntax`. References
  are read from the lexically masked source (no comment, string or heredoc text), fully
  qualified (`::A::B`) or relative (`B`, `A::B`), excluding labels and method calls `Name(`.
  A single-segment unrooted `Name = value` (not `==`, `=~` or `=>`) is a local definition of
  Name in the innermost enclosing namespace, never a reference; `Name ||= value` is not a
  definition but a reference, since it reads and keeps an existing Name; a rooted
  `::A::Name = value` defines `A::Name` and a qualified `A::Name = value` defines Name in the A
  that lexical lookup finds (as for a qualified reopen below), and neither is a reference to
  that constant, though the namespace A it names is one (V1-1714); every `class`/`module` the
  file opens is also a local definition, and each local definition's namespace prefixes exist.
  A relative reference resolves to the innermost enclosing `module`/`class` namespace N for
  which N::first-segment is a mapped constant, a local definition, or a namespace prefix of
  either, else to itself, so a local definition shadows an autoloaded constant of the same
  name. A simple name after a `class` or `module` keyword is defined inside its enclosing
  namespace; a qualified `A::B` reopens B inside the A that lexical lookup finds from the
  enclosing namespace (the innermost N with N::A mapped or locally defined so far, else the
  top-level A), and the body's enclosing namespaces are that constant then the outer ones. A
  superclass resolves in the scope outside its class. Every block opener (`module`, `class`,
  `class << self`, `def` with a body, `begin`, `case`, statement `if`/`unless`/`while`/`until`/`for`,
  and `do`) counts toward a nesting bound of 256 open blocks; scanning stays linear in the
  source size. A file whose block openers and `end`s do not balance, or that nests past the
  bound, resolves each relative reference to itself and under every namespace it opens, never
  fewer. Failure mode (V1-1714, recorded): a qualified reopen or qualified assignment resolves
  its first segment against local definitions made earlier in the file only, so a later local
  definition of that namespace does not redirect it. This matches Ruby's own lookup when the
  file is loaded top to bottom; code that runs the reopen later (from a method or block) is not
  modelled.
- `NGI-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1674)
  Metaprogrammed names MUST stay unknown, never become a reference: other Ruby sources'
  `constantize`, `safe_constantize`, `const_get`, `send`, `public_send` and `__send__` words
  count in `ruby-constant-lookup-dynamic`; quoted constant-shaped strings naming C, its last
  segment or `...::last`, and association symbols (`belongs_to`, `has_one`, `has_many`,
  `has_and_belongs_to_many`) camelizing to the last segment (also after dropping a trailing
  `s`) count in `ruby-constant-name-literal`; unreadable sources count in
  `ruby-constant-source-unavailable`. The baseline `dynamic-dispatch-unresolved` row remains
  on every Ruby path, and `reverse-import-rule-unavailable` on every Ruby path the mapper does
  not resolve (amended by `NGI-V0-036`). Ancestor-chain lookup,
  `include`d namespaces, collapsed directories and runtime-configured roots are not modelled.
- `NGI-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1674) The
  historical default path receipt, range receipts and syntax-profile receipts without a Ruby
  changed path MUST keep their exact bytes (a Ruby path now selects the reading by default,
  `NGI-V0-033`). Repeat runs over the same snapshot MUST be
  byte-identical. No source, declaration or configuration text enters the receipt beyond the
  validated declared words, constants and paths.
- `NGI-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1674) A
  syntax-profile receipt with a Ruby changed path MUST carry `ruby_autoload` naming the roots
  applied, the ignored directories, `roots_source` (`default`, `declared` or
  `declaration-invalid`), the declaration path when one is present, and every inflection
  applied as `{word, constant, kind, path, line}` with `kind` `acronym`, `inflect` or
  `declared` (line 0), sorted by word then kind.

- `NGI-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1794) A
  default path impact request naming at least one Ruby path (`NonGoImpactLanguage` `ruby`)
  MUST be answered under the syntax frontier, byte-identical to the `non-go-syntax-v0`
  request for the same paths: the `language_profile`, `unknowns`, `ruby_autoload` block and
  the reverse references of `NGI-V0-016` to `NGI-V0-021` and `NGI-V0-034` to `NGI-V0-036`.
  This amends the opt-in of `NGI-V0-003`/`NGI-V0-020`. A default request with no Ruby path,
  including every Go-only and JS/TS-only request, MUST keep its exact bytes.
- `NGI-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1794) For
  each first-hop referrer R (`NGI-V0-018`) that the mapper itself maps to a constant D, every
  indexed Ruby source with a reference resolving to D or a constant nested in D, and not the
  changed path or a path already reached, MUST be one second-hop result: kind `reference`
  score 475, or kind `test` score 450 for a test path, below every first hop; `via` names R;
  its summary and every evidence reason name D and the changed path's constant; its at most
  10 evidence rows have confidence `low` and authority `syntax`. Each path is reported at
  most once, at its first hop when both hops reach it and through the first referrer in path
  order otherwise. No third hop is followed. Each constant is traversed at most once, however
  many first-hop paths map to it, so the scan stays linear in the indexed references.
- `NGI-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1794) Every
  autoload-convention result (both hops) MUST carry `"advisory": true` and count in
  `coverage.advisory_results`, not `authoritative_results`; a receipt with any such row MUST
  add the coverage uncertainty line `Ruby autoload-convention references are advisory syntax
  hints: a named constant does not establish load order, runtime dispatch or test closure`.
  The learned-path advisory line is added only when a learned-path row is present.
- `NGI-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1794) A Ruby
  changed path the mapper resolves to a constant MUST NOT carry
  `reverse-import-rule-unavailable`: its named-constant rule is the reverse-reference rule.
  An unmapped Ruby path (including one under an invalid declaration) keeps it, and every Ruby
  path keeps `dynamic-dispatch-unresolved`. This amends `NGI-V0-019`. For the same reason a
  mapped Ruby path MUST NOT be counted in the `GPK-V0-027` no-named-resolution-rule coverage
  disclosure, in the default or the budgeted receipt; an unmapped non-test Ruby path still is
  (V1-1814).
- `NGI-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1794) A
  default or expanded committed range admitting a Ruby path MUST attach, for each admitted
  Ruby path, the same advisory rows (both hops) and Ruby unknown rows as path impact, merged
  with the range's other results by the existing deduplication, and the `ruby_autoload` block.
  A range admitting no Ruby path MUST keep its exact bytes.
- `NGI-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1700) The
  Ruby autoload reading's test-path rule (`isRubyTestPath`) MUST classify a `.rb` path named
  `*_spec.rb` or below a `spec/` directory as a test, in addition to the shared rule, so RSpec
  specs and support files rank as tests among the reading's rows and their configuration is
  not read for inflections. The shared rule (`isTestPath`) MUST keep its classification, so every receipt
  for a request naming no Ruby path stays byte-identical, including a feature marker shared
  with an RSpec path.
- `NGI-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1794) The
  path-to-constant mapping of `NGI-V0-016`, `NGI-V0-017` and `NGI-V0-021` MUST live in the
  importable package `internal/rubyautoload` with a documented API: `New(Repository)`,
  `Mapper.Constant`, `Mapper.Path`, `Mapper.Known`, `Mapper.Camelize`, `Mapper.Report`,
  `Mapper.DeclarationInvalid`, `Mapper.ConfigUnresolved` and `DefaultRoots`. The package
  reads no file itself (the caller supplies paths, declaration text and sources), performs no
  I/O, and is deterministic over its inputs.
- `NGI-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1794) The
  Ruby reference index MUST be bounded: an unbalanced source resolves each reference against
  at most 64 of the namespaces it opens, and the index retains at most 1,048,576 reference
  sites in total. Every reference whose resolutions or sites a bound cut, and every reference
  of a source that nests more than 256 blocks (whose deeper namespaces are not tracked;
  TSA-V0-461, V1-1720), MUST be counted in a `ruby-reference-index-bounded` unknown row on the
  mapped changed Ruby paths `NGI-V0-064` selects, never dropped silently; below every bound no row
  is added.
- `NGI-V0-063`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1823) In a path
  or committed-range request naming or admitting a Ruby path, every row MUST classify paths with
  the Ruby reading's test-path rule (`isRubyTestPath`, `NGI-V0-038`), not only the autoload rows:
  an RSpec `*_spec.rb` path or a `.rb` path below `spec/` cited as a feature or scenario marker,
  or reached as a reverse import or same-package reference, is a test (`test-marker` authority,
  the test score). A request naming no Ruby path MUST keep the shared rule and its exact bytes.
  Non-goals: the coverage gap classification in `receipt.go` (`reverseImportProfileGap`), which
  stays on the shared rule; a Ruby test rule for non-Ruby requests.
  Failure modes: a mixed request (a Ruby path beside a Go or JS path) now ranks RSpec markers the
  non-Ruby path reaches as tests; that is the intended reading, since the request is Ruby work.
  Acceptance evidence: `TestNGIV0063RubyRequestClassifiesSpecMarkersAsTests`,
  `TestNGIV0063RubyRangeClassifiesSpecMarkersAsTests`, `TestRubySpecPathNonRubyRequestBaseBytes`.
  Rollback: drop the `withRubyTests` view in `impact` and `compileRangeResults`.
- `NGI-V0-064`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1823) A mapped
  changed Ruby path's `ruby-reference-index-bounded` count MUST be the number of cut references,
  outside the changed path itself, that could name a constant its plan traverses (the changed
  constant and each second-hop constant): a reference whose sites the site bound left unrecorded
  names a candidate equal to or below the constant; a non-rooted reference of a source past the
  nesting bound could name any constant; a reference the namespace bound truncated names the
  constant through a namespace past the bound that the mapper knows. A changed path no cut could
  reach carries no row, so the unknown is no longer a repository-wide count on every mapped path.
  Non-goals: a third hop; counting cuts that hide only an untraversed constant's referrers (the
  cut that hid the hop is already counted).
  Failure modes: the rule is conservative for nesting overflow, so a deeply nested source still
  counts on every plan; a cut inside the changed file is not counted, since its path row is
  already reported.
  Acceptance evidence: `TestRubyAutoloadResolutionBound`, `TestRubyAutoloadSiteBound`,
  `TestAnalyzerBoundsFailClosed`.
  Rollback: emit `rubyAutoload.bounded` in place of `cutReferences`.

### Ruby verification command (V1-1796; GitHub #1130 item 4)

- `NGI-V0-027`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1796) When an
  evidence path ends in `.rb` and the index tracks a repository-root `Gemfile` together with a
  root `.rspec` or any tracked path under the root `spec/` directory, the receipt's
  `verification` member MUST name `bundle exec rspec` once, before the closing gate command, for
  `corvint impact` path receipts and every other receipt built from the shared verification plan
  (`verificationCommands`). Because that is a project-owned signal, the receipt MUST then omit the
  `no language-specific verification command is known for this repository` line (the GPK
  unknown-language rule). Without a tracked root `Gemfile`, a nested `Gemfile` only, or a
  `Gemfile` with neither `.rspec` nor `spec/`, no Ruby command is named and the unknown-language
  line stays. The RSpec signal is not repository-wide: a receipt whose plan does not name
  `bundle exec rspec` (for example a JS path with no test script in an RSpec repository) MUST
  keep the unknown-language line exactly as before. Plans for Go, Python, JS/TS, web and
  Makefile-gated paths, and every receipt without a qualifying Ruby path, MUST keep their exact
  bytes; this amends `NGI-V0-020` only for
  the `verification` member and the unknown-language line of a Ruby path in a qualifying
  repository.
  Non-goals: no per-file spec selection in impact (affected advice owns that, `AFP-V0-066`); no
  read of `.rspec` content, no Bundler or Ruby execution, and no change to the 8-command plan cap
  or the gate command. The former non-goals are replaced: committed-range receipts name the Ruby
  command under `NGI-V0-051`, the RSpec signal also needs a declared rspec gem (`NGI-V0-052`), and
  Minitest commands are named under `NGI-V0-053`.
  Failure modes: an untracked or ignored `Gemfile` does not count, so the unknown-language line
  stays; the former failure mode (a Minitest-only repository tracking `spec/` or `.rspec` told
  `bundle exec rspec`) is closed by `NGI-V0-052`.
  Acceptance evidence: `TestNGIV0027RubyImpactNamesBundleExecRSpec`,
  `TestNGIV0027GoAndWebPlansUnchangedByRubySignals`,
  `TestNGIV0027RubyCommandAppearsOnceAcrossPaths`,
  `TestNGIV0027RubySignalKeepsOtherReceiptsUnknownLine`, and the GitHub #1130 reproduction run in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1796-ruby-verification-command-1796.md`. Rollback: remove the
  `.rb` case in `toolchainCommand`, `rubyVerificationCommand` and `indexTracks`, and the tests.

### Ruby verification in ranges and Gemfile-gated detection (V1-1804, V1-1805; GitHub #1128, #1130)

- `NGI-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1804) A
  committed-range receipt (`corvint impact --base`, default or expanded profile) whose admitted
  changed paths include a `.rb` path MUST name the repository's Ruby command from `NGI-V0-052` or
  `NGI-V0-053` once in its `verification` member, after the Go package commands and before
  `go mod verify` and the closing gate. A range with no admitted Ruby path reads no Ruby manifest
  and MUST keep its exact `verification` bytes, so Go range plans are byte-unchanged; a Ruby range
  whose repository proves no Ruby command also keeps its former plan. This replaces the
  range non-goal of `NGI-V0-027`.
  Non-goals: per-file spec selection in a range. The former non-goal (no unknown-language line in
  a range receipt) is replaced by `NGI-V0-059`.
  Failure modes: the command is repository-wide, not narrowed to the changed path's specs; a
  Ruby range in a repository with no proved suite names no Ruby command, and `NGI-V0-059` names
  that absence.
  Acceptance evidence: `TestNGIV0051RangeImpactNamesRubyCommand`,
  `TestNGIV0051GoRangePlanUnchangedByRubySignals`, `TestNonGoRangeImpactLegacyGoBytes`, and the
  GitHub #1130 fixture run in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1804-ruby-range-verification.md`.
  Rollback: remove the `rangeHasRubyPath` branch in `rangeVerification` and its parameter.
- `NGI-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1805) The
  `bundle exec rspec` signal of `NGI-V0-027` MUST additionally require that the repository-root
  `Gemfile` declares (`gem "rspec"`, `gem "rspec-core"` or `gem "rspec-rails"`, not in a comment)
  or the root `Gemfile.lock` lists (a two-space dependency or four-space spec entry) one of those
  gems. Each manifest is read as the committed blob at the index revision, through the admitted
  source's pinned bytes or one read-only `git cat-file blob` bounded at 256 KiB; an absent,
  oversized or unreadable manifest declares nothing, which can only withhold the command. The
  manifests are read only for a plan that has a Ruby path. A Minitest repository that tracks
  `spec/` or `.rspec` without an rspec gem MUST get no `bundle exec rspec` and keep the
  unknown-language line unless `NGI-V0-053` names its command.
  Non-goals: Bundler resolution, `gemspec` or `eval_gemfile` expansion, groups or platforms.
  Failure modes: an rspec gem declared only by a gemspec and not yet locked, or a Gemfile over
  the bound, withholds the command; a gem requirement that is only a nested lockfile line (six
  spaces) does not count.
  Acceptance evidence: `TestNGIV0052RSpecNeedsDeclaredGem`,
  `TestNGIV0027RubyImpactNamesBundleExecRSpec`, `TestNGIV0051RangeImpactNamesRubyCommand`.
  Rollback: return `rubyVerificationCommand` to the tracked-path test alone.
- `NGI-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1805) When
  `NGI-V0-052` names no RSpec command and the root `Gemfile` is tracked, a Ruby plan MUST name
  `bin/rails test` when the manifests declare `rails` or `railties`, `bin/rails` is tracked and a
  path under root `test/` is tracked; otherwise `bundle exec rake test` when they declare
  `minitest`, a path under root `test/` is tracked and the committed root `Rakefile` (bounded as
  in `NGI-V0-052`) names `Rake::TestTask` or `Minitest::TestTask`; otherwise no Ruby command and
  the unknown-language line stays. A plan naming either command omits the unknown-language line
  exactly as `NGI-V0-027` does for RSpec.
  Non-goals: Test::Unit, custom Rake task names, per-file test selection, running any command.
  Failure modes: a Rakefile that builds its test task indirectly is not recognised and the line
  stays; a repository with both suites gets only `bundle exec rspec`.
  Acceptance evidence: `TestNGIV0053MinitestCommands`. Rollback: remove the two Minitest cases
  in `rubyVerificationCommand` and `rakefileDeclaresTestTask`.
- `NGI-V0-059`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1827) A
  committed-range receipt whose admitted changed paths include a `.rb` path and whose
  `verification` member names no language-specific command (only the closing gate
  `git diff --check`) MUST carry the unknown-language line path impact reports for the same
  repository ("no language-specific verification command is known for this repository") in
  `coverage.uncertainty`, decided by the same rule path impact uses, after the authority lines and
  before the Ruby advisory line. A range with no admitted Ruby path never evaluates the rule, so Go
  range receipts keep their exact bytes; a range whose plan names a Go or Ruby command omits the
  line, as path impact does.
  Non-goals: the line for other non-Go languages in a range; changing the range `verification`
  member or reading any manifest beyond `NGI-V0-052`/`NGI-V0-053`.
  Failure modes: before `NGI-V0-062` a Ruby repository whose Makefile declares a gate target got
  the line in a range (whose gate was the profile gate) but not in path impact (whose fallback gate
  reads the Makefile); both plans now close with the same gate, so they agree on the line.
  Acceptance evidence: `TestNGIV0059RubyRangeReportsUnknownVerification`,
  `TestNGIV0059RangeWithoutRubyPathKeepsUncertainty`, `TestNonGoRangeImpactLegacyGoBytes`.
  Rollback: remove the `rangeHasRubyPath` and `lacksLanguageVerification` branch in `RangeImpact`
  and the tests.

- `NGI-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2071) When the
  committed root `AGENTS.md` declares verify commands, path impact and committed-range impact MUST
  close their `verification` member with the declared mandatory commands, in declaration order and
  deduplicated, in place of the inferred closing gate (the Makefile or Taskfile gate target, else
  the profile gate, else `git diff --check`). The declaration is the one `corvint affected` reads
  (`AFP-V0-009`, `AFP-V0-196`) through the shared `internal/projectprofile` parser: the commands of
  sh/bash/console fences under a heading whose text is exactly "Verify" and the code spans an
  inline `**Verify:**` line names, minus launch or background commands and commands a Verify scope
  sentence makes conditional. At most 8 declared commands are used, the first 256 KiB of the file
  are read, and the language commands before them are trimmed so the member keeps its 8-entry
  ceiling. A repository without the file, or whose file declares nothing mandatory, keeps its exact
  receipt bytes.
  Non-goals: running or validating a declared command; reading instruction files other than the
  root `AGENTS.md` or headings other than "Verify"; changing the language commands named before the
  gate.
  Failure modes: a declaration that names a command needing a service the agent lacks is still
  named, because project authority outranks inference (AGENTS.md invariant 3); a declaration
  spread across non-shell fences or `## Build / gate` headings is not read and the inferred gate
  stays.
  Acceptance evidence: `TestNGIV0060DeclaredVerifyCommandClosesMakefileLessPlan`,
  `TestNGIV0060MandatoryVerifyCommands`. Rollback: make `closingGates` return the inferred gate only.
- `NGI-V0-061`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2071) A path or
  range receipt whose plan uses `NGI-V0-060` declared commands and whose `verification` member is
  not empty MUST carry a `verification_authority` object with `source` (`"AGENTS.md"`),
  `mandatory` (the declared commands in the member) and `inferred` (every other command in the
  member, which Corvint inferred from the repository's shape), so an agent can tell the commands
  the project requires from the ones Corvint suggests. A receipt without a declaration carries no
  such member.
  Non-goals: labelling commands in receipts without a declaration; ranking or dropping inferred
  commands.
  Failure modes: a budget-trimmed receipt drops the member with its empty `verification`.
  Acceptance evidence: `TestNGIV0060DeclaredVerifyCommandClosesMakefileLessPlan`,
  `TestNGIV0061InferredCommandBesideDeclaredGate`. Rollback: remove `setVerification`'s authority
  branch.
- `NGI-V0-062`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1848) Path impact
  and committed-range impact MUST choose their closing gate through one rule (`closingGates`): the
  `NGI-V0-060` declared commands, else the Makefile or Taskfile gate target, else the profile gate,
  else `git diff --check`. A range over a Ruby repository with a Makefile `test` target therefore
  closes with `make test`, as path impact does, not with `git diff --check`.
  Non-goals: aligning the language commands named before the gate, which `NGI-V0-051` governs.
  Failure modes: a committed-range receipt over a repository with a Makefile gate gains that gate
  where it used to name only the profile gate; Go range receipts in repositories without one keep
  their exact bytes.
  Acceptance evidence: `TestNGIV0062RangeAndPathChooseOneGate`, `TestNonGoRangeImpactLegacyGoBytes`.
  Rollback: restore `rangeVerification`'s profile-only gate.
- `NGI-V0-780`: (accepted under the owner's standing auto-accept (2026-10-31); V1-3498) Impact's
  `verification` member is governed by two rules: a repository-declared gate (`NGI-V0-060`) closes
  it, and the language command each changed path implies precedes it. For a JavaScript or
  TypeScript path whose root `package.json` declares a non-empty `test` script, that command MUST
  run the script under the repository's Node package manager: `npm test`, `pnpm test`,
  `yarn test` or `bun run test`. The manager is the root `package.json` `packageManager` member's
  name when it is `npm`, `pnpm`, `yarn` or `bun`; else the one manager whose tracked root file
  signals it (`package-lock.json` or `npm-shrinkwrap.json` npm, `pnpm-lock.yaml` or
  `pnpm-workspace.yaml` pnpm, `yarn.lock` yarn, `bun.lock` or `bun.lockb` bun); else npm, the
  Node default. When tracked files signal more than one manager and no `packageManager` member
  settles it, no Node command is named, since which one runs the script is not known. A
  repository with no such file and no member keeps its exact receipt bytes. Committed-range impact
  names no Node command and is unchanged.
  Non-goals: reading nested workspace packages' manifests or lockfiles; validating that the
  manager is installed; running the script.
  Failure modes (V1-3498): a pnpm workspace (`packageManager: pnpm@...`, `pnpm-lock.yaml`) was told
  to run `npm test`. A declared `pnpm check` that impact did not read, because the root `AGENTS.md`
  is a symlink, is V1-1272, not this rule.
  Acceptance evidence: `TestNGIV0780NodeTestCommandFollowsPackageManager`,
  `TestNGIV0060DeclaredVerifyCommandClosesMakefileLessPlan`.
  Rollback: make `nodeTestCommand` return `npm test`.

### Whole-tree import export (V1-3090; GitHub #1986)

The owner requested a pinned file import graph for coverage and ownership consumers on
2026-10-28. This additive experimental profile leaves frozen impact receipts unchanged.
It enumerates indexed syntax relations, not every runtime dependency or every tracked file.

- `NGI-V0-301`: `corvint graph imports --revision FULL_COMMIT [--kinds KINDS]
  [--format jsonl]` MUST emit deterministic `graph-imports/1` JSONL, without impact's
  path-count or result-count cap. Every record carries `schema`, `revision` and `type`.
  The first `scope` record binds the Git tree and names `indexed-syntax-only`; an `edge`
  carries repository-relative `from`, `to`, `kind` and `evidence` with `path`, positive
  `line`, `blob_hash`, `specifier` and `authority` (`syntax`, or `convention` for
  `NGI-V0-307`). Order is target path, importer path, specifier, then kind in the
  `NGI-V0-308` order; uncertainties follow that target's edges, then sorted index omissions.
  The requested commit may differ from HEAD; later commits and dirty working files MUST
  NOT alter its output. The build MUST read only Git objects, never HEAD or a working
  file, so an unborn or detached HEAD and a dirty checkout neither block nor alter it.
  Reading MUST create no persistent index or repository state.
- `NGI-V0-302`: Every Ruby/JS/TS path MUST retain the structured syntax unknowns that
  impact produces, including declared registry and Ruby autoload detector unknowns.
  Unknowns are separate `uncertainty` records with an `unknown` object. Ruby constants
  MUST NOT become `import` edges; `ruby-autoload-not-import` names that limit, and only
  `NGI-V0-307` may add a separately kinded convention edge. JS/TS MUST disclose
  `framework-resources-unresolved` (selectors and every framework resource other than
  `NGI-V0-306` literals) and `string-keyed-di-unresolved` even when no detector observes a site.
  Every other indexed path (the index admits paths by suffix) without an import rule MUST
  disclose `reverse-import-rule-unavailable`, naming its suffix as `language`. Index exclusions and unparsed facts MUST remain explicit (`index-excluded` and the
  extractor's refusal code). A missing Go module is `go-module-unavailable`; a resolved
  pair with no provable statement anchor is `import-evidence-unavailable`, never an edge.
  No empty graph or absence of detector hits proves dependency closure.
- `NGI-V0-303`: Import resolution and evidence sites MUST agree with impact on the same
  indexed fixture. Go package imports expand to the indexed files of that package;
  Python uses the existing module candidates; JS/TS uses relative/profile/tsconfig rules.
  Import kinds are `import` and `type-import`; `NGI-V0-308` closes the kind filter. The graph includes test files too,
  without impact's ranking, marker expansion, advisory references or result budget.
- `NGI-V0-304`: A missing, abbreviated, symbolic, uppercase or non-commit revision MUST
  refuse `graph-invalid-revision`; an unavailable full ID MUST refuse
  `graph-unknown-revision`. Invalid kinds refuse `graph-invalid-kinds`; malformed CLI
  arguments, duplicates and non-JSONL formats refuse `graph-invalid-arguments` before
  writing output. Cancellation and writer failures MUST stop traversal and return failure.
  Consumers MUST check the terminal exit status; JSONL streaming is not atomic publication.
- `NGI-V0-305`: The explicit MCP `--tool-profile import-graph` profile MUST add only
  `corvint.graph.imports` to the default tools. The new tool MUST accept a
  full `revision`, optional `kinds` string and nonnegative safe-integer `offset` (default
  zero). Default V0, task-review, task-review-lsp and flows tool enumeration stays unchanged. A bounded response carries
  `schema: graph-imports/1`, `revision`, `format: jsonl`, `jsonl`, `offset`, `done` and,
  unless done, `nextOffset`. Concatenating pages with identical revision/kinds MUST
  reproduce CLI bytes with no total-result cap. Each page is at most 128 records and
  48 KiB raw JSONL; existing MCP frame limits remain enforced, never silently truncated.
  Offsets beyond the stream refuse; an individual record that cannot fit abstains with
  `OUTPUT_BUDGET_EXCEEDED`. MCP arguments are closed and duplicate keys refuse. Because
  the build never observes the checkout, the result's `repository.worktreeState` is
  `NOT_OBSERVED` with `dirtyPathCount` zero, never `CLEAN`.
- `NGI-V0-306`: A JS/TS source's `@Component({...})` decorator argument (in code, not a
  comment or string) MUST yield an `angular-template` edge for a `templateUrl` and an
  `angular-style` edge for a `styleUrl` or each `styleUrls` array element whose value is one
  quoted or backtick string literal with no escape or `${}` substitution. The member name
  may be an identifier or a quoted string; a template literal, including any nested
  `${}` substitution, is string data and yields nothing. The literal resolves
  relative to the component file's directory and MUST name a path indexed at the pinned
  commit; the edge's `specifier` is the literal, its `evidence` adds `property`, and its
  `line` is the literal's line. A value that is not such a literal is
  `angular-resource-nonliteral`; an empty, absolute, backslashed, repository-escaping or
  unindexed literal is
  `angular-resource-unresolved`, carrying the joined repository `candidate` when one exists.
  Both rows name `path`, `language`, `line` and `property` and are never edges.
- `NGI-V0-307`: Each Ruby constant reference the `NGI-V0-016` scanner reads in a readable
  indexed Ruby file (tests included) MUST resolve by `NGI-V0-040` lexical lookup to the
  indexed files the autoload mapper (default roots `app/*/`, `app/*/concerns`, `lib`, or the
  declaration) maps those constants to. A complete, single-constant resolution with exactly
  one candidate file other than the referencing file is a `ruby-constant` edge whose
  `specifier` is the constant, with `authority: convention` and `convention: zeitwerk`, at
  the first referencing line per importer and target. Zero candidates is
  `ruby-constant-unresolved`; several candidates, several resolved constants, an unbalanced
  file's resolution or a nesting-bound overflow is `ruby-constant-ambiguous`. Each row names
  `path`, `language`, first `line`, `count` of references, sorted `constants` and sorted
  `candidates`. The file's own constant, its locally defined unmapped constants and implicit
  namespaces (a mapped prefix with no file) yield neither. An invalid declaration maps no
  file and yields no constant edge.
- `NGI-V0-308`: `--kinds` and MCP `kinds` MUST be a unique comma-separated subset of the
  closed set `import`, `type-import`, `angular-template`, `angular-style`, `ruby-constant`
  (the tie order of `NGI-V0-301`); anything else refuses `graph-invalid-kinds`. The filter
  removes only edges and MUST NOT suppress or reorder any uncertainty record; the same
  inputs yield the same bytes.

Non-goals: no Angular evaluation (constants, imported strings, concatenation, `${}`),
no `@Component` reached through an alias, re-export or call, no selector, `loadChildren`,
route or DI-token resolution, no Ruby `require`, `include`, ancestor, Rails configuration
or collapsed-directory resolution, and no runtime ownership claim from a convention edge.
Failure modes: a misparsed decorator yields an unknown or no edge, never an edge to an
unindexed file; a Ruby constant redefined outside the convention may yield a convention
edge, which `authority: convention` and `convention: zeitwerk` label as not syntax-proved.

Rollback removes the graph command and opt-in MCP tool; no persistent format migration,
legacy impact change, new language resolver, dynamic import inference or ownership gate
is authorized by this profile. Roll back `NGI-V0-306` to `NGI-V0-308` alone by removing
`import_graph_edges.go` and its walk wiring and restoring the two-kind filter; `import`
and `type-import` bytes are unchanged by them. Local fixture evidence does not qualify
runtime completeness.

### Controller request specs by convention (NGI-V0-761; V1-1851)

- `NGI-V0-761`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1851)
  The Ruby autoload reading of `impact` SHALL list, for a changed `app/controllers/<name>_controller.rb`
  (namespaced `<name>` included, never `application`), the tracked `spec/requests/<name>_spec.rb` as
  an advisory `test` row scored 440, below every constant reference, with one `syntax` evidence row of
  `low` confidence whose reason names the convention and that route dispatch is not read. A
  controller with no such tracked spec, or one a constant reference already reached, adds nothing:
  the reading abstains rather than guess a route, and `config/routes.rb` is not read. The
  `request-spec` and `controller-spec` rows of `conformance/ruby-profile-v0/rows.json` now list
  `spec/requests/users_spec.rb`. Non-goals: no route-file reading, no `test/integration` or
  `test/controllers` mapping. Rollback: remove `requestSpec`, its call, and the two row entries.

### Swift, Kotlin and C# module reverse references (NGI-V0-980 to NGI-V0-987; V1-3724)

`GPK-V0-027` names no reverse-import rule for `.swift`, `.kt` or `.cs`, and its frozen oracle is
not widened: `ImpactRuleNamed` stays false for these suffixes. Path `impact` instead reads each
language's module membership from pinned index text, under every language profile. The index does
not admit `.csproj`, `.sln`, Groovy `build.gradle` or `project.pbxproj`, so only SwiftPM manifests
supply a build-unit graph; the Kotlin and C# rules read package and namespace syntax.

- `NGI-V0-980`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3724) For a changed non-test `.swift` path that a SwiftPM
  target claims, as the affected Swift model (`internal/liveverify/affected/swift`) parses the
  indexed `Package.swift` manifests, `impact` SHALL list each indexed `.swift` file outside that
  target whose code (comments and strings excluded) imports the target's module as a
  `reverse-import` row (score 700, 650 for a test path) with one `syntax` evidence row of `high`
  confidence naming the module. A module name two targets share is reported for each and disclosed.
- `NGI-V0-981`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3724) For a changed non-test `.kt` path in package `P`, an
  indexed `.kt` file in another package whose explicit `import P.N` (or `P.N.x`) names a
  declaration `N` the changed path declares SHALL be a `high` `reverse-import` row; a wildcard
  `import P.*` SHALL be a `medium` row (score 650) only when the importer's code also names a
  declaration of at least four runes. A path in the default package has no importer. `.kts` carries
  no reading and keeps the profile gap.
- `NGI-V0-982`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3724) For a changed non-test `.cs` path, a `using static` or
  alias directive (`global` and `global::` forms included) naming a type `ns.T` the path declares in
  its namespace `ns` SHALL be a `high` `reverse-import` row; a `using ns;` directive SHALL be a
  `medium` row only when the file's code also names a declaration. A `global using ns;` adds a
  coverage line naming its unenumerated project-wide reach.
- `NGI-V0-983`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3724) An indexed file in the changed path's SwiftPM target,
  Kotlin package, or a C# namespace enclosing the file's own (the global namespace encloses
  every one) SHALL be a `reference` row (score 775; `test`, 650, for a test path) only when its
  code, with import, package, using and namespace directives blanked, names a declaration of at
  least four runes; the evidence lists the matched lines.
- `NGI-V0-984`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3724) A path a reading resolves (a SwiftPM target member, or
  any loaded `.kt` or `.cs` path) SHALL withhold the no-named-resolution-rule profile gap; an
  unclaimed `.swift` file and every `.kts` path SHALL keep it and report no `reverse-import` row.
- `NGI-V0-985`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3724) Every request naming a non-test path of a language SHALL
  carry that language's fixed coverage line naming the unresolved limits: for Swift, Xcode
  project membership, `@_exported` re-exports, conditional compilation and dependents beyond one
  import; for Kotlin, Gradle module visibility, Java importers, fully qualified references,
  extension calls and reflection; for C#, project references, global using reach, extension
  methods, fully qualified references and reflection. A row is syntax evidence, never closure.
- `NGI-V0-986`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3724) `corvint impact --capabilities` SHALL add the sorted
  `suffixes.withModuleReferenceRule` list (`.cs`, `.kt`, `.swift`), each also in
  `admittedWithoutRule`, and `corvint impact --help` SHALL name the three rules and mark `.kts` and
  `.csproj` NOT_SUPPORTED.
- `NGI-V0-987`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-3724) `swift.ImportedModules` SHALL return each module a Swift
  source imports, with its line, from code only (attributed and kind-qualified imports included),
  and `swift.TargetModule` SHALL return the module of a `swift:spm:` unit ID.

## Trust and failure boundaries

Core performs bounded local immutable reads; it adds no network, process provider,
state write, execution, daemon or authority. Existing unsupported statuses/modes,
unsafe paths, binary content, excluded source and snapshot races remain refused.
A syntactic relation has syntax authority; only independently accepted intent has
project authority. Structured unknowns contain paths, counts and closed codes,
not excerpts or source-controlled diagnostic text. An output limit must never
remove the semantic frontier. Existing non-Go outside-profile omissions remain
for languages not admitted by this new range capability.

## Closed syntax unknown codes

| Code | Meaning |
|---|---|
| `dynamic-dispatch-unresolved` | Baseline: syntax does not establish runtime dispatch or test closure. |
| `reverse-import-rule-unavailable` | Ruby reverse-import semantics are unavailable; omitted for a Ruby path the autoload mapper resolves (`NGI-V0-036`). |
| `source-analysis-unavailable` | The requested source is missing, excluded or unavailable to syntax analysis. |
| `observed-reflective-dispatch` | A lexical reflection/metaprogramming candidate was observed. |
| `observed-computed-dispatch` | A computed-call candidate was observed. |
| `autoload-unresolved` | Ruby autoload was observed without resolved runtime semantics. |
| `dynamic-load-unresolved` | A load argument is dynamic, interpolated or concatenated. |
| `bare-import-unresolved` | Bare JS/TS import specifiers resolve to no repository file or declared package, so reverse importers are not closed; `count` is the repository-wide specifier count (`GPK-V0-080`, proposed). |
| `registry-declaration-invalid` | A manifest `registries` member does not decode or validate; `count` is the number of such manifests (`NGI-V0-007`). |
| `registry-key-unresolved` | Entries of the declared registry `registry` have no literal key or imported-binding value; selection is widened (`NGI-V0-008`, `NGI-V0-010`) on every JS/TS changed path (`NGI-V0-012`); a lazy entry counts unless `NGI-V0-013` resolves it. |
| `registry-export-unresolved` | The declared export of `registry` is not a readable object literal, so none of its keys are known; selection is widened (`NGI-V0-008`, `NGI-V0-010`) on every JS/TS changed path only. |
| `registry-written-after-declaration` | `count` writes to the declared registry `registry` follow its declaration, so its keys are not closed; every caller is selected on every JS/TS changed path (`NGI-V0-066`). |
| `registry-constructor-arguments-open` | `count` eager `new X(args)` entries of `registry` pass arguments that are not closed literals; callers passing those keys are selected on every JS/TS changed path (`NGI-V0-067`). |
| `accessor-key-nonliteral` | `count` calls of `accessor` pass a key that is not a literal; selection is widened to every caller (`NGI-V0-010`). |
| `registry-caller-unavailable` | Sources naming a declared accessor could not be read or lexed completely (`NGI-V0-010`). |
| `ruby-constant-unmapped` | The changed Ruby path is below no autoload root, below an ignored directory, or its path does not camelize to a constant (`NGI-V0-016`). |
| `ruby-autoload-declaration-invalid` | `.corvint/ruby-autoload.json` is present but invalid or unavailable, so no path is mapped (`NGI-V0-017`). |
| `ruby-autoload-config-unresolved` | `count` autoload root, collapse or inflection configuration sites were not literal or not modelled (`NGI-V0-017`). |
| `ruby-constant-lookup-dynamic` | `count` dynamic constant lookups or dynamic sends in other Ruby sources may name the constant (`NGI-V0-019`). |
| `ruby-constant-name-literal` | `count` strings or association symbols in other Ruby sources may name the constant without a resolvable reference (`NGI-V0-019`). |
| `ruby-constant-source-unavailable` | `count` other Ruby sources could not be read for constant references (`NGI-V0-019`). |
| `ruby-reference-index-bounded` | `count` Ruby references that could name a constant this path's plan traverses were indexed incompletely because the resolution, retained-site or nesting bound cut them, so the named-constant rows may be incomplete (`NGI-V0-040`, `NGI-V0-064`). |

These codes are uncertainty, never execution evidence. The versioned receipt
names `language_profile: non-go-syntax-v0`; line 0 denotes a baseline frontier.
The registry rows' `registry` and `accessor` fields carry only a declared path
and a validated declared identifier, never source text.

## Acceptance and traceability

| Requirement | Implementation | Executable evidence |
|---|---|---|
| `NGI-V0-001` | shared suffix helper; MCP descriptor/validator | `TestNonGoImpactMCPAdmission` |
| `NGI-V0-002` | shared range compiler language classification | `TestNonGoRangeImpact` |
| `NGI-V0-003` | shared lexical mask and structured frontier; `bare-import-unresolved` in `nonGoImpactUnknowns` | `TestNonGoImpactDynamicUnknowns`; `TestImpactSyntaxReportsBareImportUnresolved` and `TestImpactKeepsFrozenBareReadingWithoutManifest` (`internal/contextindex/webresolve_test.go`) |
| `NGI-V0-004` | existing range bindings/reducer | `TestNonGoRangeImpactLegacyGoBytes`, `TestNonGoRangeImpactGoModuleValidationPriority`, `TestNonGoImpactCLIProfile` |
| `NGI-V0-005` | impactHelp and MCP description | `TestNonGoImpactMCPAdmission`; actual CLI help check in the build-log |
| `NGI-V0-006` | committed fixture conformance | `TestNonGoRangeImpactMixed`, `TestNonGoRangeImpactRefusals`; tests above and existing range/impact regressions |
| `NGI-V0-007` | `ValidateRegistryDeclarations`, `declaredRegistries` (`internal/contextindex/registry_dialect.go`); `decodeManifest` (`internal/appmap/manifest.go`) | `TestValidateRegistryDeclarations`, `TestRegistryDialectDeclarationFailures`, `TestManifestRegistriesMember` |
| `NGI-V0-008` | `parseRegistry`, `registryImports`, `registryEntryOf`; template body in `readTemplate` (`webimports.go`) | `TestRegistryDialectSelectsTestsPassingTheKey`, `TestRegistryDialectUnresolvedRegistryWidens`, `TestRegistryDialectReexportedRegistryIsUnresolved` |
| `NGI-V0-009` | `registryCallSites`, `registryDialect.impact`; wiring in `impact` | `TestRegistryDialectSelectsTestsPassingTheKey`, `TestRegistryDialectAliasedAccessorImport`, `TestRegistryDialectKeyCollisionAcrossRegistries` |
| `NGI-V0-010` | `registryDialect.impact`; `extra` rows in `nonGoImpactUnknowns` | `TestRegistryDialectNonLiteralKeyWidens`, `TestRegistryDialectChangedRegistrySelectsEveryCaller`, `TestRegistryDialectUnresolvedRegistryWidens`, `TestRegistryDialectReexportedRegistryIsUnresolved`, `TestRegistryDialectExportUnresolvedIsWebOnly` |
| `NGI-V0-011` | `internal/contextindex/registry_dialect_test.go`, `internal/appmap/registries_test.go` | tests above; real CLI output in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-14-v1-1365-string-keyed-registry.md` |
| `NGI-V0-012` | `registryDialect.impact` (`openEntry`) | `TestRegistryDialectLazyRequireNeverFailsOpen`, `TestRegistryDialectUnresolvedEntryAppliesToEveryPath`; real CLI output in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-registry-dialect-lazy-const.md` |
| `NGI-V0-013` | `registryLazyBinding`, `registryThenCallback`, `registryAssertionTail`, `registryRequireFree`, `registryAmbientDeclaration`, `registryLazyTarget`, `parseRegistry`; `bound` in `registryDialect.impact` | `TestRegistryDialectLazyEntriesResolve`, `TestRegistryDialectLazyEntriesStayClosed`, `TestRegistryDialectAmbientRequireIsLoader` |
| `NGI-V0-014` | `registryObject`, `registryExportedOtherwise` | `TestRegistryDialectNonExportedRegistry` |
| `NGI-V0-015` | `registryCallSites`, `registryConstants`, `registryMember` | `TestRegistryDialectConstAccessorKey`, `TestRegistryDialectConstAccessorKeyStaysNonLiteral`, `TestRegistryDialectNonLiteralKeyWidens`; real CLI output in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-registry-dialect-lazy-const.md` |
| `NGI-V0-065` | `registryCallTokens`, `registryLexTokens` (`registry_dialect.go`); `substitutions` in `skipSubstitution` (`webimports.go`) | `TestRegistryDialectAccessorInsideSubstitution`, `TestSelectSetupRegistryKeyInSubstitution` |
| `NGI-V0-066` | `registryWrites`, `registryWriteCount`, `registryWriteAt`, `registryAssignmentAt`; `written` in `registryDialect.impact` and `setupSelector.lookup` | `TestRegistryDialectWrittenAfterDeclarationWidens`, `TestSelectSetupRegistryKeyInSubstitution` |
| `NGI-V0-067` | `registryValueBinding`, `registryArgumentsOpen`; `argumentKeys` in `registryDialect.impact`; `arguments` in `setupSelector.lookup` | `TestRegistryDialectConstructorArgumentsOpen`, `TestSelectSetupRegistryKeyInSubstitution` |
| `NGI-V0-068` | `PathImpactSyntax`, `SyntaxProfileSelected` (`internal/contextindex/eval_query.go`); `standaloneImpactContext`, `impactBudget` (`cmd/corvint/main.go`); `callImpact` (`internal/mcp/bridge/bridge.go`) | `TestImpactCLIMatchesMCPOnRuby` (`cmd/corvint/impact_mcp_parity_test.go`) |
| `NGI-V0-069` | `parseImpactArgumentsForPlatform` (`cmd/corvint/main.go`) | `TestImpactLanguageProfileAnyPosition` (`cmd/corvint/impact_mcp_parity_test.go`), `TestNonGoImpactCLIProfileAdmission` |
| `NGI-V0-016` | `Mapper.Constant`, `Mapper.Camelize` (`internal/rubyautoload/rubyautoload.go`) | `TestRubyAutoloadConstantMapping` (`internal/rubyautoload`), `TestRubyAutoloadNamedConstantReverseReference` |
| `NGI-V0-017` | `loadRubyAutoload`, `rubyAutoloadDeclaration` (`internal/contextindex/ruby_autoload.go`); `New`, `DefaultRoots`, declaration decoding and `readConfiguredInflections` (`internal/rubyautoload`) | `TestRubyAutoloadConstantMapping`, `TestRubyAutoloadMapperDeclaration` (`internal/rubyautoload`), `TestRubyAutoloadInflections`, `TestRubyAutoloadDeclaration` |
| `NGI-V0-018` | `scanRubyConstantSource`, `rubyDefinedConstant`, `rubyAssignsConstant`, `resolutions`, `rubyAutoload.impact`; wiring in `impact` | `TestRubyAutoloadNamedConstantReverseReference`, `TestRubyAutoloadNamespacedConstant`, `TestRubyAutoloadMetaprogrammingStaysUnknown`, `TestRubyAutoloadNestingBound`, `TestRubyAutoloadQualifiedClassInNamespace`, `TestRubyAutoloadLocalConstantShadows`, `TestRubyAutoloadQualifiedAndOrAssignmentsDefine` |
| `NGI-V0-019` | `rubyAutoload.impact` unknown rows | `TestRubyAutoloadMetaprogrammingStaysUnknown`, `TestRubyAutoloadNamedConstantReverseReference` |
| `NGI-V0-020` | syntax-frontier guard in `impact`; `attachNonGoImpactUnknowns` | `TestRubyAutoloadDefaultOn`, `TestRubyAutoloadNamedConstantReverseReference`; `TestNonGoImpactCLIProfile`, `TestNonGoRangeImpactLegacyGoBytes` |
| `NGI-V0-021` | `rubyAutoload.report` over `Mapper.Report`, `attachNonGoImpactUnknowns` | `TestRubyAutoloadInflections`, `TestRubyAutoloadDeclaration`; real CLI output in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1674-ruby-autoload-reverse-refs.md` |
| `NGI-V0-027` | `toolchainCommand`, `indexTracks` (`internal/contextindex/receipt.go`); `rubyVerificationCommand` (`internal/contextindex/ruby_verification.go`) | `TestNGIV0027RubyImpactNamesBundleExecRSpec`, `TestNGIV0027GoAndWebPlansUnchangedByRubySignals`, `TestNGIV0027RubyCommandAppearsOnceAcrossPaths`, `TestNGIV0027RubySignalKeepsOtherReceiptsUnknownLine`; real CLI output in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1796-ruby-verification-command-1796.md` |
| `NGI-V0-051` | `rangeVerification`, `rangeHasRubyPath` (`internal/contextindex/range_impact.go`, `ruby_verification.go`) | `TestNGIV0051RangeImpactNamesRubyCommand`, `TestNGIV0051GoRangePlanUnchangedByRubySignals`, `TestNonGoRangeImpactLegacyGoBytes`; real CLI output in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1804-ruby-range-verification.md` |
| `NGI-V0-052` | `rubyVerificationCommand`, `declaredGems`, `committedRootText` (`internal/contextindex/ruby_verification.go`) | `TestNGIV0052RSpecNeedsDeclaredGem`, `TestNGIV0027RubyImpactNamesBundleExecRSpec` |
| `NGI-V0-053` | `rubyVerificationCommand`, `rakefileDeclaresTestTask` | `TestNGIV0053MinitestCommands` |
| `NGI-V0-059` | `RangeImpact` unknown-language branch (`internal/contextindex/range_impact.go`), `lacksLanguageVerification` | `TestNGIV0059RubyRangeReportsUnknownVerification`, `TestNGIV0059RangeWithoutRubyPathKeepsUncertainty` |
| `NGI-V0-060` | `closingGates`, `declaredVerification` (`internal/contextindex/receipt.go`); `MandatoryVerifyCommands` (`internal/projectprofile/verify.go`) | `TestNGIV0060DeclaredVerifyCommandClosesMakefileLessPlan`, `TestNGIV0060MandatoryVerifyCommands` |
| `NGI-V0-061` | `setVerification` (`internal/contextindex/receipt.go`) | `TestNGIV0060DeclaredVerifyCommandClosesMakefileLessPlan`, `TestNGIV0061InferredCommandBesideDeclaredGate` |
| `NGI-V0-780` | `nodeTestCommand`, `toolchainCommand` (`internal/contextindex/receipt.go`); `NodePackageManager`, `NodeTestCommand` (`internal/projectprofile/nodepm.go`) | `TestNGIV0780NodeTestCommandFollowsPackageManager` |
| `NGI-V0-062` | `rangeVerification` (`internal/contextindex/range_impact.go`), `closingGates` | `TestNGIV0062RangeAndPathChooseOneGate`, `TestNonGoRangeImpactLegacyGoBytes` |
| `NGI-V0-033` | `rubyImpactRequested` and the syntax-frontier selection in `impact` | `TestRubyAutoloadDefaultOn`, `TestRubyAutoloadIssue1130DefaultTransitive`, `TestNonGoImpactCLIProfile`; Go/JS goldens `conformance/cli-parity-v0`, `TestNonGoRangeImpactLegacyGoBytes` |
| `NGI-V0-034` | `rubyAutoload.impact` second hop (`traversed`), `referrers`, `result` | `TestRubyAutoloadIssue1130DefaultTransitive`, `TestRubyAutoloadSecondHopOnce`, `TestRubyAutoloadSecondHopConstantOnce` |
| `NGI-V0-035` | `result` (`advisory`); `setCoverage` (`internal/contextindex/receipt.go`); range coverage in `compileRangeImpact` | `TestRubyAutoloadIssue1130DefaultTransitive`, `TestRubyAutoloadRangeImpact`, `TestRubyAutoloadNamedConstantReverseReference` |
| `NGI-V0-036` | `rubyAutoload.mapped`; `nonGoImpactUnknowns`; `reverseImportProfileGap` | `TestRubyAutoloadIssue1130DefaultTransitive`, `TestRubyAutoloadNamedConstantReverseReference`, `TestRubyAutoloadMappedPathHasNoProfileGap` |
| `NGI-V0-037` | `rangeRubyAutoload`, wiring in `compileRangeImpact` | `TestRubyAutoloadRangeImpact`; `TestNonGoRangeImpactLegacyGoBytes` |
| `NGI-V0-038` | `isRubyTestPath` (`internal/contextindex/ruby_autoload.go`); `isTestPath` unchanged | `TestIsTestPathRSpec`, `TestRubyAutoloadIssue1130DefaultTransitive`, `TestRubySpecPathNonRubyRequestBaseBytes` |
| `NGI-V0-039` | `internal/rubyautoload` package | `TestRubyAutoloadConstantMapping`, `TestRubyAutoloadMapperDeclaration`; real CLI output in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1794-ruby-constant-impact.md` |
| `NGI-V0-040` | `resolutions`, `indexSites`, `loadRubyAutoload`; `ruby-reference-index-bounded` in `rubyAutoload.impact` | `TestRubyAutoloadResolutionBound`, `TestRubyAutoloadSiteBound`, `TestRubyNestingOverflowIsBounded` |
| `NGI-V0-063` | `withRubyTests`, `isTest` (`internal/contextindex/ruby_autoload.go`); the view in `impact` and `compileRangeResults`; `rangeMarkerResult` | `TestNGIV0063RubyRequestClassifiesSpecMarkersAsTests`, `TestNGIV0063RubyRangeClassifiesSpecMarkersAsTests`, `TestRubySpecPathNonRubyRequestBaseBytes` |
| `NGI-V0-064` | `rubyCut`, `cutReferences`, `cutNames` (`internal/contextindex/ruby_autoload.go`) | `TestRubyAutoloadResolutionBound`, `TestRubyAutoloadSiteBound`, `TestAnalyzerBoundsFailClosed` |

Names in this matrix identify evidence rather than implying passing execution.
Observed execution and independent findings are retained in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-09-30-non-go-impact.md`. No broad formal qualification or promotion is claimed.

| Requirement | Import graph implementation | Acceptance witness |
|---|---|---|
| `NGI-V0-301` | `BuildImportGraph`, `WalkImportGraph` (`internal/contextindex/import_graph.go`) | `TestImportGraphHistoricalUncapped`, `TestImportGraphImpactParity`, `TestImportGraphRevisionObjectOnly` |
| `NGI-V0-302` | `WalkImportGraph` (`internal/contextindex/import_graph.go`) | `TestImportGraphUncertainty`, `TestImportGraphRuleUnavailableCoversAdmittedSources` |
| `NGI-V0-303` | `ImportGraphKinds`, `WalkImportGraph` (`internal/contextindex/import_graph.go`) | `TestImportGraphImpactParity`, `TestImportGraphLookupParity` |
| `NGI-V0-304` | `BuildImportGraph`; `runGraphImports` (`cmd/corvint/graph_imports.go`) | `TestImportGraphRefusalsAndCancellation`, `TestGraphImportsCLIAdmission` |
| `NGI-V0-305` | `callGraphImports` (`internal/mcp/bridge/import_graph.go`) | `TestGraphImportsCLIMCPParity`, `TestGraphImportsToolAdmission`, `TestImportGraphMCPProfile`, `TestToolsExposeOnlyDeliveredClosedReadSurface` |
| `NGI-V0-306` | `angularResourceEdges` (`internal/contextindex/import_graph_edges.go`) | `TestImportGraphAngularResources` |
| `NGI-V0-307` | `rubyConstantEdges` (`internal/contextindex/import_graph_edges.go`) | `TestImportGraphRubyConstants`, `TestImportGraphRubyIncompleteResolution` |
| `NGI-V0-308` | `ImportGraphKinds`, `writeGraphEdge`; `importGraphKindsPattern` (`internal/mcp/bridge/import_graph.go`) | `TestImportGraphKindsClosedSet`, `TestImportGraphAngularResources`, `TestImportGraphRubyConstants` |
| `NGI-V0-761` | `requestSpec` and `rubyRequestSpecConventionScore` in `internal/contextindex/ruby_autoload.go`; `conformance/ruby-profile-v0/rows.json` | `TestRubyControllerReachesItsRequestSpecByConvention_NGIV0761`; `TestRubyProfileImpactRows_RUBYAFFECTEDV0014` (`request-spec`, `controller-spec`) |
| `NGI-V0-980` | `moduleReading.swiftImpact`, `buildSwiftTargetMap` (`internal/contextindex/module_impact.go`) | `TestNGIV0980SwiftReverseImpactFollowsSPMTargetMembership` |
| `NGI-V0-981` | `moduleReading.kotlinImpact`, `kotlinImport` | `TestNGIV0981KotlinReverseImpactFollowsPackageImports` |
| `NGI-V0-982` | `moduleReading.csharpImpact`, `csharpUsing`, `csharpEncloses` | `TestNGIV0982CSharpReverseImpactFollowsUsingDirectives` |
| `NGI-V0-983` | `moduleReading.referenceRow`, `references`, `moduleReading.file` | `TestNGIV0983SameModuleMembersAreReferences` |
| `NGI-V0-984` | `withModuleMapped`; `moduleMapped` in `reverseImportProfileGap` (`internal/contextindex/receipt.go`) | `TestNGIV0984UnmappedModulePathKeepsProfileGap`, `TestImpactProfileGapWordingIsLanguageNeutral` |
| `NGI-V0-985` | `moduleReading.disclosures` | `TestNGIV0980SwiftReverseImpactFollowsSPMTargetMembership`, `TestNGIV0981KotlinReverseImpactFollowsPackageImports`, `TestNGIV0982CSharpReverseImpactFollowsUsingDirectives` |
| `NGI-V0-986` | `ImpactCapabilities` (`internal/contextindex/impact_capabilities.go`); `impact` help (`cmd/corvint/help.go`) | `TestNGIV0986CapabilitiesListModuleReferenceSuffixes`, `TestGPKV0085ImpactCapabilitiesAgreeWithAdmission` |
| `NGI-V0-987` | `ImportedModules`, `TargetModule` (`internal/liveverify/affected/swift/imports.go`) | `TestNGIV0987ImportedModulesReadsCodeOnlyImports` |

## Non-goals, rollout and rollback

No Ruby runtime call-graph resolver, compiler/type-checker, complete dynamic
semantic impact, dynamic capacity increase, test executor, new provider transport
or working-tree non-Go admission. Existing wider positional suffix support is not
removed. The registry dialect performs no runtime evaluation, infers no accessor
or registry, and folds no constant across files or beyond a single literal; it
does not follow destructured or reassigned accessors, bracket-string member
calls, or registries assembled at runtime, and it does not change range impact.
Roll it back by removing the `registries` declarations or the dialect code. The Ruby
autoload reading (`NGI-V0-016` to `NGI-V0-021`) evaluates no Ruby, loads no Rails
configuration, follows no `require`, ancestor chain or `include`, and supports no collapsed
directory; its reverse references are syntax-authority hints, never closure. The second hop
(`NGI-V0-034`) is a fixed two-hop bound, not a transitive closure; advisory rows never satisfy
an authoritative selection. Roll back `NGI-V0-033` to `NGI-V0-038` and `NGI-V0-040` by removing
the `rubyImpactRequested` selection, the second-hop loop, the `advisory` count, the `mapped`
guard, `rangeRubyAutoload`, `isRubyTestPath` and the index bounds; roll back the whole reading by removing
`ruby_autoload.go`, `internal/rubyautoload` and their wiring in `impact`; the declaration
file is then ignored. The implementation stays experimental pending human acceptance and
integration. The Swift, Kotlin and C# readings (`NGI-V0-980` to `NGI-V0-987`) compile nothing,
read no `.csproj`, Gradle or Xcode project, follow one import hop, and change neither range
impact, `corvint graph`, the `GPK-V0-027` oracle nor the historical packet budget; roll them back
by removing `module_impact.go`, `imports.go`, the `moduleMapped` view and their wiring in `impact`,
`reverseImportProfileGap`, `ImpactCapabilities` and the help text. Revert task-owned code/spec changes to roll back; preserve original
receipts and the failed/unsupported outcomes. Kill or repair the extension if it
changes legacy Go bytes, invents semantic closure or weakens immutable binding.
