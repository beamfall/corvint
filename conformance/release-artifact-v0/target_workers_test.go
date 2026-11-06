// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// GAG-V0-9960: overlap is observable, with no more than the selected worker bound.
func TestTargetWorkersBoundedOverlap(t *testing.T) {
	t.Setenv("CORVINT_ARCHIVE_WORKERS", "2")
	started := make(chan int, 5)
	release := make(chan struct{})
	var active, peak atomic.Int32
	done := make(chan []targetResult[int], 1)
	go func() {
		done <- runTargetWorkers(workerTestContext(t), 5, func(ctx context.Context, index int) (int, error) {
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			started <- index
			<-release
			active.Add(-1)
			return index, nil
		})
	}()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("two targets did not overlap")
		}
	}
	select {
	case index := <-started:
		t.Fatalf("third target %d exceeded the bound", index)
	default:
	}
	close(release)
	select {
	case results := <-done:
		for index, result := range results {
			if result.err != nil || result.value != index {
				t.Fatalf("result %d: %+v", index, result)
			}
		}
		if len(results) != 5 || peak.Load() != 2 || active.Load() != 0 {
			t.Fatalf("results=%d peak=%d active=%d", len(results), peak.Load(), active.Load())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("workers did not retire")
	}
}

func TestTargetWorkerOverride(t *testing.T) {
	// GAG-V0-9960: malformed overrides leave the CPU-derived default intact.
	for _, tc := range []struct {
		value string
		want  int
	}{
		{"", min(runtime.NumCPU(), 5)}, {"0", min(runtime.NumCPU(), 5)},
		{"-2", min(runtime.NumCPU(), 5)}, {"bad", min(runtime.NumCPU(), 5)},
		{"1", 1}, {"2", 2}, {"999999", 5},
	} {
		t.Setenv("CORVINT_ARCHIVE_WORKERS", tc.value)
		if got := targetWorkers(5); got != tc.want {
			t.Fatalf("override %q: %d want %d", tc.value, got, tc.want)
		}
	}
}

func workerManifest(t *testing.T) Manifest {
	t.Helper()
	manifest, err := loadManifest("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

// GAG-V0-9961: completion order cannot change loose JSON, summaries or FAIL lines.
// Report-level findings continue collecting, exactly as the original loose loop did.
func TestParallelLooseReportMatchesSerial(t *testing.T) {
	manifest := workerManifest(t)
	output := t.TempDir()
	var serial Report
	var serialChecksums []byte
	for _, workers := range []string{"1", "5"} {
		t.Setenv("CORVINT_ARCHIVE_WORKERS", workers)
		last := make(chan struct{})
		report := Report{Commit: "commit", build: "123"}
		err := buildTargetsUsing(workerTestContext(t), Options{Output: output}, manifest, &report,
			func(ctx context.Context, options Options, manifest Manifest, target Target, local *Report) (TargetReport, error) {
				index := targetIndex(manifest, target)
				if workers == "5" {
					if index == 4 {
						close(last)
					} else {
						waitForWorker(ctx, last)
					}
				}
				if index == 2 || index == 4 {
					local.fail(reasonBuildFailed, target.id(), "injected build failure")
				}
				return TargetReport{GOOS: target.GOOS, GOARCH: target.GOARCH, SHA256: strconv.Itoa(index), Artifact: filepath.Join(output, target.id())}, nil
			})
		if err != nil {
			t.Fatal(err)
		}
		report.Verdict = verdict(report)
		checksums, err := os.ReadFile(filepath.Join(output, "SHA256SUMS"))
		if err != nil {
			t.Fatal(err)
		}
		if workers == "1" {
			serial = report
			serialChecksums = checksums
			continue
		}
		if !reflect.DeepEqual(report, serial) || !bytes.Equal(checksums, serialChecksums) {
			t.Fatalf("parallel report/checksums differ: %+v vs %+v", report, serial)
		}
		if looseGateFailure(report).Error() != looseGateFailure(serial).Error() {
			t.Fatal("loose stderr differs")
		}
		var a, b bytes.Buffer
		writeSummary(&a, serial)
		writeSummary(&b, report)
		if a.String() != b.String() || len(report.Targets) != 5 || len(report.Reasons) != 2 {
			t.Fatalf("summary/findings changed: %s / %s", a.String(), b.String())
		}
	}
}

func targetIndex(manifest Manifest, target Target) int {
	for index, candidate := range manifest.Targets {
		if candidate == target {
			return index
		}
	}
	panic("unknown target")
}

// GAG-V0-9961: a higher-index error arrives first, but the original serial error
// and its exact target/classification win; suffix candidates are never retained.
func TestParallelArchiveFailureMatchesSerial(t *testing.T) {
	manifest := workerManifest(t)
	middleErr := errors.New("double-build-mismatch: injected middle target")
	var serial ArchiveReport
	for _, workers := range []string{"1", "5"} {
		t.Setenv("CORVINT_ARCHIVE_WORKERS", workers)
		highFailed := make(chan struct{})
		report, retained, err := buildArchiveTargets(workerTestContext(t), manifest, ArchiveReport{Verdict: statusFail}, func(ctx context.Context, index int) (archiveTargetResult, error) {
			target := manifest.Targets[index]
			result := archiveTargetResult{entry: ArchiveTargetReport{GOOS: target.GOOS, GOARCH: target.GOARCH, ArchiveName: target.id()}, content: []byte{byte(index)}, limit: int64(index + 1)}
			if index == 4 {
				close(highFailed)
				return result, errors.New("double-assembly-mismatch: later failure")
			}
			if index == 2 {
				if workers == "5" {
					waitForWorker(ctx, highFailed)
				}
				return result, middleErr
			}
			if workers == "5" && index < 2 {
				waitForWorker(ctx, highFailed)
			}
			return result, nil
		})
		if err != middleErr || retained != nil {
			t.Fatalf("error=%v retained=%v", err, retained)
		}
		if len(report.Targets) != 3 || len(report.Reasons) != 1 || report.Reasons[0].Code != "double-build-mismatch" || report.Reasons[0].Target != manifest.Targets[2].id() {
			t.Fatalf("wrong serial prefix: %+v", report)
		}
		if workers == "1" {
			serial = report
		} else if !reflect.DeepEqual(report, serial) {
			t.Fatalf("parallel report differs: %+v vs %+v", report, serial)
		}
	}
}

func TestParallelArchiveSuccessMatchesSerial(t *testing.T) {
	// GAG-V0-9961: retained bytes, ordering and aggregate resource ceilings agree.
	manifest := workerManifest(t)
	var serial ArchiveReport
	var serialRetained map[string][]byte
	for _, workers := range []string{"1", "5"} {
		t.Setenv("CORVINT_ARCHIVE_WORKERS", workers)
		last := make(chan struct{})
		report, retained, err := buildArchiveTargets(workerTestContext(t), manifest, ArchiveReport{}, func(ctx context.Context, index int) (archiveTargetResult, error) {
			if workers == "5" {
				if index == 4 {
					close(last)
				} else {
					waitForWorker(ctx, last)
				}
			}
			target := manifest.Targets[index]
			return archiveTargetResult{entry: ArchiveTargetReport{GOOS: target.GOOS, GOARCH: target.GOARCH, ArchiveName: target.id()}, content: []byte{byte(index)}, limit: int64(index + 1)}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if workers == "1" {
			serial = report
			serialRetained = retained
			continue
		}
		if !reflect.DeepEqual(report, serial) || !reflect.DeepEqual(retained, serialRetained) || report.Limits.MaximumTotalBytes != 15 || report.Limits.MaximumArchiveBytes != 5 {
			t.Fatalf("parallel aggregation differs: %+v", report)
		}
	}
}

func TestTargetWorkersCancelOnlySuffixAndJoin(t *testing.T) {
	// GAG-V0-9962: cancel later work immediately without erasing a lower failure.
	t.Setenv("CORVINT_ARCHIVE_WORKERS", "5")
	allStarted := make(chan struct{})
	var started atomic.Int32
	var joined atomic.Int32
	highFailed := make(chan struct{})
	suffixCanceled := make(chan struct{})
	lowErr := fmt.Errorf("lower-index failure")
	results := runTargetWorkers(workerTestContext(t), 5, func(ctx context.Context, index int) (int, error) {
		if started.Add(1) == 5 {
			close(allStarted)
		}
		waitForWorker(ctx, allStarted)
		defer joined.Add(1)
		switch index {
		case 0:
			waitForWorker(ctx, suffixCanceled)
			if ctx.Err() != nil {
				return index, ctx.Err()
			}
			return index, nil
		case 1:
			waitForWorker(ctx, suffixCanceled)
			return index, lowErr
		case 2:
			close(highFailed)
			return index, errors.New("earlier in time, later in manifest")
		case 3:
			waitForWorker(ctx, highFailed)
			<-ctx.Done()
			close(suffixCanceled)
			return index, ctx.Err()
		default:
			<-ctx.Done()
			return index, ctx.Err()
		}
	})
	if len(results) != 2 || results[1].err != lowErr || results[0].err != nil || joined.Load() != 5 {
		t.Fatalf("prefix=%+v joined=%d", results, joined.Load())
	}
}

func TestTargetWorkersParentCancellation(t *testing.T) {
	// GAG-V0-9962: parent cancellation reaches every active target and is joined.
	t.Setenv("CORVINT_ARCHIVE_WORKERS", "2")
	ctx, cancel := context.WithCancel(workerTestContext(t))
	defer cancel()
	var started atomic.Int32
	var joined atomic.Int32
	results := runTargetWorkers(ctx, 5, func(ctx context.Context, index int) (int, error) {
		if started.Add(1) == 2 {
			cancel()
		}
		<-ctx.Done()
		joined.Add(1)
		return index, ctx.Err()
	})
	if len(results) != 1 || !errors.Is(results[0].err, context.Canceled) || joined.Load() != 2 {
		t.Fatalf("results=%+v joined=%d", results, joined.Load())
	}
}

func TestTargetWorkersEmpty(t *testing.T) {
	ran := false
	got := runTargetWorkers(workerTestContext(t), 0, func(context.Context, int) (int, error) { ran = true; return 0, nil })
	if len(got) != 0 || ran {
		t.Fatalf("empty work ran: %v", got)
	}
}

func workerTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func waitForWorker(ctx context.Context, ready <-chan struct{}) {
	select {
	case <-ready:
	case <-ctx.Done():
	}
}
