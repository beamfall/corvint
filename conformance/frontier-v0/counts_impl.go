// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// The CF-V0-023 counts universes.
//
// A resource-limit case declares no hunk and no obligation. It declares a
// SIZE — "2,048 HUNK_BASIS items", "64 related IDs on one item" — and the
// universe that realizes that size has to be DERIVED from the bound being
// driven and then built for real, like any other fixture universe.
//
// Deriving it also settles satisfiability, which is the more interesting half.
// Every Frontier count is fed by an upstream artifact whose own ceiling is a
// published constant, and for five of the seven bounds those two numbers are
// EQUAL: a CEM map holds at most 2,048 hunks and Frontier admits at most 2,048
// hunk items; the OCM verifier admits at most 256 obligations and Frontier
// admits at most 256 intent-change items; an OCM reference array holds at most
// 64 IDs and an item carries at most 64 related IDs. Where the two coincide the
// Frontier ceiling is defence in depth over a narrower artifact algebra and its
// at-N+1 case cannot be materialized at all — the same shape as the reason-union
// ceiling in capabilities.go, discovered one bound at a time here.
//
// So planCounts returns either a universe to build or the specific derivation
// that puts that size out of reach, and the derivation is the skip message.

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// countsPlan is the universe one counts case needs, in the same vocabulary the
// other fixtures declare directly.
type countsPlan struct {
	// unknownHunks are CEM `unknown` hunks: one HUNK_BASIS item each, and not
	// referenceable by a linked obligation.
	unknownHunks int
	// basisHunk is one CEM `supported` hunk whose only basis LRF rejects. It
	// emits a HUNK_BASIS item AND satisfies the OCM verifier's requirement that
	// a linked obligation name a supported hunk.
	basisHunk bool
	// closingHunk is one CEM `supported` hunk with a qualifying LRF basis. It
	// emits nothing (CF-V0-008) and is likewise referenceable.
	closingHunk bool
	// linkedObligations each emit exactly one INTENT_CHANGE and exactly one
	// INTENT_TEST item (CF-V0-012, CF-V0-015).
	linkedObligations int
}

// items is the item count this plan produces, read off the projection clauses.
func (plan countsPlan) items() int {
	hunkItems := plan.unknownHunks
	if plan.basisHunk {
		hunkItems++
	}
	return hunkItems + 2*plan.linkedObligations
}

// PlanCounts derives the universe one counts case needs. The second result is
// empty when the plan is buildable and otherwise is the derivation that puts
// the declared size out of reach — which becomes the case's skip message, so it
// says what is unreachable and why, never merely that something is.
func PlanCounts(c Case) (countsPlan, string) {
	for _, limit := range Limits {
		declared, driven := c.Declared.Counts[limit.Name]
		if !driven {
			continue
		}
		plan, reason := planForLimit(limit, declared)
		if reason != "" {
			return countsPlan{}, reason
		}
		// A plan whose arithmetic disagrees with the fixture would execute a
		// different universe than the one declared and still look green.
		want, err := decimalCount(c.Expect.ItemCount)
		if err != nil {
			return countsPlan{}, fmt.Sprintf("the declared item count is unreadable: %v", err)
		}
		if got := plan.items(); got != want {
			return countsPlan{}, fmt.Sprintf(
				"the universe derived for %s = %s projects %d items, but the case declares %d; "+
					"the derivation and the fixture disagree and neither may be trusted",
				limit.Name, declared, got, want)
		}
		return plan, ""
	}
	return countsPlan{}, "the case declares no CF-V0-023 count, so no universe can be derived for it"
}

// planForLimit derives one bound's universe, at N and at N+1.
func planForLimit(limit Limit, declared string) (countsPlan, string) {
	over := declared != fmt.Sprint(limit.Value)
	switch limit.Name {
	case "hunkItems":
		if over {
			return countsPlan{}, reasonHunkItemsOverBound
		}
		return countsPlan{}, reasonHunkItemsAtBound
	case "intentChangeItems", "intentTestItems":
		if over {
			return countsPlan{}, intentOverBoundReason(limit.Name)
		}
		// N linked obligations give N intent-change AND N intent-test items;
		// the one closing hunk keeps the patch non-empty, gives the obligations
		// a supported hunk to name, and emits nothing itself.
		return countsPlan{closingHunk: true, linkedObligations: limit.Value}, ""
	case "totalItems":
		if over {
			return countsPlan{}, reasonTotalItemsOverBound
		}
		// The total bound is reached with every kind at its own: 2,048 hunk
		// items, of which one must be a SUPPORTED hunk so the obligations have
		// something to name, plus 256 linked obligations.
		return countsPlan{
			unknownHunks:      LimitHunkItems - 1,
			basisHunk:         true,
			linkedObligations: LimitIntentChangeItems,
		}, ""
	case "relatedIdsPerItem":
		if over {
			return countsPlan{}, reasonRelatedIDsOverBound
		}
		return countsPlan{}, reasonRelatedIDsAtBound
	case "outputBytes":
		if over {
			return countsPlan{}, reasonOutputBytesOverBound
		}
		return countsPlan{}, reasonOutputBytesAtBound
	}
	return countsPlan{}, "no universe derivation is recorded for the " + limit.Name + " bound"
}

// The recorded derivations. Each one names the artifact ceiling that makes the
// declared size unreachable, so the next reader does not have to rediscover it.

const reasonHunkItemsAtBound = "2,048 HUNK_BASIS items in a result whose declared item count is also " +
	"2,048. That leaves room for no intent item at all, and every verified universe carries at least " +
	"one obligation, which emits at least one — so this size is the obligation-free universe under " +
	"another name. See IntentItemsPerObligation for the four links and " +
	"TestEmptyStateIsUnreachableThroughTheEntryPoint, which drives the refusal against a real " +
	"repository. Like the sixteen cases that skip with it, this is the recorded `spec-gap`, not a " +
	"defect: relaxing the shared `missing-requirements` refusal to unblock it would create a " +
	"divergence no parity case catches."

const reasonHunkItemsOverBound = "2,049 HUNK_BASIS items. One HUNK_BASIS item is emitted per CEM hunk " +
	"(CF-V0-008..CF-V0-010), and internal/cem/wire caps a canonical map at MaxHunks = 2048 — the SAME " +
	"number as the Frontier bound — so no verifiable CEM artifact can carry a 2,049th hunk for the item " +
	"to come from. Frontier's ceiling is defence in depth over a narrower artifact algebra, and cannot " +
	"be the first one a real universe reaches. The guard itself is tested where it is enforced, in " +
	"internal/frontier."

func intentOverBoundReason(name string) string {
	if name == "intentTestItems" {
		return "257 INTENT_TEST items. An INTENT_TEST item is emitted only for a LINKED obligation " +
			"(CF-V0-015), and EVERY obligation — linked or not — also emits an INTENT_CHANGE item " +
			"(CF-V0-011, CF-V0-012), so intent-test items can never outnumber intent-change items. " +
			"Both bounds are 256 and the change items are projected first, so a 257th linked " +
			"obligation trips the intent-change ceiling and the intent-test ceiling is unreachable by " +
			"construction — and the OCM verifier caps obligations at 256 before either fires."
	}
	return "257 INTENT_CHANGE items. One is emitted per OCM obligation (CF-V0-011, CF-V0-012), and the " +
		"OCM verifier caps BOTH the `## Requirements` enumeration and the obligations array at 256 — the " +
		"SAME number as the Frontier bound — so no verifiable OCM artifact can carry a 257th obligation " +
		"for the item to come from."
}

const reasonTotalItemsOverBound = "2,561 items. The three kind bounds sum to exactly the total bound " +
	"(2048 + 256 + 256 = 2560; see TotalItemsIsNotTheSumOfKindLimits), so a 2,561st item must exceed a " +
	"KIND ceiling first, and no universe can reach the total ceiling with every kind still inside its " +
	"own. The coincidence the fixture warns about is real in the other direction too: the total bound " +
	"is reachable at N — that case executes — but never exceedable."

const reasonRelatedIDsAtBound = "64 related IDs on one item, in a result whose declared item count is " +
	"1. Only a LINKED obligation's INTENT_CHANGE item can carry 64 references — the OCM verifier caps a " +
	"reference array at exactly 64 — and a linked obligation also emits an INTENT_TEST item, so that " +
	"universe holds two items and not one. The alternatives are narrower still: a HUNK_BASIS item's " +
	"related IDs are its rejected bases' evidence, which internal/cem/wire caps at MaxBases = 32, and " +
	"it would need an obligation's item beside it in any case; an OCM `unknown` obligation's item " +
	"carries no related ID at all (CF-V0-011). No universe puts 64 related IDs on the only item in a " +
	"result."

const reasonRelatedIDsOverBound = "65 related IDs on one item. Related IDs are an OCM reference array " +
	"or a supported hunk's rejected basis evidence. The OCM verifier caps a reference array at 64 — the " +
	"SAME number as the Frontier bound — and internal/cem/wire caps one hunk's bases at MaxBases = 32, " +
	"so no verifiable artifact can present an item with a 65th related ID."

const reasonOutputBytesAtBound = "exactly 4,194,304 output bytes from a result whose declared item " +
	"count is 1. One item is bounded by the same algebra as every other: at most 13 reasons " +
	"(ReasonAlgebraCeilings) and at most 64 related IDs, each a fixed-width content address, so one " +
	"item plus the CF-V0-018 envelope is on the order of a kilobyte — three orders of magnitude below " +
	"the declared byte count, with nothing else in the document caller-sized. This case was previously " +
	"recorded as reachable in principle through a ~2,000-hunk repository tuned to the exact byte; that " +
	"reading took the byte count without its declared itemCount of 1, and a 2,000-hunk repository emits " +
	"2,000 items, which this case forbids. It is unsatisfiable by construction, not merely large."

const reasonOutputBytesOverBound = "4,194,305 output bytes. Every input to the document's size is " +
	"itself capped, and the arithmetic lands under the bound. The item-count-MAXIMAL universe is the " +
	"one the totalItems at-N case now builds — 2,560 items, every kind at its own ceiling — and it " +
	"emits MeasuredMaximalDocumentBytes, about a fifth of the limit. What is left to add is related " +
	"IDs, and those are capped in aggregate too: a hunk item's evidence references by LRF's " +
	"8,192-issue ceiling across the whole result, an obligation's hunk references by the OCM verifier's " +
	"64-per-array cap over at most 256 obligations, and its claim references by that verifier's " +
	"512-claim ceiling — about 25,000 references of at most 83 bytes, some 2.1 MB. Roughly 3.0 MB " +
	"against a 4,194,304-byte bound. That is arithmetic over ONE measured document, NOT a closed " +
	"derivation the way the reason-union ceiling is, so the case is skipped as unsatisfiable in " +
	"practice rather than declared impossible: materializing even that universe needs on the order of " +
	"ten thousand real CEM citations, each rewriting the whole map, which this runner will not do. The " +
	"ceiling itself is enforced at the seal boundary and covered by internal/frontier's limit tests."

// MeasuredMaximalDocumentBytes is the size of the largest Frontier document
// this suite has actually produced: the totalItems at-N universe, which carries
// 2,560 items — every CF-V0-023 kind at its exact ceiling — and almost no
// related IDs. It is a MEASUREMENT, recorded so the outputBytes derivation
// above rests on an observed number rather than on an estimate of item width.
const MeasuredMaximalDocumentBytes = 892619

// UpstreamCeilings records the artifact bounds the derivations above depend on.
// Five of the seven CF-V0-023 counts are fed by an artifact whose own ceiling
// equals or undercuts the Frontier bound, which is what makes their at-N+1
// cases unsatisfiable rather than merely large. The CEM entries are read from
// internal/cem/wire directly; the OCM ones are transcribed from
// internal/lrfrepo/ocm.go, whose limits are unexported, and are cited in each
// derivation so a drift shows up as a wrong reason rather than a silent one.
var UpstreamCeilings = map[string]int{
	"cem.MaxHunks":       wire.MaxHunks,
	"cem.MaxBases":       wire.MaxBases,
	"ocm.maxObligations": 256,
	"ocm.maxReferences":  64,
	"ocm.maxClaims":      512,
}

// declare expands the plan into the ordinary declared vocabulary. Supported
// hunks come FIRST so the one or two evidence files this needs keep distinct
// basename stems, which LRF's subject terms depend on.
func (plan countsPlan) declare() ([]DeclaredHunk, []DeclaredOblig) {
	hunks := []DeclaredHunk{}
	if plan.closingHunk {
		hunks = append(hunks, DeclaredHunk{
			ID: countsHunkSymbol(0), Disposition: "supported", Bases: []string{"qualifying"},
		})
	}
	if plan.basisHunk {
		hunks = append(hunks, DeclaredHunk{
			ID: countsHunkSymbol(0), Disposition: "supported",
			Bases: []string{"rejected-insufficient-lexical-support"},
		})
	}
	for index := 0; index < plan.unknownHunks; index++ {
		hunks = append(hunks, DeclaredHunk{
			ID: countsHunkSymbol(index + 1), Disposition: "unknown", Reason: "no-evidence",
		})
	}
	obligations := make([]DeclaredOblig, 0, plan.linkedObligations)
	for index := 0; index < plan.linkedObligations; index++ {
		obligations = append(obligations, DeclaredOblig{
			ID:          countsObligationID(index),
			Disposition: "linked",
			HunkIDs:     []string{countsHunkSymbol(0)},
			ClaimIDs:    []string{countsClaimSymbol(index)},
		})
	}
	return hunks, obligations
}

// countsObligationID produces the ordinal requirement IDs the OCM verifier's
// frozen requirement syntax admits: an uppercase stem, then a hyphen and
// exactly three digits.
func countsObligationID(index int) string { return fmt.Sprintf("CFLIMIT-%03d", index+1) }

// countsHunkSymbol and countsClaimSymbol are symbol names, not content
// addresses: a generated universe's real identities are only known after it is
// built, and Outcome.Symbols is what binds them. Index 0 is the supported hunk,
// which is the one a linked obligation may name.
func countsHunkSymbol(index int) string  { return fmt.Sprintf("hunk:symbol:%04d", index) }
func countsClaimSymbol(index int) string { return fmt.Sprintf("claim:symbol:%04d", index) }

// CountBoundsAreDefenceInDepth reports, for each CF-V0-023 count whose at-N+1
// case is skipped as unreachable, whether the upstream ceiling that makes it
// unreachable still coincides with the Frontier bound. A false here means a
// recorded derivation has stopped being true and the case must be re-examined,
// not that the suite should quietly keep skipping it.
func CountBoundsAreDefenceInDepth() map[string]bool {
	return map[string]bool{
		// A HUNK_BASIS item needs a CEM hunk to be about.
		"hunkItems": UpstreamCeilings["cem.MaxHunks"] <= LimitHunkItems,
		// An INTENT_CHANGE item needs an OCM obligation to be about.
		"intentChangeItems": UpstreamCeilings["ocm.maxObligations"] <= LimitIntentChangeItems,
		// An INTENT_TEST item needs a LINKED obligation, which also emits an
		// INTENT_CHANGE item, so its own ceiling can never bind first.
		"intentTestItems": LimitIntentTestItems >= LimitIntentChangeItems,
		// The kind ceilings sum to exactly the total ceiling.
		"totalItems": LimitHunkItems+LimitIntentChangeItems+LimitIntentTestItems <= LimitTotalItems,
		// Related IDs come from an OCM reference array or a hunk's bases.
		"relatedIdsPerItem": UpstreamCeilings["ocm.maxReferences"] <= LimitRelatedIDsPerItem &&
			UpstreamCeilings["cem.MaxBases"] <= LimitRelatedIDsPerItem,
	}
}
