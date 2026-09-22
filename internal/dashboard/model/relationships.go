package model

func validateTraceRelationships(metrics []Metric, sources []Source, cohorts []Cohort) error {
	cohortByID := make(map[string]Cohort, len(cohorts))
	for _, cohort := range cohorts {
		cohortByID[cohort.CohortID] = cohort
	}
	sourceByID := make(map[string]Source, len(sources))
	for _, source := range sources {
		sourceByID[source.ID] = source
	}
	totals := make(map[string]uint64)
	trace := make([]Metric, 0)
	for _, metric := range metrics {
		if metric.Name != "usage.trace.retained" || metric.ScopeClass == ScopeUnavailable {
			continue
		}
		trace = append(trace, metric)
		if len(metric.SourceIDs) == 0 {
			return invalidArgument()
		}
		value, ok := parseDecimal(pointerValue(metric.Value))
		if !ok {
			return invalidArgument()
		}
		cohortID := metric.CohortIDs[0]
		if totals[cohortID] > ^uint64(0)-value {
			return resourceExhausted()
		}
		totals[cohortID] += value
		values := make(map[string]*string, len(metric.Dimensions))
		for _, dimension := range metric.Dimensions {
			values[dimension.Name] = dimension.Value
		}
		outcome := pointerValue(values["outcome"])
		if !oneOf(outcome, "passed", "failed", "blocked") {
			return invalidArgument()
		}
		cohort, ok := cohortByID[metric.CohortIDs[0]]
		if !ok || pointerValue(values["revision"]) != pointerValue(cohort.SourceRevision) {
			return invalidArgument()
		}
		for _, id := range metric.SourceIDs {
			source, ok := sourceByID[id]
			if !ok || source.AdapterID != "local-trace-v1" || !containsString(source.CohortIDs, metric.CohortIDs[0]) {
				return invalidArgument()
			}
		}
	}
	for _, metric := range trace {
		denominator, ok := parseDecimal(pointerValue(metric.Denominator))
		if !ok || denominator != totals[metric.CohortIDs[0]] {
			return invalidArgument()
		}
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
