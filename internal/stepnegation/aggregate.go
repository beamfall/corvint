package stepnegation

import (
	"strconv"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Join reasons (LPCV-V0-068).
const (
	ReasonStepControlsKilled     = "step-controls-killed"
	ReasonStepControlsIncomplete = "step-controls-incomplete"
	ReasonAssertionsOutsideSteps = "assertions-outside-steps"
)

// Aggregate is the PTF-V0-006 aggregation of a joined document over its
// retained inventory denominator (LPCV-V0-068).
func Aggregate(document Document) testvalidity.Axis {
	measured := map[int]Step{}
	var survived []string
	for _, step := range document.Steps {
		if step.Ordinal == 0 {
			continue
		}
		measured[step.Ordinal] = step
		if step.Strength.State == testvalidity.StrengthSurvived {
			survived = append(survived, "step:"+strconv.Itoa(step.Ordinal))
		}
	}
	if len(survived) > 0 {
		return testvalidity.Axis{State: testvalidity.StrengthSurvived, Reason: ReasonFaultSurvived, Anchors: survived}
	}
	if document.Inventory.AssertionsOutsideSteps > 0 {
		return notMeasured(ReasonAssertionsOutsideSteps, "assertionsOutsideSteps:"+strconv.Itoa(document.Inventory.AssertionsOutsideSteps))
	}
	var killed, unproven []string
	for _, step := range document.Inventory.Steps {
		if step.Assertions == 0 {
			continue
		}
		entry, ok := measured[step.Ordinal]
		if ok && entry.Strength.State == testvalidity.StrengthKilled {
			killed = append(killed, "plan:"+entry.PlanDigest)
			continue
		}
		unproven = append(unproven, "step:"+strconv.Itoa(step.Ordinal))
	}
	if len(unproven) == 0 && len(killed) > 0 {
		return testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: ReasonStepControlsKilled, Anchors: killed}
	}
	return notMeasured(ReasonStepControlsIncomplete, unproven...)
}
