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
`TestLegacyPreconditionDigestCompileGap`, `TestLegacyDiscoveryRefusals` (`./internal/doccorpus`) and
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
- Only top-level calls of a hook body are preconditions; a call nested inside a conditional, loop
  or callback in a hook is not recorded.
- External utility on a real legacy suite is `NOT_OBSERVED`.
