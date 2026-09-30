# Cross-repository breakage map V0

Owner: Russell Lewis
Date: 2026-09-29
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner-approved six-feature wow slice, native ticket V1-0503,
`AGENTS.md`, `docs/specs/external-evidence-provider-v1.md`,
`docs/specs/external-evidence-provider-v2.md`.

## Agent digest
- Claim: A selected API maps pinned Go syntax references and original declared provider relationships across explicit local repositories.
- Status: proposed/experimental
- Exists: `internal/breakagemap`, public `breakage` CLI dispatch/help and focused tests.
- Blocked on: Behavioral breakage, adequate test coverage, complete inventory and V1-0417 promotion remain unqualified.
- Read next: Requirements; Failure modes; Acceptance evidence and rollback.

## Human-owned intent and user job

Given a selected changed API, show the exact caller and test reference sites and the
explicit flow/document relationships in a locally declared repository set. Expose all
unresolved edges and unsupported language boundaries. The user needs evidence to inspect,
not an invented promise that every downstream behavior will break or every test covers it.
The measured job is one actual two-repository fixture plus source use from Corvint and
the Beamfall shell consumer, with its relationship explicitly declared rather than
presented as discovery by the Go analyzer.

## Input and output

```
corvint [--root DIR] breakage --manifest FILE --api REPO:PATH:SYMBOL \
  --repository REPO=CHECKOUT [--repository OTHER=CHECKOUT] [--base FULL_COMMIT]
```

Manifest schema `corvint-breakage-manifest/0` contains `repositories` (each `id`,
`origin`, `commit`, `tree`), `sources` (each `repository`, `path`, `blob`, `start`,
`end`) and `providers` (embedded original `external-evidence-provider/1` or `/2`
records). Origins are root commits. Every source span is nonempty and 1-based inclusive.
Locations are operator arguments, not repository identities or manifest authority.
Relative manifest and checkout locations resolve against `--root`.

Output `corvint-breakage-map/0` contains the manifest digest, exact API selection,
repository bindings without local directory disclosure, change classification,
typed edges, unknowns, bounds and truncation. Each source anchor pins repository,
commit, tree, path, blob, inclusive span and SHA-256 of newline-joined span bytes.
`syntax-call`, `syntax-reference`, test-prefixed variants, `import-only`, and
`provider-relationship` remain distinct. A test reference is not covering-test evidence.
Provider edges retain the original relation, evidence category, rule and reference,
with the embedded provider bytes' SHA-256. Entity declarations are not source anchors.

## Requirements

- `BKM-V0-001`: The command MUST be explicit, read-only, local and experimental. It
  MUST NOT execute providers, discover checkout directories, clone, fetch, run tests,
  start services, change ranking or change the existing impact receipt.
- `BKM-V0-002`: The manifest MUST be one strict bounded UTF-8 JSON document with no
  duplicate or unknown members. Repository identities, immutable commit/tree/blob
  pins and unique exact relative source paths MUST be validated. Checkout bindings
  MUST name canonical Git top levels with the declared origin in both the selected
  commit and HEAD histories. Invalid pins MUST produce refusal or unresolved evidence,
  never witnesses derived from replacement worktree bytes.
- `BKM-V0-003`: Each emitted source witness MUST name an immutable Git blob and
  exact span/hash. Unrelated dirty files MUST NOT be read. Nonregular Git modes,
  missing objects, unavailable repositories, invalid spans and unsupported sources
  MUST remain unavailable or refused. HEAD differing from the pinned commit MUST be
  disclosed as historical; the report describes the explicit pinned bytes.
- `BKM-V0-004`: Go references MUST match the selected function's pinned module
  import path and parsed package name, including explicit aliases. The selected API's
  bounded ancestor go.mod metadata MUST be checked; an undeclared nested module
  withholds syntax mapping. Shadowed qualifiers, dot imports, receivers, build constraints,
  malformed sources and ambiguous declarations MUST NOT become resolved callers.
  Import-only candidates remain separate. Same-package unqualified calls are syntactic;
  non-call name ambiguity, references within the selected file, dependency replacement,
  type resolution and complete module inventories remain explicit gaps.
- `BKM-V0-005`: EEP V1 repository/entity composition and V2 path composition MUST
  reuse the existing strict EEP decoder and preserve original relation categories.
  Provider repository pins MUST agree with the manifest. Path endpoints MUST resolve
  only through admitted pinned sources; mismatches and missing endpoints MUST remain
  unresolved and MUST NOT advance traversal. Traversal may inspect a relation in either
  direction for association, but MUST retain its original direction and MUST NOT infer
  causal impact. Directory scopes are unsupported, not recursively expanded.
  Every provider projection MUST disclose that file/entity association does not
  establish dependency on the selected symbol merely through shared file membership.
- `BKM-V0-006`: Byte verification MUST NOT establish assertion truth, behavioral
  breakage, test execution, adequacy or complete coverage. Entity-only witnesses retain
  declaration state. Flow and documentation relationships are existing provider
  declarations; they MUST NOT acquire accepted intent, reviewed status or observed
  execution by being included. Unsupported languages retain declaration-only gaps.
- `BKM-V0-007`: `--base` MUST identify an available ancestor of the selected API
  repository's pinned commit. The selected package function's declaration bytes are
  compared at that same path. Unchanged declarations, changed declarations, unavailable
  base/target paths and unresolved declarations MUST be distinguished. Deletion,
  addition and rename MUST NOT be guessed from similarity. This command does not
  implement automatic changed-declaration inventory or close V1-0417.
- `BKM-V0-008`: Limits MUST be enforced: 8 repositories, 256 source files, 32 path
  components, 1 MiB manifest/blob, 16 MiB aggregate source, 4 existing bounded EEP
  records, 2000 edges and 2000 unique unknowns, a 30-second request context and
  10-second bounded contained Git calls. Input overflow refuses; output overflow
  marks truncation. Output is deterministic for the same bytes/bindings, always
  `scope: INCOMPLETE`, and never claims that the explicit inventory is exhaustive.

## Failure modes and non-goals

Untrusted manifests carry data, never commands. Secrets-shaped manifest content is
refused. Git subprocesses use the existing contained process runner and sanitized
environment, disabling network/lazy fetching, credentials, replacement objects and
optional locks. Only named immutable blobs and bounded ancestor module metadata are
read; no new language scanner or whole-repository index is introduced.

The Go profile recognizes package functions, not receiver dispatch or general symbol
semantics. Package matches are syntax evidence; actual dependency versions and runtime
bindings remain unverified. Provider text is untrusted external declaration. A fresh
blob does not validate its author's relationship claim. No source inference creates
flow intent, accepted documentation or a passing test. Existing optional console,
flow providers and documentation corpus contracts retain their own authority.

## Acceptance and rollback

`TestCrossRepositoryBreakageMap` creates separate Git histories and checks aliases,
direct test references, import-only candidates, shadowing/dot/method/build constraints,
malformed sources, original flow/docs declarations, immutable spans, deterministic
results and clean worktrees. `TestBreakagePinsFailClosed` challenges origin/tree/blob,
unbound checkout, endpoint blob and provider revision pins. `TestBreakageManifestRefusals`
challenges ambiguous JSON and paths. `TestBreakageSymlinkAndHistoricalPins` verifies
mode refusal and historical disclosure. `TestBreakageV1EntityComposition` covers
the V1 entity seam and missing entities. `TestBreakageBaseAndModuleGaps` covers
unchanged/renamed/deleted API states, foreign bases and hidden nested modules.
`TestBreakageEdgeBound` checks truncation. CLI argument tests exercise the same runner.

Retain an actual source-bound Corvint map and an explicitly declared Beamfall shell
association with commit/tree/blob/span pins, the source manifest, output and candidate
identity. The shell consumer's references to `corvint` and `corvint-harness-event/0`
do not establish Go semantic discovery, deployment qualification or V1-0417 closure.
Root integration owns live receipts, independent review, CEM and native completion.
Focused tests do not imply a full repository gate was run.

Rollback removes the optional command registration and this package. There is no
persistent state, migration, daemon or changed default ranking to unwind.

## Named diagnostics

| Code | Meaning |
|---|---|
| `checkout-unbound` | A declared repository has no explicit checkout binding; retain an unresolved repository rather than discovering or fetching a checkout. |
