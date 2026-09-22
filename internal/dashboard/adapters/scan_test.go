package adapters

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/model"
	"github.com/Beamfall/corvint/internal/dashboard/source"
)

const scanGeneratedAt = "2026-08-23T12:00:00.000000000Z"

func TestScanCompilesFixedClockEmptyTraceStoreWithoutOwningAuthorityFinish(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".context-corvint", "traces"), 0o700); err != nil {
		t.Fatal(err)
	}
	authority := qualifiedAuthority()
	raw, err := Scan(context.Background(), scanRequest(root, authority))
	if err != nil {
		t.Fatal(err)
	}
	if authority.finishCalls != 0 {
		t.Fatalf("Scan called Finish %d times", authority.finishCalls)
	}
	snapshot, err := model.VerifyCanonical(raw)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.GeneratedAt != scanGeneratedAt || snapshot.Observation.Start != scanGeneratedAt || snapshot.Observation.End != scanGeneratedAt ||
		snapshot.Observation.ClockSource != model.ClockCaller || snapshot.Observation.ScanState != model.ScanComplete {
		t.Fatalf("observation = %#v", snapshot.Observation)
	}
	if len(snapshot.Sources) != 1 || snapshot.Sources[0].Members == nil || len(*snapshot.Sources[0].Members) != 0 ||
		snapshot.Sources[0].ObservationStart != nil || snapshot.Sources[0].ObservationEnd != nil {
		t.Fatalf("empty source = %#v", snapshot.Sources)
	}
	if countIssues(snapshot.Issues, "OBSERVATION_TIME_UNKNOWN") != 1 {
		t.Fatalf("issues = %#v", snapshot.Issues)
	}
}

func TestScanFixedClockValidAndRejectedTraceMembers(t *testing.T) {
	for _, test := range []struct {
		name        string
		body        []byte
		wantState   model.ScanState
		wantValid   model.Validity
		wantMembers int
		wantIssue   string
	}{
		{name: "valid", body: nil, wantState: model.ScanComplete, wantValid: model.ValidityValid, wantMembers: 1},
		{name: "rejected", body: []byte("secret"), wantState: model.ScanPartial, wantValid: model.ValidityInvalid, wantMembers: 0, wantIssue: "SOURCE_INVALID_SCHEMA"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, ".context-corvint", "traces")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			body := test.body
			if body == nil {
				body = traceFixture(t, "safe", "passed", []string{"internal/a.go"}, nil, nil)
			}
			if err := os.WriteFile(filepath.Join(directory, testRevision+".jsonl"), body, 0o600); err != nil {
				t.Fatal(err)
			}
			snapshotBytes, err := Scan(context.Background(), scanRequest(root, qualifiedAuthority()))
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := model.VerifyCanonical(snapshotBytes)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Observation.ScanState != test.wantState || len(snapshot.Sources) != 1 || snapshot.Sources[0].Validity != test.wantValid ||
				snapshot.Sources[0].Members == nil || len(*snapshot.Sources[0].Members) != test.wantMembers {
				t.Fatalf("snapshot = %#v", snapshot)
			}
			if test.wantMembers != 0 && (snapshot.Sources[0].ObservationStart == nil || *snapshot.Sources[0].ObservationStart != scanGeneratedAt ||
				snapshot.Sources[0].ObservationEnd == nil || *snapshot.Sources[0].ObservationEnd != scanGeneratedAt) {
				t.Fatalf("fixed source interval = %#v", snapshot.Sources[0])
			}
			if countIssues(snapshot.Issues, "OBSERVATION_TIME_UNKNOWN") != 1 ||
				(test.wantIssue != "" && countIssues(snapshot.Issues, test.wantIssue) != 1) {
				t.Fatalf("issues = %#v", snapshot.Issues)
			}
		})
	}
}

func TestScanMissingTraceStoreAndDeclaredUnsupportedInputsRemainObservable(t *testing.T) {
	root := t.TempDir()
	request := scanRequest(root, qualifiedAuthority())
	request.Sources = []ConfiguredSource{{AdapterID: AdapterQueryEnvelope, RelativePath: "not-read.json"}}
	request.CEMOCM = &CEMOCMBinding{
		CEMPath: "cem.json", OCMPath: "ocm.json", ExpectedBase: testRevision, Target: testRevision,
		Profile: "cem/0.2+ocm/0.1",
	}
	raw, err := Scan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := model.VerifyCanonical(raw)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Observation.ScanState != model.ScanComplete || len(snapshot.Sources) != 3 ||
		countIssues(snapshot.Issues, "SOURCE_NOT_PRESENT") != 1 || countIssues(snapshot.Issues, "SOURCE_UNSUPPORTED") != 2 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	missing := findIssue(snapshot.Issues, "SOURCE_NOT_PRESENT")
	if missing == nil || missing.Observed != nil || missing.Limit != nil {
		t.Fatalf("missing-store issue = %#v", missing)
	}
	for _, sourceRow := range snapshot.Sources {
		if sourceRow.AdapterID != string(AdapterLocalTrace) && sourceRow.Validity != model.ValidityUnsupported {
			t.Fatalf("unsupported source = %#v", sourceRow)
		}
	}
}

func TestScanRepositoryObjectUnavailableIsMemberTerminal(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, testRevision+".jsonl"), traceFixture(t, "safe", "passed", nil, nil, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	authority := qualifiedAuthority()
	authority.result.Code = AuthorityUnavailable
	raw, err := Scan(context.Background(), scanRequest(root, authority))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := model.VerifyCanonical(raw)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Observation.ScanState != model.ScanPartial || len(snapshot.Sources) != 1 ||
		snapshot.Sources[0].Validity != model.ValidityInvalid || snapshot.Sources[0].Members == nil ||
		len(*snapshot.Sources[0].Members) != 0 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	issue := findIssue(snapshot.Issues, "REPOSITORY_OBJECT_UNAVAILABLE")
	if issue == nil || issue.Observed == nil || *issue.Observed != "1" || issue.Limit != nil || issue.SourceID == nil ||
		len(snapshot.Sources[0].Exclusions) != 1 || snapshot.Sources[0].Exclusions[0] != issue.ID ||
		countIssues(snapshot.Issues, "OBSERVATION_TIME_UNKNOWN") != 1 {
		t.Fatalf("terminal issue/source = %#v / %#v", issue, snapshot.Sources[0])
	}
}

func TestScanAuthorityInterruptionIsFatalAndNeverSerialized(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	body := traceFixture(t, "safe", "passed", []string{"internal/a.go"}, nil, nil)
	if err := os.WriteFile(filepath.Join(directory, testRevision+".jsonl"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	authority := qualifiedAuthority()
	authority.result.Code = AuthorityInterrupted
	raw, err := Scan(context.Background(), scanRequest(root, authority))
	var failure *ScanError
	if len(raw) != 0 || !errors.As(err, &failure) || failure.Code != "DASHBOARD_INTERRUPTED" {
		t.Fatalf("raw=%q error=%v", raw, err)
	}
	if authority.qualifyCalls != 1 {
		t.Fatalf("qualify calls=%d", authority.qualifyCalls)
	}
}

func TestScanReportsExactTraceStoreBounds(t *testing.T) {
	for _, test := range []struct {
		name      string
		populate  func(*testing.T, string)
		wantLimit string
	}{
		{
			name: "directory entry count",
			populate: func(t *testing.T, directory string) {
				for index := 0; index <= source.MaxTraceStoreEntries; index++ {
					name := filepath.Join(directory, "ignored-"+strconv.Itoa(index))
					if err := os.WriteFile(name, nil, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			},
			wantLimit: strconv.FormatUint(source.MaxTraceStoreEntries, 10),
		},
		{
			name: "aggregate rows",
			populate: func(t *testing.T, directory string) {
				body := bytes.Repeat([]byte("x\n"), maxTraceRows+1)
				if err := os.WriteFile(filepath.Join(directory, testRevision+".jsonl"), body, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantLimit: strconv.FormatUint(maxTraceRows, 10),
		},
		{
			name: "member row bytes",
			populate: func(t *testing.T, directory string) {
				body := make([]byte, maxTraceRowBytes+1)
				for index := range body {
					body[index] = 'x'
				}
				if err := os.WriteFile(filepath.Join(directory, testRevision+".jsonl"), body, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantLimit: strconv.FormatUint(maxTraceRowBytes, 10),
		},
		{
			name: "member row bytes after malformed row",
			populate: func(t *testing.T, directory string) {
				body := append([]byte("x\n"), bytes.Repeat([]byte{'x'}, maxTraceRowBytes+1)...)
				if err := os.WriteFile(filepath.Join(directory, testRevision+".jsonl"), body, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantLimit: strconv.FormatUint(maxTraceRowBytes, 10),
		},
		{
			name: "aggregate store bytes",
			populate: func(t *testing.T, directory string) {
				line := append(bytes.Repeat([]byte{'x'}, 17*1024-1), '\n')
				body := bytes.Repeat(line, 500)
				for _, revision := range []string{testRevision, strings.Repeat("d", 40)} {
					if err := os.WriteFile(filepath.Join(directory, revision+".jsonl"), body, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			},
			wantLimit: strconv.FormatUint(maxTraceStoreBytes, 10),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, ".context-corvint", "traces")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			test.populate(t, directory)
			authority := qualifiedAuthority()
			raw, err := Scan(context.Background(), scanRequest(root, authority))
			if err != nil {
				t.Fatal(err)
			}
			if authority.qualifyCalls != 0 {
				t.Fatalf("aggregate precedence invoked authority %d times", authority.qualifyCalls)
			}
			snapshot, err := model.VerifyCanonical(raw)
			if err != nil {
				t.Fatal(err)
			}
			issue := findIssue(snapshot.Issues, "TRACE_STORE_BOUND")
			if snapshot.Observation.ScanState != model.ScanPartial || issue == nil || issue.Limit == nil ||
				*issue.Limit != test.wantLimit || issue.Observed != nil || countIssues(snapshot.Issues, "OBSERVATION_TIME_UNKNOWN") != 0 {
				t.Fatalf("snapshot/issue = %#v / %#v", snapshot.Observation, issue)
			}
		})
	}
}

func TestScanCapsTraceRowAllocationBeforeParsing(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	body := bytes.Repeat([]byte{'\n'}, 1<<20)
	if err := os.WriteFile(filepath.Join(directory, testRevision+".jsonl"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	authority := qualifiedAuthority()
	result := testing.Benchmark(func(benchmark *testing.B) {
		for range benchmark.N {
			request := scanRequest(root, authority)
			request.SourceBudget = source.NewBudget(source.MaxAggregateBytes)
			if raw, err := Scan(context.Background(), request); err != nil || len(raw) == 0 {
				benchmark.Fatalf("Scan raw=%d error=%v", len(raw), err)
			}
		}
	})
	if allocated := result.AllocedBytesPerOp(); allocated > 32<<20 {
		t.Fatalf("Scan allocated %d bytes for an over-bound row set", allocated)
	}
}

func TestTraceStoreBoundRejectsMissingOrUnknownLimit(t *testing.T) {
	configured := plannedSource{adapterID: AdapterLocalTrace, profile: "corvint-local-trace/1"}
	for _, result := range []source.TraceStoreResult{
		{Issue: source.IssueTraceStoreBound},
		{Issue: source.IssueTraceStoreBound, Limit: 1},
		{Issue: source.IssueTraceStoreBound, Limit: maxTraceRowBytes},
		{Issue: source.IssueTraceStoreBound, Limit: maxTraceStoreBytes + 1},
		{Issue: source.IssueSourceUnavailable, Limit: 1},
	} {
		_, _, _, _, failure := traceStoreRootFailure(configured, result)
		if failure == nil || failure.Code != "DASHBOARD_INTERNAL_ERROR" {
			t.Fatalf("result %#v failure = %#v", result, failure)
		}
	}
}

func TestScanPreflightsCompiledSourceBoundsBeforeOpeningRoot(t *testing.T) {
	generated := scanGeneratedAt
	request := ScanRequest{
		Root: filepath.Join(t.TempDir(), "missing"), GeneratedAt: &generated, ClockSource: "CALLER",
		Authority: qualifiedAuthority(), SourceBudget: source.NewBudget(maxTraceStoreBytes - 1),
	}
	_, err := Scan(context.Background(), request)
	var failure *ScanError
	if !errors.As(err, &failure) || failure.Code != "DASHBOARD_RESOURCE_EXHAUSTED" {
		t.Fatalf("preflight error = %v", err)
	}
}

func TestScanPreflightDoesNotDoubleChargeRetryBudget(t *testing.T) {
	root := t.TempDir()
	budget := source.NewBudget(source.MaxAggregateBytes)
	request := scanRequest(root, qualifiedAuthority())
	request.SourceBudget = budget
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := Scan(context.Background(), request); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
	}
	if got := budget.Used(); got != maxTraceStoreBytes {
		t.Fatalf("logical reservation = %d, want %d", got, maxTraceStoreBytes)
	}
}

func scanRequest(root string, authority *fakeAuthority) ScanRequest {
	generated := scanGeneratedAt
	return ScanRequest{
		Root: root, GeneratedAt: &generated, ClockSource: "CALLER", Authority: authority,
		SourceBudget: source.NewBudget(source.MaxAggregateBytes),
	}
}

func countIssues(issues []model.Issue, code string) int {
	count := 0
	for _, issue := range issues {
		if issue.Code == code {
			count++
		}
	}
	return count
}

func findIssue(issues []model.Issue, code string) *model.Issue {
	for index := range issues {
		if issues[index].Code == code {
			return &issues[index]
		}
	}
	return nil
}
