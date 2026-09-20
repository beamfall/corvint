# Frontier decision brief — 2026-08-29

Owner: Russell Lewis (repository owner). **This document decides nothing and ratifies nothing.** It
is a reading aid for the eleven questions parked in the three frontier drafts, so they can be
answered in one sitting. It amends no clause, and where it says a clause already settles something it
cites the clause rather than asserting it.

Intent status: not-applicable (reading aid; decides nothing)
Delivery status: not-applicable

Scope: the "Questions only the repository owner can answer" sections at
`docs/specs/change-frontier-profile-1.md:315`, `docs/specs/change-witness-relation-v0.md:252`, and
`docs/specs/harness-authority-relation-v0.md:219`.

## Agent digest
- Claim: This non-authoritative brief deduplicates seven owner-only Frontier decisions without ratifying any clause.
- Status: not-applicable (reading aid; decides nothing)/not-applicable
- Exists: nothing executable — seven deduplicated owner questions and cost evidence only.
- Blocked on: repository-owner answers to the seven questions.
- Read next: `change-frontier-profile-1.md`, `change-witness-relation-v0.md`, and `harness-authority-relation-v0.md`.

**Ordering rule observed throughout.** No recommendation below rests on "the implementation already
does it this way". Where a code fact appears it is evidence of *cost* — what exists, what would have
to be built — never of correctness. Two places where the distinction is easy to blur are flagged
inline.

---

## 1. The eleven questions are seven

| Deduped | Raw sources | Note |
|---|---|---|
| **D1** definition resolver | `change-frontier-profile-1.md:317`, `change-witness-relation-v0.md:254` | Identical. The profile draft says so itself: "(`CWR-V0`'s Q1.)" |
| **D2** new authority classes | `change-frontier-profile-1.md:321`, `change-witness-relation-v0.md:259` | The profile bundles two classes; the relation asks about one. **They should not be answered together — see D2.** |
| **D3** self-authored obligations | `change-frontier-profile-1.md:324`, `change-witness-relation-v0.md:262` | Identical |
| **D4** stop-point inputs | `change-frontier-profile-1.md:328` | Unique. The profile calls it "the question `CF-V0-027` does not ask" |
| **D5** execution authority root vs. the trust boundary | `harness-authority-relation-v0.md:221` | Unique |
| **D6** policy acknowledgement instead | `harness-authority-relation-v0.md:227` | Unique |
| **D7** amend `CF-V0-027` | `change-frontier-profile-1.md:333`, `harness-authority-relation-v0.md:233` | Identical |

## 2. Two findings that change how several questions read

**(a) Almost nothing being cited as a constraint is ratified.** `change-frontier-v0.md:8-11` states
that ratification is per clause and that **only `CF-V0-031` and `CF-V0-032` are accepted**; "every
other clause keeps the document's `proposed` status". So `CF-V0-014`, `CF-V0-017`, `CF-V0-026`,
`CF-V0-027`, and `CF-V0-029` — the five clauses the drafts lean on hardest — are proposed text, not
policy. Two consequences:

- The permanence arguments in this brief come from **shipped wire bytes**, not from ratified clause
  text. `frontier/0` is implemented and carries 62 vectors plus 28 adversarial fixtures
  (`change-frontier-v0.md:15-17`); that is what makes a profile identifier or an enum member
  expensive to take back, and it is true whatever the clause status is.
- If you want the additive-evolution rule to actually bind the answers you give below, **ratifying
  `CF-V0-029` is a separate, cheap act** and is a prerequisite for D2's cost model to mean anything.

**(b) An accepted clause already requires what a proposed clause defers.** `AHI-006` — in a document
whose intent status is "accepted direction" (`agent-harness-integration-v0.md:5`) — says "Before a
platform declares a change task complete, its safest available stop lifecycle point MUST request an
Corvint frontier check" (`:108-109`). `CF-V0-027` — proposed — says "Harness integration waits for
separately accepted change-witness and harness-authority relations under a new Frontier profile"
(`change-frontier-v0.md:397-398`). These point opposite ways, and the one pointing at *do it* has the
stronger status. Note the exact wording differs ("accepted direction" vs. decision 0006's `accepted`);
if you intend one to bind, say which.

This asymmetry is the reason D4 is the highest-leverage question in the set.

## 3. What existing clause text already answers

| Question | Already settled by | What is left |
|---|---|---|
| D3 | **`CF-V0-031`, accepted** (`change-frontier-v0.md:5-11`, `docs/decisions/0006-caller-authored-authority.md`), principle at `:358-359` | Only cost acceptance. One of the three options contradicts an accepted clause. |
| D1 | `CWR-V0-006` (`change-witness-relation-v0.md:150-155`) already forbids "a persisted index built at another revision, a dirty worktree, or a cache whose freshness the universe does not bind" | Not "may we use Corvint's index" — that is already no — but "may we share parser code" |
| D5 | `HAR-V0-005` (`harness-authority-relation-v0.md:124-129`) + `CF-V0-014` (`change-frontier-v0.md:193-197`) already answer that a separate local process and a signature are not roots | Only the off-host case, which the draft does not separately examine |
| D6 | `HAR-V0-010` (`harness-authority-relation-v0.md:150-155`) + `CF-V0-005` (`change-frontier-v0.md:105-109`) already forbid acknowledgement minting `HARNESS_ATTESTED` | Only whether a separate, differently-classed acknowledgement mechanism should exist |
| D2 | `CF-V0-029` (`change-frontier-v0.md:402-404`) already permits additive authority semantics under a new identifier — **in proposed text** | Which classes, and when |

D4 and D7 are untouched by any existing clause.

---

## D1 — May the definition resolver be Corvint's own index?

**In plain terms.** To say "this change touched the thing the requirement names", something has to
turn a name in a requirement into a region of a file. Should that something be the search index Corvint
already builds, or a small piece of machinery that lives inside the Frontier verifier?

> "May a Frontier verifier depend on a Corvint-built symbol resolver?"
> — `docs/specs/change-frontier-profile-1.md:317`; identically `docs/specs/change-witness-relation-v0.md:254`

The drafts frame this as existential: "This decision determines whether the relation is implementable
at all" (`change-witness-relation-v0.md:257-258`). **That framing is too strong**, for a reason
internal to the drafts rather than to the code:

- `CWR-V0-006` (`change-witness-relation-v0.md:150-155`) already forbids consulting a persisted index
  built at another revision or a cache the universe does not bind. Corvint's index is exactly such an
  artifact. So the relation's *own* clause forecloses the index-reuse reading before `CF-V0-026`
  (`change-frontier-v0.md:340-344`) is consulted at all.
- What the relation needs is a **definition span**, "the exact `(path, blobOid, startLine, endLine)`"
  (`change-witness-relation-v0.md:95-96`). *Cost evidence, not authority:* the index's `Symbol` record
  is `{Kind, Name, Path, BlobHash, Line}` (`internal/contextindex/index.go:186-188`) — a definition
  line, no end line — and no exported build entry point accepts a revision (`Build(ctx, root)`,
  `BuildEval(ctx, root)`, `BuildQuery(ctx, root, text)` at `internal/contextindex/index.go:268,294,387`;
  the tree comes from `ls-tree ... identity.treeRevision`, `internal/contextindex/git.go:370`). So
  "reuse the index" would not shorten the work much even if permitted.

**Options.**

1. **Span resolver inside the Frontier library, from target-revision Git objects.** Commits Corvint to
   per-language end-line derivation. Forecloses nothing. `CF-V0-026` and `CWR-V0-006` both stand
   unamended. Build cost: end-line derivation per supported grammar, plus the token scan
   `change-witness-relation-v0.md:244-246` already scopes; the existing grammar code
   (`internal/contextindex/parse.go`, `pythonsymbols.go`) is available as an ordinary library
   dependency without importing the index.
2. **Read `CF-V0-026` narrowly and let the verifier call the Corvint index.** Requires *also* weakening
   `CWR-V0-006`, because an index built at repository HEAD is a persisted index at another revision
   whenever `--target` is not HEAD. Commits the Frontier verifier to an index-freshness dependency the
   universe ID does not bind — the specific hazard `CWR-V0-006` was written against. Build cost is
   **not lower**: end lines and a revision-parameterised build are needed either way.
3. **Reject `ocm-change-witnessed-v0`.** Zero build cost; `frontier/0` behaviour stands.

**Hard to reverse.** Almost nothing. D1 ships no identifier, no enum member, no receipt field. It
chooses where code lives. Of the seven questions it is the **least permanent**, and it should not be
treated as the gate the drafts make it look like.

**Recommendation: option 1.** Not because code is arranged that way today — it is not; nothing under
`internal/frontier` or `internal/lrfrepo` references `internal/contextindex` — but because option 2
buys no build saving and costs an amendment to `CWR-V0-006`, a clause whose whole subject is
freshness binding. Re-pose the question as **"may Frontier share parser code, given that it may not
share the index?"** Sharing a grammar is not retrieval, a database, a daemon, or a cache, so
`CF-V0-026` does not reach it, and no amendment is needed to answer yes.

---

## D2 — Two new authority classes, or one?

**In plain terms.** Every frontier item is stamped with how much its evidence is worth. Today there
are three stamps. The drafts want to add two: one for facts the verifier recomputed itself, one for
facts a trusted test-runner attested. Should both ship, and should they ship together?

> "Are `VERIFIER_DERIVED` and `HARNESS_ATTESTED` acceptable as fourth and fifth authority classes?"
> — `docs/specs/change-frontier-profile-1.md:321`
> "Is `VERIFIER_DERIVED` an acceptable fourth authority class?"
> — `docs/specs/change-witness-relation-v0.md:259`

**This is two decisions wearing one coat, and they do not have the same answer.**

- `VERIFIER_DERIVED` (`CWR-V0-003`, `change-witness-relation-v0.md:114-120`) gets instances the moment
  `ocm-change-witnessed-v0` exists.
- `HARNESS_ATTESTED` (`HAR-V0-002`, `harness-authority-relation-v0.md:109-112`) gets instances only
  when the accepted root set is non-empty — and `HAR-V0-004` states it is empty and that the document
  does not add to it (`:119-123`). The same draft's kill criteria say the relation "MUST NOT be
  implemented while `HAR-V0-004`'s set is empty" (`:193-195`).

So shipping `HARNESS_ATTESTED` in `frontier/1` ships a permanent enum member with **zero reachable
instances**, while `CF-V1-007` (`change-frontier-profile-1.md:142-145`) makes every consumer refuse a
document unless it can distinguish all five. Every consumer pays to implement a class that cannot
appear, and — if D5 is answered "keep the boundary" — never will.

**Options.**

1. **Ship both in `frontier/1`.** Commits every present and future consumer to a five-member
   enumeration. Forecloses removing the unreachable member: dropping it later is not additive, it is
   changing `frontier/1` in place, which is exactly the shape `CF-V0-029` forbids
   (`change-frontier-v0.md:402-404`). **Irreversible.**
2. **Ship `VERIFIER_DERIVED` only.** `HARNESS_ATTESTED` arrives with `frontier/2` if and when a root
   is accepted. Cost: a profile increment later — *conditional* on D5 being "yes". Build cost of the
   increment is the `CF-V1-001..004` identity work over again, which the profile draft already
   scopes; `CF-V0-029` exists to make exactly that cheap. **Reversible forward.**
3. **Ship neither** (reject `frontier/1`). Zero cost, zero gain.

**Hard to reverse.** The enum member itself, and the `frontier/1` identifier that carries it. Once a
`frontier/1` document exists in the wild with five classes, four is a different profile.

**Recommendation: option 2, decisively.** The asymmetry settles it: option 1's cost is unconditional
and permanent; option 2's cost is conditional on a future the `harness-authority` draft argues
against. Paying a profile increment you might not owe is strictly better than shipping a class you
may never be able to populate. Pair this with ratifying `CF-V0-029`, or the rule that makes the
increment legitimate is itself unratified.

---

## D3 — Can an obligation written in the same change set ever be witnessed?

**In plain terms.** If you write the requirement and the code that satisfies it in one change, may
Corvint say the requirement was witnessed? The drafts say no — and on this repository that means
essentially never, so Corvint cannot use the feature on itself.

> "Is a per-obligation withholding that fires on essentially every Corvint self-change acceptable?"
> — `docs/specs/change-frontier-profile-1.md:324`; identically `docs/specs/change-witness-relation-v0.md:288`

**The principle is already ratified.** `CF-V0-031` is `accepted` (`change-frontier-v0.md:5-11`, under
`docs/decisions/0006-caller-authored-authority.md`) and states it directly: "a governing document
confers authority on a change set only when it is resolved from a revision the caller did not author"
(`:358-359`). `CWR-V0-005` (`change-witness-relation-v0.md:138-149`) is that principle applied to the
object being witnessed rather than to a cited authority.

**Options.**

1. **Accept the withholding as drafted.** Commits Corvint to a relation whose closure rate on its own
   repository is near zero. Forecloses self-evaluation. Build cost: none beyond the relation.
2. **Resolve the requirement at base/merge-base and witness against that text** — the refinement
   `CF-V0-031` itself names as permitted under a new identifier (`change-frontier-v0.md:370-372`), and
   which `change-frontier-profile-1.md:308-311` defers to `frontier/2`. Build cost: a second
   resolution path plus a profile increment.
3. **Drop the guard.** Contradicts accepted `CF-V0-031` and reopens the laundering hole decision 0006
   closed. Requires amending an accepted clause — a materially heavier act than accepting a draft.

**The finding worth your attention: option 2 does not fix the complaint.** The dogfood problem is
that Corvint's clauses are *introduced* alongside their implementation, not modified. `CWR-V0-005`
already withholds when "the intent path is absent at base, or the obligation ID is absent at base"
(`change-witness-relation-v0.md:140-142`), and base-resolution inherits that: there is no base text to
witness against for a requirement that did not exist. Option 2 buys witnesses only for *modified*
requirements — precisely the case the dogfood objection is not about. The two options differ far less
than the question's framing implies.

**Hard to reverse.** `SELF_AUTHORED_OBLIGATION` is a new reason string whose ordering position
`CF-V1-008` fixes ahead of `SUBJECT_TERM_BOUND_EXCEEDED` (`change-frontier-profile-1.md:146-150`), and
reason order is frozen per profile. Shipped, its position in `frontier/1` is permanent.

**Recommendation: option 1, and reframe.** The real decision is not "is the withholding acceptable" —
an accepted clause has already answered that — but **"is Corvint's own repository a valid evaluation
cohort for this relation?"** No, by the relation's own kill criteria
(`change-witness-relation-v0.md:230-233`). That has a consequence the drafts do not price: `frontier/1`
must be evaluated on the sealed external cohort, and `change-frontier-v0.md:17-19` records that cohort
as **already unavailable** — "no artifact for the sealed 20-PR cohort this document and
`verified-absence-frontier-v0.md` both require could be located in this repository". So answering D3
"accept" means committing to source that cohort before `frontier/1` can be evaluated at all. That, not
the withholding, is D3's real cost.

---

## D4 — Where does a stop hook get the four inputs a frontier needs?

**In plain terms.** When an agent finishes and the stop hook fires, Corvint would have to compute a
frontier to say anything useful. Computing one needs four things the stop event does not carry. Who
supplies them, and how?

> "Where does a stop lifecycle point get a CEM 0.2 artifact, a bound OCM artifact, an independent
> expected base, and an independent target?"
> — `docs/specs/change-frontier-profile-1.md:328`

*Cost evidence, not authority:* a `stop` event admits exactly `{stopHookActive, changedPaths}` plus
session binding (`internal/gokernel/harness.go:186-187`); the frontier surface requires
`--cem MAP --ocm MAP --expected-base REV --target REV`, with base and target "never inferred"
(`cmd/corvint/help.go:463-470`); and `AHI-014` restricts wrappers to "the minimum event fields
admitted by `corvint-harness-event/0`" (`agent-harness-integration-v0.md:140-142`).

One option that looks available is not: the harness cannot recover the session's own start revision,
because it persists nothing — `outcome-persistence-unavailable` is emitted whenever a session-end
carries an outcome (`internal/gokernel/harness.go:398-401`). Every receipt binds `commitRevision` and
`treeRevision` (`internal/gokernel/repository.go:31-39`, `:43-51`), but nothing stores them for later.

**Options.**

1. **Read a committed CEM/OCM pair from a declared repository path.** The convention exists
   (`.corvint/change.cem.json`, `.corvint/change.ocm.json` — `cmd/corvint/ocm.go:247`,
   `cmd/corvint/help.go:816@e432ec24`, workflow at `README.md:188-198@3297e31e`). Cost: this supplies two of four inputs.
   Taking base and target *from the artifact* is what `CF-V0-001` calls inferred revision authority and
   fails `unsupported-frontier-context` (`change-frontier-v0.md:82-87`), so this option is incomplete
   on its own and must be paired with option 2 for the revisions.
2. **Grow the `stop` event contract** to admit `cemPath`, `ocmPath`, `expectedBase`, `target`.
   Commits Corvint to a new `corvint-harness-event` profile identifier and re-pinning four adapters.
   Forecloses nothing. Honest about who is speaking: the host declares its own scope, and the
   declaration is recorded rather than guessed — which is the disposition `CF-V0-001`'s "never
   inferred" is asking for.
3. **Compute nothing at stop; keep `UNAVAILABLE`.** Zero cost. Leaves `AHI-006`
   (`agent-harness-integration-v0.md:93-95`) unsatisfied indefinitely, under a document whose intent
   status is "accepted direction".
4. **Compute a frontier only when the artifacts and revisions are present, and report a named gap
   otherwise.** Option 2's contract plus a fail-visible rule, so the common case (no CEM authored) is
   reported as a missing declaration rather than as an unavailable frontier.

**A caution that belongs on the record.** Whoever supplies base and target at a stop point is the
party whose work is being judged — the same structural shape as D3 and D5. This is *not* an
authority-laundering hole of the `CF-V0-031` kind, because the CEM is a caller declaration by
construction and the verifier binds it to Git objects; but base/target selection does determine what
is in scope, so the declaration must be recorded in the receipt, not merely consumed.

**Hard to reverse.** The event-contract fields and the harness-event profile identifier. Also the
corpus migration, which `change-frontier-profile-1.md:208-265` has already measured: 9 of 18 parity
cases change bytes; `src/` is frozen so the change produces a divergence-register entry with a
spec-authored expectation rather than a rebaseline (`conformance/divergence-register.md:13,15`); and
`conformance/perf-v0/results/**` must **not** be rewritten, because those are dated observation
records.

**Recommendation: option 4.** It is the only answer that changes the receipt the degradation lives in,
and it does so without touching a single relation, authority class, or trust boundary. Note precisely
what it does and does not do: a computed `frontier/0` at stop yields `OPEN` with items, so
`frontier.state` stops being `UNAVAILABLE` **when a frontier was actually computed** — which is what
`CF-V1-019` permits (`change-frontier-profile-1.md:195-198`) — while the `INTENT_TEST` authority gap
remains and keeps a degradation naming it. Do **not** redefine `UNAVAILABLE` to mean something else:
`UNAVAILABLE` keeps its current meaning, "no frontier computed", per the standing rule that a shipped
state is never renamed (`docs/specs/live-proof-carrying-verification-v0.md:121-122@e6efc1a1`).

---

## D5 — Leave the local-only boundary to get an execution authority root?

**In plain terms.** For Corvint to say "the tests really ran and really passed", something has to run
them that the author cannot impersonate. On a single-user laptop with no daemon and no network, that
thing cannot exist. Do you give up the laptop-only promise to get it?

> "Is Corvint willing to leave the local-only, daemon-free, network-free, single-UID boundary in order
> to gain an execution authority root?"
> — `docs/specs/harness-authority-relation-v0.md:221`

Two sub-questions are already closed: a separate local process is not a root (`HAR-V0-005`,
`harness-authority-relation-v0.md:124-129`) and a signature is not a root (`HAR-V0-006`, `:130-132`;
`CF-V0-014`, `change-frontier-v0.md:193-197`). What remains is only the boundary trade.

**Options.**

1. **Keep the boundary.** `INTENT_TEST` never closes. Zero build cost. Forecloses an empty frontier
   permanently, and makes `CF-V1-006` (`change-frontier-profile-1.md:134-141`) a permanent statement
   rather than a provisional one.
2. **Leave it for a local root** — separate UID, container, VM, or OS attestation facility. Commits
   Corvint to a privileged component on the developer's machine and to per-OS attestation surfaces.
   Contradicts `agent-harness-integration-v0.md:179-181` directly. Large, ongoing, per-platform cost.
3. **Off-host root: accept an attestation produced by CI**, and keep the verifier local. The drafts do
   not examine this separately, and the finding that "there is no root" is scoped to *same-UID local*
   roots. `HAR-V0-006`'s reason for rejecting signatures is structural — "on one UID the signer, the
   key, and the signed bytes are all reachable by the same actor" (`docs/specs/harness-authority-relation-v0.md:130-132`) — and it does not apply
   when the signer is a CI provider the developer's UID cannot reach. `CF-V0-026`
   (`change-frontier-v0.md:340-344`) requires the *library and verifier* to work without network or
   service; a verifier that checks bytes handed to it makes no network call, so the clause is arguably
   satisfied unamended. CI already appears in Corvint's vocabulary as an independent reference, though in
   a different role — an oracle to shadow against, not an authority
   (`docs/specs/live-proof-carrying-verification-v0.md:442-448@d51660e9`, `:484@50bde519`). Build cost is real and large: an
   attestation wire and canonical encoding (explicitly deferred at
   `harness-authority-relation-v0.md:212-213`), key distribution, rotation, and revocation.
4. **Tier it:** closure exists where a repository already has an accepted root; local-only deployments
   keep the item open. `HAR-V0-004` already defines the accepted root set as "the set of authority
   roots the repository owner has explicitly accepted" (`harness-authority-relation-v0.md:119-123`) —
   nothing requires every deployment to have one.

**Hard to reverse.** Option 2 and option 3 both add a trust dependency that, once a product claim
rests on it, cannot be withdrawn without withdrawing the claim. The attestation wire's canonical
encoding is a permanent byte format.

**Recommendation: option 1 now; record option 3 as the only viable root shape and defer it.** Two
reasons. First, option 3's cost is the entire attestation programme the draft deliberately defers, and
it is not on the path to anything else in this set. Second and decisively for sequencing: **a CI root
does not help the stop hook**, because a stop hook fires before CI runs. So even paying option 3 in
full leaves the receipt the owner is asking about unchanged. If closure ever matters more than local
purity, option 3 tiered as option 4 is the shape to build; it is not this quarter's shape.

---

## D6 — Close the test half by attributable acknowledgement instead?

**In plain terms.** If nothing can prove the tests ran, should Corvint let a named human say "I checked"
and treat that as closing?

> "If the answer to Q1 is no, should the test half of the frontier close by attributable policy
> acknowledgement instead?"
> — `docs/specs/harness-authority-relation-v0.md:227`

Already closed: acknowledgement may not mint `HARNESS_ATTESTED` (`HAR-V0-010`,
`harness-authority-relation-v0.md:150-155`), and `strict-v0` has no acknowledgement input at all
(`CF-V0-005`, `change-frontier-v0.md:105-109`). `CF-V0`'s deferred list gives the reason acknowledgement
waits: "current wires have no authority capable of expressing them" (`:486-487`).

**Options.**

1. **No.** The item stays open. Zero cost. `UNAVAILABLE`/`OPEN` remains the honest report.
2. **Yes** — a separate acknowledgement mechanism with its own class and a visible, attributable
   acknowledger. Commits Corvint to a policy engine, an identity notion, and a stored acknowledgement
   record — every one of which the profile draft lists among what `frontier/1` adds none of
   (`change-frontier-profile-1.md:303-304`). Changes the product claim from "verified" to "someone
   signed off", which the draft correctly calls an owner call rather than an engineering one
   (`harness-authority-relation-v0.md:231-232`).
3. **Neither close nor acknowledge — report.** Leave `INTENT_TEST` open exactly as today, and let the
   harness surface the frontier it computed so the maintainer reads "N open items, here they are"
   instead of "UNAVAILABLE". This is D4 with a wiring clause; it adds no class, no policy, and no
   acknowledger.

**Hard to reverse.** Option 2 introduces an acknowledgement wire and an acknowledger identity. Both
are permanent once a receipt carries them, and an acknowledgement mechanism is very hard to remove
once teams depend on it to ship.

**Recommendation: option 3, then option 1 for the residual.** Option 3 delivers the entire *observable*
benefit people actually want from acknowledgement — knowing what is outstanding at the moment work
stops — without buying an authority Corvint would then have to defend. Answer the literal question "no"
(option 1), and answer the underlying want with D4.

---

## D7 — Should `CF-V0-027` be amended?

**In plain terms.** `CF-V0-027` currently reads as though harness integration is waiting for two
relations to be written. If those relations are not coming, or would not be enough, the clause should
say what is actually true.

> "If the answer to `harness-authority-relation-v0.md`'s Q1 and Q2 is 'no' … should `CF-V0-027` be
> amended to say the `INTENT_TEST` half never closes?"
> — `docs/specs/change-frontier-profile-1.md:333`; similarly `docs/specs/harness-authority-relation-v0.md:233`

**Options.**

1. **Amend to record permanence:** `INTENT_TEST` never closes; `UNAVAILABLE` is a documented product
   boundary. Honest if D5 is option 1 *and* option 3 is abandoned. Forecloses the tiered path
   rhetorically even though nothing technical stops it.
2. **Amend to record the root shape:** `INTENT_TEST` closes only by an accepted execution authority
   root, the only known shape is off-host, and it is deferred. Keeps D5 option 3/4 open without
   promising it.
3. **Leave as written.** Costs nothing today; leaves a clause that reads as a schedule for work
   nobody intends.

**A second amendment, independent of D5 and D6, and the more important one.** `CF-V0-027`'s sentence
"Harness integration waits for separately accepted change-witness and harness-authority relations
under a new Frontier profile" (`change-frontier-v0.md:397-398`) states a **precondition and implies a
completion condition**. Accepting both relations would satisfy everything that sentence asks for and
still leave `AHI-006`'s required frontier check unperformed, because neither relation says anything
about how a lifecycle point obtains `CF-V0-001`'s four inputs — that is D4, and the profile draft
notes it is "the question `CF-V0-027` does not ask" (`change-frontier-profile-1.md:332`). *This is
an argument about the clause's coverage against `AHI-006`, not an argument from what the code does; it
holds whatever the harness is currently wired to.*

**Hard to reverse.** Nothing. `CF-V0-027` is proposed (`change-frontier-v0.md:11`); amending proposed
text is the cheapest act in this brief.

**Recommendation: option 2, plus the second amendment.** Say that harness integration waits on three
things, not two — the two relations *and* a stop-point input contract — and that of the three only the
input contract is on the near path. That single edit converts D4 from something that looks
out-of-order into the legitimately next work item.

---

## Dependency order

```
  D4  (stop-point inputs)  ── no predecessor ──► the only answer that changes a receipt


  D5  (authority root) ──► D6 (acknowledge instead) ──► D7 (amend CF-V0-027)
        │
        └───────────────┐
                        ▼
  D3  (self-authored) ──► D2 (which classes) ──► D1 (resolver locus)
```

**Strict precedences.**

- **D5 → D6 → D7.** D6 is conditional on D5 by its own text ("If the answer to Q1 is no",
  `harness-authority-relation-v0.md:227`); D7 is conditional on both ("If the answer to both Q1 and
  Q2 is no", `:233`). Answering D6 or D7 before D5 answers nothing.
- **D5 → D2.** Whether `HARNESS_ATTESTED` can ever have an instance is D5's answer. If D5 is "keep the
  boundary", D2's option 1 ships a permanently unreachable enum member and is simply wrong.
- **D3 → D1 → build.** D3 decides whether `ocm-change-witnessed-v0` is worth having; D1 only decides
  where its resolver lives. Answering D1 before D3 spends a decision on a relation you may not want.
- **D3 → D2.** `VERIFIER_DERIVED` exists only to label this relation's output (`CWR-V0-003`,
  `change-witness-relation-v0.md:114-120`). No relation, no class.

**D4 has no predecessor.** It shares no input with any other question. It is answerable today.

**Single highest-leverage answer: D4.** It is the only question in the set whose answer changes a byte
of the thing the repository owner has been asking about; it is required by an accepted-direction clause
(`AHI-006`, `agent-harness-integration-v0.md:93-95`) that a proposed clause currently defers; it needs
no relation, no new authority class, and no change to the trust boundary; and it is the prerequisite
for the only good answer to D6. Answering D4 alone unblocks more than answering D1, D2, D3, D5, D6, and
D7 combined — all six of which, answered as recommended above, leave every receipt byte-identical.

**Questions that are the same question.**

- **D1 and D2 and D3 are one decision in three parts.** All three exist only if `ocm-change-witnessed-v0`
  ships. Answering "reject the relation" answers all three at once, and answering D3 "accept the
  withholding" makes D1 and D2 mechanical.
- **D6 is D5 restated as a product claim.** D5 asks whether a machine can vouch for the test run; D6
  asks whether a person may instead. They are the same closure slot with different fillers, which is
  why `HAR-V0-010` (`harness-authority-relation-v0.md:150-155`) has to forbid the second from wearing
  the first's label.
- **D7 is not a question so much as bookkeeping** for whatever D5 and D6 return — with one exception:
  the coverage amendment recommended above is independent of both and should be made regardless.
- **D3, D4, and D5 share one structural shape** — in each, the party supplying the evidence is the
  party being judged. That shape is fully resolved only in D3, by accepted `CF-V0-031`.

## What this brief did not verify

- Whether the sealed 20-PR cohort exists anywhere outside this repository. `change-frontier-v0.md:17-19`
  records it as not locatable here; D3's recommendation assumes that still holds.
- Any claim about `src/`. It was not read and not touched; `git status --short src/` is 0.
- The exact build cost of end-line derivation per grammar (D1 option 1). The gap is verified
  (`internal/contextindex/index.go:186-188`); the effort to close it is not estimated.
