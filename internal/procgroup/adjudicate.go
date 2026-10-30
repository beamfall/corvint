package procgroup

// These vocabularies are deliberately independent (CRR-V0-006).
type Status string
type Outcome string
type CaseInconclusiveState string
type PrelaunchRefusal string

const (
	StatusPass                  Status                = "PASS"
	StatusFail                  Status                = "FAIL"
	StatusNotRun                Status                = "NOT_RUN"
	OutcomeCompatible           Outcome               = "compatible"
	OutcomeDifferent            Outcome               = "different"
	OutcomeInstability          Outcome               = "instability"
	OutcomeWithheld             Outcome               = "withheld"
	InconclusiveNone            CaseInconclusiveState = ""
	InconclusiveTimeout         CaseInconclusiveState = "timeout"
	InconclusiveSetup           CaseInconclusiveState = "compilation/setup failure"
	InconclusiveResource        CaseInconclusiveState = "resource exhaustion"
	InconclusiveSkipped         CaseInconclusiveState = "skipped test"
	InconclusiveSandbox         CaseInconclusiveState = "missing sandbox"
	InconclusiveInconsistent    CaseInconclusiveState = "inconsistent run"
	RefusalNone                 PrelaunchRefusal      = "none"
	RefusalDescriptorInvalid    PrelaunchRefusal      = "descriptor-invalid"
	RefusalDigestMismatch       PrelaunchRefusal      = "digest-mismatch"
	RefusalFixtureNotRegistered PrelaunchRefusal      = "fixture-not-registered"
	RefusalMissingSandbox       PrelaunchRefusal      = "missing-sandbox"
	RefusalResourceBound        PrelaunchRefusal      = "resource-bound"
	RefusalBudgetExpired        PrelaunchRefusal      = "budget-expired"
	ReasonDescriptorInvalid                           = "descriptor-invalid"
	ReasonDigestMismatch                              = "digest-mismatch"
	ReasonFixtureNotRegistered                        = "fixture-not-registered"
	ReasonStartFailed                                 = "start-failed"
	ReasonMissingSandbox                              = "missing-sandbox"
	ReasonResourceBound                               = "resource-bound"
	ReasonBudgetExpired                               = "budget-expired"
	ReasonNoLaunchObserved                            = "no-launch-observed"
)

type Adjudication struct {
	Status                Status                `json:"status"`
	Outcome               Outcome               `json:"outcome"`
	RecordCount           int                   `json:"record_count"`
	Launches              int                   `json:"launches"`
	UnobservedLaunches    int                   `json:"unobserved_launches"`
	CaseInconclusiveState CaseInconclusiveState `json:"-"`
	Reason                string                `json:"reason"`
	FullyObserved         bool                  `json:"-"`
}

// Adjudicate decides only rows 1–7. A fully observed result delegates comparison
// and the net scratch-effect predicate to the caller; it never invents a verdict.
func Adjudicate(planned int, refusal PrelaunchRefusal, observations []Observation) Adjudication {
	a := Adjudication{Status: StatusNotRun, Outcome: OutcomeWithheld, CaseInconclusiveState: InconclusiveSetup}
	completed, overflow, terminated, setup, cancelled := 0, false, false, false, false
	for _, o := range observations {
		if o.ExitObserved {
			a.RecordCount++
		}
		if !o.Started {
			cancelled = cancelled || o.Cancelled
			setup = setup || !o.Cancelled
			continue
		}
		a.Launches++
		if !o.ExitObserved {
			a.UnobservedLaunches++
		}
		overflow = overflow || o.StdoutOverflow || o.StderrOverflow
		terminated = terminated || o.TimedOut || o.Cancelled
		if o.ExitObserved && !o.TimedOut && !o.Cancelled && !o.StdoutOverflow && !o.StderrOverflow && o.PipesDrained && o.OwnedProcessGroupCleanup {
			completed++
		}
	}
	if a.Launches == 0 {
		switch refusal {
		case RefusalMissingSandbox:
			a.CaseInconclusiveState, a.Reason = InconclusiveSandbox, ReasonMissingSandbox
		case RefusalDescriptorInvalid:
			a.Reason = ReasonDescriptorInvalid
		case RefusalDigestMismatch:
			a.Reason = ReasonDigestMismatch
		case RefusalFixtureNotRegistered:
			a.Reason = ReasonFixtureNotRegistered
		case RefusalResourceBound:
			a.CaseInconclusiveState, a.Reason = InconclusiveResource, ReasonResourceBound
		case RefusalBudgetExpired:
			a.CaseInconclusiveState, a.Reason = InconclusiveTimeout, ReasonBudgetExpired
		default:
			switch {
			// CRR-V0-006(e): start-failed covers the child never starting, whether
			// Cmd.Start failed or the caller's pre-start setup did.
			case setup:
				a.Reason = ReasonStartFailed
			case cancelled:
				a.CaseInconclusiveState, a.Reason = InconclusiveTimeout, ReasonBudgetExpired
			default:
				a.Reason = ReasonNoLaunchObserved
			}
		}
		return a
	}
	a.Status = StatusFail
	switch {
	case overflow:
		a.CaseInconclusiveState = InconclusiveResource
	case terminated:
		a.CaseInconclusiveState = InconclusiveTimeout
	case completed != planned || len(observations) != planned || planned <= 0:
		switch {
		case setup || refusal == RefusalDescriptorInvalid || refusal == RefusalDigestMismatch || refusal == RefusalFixtureNotRegistered:
			a.CaseInconclusiveState = InconclusiveSetup
		case cancelled || refusal == RefusalBudgetExpired:
			a.CaseInconclusiveState = InconclusiveTimeout
		default:
			a.CaseInconclusiveState = InconclusiveInconsistent
		}
	default:
		a.Status, a.Outcome, a.CaseInconclusiveState, a.FullyObserved = "", "", InconclusiveNone, true
	}
	return a
}
