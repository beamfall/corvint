package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/adapters"
	"github.com/Beamfall/corvint/internal/dashboard/authority"
	"github.com/Beamfall/corvint/internal/dashboard/repository"
	"github.com/Beamfall/corvint/internal/dashboard/source"
)

type fakeBridgeAuthority struct {
	finishCode  authority.FinishCode
	finishCalls int
	snapshot    authority.Snapshot
}

func (fake *fakeBridgeAuthority) Snapshot() authority.Snapshot { return fake.snapshot }

func (*fakeBridgeAuthority) QualifyTrace(context.Context, string, []string) authority.TraceQualification {
	return authority.TraceQualification{Code: authority.TraceObjectUnavailable}
}

func (fake *fakeBridgeAuthority) Finish(context.Context) authority.FinishResult {
	fake.finishCalls++
	return authority.FinishResult{Code: fake.finishCode}
}

func TestCompileSnapshotRetriesWithAttemptScopedProbesAndCumulativeSourceLedger(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".context-corvint", "traces"), 0o700); err != nil {
		t.Fatal(err)
	}
	repositorySnapshot := authority.Snapshot{
		DirtyPathsSHA256: "sha256:" + strings.Repeat("0", 64),
		HeadRevision:     testBase,
		ObjectFormat:     "sha1",
		TreeRevision:     testTarget,
		WorktreeState:    authority.WorktreeClean,
	}
	authorities := []*fakeBridgeAuthority{
		{finishCode: authority.FinishChanged, snapshot: repositorySnapshot},
		{finishCode: authority.FinishStable, snapshot: repositorySnapshot},
	}
	var scanBudgets []*source.Budget
	attempt := 0
	dependencies := bridgeDependencies{
		newRepositoryBudget: repository.NewBudget,
		newSourceBudget:     source.NewBudget,
		newAttempt: func(string, *repository.Budget) (authority.Authority, error) {
			result := authorities[attempt]
			attempt++
			return result, nil
		},
		scan: func(ctx context.Context, request adapters.ScanRequest) ([]byte, error) {
			scanBudgets = append(scanBudgets, request.SourceBudget)
			return adapters.Scan(ctx, request)
		},
	}
	encoded, err := compileSnapshotWith(context.Background(), compileRequest{Root: root, GeneratedAt: testTime}, dependencies)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("encoded bytes=%d err=%v", len(encoded), err)
	}
	if len(scanBudgets) != 2 || scanBudgets[0] == nil || scanBudgets[1] == nil {
		t.Fatalf("scan budgets=%#v", scanBudgets)
	}
	if scanBudgets[0] == scanBudgets[1] {
		t.Fatal("whole-scan retry shared its overflow-probe allowance")
	}
	if scanBudgets[0].Used() != source.MaxSourceBytes || scanBudgets[1].Used() != source.MaxSourceBytes {
		t.Fatalf("logical bytes=%d/%d, want cumulative %d", scanBudgets[0].Used(), scanBudgets[1].Used(), source.MaxSourceBytes)
	}
}

func TestCompileSnapshotRetriesOnceWithCumulativeBudgetsAndTerminalFinish(t *testing.T) {
	authorities := []*fakeBridgeAuthority{
		{finishCode: authority.FinishChanged},
		{finishCode: authority.FinishStable},
	}
	var repositoryBudgets []*repository.Budget
	var sourceBudgets []*source.Budget
	var scanRequests []adapters.ScanRequest
	scanIndex := 0
	dependencies := bridgeDependencies{
		newRepositoryBudget: repository.NewBudget,
		newSourceBudget: func(limit uint64) *source.Budget {
			if limit != source.MaxAggregateBytes {
				t.Fatalf("source budget limit=%d", limit)
			}
			return source.NewBudget(limit)
		},
		newAttempt: func(_ string, budget *repository.Budget) (authority.Authority, error) {
			repositoryBudgets = append(repositoryBudgets, budget)
			return authorities[len(repositoryBudgets)-1], nil
		},
		scan: func(_ context.Context, request adapters.ScanRequest) ([]byte, error) {
			sourceBudgets = append(sourceBudgets, request.SourceBudget)
			scanRequests = append(scanRequests, request)
			scanIndex++
			if scanIndex == 1 {
				return []byte("discarded"), nil
			}
			return []byte("accepted"), nil
		},
	}
	request := compileRequest{
		Root: "/repo", GeneratedAt: testTime,
		Sources: []sourceOption{{AdapterID: "local-trace-v1", RelativePath: "traces=retained"}},
		Bundle: &bundleOption{
			CEM: "cem.json", OCM: "ocm.json", Profile: "cem/0.2+ocm/0.1",
			ExpectedBase: testBase, Target: testTarget,
		},
	}
	encoded, err := compileSnapshotWith(context.Background(), request, dependencies)
	if err != nil || string(encoded) != "accepted" {
		t.Fatalf("encoded=%q err=%v", encoded, err)
	}
	if len(repositoryBudgets) != 2 || repositoryBudgets[0] != repositoryBudgets[1] {
		t.Fatal("repository budget was not shared across exactly two attempts")
	}
	if len(sourceBudgets) != 2 || sourceBudgets[0] == nil || sourceBudgets[1] == nil || sourceBudgets[0] == sourceBudgets[1] {
		t.Fatal("source attempts did not receive distinct probe budgets")
	}
	for index, fake := range authorities {
		if fake.finishCalls != 1 {
			t.Fatalf("attempt %d finish calls=%d", index, fake.finishCalls)
		}
	}
	for _, got := range scanRequests {
		if got.ClockSource != "CALLER" || got.GeneratedAt == nil || *got.GeneratedAt != testTime || got.Authority == nil {
			t.Fatalf("clock/authority mapping=%#v", got)
		}
		if len(got.Sources) != 1 || got.Sources[0].RelativePath != "traces=retained" || got.CEMOCM == nil || got.CEMOCM.Profile != "cem/0.2+ocm/0.1" {
			t.Fatalf("source/bundle mapping=%#v", got)
		}
	}
}

func TestCompileSnapshotRepositoryFinishFailuresAreClosed(t *testing.T) {
	for _, test := range []struct {
		name       string
		finish     []authority.FinishCode
		wantScans  int
		wantFinish int
	}{
		{name: "unavailable", finish: []authority.FinishCode{authority.FinishUnavailable}, wantScans: 1, wantFinish: 1},
		{name: "changed twice", finish: []authority.FinishCode{authority.FinishChanged, authority.FinishChanged}, wantScans: 2, wantFinish: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			var authorities []*fakeBridgeAuthority
			for _, code := range test.finish {
				authorities = append(authorities, &fakeBridgeAuthority{finishCode: code})
			}
			attempts, scans := 0, 0
			dependencies := bridgeDependencies{
				newRepositoryBudget: repository.NewBudget,
				newSourceBudget:     source.NewBudget,
				newAttempt: func(string, *repository.Budget) (authority.Authority, error) {
					result := authorities[attempts]
					attempts++
					return result, nil
				},
				scan: func(context.Context, adapters.ScanRequest) ([]byte, error) {
					scans++
					return []byte("discarded"), nil
				},
			}
			_, err := compileSnapshotWith(context.Background(), compileRequest{Root: "/repo"}, dependencies)
			assertDashboardErrorCode(t, err, errorRepository)
			if scans != test.wantScans {
				t.Fatalf("scans=%d, want=%d", scans, test.wantScans)
			}
			finished := 0
			for _, fake := range authorities {
				finished += fake.finishCalls
			}
			if finished != test.wantFinish {
				t.Fatalf("finish calls=%d, want=%d", finished, test.wantFinish)
			}
		})
	}
}

func TestCompileSnapshotRetriesRepositoryStartDriftOnceWithSameBudgets(t *testing.T) {
	t.Run("first changed then stable", func(t *testing.T) {
		stable := &fakeBridgeAuthority{finishCode: authority.FinishStable}
		attempts, scans := 0, 0
		var repositoryBudgets []*repository.Budget
		var sourceBudget *source.Budget
		dependencies := bridgeDependencies{
			newRepositoryBudget: repository.NewBudget,
			newSourceBudget: func(limit uint64) *source.Budget {
				sourceBudget = source.NewBudget(limit)
				if !sourceBudget.Preflight("test-source", 0, 1).Allowed() {
					t.Fatal("source budget preflight failed")
				}
				return sourceBudget
			},
			newAttempt: func(_ string, budget *repository.Budget) (authority.Authority, error) {
				repositoryBudgets = append(repositoryBudgets, budget)
				attempts++
				if attempts == 1 {
					return nil, &repository.Failure{Code: repository.FailureRepositoryChanged, Reason: repository.ReasonChanged}
				}
				return stable, nil
			},
			scan: func(_ context.Context, request adapters.ScanRequest) ([]byte, error) {
				scans++
				if request.SourceBudget == nil || request.SourceBudget == sourceBudget || request.SourceBudget.Used() != 1 {
					t.Fatal("scan attempt did not retain the cumulative source ledger")
				}
				return []byte("accepted"), nil
			},
		}
		encoded, err := compileSnapshotWith(context.Background(), compileRequest{Root: "/repo"}, dependencies)
		if err != nil || string(encoded) != "accepted" || attempts != 2 || scans != 1 || stable.finishCalls != 1 {
			t.Fatalf("encoded=%q err=%v attempts=%d scans=%d finish=%d", encoded, err, attempts, scans, stable.finishCalls)
		}
		if len(repositoryBudgets) != 2 || repositoryBudgets[0] != repositoryBudgets[1] {
			t.Fatal("repository budget changed across start retry")
		}
	})

	t.Run("changed twice", func(t *testing.T) {
		attempts, scans := 0, 0
		dependencies := fakeBridgeDependencies(nil, func(context.Context, adapters.ScanRequest) ([]byte, error) {
			scans++
			return nil, nil
		})
		dependencies.newAttempt = func(string, *repository.Budget) (authority.Authority, error) {
			attempts++
			return nil, &repository.Failure{Code: repository.FailureRepositoryChanged, Reason: repository.ReasonChanged}
		}
		_, err := compileSnapshotWith(context.Background(), compileRequest{Root: "/repo"}, dependencies)
		assertDashboardErrorCode(t, err, errorRepository)
		if attempts != 2 || scans != 0 {
			t.Fatalf("attempts=%d scans=%d", attempts, scans)
		}
	})
}

func TestCompileSnapshotFinishesAfterScanErrorAndPanic(t *testing.T) {
	t.Run("closed scan error", func(t *testing.T) {
		authorityAttempt := &fakeBridgeAuthority{finishCode: authority.FinishStable}
		dependencies := fakeBridgeDependencies(authorityAttempt, func(context.Context, adapters.ScanRequest) ([]byte, error) {
			return nil, &adapters.ScanError{Code: errorResource}
		})
		_, err := compileSnapshotWith(context.Background(), compileRequest{Root: "/repo"}, dependencies)
		assertDashboardErrorCode(t, err, errorResource)
		if authorityAttempt.finishCalls != 1 {
			t.Fatalf("finish calls=%d", authorityAttempt.finishCalls)
		}
	})

	t.Run("panic", func(t *testing.T) {
		authorityAttempt := &fakeBridgeAuthority{finishCode: authority.FinishStable}
		dependencies := fakeBridgeDependencies(authorityAttempt, func(context.Context, adapters.ScanRequest) ([]byte, error) {
			panic("hostile /secret/path")
		})
		defer func() {
			if recover() == nil {
				t.Fatal("scan panic was not propagated to the CLI containment boundary")
			}
			if authorityAttempt.finishCalls != 1 {
				t.Fatalf("finish calls=%d", authorityAttempt.finishCalls)
			}
		}()
		_, _ = compileSnapshotWith(context.Background(), compileRequest{Root: "/repo"}, dependencies)
	})
}

func TestCompileSnapshotMapsAttemptAndCancellationFailures(t *testing.T) {
	t.Run("attempt initialization", func(t *testing.T) {
		dependencies := fakeBridgeDependencies(nil, nil)
		dependencies.newAttempt = func(string, *repository.Budget) (authority.Authority, error) {
			return nil, &repository.Failure{Code: repository.FailureRepositoryUnavailable, Reason: repository.ReasonUnavailable}
		}
		_, err := compileSnapshotWith(context.Background(), compileRequest{Root: "/repo"}, dependencies)
		assertDashboardErrorCode(t, err, errorRepository)
	})

	t.Run("parent cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		attempts := 0
		dependencies := fakeBridgeDependencies(nil, nil)
		dependencies.newAttempt = func(string, *repository.Budget) (authority.Authority, error) {
			attempts++
			return nil, errors.New("must not run")
		}
		_, err := compileSnapshotWith(ctx, compileRequest{Root: "/repo"}, dependencies)
		assertDashboardErrorCode(t, err, errorInterrupted)
		if attempts != 0 {
			t.Fatalf("attempts=%d", attempts)
		}
	})
}

func fakeBridgeDependencies(repositoryAuthority authority.Authority, scan func(context.Context, adapters.ScanRequest) ([]byte, error)) bridgeDependencies {
	if scan == nil {
		scan = func(context.Context, adapters.ScanRequest) ([]byte, error) { return nil, nil }
	}
	return bridgeDependencies{
		newRepositoryBudget: repository.NewBudget,
		newSourceBudget:     source.NewBudget,
		newAttempt: func(string, *repository.Budget) (authority.Authority, error) {
			return repositoryAuthority, nil
		},
		scan: scan,
	}
}

func assertDashboardErrorCode(t testing.TB, err error, want string) {
	t.Helper()
	var failure *dashboardError
	if !errors.As(err, &failure) || failure.code != want {
		t.Fatalf("error=%v, want=%s", err, want)
	}
}
