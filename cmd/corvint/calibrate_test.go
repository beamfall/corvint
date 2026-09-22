package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

func calibrateRepository(t *testing.T, records int) string {
	t.Helper()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	outcomes := []string{"passed", "failed", "blocked"}
	for index := 0; index < records; index++ {
		_, err := tracerecordrepo.Record(context.Background(), root, tracerecordrepo.Input{
			Task:         fmt.Sprintf("Split demux key %d", index),
			OpenedPaths:  []string{"cache/demux.go"},
			ChangedPaths: []string{"cache/reader.go"},
			Verification: []string{"go test ./..."},
			Outcome:      outcomes[index%len(outcomes)],
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runCalibrateForTest(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := runCalibrate(t.Context(), arguments, stdout, stderr)
	return code, stdout.String(), stderr.String()
}

func calibrateTreeDigest(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		fmt.Fprintf(digest, "%s\x00%x\x00", path, sha256.Sum256(content))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// OCL-V0-001: the verb accepts `--root`, `--since`, `--window` and `--format`,
// refuses anything else, and refuses the two mutually exclusive windows.
func TestCalibrateInvocationFlags_OCL001(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 1)
	if !calibrateInvoked([]string{"--root", root, "calibrate"}) || calibrateInvoked([]string{"--root", root, "query"}) {
		t.Fatal("verb selection")
	}
	invocation, err := parseCalibrateInvocation([]string{"--root", root, "calibrate", "--window", "5", "--format", "table"})
	if err != nil || invocation.window != 5 || invocation.format != "table" {
		t.Fatalf("%+v %v", invocation, err)
	}
	since, err := parseCalibrateInvocation([]string{"--root=" + root, "calibrate", "--since=abc123"})
	if err != nil || since.since != "abc123" || since.format != "json" {
		t.Fatalf("%+v %v", since, err)
	}
	for _, arguments := range [][]string{
		{"--root", root, "calibrate", "--limit", "5"},
		{"--root", root, "calibrate", "--window", "0"},
		{"--root", root, "calibrate", "--format", "yaml"},
		{"--root", root, "calibrate", "--since", "abc", "--window", "5"},
	} {
		if _, err := parseCalibrateInvocation(arguments); err == nil {
			t.Fatalf("accepted %v", arguments)
		}
		if code, _, stderr := runCalibrateForTest(t, arguments...); code != 2 || !strings.Contains(stderr, "\"ok\": false") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	}
}

// OCL-V0-002: calibrate is a read command; nothing under the root changes.
func TestCalibrateWritesNothing_OCL002(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 4)
	before := calibrateTreeDigest(t, root)
	code, stdout, stderr := runCalibrateForTest(t, "--root", root, "calibrate")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if calibrateTreeDigest(t, root) != before {
		t.Fatal("calibrate mutated the worktree or the local trace store")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["mutates"] != false || payload["tool"] != "calibrate" {
		t.Fatalf("payload: %v", payload)
	}
}

// OCL-V0-003: recorded outcomes join the report, and because the record carries
// no packet stance every row is counted as unknown with its reason.
func TestCalibrateJoinsRecordedOutcomesAsUnknown_OCL003(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 6)
	code, stdout, stderr := runCalibrateForTest(t, "--root", root, "calibrate")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["sample_size"] != 6.0 {
		t.Fatalf("sample: %v", payload["sample_size"])
	}
	unknown := payload["unknown"].(map[string]any)
	if unknown["total"] != 6.0 || unknown["success"] != 2.0 || unknown["failure"] != 4.0 {
		t.Fatalf("unknown: %v", unknown)
	}
	if payload["abstention_accuracy"] != nil || payload["unknown_reason"] == nil || payload["score_coverage"] != 0.0 {
		t.Fatalf("payload invented a stance signal: %v", payload)
	}
}

// OCL-V0-007: a local sample below the minimum carries no proposal.
func TestCalibrateInsufficientLocalSample_OCL007(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 3)
	_, stdout, _ := runCalibrateForTest(t, "--root", root, "calibrate")
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["insufficient_sample"] != true || payload["proposal"] != nil || payload["minimum_sample"] != 20.0 {
		t.Fatalf("payload: %v", payload)
	}
}

// OCL-V0-008: both formats are deterministic, `--window` bounds the sample, and
// `--since` on a revision the local store does not hold selects nothing.
func TestCalibrateOutputIsDeterministic_OCL008(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 5)
	_, first, _ := runCalibrateForTest(t, "--root", root, "calibrate")
	_, second, _ := runCalibrateForTest(t, "--root", root, "calibrate")
	if first != second || !strings.HasSuffix(first, "\n") {
		t.Fatal("json output is not deterministic")
	}
	code, table, stderr := runCalibrateForTest(t, "--root", root, "calibrate", "--format", "table")
	if code != 0 || !strings.Contains(table, "sample_size 5 (minimum 20)") || !strings.Contains(table, "abstention_accuracy null") {
		t.Fatalf("code=%d stderr=%s table=%s", code, stderr, table)
	}
	_, windowed, _ := runCalibrateForTest(t, "--root", root, "calibrate", "--window", "2", "--format", "table")
	if !strings.Contains(windowed, "sample_size 2 ") {
		t.Fatalf("window ignored: %s", windowed)
	}
	_, foreign, _ := runCalibrateForTest(t, "--root", root, "calibrate", "--since", "0000000000000000000000000000000000000000", "--format", "table")
	if !strings.Contains(foreign, "sample_size 0 ") {
		t.Fatalf("since selected unpinned history: %s", foreign)
	}
}

// OCL-V0: ctx is main's signal context, so a cancellation already in flight
// when the verb starts aborts the index load instead of completing it.
func TestRunCalibrateHonorsCancellation(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 3)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	code := runContext(ctx, []string{"--root", root, "calibrate"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "cancel") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}
