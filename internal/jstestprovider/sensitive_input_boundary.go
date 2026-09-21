package jstestprovider

import (
	"encoding/json"
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

func RedactSensitiveInputEvidence(receipt Receipt) (Receipt, error) {
	policy, err := sensitivePolicy(receipt.SensitiveInputPolicy)
	if err != nil {
		return Receipt{}, err
	}
	if findings := sensitiveInputShapeFindings(receipt, policy); len(findings) != 0 {
		return Receipt{}, &SensitiveInputValidationError{Findings: findings}
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return Receipt{}, err
	}
	var out Receipt
	if err := json.Unmarshal(data, &out); err != nil {
		return Receipt{}, err
	}
	values := boundaryCollectSensitiveValues(out, policy)
	anySensitive := false
	for ti := range out.Tests {
		for ai := range out.Tests[ti].Attempts {
			anySensitive = boundaryRedactSteps(out.Tests[ti].Attempts[ai].Steps, policy) || anySensitive
		}
	}
	if anySensitive {
		for ti := range out.Tests {
			boundaryRedactOutcome(&out.Tests[ti])
		}
	}
	if anySensitive && out.Infrastructure != nil && out.Infrastructure.Detail != "" {
		out.Infrastructure.Detail = SensitiveInputRedactionMarker
	}
	boundaryScrubReceiptRiskFields(&out, values)
	return out, nil
}

func ValidateSensitiveInputEvidence(receipt Receipt) []SensitiveInputFinding {
	policy, err := sensitivePolicy(receipt.SensitiveInputPolicy)
	if err != nil {
		return []SensitiveInputFinding{{Code: "sensitive-input-policy-invalid", Path: "sensitiveInputPolicy"}}
	}
	if findings := sensitiveInputShapeFindings(receipt, policy); len(findings) != 0 {
		return findings
	}
	values := boundaryCollectSensitiveValues(receipt, policy)
	findings := []SensitiveInputFinding{}
	protectedReport := false
	for ti := range receipt.Tests {
		testSensitive := false
		for ai := range receipt.Tests[ti].Attempts {
			testSensitive = boundaryValidateSteps(receipt.Tests[ti].Attempts[ai].Steps, policy, fmt.Sprintf("tests[%d].attempts[%d].steps", ti, ai), &findings) || testSensitive
		}
		protectedReport = protectedReport || testSensitive
	}
	boundaryValidateRiskFields(receipt, values, protectedReport, &findings)
	return findings
}

func boundaryCollectSensitiveValues(receipt Receipt, policy normalizedSensitivePolicy) []string {
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if value != "" && value != SensitiveInputRedactionMarker {
			seen[value] = true
		}
	}
	var visit func([]BrowserStep)
	visit = func(steps []BrowserStep) {
		for _, step := range steps {
			display, tail := boundaryActionMatch(step.Title, policy)
			if display != "" {
				boundaryTailCandidates(tail, add)
			}
			for key, value := range step.Metadata {
				if policy.fields[strings.ToLower(key)] {
					add(value)
				}
			}
			visit(step.Steps)
		}
	}
	for _, test := range receipt.Tests {
		for _, attempt := range test.Attempts {
			visit(attempt.Steps)
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	return values
}

func boundaryScrubText(value string, sensitive []string) string {
	if value == SensitiveInputRedactionMarker {
		return value
	}
	for _, item := range sensitive {
		value = strings.ReplaceAll(value, item, SensitiveInputRedactionMarker)
	}
	return value
}

func boundaryScrubReceiptRiskFields(receipt *Receipt, sensitive []string) {
	var scrubSteps func([]BrowserStep)
	scrubSteps = func(steps []BrowserStep) {
		for i := range steps {
			steps[i].Error = boundaryScrubText(steps[i].Error, sensitive)
			for j := range steps[i].Attachments {
				steps[i].Attachments[j].Name = boundaryScrubText(steps[i].Attachments[j].Name, sensitive)
				steps[i].Attachments[j].Path = boundaryScrubText(steps[i].Attachments[j].Path, sensitive)
			}
			scrubSteps(steps[i].Steps)
		}
	}
	for i := range receipt.Tests {
		receipt.Tests[i].FailureMessage = boundaryScrubText(receipt.Tests[i].FailureMessage, sensitive)
		for j := range receipt.Tests[i].Artifacts {
			receipt.Tests[i].Artifacts[j].Name = boundaryScrubText(receipt.Tests[i].Artifacts[j].Name, sensitive)
			receipt.Tests[i].Artifacts[j].Path = boundaryScrubText(receipt.Tests[i].Artifacts[j].Path, sensitive)
		}
		for j := range receipt.Tests[i].Attempts {
			scrubSteps(receipt.Tests[i].Attempts[j].Steps)
		}
	}
	if receipt.Infrastructure != nil {
		receipt.Infrastructure.Detail = boundaryScrubText(receipt.Infrastructure.Detail, sensitive)
	}
}

func boundaryValidateRiskFields(receipt Receipt, sensitive []string, protectedReport bool, findings *[]SensitiveInputFinding) {
	unsafe := func(value string, protected bool) bool {
		if protected {
			return value != "" && value != SensitiveInputRedactionMarker
		}
		for _, item := range sensitive {
			if item != "" && strings.Contains(value, item) {
				return true
			}
		}
		return false
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

func boundaryCanonicalTitle(display string) string {
	return display + ` "` + SensitiveInputRedactionMarker + `"`
}

func sensitiveInputShapeFindings(receipt Receipt, policy normalizedSensitivePolicy) []SensitiveInputFinding {
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
		if current.depth > sensitiveInputMaxDepth {
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

func boundaryTooLong(value string) bool { return len(value) > sensitiveInputMaxStringBytes }
func boundaryBoundFinding(code string) []SensitiveInputFinding {
	return []SensitiveInputFinding{{Code: code, Path: "receipt"}}
}

func boundaryRedactSteps(steps []BrowserStep, policy normalizedSensitivePolicy) bool {
	any := false
	for i := range steps {
		step := &steps[i]
		display, sensitive := boundaryActionDisplay(step.Title, policy)
		sensitive = sensitive || step.Redacted
		for key := range step.Metadata {
			sensitive = sensitive || policy.fields[strings.ToLower(key)]
		}
		childSensitive := boundaryRedactSteps(step.Steps, policy)
		if sensitive {
			if display == "" {
				display = "Input"
			}
			step.Title = boundaryCanonicalTitle(display)
			for key := range step.Metadata {
				if policy.fields[strings.ToLower(key)] {
					step.Metadata[key] = SensitiveInputRedactionMarker
				}
			}
			step.Redacted = true
		}
		if sensitive || childSensitive {
			if step.Error != "" {
				step.Error = SensitiveInputRedactionMarker
			}
			for j := range step.Attachments {
				step.Attachments[j] = FailureArtifact{Name: SensitiveInputRedactionMarker, Path: SensitiveInputRedactionMarker}
			}
			any = true
		}
	}
	return any
}

func boundaryValidateSteps(steps []BrowserStep, policy normalizedSensitivePolicy, path string, findings *[]SensitiveInputFinding) bool {
	any := false
	for i, step := range steps {
		stepPath := fmt.Sprintf("%s[%d]", path, i)
		display, sensitive := boundaryActionDisplay(step.Title, policy)
		sensitive = sensitive || step.Redacted
		for key := range step.Metadata {
			sensitive = sensitive || policy.fields[strings.ToLower(key)]
		}
		if sensitive {
			if display == "" {
				display = "Input"
			}
			if step.Title != boundaryCanonicalTitle(display) || !step.Redacted {
				boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: stepPath})
			}
			for key, value := range step.Metadata {
				if policy.fields[strings.ToLower(key)] && value != SensitiveInputRedactionMarker {
					boundaryAppendFinding(findings, SensitiveInputFinding{Code: SensitiveInputUnredacted, Path: stepPath})
				}
			}
		}
		childSensitive := boundaryValidateSteps(step.Steps, policy, stepPath+".steps", findings)
		any = sensitive || childSensitive || any
	}
	return any
}

func boundaryRedactOutcome(outcome *TestOutcome) {
	if outcome.FailureMessage != "" {
		outcome.FailureMessage = SensitiveInputRedactionMarker
	}
	for i := range outcome.Artifacts {
		outcome.Artifacts[i] = FailureArtifact{Name: SensitiveInputRedactionMarker, Path: SensitiveInputRedactionMarker}
	}
	for i := range outcome.Attempts {
		boundaryRedactStepRiskFields(outcome.Attempts[i].Steps)
	}
}

func boundaryRedactStepRiskFields(steps []BrowserStep) {
	for i := range steps {
		if steps[i].Error != "" {
			steps[i].Error = SensitiveInputRedactionMarker
		}
		for j := range steps[i].Attachments {
			steps[i].Attachments[j] = FailureArtifact{Name: SensitiveInputRedactionMarker, Path: SensitiveInputRedactionMarker}
		}
		boundaryRedactStepRiskFields(steps[i].Steps)
	}
}

func boundaryAppendFinding(findings *[]SensitiveInputFinding, finding SensitiveInputFinding) {
	if len(*findings) >= sensitiveInputMaxFindings {
		(*findings)[sensitiveInputMaxFindings-1] = SensitiveInputFinding{Code: SensitiveInputFindingBoundExceeded, Path: "receipt"}
		return
	}
	*findings = append(*findings, finding)
}
