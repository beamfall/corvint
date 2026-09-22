package model

import "sort"

func validateAndCloneMembers(input SourceInput, cohortIDs []string) (*[]TraceMember, error) {
	if input.AdapterID != "local-trace-v1" {
		if input.Members != nil {
			return nil, invalidArgument()
		}
		return nil, nil
	}
	if input.Members == nil {
		if input.Validity == ValidityValid {
			return nil, invalidArgument()
		}
		return nil, nil
	}
	if !oneOf(input.Validity, ValidityValid, ValidityInvalid) {
		return nil, invalidArgument()
	}
	members := append([]TraceMember(nil), (*input.Members)...)
	if len(members) > int(maxTraceCandidates) {
		return nil, resourceExhausted()
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Revision < members[j].Revision })
	var total uint64
	for index, member := range members {
		if index > 0 && members[index-1].Revision == member.Revision {
			return nil, invalidArgument()
		}
		if !validateSHA256(member.ContentSHA256) || !validateDecimal(member.ByteCount) {
			return nil, invalidArgument()
		}
		value, ok := parseDecimal(member.ByteCount)
		if !ok || total > ^uint64(0)-value {
			return nil, resourceExhausted()
		}
		total += value
	}
	if input.Validity == ValidityInvalid {
		if len(members) != 0 || len(input.Cohorts) != 0 || input.ByteCount != nil || input.ContentSHA256 != nil || input.ObservationStart != nil || input.ObservationEnd != nil {
			return nil, invalidArgument()
		}
		return &members, nil
	}
	if input.ByteCount == nil || *input.ByteCount != decimalUint(total) {
		return nil, invalidArgument()
	}
	digest, err := domainHash("sha256:", traceStoreDomain, members)
	if err != nil || input.ContentSHA256 == nil || *input.ContentSHA256 != digest {
		return nil, invalidArgument()
	}
	if len(members) != len(input.Cohorts) || len(cohortIDs) != len(input.Cohorts) {
		return nil, invalidArgument()
	}
	cohortByRevision := make(map[string]CohortIdentity, len(input.Cohorts))
	for _, cohort := range input.Cohorts {
		if cohort.SourceRevision == nil {
			return nil, invalidArgument()
		}
		if _, exists := cohortByRevision[*cohort.SourceRevision]; exists {
			return nil, invalidArgument()
		}
		cohortByRevision[*cohort.SourceRevision] = cohort
	}
	var minimum, maximum *string
	for _, member := range members {
		cohort, ok := cohortByRevision[member.Revision]
		if !ok || cohort.SourceObservationStart == nil || cohort.SourceObservationEnd == nil {
			return nil, invalidArgument()
		}
		if minimum == nil || *cohort.SourceObservationStart < *minimum {
			minimum = cloneString(cohort.SourceObservationStart)
		}
		if maximum == nil || *cohort.SourceObservationEnd > *maximum {
			maximum = cloneString(cohort.SourceObservationEnd)
		}
	}
	if pointerValue(input.ObservationStart) != pointerValue(minimum) || pointerValue(input.ObservationEnd) != pointerValue(maximum) ||
		(input.ObservationStart == nil) != (minimum == nil) || (input.ObservationEnd == nil) != (maximum == nil) {
		return nil, invalidArgument()
	}
	return &members, nil
}
