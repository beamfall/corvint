package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TestSTRV0073DryRunNamesHeavyWholeReason pins V1-3120: the dry-run plan
// prints one ready, single-quoted HEAVY_WHOLE_REASON line for each go test it
// runs whole, built from the units' own reasons (or the plan's), and none for
// a -run slice or a non-Go declared command.
func TestSTRV0073DryRunNamesHeavyWholeReason(t *testing.T) {
	planned := []sessiontest.Planned{
		{Adapter: "go", Argv: []string{"go", "test", "-json", "example.com/a", "example.com/b"}, Units: []sessiontest.Unit{
			{Unit: "go:example.com/a", Reason: "whole: DIRECT_SOURCE_CHANGE via a.go"},
			{Unit: "go:example.com/b", Reason: "whole: it's a dependency"},
		}},
		{Adapter: "go", Argv: []string{"go", "test", "-json", "-run", "^TestX$", "example.com/c"}, Units: []sessiontest.Unit{{Unit: "go:example.com/c", Reason: "slice"}}},
		{Adapter: "go", Argv: []string{"go", "test", "-json", "-run=^TestY$", "example.com/d"}, Units: []sessiontest.Unit{{Unit: "go:example.com/d"}}},
		{Adapter: "declared", Argv: []string{"make", "shell-test"}, Units: []sessiontest.Unit{{Unit: "declared:shell"}}},
		{Adapter: "go", Argv: []string{"go", "test", "-json", "example.com/e"}, Units: []sessiontest.Unit{{Unit: "go:example.com/e"}}},
	}
	var stderr bytes.Buffer
	testWholeReasons(planned, "FULL: no merge base", &stderr)
	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	want := []string{
		`corvint-test: whole-package go test planned; under script/heavy.sh run it with HEAVY_WHOLE_REASON='corvint test selected example.com/a example.com/b whole: whole: DIRECT_SOURCE_CHANGE via a.go; whole: it'\''s a dependency' (STR-V0-073)`,
		`corvint-test: whole-package go test planned; under script/heavy.sh run it with HEAVY_WHOLE_REASON='corvint test selected example.com/e whole: FULL: no merge base' (STR-V0-073)`,
	}
	if len(lines) != len(want) {
		t.Fatalf("stderr lines = %d, want %d:\n%s", len(lines), len(want), stderr.String())
	}
	for index := range want {
		if lines[index] != want[index] {
			t.Errorf("line %d:\n got %s\nwant %s", index, lines[index], want[index])
		}
	}
}
