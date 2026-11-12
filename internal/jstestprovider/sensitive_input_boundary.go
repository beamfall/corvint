package jstestprovider

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

func RedactSensitiveInputEvidence(receipt Receipt) (Receipt, error) {
	policy, err := jstestreceipt.SensitivePolicy(receipt.SensitiveInputPolicy)
	if err != nil {
		return Receipt{}, err
	}
	if findings := jstestreceipt.SensitiveInputShapeFindings(receipt, policy); len(findings) != 0 {
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

// ValidateSensitiveInputEvidence is jstestreceipt.ValidateSensitiveInputEvidence, kept under its old name.
func ValidateSensitiveInputEvidence(receipt Receipt) []SensitiveInputFinding {
	return jstestreceipt.ValidateSensitiveInputEvidence(receipt)
}

func boundaryCollectSensitiveValues(receipt Receipt, policy jstestreceipt.NormalizedSensitivePolicy) []string {
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
			display, tail := jstestreceipt.BoundaryActionMatch(step.Title, policy)
			if display != "" {
				boundaryTailCandidates(tail, add)
			}
			for key, value := range step.Metadata {
				if policy.Fields[strings.ToLower(key)] {
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
	// Longest first, so a value that is a prefix of another never leaves the
	// longer value's tail behind; the bytewise tiebreak keeps output deterministic.
	sort.Slice(values, func(i, j int) bool {
		if len(values[i]) != len(values[j]) {
			return len(values[i]) > len(values[j])
		}
		return values[i] < values[j]
	})
	return values
}

// boundaryScrubReplacer builds one strings.Replacer for the whole receipt: a single pass per
// field instead of one strings.ReplaceAll per sensitive value per field. Patterns keep
// boundaryCollectSensitiveValues' longest-first order, which Replacer also uses to pick the
// match at a given position, so a value that is a prefix of another still never leaves the
// longer value's tail behind.
func boundaryScrubReplacer(sensitive []string) *strings.Replacer {
	pairs := make([]string, 0, 2*len(sensitive))
	for _, item := range sensitive {
		pairs = append(pairs, item, SensitiveInputRedactionMarker)
	}
	return strings.NewReplacer(pairs...)
}

func boundaryScrubText(replacer *strings.Replacer, value string) string {
	if value == SensitiveInputRedactionMarker {
		return value
	}
	return replacer.Replace(value)
}

func boundaryScrubReceiptRiskFields(receipt *Receipt, sensitive []string) {
	replacer := boundaryScrubReplacer(sensitive)
	var scrubSteps func([]BrowserStep)
	scrubSteps = func(steps []BrowserStep) {
		for i := range steps {
			steps[i].Error = boundaryScrubText(replacer, steps[i].Error)
			for j := range steps[i].Attachments {
				steps[i].Attachments[j].Name = boundaryScrubText(replacer, steps[i].Attachments[j].Name)
				steps[i].Attachments[j].Path = boundaryScrubText(replacer, steps[i].Attachments[j].Path)
			}
			scrubSteps(steps[i].Steps)
		}
	}
	for i := range receipt.Tests {
		receipt.Tests[i].FailureMessage = boundaryScrubText(replacer, receipt.Tests[i].FailureMessage)
		receipt.Tests[i].AttemptDetails = nil
		for j := range receipt.Tests[i].Artifacts {
			receipt.Tests[i].Artifacts[j].Name = boundaryScrubText(replacer, receipt.Tests[i].Artifacts[j].Name)
			receipt.Tests[i].Artifacts[j].Path = boundaryScrubText(replacer, receipt.Tests[i].Artifacts[j].Path)
		}
		for j := range receipt.Tests[i].Attempts {
			scrubSteps(receipt.Tests[i].Attempts[j].Steps)
		}
	}
	if receipt.Infrastructure != nil {
		receipt.Infrastructure.Detail = boundaryScrubText(replacer, receipt.Infrastructure.Detail)
	}
}

func boundaryRedactSteps(steps []BrowserStep, policy jstestreceipt.NormalizedSensitivePolicy) bool {
	any := false
	for i := range steps {
		step := &steps[i]
		display, sensitive := jstestreceipt.BoundaryActionDisplay(step.Title, policy)
		sensitive = sensitive || step.Redacted
		for key := range step.Metadata {
			sensitive = sensitive || policy.Fields[strings.ToLower(key)]
		}
		childSensitive := boundaryRedactSteps(step.Steps, policy)
		if sensitive {
			if display == "" {
				display = "Input"
			}
			step.Title = jstestreceipt.BoundaryCanonicalTitle(display)
			for key := range step.Metadata {
				if policy.Fields[strings.ToLower(key)] {
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

func boundaryRedactOutcome(outcome *TestOutcome) {
	outcome.AttemptDetails = nil
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
