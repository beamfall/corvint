package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PRB-V0-029: the command exits 2, distinct from a budget failure's 1, when the harness cannot
// run: bad arguments, an unreadable budgets file, or a scale other than the budgets'.
func TestPRBV0029HarnessCannotRunExitsTwo(t *testing.T) {
	smoke := filepath.Join(t.TempDir(), "budgets.json")
	committed, err := os.ReadFile(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(smoke, committed, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"unknown flag":    {"--bogus"},
		"positional":      {"extra"},
		"missing budgets": {"--budgets", filepath.Join(t.TempDir(), "absent.json")},
		"unknown scale":   {"--budgets", smoke, "--scale", "huge"},
		"scale mismatch":  {"--budgets", smoke, "--scale", "smoke"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Errorf("%s: exit %d stdout %q", name, code, &stdout)
		}
		if name != "unknown flag" && name != "positional" && !strings.HasPrefix(stderr.String(), "perf-budget: ") {
			t.Errorf("%s: stderr %q", name, &stderr)
		}
	}
}
