package model

const maxTraceCandidates uint64 = 1_000

func validateTraceIssueRelationships(sources []Source, issues []Issue) error {
	issuesByID := make(map[string]Issue, len(issues))
	issuesBySource := make(map[string][]Issue)
	sourcesByID := make(map[string]Source, len(sources))
	for _, source := range sources {
		sourcesByID[source.ID] = source
	}
	for _, issue := range issues {
		issuesByID[issue.ID] = issue
		if issue.SourceID != nil {
			issuesBySource[*issue.SourceID] = append(issuesBySource[*issue.SourceID], issue)
		}
		_, terminal := memberTerminalIssueCodes[issue.Code]
		if terminal && issue.Observed != nil {
			if issue.SourceID == nil || sourcesByID[*issue.SourceID].AdapterID != "local-trace-v1" {
				return invalidArgument()
			}
		}
		if issue.Code == "OBSERVATION_TIME_UNKNOWN" && (issue.SourceID == nil || sourcesByID[*issue.SourceID].AdapterID != "local-trace-v1") {
			return invalidArgument()
		}
	}
	for _, source := range sources {
		if source.AdapterID != "local-trace-v1" {
			continue
		}
		if source.Members == nil {
			if err := validateTraceRootIssue(source, issuesByID, issuesBySource[source.ID]); err != nil {
				return err
			}
			continue
		}
		exclusions := make(map[string]struct{}, len(source.Exclusions))
		seenCodes := make(map[string]struct{}, len(source.Exclusions))
		var terminalTotal uint64
		for _, id := range source.Exclusions {
			issue, ok := issuesByID[id]
			if !ok || !isExactMemberTerminalIssue(issue, source.ID) {
				return invalidArgument()
			}
			if _, duplicate := seenCodes[issue.Code]; duplicate {
				return invalidArgument()
			}
			seenCodes[issue.Code] = struct{}{}
			exclusions[id] = struct{}{}
			count, valid := parseDecimal(pointerValue(issue.Observed))
			if !valid || count == 0 || count > maxTraceCandidates || terminalTotal > maxTraceCandidates-count {
				return invalidArgument()
			}
			terminalTotal += count
		}
		observationIssues := 0
		for _, issue := range issuesBySource[source.ID] {
			if _, terminal := memberTerminalIssueCodes[issue.Code]; terminal && issue.Observed != nil {
				if _, excluded := exclusions[issue.ID]; !excluded {
					return invalidArgument()
				}
				continue
			}
			if issue.Code != "OBSERVATION_TIME_UNKNOWN" || issue.Severity != SeverityInfo || issue.Observed != nil || issue.Limit != nil {
				return invalidArgument()
			}
			observationIssues++
		}
		if observationIssues != 1 || uint64(len(*source.Members))+terminalTotal > maxTraceCandidates {
			return invalidArgument()
		}
		if terminalTotal == 0 {
			if source.Validity != ValidityValid || source.Completeness != CompletenessComplete {
				return invalidArgument()
			}
		} else if source.Completeness != CompletenessPartial ||
			(len(*source.Members) == 0 && source.Validity != ValidityInvalid) ||
			(len(*source.Members) != 0 && source.Validity != ValidityValid) {
			return invalidArgument()
		}
	}
	return nil
}

func isExactMemberTerminalIssue(issue Issue, sourceID string) bool {
	_, terminal := memberTerminalIssueCodes[issue.Code]
	return terminal && issue.SourceID != nil && *issue.SourceID == sourceID &&
		issue.Severity == SeverityError && issue.Observed != nil && *issue.Observed != "0" && issue.Limit == nil
}

func validateTraceRootIssue(source Source, issuesByID map[string]Issue, scoped []Issue) error {
	if len(source.Exclusions) != 1 || len(scoped) != 1 || scoped[0].ID != source.Exclusions[0] {
		return invalidArgument()
	}
	issue, ok := issuesByID[source.Exclusions[0]]
	if !ok || issue.SourceID == nil || *issue.SourceID != source.ID {
		return invalidArgument()
	}
	switch issue.Code {
	case "SOURCE_NOT_PRESENT":
		if source.Validity != ValidityNotPresent || issue.Severity != SeverityWarning || issue.Observed != nil || issue.Limit != nil {
			return invalidArgument()
		}
	case "SOURCE_INACCESSIBLE", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK", "SOURCE_MULTILINK_UNQUALIFIED",
		"STORE_CHANGED":
		if issue.Severity != SeverityError || issue.Observed != nil || issue.Limit != nil {
			return invalidArgument()
		}
	case "TRACE_STORE_BOUND":
		if issue.Severity != SeverityError || issue.Observed != nil || issue.Limit == nil ||
			!oneOf(*issue.Limit, "1000", "262144", "16777216") {
			return invalidArgument()
		}
	default:
		return invalidArgument()
	}
	return nil
}
