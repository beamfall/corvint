package model

import (
	"bytes"
	"sort"
)

func Compile(input Input) (*Snapshot, []byte, error) {
	if err := validateInputHeader(input); err != nil {
		return nil, nil, err
	}
	registry, registryHash, err := normalizeRegistry(input.Registry)
	if err != nil {
		return nil, nil, err
	}
	if registryHash != ExpectedAdapterRegistrySHA256 {
		return nil, nil, invalidArgument()
	}
	sources, cohorts, configuredHash, err := compileSources(input.Sources, registry, input.Repository)
	if err != nil {
		return nil, nil, err
	}
	issues := make([]Issue, 0, len(input.Issues))
	for _, candidate := range input.Issues {
		issue, issueErr := NewIssue(candidate)
		if issueErr != nil {
			return nil, nil, issueErr
		}
		issues = append(issues, issue)
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].ID < issues[j].ID })
	for index := 1; index < len(issues); index++ {
		if issues[index-1].ID == issues[index].ID {
			return nil, nil, invalidArgument()
		}
	}
	if err := validateTraceIssueRelationships(sources, issues); err != nil {
		return nil, nil, err
	}

	metrics, err := compileMetrics(input.Metrics)
	if err != nil {
		return nil, nil, err
	}
	metrics["data"], err = deriveDataMetrics(sources, cohorts, issues, input.GeneratedAt, input.Observation.ScanState)
	if err != nil {
		return nil, nil, err
	}
	metricCount := 0
	for _, group := range metrics {
		metricCount += len(group)
	}
	if metricCount > MaxMetricSamples {
		return nil, nil, resourceExhausted()
	}
	snapshot := &Snapshot{
		Schema: SnapshotSchema, GeneratedAt: input.GeneratedAt,
		Observation: Observation{
			AdapterRegistrySHA256: registryHash, ClockSource: input.Observation.ClockSource,
			ConfiguredSourceSetSHA256: configuredHash, End: input.Observation.End,
			LimitsProfile: LimitsProfile, ScanState: input.Observation.ScanState,
			Start: input.Observation.Start,
		},
		Repository: cloneRepository(input.Repository), Cohorts: cohorts, Sources: sources,
		Data: metrics["data"], Usage: metrics["usage"], Verification: metrics["verification"],
		Frontier: metrics["frontier"], Harnesses: []Metric{}, Beamfall: metrics["beamfall"],
		Privacy: Privacy{
			Collection: "DISABLED", OutboundNetwork: "NONE", PathDisclosure: "NONE",
			RawBodies: "EXCLUDED", ThreatBoundary: "LOCAL_ACCOUNT_NOT_DEFENDED",
		},
		Issues: issues,
	}
	if err := validateSnapshot(snapshot); err != nil {
		return nil, nil, err
	}
	if err := validateReferences(snapshot); err != nil {
		return nil, nil, err
	}
	if err := validateTraceRelationships(snapshot.Usage, snapshot.Sources, snapshot.Cohorts); err != nil {
		return nil, nil, err
	}
	preimage, err := canonicalJSON(snapshot)
	if err != nil {
		return nil, nil, internalError()
	}
	digest, err := domainHash("sha256:", snapshotDomain, jsonRaw(preimage))
	if err != nil {
		return nil, nil, internalError()
	}
	snapshot.SnapshotSHA256 = &digest
	encoded, err := canonicalJSON(snapshot)
	if err != nil {
		return nil, nil, internalError()
	}
	encoded = append(encoded, '\n')
	if len(encoded) > MaxSnapshotBytes {
		return nil, nil, resourceExhausted()
	}
	if _, verifyErr := VerifyCanonical(encoded); verifyErr != nil {
		return nil, nil, internalError()
	}
	return snapshot, encoded, nil
}

// jsonRaw prevents the already canonical snapshot preimage from being quoted.
// domainHash handles it specially.
type jsonRaw []byte

func normalizeRegistry(input []AdapterRegistration) ([]AdapterRegistration, string, error) {
	if len(input) == 0 {
		return nil, "", invalidArgument()
	}
	registry := append([]AdapterRegistration(nil), input...)
	for index := range registry {
		row := &registry[index]
		if row.AdapterID == "" || row.SourceKind == "" || row.VerifierID == "" || !validateDecimal(row.MaxBytes) ||
			!oneOf(row.DeliveryStage, DeliveryAccepted, DeliveryValidated, DeliveryImplemented, DeliveryExperimental, DeliveryNotStarted, DeliveryFailed, DeliveryUnsupported) {
			return nil, "", invalidArgument()
		}
		var err error
		if row.AcceptedProfiles, err = normalizeSet(row.AcceptedProfiles); err != nil {
			return nil, "", err
		}
		if row.IssueCodes, err = normalizeSet(row.IssueCodes); err != nil {
			return nil, "", err
		}
		for _, code := range row.IssueCodes {
			if _, ok := issueCodes[code]; !ok {
				return nil, "", invalidArgument()
			}
		}
		for _, value := range []string{row.AdapterID, row.SourceKind, row.VerifierID} {
			if validateWireString(value) != nil {
				return nil, "", invalidArgument()
			}
		}
		if row.DefaultLocation != nil && validateWireString(*row.DefaultLocation) != nil {
			return nil, "", invalidArgument()
		}
	}
	sort.Slice(registry, func(i, j int) bool { return registry[i].AdapterID < registry[j].AdapterID })
	for index := 1; index < len(registry); index++ {
		if registry[index-1].AdapterID == registry[index].AdapterID {
			return nil, "", invalidArgument()
		}
	}
	hash, err := domainHash("sha256:", registryDomain, registry)
	return registry, hash, err
}

func compileSources(inputs []SourceInput, registry []AdapterRegistration, repository Repository) ([]Source, []Cohort, string, error) {
	if len(inputs) > MaxConfiguredArtifacts {
		return nil, nil, "", resourceExhausted()
	}
	registrations := make(map[string]AdapterRegistration, len(registry))
	for _, row := range registry {
		registrations[row.AdapterID] = row
	}
	sources := make([]Source, 0, len(inputs))
	cohortByID := make(map[string]Cohort)
	configured := make([]configuredSourceIdentity, 0, len(inputs))
	configuredKeys := make(map[string]struct{}, len(inputs))
	var aggregate uint64
	for _, input := range inputs {
		registration, ok := registrations[input.AdapterID]
		if !ok || !validateDecimal(input.ConfiguredOrdinal) || input.Profile == "" {
			return nil, nil, "", invalidArgument()
		}
		if input.VerifierID != registration.VerifierID || input.DeliveryStage != registration.DeliveryStage ||
			!profileAllowed(input.Profile, registration) {
			return nil, nil, "", invalidArgument()
		}
		if err := validateSourceSemantics(input, registration); err != nil {
			return nil, nil, "", err
		}
		if input.ContentSHA256 != nil && !validateSHA256(*input.ContentSHA256) {
			return nil, nil, "", invalidArgument()
		}
		configuredKey := input.AdapterID + "\x00" + input.ConfiguredOrdinal
		if _, exists := configuredKeys[configuredKey]; exists {
			return nil, nil, "", invalidArgument()
		}
		configuredKeys[configuredKey] = struct{}{}
		if input.ByteCount != nil {
			count, ok := parseDecimal(*input.ByteCount)
			limit, limitOK := parseDecimal(registration.MaxBytes)
			if !ok || !limitOK || count > limit || aggregate > MaxAggregateInputBytes-count {
				return nil, nil, "", resourceExhausted()
			}
			aggregate += count
		}
		var readsHash *string
		if witnessErr := validateRepositoryWitnessSemantics(input, repository); witnessErr != nil {
			return nil, nil, "", witnessErr
		}
		if input.RepositoryWitnesses != nil {
			hash, hashErr := ComputeRepositoryReadsSHA256(input.RepositoryWitnesses)
			if hashErr != nil {
				return nil, nil, "", hashErr
			}
			readsHash = &hash
		}
		cohortIDs := make([]string, 0, len(input.Cohorts))
		for _, cohortInput := range input.Cohorts {
			if cohortInput.AdapterID != input.AdapterID || cohortInput.Profile != input.Profile {
				return nil, nil, "", invalidArgument()
			}
			if cohortErr := validateCohortIdentity(cohortInput, input.VerifierID); cohortErr != nil {
				return nil, nil, "", cohortErr
			}
			id, cohortErr := ComputeCohortID(cohortInput)
			if cohortErr != nil {
				return nil, nil, "", cohortErr
			}
			cohortIDs = append(cohortIDs, id)
			cohort := cohortFromIdentity(id, cohortInput)
			if previous, exists := cohortByID[id]; exists {
				left, _ := canonicalJSON(previous)
				right, _ := canonicalJSON(cohort)
				if !bytes.Equal(left, right) {
					return nil, nil, "", internalError()
				}
			} else {
				cohortByID[id] = cohort
			}
		}
		cohortIDs, cohortErr := normalizeSet(cohortIDs)
		if cohortErr != nil {
			return nil, nil, "", cohortErr
		}
		members, memberErr := validateAndCloneMembers(input, cohortIDs)
		if memberErr != nil {
			return nil, nil, "", memberErr
		}
		id, idErr := ComputeSourceID(SourceIdentity{input.AdapterID, input.ConfiguredOrdinal, input.ContentSHA256, input.Profile, readsHash})
		if idErr != nil {
			return nil, nil, "", idErr
		}
		exclusions, exclusionErr := normalizeSet(input.Exclusions)
		if exclusionErr != nil {
			return nil, nil, "", exclusionErr
		}
		sources = append(sources, Source{
			AdapterID: input.AdapterID, AuthorityClass: input.AuthorityClass, ByteCount: cloneString(input.ByteCount),
			CohortIDs: cohortIDs, Completeness: input.Completeness, ConfiguredOrdinal: input.ConfiguredOrdinal,
			ContentSHA256: cloneString(input.ContentSHA256), Currency: input.Currency,
			DeliveryStage: input.DeliveryStage, DisplayLabel: input.AdapterID + "#" + input.ConfiguredOrdinal,
			EpistemicClass: input.EpistemicClass, Exclusions: exclusions, ID: id, Members: members,
			ObservationEnd: cloneString(input.ObservationEnd), ObservationStart: cloneString(input.ObservationStart),
			ObservationTime: cloneString(input.ObservationTime), Profile: input.Profile,
			RepositoryReadsSHA256: readsHash, Validity: input.Validity, VerifierID: input.VerifierID,
		})
		configured = append(configured, configuredSourceIdentity{input.AdapterID, input.ConfiguredOrdinal, input.ContentSHA256})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })
	for index := 1; index < len(sources); index++ {
		if sources[index-1].ID == sources[index].ID {
			return nil, nil, "", invalidArgument()
		}
	}
	cohorts := make([]Cohort, 0, len(cohortByID))
	for _, cohort := range cohortByID {
		cohorts = append(cohorts, cohort)
	}
	sort.Slice(cohorts, func(i, j int) bool { return cohorts[i].CohortID < cohorts[j].CohortID })
	sort.Slice(configured, func(i, j int) bool {
		left, _ := canonicalJSON(configured[i])
		right, _ := canonicalJSON(configured[j])
		return bytes.Compare(left, right) < 0
	})
	configuredHash, err := domainHash("sha256:", configuredSourcesDomain, configured)
	return sources, cohorts, configuredHash, err
}

func cohortFromIdentity(id string, input CohortIdentity) Cohort {
	return Cohort{input.AdapterID, id, cloneString(input.DirtyPathsSHA256), cloneString(input.ProducerIdentity),
		input.Profile, cloneString(input.RepositoryObjectFormat), cloneString(input.SourceObservationEnd),
		cloneString(input.SourceObservationStart), cloneString(input.SourceRevision), cloneString(input.SourceTreeRevision)}
}

func cloneRepository(input Repository) Repository {
	return Repository{
		DirtyPathCount: cloneString(input.DirtyPathCount), DirtyPathsSHA256: cloneString(input.DirtyPathsSHA256),
		HeadRevision: cloneString(input.HeadRevision), ObjectFormat: cloneString(input.ObjectFormat),
		TreeRevision: cloneString(input.TreeRevision), WorktreeState: input.WorktreeState,
	}
}
