# Decision 0007 — omnibus ratification of the 2026-08-29 brief

Date: 2026-08-29. Status: accepted. Authority: repository owner, verbatim instruction "just ratify
everything" (2026-08-29), given in response to `docs/RATIFICATION-BRIEF-2026-08-29.md`.

## Scope

The brief put fourteen items to the owner. Each carried a written recommendation. The owner ratified
the set. This record fixes what that means item by item, because "everything" is not self-executing:
three of the fourteen recommend *not* changing a clause, one is a question of fact that no
ratification can answer, and one was already resolved in code before the brief was read.

Each item below states its disposition, the clause it moves, and — where the ruling is anything
other than a plain yes — why the ruling is what it is rather than a blanket approval.

**This record is the authority.** Where an item requires spec text, that text is amended in the same
change as this record. Where an item requires implementation, the implementation is separately
tracked and cites this decision.

## Dispositions

### D1 — Beta rung: **RATIFIED**

`GPK-V0-042` (not `041`, which the brief predicted as free and which has since been taken) inserts a
beta rung between "opt-in candidate use" and "packaged as `corvint`" in `GPK-V0-023`'s fixed order.
Ratified narrowly, on the five terms the brief named:

1. **Install identity stays `corvint`.** The beta is not installed as `corvint`. `GPK-V0-015` is
   unweakened.
2. **A third matrix label, `BETA`,** distinct from both `FALLBACK` and `FULL`. **Clarified
   2026-08-29:** `GPK-V0-042` requires this of *reports*, not of receipts. The harness receipt's
   `support` field is oracle-compared and the oracle emits `FALLBACK` unconditionally from frozen
   `src/`, so a receipt asserting `BETA` could have no oracle-authored expectation for those bytes
   at all. The label therefore lives in the compatibility matrix and release reporting. This is
   what the clause already says; it is not a relaxation, and no register adjudication or receipt
   flag day is required to satisfy it.
3. **Per command and per integration surface,** reusing `GPK-V0-026`'s existing shape. There is no
   global beta.
4. **Admission bar:** the `GPK-V0-017` criteria, the W11 reproducible-build proof, and the W12
   dogfood and rollback runs. No new evidence class is invented.

   **Correction, 2026-08-29 (same day).** This term originally asserted "All three are already
   green." **That was false, and it was the load-bearing error in this ratification.** At the head
   this decision was written, none of the three was green:

   - `GPK-V0-017` is 11 PASS / 0 FAIL / 3 NOT_RUN with outcome `insufficient_evidence`, and every
     one of the six reports under `conformance/perf-v0/results/` carries that same outcome.
   - W11 is PASS on reproducibility but carries `pendingEvidence: ["W10-performance-GPK-V0-016-017"]`
     and smoke `NOT_RUN` on four of five targets, so `GPK-V0-018` execution evidence exists on
     darwin/arm64 alone.
   - W12 closes `GPK-V0-021` **NOT_PROVEN** overall, carries `NOT_APPLICABLE` and `PARTIAL` rows on
     `GPK-V0-022`, and leaves `GPK-V0-024` sentence 4 `NOT_PROVEN`.

   The brief was more careful than this record: it quoted "11 PASS / 0 FAIL / 3 NOT_RUN" accurately
   and this decision then paraphrased it as "already pass". Compressing a tally that contains
   NOT_RUNs into "green" is precisely the move `GPK-V0-017`'s closing sentence forbids — "Any
   threshold not measured is `NOT_RUN`, not waived."

   **The bar itself is unchanged and is not weakened by this correction.** What changes is the
   claim about how close the evidence already was: the answer is that zero commands qualified on
   the day the rung was ratified. Nothing was admitted on the false premise, because the admission
   record was built by reading the evidence files rather than this paragraph.
5. **Open register entries do not block beta admission.** An open entry blocks a command's
   *retirement contribution* (`GPK-V0-034` unchanged) but not its admission to beta, **provided the
   divergence is disclosed in the beta's own notes.**

Term 5 is the contestable one and the brief said so. The ruling is deliberate: a beta that must
first have zero known divergences is indistinguishable from the cutover gate it sits below, and
would make DR-0005 a beta blocker by inheritance. Beta here means *we tell you what diverges*, not
*nothing diverges*. Disclosure is therefore a condition of admission, not a courtesy.

This rung weakens no cutover gate. `GPK-V0-024`'s rollback requirement applies to it unchanged.

### D2 — Impact beyond `.go`: **RATIFIED**, as recommended — (a)=B, (b) `.py` first, then web

`GPK-V0-027`'s supported profile is widened from "admitted nested `.go` paths" to **admitted paths
for which this specification names a reverse-import resolution rule**, and the `.py` and web
(`.ts .tsx .js .jsx .mjs .cjs`) rules are named in the same amendment. Suffixes with no named rule
still receive a typed refusal; the clause keeps its fail-closed character.

**The five newly-admitted languages (`.rs .cs .swift .kt .kts .rb`) are explicitly NOT widened
into.** They have no oracle branch, so admitting them would create behaviour whose only authority is
the candidate — which `GPK-V0-033` forbids. They stay indexed and searchable. This is a limit on the
ratification, not an oversight in it.

### D3 — DR-0005: **re-measurement authorized; adjudication pre-approved as A**

The brief's recommendation was "re-measure first, then decide — and expect to choose A", because
DR-0005's mechanism section describes a query path that no longer exists. Ratifying "everything"
cannot ratify an adjudication over stale evidence, so this ruling has two parts:

- The re-measurement is authorized and required.
- Its adjudication is **pre-approved as A** — the `user-prompt` context block is bound by
  `GPK-V0-002` exact parity and MUST carry symbol results — **unless the re-measurement contradicts
  A**, in which case the result returns to the owner rather than defaulting to B.

That conditional is the whole content of the ruling. A blanket yes here would have ratified a
conclusion about a measurement nobody has taken.

### D4 — Symbol context window end-clamp: **RATIFIED, sequenced after D3**

The window is clamped to the declaration end. This changes receipt bytes, so the corpus expectation
recapture is part of the work and MUST come from the Python oracle only, never from candidate
output. Sequenced after D3 because the two share the window's definition.

The brief flagged that Go's window also starts one line earlier than Python's and that it could not
tell whether that was deliberate. **The start offset is not ratified.** Clamp the end; leave the
start alone and record it as a separate question.

**Amended 2026-08-29, same day, on measurement.** The separate question is answered and the answer
reverses this sub-ruling. The DR-0005 re-measurement probed both variants directly, applied and
reverted against a verified-clean tree:

- End-clamp alone — exactly what the paragraph above ratified — leaves the divergence open. The
  probe symbol still scores 120 against the oracle's 108, so stdout stays unequal.
- End-clamp **plus** the Python start offset produces a packet identical to the oracle in every
  field, `packet_bytes` included.

So **the start offset is load-bearing and is now ratified along with the end clamp.** The candidate
uses one window (`Line-4 … Line+20`) for every language; the oracle uses three — `-4` for Go, `-3`
for web, and `-3` for Python clamped to the declaration end. Matching them is a **port from the
oracle, not a candidate-authored change**, which is what makes it admissible under `GPK-V0-033` on
the same authority as any other ported behaviour. Web symbols carry the identical defect and were
not measured; they move with Python.

The original sub-ruling is left standing above rather than rewritten, because the reasoning behind
it was sound on the evidence available — the brief said plainly that it trusted the arithmetic and
not the intent — and it was overturned by taking the measurement it asked for, which is the process
working rather than failing.

### D5 — `query` limit range: **RATIFIED**

`GPK-V0-028` admits a wider limit range. The intent gate and the ASCII gate are **not** touched —
the brief recommended leaving them and this ratification does. New parity rows are captured from the
oracle.

### D6 — Rust analyzer family: **RATIFIED**

`docs/specs/rust-analyzer-candidate-v0.md` moves to `Intent status: accepted`, and `rust` is added
to the analyzer family list. Cheapest item on the list; serves a stated want.

### D7 — `analyzergo` and real `go.mod` versions: **RATIFIED as A**, range bounded and stated

The `Go | go.mod` row admits real patch versions over a **stated, bounded** range rather than the
single pinned version. A patch bump does not change the schema the analyzer reads.

The brief noted it could not find another agent's competing draft at that head and that such a draft
would supersede this. If one surfaces, it supersedes D7 and only D7.

### D8 — Harness `receiptId` basis: **RATIFIED as B — defer the flag day, document now**

The receipt covers the request, not the answer. The profile is **not** bumped to
`corvint-harness-event/1` now; the flag day across four adapters is deferred. What lands now is the
documentation fix, so the receipt stops reading as an integrity guarantee it does not provide.

This is a ratification of *not* changing the wire format. Recorded explicitly so it is not later
read as an oversight.

### D9 — `perf-v0` across an accepted divergence: **RATIFIED, narrowly**

The perf manifest gains an `acceptedDivergence` **keyed to DR-0004 by name**. It is not a general
escape hatch, and `structuralComparison` (decision 0005) remains deliberately not the mechanism.
Any future entry needs its own ratification.

This relaxes a preregistered predicate in `GPK-V0-016`, which is the integrity-sensitive direction.
It is ratified because the alternative is a threshold that is unmeasurable by construction, and a
disclosed relaxation is better than an unmeasurable claim.

### D10 — `.claude` in `forbiddenParts`: **RATIFIED as "leave until W15"**

Measured impact is zero files. Go's behaviour is the better one. No register entry is opened — one
would block `PASS` for query and eval over a difference nothing can observe. Added to the W15
checklist so it is not rediscovered.

### D11 — PNC-001: **already resolved; no ratification required**

Both halves are closed in code. The two stale memory entries that still describe it as open are
deleted.

### D12 — `CF-V0-004` `EMPTY`: **RATIFIED as B**

`EMPTY` and exit 0 are recorded as unreachable in V0 in `CF-V0-004`/`CF-V0-018`. `OCM-V0-001` is not
widened. Dead-but-documented is cheaper and safer than widening a canonical artifact format to reach
a state nothing needs.

### D13 — The sealed 20-PR cohort: **NOT RATIFIABLE — ruled non-blocking**

This is the one item a ratification cannot discharge. "Does the cohort exist outside this
repository?" is a question of fact about the world, and no clause can make the answer true. What is
ruled here is the *disposition*:

- No such artifact exists in this repository. That is verified, not assumed.
- `frontier/0` therefore stays `not-started` and is **not** a blocker on the beta rung, on cutover,
  or on any other item in this brief.
- Construction of a cohort is **not** scheduled. "Sealed before Corvint ran" cannot be re-established
  retroactively for any PR Corvint has already seen, so a cohort built now would be worth
  substantially less than one that already exists.

The existence question stays open as the single outstanding owner input in this brief. It blocks
only `frontier/0`'s own promotion-or-kill, which nothing else depends on.

### D14 — Go binary-archive format: **UNBLOCKED by D1; scheduled, not yet specified**

The brief recommended leaving this until D1 was answered, because the archive shape depends on how a
beta is distributed. D1 is now answered, so the dependency is discharged and D14 becomes actionable.
The format is not decided in this record — it follows from `GPK-V0-042`'s distribution shape and is
specified separately. `GPK-V0-020` stays UNVERIFIED for the Go artifact until it is.

## What this decision does not do

- It does not weaken any cutover gate. Every gate in `GPK-V0-023`, `GPK-V0-025` and `GPK-V0-015`
  stands at its prior strength.
- It does not authorize installing the candidate as `corvint`.
- It does not repair `src/`. The oracle stays frozen until W15 and remains the sole source of
  expected bytes for every corpus recapture this decision sets in motion.
- It does not adjudicate DR-0005 (see D3), decide the archive format (D14), or answer D13's question
  of fact.

## Consequences

Ten of the fourteen items move a clause or land code. Three (D8, D10, D13) are ratified decisions
*not* to change something, recorded so the absence is legible as a choice. One (D11) needed nothing.

The largest single consequence is D1: Corvint can now have a beta. Before this record, the rollout
order contained no rung between opt-in candidate use and full cutover, so no beta was legal at any
code quality. D2 is what makes one worth shipping — without it, impact remains unavailable on four
of the five product repositories.
