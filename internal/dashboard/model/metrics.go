package model

import "strings"

type metricDefinition struct {
	group string
	unit  string
	stage DeliveryStage
}

var metricDefinitions = map[string]metricDefinition{
	"data.artifact.bytes":          {"data", "BYTES", DeliveryImplemented},
	"data.artifact.count":          {"data", "COUNT", DeliveryImplemented},
	"usage.trace.retained":         {"usage", "COUNT", DeliveryNotStarted},
	"usage.query.retained":         {"usage", "COUNT", DeliveryUnsupported},
	"usage.impact.retained":        {"usage", "COUNT", DeliveryUnsupported},
	"usage.harness.retained":       {"usage", "COUNT", DeliveryUnsupported},
	"usage.response.bytes":         {"usage", "BYTES", DeliveryUnsupported},
	"usage.latency.p50":            {"usage", "NANOSECONDS", DeliveryUnsupported},
	"usage.latency.p95":            {"usage", "NANOSECONDS", DeliveryUnsupported},
	"verification.cem.hunks":       {"verification", "COUNT", DeliveryNotStarted},
	"verification.ocm.obligations": {"verification", "COUNT", DeliveryNotStarted},
	"verification.closure":         {"verification", "RATIO", DeliveryNotStarted},
	"verification.live.runs":       {"verification", "COUNT", DeliveryUnsupported},
	"verification.pulse.runs":      {"verification", "COUNT", DeliveryUnsupported},
	"frontier.items":               {"frontier", "COUNT", DeliveryUnsupported},
	"beamfall.shadow.runs":         {"beamfall", "COUNT", DeliveryUnsupported},
}

func defaultUnavailableMetric(name string, definition metricDefinition) Metric {
	return Metric{
		AuthorityClass: AuthorityNone, CohortIDs: []string{}, Completeness: CompletenessUnknown,
		Currency: CurrencyUnknown, DeliveryStage: definition.stage, Dimensions: []Dimension{},
		EpistemicClass: EpistemicNotObserved, Exclusions: []string{}, Name: name,
		ScopeClass: ScopeUnavailable, SourceIDs: []string{}, Unit: definition.unit,
		Validity: ValidityUnsupported,
	}
}

func metricGroup(name string) string {
	if definition, ok := metricDefinitions[name]; ok {
		return definition.group
	}
	return strings.SplitN(name, ".", 2)[0]
}
