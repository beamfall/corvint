package model

func validateDecodedTraceAggregate(source Source, cohorts []Cohort) error {
	if source.AdapterID != "local-trace-v1" {
		if source.Members != nil {
			return invalidArgument()
		}
		return nil
	}
	if !oneOf(source.Validity, ValidityValid, ValidityInvalid) {
		if source.Members != nil {
			return invalidArgument()
		}
		return nil
	}
	identities := make([]CohortIdentity, 0, len(source.CohortIDs))
	byID := make(map[string]Cohort, len(cohorts))
	for _, cohort := range cohorts {
		byID[cohort.CohortID] = cohort
	}
	for _, id := range source.CohortIDs {
		cohort, ok := byID[id]
		if !ok {
			return invalidArgument()
		}
		identities = append(identities, CohortIdentity{cohort.AdapterID, cohort.Profile, cohort.RepositoryObjectFormat,
			cohort.SourceRevision, cohort.SourceTreeRevision, cohort.DirtyPathsSHA256, cohort.ProducerIdentity,
			cohort.SourceObservationStart, cohort.SourceObservationEnd})
	}
	input := SourceInput{AdapterID: source.AdapterID, ByteCount: source.ByteCount, ContentSHA256: source.ContentSHA256,
		Validity: source.Validity, Cohorts: identities, Members: source.Members,
		ObservationStart: source.ObservationStart, ObservationEnd: source.ObservationEnd}
	_, err := validateAndCloneMembers(input, source.CohortIDs)
	return err
}
