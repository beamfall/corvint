package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/releasecandidate"
)

func TestSRRV1012ModesTakeTheirOwnFlagsOnly(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "readiness.json")
	for name, arguments := range map[string][]string{
		"no flags":              nil,
		"build without output":  {"-candidate", directory, "-source-root", directory, "-evidence-file", "e.tsv"},
		"build without source":  {"-candidate", directory, "-evidence-file", "e.tsv", "-output", output},
		"store release alone":   {"-candidate", directory, "-source-root", directory, "-evidence-file", "e.tsv", "-output", output, "-store-release", "v1-0"},
		"store digest alone":    {"-candidate", directory, "-source-root", directory, "-evidence-file", "e.tsv", "-output", output, "-store-candidate-sha256", strings.Repeat("a", 64)},
		"verify with output":    {"-verify", "r.json", "-candidate", directory, "-evidence-file", "e.tsv", "-output", output},
		"verify with source":    {"-verify", "r.json", "-candidate", directory, "-evidence-file", "e.tsv", "-source-root", directory},
		"verify with store":     {"-verify", "r.json", "-candidate", directory, "-evidence-file", "e.tsv", "-store-release", "v1-0"},
		"positional argument":   {"-verify", "r.json", "-candidate", directory, "-evidence-file", "e.tsv", "extra"},
		"empty verify":          {"-verify", "", "-candidate", directory, "-evidence-file", "e.tsv"},
		"empty verify to build": {"-verify", "", "-candidate", directory, "-source-root", directory, "-evidence-file", "e.tsv", "-output", output},
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

func TestSRRV1012RunPassesEachFlagToItsMode(t *testing.T) {
	directory := t.TempDir()
	evidenceFile := filepath.Join(directory, "evidence.tsv")
	if err := os.WriteFile(evidenceFile, []byte("gate/full-gate\tPASS\tgate.log\t\t\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	evidence := map[string]releasecandidate.ReadinessEvidence{"gate/full-gate": {Status: "PASS", Path: filepath.Join(directory, "gate.log")}}
	record := &releasecandidate.ReadinessRecord{Identity: releasecandidate.ReadinessIdentity{Version: "1.0.0-rc.1", Commit: strings.Repeat("c", 40)}}
	previousWrite, previousVerify := writeRecord, verifyRecord
	t.Cleanup(func() { writeRecord, verifyRecord = previousWrite, previousVerify })
	var built []any
	writeRecord = func(_ context.Context, options releasecandidate.ReadinessOptions, output string) (*releasecandidate.ReadinessRecord, error) {
		built = []any{options, output}
		return record, nil
	}
	var verified []any
	verifyRecord = func(_ context.Context, path, candidate string, evidence map[string]releasecandidate.ReadinessEvidence) (*releasecandidate.ReadinessRecord, error) {
		verified = []any{path, candidate, evidence}
		return record, nil
	}
	digest := strings.Repeat("a", 64)
	var stdout, stderr bytes.Buffer
	arguments := []string{"-candidate", "candidate", "-source-root", "source", "-evidence-file", evidenceFile, "-store-release", "v1-0", "-store-candidate-sha256", digest, "-output", "out.json"}
	if code := run(arguments, &stdout, &stderr); code != 0 || !strings.HasPrefix(stdout.String(), "record: out.json\n") {
		t.Fatalf("build exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	options := releasecandidate.ReadinessOptions{CandidateDirectory: "candidate", SourceRoot: "source", StoreReleaseID: "v1-0", StoreCandidateSHA256: digest, Evidence: evidence}
	if want := []any{options, "out.json"}; !reflect.DeepEqual(built, want) || verified != nil {
		t.Fatalf("build passed %#v, want %#v; verify called with %#v", built, want, verified)
	}
	stdout.Reset()
	built = nil
	if code := run([]string{"-verify", "r.json", "-candidate", "candidate", "-evidence-file", evidenceFile}, &stdout, &stderr); code != 0 || !strings.HasPrefix(stdout.String(), "record: r.json\n") {
		t.Fatalf("verify exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if want := []any{"r.json", "candidate", evidence}; !reflect.DeepEqual(verified, want) || built != nil {
		t.Fatalf("verify passed %#v, want %#v; build called with %#v", verified, want, built)
	}
}
