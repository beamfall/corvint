# Change Frontier V0

Owner: Russell Lewis
Frozen: 2026-08-23
Amendments: `CF-V0-031` and `CF-V0-032` ratified accepted 2026-08-29 under repository-owner
delegation; see
[`../decisions/0006-caller-authored-authority.md`](../decisions/0006-caller-authored-authority.md).
Decision 0012 accepts the `CF-V0-004`/`CF-V0-018` unreachability amendments and `CF-V0-030`'s
candidate-only/deferred-replay policy on 2026-09-01. Ratification is limited to those amendments and
clauses; every other clause keeps the document's proposed status. Decision 0162 (2026-09-12) adds
`CF-V0-033`, the repository read-only requirement the already-shipped `corvint frontier` verb and
`internal/frontier` already satisfy.
Intent status: proposed overall; accepted clauses/amendments: `CF-V0-004`/`CF-V0-018` unreachability and `CF-V0-030` candidate-only replay policy (2026-09-01), `CF-V0-031`/`CF-V0-032` (2026-08-29; `CF-V0-032` uncertainty wording 2026-09-12, decision 0101), `CF-V0-033` read-only guarantee (2026-09-12, decision 0162)
Delivery status: experimental (candidate-only; no promotion or outcome claim).
`CF-V0-030` holds this field at `experimental` candidate-only until implementation, canonical vectors,
adversarial fixtures, full local gate, and independent review exist.
As of 2026-08-29 the first three exist -- `internal/frontier` and `internal/wp3codec` implement
clauses `CF-V0-001..026`, and `conformance/frontier-v0` carries 62 vectors and 28 adversarial fixtures
authored independently of the implementation under `CF-V0-028`. The remaining local evidence is
incomplete. The former sealed-cohort outcome gates are withdrawn by decision 0012; a replacement
third-party-PR replay protocol is deferred until the first release cut and remains `NOT_RUN`. Two
surfaces are still injected seams rather than Git-backed implementations, so `CF-V0-021` stage 5 is
partial.
Authoritative inputs: `docs/PRODUCT.md`,
`docs/specs/cem-0.2-canonical-binding.md`, `docs/specs/ocm-v0-dogfood.md`,
`docs/specs/lexical-relevance-floor-v0.md`, `docs/specs/test-claim-qualification-v0.md`

## Agent digest
- Claim: Change Frontier composes CEM, OCM, LRF, and TCQ into a bounded Git-pinned queue of obligations that still lack accepted witnesses.
- Status: proposed overall; accepted clauses/amendments: `CF-V0-004`/`CF-V0-018` unreachability and `CF-V0-030` candidate-only replay policy (2026-09-01), `CF-V0-031`/`CF-V0-032` (2026-08-29; `CF-V0-032` uncertainty wording 2026-09-12, decision 0101), `CF-V0-033` read-only guarantee (2026-09-12, decision 0162)/experimental (candidate-only; no promotion or outcome claim).
- Exists: `internal/frontier`, `internal/frontierrepo`, and `conformance/frontier-v0`.
- Blocked on: local implementation/review completion; the third-party-PR replay protocol is deferred to the first release cut and remains `NOT_RUN`.
- Read next: `change-frontier-profile-1.md` and `verified-absence-frontier-v0.md`.

## User and job

For one exact change and one pinned intent scope, a maintainer or agent needs one bounded answer to:
which declared obligations still lack an accepted witness, and what exact class of action can move
each one forward?

Change Frontier V0 (`frontier/0`) is the deterministic composition of canonical CEM 0.2, OCM 0.1,
Lexical Relevance Floor V0, and Test Claim Qualification V0. It is a local **frontier preview** and
review queue. It is not a delivered Stop hook. In particular, TCQ V0 is caller-reported and cannot
close a test obligation, so a linked OCM obligation always leaves an `INTENT_TEST` item in this
profile.

V0 reports absence only inside the exact declared universe. It does not search for undisclosed
evidence, prove that evidence does not exist, establish behavior, judge correctness, or claim
whole-program completeness.

## Verified current state

- CEM 0.2 supplies canonical base-to-target patch authority and exact sidecar exclusion.
- OCM 0.1 supplies the complete pinned intent-obligation set and structural hunk/claim links, but
  `linked` is not semantic or execution closure.
- LRF V0 may qualify a CEM hunk-basis edge as `cem-lexical-v0`; its OCM result remains only a
  `lexically-proximate-candidate` and never witnesses or closes an intent obligation.
- TCQ V0 deterministically associates and classifies selected test claims and exact JUnit rows, but
  every edge has `authorityClass: CALLER_REPORTED`. Its `test-report-matched-v0` relation is
  explicitly non-closing under default frontier policy.
- No accepted frontier wire, runtime, authenticated harness relation, acknowledgement authority, or
  Stop-hook integration exists.

## Definitions

- **declared universe**: one independently supplied expected base and target, their canonical CEM
  0.2 patch and exclusion, and one exact OCM intent scope;
- **frontier item**: one unresolved hunk-basis, intent-change, or intent-test obligation in that
  universe;
- **qualifying relation**: a relation that the owning profile explicitly permits to remove its exact
  frontier item;
- **verified absence**: the bounded result that no qualifying relation was accepted for one named
  obligation from the declared, verified inputs;
- **empty frontier**: a valid result with no items;
- **open frontier**: a valid result with at least one item;
- **operational failure**: invalid, stale, mismatched, unsupported, unavailable, or exhausted input;
  it emits no frontier result;
- **caller-reported**: integrity-bound caller input without an independent execution authority root.

## Requirements

### Input and state boundary

- `CF-V0-001`: one invocation MUST accept exactly one verified `cem/0.2` artifact under its canonical
  patch-binding workflow, exactly one canonical `ocm/0.1-experimental` artifact bound to it, one
  independent expected-base input, one independent target input, and one validated repository
  boundary. CEM 0.1, OCM bound to CEM 0.1, multiple repositories, multiple intent scopes, or inferred
  revision authority fail `unsupported-frontier-context`.
- `CF-V0-002`: the implementation MUST pass the same bounded immutable CEM and OCM byte copies and
  independent revisions through the shared OCM-consuming CEM 0.2 verifier. It MUST recompute LRF and
  TCQ from those verified inputs and target Git objects. A caller-supplied LRF or TCQ result is never
  accepted as authority or as a shortcut.
- `CF-V0-003`: optional dynamic test input is exactly the all-or-none tuple `(canonical command,
  canonical observation, raw JUnit report)`. With all three absent, Frontier computes static TCQ and
  records `testMode: STATIC`. With all three present, it performs complete TCQ dynamic verification
  and records `testMode: DYNAMIC_CALLER_REPORTED`. Every partial combination fails
  `invalid-frontier-input`. Both modes always bind the recomputed TCQ ID.
- `CF-V0-004`: valid global states are exactly `frontierState: EMPTY|OPEN`. `EMPTY` requires exact
  `items: []`; `OPEN` requires at least one item. No other combination is valid. Exit `0` means valid
  empty, exit `1` means valid open, and exit `2` means operational failure with no Frontier JSON on
  stdout. V0 has no stop-decision field or hook authority.
  `EMPTY` and exit `0` are unreachable through the V0 entry point: canonical OCM rejects an empty
  requirement enumeration, and every accepted obligation emits an item under `CF-V0-011`,
  `CF-V0-012`, or `CF-V0-015`. The `EMPTY` wire shape remains a canonical verification/refusal
  case only; this amendment changes no `internal/frontier` behavior. A V0 implementation MUST still
  reject any other state combination, and MUST NOT repurpose `EMPTY` or exit `0` for another meaning.
- `CF-V0-005`: V0 policy is the literal `strict-v0`. It has no acknowledgement, exception, score,
  confidence, or permissive-success input. OCM `unknown` is an unresolved producer disposition, not
  a policy acknowledgement. `READY`, `NEEDS_WIDENING`, and `OUT_OF_SCOPE` are retrieval-packet terms
  and MUST NOT appear in the frontier wire.

### Stable universe and item identity

- `CF-V0-006`: the universe ID is:

  ```text
  "frontier-universe:sha256:" +
  SHA-256(UTF8("corvint-frontier-universe/0") || 0x00 || codec({
    "baseRevision":baseRevision,
    "excludedPath":".corvint/change.cem.json",
    "intent":{
      "blobOid":intentBlobOid,
      "path":intentPath,
      "span":{"end":decimalIntentEnd,"start":decimalIntentStart},
      "spanSha256":intentSpanSha256
    },
    "objectFormat":objectFormat,
    "patchSha256":patchSha256,
    "targetRevision":targetRevision
  }))
  ```

  `objectFormat` is exactly `sha1|sha256`; both intent offsets are canonical decimal strings under
  the exact `codec` in `CF-V0-019`. Map, LRF, TCQ, command, observation, and report identities do not
  enter the universe ID because they are attempts to satisfy one stable obligation universe.
  Identical Git content across clones intentionally has identical identity. A Frontier document
  verifier MUST rederive this ID from the emitted scope and require exact equality; validating only
  the prefix and digest grammar is insufficient.
- `CF-V0-007`: item kinds are exactly `HUNK_BASIS`, `INTENT_CHANGE`, and `INTENT_TEST`. Subject ID is
  the hunk ID for `HUNK_BASIS` and the obligation ID for both intent kinds. Item ID is:

  ```text
  "frontier-item:sha256:" +
  SHA-256(UTF8("corvint-frontier-item/0") || 0x00 || codec({
    "kind":kind,
    "subjectId":subjectId,
    "universeId":universeId
  }))
  ```

  Evidence repair may remove an item but cannot change the identity of an unresolved obligation in
  the same universe. A Frontier document verifier MUST rederive every item ID from its emitted kind,
  subject ID, and the reverified universe ID and require exact equality.

### Exact closure and non-closure

- `CF-V0-008`: a verified CEM hunk emits no `HUNK_BASIS` item only when its disposition is a
  mechanically reverified `whitespace-only|line-ending-only`, or at least one of its LRF basis edges
  emits `cem-lexical-v0`. A CEM `supported` disposition alone never closes it. A qualifying basis is
  existential: rejected extra bases remain in the recomputed LRF diagnostics but do not reopen the
  hunk or enter a blocking frontier item.
- `CF-V0-009`: a CEM `unknown` hunk emits exactly one `HUNK_BASIS` item. Its exact reason maps are
  `no-evidence -> HUNK_NO_EVIDENCE`, `insufficient-evidence -> HUNK_INSUFFICIENT_EVIDENCE`, and
  `conflicting-evidence -> HUNK_CONFLICTING_EVIDENCE`. It has `authorityClass: NONE`, empty
  `relatedIds`, `resolutionClass: ACTIONABLE`, and `nextAction: SUPPLY_HUNK_BASIS`.
- `CF-V0-010`: a supported hunk without a qualifying LRF basis emits exactly one `HUNK_BASIS` item.
  Deletion abstention maps to `DELETION_RELATION_REQUIRED`; local term-bound abstention maps to
  `SUBJECT_TERM_BOUND_EXCEEDED`. Remaining failed basis reasons are the unique union across every
  rejected or term-bound-abstained basis, stored in exact order `SUBJECT_TERM_BOUND_EXCEEDED`, `EVIDENCE_SPAN_TOO_BROAD`,
  `SELF_REFERENTIAL_BASIS`, `INSUFFICIENT_LEXICAL_SUPPORT`. Its `authorityClass` is exactly
  `PRODUCER_DECLARED`; its `relatedIds` are exactly the sorted unique evidence IDs whose LRF edge
  contributes a retained reason. A deletion or local term-bound abstention uses `PROFILE_REQUIRED`
  and `DEFINE_SUPPORTED_PROFILE`; every other such item uses `ACTIONABLE` and
  `NARROW_OR_REPLACE_BASIS`. These hunk reasons have one action class, so later retained reasons are
  diagnostics rather than an ignored competing action.
- `CF-V0-011`: an OCM `unknown` obligation emits exactly one `INTENT_CHANGE` item and no
  `INTENT_TEST` item because no selected test edge exists. Its exact reason maps are
  `unassessed -> OBLIGATION_UNASSESSED`, `no-test-claim -> OBLIGATION_NO_TEST_CLAIM`,
  `insufficient-evidence -> OBLIGATION_INSUFFICIENT_EVIDENCE`, and
  `conflicting-evidence -> OBLIGATION_CONFLICTING_EVIDENCE`. It uses `authorityClass: NONE`, empty
  `relatedIds`, `ACTIONABLE`, and `LINK_OBLIGATION`.
- `CF-V0-012`: every structurally linked OCM obligation emits exactly one `INTENT_CHANGE` item.
  When one or more referenced hunk edges emit LRF `lexically-proximate-candidate`, the item uses
  reason `LEXICAL_CANDIDATE_NONCLOSING`, `authorityClass: PRODUCER_DECLARED`, `relatedIds` equal to
  exactly the sorted unique candidate hunk IDs, `AUTHORITY_REQUIRED`, and
  `ESTABLISH_CHANGE_WITNESS`. The candidate is a retrieval aid, never a witness. With no candidate,
  the item instead uses reason `SUBJECT_TERM_BOUND_EXCEEDED` when any referenced OCM hunk edge
  abstains for that issue, the exact sorted unique set of affected hunk IDs, `PRODUCER_DECLARED`,
  `PROFILE_REQUIRED`, and `DEFINE_SUPPORTED_PROFILE`. Otherwise the item uses reason
  `NO_MATERIAL_CHANGE_WITNESS`, the exact sorted unique set of every
  structurally referenced hunk ID in `relatedIds`, `PRODUCER_DECLARED`, `ACTIONABLE`, and
  `LINK_MATERIAL_HUNK`. A future separately accepted relation and new Frontier profile may close
  intent change; V0 cannot. Each obligation is evaluated independently even when hunks are shared.
- `CF-V0-013`: V0 defines no test-claim quantifier because no TCQ V0 relation closes a test item.
  Every selected claim edge is evaluated independently and its mapped diagnostics participate in the
  reason union. `ANY`, `ALL`, alternative groups, and required groups require an OCM contract and a
  new Frontier profile; V0 MUST NOT infer them.
- `CF-V0-014`: `tcq/0` defines no qualifying frontier test relation. Its
  `test-report-matched-v0` plus `CALLER_REPORTED` emits `CALLER_REPORTED_NONCLOSING`; it never removes
  an `INTENT_TEST` item. Signatures over caller-controlled V0 bytes, clean-target statements, passing
  rows, and exit zero cannot upgrade that authority. A stronger harness relation requires a new TCQ
  and Frontier profile and MUST NOT reinterpret V0 bytes.
- `CF-V0-015`: every structurally linked obligation emits exactly one `INTENT_TEST` item in Frontier
  V0. Its `authorityClass` is exactly `CALLER_REPORTED` in both static and dynamic modes, matching
  every TCQ V0 selected edge. Its `relatedIds` are exactly all OCM-selected claim IDs for that
  obligation, sorted and deduplicated, whether their TCQ edge associates, abstains, matches, or
  fails. All selected claim edges are evaluated, and every applicable mapped reason is retained even
  though only the highest-priority reason selects the next action.

### Reason, action, and wire contracts

- `CF-V0-016`: `INTENT_TEST` reason priority is the frozen order below. Reasons are unique and stored
  in this order, not lexical order, after taking the union across every selected claim edge.
  `HUNK_BASIS` uses the exact order in `CF-V0-010`; `INTENT_CHANGE` selects
  `SUBJECT_TERM_BOUND_EXCEEDED` before its two ordinary reasons and has exactly one reason.
  Recomputed LRF and TCQ retain nonblocking edge diagnostics when an existential witness succeeds;
  Frontier neither discards nor promotes those diagnostics.

  | Order | Reason | Resolution | Next action |
  |---:|---|---|---|
  | 1 | `CALLER_REPORTED_NONCLOSING` | `AUTHORITY_REQUIRED` | `ESTABLISH_HARNESS_AUTHORITY` |
  | 2 | `TARGET_CLEANLINESS_NOT_ATTESTED` | `ACTIONABLE` | `RERUN_TEST_COMMAND` |
  | 3 | `COMMAND_FAILED` | `ACTIONABLE` | `RERUN_TEST_COMMAND` |
  | 4 | `TEST_ERROR` | `ACTIONABLE` | `FIX_OR_RERUN_TEST` |
  | 5 | `TEST_FAILED` | `ACTIONABLE` | `FIX_OR_RERUN_TEST` |
  | 6 | `TEST_SKIPPED` | `ACTIONABLE` | `FIX_OR_RERUN_TEST` |
  | 7 | `TEST_IDENTITY_AMBIGUOUS` | `ACTIONABLE` | `DISAMBIGUATE_TEST_IDENTITY` |
  | 8 | `TEST_ROW_IDENTITY_UNAVAILABLE` | `ACTIONABLE` | `SUPPLY_TEST_OBSERVATION` |
  | 9 | `TEST_NOT_MATCHED` | `ACTIONABLE` | `SUPPLY_TEST_OBSERVATION` |
  | 10 | `TEST_CLAIM_EMPTY` | `ACTIONABLE` | `REPAIR_TEST_CLAIM` |
  | 11 | `TEST_CLAIM_UNCONDITIONAL_SKIP` | `ACTIONABLE` | `REPAIR_TEST_CLAIM` |
  | 12 | `TEST_CLAIM_UNASSOCIATED` | `ACTIONABLE` | `REPAIR_TEST_CLAIM` |
  | 13 | `TEST_CLAIM_UNSUPPORTED` | `PROFILE_REQUIRED` | `DEFINE_SUPPORTED_PROFILE` |

  TCQ mapping is exact: `target-cleanliness-not-attested` maps to order 2; `command-failed` to 3;
  `test-error` to 4; `test-failed` to 5; `test-skipped` to 6;
  `execution-identity-ambiguous|repeated-test-rows` to 7; `row-identity-unavailable` to 8;
  `test-not-matched` to 9; `empty-body` to 10; `unconditional-skip` to 11;
  `claim-association-missing|claim-association-ambiguous|python-offset-mismatch|unparseable-test-unit`
  to 12; and `unsupported-anchor-profile|unsupported-python-grammar` to 13. A TCQ relation maps to
  order 1. The first retained reason supplies `resolutionClass` and `nextAction`; later reasons stay
  visible and cannot alter them.
- `CF-V0-017`: each item has exactly this closed shape; no field is optional:

  ```json
  {"authorityClass":"CALLER_REPORTED","id":"frontier-item:sha256:0000000000000000000000000000000000000000000000000000000000000000","kind":"INTENT_TEST","nextAction":"SUPPLY_TEST_OBSERVATION","reasons":["TEST_NOT_MATCHED"],"relatedIds":["claim:sha256:0000000000000000000000000000000000000000000000000000000000000000"],"resolutionClass":"ACTIONABLE","subjectId":"CF-V0-001"}
  ```

  `authorityClass` is exactly `NONE|PRODUCER_DECLARED|CALLER_REPORTED`.
  `resolutionClass` is exactly `ACTIONABLE|AUTHORITY_REQUIRED|PROFILE_REQUIRED`. `relatedIds` uses
  only verified evidence, hunk, or claim IDs and its membership is fixed per kind: CEM unknown is
  empty; unresolved supported CEM uses `CF-V0-010`; OCM unknown is empty; linked intent change uses
  `CF-V0-012`; and linked intent test uses `CF-V0-015`. No reason-priority choice may change that
  membership. A verifier MUST enforce the exact authority class that `CF-V0-009..012` and
  `CF-V0-015` assign to the item's kind and retained reason; accepting another value merely because
  it belongs to the authority enum is noncanonical.
- `CF-V0-018`: every complete result has exactly this top-level shape. This example is valid open
  output and deliberately contains one item:

  ```json
  {"frontierState":"OPEN","id":"frontier:sha256:0000000000000000000000000000000000000000000000000000000000000000","inputs":{"cemSha256":"0000000000000000000000000000000000000000000000000000000000000000","lrfSha256":"0000000000000000000000000000000000000000000000000000000000000000","ocmSha256":"0000000000000000000000000000000000000000000000000000000000000000","policy":"strict-v0","tcqId":"tcq:sha256:0000000000000000000000000000000000000000000000000000000000000000","testMode":"STATIC"},"items":[{"authorityClass":"CALLER_REPORTED","id":"frontier-item:sha256:0000000000000000000000000000000000000000000000000000000000000000","kind":"INTENT_TEST","nextAction":"SUPPLY_TEST_OBSERVATION","reasons":["TEST_NOT_MATCHED"],"relatedIds":["claim:sha256:0000000000000000000000000000000000000000000000000000000000000000"],"resolutionClass":"ACTIONABLE","subjectId":"CF-V0-001"}],"profile":"frontier/0","scope":{"baseRevision":"0000000000000000000000000000000000000000","excludedPath":".corvint/change.cem.json","intentBlobOid":"0000000000000000000000000000000000000000","intentPath":"docs/specs/change-frontier-v0.md","intentSpan":{"end":"1","start":"0"},"intentSpanSha256":"0000000000000000000000000000000000000000000000000000000000000000","objectFormat":"sha1","patchSha256":"0000000000000000000000000000000000000000000000000000000000000000","targetRevision":"0000000000000000000000000000000000000000"},"universeId":"frontier-universe:sha256:0000000000000000000000000000000000000000000000000000000000000000"}
  ```

  The top-level keys are exactly `frontierState,id,inputs,items,profile,scope,universeId`. `inputs`
  has exactly `cemSha256,lrfSha256,ocmSha256,policy,tcqId,testMode`; `scope` has exactly the keys shown.
  Digests are lowercase 64-hex, IDs use their owning profile grammar, `policy` is `strict-v0`, and
  `testMode` is exactly `STATIC|DYNAMIC_CALLER_REPORTED`. Scope revisions and the intent blob are lowercase full object IDs of the exact width named by `objectFormat`; `intentPath` satisfies the inherited CEM relative POSIX path grammar, and the intent span is non-negative and nonempty. A verifier MUST reject a scope that violates these grammars even when all enclosing IDs are rebound. Valid empty output has the same shape and
  inputs, exact `frontierState: EMPTY`, and `items: []`. `EMPTY` and exit `0` are unreachable through
  the V0 entry point under `CF-V0-004` and `OCM-V0-001`; the shape remains canonical for verification
  and refusal only.
- `CF-V0-019`: `cemSha256`, `ocmSha256`, and `lrfSha256` bind the exact bytes specified below.
  Frontier ID is `frontier:sha256:` plus
  `SHA-256(UTF8("corvint-frontier/0") || 0x00 || codec(document without id))`.
  The commitment codec is a dependency-free canonical JSON subset. Values are only JSON `null`,
  booleans, Unicode-scalar strings, arrays, and objects; JSON numbers are forbidden, and every count,
  ordinal, or offset is instead a base-10 string with no sign or leading zero except `"0"`, whose value
  fits a signed 64-bit integer; a verifier rejects a longer digit run rather than wrapping it. Input with
  a duplicate object key, invalid UTF-8, a BOM, an unpaired surrogate, or a forbidden value is invalid.
  Object keys sort by their raw UTF-8 bytes. Arrays preserve declared order. Serialization emits no
  whitespace or terminal LF; it writes `true`, `false`, and `null` literally, escapes `"` as `\"` and
  `\` as `\\`, escapes every U+0000..U+001F scalar as lowercase `\u00xx`, and emits every other scalar
  as its original UTF-8 bytes without Unicode normalization or optional escaping. A verifier MUST
  parse, reserialize, and require byte equality before hashing. Complete Frontier documents are
  `codec(document)` plus exactly one LF. This definition is byte-identical to the accepted WP3
  contract and MUST be reused, not independently approximated. CEM and OCM digests hash their exact
  verified bounded raw copies; LRF hashes its exact complete canonical result bytes.
- `CF-V0-020`: items sort by kind order `HUNK_BASIS`, `INTENT_CHANGE`, `INTENT_TEST`. Hunk items use
  canonical patch order. Both intent kinds use frozen intent requirement order. ID or caller array
  ordering cannot override these orders. Reasons use `CF-V0-016`; related IDs sort lexicographically.

### Failure, bounds, and privacy

- `CF-V0-021`: operational validation stops at the first failure in this order:

  1. library call shape, immutable-byte types, and optional test-bundle completeness;
  2. inherited raw byte ceilings and bounded JSON preflight;
  3. canonical OCM, CEM, command, and observation parsing in shared OCM-consuming order, then
     Frontier admitted-profile/context compatibility rejection;
  4. missing expected base, then missing target;
  5. one shared OCM-consuming verification call whose native order is not interleaved or
     reimplemented: repository boundary and alternate denial, expected-base resolution and equality,
     caller-target resolution and equality, base-side historical-sidecar check, target-side CEM
     artifact binding, canonical patch derivation and exclusion, then remaining CEM/OCM map
     verification;
  6. LRF recomputation and complete normal result;
  7. static or dynamic TCQ recomputation and complete normal result;
  8. universe and item derivation;
  9. Frontier bounds, canonical encoding, state invariant, and ID.

  An aggregate LRF bound document, TCQ failure, invalid JUnit, stale reference, unsupported object, or a
  catchable timeout, allocation failure, or interruption is operational Frontier failure, never an
  open item. Native CEM/OCM precedence, including the exact alternate/repository/revision/sidecar/
  patch ordering, wins over every later Frontier check.
- `CF-V0-022`: JSON mode operational failure emits no stdout, exit `2`, and exactly one bounded
  stderr object `{"code":"CODE","profile":"frontier-error/0"}` plus one LF. Translation is
  exhaustive and has no prefix or message matching:

  | Caught outcome | Frontier code |
  |---|---|
  | explicit public validation error admitted by the pinned CEM/OCM verifier | exact inherited code |
  | explicit `unsupported-lrf-context` or explicit public non-resource TCQ validation error admitted by `TCQ-V0-042` | exact upstream code |
  | complete LRF bound document containing only profile issue `relevance-bound-exceeded` | `frontier-resource-exhausted` |
  | explicit `tcq-resource-exhausted`, Frontier limit failure, or caught timeout/`MemoryError` | `frontier-resource-exhausted` |
  | caught interruption for which the process remains able to render | `frontier-interrupted` |
  | admitted-input/profile mismatch owned by Frontier | `invalid-frontier-input` or `unsupported-frontier-context` |
  | Frontier document shape, codec, state, order, or ID mismatch | `noncanonical-frontier` |
  | any other caught upstream code or caught internal exception | `frontier-internal-error` |

  The implementation MUST maintain closed allowlists for the exact pinned upstream profile versions;
  an unknown future code cannot pass through. `relevance-bound-exceeded` is an LRF result issue, not
  an operational error code, and MUST NOT appear in a Frontier error envelope. Human mode may render
  one static message for the same code and MUST NOT include unverified values. Process kill,
  uncatchable signal, interpreter abort, or allocator/OS termination may prevent all output; V0 makes
  no JSON, stderr, or exit-code guarantee when no handler can execute.
- `CF-V0-023`: Frontier-owned limits are 2,048 hunk items, 256 intent-change items, 256 intent-test
  items, 2,560 total items, 64 related IDs per item, 32 reasons per item, and 4,194,304 complete
  output bytes. Checked counts occur before append and checked byte arithmetic occurs before output
  allocation. Exceeding a limit fails `frontier-resource-exhausted`, exit `2`, with no partial
  Frontier result.
- `CF-V0-024`: result and error output MUST be source-body-free, diff-body-free, command-body-free,
  and report-body-free. Invalid input MUST NOT echo an unverified path, OID, digest, ID, count, XML
  value, argv element, sibling canary, or exception. Verified paths, IDs, selectors, and hashes remain
  sensitive local metadata and are emitted only in the exact valid result fields above.
- `CF-V0-025`: a valid item asserts only that no qualifying relation was accepted for its named
  obligation after deterministic verification of the declared inputs and profiles. It MUST NOT be
  described as proof that evidence does not exist, that code is wrong, that a test lacks behavioral
  coverage, that a capability is impossible, or that the repository was exhaustively searched.
- `CF-V0-026`: JSON and human output MUST be two renderings of one computation. The library and
  verifier remain usable without Corvint retrieval, a model call, network access, database, daemon,
  account, or service. A command wrapper may read files through an existing separately hardened
  reader, but path acquisition is not part of the Frontier wire or verifier.

### Caller-authored authority

Amended 2026-08-29; **ratified `accepted` 2026-08-29** under repository-owner delegation, recorded as
[decision 0006](../decisions/0006-caller-authored-authority.md). `CF-V0-031` is binding spec text: it
is the conformance authority for authority resolution under `GPK-V0-033`, and an expectation for any
surface it governs is authored from the clause below, never transcribed from either runtime.

`CF-V0-014` freezes caller-reported evidence as non-closing so that an actor
cannot certify its own work. That invariant was stated only over the TCQ harness relation, and every
other authority-conferring surface was left to resolve status at the target revision. A caller who
authors the governing document in the same change set is a caller, not an independent authority, so
the invariant is restated here over authority resolution itself.

- `CF-V0-031`: a governing document confers authority on a change set only when it is resolved from a
  revision the caller did not author. A document whose path is introduced or modified by the same
  declared change set is caller-controlled: its `status`, and every authority label derived from it,
  are caller-reported bytes. Such a document MUST NOT emit `authoritative` confidence, and MUST NOT
  emit an `accepted-contract`, `partially-superseded-contract`, or any successor accepted-authority
  label, for the change set that authors it. The resolver MUST NOT silently drop the citation: it
  MUST record one explicit uncertainty entry naming the cited identifier and the reason the authority
  was withheld, exactly as an unresolvable or non-accepted citation already does. Withholding
  authority is the only permitted disposition; a signature, an integrity digest, a clean-target
  statement, or the document's own declared status cannot upgrade it, because all of them are inside
  the authored bytes. A surface whose input contract supplies no revision the caller did not author
  cannot satisfy this clause and MUST NOT claim to; it either gains such a revision, or its
  accepted-authority labels are unverified and MUST be reported as such. Resolving the document's
  status at an independently supplied base or merge-base revision is a permitted refinement and
  requires a new profile identifier under `CF-V0-029`; it MUST NOT change `frontier/0` in place.
- `CF-V0-032`: the "reported as such" disposition `CF-V0-031` requires is a closed, named form, so
  that a conformance expectation for it is authored from this clause rather than from a runtime. A
  resolver withholding authority MUST keep emitting the citation — its path, line, blob identity, and
  reason are unchanged — and MUST replace only its confidence and authority labels, with confidence
  `low` and authority exactly:
  - `unverified-contract` where the withheld label would have been `accepted-contract` or
    `partially-superseded-contract` (a cited governing decision);
  - `unverified-ledger` where the withheld label would have been `canonical-ledger` (a canonical
    ledger record the change set declares).

  The accompanying uncertainty entry `CF-V0-031` requires MUST be derived from the emitted evidence
  rather than carried beside it, so that a projection which drops the citation drops its explanation (decision 0101: on a surface that takes `CF-V0-031`'s unverified disposition because its input contract supplies no caller-independent revision, each entry MUST be exactly `<reason>: no caller-independent revision is available at which to resolve the cited decision status, so accepted authority is withheld`, where `<reason>` is the withheld `unverified-contract` citation's unchanged `reason`; identical entries collapse to one, entries are in byte order, and they precede every other `coverage.uncertainty` line)
  with it and the two can never disagree. A resolver MUST NOT report a withheld citation as absent,
  as `non-binding`, or as an operational failure: those say something the resolver did not determine,
  and `CF-V0-025` forbids asserting a control that closes nothing.

### Delivery and compatibility

- `CF-V0-027`: V0 delivers only a library/wire preview and deterministic review queue. Implementation
  MUST NOT begin or integrate until the WP5 implementation commit descends from a commit where
  `docs/specs/lexical-relevance-floor-v0.md` has human-accepted intent status and contains the exact
  candidate-nonclosure, result tuple, issue, bound, and dependency-free codec contracts consumed
  here. The implementation MUST record that dependency commit; a proposed WP3 draft or copied text
  is insufficient. V0 MUST NOT advertise a completed Stop hook, proof of done, or empty intent
  frontier while lexical candidates and TCQ are non-closing. Harness integration waits for separately
  accepted change-witness and harness-authority relations under a new Frontier profile.
- `CF-V0-028`: an independent consumer MUST reproduce every reference result byte-for-byte from the
  same raw inputs and Git objects. Corvint producer-to-Corvint verifier tests alone cannot satisfy this
  interoperability requirement.
- `CF-V0-029`: profile evolution is additive by new profile identifier. Future authority,
  acknowledgement, claim quantifier, deletion relation, or policy semantics MUST NOT change
  `frontier/0`, its item IDs, reason ordering, or interpretation in place.
- `CF-V0-030`: delivery remains `experimental` and candidate-only until implementation, canonical vectors,
  adversarial fixtures, full local gate, and independent review exist. A spec, self-authored fixture,
  or green conformance suite alone is not product-value evidence. Decision 0012 withdraws the
  superseded historical thresholds; the replacement preregistered third-party-PR replay protocol is
  due at the first release cut, is constructed separately, and remains `NOT_RUN` until then.

### Repository read-only guarantee

Added 2026-09-12 by `docs/decisions/0162-change-frontier-read-only-requirement-2026-09-12.md`.

- `CF-V0-033`: `corvint frontier` MUST be read-only, per AGENTS.md invariant 4: on every path —
  a valid `EMPTY`/`OPEN` result, an operational `frontier-error/0` failure, or a refused invocation —
  it writes no tracked or untracked repository content, `.git`, or `.corvint` state, leaving the
  repository byte-for-byte unchanged. `internal/frontier` and the CEM/OCM/LRF/TCQ verification it
  composes under `CF-V0-002` read only Git objects and the caller-supplied immutable artifacts
  `CF-V0-001` and `CF-V0-003` admit; no call in that composition creates, prunes, or mutates
  persisted state.

## Conformance and adversarial matrix

| Fixture | Required result |
|---|---|
| CEM 0.1 or OCM bound to 0.1 | `unsupported-frontier-context`, exit 2, no stdout |
| CEM 0.1 or OCM bound to 0.1 plus missing expected base or target | `unsupported-frontier-context`, exit 2, no stdout before revision-required validation |
| missing/mismatched expected base or target; forged sidecar | inherited exact failure, no Frontier result |
| partial command/observation/report tuple | `invalid-frontier-input` |
| no dynamic tuple | static TCQ ID, `testMode: STATIC` |
| exact dynamic caller tuple | dynamic TCQ ID, `DYNAMIC_CALLER_REPORTED`; never test closure |
| verified whitespace-only or line-ending-only hunk | no hunk item |
| each CEM unknown reason | stable item ID and exact mapped reason/action |
| one qualifying and one rejected basis | hunk closes; rejected edge remains in LRF diagnostics |
| broad, self-referential, and unrelated bases with no qualifier | one item; all mapped reasons retained in priority order |
| local CEM or OCM `subject-term-bound-exceeded` beside unrelated valid edges | affected subject remains OPEN with `PROFILE_REQUIRED` / `DEFINE_SUPPORTED_PROFILE`; unrelated results remain available |
| supported deletion | `DELETION_RELATION_REQUIRED`, `PROFILE_REQUIRED` |
| OCM unknown | one intent-change item; no fabricated intent-test item |
| linked obligation without material candidate | `NO_MATERIAL_CHANGE_WITNESS` |
| linked obligation with one or more lexical candidates | one `LEXICAL_CANDIDATE_NONCLOSING` intent-change item containing exactly candidate hunk IDs; never closes |
| one hunk shared across obligations | independent intent-change result per obligation |
| empty, skipped, unsupported, unassociated, ambiguous, failed, errored, and unmatched claim | exact reason union and priority-selected action |
| exact passing TCQ caller report | `CALLER_REPORTED_NONCLOSING`, `AUTHORITY_REQUIRED`, `frontierState: OPEN` |
| forged TCQ authority or unknown relation | upstream canonical rejection |
| evidence maps repaired without patch/intent change | same universe/item IDs; resolved items disappear |
| patch or intent identity changed | new universe and item IDs |
| OPEN with empty items, EMPTY with items, or any extra stop-decision field | `noncanonical-frontier` |
| decimal offset beyond the signed 64-bit range | `noncanonical-frontier`; never wrapped onto a bound value |
| aggregate LRF `relevance-bound-exceeded` result | `frontier-resource-exhausted`; never pass the issue through as an operational code |
| caught timeout, `MemoryError`, interruption, unknown upstream code, and uncatchable termination | exact `CF-V0-022` translation; no output guarantee only for the uncatchable case |
| reordered caller arrays and two fresh processes | byte-identical result |
| source, report, command, sibling, and exception canaries | absent from valid and error output |
| every Frontier limit and limit plus one | complete result versus fail-closed exhaustion |
| independent implementation | byte-identical canonical vectors |

## Evaluation and kill criteria

- Change Frontier ships candidate-only. At the first release cut, preregister a third-party-PR replay
  protocol before selecting or replaying evidence; protocol construction, frozen sampling, outcome
  measures, and promotion/kill thresholds are separate work and remain `NOT_RUN` until then.
- A lexical OCM candidate or caller-reported TCQ relation closing one item, invalid upstream input
  becoming an item, partial output on exhaustion, source/report/command leakage, or an
  independent-consumer byte mismatch is an immediate release blocker.
- Evidence gaming above `0.20` of audited material hunks kills the lexical-witness claim rather than
  adding a learned semantic judge.
- Do not run or advertise the prospective Stop-hook trial until a stronger harness authority profile
  exists. That later trial retains the product gates: at least 20% fewer reviewer misses, no more
  than 15% median latency increase, zero treatment-only critical misses, at least 80% allowed stops,
  and bypass/disable at most 20%.

### Withdrawn predecessor outcome gates and retained non-goals

Decision 0012 withdraws the retained `0.70` precision, `0.50` recall, and `0.25` kill thresholds
from the superseded Verified Absence Frontier. Their replacement is the deferred preregistered
third-party-PR replay protocol above, whose deadline is the first release cut; none of those three
thresholds survives as promotion, kill, or release authority. The separately stated evidence-gaming
and prospective Stop-hook gates above remain unchanged. The predecessor's Stop-hook and
policy-acknowledgement clauses remain excluded because they contradict `CF-V0-005` and
`CF-V0-027`, whose non-goals remain: no whole-program completeness claim, confidence score, implicit
acknowledgement, or conversion of caller-reported execution into authority.

## Simpler baseline and YAGNI cuts

The baseline is separate CEM, OCM, LRF, and TCQ reports plus human composition. V0 adds only the
content-addressed unresolved-item projection and its `EMPTY|OPEN` review state.

V0 adds no database, daemon, graph store, vector store, embedding, model call, ranker, confidence,
repository-wide discovery, multi-repository scope, policy engine, account, role, ACL, signature,
human-attestation wire, ticket/wiki ingestion, test selection, test execution, retry inference,
coverage model, behavioral model, stored frontier history, `corvint why`, free-form remediation
command, UI, or network service.

## Unresolved decisions deliberately deferred

- The first closing test authority should be the smallest harness-controlled boundary: one process
  owns the exact command launch, target, report bytes, and exit result and passes a non-caller
  capability directly to a new profile. Its wire and portability remain a separate decision.
- Attributable acknowledgement and permissive policy remain deferred because current wires have no
  authority capable of expressing them.
- Test quantifiers remain deferred. Supporting `ANY`, `ALL`, or explicit claim groups requires an
  OCM wire change, a closing authority relation, and a new Frontier profile.

None of these deferred decisions may be silently resolved by the V0 implementation.

## Traceability and rollback

| Requirement | Planned implementation surface | Required evidence |
|---|---|---|
| `CF-V0-001..005`, `021..026` | Frontier library boundary and shared verifier composition | input, precedence, error, privacy, and resource fixtures |
| `CF-V0-006..020` | canonical universe, item, and result projection | identity, closure, ordering, wire, and fresh-process vectors; `TestVerifyDocumentRebindsNestedIdentities` requires the verifier to reject forged universe and item IDs even when the outer document ID is rebound; `TestVerifyDocumentRejectsForgedItemAuthority` rejects an admitted but semantically false authority class; `TestVerifyDocumentRejectsNoncanonicalScopeIdentity` covers revision, path, object-format width, and span scope grammar |
| `CF-V0-027..030` | candidate-only preview integration and independent consumer | non-closure, compatibility, interoperability, and deferred third-party-PR replay `NOT_RUN` evidence |
| `CF-V0-031` (accepted 2026-08-29) | every authority-conferring resolver, Frontier-owned or not | an adversarial case where the change set authors its own accepted governing document and MUST NOT receive `authoritative`/`accepted-contract`, plus the named uncertainty entry. Held by `TestRangeImpactRefusesAuthorityFromSelfAuthoredADR` and `TestRangeImpactWithholdsSelfAuthoredLedgerAuthority` (`internal/contextindex/range_impact_test.go`), `TestImpactWithholdsUnverifiableADRAuthority` (`internal/contextindex/impact_test.go`), and parity case `impact-self-authored-adr` (`conformance/cli-parity-v0`) |
| `CF-V0-032` (accepted 2026-08-29) | the withheld-authority reporting form on every such resolver | the same cases, asserting confidence `low` with authority exactly `unverified-contract` / `unverified-ledger` and an uncertainty entry derived from the emitted evidence; its exact `impact` wording and leading placement are held by `TestImpactWithholdsUnverifiableADRAuthority` |
| `CF-V0-033` (accepted 2026-09-12, decision 0162) | `cmd/corvint/frontier.go` and `internal/frontier` | `cmd/corvint`: `TestCLIReadVerbsLeaveTheRepositoryByteIdentical`, subtest "frontier reports an EMPTY frontier through the registered adapters", and `TestRunFrontierUnsupportedRefusalLeavesRepositoryUnchanged` (successful and refused CLI-level repository-byte assertions) |
| Outcome protocol | preregistered third-party-PR replay, constructed separately at first release cut | explicit `PASS`, `FAIL`, or `NOT_RUN` record; currently `NOT_RUN` |

Before promotion, rollback removes the derived Frontier result and preview wrapper. It preserves CEM,
OCM, LRF, TCQ, explicit unknowns, failed evaluations, and this proposed specification as decision
history. A failed preview does not justify weakening upstream verification or relabelling lexical or
caller-reported evidence as closure.
