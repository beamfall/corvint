package ticket

// Test-boundary citations, owner-gated scope decisions and witness candidates
// (TOL-V0-079..086, GitHub #1180 and #1182). Each is an optional OBLIGATIONS_SET
// change member and an optional entry member; a ledger that never records one
// keeps its exact earlier bytes.
import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	// MaxObligationCites bounds an entry's cited source locations.
	MaxObligationCites = 16
	// MaxObligationScopeEvidenceBytes bounds a scope decision's evidence
	// reference: 1..512 bytes (TOL-V0-080), wider than the 256-byte witness
	// evidence reference.
	MaxObligationScopeEvidenceBytes = 512

	ObligationScopePropose  = "PROPOSE"
	ObligationScopeAccept   = "ACCEPT"
	ObligationScopeProposed = "PROPOSED"
	ObligationScopeAccepted = "ACCEPTED"

	// ObligationScopeNoProposalDetail prefixes the refusal of an ACCEPT with
	// no pending proposal for the same core value (TOL-V0-081).
	ObligationScopeNoProposalDetail = "OBLIGATION_SCOPE_NO_PROPOSAL:"
	// ObligationScopeSelfAcceptDetail prefixes the refusal of an ACCEPT by
	// the actor who proposed the decision (TOL-V0-081).
	ObligationScopeSelfAcceptDetail = "OBLIGATION_SCOPE_SELF_ACCEPT:"
	// ObligationScopeDecisionRequiredDetail prefixes the refusal of a plain
	// core change under the requireScopeDecision policy (TOL-V0-082).
	ObligationScopeDecisionRequiredDetail = "OBLIGATION_SCOPE_DECISION_REQUIRED:"
	// ObligationCiteRepositoryDetail prefixes the TOL-V0-102 refusal of a
	// cite whose repository label the policy does not declare.
	ObligationCiteRepositoryDetail = "OBLIGATION_CITE_REPOSITORY_UNDECLARED:"

	// VerdictFindingNoFalsifiableText marks a verdict whose reviewer found
	// the obligation itself states nothing a test can falsify (TOL-V0-085).
	VerdictFindingNoFalsifiableText = "NO_FALSIFIABLE_TEXT"
)

// ObligationCite is one cited source location (TOL-V0-079): a repository
// path in the store's own repository (Repository nil) or in a test
// repository the policy declares by name.
type ObligationCite struct {
	Repository *string
	Path       string
}

func (c ObligationCite) Value() wire.Value {
	return noteObject("repository", wire.StringOrNull(c.Repository), "path", wire.String(c.Path))
}

func obligationCitesValue(cites []ObligationCite) wire.Value {
	vs := make([]wire.Value, 0, len(cites))
	for _, c := range cites {
		vs = append(vs, c.Value())
	}
	return wire.Array(vs...)
}

// readObligationCites reads 1..16 canonical-sorted unique cites.
func readObligationCites(r *wire.Reader) []ObligationCite {
	items := r.Array(MaxObligationCites, false)
	if r.Err() != nil {
		return nil
	}
	if len(items) == 0 {
		r.Fail(wire.CodeMalformed, "cites needs at least one entry")
		return nil
	}
	out := make([]ObligationCite, 0, len(items))
	for _, it := range items {
		it.Closed("repository", "path")
		c := ObligationCite{Repository: it.Field("repository").LabelOrNull(), Path: it.Field("path").Path()}
		if it.Err() != nil {
			return nil
		}
		out = append(out, c)
	}
	return out
}

// ObligationScopeChange is the scope member of one OBLIGATIONS_SET change
// (TOL-V0-080): PROPOSE records a decision to set core with an evidence
// reference; ACCEPT applies the pending proposal for the same core value.
type ObligationScopeChange struct {
	Action   string
	Core     bool
	Evidence string
}

func (s ObligationScopeChange) Value() wire.Value {
	if s.Action == ObligationScopeAccept {
		return noteObject("action", wire.String(s.Action), "core", wire.Bool(s.Core))
	}
	return noteObject("action", wire.String(s.Action), "core", wire.Bool(s.Core), "evidence", wire.String(s.Evidence))
}

func readObligationScopeChange(r *wire.Reader) *ObligationScopeChange {
	if r.Value().Kind != wire.KindObject || !wire.Has(r.Value(), "action") {
		r.Closed("action")
		return nil
	}
	s := &ObligationScopeChange{Action: r.Field("action").Enum(ObligationScopePropose, ObligationScopeAccept)}
	if r.Err() != nil {
		return nil
	}
	if s.Action == ObligationScopeAccept {
		r.Closed("action", "core")
	} else {
		r.Closed("action", "core", "evidence")
		s.Evidence = readNonblank(r.Field("evidence"), MaxObligationScopeEvidenceBytes)
	}
	s.Core = r.Field("core").Bool()
	if r.Err() != nil {
		return nil
	}
	return s
}

// ObligationScope is an entry's scope decision (TOL-V0-081): who proposed
// setting core to Core, when, why and on what evidence, and, once ACCEPTED,
// who accepted it and when.
type ObligationScope struct {
	Status         string
	Core           bool
	Reason         string
	Evidence       string
	ProposedByID   string
	ProposedByRole string
	ProposedAt     wire.Timestamp
	AcceptedByID   string
	AcceptedByRole string
	AcceptedAt     wire.Timestamp
}

func (s ObligationScope) Value() wire.Value {
	v := noteObject("status", wire.String(s.Status), "core", wire.Bool(s.Core), "reason", wire.String(s.Reason), "evidence", wire.String(s.Evidence),
		"proposedBy", noteObject("id", wire.String(s.ProposedByID), "role", wire.String(s.ProposedByRole)), "proposedAt", wire.String(string(s.ProposedAt)))
	if s.Status == ObligationScopeAccepted {
		v.Obj.Set("acceptedBy", noteObject("id", wire.String(s.AcceptedByID), "role", wire.String(s.AcceptedByRole)))
		v.Obj.Set("acceptedAt", wire.String(string(s.AcceptedAt)))
	}
	return v
}

func readObligationScope(r *wire.Reader) *ObligationScope {
	if r.Value().Kind != wire.KindObject || !wire.Has(r.Value(), "status") {
		r.Closed("status")
		return nil
	}
	s := &ObligationScope{Status: r.Field("status").Enum(ObligationScopeProposed, ObligationScopeAccepted)}
	if r.Err() != nil {
		return nil
	}
	keys := []string{"status", "core", "reason", "evidence", "proposedBy", "proposedAt"}
	if s.Status == ObligationScopeAccepted {
		keys = append(keys, "acceptedBy", "acceptedAt")
	}
	r.Closed(keys...)
	s.Core = r.Field("core").Bool()
	s.Reason = readNonblank(r.Field("reason"), MaxObligationReasonBytes)
	s.Evidence = readNonblank(r.Field("evidence"), MaxObligationScopeEvidenceBytes)
	s.ProposedByID, s.ProposedByRole = readObligationActor(r.Field("proposedBy"))
	s.ProposedAt = r.Field("proposedAt").Timestamp()
	if s.Status == ObligationScopeAccepted {
		s.AcceptedByID, s.AcceptedByRole = readObligationActor(r.Field("acceptedBy"))
		s.AcceptedAt = r.Field("acceptedAt").Timestamp()
		if r.Err() == nil && s.AcceptedByID == s.ProposedByID {
			r.Fail(wire.CodeMalformed, "%s an accepted scope decision names its proposer as acceptor", ObligationScopeSelfAcceptDetail)
		}
	}
	if r.Err() != nil {
		return nil
	}
	return s
}

func readObligationActor(r *wire.Reader) (string, string) {
	r.Closed("id", "role")
	return r.Field("id").Label(), r.Field("role").Enum("OWNER", "OPERATOR", "WORKER")
}

// ObligationCandidate is a recorded witness candidate awaiting review
// (TOL-V0-084): the new example's path and line range, and when it was
// recorded. A later REVIEWED witness or credit clears it.
type ObligationCandidate struct {
	Path      string
	StartLine int64
	EndLine   int64
	At        wire.Timestamp
}

func (c ObligationCandidate) Value() wire.Value {
	v := noteObject("path", wire.String(c.Path), "startLine", wire.String(string(wire.CountOf(c.StartLine))),
		"endLine", wire.String(string(wire.CountOf(c.EndLine))))
	if c.At != "" {
		v.Obj.Set("at", wire.String(string(c.At)))
	}
	return v
}

// readObligationCandidate reads {path, startLine, endLine} and, on an entry
// (withAt), the recording time.
func readObligationCandidate(r *wire.Reader, withAt bool) *ObligationCandidate {
	if withAt {
		r.Closed("path", "startLine", "endLine", "at")
	} else {
		r.Closed("path", "startLine", "endLine")
	}
	c := &ObligationCandidate{Path: r.Field("path").Path(), StartLine: r.Field("startLine").Count().Int(), EndLine: r.Field("endLine").Count().Int()}
	if withAt {
		c.At = r.Field("at").Timestamp()
	}
	if r.Err() == nil && (c.StartLine < 1 || c.EndLine < c.StartLine) {
		r.Fail(wire.CodeMalformed, "candidate lines are 1-based with startLine <= endLine")
	}
	if r.Err() != nil {
		return nil
	}
	return c
}

// applyScopeChange folds one scope change onto e (TOL-V0-081). PROPOSE
// records the decision and leaves core unchanged; ACCEPT needs the pending
// proposal for the same core value from another actor, and only then sets
// core. Authority (ACCEPT is OWNER-only) is the mutation layer's.
func applyScopeChange(e *ObligationEntry, s *ObligationScopeChange, reason string, q *ObligationRequest) error {
	if s.Action == ObligationScopePropose {
		e.Scope = &ObligationScope{Status: ObligationScopeProposed, Core: s.Core, Reason: reason, Evidence: s.Evidence,
			ProposedByID: q.ActorID, ProposedByRole: q.ActorRole, ProposedAt: q.IssuedAt}
		return nil
	}
	p := e.Scope
	if p == nil || p.Status != ObligationScopeProposed || p.Core != s.Core {
		return wire.Errorf(wire.CodeMalformed, "/request/payload/changes", "%s obligation %s has no pending scope proposal for core %t", ObligationScopeNoProposalDetail, e.ID, s.Core)
	}
	if p.ProposedByID == q.ActorID {
		return wire.Errorf(wire.CodeMalformed, "/request/payload/changes", "%s %s proposed the scope decision on %s and cannot accept it", ObligationScopeSelfAcceptDetail, q.ActorID, e.ID)
	}
	accepted := *p
	accepted.Status, accepted.AcceptedByID, accepted.AcceptedByRole, accepted.AcceptedAt = ObligationScopeAccepted, q.ActorID, q.ActorRole, q.IssuedAt
	e.Scope = &accepted
	e.Core = s.Core
	return nil
}
