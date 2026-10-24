package adapters

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/source"
)

// absoluteFixtureRoot is an absolute root that need not exist: \repo alone has
// no volume and is not absolute on Windows (V1-2488).
func absoluteFixtureRoot() string {
	return filepath.Join(filepath.VolumeName(os.TempDir())+string(filepath.Separator), "repo")
}

func TestPlanRequestSortsBeforeAssigningPathFreeOrdinals(t *testing.T) {
	generated := "2026-08-23T12:00:00.000000000Z"
	root := absoluteFixtureRoot()
	planned, failure := planRequest(ScanRequest{
		Root: root, GeneratedAt: &generated, ClockSource: "CALLER",
		Authority: qualifiedAuthority(), SourceBudget: source.NewBudget(source.MaxAggregateBytes),
		Sources: []ConfiguredSource{
			{AdapterID: AdapterLocalTrace, RelativePath: "z.jsonl"},
			{AdapterID: AdapterQueryEnvelope, RelativePath: "query.json"},
			{AdapterID: AdapterLocalTrace, RelativePath: "a.jsonl"},
		},
	})
	if failure != nil {
		t.Fatal(failure)
	}
	want := []plannedSource{
		{adapterID: AdapterLocalTrace, relativePath: ".context-corvint/traces", ordinal: 0, profile: "corvint-local-trace/1"},
		{adapterID: AdapterLocalTrace, relativePath: "a.jsonl", ordinal: 1, profile: "corvint-local-trace/1"},
		{adapterID: AdapterLocalTrace, relativePath: "z.jsonl", ordinal: 2, profile: "corvint-local-trace/1"},
		{adapterID: AdapterQueryEnvelope, relativePath: "query.json", ordinal: 0, profile: "unsupported"},
	}
	if !reflect.DeepEqual(planned, want) {
		t.Fatalf("plan = %#v, want %#v", planned, want)
	}
}

func TestPlanRequestRejectsDuplicatesAndTraversal(t *testing.T) {
	root := absoluteFixtureRoot()
	base := ScanRequest{Root: root, ClockSource: "PROCESS", Authority: qualifiedAuthority(), SourceBudget: source.NewBudget(source.MaxAggregateBytes)}
	tests := []ScanRequest{
		{Root: "repo", ClockSource: "PROCESS"},
		{Root: root + string(filepath.Separator) + ".." + string(filepath.Separator) + "repo", ClockSource: "PROCESS"},
		{Root: root + "\x00", ClockSource: "PROCESS"},
		{Root: string(filepath.Separator) + strings.Repeat("a", 4097), ClockSource: "PROCESS"},
		{Root: root, ClockSource: "CALLER"},
		{Root: root, ClockSource: "PROCESS", GeneratedAt: pointer("2026-08-23T12:00:00.000000000Z")},
		{Root: root, ClockSource: "CALLER", GeneratedAt: pointer("2026-08-23T12:00:00Z")},
		{Root: root, ClockSource: "PROCESS", Sources: []ConfiguredSource{{AdapterID: AdapterStableRead, RelativePath: "x"}}},
		{Root: root, ClockSource: "PROCESS", Sources: []ConfiguredSource{{AdapterID: AdapterLocalTrace, RelativePath: "../x"}}},
		{Root: root, ClockSource: "PROCESS", Sources: []ConfiguredSource{{AdapterID: AdapterLocalTrace, RelativePath: "C:/x"}}},
		{Root: root, ClockSource: "PROCESS", Sources: []ConfiguredSource{{AdapterID: AdapterLocalTrace, RelativePath: ".context-corvint/traces"}}},
		{Root: root, ClockSource: "PROCESS", Sources: []ConfiguredSource{{AdapterID: AdapterLocalTrace, RelativePath: "x"}, {AdapterID: AdapterLocalTrace, RelativePath: "x"}}},
	}
	for index, request := range tests {
		request.Authority = qualifiedAuthority()
		request.SourceBudget = source.NewBudget(source.MaxAggregateBytes)
		if _, failure := planRequest(request); failure == nil || failure.Code != "DASHBOARD_INVALID_ARGUMENT" {
			t.Fatalf("case %d failure = %#v", index, failure)
		}
	}
	if _, failure := planRequest(base); failure != nil {
		t.Fatalf("minimal request failed: %v", failure)
	}
}

func TestPlanRequestRequiresCallerOwnedBudgetAndAuthority(t *testing.T) {
	root := absoluteFixtureRoot()
	base := ScanRequest{Root: root, ClockSource: "PROCESS", Authority: qualifiedAuthority(), SourceBudget: source.NewBudget(source.MaxAggregateBytes)}
	withoutAuthority := base
	withoutAuthority.Authority = nil
	withoutBudget := base
	withoutBudget.SourceBudget = nil
	for index, request := range []ScanRequest{withoutAuthority, withoutBudget} {
		if _, failure := planRequest(request); failure == nil {
			t.Fatalf("case %d accepted", index)
		}
	}
	planned, failure := planRequest(base)
	if failure != nil || len(planned) != 1 || planned[0].adapterID != AdapterLocalTrace {
		t.Fatalf("planned=%#v failure=%v", planned, failure)
	}
}

func pointer(value string) *string { return &value }
