package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNTPV0008CommandBoundary(t *testing.T) {
	t.Run("NTP-V0-008 explicit fixture command", func(t *testing.T) {
		for _, args := range [][]string{
			{"plan-fixture"},
			{"plan-fixture", "--executor", "relative", "--observations", "missing"},
			{"plan-fixture", "--executor", "/missing", "--executor", "/another"},
			{"plan-fixture", "--unknown", "value"},
		} {
			var out, err bytes.Buffer
			if code := runWork(context.Background(), t.TempDir(), args, &out, &err); code != 2 || out.Len() != 0 || !strings.HasPrefix(err.String(), "taskman fixture:") {
				t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, out.String(), err.String())
			}
		}
		if _, err := parseWorkOptions([]string{"observe"}); err != nil {
			t.Fatal(err)
		}
		if _, err := parseWorkOptions([]string{"observe", "--executor", "/fixture"}); err == nil {
			t.Fatal("native fixture flags admitted to WQO")
		}
	})
}

func TestTaskmanFixtureWriteFailureReportsDiagnostic(t *testing.T) {
	previous := taskmanPreview
	taskmanPreview = func(context.Context, string, string, string) ([]byte, error) { return []byte("{}\n"), nil }
	t.Cleanup(func() { taskmanPreview = previous })
	var out stdoutBrokenPipeWriter
	var err bytes.Buffer
	code := runWork(context.Background(), t.TempDir(), []string{"plan-fixture", "--executor", "/fixture", "--observations", "observations.json"}, &out, &err)
	if code != 2 || !strings.Contains(err.String(), `"output-failed"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), err.String())
	}
}

// TestNTPV0246PlanFixtureHelpStatesBound pins that plan-fixture --help states
// the ticket bound and answers without launching an executor (NTP-V0-246).
func TestNTPV0246PlanFixtureHelpStatesBound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runTaskmanFixture(context.Background(), t.TempDir(), []string{"--help"}, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "at most 16384 tickets") || !strings.Contains(stdout.String(), "16384 plan entries") {
		t.Fatalf("help lacks the bound:\n%s", stdout.String())
	}
}
