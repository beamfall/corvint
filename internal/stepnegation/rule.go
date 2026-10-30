package stepnegation

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Run is the observation of one faulted run.
type Run struct {
	// Status is the test's final status: passed, failed, timedOut,
	// interrupted or skipped.
	Status         string
	Attempts       int
	Infrastructure bool
	// Trace is the run's parsed trace; nil when none was recorded.
	Trace *Trace
	// Applied counts fault applications by install ID (LPCV-V0-060).
	Applied map[string]int
	// Hidden records a visibility fault observed in force at settle.
	Hidden map[string]bool
}

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// showsMarker reports whether a transient error message shows the marker as
// the received value.
func showsMarker(message, marker string) bool {
	for _, line := range strings.Split(ansiPattern.ReplaceAllString(message, ""), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Received") && strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

func notMeasured(reason string, anchors ...string) testvalidity.Axis {
	return testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: reason, Anchors: nonNil(anchors)}
}

func nonNil(anchors []string) []string {
	if anchors == nil {
		return []string{}
	}
	return anchors
}

func faultAnchors(fault *Fault, witnessed *Assertion) []string {
	return []string{"plan:" + fault.Digest, "assertion:" + witnessed.Location + "#" + strconv.Itoa(witnessed.Occurrence)}
}

// runFailure classifies the run-level conditions shared by the single-step
// and joint rules, returning "" when none applies.
func runFailure(run Run) string {
	switch {
	case run.Infrastructure || run.Status == "interrupted" || run.Status == "skipped" || run.Trace == nil:
		return ReasonFaultedRunInfrastructure
	case run.Status == "timedOut":
		return ReasonFaultedRunTimeout
	case run.Attempts != 1:
		return ReasonFaultedRunRetried
	}
	return ""
}

// ancestors returns the ordinals enclosing ordinal in trace.
func ancestors(trace *Trace, ordinal int) map[int]bool {
	result := map[int]bool{}
	for step := trace.Step(ordinal); step != nil && step.Parent != 0 && !result[step.Parent]; step = trace.Step(step.Parent) {
		result[step.Parent] = true
	}
	return result
}

// EvaluateSingle applies the single-step pass rule of LPCV-V0-063 to one
// faulted run with exactly one fault active.
func EvaluateSingle(baseline *Trace, derivation Derivation, fault *Fault, run Run, baselinePasses int) testvalidity.Axis {
	anchors := faultAnchors(fault, derivation.Witnessed)
	if reason := runFailure(run); reason != "" {
		return notMeasured(reason, anchors...)
	}
	if run.Applied[fault.Digest] == 0 {
		return notMeasured(ReasonFaultNotApplied, anchors...)
	}
	trace := run.Trace
	if run.Status == "passed" {
		for _, step := range trace.Steps {
			if step.Failed {
				return notMeasured(ReasonUnattributedFailure, anchors...)
			}
		}
		return testvalidity.Axis{State: testvalidity.StrengthSurvived, Reason: ReasonFaultSurvived, Anchors: anchors}
	}
	if run.Status != "failed" {
		return notMeasured(ReasonFaultedRunInfrastructure, anchors...)
	}
	named := trace.Step(derivation.Ordinal)
	if named == nil || named.Title != derivation.Title {
		return notMeasured(ReasonFaultNotStepIsolated, anchors...)
	}
	// Condition 3: every failure belongs to the named step; an ancestor
	// fails only by propagating that same error.
	enclosing := ancestors(trace, derivation.Ordinal)
	for _, assertion := range trace.Assertions {
		if assertion.Failed && assertion.Step != derivation.Ordinal {
			return notMeasured(ReasonFaultNotStepIsolated, anchors...)
		}
	}
	for _, failure := range trace.Failures {
		if failure.Step != derivation.Ordinal {
			return notMeasured(ReasonFaultNotStepIsolated, anchors...)
		}
	}
	for _, step := range trace.Steps {
		if !step.Failed || step.Ordinal == derivation.Ordinal {
			continue
		}
		if !enclosing[step.Ordinal] || !named.Failed || step.Error != named.Error {
			return notMeasured(ReasonFaultNotStepIsolated, anchors...)
		}
	}
	// Condition 5: every earlier step passed as it did in the baseline.
	for _, step := range baseline.Steps {
		if step.Ordinal >= derivation.Ordinal || enclosing[step.Ordinal] {
			continue
		}
		observed := trace.Step(step.Ordinal)
		if observed == nil || observed.Title != step.Title || observed.Failed {
			return notMeasured(ReasonFaultNotStepIsolated, anchors...)
		}
	}
	// Condition 4: the witnessed assertion failed showing the marker, or
	// the visibility fault was observed with at least two baseline passes.
	witnessed := trace.FindAssertion(derivation.Witnessed.Location, derivation.Witnessed.Occurrence)
	if witnessed == nil || !witnessed.Failed || witnessed.Step != derivation.Ordinal {
		return notMeasured(ReasonUnattributedFailure, anchors...)
	}
	if fault.Install.Action == ActionHide {
		if !run.Hidden[fault.Digest] || baselinePasses < 2 {
			return notMeasured(ReasonUnattributedFailure, anchors...)
		}
	} else if !showsMarker(witnessed.Error, fault.Install.Marker) {
		return notMeasured(ReasonUnattributedFailure, anchors...)
	}
	return testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: ReasonKilled, Anchors: anchors}
}

// Underivable returns the entry of a step with no derivable fault.
func Underivable(derivation Derivation) Step {
	if derivation.NoAssertion {
		return Step{Title: Screen(derivation.Title), Ordinal: derivation.Ordinal, Witness: WitnessNone, Requires: RequiresManualControl, Strength: notMeasured(ReasonStepHasNoAssertion)}
	}
	return Step{Title: Screen(derivation.Title), Ordinal: derivation.Ordinal, Witness: WitnessNone, Requires: RequiresManualControl,
		Strength: notMeasured(ReasonUnderivable, "derivation:network:"+derivation.NetworkReason, "derivation:dom:"+derivation.DOMReason)}
}

// ResolveSingle combines the network and DOM outcomes of one step
// (LPCV-V0-063). A nil outcome means that fault run was not executed: the
// fault was not derivable, or the run budget prevented it.
func ResolveSingle(derivation Derivation, network, dom *testvalidity.Axis) Step {
	if derivation.Network == nil && derivation.DOM == nil {
		return Underivable(derivation)
	}
	entry := resolveSingle(derivation, network, dom)
	// A witness names the fault whose run decided a KILLED or SURVIVED
	// result; an unmeasured entry keeps its plan digest but has no witness,
	// as in a joint run (LPCV-V0-064).
	if entry.Strength.State != testvalidity.StrengthKilled && entry.Strength.State != testvalidity.StrengthSurvived {
		entry.Witness = WitnessNone
	}
	return entry
}

func resolveSingle(derivation Derivation, network, dom *testvalidity.Axis) Step {
	entry := Step{Title: Screen(derivation.Title), Ordinal: derivation.Ordinal}
	if derivation.Network != nil {
		entry.PlanDigest, entry.Witness = derivation.Network.Digest, WitnessNetwork
		if network == nil {
			entry.Strength = notMeasured(ReasonRunBudgetExhausted, faultAnchors(derivation.Network, derivation.Witnessed)...)
			return entry
		}
		if network.State == testvalidity.StrengthKilled {
			entry.Strength = *network
			return entry
		}
	}
	networkSurvived := network != nil && network.State == testvalidity.StrengthSurvived
	if derivation.DOM == nil {
		if networkSurvived {
			entry.Requires = RequiresManualControl
			entry.Strength = notMeasured(ReasonNetworkFaultSurvived, network.Anchors...)
			return entry
		}
		entry.Strength = *network
		return entry
	}
	entry.PlanDigest, entry.Witness = derivation.DOM.Digest, WitnessDOM
	if dom == nil {
		anchors := faultAnchors(derivation.DOM, derivation.Witnessed)
		if networkSurvived {
			anchors = append(anchors, "reason:"+ReasonNetworkFaultSurvived)
		}
		entry.Strength = notMeasured(ReasonRunBudgetExhausted, anchors...)
		return entry
	}
	entry.Strength = testvalidity.Axis{State: dom.State, Reason: dom.Reason, Anchors: append([]string{}, dom.Anchors...)}
	if networkSurvived && dom.State != testvalidity.StrengthSurvived {
		entry.Strength.Anchors = append(entry.Strength.Anchors, "reason:"+ReasonNetworkFaultSurvived)
	}
	if dom.State == testvalidity.StrengthNotMeasured && networkSurvived {
		entry.Requires = RequiresManualControl
	}
	return entry
}

// JointFault pairs a step with the one fault it carries in a joint run.
type JointFault struct {
	Derivation Derivation
	Fault      *Fault
}

// PlanJoint assigns each derivable step its joint fault (LPCV-V0-064): the
// network fault when derivable, else a text/value DOM fault. A step whose
// only fault is a visibility fault, or whose fault collides with an earlier
// step's occurrence or marker, gets none and reads requires-single-step-run.
func PlanJoint(derivations []Derivation) (faults []JointFault, entries map[int]Step) {
	entries = map[int]Step{}
	occurrences, markers := map[string]bool{}, map[string]bool{}
	for _, derivation := range derivations {
		if derivation.Network == nil && derivation.DOM == nil {
			entries[derivation.Ordinal] = Underivable(derivation)
			continue
		}
		fault := derivation.Network
		if fault == nil && derivation.DOM.Install.Action != ActionHide {
			fault = derivation.DOM
		}
		var occurrence string
		if fault != nil {
			install := fault.Install
			if install.Kind == KindNetwork {
				occurrence = strings.Join([]string{install.Method, install.OriginPath, strconv.Itoa(install.RequestOrdinal), install.Search}, "\x00")
			} else {
				occurrence = strings.Join([]string{install.Selector, install.Expression, strconv.Itoa(install.DOMOrdinal)}, "\x00")
			}
		}
		if fault == nil || occurrences[occurrence] || markers[fault.Install.Marker] {
			entries[derivation.Ordinal] = Step{Title: Screen(derivation.Title), Ordinal: derivation.Ordinal, Witness: WitnessNone, Requires: RequiresSingleStepRun, Strength: notMeasured(ReasonRequiresSingleStepRun)}
			continue
		}
		occurrences[occurrence], markers[fault.Install.Marker] = true, true
		faults = append(faults, JointFault{Derivation: derivation, Fault: fault})
	}
	return faults, entries
}

// EvaluateJoint applies the joint attribution rule of LPCV-V0-064 to the
// one joint faulted run.
func EvaluateJoint(faults []JointFault, run Run) map[int]Step {
	result := map[int]Step{}
	entry := func(joint JointFault, axis testvalidity.Axis) Step {
		witness := WitnessJointDOM
		if joint.Fault.Install.Kind == KindNetwork {
			witness = WitnessJointNetwork
		}
		if axis.State != testvalidity.StrengthKilled {
			witness = WitnessNone
		}
		return Step{PlanDigest: joint.Fault.Digest, Title: Screen(joint.Derivation.Title), Ordinal: joint.Derivation.Ordinal, Witness: witness, Strength: axis}
	}
	if reason := runFailure(run); reason != "" {
		for _, joint := range faults {
			result[joint.Derivation.Ordinal] = entry(joint, notMeasured(reason, faultAnchors(joint.Fault, joint.Derivation.Witnessed)...))
		}
		return result
	}
	byOrdinal := map[int]JointFault{}
	for _, joint := range faults {
		byOrdinal[joint.Derivation.Ordinal] = joint
	}
	attributed := len(run.Trace.Failures) == 0
	for _, assertion := range run.Trace.Assertions {
		if !assertion.Failed {
			continue
		}
		joint, ok := byOrdinal[assertion.Step]
		if !ok || !showsMarker(assertion.Error, joint.Fault.Install.Marker) {
			attributed = false
		}
	}
	for _, joint := range faults {
		anchors := faultAnchors(joint.Fault, joint.Derivation.Witnessed)
		ordinal := joint.Derivation.Ordinal
		var axis testvalidity.Axis
		witnessed := run.Trace.FindAssertion(joint.Derivation.Witnessed.Location, joint.Derivation.Witnessed.Occurrence)
		step := run.Trace.Step(ordinal)
		switch {
		case !attributed:
			axis = notMeasured(ReasonJointAttributionFailed, anchors...)
		case step == nil || step.Title != joint.Derivation.Title || witnessed == nil:
			axis = notMeasured(ReasonStepNotReached, anchors...)
		case run.Applied[joint.Fault.Digest] == 0:
			axis = notMeasured(ReasonFaultNotApplied, anchors...)
		case witnessed.Failed && showsMarker(witnessed.Error, joint.Fault.Install.Marker):
			axis = testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: ReasonKilled, Anchors: anchors}
		default:
			axis = notMeasured(ReasonJointSurvivalUnconfirmed, anchors...)
		}
		result[ordinal] = entry(joint, axis)
	}
	return result
}
