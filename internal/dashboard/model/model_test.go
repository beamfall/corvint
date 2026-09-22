package model

import (
	"bytes"
	"errors"
	"slices"
	"sort"
	"strconv"
	"testing"
)

const testTime = "2026-08-23T20:00:00.000000000Z"

func ptr(value string) *string { return &value }

func testRegistry() []AdapterRegistration {
	return []AdapterRegistration{
		{AdapterID: "beamfall-shadow-v0", DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "BEAMFALL_SHADOW", VerifierID: "unsupported"},
		{AdapterID: "cem-ocm-bundle-v0", AcceptedProfiles: []string{"cem/0.1+ocm/0.1", "cem/0.2+ocm/0.1"}, DeliveryStage: DeliveryNotStarted, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "CEM_OCM_BUNDLE", VerifierID: "go-cem-ocm-bundle-v0"},
		{AdapterID: "frontier-usage-v0", DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "FRONTIER_RECEIPT", VerifierID: "unsupported"},
		{AdapterID: "go-live-usage-v0", DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "GO_LIVE_RECEIPT", VerifierID: "unsupported"},
		{AdapterID: "harness-usage-v0", DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "HARNESS_RECEIPT", VerifierID: "unsupported"},
		{AdapterID: "head-spec-index-v0", DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "HEAD_SPEC_INDEX", VerifierID: "unsupported"},
		{AdapterID: "impact-envelope-v1", DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "IMPACT_ENVELOPE", VerifierID: "unsupported"},
		{AdapterID: "local-trace-v1", AcceptedProfiles: []string{"corvint-local-trace/1"}, DefaultLocation: ptr(".context-corvint/traces"), DeliveryStage: DeliveryNotStarted, IssueCodes: []string{"OBSERVATION_TIME_UNKNOWN", "REPOSITORY_OBJECT_UNAVAILABLE", "SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_INVALID_IDENTITY", "SOURCE_INVALID_SCHEMA", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK", "STORE_CHANGED", "TRACE_ANCESTRY_BOUND", "TRACE_STORE_BOUND", "UNSUPPORTED_OBJECT_ALTERNATES", "VERIFIER_REJECTED"}, MaxBytes: "16777216", SourceKind: "LOCAL_TRACE_STORE", VerifierID: "go-local-trace-v1"},
		{AdapterID: "pulse-dogfood-v0", DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "PULSE_RECEIPT", VerifierID: "unsupported"},
		{AdapterID: "query-envelope-v1", DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "QUERY_ENVELOPE", VerifierID: "unsupported"},
		{AdapterID: "stable-read-v0", AcceptedProfiles: []string{"dashboard-stable-read/0"}, DeliveryStage: DeliveryNotStarted, IssueCodes: []string{"SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK"}, MaxBytes: "16777216", SourceKind: "INTERNAL", VerifierID: "go-stable-read-v0"},
	}
}

func baseInput() Input {
	return Input{GeneratedAt: testTime, Observation: ObservationInput{ClockSource: ClockCaller, Start: testTime, End: testTime, ScanState: ScanComplete}, Repository: Repository{WorktreeState: WorktreeUnknown}, Registry: testRegistry()}
}

func TestRegistryGolden(t *testing.T) {
	registry := testRegistry()
	slices.Reverse(registry)
	_, digest, err := normalizeRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}
	if digest != ExpectedAdapterRegistrySHA256 {
		t.Fatalf("registry digest %s", digest)
	}
}

func TestCompileMissingIsNotZero(t *testing.T) {
	sourceIdentity := SourceIdentity{"local-trace-v1", "0", nil, "corvint-local-trace/1", nil}
	sourceID, _ := ComputeSourceID(sourceIdentity)
	issue, _ := NewIssue(IssueInput{Code: "SOURCE_NOT_PRESENT", Severity: SeverityWarning, SourceID: &sourceID})
	input := baseInput()
	input.Sources = []SourceInput{{AdapterID: "local-trace-v1", ConfiguredOrdinal: "0", Profile: "corvint-local-trace/1", AuthorityClass: AuthorityNone, Completeness: CompletenessUnknown, Currency: CurrencyUnknown, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicNotObserved, Exclusions: []string{issue.ID}, Validity: ValidityNotPresent, VerifierID: "go-local-trace-v1"}}
	input.Issues = []IssueInput{{Code: "SOURCE_NOT_PRESENT", Severity: SeverityWarning, SourceID: &sourceID}}
	snapshot, raw, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCanonical(raw); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Data) != 2 {
		t.Fatalf("data metrics=%d", len(snapshot.Data))
	}
	for _, metric := range snapshot.Data {
		switch metric.Name {
		case "data.artifact.count":
			if pointerValue(metric.Value) != "1" || pointerValue(metric.Denominator) != "1" {
				t.Fatalf("count=%v/%v", metric.Value, metric.Denominator)
			}
		case "data.artifact.bytes":
			if metric.Value != nil || metric.Numerator != nil {
				t.Fatal("missing bytes became measured zero")
			}
		}
	}
	if bytes.Contains(raw, []byte("context-corvint")) {
		t.Fatal("path leaked")
	}
}

func TestCompileDeterministicOrdering(t *testing.T) {
	input := baseInput()
	input.Sources = []SourceInput{
		unsupportedSource("query-envelope-v1", "1"), unsupportedSource("beamfall-shadow-v0", "0"),
	}
	_, first, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(input.Registry)
	slices.Reverse(input.Sources)
	_, second, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("input ordering changed canonical snapshot")
	}
}

func TestConfiguredInventoryGroupsExactMetricKey(t *testing.T) {
	input := baseInput()
	input.Sources = []SourceInput{unsupportedSource("beamfall-shadow-v0", "1"), unsupportedSource("beamfall-shadow-v0", "0")}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	countRows := 0
	for _, metric := range snapshot.Data {
		if metric.Name == "data.artifact.count" && dimensionValue(metric.Dimensions, "artifactKind") == "CONFIGURED_SOURCE" {
			countRows++
			if pointerValue(metric.Value) != "2" || pointerValue(metric.Denominator) != "2" || len(metric.SourceIDs) != 2 {
				t.Fatalf("ungrouped metric=%+v", metric)
			}
		}
	}
	if countRows != 1 {
		t.Fatalf("configured count rows=%d", countRows)
	}
}

func TestConfiguredInventoryGroupsAcrossCohorts(t *testing.T) {
	cohorts := []Cohort{{CohortID: "dashboard-cohort:sha256:" + string(bytes.Repeat([]byte{'1'}, 64))}, {CohortID: "dashboard-cohort:sha256:" + string(bytes.Repeat([]byte{'2'}, 64))}}
	sources := []Source{
		{AdapterID: "local-trace-v1", AuthorityClass: AuthorityAdapterQualified, CohortIDs: []string{cohorts[0].CohortID}, Completeness: CompletenessComplete, ConfiguredOrdinal: "0", Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, ID: "dashboard-source:sha256:" + string(bytes.Repeat([]byte{'a'}, 64)), Validity: ValidityValid},
		{AdapterID: "local-trace-v1", AuthorityClass: AuthorityAdapterQualified, CohortIDs: []string{cohorts[1].CohortID}, Completeness: CompletenessComplete, ConfiguredOrdinal: "1", Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, ID: "dashboard-source:sha256:" + string(bytes.Repeat([]byte{'b'}, 64)), Validity: ValidityValid},
	}
	metrics, err := deriveDataMetrics(sources, cohorts, nil, testTime, ScanComplete)
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, metric := range metrics {
		if metric.Name == "data.artifact.count" && dimensionValue(metric.Dimensions, "artifactKind") == "CONFIGURED_SOURCE" {
			rows++
			if pointerValue(metric.Value) != "2" || len(metric.CohortIDs) != 2 || len(metric.SourceIDs) != 2 {
				t.Fatalf("group=%+v", metric)
			}
		}
	}
	if rows != 1 {
		t.Fatalf("configured rows=%d", rows)
	}
}

func TestRetainedMemberInventoryGroupsExactMetricKeyAcrossSources(t *testing.T) {
	revision := "1111111111111111111111111111111111111111"
	cohortID := "dashboard-cohort:sha256:" + string(bytes.Repeat([]byte{'1'}, 64))
	cohorts := []Cohort{{CohortID: cohortID, SourceRevision: &revision}}
	firstMembers := []TraceMember{{Revision: revision, ContentSHA256: "sha256:" + string(bytes.Repeat([]byte{'a'}, 64)), ByteCount: "3"}}
	secondMembers := []TraceMember{{Revision: revision, ContentSHA256: "sha256:" + string(bytes.Repeat([]byte{'b'}, 64)), ByteCount: "4"}}
	sources := []Source{
		{AdapterID: "local-trace-v1", AuthorityClass: AuthorityAdapterQualified, CohortIDs: []string{cohortID}, Completeness: CompletenessComplete, ConfiguredOrdinal: "0", Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, ID: "dashboard-source:sha256:" + string(bytes.Repeat([]byte{'a'}, 64)), Members: &firstMembers, Validity: ValidityValid},
		{AdapterID: "local-trace-v1", AuthorityClass: AuthorityAdapterQualified, CohortIDs: []string{cohortID}, Completeness: CompletenessComplete, ConfiguredOrdinal: "1", Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, ID: "dashboard-source:sha256:" + string(bytes.Repeat([]byte{'b'}, 64)), Members: &secondMembers, Validity: ValidityValid},
	}
	metrics, err := deriveDataMetrics(sources, cohorts, nil, testTime, ScanComplete)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"data.artifact.count": "2", "data.artifact.bytes": "7"}
	seen := make(map[string]int)
	for _, metric := range metrics {
		if dimensionValue(metric.Dimensions, "artifactKind") != "RETAINED_MEMBER" {
			continue
		}
		seen[metric.Name]++
		if pointerValue(metric.Value) != want[metric.Name] || pointerValue(metric.Numerator) != want[metric.Name] || pointerValue(metric.Denominator) != want[metric.Name] || len(metric.SourceIDs) != 2 {
			t.Fatalf("retained member group=%+v", metric)
		}
	}
	if seen["data.artifact.count"] != 1 || seen["data.artifact.bytes"] != 1 {
		t.Fatalf("retained rows=%v", seen)
	}
}

func TestUnavailableConfiguredBytesAreNotObserved(t *testing.T) {
	input := baseInput()
	input.Sources = []SourceInput{unsupportedSource("beamfall-shadow-v0", "0")}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, metric := range snapshot.Data {
		if metric.Name == "data.artifact.bytes" && metric.Value == nil && metric.EpistemicClass != EpistemicNotObserved {
			t.Fatalf("unavailable bytes epistemic class=%s", metric.EpistemicClass)
		}
	}
}

func TestCompileRejectsCrossSourceIssueExclusion(t *testing.T) {
	first := unsupportedSource("beamfall-shadow-v0", "0")
	second := unsupportedSource("beamfall-shadow-v0", "1")
	firstID, _ := ComputeSourceID(SourceIdentity{first.AdapterID, first.ConfiguredOrdinal, nil, first.Profile, nil})
	issue, _ := NewIssue(IssueInput{Code: "SOURCE_UNSUPPORTED", Severity: SeverityInfo, SourceID: &firstID})
	second.Exclusions = []string{issue.ID}
	input := baseInput()
	input.Sources = []SourceInput{first, second}
	input.Issues = []IssueInput{{Code: "SOURCE_UNSUPPORTED", Severity: SeverityInfo, SourceID: &firstID}}
	if _, _, err := Compile(input); err == nil {
		t.Fatal("source accepted an exclusion owned by another source")
	}
}

func TestCompileClonesRepositoryInput(t *testing.T) {
	dirtyCount := "0"
	input := baseInput()
	input.Repository.DirtyPathCount = &dirtyCount
	snapshot, raw, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	dirtyCount = "1"
	after, err := CanonicalSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, after) {
		t.Fatal("caller mutation changed compiled snapshot")
	}
}

func TestRepositoryWitnessHashIsSemanticSet(t *testing.T) {
	revision := "1111111111111111111111111111111111111111"
	head := RepositoryWitness{Kind: "SNAPSHOT_HEAD", ObjectFormat: "sha1", ObjectID: revision, ObjectType: "commit"}
	trace := RepositoryWitness{Kind: "TRACE_REVISION", ObjectFormat: "sha1", ObjectID: revision, ObjectType: "commit", Revision: &revision}
	first, err := ComputeRepositoryReadsSHA256([]RepositoryWitness{trace, head, trace})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ComputeRepositoryReadsSHA256([]RepositoryWitness{head, trace})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("duplicate/traversal order changed hash: %s != %s", first, second)
	}
	bad := head
	bad.ObjectType = "blob"
	if _, err := ComputeRepositoryReadsSHA256([]RepositoryWitness{bad}); err == nil {
		t.Fatal("invalid semantic witness accepted")
	}
}

func TestRepositoryWitnessSemanticsRequireMemberCommitWitness(t *testing.T) {
	revision := "1111111111111111111111111111111111111111"
	format := "sha1"
	members := []TraceMember{{Revision: revision, ContentSHA256: "sha256:" + string(bytes.Repeat([]byte{'a'}, 64)), ByteCount: "1"}}
	source := SourceInput{
		AdapterID: "local-trace-v1", Validity: ValidityValid, Members: &members,
		RepositoryWitnesses: []RepositoryWitness{
			{Kind: "SNAPSHOT_HEAD", ObjectFormat: format, ObjectID: revision, ObjectType: "commit"},
			{Kind: "TRACE_PATH_OBJECT", ObjectFormat: format, ObjectID: "2222222222222222222222222222222222222222", ObjectType: "blob", Revision: &revision},
		},
	}
	repository := Repository{HeadRevision: &revision, ObjectFormat: &format, WorktreeState: WorktreeUnknown}
	if err := validateRepositoryWitnessSemantics(source, repository); err == nil {
		t.Fatal("member accepted without its TRACE_REVISION witness")
	}
}

func TestRepositoryWitnessSemanticsRequireCohortObjectFormat(t *testing.T) {
	revision := "1111111111111111111111111111111111111111"
	format, wrongFormat := "sha1", "sha256"
	members := []TraceMember{{Revision: revision, ContentSHA256: "sha256:" + string(bytes.Repeat([]byte{'a'}, 64)), ByteCount: "1"}}
	source := SourceInput{
		AdapterID: "local-trace-v1", Validity: ValidityValid, Members: &members,
		Cohorts: []CohortIdentity{{RepositoryObjectFormat: &wrongFormat}},
		RepositoryWitnesses: []RepositoryWitness{
			{Kind: "SNAPSHOT_HEAD", ObjectFormat: format, ObjectID: revision, ObjectType: "commit"},
			{Kind: "TRACE_REVISION", ObjectFormat: format, ObjectID: revision, ObjectType: "commit", Revision: &revision},
		},
	}
	repository := Repository{HeadRevision: &revision, ObjectFormat: &format, WorktreeState: WorktreeUnknown}
	if err := validateRepositoryWitnessSemantics(source, repository); err == nil {
		t.Fatal("cohort object format diverged from repository authority")
	}
}

func TestNotStartedAdapterCanReportUnsupportedSource(t *testing.T) {
	input := baseInput()
	input.Sources = []SourceInput{{
		AdapterID: "cem-ocm-bundle-v0", ConfiguredOrdinal: "0", Profile: "cem/0.2+ocm/0.1",
		AuthorityClass: AuthorityNone, Completeness: CompletenessUnknown, Currency: CurrencyUnknown,
		DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicNotObserved,
		Validity: ValidityUnsupported, VerifierID: "go-cem-ocm-bundle-v0",
	}}
	if _, _, err := Compile(input); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSnapshotRejectsMalformedSourceTruthAxes(t *testing.T) {
	digest := "sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	tests := []Source{
		{AdapterID: "local-trace-v1", ConfiguredOrdinal: "0", DisplayLabel: "local-trace-v1#0", Profile: "corvint-local-trace/1", VerifierID: "go-local-trace-v1", AuthorityClass: AuthorityAdapterQualified, Completeness: CompletenessComplete, Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicObserved, Validity: ValidityValid},
		{AdapterID: "query-envelope-v1", ByteCount: ptr("x"), ConfiguredOrdinal: "0", ContentSHA256: &digest, DisplayLabel: "query-envelope-v1#0", Profile: "unsupported", VerifierID: "unsupported", AuthorityClass: AuthorityNone, Completeness: CompletenessUnknown, Currency: CurrencyUnknown, DeliveryStage: DeliveryUnsupported, EpistemicClass: EpistemicNotObserved, Validity: ValidityUnsupported},
		{AdapterID: "query-envelope-v1", ConfiguredOrdinal: "0", DisplayLabel: "query-envelope-v1#0", Profile: "unsupported", RepositoryReadsSHA256: &digest, VerifierID: "unsupported", AuthorityClass: AuthorityNone, Completeness: CompletenessUnknown, Currency: CurrencyUnknown, DeliveryStage: DeliveryUnsupported, EpistemicClass: EpistemicNotObserved, Validity: ValidityUnsupported},
	}
	for index, source := range tests {
		snapshot := &Snapshot{Schema: SnapshotSchema, Observation: Observation{LimitsProfile: LimitsProfile, ScanState: ScanPartial}, Sources: []Source{source}}
		if err := validateSnapshot(snapshot); err == nil {
			t.Fatalf("malformed source %d accepted", index)
		}
	}
}

func TestCohortIdentityRejectsHalfObservationInterval(t *testing.T) {
	producer := "go-local-trace-v1"
	cohort := CohortIdentity{AdapterID: "local-trace-v1", Profile: "corvint-local-trace/1", ProducerIdentity: &producer, SourceObservationStart: ptr(testTime)}
	if err := validateCohortIdentity(cohort, producer); err == nil {
		t.Fatal("half observation interval accepted")
	}
}

func unsupportedSource(adapter, ordinal string) SourceInput {
	return SourceInput{AdapterID: adapter, ConfiguredOrdinal: ordinal, Profile: "unsupported", AuthorityClass: AuthorityNone, Completeness: CompletenessUnknown, Currency: CurrencyUnknown, DeliveryStage: DeliveryUnsupported, EpistemicClass: EpistemicNotObserved, Validity: ValidityUnsupported, VerifierID: "unsupported"}
}

func TestCompileTraceAggregateAndDenominator(t *testing.T) {
	revision := "1111111111111111111111111111111111111111"
	tree := "2222222222222222222222222222222222222222"
	dirty := "sha256:" + string(bytes.Repeat([]byte{'d'}, 64))
	memberDigest := "sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	members := []TraceMember{{Revision: revision, ContentSHA256: memberDigest, ByteCount: "3"}}
	aggregateDigest, _ := domainHash("sha256:", traceStoreDomain, members)
	producer, format := "go-local-trace-v1", "sha1"
	cohort := CohortIdentity{AdapterID: "local-trace-v1", Profile: "corvint-local-trace/1", RepositoryObjectFormat: &format, SourceRevision: &revision, SourceTreeRevision: &tree, DirtyPathsSHA256: &dirty, ProducerIdentity: &producer, SourceObservationStart: ptr(testTime), SourceObservationEnd: ptr(testTime)}
	cohortID, _ := ComputeCohortID(cohort)
	witnesses := []RepositoryWitness{{Kind: "SNAPSHOT_HEAD", ObjectFormat: format, ObjectID: revision, ObjectType: "commit"}, {Kind: "TRACE_REVISION", ObjectFormat: format, ObjectID: revision, ObjectType: "commit", Revision: &revision}}
	reads, _ := ComputeRepositoryReadsSHA256(witnesses)
	sourceID, _ := ComputeSourceID(SourceIdentity{"local-trace-v1", "0", &aggregateDigest, "corvint-local-trace/1", &reads})
	one := "1"
	input := baseInput()
	input.Repository = Repository{DirtyPathsSHA256: &dirty, HeadRevision: &revision, ObjectFormat: &format, TreeRevision: &tree, WorktreeState: WorktreeUnknown}
	input.Sources = []SourceInput{{AdapterID: "local-trace-v1", ConfiguredOrdinal: "0", Profile: "corvint-local-trace/1", AuthorityClass: AuthorityAdapterQualified, ByteCount: ptr("3"), Completeness: CompletenessComplete, ContentSHA256: &aggregateDigest, Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicObserved, RepositoryWitnesses: witnesses, Validity: ValidityValid, VerifierID: producer, Cohorts: []CohortIdentity{cohort}, Members: &members, ObservationStart: ptr(testTime), ObservationEnd: ptr(testTime)}}
	input.Issues = []IssueInput{{Code: "OBSERVATION_TIME_UNKNOWN", Severity: SeverityInfo, SourceID: &sourceID}}
	input.Metrics = []MetricInput{{AuthorityClass: AuthorityAdvisory, CohortIDs: []string{cohortID}, Completeness: CompletenessComplete, Currency: CurrencyValidatedAt, Denominator: &one, DeliveryStage: DeliveryNotStarted, Dimensions: []Dimension{{Name: "outcome", Value: ptr("passed")}, {Name: "revision", Value: &revision}}, EpistemicClass: EpistemicObserved, Name: "usage.trace.retained", Numerator: &one, ScopeClass: ScopeSingleCohort, SourceIDs: []string{sourceID}, Unit: "COUNT", Validity: ValidityValid, Value: &one}}
	snapshot, raw, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCanonical(raw); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Sources[0].CohortIDs) != 1 || snapshot.Sources[0].Members == nil {
		t.Fatal("aggregate proof omitted")
	}
}

func TestCompileRejectsTraceDenominatorLie(t *testing.T) {
	// The full positive fixture above covers admitted trace arithmetic; mutate its
	// canonical output to prove independent verification does not trust a count.
	input := baseInput()
	input.Metrics = []MetricInput{{Name: "usage.trace.retained"}}
	if _, _, err := Compile(input); err == nil {
		t.Fatal("invalid trace metric accepted")
	}
}

func TestPartialRejectedMembersHaveNoFalseDenominator(t *testing.T) {
	members := []TraceMember{}
	sourceID, _ := ComputeSourceID(SourceIdentity{"local-trace-v1", "0", nil, "corvint-local-trace/1", nil})
	observed := "2"
	issue, _ := NewIssue(IssueInput{Code: "SOURCE_INVALID_SCHEMA", Severity: SeverityError, SourceID: &sourceID, Observed: &observed})
	input := baseInput()
	input.Observation.ScanState = ScanPartial
	input.Sources = []SourceInput{{AdapterID: "local-trace-v1", ConfiguredOrdinal: "0", Profile: "corvint-local-trace/1", AuthorityClass: AuthorityAdapterQualified, Completeness: CompletenessPartial, Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicObserved, Exclusions: []string{issue.ID}, Validity: ValidityInvalid, VerifierID: "go-local-trace-v1", Members: &members}}
	input.Issues = []IssueInput{
		{Code: "SOURCE_INVALID_SCHEMA", Severity: SeverityError, SourceID: &sourceID, Observed: &observed},
		{Code: "OBSERVATION_TIME_UNKNOWN", Severity: SeverityInfo, SourceID: &sourceID},
	}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, metric := range snapshot.Data {
		if metric.Name == "data.artifact.count" && dimensionValue(metric.Dimensions, "artifactKind") == "RETAINED_MEMBER" {
			found = true
			if pointerValue(metric.Value) != "2" || metric.Denominator != nil || metric.Validity != ValidityInvalid {
				t.Fatalf("rejected metric=%+v", metric)
			}
		}
	}
	if !found {
		t.Fatal("rejected member count omitted")
	}
}

func TestStoreChangedEmitsUnobservedMemberUniverse(t *testing.T) {
	sourceID, _ := ComputeSourceID(SourceIdentity{"local-trace-v1", "0", nil, "corvint-local-trace/1", nil})
	issue, _ := NewIssue(IssueInput{Code: "STORE_CHANGED", Severity: SeverityError, SourceID: &sourceID})
	input := baseInput()
	input.Observation.ScanState = ScanInvalid
	input.Sources = []SourceInput{{AdapterID: "local-trace-v1", ConfiguredOrdinal: "0", Profile: "corvint-local-trace/1", AuthorityClass: AuthorityAdapterQualified, Completeness: CompletenessUnknown, Currency: CurrencyUnknown, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicObserved, Exclusions: []string{issue.ID}, Validity: ValidityInvalid, VerifierID: "go-local-trace-v1"}}
	input.Issues = []IssueInput{{Code: "STORE_CHANGED", Severity: SeverityError, SourceID: &sourceID}}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, metric := range snapshot.Data {
		if dimensionValue(metric.Dimensions, "artifactKind") == "RETAINED_MEMBER" {
			rows++
			if metric.Value != nil || metric.Numerator != nil || metric.Denominator != nil || metric.EpistemicClass != EpistemicNotObserved || metric.Validity != ValidityInvalid || metric.Completeness != CompletenessUnknown {
				t.Fatalf("store-changed row=%+v", metric)
			}
		}
	}
	if rows != 2 {
		t.Fatalf("member rows=%d", rows)
	}
}

func TestConfiguredBucketWithoutCompleteBytesHasNullBytes(t *testing.T) {
	digest := "sha256:" + string(bytes.Repeat([]byte{'0'}, 64))
	withBytes := validStableReadSource("0", "5", digest)
	withBytes.Validity, withBytes.Completeness = ValidityInvalid, CompletenessPartial
	without := validStableReadSource("1", "5", digest)
	without.Validity, without.Completeness, without.EpistemicClass = ValidityInvalid, CompletenessPartial, EpistemicNotObserved
	without.ByteCount, without.ContentSHA256 = nil, nil
	input := baseInput()
	input.Observation.ScanState = ScanPartial
	input.Sources = []SourceInput{withBytes, without}
	snapshot, raw, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCanonical(raw); err != nil {
		t.Fatal(err)
	}
	for _, metric := range snapshot.Data {
		if metric.Name == "data.artifact.bytes" && len(metric.SourceIDs) == 2 {
			if metric.Value != nil || metric.Numerator != nil || metric.EpistemicClass != EpistemicNotObserved {
				t.Fatalf("mixed bucket bytes=%+v", metric)
			}
			return
		}
	}
	t.Fatal("mixed configured bucket omitted")
}

func TestInvalidScanRejectsObservedTraceUsage(t *testing.T) {
	input, _, _ := oneTraceInput(t)
	if _, _, err := Compile(input); err != nil {
		t.Fatal(err)
	}
	input.Observation.ScanState = ScanInvalid
	if _, _, err := Compile(input); err == nil {
		t.Fatal("INVALID scan accepted an observed usage.trace.retained row")
	}
}

func TestSourceIssueCodeMustBelongToAdapterRegistry(t *testing.T) {
	source := unsupportedSource("beamfall-shadow-v0", "0")
	sourceID, _ := ComputeSourceID(SourceIdentity{source.AdapterID, source.ConfiguredOrdinal, nil, source.Profile, nil})
	issueInput := IssueInput{Code: "LIMIT_SAMPLES", Severity: SeverityError, SourceID: &sourceID}
	issue, _ := NewIssue(issueInput)
	source.Exclusions = []string{issue.ID}
	input := baseInput()
	input.Sources = []SourceInput{source}
	input.Issues = []IssueInput{issueInput}
	if _, _, err := Compile(input); err == nil {
		t.Fatal("compiled a source issue code outside its adapter registry row")
	}
	supported := IssueInput{Code: "SOURCE_UNSUPPORTED", Severity: SeverityInfo, SourceID: &sourceID}
	supportedIssue, _ := NewIssue(supported)
	input.Sources[0].Exclusions = []string{supportedIssue.ID}
	input.Issues = []IssueInput{supported}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Issues = []Issue{issue}
	snapshot.Sources[0].Exclusions = []string{issue.ID}
	if snapshot.Data, err = deriveDataMetrics(snapshot.Sources, snapshot.Cohorts, snapshot.Issues, snapshot.GeneratedAt, snapshot.Observation.ScanState); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCanonical(rehashSnapshot(t, snapshot)); err == nil {
		t.Fatal("verified a source issue code outside its adapter registry row")
	}
}

func TestValidateMetricRejectsNoncanonicalReferenceSets(t *testing.T) {
	first := "dashboard-source:sha256:" + string(bytes.Repeat([]byte{'1'}, 64))
	second := "dashboard-source:sha256:" + string(bytes.Repeat([]byte{'2'}, 64))
	cohortFirst := "dashboard-cohort:sha256:" + string(bytes.Repeat([]byte{'1'}, 64))
	cohortSecond := "dashboard-cohort:sha256:" + string(bytes.Repeat([]byte{'2'}, 64))
	issueFirst := "dashboard-issue:sha256:" + string(bytes.Repeat([]byte{'1'}, 64))
	issueSecond := "dashboard-issue:sha256:" + string(bytes.Repeat([]byte{'2'}, 64))
	metric := Metric{
		AuthorityClass: AuthorityAdapterQualified, CohortIDs: []string{cohortFirst, cohortSecond},
		Completeness: CompletenessComplete, Currency: CurrencyValidatedAt, Denominator: ptr("1"),
		DeliveryStage: DeliveryNotStarted, Dimensions: artifactDimensions(Source{AdapterID: "local-trace-v1", AuthorityClass: AuthorityAdapterQualified, DeliveryStage: DeliveryNotStarted}, "CONFIGURED_SOURCE", nil, "UNKNOWN", ValidityValid),
		EpistemicClass: EpistemicObserved, Exclusions: []string{issueFirst, issueSecond}, Name: "data.artifact.count",
		Numerator: ptr("1"), ScopeClass: ScopeMultiCohortInventory, SourceIDs: []string{first, second},
		Unit: "COUNT", Validity: ValidityValid, Value: ptr("1"),
	}
	if err := validateMetric(metric, metricDefinitions[metric.Name]); err != nil {
		t.Fatal(err)
	}
	tests := []func(*Metric){
		func(value *Metric) { value.SourceIDs = []string{second, first} },
		func(value *Metric) { value.SourceIDs = []string{first, first} },
		func(value *Metric) { value.CohortIDs = []string{cohortSecond, cohortFirst} },
		func(value *Metric) { value.CohortIDs = []string{cohortFirst, cohortFirst} },
		func(value *Metric) { value.Exclusions = []string{issueSecond, issueFirst} },
		func(value *Metric) { value.Exclusions = []string{issueFirst, issueFirst} },
	}
	for index, mutate := range tests {
		candidate := metric
		mutate(&candidate)
		if err := validateMetric(candidate, metricDefinitions[candidate.Name]); err == nil {
			t.Fatalf("noncanonical reference set %d accepted", index)
		}
	}
}

func TestVerifyCanonicalRejectsSelfRehashedRegistryForgery(t *testing.T) {
	input := baseInput()
	input.Sources = []SourceInput{unsupportedSource("beamfall-shadow-v0", "0")}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Sources[0].VerifierID = "forged-verifier"
	forged := rehashSnapshot(t, snapshot)
	if _, err := VerifyCanonical(forged); err == nil {
		t.Fatal("self-rehashed source registry forgery accepted")
	}
}

func TestVerifyCanonicalRejectsSelfRehashedMetricSetForgery(t *testing.T) {
	input := baseInput()
	input.Sources = []SourceInput{unsupportedSource("beamfall-shadow-v0", "0"), unsupportedSource("beamfall-shadow-v0", "1")}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	for index := range snapshot.Data {
		if len(snapshot.Data[index].SourceIDs) == 2 {
			slices.Reverse(snapshot.Data[index].SourceIDs)
			forged := rehashSnapshot(t, snapshot)
			if _, err := VerifyCanonical(forged); err == nil {
				t.Fatal("self-rehashed unsorted metric source IDs accepted")
			}
			return
		}
	}
	t.Fatal("two-source inventory metric not found")
}

func TestVerifyCanonicalRejectsSelfRehashedSourceByteLimitForgery(t *testing.T) {
	digest := "sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	input := baseInput()
	input.Sources = []SourceInput{validStableReadSource("0", "1", digest)}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	tooLarge := "16777217"
	snapshot.Sources[0].ByteCount = &tooLarge
	snapshot.Data, err = deriveDataMetrics(snapshot.Sources, snapshot.Cohorts, snapshot.Issues, snapshot.GeneratedAt, snapshot.Observation.ScanState)
	if err != nil {
		t.Fatal(err)
	}
	forged := rehashSnapshot(t, snapshot)
	if _, err := VerifyCanonical(forged); err == nil {
		t.Fatal("self-rehashed per-source byte-limit forgery accepted")
	}
}

func TestVerifyCanonicalRejectsSelfRehashedZeroMaxBytesObservation(t *testing.T) {
	input := baseInput()
	input.Observation.ScanState = ScanPartial
	input.Sources = []SourceInput{{AdapterID: "cem-ocm-bundle-v0", ConfiguredOrdinal: "0", Profile: "cem/0.1+ocm/0.1", AuthorityClass: AuthorityNone, Completeness: CompletenessUnknown, Currency: CurrencyUnknown, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicNotObserved, Validity: ValidityInvalid, VerifierID: "go-cem-ocm-bundle-v0"}}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	zero, digest := "0", "sha256:"+string(bytes.Repeat([]byte{'a'}, 64))
	source := &snapshot.Sources[0]
	source.ByteCount, source.ContentSHA256 = &zero, &digest
	source.ID, _ = ComputeSourceID(SourceIdentity{source.AdapterID, source.ConfiguredOrdinal, source.ContentSHA256, source.Profile, nil})
	configured := []configuredSourceIdentity{{source.AdapterID, source.ConfiguredOrdinal, source.ContentSHA256}}
	snapshot.Observation.ConfiguredSourceSetSHA256, _ = domainHash("sha256:", configuredSourcesDomain, configured)
	snapshot.Data, err = deriveDataMetrics(snapshot.Sources, snapshot.Cohorts, snapshot.Issues, snapshot.GeneratedAt, snapshot.Observation.ScanState)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCanonical(rehashSnapshot(t, snapshot)); err == nil {
		t.Fatal("self-rehashed byte observation from a maxBytes 0 adapter accepted")
	}
}

func TestVerifyCanonicalRejectsSelfRehashedConfiguredOrdinalCollision(t *testing.T) {
	firstDigest := "sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	secondDigest := "sha256:" + string(bytes.Repeat([]byte{'b'}, 64))
	input := baseInput()
	input.Sources = []SourceInput{
		validStableReadSource("0", "1", firstDigest),
		validStableReadSource("1", "1", secondDigest),
	}
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	changed := 0
	if snapshot.Sources[changed].ConfiguredOrdinal != "1" {
		changed = 1
	}
	oldID := snapshot.Sources[changed].ID
	snapshot.Sources[changed].ConfiguredOrdinal = "0"
	snapshot.Sources[changed].DisplayLabel = "stable-read-v0#0"
	newID, _ := ComputeSourceID(SourceIdentity{snapshot.Sources[changed].AdapterID, "0", snapshot.Sources[changed].ContentSHA256, snapshot.Sources[changed].Profile, nil})
	snapshot.Sources[changed].ID = newID
	for index := range snapshot.Data {
		for sourceIndex, id := range snapshot.Data[index].SourceIDs {
			if id == oldID {
				snapshot.Data[index].SourceIDs[sourceIndex] = newID
			}
		}
		sort.Strings(snapshot.Data[index].SourceIDs)
	}
	sort.Slice(snapshot.Sources, func(i, j int) bool { return snapshot.Sources[i].ID < snapshot.Sources[j].ID })
	configured := make([]configuredSourceIdentity, 0, len(snapshot.Sources))
	for _, source := range snapshot.Sources {
		configured = append(configured, configuredSourceIdentity{source.AdapterID, source.ConfiguredOrdinal, source.ContentSHA256})
	}
	sort.Slice(configured, func(i, j int) bool {
		left, _ := canonicalJSON(configured[i])
		right, _ := canonicalJSON(configured[j])
		return bytes.Compare(left, right) < 0
	})
	snapshot.Observation.ConfiguredSourceSetSHA256, _ = domainHash("sha256:", configuredSourcesDomain, configured)
	forged := rehashSnapshot(t, snapshot)
	if _, err := VerifyCanonical(forged); err == nil {
		t.Fatal("self-rehashed configured ordinal collision accepted")
	}
}

func TestDecodedBoundsRejectAggregateInputAndMetricSamples(t *testing.T) {
	tooManyMetrics := minimallyDecodedSnapshot(t)
	tooManyMetrics.Data = make([]Metric, MaxMetricSamples+1)
	if err := validateDecodedSnapshot(tooManyMetrics); err == nil {
		t.Fatal("metric sample bound not enforced")
	}

	aggregate := minimallyDecodedSnapshot(t)
	hexDigits := "0123456789abcdef"
	for ordinal := 0; ordinal < 17; ordinal++ {
		digest := "sha256:" + string(bytes.Repeat([]byte{hexDigits[ordinal%len(hexDigits)]}, 64))
		input := validStableReadSource(strconv.Itoa(ordinal), "16777216", digest)
		id, _ := ComputeSourceID(SourceIdentity{input.AdapterID, input.ConfiguredOrdinal, input.ContentSHA256, input.Profile, nil})
		aggregate.Sources = append(aggregate.Sources, Source{
			AdapterID: input.AdapterID, AuthorityClass: input.AuthorityClass, ByteCount: input.ByteCount,
			CohortIDs: []string{}, Completeness: input.Completeness, ConfiguredOrdinal: input.ConfiguredOrdinal,
			ContentSHA256: input.ContentSHA256, Currency: input.Currency, DeliveryStage: input.DeliveryStage,
			DisplayLabel: input.AdapterID + "#" + input.ConfiguredOrdinal, EpistemicClass: input.EpistemicClass,
			Exclusions: []string{}, ID: id, Profile: input.Profile, Validity: input.Validity, VerifierID: input.VerifierID,
		})
	}
	sort.Slice(aggregate.Sources, func(i, j int) bool { return aggregate.Sources[i].ID < aggregate.Sources[j].ID })
	if err := validateDecodedSnapshot(aggregate); err == nil {
		t.Fatal("aggregate input bound not enforced")
	}
}

func TestLocalTraceCohortRequiresAuthorityQualifiedTree(t *testing.T) {
	revision := "1111111111111111111111111111111111111111"
	format, producer := "sha1", "go-local-trace-v1"
	dirty := "sha256:" + string(bytes.Repeat([]byte{'d'}, 64))
	cohort := CohortIdentity{AdapterID: "local-trace-v1", Profile: "corvint-local-trace/1", RepositoryObjectFormat: &format, SourceRevision: &revision, DirtyPathsSHA256: &dirty, ProducerIdentity: &producer, SourceObservationStart: ptr(testTime), SourceObservationEnd: ptr(testTime)}
	if err := validateCohortIdentity(cohort, producer); err == nil {
		t.Fatal("local trace cohort without source tree accepted")
	}
}

func TestTerminalIssueContractRejectsWrongSeverityAndNonterminalCounts(t *testing.T) {
	sourceID := "dashboard-source:sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	observed := "1"
	if _, err := NewIssue(IssueInput{Code: "SOURCE_INVALID_SCHEMA", Severity: SeverityWarning, SourceID: &sourceID, Observed: &observed}); err == nil {
		t.Fatal("WARNING member-terminal issue accepted")
	}
	nonterminal, err := NewIssue(IssueInput{Code: "OBSERVATION_TIME_UNKNOWN", Severity: SeverityInfo, SourceID: &sourceID, Observed: &observed})
	if err != nil {
		t.Fatal(err)
	}
	members := []TraceMember{}
	source := Source{AdapterID: "local-trace-v1", ID: sourceID, Completeness: CompletenessPartial, Exclusions: []string{nonterminal.ID}, Members: &members, Validity: ValidityInvalid}
	if err := validateTraceIssueRelationships([]Source{source}, []Issue{nonterminal}); err == nil {
		t.Fatal("nonterminal numeric issue counted as rejected member")
	}
	terminal, err := NewIssue(IssueInput{Code: "SOURCE_INVALID_SCHEMA", Severity: SeverityError, SourceID: &sourceID, Observed: &observed})
	if err != nil {
		t.Fatal(err)
	}
	wrongSource := Source{AdapterID: "stable-read-v0", ID: sourceID}
	if err := validateTraceIssueRelationships([]Source{wrongSource}, []Issue{terminal}); err == nil {
		t.Fatal("member-terminal issue owned by a non-trace source")
	}
}

func TestTerminalIssueContractChecksAggregateRejectedCount(t *testing.T) {
	sourceID := "dashboard-source:sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	firstCount, secondCount := "700", "400"
	first, err := NewIssue(IssueInput{Code: "SOURCE_INVALID_SCHEMA", Severity: SeverityError, SourceID: &sourceID, Observed: &firstCount})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewIssue(IssueInput{Code: "VERIFIER_REJECTED", Severity: SeverityError, SourceID: &sourceID, Observed: &secondCount})
	if err != nil {
		t.Fatal(err)
	}
	observation, err := NewIssue(IssueInput{Code: "OBSERVATION_TIME_UNKNOWN", Severity: SeverityInfo, SourceID: &sourceID})
	if err != nil {
		t.Fatal(err)
	}
	exclusions := []string{first.ID, second.ID}
	sort.Strings(exclusions)
	members := []TraceMember{}
	source := Source{AdapterID: "local-trace-v1", ID: sourceID, Completeness: CompletenessPartial, Exclusions: exclusions, Members: &members, Validity: ValidityInvalid}
	if err := validateTraceIssueRelationships([]Source{source}, []Issue{first, second, observation}); err == nil {
		t.Fatal("rejected-member total above the candidate bound accepted")
	}
	issueMap := map[string]Issue{first.ID: first, second.ID: second, observation.ID: observation}
	if _, err := memberMetrics(source, nil, issueMap); err == nil {
		t.Fatal("member metric derivation overflowed the rejected-member universe")
	}
}

func TestRepositoryUnavailableIsMemberTerminalNotAggregateRoot(t *testing.T) {
	input, sourceID, _ := oneTraceInput(t)
	observed := "2"
	terminal, err := NewIssue(IssueInput{Code: "REPOSITORY_OBJECT_UNAVAILABLE", Severity: SeverityError, SourceID: &sourceID, Observed: &observed})
	if err != nil {
		t.Fatal(err)
	}
	input.Observation.ScanState = ScanPartial
	input.Sources[0].Completeness = CompletenessPartial
	input.Sources[0].Exclusions = []string{terminal.ID}
	input.Issues = append(input.Issues, IssueInput{Code: "REPOSITORY_OBJECT_UNAVAILABLE", Severity: SeverityError, SourceID: &sourceID, Observed: &observed})
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, metric := range snapshot.Data {
		if metric.Name == "data.artifact.count" && dimensionValue(metric.Dimensions, "artifactKind") == "RETAINED_MEMBER" && metric.Validity == ValidityInvalid {
			found = true
			if pointerValue(metric.Value) != "2" || metric.Denominator != nil || !slices.Equal(metric.Exclusions, []string{terminal.ID}) {
				t.Fatalf("partial rejected-member metric=%+v", metric)
			}
		}
	}
	if !found {
		t.Fatal("partial rejected-member count missing")
	}

	rootSourceID, err := ComputeSourceID(SourceIdentity{"local-trace-v1", "0", nil, "corvint-local-trace/1", nil})
	if err != nil {
		t.Fatal(err)
	}
	allRejectedCount := "3"
	allRejectedIssue, err := NewIssue(IssueInput{Code: "REPOSITORY_OBJECT_UNAVAILABLE", Severity: SeverityError, SourceID: &rootSourceID, Observed: &allRejectedCount})
	if err != nil {
		t.Fatal(err)
	}
	emptyMembers := []TraceMember{}
	allRejected := baseInput()
	allRejected.Observation.ScanState = ScanPartial
	allRejected.Sources = []SourceInput{{AdapterID: "local-trace-v1", ConfiguredOrdinal: "0", Profile: "corvint-local-trace/1", AuthorityClass: AuthorityAdapterQualified, Completeness: CompletenessPartial, Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicObserved, Exclusions: []string{allRejectedIssue.ID}, Validity: ValidityInvalid, VerifierID: "go-local-trace-v1", Members: &emptyMembers}}
	allRejected.Issues = []IssueInput{
		{Code: "REPOSITORY_OBJECT_UNAVAILABLE", Severity: SeverityError, SourceID: &rootSourceID, Observed: &allRejectedCount},
		{Code: "OBSERVATION_TIME_UNKNOWN", Severity: SeverityInfo, SourceID: &rootSourceID},
	}
	allRejectedSnapshot, _, err := Compile(allRejected)
	if err != nil {
		t.Fatal(err)
	}
	allRejectedFound := false
	for _, metric := range allRejectedSnapshot.Data {
		if metric.Name == "data.artifact.count" && dimensionValue(metric.Dimensions, "artifactKind") == "RETAINED_MEMBER" {
			allRejectedFound = true
			if pointerValue(metric.Value) != "3" || metric.Denominator != nil {
				t.Fatalf("all-rejected member count=%v denominator=%v", metric.Value, metric.Denominator)
			}
		}
	}
	if !allRejectedFound {
		t.Fatal("all-rejected member count missing")
	}

	rootInput := baseInput()
	rootInput.Observation.ScanState = ScanInvalid
	rootIssue, err := NewIssue(IssueInput{Code: "REPOSITORY_OBJECT_UNAVAILABLE", Severity: SeverityError, SourceID: &rootSourceID})
	if err != nil {
		t.Fatal(err)
	}
	rootInput.Sources = []SourceInput{{AdapterID: "local-trace-v1", ConfiguredOrdinal: "0", Profile: "corvint-local-trace/1", AuthorityClass: AuthorityAdapterQualified, Completeness: CompletenessUnknown, Currency: CurrencyUnknown, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicObserved, Exclusions: []string{rootIssue.ID}, Validity: ValidityInvalid, VerifierID: "go-local-trace-v1"}}
	rootInput.Issues = []IssueInput{{Code: "REPOSITORY_OBJECT_UNAVAILABLE", Severity: SeverityError, SourceID: &rootSourceID}}
	if _, _, err := Compile(rootInput); err == nil {
		t.Fatal("observed:null aggregate-root REPOSITORY_OBJECT_UNAVAILABLE accepted")
	}
}

func TestVerifyCanonicalRejectsRetainedTotalOverflow(t *testing.T) {
	input, _, _ := oneTraceInput(t)
	snapshot, _, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	var passed Metric
	usage := make([]Metric, 0, len(snapshot.Usage)+1)
	for _, metric := range snapshot.Usage {
		if metric.Name == "usage.trace.retained" {
			passed = metric
			continue
		}
		usage = append(usage, metric)
	}
	if passed.Name == "" {
		t.Fatal("trace retained metric missing")
	}
	failed := passed
	failed.Dimensions = cloneDimensions(passed.Dimensions)
	for index := range failed.Dimensions {
		if failed.Dimensions[index].Name == "outcome" {
			failed.Dimensions[index].Value = ptr("failed")
		}
	}
	maximum := decimalUint(^uint64(0))
	failed.Value, failed.Numerator, failed.Denominator = &maximum, &maximum, &maximum
	one := "1"
	passed.Value, passed.Numerator, passed.Denominator = &one, &one, &maximum
	snapshot.Usage = append(usage, failed, passed)
	sort.Slice(snapshot.Usage, func(i, j int) bool { return metricLess(snapshot.Usage[i], snapshot.Usage[j]) })
	forged := rehashSnapshot(t, snapshot)
	_, err = VerifyCanonical(forged)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "DASHBOARD_RESOURCE_EXHAUSTED" {
		t.Fatalf("self-rehashed retained total overflow error=%v", err)
	}
}

func TestVerifyCanonicalRejectsOrphanAndMismatchedCohorts(t *testing.T) {
	t.Run("orphan", func(t *testing.T) {
		input, _, _ := oneTraceInput(t)
		snapshot, _, err := Compile(input)
		if err != nil {
			t.Fatal(err)
		}
		producer := "go-stable-read-v0"
		identity := CohortIdentity{AdapterID: "stable-read-v0", Profile: "dashboard-stable-read/0", ProducerIdentity: &producer}
		id, err := ComputeCohortID(identity)
		if err != nil {
			t.Fatal(err)
		}
		snapshot.Cohorts = append(snapshot.Cohorts, cohortFromIdentity(id, identity))
		sort.Slice(snapshot.Cohorts, func(i, j int) bool { return snapshot.Cohorts[i].CohortID < snapshot.Cohorts[j].CohortID })
		if _, err := VerifyCanonical(rehashSnapshot(t, snapshot)); err == nil {
			t.Fatal("self-rehashed orphan cohort accepted")
		}
	})

	t.Run("producer", func(t *testing.T) {
		input, _, _ := oneTraceInput(t)
		snapshot, _, err := Compile(input)
		if err != nil {
			t.Fatal(err)
		}
		oldID := snapshot.Cohorts[0].CohortID
		forgedProducer := "forged-verifier"
		snapshot.Cohorts[0].ProducerIdentity = &forgedProducer
		refreshSingleCohortSnapshot(t, snapshot, oldID)
		if _, err := VerifyCanonical(rehashSnapshot(t, snapshot)); err == nil {
			t.Fatal("self-rehashed cohort/source verifier mismatch accepted")
		}
	})

	t.Run("repository", func(t *testing.T) {
		input, _, _ := oneTraceInput(t)
		snapshot, _, err := Compile(input)
		if err != nil {
			t.Fatal(err)
		}
		oldID := snapshot.Cohorts[0].CohortID
		mismatchedDirty := "sha256:" + string(bytes.Repeat([]byte{'e'}, 64))
		snapshot.Cohorts[0].DirtyPathsSHA256 = &mismatchedDirty
		refreshSingleCohortSnapshot(t, snapshot, oldID)
		if _, err := VerifyCanonical(rehashSnapshot(t, snapshot)); err == nil {
			t.Fatal("self-rehashed cohort/repository mismatch accepted")
		}
	})
}

func TestAdapterPoliciesMatchFrozenRegistry(t *testing.T) {
	registry := testRegistry()
	if len(adapterPolicies) != len(registry) {
		t.Fatalf("policy rows=%d registry rows=%d", len(adapterPolicies), len(registry))
	}
	for _, row := range registry {
		policy, ok := adapterPolicies[row.AdapterID]
		maxBytes, valid := parseDecimal(row.MaxBytes)
		if !ok || !valid || policy.stage != row.DeliveryStage || policy.verifier != row.VerifierID || policy.maxBytes != maxBytes || !slices.Equal(policy.profiles, row.AcceptedProfiles) || !slices.Equal(policy.issueCodes, row.IssueCodes) {
			t.Fatalf("policy drift for %s", row.AdapterID)
		}
	}
}

func oneTraceInput(t *testing.T) (Input, string, string) {
	t.Helper()
	revision := "1111111111111111111111111111111111111111"
	tree := "2222222222222222222222222222222222222222"
	dirty := "sha256:" + string(bytes.Repeat([]byte{'d'}, 64))
	memberDigest := "sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	members := []TraceMember{{Revision: revision, ContentSHA256: memberDigest, ByteCount: "3"}}
	aggregateDigest, err := domainHash("sha256:", traceStoreDomain, members)
	if err != nil {
		t.Fatal(err)
	}
	producer, format := "go-local-trace-v1", "sha1"
	cohort := CohortIdentity{AdapterID: "local-trace-v1", Profile: "corvint-local-trace/1", RepositoryObjectFormat: &format, SourceRevision: &revision, SourceTreeRevision: &tree, DirtyPathsSHA256: &dirty, ProducerIdentity: &producer, SourceObservationStart: ptr(testTime), SourceObservationEnd: ptr(testTime)}
	cohortID, err := ComputeCohortID(cohort)
	if err != nil {
		t.Fatal(err)
	}
	witnesses := []RepositoryWitness{{Kind: "SNAPSHOT_HEAD", ObjectFormat: format, ObjectID: revision, ObjectType: "commit"}, {Kind: "TRACE_REVISION", ObjectFormat: format, ObjectID: revision, ObjectType: "commit", Revision: &revision}}
	reads, err := ComputeRepositoryReadsSHA256(witnesses)
	if err != nil {
		t.Fatal(err)
	}
	sourceID, err := ComputeSourceID(SourceIdentity{"local-trace-v1", "0", &aggregateDigest, "corvint-local-trace/1", &reads})
	if err != nil {
		t.Fatal(err)
	}
	one := "1"
	input := baseInput()
	input.Repository = Repository{DirtyPathsSHA256: &dirty, HeadRevision: &revision, ObjectFormat: &format, TreeRevision: &tree, WorktreeState: WorktreeUnknown}
	input.Sources = []SourceInput{{AdapterID: "local-trace-v1", ConfiguredOrdinal: "0", Profile: "corvint-local-trace/1", AuthorityClass: AuthorityAdapterQualified, ByteCount: ptr("3"), Completeness: CompletenessComplete, ContentSHA256: &aggregateDigest, Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted, EpistemicClass: EpistemicObserved, RepositoryWitnesses: witnesses, Validity: ValidityValid, VerifierID: producer, Cohorts: []CohortIdentity{cohort}, Members: &members, ObservationStart: ptr(testTime), ObservationEnd: ptr(testTime)}}
	input.Metrics = []MetricInput{{AuthorityClass: AuthorityAdvisory, CohortIDs: []string{cohortID}, Completeness: CompletenessComplete, Currency: CurrencyValidatedAt, Denominator: &one, DeliveryStage: DeliveryNotStarted, Dimensions: []Dimension{{Name: "outcome", Value: ptr("passed")}, {Name: "revision", Value: &revision}}, EpistemicClass: EpistemicObserved, Name: "usage.trace.retained", Numerator: &one, ScopeClass: ScopeSingleCohort, SourceIDs: []string{sourceID}, Unit: "COUNT", Validity: ValidityValid, Value: &one}}
	input.Issues = []IssueInput{{Code: "OBSERVATION_TIME_UNKNOWN", Severity: SeverityInfo, SourceID: &sourceID}}
	return input, sourceID, cohortID
}

func refreshSingleCohortSnapshot(t *testing.T, snapshot *Snapshot, oldID string) {
	t.Helper()
	cohort := snapshot.Cohorts[0]
	identity := CohortIdentity{cohort.AdapterID, cohort.Profile, cohort.RepositoryObjectFormat,
		cohort.SourceRevision, cohort.SourceTreeRevision, cohort.DirtyPathsSHA256, cohort.ProducerIdentity,
		cohort.SourceObservationStart, cohort.SourceObservationEnd}
	newID, err := ComputeCohortID(identity)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Cohorts[0].CohortID = newID
	for sourceIndex := range snapshot.Sources {
		for cohortIndex, id := range snapshot.Sources[sourceIndex].CohortIDs {
			if id == oldID {
				snapshot.Sources[sourceIndex].CohortIDs[cohortIndex] = newID
			}
		}
		sort.Strings(snapshot.Sources[sourceIndex].CohortIDs)
	}
	for metricIndex := range snapshot.Usage {
		for cohortIndex, id := range snapshot.Usage[metricIndex].CohortIDs {
			if id == oldID {
				snapshot.Usage[metricIndex].CohortIDs[cohortIndex] = newID
			}
		}
		sort.Strings(snapshot.Usage[metricIndex].CohortIDs)
	}
	snapshot.Data, err = deriveDataMetrics(snapshot.Sources, snapshot.Cohorts, snapshot.Issues, snapshot.GeneratedAt, snapshot.Observation.ScanState)
	if err != nil {
		t.Fatal(err)
	}
}

func validStableReadSource(ordinal, byteCount, digest string) SourceInput {
	return SourceInput{
		AdapterID: "stable-read-v0", ConfiguredOrdinal: ordinal, Profile: "dashboard-stable-read/0",
		AuthorityClass: AuthorityAdapterQualified, ByteCount: &byteCount, Completeness: CompletenessComplete,
		ContentSHA256: &digest, Currency: CurrencyValidatedAt, DeliveryStage: DeliveryNotStarted,
		EpistemicClass: EpistemicObserved, Validity: ValidityValid, VerifierID: "go-stable-read-v0",
	}
}

func minimallyDecodedSnapshot(t *testing.T) *Snapshot {
	configuredHash, err := domainHash("sha256:", configuredSourcesDomain, []configuredSourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + string(bytes.Repeat([]byte{'0'}, 64))
	return &Snapshot{
		Schema: SnapshotSchema, GeneratedAt: testTime,
		Observation: Observation{AdapterRegistrySHA256: ExpectedAdapterRegistrySHA256, ClockSource: ClockCaller, ConfiguredSourceSetSHA256: configuredHash, End: testTime, LimitsProfile: LimitsProfile, ScanState: ScanComplete, Start: testTime},
		Repository:  Repository{WorktreeState: WorktreeUnknown}, Privacy: Privacy{Collection: "DISABLED", OutboundNetwork: "NONE", PathDisclosure: "NONE", RawBodies: "EXCLUDED", ThreatBoundary: "LOCAL_ACCOUNT_NOT_DEFENDED"}, SnapshotSHA256: &digest,
	}
}

func rehashSnapshot(t *testing.T, snapshot *Snapshot) []byte {
	t.Helper()
	snapshot.SnapshotSHA256 = nil
	preimage, err := canonicalJSON(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := domainHash("sha256:", snapshotDomain, jsonRaw(preimage))
	if err != nil {
		t.Fatal(err)
	}
	snapshot.SnapshotSHA256 = &digest
	encoded, err := canonicalJSON(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func dimensionValue(dimensions []Dimension, name string) string {
	for _, dimension := range dimensions {
		if dimension.Name == name {
			return pointerValue(dimension.Value)
		}
	}
	return ""
}

func TestCompileFourMiBBound(t *testing.T) {
	input := baseInput()
	input.Sources = make([]SourceInput, 10_000)
	for index := range input.Sources {
		input.Sources[index] = unsupportedSource("beamfall-shadow-v0", strconv.Itoa(index))
	}
	_, _, err := Compile(input)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "DASHBOARD_RESOURCE_EXHAUSTED" {
		t.Fatalf("error=%v", err)
	}
}

func FuzzVerifyCanonical(f *testing.F) {
	f.Add([]byte("{}\n"))
	f.Fuzz(func(t *testing.T, raw []byte) { _, _ = VerifyCanonical(raw) })
}
