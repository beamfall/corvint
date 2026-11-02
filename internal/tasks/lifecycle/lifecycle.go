// Package lifecycle derives a ticket's delivery lifecycle (TLC-V0-001) from
// its status, holds, live attempt, effective progress and observed Git refs.
// The value is computed on each read and never stored; no admission,
// eligibility, planning, gate, completion or authority read consults it.
package lifecycle

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The TLC-V0-001 states.
const (
	Draft               = "DRAFT"
	Open                = "OPEN"
	InProgress          = "IN_PROGRESS"
	Submitted           = "SUBMITTED"
	InReview            = "IN_REVIEW"
	AwaitingIntegration = "AWAITING_INTEGRATION"
	AwaitingCloseout    = "AWAITING_CLOSEOUT"
	Held                = "HELD"
	Completed           = "COMPLETED"
	Archived            = "ARCHIVED"
	Unknown             = "UNKNOWN"
)

// States lists every state in byte order, the facet key set.
var States = []string{Archived, AwaitingCloseout, AwaitingIntegration, Completed, Draft, Held, InProgress, InReview, Open, Submitted, Unknown}

// IsState reports whether s is a TLC-V0-001 state.
func IsState(s string) bool {
	i := sort.SearchStrings(States, s)
	return i < len(States) && States[i] == s
}

// The TLC-V0-002 and TLC-V0-004 reasons.
const (
	ReasonAttemptsNotObserved     = "ATTEMPTS_NOT_OBSERVED"
	ReasonCommitMissing           = "COMMIT_MISSING"
	ReasonRefDeleted              = "REF_DELETED"
	ReasonHeadNotOnRef            = "HEAD_NOT_ON_REF"
	ReasonNotOnIntegrationRef     = "NOT_ON_INTEGRATION_REF"
	ReasonGitNotObserved          = "GIT_NOT_OBSERVED"
	ReasonIntegrationRefMissing   = "INTEGRATION_REF_MISSING"
	ReasonReachabilityNotObserved = "REACHABILITY_NOT_OBSERVED"
	ReasonReportedBlocked         = "REPORTED_BLOCKED"
)

// Attempt is the live attempt evidence of one ticket.
type Attempt struct {
	ID, Phase, Stage string
	CandidateTreeOid *string
}

// Git is the observation of one delivered progress head. Each tri-state
// member is nil when it was not observed.
type Git struct {
	Failed                            bool // no repository, or a Git read failed
	IntegrationRef, IntegrationCommit string
	Ref, RefCommit                    string
	HeadExists, Integrated, HeadOnRef *bool
}

// Input is everything one derivation reads.
type Input struct {
	Status           string
	HoldIDs          []string
	AttemptsObserved bool
	Attempt          *Attempt // nil when no live attempt (or not observed)
	Progress         *ticket.Progress
	Git              *Git // nil when the progress phase needs no Git check
}

// NeedsGit reports whether the effective progress phase p is checked
// against Git (TLC-V0-003).
func NeedsGit(p *ticket.Progress) bool {
	return p != nil && p.Head != nil && (p.Phase == ticket.PhaseDeliveredOnBranch || p.Phase == ticket.PhaseMergedAwaitingCloseout)
}

// Lifecycle is one derived value with the evidence it used.
type Lifecycle struct {
	State, Reason, DerivedFrom string
	in                         Input
}

// Derive applies the TLC-V0-002 precedence.
func Derive(in Input) Lifecycle {
	l := Lifecycle{in: in}
	set := func(state, reason, from string) Lifecycle {
		l.State, l.Reason, l.DerivedFrom = state, reason, from
		return l
	}
	switch in.Status {
	case ticket.StatusCompleted:
		return set(Completed, "", "status")
	case ticket.StatusArchived:
		return set(Archived, "", "status")
	case ticket.StatusDraft:
		return set(Draft, "", "status")
	case ticket.StatusHeld:
		return set(Held, "", "hold")
	}
	if a := in.Attempt; in.AttemptsObserved && a != nil {
		switch {
		case a.Phase == "READY_FOR_INTEGRATION":
			return set(AwaitingIntegration, "", "attempt")
		case a.Phase == "REVIEWING" || a.Stage == "review":
			return set(InReview, "", "attempt")
		case a.CandidateTreeOid != nil:
			return set(Submitted, "", "attempt")
		}
		return set(InProgress, "", "attempt")
	}
	state, reason, verified := fromProgress(in)
	if state != "" && (verified || in.AttemptsObserved) {
		return set(state, reason, "progress")
	}
	if !in.AttemptsObserved {
		return set(Unknown, ReasonAttemptsNotObserved, "none")
	}
	return set(Open, "", "none")
}

// fromProgress maps the effective progress phase; verified reports a
// state Git confirmed, which stands even when attempts are not observed.
func fromProgress(in Input) (state, reason string, verified bool) {
	p := in.Progress
	if p == nil {
		return "", "", false
	}
	switch p.Phase {
	case ticket.PhaseInProgress:
		return InProgress, "", false
	case ticket.PhaseInReview, ticket.PhaseInMergeQueue:
		return InReview, "", false
	case ticket.PhaseBlocked:
		return Open, ReasonReportedBlocked, false
	}
	if !NeedsGit(p) {
		return "", "", false
	}
	g := in.Git
	switch {
	case g == nil || g.Failed:
		return Unknown, ReasonGitNotObserved, false
	case g.HeadExists == nil:
		return Unknown, ReasonReachabilityNotObserved, false
	case !*g.HeadExists:
		return Open, ReasonCommitMissing, false
	case g.IntegrationRef == "":
		return Unknown, ReasonIntegrationRefMissing, false
	case g.Integrated == nil:
		return Unknown, ReasonReachabilityNotObserved, false
	case *g.Integrated:
		return AwaitingCloseout, "", true
	case p.Phase == ticket.PhaseMergedAwaitingCloseout:
		return Open, ReasonNotOnIntegrationRef, false
	case g.Ref == "":
		return Open, ReasonRefDeleted, false
	case g.HeadOnRef == nil:
		return Unknown, ReasonReachabilityNotObserved, false
	case !*g.HeadOnRef:
		return Open, ReasonHeadNotOnRef, false
	}
	return AwaitingIntegration, "", true
}

func nullableString(s string) wire.Value {
	if s == "" {
		return wire.Null()
	}
	return wire.String(s)
}

func tri(b *bool) wire.Value {
	if b == nil {
		return wire.Null()
	}
	return wire.Bool(*b)
}

// Value is the closed TLC-V0-005 object.
func (l Lifecycle) Value() wire.Value {
	in := l.in
	o := wire.NewObject()
	o.Set("state", wire.String(l.State))
	o.Set("reason", nullableString(l.Reason))
	o.Set("derivedFrom", wire.String(l.DerivedFrom))
	o.Set("status", wire.String(in.Status))
	holds := append([]string{}, in.HoldIDs...)
	sort.Strings(holds)
	o.Set("holdIds", wire.Strings(holds))
	switch a := in.Attempt; {
	case !in.AttemptsObserved:
		o.Set("attempt", wire.String("NOT_OBSERVED"))
	case a == nil:
		o.Set("attempt", wire.Null())
	default:
		ao := wire.NewObject()
		ao.Set("attemptId", wire.String(a.ID))
		ao.Set("phase", wire.String(a.Phase))
		ao.Set("stage", nullableString(a.Stage))
		if a.CandidateTreeOid != nil {
			ao.Set("candidateTreeOid", wire.String(*a.CandidateTreeOid))
		} else {
			ao.Set("candidateTreeOid", wire.Null())
		}
		o.Set("attempt", wire.ObjectValue(ao))
	}
	if in.Progress != nil {
		o.Set("progress", ticket.ProgressEntryValue(*in.Progress))
	} else {
		o.Set("progress", wire.Null())
	}
	if g := in.Git; g != nil {
		gobs := wire.NewObject()
		gobs.Set("integrationRef", nullableString(g.IntegrationRef))
		gobs.Set("integrationCommit", nullableString(g.IntegrationCommit))
		gobs.Set("ref", nullableString(g.Ref))
		gobs.Set("refCommit", nullableString(g.RefCommit))
		gobs.Set("headExists", tri(g.HeadExists))
		gobs.Set("integrated", tri(g.Integrated))
		gobs.Set("headOnRef", tri(g.HeadOnRef))
		o.Set("git", wire.ObjectValue(gobs))
	} else {
		o.Set("git", wire.Null())
	}
	return wire.ObjectValue(o)
}
