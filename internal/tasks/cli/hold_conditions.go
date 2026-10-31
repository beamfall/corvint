package cli

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Bounds of one status or doctor read's hold re-evaluation (SCC-V0-013,
// SCC-V0-014): typed conditions past the first holdConditionEvaluationBound
// are reported UNKNOWN unevaluated; free-text candidates past
// holdFreeTextCandidateBound are not evaluated and the doctor warns.
const (
	holdConditionEvaluationBound = 64
	holdFreeTextPerHold          = 8
	holdFreeTextCandidateBound   = 128
)

// holdFreeTextDeployRef is the deploy ref free-text holds are tested
// against (SCC-V0-014): the local remote-tracking default branch, a
// symbolic ref `git clone` writes. The doctor never fetches it.
const holdFreeTextDeployRef = "refs/remotes/origin/HEAD"

// heldHold is one hold of a HELD ticket, collected inside the store read
// and evaluated after it, so Git reads never extend the read.
type heldHold struct {
	ticketID string
	hold     ticket.Hold
}

// collectHeldHolds lists the holds of every HELD ticket in inventory order.
func collectHeldHolds(inv *ticket.Inventory) []heldHold {
	var out []heldHold
	for _, id := range inv.IDs() {
		if closedTicket(inv, id) {
			continue
		}
		rec, ok := inv.Get(id)
		if !ok || rec.Status != ticket.StatusHeld {
			continue
		}
		for _, h := range rec.Holds {
			out = append(out, heldHold{ticketID: rec.TicketID.Raw, hold: h})
		}
	}
	return out
}

// heldConditionResult is one typed condition's evaluation.
type heldConditionResult struct {
	heldHold
	res store.HoldConditionResult
}

// evaluateHeldConditions evaluates each typed condition at its ref's current
// commit in root, up to the evaluation bound.
func evaluateHeldConditions(root string, holds []heldHold) []heldConditionResult {
	var out []heldConditionResult
	for _, h := range holds {
		if h.hold.Condition == nil {
			continue
		}
		r := heldConditionResult{heldHold: h}
		if len(out) >= holdConditionEvaluationBound {
			r.res = store.HoldConditionResult{State: store.HoldConditionUnknown,
				Detail: "not evaluated: this read evaluates at most " + strconv.Itoa(holdConditionEvaluationBound) + " hold conditions"}
		} else {
			r.res = store.EvaluateHoldCondition(root, h.hold.Condition)
		}
		out = append(out, r)
	}
	return out
}

// holdConditionsValue renders queue status's holdConditions member.
func holdConditionsValue(results []heldConditionResult) wire.Value {
	arr := make([]wire.Value, 0, len(results))
	for _, r := range results {
		o := wire.NewObject().Set("ticketId", wire.String(r.ticketID)).Set("holdId", wire.String(r.hold.HoldID)).
			Set("condition", r.hold.Condition.Value()).Set("state", wire.String(r.res.State))
		if r.res.RefCommit != "" {
			o.Set("refCommit", wire.String(r.res.RefCommit))
		}
		if c := r.res.Containment; c != nil {
			co := wire.NewObject().Set("state", wire.String(c.State))
			if c.Equivalent != "" {
				co.Set("equivalent", wire.String(c.Equivalent))
			}
			if c.PatchDiffers != "" {
				co.Set("patchDiffers", wire.String(c.PatchDiffers))
			}
			if c.Reason != "" {
				co.Set("reason", wire.String(c.Reason))
			}
			o.Set("containment", wire.ObjectValue(co))
		}
		o.Set("detail", wire.String(prose(r.res.Detail)))
		arr = append(arr, wire.ObjectValue(o))
	}
	return wire.Array(arr...)
}

// doctorHoldConditionFindings names typed conditions that are met
// (HOLD_CONDITION_MET) or cannot be evaluated (HOLD_CONDITION_UNKNOWN).
func doctorHoldConditionFindings(results []heldConditionResult) []doctorFinding {
	var out []doctorFinding
	for _, r := range results {
		kind := ""
		switch r.res.State {
		case store.HoldConditionUnblocked:
			kind = "HOLD_CONDITION_MET"
		case store.HoldConditionUnknown:
			kind = "HOLD_CONDITION_UNKNOWN"
		default:
			continue
		}
		out = append(out, doctorFinding{kind: kind, who: r.ticketID + "/" + r.hold.HoldID,
			detail: prose("hold " + r.hold.HoldID + ": " + r.res.Detail)})
	}
	return out
}

var (
	holdFreeTextSHA  = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	holdFreeTextPath = regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+/?$`)
)

// holdFreeTextCandidates extracts the commit-id and path tokens of a
// free-text hold reason: a lowercase hex word of 7..64 characters holding a
// digit, or a slash-separated relative path that is not a URL or a refname.
func holdFreeTextCandidates(reason string) (shas, paths []string) {
	seen := map[string]bool{}
	for _, w := range strings.Fields(reason) {
		if strings.Contains(w, "://") {
			continue
		}
		w = strings.Trim(w, "\"'`()[]{}<>,;:.!?")
		if w == "" || seen[w] || len(shas)+len(paths) >= holdFreeTextPerHold {
			continue
		}
		switch {
		case holdFreeTextSHA.MatchString(w) && strings.ContainsAny(w, "0123456789"):
			seen[w] = true
			shas = append(shas, w)
		case holdFreeTextPath.MatchString(w) && !strings.HasPrefix(w, "refs/") && !strings.Contains(w, ".."):
			seen[w] = true
			paths = append(paths, w)
		}
	}
	return shas, paths
}

// doctorFreeTextHoldFindings names free-text holds (no typed condition)
// whose reason names a commit contained in deployRef, or a path that exists
// at it (FREE_TEXT_HOLD_MET, SCC-V0-014). A token that does not resolve, or
// whose answer is unknown, yields nothing: the scan never reports a
// condition it could not observe. An unresolvable deployRef is one warning,
// emitted only when some free-text hold names a candidate.
func doctorFreeTextHoldFindings(root, deployRef string, holds []heldHold, scan *doctorScan) []doctorFinding {
	var out []doctorFinding
	evaluated, skipped := 0, 0
	for _, h := range holds {
		if h.hold.Condition != nil {
			continue
		}
		shas, paths := holdFreeTextCandidates(h.hold.Reason)
		var conds []*ticket.HoldCondition
		for _, s := range shas {
			conds = append(conds, &ticket.HoldCondition{Kind: ticket.HoldConditionCommitContained, Ref: deployRef, Commit: s})
		}
		for _, p := range paths {
			conds = append(conds, &ticket.HoldCondition{Kind: ticket.HoldConditionPathExists, Ref: deployRef, Path: p})
		}
		for _, c := range conds {
			if evaluated >= holdFreeTextCandidateBound {
				skipped++
				continue
			}
			evaluated++
			r := store.EvaluateHoldCondition(root, c)
			if r.RefCommit == "" {
				scan.warnings = append(scan.warnings, prose("deploy ref "+deployRef+" does not resolve to a commit; free-text hold findings NOT_OBSERVED"))
				return out
			}
			if r.State != store.HoldConditionUnblocked {
				continue
			}
			name := "commit " + c.Commit
			if c.Kind == ticket.HoldConditionPathExists {
				name = "path " + c.Path
			}
			out = append(out, doctorFinding{kind: "FREE_TEXT_HOLD_MET", who: h.ticketID + "/" + h.hold.HoldID,
				detail: prose("free-text hold " + h.hold.HoldID + " names " + name + "; " + r.Detail)})
			break
		}
	}
	if skipped > 0 {
		scan.warnings = append(scan.warnings, "free-text hold scan evaluated "+strconv.Itoa(holdFreeTextCandidateBound)+" candidates and left "+strconv.Itoa(skipped)+" unevaluated")
	}
	return out
}
