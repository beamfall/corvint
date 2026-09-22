# Change Frontier Profile `frontier/1`

Owner: Russell Lewis (repository owner). Accepted by the owner in decision 0047 (2026-09-04).
Nothing here amends
`change-frontier-v0.md`; where a `frontier/0` clause would have to change, this document *describes*
the change rather than making it.
Date: 2026-08-29
Intent status: accepted (decision 0047, 2026-09-04)
Delivery status: not-started
Authoritative inputs: `docs/specs/change-frontier-v0.md`,
`docs/specs/change-witness-relation-v0.md`, `docs/specs/harness-authority-relation-v0.md`
(superseded by `docs/decisions/0009-harness-authority-boundary.md`),
`docs/specs/agent-harness-integration-v0.md`, `docs/decisions/0006-caller-authored-authority.md`

## Agent digest
- Claim: The accepted `frontier/1` draft proposes closing witnessed change obligations without altering `frontier/0` authority.
- Status: accepted (decision 0047, 2026-09-04)/not-started
- Exists: nothing executable — draft profile and relation contracts only.
- Blocked on: dependency gates and owner inputs preserved by decision 0047; harness integration remains unavailable.
- Read next: The finding that matters most; CF-V1-020; decision 0047; `0009-harness-authority-boundary.md` (successor to the harness-authority relation).

`CF-V0-027` says "Harness integration waits for separately accepted change-witness and
harness-authority relations under a new Frontier profile"
(`docs/specs/change-frontier-v0.md:397-398`). This document is that profile. Its two relations are
[`change-witness-relation-v0.md`](change-witness-relation-v0.md) (`ocm-change-witnessed-v0`) and
[`harness-authority-relation-v0.md`](harness-authority-relation-v0.md)
(`harness-execution-attested-v0`), the latter now superseded by
[`docs/decisions/0009-harness-authority-boundary.md`](../decisions/0009-harness-authority-boundary.md).

**This document does not close the frontier gap and must not be cited as having closed it.**
Decision 0047 accepts intent only. Read `CF-V1-020` before drawing any conclusion about the
`frontier-authority-unavailable` degradation.

## The finding that matters most

`CF-V0-027` names two missing relations. **Three things are missing, not two.** The third is not
named anywhere in `change-frontier-v0.md` or `agent-harness-integration-v0.md`, and it is the one
that actually holds the degradation in place:

`internal/gokernel/harness.go` **never consults Frontier.** It contains no import of
`internal/frontier`. The degradation is the literal
`degradations := []any{"frontier-authority-unavailable"}` at `internal/gokernel/harness.go:396@a4869098`,
appended unconditionally to **every** event — `session-start`, `user-prompt`, `file-change`,
`post-tool`, `stop`, `session-end` alike — and the `stop` response's frontier block at
`internal/gokernel/harness.go:436-440@e567ed2d` is a hardcoded constant map
(`reason: "frontier-authority-unavailable"`, `shouldContinue: false`, `state: "UNAVAILABLE"`).

A Frontier command surface does exist and is wired to real Git-backed adapters
(`cmd/corvint/frontier.go`, `cmd/corvint/frontier_adapters.go:24-29`, which binds
`frontierrepo.New`). Nothing connects it to the harness. So accepting both relations and this profile
would change **zero bytes** of any harness receipt. A fourth work item — wiring
`corvint harness event --event stop` to compute a frontier over a declared universe — is required, and
it needs inputs the harness event contract does not currently carry (`CF-V0-001` requires a verified
CEM 0.2 artifact, a bound OCM artifact, an independent expected base, and an independent target;
`AHI-014` restricts the adapter to "the minimum event fields admitted by `corvint-harness-event/0`",
`docs/specs/agent-harness-integration-v0.md:140-142`). Where those artifacts come from at a stop
lifecycle point is Q4 below.

## User and job

A maintainer or agent using `frontier/0` gets a review queue that can never be empty for any
repository with a linked obligation, and a stop hook that always reports `UNAVAILABLE`. `frontier/1`
is the profile in which a witnessed obligation can stop producing items, so that "no open frontier
items" becomes a statement a repository can actually reach — and, just as importantly, so that
reaching it means something.

## Verified current state

- `frontier/0` is implemented and its profile constant is `Profile = "frontier/0"`
  (`internal/frontier/types.go:9`), with `ErrorProfile = "frontier-error/0"` (`:10`).
- `frontier/0` cannot ever be `EMPTY` over a real repository. This is not a defect and not an
  opinion; it is a recorded derivation with an executing test.
  `conformance/frontier-v0/capabilities.go:119-127` states that "no verified universe can carry zero
  obligations, so the CF-V0-004 EMPTY state cannot be produced from any repository", derived in four
  links at `:130-161`: `CF-V0-001` requires a bound OCM artifact; both runtimes refuse an intent
  scope with no enumerated requirement (`internal/lrfrepo/ocm.go` and the frozen oracle
  `src/context_corvint_ocm.py`) with code `missing-requirements`; `OCM-V0-003` requires one obligation
  per requirement; and every obligation emits at least one intent item
  (`IntentItemsPerObligation` at `:162-165`: `unknown` → 1, `linked` → 2). Sixteen fixture cases skip
  on this (`ObligationFreeCaseCount = 16`, `:171`), and
  `TestEmptyStateIsUnreachableThroughTheEntryPoint` (`conformance/frontier-v0/fixtures_test.go:666`)
  drives the refusal for real rather than asserting it.
- Decision 0012 closes the former spec gap by narrowing `OCM-V0-001`: an empty requirement
  enumeration is normatively invalid with `missing-requirements`. `CF-V0-004` and `CF-V0-018`
  retain `EMPTY` as a canonical verification/refusal shape that is unreachable through V0.
- `CF-V0-029` requires additive evolution: "Future authority, acknowledgement, claim quantifier,
  deletion relation, or policy semantics MUST NOT change `frontier/0`, its item IDs, reason ordering,
  or interpretation in place" (`docs/specs/change-frontier-v0.md:402-404`).
- `CF-V0-014` freezes `tcq/0` + `CALLER_REPORTED` as mechanically non-closing
  (`docs/specs/change-frontier-v0.md:193-197`), and nothing in this profile relaxes it.
- A premise this draft expected and did **not** find: the comment at `cmd/corvint/frontier.go:29-32`
  says the adapters "land independently of this command surface" and that while the seam "is nil the
  command REFUSES". That comment is stale — `cmd/corvint/frontier_adapters.go` assigns the seam in
  an `init` function (commit `5b05b44`, "frontier: give Change Frontier V0 a command surface"), so
  the command does not refuse. The Frontier library is reachable from the CLI today; only the harness
  is not connected to it.

## Requirements

### Profile identity and non-interference

- `CF-V1-001`: the profile identifier is exactly `frontier/1`; its error profile is exactly
  `frontier-error/1`. A document emitting `profile: "frontier/1"` is a `frontier/1` document and MUST
  be verified only by a `frontier/1` verifier.
- `CF-V1-002`: `frontier/0` is untouched in every respect — clause text, item kinds, item IDs, reason
  ordering, wire shape, error translation, limits, and interpretation. A repository may compute both
  profiles over the same universe; neither result is evidence about the other. This is `CF-V0-029`
  discharged rather than restated.
- `CF-V1-003`: identity domain separation is deliberate and asymmetric:
  - the **universe** domain stays `corvint-frontier-universe/0` (`CF-V0-006`), because a universe is
    the declared inputs and carries no closure semantics. Keeping it stable is what lets a maintainer
    say "this is the same universe" across profiles;
  - the **item** domain becomes `corvint-frontier-item/1` and the **document** domain becomes
    `corvint-frontier/1`, because both encode closure semantics this profile changes. A `frontier/0`
    consumer must never be able to mistake a `frontier/1` item or document for one of its own.

  The cost is explicit: an item for the same obligation has a different ID in each profile. Tracking
  one obligation across profiles uses the `(universeId, kind, subjectId)` triple, which is stable.
- `CF-V1-004`: `frontier/1` reuses the `CF-V0-019` commitment codec byte-for-byte. It MUST be the
  same accepted WP3 contract (`internal/wp3codec`), not an independent approximation.

### What changes relative to `frontier/0`

`CF-V1-005` through `CF-V1-008` are the only behavioural differences. Every other `CF-V0` clause
applies unchanged, with `frontier/0` read as `frontier/1`.

- `CF-V1-005`: **`CF-V0-012` becomes conditional.** A structurally linked OCM obligation emits an
  `INTENT_CHANGE` item unless `ocm-change-witnessed-v0` (`CWR-V0-004`) holds for it. When the
  relation holds, no `INTENT_CHANGE` item is emitted. When it is *withheld* under `CWR-V0-005`
  (caller-authored requirement), the item IS emitted, with reason `SELF_AUTHORED_OBLIGATION`,
  `resolutionClass: AUTHORITY_REQUIRED`, and `nextAction: ESTABLISH_CHANGE_WITNESS`. Every other
  `CF-V0-012` outcome — `LEXICAL_CANDIDATE_NONCLOSING`, `SUBJECT_TERM_BOUND_EXCEEDED`,
  `NO_MATERIAL_CHANGE_WITNESS` — is preserved with its `frontier/0` reason, class, and action.
- `CF-V1-006`: **`CF-V0-015` becomes conditional in form and unchanged in effect.** A structurally
  linked obligation emits an `INTENT_TEST` item unless `harness-execution-attested-v0` (`HAR-V0-008`)
  closes it. Because `HAR-V0-004`'s accepted root set is empty, **no obligation is closed by this
  clause today, and every linked obligation still emits an `INTENT_TEST` item.** `CF-V0-014` and
  `CF-V0-016`'s order-1 row (`CALLER_REPORTED_NONCLOSING` → `AUTHORITY_REQUIRED` →
  `ESTABLISH_HARNESS_AUTHORITY`) are preserved exactly. An implementation MUST NOT ship a code path
  that removes an `INTENT_TEST` item while the accepted root set is empty; there is nothing for it to
  do, and its existence is an invitation to relax `HAR-V0-005`.
- `CF-V1-007`: `CF-V0-017`'s `authorityClass` enumeration becomes
  `NONE|PRODUCER_DECLARED|CALLER_REPORTED|VERIFIER_DERIVED|HARNESS_ATTESTED`. The two new members are
  defined by `CWR-V0-003` and `HAR-V0-002`. A consumer that cannot distinguish all five MUST refuse
  the document rather than collapse a class it does not know.
- `CF-V1-008`: `CF-V0-016`'s `INTENT_CHANGE` reason handling gains exactly one reason,
  `SELF_AUTHORED_OBLIGATION`, selected ahead of `SUBJECT_TERM_BOUND_EXCEEDED` and the two ordinary
  reasons. `INTENT_CHANGE` still has exactly one reason. The `INTENT_TEST` table and the `HUNK_BASIS`
  order are unchanged.

### What does not change

- `CF-V1-009`: policy is still the literal `strict-v0`. `frontier/1` adds no acknowledgement,
  exception, score, confidence, or permissive-success input. `CF-V0-005` applies unchanged
  (`docs/specs/change-frontier-v0.md:105-109`).
- `CF-V1-010`: `frontier/1` defines no test-claim quantifier. `CF-V0-013` applies unchanged
  (`:189-192`); a quantifier needs an OCM wire change and MUST NOT be inferred.
- `CF-V1-011`: hunk-basis closure is unchanged. `CF-V0-008`, `CF-V0-009`, and `CF-V0-010` apply as
  written; this profile is about intent items only.
- `CF-V1-012`: the error vocabulary, limits, privacy rules, and the `CF-V0-025` humility clause apply
  unchanged. In particular a `frontier/1` item still asserts only that no qualifying relation was
  accepted, and `frontier/1`'s new closure MUST NOT be described as proof that a requirement is
  satisfied.
- `CF-V1-013`: `frontier/1` still executes nothing, opens no path, and reaches no network. Under `CWR-V0-014` and decision
  `docs/decisions/0151-change-witness-symbol-resolver-boundary-2026-09-12.md`, its resolver is Go `go/parser` plus `go/ast` over target Git objects, with no type checking, imports, external tool, persisted index, or fallback; other languages abstain, and `CF-V0-026` needs no narrowing.

### Reachability of `EMPTY`, stated honestly

- `CF-V1-014`: under `frontier/1`, a universe whose every linked obligation is witnessed and whose
  every hunk closes would emit no items, so `frontierState: EMPTY` is **reachable in principle** —
  link 4 of the `conformance/frontier-v0/capabilities.go:130-161` derivation is exactly what
  `CF-V1-005` breaks.
- `CF-V1-015`: **but it is not reachable in practice today, for two independent reasons, and neither
  is closed by this document.**
  1. `CF-V1-006`: `INTENT_TEST` still emits for every linked obligation, so a linked obligation
     always contributes at least one item. Until `HAR-V0-004`'s root set is non-empty, `EMPTY`
     requires a universe with **no linked obligations at all** — and an `unknown` obligation still
     emits an `INTENT_CHANGE` item under `CF-V0-011`, which `frontier/1` does not change.
  2. `OCM-V0-001`, narrowed by decision 0012, requires at least one enumerated requirement and exact
     `missing-requirements` refusal otherwise, so a universe always carries at least one obligation.
     The sixteen skipped fixture cases (`ObligationFreeCaseCount = 16`) stay skipped.
- `CF-V1-016`: consequently `frontier/1` MUST NOT be described as making `EMPTY` reachable, as
  closing the `CF-V0-004` gap, or as enabling a Stop hook. `CF-V0-027`'s prohibition — do not
  advertise a completed Stop hook, proof of done, or empty intent frontier while non-closing evidence
  remains — applies to this profile verbatim.

### Relationship to the harness

- `CF-V1-017`: `frontier/1` defines no harness behaviour. It is a library and wire profile, exactly
  as `frontier/0` is under `CF-V0-027`.
- `CF-V1-018`: a harness that computes a `frontier/1` result MUST still obey `AHI-006` (advise or
  block only where repository policy and the host permission model explicitly allow it,
  `docs/specs/agent-harness-integration-v0.md:109-110`) and `AHI-009` (fail closed for evidence
  capture, fail open for unrelated coding, `:117-118`).
- `CF-V1-019`: a harness MUST NOT report `frontier.state` as anything other than `UNAVAILABLE` unless
  it actually computed a frontier over a declared universe. Reporting a state it did not compute is
  the `CF-V0-025` violation this whole profile exists to avoid, in the surface where a user will
  actually read it.
- `CF-V1-020`: **accepting this profile does not remove `frontier-authority-unavailable` from any
  receipt.** The literal at `internal/gokernel/harness.go:396@a4869098` is unconditional and the `stop` block
  at `:435-441@2d42b7c2` is a constant; neither consults `internal/frontier`. The degradation is honest today
  and remains honest after acceptance. Removing it requires, in order: (a) an accepted
  `harness-execution-attested-v0` root, without which `INTENT_TEST` never closes; (b) a way for a
  stop lifecycle point to obtain the `CF-V0-001` inputs (Q4); (c) the wiring itself; and (d) the
  corpus migration in the next section. An implementation that removes the literal before (a) is
  claiming an authority that does not exist.

## Migration cost of ever removing the degradation string

Measured in the tree at this date. The task brief recorded **18** `harness event` parity cases whose
receipt bytes would change. The 18 is right as a family count and the receipt-bytes half needs one
correction, recorded here rather than passed over:

`conformance/cli-parity-v0/manifest.json` carries exactly **18** cases whose `argv` begins
`["harness","event"]`. Of those, **9 exit 0 and carry the literal in `stdout`**
(`harness-file-change` 1,640 B, `harness-post-tool` 819 B, `harness-session-end-outcome` 971 B,
`harness-session-start` 866 B, `harness-session-start-compact-clean` 866 B,
`harness-session-start-compact-rehydrated` 2,032 B, `harness-stop` 833 B, `harness-user-prompt`
3,010 B, `harness-user-prompt-out-of-scope` 2,495 B — 13,532 B total). The other **9 exit 2 with
`stdoutBytes: 0`**; their 83–193 B of stderr is the bounded error envelope and does not contain the
literal. So **9 cases change bytes; all 18 must be re-run and re-attested** because the family shares
one fixture repository and one oracle.

A rebaseline is therefore not a re-capture, and this is the expensive part:

1. **Every one of those expectations is `expectationSource: python-oracle`**, and the oracle emits
   the literal at `src/context_corvint_harness.py` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 518) and historical line 557. `src/` is frozen. Per
   `conformance/divergence-register.md:13`, a `python-defect` outcome explicitly directs "**Do NOT
   repair `src/`**". So the migration produces a *divergence*, handled the way `DR-0007` was: a
   register entry, a **spec-authored** candidate-side expectation, and the case marked
   known-divergent (`docs/decisions/0006-caller-authored-authority.md`, Consequences).
2. **The spec-authored expectation does not exist yet.** With no accepted clause saying the
   degradation must stop being emitted, the adjudication is `spec-gap`, whose rule is "park the case;
   amend the owning spec first; neither runtime wins by default"
   (`conformance/divergence-register.md:15`). **This is the concrete, mechanical reason the
   degradation cannot be removed by an implementation change alone, and the concrete thing accepting
   `CF-V1-020` plus a wiring clause would unblock.**
3. **A second independent implementation must move in lockstep.**
   `conformance/harness-event-v0/boundary_corvint.py` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 38) and historical line 71 hardcode the same literal as an
   independent boundary; leaving it behind makes harness-event conformance disagree with cli-parity.
4. **Receipt identities are safe.** The receipt basis is `{adapter, event, input, repository}`
   (`internal/gokernel/harness.go:450-455`) and does not include `degradations`, so every
   `receiptId` is stable across this change. Only the enclosing stdout document's digest moves.
5. **Adapters need no change, by design.** `integrations/compatibility.json:58-70` declares
   `receiptDegradationPolicy` with `"match": "subset-of-recognised"` and the note that an adapter
   "accepts any subset of the recognised codes, including the empty set, so that core closing a gap
   is never an adapter break". `tests/test_harness_claude.py` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 532-536) already exercises the empty set.
   Only the stale `"accepted-frontier-authority-unavailable"` entry in `globalDegradations`
   (`integrations/compatibility.json:52-57`) and the three per-adapter `compatibility.json` files
   would need editing. The current shared compatibility declaration names
   `conformingAdapters` as `["claude-code", "codex", "gemini-cli", "opencode"]`; this is a
   compatibility declaration, not evidence that every adapter independently enforces the empty-set
   case. That case remains unqualified pending per-adapter evidence. `host-version-unknown` is appended
   whenever `hostVersion == "unknown"` (`internal/gokernel/harness.go:403-405`, and independently
   `conformance/harness-event-v0/boundary_corvint.py` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 53-54)), which is the standing case for codex,
   gemini-cli, and opencode, so those three receive a two-element list on every production call and
   an exact-match rule would have refused all of them.
6. **Twelve Python assertion sites** across `tests/test_context_corvint_harness.py` (2),
   `tests/test_harness_claude.py` (5), `tests/test_harness_gemini.py` (2),
   `tests/test_harness_opencode.py` (2), `tests/test_harness_codex.py` (1) assert the literal. Under
   the `python-defect` rule these stay: they test the frozen oracle, which keeps cross-checking.
7. **Historical results MUST NOT be rebaselined.** The 20+ occurrences under
   `conformance/perf-v0/results/**` are dated observation records of runs that happened. Rewriting
   them would falsify evidence, not migrate it.

## Conformance and adversarial matrix

| Fixture | Required result |
|---|---|
| `frontier/1` document presented to a `frontier/0` verifier | refused; different profile and document domain |
| `frontier/0` document presented to a `frontier/1` verifier | refused; not silently upgraded |
| same universe computed under both profiles | identical `universeId`, different item IDs, both valid |
| witnessed linked obligation | no `INTENT_CHANGE` item; `INTENT_TEST` item still present |
| withheld (caller-authored) linked obligation | `INTENT_CHANGE` item with `SELF_AUTHORED_OBLIGATION` |
| unwitnessed linked obligation | byte-identical `INTENT_CHANGE` reason/class/action to `frontier/0` |
| OCM `unknown` obligation | unchanged from `CF-V0-011` |
| any obligation, accepted root set empty | `INTENT_TEST` item present with `CALLER_REPORTED_NONCLOSING` |
| an implementation path that removes an `INTENT_TEST` item | refused; `CF-V1-006` |
| every hunk closes and every linked obligation witnessed | still OPEN, because `INTENT_TEST` remains |
| a consumer collapsing `VERIFIER_DERIVED` to `PRODUCER_DECLARED` | refusal, not downgrade; `CF-V1-007` |
| a harness reporting a `frontier.state` it did not compute | refused; `CF-V1-019` |

## Evaluation and kill criteria

- `frontier/1` is worth accepting only if `ocm-change-witnessed-v0` proves informative on a sealed
  cohort (see `change-witness-relation-v0.md`'s kill criteria). If the witness rate is near 100% or
  near 0%, the profile adds a wire and a class for no discrimination.
- One `frontier/0` document altered, reinterpreted, or re-verified by `frontier/1` machinery is an
  immediate blocker: it is exactly what `CF-V0-029` forbids.
- One `INTENT_TEST` item removed while the accepted root set is empty is an immediate blocker, on the
  same footing as `CF-V0`'s "caller-reported TCQ relation closing one item"
  (`docs/specs/change-frontier-v0.md:449-452`).
- Advertising `EMPTY`, a Stop hook, or a cleared degradation on the strength of this profile alone is
  an immediate blocker under `CF-V1-016`.

## Simpler baseline and YAGNI cuts

The baseline is `frontier/0` as shipped plus a human reading the `INTENT_CHANGE` items. `frontier/1`
buys exactly one thing: obligations whose referenced hunks demonstrably change what they name stop
appearing in the queue. If the queue is short enough that a human reads it anyway, that is not worth
a second profile, a fourth and fifth authority class, and a corpus migration.

`frontier/1` adds no database, daemon, network, model call, policy engine, acknowledgement wire,
stored history, UI, or quantifier.

## Unresolved decisions deliberately deferred

- Whether `frontier/2` should resolve the OCM intent scope at the base or merge-base revision, which
  `CF-V0-031` already names as "a permitted refinement" requiring a new profile identifier
  (`docs/specs/change-frontier-v0.md:370-372`). That would make `CWR-V0-005` a resolution rule rather
  than a withholding rule.
- Test quantifiers, which stay deferred for `CF-V0-013`'s reason.
- Whether an attested *failing* run should sort differently from an unattested one.

## Questions only the repository owner can answer

- **Q1 (answered 2026-09-12 by decision 0151 and `CWR-V0-014`). May a Frontier verifier depend on a Corvint-built symbol resolver?**
  Only on the specified Go standard-library parser over target-revision Git objects, with no Corvint retrieval, persisted index, type checking,
  import resolution, network, external tool, or scanner fallback; unsupported languages and unparsed Go sources abstain. This settles only
  the resolver boundary: `frontier/1` remains not-started and the other questions below remain open.
- **Q2. Are `VERIFIER_DERIVED` and `HARNESS_ATTESTED` acceptable as fourth and fifth authority
  classes?** `CF-V0-017` freezes three. Additive is permitted by `CF-V0-029`, but every consumer must
  learn them.
- **Q3. Is a per-obligation withholding that fires on essentially every Corvint self-change
  acceptable?** `CWR-V0-005` withholds when the requirement is authored by the same change set, which
  on this repository is nearly always. It is the honest consequence of accepted `CF-V0-031`, and it
  means Corvint cannot dogfood its own change-witness relation.
- **Q4. Where does a stop lifecycle point get a CEM 0.2 artifact, a bound OCM artifact, an
  independent expected base, and an independent target?** `CF-V0-001` requires all four; `AHI-014`
  restricts the adapter to the minimum admitted event fields. Until this is answered, the wiring in
  `CF-V1-020` cannot be specified, and the degradation stays regardless of how many relations are
  accepted. This is the question `CF-V0-027` does not ask.
- **Q5. If the answer to `harness-authority-relation-v0.md`'s Q1 and Q2 is "no" — no authority root,
  no acknowledgement wire — should `CF-V0-027` be amended to say the `INTENT_TEST` half never
  closes?** In that case `frontier/1` should ship with only `CF-V1-005`, and
  `frontier.state: UNAVAILABLE` becomes a permanent, documented product boundary rather than a
  provisional degradation.

## Traceability and rollback

| Requirement | Planned surface | Required evidence |
|---|---|---|
| `CF-V1-001..004` | profile constants and identity domains | cross-profile refusal vectors; identical `universeId` across profiles |
| `CF-V1-005`, `CF-V1-008` | conditional `INTENT_CHANGE` emission | the witnessed / withheld / unwitnessed rows above |
| `CF-V1-006` | conditional `INTENT_TEST` emission | the empty-root-set row, asserted rather than assumed; the removal path must be absent |
| `CF-V1-007` | authority-class enumeration | a consumer that collapses a class is refused |
| `CF-V1-009..013` | inherited clauses | the existing `frontier/0` fixtures, re-run under `frontier/1` |
| `CF-V1-014..016` | reachability claims | `EMPTY` remains unreachable; the 16 skipped cases stay skipped |
| `CF-V1-017..020` | harness boundary | the degradation literal still emitted; no harness state reported without a computed frontier |

Rollback removes the `frontier/1` profile and both relations. `frontier/0`, CEM, OCM, LRF, TCQ, the
harness receipt shape, the parity corpus, and every recorded unknown are untouched, because nothing
in this document modifies any of them. A failed `frontier/1` does not justify weakening `CF-V0-014`,
`CF-V0-031`, or `CWR-V0-005`, and does not justify removing a degradation that is still true.
