package model

import (
	"bytes"
	"sort"
	"strings"
)

func compileMetrics(inputs []MetricInput) (map[string][]Metric, error) {
	if len(inputs) > MaxMetricSamples {
		return nil, resourceExhausted()
	}
	grouped := map[string][]Metric{
		"data": {}, "usage": {}, "verification": {}, "frontier": {}, "beamfall": {},
	}
	present := make(map[string]bool)
	keys := make(map[string]struct{})
	for _, input := range inputs {
		definition, ok := metricDefinitions[input.Name]
		if !ok {
			return nil, invalidArgument()
		}
		if definition.group == "data" {
			return nil, invalidArgument()
		}
		metric := Metric{
			AuthorityClass: input.AuthorityClass, Completeness: input.Completeness, Currency: input.Currency,
			Denominator: cloneString(input.Denominator), DeliveryStage: input.DeliveryStage,
			Dimensions: cloneDimensions(input.Dimensions), EpistemicClass: input.EpistemicClass,
			Name: input.Name, Numerator: cloneString(input.Numerator), ScopeClass: input.ScopeClass,
			Unit: input.Unit, Validity: input.Validity, Value: cloneString(input.Value),
		}
		var err error
		if metric.CohortIDs, err = normalizeSet(input.CohortIDs); err != nil {
			return nil, err
		}
		if metric.Exclusions, err = normalizeSet(input.Exclusions); err != nil {
			return nil, err
		}
		if metric.SourceIDs, err = normalizeSet(input.SourceIDs); err != nil {
			return nil, err
		}
		sort.Slice(metric.Dimensions, func(i, j int) bool {
			if metric.Dimensions[i].Name != metric.Dimensions[j].Name {
				return metric.Dimensions[i].Name < metric.Dimensions[j].Name
			}
			return pointerValue(metric.Dimensions[i].Value) < pointerValue(metric.Dimensions[j].Value)
		})
		metric.Window = cloneWindow(input.Window)
		if err := validateMetric(metric, definition); err != nil {
			return nil, err
		}
		keyBytes, _ := canonicalJSON(struct {
			Name       string      `json:"name"`
			Dimensions []Dimension `json:"dimensions"`
			CohortIDs  []string    `json:"cohortIds"`
		}{metric.Name, metric.Dimensions, metric.CohortIDs})
		key := string(keyBytes)
		if _, exists := keys[key]; exists {
			return nil, invalidArgument()
		}
		keys[key] = struct{}{}
		present[input.Name] = true
		grouped[definition.group] = append(grouped[definition.group], metric)
	}
	for name, definition := range metricDefinitions {
		if definition.group == "data" {
			continue
		}
		if !present[name] {
			grouped[definition.group] = append(grouped[definition.group], defaultUnavailableMetric(name, definition))
		}
	}
	for group := range grouped {
		sort.Slice(grouped[group], func(i, j int) bool { return metricLess(grouped[group][i], grouped[group][j]) })
	}
	return grouped, nil
}

func validateMetric(metric Metric, definition metricDefinition) error {
	if metric.Unit != definition.unit || !validAxes(metric.Validity, metric.EpistemicClass, metric.AuthorityClass, metric.Completeness, metric.Currency, metric.DeliveryStage) ||
		!oneOf(metric.ScopeClass, ScopeSingleCohort, ScopeMultiCohortInventory, ScopeUnavailable) {
		return invalidArgument()
	}
	for _, values := range [][]string{metric.CohortIDs, metric.SourceIDs, metric.Exclusions} {
		if !isCanonicalSet(values) {
			return invalidArgument()
		}
	}
	for _, value := range append(append([]string{}, metric.CohortIDs...), append(metric.SourceIDs, metric.Exclusions...)...) {
		if !strings.Contains(value, ":sha256:") {
			return invalidArgument()
		}
	}
	if metric.Window != nil && (!validateTimestamp(metric.Window.Start) || !validateTimestamp(metric.Window.End) || metric.Window.Start > metric.Window.End) {
		return invalidArgument()
	}
	for _, value := range []*string{metric.Value, metric.Numerator, metric.Denominator} {
		if value != nil && !validateDecimal(*value) {
			return invalidArgument()
		}
	}
	if metric.ScopeClass == ScopeUnavailable {
		if len(metric.Dimensions) != 0 || len(metric.CohortIDs) != 0 || len(metric.SourceIDs) != 0 || metric.Value != nil || metric.Numerator != nil || metric.Denominator != nil ||
			metric.EpistemicClass != EpistemicNotObserved || metric.Completeness != CompletenessUnknown || metric.AuthorityClass != AuthorityNone ||
			metric.Currency != CurrencyUnknown || metric.Validity != ValidityUnsupported || len(metric.Exclusions) != 0 || metric.Window != nil || metric.DeliveryStage != definition.stage {
			return invalidArgument()
		}
		return nil
	}
	var names []string
	for _, dimension := range metric.Dimensions {
		names = append(names, dimension.Name)
	}
	if hasDuplicate(names) {
		return invalidArgument()
	}
	switch metric.Name {
	case "data.artifact.count", "data.artifact.bytes":
		if strings.Join(names, ",") != "adapterId,ageBucket,artifactKind,authorityClass,deliveryStage,revision,validity" || metric.ScopeClass != ScopeMultiCohortInventory {
			return invalidArgument()
		}
	case "usage.trace.retained":
		if strings.Join(names, ",") != "outcome,revision" || metric.ScopeClass != ScopeSingleCohort || len(metric.CohortIDs) != 1 {
			return invalidArgument()
		}
		if metric.AuthorityClass != AuthorityAdvisory || metric.EpistemicClass != EpistemicObserved || metric.Completeness != CompletenessComplete ||
			metric.Currency != CurrencyValidatedAt || metric.DeliveryStage != DeliveryNotStarted || metric.Validity != ValidityValid || metric.Value == nil || metric.Numerator == nil || metric.Denominator == nil || *metric.Value != *metric.Numerator {
			return invalidArgument()
		}
	default:
		return invalidArgument() // At-base metrics other than inventory/trace are unavailable.
	}
	return nil
}

func isCanonicalSet(values []string) bool {
	normalized, err := normalizeSet(values)
	if err != nil || len(normalized) != len(values) {
		return false
	}
	for index := range values {
		if values[index] != normalized[index] {
			return false
		}
	}
	return true
}

func metricLess(left, right Metric) bool {
	if left.Name != right.Name {
		return left.Name < right.Name
	}
	leftDimensions, _ := canonicalJSON(left.Dimensions)
	rightDimensions, _ := canonicalJSON(right.Dimensions)
	if comparison := bytes.Compare(leftDimensions, rightDimensions); comparison != 0 {
		return comparison < 0
	}
	leftCohorts, _ := canonicalJSON(left.CohortIDs)
	rightCohorts, _ := canonicalJSON(right.CohortIDs)
	return bytes.Compare(leftCohorts, rightCohorts) < 0
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func cloneWindow(value *Window) *Window {
	if value == nil {
		return nil
	}
	return &Window{End: value.End, Start: value.Start}
}

func cloneDimensions(values []Dimension) []Dimension {
	result := make([]Dimension, len(values))
	for index, value := range values {
		result[index] = Dimension{Name: value.Name, Value: cloneString(value.Value)}
	}
	return result
}
