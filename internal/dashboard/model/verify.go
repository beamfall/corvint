package model

import (
	"bytes"
	json "encoding/json/v2"
	"sort"
)

func CanonicalSnapshot(snapshot *Snapshot) ([]byte, error) {
	if snapshot == nil || snapshot.SnapshotSHA256 == nil {
		return nil, invalidArgument()
	}
	encoded, err := canonicalJSON(snapshot)
	if err != nil {
		return nil, invalidArgument()
	}
	encoded = append(encoded, '\n')
	verified, err := VerifyCanonical(encoded)
	if err != nil {
		return nil, err
	}
	_ = verified
	return encoded, nil
}

func VerifyCanonical(raw []byte) (*Snapshot, error) {
	if len(raw) == 0 || len(raw) > MaxSnapshotBytes || raw[len(raw)-1] != '\n' || bytes.Contains(raw[:len(raw)-1], []byte{'\n'}) {
		return nil, invalidArgument()
	}
	var snapshot Snapshot
	if err := json.Unmarshal(raw[:len(raw)-1], &snapshot, json.RejectUnknownMembers(true), json.MatchCaseInsensitiveNames(false)); err != nil {
		return nil, invalidArgument()
	}
	if err := validateDecodedSnapshot(&snapshot); err != nil {
		return nil, err
	}
	canonical, err := canonicalJSON(&snapshot)
	if err != nil || !bytes.Equal(canonical, raw[:len(raw)-1]) {
		return nil, invalidArgument()
	}
	return &snapshot, nil
}

func validateDecodedSnapshot(snapshot *Snapshot) error {
	if snapshot.SnapshotSHA256 == nil || !validateSHA256(*snapshot.SnapshotSHA256) ||
		snapshot.Observation.AdapterRegistrySHA256 != ExpectedAdapterRegistrySHA256 ||
		!validateSHA256(snapshot.Observation.ConfiguredSourceSetSHA256) ||
		!validateTimestamp(snapshot.GeneratedAt) || !validateTimestamp(snapshot.Observation.Start) ||
		!validateTimestamp(snapshot.Observation.End) || snapshot.Observation.Start > snapshot.Observation.End ||
		!oneOf(snapshot.Observation.ClockSource, ClockProcess, ClockCaller) ||
		!oneOf(snapshot.Observation.ScanState, ScanComplete, ScanPartial, ScanInvalid) {
		return invalidArgument()
	}
	if err := validateRepository(snapshot.Repository); err != nil {
		return err
	}
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	if len(snapshot.Sources) > MaxConfiguredArtifacts {
		return invalidArgument()
	}

	cohortIDs := make(map[string]struct{}, len(snapshot.Cohorts))
	for index, cohort := range snapshot.Cohorts {
		identity := CohortIdentity{cohort.AdapterID, cohort.Profile, cohort.RepositoryObjectFormat,
			cohort.SourceRevision, cohort.SourceTreeRevision, cohort.DirtyPathsSHA256, cohort.ProducerIdentity,
			cohort.SourceObservationStart, cohort.SourceObservationEnd}
		expected, err := ComputeCohortID(identity)
		if err != nil || validateCohortIdentity(identity, pointerValue(identity.ProducerIdentity)) != nil || expected != cohort.CohortID || (index > 0 && snapshot.Cohorts[index-1].CohortID >= cohort.CohortID) {
			return invalidArgument()
		}
		cohortIDs[cohort.CohortID] = struct{}{}
	}
	configured := make([]configuredSourceIdentity, 0, len(snapshot.Sources))
	sourceIDs := make(map[string]struct{}, len(snapshot.Sources))
	configuredKeys := make(map[string]struct{}, len(snapshot.Sources))
	var aggregateInputBytes uint64
	for index, source := range snapshot.Sources {
		if err := validateDecodedSourcePolicy(source); err != nil {
			return err
		}
		configuredKey := source.AdapterID + "\x00" + source.ConfiguredOrdinal
		if _, duplicate := configuredKeys[configuredKey]; duplicate {
			return invalidArgument()
		}
		configuredKeys[configuredKey] = struct{}{}
		if source.ByteCount != nil {
			count, valid := parseDecimal(*source.ByteCount)
			if !valid || count > MaxAggregateInputBytes || aggregateInputBytes > MaxAggregateInputBytes-count {
				return invalidArgument()
			}
			aggregateInputBytes += count
		}
		expected, err := ComputeSourceID(SourceIdentity{source.AdapterID, source.ConfiguredOrdinal, source.ContentSHA256, source.Profile, source.RepositoryReadsSHA256})
		if err != nil || expected != source.ID || (index > 0 && snapshot.Sources[index-1].ID >= source.ID) {
			return invalidArgument()
		}
		for _, cohortID := range source.CohortIDs {
			if _, ok := cohortIDs[cohortID]; !ok {
				return invalidArgument()
			}
		}
		sourceIDs[source.ID] = struct{}{}
		configured = append(configured, configuredSourceIdentity{source.AdapterID, source.ConfiguredOrdinal, source.ContentSHA256})
		if err := validateDecodedTraceAggregate(source, snapshot.Cohorts); err != nil {
			return err
		}
	}
	sort.Slice(configured, func(i, j int) bool {
		left, _ := canonicalJSON(configured[i])
		right, _ := canonicalJSON(configured[j])
		return bytes.Compare(left, right) < 0
	})
	configuredHash, _ := domainHash("sha256:", configuredSourcesDomain, configured)
	if configuredHash != snapshot.Observation.ConfiguredSourceSetSHA256 {
		return invalidArgument()
	}

	issueIDs := make(map[string]struct{}, len(snapshot.Issues))
	for index, issue := range snapshot.Issues {
		expected, err := NewIssue(IssueInput{issue.Code, issue.Severity, issue.SourceID, issue.Observed, issue.Limit})
		if err != nil || expected.ID != issue.ID || (index > 0 && snapshot.Issues[index-1].ID >= issue.ID) {
			return invalidArgument()
		}
		if issue.SourceID != nil {
			if _, ok := sourceIDs[*issue.SourceID]; !ok {
				return invalidArgument()
			}
		}
		issueIDs[issue.ID] = struct{}{}
	}
	for _, source := range snapshot.Sources {
		for _, id := range source.Exclusions {
			if _, ok := issueIDs[id]; !ok {
				return invalidArgument()
			}
		}
	}
	if err := validateTraceIssueRelationships(snapshot.Sources, snapshot.Issues); err != nil {
		return err
	}
	present := make(map[string]bool)
	groups := []struct {
		name    string
		metrics []Metric
	}{
		{"data", snapshot.Data}, {"usage", snapshot.Usage}, {"verification", snapshot.Verification},
		{"frontier", snapshot.Frontier}, {"beamfall", snapshot.Beamfall},
	}
	metricCount := 0
	for _, groupEntry := range groups {
		group := groupEntry.metrics
		metricCount += len(group)
		if metricCount > MaxMetricSamples {
			return invalidArgument()
		}
		for index, metric := range group {
			definition, ok := metricDefinitions[metric.Name]
			if !ok || definition.group != groupEntry.name || validateMetric(metric, definition) != nil || (index > 0 && !metricLess(group[index-1], metric)) {
				return invalidArgument()
			}
			for _, id := range metric.SourceIDs {
				if _, ok := sourceIDs[id]; !ok {
					return invalidArgument()
				}
			}
			for _, id := range metric.CohortIDs {
				if _, ok := cohortIDs[id]; !ok {
					return invalidArgument()
				}
			}
			for _, id := range metric.Exclusions {
				if _, ok := issueIDs[id]; !ok {
					return invalidArgument()
				}
			}
			present[metric.Name] = true
		}
	}
	if len(snapshot.Harnesses) != 0 {
		return invalidArgument()
	}
	for name := range metricDefinitions {
		if !present[name] {
			return invalidArgument()
		}
	}
	expectedData, err := deriveDataMetrics(snapshot.Sources, snapshot.Cohorts, snapshot.Issues, snapshot.GeneratedAt, snapshot.Observation.ScanState)
	if err != nil {
		return err
	}
	left, _ := canonicalJSON(snapshot.Data)
	right, _ := canonicalJSON(expectedData)
	if !bytes.Equal(left, right) {
		return invalidArgument()
	}
	if err := validateTraceRelationships(snapshot.Usage, snapshot.Sources, snapshot.Cohorts); err != nil {
		return err
	}
	if err := validateReferences(snapshot); err != nil {
		return err
	}

	digest := *snapshot.SnapshotSHA256
	snapshot.SnapshotSHA256 = nil
	preimage, err := canonicalJSON(snapshot)
	snapshot.SnapshotSHA256 = &digest
	if err != nil {
		return invalidArgument()
	}
	expectedDigest, _ := domainHash("sha256:", snapshotDomain, jsonRaw(preimage))
	if digest != expectedDigest {
		return invalidArgument()
	}
	return nil
}
