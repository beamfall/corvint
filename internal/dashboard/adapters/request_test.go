package adapters

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/source"
)

func TestPlanRequestSortsBeforeAssigningPathFreeOrdinals(t *testing.T) {
	generated := "2026-08-23T12:00:00.000000000Z"
	root := filepath.Join(string(filepath.Separator), "repo")
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

func TestPlanRequestRejectsDuplicatesTraversalAndPartialBundle(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
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
		{Root: root, ClockSource: "PROCESS", CEMOCM: &CEMOCMBinding{CEMPath: "cem.json", OCMPath: "", ExpectedBase: testRevision, Target: testRevision}},
		{Root: root, ClockSource: "PROCESS", CEMOCM: &CEMOCMBinding{CEMPath: "same.json", OCMPath: "same.json", ExpectedBase: testRevision, Target: testRevision}},
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

func TestPlanRequestRequiresCallerOwnedBudgetAuthorityAndExplicitBundleProfile(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	base := ScanRequest{Root: root, ClockSource: "PROCESS", Authority: qualifiedAuthority(), SourceBudget: source.NewBudget(source.MaxAggregateBytes)}
	withoutAuthority := base
	withoutAuthority.Authority = nil
	withoutBudget := base
	withoutBudget.SourceBudget = nil
	missingProfile := base
	missingProfile.CEMOCM = &CEMOCMBinding{CEMPath: "cem.json", OCMPath: "ocm.json", ExpectedBase: testRevision, Target: testRevision}
	for index, request := range []ScanRequest{withoutAuthority, withoutBudget, missingProfile} {
		if _, failure := planRequest(request); failure == nil {
			t.Fatalf("case %d accepted", index)
		}
	}
	withBundle := base
	withBundle.CEMOCM = &CEMOCMBinding{CEMPath: "cem.json", OCMPath: "ocm.json", ExpectedBase: testRevision, Target: testRevision, Profile: "cem/0.2+ocm/0.1"}
	planned, failure := planRequest(withBundle)
	if failure != nil || len(planned) != 2 || planned[1] != (plannedSource{adapterID: AdapterCEMOCMBundle, ordinal: 0, profile: "cem/0.2+ocm/0.1"}) {
		t.Fatalf("planned=%#v failure=%v", planned, failure)
	}
}

func pointer(value string) *string { return &value }
