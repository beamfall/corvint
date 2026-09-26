package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/releasecandidate"
)

func TestSRRV1012ModesTakeTheirOwnFlagsOnly(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "readiness.json")
	for name, arguments := range map[string][]string{
		"no flags":             nil,
		"build without output": {"-candidate", directory, "-source-root", directory, "-evidence-file", "e.tsv"},
		"build without source": {"-candidate", directory, "-evidence-file", "e.tsv", "-output", output},
		"store release alone":  {"-candidate", directory, "-source-root", directory, "-evidence-file", "e.tsv", "-output", output, "-store-release", "v1-0"},
		"store digest alone":   {"-candidate", directory, "-source-root", directory, "-evidence-file", "e.tsv", "-output", output, "-store-candidate-sha256", strings.Repeat("a", 64)},
		"verify with output":   {"-verify", "r.json", "-candidate", directory, "-evidence-file", "e.tsv", "-output", output},
		"verify with source":   {"-verify", "r.json", "-candidate", directory, "-evidence-file", "e.tsv", "-source-root", directory},
		"verify with store":    {"-verify", "r.json", "-candidate", directory, "-evidence-file", "e.tsv", "-store-release", "v1-0"},
		"positional argument":  {"-verify", "r.json", "-candidate", directory, "-evidence-file", "e.tsv", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(arguments, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("%s: exit %d stdout %q", name, code, stdout.String())
		}
	}
	var stdout, stderr bytes.Buffer
	arguments := []string{"-candidate", directory, "-source-root", directory, "-evidence-file", filepath.Join(directory, "missing.tsv"), "-output", output}
	if code := run(arguments, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "missing.tsv") {
		t.Fatalf("missing evidence file: exit %d stderr %q", code, stderr.String())
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatalf("refused build wrote %s: %v", output, err)
	}
}

func TestSRRV1012ReportNamesRecordAndEveryRow(t *testing.T) {
	record := &releasecandidate.ReadinessRecord{
		Identity: releasecandidate.ReadinessIdentity{Version: "1.0.0-rc.1", Commit: strings.Repeat("c", 40)},
		Rows:     []releasecandidate.ReadinessRow{{ID: "gate/full-gate", Status: "PASS"}, {ID: "owner/tag", Status: "NOT_RUN"}},
	}
	var stdout, stderr bytes.Buffer
	if code := report(&stdout, &stderr, "r.json", record, nil); code != 0 || stderr.Len() != 0 {
		t.Fatalf("report exit %d stderr %q", code, stderr.String())
	}
	want := "record: r.json\nversion: 1.0.0-rc.1\ncommit: " + strings.Repeat("c", 40) + "\ngate/full-gate: PASS\nowner/tag: NOT_RUN\n"
	if stdout.String() != want {
		t.Fatalf("report = %q, want %q", stdout.String(), want)
	}
	stdout.Reset()
	if code := report(&stdout, &stderr, "r.json", nil, errors.New("refused")); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "refused") {
		t.Fatalf("refusal: exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}
