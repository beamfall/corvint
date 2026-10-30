package adapters

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/dashboard/model"
)

const dashboardSourceIDPrefix = "dashboard-source:sha256:"

type TraceTerminalCount struct {
	Code  AdapterIssueCode
	Count uint64
}

var traceTerminalCodes = map[AdapterIssueCode]struct{}{
	"REPOSITORY_OBJECT_UNAVAILABLE": {},
	"SOURCE_CHANGED_DURING_READ":    {},
	"SOURCE_INACCESSIBLE":           {},
	"SOURCE_INVALID_IDENTITY":       {},
	"SOURCE_INVALID_SCHEMA":         {},
	"SOURCE_MULTILINK_UNQUALIFIED":  {},
	"SOURCE_OVERSIZED":              {},
	"SOURCE_SPECIAL_FILE":           {},
	"SOURCE_SYMLINK":                {},
	"TRACE_ANCESTRY_BOUND":          {},
	"VERIFIER_REJECTED":             {},
}

// TraceTerminalIssueInputs converts the closed rejected-member count set into
// exact model inputs and the corresponding sorted exclusion IDs. It cannot
// represent aggregate controls or zero/duplicate terminal counts.
func TraceTerminalIssueInputs(sourceID string, counts []TraceTerminalCount) ([]model.IssueInput, []string, error) {
	if !validDashboardSourceID(sourceID) {
		return nil, nil, invalidArgumentError()
	}
	ordered := append([]TraceTerminalCount(nil), counts...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].Code < ordered[right].Code })
	issues := make([]model.IssueInput, 0, len(ordered))
	exclusions := make([]string, 0, len(ordered))
	for index, count := range ordered {
		if _, terminal := traceTerminalCodes[count.Code]; !terminal || count.Count == 0 ||
			(index > 0 && ordered[index-1].Code == count.Code) {
			return nil, nil, invalidArgumentError()
		}
		observed := strconv.FormatUint(count.Count, 10)
		input := model.IssueInput{
			Code: string(count.Code), Severity: model.SeverityError,
			SourceID: stringPointer(sourceID), Observed: &observed,
		}
		issue, err := model.NewIssue(input)
		if err != nil {
			return nil, nil, invalidArgumentError()
		}
		issues = append(issues, input)
		exclusions = append(exclusions, issue.ID)
	}
	sort.Strings(exclusions)
	return issues, exclusions, nil
}

func validDashboardSourceID(value string) bool {
	digest, ok := strings.CutPrefix(value, dashboardSourceIDPrefix)
	return ok && lowerHex(digest, 64)
}
