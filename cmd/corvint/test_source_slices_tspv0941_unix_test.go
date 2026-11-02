//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TSP-V0-941: compiled positions are observable, so an edit that shifts the
// line of an unchanged function seeds that function, and one that shifts a
// method, init or var declaration keeps the package whole. TestLine pins the
// line runtime.Caller reports and must stay selected (V1-3507).
func TestTSPV0941DryRunSeedsMovedDeclarations(t *testing.T) {
	root, _ := sourceSliceFixture(t)
	line := "package l\n\nimport \"runtime\"\n\nfunc Other() int { return 1 }\n\n" +
		"func Line() int { _, _, n, _ := runtime.Caller(0); return n }\n\nfunc Unused() int { return 3 }\n"
	here := "package h\n\nimport \"runtime\"\n\nvar Here = here()\n\n" +
		"func here() int { _, _, n, _ := runtime.Caller(0); return n }\n\nfunc Plain() int { return 1 }\n"
	fixture := map[string]string{
		"l/l.go": line,
		"l/l_test.go": "package l\n\nimport \"testing\"\n\n" +
			"func TestLine(t *testing.T) { if Line() != 7 { t.Fatal(Line()) } }\n\n" +
			"func TestOther(t *testing.T) { _ = Other() }\n\nfunc TestKeep(t *testing.T) {}\n",
		"h/h.go": here,
		"h/h_test.go": "package h\n\nimport \"testing\"\n\n" +
			"func TestHere(t *testing.T) { if Here != 5 { t.Fatal(Here) } }\n\nfunc TestPlain(t *testing.T) { _ = Plain() }\n",
	}
	for name, body := range fixture {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "positions")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	plan := func(t *testing.T, edits map[string]string) map[string]sessiontest.Unit {
		t.Helper()
		affectedGit(t, root, "checkout", "-q", "--", ".")
		affectedGit(t, root, "clean", "-qfd")
		for name, body := range edits {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", filepath.Join(t.TempDir(), "m.json"))
		if code != 0 {
			t.Fatalf("code %d stderr %q", code, stderr)
		}
		var printed struct {
			Invocations []struct {
				Units []sessiontest.Unit `json:"units"`
			} `json:"invocations"`
		}
		if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
			t.Fatal(err)
		}
		units := make(map[string]sessiontest.Unit)
		for _, invocation := range printed.Invocations {
			for _, unit := range invocation.Units {
				units[unit.Unit] = unit
			}
		}
		return units
	}
	const lineID = "go:example.test/m/l"
	for _, tc := range []struct {
		name, pattern, reason string
		edits                 map[string]string
	}{
		{"a const inserted above", "^(TestLine)$",
			"selected: the tests reaching Extra, Line, Unused changed in l/l.go (Line, Unused moved to another line)",
			map[string]string{"l/l.go": strings.Replace(line, "func Line", "const Extra = 1\n\nfunc Line", 1)}},
		{"a body edit that adds lines", "^(TestLine|TestOther)$",
			"selected: the tests reaching Line, Other, Unused changed in l/l.go (Line, Unused moved to another line)",
			map[string]string{"l/l.go": strings.Replace(line, "{ return 1 }", "{\n\treturn 1\n}", 1)}},
		{"an edit below leaves it unmoved", "^$",
			"selected: no test reaches Unused changed in l/l.go; build and vet only",
			map[string]string{"l/l.go": strings.Replace(line, "return 3", "return 4", 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := plan(t, tc.edits)[lineID]
			want := sessiontest.Unit{Adapter: sessiontest.AdapterGo, Flag: "-run", Pattern: tc.pattern, Reason: tc.reason, Slice: sessiontest.SliceSelected, Unit: lineID}
			if got != want {
				t.Fatalf("unit %+v, want %+v", got, want)
			}
		})
	}
	t.Run("a shifted var declaration", func(t *testing.T) {
		got := plan(t, map[string]string{"h/h.go": strings.Replace(here, "package h\n", "// Package h records a line.\npackage h\n", 1)})["go:example.test/m/h"]
		if got.Slice != sessiontest.SliceWhole || got.Pattern != "" || !strings.Contains(got.Reason, "h/h.go shifts the line of a method, init or var declaration") {
			t.Fatalf("unit %+v, want whole naming the shifted var", got)
		}
	})
}
