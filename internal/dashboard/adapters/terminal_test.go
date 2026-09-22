package adapters

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/model"
)

func TestTraceTerminalIssueInputsAreClosedCountedAndPathFree(t *testing.T) {
	sourceID := dashboardSourceIDPrefix + strings.Repeat("a", 64)
	counts := []TraceTerminalCount{
		{Code: IssueVerifierRejected, Count: 2},
		{Code: IssueSourceInvalidIdentity, Count: 1},
	}
	issues, exclusions, err := TraceTerminalIssueInputs(sourceID, counts)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 || issues[0].Code != string(IssueSourceInvalidIdentity) ||
		issues[0].Observed == nil || *issues[0].Observed != "1" ||
		issues[1].Code != string(IssueVerifierRejected) || issues[1].Observed == nil || *issues[1].Observed != "2" {
		t.Fatalf("issues = %#v", issues)
	}
	wantExclusions := make([]string, 0, len(issues))
	for _, input := range issues {
		if input.Severity != model.SeverityError || input.SourceID == nil || *input.SourceID != sourceID || input.Limit != nil {
			t.Fatalf("terminal issue shape = %#v", input)
		}
		issue, issueErr := model.NewIssue(input)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		wantExclusions = append(wantExclusions, issue.ID)
	}
	sort.Strings(wantExclusions)
	if !reflect.DeepEqual(exclusions, wantExclusions) {
		t.Fatalf("exclusions = %q, want %q", exclusions, wantExclusions)
	}
}

func TestTraceTerminalIssueInputsRejectNonterminalZeroDuplicateAndBadSource(t *testing.T) {
	sourceID := dashboardSourceIDPrefix + strings.Repeat("a", 64)
	tests := []struct {
		source string
		counts []TraceTerminalCount
	}{
		{source: sourceID, counts: []TraceTerminalCount{{Code: "STORE_CHANGED", Count: 1}}},
		{source: sourceID, counts: []TraceTerminalCount{{Code: IssueVerifierRejected, Count: 0}}},
		{source: sourceID, counts: []TraceTerminalCount{{Code: IssueVerifierRejected, Count: 1}, {Code: IssueVerifierRejected, Count: 2}}},
		{source: "dashboard-source:sha256:" + strings.Repeat("A", 64), counts: nil},
	}
	for index, test := range tests {
		if _, _, err := TraceTerminalIssueInputs(test.source, test.counts); err == nil {
			t.Fatalf("case %d accepted", index)
		}
	}
}

func TestTraceTerminalIssueInputsAdmitEveryFrozenTerminalCode(t *testing.T) {
	sourceID := dashboardSourceIDPrefix + strings.Repeat("b", 64)
	counts := make([]TraceTerminalCount, 0, len(traceTerminalCodes))
	for code := range traceTerminalCodes {
		counts = append(counts, TraceTerminalCount{Code: code, Count: 1})
	}
	issues, exclusions, err := TraceTerminalIssueInputs(sourceID, counts)
	if err != nil || len(issues) != 11 || len(exclusions) != 11 {
		t.Fatalf("issues=%d exclusions=%d err=%v", len(issues), len(exclusions), err)
	}
}
