//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSTRV0073CommandNamesHeavyWholeReasonInBothModes drives the real
// `corvint test` command, not only the helper: with --dry-run and when it
// executes, a change to package b's source runs b whole, so stderr carries
// b's ready HEAVY_WHOLE_REASON line, and stdout never does (the dry-run JSON
// plan is unchanged; V1-3120 review).
func TestSTRV0073CommandNamesHeavyWholeReasonInBothModes(t *testing.T) {
	root, base := sliceFixture(t)
	if err := os.WriteFile(filepath.Join(root, "b/b.go"), []byte("package b\n\nimport \"example.test/m/a\"\n\nfunc B() int { return a.A() + 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "corvint-test: whole-package go test planned; under script/heavy.sh run it with HEAVY_WHOLE_REASON='corvint test selected example.test/m/b whole"
	for _, mode := range []struct {
		name string
		args []string
	}{
		{"dry-run", []string{"--root", root, "test", "--base", base, "--dry-run"}},
		{"execute", []string{"--root", root, "test", "--base", base}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, mode.args...)
			if code != 0 {
				t.Fatalf("code %d stderr %q", code, stderr)
			}
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr lacks the HEAVY_WHOLE_REASON line %q:\n%s", want, stderr)
			}
			if strings.Contains(stdout, "HEAVY_WHOLE_REASON") {
				t.Fatalf("stdout carries the reason line:\n%s", stdout)
			}
			if mode.name == "dry-run" {
				var plan map[string]any
				if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
					t.Fatalf("dry-run stdout is not the JSON plan: %v\n%s", err, stdout)
				}
			}
		})
	}
}
