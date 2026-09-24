# Corvint Glossary

Owner: Russell Lewis
Drafted: 2026-08-29
Intent status: proposed
Delivery status: not-started
Authoritative inputs: `docs/PRODUCT.md`, `docs/ARCHITECTURE.md`,
`docs/specs/local-observability-dashboard-v0.md`, `docs/specs/change-frontier-v0.md`,
`docs/specs/lexical-relevance-floor-v0.md`, `docs/specs/test-claim-qualification-v0.md`,
`docs/specs/verification-planner-observer-v0.md`, `docs/specs/mcp-server-2026-07-28-v0.md`,
`docs/lrf-0.schema.json`, `docs/tcq-0.schema.json`

## User and job

An engineer, coding agent, or reviewer reading two Corvint specifications must be able to determine
whether a term used in both means the same thing, and whether two values of that term may be
compared. Today they cannot: `authorityClass` — the field on which every Corvint authority claim
rests — carries five disjoint value sets in two casing conventions, and its ordering is declared in
exactly one document that covers only one of the five.

This file is the single place a term is defined. Every other document cites it rather than
redeclaring the term.

## Status of this document

This document is **not ratified**. It separates two kinds of statement and never mixes them:

- **Current state.** A description of what the repository declares today. Every such claim carries a
  `file:line` citation and can be checked mechanically.
- **Proposal.** A recommendation that no owner has accepted. Every proposal appears under a heading
  or bullet explicitly marked *Proposal*. Nothing in this document changes a shipped wire value, a
  frozen profile, or a schema `const` until its owning specification adopts it.

No implementation is authorised by this file. The migration mapping in
[Proposed `authorityClass` migration](#proposed-authorityclass-migration) is a mapping table only.

## Verified current state

The following was checked against the working tree on branch `integration/go-migration`,
2026-08-29.

- No glossary contract exists. `docs/` contains no glossary file; the only occurrences of the word
  are `docs/CORVINT-TECHNOLOGY-REVIEW-2026-08-29.md:36`, which records the absence, and
  `docs/TECHNICAL-BRAIN.md:153`, which names a checked-in glossary as an admission precondition that
  nothing satisfies.
- `authorityClass` is declared with five disjoint value sets across five owners
  (`internal/dashboard/model/types.go:39-45`, `internal/mcp/bridge/bridge.go:803`,
  `docs/specs/change-frontier-v0.md:244`, `internal/lrf/types.go:12`,
  `docs/specs/verification-planner-observer-v0.md:285,389`), plus a sixth single-value constraint in
  `docs/tcq-0.schema.json:125`.
- A total order over `authorityClass` is declared exactly once, at
  `docs/specs/local-observability-dashboard-v0.md:84`, and covers only the seven dashboard values.
  No other surface's values are placed in it.
- The divergence is already recorded as debt at `docs/SPEC-TOOLCHAIN-INTEGRATION.md:118-123`, which
  names four of the five sets and requires "one ordered lattice, one casing, one owner — before the
  adapter, not after". This document is the response to that requirement.
- `frontier` is used at three different scopes without a qualifier that distinguishes them
  (`docs/PRODUCT.md:13`, `docs/ARCHITECTURE.md:82`, `docs/ARCHITECTURE.md:198`).

## Terms

Each entry gives one normative definition, the contract that owns it, and every location that
currently declares a variant of it.

### `authorityClass`

**Normative definition.** The strongest authority root that actually supports a value, on an axis
independent of whether the value was observed, whether it is complete, and whether it is current.
It never states that a proposition is true.

**Owning contract.** `docs/specs/local-observability-dashboard-v0.md:79` supplies the definition and
the only declared ordering (`:84`). *Proposal:* ownership of the axis moves to this file, and the
dashboard spec cites it.

**Declared variants.**

| Surface | Values | Casing | Citation |
|---|---|---|---|
| Dashboard model | `REPOSITORY_ACCEPTED`, `OWNING_VERIFIER`, `PROVIDER_QUALIFIED`, `ADAPTER_QUALIFIED`, `CALLER_REPORTED`, `ADVISORY`, `NONE` | SCREAMING_SNAKE | `internal/dashboard/model/types.go:39-45`; `docs/specs/local-observability-dashboard-v0.md:79` |
| MCP bridge result | `REPOSITORY_EVIDENCE`, `GIT_REPOSITORY`, `NONE` | SCREAMING_SNAKE | `internal/mcp/bridge/bridge.go:803`; `docs/specs/mcp-server-2026-07-28-v0.md:178` |
| Change Frontier item | `NONE`, `PRODUCER_DECLARED`, `CALLER_REPORTED` | SCREAMING_SNAKE | `docs/specs/change-frontier-v0.md:244` |
| Lexical Relevance Floor edge | `producer-declared` | lowercase-kebab | `internal/lrf/types.go:12`; `docs/lrf-0.schema.json:65,69`; `docs/specs/lexical-relevance-floor-v0.md:267,269` |
| Verification Planner/Observer | `CORVINT_PROCESS_OBSERVED`, `CALLER_REPORTED` | SCREAMING_SNAKE | `docs/specs/verification-planner-observer-v0.md:389,285` |
| Test Claim Qualification | `CALLER_REPORTED` (schema `const`) | SCREAMING_SNAKE | `docs/tcq-0.schema.json:125`; `docs/specs/test-claim-qualification-v0.md:43` |

`REPOSITORY_EVIDENCE` and `GIT_REPOSITORY` appear in no other set.
`PRODUCER_DECLARED` is absent from the dashboard lattice. `CORVINT_PROCESS_OBSERVED` is absent from
every other set.

### `epistemicClass`

**Normative definition.** Whether a value was measured, copied from a declaration, derived from
non-authoritative evidence, or is unavailable. Independent of authority.

**Owning contract.** `docs/specs/local-observability-dashboard-v0.md:78`.

**Declared variants.** `OBSERVED`, `DECLARED`, `ADVISORY`, `NOT_OBSERVED`
(`internal/dashboard/model/types.go:30-33`). The MCP bridge admits only the two-value subset
`OBSERVED`, `NOT_OBSERVED` (`internal/mcp/bridge/bridge.go:802`). The subset is a restriction, not
a divergence: both members carry the dashboard meaning.

### `ADVISORY`

**Current state, unresolved.** The token `ADVISORY` is a member of two different axes in the same
table: `epistemicClass` at `docs/specs/local-observability-dashboard-v0.md:78` and `authorityClass`
at `:79`. `docs/specs/local-observability-dashboard-v0.md:85-86` uses both in one sentence — a
proposed repository declaration is `DECLARED` with authority `ADVISORY`.

This file does not resolve the collision, because renaming a shipped enum member is the owning
spec's decision. It records the reading rule instead: a bare `ADVISORY` is never a complete
statement. Every use MUST name its axis (`epistemicClass: ADVISORY` or `authorityClass: ADVISORY`).
An unqualified `ADVISORY` in prose is a defect in that prose.

### Abstention

**Normative definition.** A bounded, valid, non-error result stating that the profile declined to
produce a witness or a closing relation for a named subject, together with the exact reason. An
abstention is a result, never a missing field and never a failure
(`docs/MCP-SERVER.md:100-101`).

**Owning contract.** *Proposal:* none exists; each profile names its own field. This file is the
cross-profile definition and the following table is the mapping between the field names.

**Declared variants.**

| Surface | Field | Values | Citation |
|---|---|---|---|
| MCP bridge | `state` | `READY`, `ABSTAINED` | `internal/mcp/bridge/bridge.go:801`; `docs/MCP-SERVER.md:145` |
| LRF edge tuple | `outcome` | `cem-lexical-v0` / `lexically-proximate-candidate`, `rejected`, `abstained` | `docs/specs/lexical-relevance-floor-v0.md:267,270`; `internal/lrf/evaluate.go:195,206` |
| TCQ claim result | `associationState` | `ASSOCIATED`, `ABSTAINED` | `docs/tcq-0.schema.json:124`; `docs/specs/test-claim-qualification-v0.md:116` |
| TCQ claim result | `hygieneState` | (independent axis, same result object) | `docs/specs/test-claim-qualification-v0.md:111` |

LRF is the only surface that distinguishes **abstained** from **rejected**, and the distinction is
normative for the whole vocabulary: `abstained` means the inputs could not be evaluated and the
subject stays unknown (`docs/specs/lexical-relevance-floor-v0.md:65-67`); `rejected` means the
inputs were evaluable and did not meet the floor (`:68`). *Proposal:* every profile that today has
only a two-value abstention field states which of the two LRF senses its `ABSTAINED` carries, or
splits it. MCP `ABSTAINED` currently carries both senses (`docs/MCP-SERVER.md:103-105`).

### `NONE`

**Normative definition.** Absence of any authority root. It is a value of the `authorityClass` axis
only. It is legal only alongside an active abstention: the MCP bridge rejects any result that pairs
`NONE` with a non-abstaining state (`internal/mcp/bridge/bridge.go:815-816`).

**Declared variants.** `internal/dashboard/model/types.go:45`;
`internal/mcp/bridge/bridge.go:803`; `docs/specs/change-frontier-v0.md:244`.

### `UNKNOWN`

**Normative definition.** A bounded proposition that obligation verification neither proved,
refuted, nor found conflicting evidence for (`docs/ARCHITECTURE.md:85-86`). `UNKNOWN` is a useful
result and MUST never be converted into `cannot happen` (`docs/ARCHITECTURE.md:200-201`).

**Declared variants.** As a proposition verdict alongside `PROVED`, `REFUTED`, `CONFLICTED`
(`docs/ARCHITECTURE.md:85`). As a `completeness` value meaning the denominator is not closed over
the cohort (`docs/specs/local-observability-dashboard-v0.md:80`;
`internal/dashboard/model/types.go:51-55`). As a `currency` value
(`docs/specs/local-observability-dashboard-v0.md:81`).

*Current state, unresolved:* the proposition-verdict `UNKNOWN` and the completeness `UNKNOWN` are
different axes carrying the same token, the same collision as `ADVISORY`. The same reading rule
applies: name the axis.

### `NOT_OBSERVED`

**Normative definition.** The value was not measured. It is a value of the `epistemicClass` axis.
It is distinct from a measured zero: missing, invalid, inaccessible, unsupported, disabled,
expired, or unretained input produces `NOT_OBSERVED`, `PARTIAL`, or `INVALID`, never zero
(`docs/specs/local-observability-dashboard-v0.md:90-92`).

**Declared variants.** `internal/dashboard/model/types.go:33`;
`internal/mcp/bridge/bridge.go:661,802`.

### `NOT_RUN`

**Normative definition.** A named gate, trial, or promotion step has not been executed. It
describes **delivery evidence about Corvint itself**, never a value inside a receipt. `NOT_RUN`
evidence stays visible and is never converted into a pass
(`docs/SPEC-DRIVEN-DEVELOPMENT.md:97@ef8f012f`).

**Declared variants.** Used throughout the specification index and candidate profiles as a delivery
state (`docs/specs/README.md:27,29,31,37,38,40,41`;
`docs/specs/analyzer-candidate-profiles.md:412@2f2614b9`;
`docs/specs/analyzer-candidate-profiles.md:500@6424bf91`). It is also used once for a metric denominator
— a zero positive denominator is `NOT_RUN`
(`docs/specs/lexical-relevance-floor-v0.md:808`) — which is the receipt sense this definition
excludes. *Proposal:* that one use adopts `completeness: UNKNOWN` instead.

### `UNAVAILABLE`

**Normative definition.** The operation exists in the contract but no accepted authority backs it at
this revision, so it produces no result and cannot block. Distinct from `NOT_OBSERVED` (which is a
measurement outcome) and from `NOT_RUN` (which is delivery evidence).

**Declared variants.** `docs/specs/agent-harness-integration-v0.md:67@ab0adf38` — until a separately accepted
closing authority exists, `stop` returns `frontier.state: UNAVAILABLE` with
`shouldContinue: false`. `docs/specs/analyzer-candidate-profiles.md:68@e7dc63fe` and
`docs/specs/analyzer-candidate-profiles.md:202@88e7795c` use the compound
`EXACT_BINDING_UNAVAILABLE` for the same shape at binding scope.

### `CALLER_REPORTED`

**Normative definition.** Integrity-bound caller input without an independent execution authority
root (`docs/specs/change-frontier-v0.md:76`). It is evidence for review and is mechanically
non-closing: it cannot remove a frontier item
(`docs/specs/test-claim-qualification-v0.md:43`; `docs/TEST-CLAIM-QUALIFICATION.md:16`).

**Declared variants.** `internal/dashboard/model/types.go:43`;
`docs/specs/change-frontier-v0.md:244`; `docs/tcq-0.schema.json:125`;
`docs/specs/verification-planner-observer-v0.md:285`. The four agree; this term is not in drift.

### `PRODUCER_DECLARED`

**Normative definition.** The producing profile declared the value from its own deterministic
evaluation of admitted inputs, with no independent verifier or execution root confirming it. It is
stronger than caller input, because the producer is a Corvint profile bound to a frozen contract, and
weaker than an owning verifier, because nothing outside the producer checked it.

**Declared variants.** `docs/specs/change-frontier-v0.md:166,179,183,186,244` (SCREAMING_SNAKE);
`internal/lrf/types.go:12`, `docs/lrf-0.schema.json:65,69`,
`docs/specs/lexical-relevance-floor-v0.md:267,269` (lowercase-kebab `producer-declared`). Same
concept, two casings.

### `CORVINT_PROCESS_OBSERVED`

**Current state.** Declared only at `docs/specs/verification-planner-observer-v0.md:389`. Its
meaning is fixed by VPO-V0-029 (`:452-456`): Corvint observed the declared direct child, available
group/job events, exit, and streamed byte digests. With an unqualified executable or containment
class it does not assert exact executed bytes or escaped-descendant absence, and it never asserts
command truth, hermeticity, network denial, provider qualification, all-tests-pass, source safety,
frontier closure, or mergeability. `mergeAuthority` is always false
(`:456`). `PROVIDER_UNQUALIFIED` is permanently in that profile's unknown set (`:448`).

**Proposal: retire it from the `authorityClass` axis.** See
[Proposed `authorityClass` migration](#proposed-authorityclass-migration) for the reasoning and the
replacement encoding.

### `declared universe`

**Normative definition.** One independently supplied expected base and target, their canonical CEM
0.2 patch and exclusion, and one exact OCM intent scope
(`docs/specs/change-frontier-v0.md:64-65`). Every absence claim Corvint makes is bounded by exactly
one declared universe.

**Owning contract.** `docs/specs/change-frontier-v0.md:64-65`. The analogous construct at repository
scope is the negative-scope certificate (`docs/ARCHITECTURE.md:79-82`), which pins revision, roots,
file classes, adapters, query semantics, exclusions, budgets, and unsupported surfaces.

### `verified absence`

**Normative definition.** The bounded result that no qualifying relation was accepted for one named
obligation from the declared, verified inputs
(`docs/specs/change-frontier-v0.md:70-71`). It proves only `no qualifying witness found in this
declared universe`; it never proves a global non-capability (`docs/ARCHITECTURE.md:81-82`).

### `qualifying relation`

**Normative definition.** A relation that the owning profile explicitly permits to remove its exact
frontier item (`docs/specs/change-frontier-v0.md:68-69`). Whether a relation qualifies is a property
of the owning profile, not of the relation's authority: TCQ's `test-report-matched-v0` is
explicitly non-closing under default frontier policy despite being a real observation
(`docs/specs/change-frontier-v0.md:57-58`).

### `frontier`

Three distinct concepts currently share this word. See
[Frontier disambiguation](#frontier-disambiguation).

### `mergeAuthority`

**Normative definition.** A boolean stating whether a receipt may contribute to a merge decision.
It is always `false` in Verification Planner/Observer V0
(`docs/specs/verification-planner-observer-v0.md:456`). It is not an authority *class*; it
is a separate permission field and MUST NOT be derived from `authorityClass` rank.

## Proposed `authorityClass` lattice

*Proposal. Not ratified.* One value set, one casing (SCREAMING_SNAKE), one declared total order.
Ranks are ordinal labels spaced by ten so a future member can be inserted without renumbering.

| Rank | Value | Authority root |
|---:|---|---|
| 70 | `REPOSITORY_ACCEPTED` | The repository accepted the declaration at that revision: a merged specification, decision, policy, or owner record with the declared status and scope. |
| 60 | `OWNING_VERIFIER` | The exact verifier that owns the artifact validated it, and it passed. A failed owning verifier does not lower the class; it makes the source `INVALID` (`docs/specs/local-observability-dashboard-v0.md:86`). |
| 50 | `PROVIDER_QUALIFIED` | A qualified execution or evidence provider, identified and admitted under its own profile, attested the value. |
| 40 | `ADAPTER_QUALIFIED` | A registered, compiled Corvint adapter produced the value from admitted bytes. |
| 30 | `PRODUCER_DECLARED` | The producing Corvint profile declared it from its own deterministic evaluation, with no independent root. |
| 20 | `CALLER_REPORTED` | Integrity-bound caller input with no independent execution authority root. |
| 10 | `ADVISORY` | Derived from non-authoritative evidence. Never contributes to closure or to a success numerator. |
| 0 | `NONE` | No authority root. Legal only alongside an active abstention. |

Ranks 70–40, 20, 10, and 0 are the dashboard set unchanged
(`docs/specs/local-observability-dashboard-v0.md:79`) in its declared order (`:84`). Rank 30 is the
single new member, admitted because `PRODUCER_DECLARED` is already shipped in two profiles
(`docs/specs/change-frontier-v0.md:244`; `internal/lrf/types.go:12`) and has no dashboard
equivalent.

## Proposed `authorityClass` migration

*Proposal. Not ratified. This is a mapping table only; no implementation is specified.*

| Current value | Surface | Maps to | Rank | Reason |
|---|---|---|---:|---|
| `REPOSITORY_ACCEPTED` | dashboard | `REPOSITORY_ACCEPTED` | 70 | Identity. |
| `OWNING_VERIFIER` | dashboard | `OWNING_VERIFIER` | 60 | Identity. |
| `PROVIDER_QUALIFIED` | dashboard | `PROVIDER_QUALIFIED` | 50 | Identity. |
| `ADAPTER_QUALIFIED` | dashboard | `ADAPTER_QUALIFIED` | 40 | Identity. |
| `CALLER_REPORTED` | dashboard, frontier, TCQ, VPO | `CALLER_REPORTED` | 20 | Identity; all four already agree. |
| `ADVISORY` | dashboard | `ADVISORY` | 10 | Identity. Axis must be named at every use; see [`ADVISORY`](#advisory). |
| `NONE` | dashboard, MCP, frontier | `NONE` | 0 | Identity; all three already agree. |
| `PRODUCER_DECLARED` | frontier | `PRODUCER_DECLARED` | 30 | Identity. |
| `producer-declared` | LRF | `PRODUCER_DECLARED` | 30 | Casing normalisation only. The LRF definition — a producer-evaluated edge that witnesses only a typed lexical relation and never closes an obligation (`docs/specs/lexical-relevance-floor-v0.md:57-60,69`) — is exactly rank 30. |
| `GIT_REPOSITORY` | MCP | `OWNING_VERIFIER` | 60 | The authority root is Git's content-addressed object store, read directly by Corvint. That store is the exact verifier that owns repository identity and revision facts, and it validated them. It is **not** `REPOSITORY_ACCEPTED`: nothing about a revision hash is an accepted declaration, and mapping it to rank 70 would let a commit SHA outrank an accepted specification. |
| `REPOSITORY_EVIDENCE` | MCP | `ADAPTER_QUALIFIED` | 40 | The value is compiled by Corvint analyzers and adapters from repository bytes at a pinned revision. The receipt binding that the bridge enforces (`internal/mcp/bridge/bridge.go:813`) proves revision binding, not that each cited artifact's own owning verifier ran. Rank 40 is the honest floor. The name's resemblance to `REPOSITORY_ACCEPTED` is a false cognate and must not drive the mapping. |
| `CORVINT_PROCESS_OBSERVED` | VPO | `ADAPTER_QUALIFIED` (retire the token) | 40 | See below. |

**`CORVINT_PROCESS_OBSERVED` — retire.** The token names an *observation mechanism*, not an authority
root, so it sits on the wrong axis. Its content is already carried by two other axes that the
dashboard contract requires to stay independent: `epistemicClass: OBSERVED`
(`docs/specs/local-observability-dashboard-v0.md:78`) and the profile's own `containmentClass`
(`docs/specs/verification-planner-observer-v0.md:392`). Keeping it as an `authorityClass` value
collapses epistemic class into authority, which
`docs/specs/local-observability-dashboard-v0.md:72-73` forbids in exactly those words. On the authority
axis a VPO observation is the output of Corvint's own compiled process observer, which is rank 40; it
can never be rank 50, because VPO-V0-028 places `PROVIDER_UNQUALIFIED` permanently in that profile's
unknown set (`docs/specs/verification-planner-observer-v0.md:448`) and VPO-V0-029 explicitly
disclaims provider qualification (`:454-456`).

**Ordering safety.** No mapping above inverts an ordering that any surface declares today. The MCP
bridge never compares `GIT_REPOSITORY` against `REPOSITORY_EVIDENCE`: each is a per-tool constant
(`internal/mcp/bridge/bridge.go:811,813`). The dashboard order is preserved exactly. The only newly
declared relations are those involving rank 30, which no surface previously ordered.

## Proposed comparison rules

*Proposal. Not ratified.*

1. **`authorityClass` is totally ordered** by the rank column. For any two values `a` and `b` from
   the lattice, exactly one of `rank(a) < rank(b)`, `rank(a) = rank(b)`, `rank(a) > rank(b)` holds.
2. **Comparison is legal only after normalisation.** Two `authorityClass` values from different
   surfaces MAY be compared if and only if both have been mapped onto this lattice. Comparing a raw
   `producer-declared` against a raw `ADAPTER_QUALIFIED` is undefined and MUST be refused, not
   guessed.
3. **Ranks are ordinal.** `min`, `max`, and threshold tests (`rank(a) >= rank(threshold)`) are the
   only legal operations. Differences, sums, averages, and ratios of ranks have no meaning and MUST
   NOT be computed.
4. **Derivation takes the weakest contributor.** A value derived from several sources carries the
   minimum rank among them (`docs/specs/local-observability-dashboard-v0.md:84-85`).
5. **Cross-axis comparison is illegal.** `authorityClass` MUST NOT be compared against, substituted
   for, or collapsed into `epistemicClass`, `validity`, `completeness`, `currency`, or
   `deliveryStage` (`docs/specs/local-observability-dashboard-v0.md:72-73`). A token that appears on
   two axes — `ADVISORY`, `UNKNOWN` — is two different values and the axis must be named.
6. **Rank is not a verdict.** A higher rank never means a proposition is true
   (`docs/ARCHITECTURE.md:87`), never means a test passed, and never confers merge permission; see
   [`mergeAuthority`](#mergeauthority).
7. **Rank does not confer closure.** Whether a relation may remove a frontier item is decided by the
   owning profile, not by rank (`docs/specs/change-frontier-v0.md:68-69`). A rank-60 relation that
   its profile declares non-closing still leaves the item open.
8. **Cross-universe comparison is illegal.** Two values bounded by different declared universes
   (`docs/specs/change-frontier-v0.md:64-65`) or different negative-scope certificates
   (`docs/ARCHITECTURE.md:79-82`) describe different questions and MUST NOT be ordered against each
   other. Normalisation makes values commensurable in vocabulary, not in scope.
9. **`NONE` requires an abstention.** Emitting rank 0 alongside a non-abstaining state is invalid
   (`internal/mcp/bridge/bridge.go:815-816`).

## Frontier disambiguation

**Current state.** The word carries three scopes with no distinguishing qualifier.

| Scope | Meaning as used | Citation |
|---|---|---|
| One query | The bounded set of unknowns returned with a single abstaining answer: "abstain with a precise unknown frontier". | `docs/PRODUCT.md:13`; `docs/PRODUCT.md:229` |
| One change / one certificate | The unresolved obligations of one exact change in one declared universe (`frontier/0`, `corvint frontier`), and the complement of one negative-scope certificate's declared universe: "Anything outside the certificate remains in the unknown frontier". | `docs/specs/change-frontier-v0.md:64,66-67`; `docs/PRODUCT.md:22,46`; `docs/ARCHITECTURE.md:82` |
| One repository | The standing set of reachable code, declared behavior, acceptance criteria, and changed surfaces for which no applicable proving claim exists. | `docs/ARCHITECTURE.md:198-201`; `docs/ARCHITECTURE.md:180` ("known frontier") |

The first two are enumerable and bounded by exactly one declared input. The third is a standing
property of the repository, is not enumerable as items, and is also a programme name
(`docs/specs/README.md:22`, "Unknown Frontier outcome gate"). `docs/ARCHITECTURE.md:82` and `:198`
are not the same set either: `:82` is the complement of one certificate, `:198` is the repository
residue.

**Proposal: the frontier scope-qualifier rule.** *Not ratified.*

- The bare word `frontier` is prohibited in normative text. Every use carries exactly one scope
  qualifier from the closed set `query`, `change`, `repository`.
- **query frontier** — the unknown set returned with one abstaining answer. Replaces the usage at
  `docs/PRODUCT.md:13,229`.
- **change frontier** — the unresolved obligations of one change in one declared universe. Already
  the name of the owning contract (`docs/specs/change-frontier-v0.md`, profile `frontier/0`); it
  additionally absorbs the certificate-complement usage at `docs/ARCHITECTURE.md:82`, which is the
  same shape at certificate rather than change scope.
- **repository unknown frontier** — the standing residue at `docs/ARCHITECTURE.md:198`. Reserved
  also for the programme name.
- `empty frontier` and `open frontier` (`docs/specs/change-frontier-v0.md:72-73`) are properties of
  a change frontier result only, and stay unqualified because they are already scoped by it.
- The qualifier is part of the term, not decoration: a document that cannot name the scope of a
  frontier it is describing has not determined which one it means.

## How to add a term

*Proposal. Not ratified.*

1. Before introducing a term in a new specification, search this file. If the term is here, cite it
   (`docs/GLOSSARY.md#term`) and do not restate the definition. A specification that restates a
   definition creates a second owner and a future drift.
2. If the term is not here, add it here first, in the same change that introduces it. The entry
   states the one normative definition, the owning contract, and every location that declares a
   variant, each with `file:line`.
3. A new value in an existing closed set is added to that term's entry, not declared only in the
   consuming spec. A new `authorityClass` member additionally states its rank and its position
   relative to both neighbours, and why nothing already in the lattice covers it.
4. Casing is SCREAMING_SNAKE for closed enumeration members on the wire, and lowercase-kebab only
   where a frozen profile already ships it. A new profile does not introduce a second casing for an
   existing term.
5. A term whose meaning depends on scope carries a scope qualifier from a closed set, as `frontier`
   does. Do not ship a term that reads correctly only when the reader already knows which scope is
   meant.
6. When a term's definition changes, update the entry and every cited variant in the same change. An
   entry with a stale `file:line` is a defect in this file.
