# Decision 0006 — ratifying `CF-V0-031` and the withheld-authority form for caller-authored authority

Date: 2026-08-29. Status: accepted. Authority: repository-owner delegation (2026-08-29), `GPK-V0-033`
(`spec-gap` resolution), `GPK-V0-034`.

## Problem

Commit `8e4b682` closed an authority-laundering hole in `range impact` and recorded it as `DR-0007`.
An actor could add `docs/adr/NNNN-*.md` with `status: accepted` **inside its own change set**, cite
`ADR-NNNN`, and receive `"confidence": "authoritative", "authority": "accepted-contract"` for that
same change. The Python oracle has the identical hole (`src/context_corvint_index.py`), so
the two runtimes agreed and neither could adjudicate: `spec-gap` under `GPK-V0-033`.

That commit amended `docs/specs/change-frontier-v0.md` with `CF-V0-031` ("Caller-authored
authority") and left two things open, both of which this decision closes:

1. `CF-V0-031` amends a specification marked `Frozen: 2026-08-23`, so it was not owner-accepted and
   could not be relied on as project policy.
2. The oracle-shared `impact` surface was **parked unrepaired**, on the correct reading that
   `GPK-V0-033` forbids repairing either side of an open `spec-gap`.

## Authority

The repository owner delegated ratification for this work, verbatim: "I want you to self unblock.
ratify where needed." This record is that ratification. It is per clause, not per document: the rest
of Change Frontier V0 keeps its `proposed` intent status.

## Decision

**1. `CF-V0-031` is ratified `accepted`, dated 2026-08-29.** It is binding on every
authority-conferring resolver in this repository, Frontier-owned or not, and it is the conformance
authority for authority resolution under `GPK-V0-033`. Expectations for surfaces it governs are
authored from its text.

**2. `CF-V0-032` is added and ratified with it**, naming the exact reporting form `CF-V0-031`'s
"reported as such" disposition requires: the citation keeps its path, line, blob identity, and
reason; only confidence and authority change, to `low` plus `unverified-contract` (for a withheld
`accepted-contract` / `partially-superseded-contract`) or `unverified-ledger` (for a withheld
`canonical-ledger`); and the required uncertainty entry is **derived from the emitted evidence**
rather than carried beside it. Without a spec-named form, a conformance expectation for the withheld
disposition could only be transcribed from the candidate — which is the self-certification
`GPK-V0-002` and `GPK-V0-033` exist to prevent. Naming it in the spec is what makes the corpus's
candidate-side expectation spec-authored.

**3. The parked `impact` surface takes the second of `CF-V0-031`'s two permitted dispositions.**
`Impact(index, paths, limit)` receives only caller-declared paths and no base revision, so it has no
revision the caller did not author and **cannot** satisfy the clause. `CF-V0-031` then admits exactly
two outcomes: gain such a revision, or report the accepted-authority labels as unverified. `impact`
does not gain one in this decision, so it reports. It withholds unconditionally — not on a predicate
over `paths` — because `paths` is the caller's own declaration:

- a predicate keyed on `paths` penalises an honest caller who declares the governing document it
  touched, and
- it never touches an attacker, who simply omits it: the citation reaches `recordResult` through a
  ledger record's `adr:` field, not through `paths` at all.

Such a predicate closes nothing while appearing to, which `CF-V0-025` forbids asserting. Withholding
for every citation on this surface is the only disposition that does not depend on the caller's own
declaration, and it is honest about what the surface can and cannot determine.

**4. `rangeMarkerResult` is covered by the same reasoning**, as `DR-0007` already recorded. `range
impact` **does** have a caller-independent base, so its refusal is scoped rather than blanket: it
withholds `authoritative`/`canonical-ledger` only where the ledger file declaring the record is
inside the change set Git computed between the verified base and the captured HEAD.

**5. `src/` is not repaired.** Under `GPK-V0-033` the oracle now contradicts ratified spec text while
the candidate matches it, so `DR-0007` is re-adjudicated `python-defect` / known-divergent. Python
keeps cross-checking; it is no longer the expectation.

## Consequences

- Go is deliberately **more honest than the oracle** on `impact`. The disagreement is registered in
  `conformance/divergence-register.md` (`DR-0007`) and carried by parity case
  `impact-self-authored-adr`, whose oracle bytes are captured from the pinned Python revision and
  whose candidate-side divergent values are authored from `CF-V0-032`.
- The parity corpus gains its first fixture with a `docs/adr/` tree. Before this decision the
  authority branch was unreachable by all 97 cases, so no runtime change to authority resolution was
  observable — a green ratchet over a corpus that could not express the defect.
- `impact` results carrying a cited decision now report `low`/`unverified-contract` even when the
  cited decision is genuinely pre-existing and accepted. That loss of resolution is the correct
  report: at this input contract the surface cannot tell the two apart, and `CF-V0-025` forbids
  claiming otherwise. The refinement that recovers it is a caller-independent base revision on
  `impact`, which `CF-V0-031` already admits and `CF-V0-029` already requires a new profile
  identifier for.

## Rejected alternatives

**Leave `impact` parked.** `GPK-V0-033`'s park rule applies while a `spec-gap` is open. Ratifying
`CF-V0-031` closes it, and the clause's own text decides the observable for a surface with no
caller-independent revision. Continuing to park would be reading the park rule as permanent.

**Guard `impact` on the declared `paths`.** Rejected in full above: penalises the honest caller,
misses the attacker, and asserts a control that closes nothing.

**Keep `accepted-contract` and add only an uncertainty entry.** Rejected. `CF-V0-031` forbids the
label itself, and an authority label a consumer reads is not neutralised by prose it may not.

**Drop the citation instead of withholding it.** Rejected by `CF-V0-031` explicitly: silent dropping
destroys the fact that a decision was cited at all, which is information the caller needs and the
resolver did determine.
