# Harness Authority Relation V0

Owner: Russell Lewis (repository owner). Acceptance of this document is a repository-owner decision.
This draft carries no delegated acceptance and does not claim one. **Its central conclusion is a
refusal**, so accepting it does not authorize an implementation; it records what cannot be built and
what would have to change first.
Date: 2026-08-29
Intent status: superseded by accepted decision 0009 option 2 (local non-authority workflow policy)
Delivery status: superseded
Successor (option 2 accepted 2026-09-06): `docs/decisions/0009-harness-authority-boundary.md`

## Agent digest
- Successor: `docs/decisions/0009-harness-authority-boundary.md` accepts local non-authority workflow policy; it accepts no execution authority root or Frontier closure.
- Why: this draft's conditional relation was superseded because no accepted same-UID harness authority root exists.

This is the second of the two relations `CF-V0-027` names
(`docs/specs/change-frontier-v0.md:397-398`). Its companions are
[`change-witness-relation-v0.md`](change-witness-relation-v0.md) and
[`change-frontier-profile-1.md`](change-frontier-profile-1.md).

## Summary of the finding

`CF-V0-027` reads as though drafting this relation is the remaining work. It is not. The relation's
wire is easy; its **authority root** does not exist, and cannot exist under the trust boundary Corvint
has committed to. Every candidate root in this repository is caller-owned:

| Candidate root | Why it is not an authority | Citation |
|---|---|---|
| TCQ dynamic tuple `(command, observation, report)` | supplied by the caller by construction | `CF-V0-003`, `docs/specs/change-frontier-v0.md:91-95` |
| any signature over those bytes | the bytes and the key are both caller-controlled | `CF-V0-014`, `:193-197` |
| the Go live-test provider receipt | its own type doc says **"caller-owned, non-persistent canonical transcript"** | `internal/liveverify/provider/receipt.go:57` |
| the harness adapter itself | it normalizes host-supplied fields and obeys host policy; the host is the caller | `AHI-014`, `docs/specs/agent-harness-integration-v0.md:140-142` |
| the Frontier verifier running the tests itself | forbidden: no shell runner, no execution | `CF-V0-026` (`:340-344`), `TCQ-V0-045` (`:561-565`) |

And TCQ V0 states the boundary directly: Corvint "does **not** resist a malicious or compromised
same-UID caller or task agent. That caller can fabricate a command artifact, JUnit report,
observation, or clean-target statement. V0 has no portable authority, authentication, signature,
trusted runner, or execution attestation" (`docs/specs/test-claim-qualification-v0.md:42-44`).

**On a single-UID developer machine, with no daemon, no network, and no service —
the boundary `docs/specs/agent-harness-integration-v0.md:261-263` commits to — there is no process
whose output a same-UID caller cannot fabricate.** A closing harness relation therefore cannot be
specified as closing until either that boundary changes or the closure decision moves from authority
to policy. Both are repository-owner decisions (Q1, Q2 below).

So this document specifies the relation **conditionally**: its wire, its authority class, and the
exact property a root must have, together with a clause that makes it close nothing while the set of
accepted roots is empty — which it is, and which this document does not change.

## User and job

Before a harness may say "this change is done", a maintainer needs to know whether the tests that
allegedly qualify it were run by something the author could not have faked. Today they cannot know
this, and Corvint correctly refuses to pretend otherwise: `CF-V0-015` gives every structurally linked
obligation an `INTENT_TEST` item with `authorityClass: CALLER_REPORTED`
(`docs/specs/change-frontier-v0.md:198-206`), and `CF-V0-016` gives it next action
`ESTABLISH_HARNESS_AUTHORITY` (`:216`). This document is the specification of that next action, and
its finding is that the action is not currently available.

## Verified current state

- `frontier/0`'s `INTENT_TEST` item is unconditional: "every structurally linked obligation emits
  exactly one `INTENT_TEST` item in Frontier V0" (`CF-V0-015`, `:198`).
- `tcq/0` has exactly one relation and it is frozen non-closing: `CF-V0-014` says "Signatures over
  caller-controlled V0 bytes, clean-target statements, passing rows, and exit zero cannot upgrade
  that authority. A stronger harness relation requires a new TCQ and Frontier profile and MUST NOT
  reinterpret V0 bytes" (`:193-197`). The constant is `internal/tcq/canonical.go:17`; Frontier's copy
  is `internal/frontier/closure.go:165`.
- `TCQ-V0-046` reserves the upgrade path: "Only a future WP6 authority root and new relation/profile
  can define stronger behavior; signatures over V0 caller-controlled bytes do not qualify"
  (`docs/specs/test-claim-qualification-v0.md:532-537`).
- `CF-V0`'s own deferred-decision list already sketches the shape: "one process owns the exact
  command launch, target, report bytes, and exit result and passes a non-caller capability directly
  to a new profile. Its wire and portability remain a separate decision"
  (`docs/specs/change-frontier-v0.md:514-516`).
- An execution surface exists in the tree and is **not** an authority root:
  `internal/liveverify/{gorunner,provider,gotest,godiscovery,parentverify,affected}` implements a Go
  test runner with canonical receipts and process ownership, under `go-live-test-provider-v0.md`
  (intent `proposed`, delivery `not-started`). Its receipt type is documented "caller-owned"
  (`internal/liveverify/provider/receipt.go:57`), and the spec itself disclaims authority: "no result
  from this profile is current LPCV evidence or policy input"
  (`docs/specs/go-live-test-provider-v0.md:48-50`).
- A premise this draft expected and did **not** find: there is no registry, key store, capability
  table, or attestation verifier anywhere in `internal/` that could hold an accepted authority root.
  `internal/witness` is a rendering package for the `witness` command, not an attestation root.
- **The harness does not consult Frontier at all.** `internal/gokernel/harness.go` contains no import
  of `internal/frontier`; the degradation is the literal `degradations := []any{
  "frontier-authority-unavailable"}` at `internal/gokernel/harness.go:396@a4869098`, appended to **every**
  event, and the `stop` block at `:436-440@e567ed2d` is a hardcoded constant map. This matters here: even an
  accepted closing relation would not change one byte of a harness receipt until that wiring exists.

## Definitions

- **authority root**: a process whose identity a same-UID caller cannot assume, that solely owns a
  test execution's command launch, target materialization, result rows, and exit status;
- **non-caller capability**: a value the root passes to the verifier that a caller cannot mint,
  replay across targets, or transplant between runs;
- **accepted root set**: the set of authority roots the repository owner has explicitly accepted.
  **It is empty, and this document does not add to it.**

## Requirements

### Relation identity

- `HAR-V0-001`: the relation string is exactly `harness-execution-attested-v0`. It is a **new**
  relation under a **new** TCQ profile; it MUST NOT be emitted by `tcq/0`, MUST NOT reinterpret any
  `tcq/0` byte, and MUST NOT be recognised by `frontier/0`. This is `CF-V0-014`'s requirement
  (`:196-197`) and `TCQ-V0-046`'s (`:536-537`) restated as an identity constraint.
- `HAR-V0-002`: the relation's authority class is exactly `HARNESS_ATTESTED`. It is a fourth class
  beside `CF-V0-017`'s three and MUST NOT alias, subsume, or be rendered as `CALLER_REPORTED`. A
  consumer that cannot distinguish the two MUST refuse the document rather than downgrade it.

### The root requirement, and the clause that makes this relation inert

- `HAR-V0-003`: an `harness-execution-attested-v0` edge is admissible only when its attestation binds
  **all** of: the exact target tree object ID; the exact command; the exact result rows and their
  execution keys; the exact exit status; and one root identity. Any missing binding is
  `invalid-harness-attestation`, never a weaker edge.
- `HAR-V0-004`: the root identity MUST resolve to a member of the **accepted root set**. **The
  accepted root set is empty.** Therefore, at this document's date, every `harness-execution-attested-v0`
  edge is inadmissible, and the relation closes nothing. This clause is the honest statement of the
  gap, not a placeholder to be quietly deleted: the set gains a member only by an explicit
  repository-owner acceptance naming a root and the evidence that qualified it.
- `HAR-V0-005`: **a root MUST NOT be admitted merely because it is a separate process.** A same-UID
  caller can start any local process, hand it any inputs, and read or forge its outputs, which is
  precisely the boundary `docs/specs/test-claim-qualification-v0.md:42-44` records. Admission
  requires a mechanism whose forgery a same-UID caller cannot perform. No such mechanism exists in
  this repository, and adding one means leaving the local-only, daemon-free, network-free boundary of
  `docs/specs/agent-harness-integration-v0.md:261-263`. That trade is Q1.
- `HAR-V0-006`: signing does not qualify a root. `CF-V0-014` already forecloses it (`docs/specs/change-frontier-v0.md:193-197`), and
  the reason is structural, not cryptographic: on one UID the signer, the key, and the signed bytes
  are all reachable by the same actor.
- `HAR-V0-007`: an inadmissible attestation MUST be reported, not dropped, in the `CF-V0-032` form
  (`docs/specs/change-frontier-v0.md:373-389`): the edge keeps its subject, command identity, and
  reason, and only its authority label changes — to `unverified-attestation`, with the item's
  `resolutionClass` staying `AUTHORITY_REQUIRED` and its `nextAction` staying
  `ESTABLISH_HARNESS_AUTHORITY`. Reporting it as absent, as caller-reported, or as an operational
  failure asserts something the verifier did not determine, which `CF-V0-025` forbids (`:336-339`).

### Closure, if a root is ever accepted

- `HAR-V0-008`: with a root accepted, an admissible edge removes an `INTENT_TEST` item only when the
  attested rows match every OCM-selected claim edge for that obligation at the target, under the
  quantifier the admitting profile declares. `CF-V0-013` forbids inferring a quantifier
  (`:189-192`); this relation inherits that prohibition and MUST NOT default to `ANY`.
- `HAR-V0-009`: closure remains scoped to execution. An attested passing run establishes that the
  named rows ran and passed at the named target under the named command. It does not establish
  adequacy, coverage, requirement satisfaction, or correctness, and MUST NOT be rendered as any of
  them.
- `HAR-V0-010`: `HARNESS_ATTESTED` MUST NOT be reachable by policy. `CF-V0-005` excludes
  acknowledgement and permissive policy from `strict-v0` (`:105-109`), and an acknowledgement path that
  produced this class would make the class meaningless. If the owner chooses policy acknowledgement
  instead of authority (Q2), that is a **different** mechanism with a different class and a visible,
  attributable acknowledger — it MUST NOT borrow this one.

### Non-goals

- `HAR-V0-011`: this relation adds no daemon, service, account, network call, key store, remote
  attestation client, or telemetry to Corvint by itself. Every one of those is a consequence a root
  might require, and each is Q1's subject, not something this document grants.

## What this permits that is not permitted today, and what stays forbidden

**Newly permitted:** nothing, today. That is the finding. With `HAR-V0-004`'s set empty, a harness
gains no new power, and the relation's only present function is to make the missing component
nameable: not "a relation", but *an execution authority root Corvint has no way to have*.

**Newly permitted if and only if a root is accepted:** a harness's stop lifecycle point could receive
a frontier in which `INTENT_TEST` items for attested obligations are absent, and could therefore
report a state other than `UNAVAILABLE`.

**Still forbidden in every case:** blocking or continuing a host beyond what repository policy and
the host permission model allow (`AHI-006`, `docs/specs/agent-harness-integration-v0.md:108-110`);
persisting prompts or task text (`AHI-005`, `:106-107`); claiming an empty frontier or a completed stop
hook while any non-closing evidence remains (`CF-V0-027`, `:397-404`); and treating a Corvint harness
event as proof that a merge or test succeeded (`AHI-007`, `:113-114`).

## Conformance and adversarial matrix

| Fixture | Required result |
|---|---|
| any `harness-execution-attested-v0` edge, accepted root set empty | inadmissible; item stays OPEN; `unverified-attestation` per `HAR-V0-007` |
| attestation missing target OID, command, rows, exit, or root identity | `invalid-harness-attestation`; never a weaker edge |
| attestation signed with a locally generated key | inadmissible; `HAR-V0-006` |
| attestation replayed from a different target revision | inadmissible; target binding fails |
| `tcq/0` bytes relabelled as this relation | rejected; `HAR-V0-001` |
| this relation presented to a `frontier/0` verifier | `noncanonical-frontier` |
| a consumer rendering `HARNESS_ATTESTED` as `CALLER_REPORTED` | refusal, not downgrade; `HAR-V0-002` |
| policy acknowledgement attempting to mint this class | rejected; `HAR-V0-010` |

## Evaluation and kill criteria

- This relation MUST NOT be implemented while `HAR-V0-004`'s set is empty. Implementing an inert
  relation adds a wire, a class, and a conformance surface in exchange for no observable change, and
  it creates the standing temptation to weaken `HAR-V0-005` to make the investment pay.
- If the owner declines Q1 (keep the local-only boundary), this relation should be **rejected**, not
  parked, and `CF-V0-027` should be amended to say that the test half of the frontier closes by
  attributable policy acknowledgement or not at all. A spec that waits forever on a component the
  product boundary forbids is worse than a recorded refusal.
- One relabelling of caller-reported evidence as attested is an immediate release blocker, on the
  same footing as `CF-V0`'s existing blocker list (`:449-452`).

## Simpler baseline and YAGNI cuts

The baseline is what ships today: the `INTENT_TEST` item stays, `nextAction` stays
`ESTABLISH_HARNESS_AUTHORITY`, and the harness reports `frontier.state: UNAVAILABLE`. That baseline
is honest and costs nothing. Every alternative that removes the item without a root is a lie, and
every alternative that keeps the item while adding this relation is machinery with no observable.

## Unresolved decisions deliberately deferred

- The attestation wire, its canonical encoding, and its portability across hosts. There is no point
  freezing bytes for a root that does not exist.
- The quantifier (`HAR-V0-008`), which `CF-V0-013` already defers and which needs an OCM wire change.
- Whether an attested failing run is different from an unattested one for frontier purposes. It is a
  real question — an attested failure is stronger evidence than no evidence — but it changes reason
  ordering, and reason ordering is frozen per profile.

## Questions only the repository owner can answer

- **Q1. Is Corvint willing to leave the local-only, daemon-free, network-free, single-UID boundary in
  order to gain an execution authority root?** This is the whole question. Everything else in this
  document is downstream of it. Concretely: a root implies at minimum a separate trust domain — a
  different UID, a container or VM boundary, an OS attestation facility, or a remote service. Each
  contradicts `docs/specs/agent-harness-integration-v0.md:261-263` and, depending on the choice,
  `CF-V0-026` (`:340-344`).
- **Q2. If the answer to Q1 is no, should the test half of the frontier close by attributable policy
  acknowledgement instead?** `CF-V0-005` excludes acknowledgement from `strict-v0` and `CF-V0`'s
  deferred list says acknowledgement waits "because current wires have no authority capable of
  expressing them" (`:486-487`). An acknowledgement wire is buildable inside the current boundary;
  an authority root is not. But it changes the product claim from "verified" to "someone signed off",
  and that is an owner call, not an engineering one.
- **Q3. If the answer to both Q1 and Q2 is no, should `CF-V0-027` be amended to say so?** As written
  it implies a wait for work that can be done. If neither path is taken, the honest text is that
  `INTENT_TEST` never closes and the harness's `UNAVAILABLE` is permanent, not provisional.

## Traceability and rollback

| Requirement | Planned surface | Required evidence |
|---|---|---|
| `HAR-V0-001..002` | relation and class constants under a new TCQ profile | a `tcq/0` or `frontier/0` document carrying either is refused |
| `HAR-V0-003..007` | admissibility gate | every row of the matrix above; in particular the empty-root-set row, which is the current behaviour and must be asserted rather than assumed |
| `HAR-V0-008..010` | closure path | `NOT_RUN`, and MUST stay `NOT_RUN` while the accepted root set is empty |
| `HAR-V0-011` | boundary | absence of daemon/network/key-store dependencies |

Rollback is trivial by construction: nothing observable depends on this document. Rejecting it costs
only the amendment named in Q3.
