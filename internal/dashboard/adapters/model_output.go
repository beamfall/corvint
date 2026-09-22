package adapters

import (
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/dashboard/model"
)

// TraceModelInputs translates one fully verified default trace-store
// aggregate into the model's closed source and retained-usage inputs. Paths,
// tasks, commands, trace IDs, and verifier errors are absent by construction.
func TraceModelInputs(configuredOrdinal uint64, aggregate TraceAggregate, members []VerifiedTraceMember, dirtyPathsSHA256 *string, completeness model.Completeness, terminalCounts []TraceTerminalCount) (model.SourceInput, []model.MetricInput, []model.IssueInput, error) {
	snapshotHead, ok := aggregateSnapshotHead(aggregate)
	if !ok {
		return model.SourceInput{}, nil, nil, invalidArgumentError()
	}
	recomputed, code := AggregateTraceMembers(snapshotHead, members)
	if code != "" || !traceAggregatesEqual(aggregate, recomputed) ||
		(completeness != model.CompletenessComplete && completeness != model.CompletenessPartial) {
		return model.SourceInput{}, nil, nil, invalidArgumentError()
	}
	profile := "corvint-local-trace/1"
	verifier := "go-local-trace-v1"
	adapterID := string(AdapterLocalTrace)
	ordinal := strconv.FormatUint(configuredOrdinal, 10)
	cohorts := make([]model.CohortIdentity, 0, len(members))
	cohortIDs := make(map[string]string, len(members))
	for _, member := range members {
		revision := member.Summary.Revision
		tree := member.Summary.TreeRevision
		objectFormat := member.Summary.ObjectFormat
		producer := verifier
		cohort := model.CohortIdentity{
			AdapterID: adapterID, Profile: profile, RepositoryObjectFormat: &objectFormat,
			SourceRevision: &revision, SourceTreeRevision: &tree,
			DirtyPathsSHA256: cloneOptional(dirtyPathsSHA256), ProducerIdentity: &producer,
			SourceObservationStart: stringPointer(member.Start), SourceObservationEnd: stringPointer(member.End),
		}
		cohortID, err := model.ComputeCohortID(cohort)
		if err != nil {
			return model.SourceInput{}, nil, nil, invalidArgumentError()
		}
		cohorts = append(cohorts, cohort)
		cohortIDs[revision] = cohortID
	}
	modelMembers := make([]model.TraceMember, len(aggregate.Members))
	for index, member := range aggregate.Members {
		modelMembers[index] = model.TraceMember{
			Revision: member.Revision, ContentSHA256: member.ContentSHA256, ByteCount: member.ByteCount,
		}
	}
	byteCount := strconv.FormatUint(aggregate.ByteCount, 10)
	repositoryWitnesses := cloneWitnesses(aggregate.repositoryWitnesses)
	sourceInput := model.SourceInput{
		AdapterID: adapterID, ConfiguredOrdinal: ordinal, Profile: profile,
		AuthorityClass: model.AuthorityAdapterQualified, ByteCount: &byteCount,
		Completeness: completeness, ContentSHA256: stringPointer(aggregate.ContentSHA256),
		Currency: model.CurrencyValidatedAt, DeliveryStage: model.DeliveryNotStarted,
		EpistemicClass:      model.EpistemicObserved,
		RepositoryWitnesses: repositoryWitnesses,
		Validity:            model.ValidityValid, VerifierID: verifier, Cohorts: cohorts,
		Members: &modelMembers, ObservationStart: cloneOptional(aggregate.ObservationStart),
		ObservationEnd: cloneOptional(aggregate.ObservationEnd),
	}
	readsHash, err := model.ComputeRepositoryReadsSHA256(sourceInput.RepositoryWitnesses)
	if err != nil {
		return model.SourceInput{}, nil, nil, invalidArgumentError()
	}
	sourceID, err := model.ComputeSourceID(model.SourceIdentity{
		AdapterID: adapterID, ConfiguredOrdinal: ordinal, ContentSHA256: sourceInput.ContentSHA256,
		Profile: profile, RepositoryReadsSHA256: &readsHash,
	})
	if err != nil {
		return model.SourceInput{}, nil, nil, invalidArgumentError()
	}
	terminalIssues, terminalExclusions, err := TraceTerminalIssueInputs(sourceID, terminalCounts)
	if err != nil || (completeness == model.CompletenessComplete && len(terminalIssues) != 0) ||
		(completeness == model.CompletenessPartial && len(terminalIssues) == 0) {
		return model.SourceInput{}, nil, nil, invalidArgumentError()
	}
	sourceInput.Exclusions = terminalExclusions
	observationIssue := model.IssueInput{
		Code: "OBSERVATION_TIME_UNKNOWN", Severity: model.SeverityInfo,
		SourceID: stringPointer(sourceID),
	}
	if _, err := model.NewIssue(observationIssue); err != nil {
		return model.SourceInput{}, nil, nil, invalidArgumentError()
	}
	metrics := make([]model.MetricInput, 0, len(members)*3)
	for _, member := range members {
		counts := map[string]uint64{"blocked": 0, "failed": 0, "passed": 0}
		for _, outcome := range member.Summary.Outcomes {
			counts[outcome.Outcome] = outcome.Count
		}
		denominator := strconv.FormatUint(member.Summary.RetainedRows, 10)
		for _, outcome := range []string{"blocked", "failed", "passed"} {
			value := strconv.FormatUint(counts[outcome], 10)
			revision := member.Summary.Revision
			metrics = append(metrics, model.MetricInput{
				AuthorityClass: model.AuthorityAdvisory, CohortIDs: []string{cohortIDs[revision]},
				Completeness: model.CompletenessComplete, Currency: model.CurrencyValidatedAt,
				Denominator: &denominator, DeliveryStage: model.DeliveryNotStarted,
				Dimensions:     []model.Dimension{{Name: "outcome", Value: stringPointer(outcome)}, {Name: "revision", Value: &revision}},
				EpistemicClass: model.EpistemicObserved, Name: "usage.trace.retained",
				Numerator: &value, ScopeClass: model.ScopeSingleCohort, SourceIDs: []string{sourceID},
				Unit: "COUNT", Validity: model.ValidityValid, Value: &value,
			})
		}
	}
	issues := append(terminalIssues, observationIssue)
	return sourceInput, metrics, issues, nil
}

// MergeTraceUsageMetrics groups per-source trace rows by the snapshot's exact
// metric key and unions their source IDs. Counts and denominators are checked
// sums, so multiple configured trace sources cannot create duplicate keys.
func MergeTraceUsageMetrics(inputs []model.MetricInput) ([]model.MetricInput, error) {
	type group struct {
		metric  model.MetricInput
		sources map[string]struct{}
	}
	groups := make(map[string]*group, len(inputs))
	keys := make([]string, 0, len(inputs))
	for _, input := range inputs {
		key, ok := traceUsageMetricKey(input)
		if !ok {
			return nil, invalidArgumentError()
		}
		current, exists := groups[key]
		if !exists {
			cloned := cloneMetricInput(input)
			current = &group{metric: cloned, sources: make(map[string]struct{}, len(input.SourceIDs))}
			groups[key] = current
			keys = append(keys, key)
		} else {
			var ok bool
			if current.metric.Value, ok = addDecimalPointers(current.metric.Value, input.Value); !ok {
				return nil, invalidArgumentError()
			}
			if current.metric.Numerator, ok = addDecimalPointers(current.metric.Numerator, input.Numerator); !ok {
				return nil, invalidArgumentError()
			}
			if current.metric.Denominator, ok = addDecimalPointers(current.metric.Denominator, input.Denominator); !ok {
				return nil, invalidArgumentError()
			}
		}
		for _, sourceID := range input.SourceIDs {
			if !validDashboardSourceID(sourceID) {
				return nil, invalidArgumentError()
			}
			current.sources[sourceID] = struct{}{}
		}
	}
	sort.Strings(keys)
	result := make([]model.MetricInput, 0, len(keys))
	for _, key := range keys {
		current := groups[key]
		current.metric.SourceIDs = current.metric.SourceIDs[:0]
		for sourceID := range current.sources {
			current.metric.SourceIDs = append(current.metric.SourceIDs, sourceID)
		}
		sort.Strings(current.metric.SourceIDs)
		result = append(result, current.metric)
	}
	return result, nil
}

func traceUsageMetricKey(input model.MetricInput) (string, bool) {
	if input.Name != "usage.trace.retained" || len(input.Dimensions) != 2 || len(input.CohortIDs) != 1 ||
		len(input.SourceIDs) == 0 || len(input.Exclusions) != 0 || input.Window != nil ||
		input.AuthorityClass != model.AuthorityAdvisory || input.Completeness != model.CompletenessComplete ||
		input.Currency != model.CurrencyValidatedAt || input.DeliveryStage != model.DeliveryNotStarted ||
		input.EpistemicClass != model.EpistemicObserved || input.ScopeClass != model.ScopeSingleCohort ||
		input.Unit != "COUNT" || input.Validity != model.ValidityValid || input.Value == nil ||
		input.Numerator == nil || input.Denominator == nil || *input.Value != *input.Numerator {
		return "", false
	}
	values := make(map[string]string, 2)
	for _, dimension := range input.Dimensions {
		if dimension.Value == nil || (dimension.Name != "outcome" && dimension.Name != "revision") {
			return "", false
		}
		if _, duplicate := values[dimension.Name]; duplicate {
			return "", false
		}
		values[dimension.Name] = *dimension.Value
	}
	if (values["outcome"] != "blocked" && values["outcome"] != "failed" && values["outcome"] != "passed") ||
		!validAnyObjectID(values["revision"]) || !validDashboardCohortID(input.CohortIDs[0]) {
		return "", false
	}
	if _, err := strconv.ParseUint(*input.Value, 10, 64); err != nil {
		return "", false
	}
	if _, err := strconv.ParseUint(*input.Denominator, 10, 64); err != nil {
		return "", false
	}
	return values["outcome"] + "\x00" + values["revision"] + "\x00" + input.CohortIDs[0], true
}

func validDashboardCohortID(value string) bool {
	digest, ok := strings.CutPrefix(value, "dashboard-cohort:sha256:")
	return ok && lowerHex(digest, 64)
}

func addDecimalPointers(left, right *string) (*string, bool) {
	if left == nil || right == nil {
		return nil, false
	}
	leftValue, leftErr := strconv.ParseUint(*left, 10, 64)
	rightValue, rightErr := strconv.ParseUint(*right, 10, 64)
	if leftErr != nil || rightErr != nil || leftValue > ^uint64(0)-rightValue {
		return nil, false
	}
	value := strconv.FormatUint(leftValue+rightValue, 10)
	return &value, true
}

func cloneMetricInput(input model.MetricInput) model.MetricInput {
	result := input
	result.CohortIDs = append([]string(nil), input.CohortIDs...)
	result.Dimensions = append([]model.Dimension(nil), input.Dimensions...)
	for index := range result.Dimensions {
		result.Dimensions[index].Value = cloneOptional(result.Dimensions[index].Value)
	}
	result.Exclusions = append([]string(nil), input.Exclusions...)
	result.SourceIDs = append([]string(nil), input.SourceIDs...)
	result.Value = cloneOptional(input.Value)
	result.Numerator = cloneOptional(input.Numerator)
	result.Denominator = cloneOptional(input.Denominator)
	return result
}

func traceAggregatesEqual(left, right TraceAggregate) bool {
	return left.ByteCount == right.ByteCount && left.RetainedRows == right.RetainedRows && left.ContentSHA256 == right.ContentSHA256 &&
		reflect.DeepEqual(left.Members, right.Members) &&
		reflect.DeepEqual(left.ObservationStart, right.ObservationStart) &&
		reflect.DeepEqual(left.ObservationEnd, right.ObservationEnd) &&
		reflect.DeepEqual(left.repositoryWitnesses, right.repositoryWitnesses)
}

func aggregateSnapshotHead(aggregate TraceAggregate) (model.RepositoryWitness, bool) {
	var result model.RepositoryWitness
	found := false
	for _, witness := range aggregate.repositoryWitnesses {
		if witness.Kind != "SNAPSHOT_HEAD" {
			continue
		}
		if found {
			return model.RepositoryWitness{}, false
		}
		result = cloneWitness(witness)
		found = true
	}
	return result, found
}

func cloneWitnesses(input []model.RepositoryWitness) []model.RepositoryWitness {
	if input == nil {
		return nil
	}
	result := make([]model.RepositoryWitness, len(input))
	for index, witness := range input {
		result[index] = cloneWitness(witness)
	}
	return result
}

func cloneWitness(witness model.RepositoryWitness) model.RepositoryWitness {
	result := witness
	result.Revision = cloneOptional(witness.Revision)
	return result
}

func cloneOptional(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
