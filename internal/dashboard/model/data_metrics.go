package model

import (
	"sort"
	"time"
)

func deriveDataMetrics(sources []Source, cohorts []Cohort, issues []Issue, generatedAt string, scan ScanState) ([]Metric, error) {
	cohortByID := make(map[string]Cohort, len(cohorts))
	for _, cohort := range cohorts {
		cohortByID[cohort.CohortID] = cohort
	}
	issueByID := make(map[string]Issue, len(issues))
	for _, issue := range issues {
		issueByID[issue.ID] = issue
	}
	var totalConfiguredBytes uint64
	for _, source := range sources {
		if source.ByteCount != nil {
			value, _ := parseDecimal(*source.ByteCount)
			totalConfiguredBytes += value
		}
	}
	type configuredBucket struct {
		dimensions []Dimension
		cohortIDs  []string
		sources    []Source
		bytes      uint64
		hasBytes   bool
	}
	buckets := make(map[string]*configuredBucket)
	metrics := make([]Metric, 0, 2*len(sources)+4)
	memberCandidates := make([]Metric, 0)
	for _, source := range sources {
		age, err := sourceAgeBucket(source, generatedAt)
		if err != nil {
			return nil, err
		}
		var revision *string
		if source.AdapterID != "local-trace-v1" && len(source.CohortIDs) == 1 {
			revision = cloneString(cohortByID[source.CohortIDs[0]].SourceRevision)
		}
		dimensions := artifactDimensions(source, "CONFIGURED_SOURCE", revision, age, source.Validity)
		keyBytes, _ := canonicalJSON(dimensions)
		bucket := buckets[string(keyBytes)]
		if bucket == nil {
			bucket = &configuredBucket{dimensions: dimensions, hasBytes: true}
			buckets[string(keyBytes)] = bucket
		}
		bucket.cohortIDs = append(bucket.cohortIDs, source.CohortIDs...)
		bucket.sources = append(bucket.sources, source)
		if source.ByteCount == nil {
			bucket.hasBytes = false
		} else {
			value, _ := parseDecimal(*source.ByteCount)
			bucket.bytes += value
		}
		if source.AdapterID == "local-trace-v1" {
			memberRows, memberErr := memberMetrics(source, cohortByID, issueByID)
			if memberErr != nil {
				return nil, memberErr
			}
			memberCandidates = append(memberCandidates, memberRows...)
		}
	}
	groupedMembers, err := groupRetainedMemberMetrics(memberCandidates)
	if err != nil {
		return nil, err
	}
	metrics = append(metrics, groupedMembers...)
	for _, bucket := range buckets {
		bucket.cohortIDs = uniqueSorted(bucket.cohortIDs)
		count := decimalUint(uint64(len(bucket.sources)))
		metrics = append(metrics, configuredBucketMetric(*bucket, "data.artifact.count", "COUNT", &count, uint64(len(sources)), scan))
		var byteValue *string
		if bucket.hasBytes {
			value := decimalUint(bucket.bytes)
			byteValue = &value
		}
		metrics = append(metrics, configuredBucketMetric(*bucket, "data.artifact.bytes", "BYTES", byteValue, totalConfiguredBytes, scan))
	}
	if len(sources) == 0 {
		metrics = append(metrics, defaultUnavailableMetric("data.artifact.bytes", metricDefinitions["data.artifact.bytes"]))
		metrics = append(metrics, defaultUnavailableMetric("data.artifact.count", metricDefinitions["data.artifact.count"]))
	}
	sort.Slice(metrics, func(i, j int) bool { return metricLess(metrics[i], metrics[j]) })
	return metrics, nil
}

func configuredBucketMetric(bucket struct {
	dimensions []Dimension
	cohortIDs  []string
	sources    []Source
	bytes      uint64
	hasBytes   bool
}, name, unit string, value *string, denominator uint64, scan ScanState) Metric {
	var denominatorText *string
	completeness := CompletenessComplete
	if scan == ScanComplete {
		denominatorText = ptrDecimal(decimalUint(denominator))
	} else if scan == ScanPartial {
		completeness = CompletenessPartial
	} else {
		completeness = CompletenessUnknown
	}
	sourceIDs, exclusions := []string{}, []string{}
	currencies := map[Currency]struct{}{}
	for _, source := range bucket.sources {
		sourceIDs = append(sourceIDs, source.ID)
		exclusions = append(exclusions, source.Exclusions...)
		currencies[source.Currency] = struct{}{}
	}
	currency := CurrencyMixed
	if len(currencies) == 1 {
		for candidate := range currencies {
			currency = candidate
		}
	}
	first := bucket.sources[0]
	epistemic := EpistemicObserved
	if value == nil {
		epistemic = EpistemicNotObserved
	}
	return Metric{AuthorityClass: first.AuthorityClass, CohortIDs: bucket.cohortIDs, Completeness: completeness,
		Currency: currency, Denominator: denominatorText, DeliveryStage: first.DeliveryStage,
		Dimensions: bucket.dimensions, EpistemicClass: epistemic, Exclusions: uniqueSorted(exclusions),
		Name: name, Numerator: cloneString(value), ScopeClass: ScopeMultiCohortInventory,
		SourceIDs: uniqueSorted(sourceIDs), Unit: unit, Validity: first.Validity, Value: cloneString(value)}
}

func groupRetainedMemberMetrics(candidates []Metric) ([]Metric, error) {
	type metricKey struct {
		Name       string      `json:"name"`
		Dimensions []Dimension `json:"dimensions"`
		CohortIDs  []string    `json:"cohortIds"`
	}
	grouped := make(map[string]Metric, len(candidates))
	for _, candidate := range candidates {
		encoded, err := canonicalJSON(metricKey{candidate.Name, candidate.Dimensions, candidate.CohortIDs})
		if err != nil {
			return nil, internalError()
		}
		key := string(encoded)
		current, exists := grouped[key]
		if !exists {
			grouped[key] = candidate
			continue
		}
		merged, err := mergeRetainedMemberMetric(current, candidate)
		if err != nil {
			return nil, err
		}
		grouped[key] = merged
	}
	metrics := make([]Metric, 0, len(grouped))
	for _, metric := range grouped {
		metrics = append(metrics, metric)
	}
	sort.Slice(metrics, func(i, j int) bool { return metricLess(metrics[i], metrics[j]) })
	return metrics, nil
}

func mergeRetainedMemberMetric(left, right Metric) (Metric, error) {
	if left.Name != right.Name || left.Unit != right.Unit || left.ScopeClass != right.ScopeClass ||
		left.AuthorityClass != right.AuthorityClass || left.DeliveryStage != right.DeliveryStage ||
		left.Validity != right.Validity || left.Window != nil || right.Window != nil {
		return Metric{}, internalError()
	}
	value, err := addMetricDecimals(left.Value, right.Value)
	if err != nil {
		return Metric{}, err
	}
	denominator, err := addMetricDecimals(left.Denominator, right.Denominator)
	if err != nil {
		return Metric{}, err
	}
	left.Value = value
	left.Numerator = cloneString(value)
	left.Denominator = denominator
	left.SourceIDs = uniqueSorted(append(left.SourceIDs, right.SourceIDs...))
	left.Exclusions = uniqueSorted(append(left.Exclusions, right.Exclusions...))
	if left.Currency != right.Currency {
		left.Currency = CurrencyMixed
	}
	if value == nil {
		left.EpistemicClass = EpistemicNotObserved
	} else if left.EpistemicClass != EpistemicObserved || right.EpistemicClass != EpistemicObserved {
		return Metric{}, internalError()
	}
	left.Completeness = combinedCompleteness(left.Completeness, right.Completeness)
	if denominator != nil {
		if left.Completeness != CompletenessComplete {
			return Metric{}, internalError()
		}
	} else if left.Completeness == CompletenessComplete {
		return Metric{}, internalError()
	}
	return left, nil
}

func addMetricDecimals(left, right *string) (*string, error) {
	if left == nil || right == nil {
		return nil, nil
	}
	leftValue, leftOK := parseDecimal(*left)
	rightValue, rightOK := parseDecimal(*right)
	if !leftOK || !rightOK {
		return nil, internalError()
	}
	if leftValue > ^uint64(0)-rightValue {
		return nil, resourceExhausted()
	}
	value := decimalUint(leftValue + rightValue)
	return &value, nil
}

func combinedCompleteness(left, right Completeness) Completeness {
	if left == CompletenessUnknown || right == CompletenessUnknown {
		return CompletenessUnknown
	}
	if left == CompletenessPartial || right == CompletenessPartial {
		return CompletenessPartial
	}
	return CompletenessComplete
}

func memberMetrics(source Source, cohorts map[string]Cohort, issues map[string]Issue) ([]Metric, error) {
	if source.Members == nil {
		storeIssues := []string{}
		for _, id := range source.Exclusions {
			if issue, ok := issues[id]; ok && issue.Code == "STORE_CHANGED" {
				storeIssues = append(storeIssues, id)
			}
		}
		if len(storeIssues) == 0 {
			return nil, nil
		}
		return []Metric{
			retainedMemberMetric(source, "data.artifact.count", "COUNT", nil, "", nil, 0, false, ValidityInvalid, storeIssues),
			retainedMemberMetric(source, "data.artifact.bytes", "BYTES", nil, "", nil, 0, false, ValidityInvalid, storeIssues),
		}, nil
	}
	members := *source.Members
	var totalBytes uint64
	for _, member := range members {
		value, valid := parseDecimal(member.ByteCount)
		if !valid || value > MaxAggregateInputBytes || totalBytes > MaxAggregateInputBytes-value {
			return nil, invalidArgument()
		}
		totalBytes += value
	}
	closed := source.Validity == ValidityValid && source.Completeness == CompletenessComplete
	result := make([]Metric, 0, 2*len(members)+4)
	for _, member := range members {
		var cohortID string
		for _, id := range source.CohortIDs {
			if pointerValue(cohorts[id].SourceRevision) == member.Revision {
				cohortID = id
				break
			}
		}
		result = append(result, retainedMemberMetric(source, "data.artifact.count", "COUNT", &member.Revision, cohortID, ptrDecimal("1"), uint64(len(members)), closed, ValidityValid, nil))
		result = append(result, retainedMemberMetric(source, "data.artifact.bytes", "BYTES", &member.Revision, cohortID, &member.ByteCount, totalBytes, closed, ValidityValid, nil))
	}
	if len(members) == 0 && source.Validity == ValidityValid {
		zero := "0"
		result = append(result, retainedMemberMetric(source, "data.artifact.count", "COUNT", nil, "", &zero, 0, closed, ValidityValid, nil))
		result = append(result, retainedMemberMetric(source, "data.artifact.bytes", "BYTES", nil, "", &zero, 0, closed, ValidityValid, nil))
	}
	var rejected uint64
	terminal := make([]string, 0)
	for _, id := range source.Exclusions {
		issue, ok := issues[id]
		if !ok || !isExactMemberTerminalIssue(issue, source.ID) {
			return nil, invalidArgument()
		}
		value, valid := parseDecimal(*issue.Observed)
		if !valid || value == 0 || value > maxTraceCandidates || rejected > maxTraceCandidates-value {
			return nil, invalidArgument()
		}
		rejected += value
		terminal = append(terminal, id)
	}
	if rejected != 0 {
		value := decimalUint(rejected)
		result = append(result, retainedMemberMetric(source, "data.artifact.count", "COUNT", nil, "", &value, uint64(len(members)), false, ValidityInvalid, terminal))
		result = append(result, retainedMemberMetric(source, "data.artifact.bytes", "BYTES", nil, "", nil, uint64(len(members)), false, ValidityInvalid, terminal))
	}
	return result, nil
}

func retainedMemberMetric(source Source, name, unit string, revision *string, cohortID string, value *string, denominator uint64, closed bool, validity Validity, exclusions []string) Metric {
	cohortIDs := []string{}
	if cohortID != "" {
		cohortIDs = []string{cohortID}
	}
	var denominatorText *string
	completeness := source.Completeness
	if completeness == CompletenessComplete {
		completeness = CompletenessPartial
	}
	if closed {
		denominatorText = ptrDecimal(decimalUint(denominator))
		completeness = CompletenessComplete
	}
	epistemic := EpistemicObserved
	if value == nil {
		epistemic = EpistemicNotObserved
	}
	return Metric{AuthorityClass: source.AuthorityClass, CohortIDs: cohortIDs, Completeness: completeness,
		Currency: source.Currency, Denominator: denominatorText, DeliveryStage: source.DeliveryStage,
		Dimensions: artifactDimensions(source, "RETAINED_MEMBER", revision, "UNKNOWN", validity), EpistemicClass: epistemic,
		Exclusions: uniqueSorted(exclusions), Name: name, Numerator: cloneString(value), ScopeClass: ScopeMultiCohortInventory,
		SourceIDs: []string{source.ID}, Unit: unit, Validity: validity, Value: cloneString(value)}
}

func artifactDimensions(source Source, kind string, revision *string, age string, metricValidity Validity) []Dimension {
	adapterID, authority, delivery, validity := source.AdapterID, string(source.AuthorityClass), string(source.DeliveryStage), string(metricValidity)
	return []Dimension{{Name: "adapterId", Value: &adapterID}, {Name: "ageBucket", Value: &age}, {Name: "artifactKind", Value: &kind},
		{Name: "authorityClass", Value: &authority}, {Name: "deliveryStage", Value: &delivery}, {Name: "revision", Value: cloneString(revision)}, {Name: "validity", Value: &validity}}
}

func sourceAgeBucket(source Source, generatedAt string) (string, error) {
	if source.ObservationTime == nil {
		return "UNKNOWN", nil
	}
	observed, err := time.Parse("2006-01-02T15:04:05.000000000Z", *source.ObservationTime)
	if err != nil {
		return "", invalidArgument()
	}
	generated, err := time.Parse("2006-01-02T15:04:05.000000000Z", generatedAt)
	if err != nil {
		return "", invalidArgument()
	}
	if observed.After(generated) {
		if source.Validity != ValidityInvalid {
			return "", invalidArgument()
		}
		return "UNKNOWN", nil
	}
	age := generated.Sub(observed)
	switch {
	case age < time.Hour:
		return "LT_1H", nil
	case age < 24*time.Hour:
		return "H1_TO_24H", nil
	case age < 7*24*time.Hour:
		return "D1_TO_7D", nil
	default:
		return "GE_7D", nil
	}
}

func uniqueSorted(values []string) []string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	if len(values) == 0 {
		return []string{}
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func ptrDecimal(value string) *string { return &value }
