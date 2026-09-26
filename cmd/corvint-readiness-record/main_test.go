package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSRRV1012ModesTakeTheirOwnFlagsOnly(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "readiness.json")
	for name, arguments := range map[string][]string{
		"no flags":             nil,
		"build without output": {"-candidate", directory, "-source-root", directory, "-evidence-file", "e.tsv"},
		"build without source": {"-candidate", directory, "-evidence-file", "e.tsv", "-output", output},
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
