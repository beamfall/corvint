# Change Witness Relation V0

Owner: Russell Lewis (repository owner). Accepted by the owner in decision 0047 (2026-09-04); the
symbol-resolver boundary (Q1) is decided by
`docs/decisions/0151-change-witness-symbol-resolver-boundary-2026-09-12.md`.
Date: 2026-08-29
Intent status: accepted (decision 0047, 2026-09-04)
Delivery status: experimental
Authoritative inputs: `docs/specs/change-frontier-v0.md`, `docs/specs/ocm-v0-dogfood.md`,
`docs/specs/lexical-relevance-floor-v0.md`, `docs/specs/cem-0.2-canonical-binding.md`,
`docs/decisions/0006-caller-authored-authority.md`

## Agent digest
- Claim: The accepted change-witness relation proposes structural proof that a referenced hunk changed the uniquely named definition.
- Status: accepted (decision 0047, 2026-09-04)/experimental
- Exists: pure evaluator `internal/changewitness` with test vectors; no Frontier profile consumes it.
- Blocked on: a `frontier/1` implementation to consume the evaluator (CF-V1-005..008).
- Read next: `change-frontier-profile-1.md` and `change-frontier-v0.md`.

This is one of the two relations `CF-V0-027` names ("Harness integration waits for separately
accepted change-witness and harness-authority relations under a new Frontier profile",
`docs/specs/change-frontier-v0.md:397-398`). Its companions are
[`harness-authority-relation-v0.md`](harness-authority-relation-v0.md) and the profile
[`change-frontier-profile-1.md`](change-frontier-profile-1.md). None of the three closes the
frontier gap by existing; each is a proposal that must be separately accepted.

## User and job

For one structurally linked OCM obligation in one pinned universe, a maintainer needs a deterministic
answer to a narrower question than "was the requirement satisfied": **did this change set modify the
thing the requirement names?** Under `frontier/0` that question has no closing answer at all — every
structurally linked obligation emits exactly one `INTENT_CHANGE` item
(`CF-V0-012`, `docs/specs/change-frontier-v0.md:177-188`), whether or not the referenced hunks touch
anything the requirement mentions.

`ocm-change-witnessed-v0` is a **structural containment relation**, not a semantic one. It says:

> At the target revision, this requirement span names an identifier that resolves to exactly one
> definition, at least one hunk the obligation structurally references changes lines inside that
> definition's span, and the requirement span itself is not authored by this change set.

It does not establish entailment, correctness, adequacy, completeness, causality, test coverage, or
that the change is the right change. It removes exactly one failure mode: an obligation "linked" to
hunks that demonstrably do not touch what it names.

## Verified current state

Every statement below was opened in the tree at this date; the citation is where to check it.

- `frontier/0` emits `NO_MATERIAL_CHANGE_WITNESS` for a linked obligation with no lexical candidate
  and `LEXICAL_CANDIDATE_NONCLOSING` for one with candidates; both are unconditional and neither can
  be removed (`CF-V0-012`, `docs/specs/change-frontier-v0.md:177-188`). The clause itself names this
  document's slot: "A future separately accepted relation and new Frontier profile may close intent
  change; V0 cannot" (`:187-188`).
- LRF's OCM-side result is capped at `lexically-proximate-candidate` and "never witnesses or closes
  an intent obligation" (`docs/specs/change-frontier-v0.md:54-55`, restating
  `LRF-V0-008` at `docs/specs/lexical-relevance-floor-v0.md:171`; the `cem-lexical-v0` definition
  that the CEM hunk side uses instead is `lexical-relevance-floor-v0.md:57-60`). The Go closure
  table reads exactly that field (`internal/frontier/closure.go:191`).
- Relation names in this repository are versioned string constants:
  `MatchedRelation = "test-report-matched-v0"` (`internal/tcq/canonical.go:17`), consumed by Frontier
  as `const tcqRelation = "test-report-matched-v0"` (`internal/frontier/closure.go:165`), and
  `"cem-lexical-v0"` (`internal/lrf/evaluate.go:211`, `internal/frontier/closure.go:191`). This
  document follows that convention.
- **The OCM intent scope is resolved at the target revision, not the base.**
  `internal/lrfrepo/ocm.go:444` reads the intent blob with `reader.blob(document.target, ...)`. A
  change set therefore supplies its own requirement text.
- **A partial guard against that already exists, and it is narrower than it looks.**
  `enforceBootstrap` (`internal/lrfrepo/ocm.go:680-699`) fires only when the intent path does **not**
  exist at `cem.BaseRevision`, and then requires that path's hunk to stay `unknown`. An intent path
  that exists at base and is *modified* by the change set passes with any disposition. So a caller
  may rewrite an existing requirement to describe whatever it happened to change.
- `CF-V0-031` — accepted 2026-08-29 under repository-owner delegation
  (`docs/decisions/0006-caller-authored-authority.md`) — states the general invariant: "a governing
  document confers authority on a change set only when it is resolved from a revision the caller did
  not author" (`docs/specs/change-frontier-v0.md:358-359`). `CF-V0-032` fixes the exact withholding
  form (`:373-389`).
- `CF-V0-026` requires the Frontier library and verifier to remain usable "without Corvint retrieval, a
  model call, network access, database, daemon, account, or service"
  (`docs/specs/change-frontier-v0.md:340-344`). Whether the definition resolver this relation needs
  is inside or outside that prohibition is **not** settled by any clause; it is question Q1 below.
- A premise this draft expected and did **not** find: nothing in the tree computes a
  definition-span-containment relation between a CEM hunk and an OCM requirement today.
  `internal/contextindex` resolves symbols for `query`/`impact`, but no code path connects it to
  `internal/frontier` or `internal/lrfrepo` (no import of `internal/contextindex` appears in either).
  This relation is new work, not a rename of something already computed.

## Definitions

- **requirement span**: the exact pinned OCM intent sub-span for one obligation ID, as already
  verified by the OCM verifier;
- **base-stable requirement**: a requirement span whose ID and exact bytes are identical at the
  independently supplied expected base and at the target;
- **named identifier**: a token occurring verbatim in the requirement span that the target-revision
  definition resolver maps to exactly one definition;
- **definition span**: the exact `(path, blobOid, startLine, endLine)` of that one definition at the
  target revision;
- **material intersection**: a non-empty overlap between a referenced hunk's changed target line
  range and a definition span, both taken from verified Git objects;
- **withheld witness**: a determination that a witness was computed but MUST NOT confer closure,
  reported in the `CF-V0-032` form rather than dropped.

## Requirements

### Relation identity and inputs

- `CWR-V0-001`: the relation string is exactly `ocm-change-witnessed-v0`. It is emitted only by a
  Frontier profile that admits it. `frontier/0` MUST NOT emit, consume, or recognise it; a
  `frontier/0` document containing it is `noncanonical-frontier` under `CF-V0-018`.
- `CWR-V0-002`: the relation is computed only over inputs the Frontier verifier already verified
  under `CF-V0-001` and `CF-V0-002`: the canonical CEM 0.2 patch, the bound OCM artifact, the
  independent expected base, the independent target, and target Git objects. A caller-supplied
  witness, definition span, symbol table, or index snapshot is never accepted as authority or as a
  shortcut, exactly as `CF-V0-002` already forbids for LRF and TCQ.
- `CWR-V0-003`: the relation's authority class is exactly `VERIFIER_DERIVED`. This is a new class,
  distinct from the three `CF-V0-017` admits (`NONE|PRODUCER_DECLARED|CALLER_REPORTED`). It means:
  every byte of the witness was recomputed by the verifier from objects already bound into the
  universe ID (`CF-V0-006`), so a caller cannot change the witness without changing
  `patchSha256`, `targetRevision`, or `intentBlobOid` and therefore the universe. It does **not**
  mean the witness is independently attested; the requirement text it reads is still authored by
  someone, which is what `CWR-V0-005` exists for.

### Witness conditions

- `CWR-V0-004`: an obligation carries `ocm-change-witnessed-v0` only when **all** of the following
  hold. Failure of any one leaves the obligation exactly as `frontier/0` would have left it.
  1. the obligation's OCM disposition is `linked` (an `unknown` obligation is out of scope: it has
     no structural references to witness, per `CF-V0-011`);
  2. the requirement span is **base-stable** (`CWR-V0-005`);
  3. at least one named identifier exists in the requirement span (`CWR-V0-006`);
  4. at least one hunk the obligation structurally references has a material intersection with that
     identifier's definition span (`CWR-V0-007`);
  5. that hunk's CEM disposition is `supported` and its path is not the OCM intent path
     (`CWR-V0-008`).

  The witness is **existential**, matching `CF-V0-008`'s shape for CEM bases: one qualifying
  identifier/hunk pair witnesses the obligation, and identifiers that abstain or fail remain
  diagnostics rather than reopening it.
- `CWR-V0-005`: **base-stability is the self-authorship guard and is not optional.** The relation
  resolves the obligation's requirement span at the independently supplied expected base as well as
  at the target. When the intent path is absent at base, or the obligation ID is absent at base, or
  the requirement span bytes at base differ from those at target, the obligation's requirement text
  is caller-authored for this change set and the relation MUST be withheld. This is `CF-V0-031`
  applied to the object being witnessed rather than to a cited authority: a caller who writes the
  requirement and the change in one change set is a caller on both sides, and a signature, a clean
  worktree, or the OCM artifact's own validity cannot upgrade that, because all of them are inside
  the authored bytes. `enforceBootstrap` (`internal/lrfrepo/ocm.go:680-699`) already implements the
  absent-at-base half of this for the introduced-document case; this clause generalizes it to
  modification and moves the consequence from "the hunk must stay unknown" to "the witness is
  withheld".
- `CWR-V0-006`: identifier resolution is deterministic and abstains rather than guesses. A token
  resolving to zero definitions, or to more than one, is not a named identifier and contributes the
  diagnostic `IDENTIFIER_UNRESOLVED` or `IDENTIFIER_AMBIGUOUS`. Resolution reads only target-revision
  Git objects through the profile's declared resolver; it MUST NOT consult a persisted index built at
  another revision, a dirty worktree, or a cache whose freshness the universe does not bind. The
  resolver is fixed by `CWR-V0-014`.
- `CWR-V0-007`: a material intersection is computed from the canonical CEM patch's target-side line
  ranges and the definition span's line range at the same revision. Both are already verified inputs;
  neither may be recomputed from a caller-supplied diff. An intersection of zero lines is not a
  witness. Whitespace-only and line-ending-only hunks, which `CF-V0-008` already excuses from the
  hunk frontier, MUST NOT witness an obligation: they close a basis question, not a change question.
- `CWR-V0-008`: a hunk inside the OCM intent path MUST NOT witness an obligation in that same intent
  path. This is the self-referential guard `LRF-V0` already applies as `SELF_REFERENTIAL_BASIS`
  (`docs/specs/change-frontier-v0.md:164-165`): editing the requirement is not evidence of
  implementing it.

### Withheld witnesses and reporting

- `CWR-V0-009`: a withheld witness MUST be reported, never dropped, in exactly the `CF-V0-032` form
  (`docs/specs/change-frontier-v0.md:373-389`): the determination keeps its subject, its referenced
  hunk IDs, and its reason; only its closing power is removed. The admitting profile emits reason
  `SELF_AUTHORED_OBLIGATION` for a `CWR-V0-005` withholding, with a `resolutionClass` of
  `AUTHORITY_REQUIRED` and a `nextAction` of `ESTABLISH_CHANGE_WITNESS` — the item stays OPEN and its
  next action is unchanged from `frontier/0`, because the maintainer's move is the same one.
- `CWR-V0-010`: a withheld witness MUST NOT be reported as an absent witness, as a lexical candidate,
  or as an operational failure. Each of those says something the relation did not determine, which
  `CF-V0-025` forbids (`docs/specs/change-frontier-v0.md:336-339`).
- `CWR-V0-011`: the relation is source-body-free, diff-body-free, and identifier-value-free in error
  output, inheriting `CF-V0-024` unchanged. A resolved identifier name and definition path are
  verified metadata and may appear only in a valid result.

### Non-goals

- `CWR-V0-012`: the relation adds no model call, embedding, ranker, confidence score, learned judge,
  acknowledgement, policy engine, network access, daemon, database, or stored history. It adds no OCM
  wire field: it reads the requirement span and structural references OCM already carries.
- `CWR-V0-013`: the relation MUST NOT be described as proof that a requirement is satisfied, that a
  change is correct, that a definition is the right definition, that the repository was exhaustively
  searched, or that an unwitnessed obligation is defective. It is the narrowest available refutation
  of "linked to hunks that touch nothing it names".

### Resolver boundary and evaluator vocabulary

Added 2026-09-12 by `docs/decisions/0151-change-witness-symbol-resolver-boundary-2026-09-12.md`.

- `CWR-V0-014`: the declared resolver is exactly Go `go/parser` plus `go/ast` over the target
  revision's `.go` blobs, read from Git objects and never the worktree, with no type checking, import
  resolution, network, external tool, persisted index, or scanner fallback. Its definitions are the
  top-level `func` (methods by name, receiver ignored), `type`, `var`, and `const` names other than
  `_`; a definition span is the declaration from its keyword to its end, or the member spec alone
  inside a parenthesized group. Spans use physical blob lines, ignoring Go line directives. When any target `.go` blob is refused by the grammar, no identifier
  resolves and the obligation abstains with `RESOLVER_SOURCE_UNPARSED`, because an unparsed blob can
  hold a second definition. Other languages have no resolver: a referenced hunk outside a `.go` path
  cannot witness and counts toward `RESOLVER_LANGUAGE_UNSUPPORTED`. Requirement tokens are Go
  identifier-shaped words matched case-sensitively; qualified names are not tokens.
- `CWR-V0-015`: the evaluator returns exactly one outcome, `witnessed`, `withheld`, or `abstained`.
  `withheld` carries only `SELF_AUTHORED_OBLIGATION` and the sorted unique referenced hunk IDs.
  `abstained` carries the first failing `CWR-V0-004` condition as `OBLIGATION_NOT_LINKED` (1),
  `RESOLVER_SOURCE_UNPARSED` (`CWR-V0-014`), `NO_NAMED_IDENTIFIER` (3), `NO_MATERIAL_INTERSECTION`
  (4), or `INELIGIBLE_WITNESS_HUNK` (5). `witnessed` carries every qualifying identifier, definition
  span, and hunk ID, sorted. Diagnostics are value-free counts of `IDENTIFIER_UNRESOLVED`,
  `IDENTIFIER_AMBIGUOUS`, and `RESOLVER_LANGUAGE_UNSUPPORTED`. Base-stability (condition 2) is checked
  before resolution, so `SELF_AUTHORED_OBLIGATION` takes precedence as `CF-V1-008` requires. Only
  `SELF_AUTHORED_OBLIGATION` is a Frontier reason; an abstained obligation keeps its `frontier/0`
  reason under `CF-V1-005`, and these abstention codes are not wire reasons.

## What this permits that is not permitted today, and what stays forbidden

**Newly permitted, under an accepting profile only.** A structurally linked obligation whose
requirement predates the change set and whose referenced hunks demonstrably change the definition it
names may emit **no** `INTENT_CHANGE` item. Under `frontier/0` that is impossible: `CF-V0-012` emits
one unconditionally.

**Still forbidden.** The relation cannot close an `INTENT_TEST` item — that is
`harness-authority-relation-v0.md`'s subject, and `CF-V0-014` and `CF-V0-015` are untouched here. It
cannot close an obligation whose requirement this change set wrote. It cannot close an OCM `unknown`
obligation. It cannot close a hunk-basis item. It cannot be acknowledged, overridden, scored, or
supplied by a caller. And it does not make `frontierState: EMPTY` reachable on its own: while
`INTENT_TEST` has no closing relation, every linked obligation still emits at least one item.

## Conformance and adversarial matrix

| Fixture | Required result |
|---|---|
| linked obligation, base-stable requirement, referenced hunk changes the named definition | witness; no `INTENT_CHANGE` item |
| same, but requirement span bytes differ at base | withheld; `SELF_AUTHORED_OBLIGATION`, item OPEN |
| same, but intent path absent at base | withheld; `SELF_AUTHORED_OBLIGATION`, item OPEN |
| named identifier resolves to two definitions | `IDENTIFIER_AMBIGUOUS` diagnostic; no witness from that token |
| one ambiguous identifier and one qualifying identifier | witness; ambiguous token remains a diagnostic |
| referenced hunk intersects nothing the requirement names | no witness; `frontier/0` reason preserved |
| witnessing hunk is inside the OCM intent path | no witness; `CWR-V0-008` |
| witnessing hunk is whitespace-only | no witness; `CWR-V0-007` |
| caller supplies a precomputed witness or definition span | rejected as input, not accepted as a shortcut |
| OCM `unknown` obligation | out of scope; unchanged `INTENT_CHANGE` item |
| two obligations sharing one hunk | independent result per obligation, as `CF-V0-012` already requires |
| identical inputs, two fresh processes | byte-identical relation output |
| identifier name, definition path, and requirement text canaries | absent from error output |

## Evaluation and kill criteria

- The relation is worth accepting only if it refutes real links. Measure on a sealed cohort: the
  fraction of `linked` obligations that carry **no** material intersection. If that fraction is near
  zero, the relation closes almost everything it sees and is a rubber stamp — kill it.
- Adversarial gate: an author who rewrites a requirement to match whatever they changed MUST be
  withheld by `CWR-V0-005` in 100% of attempts. One escape is an immediate blocker.
- Self-dogfood gate, stated as a warning rather than a target: on Corvint's own repository nearly every
  obligation is authored in the same change set as its implementation, so `CWR-V0-005` will withhold
  nearly every Corvint self-dogfood witness. That is the correct report, and it means Corvint cannot
  evaluate this relation on itself. See Q3.

## Simpler baseline and YAGNI cuts

The baseline is the `frontier/0` behaviour that exists: report `NO_MATERIAL_CHANGE_WITNESS` and let a
human read the hunks. This relation is worth its cost only if the withheld/witnessed split is
informative on a real cohort. If the cohort shows it is not, delete the relation and keep the
`frontier/0` item; do not weaken `CWR-V0-005` to raise the closure rate.

## Unresolved decisions deliberately deferred

- Identifier extraction from a requirement span is a token scan, not a parse. Whether it should
  admit qualified names, method receivers, or file paths is deferred; V0 admits only tokens that
  resolve exactly.
- Multi-definition requirements (a requirement naming several things, all of which should change)
  need a quantifier. `CF-V0-013` already defers quantifiers for test claims for the same reason;
  this relation defers them for the same reason and MUST NOT infer them.
- Cross-language resolution is deferred; `CWR-V0-014` resolves Go only.

## Questions only the repository owner can answer

- **Q1 (answered 2026-09-12, `CWR-V0-014`: Git objects plus the Go standard-library parser only). May the Frontier verifier depend on a Corvint-built symbol resolver?** `CF-V0-026` requires
  the library to work "without Corvint retrieval". A definition-span resolver is not retrieval in the
  query sense, but it is Corvint index machinery. Either `CF-V0-026` is read narrowly for the new
  profile, or this relation needs a resolver built from Git objects alone. This decision determines
  whether the relation is implementable at all.
- **Q2. Is `VERIFIER_DERIVED` an acceptable fourth authority class?** `CF-V0-017` freezes three. A new
  class is exactly the kind of additive change `CF-V0-029` contemplates, but it is a wire change a
  consumer must learn.
- **Q3. Is "an obligation authored in its own change set can never be witnessed" acceptable?** It is
  the honest consequence of `CWR-V0-005` and of accepted `CF-V0-031`. It also means Corvint's own
  repository — where every new clause and its implementation land together — gets a withheld witness
  on essentially every obligation, so Corvint cannot dogfood this relation. The alternatives are all
  worse: a base-instability predicate keyed on declared paths penalises the honest author and misses
  the attacker (the reasoning `docs/decisions/0006-caller-authored-authority.md` already recorded for
  `impact`), and dropping the guard reintroduces the laundering hole `CF-V0-031` closed.

## Traceability and rollback

Delivery is experimental: `internal/changewitness` evaluates the relation over caller-assembled
inputs. No Frontier profile, verifier, or command consumes it, and nothing yet reads its inputs from
Git objects, so no row below is evidence about a `frontier/1` document.

| Requirement | Surface | Evidence |
|---|---|---|
| `CWR-V0-001..003` | relation and authority-class constants only | `NOT_RUN`: no admitting-profile gate or `frontier/0` refusal vector |
| `CWR-V0-004` | `internal/changewitness/witness.go` `Evaluate` | `TestLinkedBaseStableObligationIsWitnessed` |
| `CWR-V0-005` | `baseStable` | `TestCallerAuthoredRequirementIsWithheld` |
| `CWR-V0-006` | `nameIdentifiers`, `identifierTokens` | `TestIdentifierResolutionAbstainsOnZeroOrManyDefinitions` |
| `CWR-V0-007` | `intersections` | `TestMaterialIntersectionExcludesEmptyAndWhitespaceHunks` |
| `CWR-V0-008` | `eligible` | `TestIntentPathAndUnsupportedHunksNeverWitness` |
| `CWR-V0-009..011` | withheld-witness reporting in a profile | `NOT_RUN`: the `CF-V0-032` form needs a `frontier/1` document |
| `CWR-V0-012..013` | boundary | `NOT_RUN`: no dependency or canary gate |
| `CWR-V0-014` | `internal/changewitness/resolve.go` | `TestResolverBoundaryAbstainsOutsideParsedGo`, `TestMaterialIntersectionExcludesEmptyAndWhitespaceHunks` (grouped spans), `TestDefinitionSpansIgnoreLineDirectives` |
| `CWR-V0-015` | `Result` vocabulary and ordering | `TestEvaluationIsInputOrderIndependent` plus the exact-result vectors above; two-process byte identity `NOT_RUN` |
| kill criteria | sealed cohort | `NOT_RUN` |

Rollback removes the relation and the profile clause that admits it. `frontier/0`, CEM, OCM, LRF,
TCQ, and every recorded unknown are untouched, because nothing in this document modifies them.
