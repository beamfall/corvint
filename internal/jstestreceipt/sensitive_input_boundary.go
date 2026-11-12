package jstestreceipt

import (
	"fmt"
	"strings"
)

var boundaryDefaultActions = []struct {
	patterns []string
	display  string
}{
	{[]string{"fill"}, "Fill"},
	{[]string{"type"}, "Type"},
	{[]string{"inserttext", "insert text", "insert-text"}, "InsertText"},
	{[]string{"presssequentially", "press sequentially"}, "PressSequentially"},
}

func ValidateSensitiveInputEvidence(receipt Receipt) []SensitiveInputFinding {
	policy, err := SensitivePolicy(receipt.SensitiveInputPolicy)
	if err != nil {
		return []SensitiveInputFinding{{Code: "sensitive-input-policy-invalid", Path: "sensitiveInputPolicy"}}
	}
	if findings := SensitiveInputShapeFindings(receipt, policy); len(findings) != 0 {
		return findings
	}
	findings := []SensitiveInputFinding{}
	protectedReport := false
	for ti := range receipt.Tests {
		testSensitive := false
		for ai := range receipt.Tests[ti].Attempts {
			testSensitive = boundaryValidateSteps(receipt.Tests[ti].Attempts[ai].Steps, policy, fmt.Sprintf("tests[%d].attempts[%d].steps", ti, ai), &findings) || testSensitive
		}
		protectedReport = protectedReport || testSensitive
	}
	boundaryValidateRiskFields(receipt, protectedReport, &findings)
	return findings
}

// A non-empty sensitive value set always comes from a step that
// boundaryValidateSteps also flags, so protectedReport is true whenever there
// is anything to search for; risk fields on a protected report must already be
// empty or the marker, and no substring search is needed.
func boundaryValidateRiskFields(receipt Receipt, protectedReport bool, findings *[]SensitiveInputFinding) {
	unsafe := func(value string, protected bool) bool {
		return protected && value != "" && value != SensitiveInputRedactionMarker
	}
	var visit func([]BrowserStep, string, bool)
	visit = func(steps []BrowserStep, path string, protected bool) {
		for i, step := range steps {
			stepPath := fmt.Sprintf("%s[%d]", path, i)
			if unsafe(step.Error, protected) {
				boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: stepPath + ".error"})
			}
			for _, attachment := range step.Attachments {
				if unsafe(attachment.Name, protected) || unsafe(attachment.Path, protected) {
					boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: stepPath + ".attachments"})
				}
			}
			visit(step.Steps, stepPath+".steps", protected)
		}
	}
	for i, test := range receipt.Tests {
		protected := protectedReport
		path := fmt.Sprintf("tests[%d]", i)
		if unsafe(test.FailureMessage, protected) {
			boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: path + ".failureMessage"})
		}
		for _, artifact := range test.Artifacts {
			if unsafe(artifact.Name, protected) || unsafe(artifact.Path, protected) {
				boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: path + ".artifacts"})
			}
		}
		for j, attempt := range test.Attempts {
			visit(attempt.Steps, fmt.Sprintf("%s.attempts[%d].steps", path, j), protected)
		}
	}
	if receipt.Infrastructure != nil && unsafe(receipt.Infrastructure.Detail, protectedReport) {
		boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: "infrastructure.detail"})
	}
}

func BoundaryCanonicalTitle(display string) string {
	return display + ` "` + SensitiveInputRedactionMarker + `"`
}

func SensitiveInputShapeFindings(receipt Receipt, policy NormalizedSensitivePolicy) []SensitiveInputFinding {
	type frame struct {
		steps []BrowserStep
		depth int
	}
	stack := []frame{}
	for _, test := range receipt.Tests {
		if boundaryTooLong(test.FailureMessage) {
			return boundaryBoundFinding(SensitiveInputStringBoundExceeded)
		}
		for _, artifact := range test.Artifacts {
			if boundaryTooLong(artifact.Name) || boundaryTooLong(artifact.Path) {
				return boundaryBoundFinding(SensitiveInputStringBoundExceeded)
			}
		}
		for _, attempt := range test.Attempts {
			stack = append(stack, frame{attempt.Steps, 1})
		}
	}
	if receipt.Infrastructure != nil && boundaryTooLong(receipt.Infrastructure.Detail) {
		return boundaryBoundFinding(SensitiveInputStringBoundExceeded)
	}
	count := 0
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current.depth > SensitiveInputMaxDepth {
			return boundaryBoundFinding(SensitiveInputDepthExceeded)
		}
		count += len(current.steps)
		if count > sensitiveInputMaxSteps {
			return boundaryBoundFinding(SensitiveInputStepBoundExceeded)
		}
		for i := range current.steps {
			step := &current.steps[i]
			if boundaryTooLong(step.Title) || boundaryTooLong(step.Category) || boundaryTooLong(step.Error) {
				return boundaryBoundFinding(SensitiveInputStringBoundExceeded)
			}
			for key, value := range step.Metadata {
				if boundaryTooLong(key) || boundaryTooLong(value) {
					return boundaryBoundFinding(SensitiveInputStringBoundExceeded)
				}
			}
			for _, attachment := range step.Attachments {
				if boundaryTooLong(attachment.Name) || boundaryTooLong(attachment.Path) {
					return boundaryBoundFinding(SensitiveInputStringBoundExceeded)
				}
			}
			if boundaryUnsupportedSensitiveAction(step.Title, policy) {
				return boundaryBoundFinding(SensitiveInputActionSyntaxUnsupported)
			}
			if len(step.Steps) > 0 {
				stack = append(stack, frame{step.Steps, current.depth + 1})
			}
		}
	}
	return nil
}

func boundaryTooLong(value string) bool { return len(value) > SensitiveInputMaxStringBytes }

func boundaryBoundFinding(code string) []SensitiveInputFinding {
	return []SensitiveInputFinding{{Code: code, Path: "receipt"}}
}

func boundaryValidateSteps(steps []BrowserStep, policy NormalizedSensitivePolicy, path string, findings *[]SensitiveInputFinding) bool {
	any := false
	for i, step := range steps {
		stepPath := fmt.Sprintf("%s[%d]", path, i)
		display, sensitive := BoundaryActionDisplay(step.Title, policy)
		sensitive = sensitive || step.Redacted
		for key := range step.Metadata {
			sensitive = sensitive || policy.Fields[strings.ToLower(key)]
		}
		if sensitive {
			if display == "" {
				display = "Input"
			}
			if step.Title != BoundaryCanonicalTitle(display) || !step.Redacted {
				boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: stepPath})
			}
			for key, value := range step.Metadata {
				if policy.Fields[strings.ToLower(key)] && value != SensitiveInputRedactionMarker {
					boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: stepPath})
				}
			}
		}
		childSensitive := boundaryValidateSteps(step.Steps, policy, stepPath+".steps", findings)
		any = sensitive || childSensitive || any
	}
	return any
}

// boundaryAppendFinding keeps the first sensitiveInputMaxFindings-1 findings
// and turns the last slot into the finding-bound-exceeded marker (spec code
// sensitive-input-finding-bound-exceeded); later findings are dropped.
func boundaryAppendFinding(findings *[]SensitiveInputFinding, finding SensitiveInputFinding) {
	if len(*findings) >= SensitiveInputMaxFindings {
		(*findings)[SensitiveInputMaxFindings-1] = SensitiveInputFinding{Code: SensitiveInputFindingBoundExceeded, Path: "receipt"}
		return
	}
	*findings = append(*findings, finding)
}
