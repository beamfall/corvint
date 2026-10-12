# 2026-10-11 docs corpus discover-legacy (GitHub #811)

## Decision

`corvint docs corpus discover-legacy --runner jasmine|mocha --revision FULL_COMMIT --path PATH
[--input REGISTRY.json]` is an experimental static producer of the existing legacy inventory shape
(`DCP-V1-060..069` in `docs/specs/documentation-corpus-v1.md`). It reads Git blobs only, with a new
comment- and literal-aware tokenizer (`internal/doccorpus/behavior_legacy_lex.go`) in the style of
the Playwright producers; no external parser, Node runtime or download. The legacy model types in
`behavior_legacy.go` are unchanged; the producer and crosswalk live in new files so the parallel
issue-812 change (manual legacy cases) stays additive.

Design choices:

- Criterion IDs are ordinal (`c1..cn`) per case, so a changed expected value still resolves to the
  same criterion and the reviewed mapping is refused rather than reported as missing.
- A fixture identity is `kind:callee@sha256:<digest of canonical call text>`. Because the identity
  embeds the digest, a different seed both makes the crosswalk check `not-parity` and makes the
  existing compiler emit `legacy-precondition-mismatch` with no code change in the compiler.
- Canonical call text drops layout (a space only between two word tokens), so reformatting keeps the
  digest and any literal or name change alters it.
- The crosswalk is a suggestion list plus a re-check of reviewed mappings; it never emits
  `legacy_criteria` or a review. A refused `same`/`stronger` mapping exits 1 with the full record,
  like `behavior-adapter --check`.

## Evidence

Focused tests (go1.27.2, `-p 1 -count=1`): `TestLegacyDiscoveryInventory`,
`TestLegacyDiscoveryDeterministicReadOnly`, `TestLegacyCrosswalk`,
`TestLegacyPreconditionDigestCompileGap`, `TestLegacyDiscoveryRefusals`,
`TestLegacyDiscoveryStaticEdgeCases`, `TestLegacyLexerCanonicalAndEscapes` (`./internal/doccorpus`) and
`TestDocsCorpusDiscoverLegacyCLI` plus the existing `Docs|Corpus|Help` tests (`./cmd/corvint`).
The fixtures are a Jasmine suite (8 cases) and a Cypress/Mocha suite (5 cases, with `it.skip`,
`context`, `it.only`, a `should` callback and a bare `.should`) under
`internal/doccorpus/testdata/legacy-suite/`, committed into a temporary Git repository per test.

## Remaining limits

- Static extraction only. No legacy test is executed, so runtime parity still needs a qualified
  `corvint-legacy-behavior-run/1` baseline; `parity-candidate` always carries
  `runtime-baseline-required`.
- Helper, custom-command, support and fixture-file bodies are not followed; a precondition binds its
  call text, not the seed data it names.
- Roles are not inferred and are always empty; a reviewed mapping whose target requires roles is
  `not-parity`.
- Global support hooks and root-level hooks in other files are not seen.
- Assertions inside hooks only contribute to precondition digests, not to criteria.
- JSX, decorators or other syntax the tokenizer cannot classify make the whole file a `lex-failed`
  unknown; no case is invented.
- Only top-level call statements of a hook body are preconditions; a call nested inside a
  conditional, loop or callback in a hook is not recorded, and every case under that hook carries a
  `hook-not-static` unknown so it cannot reach `parity-candidate`.
- A `/` directly after `}` is refused as `lex-failed` rather than classified, because its regex or
  division reading depends on statement context the static reader does not track.
- External utility on a real legacy suite is `NOT_OBSERVED`.

## Review round 1

Codex round 1 failed with nine P2 findings; all were fixed with regressions in
`TestLegacyDiscoveryStaticEdgeCases`, `TestLegacyCrosswalk` and `TestLegacyDiscoveryInventory`:
regex after `)` now follows the parenthesised keyword and `/` after `}` refuses the file; string
escapes are decoded or refused; template substitutions are lexed so assertions and runner calls in
them are reported; `withContext` is skipped and chained matcher continuations are unknowns; fixture
identities keep execution order; nested body preconditions are recorded with a
`nested-precondition` unknown; unbindable hooks propagate `hook-not-static` to every case beneath;
precondition text is the full canonical text its digest covers; and the crosswalk validates the
registry header and identities and requires a current mapping review, a declared target criterion
and a reviewed target annotation before `parity-candidate`.

## Review round 2

Codex round 2 failed with ten P2 findings, all fixed with regressions in
`TestLegacyDiscoveryStaticEdgeCases`, `TestLegacyLexerCanonicalAndEscapes` and
`TestLegacyCrosswalk`: multi-character operators are tokenized and canonical text separates
adjacent operators, so `x + ++y` and `x++ + y` digest differently; surrogate pairs decode and lone
surrogates refuse the file; an unchained Cypress `.and(...)` and an indexed or called matcher
continuation are `unparsed-assertion` unknowns; a runner call that is not a statement of its own
(with automatic semicolon insertion at line breaks) is dynamic; a recorded hook call that passes a
callback or template substitution, and a root or dynamic hook, give cases `hook-not-static`, and
root or dynamic hooks no longer contribute preconditions; the first of repeated titles also gets
`duplicate-title`; duplicate registry mappings are refused; and body preconditions inside
template substitutions are recorded with a `nested-precondition` unknown.

## Review round 3

Codex round 3 (the last review the lane allows) failed with seven P2 findings
(`/private/tmp/claude-501/w13-legacy-codex-r3.log`). All seven were fixed after the review with
regressions in `TestLegacyDiscoveryStaticEdgeCases` and `TestLegacyLexerCanonicalAndEscapes`: a
hook call inside a template substitution gives every case in its scope `hook-not-static`; Cypress
`.contains`/`.and` inside a template substitution and matchers reached through a computed member
(`['should']`, `cy['contains']`) are `unparsed-assertion` unknowns; a `.contains` whose second
argument is neither a literal nor a plain options object is an unknown instead of a guessed tuple;
an assertion inside a function the case defines but does not pass directly as a call argument is
an unknown, not a criterion; canonical text writes an ASI restricted-production line break as `;`
so `return\n1` and `return 1` digest differently; and a backslash before U+2028/U+2029 is a line
continuation while template CR/CRLF cooks to LF. These fixes have no independent re-review, so the
verdict stays FAIL and DCP-V1-060..069 remain proposed/experimental, not accepted.

## Review round 4 repairs (fail-closed residue check)

Codex round 4 failed with four P2 findings (`/private/tmp/claude-501/w13-legacy-codex-r4.log`):
comments ended only at LF; a computed Cypress subject (`cy['get']('nav').contains(...)`) made the
assertion vanish; parenthesised arrows and object methods escaped the helper filter; and
`cy['task'](...)` was dropped without uncertainty. Each was a variant of one gap: extraction
recognised known forms and silently ignored the rest. The repair makes discovery fail closed.

- One lexer helper treats LF, CR, CRLF, U+2028 and U+2029 as line terminators. They end `//`
  comments, count for ASI and restricted productions through a per-token line-break flag, and
  refuse regex literals. Anchor lines stay LF-delimited, matching every other corpus anchor.
- A residue pass runs after extraction. Every code-token `expect`, `expectAsync`, `assert`,
  `should`, `and`, `contains`, precondition word, `cy`/`browser`/`page` chain or runner call that
  no extraction consumed adds an `unparsed-assertion`, `unparsed-command` or
  `runner-call-in-case` unknown. Computed string members naming those words, `cy` reached as a
  member, and optional or computed root chains are covered too. A member `expect` is no longer
  taken for the runner's.
- Only a function or arrow passed whole as one direct call argument counts as a proven helper.
  Every other function-like construct is unproven, including grouped or IIFE functions, object
  and class methods, getters and class bodies.
- `cy['task']` (one static identifier-name string) resolves to `cy.task` and is recorded with its
  full-text digest, so `'A'` and `'B'` differ. Any other computed member is an unknown.

`TestLegacyResidueFailsClosed` pins each repro and novel forms such as `cy?.get(...)`,
`(0, expect)(a)`, `const c = cy` and `window.cy.task(...)`. Every repro other than the comment
terminator yields an unknown and `legacy-case-has-unknowns`. The comment-terminator repro is
repaired by correct extraction: the formerly hidden assertion becomes a visible criterion.

Inventory output for the existing fixtures and edge suites was byte-compared with `65fee2174`.
It is unchanged except that one duplicate computed-continuation unknown is gone; that case keeps
its continuation unknown.

## Review round 5 repairs

Codex round 5 (`/private/tmp/claude-501/w13-legacy-codex-r5.log`) confirmed the four round-4
repros fixed. It found eight P2 variants that still escaped the residue check:

- a keyword property name (`obj.return / ...`) was read as starting a regex;
- `1..should` was swallowed into a number;
- `cy.task[method](...)` was recorded as `cy.task`;
- a grouped computed name (`globalThis[('expect')]`) escaped;
- a computed name inside a template escaped;
- a member-qualified `browser`/`page` precondition escaped;
- `beforeEach.call(...)` added no doubt;
- keyword-named object methods (`{ return() {} }`) escaped the helper filter.

The repairs:

- A keyword after `.` ends an expression.
- Numeric literals follow the JavaScript grammar, and an identifier directly after one refuses the
  file.
- A non-literal computed member makes the callee unresolved: no precondition in a case, a
  `hook-not-static` doubt in a hook.
- A watched string anywhere inside a bracket is an unknown.
- Template substitutions are checked for watched strings and matcher names.
- A member-qualified `browser`/`page` precondition, or a precondition word not reached through
  plain members, is an unknown.
- A runner function used as a value is `runner-reference-not-static` at suite scope (a hook gives
  its cases `hook-not-static`) and `runner-call-in-case` in a body.
- A keyword method name is unproven unless it is a statement header outside an object literal.

A generic backstop now reports any unbound matcher member (`to`, `toX...` other than built-ins
such as `toString`), whatever expression reached it. `TestLegacyResidueFailsClosed` gains one
regression per variant. The byte comparison with `65fee2174` is unchanged from round 4.

## Review round 6 findings and closed-world allowlist

Codex round 6 (`/private/tmp/claude-501/w13-legacy-codex-r6.log`) found seven P2 variants that
still reached `parity-candidate`:

- a regex read as division after a declared `of`;
- an escaped identifier (`cy.task(...)`);
- an indexed or non-identifier computed precondition member (`cy.task[0]`, `cy.task['x-y']`);
- a computed matcher member (`['toBe']`);
- a member-qualified or computed hook (`globalThis.beforeEach`, `globalThis['beforeEach']`);
- an alias (`const p = page; p.goto(...)`);
- an opaque helper call (`checkHidden()`).

Each round had found new escapes from a blocklist, so the verdict was redesigned (coordinator
decision). The residue check stays as a diagnostic. The verdict now comes from a closed-world
allowlist in `behavior_legacy_allow.go`. A case is admitted only when every token of these regions
belongs to a recognized statement form:

- the file structure;
- each enclosing hook body;
- the case callback and body.

The forms are:

- `expect` chains of known matchers with literal arguments;
- `cy` query chains ending in `should`/`and`/`contains`;
- `cy`/`browser`/`page` precondition calls with literal arguments;
- an optional `await`;
- operands that are literals or plain member paths;
- `;`, line breaks and comments.

The first refused token gives one `not-allowlisted` unknown, which names its region. As defence in
depth, the key token of every admitted statement must have been consumed by extraction.

Over-refusal is accepted scope. Helper calls, Cypress actions, declarations, imports, `done`
callbacks, `afterEach` and regex literals all make a case unknown. In the checked-in fixtures,
`shows the cart total` stays `parity-candidate`.

The allowlist does not prove the following. They are recorded as limitations in DCP-V1-062/063 and
in the inventory:

- other cases' bodies and the hooks of non-enclosing suites, since cross-test state is a
  runtime-baseline question;
- getters on operand paths;
- custom matchers or commands that override a known name;
- support files.

`TestLegacyResidueFailsClosed` now requires `not-allowlisted` for every residue repro. It adds the
seven round-6 repros and further hook, preamble and callback-parameter forms, plus positive checks
for the admitted forms. `TestLegacyDiscoveryStaticEdgeCases` checks that every earlier-round edge
case is refused. The exception is the root-hook case, which keeps `hook-not-static`.

## Review round 7 repairs

Codex round 7 (`/private/tmp/claude-501/w13-legacy-codex-r7.log`) found no tokenizer escape from
the allowlist. Instead it found six P2 forms that the allowlist admitted even though their criteria
or fixtures did not describe what runs. The orchestrator decided to refuse each one rather than
design a new encoding; each refusal is an accepted limit in DCP-V1-062/063:

- **Function-calling chainers.** A `should`/`and` chainer that calls a function (`not.throw`
  through `.its(...)`, `satisfy`, `change`, `increase`, `decrease`, `respondTo`) is refused. Every
  chainer word must now be a known chai or chai-jquery word, and chainers cannot take a callback.
- **`contains` options.** `contains` is admitted only as `(content)` or `(selector, content)`, so
  an options argument the criterion would drop is refused.
- **Subject-changing chainers.** After a chainer that yields a new subject (`attr`, `prop`, `css`,
  `data`, `property`, `ownProperty`, `length`, ...), a following `.and` or `.should` is refused.
  The criterion locator names only the original subject.
- **Ambiguous string values.** The criterion value field stays as it is, with no wire change. A
  single string value that could read as another literal's canonical text is refused instead:
  - it starts with a digit, `.`, `-`, `+`, `[`, `{`, `(` or a quote;
  - or it starts with `true`, `false` or `null`.

  This removes `toBe(1)`/`toBe('1')` and the no-value marker collisions.
- **Completion order.** `await` is refused, as are `async` and expression-bodied callbacks,
  because completion order is not visible in the source. This makes awaited Playwright and
  WebdriverIO cases unknown.
- **Criterion bound.** A case over the registry's 4,096-criterion bound gets `criteria-over-bound`
  and an empty criterion list, never a truncated one.

`TestLegacyResidueFailsClosed` adds one regression per form, plus hook and case-callback
variants. It also checks that an unambiguous string value is still admitted and that a compact
over-bound case gets the new unknown.
