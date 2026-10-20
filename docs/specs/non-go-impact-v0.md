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
- Status: proposed/experimental; local source and focused conformance implemented; human acceptance and integration pending. NGI-V0-007..011 (declared `string-key-registry/0` dialect: literal registry keys to the tests passing them) are accepted under the owner's standing auto-accept (2026-10-11; V1-1365) and implemented over synthetic repositories. NGI-V0-012..015 (an unresolved entry widens every JS/TS path; lazy `require()`/`import()` entries; a non-exported top-level registry; a same-file top-level `const` key) are accepted under the same standing auto-accept (V1-1455, V1-1456, V1-1457). NGI-V0-016..021 (Ruby named-constant reverse references by autoload path convention: path-to-constant mapping, default roots or a declared `ruby-autoload/0` file, literal inflections, a reference class ranked below imports, metaprogrammed names kept unknown, and a report of the roots and inflections applied) are accepted under the same standing auto-accept (V1-1674). NGI-V0-027 (`bundle exec rspec` named for a Ruby path when the tree tracks a root Gemfile and `.rspec` or `spec/`) is accepted under the same standing auto-accept (V1-1796; GitHub #1130). NGI-V0-051..053 (the Ruby command in committed-range receipts; RSpec gated on an rspec gem in the committed Gemfile or Gemfile.lock; `bin/rails test` or `bundle exec rake test` for a proved Minitest suite) are accepted under the same standing auto-accept (V1-1804, V1-1805; GitHub #1128, #1130). NGI-V0-059 (the unknown-language line in a Ruby range receipt whose plan names no language command) is accepted under the same standing auto-accept (V1-1827; GitHub #1130). NGI-V0-033..040 (Ruby autoload reverse references on by default for any request naming a Ruby path, a bounded second hop, advisory marking, no `reverse-import-rule-unavailable` for a mapped path, range impact, RSpec test paths in the Ruby reading's test rule, the reusable `internal/rubyautoload` mapper, and a bounded reference index) are accepted under the same standing auto-accept (V1-1794, V1-1700; GitHub #1129).
- Exists: Ruby/JS/TS versioned positional, committed-range and MCP admission; bounded structured unknowns; existing JS/TS import rules; a declared string-keyed registry dialect and advisory Ruby named-constant reverse references by autoload convention, two hops deep, for default, syntax-profile and range impact.
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
  adds `registry-export-unresolved` and applies to every JS/TS changed path. Each of these, and
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
  function declaration. The edge binds a changed path exactly as rule (c) binds an importer to
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
  `autoload_lib_once` or `push_dir` site counts in `ruby-autoload-config-unresolved`.
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
  Name in the innermost enclosing namespace, never a reference; every `class`/`module` the
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
  fewer.
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
  sites in total. Every reference whose resolutions or sites a bound cut MUST be counted in a
  `ruby-reference-index-bounded` unknown row on each mapped changed Ruby path, never dropped
  silently; below both bounds no row is added.

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
  `../build-log/2026-10-18-v1-1796-ruby-verification-command-1796.md`. Rollback: remove the
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
  GitHub #1130 fixture run in `../build-log/2026-10-18-v1-1804-ruby-range-verification.md`.
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
  Failure modes: a Ruby repository whose Makefile declares a gate target gets the line in a range
  (whose gate is the profile gate) but not in path impact (whose fallback gate reads the
  Makefile); that range plan still names no language command, so the line is accurate.
  Acceptance evidence: `TestNGIV0059RubyRangeReportsUnknownVerification`,
  `TestNGIV0059RangeWithoutRubyPathKeepsUncertainty`, `TestNonGoRangeImpactLegacyGoBytes`.
  Rollback: remove the `rangeHasRubyPath` and `lacksLanguageVerification` branch in `RangeImpact`
  and the tests.

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
| `registry-export-unresolved` | The declared export of `registry` is not a readable object literal, so none of its keys are known; selection is widened (`NGI-V0-008`, `NGI-V0-010`). |
| `accessor-key-nonliteral` | `count` calls of `accessor` pass a key that is not a literal; selection is widened to every caller (`NGI-V0-010`). |
| `registry-caller-unavailable` | Sources naming a declared accessor could not be read or lexed completely (`NGI-V0-010`). |
| `ruby-constant-unmapped` | The changed Ruby path is below no autoload root, below an ignored directory, or its path does not camelize to a constant (`NGI-V0-016`). |
| `ruby-autoload-declaration-invalid` | `.corvint/ruby-autoload.json` is present but invalid or unavailable, so no path is mapped (`NGI-V0-017`). |
| `ruby-autoload-config-unresolved` | `count` autoload root, collapse or inflection configuration sites were not literal or not modelled (`NGI-V0-017`). |
| `ruby-constant-lookup-dynamic` | `count` dynamic constant lookups or dynamic sends in other Ruby sources may name the constant (`NGI-V0-019`). |
| `ruby-constant-name-literal` | `count` strings or association symbols in other Ruby sources may name the constant without a resolvable reference (`NGI-V0-019`). |
| `ruby-constant-source-unavailable` | `count` other Ruby sources could not be read for constant references (`NGI-V0-019`). |
| `ruby-reference-index-bounded` | `count` Ruby references were indexed incompletely because the resolution or retained-site bound cut them, so the named-constant rows may be incomplete (`NGI-V0-040`). |

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
| `NGI-V0-010` | `registryDialect.impact`; `extra` rows in `nonGoImpactUnknowns` | `TestRegistryDialectNonLiteralKeyWidens`, `TestRegistryDialectChangedRegistrySelectsEveryCaller`, `TestRegistryDialectUnresolvedRegistryWidens`, `TestRegistryDialectReexportedRegistryIsUnresolved` |
| `NGI-V0-011` | `internal/contextindex/registry_dialect_test.go`, `internal/appmap/registries_test.go` | tests above; real CLI output in `../build-log/2026-10-14-v1-1365-string-keyed-registry.md` |
| `NGI-V0-012` | `registryDialect.impact` (`openEntry`) | `TestRegistryDialectLazyRequireNeverFailsOpen`, `TestRegistryDialectUnresolvedEntryAppliesToEveryPath`; real CLI output in `../build-log/2026-10-15-registry-dialect-lazy-const.md` |
| `NGI-V0-013` | `registryLazyBinding`, `registryThenCallback`, `registryAssertionTail`, `registryRequireFree`, `registryLazyTarget`, `parseRegistry`; `bound` in `registryDialect.impact` | `TestRegistryDialectLazyEntriesResolve`, `TestRegistryDialectLazyEntriesStayClosed` |
| `NGI-V0-014` | `registryObject`, `registryExportedOtherwise` | `TestRegistryDialectNonExportedRegistry` |
| `NGI-V0-015` | `registryCallSites`, `registryConstants`, `registryMember` | `TestRegistryDialectConstAccessorKey`, `TestRegistryDialectConstAccessorKeyStaysNonLiteral`, `TestRegistryDialectNonLiteralKeyWidens`; real CLI output in `../build-log/2026-10-15-registry-dialect-lazy-const.md` |
| `NGI-V0-016` | `Mapper.Constant`, `Mapper.Camelize` (`internal/rubyautoload/rubyautoload.go`) | `TestRubyAutoloadConstantMapping` (`internal/rubyautoload`), `TestRubyAutoloadNamedConstantReverseReference` |
| `NGI-V0-017` | `loadRubyAutoload`, `rubyAutoloadDeclaration` (`internal/contextindex/ruby_autoload.go`); `New`, `DefaultRoots`, declaration decoding and `readConfiguredInflections` (`internal/rubyautoload`) | `TestRubyAutoloadConstantMapping`, `TestRubyAutoloadMapperDeclaration` (`internal/rubyautoload`), `TestRubyAutoloadInflections`, `TestRubyAutoloadDeclaration` |
| `NGI-V0-018` | `scanRubyConstantSource`, `rubyDefinedConstant`, `rubyAssignsConstant`, `resolutions`, `rubyAutoload.impact`; wiring in `impact` | `TestRubyAutoloadNamedConstantReverseReference`, `TestRubyAutoloadNamespacedConstant`, `TestRubyAutoloadMetaprogrammingStaysUnknown`, `TestRubyAutoloadNestingBound`, `TestRubyAutoloadQualifiedClassInNamespace`, `TestRubyAutoloadLocalConstantShadows` |
| `NGI-V0-019` | `rubyAutoload.impact` unknown rows | `TestRubyAutoloadMetaprogrammingStaysUnknown`, `TestRubyAutoloadNamedConstantReverseReference` |
| `NGI-V0-020` | syntax-frontier guard in `impact`; `attachNonGoImpactUnknowns` | `TestRubyAutoloadDefaultOn`, `TestRubyAutoloadNamedConstantReverseReference`; `TestNonGoImpactCLIProfile`, `TestNonGoRangeImpactLegacyGoBytes` |
| `NGI-V0-021` | `rubyAutoload.report` over `Mapper.Report`, `attachNonGoImpactUnknowns` | `TestRubyAutoloadInflections`, `TestRubyAutoloadDeclaration`; real CLI output in `../build-log/2026-10-18-v1-1674-ruby-autoload-reverse-refs.md` |
| `NGI-V0-027` | `toolchainCommand`, `indexTracks` (`internal/contextindex/receipt.go`); `rubyVerificationCommand` (`internal/contextindex/ruby_verification.go`) | `TestNGIV0027RubyImpactNamesBundleExecRSpec`, `TestNGIV0027GoAndWebPlansUnchangedByRubySignals`, `TestNGIV0027RubyCommandAppearsOnceAcrossPaths`, `TestNGIV0027RubySignalKeepsOtherReceiptsUnknownLine`; real CLI output in `../build-log/2026-10-18-v1-1796-ruby-verification-command-1796.md` |
| `NGI-V0-051` | `rangeVerification`, `rangeHasRubyPath` (`internal/contextindex/range_impact.go`, `ruby_verification.go`) | `TestNGIV0051RangeImpactNamesRubyCommand`, `TestNGIV0051GoRangePlanUnchangedByRubySignals`, `TestNonGoRangeImpactLegacyGoBytes`; real CLI output in `../build-log/2026-10-18-v1-1804-ruby-range-verification.md` |
| `NGI-V0-052` | `rubyVerificationCommand`, `declaredGems`, `committedRootText` (`internal/contextindex/ruby_verification.go`) | `TestNGIV0052RSpecNeedsDeclaredGem`, `TestNGIV0027RubyImpactNamesBundleExecRSpec` |
| `NGI-V0-053` | `rubyVerificationCommand`, `rakefileDeclaresTestTask` | `TestNGIV0053MinitestCommands` |
| `NGI-V0-059` | `RangeImpact` unknown-language branch (`internal/contextindex/range_impact.go`), `lacksLanguageVerification` | `TestNGIV0059RubyRangeReportsUnknownVerification`, `TestNGIV0059RangeWithoutRubyPathKeepsUncertainty` |
| `NGI-V0-033` | `rubyImpactRequested` and the syntax-frontier selection in `impact` | `TestRubyAutoloadDefaultOn`, `TestRubyAutoloadIssue1130DefaultTransitive`, `TestNonGoImpactCLIProfile`; Go/JS goldens `conformance/cli-parity-v0`, `TestNonGoRangeImpactLegacyGoBytes` |
| `NGI-V0-034` | `rubyAutoload.impact` second hop (`traversed`), `referrers`, `result` | `TestRubyAutoloadIssue1130DefaultTransitive`, `TestRubyAutoloadSecondHopOnce`, `TestRubyAutoloadSecondHopConstantOnce` |
| `NGI-V0-035` | `result` (`advisory`); `setCoverage` (`internal/contextindex/receipt.go`); range coverage in `compileRangeImpact` | `TestRubyAutoloadIssue1130DefaultTransitive`, `TestRubyAutoloadRangeImpact`, `TestRubyAutoloadNamedConstantReverseReference` |
| `NGI-V0-036` | `rubyAutoload.mapped`; `nonGoImpactUnknowns`; `reverseImportProfileGap` | `TestRubyAutoloadIssue1130DefaultTransitive`, `TestRubyAutoloadNamedConstantReverseReference`, `TestRubyAutoloadMappedPathHasNoProfileGap` |
| `NGI-V0-037` | `rangeRubyAutoload`, wiring in `compileRangeImpact` | `TestRubyAutoloadRangeImpact`; `TestNonGoRangeImpactLegacyGoBytes` |
| `NGI-V0-038` | `isRubyTestPath` (`internal/contextindex/ruby_autoload.go`); `isTestPath` unchanged | `TestIsTestPathRSpec`, `TestRubyAutoloadIssue1130DefaultTransitive`, `TestRubySpecPathNonRubyRequestBaseBytes` |
| `NGI-V0-039` | `internal/rubyautoload` package | `TestRubyAutoloadConstantMapping`, `TestRubyAutoloadMapperDeclaration`; real CLI output in `../build-log/2026-10-18-v1-1794-ruby-constant-impact.md` |
| `NGI-V0-040` | `resolutions`, `indexSites`, `loadRubyAutoload`; `ruby-reference-index-bounded` in `rubyAutoload.impact` | `TestRubyAutoloadResolutionBound`, `TestRubyAutoloadSiteBound` |

Names in this matrix identify evidence rather than implying passing execution.
Observed execution and independent findings are retained in
`../build-log/2026-09-30-non-go-impact.md`. No broad formal qualification or promotion is claimed.

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
integration. Revert task-owned code/spec changes to roll back; preserve original
receipts and the failed/unsupported outcomes. Kill or repair the extension if it
changes legacy Go bytes, invents semantic closure or weakens immutable binding.
