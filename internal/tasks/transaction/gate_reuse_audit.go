package transaction

import (
	"bytes"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// GateReuseReceiptAudit binds every reused gate result to the receipt that
// records it (CAL-V0-243). Receipts are folded in sequence order. A gate
// result an attempt post newly names whose reusedFrom is set must be posted
// by a GATE_RESULT receipt as that attempt's result, and must be byte for
// byte its source rebound to the attempt (ReusedGateResult); the source must
// be an unreused PASSED result of the same gate at the attempt's candidate
// tree that its own attempt names as folded before this receipt, and the
// policy the history had posted must declare the gate reusable with the
// source's definition digest. A result without reusedFrom, the only kind
// any earlier writer produced, is not examined, so existing journals fold
// unchanged. The generic journal audit treats evidence/ posts as opaque
// blobs, so this is where a rehashed but untrue reuse is caught. It is
// pure: blob returns retained evidence bytes by digest, and its memory is
// the latest attempt and policy posts and the reused results it accepted.
// An attempt may name an identical reused record again (it returned to a
// tree after executing elsewhere); the record is already retained, so that
// receipt need not post it, and it is checked again in full. A named result
// that does not decode but claims a reusedFrom is refused, never skipped.
// A source at another tree is accepted only when Descent, the caller's git
// observation, reports that a commit carrying the attempt's candidate tree is
// an ancestor of a commit carrying the source's tree on the intent branch the
// history had posted (TCD-V0-153); without Descent such a reuse is refused.
//
// A result whose source is a retained CI run record (TCD-V0-157) must be
// byte for byte CIGateResult for its own tree under the policy the history
// had posted, the run must qualify under that policy's ciEvidence entry, and
// CIDescent, the caller's git observation, must report that the run's head
// commit is on the posted intent branch carrying that tree and descends from
// a commit carrying the attempt's candidate tree; a rewound branch fails.
type GateReuseReceiptAudit struct {
	Descent   func(seal, pass, branch string) (string, error)
	CIDescent func(seal, commit, tree, branch string) (string, error)
	attempts  map[string]*snapshot.Attempt
	policy    *intent.Policy
	policyErr string
	branch    string
	accepted  map[string]bool
}

func (a *GateReuseReceiptAudit) fail(rc *snapshot.Receipt, f string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, "receipts/"+string(rc.Seq), "gate reuse binding: "+f, args...)
}

// Step folds one validated receipt; sum is unused but keeps the audit
// signature of the other material binding folds.
func (a *GateReuseReceiptAudit) Step(rc *snapshot.Receipt, _ wire.Digest, blob ExternalReviewBlob) error {
	if a.attempts == nil {
		a.attempts = map[string]*snapshot.Attempt{}
		a.accepted = map[string]bool{}
		a.policyErr = "no policy was posted"
	}
	next := map[string]*snapshot.Attempt{}
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "attempts/") || p.Sha256 == nil {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			// Unretained attempt bytes cannot carry a reuse this audit could
			// check; the generic journal audit owns their integrity.
			continue
		}
		at, err := snapshot.DecodeAttempt(raw)
		if err != nil {
			continue
		}
		if err := a.bind(rc, at, blob); err != nil {
			return err
		}
		next[strings.TrimSuffix(strings.TrimPrefix(p.Path, "attempts/"), ".json")] = at
	}
	for _, p := range rc.Post {
		switch {
		case strings.HasPrefix(p.Path, "attempts/"):
			id := strings.TrimSuffix(strings.TrimPrefix(p.Path, "attempts/"), ".json")
			delete(a.attempts, id)
			if at := next[id]; at != nil {
				a.attempts[id] = at
			}
		case p.Path == "intent/queue.json":
			a.branch = postedBranch(p, blob)
		case p.Path == "intent/policy.json":
			a.policy, a.policyErr = nil, "the retained policy was removed"
			if p.Sha256 == nil {
				continue
			}
			raw, err := externalPostBytes(p, blob)
			if err == nil {
				a.policy, err = intent.DecodePolicy(raw)
			}
			if err != nil {
				a.policy, a.policyErr = nil, "the retained policy is unreadable: "+err.Error()
			}
		}
	}
	return nil
}

// bind checks each gate result at names that its folded predecessor did
// not; only a reused result is examined.
func (a *GateReuseReceiptAudit) bind(rc *snapshot.Receipt, at *snapshot.Attempt, blob ExternalReviewBlob) error {
	var prior []string
	if p := a.attempts[at.AttemptID]; p != nil {
		prior = p.GateResults
	}
	for _, d := range at.GateResults {
		if slices.Contains(prior, d) {
			continue
		}
		raw, ok := blob(wire.Digest(d))
		if !ok || wire.Sum(raw) != wire.Digest(d) {
			continue
		}
		g, err := snapshot.DecodeGateResult(raw)
		if err != nil {
			if claimsReuse(raw) {
				return a.fail(rc, "reused gate result %s: %v", d, err)
			}
			continue
		}
		if g.ReusedFrom == nil {
			continue
		}
		if err := a.reuse(rc, at, d, raw, g, blob); err != nil {
			return err
		}
		a.accepted[d] = true
	}
	return nil
}

func (a *GateReuseReceiptAudit) reuse(rc *snapshot.Receipt, at *snapshot.Attempt, d string, raw []byte, g *snapshot.GateResult, blob ExternalReviewBlob) error {
	from := string(*g.ReusedFrom)
	if rc.Kind != "GATE_RESULT" {
		return a.fail(rc, "reused gate result %s is named by a %s receipt", d, rc.Kind)
	}
	posted := a.accepted[d]
	for _, p := range rc.Post {
		posted = posted || (p.Path == "evidence/"+d && p.Sha256 != nil && *p.Sha256 == wire.Digest(d))
	}
	if !posted {
		return a.fail(rc, "reused gate result %s is not posted by the receipt that names it or retained earlier", d)
	}
	srcRaw, ok := blob(*g.ReusedFrom)
	if !ok || wire.Sum(srcRaw) != *g.ReusedFrom {
		return a.fail(rc, "reused source %s is absent or differs from its digest", from)
	}
	if ci, err := snapshot.DecodeCIRun(srcRaw); err == nil {
		return a.ciReuse(rc, at, d, raw, g, ci, srcRaw)
	}
	src, err := snapshot.DecodeGateResult(srcRaw)
	if err != nil {
		return a.fail(rc, "reused source %s: %v", from, err)
	}
	owner := a.attempts[src.AttemptID]
	switch {
	case src.ReusedFrom != nil || src.State != "PASSED" || src.GateID != g.GateID:
		return a.fail(rc, "reused source %s is not an executed PASSED result of gate %s", from, g.GateID)
	case owner == nil || !slices.Contains(owner.GateResults, from):
		return a.fail(rc, "reused source %s is not named by its attempt %s", from, src.AttemptID)
	case at.CandidateTreeOid == nil:
		return a.fail(rc, "reused source %s is not at the attempt's candidate tree", from)
	case src.CandidateTreeOid != *at.CandidateTreeOid:
		if err := a.descends(rc, from, *at.CandidateTreeOid, src.CandidateTreeOid); err != nil {
			return err
		}
	}
	switch {
	case a.policy == nil:
		return a.fail(rc, "the gate definition cannot be recovered: %s", a.policyErr)
	}
	if def := a.policy.Gate(g.GateID); def == nil || !def.Reusable || a.policy.GateDefinitionSha256(g.GateID) != src.DefinitionSha256 {
		return a.fail(rc, "the policy does not declare gate %s reusable with the source's definition", g.GateID)
	}
	want := ReusedGateResult(src, *g.ReusedFrom, at)
	enc, err := want.Encode()
	if err != nil || !bytes.Equal(enc, raw) {
		return a.fail(rc, "reused gate result %s is not its source rebound to attempt %s", d, at.AttemptID)
	}
	return nil
}

// descends accepts a source at another tree only as a descendant of the
// candidate on the posted intent branch (TCD-V0-153). An unobservable answer
// is returned as is, so a missing git never reads as a forked journal.
func (a *GateReuseReceiptAudit) descends(rc *snapshot.Receipt, from, seal, pass string) error {
	if a.Descent == nil || a.branch == "" {
		return a.fail(rc, "reused source %s is not at the attempt's candidate tree and its ancestry cannot be verified", from)
	}
	why, err := a.Descent(seal, pass, a.branch)
	if err != nil {
		return err
	}
	if why != "" {
		return a.fail(rc, "reused source %s: %s", from, why)
	}
	return nil
}

// postedBranch is the intent branch a queue post names, "" when it is
// removed or unreadable.
func postedBranch(p snapshot.PostEntry, blob ExternalReviewBlob) string {
	if p.Sha256 == nil {
		return ""
	}
	raw, err := externalPostBytes(p, blob)
	if err != nil {
		return ""
	}
	q, err := intent.DecodeQueue(raw)
	if err != nil {
		return ""
	}
	return q.IntentBranch
}

// claimsReuse reports whether undecodable gate result bytes still carry a
// non-null reusedFrom, so a malformed reuse cannot pass as a skipped result.
func claimsReuse(raw []byte) bool {
	v, err := wire.Parse(raw)
	if err != nil || v.Kind != wire.KindObject {
		return false
	}
	r, ok := v.Obj.Get("reusedFrom")
	return ok && r.Kind != wire.KindNull
}

// ciReuse checks a CI-backed gate result (TCD-V0-157): the run record is
// retained, qualifies under the posted policy, the result is exactly its
// CIGateResult, and its head commit still descends from the candidate on the
// posted intent branch. An unobservable answer is returned as is.
func (a *GateReuseReceiptAudit) ciReuse(rc *snapshot.Receipt, at *snapshot.Attempt, d string, raw []byte, g *snapshot.GateResult, ci *snapshot.CIRun, ciRaw []byte) error {
	from := string(*g.ReusedFrom)
	switch {
	case at.CandidateTreeOid == nil:
		return a.fail(rc, "CI-backed gate result %s names an attempt with no candidate tree", d)
	case a.policy == nil:
		return a.fail(rc, "the CI evidence policy cannot be recovered: %s", a.policyErr)
	}
	if code, detail := CIRunRefusal(a.policy, g.GateID, ci); code != "" {
		return a.fail(rc, "CI run %s: %s", from, detail)
	}
	want := CIGateResult(a.policy, g.GateID, ci, ciRaw, at, g.CandidateTreeOid)
	enc, err := want.Encode()
	if err != nil || !bytes.Equal(enc, raw) {
		return a.fail(rc, "CI-backed gate result %s is not CI run %s bound to attempt %s", d, from, at.AttemptID)
	}
	if a.CIDescent == nil || a.branch == "" {
		return a.fail(rc, "CI run %s: its head commit's ancestry cannot be verified", from)
	}
	why, err := a.CIDescent(*at.CandidateTreeOid, ci.HeadSha, g.CandidateTreeOid, a.branch)
	if err != nil {
		return err
	}
	if why != "" {
		return a.fail(rc, "CI run %s: %s", from, why)
	}
	return nil
}
