package model

import (
	"slices"
	"sort"
)

func validateInputHeader(input Input) error {
	if !validateTimestamp(input.GeneratedAt) || !validateTimestamp(input.Observation.Start) || !validateTimestamp(input.Observation.End) ||
		input.Observation.Start > input.Observation.End ||
		!oneOf(input.Observation.ClockSource, ClockProcess, ClockCaller) ||
		!oneOf(input.Observation.ScanState, ScanComplete, ScanPartial, ScanInvalid) {
		return invalidArgument()
	}
	return validateRepository(input.Repository)
}

func validateRepository(repository Repository) error {
	if !oneOf(repository.WorktreeState, WorktreeClean, WorktreeMixed, WorktreeUnknown) {
		return invalidArgument()
	}
	if repository.DirtyPathCount != nil && !validateDecimal(*repository.DirtyPathCount) {
		return invalidArgument()
	}
	if repository.DirtyPathsSHA256 != nil && !validateSHA256(*repository.DirtyPathsSHA256) {
		return invalidArgument()
	}
	if repository.ObjectFormat != nil && *repository.ObjectFormat != "sha1" && *repository.ObjectFormat != "sha256" {
		return invalidArgument()
	}
	for _, objectID := range []*string{repository.HeadRevision, repository.TreeRevision} {
		if objectID != nil && !validateObjectID(*objectID, repository.ObjectFormat) {
			return invalidArgument()
		}
	}
	return nil
}

func validateObjectID(value string, format *string) bool {
	if format == nil {
		return false
	}
	length := 40
	if *format == "sha256" {
		length = 64
	}
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validAxes(validity Validity, epistemic EpistemicClass, authority AuthorityClass, completeness Completeness, currency Currency, delivery DeliveryStage) bool {
	return oneOf(validity, ValidityValid, ValidityInvalid, ValidityNotPresent, ValidityInaccessible, ValidityUnsupported, ValidityDisabled, ValidityExpired) &&
		oneOf(epistemic, EpistemicObserved, EpistemicDeclared, EpistemicAdvisory, EpistemicNotObserved) &&
		oneOf(authority, AuthorityRepositoryAccepted, AuthorityOwningVerifier, AuthorityProviderQualified, AuthorityAdapterQualified, AuthorityCallerReported, AuthorityAdvisory, AuthorityNone) &&
		oneOf(completeness, CompletenessComplete, CompletenessPartial, CompletenessUnknown) &&
		oneOf(currency, CurrencyValidatedAt, CurrencyHistorical, CurrencyStale, CurrencyMixed, CurrencyUnknown) &&
		oneOf(delivery, DeliveryAccepted, DeliveryValidated, DeliveryImplemented, DeliveryExperimental, DeliveryNotStarted, DeliveryFailed, DeliveryUnsupported)
}

func validateSnapshot(snapshot *Snapshot) error {
	if snapshot.Schema != SnapshotSchema || snapshot.Observation.LimitsProfile != LimitsProfile {
		return internalError()
	}
	for _, source := range snapshot.Sources {
		if !validAxes(source.Validity, source.EpistemicClass, source.AuthorityClass, source.Completeness, source.Currency, source.DeliveryStage) ||
			source.AdapterID == "" || source.Profile == "" || source.VerifierID == "" ||
			!validateDecimal(source.ConfiguredOrdinal) || source.DisplayLabel != source.AdapterID+"#"+source.ConfiguredOrdinal ||
			(source.ByteCount == nil) != (source.ContentSHA256 == nil) {
			return invalidArgument()
		}
		if source.ByteCount != nil && !validateDecimal(*source.ByteCount) {
			return invalidArgument()
		}
		if source.ContentSHA256 != nil && !validateSHA256(*source.ContentSHA256) {
			return invalidArgument()
		}
		if source.RepositoryReadsSHA256 != nil && (!validateSHA256(*source.RepositoryReadsSHA256) || source.AdapterID != "local-trace-v1" || source.Validity != ValidityValid) {
			return invalidArgument()
		}
		if source.AdapterID == "local-trace-v1" && source.Validity == ValidityValid && source.RepositoryReadsSHA256 == nil {
			return invalidArgument()
		}
		if source.AdapterID == "local-trace-v1" && source.ObservationTime != nil {
			return invalidArgument()
		}
		switch source.Validity {
		case ValidityValid:
			if source.ByteCount == nil || source.ContentSHA256 == nil || source.EpistemicClass != EpistemicObserved || !oneOf(source.Completeness, CompletenessComplete, CompletenessPartial) {
				return invalidArgument()
			}
		case ValidityUnsupported:
			if source.ByteCount != nil || len(source.CohortIDs) != 0 || source.Members != nil || source.ObservationStart != nil || source.ObservationEnd != nil || source.EpistemicClass != EpistemicNotObserved || source.AuthorityClass != AuthorityNone || source.Completeness != CompletenessUnknown || source.Currency != CurrencyUnknown {
				return invalidArgument()
			}
		case ValidityNotPresent, ValidityDisabled:
			if source.ByteCount != nil || len(source.CohortIDs) != 0 || source.Members != nil || source.ObservationStart != nil || source.ObservationEnd != nil || source.EpistemicClass != EpistemicNotObserved || source.Completeness != CompletenessUnknown {
				return invalidArgument()
			}
		case ValidityInvalid, ValidityInaccessible, ValidityExpired:
			if source.Completeness == CompletenessComplete {
				return invalidArgument()
			}
		}
		if source.ObservationTime != nil && !validateTimestamp(*source.ObservationTime) {
			return invalidArgument()
		}
		if source.ObservationStart != nil && !validateTimestamp(*source.ObservationStart) {
			return invalidArgument()
		}
		if source.ObservationEnd != nil && !validateTimestamp(*source.ObservationEnd) {
			return invalidArgument()
		}
		if (source.ObservationStart == nil) != (source.ObservationEnd == nil) || (source.ObservationStart != nil && *source.ObservationStart > *source.ObservationEnd) {
			return invalidArgument()
		}
		cohortIDs, err := normalizeSet(source.CohortIDs)
		if err != nil || len(cohortIDs) != len(source.CohortIDs) {
			return invalidArgument()
		}
		for index := range cohortIDs {
			if cohortIDs[index] != source.CohortIDs[index] {
				return invalidArgument()
			}
		}
		exclusions, err := normalizeSet(source.Exclusions)
		if err != nil || len(exclusions) != len(source.Exclusions) {
			return invalidArgument()
		}
		for index := range exclusions {
			if exclusions[index] != source.Exclusions[index] {
				return invalidArgument()
			}
		}
	}
	problematic := false
	for _, source := range snapshot.Sources {
		if oneOf(source.Validity, ValidityInvalid, ValidityInaccessible, ValidityExpired) || source.Completeness == CompletenessPartial {
			problematic = true
		}
	}
	if snapshot.Observation.ScanState == ScanComplete && problematic {
		return invalidArgument()
	}
	if snapshot.Observation.ScanState != ScanInvalid {
		return nil
	}
	for _, metric := range snapshot.Usage {
		if metric.ScopeClass != ScopeUnavailable {
			return invalidArgument() // An INVALID scan leaves every usage metric not observed.
		}
	}
	return nil
}

func profileAllowed(profile string, registration AdapterRegistration) bool {
	if registration.DeliveryStage == DeliveryUnsupported && registration.VerifierID == "unsupported" {
		return profile == "unsupported"
	}
	index := sort.SearchStrings(registration.AcceptedProfiles, profile)
	return index < len(registration.AcceptedProfiles) && registration.AcceptedProfiles[index] == profile
}

func validateSourceSemantics(source SourceInput, registration AdapterRegistration) error {
	if !validAxes(source.Validity, source.EpistemicClass, source.AuthorityClass, source.Completeness, source.Currency, source.DeliveryStage) {
		return invalidArgument()
	}
	if (source.ByteCount == nil) != (source.ContentSHA256 == nil) {
		return invalidArgument()
	}
	if registration.MaxBytes == "0" && (source.ByteCount != nil || source.ContentSHA256 != nil) {
		return invalidArgument()
	}
	switch source.Validity {
	case ValidityValid:
		if source.ByteCount == nil || source.ContentSHA256 == nil || source.EpistemicClass != EpistemicObserved || !oneOf(source.Completeness, CompletenessComplete, CompletenessPartial) {
			return invalidArgument()
		}
	case ValidityUnsupported:
		if source.ByteCount != nil || source.ContentSHA256 != nil || len(source.Cohorts) != 0 || source.Members != nil || source.ObservationStart != nil || source.ObservationEnd != nil || source.EpistemicClass != EpistemicNotObserved || source.AuthorityClass != AuthorityNone || source.Completeness != CompletenessUnknown || source.Currency != CurrencyUnknown {
			return invalidArgument()
		}
	case ValidityNotPresent, ValidityDisabled:
		if source.ByteCount != nil || source.ContentSHA256 != nil || len(source.Cohorts) != 0 || source.Members != nil || source.ObservationStart != nil || source.ObservationEnd != nil || source.EpistemicClass != EpistemicNotObserved || source.Completeness != CompletenessUnknown {
			return invalidArgument()
		}
	case ValidityInvalid, ValidityInaccessible, ValidityExpired:
		if source.Completeness == CompletenessComplete {
			return invalidArgument()
		}
	}
	return nil
}

func validateReferences(snapshot *Snapshot) error {
	sourceIDs := make(map[string]Source, len(snapshot.Sources))
	cohortIDs := make(map[string]Cohort, len(snapshot.Cohorts))
	issueIDs := make(map[string]Issue, len(snapshot.Issues))
	referencedCohorts := make(map[string]struct{}, len(snapshot.Cohorts))
	for _, source := range snapshot.Sources {
		sourceIDs[source.ID] = source
	}
	for _, cohort := range snapshot.Cohorts {
		cohortIDs[cohort.CohortID] = cohort
	}
	for _, issue := range snapshot.Issues {
		if issue.SourceID != nil {
			source, ok := sourceIDs[*issue.SourceID]
			if !ok || !slices.Contains(adapterPolicies[source.AdapterID].issueCodes, issue.Code) {
				return invalidArgument()
			}
		}
		issueIDs[issue.ID] = issue
	}
	for _, source := range snapshot.Sources {
		for _, id := range source.CohortIDs {
			cohort, ok := cohortIDs[id]
			if !ok || cohort.AdapterID != source.AdapterID || cohort.Profile != source.Profile || pointerValue(cohort.ProducerIdentity) != source.VerifierID {
				return invalidArgument()
			}
			if source.AdapterID == "local-trace-v1" && (snapshot.Repository.ObjectFormat == nil || snapshot.Repository.DirtyPathsSHA256 == nil ||
				cohort.RepositoryObjectFormat == nil || *cohort.RepositoryObjectFormat != *snapshot.Repository.ObjectFormat ||
				cohort.DirtyPathsSHA256 == nil || *cohort.DirtyPathsSHA256 != *snapshot.Repository.DirtyPathsSHA256) {
				return invalidArgument()
			}
			referencedCohorts[id] = struct{}{}
		}
		for _, id := range source.Exclusions {
			issue, ok := issueIDs[id]
			if !ok || issue.SourceID == nil || *issue.SourceID != source.ID {
				return invalidArgument()
			}
		}
	}
	if len(referencedCohorts) != len(cohortIDs) {
		return invalidArgument()
	}
	for _, group := range [][]Metric{snapshot.Data, snapshot.Usage, snapshot.Verification, snapshot.Frontier, snapshot.Beamfall} {
		for _, metric := range group {
			metricSources := make(map[string]struct{}, len(metric.SourceIDs))
			metricCohorts := make(map[string]struct{})
			for _, id := range metric.SourceIDs {
				source, ok := sourceIDs[id]
				if !ok {
					return invalidArgument()
				}
				metricSources[id] = struct{}{}
				for _, cohortID := range source.CohortIDs {
					metricCohorts[cohortID] = struct{}{}
				}
			}
			for _, id := range metric.CohortIDs {
				if _, ok := cohortIDs[id]; !ok {
					return invalidArgument()
				}
				if _, ok := metricCohorts[id]; !ok {
					return invalidArgument()
				}
			}
			for _, id := range metric.Exclusions {
				issue, ok := issueIDs[id]
				if !ok {
					return invalidArgument()
				}
				if issue.SourceID != nil {
					if _, ok := metricSources[*issue.SourceID]; !ok {
						return invalidArgument()
					}
				}
			}
		}
	}
	return nil
}

func validateCohortIdentity(cohort CohortIdentity, verifierID string) error {
	if cohort.AdapterID == "" || cohort.Profile == "" || cohort.ProducerIdentity == nil || *cohort.ProducerIdentity != verifierID {
		return invalidArgument()
	}
	if cohort.DirtyPathsSHA256 != nil && !validateSHA256(*cohort.DirtyPathsSHA256) {
		return invalidArgument()
	}
	if cohort.RepositoryObjectFormat != nil && *cohort.RepositoryObjectFormat != "sha1" && *cohort.RepositoryObjectFormat != "sha256" {
		return invalidArgument()
	}
	for _, objectID := range []*string{cohort.SourceRevision, cohort.SourceTreeRevision} {
		if objectID != nil && !validateObjectID(*objectID, cohort.RepositoryObjectFormat) {
			return invalidArgument()
		}
	}
	for _, timestamp := range []*string{cohort.SourceObservationStart, cohort.SourceObservationEnd} {
		if timestamp != nil && !validateTimestamp(*timestamp) {
			return invalidArgument()
		}
	}
	if (cohort.SourceObservationStart == nil) != (cohort.SourceObservationEnd == nil) {
		return invalidArgument()
	}
	if cohort.AdapterID == "local-trace-v1" && (cohort.DirtyPathsSHA256 == nil || cohort.RepositoryObjectFormat == nil ||
		cohort.SourceRevision == nil || cohort.SourceTreeRevision == nil || cohort.SourceObservationStart == nil || cohort.SourceObservationEnd == nil) {
		return invalidArgument()
	}
	if cohort.SourceObservationStart != nil && cohort.SourceObservationEnd != nil && *cohort.SourceObservationStart > *cohort.SourceObservationEnd {
		return invalidArgument()
	}
	return nil
}
